// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

// The log (docs/store.md section 9 (d)): the events a Cube's stream
// sends, kept so that GET …/log can page back through them, and forward
// from a cursor (after=, AP LogCursor). A Cube with a row in the store
// keeps them there (remember hands them to the store's writer), which
// survives a restart and keeps log.keep_days of them; a Cube without
// one, a Cube of serve -addr or a server without a store, keeps the last
// logSize in the runtime's ring, as before the store, each with an id of
// the server's ring counter (logRing). The stored copy leaves out a
// device's received times, which the tiles' ages read and the log does
// not, and its mismatches, which the log's own mismatch events say.

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"maps"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.metaworx.ch/massimo/internal/protocol/cube"
	"go.metaworx.ch/massimo/internal/store"
	"go.metaworx.ch/massimo/internal/view"
)

// eventKinds are the kinds of event the stream sends and the log keeps,
// in this order: the Cube driver's (cubelan.EventKind), then the
// server's own. The web UI's kind choice (log.js LOG_KINDS) lists the
// same; TestLogKinds holds the three together.
var eventKinds = []string{
	"connected", "disconnected", "hello", "metadata", "config", "state", "pairing_started", "paired", "pairing_ended",
	"device_forgotten", "cube_found", "restore_step", "restore_done", "disagree", "drift",
	"cube_gone", "address_changed", "server_started", "server_stopping", "settings_read", "restart", "reboot", "reboot_cancelled",
	"update", "update_result", "update_postponed", "backup", "backup_deleted", "backups_pruned", "log_pruned", "log_rotated", "restore_started", "open_setup",
	"server_backup", "change", "command", "hold", "structure", "unconfirmed", "error_reset", "error_watch", "slots", "mismatch", "login", "login_failed", "locked_out", "audit_pruned",
	"proxy_connected", "proxy_disconnected", "proxy_refused", "proxy_discarded", "proxy_deleted", "proxy_delete_refused", "proxy_pairing", "proxy_warning",
}

// Limits of a page of the log.
const (
	defaultLogPage = 200
	maxLogPage     = store.MaxEventsPage
)

// storedEvent is an event as the store keeps it: compact, without the
// device view's received times.
func storedEvent(cubeID int64, ev event) store.Event {
	if dv, ok := ev.State.(view.Device); ok && (dv.Received != nil || dv.Mismatches != nil) {
		dv.Received, dv.Mismatches = nil, nil // the mismatch events say them (congruence.go)
		ev.State = dv
	}
	ev.settle()
	level, _ := store.ParseLevel(ev.Level)
	userID := ev.userID
	ev.By, ev.EffectiveUser, ev.RealUser = nil, nil, nil // the row keeps the users' ids, the page names them
	data, _ := json.Marshal(ev)
	vis := store.VisibilityRead
	if ev.audit {
		vis = store.VisibilityAudit
	}
	return store.Event{CubeID: cubeID, Time: ev.Time, Kind: ev.Kind, Source: ev.Source, Level: level, Device: ev.Device, Serial: ev.Serial,
		EffectiveUserID: userID, RealUserID: ev.realUserID, Data: data, CommandID: ev.Command, Visibility: vis}
}

// learnStoreID finds the row of a runtime that has none yet, at its
// connection: the plain settings-file Cube once seeded, or a Cube of this
// run that discovery has noted.
func (s *Server) learnStoreID(ctx context.Context, rt *runtime) {
	st := s.opts.Store
	if st == nil || rt.storeID.Load() != 0 || rt.cubeSerial() == "" {
		return
	}
	row, ok, err := st.Cube(ctx, rt.cubeSerial())
	if err != nil || !ok {
		return
	}
	rt.storeID.Store(row.ID)
}

// logSources are the sources an event comes from (event.settle).
var logSources = []string{"device", "cube", "system", "config", "queue", "server", "proxy"}

// logQuery is a request's page of the log.
type logQuery struct {
	// after is the cursor of a page forward, with forward: the first
	// limit events whose id is above it, oldest first; 0 is the start.
	after    int64
	forward  bool
	beforeID int64
	before   time.Time
	since    time.Time
	kind     string
	source   string
	device   string // a radio address, for a device whose serial the Cube does not say
	serial   string // a serial number
	user     string // a login name
	userID   int64  // its row, once resolved
	command  int64  // a command's id
	audit    bool   // the rows of the audit visibility too: the caller holds audit
	minLevel int
	limit    int
}

