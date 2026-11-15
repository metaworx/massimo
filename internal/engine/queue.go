// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

// Package engine holds the parts that sit between the drivers and the API:
// the command queue, later the arbitration and scheduler.
package engine

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"sort"
	"sync"
	"time"

	"go.metaworx.ch/massimo/internal/cubelan"
	"go.metaworx.ch/massimo/internal/protocol/cube"
	"go.metaworx.ch/massimo/internal/slogx"
)

// Gateway is what the queue needs from a Cube driver. *cubelan.Client
// satisfies it.
type Gateway interface {
	Send(ctx context.Context, m cube.Message) (cube.Message, error)
	SendRadio(ctx context.Context, m cube.Message) (*cube.SendResult, error)
	Snapshot() *cubelan.Snapshot
	Budget() cubelan.Budget
}

// Priority orders queued commands; higher runs first.
type Priority int

const (
	PriorityLow    Priority = 0 // bulk work such as restores
	PriorityNormal Priority = 1
	PriorityHigh   Priority = 2 // direct user actions
)

// Command is a unit of work for the Cube.
type Command struct {
	// Key coalesces commands: a new command with the same non-empty Key
	// replaces one still waiting in the queue (e.g. "setpoint:room:3").
	Key      string
	Priority Priority
	// Target is the device the radio command addresses; used to decide
	// whether an r: (reset error) must precede it. Zero for none.
	Target cube.RFAddress
	// RoomID with Wakeup set sends z: for the room first, as the original
	// software does before configuration writes.
	RoomID uint8
	Wakeup bool
	// Lines are sent in order. s: and r: lines are radio commands and
	// consume budget; everything else is a plain Cube command.
	Lines []cube.Message
	// Passthrough sends the lines exactly as given, for clients of the
	// LAN proxy that speak the Cube protocol themselves: no budget wait,
	// no wakeup or error-reset prelude, no retry, and a discarded reply
	// is reported in Replies rather than as ErrDiscarded.
	Passthrough bool
	// Reconnect, with no Lines, drops the connection to the Cube and
	// waits for a fresh greeting, which re-reads everything the Cube
	// holds; run in queue order, it never cuts off another command. The
	// gateway must support it (see Reconnector).
	Reconnect bool
	// Moved, with Reconnect, is the reconnect for a Cube found at a new
	// address: it drops only a connection to the old one, since the
	// gateway may have dialled the new one already (see Mover).
	Moved bool
	// ID names the request the command belongs to (the server's command
	// row), for the Observer; empty for work of the server's own, which
	// is not observed.
	ID string
	// Seq is the queue's number of the command, set by Enqueue, which
	// tells the Observer's calls for one command from another's.
	Seq uint64
}

// Observer is told what becomes of every command with an ID: that it
// joined the queue, that the queue began sending it, and its result,
// also when it was superseded by a newer command with its Key or
// drained at the end. The server writes the commands table with it.
// Enqueued, and Finished for a command it supersedes, are called with
// the queue's lock held, before the command can run, so that they come
// before its Started: they must return at once and not call the queue.
type Observer interface {
	Enqueued(cmd Command)
	Started(cmd Command)
	Finished(cmd Command, res Result)
}

// Reconnector is a Gateway that can drop its connection and wait for a
// fresh greeting. *cubelan.Client satisfies it.
type Reconnector interface {
	Reconnect(ctx context.Context) error
}

// Mover is a Reconnector that can drop only a connection to an address
// it has since left (Command.Moved). *cubelan.Client satisfies it.
type Mover interface {
	ReconnectIfMoved(ctx context.Context) error
}

// Result is the outcome of a Command.
type Result struct {
	Discarded bool // the Cube rejected a radio command (budget exhausted)
	// Retried says the Cube discarded a radio command, and the queue
	// reset the communication error of the devices it reaches and sent
	// it again.
	Retried bool
	Budget  cubelan.Budget
	Err     error
	// Replies holds the Cube's answer to each of the command's Lines, in
	// order, up to the first failure. Radio replies are S lines.
	Replies []cube.Message
}

