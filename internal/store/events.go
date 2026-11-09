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
	"strings"
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
	eventsQuiet = 10 * time.Second // how long FlushEvents waits for the writer's answer
)

// pruneStep runs after each chunk of a prune, nil but in the tests.
var pruneStep func()

// MaxEventsPage is the most events a page of Events holds.
const MaxEventsPage = 1000

// The levels of an event, slog's numbers (migration 0009), so that "this
// level and above" is level >= ?.
const (
	LevelDebug = -4
	LevelInfo  = 0
	LevelWarn  = 4
	LevelError = 8
)

// LevelName is a level's name as MASSIMO_LOG_LEVEL and the API write it.
func LevelName(level int) string {
	switch {
	case level < LevelInfo:
		return "debug"
	case level < LevelWarn:
		return "info"
	case level < LevelError:
		return "warn"
	}
	return "error"
}

// ParseLevel reads a level's name, in any case.
func ParseLevel(name string) (int, bool) {
	switch strings.ToLower(name) {
	case "debug":
		return LevelDebug, true
	case "info":
		return LevelInfo, true
	case "warn", "warning":
		return LevelWarn, true
	case "error":
		return LevelError, true
	}
	return 0, false
}

// Event is one row of events: a Cube's, or with CubeID 0 the server's.
type Event struct {
	ID     int64
	CubeID int64 // 0: the server's, cube_id NULL
	Time   time.Time
	Kind   string
	Source string // device, cube, system or config
	Level  int    // LevelDebug … LevelError
	Device string // a radio address, "" for an event of no device
	Serial string // the device's serial number, or the Cube's for its own lines
	UserID int64  // who started it, 0 for nobody
	Data   json.RawMessage
}

