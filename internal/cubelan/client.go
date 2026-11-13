// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

// Package cubelan is the driver for a MAX! Cube reached over the LAN
// (TCP port 62910). It keeps one persistent connection, because the Cube
// accepts a single client at a time and has been observed to lose its
// configuration when clients connect and disconnect too often.
package cubelan

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"go.metaworx.ch/massimo/internal/protocol/cube"
	"go.metaworx.ch/massimo/internal/slogx"
)

// Options configure a Client.
type Options struct {
	Addr         string        // host[:port]; the default port is 62910
	PollInterval time.Duration // how often to request the device list; 0 disables polling
	ReplyTimeout time.Duration // how long to wait for a command's reply
	Logger       *slog.Logger
	Dialer       func(ctx context.Context, addr string) (net.Conn, error) // for tests; nil uses net.Dialer
	// Backoff is the wait after the first failed attempt; it doubles
	// with each failure to about a minute. 0 is one second; the tests
	// set less.
	Backoff time.Duration
	// RemindEvery is how often a Cube that stays unreachable is warned
	// of again. 0 is a day; the tests set less.
	RemindEvery time.Duration
	// EventLines says that the caller writes a line of its own for each
	// event (serve's server, which logs every event it keeps): the
	// client's lines of a connection, a loss, a device paired and a
	// device forgotten go to debug then, so that the log says each once.
	EventLines bool
}

// Client is a persistent, reconnecting connection to one Cube.
type Client struct {
	opts Options
	snap *Snapshot
	log  *slog.Logger
	// evlog writes the client's lines of its events: log, or log at
	// debug where the caller writes each event's own line
	// (Options.EventLines).
	evlog *slog.Logger
	evs   chan Event
	ready chan struct{} // closed when the first greeting is complete
	once  sync.Once
	// greetedOnce is set by the first complete greeting: a later one is
	// after a loss or a reconnect asked for (EventConnected's Cause).
	greetedOnce atomic.Bool

	mu       sync.Mutex
	conn     net.Conn
	connAddr string // the address conn was dialled at
	pending  *pendingReply
	// turn, of capacity 1, is held by the Send whose command waits for
	// its reply, from before the write until Send returns: one command
	// at a time, as the Cube answers them.
	turn    chan struct{}
	writeMu sync.Mutex
	budget  budgetTracker
	// greeted is closed, and replaced, whenever a greeting is complete.
	greeted chan struct{}
	// reconnecting marks a connection closed on purpose by Reconnect.
	reconnecting atomic.Bool
	// online is set while a connection is up and its greeting complete.
	online atomic.Bool
	// lost is set from the loss Run reports to the next complete
	// greeting, see Lost.
	lost atomic.Bool
	// inclusion is the pairing state, see inclusion.go.
	inclusion inclusion

	addrMu sync.Mutex
	addr   string        // host:port; "" until known, see SetAddress
	hasAdr chan struct{} // closed once addr is set
	// wake cuts Run's wait between two dials short: a new address, or
	// a Reconnect while disconnected.
	wake chan struct{}
	// beforeApply, set by a test before Run, is called with each line
	// right before it goes into the snapshot; nil otherwise.
	beforeApply func(cube.Message)
}

type pendingReply struct {
	want byte
	left int // replies still due: one, or one A per device of a t:
	ch   chan cube.Message
}

// Errors returned by Send.
var (
	ErrNotConnected = errors.New("cubelan: not connected")
	// ErrBusy is the poll's: it finds another command waiting for its
	// reply and sends nothing. Send itself waits its turn.
	ErrBusy = errors.New("cubelan: another command is waiting for its reply")
)

// replyTimeout is Send's error for a command the Cube did not answer
// within the reply timeout.
type replyTimeout struct {
	want, cmd byte
	after     time.Duration
}

func (e *replyTimeout) Error() string {
	return fmt.Sprintf("cubelan: no %c reply to %c command within %s", e.want, e.cmd, e.after)
}

