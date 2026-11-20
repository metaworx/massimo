// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package store

// The Cubes' events (docs/store.md section 9 (d)): what the server's
// event stream sends, kept in the log table (the events table before
// migration 0014, and a view of it since) so that the log survives
// a restart and reaches back further than the 500 events the server
// held in memory. AddEvent never blocks: it hands the event to one
// goroutine, RunEvents, which writes a batch in one transaction at 100
// events or a second after the first, before every read of the log
// (FlushEvents) and at its stop, so that a burst after a reconnect is
// one write to the SD card rather than hundreds. The same goroutine
// prunes the events older than log.keep_days, and the audit's rows
// older than audit.keep_days, at its start, daily and when a setting
// changes, in chunks, so that the one connection is never held for
// long. The server never runs VACUUM: the pages freed are reused by the
// next days' events, and store backup writes a compact copy.

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
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

// warnTerm is the WHERE of migration 0011's index of the warnings and
// errors, events_cube_warn, as a page's query writes it (eventsQuery).
var warnTerm = fmt.Sprintf("level >= %d", LevelWarn)

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

// The visibilities of a row: read for what a user with read sees, audit
// for the rows only a user with audit sees (migration 0014).
const (
	VisibilityRead  = "read"
	VisibilityAudit = "audit"
)

// Event is one row of the log: a Cube's, or with CubeID 0 the server's.
type Event struct {
	ID     int64
	CubeID int64 // 0: the server's, cube_id NULL
	Time   time.Time
	Kind   string
	Source string // device, cube, system or config
	Level  int    // LevelDebug … LevelError
	Device string // a radio address, "" for an event of no device
	Serial string // the device's serial number, or the Cube's for its own lines
	// EffectiveUserID is the user whose rights it was started with, 0 for
	// nobody; RealUserID who started it, the same user but where an
	// integration acted for a user linked to its own (migration 0024),
	// EffectiveUserID where 0.
	EffectiveUserID int64
	RealUserID      int64
	Data            json.RawMessage
	// CommandID is the command the row belongs to, 0 for none; the
	// prune of the command clears it.
	CommandID int64
	// Visibility is VisibilityRead or VisibilityAudit; "" is read.
	Visibility string
}

