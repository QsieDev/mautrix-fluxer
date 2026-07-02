// mautrix-fluxer - A Matrix-Fluxer puppeting bridge.
// Copyright (C) 2022 Tulir Asokan
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package main

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
	"maunium.net/go/mautrix/id"

	"github.com/qsiedev/mautrix-fluxer/database"
)

type astFluxerTag struct {
	ast.BaseInline
	portal *Portal
	id     int64
}

var _ ast.Node = (*astFluxerTag)(nil)
var astKindFluxerTag = ast.NewNodeKind("FluxerTag")

func (n *astFluxerTag) Dump(source []byte, level int) {
	ast.DumpHelper(n, source, level, nil, nil)
}

func (n *astFluxerTag) Kind() ast.NodeKind {
	return astKindFluxerTag
}

type astFluxerUserMention struct {
	astFluxerTag
	hasNick bool
}

func (n *astFluxerUserMention) String() string {
	if n.hasNick {
		return fmt.Sprintf("<@!%d>", n.id)
	}
	return fmt.Sprintf("<@%d>", n.id)
}

type astFluxerRoleMention struct {
	astFluxerTag
}

func (n *astFluxerRoleMention) String() string {
	return fmt.Sprintf("<@&%d>", n.id)
}

type astFluxerChannelMention struct {
	astFluxerTag

	guildID int64
	name    string
}

func (n *astFluxerChannelMention) String() string {
	if n.guildID != 0 {
		return fmt.Sprintf("<#%d:%d:%s>", n.id, n.guildID, n.name)
	}
	return fmt.Sprintf("<#%d>", n.id)
}

type fluxerTimestampStyle rune

func (dts fluxerTimestampStyle) Format() string {
	switch dts {
	case 't':
		return "15:04 MST"
	case 'T':
		return "15:04:05 MST"
	case 'd':
		return "2006-01-02 MST"
	case 'D':
		return "2 January 2006 MST"
	case 'F':
		return "Monday, 2 January 2006 15:04 MST"
	case 'f':
		fallthrough
	default:
		return "2 January 2006 15:04 MST"
	}
}

type astFluxerTimestamp struct {
	astFluxerTag

	timestamp int64
	style     fluxerTimestampStyle
}

func (n *astFluxerTimestamp) String() string {
	if n.style == 'f' {
		return fmt.Sprintf("<t:%d>", n.timestamp)
	}
	return fmt.Sprintf("<t:%d:%c>", n.timestamp, n.style)
}

type astFluxerCustomEmoji struct {
	astFluxerTag
	name     string
	animated bool
}

func (n *astFluxerCustomEmoji) String() string {
	if n.animated {
		return fmt.Sprintf("<a%s%d>", n.name, n.id)
	}
	return fmt.Sprintf("<%s%d>", n.name, n.id)
}

type fluxerTagParser struct{}

// Regex to match everything in https://fluxer.app/developers/docs/reference#message-formatting
var fluxerTagRegex = regexp.MustCompile(`<(a?:\w+:|@[!&]?|#|t:)(\d+)(?::([tTdDfFR])|(\d+):(.+?))?>`)
var defaultFluxerTagParser = &fluxerTagParser{}

func (s *fluxerTagParser) Trigger() []byte {
	return []byte{'<'}
}

var parserContextPortal = parser.NewContextKey()

func (s *fluxerTagParser) Parse(parent ast.Node, block text.Reader, pc parser.Context) ast.Node {
	portal := pc.Get(parserContextPortal).(*Portal)
	//before := block.PrecendingCharacter()
	line, _ := block.PeekLine()
	match := fluxerTagRegex.FindSubmatch(line)
	if match == nil {
		return nil
	}
	//seg := segment.WithStop(segment.Start + len(match[0]))
	block.Advance(len(match[0]))

	id, err := strconv.ParseInt(string(match[2]), 10, 64)
	if err != nil {
		return nil
	}
	tag := astFluxerTag{id: id, portal: portal}
	tagName := string(match[1])
	switch {
	case tagName == "@":
		return &astFluxerUserMention{astFluxerTag: tag}
	case tagName == "@!":
		return &astFluxerUserMention{astFluxerTag: tag, hasNick: true}
	case tagName == "@&":
		return &astFluxerRoleMention{astFluxerTag: tag}
	case tagName == "#":
		var guildID int64
		var channelName string
		if len(match[4]) > 0 && len(match[5]) > 0 {
			guildID, _ = strconv.ParseInt(string(match[4]), 10, 64)
			channelName = string(match[5])
		}
		return &astFluxerChannelMention{astFluxerTag: tag, guildID: guildID, name: channelName}
	case tagName == "t:":
		var style fluxerTimestampStyle
		if len(match[3]) == 0 {
			style = 'f'
		} else {
			style = fluxerTimestampStyle(match[3][0])
		}
		return &astFluxerTimestamp{
			astFluxerTag: tag,
			timestamp:    id,
			style:        style,
		}
	case strings.HasPrefix(tagName, ":"):
		return &astFluxerCustomEmoji{name: tagName, astFluxerTag: tag}
	case strings.HasPrefix(tagName, "a:"):
		return &astFluxerCustomEmoji{name: tagName[1:], astFluxerTag: tag, animated: true}
	default:
		return nil
	}
}

func (s *fluxerTagParser) CloseBlock(parent ast.Node, pc parser.Context) {
	// nothing to do
}

