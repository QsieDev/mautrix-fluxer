// mautrix-fluxer - A Matrix-Fluxer puppeting bridge.
// Copyright (C) 2023 Tulir Asokan
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
	"regexp"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

type astFluxerEveryone struct {
	ast.BaseInline
	onlyHere bool
}

var _ ast.Node = (*astFluxerEveryone)(nil)
var astKindFluxerEveryone = ast.NewNodeKind("FluxerEveryone")

func (n *astFluxerEveryone) Dump(source []byte, level int) {
	ast.DumpHelper(n, source, level, nil, nil)
}

func (n *astFluxerEveryone) Kind() ast.NodeKind {
	return astKindFluxerEveryone
}

func (n *astFluxerEveryone) String() string {
	if n.onlyHere {
		return "@here"
	}
	return "@everyone"
}

type fluxerEveryoneParser struct{}

var fluxerEveryoneRegex = regexp.MustCompile(`@(everyone|here)`)
var defaultFluxerEveryoneParser = &fluxerEveryoneParser{}

func (s *fluxerEveryoneParser) Trigger() []byte {
	return []byte{'@'}
}

func (s *fluxerEveryoneParser) Parse(parent ast.Node, block text.Reader, pc parser.Context) ast.Node {
	line, _ := block.PeekLine()
	match := fluxerEveryoneRegex.FindSubmatch(line)
	if match == nil {
		return nil
	}
	block.Advance(len(match[0]))
	return &astFluxerEveryone{
		onlyHere: string(match[1]) == "here",
	}
}

func (s *fluxerEveryoneParser) CloseBlock(parent ast.Node, pc parser.Context) {
	// nothing to do
}

type fluxerEveryoneHTMLRenderer struct{}

func (r *fluxerEveryoneHTMLRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(astKindFluxerEveryone, r.renderFluxerEveryone)
}

func (r *fluxerEveryoneHTMLRenderer) renderFluxerEveryone(w util.BufWriter, source []byte, n ast.Node, entering bool) (status ast.WalkStatus, err error) {
	status = ast.WalkContinue
	if !entering {
		return
	}
	mention, _ := n.(*astFluxerEveryone)
	class := "everyone"
	if mention != nil && mention.onlyHere {
		class = "here"
	}
	_, _ = fmt.Fprintf(w, `<span class="fluxer-mention-%s">@room</span>`, class)
	return
}

type fluxerEveryone struct{}

var ExtFluxerEveryone = &fluxerEveryone{}

func (e *fluxerEveryone) Extend(m goldmark.Markdown) {
	m.Parser().AddOptions(parser.WithInlineParsers(
		util.Prioritized(defaultFluxerEveryoneParser, 600),
	))
	m.Renderer().AddOptions(renderer.WithNodeRenderers(
		util.Prioritized(&fluxerEveryoneHTMLRenderer{}, 600),
	))
}
