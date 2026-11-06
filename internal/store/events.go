// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package store

// The Cubes' events (docs/store.md section 9 (d)): what the server's
// event stream sends, kept in the events table so that the log survives
// a restart and reaches back further than the 500 events the server
// held in memory. AddEvent never blocks: it hands the event to one
// goroutine, RunEvents, which writes a batch in one transaction at 100
// events or a second after the first, before every read of the log
// (FlushEvents) and at its stop, so that a burst after a reconnect is
// one write to the SD card rather than hundreds. The same goroutine
// prunes the events older than log.keep_days, at its start, daily and
// when the setting changes, in chunks, so that the one connection is
// never held for long. The server never runs VACUUM: the pages freed
// are reused by the next days' events, and store backup writes a
// compact copy.

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// LogKeepDaysKey is the setting that says how many days the events are
// kept; 0 keeps them, unset is DefaultLogKeepDays.
const LogKeepDaysKey = "log.keep_days"

// DefaultLogKeepDays is the events' retention when the setting is unset
// (docs/store.md section 10, decision 5).
const DefaultLogKeepDays = 30

// The writer's tuning; variables so that the tests may lower them.
var (
	flushAt     = 100              // pending events that start a write
	flushAfter  = time.Second      // the longest an event waits for its write
	eventBuffer = 4096             // events handed over and not yet taken
	pruneEvery  = 24 * time.Hour   // between two prunes
	pruneChunk  = 5000             // rows a prune deletes per statement
	warnEvery   = time.Minute      // between two warnings of one kind
	eventsQuiet = 10 * time.Second // how long FlushEvents waits for a writer that is gone
)

// MaxEventsPage is the most events a page of Events holds.
const MaxEventsPage = 1000

// Event is one row of events.
type Event struct {
	ID     int64
	CubeID int64
	Time   time.Time
	Kind   string
	Device string // a radio address, "" for a Cube-level event
	Data   json.RawMessage
}

// EventQuery selects a page of a Cube's events: those before an id or a
// time, of a kind, of a device, at most Limit (MaxEventsPage).
type EventQuery struct {
	CubeID   int64
	BeforeID int64     // 0: no bound
	Before   time.Time // zero: no bound
	Kind     string    // "": every kind
	Device   string    // "": every device
	Limit    int
}

// eventWriter is the state of the writer goroutine.
type eventWriter struct {
	queue chan Event
	flush chan chan error
	prune chan struct{}

	mu   sync.Mutex
	done chan struct{} // closed when the running writer stops; nil without one

	written, dropped, flushes atomic.Int64

	warnMu   sync.Mutex
	lastWarn map[string]time.Time
}

func (w *eventWriter) init() {
	w.queue = make(chan Event, eventBuffer)
	w.flush = make(chan chan error)
	w.prune = make(chan struct{}, 1)
	w.lastWarn = map[string]time.Time{}
}

// warn reports whether a warning of a kind is due: once a minute.
func (s *Store) warn(kind string) bool {
	w := &s.ev
	w.warnMu.Lock()
	defer w.warnMu.Unlock()
	now := time.Now()
	if now.Sub(w.lastWarn[kind]) < warnEvery {
		return false
	}
	w.lastWarn[kind] = now
	return true
}

// AddEvent hands an event to the writer and returns at once. An event
// without a Cube or with data that is not JSON is dropped, as is one the
// writer has no room for, or one for a store opened read-only; a drop is
// counted (EventStats) and warned of once a minute.
func (s *Store) AddEvent(e Event) {
	if e.CubeID == 0 || len(e.Data) == 0 || !json.Valid(e.Data) {
		s.log.Debug("store: event dropped: no cube or no JSON", "kind", e.Kind)
		s.ev.dropped.Add(1)
		return
	}
	if err := s.writable(); err != nil {
		s.ev.dropped.Add(1)
		if s.warn("readonly") {
			s.log.Error("store: event dropped", "error", err)
		}
		return
	}
	select {
	case s.ev.queue <- e:
	default:
		n := s.ev.dropped.Add(1)
		if s.warn("full") {
			s.log.Warn("store: events dropped, the writer is behind", "dropped", n)
		}
	}
}

// EventStats returns how many events were written, how many dropped, and
// in how many transactions.
func (s *Store) EventStats() (written, dropped, flushes int64) {
	return s.ev.written.Load(), s.ev.dropped.Load(), s.ev.flushes.Load()
}

// RunEvents is the writer: it writes the events AddEvent hands over and
// prunes them, until ctx ends, then writes what is pending and returns.
// One runs at a time; a store opened read-only has none.
func (s *Store) RunEvents(ctx context.Context) {
	if s.readOnly {
		return
	}
	w := &s.ev
	done := make(chan struct{})
	w.mu.Lock()
	if w.done != nil {
		w.mu.Unlock()
		s.log.Error("store: an events writer runs already")
		return
	}
	w.done = done
	w.mu.Unlock()
	defer func() {
		w.mu.Lock()
		w.done = nil
		w.mu.Unlock()
		close(done)
	}()

	var batch []Event
	var timer *time.Timer
	var due <-chan time.Time
	write := func() error {
		if timer != nil {
			timer.Stop()
			timer, due = nil, nil
		}
		if len(batch) == 0 {
			return nil
		}
		err := s.writeEvents(context.WithoutCancel(ctx), batch)
		batch = batch[:0]
		return err
	}
	drain := func() {
		for {
			select {
			case e := <-w.queue:
				batch = append(batch, e)
			default:
				return
			}
		}
	}
	prune := func() {
		if _, err := s.PruneEvents(context.WithoutCancel(ctx), s.now()); err != nil && s.warn("prune") {
			s.log.Error("store: events not pruned", "error", err)
		}
	}
	prune()
	ticker := time.NewTicker(pruneEvery)
	defer ticker.Stop()
	for {
		select {
		case e := <-w.queue:
			batch = append(batch, e)
			if len(batch) >= flushAt {
				write()
			} else if due == nil {
				timer = time.NewTimer(flushAfter)
				due = timer.C
			}
		case <-due:
			write()
		case reply := <-w.flush:
			drain()
			reply <- write()
		case <-w.prune:
			drain()
			write()
			prune()
		case <-ticker.C:
			drain()
			write()
			prune()
		case <-ctx.Done():
			drain()
			write()
			return
		}
	}
}