type fluxerTagHTMLRenderer struct{}

var defaultFluxerTagHTMLRenderer = &fluxerTagHTMLRenderer{}

func (r *fluxerTagHTMLRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(astKindFluxerTag, r.renderFluxerMention)
}

func relativeTimeFormat(ts time.Time) string {
	now := time.Now()
	if ts.Year() >= 2262 {
		return "date out of range for relative format"
	}
	duration := ts.Sub(now)
	word := "in %s"
	if duration < 0 {
		duration = -duration
		word = "%s ago"
	}
	var count int
	var unit string
	switch {
	case duration < time.Second:
		count = int(duration.Milliseconds())
		unit = "millisecond"
	case duration < time.Minute:
		count = int(math.Round(duration.Seconds()))
		unit = "second"
	case duration < time.Hour:
		count = int(math.Round(duration.Minutes()))
		unit = "minute"
	case duration < 24*time.Hour:
		count = int(math.Round(duration.Hours()))
		unit = "hour"
	case duration < 30*24*time.Hour:
		count = int(math.Round(duration.Hours() / 24))
		unit = "day"
	case duration < 365*24*time.Hour:
		count = int(math.Round(duration.Hours() / 24 / 30))
		unit = "month"
	default:
		count = int(math.Round(duration.Hours() / 24 / 365))
		unit = "year"
	}
	var diff string
	if count == 1 {
		diff = fmt.Sprintf("a %s", unit)
	} else {
		diff = fmt.Sprintf("%d %ss", count, unit)
	}
	return fmt.Sprintf(word, diff)
}

func (r *fluxerTagHTMLRenderer) renderFluxerMention(w util.BufWriter, source []byte, n ast.Node, entering bool) (status ast.WalkStatus, err error) {
	status = ast.WalkContinue
	if !entering {
		return
	}
	switch node := n.(type) {
	case *astFluxerUserMention:
		var mxid id.UserID
		var name string
		if puppet := node.portal.bridge.GetPuppetByID(strconv.FormatInt(node.id, 10)); puppet != nil {
			mxid = puppet.MXID
			name = puppet.Name
		}
		if user := node.portal.bridge.GetUserByID(strconv.FormatInt(node.id, 10)); user != nil {
			mxid = user.MXID
			if name == "" {
				name = user.MXID.Localpart()
			}
		}
		_, _ = fmt.Fprintf(w, `<a href="%s">%s</a>`, mxid.URI().MatrixToURL(), name)
		return
	case *astFluxerRoleMention:
		role := node.portal.bridge.DB.Role.GetByID(node.portal.GuildID, strconv.FormatInt(node.id, 10))
		if role != nil {
			_, _ = fmt.Fprintf(w, `<font color="#%06x"><strong>@%s</strong></font>`, role.Color, role.Name)
			return
		}
	case *astFluxerChannelMention:
		portal := node.portal.bridge.GetExistingPortalByID(database.PortalKey{
			ChannelID: strconv.FormatInt(node.id, 10),
			Receiver:  "",
		})
		if portal != nil {
			if portal.MXID != "" {
				_, _ = fmt.Fprintf(w, `<a href="%s">%s</a>`, portal.MXID.URI(portal.bridge.AS.HomeserverDomain).MatrixToURL(), portal.Name)
			} else {
				_, _ = w.WriteString(portal.Name)
			}
			return
		}
	case *astFluxerCustomEmoji:
		reactionMXC := node.portal.getEmojiMXCByFluxerID(strconv.FormatInt(node.id, 10), node.name, node.animated)
		if !reactionMXC.IsEmpty() {
			attrs := "data-mx-emoticon"
			if node.animated {
				attrs += " data-mau-animated-emoji"
			}
			_, _ = fmt.Fprintf(w, `<img %[3]s src="%[1]s" alt="%[2]s" title="%[2]s" height="32"/>`, reactionMXC.String(), node.name, attrs)
			return
		}
	case *astFluxerTimestamp:
		ts := time.Unix(node.timestamp, 0).UTC()
		var formatted string
		if node.style == 'R' {
			formatted = relativeTimeFormat(ts)
		} else {
			formatted = ts.Format(node.style.Format())
		}
		// https://github.com/matrix-org/matrix-spec-proposals/pull/3160
		const fullDatetimeFormat = "2006-01-02T15:04:05.000-0700"
		fullRFC := ts.Format(fullDatetimeFormat)
		fullHumanReadable := ts.Format(fluxerTimestampStyle('F').Format())
		_, _ = fmt.Fprintf(w, `<time title="%s" datetime="%s" data-fluxer-style="%c"><strong>%s</strong></time>`, fullHumanReadable, fullRFC, node.style, formatted)
	}
	stringifiable, ok := n.(fmt.Stringer)
	if ok {
		_, _ = w.WriteString(stringifiable.String())
	} else {
		_, _ = w.Write(source)
	}
	return
}

type fluxerTag struct{}

var ExtFluxerTag = &fluxerTag{}

func (e *fluxerTag) Extend(m goldmark.Markdown) {
	m.Parser().AddOptions(parser.WithInlineParsers(
		util.Prioritized(defaultFluxerTagParser, 600),
	))
	m.Renderer().AddOptions(renderer.WithNodeRenderers(
		util.Prioritized(defaultFluxerTagHTMLRenderer, 600),
	))
}
