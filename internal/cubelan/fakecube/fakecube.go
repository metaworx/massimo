// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

// Package fakecube is a TCP server that behaves like a MAX! Cube's LAN
// interface: it sends a greeting on connect and answers commands. It is
// used for tests and for developing against recorded data without
// hardware.
package fakecube

import (
	"bufio"
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.metaworx.ch/massimo/internal/protocol/cube"
)

// Server is a fake Cube.
type Server struct {
	Greeting []cube.Message // sent on connect; the last one should be L
	NTP      []string
	Duty     int  // duty cycle reported in S replies (0..100)
	Discard  bool // make S replies report the command as discarded
	// DiscardIf, when set, makes the S reply to each s: or r: line it
	// reports report the command as discarded.
	DiscardIf func(cube.Message) bool
	// Pair, when set, is a device that joins during inclusion mode and
	// is one of the fake's devices from then on, until a t: deletes it.
	Pair *Pairing
	// Keep names devices that t: does not delete, as the Cube keeps a
	// device that never answers.
	Keep map[cube.RFAddress]bool
	// ReplyGap spaces the lines of a reply of several, the A line of
	// each device a t: deletes, as the Cube answers each when it is done
	// with it; 0 writes them at once.
	ReplyGap time.Duration
	// Apply makes the fake carry out the radio commands it acknowledges
	// (apply.go): the devices' L entries and C lines change as the real
	// Cube's do once the devices confirmed them.
	Apply bool
	// ApplyAfter delays applying a command, as a device confirms one
	// after its wakeup; 0 applies it before the S reply.
	ApplyAfter time.Duration
	// BoostFor ends a boost after this long instead of the device's boost
	// time, so that a test need not wait minutes.
	BoostFor time.Duration
	// SingleClient makes the fake serve one client at a time, as the
	// Cube does (oneclient.go): while a client is connected, or a dead
	// session is held, another client is refused; on Linux its dial times
	// out, as on hardware.
	SingleClient bool
	// HoldDead is how long a SingleClient fake keeps a session that ended
	// while it was silent, as the Cube keeps one whose end the outage
	// lost; 0 is DefaultHoldDead. Release and Restart end it sooner.
	HoldDead time.Duration

	ln       net.Listener
	mu       sync.Mutex
	received []cube.Message
	conns    int
	open     map[net.Conn]bool
	joined   bool                    // the Pair device has joined, see join
	meta     []cube.Message          // the M lines an m: wrote, see writeMetadata; nil: the greeting's
	blocks   []string                // the blocks of the m: write under way
	configs  map[string]cube.Message // C lines set with SetConfig, by address
	list     *cube.Message           // the L line set with SetDeviceList
	deleted  map[cube.RFAddress]bool // deleted with t:
	quiet    chan struct{}           // open while silent, see SetSilent
	live     bool                    // a command or the control changed a line: the greeting carries the changes
	boosts   map[cube.RFAddress]*boost
	removed  map[cube.RFAddress]bool // taken out with Remove, back with Restore
	deaf     map[cube.RFAddress]bool // take no command, see SetDeaf
	control  *http.Server            // the control's HTTP server, see Control
	closed   bool                    // Close was called

	// A pause (Pause): paused until Resume; hold holds the port while the
	// listener is closed, nil where the listener stays open.
	paused bool
	hold   portHold

	// The one-client rule (SingleClient, oneclient.go).
	deadUntil time.Time     // a dead session is held until then; zero: none
	deadTimer *time.Timer   // ends the dead session at deadUntil
	refusing  bool          // the backlog is cut and a connection parked
	park      net.Conn      // the fake's own connection in the accept queue
	parkAddr  string        // its local address, to know it once accepted
	freed     chan struct{} // closed and replaced whenever the slot changes
	restarts  int           // Restart calls

	free *int // free memory slots set with SetFreeSlots

	// recvMu orders the lines received and their hooks (OnReceive): it
	// is held from a line's record to the end of its hooks, and taken
	// before mu.
	recvMu    sync.Mutex
	onReceive []func(cube.Message) // guarded by recvMu
}