// Errors reported through Result.Err.
var (
	ErrSuperseded = errors.New("engine: command superseded by a newer one with the same key")
	ErrDiscarded  = errors.New("engine: command discarded by the cube")
)

// Ticket lets the caller wait for a command's result.
type Ticket struct {
	done chan Result
}

// Wait blocks until the command finished or ctx ends.
func (t *Ticket) Wait(ctx context.Context) Result {
	select {
	case r := <-t.done:
		return r
	case <-ctx.Done():
		return Result{Err: ctx.Err()}
	}
}

// Done returns a channel that receives the result once.
func (t *Ticket) Done() <-chan Result { return t.done }

// Options tune the queue's policy.
type Options struct {
	// MaxDutyPct is the duty cycle above which radio commands wait. The
	// Cube reports 0..100.
	MaxDutyPct int
	// DutyWait is how long radio commands wait while the duty cycle is
	// at MaxDutyPct before the queue reads the Cube's budget again: by a
	// reconnect, which costs no radio time, where the gateway can, else
	// from the next command's reply. The duty cycle falls within the hour,
	// probably the share of the last 60 minutes (docs/cube-protocol.md),
	// so the queue does not wait for the next full hour.
	// DefaultDutyWait where 0.
	DutyWait time.Duration
	// MinFreeSlots is the number of free memory slots below which radio
	// commands wait, so unreachable devices cannot fill the Cube's memory.
	MinFreeSlots int
	// SlotWait is how long to wait for slots to free up before sending
	// one command anyway (which refreshes the numbers).
	SlotWait time.Duration
	// Gap is the pause between consecutive commands.
	Gap time.Duration
	// WakeupSeconds is the z: duration before configuration writes.
	WakeupSeconds uint8
	Logger        *slog.Logger
	// Now exists for tests.
	Now func() time.Time
	// Observer, when set, is told of every command with an ID; the
	// queue's own lines of such a command's outcome go to debug then,
	// since the observer records it.
	Observer Observer
	// OnHold, when set, is told of each time radio commands wait for the
	// duty cycle or for free memory slots, and the queue's own warning of
	// it goes to debug.
	OnHold func(Hold)
}

// Hold is a wait of the radio commands: Reason HoldDutyCycle until
// Until, when the budget is read again, with the duty cycle at DutyPct,
// or HoldMemorySlots for Wait, with FreeSlots free.
type Hold struct {
	Reason    string
	DutyPct   int
	FreeSlots int
	Until     time.Time
	Wait      time.Duration
}

// The reasons of a Hold.
const (
	HoldDutyCycle   = "duty_cycle"
	HoldMemorySlots = "memory_slots"
)

// stepLog takes the queue's lines of a command's outcome: at debug
// where an observer records the command.
func (q *Queue) stepLog(cmd Command) *slog.Logger {
	if cmd.ID != "" && q.opts.Observer != nil {
		return q.quiet
	}
	return q.opts.Logger
}

// holdLog takes the queue's warning of a hold: at debug where OnHold
// records it.
func (q *Queue) holdLog() *slog.Logger {
	if q.opts.OnHold != nil {
		return q.quiet
	}
	return q.opts.Logger
}

func (o *Options) defaults() {
	if o.MaxDutyPct == 0 {
		o.MaxDutyPct = 95
	}
	if o.MinFreeSlots == 0 {
		o.MinFreeSlots = 5
	}
	if o.SlotWait == 0 {
		o.SlotWait = 5 * time.Minute
	}
	if o.Gap == 0 {
		o.Gap = 200 * time.Millisecond
	}
	if o.WakeupSeconds == 0 {
		o.WakeupSeconds = 30
	}
	if o.Logger == nil {
		o.Logger = slog.Default()
	}
	if o.Now == nil {
		o.Now = time.Now
	}
	if o.DutyWait == 0 {
		o.DutyWait = DefaultDutyWait
	}
}

// DefaultDutyWait is how long radio commands wait at a full duty cycle
// before the budget is read again, where Options.DutyWait is 0.
const DefaultDutyWait = 10 * time.Minute

type entry struct {
	cmd    Command
	ticket *Ticket
	seq    uint64
}

