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
)

// Options configure a Client.
type Options struct {
	Addr         string        // host[:port]; the default port is 62910
	PollInterval time.Duration // how often to request the device list; 0 disables polling
	ReplyTimeout time.Duration // how long to wait for a command's reply
	Logger       *slog.Logger
	Dialer       func(ctx context.Context, addr string) (net.Conn, error) // for tests; nil uses net.Dialer
}

// Client is a persistent, reconnecting connection to one Cube.
type Client struct {
	opts  Options
	snap  *Snapshot
	log   *slog.Logger
	evs   chan Event
	ready chan struct{} // closed when the first greeting is complete
	once  sync.Once

	mu      sync.Mutex
	conn    net.Conn
	pending *pendingReply
	writeMu sync.Mutex
	budget  budgetTracker
	// greeted is closed, and replaced, whenever a greeting is complete.
	greeted chan struct{}
	// reconnecting marks a connection closed on purpose by Reconnect.
	reconnecting atomic.Bool
	// online is set while a connection is up and its greeting complete.
	online atomic.Bool
}

type pendingReply struct {
	want byte
	ch   chan cube.Message
}

// Errors returned by Send.
var (
	ErrNotConnected = errors.New("cubelan: not connected")
	ErrBusy         = errors.New("cubelan: another command is waiting for its reply")
)

// New creates a client; Run starts it.
func New(opts Options) *Client {
	if opts.ReplyTimeout == 0 {
		opts.ReplyTimeout = 10 * time.Second
	}
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}
	if opts.Dialer == nil {
		d := &net.Dialer{Timeout: 10 * time.Second}
		opts.Dialer = func(ctx context.Context, addr string) (net.Conn, error) { return d.DialContext(ctx, "tcp", addr) }
	}
	if _, _, err := net.SplitHostPort(opts.Addr); err != nil {
		opts.Addr = net.JoinHostPort(opts.Addr, "62910")
	}
	return &Client{opts: opts, snap: newSnapshot(), log: opts.Logger, evs: make(chan Event, 64), ready: make(chan struct{}), greeted: make(chan struct{})}
}

// Snapshot returns the client's view of the Cube.
func (c *Client) Snapshot() *Snapshot { return c.snap }

// Events delivers connection and state events. The channel is buffered;
// if nobody reads it, old events are dropped rather than blocking the reader.
func (c *Client) Events() <-chan Event { return c.evs }

// Connected reports whether the client is connected now and has
// received the connection's complete greeting.
func (c *Client) Connected() bool { return c.online.Load() }

// Ready is closed after the first complete greeting.
func (c *Client) Ready() <-chan struct{} { return c.ready }

// Run connects and keeps the connection alive until ctx is cancelled.
// It reconnects with exponential backoff after failures; the backoff
// starts over after a connection that got its greeting.
func (c *Client) Run(ctx context.Context) error {
	backoff := time.Second
	for {
		greeted, err := c.session(ctx)
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if greeted {
			backoff = time.Second
		}
		if c.reconnecting.Swap(false) {
			c.log.Info("cube reconnecting to refresh the snapshot", "addr", c.opts.Addr)
			continue
		}
		c.emit(Event{Kind: EventDisconnected, Err: err})
		c.log.Warn("cube connection lost", "addr", c.opts.Addr, "error", err, "retry_in", backoff)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(backoff):
		}
		if backoff < time.Minute {
			backoff *= 2
		}
	}
}