// SetFreeSlots makes the fake report free memory slots in its S replies
// and in the H line of its greeting, as a Cube that holds radiograms for
// devices that have not taken them yet; without it the S replies say
// 0x31 and the H line the greeting's own.
func (s *Server) SetFreeSlots(n int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.free = &n
}

// freeSlots is the free-slot field of an S reply, as two hex digits.
func (s *Server) freeSlots() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.free == nil {
		return "31"
	}
	return fmt.Sprintf("%02x", *s.free)
}

// SetSilent makes the fake a Cube whose network cable is out, or puts
// the cable back: while silent it keeps its connections, sends no
// greeting and holds every line it reads unanswered; once it speaks
// again it carries on with what it held, as TCP delivers it when the
// cable is back. Close ends a silence too.
func (s *Server) SetSilent(on bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	switch {
	case on && s.quiet == nil:
		s.quiet = make(chan struct{})
	case !on && s.quiet != nil:
		close(s.quiet)
		s.quiet = nil
	}
}

// isSilent reports whether the fake is silent (SetSilent).
func (s *Server) isSilent() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.quiet != nil
}

// audibleOr waits while the fake is silent, and reports false when
// gone closes first: the client left during the silence.
func (s *Server) audibleOr(gone <-chan struct{}) bool {
	s.mu.Lock()
	q := s.quiet
	s.mu.Unlock()
	if q == nil {
		return true
	}
	select {
	case <-q:
		return true
	case <-gone:
		return false
	}
}

// join makes the Pair device one of the fake's devices, as a Cube keeps
// a device that joined: c:, l: and a new connection's greeting carry its
// C line and its L entry from now on, added where the fake reads its
// lines while it is joined (withJoinedLocked, configRawLocked,
// liveGreetingLocked). A device that joins while it is one already
// keeps what changed in its lines; one a t: deleted starts again from
// the Pairing's. No other line changes: a greeting SetGreeting gives is
// sent as it is, the joined device's lines added.
func (s *Server) join(p *Pairing) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rf := p.Device.RFAddress
	if s.deleted[rf] && s.list != nil {
		// The L line the fake changed still holds the entry of before.
		if raw, err := base64.StdEncoding.DecodeString(s.list.Data); err == nil {
			var list []byte
			for _, e := range entries(raw) {
				if cube.RFAddressFromBytes(e[1:4]) != rf {
					list = append(list, e...)
				}
			}
			s.list = &cube.Message{Type: 'L', Data: base64.StdEncoding.EncodeToString(list)}
		}
	}
	s.joined = true
	delete(s.deleted, rf)
}

// joinedLocked returns the Pair device while it is one of the fake's.
func (s *Server) joinedLocked() *Pairing {
	if s.joined && s.Pair != nil && !s.deleted[s.Pair.Device.RFAddress] {
		return s.Pair
	}
	return nil
}

// paired returns the Pair device while it is one of the fake's.
func (s *Server) paired() *Pairing {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.joinedLocked()
}

// withJoinedEntryLocked is an L payload with the joined Pair device's
// entry after the others where it has none.
func (s *Server) withJoinedEntryLocked(raw []byte) []byte {
	p := s.joinedLocked()
	if p == nil || len(p.Entry) == 0 {
		return raw
	}
	for _, e := range entries(raw) {
		if cube.RFAddressFromBytes(e[1:4]) == p.Device.RFAddress {
			return raw
		}
	}
	return slices.Concat(raw, p.Entry)
}

// withJoinedLocked is an L line with the joined Pair device's entry
// (withJoinedEntryLocked); a line that does not decode stays as it is.
func (s *Server) withJoinedLocked(l cube.Message) cube.Message {
	raw, err := base64.StdEncoding.DecodeString(l.Data)
	if err != nil {
		return l
	}
	if with := s.withJoinedEntryLocked(raw); len(with) != len(raw) {
		return cube.Message{Type: 'L', Data: base64.StdEncoding.EncodeToString(with)}
	}
	return l
}

