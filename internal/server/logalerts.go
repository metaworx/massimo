// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

// The log's unsettled problems (AP LogAlerts v1.0): GET /api/v1/log/alerts
// answers the warnings and errors of every Cube's log and the server's
// that nothing has settled yet, so that the web UI's log button can show
// the worst one its person has not marked seen. A warning or an error is
// settled by its counterpart, a later event that says the condition is
// over, of the same Cube, or of the server for the server's own lines: a
// disconnect by the next connect, also one the link held. settleRules
// lists them; a kind without a counterpart is settled by a mark alone,
// and a hold and a lockout settle by themselves once they end. A device's
// transmit error, link error and low battery are in the log at info, in
// its state's changes: each is an alert at warn while the device has the
// flag, at the line that set it.

import (
	"context"
	"fmt"
	"maps"
	"net/http"
	"slices"
	"strconv"
	"time"

	"go.metaworx.ch/massimo/internal/store"
)

// How far back the alerts look.
const (
	alertScan  = 1000 // the newest warnings and errors read
	settleScan = 1000 // the newest events of a settling kind read
	flagScan   = 200  // a device's newest states searched for the line that set a flag
)

// settleRule says how a warning or an error of a kind is settled.
type settleRule struct {
	// by are the kinds that settle it: a later event of one of them, of
	// the same Cube (or the server's, for the server's own), below warn.
	by []string
	// match, where set, says whether such a later event settles this
	// one: the same command, the same user.
	match func(alert, later *event) bool
	// ends, where set, is when it settles by itself.
	ends func(alert *event) time.Time
}

// settleRules are the kinds the server writes at warn or error, with
// what settles each; one missing settles by a mark alone, as do
// proxy_discarded and proxy_delete_refused, which say what was refused
// once. TestSettleRulesKnowTheirKinds holds them to eventKinds.
var settleRules = map[string]settleRule{
	// the Cube's connection lost, or held while the link was down
	"disconnected": {by: []string{"connected"}},
	// a command retried, failed or dropped: the same command key
	// acknowledged for the same device later
	"command": {by: []string{"command"}, match: sameCommand},
	// the radio held for the duty cycle or the memory slots, till then
	"hold": {ends: holdEnds},
	// the Cube's memory slots full for an hour: the reading that has
	// them free again (slots.go)
	"slots": {by: []string{"slots"}},
	// an automatic backup failed: the next one taken
	"backup": {by: []string{"backup"}},
	// a restore failed: the next one done
	"restore_done": {by: []string{"restore_done"}},
	// the proxy refused a client while the Cube was not yet connected
	"proxy_refused": {by: []string{"connected", "proxy_connected"}},
	// the proxy saw the original software's pairing go wrong
	"proxy_warning":        {by: []string{"pairing_ended", "pairing_started", "proxy_pairing"}, match: pairingSettles},
	"proxy_discarded":      {},
	"proxy_delete_refused": {},
	// the server's own: an update failed or rolled back, the log's file
	// not rotated, a failed sign-in, an address locked out, a restart, a
	// reboot or an update not started
	"update_result": {by: []string{"update_result"}},
	"log_rotated":   {by: []string{"log_rotated"}},
	"login_failed":  {by: []string{"login"}, match: sameUser},
	"locked_out":    {ends: lockoutEnds},
	"restart":       {by: []string{"server_started"}},
	"reboot":        {by: []string{"server_started"}},
	"update":        {by: []string{"update", "server_started"}},
}

// deviceFlags are a device's state's flags that are alerts while set.
var deviceFlags = []string{"error", "link_error", "battery_low"}

// summaryText is a summary's value as text, "" for none.
func summaryText(ev *event, key string) string {
	v, ok := ev.Summary[key]
	if !ok || v == nil {
		return ""
	}
	return fmt.Sprint(v)
}

// sameCommand: a later command acknowledged for the same device and key.
func sameCommand(alert, later *event) bool {
	return later.Device == alert.Device && summaryText(later, "key") == summaryText(alert, "key") && summaryText(later, "state") == "acknowledged"
}