// EventQuery selects a page of a Cube's events, and with Server the
// server's as well: those before an id or a time and at or after a time,
// of a kind, a source, a device, a serial and a user, at and above a
// level, at most Limit (MaxEventsPage).
type EventQuery struct {
	CubeID   int64
	Server   bool      // the server's events too (cube_id NULL)
	BeforeID int64     // 0: no bound
	Before   time.Time // zero: no bound
	Since    time.Time // zero: no bound; the events at or after it
	Kind     string    // "": every kind
	Source   string    // "": every source
	Device   string    // "": every device; a radio address, which no index serves
	Serial   string    // "": every device; a serial number
	UserID   int64     // 0: anyone's and no one's
	MinLevel *int      // nil: every level
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

// AddEvent hands an event to the writer and returns at once; one of
// CubeID 0 is the server's. An event with data that is not JSON is
// dropped, as is one the writer has no room for, or one for a store
// opened read-only; a drop is counted (EventStats) and warned of once a
// minute.
func (s *Store) AddEvent(e Event) {
	if len(e.Data) == 0 || !json.Valid(e.Data) {
		s.log.Debug("store: event dropped: no JSON", "kind", e.Kind)
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
	// Between two chunks of a prune the writer goes on with its work: it
	// answers a flush, writes what is due, and ends the prune once ctx is
	// done, so that neither a read nor a shutdown waits for all of it.
	between := func() bool {
		drain()
		select {
		case reply := <-w.flush:
			reply <- write()
		case <-due:
			write()
		default:
			if len(batch) >= flushAt {
				write()
			} else if len(batch) > 0 && due == nil {
				timer = time.NewTimer(flushAfter)
				due = timer.C
			}
		}
		return ctx.Err() == nil
	}
	prune := func() {
		if _, err := s.pruneEvents(context.WithoutCancel(ctx), s.now(), between); err != nil && s.warn("prune") {
			s.log.Error("store: events not pruned", "error", err)
		}
	}
	if err := s.fillSources(ctx); err != nil {
		s.log.Error("store: the events of an older binary keep no source", "error", err)
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
		stmt, err := tx.PrepareContext(ctx, `INSERT INTO events (uuid, cube_id, time, kind, source, level, device, serial, user_id, data) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
		if err != nil {
			return err
		}
		defer stmt.Close()
		for _, e := range batch {
			at := e.Time
			if at.IsZero() {
				at = s.now()
			}
			if _, err := stmt.ExecContext(ctx, newUUID(), nullID(e.CubeID), formatTime(at), e.Kind, e.Source, e.Level, e.Device, e.Serial, nullID(e.UserID), string(e.Data)); err != nil {
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

// InsertEvent writes one event at once, without the writer: for a
// process that runs none, the command line's massimo user.
func (s *Store) InsertEvent(ctx context.Context, e Event) error {
	if len(e.Data) == 0 || !json.Valid(e.Data) {
		return errors.New("store: an event without JSON")
	}
	if err := s.writable(); err != nil {
		return err
	}
	return s.writeEvents(ctx, []Event{e})
}

// FlushEvents asks the writer to write what is pending and waits until
// it is readable; without a writer it returns at once, and it gives up
// after eventsQuiet, the handover to the writer included.
func (s *Store) FlushEvents(ctx context.Context) error {
	w := &s.ev
	w.mu.Lock()
	done := w.done
	w.mu.Unlock()
	if done == nil {
		return nil
	}
	quiet := time.NewTimer(eventsQuiet)
	defer quiet.Stop()
	reply := make(chan error, 1)
	select {
	case w.flush <- reply:
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-quiet.C:
		return errors.New("store: the events writer did not answer")
	}
	select {
	case err := <-reply:
		return err
	case <-ctx.Done():
		return ctx.Err()
	case <-quiet.C:
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
// per Cube and then the server's, in chunks, and returns how many it
// removed.
func (s *Store) PruneEvents(ctx context.Context, now time.Time) (int, error) {
	return s.pruneEvents(ctx, now, nil)
}

// pruneEvents is PruneEvents with between, when given, run after each
// chunk that removed rows; the prune stops where it returns false.
func (s *Store) pruneEvents(ctx context.Context, now time.Time, between func() bool) (int, error) {
	if err := s.writable(); err != nil {
		return 0, err
	}
	days, err := s.LogKeepDays(ctx)
	if err != nil || days <= 0 {
		return 0, err
	}
	cutoff := formatTime(now.Add(-time.Duration(days) * 24 * time.Hour))
	cubes, err := s.cubeIDs(ctx)
	if err != nil {
		return 0, fmt.Errorf("store: prune events: %w", err)
	}
	cubes = append(cubes, 0) // the server's, cube_id NULL
	removed, stopped := 0, false
cubes:
	for _, cube := range cubes {
		for {
			res, err := s.db.ExecContext(ctx, `DELETE FROM events WHERE id IN (SELECT id FROM events WHERE `+cubeTerm(cube)+` AND time < ? LIMIT ?)`,
				cubeArgs(cube, cutoff, pruneChunk)...)
			if err != nil {
				return removed, fmt.Errorf("store: prune events: %w", err)
			}
			n, _ := res.RowsAffected()
			removed += int(n)
			if pruneStep != nil {
				pruneStep()
			}
			if n == 0 {
				break
			}
			if between != nil && !between() {
				stopped = true
				break cubes
			}
		}
	}
	if removed > 0 || stopped {
		s.log.Info("events pruned", "removed", removed, "keep_days", days, "stopped", stopped)
	}
	if removed > 0 {
		s.logPruned(removed, days, now)
	}
	return removed, nil
}

// logPruned writes the prune's own line, an event of the server's
// (source system) as the server's log.go writes them (AP Logging v1.1,
// Block 5): the store prunes by itself, without a request.
func (s *Store) logPruned(removed, days int, now time.Time) {
	data, _ := json.Marshal(map[string]any{"kind": "log_pruned", "source": "system", "level": "info",
		"summary": map[string]any{"removed": removed, "keep_days": days}, "time": now})
	s.AddEvent(Event{Kind: "log_pruned", Source: "system", Level: LevelInfo, Time: now, Data: data})
}

// cubeIDs lists the store's Cubes by id.
func (s *Store) cubeIDs(ctx context.Context) ([]int64, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id FROM cubes ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

// cubeTerm is the term of a Cube's rows, the server's for cube 0;
// cubeArgs puts the Cube's id, where the term binds one, before args.
func cubeTerm(cube int64) string {
	if cube == 0 {
		return "cube_id IS NULL"
	}
	return "cube_id = ?"
}

func cubeArgs(cube int64, args ...any) []any {
	if cube == 0 {
		return args
	}
	return append([]any{cube}, args...)
}

// fillSources gives the rows a binary before migration 0009 wrote (an
// older binary run on this file after a rollback) the source, level and
// serial that 0009 gives the rows it copies, from their own JSON; per
// Cube, on the index of (cube_id, source).
func (s *Store) fillSources(ctx context.Context) error {
	cubes, err := s.cubeIDs(ctx)
	if err != nil {
		return err
	}
	for _, cube := range cubes {
		if _, err := s.db.ExecContext(ctx, `UPDATE events SET
			source = CASE
				WHEN coalesce(json_extract(data, '$.source'), '') != '' THEN json_extract(data, '$.source')
				WHEN kind IN ('restore_step', 'restore_done') THEN 'system'
				WHEN device = '' THEN 'cube'
				ELSE 'device' END,
			level = CASE kind WHEN 'disconnected' THEN 4 WHEN 'hello' THEN -4 ELSE 0 END,
			serial = coalesce(nullif(json_extract(data, '$.serial'), ''), nullif(json_extract(data, '$.state.serial'), ''), '')
			WHERE cube_id = ? AND source = ''`, cube); err != nil {
			return err
		}
	}
	return nil
}

// Events returns a page of a Cube's events, and with q.Server the
// server's as well, newest first: at most q.Limit (MaxEventsPage, 1 at
// least) before q.BeforeID and q.Before and not before q.Since, of
// q.Kind, q.Source, q.Device, q.Serial and q.UserID, at and above
// q.MinLevel, where given. The Cube's rows and the server's are two walks
// in id order, merged: one query for both would sort them.
func (s *Store) Events(ctx context.Context, q EventQuery) ([]Event, error) {
	limit := q.Limit
	if limit <= 0 || limit > MaxEventsPage {
		limit = MaxEventsPage
	}
	out, err := s.eventsWalk(ctx, q, q.CubeID, limit)
	if err != nil || !q.Server || q.CubeID == 0 {
		return out, err
	}
	server, err := s.eventsWalk(ctx, q, 0, limit)
	if err != nil {
		return nil, err
	}
	merged := make([]Event, 0, min(limit, len(out)+len(server)))
	for i, j := 0, 0; len(merged) < limit && (i < len(out) || j < len(server)); {
		if j >= len(server) || i < len(out) && out[i].ID > server[j].ID {
			merged, i = append(merged, out[i]), i+1
		} else {
			merged, j = append(merged, server[j]), j+1
		}
	}
	return merged, nil
}

// eventsWalk reads one walk of a page: the rows of cube, the server's
// for 0, newest first.
func (s *Store) eventsWalk(ctx context.Context, q EventQuery, cube int64, limit int) ([]Event, error) {
	query, args := eventsQuery(q, cube, limit)
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("store: events: %w", err)
	}
	defer rows.Close()
	out := []Event{}
	for rows.Next() {
		var e Event
		var cubeID, userID sql.NullInt64
		var at sql.NullString
		var data string
		if err := rows.Scan(&e.ID, &cubeID, &at, &e.Kind, &e.Source, &e.Level, &e.Device, &e.Serial, &userID, &data); err != nil {
			return nil, fmt.Errorf("store: events: %w", err)
		}
		e.CubeID, e.UserID = cubeID.Int64, userID.Int64
		e.Time, e.Data = parseTime(at), json.RawMessage(data)
		out = append(out, e)
	}
	return out, rows.Err()
}

// eventsQuery is the SELECT of one walk of a page, the rows of cube (the
// server's for 0) with only the terms q gives, every value bound, so
// that SQLite walks one index in id order from the end instead of
// sorting all of the Cube's rows. A time bound becomes an id bound
// first, the newest id before that time or the oldest at or after it,
// and stays a filter without its index (+time), which keeps the id
// order; the level is such a filter always, a range having no index.
func eventsQuery(q EventQuery, cube int64, limit int) (string, []any) {
	where, args := []string{cubeTerm(cube)}, cubeArgs(cube)
	if q.BeforeID > 0 {
		where, args = append(where, "id < ?"), append(args, q.BeforeID)
	}
	if !q.Before.IsZero() {
		at := formatTime(q.Before)
		where = append(where, "id <= (SELECT max(id) FROM events WHERE "+cubeTerm(cube)+" AND time < ?)", "+time < ?")
		args = append(args, cubeArgs(cube, at, at)...)
	}
	if !q.Since.IsZero() {
		at := formatTime(q.Since)
		where = append(where, "id >= (SELECT min(id) FROM events WHERE "+cubeTerm(cube)+" AND time >= ?)", "+time >= ?")
		args = append(args, cubeArgs(cube, at, at)...)
	}
	if q.Kind != "" {
		where, args = append(where, "kind = ?"), append(args, q.Kind)
	}
	if q.Source != "" {
		where, args = append(where, "source = ?"), append(args, q.Source)
	}
	if q.Device != "" {
		where, args = append(where, "device = ?"), append(args, q.Device)
	}
	if q.Serial != "" {
		where, args = append(where, "serial = ?"), append(args, q.Serial)
	}
	if q.UserID != 0 {
		where, args = append(where, "user_id = ?"), append(args, q.UserID)
	}
	if q.MinLevel != nil {
		where, args = append(where, "+level >= ?"), append(args, *q.MinLevel)
	}
	return `SELECT id, cube_id, time, kind, source, level, device, serial, user_id, data FROM events WHERE ` + strings.Join(where, " AND ") +
		` ORDER BY id DESC LIMIT ?`, append(args, limit)
}