// writeEvents writes a batch in one transaction; a failure drops the
// batch, and is logged once a minute.
func (s *Store) writeEvents(ctx context.Context, batch []Event) error {
	err := func() error {
		tx, err := s.db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		defer tx.Rollback()
		stmt, err := tx.PrepareContext(ctx, `INSERT INTO events (uuid, cube_id, time, kind, device, data) VALUES (?, ?, ?, ?, ?, ?)`)
		if err != nil {
			return err
		}
		defer stmt.Close()
		for _, e := range batch {
			at := e.Time
			if at.IsZero() {
				at = s.now()
			}
			if _, err := stmt.ExecContext(ctx, newUUID(), e.CubeID, formatTime(at), e.Kind, e.Device, string(e.Data)); err != nil {
				return err
			}
		}
		return tx.Commit()
	}()
	if err != nil {
		s.ev.dropped.Add(int64(len(batch)))
		if s.warn("write") {
			s.log.Error("store: events not written", "events", len(batch), "error", err)
		}
		return fmt.Errorf("store: events: %w", err)
	}
	s.ev.written.Add(int64(len(batch)))
	s.ev.flushes.Add(1)
	return nil
}

// FlushEvents asks the writer to write what is pending and waits until
// it is readable; without a writer it returns at once.
func (s *Store) FlushEvents(ctx context.Context) error {
	w := &s.ev
	w.mu.Lock()
	done := w.done
	w.mu.Unlock()
	if done == nil {
		return nil
	}
	reply := make(chan error, 1)
	select {
	case w.flush <- reply:
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
	select {
	case err := <-reply:
		return err
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(eventsQuiet):
		return errors.New("store: the events writer did not answer")
	}
}

// PruneEventsSoon asks the writer to prune now, after a change of
// log.keep_days.
func (s *Store) PruneEventsSoon() {
	select {
	case s.ev.prune <- struct{}{}:
	default: // one is pending already
	}
}

// LogKeepDays returns the events' retention in days; 0 keeps them.
func (s *Store) LogKeepDays(ctx context.Context) (int, error) {
	n, ok, err := s.SettingInt(ctx, LogKeepDaysKey)
	if err != nil {
		return 0, err
	}
	if !ok {
		return DefaultLogKeepDays, nil
	}
	return n, nil
}

// PruneEvents removes the events older than log.keep_days before now,
// per Cube and in chunks, and returns how many it removed.
func (s *Store) PruneEvents(ctx context.Context, now time.Time) (int, error) {
	if err := s.writable(); err != nil {
		return 0, err
	}
	days, err := s.LogKeepDays(ctx)
	if err != nil || days <= 0 {
		return 0, err
	}
	cutoff := formatTime(now.Add(-time.Duration(days) * 24 * time.Hour))
	rows, err := s.db.QueryContext(ctx, `SELECT id FROM cubes ORDER BY id`)
	if err != nil {
		return 0, fmt.Errorf("store: prune events: %w", err)
	}
	var cubes []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return 0, fmt.Errorf("store: prune events: %w", err)
		}
		cubes = append(cubes, id)
	}
	rows.Close()
	removed := 0
	for _, cube := range cubes {
		for {
			res, err := s.db.ExecContext(ctx, `DELETE FROM events WHERE id IN (SELECT id FROM events WHERE cube_id = ? AND time < ? LIMIT ?)`, cube, cutoff, pruneChunk)
			if err != nil {
				return removed, fmt.Errorf("store: prune events: %w", err)
			}
			n, _ := res.RowsAffected()
			removed += int(n)
			if n == 0 {
				break
			}
		}
	}
	if removed > 0 {
		s.log.Info("events pruned", "removed", removed, "keep_days", days)
	}
	return removed, nil
}

// Events returns a page of a Cube's events, newest first: at most
// q.Limit (MaxEventsPage, 1 at least) before q.BeforeID and q.Before,
// of q.Kind and q.Device where given.
func (s *Store) Events(ctx context.Context, q EventQuery) ([]Event, error) {
	limit := q.Limit
	if limit <= 0 || limit > MaxEventsPage {
		limit = MaxEventsPage
	}
	before := ""
	if !q.Before.IsZero() {
		before = formatTime(q.Before)
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id, cube_id, time, kind, device, data FROM events
		WHERE cube_id = ? AND (? = 0 OR id < ?) AND (? = '' OR time < ?) AND (? = '' OR kind = ?) AND (? = '' OR device = ?)
		ORDER BY id DESC LIMIT ?`,
		q.CubeID, q.BeforeID, q.BeforeID, before, before, q.Kind, q.Kind, q.Device, q.Device, limit)
	if err != nil {
		return nil, fmt.Errorf("store: events: %w", err)
	}
	defer rows.Close()
	out := []Event{}
	for rows.Next() {
		var e Event
		var at sql.NullString
		var data string
		if err := rows.Scan(&e.ID, &e.CubeID, &at, &e.Kind, &e.Device, &data); err != nil {
			return nil, fmt.Errorf("store: events: %w", err)
		}
		e.Time, e.Data = parseTime(at), json.RawMessage(data)
		out = append(out, e)
	}
	return out, rows.Err()
}