// writeMetadata takes an m: line, one block of the room and device list
// numbered from 00 in decimal, as the original software writes them:
// once the blocks since the last 00 are a list the fake can read, they
// are a new connection's M lines, as the Cube's next M line showed an
// m: on hardware (docs/cube-protocol.md). A block out of order drops
// the write. The blocks are the fake's, not a connection's: two clients
// writing m: at once would mix them, where a Cube takes one client.
func (s *Server) writeMetadata(data string) {
	idx, block, ok := strings.Cut(strings.TrimSpace(data), ",")
	i, err := strconv.Atoi(idx)
	s.mu.Lock()
	defer s.mu.Unlock()
	if !ok || err != nil || i != 0 && i != len(s.blocks) {
		s.blocks = nil
		return
	}
	if i == 0 {
		s.blocks = nil
	}
	s.blocks = append(s.blocks, block)
	parts := make([]cube.MetadataPart, len(s.blocks))
	for j, b := range s.blocks {
		parts[j] = cube.MetadataPart{Index: j, Count: len(s.blocks), Data: b}
	}
	if _, err := cube.ParseMetadata(parts); err != nil {
		return
	}
	s.meta = make([]cube.Message, len(parts))
	for j, b := range s.blocks {
		s.meta[j] = cube.Message{Type: 'M', Data: fmt.Sprintf("%02x,%02x,%s", j, len(parts), b)}
	}
}

// Pairing describes the device that joins after n:. Delay after the n:
// the fake sends its N line, and from then on the device is one of the
// fake's (join): c: answers for it with Config, and l: and a new
// connection's greeting carry Config and Entry, as a Cube lists a paired
// device that is in no room, until a t: deletes it. The inclusion ends
// with an empty N line when the n: timeout is over or on x:. A real Cube
// sends that line when the time is up (seen on hardware, 2026-11-14);
// the N line of a device that joins and the end by x: are how it is
// expected to behave, still to be confirmed.
type Pairing struct {
	Device cube.NewDevice
	Config cube.Message // the device's C line
	Entry  []byte       // the device's entry in the L line, with its length byte; nil for none
	Delay  time.Duration
}

// NewDeviceLine is the N line for d: type, address, serial, 0xff.
func NewDeviceLine(d cube.NewDevice) cube.Message {
	rf := d.RFAddress.Bytes()
	b := append([]byte{byte(d.Type)}, rf[:]...)
	b = append(append(b, fmt.Sprintf("%-10.10s", d.Serial)...), 0xff)
	return cube.Message{Type: 'N', Data: base64.StdEncoding.EncodeToString(b)}
}

// SetConfig makes c: for the line's device answer with line from now on,
// as the real Cube does once a device acknowledged a configuration write.
func (s *Server) SetConfig(line cube.Message) {
	rf, _, _ := strings.Cut(strings.ToLower(line.Data), ",")
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.configs == nil {
		s.configs = map[string]cube.Message{}
	}
	s.configs[rf] = line
}

// SetGreeting replaces the lines a new connection is greeted with, as
// the Cube's rooms, names and configurations change while no client is
// connected (another app, a button at a device): its M lines replace
// those an m: wrote too.
func (s *Server) SetGreeting(lines []cube.Message) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Greeting = append([]cube.Message(nil), lines...)
	s.meta, s.blocks = nil, nil
}

// greeting returns the lines a new connection is greeted with, with the
// changes the fake applied.
func (s *Server) greeting() []cube.Message {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.liveGreetingLocked()
}

// Drop closes every client connection and keeps listening, as a Cube
// that lost its network for a moment: the client connects again.
func (s *Server) Drop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for c := range s.open {
		c.Close()
	}
}

// SetDeviceList makes l: answer with line from now on, as the real Cube
// does once devices reported new states.
func (s *Server) SetDeviceList(line cube.Message) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.list = &line
}

// deviceList is the L line l: answers: the one set with SetDeviceList or
// changed by the fake, else last, the connection's greeting's; with the
// joined Pair device's entry.
func (s *Server) deviceList(last cube.Message) cube.Message {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.list != nil {
		last = *s.list
	}
	return s.withJoinedLocked(last)
}

// config returns the C line set for rf with SetConfig.
func (s *Server) config(rf string) (cube.Message, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, ok := s.configs[strings.ToLower(strings.TrimSpace(rf))]
	return c, ok
}