// parseLogQuery reads after (an event's id, 0 for the start: the page
// forward from it), before (an event's id, or an RFC 3339 time), since
// (an RFC 3339 time, not after a before time), limit (1 to 1000, 200 by
// default), kind, source (device, cube, system or config), level (debug,
// info, warn or error: that level and above, info by default), device
// (a serial number or a radio address), user (a login name) and command
// (a command's id).
func parseLogQuery(r *http.Request) (logQuery, error) {
	q := r.URL.Query()
	lq := logQuery{limit: defaultLogPage, kind: q.Get("kind"), minLevel: store.LevelInfo, user: q.Get("user")}
	if v := q.Get("after"); v != "" {
		id, err := strconv.ParseInt(v, 10, 64)
		if err != nil || id < 0 {
			return lq, fail(http.StatusBadRequest, "bad after %q: an event's id, or 0 for the start", v)
		}
		lq.after, lq.forward = id, true
	}
	if v := q.Get("before"); v != "" {
		if id, err := strconv.ParseInt(v, 10, 64); err == nil {
			if id < 1 {
				return lq, fail(http.StatusBadRequest, "bad before %q", v)
			}
			lq.beforeID = id
		} else if t, err := time.Parse(time.RFC3339Nano, v); err == nil {
			lq.before = t
		} else {
			return lq, fail(http.StatusBadRequest, "bad before %q: an event's id, or a time such as 2026-11-06T20:00:00Z", v)
		}
	}
	if v := q.Get("since"); v != "" {
		t, err := time.Parse(time.RFC3339Nano, v)
		if err != nil {
			return lq, fail(http.StatusBadRequest, "bad since %q: a time such as 2026-11-06T20:00:00Z", v)
		}
		if !lq.before.IsZero() && t.After(lq.before) {
			return lq, fail(http.StatusBadRequest, "since %s is after before %s", v, q.Get("before"))
		}
		lq.since = t
	}
	if v := q.Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > maxLogPage {
			return lq, fail(http.StatusBadRequest, "bad limit %q: 1 to %d", v, maxLogPage)
		}
		lq.limit = n
	}
	if v := q.Get("source"); v != "" {
		if !slices.Contains(logSources, v) {
			return lq, fail(http.StatusBadRequest, "bad source %q: one of %s", v, strings.Join(logSources, ", "))
		}
		lq.source = v
	}
	if v := q.Get("level"); v != "" {
		l, ok := store.ParseLevel(v)
		if !ok {
			return lq, fail(http.StatusBadRequest, "bad level %q: debug, info, warn or error", v)
		}
		lq.minLevel = l
	}
	if v := q.Get("command"); v != "" {
		id, err := strconv.ParseInt(v, 10, 64)
		if err != nil || id < 1 {
			return lq, fail(http.StatusBadRequest, "bad command %q: a command's id", v)
		}
		lq.command = id
	}
	if v := q.Get("device"); v != "" {
		if deviceSerial.MatchString(v) {
			lq.serial = strings.ToUpper(v)
		} else {
			rf, err := cube.ParseRFAddress(v)
			if err != nil {
				return lq, fail(http.StatusBadRequest, "bad device %q: a radio address of six hex digits, or a serial number", v)
			}
			lq.device = rf.String()
		}
	}
	return lq, nil
}

// resolveDevice names a page's device by its serial where the Cube's
// snapshot gives the serial of a radio address asked for, since the log
// finds a device by its serial.
func (lq *logQuery) resolveDevice(rt *runtime) {
	if lq.device == "" {
		return
	}
	if rf, err := cube.ParseRFAddress(lq.device); err == nil {
		if serial := rt.client.Snapshot().Serial(rf); serial != "" {
			lq.serial, lq.device = serial, ""
		}
	}
}

// getLog handles GET /cubes/{sn}/log: the page of the Cube's events and
// the server's, after a cursor and before one, since a time, of a kind,
// a source, a device (by its serial number or its radio address), a user
// and a command, at a level and above, at most limit, oldest first: the
// first limit after the cursor, or else the last; the same objects the
// event stream sends, each with its id; from the store, or from the
// ring. A user the store does not know has none. The rows of the audit
// visibility (sign-ins, lockouts) are only for a caller with audit. A
// cursor beyond the log's newest id is 409 (afterBeyond).
func (s *Server) getLog(w http.ResponseWriter, r *http.Request) error {
	rt, err := s.cube(r)
	if err != nil {
		return err
	}
	lq, err := parseLogQuery(r)
	if err != nil {
		return err
	}
	lq.resolveDevice(rt)
	lq.audit = principalOf(r).has("audit")
	if none, err := s.logUser(r.Context(), &lq); err != nil || none {
		if none {
			writeJSON(w, http.StatusOK, []event{})
		}
		return err
	}
	if id := rt.storeID.Load(); id != 0 && rt.store != nil {
		evs, err := s.storedLog(r.Context(), rt.store, map[int64]string{id: ""}, lq)
		if err != nil {
			return err
		}
		writeJSON(w, http.StatusOK, evs)
		return nil
	}
	rings, newest := s.ringRead(rt)
	if err := lq.beyond(newest); err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, filterEvents(rings[0], lq))
	return nil
}