// pairingSettles: a pairing not stopped settles at its end, one not
// started at the next start.
func pairingSettles(alert, later *event) bool {
	switch summaryText(alert, "what") {
	case "pairing_not_stopped":
		return later.Kind == "pairing_ended"
	case "pairing_not_started":
		return later.Kind == "pairing_started" || later.Kind == "proxy_pairing"
	}
	return false
}

// sameUser: a later sign-in of the same login name.
func sameUser(alert, later *event) bool {
	return summaryText(later, "user") == summaryText(alert, "user")
}

// holdEnds is when a hold ends: the duty cycle's reset, or the wait for
// the memory slots.
func holdEnds(ev *event) time.Time {
	if t, err := time.Parse(time.RFC3339, summaryText(ev, "until")); err == nil {
		return t
	}
	if w, err := strconv.ParseFloat(summaryText(ev, "wait_s"), 64); err == nil {
		return ev.Time.Add(time.Duration(w * float64(time.Second)))
	}
	return ev.Time
}

// lockoutEnds is when an address's lockout ends.
func lockoutEnds(ev *event) time.Time {
	if d, err := time.ParseDuration(summaryText(ev, "for")); err == nil {
		return ev.Time.Add(d)
	}
	return ev.Time
}

// logAlert is an unsettled warning or error of the log: its line, and
// for a device's flag, which.
type logAlert struct {
	ID         int64     `json:"id"`
	Kind       string    `json:"kind"`
	Level      string    `json:"level"`
	CubeSerial string    `json:"cube_serial,omitempty"`
	Device     string    `json:"device,omitempty"`
	Serial     string    `json:"serial,omitempty"`
	Flag       string    `json:"flag,omitempty"`
	Time       time.Time `json:"time"`
}

// logAlertsView is GET /api/v1/log/alerts' answer.
type logAlertsView struct {
	// Level is the worst of the alerts: error, warn, or "" for none.
	Level  string     `json:"level"`
	Alerts []logAlert `json:"alerts"` // newest first
}

// alertOf is the alert of a line.
func alertOf(ev *event, level, flag string) logAlert {
	return logAlert{ID: ev.ID, Kind: ev.Kind, Level: level, CubeSerial: ev.CubeSerial, Device: ev.Device, Serial: ev.Serial, Flag: flag, Time: ev.Time}
}

// belowWarn reports whether an event is below warn, as a counterpart is.
func belowWarn(ev *event) bool {
	l, _ := store.ParseLevel(ev.Level)
	return l < store.LevelWarn
}

// unsettled are the warnings and errors of lines (oldest first) that no
// later event of settling settles, or that have not ended by now.
func unsettled(lines, settling []event, now time.Time) []logAlert {
	var out []logAlert
	for i := range lines {
		a := &lines[i]
		rule := settleRules[a.Kind]
		if rule.ends != nil && !rule.ends(a).After(now) {
			continue
		}
		settled := false
		for j := range settling {
			l := &settling[j]
			if l.ID > a.ID && l.CubeSerial == a.CubeSerial && belowWarn(l) && slices.Contains(rule.by, l.Kind) && (rule.match == nil || rule.match(a, l)) {
				settled = true
				break
			}
		}
		if !settled {
			out = append(out, alertOf(a, a.Level, ""))
		}
	}
	return out
}