// deleteDevices takes a t: (delete devices): every device not in Keep
// is gone from the C lines and the L line the fake answers with and
// from a new connection's greeting, as the Cube drops a device once it
// answered; the Pair device may join again with the next n:.
func (s *Server) deleteDevices(addrs []cube.RFAddress) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, rf := range addrs {
		if s.Keep[rf] {
			continue
		}
		if s.deleted == nil {
			s.deleted = map[cube.RFAddress]bool{}
		}
		s.deleted[rf] = true
		delete(s.configs, rf.String())
		if s.Pair != nil && s.Pair.Device.RFAddress == rf {
			s.joined = false
		}
	}
}

// isDeleted reports whether a t: deleted rf, or Remove took it out.
func (s *Server) isDeleted(rf cube.RFAddress) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.deleted[rf] || s.removed[rf]
}

// visible returns a line the fake sends without the devices deleted
// with t:: an L line without their entries; false for a C line of one.
func (s *Server) visible(m cube.Message) (cube.Message, bool) {
	switch m.Type {
	case 'C':
		addr, _, _ := strings.Cut(m.Data, ",")
		if rf, err := cube.ParseRFAddress(addr); err == nil && s.isDeleted(rf) {
			return m, false
		}
	case 'L':
		raw, err := base64.StdEncoding.DecodeString(m.Data)
		if err != nil {
			return m, true
		}
		var kept []byte
		for i := 0; i < len(raw); {
			end := min(i+1+int(raw[i]), len(raw))
			if entry := raw[i:end]; len(entry) < 4 || !s.isDeleted(cube.RFAddressFromBytes(entry[1:4])) {
				kept = append(kept, entry...)
			}
			i = end
		}
		return cube.Message{Type: 'L', Data: base64.StdEncoding.EncodeToString(kept)}, true
	case 'M':
		return s.withoutRemoved(m), true
	}
	return m, true
}

// LoadGreeting reads protocol lines from a capture file.
func LoadGreeting(path string) ([]cube.Message, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return cube.SplitLines(string(raw))
}

// Start listens on addr ("127.0.0.1:0" for a random port).
func (s *Server) Start(addr string) (string, error) {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return "", err
	}
	return s.Serve(ln), nil
}

// Serve serves on a listener the caller opened, and closes it with the
// server. A test that brings the Cube up late holds its port this way:
// a client's dial waits for the greeting meanwhile, and nothing else
// takes the port, as it may between a listener closed and one opened.
// A fake paused before Serve starts paused.
func (s *Server) Serve(ln net.Listener) string {
	if s.NTP == nil {
		s.NTP = []string{"ntp.homematic.com", "ntp.homematic.com"}
	}
	s.mu.Lock()
	s.ln = ln
	if s.paused {
		s.holdLocked()
	}
	s.mu.Unlock()
	go s.accept(ln)
	return ln.Addr().String()
}

// Pause makes the fake a Cube that is gone, at an address that stays
// the fake's: every client is dropped, as by Close, and every new one is
// refused until Resume. On Linux a client's dial is refused, as at an
// address where nothing listens: the listener closes, and a socket bound
// without listening holds its port meanwhile (holdPort). Elsewhere the
// listener stays open, and the fake resets each connection it accepts
// before any line: the dial succeeds and the connection ends at once.
// It is for tests, in place of a fake closed and another started on its
// address, whose port another program can take between the two; a fake
// that is never paused is unchanged by it, but for a SingleClient fake
// whose listener's backlog cannot be cut (not a TCP listener): that one
// refuses a second client by a close. A second Pause does nothing.
// A SingleClient fake keeps the dead session it holds, and refuses again
// after Resume while it does.
func (s *Server) Pause() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.paused || s.closed {
		return
	}
	s.paused = true
	// The clients leave the slot at once, not once their connections'
	// goroutines have seen the close: a client that dials right after
	// Resume is greeted, where a SingleClient fake would otherwise
	// refuse it beside a client it already dropped.
	for c := range s.open {
		c.Close()
		delete(s.open, c)
	}
	// The one-client rule's refusal ends with the pause, which refuses
	// every client, and accept stops waiting for the slot (waitFree).
	s.endRefusalLocked()
	s.wakeLocked()
	if s.ln != nil {
		s.holdLocked()
	}
}