// Queue serialises commands to one Cube and enforces the radio budget.
type Queue struct {
	gw   Gateway
	opts Options
	// quiet is opts.Logger at debug, for the lines an observer or OnHold
	// records as well.
	quiet *slog.Logger

	mu      sync.Mutex
	items   []*entry
	seq     uint64
	wake    chan struct{}
	inFlght *entry
}

// NewQueue creates a queue for gw; Run starts it.
func NewQueue(gw Gateway, opts Options) *Queue {
	opts.defaults()
	return &Queue{gw: gw, opts: opts, quiet: slogx.AtDebug(opts.Logger), wake: make(chan struct{}, 1)}
}

// Enqueue adds a command and returns its ticket. A waiting command with
// the same Key is replaced and resolved with ErrSuperseded.
func (q *Queue) Enqueue(cmd Command) *Ticket {
	t := &Ticket{done: make(chan Result, 1)}
	q.mu.Lock()
	if cmd.Key != "" {
		kept := q.items[:0]
		for _, e := range q.items {
			if e.cmd.Key == cmd.Key {
				e.ticket.done <- Result{Err: ErrSuperseded}
				q.observe(e.cmd, func(o Observer) { o.Finished(e.cmd, Result{Err: ErrSuperseded}) })
				continue
			}
			kept = append(kept, e)
		}
		q.items = kept
	}
	q.seq++
	cmd.Seq = q.seq
	// Told before Run can take the command, so that the observer has
	// recorded it when Started and Finished come.
	q.observe(cmd, func(o Observer) { o.Enqueued(cmd) })
	q.items = append(q.items, &entry{cmd: cmd, ticket: t, seq: q.seq})
	q.mu.Unlock()
	select {
	case q.wake <- struct{}{}:
	default:
	}
	return t
}

// Withdraw takes a command that is still waiting out of the queue and
// resolves its ticket with err, as Finished tells the observer; it
// reports whether the command was waiting. A command that runs or ran is
// left alone, and its ticket gets its result.
func (q *Queue) Withdraw(t *Ticket, err error) bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	for i, e := range q.items {
		if e.ticket == t {
			q.items = append(q.items[:i], q.items[i+1:]...)
			e.ticket.done <- Result{Err: err}
			q.observe(e.cmd, func(o Observer) { o.Finished(e.cmd, Result{Err: err}) })
			return true
		}
	}
	return false
}

// Len returns the number of waiting commands.
func (q *Queue) Len() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.items)
}

func (q *Queue) next() *entry {
	q.mu.Lock()
	defer q.mu.Unlock()
	if len(q.items) == 0 {
		return nil
	}
	sort.SliceStable(q.items, func(i, j int) bool {
		if q.items[i].cmd.Priority != q.items[j].cmd.Priority {
			return q.items[i].cmd.Priority > q.items[j].cmd.Priority
		}
		return q.items[i].seq < q.items[j].seq
	})
	e := q.items[0]
	q.items = q.items[1:]
	q.inFlght = e
	return e
}

// Run processes commands until ctx ends. Waiting commands are then
// resolved with ctx.Err().
func (q *Queue) Run(ctx context.Context) error {
	defer func() { q.drain(ctx.Err()) }()
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		e := q.next()
		if e == nil {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-q.wake:
				continue
			}
		}
		q.observe(e.cmd, func(o Observer) { o.Started(e.cmd) })
		res := q.execute(ctx, e.cmd)
		q.observe(e.cmd, func(o Observer) { o.Finished(e.cmd, res) })
		e.ticket.done <- res
		q.mu.Lock()
		q.inFlght = nil
		q.mu.Unlock()
		q.sleep(ctx, q.opts.Gap)
	}
}

func (q *Queue) drain(err error) {
	q.mu.Lock()
	items := q.items
	q.items = nil
	q.mu.Unlock()
	for _, e := range items {
		e.ticket.done <- Result{Err: err}
		q.observe(e.cmd, func(o Observer) { o.Finished(e.cmd, Result{Err: err}) })
	}
}