// logAlerts are the log's unsettled warnings and errors, of every Cube
// with a row in the store and of the server, audit's lines for an
// auditor only, and the devices' flags set now, newest first. A Cube
// without a row keeps its lines in its ring, which has no ids to mark.
func (s *Server) logAlerts(ctx context.Context, st *store.Store, audit bool) ([]logAlert, error) {
	cubes := map[int64]string{}
	rts := map[int64]*runtime{}
	for _, rt := range s.runtimes() {
		if id := rt.storeID.Load(); id != 0 {
			cubes[id], rts[id] = rt.cubeSerial(), rt
		}
	}
	lines, err := s.storedLog(ctx, st, cubes, logQuery{minLevel: store.LevelWarn, audit: audit, limit: alertScan})
	if err != nil {
		return nil, err
	}
	var settling []event
	if len(lines) > 0 {
		kinds := map[string]bool{}
		for _, ev := range lines {
			for _, k := range settleRules[ev.Kind].by {
				kinds[k] = true
			}
		}
		for _, k := range slices.Sorted(maps.Keys(kinds)) {
			later, err := s.storedLog(ctx, st, cubes, logQuery{kind: k, since: lines[0].Time, minLevel: store.LevelDebug, audit: audit, limit: settleScan})
			if err != nil {
				return nil, err
			}
			settling = append(settling, later...)
		}
	}
	out := unsettled(lines, settling, time.Now())
	for _, id := range slices.Sorted(maps.Keys(rts)) {
		flags, err := s.flagAlerts(ctx, st, id, rts[id])
		if err != nil {
			return nil, err
		}
		out = append(out, flags...)
	}
	slices.SortFunc(out, func(a, b logAlert) int { return int(b.ID - a.ID) })
	return out, nil
}

// flagAlerts are a Cube's devices' flags set now, each at the line that
// set it: the newest state that turned it on, else the oldest state of
// the device's last ones that has it, where it was set before them.
func (s *Server) flagAlerts(ctx context.Context, st *store.Store, id int64, rt *runtime) ([]logAlert, error) {
	var out []logAlert
	for _, p := range s.problemsOf(rt) {
		set := map[string]bool{"error": p.State.Error, "link_error": p.State.LinkError, "battery_low": p.State.BatteryLow}
		var states []event
		for _, flag := range deviceFlags {
			if !set[flag] {
				continue
			}
			if states == nil {
				var err error
				states, err = s.storedLog(ctx, st, map[int64]string{id: rt.cubeSerial()}, logQuery{kind: "state", device: p.RFAddress, minLevel: store.LevelDebug, limit: flagScan})
				if err != nil {
					return nil, err
				}
			}
			if line := flagLine(states, flag); line != nil {
				out = append(out, alertOf(line, "warn", flag))
			}
		}
	}
	return out, nil
}

// flagLine is the state line (states oldest first) that set a flag, nil
// where none says it.
func flagLine(states []event, flag string) *event {
	var oldest *event
	for i := len(states) - 1; i >= 0; i-- {
		ev := &states[i]
		if ch, ok := ev.Changes[flag]; ok && ch.To == true {
			return ev
		}
		if st, _ := ev.State.(map[string]any); st != nil {
			if fl, _ := st["state"].(map[string]any); fl != nil && fl[flag] == true {
				oldest = ev
			}
		}
	}
	return oldest
}

// getLogAlerts handles GET /api/v1/log/alerts: the log's unsettled
// warnings and errors after an event's id (after) that the signed-in
// person has not marked seen (logseen.go), newest first, and the worst
// level; none without a store.
func (s *Server) getLogAlerts(w http.ResponseWriter, r *http.Request) error {
	var after int64
	if v := r.URL.Query().Get("after"); v != "" {
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil || n < 0 {
			return fail(http.StatusBadRequest, "bad after %q: an event's id", v)
		}
		after = n
	}
	out := logAlertsView{Alerts: []logAlert{}}
	if st := s.opts.Store; st != nil {
		list, err := s.logAlerts(r.Context(), st, principalOf(r).has("audit"))
		if err != nil {
			return err
		}
		seen := s.seenOf(r)
		for _, a := range list {
			if a.ID > after && !seen.seen(a.ID) {
				out.Alerts = append(out.Alerts, a)
			}
		}
	}
	for _, a := range out.Alerts {
		if a.Level == "error" || out.Level == "" {
			out.Level = a.Level
		}
	}
	writeJSON(w, http.StatusOK, out)
	return nil
}