// keepAlive finds a dead Cube between polls: after 15 s without traffic
// a probe every 5 s, and the third unanswered one ends the connection,
// about 30 s after the Cube went. The Cube acknowledges the probes
// (checked on hardware on 2026-11-09). Go's default, 15 s apart and 9
// probes, needs 150 s of silence, which a poll every minute never
// leaves. A probe is not sent while a poll's bytes are unacknowledged;
// that case is pollMisses'.
var keepAlive = net.KeepAliveConfig{Enable: true, Idle: 15 * time.Second, Interval: 5 * time.Second, Count: 3}

// pollMisses is how many polls in a row, with no line from the Cube
// between them, the Cube may leave unanswered before its connection is
// taken for lost: one may be late, two are a Cube that is gone, as one
// whose network cable is out, which TCP would notice only after many
// minutes. The read deadline is the backstop.
const pollMisses = 2

// New creates a client; Run starts it.
func New(opts Options) *Client {
	if opts.ReplyTimeout == 0 {
		opts.ReplyTimeout = 10 * time.Second
	}
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}
	if opts.Backoff == 0 {
		opts.Backoff = time.Second
	}
	if opts.RemindEvery == 0 {
		opts.RemindEvery = 24 * time.Hour
	}
	if opts.Dialer == nil {
		d := &net.Dialer{Timeout: 10 * time.Second, KeepAliveConfig: keepAlive}
		opts.Dialer = func(ctx context.Context, addr string) (net.Conn, error) { return d.DialContext(ctx, "tcp", addr) }
	}
	evlog := opts.Logger
	if opts.EventLines {
		evlog = slogx.AtDebug(opts.Logger)
	}
	c := &Client{opts: opts, snap: newSnapshot(), log: opts.Logger, evlog: evlog, evs: make(chan Event, 64), ready: make(chan struct{}), greeted: make(chan struct{}),
		hasAdr: make(chan struct{}), wake: make(chan struct{}, 1), turn: make(chan struct{}, 1)}
	if opts.Addr != "" {
		c.SetAddress(opts.Addr)
	}
	return c
}

// withPort adds the Cube's port to an address that has none.
func withPort(addr string) string {
	if _, _, err := net.SplitHostPort(addr); err != nil {
		return net.JoinHostPort(addr, "62910")
	}
	return addr
}

// Address is the Cube's address, host:port, or "" while it is unknown.
func (c *Client) Address() string {
	c.addrMu.Lock()
	defer c.addrMu.Unlock()
	return c.addr
}

// SetAddress sets the Cube's address, for a Cube found by discovery,
// and reports whether it changed. A client created without an address
// waits for it before it connects; a client between connections dials
// the new one at once; a connected one takes it with the next
// connection, so a caller that wants it now reconnects (through the
// queue, engine.Command.Reconnect).
func (c *Client) SetAddress(addr string) bool {
	addr = withPort(addr)
	c.addrMu.Lock()
	defer c.addrMu.Unlock()
	if addr == c.addr {
		return false
	}
	first := c.addr == ""
	c.addr = addr
	if first {
		close(c.hasAdr)
	}
	c.wakeDial()
	return true
}

// wakeDial ends Run's wait before its next dial.
func (c *Client) wakeDial() {
	select {
	case c.wake <- struct{}{}:
	default:
	}
}

// Snapshot returns the client's view of the Cube.
func (c *Client) Snapshot() *Snapshot { return c.snap }

// Events delivers connection and state events. The channel is buffered;
// if nobody reads it, old events are dropped rather than blocking the reader.
func (c *Client) Events() <-chan Event { return c.evs }

// Connected reports whether the client is connected now and has
// received the connection's complete greeting.
func (c *Client) Connected() bool { return c.online.Load() }

// Lost reports whether the Cube is lost: from the loss Run reports
// (EventDisconnected and the warning "cube connection lost") to the next
// complete greeting. Unlike Connected it stays false while the client
// connects again on purpose (Reconnect), and while it has not tried yet.
func (c *Client) Lost() bool { return c.lost.Load() }

// Ready is closed after the first complete greeting.
func (c *Client) Ready() <-chan struct{} { return c.ready }

