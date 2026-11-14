// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package cubelan

import (
	"errors"
	"net"
	"time"
)

// LinkState says how the client stands with its Cube, for the server's
// link field of a Cube (docs/api-v1.md, GET /api/v1/cubes).
type LinkState string

// The link states.
const (
	// LinkConnected: a connection is up and its greeting complete.
	LinkConnected LinkState = "connected"
	// LinkHeld: the Cube is lost, its connection held open (hold.go).
	LinkHeld LinkState = "held"
	// LinkStale: the Cube answers discovery but refuses a new connection,
	// and serve's side ended the last one: the Cube still holds it.
	LinkStale LinkState = "stale"
	// LinkBusy: the Cube answers discovery but refuses a new connection,
	// serve having left none behind: another client holds it.
	LinkBusy LinkState = "busy"
	// LinkRebooting: a reboot was sent (Rebooting) and the Cube has not
	// greeted since.
	LinkRebooting LinkState = "rebooting"
	// LinkOffline: anything else not connected.
	LinkOffline LinkState = "offline"
)

// Link is the client's link state with its times: since when, and for
// a hold and a reboot until when at the latest.
type Link struct {
	State LinkState `json:"state"`
	Since time.Time `json:"since,omitzero"`
	Until time.Time `json:"until,omitzero"`
}

// refusedAfter is how many dials in a row must time out before a Cube
// that answers discovery is taken to refuse serve: one may be the
// Cube's restart.
const refusedAfter = 2

// heardFresh is how recent discovery's answer must be: the search for
// a lost Cube runs every minute (internal/server, searchLost).
const heardFresh = 150 * time.Second

// Link reports the client's link state (LinkState).
func (c *Client) Link() Link {
	c.heardMu.Lock()
	defer c.heardMu.Unlock()
	return c.linkLocked(time.Now())
}

// linkLocked is Link with heardMu held.
func (c *Client) linkLocked(now time.Time) Link {
	switch {
	case now.Before(c.rebootUntil):
		// First: the connection a reboot ends may not be closed yet, and
		// the greeting after it ends the wait before it is online.
		return Link{State: LinkRebooting, Since: c.rebootAt, Until: c.rebootUntil}
	case c.online.Load():
		return Link{State: LinkConnected}
	case c.held.Load():
		return Link{State: LinkHeld, Since: c.lostAt, Until: c.holdUntil}
	case c.timeouts >= refusedAfter && !c.lastHeard.IsZero() && now.Sub(c.lastHeard) <= heardFresh:
		if c.lastEndOurs {
			return Link{State: LinkStale, Since: c.timeoutSince}
		}
		return Link{State: LinkBusy, Since: c.timeoutSince}
	}
	return Link{State: LinkOffline}
}

// noteLink logs a change of the link state into or out of a refusal:
// a warning naming what it means when the Cube begins to refuse serve,
// a line when it stops.
func (c *Client) noteLink() {
	c.heardMu.Lock()
	l := c.linkLocked(time.Now())
	was := c.linkLogged
	c.linkLogged = l.State
	c.heardMu.Unlock()
	if l.State == was {
		return
	}
	switch l.State {
	case LinkStale:
		c.log.Warn("cube holds serve's old connection", "addr", c.Address(), "refused_since", l.Since.Format(time.RFC3339),
			"hint", "the Cube answers discovery but refuses a new connection; it lets go after up to about 40 minutes, and a reboot frees it at once but drops the radiograms it has not delivered")
	case LinkBusy:
		c.log.Warn("cube busy with another client", "addr", c.Address(), "refused_since", l.Since.Format(time.RFC3339),
			"hint", "the Cube answers discovery but refuses a new connection: another client holds it, such as the MAX! software, another server or an integration")
	default:
		if was == LinkStale || was == LinkBusy {
			c.log.Info("cube no longer refuses", "addr", c.Address(), "link", string(l.State))
		}
	}
}

// noteEnd records how a session ended, for the link state: a dial that
// timed out counts toward a refusal, any connection made ends the count,
// and its end says whether serve's side closed it.
func (c *Client) noteEnd(end ending, err error) {
	now := time.Now()
	c.heardMu.Lock()
	switch {
	case end.connected:
		c.timeouts, c.timeoutSince = 0, time.Time{}
		c.lastEndOurs = end.ours
	case isTimeout(err):
		if c.timeouts == 0 {
			c.timeoutSince = now
		}
		c.timeouts++
	default:
		c.timeouts, c.timeoutSince = 0, time.Time{}
	}
	c.heardMu.Unlock()
	c.noteLink()
}

// isTimeout reports whether err is a timeout: a dial whose SYN the Cube
// left unanswered, or a connection the kernel or a deadline ended.
func isTimeout(err error) bool {
	var ne net.Error
	return errors.As(err, &ne) && ne.Timeout()
}

// endedByUs reports whether a connection's read error says that our
// side ended it, so that the Cube may still hold it: a deadline, the
// kernel's timeout (the keepalive's, TCP_USER_TIMEOUT), or our own
// Close. The Cube's close (EOF) or reset is not.
func endedByUs(err error) bool {
	return isTimeout(err) || errors.Is(err, net.ErrClosed)
}
