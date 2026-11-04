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
	"sort"
	"sync"
	"time"

	"go.metaworx.ch/massimo/internal/cubelan"
	"go.metaworx.ch/massimo/internal/protocol/cube"
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
}

// Result is the outcome of a Command.
type Result struct {
	Discarded bool // the Cube rejected a radio command (budget exhausted)
	Budget    cubelan.Budget
	Err       error
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
	// MaxDutyPct is the duty cycle above which radio commands wait for
	// the hourly reset. The Cube reports 0..100.
	MaxDutyPct int
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
	// Now and ResetAt exist for tests; ResetAt returns when the duty
	// cycle next resets (default: the next full hour plus a margin).
	Now     func() time.Time
	ResetAt func(now time.Time) time.Time
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
	if o.ResetAt == nil {
		o.ResetAt = func(now time.Time) time.Time {
			return now.Truncate(time.Hour).Add(time.Hour + 15*time.Second)
		}
	}
}

type entry struct {
	cmd    Command
	ticket *Ticket
	seq    uint64
}

// Queue serialises commands to one Cube and enforces the radio budget.
type Queue struct {
	gw   Gateway
	opts Options

	mu      sync.Mutex
	items   []*entry
	seq     uint64
	wake    chan struct{}
	inFlght *entry
}

// NewQueue creates a queue for gw; Run starts it.
func NewQueue(gw Gateway, opts Options) *Queue {
	opts.defaults()
	return &Queue{gw: gw, opts: opts, wake: make(chan struct{}, 1)}
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
				continue
			}
			kept = append(kept, e)
		}
		q.items = kept
	}
	q.seq++
	q.items = append(q.items, &entry{cmd: cmd, ticket: t, seq: q.seq})
	q.mu.Unlock()
	select {
	case q.wake <- struct{}{}:
	default:
	}
	return t
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
		res := q.execute(ctx, e.cmd)
		e.ticket.done <- res
		q.mu.Lock()
		q.inFlght = nil
		q.mu.Unlock()
		q.sleep(ctx, q.opts.Gap)
	}
}

func (q *Queue) drain(err error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	for _, e := range q.items {
		e.ticket.done <- Result{Err: err}
	}
	q.items = nil
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
	needsRadio := false
	for _, l := range cmd.Lines {
		if isRadio(l) {
			needsRadio = true
		}
	}
	if needsRadio {
		if err := q.waitForBudget(ctx); err != nil {
			return Result{Err: err, Budget: q.gw.Budget()}
		}
	}
	if cmd.Wakeup {
		if _, err := q.gw.Send(ctx, cube.CmdWakeup(q.opts.WakeupSeconds, cube.WakeupRoom, fmt.Sprintf("%02x", cmd.RoomID))); err != nil {
			return Result{Err: fmt.Errorf("wakeup: %w", err), Budget: q.gw.Budget()}
		}
		q.sleep(ctx, q.opts.Gap)
	}
	if needsRadio && cmd.Target != 0 && q.deviceInError(cmd.Target) {
		if _, err := q.gw.SendRadio(ctx, cube.CmdResetError(cmd.Target)); err != nil {
			return Result{Err: fmt.Errorf("reset error state: %w", err), Budget: q.gw.Budget()}
		}
		q.sleep(ctx, q.opts.Gap)
	}
	for i, l := range cmd.Lines {
		if i > 0 {
			q.sleep(ctx, q.opts.Gap)
		}
		if !isRadio(l) {
			if _, err := q.gw.Send(ctx, l); err != nil {
				res.Err = err
				break
			}
			continue
		}
		r, err := q.gw.SendRadio(ctx, l)
		if err != nil {
			res.Err = err
			break
		}
		if r.Discarded && cmd.Target != 0 {
			// Observed on hardware: a command to a device in error state is
			// discarded unless r: was sent first. Reset once and retry.
			q.opts.Logger.Info("command discarded, resetting error state and retrying", "target", cmd.Target)
			if _, err := q.gw.SendRadio(ctx, cube.CmdResetError(cmd.Target)); err != nil {
				res.Err = err
				break
			}
			q.sleep(ctx, q.opts.Gap)
			if r, err = q.gw.SendRadio(ctx, l); err != nil {
				res.Err = err
				break
			}
		}
		if r.Discarded {
			res.Discarded = true
			res.Err = ErrDiscarded
			break
		}
	}
	res.Budget = q.gw.Budget()
	return res
}

func (q *Queue) deviceInError(rf cube.RFAddress) bool {
	st, ok := q.gw.Snapshot().State(rf)
	return ok && (st.Error || st.LinkError)
}

// waitForBudget blocks while the duty cycle or free slots are exhausted.
func (q *Queue) waitForBudget(ctx context.Context) error {
	b := q.gw.Budget()
	if b.DutyCyclePct >= q.opts.MaxDutyPct {
		until := q.opts.ResetAt(q.opts.Now())
		q.opts.Logger.Warn("radio duty cycle exhausted, waiting for the hourly reset", "duty_pct", b.DutyCyclePct, "until", until)
		q.sleep(ctx, until.Sub(q.opts.Now()))
		if ctx.Err() != nil {
			return ctx.Err()
		}
	}
	if b.UpdatedAt.IsZero() {
		return nil // nothing known yet; the first command tells us
	}
	if b.FreeMemorySlots < q.opts.MinFreeSlots {
		q.opts.Logger.Warn("few free memory slots on the cube, holding radio commands", "free", b.FreeMemorySlots, "wait", q.opts.SlotWait)
		q.sleep(ctx, q.opts.SlotWait)
		return ctx.Err()
	}
	return nil
}