// Run connects and keeps the connection alive until ctx is cancelled.
// It reconnects with exponential backoff after failures; the backoff
// starts over after a connection that got its greeting.
//
// A loss is reported once, with an EventDisconnected and a warning: the
// end of a connection that got its greeting, or the first attempt that
// fails since the start or since a deliberate reconnect. The attempts
// after it are logged at Debug, a reminder is warned every RemindEvery
// while the Cube stays unreachable, and the next greeting logs that the
// connection is re-established and how long the Cube was down. A
// connection closed by Reconnect is no loss.
func (c *Client) Run(ctx context.Context) error {
	select {
	case <-c.hasAdr:
	case <-ctx.Done():
		return ctx.Err()
	}
	backoff := c.opts.Backoff
	var lost, warned time.Time // when the Cube was lost and last warned of; zero while it is up
	for {
		greeted, err := c.session(ctx, lost)
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if greeted {
			backoff = c.opts.Backoff
			lost = time.Time{}
		}
		if c.reconnecting.Swap(false) {
			c.log.Info("cube reconnecting", "addr", c.Address())
			continue
		}
		switch now := time.Now(); {
		case lost.IsZero():
			// The warning first, then Lost, then the event: whoever sees
			// Lost sees the warning, and whoever reads the event sees Lost.
			lost, warned = now, now
			c.evlog.Warn("cube connection lost", "addr", c.Address(), "error", err, "retry_in", backoff)
			c.lost.Store(true)
			c.emit(Event{Kind: EventDisconnected, Err: err})
		case now.Sub(warned) >= c.opts.RemindEvery:
			warned = now
			c.log.Warn("cube still unreachable", "addr", c.Address(), "down_for", now.Sub(lost).Round(time.Second), "error", err)
		default:
			c.log.Debug("cube connection attempt failed", "addr", c.Address(), "error", err, "retry_in", backoff)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(backoff):
		case <-c.wake:
		}
		if backoff < time.Minute {
			backoff *= 2
		}
	}
}

// session runs one connection until it fails and reports whether the
// greeting arrived. lost is when the Cube was lost, zero if it was not.
func (c *Client) session(ctx context.Context, lost time.Time) (bool, error) {
	select { // a wake from before this dial is spent
	case <-c.wake:
	default:
	}
	addr := c.Address()
	conn, err := c.opts.Dialer(ctx, addr)
	if err != nil {
		return false, err
	}
	defer conn.Close()
	c.mu.Lock()
	c.conn, c.connAddr = conn, addr
	c.mu.Unlock()
	defer func() {
		c.online.Store(false)
		c.mu.Lock()
		c.conn, c.connAddr = nil, ""
		if c.pending != nil {
			close(c.pending.ch)
			c.pending = nil
		}
		c.mu.Unlock()
	}()

	// The greeting ends with the L message; it replaces what the
	// snapshot knew from earlier connections.
	c.snap.beginGreeting()
	r := cube.NewReader(conn)
	var greeted atomic.Bool
	var lines atomic.Int64 // read on this connection, for the missed polls
	readErr := make(chan error, 1)
	go func() {
		for {
			conn.SetReadDeadline(time.Now().Add(c.readDeadline()))
			m, err := r.Read()
			if err != nil {
				readErr <- err
				return
			}
			lines.Add(1)
			c.handle(m)
			if m.Type == 'L' && !greeted.Load() {
				greeted.Store(true)
				c.online.Store(true)
				c.lost.Store(false)
				// Swap the greeted channel before anyone can see the
				// client as ready, so that a Reconnect right after Ready
				// waits for the next greeting, not this one.
				c.mu.Lock()
				close(c.greeted)
				c.greeted = make(chan struct{})
				c.mu.Unlock()
				c.once.Do(func() { close(c.ready) })
				ev := Event{Kind: EventConnected, Cause: CauseReconnect}
				switch {
				case !c.greetedOnce.Swap(true):
					ev.Cause = CauseStart
				case !lost.IsZero():
					ev.Cause, ev.DownFor = CauseLoss, time.Since(lost)
				}
				c.log.Debug("greeting: list", "addr", c.Address(), "devices", len(c.snap.Devices()))
				// The line before the event, as for a loss: whoever reads
				// the event sees the connection's line.
				if lost.IsZero() {
					c.evlog.Info("cube connected", "addr", c.Address())
				} else {
					c.evlog.Info("cube connection re-established", "addr", c.Address(), "down_for", time.Since(lost).Round(time.Second))
				}
				c.emit(ev)
			}
		}
	}()

	var poll <-chan time.Time
	if c.opts.PollInterval > 0 {
		t := time.NewTicker(c.opts.PollInterval)
		defer t.Stop()
		poll = t.C
	}
	// missed counts the polls in a row that went unanswered with no line
	// read between them (pollMisses); linesAtMiss is lines at the last.
	missed, linesAtMiss := 0, int64(0)
	for {
		select {
		case <-ctx.Done():
			c.write(cube.CmdQuit)
			return greeted.Load(), ctx.Err()
		case err := <-readErr:
			return greeted.Load(), err
		case <-poll:
			_, err := c.trySend(ctx, cube.CmdList)
			var timeout *replyTimeout
			switch {
			case err == nil:
				missed = 0
			case errors.Is(err, ErrBusy):
				// Another command waits for its reply, which tells the
				// same: trySend sent nothing, and nothing is counted. The
				// poll does not wait its turn, so that the loop stays free
				// for the context and a read error.
			case errors.As(err, &timeout) && c.Inclusion().Active:
				// While pairing runs, a poll the Cube leaves unanswered
				// counts nothing: whether the Cube answers l: then is
				// not confirmed.
				missed = 0
				c.log.Warn("poll failed", "error", err)
			case errors.As(err, &timeout):
				if n := lines.Load(); missed > 0 && n == linesAtMiss {
					missed++
				} else {
					missed, linesAtMiss = 1, n
				}
				if missed >= pollMisses {
					return greeted.Load(), fmt.Errorf("cubelan: %d polls in a row unanswered: %w", missed, err)
				}
				c.log.Warn("poll failed", "error", err)
			default:
				c.log.Warn("poll failed", "error", err)
			}
		}
	}
}

