// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package cubelan

import (
	"net"
	"time"
)

// The hold. A Cube keeps a session whose end it never saw, an outage of
// the network of serve's box for one, and refuses every other client
// meanwhile: on 2026-11-13 for 41½ minutes (docs/cube-protocol.md,
// "Lessons from the hardware days"). The client therefore does not end
// a connection it has lost (Lost, the warning and EventDisconnected as
// before) but holds it open, so that it resumes on the same session once
// the network is back, as TCP's retransmissions reach the Cube. While
// held, Send and Reconnect answer ErrNotConnected, and no poll is
// written.
//
// The hold ends at Options.Hold, when the Cube closes or resets the
// connection, when ReconnectIfMoved finds it elsewhere, or when
// discovery has heard the Cube at its address (Heard) for HoldHeard
// without the connection resuming: a Cube that dropped its side without
// a reset, or a source address of ours that is gone.

// DefaultHold is how long a lost connection is held: a little over the
// longest the Cube kept a dead session, 41½ minutes on 2026-11-13.
const DefaultHold = 45 * time.Minute

// DefaultHoldHeard is how long discovery may hear a Cube whose held
// connection does not resume. TCP retransmits at most two minutes
// apart, so a session the Cube still holds resumes within that.
const DefaultHoldHeard = 3 * time.Minute

// holdFor is how long a lost connection is held, 0 for none: without
// polling nothing marks a loss but the kernel's keepalive, which ends
// the connection itself.
func (o Options) holdFor() time.Duration {
	switch {
	case o.PollInterval <= 0 || o.Hold < 0:
		return 0
	case o.Hold == 0:
		return DefaultHold
	}
	return o.Hold
}

// holdHeard is Options.HoldHeard or its default.
func (o Options) holdHeard() time.Duration {
	if o.HoldHeard > 0 {
		return o.HoldHeard
	}
	return DefaultHoldHeard
}

// runState is what Run and its sessions share of a loss: when the Cube
// was lost (zero while it is up) and last warned of. Only Run's
// goroutine touches it.
type runState struct {
	lost, warned time.Time
}

// ending says how a session ended.
type ending struct {
	connected bool // the dial succeeded
	// ours says that our side ended the connection (endedByUs, the
	// missed polls, the hold's end), so that the Cube may still hold it.
	ours    bool
	greeted bool // the connection got its greeting
	// reported says that the session reported the loss itself, when its
	// hold began: Run does not report it again.
	reported bool
}

// Heard tells the client that discovery has just heard its Cube answer
// from the client's address. A hold that does not resume ends once the
// Cube has been heard for HoldHeard (see above).
func (c *Client) Heard() {
	now := time.Now()
	c.heardMu.Lock()
	c.lastHeard = now
	if c.held.Load() && c.heardInHold.IsZero() {
		c.heardInHold = now
	}
	c.heardMu.Unlock()
	c.noteLink()
}

// heardTooLong reports whether discovery has heard the Cube for
// HoldHeard since the hold began, the connection not resuming.
func (c *Client) heardTooLong(now time.Time) bool {
	c.heardMu.Lock()
	defer c.heardMu.Unlock()
	return !c.heardInHold.IsZero() && now.Sub(c.heardInHold) >= c.opts.holdHeard()
}

// Held reports whether the client holds a lost connection open.
func (c *Client) Held() bool { return c.held.Load() }

// HeldUntil is when the hold of a lost connection ends at the latest,
// zero while none is held.
func (c *Client) HeldUntil() time.Time {
	c.heardMu.Lock()
	defer c.heardMu.Unlock()
	return c.holdUntil
}

// beginHold starts holding a lost connection: the loss is reported as
// Run reports one, the warning first, then Held and Lost, then the
// event, and the client looks disconnected to its callers.
func (c *Client) beginHold(st *runState, err error) {
	now := time.Now()
	until := now.Add(c.opts.holdFor())
	st.lost, st.warned = now, now
	c.evlog.Warn("cube connection lost", "addr", c.Address(), "error", err, "held_until", until.Format(time.RFC3339))
	c.heardMu.Lock()
	c.heardInHold, c.holdUntil, c.lostAt = time.Time{}, until, now
	c.heardMu.Unlock()
	c.held.Store(true)
	c.online.Store(false)
	c.lost.Store(true)
	c.emit(Event{Kind: EventDisconnected, Err: err})
}

// resumeHold ends a hold that resumed: the Cube answered on the held
// connection. The line comes before the event, as at a greeting.
func (c *Client) resumeHold(st *runState) {
	down := time.Since(st.lost)
	c.endHold()
	c.online.Store(true)
	c.lost.Store(false)
	st.lost = time.Time{}
	c.evlog.Info("cube connection re-established", "addr", c.Address(), "down_for", down.Round(time.Second), "held", true)
	c.noteLink()
	c.emit(Event{Kind: EventConnected, Cause: CauseLoss, DownFor: down})
}

// endHold clears the hold's state.
func (c *Client) endHold() {
	c.held.Store(false)
	c.heardMu.Lock()
	c.heardInHold, c.holdUntil = time.Time{}, time.Time{}
	c.heardMu.Unlock()
}

// errHoldEnded is a hold's end without a resumption.
type errHoldEnded struct {
	why string
}

func (e *errHoldEnded) Error() string { return "cubelan: the held connection did not resume: " + e.why }

// holdCheckEvery is how often a hold looks at its deadline and at what
// discovery heard: a second, less where the test's durations are short.
func (c *Client) holdCheckEvery() time.Duration {
	return min(time.Second, c.opts.holdHeard()/4, c.opts.holdFor()/4)
}

// holdDialer is the dialer New gives a client without one: 10 s to
// connect, and the keepalive and the socket options of the hold
// (hold_<os>.go) so that the kernel does not end a held connection
// first.
func holdDialer(hold time.Duration) *net.Dialer {
	return &net.Dialer{Timeout: 10 * time.Second, KeepAliveConfig: keepAliveFor(hold), Control: holdControl(hold)}
}

// keepAliveCovering is the keepalive of a held connection where nothing
// but the probe count keeps the kernel from ending it first: one probe
// every 15 s after 15 s of silence, as many as cover the hold, at most
// 255 (Windows' limit). Without a hold it is keepAlive.
func keepAliveCovering(hold time.Duration) net.KeepAliveConfig {
	if hold <= 0 {
		return keepAlive
	}
	const every = 15 * time.Second
	n := int((hold + every - 1) / every)
	return net.KeepAliveConfig{Enable: true, Idle: 15 * time.Second, Interval: every, Count: max(keepAlive.Count, min(n, 255))}
}

// holdSeconds is the hold in whole seconds, at least one.
func holdSeconds(hold time.Duration) int {
	return max(1, int((hold+time.Second-1)/time.Second))
}