// session runs one connection until it fails and reports whether the
// greeting arrived.
func (c *Client) session(ctx context.Context) (bool, error) {
	conn, err := c.opts.Dialer(ctx, c.opts.Addr)
	if err != nil {
		return false, err
	}
	defer conn.Close()
	c.mu.Lock()
	c.conn = conn
	c.mu.Unlock()
	defer func() {
		c.online.Store(false)
		c.mu.Lock()
		c.conn = nil
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
	readErr := make(chan error, 1)
	go func() {
		for {
			conn.SetReadDeadline(time.Now().Add(c.readDeadline()))
			m, err := r.Read()
			if err != nil {
				readErr <- err
				return
			}
			c.handle(m)
			if m.Type == 'L' && !greeted.Load() {
				greeted.Store(true)
				c.online.Store(true)
				c.once.Do(func() { close(c.ready) })
				c.mu.Lock()
				close(c.greeted)
				c.greeted = make(chan struct{})
				c.mu.Unlock()
				c.emit(Event{Kind: EventConnected})
				c.log.Info("cube connected", "addr", c.opts.Addr)
			}
		}
	}()

	var poll <-chan time.Time
	if c.opts.PollInterval > 0 {
		t := time.NewTicker(c.opts.PollInterval)
		defer t.Stop()
		poll = t.C
	}
	for {
		select {
		case <-ctx.Done():
			c.write(cube.CmdQuit)
			return greeted.Load(), ctx.Err()
		case err := <-readErr:
			return greeted.Load(), err
		case <-poll:
			if _, err := c.Send(ctx, cube.CmdList); err != nil && !errors.Is(err, ErrBusy) {
				c.log.Warn("poll failed", "error", err)
			}
		}
	}
}

// readDeadline is generous: the Cube is silent between polls, and with
// polling disabled it may be silent for a long time.
func (c *Client) readDeadline() time.Duration {
	if c.opts.PollInterval > 0 {
		return 3*c.opts.PollInterval + 30*time.Second
	}
	return 24 * time.Hour
}

// handle routes an incoming message: replies go to the waiting Send,
// everything else updates the snapshot.
func (c *Client) handle(m cube.Message) {
	c.mu.Lock()
	p := c.pending
	if p != nil && m.Type == p.want {
		c.pending = nil
		c.mu.Unlock()
		p.ch <- m
		// L replies also carry state; fall through to apply them.
		if m.Type != 'L' {
			return
		}
	} else {
		c.mu.Unlock()
	}
	evs, err := c.snap.apply(m, time.Now())
	if err != nil {
		c.log.Warn("bad message from cube", "type", string(m.Type), "error", err)
	}
	if m.Type == 'H' {
		c.noteBudget(c.snap.Hello())
	}
	for _, e := range evs {
		c.emit(e)
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
	case 'n', 'x':
		return 'N'
	case 'u', 'i', 'v', 't', 'z', 'a', 'w', 'm':
		return 'A'
	}
	return 0
}

// Send writes a command and waits for its reply. Only one command can be
// in flight at a time, which matches how the Cube behaves.
func (c *Client) Send(ctx context.Context, m cube.Message) (cube.Message, error) {
	want := replyType(m.Type)
	if want == 0 {
		return cube.Message{}, c.write(m)
	}
	p := &pendingReply{want: want, ch: make(chan cube.Message, 1)}
	c.mu.Lock()
	if c.conn == nil {
		c.mu.Unlock()
		return cube.Message{}, ErrNotConnected
	}
	if c.pending != nil {
		c.mu.Unlock()
		return cube.Message{}, ErrBusy
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
		return cube.Message{}, fmt.Errorf("cubelan: no %c reply to %c command within %s", want, m.Type, c.opts.ReplyTimeout)
	case <-ctx.Done():
		c.clearPending(p)
		return cube.Message{}, ctx.Err()
	}
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
// command queue (engine.Command.Reconnect).
func (c *Client) Reconnect(ctx context.Context) error {
	c.mu.Lock()
	conn, greeted := c.conn, c.greeted
	c.mu.Unlock()
	if conn == nil {
		return ErrNotConnected
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

// ApplyLocal folds a message into the snapshot as if the Cube had sent
// it. The LAN proxy uses it after a metadata write (m:) went through,
// because the Cube does not echo the new M lines.
func (c *Client) ApplyLocal(m cube.Message) { c.handle(m) }

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