// EventQuery selects a page of a Cube's events, or of several Cubes'
// (CubeIDs), and with Server the server's as well: those after an id,
// before an id or a time and at or after a time, of a kind, a source, a
// device, a serial, a user and a command, at and above a level, of the
// read visibility and with Audit of the audit's too, at most Limit
// (MaxEventsPage): the newest of them, or with Forward the oldest.
type EventQuery struct {
	CubeID int64
	// CubeIDs, where given, are the Cubes whose rows the page holds, in
	// place of CubeID: one walk each, merged by id.
	CubeIDs []int64
	Server  bool  // the server's events too (cube_id NULL)
	AfterID int64 // 0: no bound; else the events whose id is above it
	// Forward walks from the oldest row the bounds let in, oldest first,
	// for a page after a cursor (AfterID); without it the walk is from
	// the newest, newest first.
	Forward   bool
	BeforeID  int64     // 0: no bound
	Before    time.Time // zero: no bound
	Since     time.Time // zero: no bound; the events at or after it
	Kind      string    // "": every kind
	Source    string    // "": every source
	Device    string    // "": every device; a radio address, which no index serves
	Serial    string    // "": every device; a serial number
	UserID    int64     // 0: anyone's and no one's; else the rows of its effective or real user
	CommandID int64     // 0: any command's and none's
	MinLevel  *int      // nil: every level
	Audit     bool      // the rows of the audit visibility too
	Limit     int
	// byReal makes a walk of UserID's rows by their real user alone
	// (Events).
	byReal bool
	// upTo is the newest row a walk reads, the page's bound (pageBound);
	// 0 for none.
	upTo int64
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
		if ctx.Err() != nil {
			return
		}
		if _, err := s.pruneAudit(context.WithoutCancel(ctx), s.now(), between); err != nil && s.warn("prune audit") {
			s.log.Error("store: audit not pruned", "error", err)
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
		stmt, err := tx.PrepareContext(ctx, `INSERT INTO log (uuid, cube_id, time, kind, source, level, device, serial, effective_user_id, real_user_id, data, command_id, visibility)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
		if err != nil {
			return err
		}
		defer stmt.Close()
		for _, e := range batch {
			at := e.Time
			if at.IsZero() {
				at = s.now()
			}
			vis := e.Visibility
			if vis == "" {
				vis = VisibilityRead
			}
			realID := e.RealUserID
			if realID == 0 {
				realID = e.EffectiveUserID
			}
			if _, err := stmt.ExecContext(ctx, newUUID(), nullID(e.CubeID), formatTime(at), e.Kind, e.Source, e.Level, e.Device, e.Serial,
				nullID(e.EffectiveUserID), nullID(realID), string(e.Data), nullID(e.CommandID), vis); err != nil {
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
			res, err := s.db.ExecContext(ctx, `DELETE FROM log WHERE id IN (SELECT id FROM log WHERE `+cubeTerm(cube)+` AND time < ? LIMIT ?)`,
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

// ErrBeyondNewest is a page forward whose cursor (EventQuery.AfterID) is
// beyond the log's newest row: an id the log never gave out, or gave out
// before a restore from an older copy or a prune that emptied it.
var ErrBeyondNewest = errors.New("store: the cursor is beyond the log's newest row")

// Events returns a page of a Cube's events, or of the Cubes of
// q.CubeIDs, and with q.Server the server's as well, newest first: at
// most q.Limit (MaxEventsPage, 1 at least) after q.AfterID, before
// q.BeforeID and q.Before and not before q.Since, of q.Kind, q.Source,
// q.Device, q.Serial and q.UserID, at and above q.MinLevel, where given;
// with q.Forward the oldest q.Limit of them, oldest first, and
// ErrBeyondNewest for a q.AfterID beyond the newest row. Each Cube's
// rows and the server's are a walk of their own in id order, merged:
// one query for all would sort them. The walks are statements of their
// own, between which the writer may commit; so that each sees the same
// rows, every walk of a page forward or of more than one walk stops at
// the newest row read before them (pageBound). A row at or below it is
// committed, since ids are given out in the order of the commits: a page
// leaves no row out below its last, and a page forward from its last
// row misses none. A caller that wants the events added so far in the
// page writes them first (FlushEvents).
func (s *Store) Events(ctx context.Context, q EventQuery) ([]Event, error) {
	limit := q.Limit
	if limit <= 0 || limit > MaxEventsPage {
		limit = MaxEventsPage
	}
	cubes, queries, bounded := pageWalks(q)
	if bounded {
		newest, err := s.NewestEventID(ctx)
		if err != nil {
			return nil, err
		}
		if q.Forward && q.AfterID > newest {
			return nil, ErrBeyondNewest
		}
		if newest == 0 {
			return []Event{}, nil // nothing at or below it
		}
		for i := range queries {
			queries[i] = pageBound(queries[i], newest)
		}
	}
	walks := make([][]Event, 0, len(cubes)*len(queries))
	for _, cube := range cubes {
		for _, wq := range queries {
			out, err := s.eventsWalk(ctx, wq, cube, limit)
			if err != nil {
				return nil, err
			}
			walks = append(walks, out)
		}
	}
	if len(walks) == 1 {
		return walks[0], nil
	}
	return mergeWalks(walks, limit, q.Forward), nil
}

// pageWalks are the walks of a page: the Cubes whose rows it reads, the
// server's as 0, each with each of the queries, and whether Events
// bounds them by the newest row (pageBound): a page forward, and one of
// more than one walk. A user's rows are two walks of each Cube's, each
// by its index (log_user, log_real_user): those made with its rights,
// and those it made acting as another user (an integration's user
// linked to one).
func pageWalks(q EventQuery) (cubes []int64, queries []EventQuery, bounded bool) {
	cubes = []int64{q.CubeID}
	if len(q.CubeIDs) > 0 {
		cubes = slices.Compact(slices.Sorted(slices.Values(q.CubeIDs)))
	}
	if q.Server && !slices.Contains(cubes, 0) {
		cubes = append(cubes, 0)
	}
	queries = []EventQuery{q}
	if q.UserID != 0 {
		byReal := q
		byReal.byReal = true
		queries = append(queries, byReal)
	}
	return cubes, queries, q.Forward || len(cubes)*len(queries) > 1
}

// pageBound is a walk's query bounded by the newest row read before the
// walks of its page (Events), 1 at least: no row above it, whatever the
// writer commits meanwhile.
func pageBound(q EventQuery, newest int64) EventQuery {
	q.upTo = newest
	return q
}

// mergeWalks merges walks, each newest first, into one page newest first
// of at most limit rows; with forward, walks and page oldest first.
func mergeWalks(walks [][]Event, limit int, forward bool) []Event {
	n := 0
	for _, w := range walks {
		n += len(w)
	}
	merged := make([]Event, 0, min(limit, n))
	at := make([]int, len(walks))
	for len(merged) < limit {
		next := -1
		for k, w := range walks {
			if at[k] < len(w) && (next < 0 || (w[at[k]].ID > walks[next][at[next]].ID) != forward) {
				next = k
			}
		}
		if next < 0 {
			break
		}
		merged, at[next] = append(merged, walks[next][at[next]]), at[next]+1
	}
	return merged
}

// eventsWalk reads one walk of a page: the rows of cube, the server's
// for 0, newest first, or oldest first with q.Forward.
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
		var cubeID, userID, realID, commandID sql.NullInt64
		var at sql.NullString
		var data string
		if err := rows.Scan(&e.ID, &cubeID, &at, &e.Kind, &e.Source, &e.Level, &e.Device, &e.Serial, &userID, &realID, &data, &commandID, &e.Visibility); err != nil {
			return nil, fmt.Errorf("store: events: %w", err)
		}
		e.CubeID, e.EffectiveUserID, e.RealUserID, e.CommandID = cubeID.Int64, userID.Int64, realID.Int64, commandID.Int64
		e.Time, e.Data = parseTime(at), json.RawMessage(data)
		out = append(out, e)
	}
	return out, rows.Err()
}

// eventsQuery is the SELECT of one walk of a page, the rows of cube (the
// server's for 0) with only the terms q gives, every value bound, so
// that SQLite walks one index in id order from the end, or with
// q.Forward from the start, instead of sorting all of the Cube's rows:
// each index carries the rowid after its columns, so an id bound, after
// or before, is a range of the same walk. A time bound becomes an id
// bound first, the newest id before that time or the oldest at or after
// it, and stays a filter without its index (+time), which keeps the id
// order; the level is such a filter always, a range having no index.
// A page at warn or above adds the literal level >= 4, the WHERE of the
// partial index of the warnings and errors (migration 0011): SQLite
// takes a partial index only for terms that imply its WHERE, and cannot
// tell that of a bound value. The page then reads those rows alone
// instead of every row of the Cube.
func eventsQuery(q EventQuery, cube int64, limit int) (string, []any) {
	where, args := []string{cubeTerm(cube)}, cubeArgs(cube)
	if q.AfterID > 0 {
		where, args = append(where, "id > ?"), append(args, q.AfterID)
	}
	if q.BeforeID > 0 {
		where, args = append(where, "id < ?"), append(args, q.BeforeID)
	}
	if q.upTo > 0 {
		// A filter (+id), not a range of the walk's index: SQLite would
		// take the range for selective and walk the Cube's rows by
		// log_cube in place of a kind's or a source's index. Few rows lie
		// above it, those committed since the page began.
		where, args = append(where, "+id <= ?"), append(args, q.upTo)
	}
	if !q.Before.IsZero() {
		at := formatTime(q.Before)
		where = append(where, "id <= (SELECT max(id) FROM log WHERE "+cubeTerm(cube)+" AND time < ?)", "+time < ?")
		args = append(args, cubeArgs(cube, at, at)...)
	}
	if !q.Since.IsZero() {
		at := formatTime(q.Since)
		where = append(where, "id >= (SELECT min(id) FROM log WHERE "+cubeTerm(cube)+" AND time >= ?)", "+time >= ?")
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
		if q.byReal {
			// The rows the user made acting as another: the first walk
			// has those made with its rights, its own included.
			where, args = append(where, "real_user_id = ?", "+effective_user_id <> ?"), append(args, q.UserID, q.UserID)
		} else {
			where, args = append(where, "effective_user_id = ?"), append(args, q.UserID)
		}
	}
	if q.CommandID != 0 {
		where, args = append(where, "command_id = ?"), append(args, q.CommandID)
	}
	if !q.Audit {
		// A filter, as the level: few rows are the audit's.
		where = append(where, "+visibility = '"+VisibilityRead+"'")
	}
	if q.MinLevel != nil {
		if *q.MinLevel >= LevelWarn {
			where = append(where, warnTerm)
		}
		where, args = append(where, "+level >= ?"), append(args, *q.MinLevel)
	}
	order := "DESC"
	if q.Forward {
		order = "ASC"
	}
	return `SELECT id, cube_id, time, kind, source, level, device, serial, effective_user_id, real_user_id, data, command_id, visibility FROM log WHERE ` + strings.Join(where, " AND ") +
		` ORDER BY id ` + order + ` LIMIT ?`, append(args, limit)
}

// NewestEventID is the id of the log's newest row, of any Cube, the
// server's or the audit's, 0 for an empty log: the highest id a page can
// have given out, against which a page after a cursor finds one the log
// never gave out, and at which the walks of a page stop (Events). The
// rows the writer holds are not yet counted (FlushEvents).
func (s *Store) NewestEventID(ctx context.Context) (int64, error) {
	var id int64
	if err := s.db.QueryRowContext(ctx, `SELECT coalesce(max(id), 0) FROM log`).Scan(&id); err != nil {
		return 0, fmt.Errorf("store: newest event: %w", err)
	}
	return id, nil
}