// observe tells the Observer about a command with an ID.
func (q *Queue) observe(cmd Command, call func(Observer)) {
	if cmd.ID != "" && q.opts.Observer != nil {
		call(q.opts.Observer)
	}
}

func (q *Queue) sleep(ctx context.Context, d time.Duration) {
	if d <= 0 {
		return
	}
	select {
	case <-ctx.Done():
	case <-time.After(d):
	}
}

func isRadio(m cube.Message) bool { return m.Type == 's' || m.Type == 'r' }

// execute runs one command with the budget policy and the preludes the
// original software uses.
func (q *Queue) execute(ctx context.Context, cmd Command) Result {
	var res Result
	if cmd.Reconnect {
		rc, ok := q.gw.(Reconnector)
		if !ok {
			return Result{Err: errors.New("engine: the gateway cannot reconnect"), Budget: q.gw.Budget()}
		}
		// Bounded, so that a Cube that does not come back cannot hold up
		// the queue; the commands after it then fail until it does.
		rctx, cancel := context.WithTimeout(ctx, time.Minute)
		defer cancel()
		if m, ok := q.gw.(Mover); ok && cmd.Moved {
			return Result{Err: m.ReconnectIfMoved(rctx), Budget: q.gw.Budget()}
		}
		return Result{Err: rc.Reconnect(rctx), Budget: q.gw.Budget()}
	}
	needsRadio := false
	for _, l := range cmd.Lines {
		if isRadio(l) {
			needsRadio = true
		}
	}
	if needsRadio && !cmd.Passthrough {
		if err := q.waitForBudget(ctx); err != nil {
			return Result{Err: err, Budget: q.gw.Budget()}
		}
	}
	if cmd.Wakeup && !cmd.Passthrough {
		wake := cube.CmdWakeup(q.opts.WakeupSeconds, cube.WakeupRoom, fmt.Sprintf("%02x", cmd.RoomID))
		reply, err := q.gw.Send(ctx, wake)
		if err != nil {
			return Result{Err: fmt.Errorf("wakeup: %w", err), Budget: q.gw.Budget()}
		}
		q.opts.Logger.Info("room woken up", "room", cmd.RoomID, "command", wake.String(), "reply", reply.String())
		q.sleep(ctx, q.opts.Gap)
	}
	if needsRadio && !cmd.Passthrough && cmd.Target != 0 && q.deviceInError(cmd.Target) {
		if _, err := q.gw.SendRadio(ctx, cube.CmdResetError(cmd.Target)); err != nil {
			return Result{Err: fmt.Errorf("reset the communication error: %w", err), Budget: q.gw.Budget()}
		}
		q.opts.Logger.Info("communication error reset before a command", "device", cmd.Target, "room", cmd.RoomID)
		q.sleep(ctx, q.opts.Gap)
	}
	var members map[uint8][]cube.RFAddress
	var addressees []cube.RFAddress
	if needsRadio && !cmd.Passthrough {
		var rooms []uint8
		rooms, members = q.flaggedMembers(cmd)
		for _, room := range rooms {
			if err := q.resetMembers(ctx, room, members[room]); err != nil {
				return Result{Err: fmt.Errorf("reset the communication error: %w", err), Budget: q.gw.Budget()}
			}
		}
		addressees = q.flaggedAddressees(cmd)
		for _, rf := range addressees {
			if resetForRoom(members, rf) {
				continue
			}
			if err := q.resetAddressee(ctx, rf); err != nil {
				return Result{Err: fmt.Errorf("reset the communication error: %w", err), Budget: q.gw.Budget()}
			}
		}
	}
	for i, l := range cmd.Lines {
		if i > 0 {
			q.sleep(ctx, q.opts.Gap)
		}
		if !isRadio(l) {
			reply, err := q.gw.Send(ctx, l)
			if err != nil {
				res.Err = err
				break
			}
			res.Replies = append(res.Replies, reply)
			continue
		}
		r, err := q.gw.SendRadio(ctx, l)
		if err != nil {
			res.Err = err
			break
		}
		if cmd.Passthrough {
			res.Replies = append(res.Replies, sendResultLine(r))
			res.Discarded = res.Discarded || r.Discarded
			q.stepLog(cmd).Info("proxy client's command sent", "command", l.String(), "discarded", r.Discarded,
				"duty_pct", r.DutyCycle, "free_slots", r.FreeMemorySlots)
			continue
		}
		room, dev := lineRoom(l), lineDevice(l)
		again, direct := members[room], dev != 0 && slices.Contains(addressees, dev)
		if r.Discarded && (cmd.Target != 0 || len(again) > 0 || direct) {
			// Observed on hardware: a command to a device with a
			// communication error is discarded unless r: was sent first.
			// Reset once and retry: the target, the flagged members of
			// the room a room-flagged line names, and the flagged device a
			// direct line addresses, those the prelude reset.
			q.stepLog(cmd).Info("command discarded, resetting the communication error and retrying", "target", cmd.Target, "room", room, "command", l.String(),
				"duty_pct", r.DutyCycle, "free_slots", r.FreeMemorySlots)
			res.Retried = true
			if cmd.Target != 0 {
				if _, err := q.gw.SendRadio(ctx, cube.CmdResetError(cmd.Target)); err != nil {
					res.Err = err
					break
				}
				q.sleep(ctx, q.opts.Gap)
			}
			if err := q.resetMembers(ctx, room, again); err != nil {
				res.Err = err
				break
			}
			if direct {
				if err := q.resetAddressee(ctx, dev); err != nil {
					res.Err = err
					break
				}
			}
			if r, err = q.gw.SendRadio(ctx, l); err != nil {
				res.Err = err
				break
			}
		}
		res.Replies = append(res.Replies, sendResultLine(r))
		if !r.Discarded {
			q.stepLog(cmd).Info("command accepted by the cube", "target", cmd.Target, "command", l.String(),
				"duty_pct", r.DutyCycle, "free_slots", r.FreeMemorySlots)
		}
		if r.Discarded {
			q.stepLog(cmd).Warn("command discarded by the cube", "target", cmd.Target, "command", l.String(),
				"duty_pct", r.DutyCycle, "free_slots", r.FreeMemorySlots)
			res.Discarded = true
			res.Err = ErrDiscarded
			break
		}
	}
	res.Budget = q.gw.Budget()
	return res
}