// getAllLog handles GET /log: the page of every Cube's events and the
// server's, as GET /cubes/{sn}/log reads one Cube's, each Cube's event
// with the Cube's S/N (cube_serial); a radio address names the device of
// that address on every Cube. A Cube without a row in the store yet
// (before its first connection) adds nothing. Without a store, the
// Cubes' rings merged by id, the server's events, which every ring
// holds, once.
func (s *Server) getAllLog(w http.ResponseWriter, r *http.Request) error {
	lq, err := parseLogQuery(r)
	if err != nil {
		return err
	}
	lq.audit = principalOf(r).has("audit")
	if none, err := s.logUser(r.Context(), &lq); err != nil || none {
		if none {
			writeJSON(w, http.StatusOK, []event{})
		}
		return err
	}
	rts := s.runtimes()
	if st := s.opts.Store; st != nil {
		cubes := map[int64]string{}
		for _, rt := range rts {
			if id := rt.storeID.Load(); id != 0 {
				cubes[id] = rt.cubeSerial()
			}
		}
		evs, err := s.storedLog(r.Context(), st, cubes, lq)
		if err != nil {
			return err
		}
		writeJSON(w, http.StatusOK, evs)
		return nil
	}
	evs, err := s.ringLog(rts, lq)
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, evs)
	return nil
}

// ringLog is a page of every Cube's ring, oldest first, without a store:
// each Cube's events with its S/N, and the server's own once, from the
// first Cube's ring, since every ring keeps them; merged by id, which
// the rings took from one counter.
func (s *Server) ringLog(rts []*runtime, lq logQuery) ([]event, error) {
	rings, newest := s.ringRead(rts...)
	if err := lq.beyond(newest); err != nil {
		return nil, err
	}
	var all []event
	for i, ring := range rings {
		sn := rts[i].cubeSerial()
		for _, ev := range ring {
			if ev.server {
				if i > 0 {
					continue
				}
			} else {
				ev.CubeSerial = sn
			}
			all = append(all, ev)
		}
	}
	slices.SortFunc(all, func(a, b event) int { return cmp.Compare(a.ID, b.ID) })
	return filterEvents(all, lq), nil
}

// logRing numbers the events of the rings, the log of the Cubes without
// a row in the store (remember), from one counter for the server that
// starts at its start in microseconds: the ids grow across a restart
// without a store, unless a run gave out more than one a microsecond or
// the clock went back; they stay below 2^53, which JavaScript reads
// exactly, and far above the store's. An event takes its id and goes
// into its ring under the one lock, under which a page copies the
// rings: once a page has seen an id, every lower one is in its ring
// already, which a lock of each ring's own would not hold, so that a
// page after a cursor misses nothing.
type logRing struct {
	mu   sync.Mutex
	next int64 // the id of the next event
}

// take gives out the next id; under mu.
func (r *logRing) take() int64 {
	id := r.next
	r.next++
	return id
}

// ringRead copies the rings of rts, and the newest id given out, under
// the rings' lock (logRing).
func (s *Server) ringRead(rts ...*runtime) ([][]event, int64) {
	s.ring.mu.Lock()
	defer s.ring.mu.Unlock()
	rings := make([][]event, len(rts))
	for i, rt := range rts {
		rings[i] = slices.Clone(rt.events)
	}
	return rings, s.ring.next - 1
}

// beyond is a page of the ring's answer to a cursor beyond the newest id
// the rings have given out (afterBeyond), nil for any other.
func (lq logQuery) beyond(newest int64) error {
	if lq.forward && lq.after > newest {
		return afterBeyond(lq.after)
	}
	return nil
}

// afterBeyond is the answer to a cursor beyond the log's newest id, one
// the log never gave out: a store restored from an older copy
// (store.ErrBeyondNewest; the prune keeps the newest row, and so makes
// none); a ring whose run started with the clock set back, or the ring
// of a Cube that has since gained its row in the store. 409, after which
// the client reads the log again.
func afterBeyond(after int64) error {
	return fail(http.StatusConflict, "after %d is beyond the log's newest line: reload the log", after)
}