// A portHold holds the port of a paused fake whose listener is closed
// (holdPort).
type portHold interface {
	reopen() (net.Listener, error) // listens on the port again, beside the hold
	release()                      // lets the port go
}

// holdLocked closes the listener of a paused fake where a socket that
// does not listen can hold its port meanwhile (holdPort), and leaves it
// open where none can: accept then resets what it takes. s.mu is held.
func (s *Server) holdLocked() {
	hold, err := holdPort(s.ln)
	if err != nil {
		return
	}
	s.hold = hold
	s.ln.Close()
}

// Resume ends a pause: the fake takes clients again on the same
// address, and greets them as before the pause, with what changed in
// it meanwhile. A Resume without a pause does nothing.
func (s *Server) Resume() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.paused || s.closed {
		return nil
	}
	s.paused = false
	if s.hold == nil {
		// The listener stayed open (holdLocked): accept goes on with it,
		// and refuses a client by a close while the slot is taken.
		return nil
	}
	ln, err := s.hold.reopen()
	if err != nil {
		s.paused = true
		return fmt.Errorf("fakecube: resume on %s: %w", s.ln.Addr(), err)
	}
	s.hold.release()
	s.hold = nil
	s.ln = ln
	if s.SingleClient && s.occupiedLocked() {
		// A dead session held across the pause: the new listener
		// refuses as the old one did, before accept waits on it.
		s.refuseLocked()
	}
	go s.accept(ln)
	return nil
}

// reset ends a connection with a reset rather than a close, as near as
// a listener can come to refusing it.
func reset(conn net.Conn) {
	if tc, ok := conn.(*net.TCPConn); ok {
		tc.SetLinger(0)
	}
	conn.Close()
}

// Close stops the server and drops every client connection.
func (s *Server) Close() error {
	s.mu.Lock()
	s.closed = true // from here on, Pause and Resume do nothing
	ln, hold := s.ln, s.hold
	s.hold = nil
	s.mu.Unlock()
	err := ln.Close()
	if hold != nil {
		hold.release()
		err = nil // the pause closed the listener
	}
	s.mu.Lock()
	for c := range s.open {
		c.Close()
	}
	if s.park != nil {
		s.park.Close()
	}
	if s.deadTimer != nil {
		s.deadTimer.Stop()
	}
	if s.freed != nil {
		close(s.freed)
		s.freed = nil
	}
	if s.control != nil {
		s.control.Close()
	}
	for _, b := range s.boosts {
		if b.timer != nil {
			b.timer.Stop()
		}
	}
	s.mu.Unlock()
	s.SetSilent(false)
	return err
}

// Received returns every command received so far.
func (s *Server) Received() []cube.Message {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]cube.Message(nil), s.received...)
}

// OnReceive calls f with each command the fake receives: first with
// each one received so far, then with each as it comes, one at a time
// and in the order Received has them, after Received holds it and before
// the fake answers it. It is for tests, which watch the lines this way
// rather than poll Received (internal/testkit/cubewatch); the fake
// behaves the same with it as without. f holds up the line's answer
// while it runs, so it is to return soon, and must not call OnReceive.
func (s *Server) OnReceive(f func(cube.Message)) {
	s.recvMu.Lock()
	defer s.recvMu.Unlock()
	for _, m := range s.Received() {
		f(m)
	}
	s.onReceive = append(s.onReceive, f)
}

// record keeps a command received and hands it to the OnReceive hooks.
func (s *Server) record(m cube.Message) {
	s.recvMu.Lock()
	defer s.recvMu.Unlock()
	s.mu.Lock()
	s.received = append(s.received, m)
	s.mu.Unlock()
	for _, f := range s.onReceive {
		f(m)
	}
}

// Connections returns how many clients connected so far.
func (s *Server) Connections() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.conns
}