// sendResultLine rebuilds the S line the Cube answered with.
func sendResultLine(r *cube.SendResult) cube.Message {
	d := 0
	if r.Discarded {
		d = 1
	}
	return cube.Message{Type: 'S', Data: fmt.Sprintf("%02x,%d,%02x", r.DutyCycle, d, r.FreeMemorySlots)}
}

// deviceInError reports whether rf's communication error, the error
// flag (byte 5's 0x08), is set: the Cube's last radiogram to it failed,
// and the Cube may hold back the next one until an r:. The link error
// (byte 6's 0x40) is the device's own report of its partners, which r:
// does not touch, so it calls for none (AP RadioHealth v1.2, D111-A).
func (q *Queue) deviceInError(rf cube.RFAddress) bool {
	st, ok := q.gw.Snapshot().State(rf)
	return ok && st.Error
}

// flaggedMembers finds the members a command's room-flagged radio lines
// reach (AP RadioHealth v1.2, Block 2, D110-A): a room-flagged s: line
// sets every device whose C line names its room, whichever member its
// header names (T135, docs/cube-protocol.md), and the Cube may hold one
// back for a member whose communication error is set, so each such
// member is reset first, as the original software does. It returns the
// rooms in the order the lines name them and, by room, the members
// whose error flag is set, in the Cube's order; the command's Target,
// which its own prelude resets, and the devices the command's own r:
// lines reset (cubeio's removals and moves) are left out.
func (q *Queue) flaggedMembers(cmd Command) ([]uint8, map[uint8][]cube.RFAddress) {
	var rooms []uint8
	for _, l := range cmd.Lines {
		if room := lineRoom(l); room != 0 && !slices.Contains(rooms, room) {
			rooms = append(rooms, room)
		}
	}
	if len(rooms) == 0 {
		return nil, nil
	}
	skip := ownResets(cmd)
	snap := q.gw.Snapshot()
	members := map[uint8][]cube.RFAddress{}
	for _, rf := range snap.Devices() {
		c := snap.Config(rf)
		if c == nil || c.Type == cube.DeviceCube || skip[rf] || !slices.Contains(rooms, c.RoomID) {
			continue
		}
		if st, ok := snap.State(rf); ok && st.Error {
			members[c.RoomID] = append(members[c.RoomID], rf)
		}
	}
	return rooms, members
}