// readDeadline is generous: the Cube is silent between polls, and with
// polling disabled it may be silent for a long time. The keepalive
// (keepAlive) and, with polling, two unanswered polls (pollMisses) end
// the connection to a dead Cube sooner; the deadline is the backstop.
func (c *Client) readDeadline() time.Duration {
	if c.opts.PollInterval > 0 {
		return 3*c.opts.PollInterval + 30*time.Second
	}
	return 24 * time.Hour
}

// handle routes an incoming message: replies go to the waiting Send,
// everything else updates the snapshot.
func (c *Client) handle(m cube.Message) {
	if m.Type == 'N' {
		// Inclusion mode reports devices whenever they join, long after
		// n: was written; n: and x: therefore wait for no reply.
		c.handleNewDevice(m)
		return
	}
	c.mu.Lock()
	p := c.pending
	if p != nil && m.Type == p.want && p.left > 1 {
		// An A of a t: for several devices; the last answers the Send.
		p.left--
		c.mu.Unlock()
		return
	}
	var reply *pendingReply
	if p != nil && m.Type == p.want {
		c.pending = nil
		reply = p
	}
	c.mu.Unlock()
	if reply != nil && m.Type != 'L' && m.Type != 'C' {
		reply.ch <- m
		return
	}
	// L and C replies also carry state, which goes into the snapshot
	// before the reply goes to Send, so that Send returns with it there.
	// A C line of the greeting may answer a re-read sent while the
	// greeting still comes, and the device it describes must not drop
	// out of the snapshot at the greeting's end.
	if c.beforeApply != nil {
		c.beforeApply(m)
	}
	evs, err := c.snap.apply(m, time.Now())
	if err != nil {
		c.log.Warn("bad message from cube", "type", string(m.Type), "error", err)
	}
	if m.Type == 'H' {
		c.noteBudget(c.snap.Hello())
	}
	for _, e := range evs {
		if e.Greeting {
			c.logGreeting(e)
		}
		c.emit(e) // never blocks: it drops the oldest event
	}
	if reply != nil {
		reply.ch <- m // buffered for the one reply: never blocks
	}
}