// accept serves the connections of ln, until it closes: at Close, or at
// a Pause that holds its port (holdLocked), after which Resume serves a
// listener of its own.
func (s *Server) accept(ln net.Listener) {
	for {
		// A fake that drops SYNs takes no connection while the slot is
		// taken, so that the kernel's queue fills (oneclient.go).
		if s.SingleClient && dropsSYN && !s.waitFree() {
			return
		}
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		s.mu.Lock()
		if s.isParkLocked(conn) {
			s.mu.Unlock()
			conn.Close()
			continue
		}
		if s.paused {
			// A pause that keeps its listener (holdLocked): the
			// connection ends before any line.
			s.mu.Unlock()
			reset(conn)
			continue
		}
		if s.SingleClient && s.occupiedLocked() {
			// Outside Linux, or a client that got into the queue before
			// the backlog was cut: refused.
			s.mu.Unlock()
			conn.Close()
			continue
		}
		s.conns++
		if s.open == nil {
			s.open = map[net.Conn]bool{}
		}
		s.open[conn] = true
		if s.SingleClient {
			s.refuseLocked()
		}
		s.mu.Unlock()
		go s.serve(conn)
	}
}

func (s *Server) serve(conn net.Conn) {
	// The lines are read on their own, so that the end of the connection
	// is seen while the fake is silent too: a client that leaves during
	// a silence is a FIN the outage lost, and a SingleClient fake holds
	// its session (holdDeadLocked).
	lines := make(chan string, 64)
	done := make(chan struct{})
	gone := make(chan struct{}) // the reader ended: the client left
	var diedSilent bool         // written by the reader before it closes gone
	go func() {
		defer close(lines)
		defer close(gone)
		r := bufio.NewReader(conn)
		for {
			line, err := r.ReadString('\n')
			if err != nil {
				// Not our own Close (Drop, Restart, a write that failed):
				// the client left.
				diedSilent = !errors.Is(err, net.ErrClosed) && s.isSilent()
				return
			}
			select {
			case lines <- line:
			case <-done:
				return
			}
		}
	}()
	defer func() {
		close(done)
		conn.Close()
		for range lines { // the reader's end, and diedSilent with it
		}
		s.mu.Lock()
		delete(s.open, conn)
		if s.SingleClient {
			if diedSilent && !s.closed {
				s.holdDeadLocked()
			}
			s.changedLocked()
		}
		s.mu.Unlock()
	}()
	var wmu sync.Mutex
	write := func(line string) error {
		wmu.Lock()
		defer wmu.Unlock()
		_, err := conn.Write([]byte(line))
		return err
	}
	if !s.audibleOr(gone) {
		return
	}
	greeting := s.greeting()
	for _, m := range greeting {
		m, ok := s.visible(m)
		if !ok {
			continue
		}
		if err := write(m.Wire()); err != nil {
			return
		}
	}
	var inclusion *time.Timer // ends the running inclusion
	var join *time.Timer      // makes the Pair device join it
	defer func() {
		if inclusion != nil {
			inclusion.Stop()
		}
		if join != nil {
			join.Stop()
		}
	}()
	var last cube.Message
	for _, m := range greeting {
		if m.Type == 'L' {
			last = m
		}
	}
	for line := range lines {
		m, err := cube.ParseLine(line)
		if err != nil {
			continue
		}
		s.record(m)
		if !s.audibleOr(gone) {
			return // the client left during the silence
		}
		var reply string
		switch m.Type {
		case 'q':
			return
		case 'l':
			l, _ := s.visible(s.deviceList(last))
			reply = l.Wire()
		case 'c':
			reply = "A:\r\n"
			if rf, err := cube.ParseRFAddress(m.Data); err == nil && s.isDeleted(rf) {
				break
			}
			if p := s.paired(); p != nil && strings.EqualFold(strings.TrimSpace(m.Data), p.Device.RFAddress.String()) {
				reply = p.Config.Wire()
			}
			for _, g := range greeting {
				if g.Type == 'C' && strings.HasPrefix(strings.ToLower(g.Data), strings.ToLower(strings.TrimSpace(m.Data))+",") {
					reply = g.Wire()
				}
			}
			if c, ok := s.config(m.Data); ok {
				reply = c.Wire()
			}
		case 's', 'r':
			d := 0
			if s.Discard || s.DiscardIf != nil && s.DiscardIf(m) {
				d = 1
			}
			if d == 0 && m.Type == 's' && s.Apply {
				s.apply(m)
			}
			reply = fmt.Sprintf("S:%02x,%d,%s\r\n", s.Duty, d, s.freeSlots())
		case 'f':
			if m.Data != "" {
				s.NTP = strings.Split(m.Data, ",")
			}
			reply = "F:" + strings.Join(s.NTP, ",") + "\r\n"
		case 't':
			// One A line per address, as the Cube answered on hardware.
			reply = "A:\r\n"
			if _, addrs, err := cube.ParseDeleteDevices(m.Data); err == nil && len(addrs) > 0 {
				s.deleteDevices(addrs)
				reply = strings.Repeat("A:\r\n", len(addrs))
				if s.ReplyGap > 0 && len(addrs) > 1 {
					for i := range addrs {
						if i > 0 {
							time.Sleep(s.ReplyGap)
						}
						if err := write("A:\r\n"); err != nil {
							return
						}
					}
					continue
				}
			}
		case 'e', 'd':
			reply = fmt.Sprintf("%c:%s\r\n", m.Type-32, m.Data)
		case 'm':
			s.writeMetadata(m.Data)
			reply = "A:\r\n"
		case 'n':
			// No immediate reply: N lines come when devices join. The
			// time is the low byte, as a real Cube reads it (n:012c
			// lasted 44 s on hardware); what the Cube does with 0 is
			// not known, the fake takes 60 s.
			secs, _ := strconv.ParseUint(m.Data, 16, 16)
			if secs &= 0xff; secs == 0 {
				secs = 60
			}
			if inclusion != nil {
				inclusion.Stop()
			}
			if join != nil {
				join.Stop() // a second n: makes the device join once
			}
			if p := s.Pair; p != nil {
				join = time.AfterFunc(p.Delay, func() {
					s.join(p)
					write(NewDeviceLine(p.Device).Wire())
				})
			}
			inclusion = time.AfterFunc(time.Duration(secs)*time.Second, func() { write("N:\r\n") })
			continue
		case 'x':
			if inclusion != nil {
				inclusion.Stop()
				inclusion = nil
			}
			if join != nil {
				join.Stop() // no device joins an inclusion that ended
				join = nil
			}
			reply = "N:\r\n"
		default:
			reply = "A:\r\n"
		}
		if err := write(reply); err != nil {
			return
		}
	}
}

