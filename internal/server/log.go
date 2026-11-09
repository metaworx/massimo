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
	data, _ := json.Marshal(ev)
	return store.Event{CubeID: cubeID, Time: ev.Time, Kind: ev.Kind, Device: ev.Device, Data: data}
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

// logQuery is a request's page of the log.
type logQuery struct {
	beforeID int64
	before   time.Time
	since    time.Time
	kind     string
	device   string // a radio address
	serial   string // a serial number, until resolveSerial makes it the device
	limit    int
}

// parseLogQuery reads before (an event's id, or an RFC 3339 time), since
// (an RFC 3339 time, not after a before time), limit (1 to 1000, 200 by
// default), kind and device (a serial number or a radio address).
func parseLogQuery(r *http.Request) (logQuery, error) {
	q := r.URL.Query()
	lq := logQuery{limit: defaultLogPage, kind: q.Get("kind")}
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

// resolveSerial turns a page's serial into the radio address the log
// keeps, through the Cube's snapshot; false for a serial the Cube does
// not hold, whose page is empty.
func (lq *logQuery) resolveSerial(rt *runtime) bool {
	if lq.serial == "" {
		return true
	}
	rf, ok := rt.client.Snapshot().BySerial(lq.serial)
	if !ok {
		return false
	}
	lq.device = rf.String()
	return true
}

// getLog handles GET /cubes/{sn}/log: the page of the Cube's events
// before the cursor and since a time, of a kind and a device where asked
// (by its serial number or its radio address; a serial the Cube does not
// hold has none), at most limit,
// oldest first, the same objects the event stream sends; from the store,
// with each event's id, or from the ring.
func (s *Server) getLog(w http.ResponseWriter, r *http.Request) error {
	rt, err := s.cube(r)
	if err != nil {
		return err
	}
	lq, err := parseLogQuery(r)
	if err != nil {
		return err
	}
	if !lq.resolveSerial(rt) {
		writeJSON(w, http.StatusOK, []event{})
		return nil
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

// storedLog reads a page from the store, after the writer wrote what is
// pending, and turns it round, oldest first.
func (s *Server) storedLog(ctx context.Context, st *store.Store, cubeID int64, lq logQuery) ([]event, error) {
	if err := st.FlushEvents(ctx); err != nil {
		s.log.Warn("api: the log's pending events not written", "error", err)
	}
	rows, err := st.Events(ctx, store.EventQuery{CubeID: cubeID, BeforeID: lq.beforeID, Before: lq.before, Since: lq.since, Kind: lq.kind, Device: lq.device, Limit: lq.limit})
	if err != nil {
		return nil, fail(http.StatusInternalServerError, "%v", err)
	}
	out := make([]event, 0, len(rows))
	for i := len(rows) - 1; i >= 0; i-- {
		var ev event
		if err := json.Unmarshal(rows[i].Data, &ev); err != nil {
			s.log.Warn("api: a stored event not read", "id", rows[i].ID, "error", err)
			continue
		}
		ev.ID = rows[i].ID
		out = append(out, ev)
	}
	return out, nil
}

// filterEvents is a page of the ring, oldest first: the events before the
// time and not before since, of the kind and the device, the last limit
// of them. The ring's
// events have no id, so none lies before one.
func filterEvents(all []event, lq logQuery) []event {
	out := []event{}
	if lq.beforeID != 0 {
		return out
	}
	for _, ev := range all {
		if !lq.before.IsZero() && !ev.Time.Before(lq.before) || !lq.since.IsZero() && ev.Time.Before(lq.since) ||
			lq.kind != "" && ev.Kind != lq.kind || lq.device != "" && ev.Device != lq.device {
			continue
		}
		out = append(out, ev)
	}
	if len(out) > lq.limit {
		out = out[len(out)-lq.limit:]
	}
	return out
}