func (c *Client) emit(e Event) {
	select {
	case c.evs <- e:
	default:
		// drop the oldest to make room
		select {
		case <-c.evs:
		default:
		}
		select {
		case c.evs <- e:
		default:
		}
	}
}

func (c *Client) write(m cube.Message) error {
	c.mu.Lock()
	conn := c.conn
	c.mu.Unlock()
	if conn == nil {
		return ErrNotConnected
	}
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	_, err := conn.Write([]byte(m.Wire()))
	return err
}

// replyType says which message type answers a command. Commands without a
// reply return 0.
func replyType(cmd byte) byte {
	switch cmd {
	case 's', 'r':
		return 'S'
	case 'l':
		return 'L'
	case 'f':
		return 'F'
	case 'e':
		return 'E'
	case 'd':
		return 'D'
	case 'c':
		return 'C'
	case 'u', 'i', 'v', 't', 'z', 'a', 'w', 'm':
		return 'A'
	}
	return 0
}

// replies is how many lines answer a command: one, or for a t: one A
// for each device it deletes, as the Cube answered on hardware.
func replies(m cube.Message) int {
	if m.Type == 't' {
		if _, addrs, err := cube.ParseDeleteDevices(m.Data); err == nil && len(addrs) > 1 {
			return len(addrs)
		}
	}
	return 1
}

// Send writes a command and waits for its reply, the last of them for a
// t: that deletes several devices. Only one command can be in flight at
// a time, which matches how the Cube behaves: while another waits for
// its reply, Send waits for it first, as long as ctx allows. The reply
// timeout counts from the write. An L or C reply is in the snapshot,
// its events emitted, when Send returns it.
func (c *Client) Send(ctx context.Context, m cube.Message) (cube.Message, error) {
	return c.send(ctx, m, true)
}

// trySend is Send for the poll: while another command waits for its
// reply it returns ErrBusy at once, and the session loop goes on.
func (c *Client) trySend(ctx context.Context, m cube.Message) (cube.Message, error) {
	return c.send(ctx, m, false)
}

func (c *Client) send(ctx context.Context, m cube.Message, wait bool) (cube.Message, error) {
	want := replyType(m.Type)
	if want == 0 {
		return cube.Message{}, c.write(m)
	}
	if !c.isConnected() {
		return cube.Message{}, ErrNotConnected
	}
	select {
	case c.turn <- struct{}{}:
	default:
		if !wait {
			return cube.Message{}, ErrBusy
		}
		select {
		case c.turn <- struct{}{}:
		case <-ctx.Done():
			return cube.Message{}, ctx.Err()
		}
	}
	defer func() { <-c.turn }()
	// While the turn is held, pending is nil or this Send's: only the
	// holder sets it.
	p := &pendingReply{want: want, left: replies(m), ch: make(chan cube.Message, 1)}
	c.mu.Lock()
	if c.conn == nil { // dropped while this Send waited its turn
		c.mu.Unlock()
		return cube.Message{}, ErrNotConnected
	}
	c.pending = p
	c.mu.Unlock()

	if err := c.write(m); err != nil {
		c.clearPending(p)
		return cube.Message{}, err
	}
	select {
	case reply, ok := <-p.ch:
		if !ok {
			return cube.Message{}, ErrNotConnected
		}
		return reply, nil
	case <-time.After(c.opts.ReplyTimeout):
		c.clearPending(p)
		return cube.Message{}, &replyTimeout{want: want, cmd: m.Type, after: c.opts.ReplyTimeout}
	case <-ctx.Done():
		c.clearPending(p)
		return cube.Message{}, ctx.Err()
	}
}

// isConnected reports whether a connection is open, its greeting
// complete or not.
func (c *Client) isConnected() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.conn != nil
}

func (c *Client) clearPending(p *pendingReply) {
	c.mu.Lock()
	if c.pending == p {
		c.pending = nil
	}
	c.mu.Unlock()
}