// PairingFrom builds a Pairing for a new device that is a copy of the
// device template in greeting with another address and serial, in no
// room, joining delay after n:: its C line, and its L entry where the
// greeting's L line has one of template; an L line that does not decode
// gives none, as the fake reads none from it.
func PairingFrom(greeting []cube.Message, template, rf cube.RFAddress, serial string, delay time.Duration) (*Pairing, error) {
	var entry []byte
	for _, g := range greeting {
		if g.Type != 'L' {
			continue
		}
		entry = nil
		raw, err := base64.StdEncoding.DecodeString(g.Data)
		if err != nil {
			continue
		}
		for _, e := range entries(raw) {
			if cube.RFAddressFromBytes(e[1:4]) == template {
				b := rf.Bytes()
				entry = append(append([]byte{e[0]}, b[:]...), e[4:]...)
			}
		}
	}
	for _, g := range greeting {
		if g.Type != 'C' || !strings.HasPrefix(strings.ToLower(g.Data), template.String()+",") {
			continue
		}
		raw, err := base64.StdEncoding.DecodeString(g.Data[7:])
		if err != nil {
			return nil, err
		}
		b := rf.Bytes()
		copy(raw[1:4], b[:])
		raw[5] = 0 // no room
		copy(raw[8:18], fmt.Sprintf("%-10.10s", serial))
		c, err := cube.ParseConfig(rf.String() + "," + base64.StdEncoding.EncodeToString(raw))
		if err != nil {
			return nil, err
		}
		return &Pairing{
			Device: cube.NewDevice{Type: c.Type, RFAddress: rf, Serial: c.Serial},
			Config: cube.Message{Type: 'C', Data: rf.String() + "," + base64.StdEncoding.EncodeToString(raw)},
			Entry:  entry,
			Delay:  delay,
		}, nil
	}
	return nil, fmt.Errorf("fakecube: no C line for %s in the greeting", template)
}