// logUser finds the row of a page's user, by login name; none reports a
// user the store does not know, whose page is empty.
func (s *Server) logUser(ctx context.Context, lq *logQuery) (none bool, err error) {
	if lq.user == "" {
		return false, nil
	}
	id, ok := int64(0), false
	if st := s.opts.Store; st != nil {
		if id, ok, err = st.UserIDByName(ctx, lq.user); err != nil {
			return false, fail(http.StatusInternalServerError, "%v", err)
		}
	}
	if !ok {
		return true, nil
	}
	lq.userID = id
	return false, nil
}

// storedLog reads a page from the store, the rows of the Cubes of cubes
// (each Cube's row id to the S/N its events carry as cube_serial, "" for
// none) and the server's, after the writer wrote what is pending, oldest
// first: a page back turned round, a page forward as it comes; every
// walk of the page up to the store's newest row once the pending events
// are written, against which a cursor beyond it is 409 (store.Events,
// afterBeyond). Who started an event is named as the users table names
// them now, each with its chain (names.go). Without a Cube, the server's
// rows alone.
func (s *Server) storedLog(ctx context.Context, st *store.Store, cubes map[int64]string, lq logQuery) ([]event, error) {
	if err := st.FlushEvents(ctx); err != nil {
		s.log.Warn("api: the log's pending events not written", "error", err)
	}
	minLevel := lq.minLevel
	ids := slices.Sorted(maps.Keys(cubes))
	rows, err := st.Events(ctx, store.EventQuery{CubeIDs: ids, Server: true, AfterID: lq.after, Forward: lq.forward, BeforeID: lq.beforeID,
		Before: lq.before, Since: lq.since, Kind: lq.kind, Source: lq.source, Device: lq.device, Serial: lq.serial, UserID: lq.userID,
		CommandID: lq.command, MinLevel: &minLevel, Audit: lq.audit, Limit: lq.limit})
	if errors.Is(err, store.ErrBeyondNewest) {
		return nil, afterBeyond(lq.after)
	}
	if err != nil {
		return nil, fail(http.StatusInternalServerError, "%v", err)
	}
	if !lq.forward {
		slices.Reverse(rows)
	}
	n := st.Namer()
	out := make([]event, 0, len(rows))
	for i := range rows {
		var ev event
		if err := json.Unmarshal(rows[i].Data, &ev); err != nil {
			s.log.Warn("api: a stored event not read", "id", rows[i].ID, "error", err)
			continue
		}
		ev.ID, ev.Source, ev.Level, ev.By, ev.EffectiveUser, ev.RealUser = rows[i].ID, rows[i].Source, store.LevelName(rows[i].Level), nil, nil, nil
		ev.Command = rows[i].CommandID
		if ev.Serial == "" {
			ev.Serial = rows[i].Serial
		}
		if id := rows[i].CubeID; id != 0 {
			ev.CubeSerial = cubes[id]
		}
		if id := rows[i].EffectiveUserID; id != 0 {
			ev.userID, ev.realUserID = id, rows[i].RealUserID
			nameUsers(ctx, n, &ev)
		}
		out = append(out, ev)
	}
	return out, nil
}

// filterEvents is a page of the ring, its events in id order, oldest
// first: the events after the cursor and before the id or the time and
// not before since, of the kind, the source, the device, the user and
// the command, at the level and above, the audit's only for an auditor;
// the first limit of them after a cursor, else the last limit.
func filterEvents(all []event, lq logQuery) []event {
	out := []event{}
	for _, ev := range all {
		level, _ := store.ParseLevel(ev.Level)
		if lq.forward && ev.ID <= lq.after || lq.beforeID != 0 && ev.ID >= lq.beforeID ||
			!lq.before.IsZero() && !ev.Time.Before(lq.before) || !lq.since.IsZero() && ev.Time.Before(lq.since) ||
			lq.kind != "" && ev.Kind != lq.kind || lq.source != "" && ev.Source != lq.source ||
			lq.serial != "" && ev.Serial != lq.serial || lq.device != "" && ev.Device != lq.device ||
			lq.userID != 0 && ev.userID != lq.userID && ev.realUserID != lq.userID || lq.command != 0 && ev.Command != lq.command ||
			ev.audit && !lq.audit || level < lq.minLevel {
			continue
		}
		out = append(out, ev)
		if lq.forward && len(out) == lq.limit {
			break
		}
	}
	if len(out) > lq.limit {
		out = out[len(out)-lq.limit:]
	}
	return out
}