// Reconnect closes the connection and waits until the client has
// connected again and received a complete greeting, which replaces the
// snapshot with what the Cube holds now: a full refresh, in case a write
// the Cube acknowledged did not take effect. A command waiting for its
// reply fails with ErrNotConnected, so callers run it through the
// command queue (engine.Command.Reconnect). Between connections it
// dials at once and waits for the greeting.
func (c *Client) Reconnect(ctx context.Context) error { return c.reconnect(ctx, false) }

// ReconnectIfMoved is Reconnect for a Cube found at a new address
// (SetAddress): it drops only a connection to another address than the
// client's now. A client between connections dials the new address at
// once, and one connected to it has done so since, so nothing is
// dropped then: that would make a second connection to the Cube.
func (c *Client) ReconnectIfMoved(ctx context.Context) error { return c.reconnect(ctx, true) }

func (c *Client) reconnect(ctx context.Context, onlyMoved bool) error {
	addr := c.Address()
	c.mu.Lock()
	conn, greeted := c.conn, c.greeted
	if onlyMoved && (conn == nil || c.connAddr == addr) {
		c.mu.Unlock()
		return nil
	}
	c.mu.Unlock()
	if conn == nil {
		c.wakeDial()
		select {
		case <-greeted:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	c.reconnecting.Store(true)
	c.write(cube.CmdQuit)
	conn.Close()
	select {
	case <-greeted:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// logGreeting writes a line of the greeting into the text log, at Debug:
// the steps of a connection are worth seeing when one looks for them.
func (c *Client) logGreeting(e Event) {
	snap := c.snap
	switch e.Kind {
	case EventHello:
		if h := snap.Hello(); h != nil {
			c.log.Debug("greeting: hello", "addr", c.Address(), "serial", h.Serial, "firmware", h.FirmwareVersion,
				"duty_cycle_pct", h.DutyCycle, "free_memory_slots", h.FreeMemorySlots)
		}
	case EventMetadata:
		if md := snap.Metadata(); md != nil {
			c.log.Debug("greeting: metadata", "addr", c.Address(), "rooms", len(md.Rooms), "devices", len(md.Devices))
		}
	case EventConfig:
		if cfg := snap.Config(e.Device); cfg != nil {
			c.log.Debug("greeting: config", "addr", c.Address(), "device", e.Device, "serial", cfg.Serial, "type", cfg.Type)
		}
	}
}

// ApplyLocal folds a message into the snapshot as if the Cube had sent
// it. The LAN proxy uses it after a metadata write (m:) went through,
// because the Cube does not echo the new M lines.
func (c *Client) ApplyLocal(m cube.Message) { c.handle(m) }

// ForgetDevice drops a device the Cube deleted with t: from the
// snapshot (Snapshot.Forget) and tells the observers with an
// EventForgotten, when the snapshot knew it; the event carries what the
// snapshot knew of the device.
func (c *Client) ForgetDevice(rf cube.RFAddress) {
	if gone, ok := c.snap.Forget(rf); ok {
		c.emit(Event{Kind: EventForgotten, Device: rf, Gone: &gone})
		c.evlog.Info("device forgotten", "addr", c.Address(), "device", rf, "serial", gone.Serial)
	}
}

// ApplyMetadataWrite mirrors a metadata write (the m: commands as sent,
// one per part) into the snapshot, since the Cube acknowledges such a
// write without echoing the new M lines.
func (c *Client) ApplyMetadataWrite(cmds []cube.Message) {
	for _, m := range cmds {
		idx, data, ok := strings.Cut(m.Data, ",")
		if !ok {
			return
		}
		c.ApplyLocal(cube.Message{Type: 'M', Data: idx + "," + fmt.Sprintf("%02x", len(cmds)) + "," + data})
	}
}

// SendRadio sends an s command and decodes the Cube's S reply.
func (c *Client) SendRadio(ctx context.Context, m cube.Message) (*cube.SendResult, error) {
	if m.Type != 's' && m.Type != 'r' {
		return nil, fmt.Errorf("cubelan: SendRadio needs an s or r command, got %c", m.Type)
	}
	reply, err := c.Send(ctx, m)
	if err != nil {
		return nil, err
	}
	res, err := cube.ParseSendResult(reply.Data)
	if err != nil {
		return nil, err
	}
	c.noteSendResult(res)
	return res, nil
}
