// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

// The log (docs/store.md section 9 (d)): the events a Cube's stream
// sends, kept so that GET …/log can page back through them. A Cube with
// a row in the store keeps them there (remember hands them to the
// store's writer), which survives a restart and keeps log.keep_days of
// them; a Cube without one, a Cube of serve -addr or a server without a
// store, keeps the last logSize in the runtime's ring, as before the
// store. The stored copy leaves out a device's received times, which the
// tiles' ages read and the log does not.

import (
	"context"
	"encoding/json"
	"net/http"
	"slices"
	"strconv"
	"strings"
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
	"device_forgotten", "cube_found", "restore_step", "restore_done",
	"cube_gone", "address_changed", "server_started", "server_stopping", "settings_read", "restart", "reboot", "reboot_cancelled",
	"update", "update_result", "backup", "backup_deleted", "backups_pruned", "log_pruned", "restore_started",
}

// Limits of a page of the log.
const (
	defaultLogPage = 200
	maxLogPage     = store.MaxEventsPage
)

// storedEvent is an event as the store keeps it: compact, without the
// device view's received times.
func storedEvent(cubeID int64, ev event) store.Event {
	if dv, ok := ev.State.(view.Device); ok && dv.Received != nil {
		dv.Received = nil
		ev.State = dv
	}
	ev.settle()
	level, _ := store.ParseLevel(ev.Level)
	userID := ev.userID
	ev.By = nil // the row keeps the user's id, the page names them
	data, _ := json.Marshal(ev)
	return store.Event{CubeID: cubeID, Time: ev.Time, Kind: ev.Kind, Source: ev.Source, Level: level, Device: ev.Device, Serial: ev.Serial, UserID: userID, Data: data}
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
var logSources = []string{"device", "cube", "system", "config"}

// logQuery is a request's page of the log.
type logQuery struct {
	beforeID int64
	before   time.Time
	since    time.Time
	kind     string
	source   string
	device   string // a radio address, for a device whose serial the Cube does not say
	serial   string // a serial number
	user     string // a login name
	userID   int64  // its row, once resolved
	minLevel int
	limit    int
}

// parseLogQuery reads before (an event's id, or an RFC 3339 time), since
// (an RFC 3339 time, not after a before time), limit (1 to 1000, 200 by
// default), kind, source (device, cube, system or config), level (debug,
// info, warn or error: that level and above, info by default), device
// (a serial number or a radio address) and user (a login name).
func parseLogQuery(r *http.Request) (logQuery, error) {
	q := r.URL.Query()
	lq := logQuery{limit: defaultLogPage, kind: q.Get("kind"), minLevel: store.LevelInfo, user: q.Get("user")}
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
// the server's, before the cursor and since a time, of a kind, a source,
// a device (by its serial number or its radio address) and a user, at a
// level and above, at most limit, oldest first, the same objects the
// event stream sends; from the store, with each event's id, or from the
// ring. A user the store does not know has none.
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
	if lq.user != "" {
		id, ok := int64(0), false
		if st := s.opts.Store; st != nil {
			if id, ok, err = st.UserIDByName(r.Context(), lq.user); err != nil {
				return fail(http.StatusInternalServerError, "%v", err)
			}
		}
		if !ok {
			writeJSON(w, http.StatusOK, []event{})
			return nil
		}
		lq.userID = id
	}
	if id := rt.storeID.Load(); id != 0 && rt.store != nil {
		evs, err := s.storedLog(r.Context(), rt.store, id, lq)
		if err != nil {
			return err
		}
		writeJSON(w, http.StatusOK, evs)
		return nil
	}
	writeJSON(w, http.StatusOK, filterEvents(rt.recent(0), lq))
	return nil
}

// storedLog reads a page from the store, the Cube's rows and the
// server's, after the writer wrote what is pending, and turns it round,
// oldest first; who started an event is named as the users table names
// them now.
func (s *Server) storedLog(ctx context.Context, st *store.Store, cubeID int64, lq logQuery) ([]event, error) {
	if err := st.FlushEvents(ctx); err != nil {
		s.log.Warn("api: the log's pending events not written", "error", err)
	}
	minLevel := lq.minLevel
	rows, err := st.Events(ctx, store.EventQuery{CubeID: cubeID, Server: true, BeforeID: lq.beforeID, Before: lq.before, Since: lq.since,
		Kind: lq.kind, Source: lq.source, Device: lq.device, Serial: lq.serial, UserID: lq.userID, MinLevel: &minLevel, Limit: lq.limit})
	if err != nil {
		return nil, fail(http.StatusInternalServerError, "%v", err)
	}
	users := map[int64]*eventUser{}
	out := make([]event, 0, len(rows))
	for i := len(rows) - 1; i >= 0; i-- {
		var ev event
		if err := json.Unmarshal(rows[i].Data, &ev); err != nil {
			s.log.Warn("api: a stored event not read", "id", rows[i].ID, "error", err)
			continue
		}
		ev.ID, ev.Source, ev.Level, ev.By = rows[i].ID, rows[i].Source, store.LevelName(rows[i].Level), nil
		if ev.Serial == "" {
			ev.Serial = rows[i].Serial
		}
		if id := rows[i].UserID; id != 0 {
			by, seen := users[id]
			if !seen {
				named := event{userID: id}
				s.nameUser(ctx, &named)
				by, users[id] = named.By, named.By
			}
			ev.By = by
		}
		out = append(out, ev)
	}
	return out, nil
}

// filterEvents is a page of the ring, oldest first: the events before the
// time and not before since, of the kind, the source, the device and the
// user, at the level and above, the last limit of them. The ring's
// events have no id, so none lies before one.
func filterEvents(all []event, lq logQuery) []event {
	out := []event{}
	if lq.beforeID != 0 {
		return out
	}
	for _, ev := range all {
		level, _ := store.ParseLevel(ev.Level)
		if !lq.before.IsZero() && !ev.Time.Before(lq.before) || !lq.since.IsZero() && ev.Time.Before(lq.since) ||
			lq.kind != "" && ev.Kind != lq.kind || lq.source != "" && ev.Source != lq.source ||
			lq.serial != "" && ev.Serial != lq.serial || lq.device != "" && ev.Device != lq.device ||
			lq.userID != 0 && ev.userID != lq.userID || level < lq.minLevel {
			continue
		}
		out = append(out, ev)
	}
	if len(out) > lq.limit {
		out = out[len(out)-lq.limit:]
	}
	return out
}
