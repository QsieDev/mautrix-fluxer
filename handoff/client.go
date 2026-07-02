// Package handoff implements Fluxer's cross-device handoff login flow,
// which replaces Discord's remote-auth QR websocket protocol.
//
// The flow is: initiate a handoff to get a short-lived code, show that code
// to the user (e.g. as a QR code), then poll until the user approves it from
// another logged-in Fluxer client, at which point a session token is
// returned.
package handoff

import (
	"context"
	"fmt"
	"time"

	"github.com/qsiedev/fluxergo"
)

// PollInterval is how often the client polls Fluxer to check whether a
// handoff has been approved.
var PollInterval = 2 * time.Second

// Timeout is how long the client waits for a handoff to be approved before
// giving up.
var Timeout = 5 * time.Minute

// User holds the profile of the account that approved the handoff.
type User struct {
	UserID        string
	Username      string
	Discriminator string
	AvatarHash    string

	Token string
}

// Client drives a single handoff login attempt.
type Client struct {
	sess *fluxergo.Session

	user User
	err  error
}

// New creates a new Fluxer handoff client.
func New() (*Client, error) {
	sess, err := fluxergo.New("")
	if err != nil {
		return nil, err
	}

	return &Client{sess: sess}, nil
}

// Dial starts the handoff login process. It requests a handoff code from
// Fluxer, sends it once down codeChan for display to the user, then polls in
// the background until the handoff is approved, fails, times out, or ctx is
// cancelled. doneChan is closed when the attempt finishes; call Result
// afterwards to get the outcome.
func (c *Client) Dial(ctx context.Context, codeChan chan string, doneChan chan struct{}) error {
	code, err := c.sess.HandoffInitiate()
	if err != nil {
		return err
	}

	go c.poll(ctx, code, codeChan, doneChan)

	return nil
}

func (c *Client) poll(ctx context.Context, code string, codeChan chan string, doneChan chan struct{}) {
	defer close(doneChan)

	codeChan <- code
	close(codeChan)

	deadline := time.Now().Add(Timeout)
	ticker := time.NewTicker(PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			c.err = ctx.Err()
			return
		case <-ticker.C:
			token, err := c.sess.HandoffComplete(code)
			if err != nil {
				if time.Now().After(deadline) {
					c.err = fmt.Errorf("timed out waiting for handoff approval: %w", err)
					return
				}
				continue
			}

			profile, err := c.fetchProfile(token)
			if err != nil {
				c.err = err
				return
			}

			c.user = User{
				UserID:        profile.ID,
				Username:      profile.Username,
				Discriminator: profile.Discriminator,
				AvatarHash:    profile.Avatar,
				Token:         token,
			}
			return
		}
	}
}

func (c *Client) fetchProfile(token string) (*fluxergo.User, error) {
	sess, err := fluxergo.New(token)
	if err != nil {
		return nil, err
	}

	return sess.User("@me")
}

// Result returns the outcome of the handoff attempt. It should only be
// called after doneChan (passed to Dial) has been closed.
func (c *Client) Result() (User, error) {
	return c.user, c.err
}