// flaggedAddressees finds the devices a command's direct radio lines
// address (AP RadioHealth v1.2, D182-A): an s: line without the room flag
// goes to the device its header names, as the room configuration
// write's 0x11 and 0x12 to each thermostat and a copy's steps do, and
// the Cube may hold it back where that device's communication error is
// set, so each such device is reset first, as the original software
// resets every flagged device in a command's scope. It returns the
// devices whose error flag is set, in the order the lines name them,
// each once; the command's Target, which its own prelude resets, and the
// devices the command's own r: lines reset are left out.
func (q *Queue) flaggedAddressees(cmd Command) []cube.RFAddress {
	skip, snap := ownResets(cmd), q.gw.Snapshot()
	var out []cube.RFAddress
	for _, l := range cmd.Lines {
		rf := lineDevice(l)
		if rf == 0 || skip[rf] || slices.Contains(out, rf) {
			continue
		}
		if st, ok := snap.State(rf); ok && st.Error {
			out = append(out, rf)
		}
	}
	return out
}

// ownResets is the set of devices a command resets itself: its Target,
// by the prelude, and the devices of its own r: lines (cubeio's
// removals and moves).
func ownResets(cmd Command) map[cube.RFAddress]bool {
	skip := map[cube.RFAddress]bool{cmd.Target: true}
	for _, l := range cmd.Lines {
		if l.Type != 'r' {
			continue
		}
		addrs, _ := cube.ParseResetError(l.Data)
		for _, a := range addrs {
			skip[a] = true
		}
	}
	return skip
}

// resetForRoom reports whether rf is among the flagged members the
// prelude reset for a room.
func resetForRoom(members map[uint8][]cube.RFAddress, rf cube.RFAddress) bool {
	for _, devs := range members {
		if slices.Contains(devs, rf) {
			return true
		}
	}
	return false
}

// resetMembers sends one r: for each of devs, flagged members of room,
// and logs each.
func (q *Queue) resetMembers(ctx context.Context, room uint8, devs []cube.RFAddress) error {
	for _, rf := range devs {
		if _, err := q.gw.SendRadio(ctx, cube.CmdResetError(rf)); err != nil {
			return err
		}
		q.opts.Logger.Info("communication error reset before a room command", "device", rf, "room", room)
		q.sleep(ctx, q.opts.Gap)
	}
	return nil
}

// resetAddressee sends one r: for rf, a flagged device a direct line of
// the command addresses, and logs it with the room of its C line.
func (q *Queue) resetAddressee(ctx context.Context, rf cube.RFAddress) error {
	if _, err := q.gw.SendRadio(ctx, cube.CmdResetError(rf)); err != nil {
		return err
	}
	var room uint8
	if c := q.gw.Snapshot().Config(rf); c != nil {
		room = c.RoomID
	}
	q.opts.Logger.Info("communication error reset before a command", "device", rf, "room", room)
	q.sleep(ctx, q.opts.Gap)
	return nil
}

// lineRoom is the room a room-flagged s: line names; 0 for any other
// line.
func lineRoom(l cube.Message) uint8 {
	if l.Type != 's' {
		return 0
	}
	rc, err := cube.ParseRadioCommand(l.Data)
	if err != nil || !rc.Target.Room {
		return 0
	}
	return rc.Target.RoomID
}

// lineDevice is the device an s: line without the room flag addresses;
// 0 for any other line.
func lineDevice(l cube.Message) cube.RFAddress {
	if l.Type != 's' {
		return 0
	}
	rc, err := cube.ParseRadioCommand(l.Data)
	if err != nil || rc.Target.Room {
		return 0
	}
	return rc.Target.RFAddress
}

// Concerns reports whether a command of Massimo's own that is waiting
// or in flight addresses rf or its room (the room of its C line): by
// its Target or its RoomID, or by one of its lines, a room-flagged s:
// by its room, any other s: or an r: by its device. Commands of the
// LAN proxy's clients (Passthrough) and reconnects do not count. The
// server's automatic resets ask it, so that they leave a device alone
// while a command to it or its room may reset it (AP RadioHealth v1.2,
// Block 4).
func (q *Queue) Concerns(rf cube.RFAddress) bool {
	var room uint8
	if c := q.gw.Snapshot().Config(rf); c != nil {
		room = c.RoomID
	}
	q.mu.Lock()
	cmds := make([]Command, 0, len(q.items)+1)
	for _, e := range q.items {
		cmds = append(cmds, e.cmd)
	}
	if q.inFlght != nil {
		cmds = append(cmds, q.inFlght.cmd)
	}
	q.mu.Unlock()
	for _, cmd := range cmds {
		if concerns(cmd, rf, room) {
			return true
		}
	}
	return false
}

// concerns reports whether cmd addresses rf or room (0 for none).
func concerns(cmd Command, rf cube.RFAddress, room uint8) bool {
	if cmd.Passthrough || cmd.Reconnect {
		return false
	}
	if cmd.Target == rf || room != 0 && cmd.RoomID == room {
		return true
	}
	for _, l := range cmd.Lines {
		switch l.Type {
		case 's':
			rc, err := cube.ParseRadioCommand(l.Data)
			if err == nil && (rc.Target.Room && room != 0 && rc.Target.RoomID == room || !rc.Target.Room && rc.Target.RFAddress == rf) {
				return true
			}
		case 'r':
			addrs, _ := cube.ParseResetError(l.Data)
			if slices.Contains(addrs, rf) {
				return true
			}
		}
	}
	return false
}

// rereadBudget reads the Cube's budget again by reconnecting, which
// costs no radio time: the Cube sends its H line with the duty cycle and
// the free slots at every connection. It reports false where the gateway
// cannot reconnect or the reconnect failed.
func (q *Queue) rereadBudget(ctx context.Context) bool {
	rc, ok := q.gw.(Reconnector)
	if !ok {
		return false
	}
	rctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	if err := rc.Reconnect(rctx); err != nil {
		q.opts.Logger.Warn("radio budget not read again", "error", err)
		return false
	}
	return true
}

// onHold tells OnHold, where set, that the radio commands wait.
func (q *Queue) onHold(h Hold) {
	if q.opts.OnHold != nil {
		q.opts.OnHold(h)
	}
}

// waitForBudget blocks while the duty cycle or free slots are exhausted:
// at a full duty cycle for DutyWait at a time, reading the budget again
// after each, until it has fallen or cannot be read again.
func (q *Queue) waitForBudget(ctx context.Context) error {
	b := q.gw.Budget()
	for b.DutyCyclePct >= q.opts.MaxDutyPct {
		until := q.opts.Now().Add(q.opts.DutyWait)
		q.onHold(Hold{Reason: HoldDutyCycle, DutyPct: b.DutyCyclePct, Until: until})
		q.holdLog().Warn("radio duty cycle exhausted, holding radio commands until the budget is read again", "duty_pct", b.DutyCyclePct, "until", until)
		q.sleep(ctx, q.opts.DutyWait)
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if !q.rereadBudget(ctx) {
			break // the next command's reply reads it
		}
		b = q.gw.Budget()
	}
	if b.UpdatedAt.IsZero() {
		return nil // nothing known yet; the first command tells us
	}
	if b.FreeMemorySlots < q.opts.MinFreeSlots {
		q.onHold(Hold{Reason: HoldMemorySlots, FreeSlots: b.FreeMemorySlots, Wait: q.opts.SlotWait})
		q.holdLog().Warn("few free memory slots on the cube, holding radio commands", "free", b.FreeMemorySlots, "wait", q.opts.SlotWait)
		q.sleep(ctx, q.opts.SlotWait)
		return ctx.Err()
	}
	return nil
}
