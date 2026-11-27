// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

// The server's settings in the store (GET and PUT /api/v1/settings): the
// stale threshold, which applies at once, and the poll, refresh and
// discovery intervals, which apply at the next start; the command-line
// flags win over them for one run; the whole numbers and the switches,
// which apply at once. node.id and tenant.id are shown and cannot be
// changed.

import (
	"cmp"
	"encoding/json"
	"errors"
	"net/http"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"go.metaworx.ch/massimo/internal/engine"
	"go.metaworx.ch/massimo/internal/store"
	"go.metaworx.ch/massimo/internal/update"
	"go.metaworx.ch/massimo/internal/view"
)

// durationSetting describes one duration setting.
type durationSetting struct {
	key      string
	def      time.Duration
	min, max time.Duration
	zero     bool // 0 is allowed: switched off
	now      bool // applies at once rather than at the next start
	inEffect func(s *Server) time.Duration
}

// The intervals of a server without a setting in the store; serve's
// flags have them as their defaults. The device list is asked for every
// 10 s, as the original software does: the Cube answers it from its
// memory, without radio time.
const (
	DefaultPollInterval     = 10 * time.Second
	DefaultRefreshInterval  = 6 * time.Hour
	DefaultDiscoverInterval = 30 * time.Minute
)

// intSetting describes one whole-number setting, which applies at once.
type intSetting struct {
	key      string
	def      int
	min, max int
}

var intSettings = []intSetting{
	// Finished commands are kept this many days; 0 keeps them (NOTE
	// StoreUsers v1.0: commands history).
	{key: store.CommandsKeepDaysKey, def: 0, min: 0, max: 3650},
	// The Cubes' events are kept this many days; 0 keeps them (log.go).
	{key: store.LogKeepDaysKey, def: store.DefaultLogKeepDays, min: 0, max: 3650},
	// The audit's rows are kept this many days; 0 keeps them (store
	// audit.go).
	{key: store.AuditKeepDaysKey, def: store.DefaultAuditKeepDays, min: 0, max: 3650},
	// The backups (backups.go): the automatic backup's quiet window and
	// its latest, in minutes, auto_min not above auto_max; the days an
	// automatic backup and the others are kept, 0 keeping them; how many
	// of a Cube's newest are kept whatever their age.
	{key: store.BackupsAutoMinKey, def: store.DefaultBackupsAutoMin, min: 1, max: 7 * 24 * 60},
	{key: store.BackupsAutoMaxKey, def: store.DefaultBackupsAutoMax, min: 1, max: 7 * 24 * 60},
	{key: store.BackupsKeepDaysAutoKey, def: store.DefaultBackupsKeepDaysAuto, min: 0, max: 3650},
	{key: store.BackupsKeepDaysManualKey, def: store.DefaultBackupsKeepDaysManual, min: 0, max: 3650},
	{key: store.BackupsKeepNewestKey, def: store.DefaultBackupsKeepNewest, min: 0, max: 1000},
}

// boolSetting describes one switch, which applies at once.
type boolSetting struct {
	key string
	def bool
}

var boolSettings = []boolSetting{
	// The password sign-in, POST /api/v1/login (login_api.go): off, the
	// users sign in by OPAQUE alone.
	{key: store.PasswordLoginKey, def: true},
	// Integrations may ask to be registered (integrations_api.go); the
	// announcement's reg says it on the LAN.
	{key: store.IntegrationRequestsKey, def: true},
	// A server without regular users kept open on purpose (AP FirstOwner,
	// D11): the web UI does not ask for the first owner. It counts only
	// while the server has no regular user, and goes with the first one
	// (D12); an integration's users leave it (AP LooseEnds, D1).
	{key: store.KeepOpenKey, def: false},
	// The web UI's password fields offer their eye (AP MobileApp, D84-A);
	// GET /healthz says it, for the sign-in before anyone is in.
	{key: store.PasswordRevealKey, def: true},
}

var durationSettings = []durationSetting{
	{key: store.StaleAfterKey, def: view.StaleAfter, min: time.Hour, max: 30 * 24 * time.Hour, now: true,
		inEffect: func(s *Server) time.Duration { return s.StaleAfter() }},
	{key: store.StaleAfterWallKey, def: view.StaleAfterWall, min: time.Hour, max: 30 * 24 * time.Hour, now: true,
		inEffect: func(s *Server) time.Duration { return s.StaleAfterWall() }},
	{key: store.PollIntervalKey, def: DefaultPollInterval, min: 10 * time.Second, max: time.Hour,
		inEffect: func(s *Server) time.Duration { return s.opts.PollInterval }},
	{key: store.RefreshIntervalKey, def: DefaultRefreshInterval, min: 10 * time.Minute, max: 7 * 24 * time.Hour, zero: true,
		inEffect: func(s *Server) time.Duration { return s.opts.RefreshInterval }},
	{key: store.DiscoverIntervalKey, def: DefaultDiscoverInterval, min: time.Minute, max: 24 * time.Hour, zero: true,
		inEffect: func(s *Server) time.Duration { return s.opts.DiscoverInterval }},
	// How long radio commands wait at a full duty cycle before the Cube's
	// budget is read again (AP LegacyLogs, D143-A).
	{key: store.DutyWaitKey, def: engine.DefaultDutyWait, min: time.Minute, max: time.Hour,
		inEffect: func(s *Server) time.Duration {
			if s.opts.Queue.DutyWait > 0 {
				return s.opts.Queue.DutyWait
			}
			return engine.DefaultDutyWait
		}},
	// How long a window may stay open before the web UI shows it amber,
	// then red; the second not under the first.
	{key: store.WindowWarnKey, def: view.WindowWarnAfter, min: time.Minute, max: 24 * time.Hour, now: true,
		inEffect: func(s *Server) time.Duration { w, _ := s.windowLimits(); return w }},
	{key: store.WindowAlertKey, def: view.WindowAlertAfter, min: time.Minute, max: 24 * time.Hour, now: true,
		inEffect: func(s *Server) time.Duration { _, a := s.windowLimits(); return a }},
}

// AnnounceState is a run's announcement on the LAN: whether the server
// announces itself, what decided it where the setting announce did not
// (-announce or MASSIMO_ANNOUNCE), and whether it listens on a loopback
// address, which the setting's auto keeps off.
type AnnounceState struct {
	On       bool
	FixedBy  string
	Loopback bool
}

// announceValues are the setting announce's values.
var announceValues = []string{store.AnnounceAuto, store.AnnounceOn, store.AnnounceOff}

// settingView is one setting as GET /api/v1/settings shows it.
type settingView struct {
	Value    string `json:"value"`               // the store's value, or the default
	Default  string `json:"default,omitempty"`   // "" for the read-only ones
	Set      bool   `json:"set"`                 // the store holds a value
	InEffect string `json:"in_effect,omitempty"` // what this run uses
	// Applies is "now" or "restart"; "" for the read-only ones.
	Applies  string `json:"applies,omitempty"`
	ReadOnly bool   `json:"read_only,omitempty"`
	// FixedBy names what decides this run's value instead of the setting:
	// a flag or a variable of the env file (announce), a flag (-poll and
	// the other durations; Options.SettingsFixedBy).
	FixedBy string `json:"fixed_by,omitempty"`
}

// fmtDuration writes a duration as Go does, without the zero units at
// the end: 24h, 30m, 1m30s, 0.
func fmtDuration(d time.Duration) string {
	if d == 0 {
		return "0"
	}
	s := d.String()
	if strings.HasSuffix(s, "m0s") {
		s = strings.TrimSuffix(s, "0s")
	}
	if strings.HasSuffix(s, "h0m") {
		s = strings.TrimSuffix(s, "0m")
	}
	return s
}

// settingsView builds the answer of GET /api/v1/settings.
func (s *Server) settingsView(r *http.Request) (map[string]settingView, error) {
	st, err := s.cubeStore()
	if err != nil {
		return nil, err
	}
	out := map[string]settingView{}
	for _, d := range durationSettings {
		v, ok, err := st.SettingDuration(r.Context(), d.key)
		if err != nil {
			return nil, fail(http.StatusInternalServerError, "%v", err)
		}
		if !ok {
			v = d.def
		}
		sv := settingView{Value: fmtDuration(v), Default: fmtDuration(d.def), Set: ok, InEffect: fmtDuration(d.inEffect(s)), Applies: "restart",
			FixedBy: s.opts.SettingsFixedBy[d.key]}
		if d.now {
			sv.Applies = "now"
		}
		out[d.key] = sv
	}
	for _, d := range intSettings {
		v, ok, err := st.SettingInt(r.Context(), d.key)
		if err != nil {
			return nil, fail(http.StatusInternalServerError, "%v", err)
		}
		if !ok {
			v = d.def
		}
		out[d.key] = settingView{Value: strconv.Itoa(v), Default: strconv.Itoa(d.def), Set: ok, InEffect: strconv.Itoa(v), Applies: "now"}
	}
	for _, d := range boolSettings {
		v, ok, err := st.SettingBool(r.Context(), d.key)
		if err != nil {
			return nil, fail(http.StatusInternalServerError, "%v", err)
		}
		if !ok {
			v = d.def
		}
		inEffect := v
		if d.key == store.KeepOpenKey && v && s.authMode(r.Context()) != modeOpen {
			inEffect = false // it counts only while the server has no regular users
		}
		out[d.key] = settingView{Value: strconv.FormatBool(v), Default: strconv.FormatBool(d.def), Set: ok, InEffect: strconv.FormatBool(inEffect), Applies: "now"}
	}
	tenant, err := st.TenantID()
	if err != nil {
		return nil, fail(http.StatusInternalServerError, "%v", err)
	}
	ch, err := st.SettingString(r.Context(), store.UpdateChannelKey)
	if err != nil {
		return nil, fail(http.StatusInternalServerError, "%v", err)
	}
	flavour := s.opts.Update.Flavour
	out[store.UpdateChannelKey] = settingView{Value: cmp.Or(ch, flavour), Default: flavour, Set: ch != "", InEffect: s.updateChannel(r.Context()), Applies: "now"}
	for _, d := range []struct{ key, def string }{{store.UpdateAutoKey, store.UpdateAutoOff}, {store.UpdateWindowKey, store.DefaultUpdateWindow},
		{store.ServerBackupTimeKey, store.DefaultServerBackupTime}} {
		v, err := st.SettingString(r.Context(), d.key)
		if err != nil {
			return nil, fail(http.StatusInternalServerError, "%v", err)
		}
		out[d.key] = settingView{Value: cmp.Or(v, d.def), Default: d.def, Set: v != "", InEffect: cmp.Or(v, d.def), Applies: "now"}
	}
	ann, err := st.SettingString(r.Context(), store.AnnounceKey)
	if err != nil {
		return nil, fail(http.StatusInternalServerError, "%v", err)
	}
	inEffect := store.AnnounceOff
	if s.opts.Announce.On {
		inEffect = store.AnnounceOn
	}
	out[store.AnnounceKey] = settingView{Value: cmp.Or(ann, store.AnnounceAuto), Default: store.AnnounceAuto, Set: ann != "", InEffect: inEffect, Applies: "restart",
		FixedBy: s.opts.Announce.FixedBy}
	out[store.NodeIDKey] = settingView{Value: st.NodeID(), Set: true, ReadOnly: true}
	out[store.TenantIDKey] = settingView{Value: tenant, Set: true, ReadOnly: true}
	return out, nil
}

// getSettings handles GET /api/v1/settings.
func (s *Server) getSettings(w http.ResponseWriter, r *http.Request) error {
	out, err := s.settingsView(r)
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, out)
	return nil
}

// putSettings handles PUT /api/v1/settings: an object of keys and
// values, a duration as a string ("36h", "0" to switch an interval off),
// a whole number as a number or a string, a switch as true or false, or
// null to go back to the default. Every value is checked before any is
// written.
func (s *Server) putSettings(w http.ResponseWriter, r *http.Request) error {
	st, err := s.cubeStore()
	if err != nil {
		return err
	}
	var req map[string]json.RawMessage
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return fail(http.StatusBadRequest, "bad body: %v (want {\"stale_after\": \"36h\", …}, null for the default)", err)
	}
	if len(req) == 0 {
		return fail(http.StatusBadRequest, "no setting given")
	}
	type write struct {
		key      string
		duration *time.Duration // a duration setting
		number   *int           // a whole-number setting
		on       *bool          // a switch
		name     *string        // a name of a few, or the update window
		clear    bool
	}
	var writes []write
	keys := make([]string, 0, len(req))
	for k := range req {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if k == store.NodeIDKey || k == store.TenantIDKey {
			return fail(http.StatusBadRequest, "%s cannot be changed", k)
		}
		raw := req[k]
		isNull := len(raw) == 0 || string(raw) == "null"
		if k == store.AnnounceKey {
			if isNull {
				writes = append(writes, write{key: k, clear: true})
				continue
			}
			var v string
			if json.Unmarshal(raw, &v) != nil || !slices.Contains(announceValues, v) {
				return fail(http.StatusBadRequest, "%s: want one of %s, or null for auto", k, strings.Join(announceValues, ", "))
			}
			writes = append(writes, write{key: k, name: &v})
			continue
		}
		if k == store.UpdateChannelKey {
			if isNull {
				writes = append(writes, write{key: k, clear: true})
				continue
			}
			var name string
			if json.Unmarshal(raw, &name) != nil || !slices.Contains(update.Flavours, name) {
				return fail(http.StatusBadRequest, "%s: want one of %s, or null for the build's flavour", k, strings.Join(update.Flavours, ", "))
			}
			writes = append(writes, write{key: k, name: &name})
			continue
		}
		if k == store.ServerBackupTimeKey {
			// the daily time of the server's backups, HH:MM in the box's
			// time (serverbackups.go); it applies at once
			if isNull {
				writes = append(writes, write{key: k, clear: true})
				continue
			}
			var v string
			if json.Unmarshal(raw, &v) != nil {
				return fail(http.StatusBadRequest, "%s: want a time HH:MM as a string, or null for %s", k, store.DefaultServerBackupTime)
			}
			if _, _, err := parseClock(v); err != nil {
				return fail(http.StatusBadRequest, "%s: %v", k, err)
			}
			v = strings.TrimSpace(v)
			writes = append(writes, write{key: k, name: &v})
			continue
		}
		if k == store.UpdateAutoKey || k == store.UpdateWindowKey {
			if isNull {
				writes = append(writes, write{key: k, clear: true})
				continue
			}
			var v string
			if json.Unmarshal(raw, &v) != nil {
				return fail(http.StatusBadRequest, "%s: want a string", k)
			}
			if k == store.UpdateAutoKey && v != store.UpdateAutoOff && v != store.UpdateAutoNight {
				return fail(http.StatusBadRequest, "%s: want %s or %s, or null for %s", k, store.UpdateAutoOff, store.UpdateAutoNight, store.UpdateAutoOff)
			}
			if _, _, err := parseWindow(v); k == store.UpdateWindowKey && err != nil {
				return fail(http.StatusBadRequest, "%s: %v", k, err)
			}
			writes = append(writes, write{key: k, name: &v})
			continue
		}
		var d *durationSetting
		for i := range durationSettings {
			if durationSettings[i].key == k {
				d = &durationSettings[i]
			}
		}
		if d != nil {
			if isNull {
				writes = append(writes, write{key: k, clear: true})
				continue
			}
			var text string
			if json.Unmarshal(raw, &text) != nil {
				return fail(http.StatusBadRequest, "%s: want a duration such as 30m or 24h, as a string", k)
			}
			v, err := parseSettingDuration(*d, text)
			if err != nil {
				return err
			}
			writes = append(writes, write{key: k, duration: &v})
			continue
		}
		if slices.ContainsFunc(boolSettings, func(b boolSetting) bool { return b.key == k }) {
			if isNull {
				writes = append(writes, write{key: k, clear: true})
				continue
			}
			var on bool
			if json.Unmarshal(raw, &on) != nil {
				return fail(http.StatusBadRequest, "%s: want true or false", k)
			}
			if k == store.KeepOpenKey && on && s.authMode(r.Context()) != modeOpen {
				return fail(http.StatusConflict, "%s counts only on a server without users: this one has users", k)
			}
			writes = append(writes, write{key: k, on: &on})
			continue
		}
		var n *intSetting
		for i := range intSettings {
			if intSettings[i].key == k {
				n = &intSettings[i]
			}
		}
		if n == nil {
			return fail(http.StatusBadRequest, "unknown setting %q", k)
		}
		if isNull {
			writes = append(writes, write{key: k, clear: true})
			continue
		}
		v, err := parseSettingInt(*n, raw)
		if err != nil {
			return err
		}
		writes = append(writes, write{key: k, number: &v})
	}
	if err := checkBackupWindows(r, st, func(key string) (int, bool, bool) {
		for _, wr := range writes {
			if wr.key == key {
				if wr.clear {
					return 0, true, true
				}
				return *wr.number, false, true
			}
		}
		return 0, false, false
	}); err != nil {
		return err
	}
	// The window's alert comes after its warning, not before.
	warn, alert := s.windowLimits()
	for _, wr := range writes {
		lim := map[string]*time.Duration{store.WindowWarnKey: &warn, store.WindowAlertKey: &alert}[wr.key]
		switch {
		case lim == nil:
		case wr.clear && wr.key == store.WindowWarnKey:
			*lim = view.WindowWarnAfter
		case wr.clear:
			*lim = view.WindowAlertAfter
		default:
			*lim = *wr.duration
		}
	}
	if alert < warn {
		return fail(http.StatusBadRequest, "%s (%s) must not be under %s (%s)", store.WindowAlertKey, fmtDuration(alert), store.WindowWarnKey, fmtDuration(warn))
	}
	// auth.keep_open first: its write counts the users again and may be
	// refused (409), and a refusal then comes before any other key is
	// written or audited.
	slices.SortStableFunc(writes, func(a, b write) int {
		return cmp.Compare(btoi(b.key == store.KeepOpenKey), btoi(a.key == store.KeepOpenKey))
	})
	pruneBackups := false
	for _, wr := range writes {
		var from any
		if prev, ok, err := st.Setting(r.Context(), wr.key); err == nil && ok {
			json.Unmarshal(prev, &from)
		}
		by := principalOf(r).userID()
		switch {
		case wr.clear:
			err = st.DeleteSettingBy(r.Context(), wr.key, by)
		case wr.on != nil && wr.key == store.KeepOpenKey:
			// The count of the users in the write's transaction: a first
			// user made since the check above wins.
			if err = st.SetKeepOpen(r.Context(), *wr.on, by); errors.Is(err, store.ErrHasUsers) {
				return fail(http.StatusConflict, "%s counts only on a server without users: this one has users", wr.key)
			}
		case wr.duration != nil:
			err = st.SetSettingBy(r.Context(), wr.key, fmtDuration(*wr.duration), by)
		case wr.on != nil:
			err = st.SetSettingBy(r.Context(), wr.key, *wr.on, by)
		case wr.name != nil:
			err = st.SetSettingBy(r.Context(), wr.key, *wr.name, by)
		default:
			err = st.SetSettingBy(r.Context(), wr.key, *wr.number, by)
		}
		if err != nil {
			return fail(http.StatusInternalServerError, "%v", err)
		}
		if wr.key == store.LogKeepDaysKey || wr.key == store.AuditKeepDaysKey {
			st.PruneEventsSoon() // the retention applies now
		}
		switch wr.key {
		case store.BackupsKeepDaysAutoKey, store.BackupsKeepDaysManualKey, store.BackupsKeepNewestKey:
			pruneBackups = true
		}
		if wr.key == store.StaleAfterKey || wr.key == store.StaleAfterWallKey {
			set := s.SetStaleAfter
			if wr.key == store.StaleAfterWallKey {
				set = s.SetStaleAfterWall
			}
			if wr.duration == nil {
				set(0)
			} else {
				set(*wr.duration)
			}
		}
		if wr.key == store.WindowWarnKey || wr.key == store.WindowAlertKey {
			s.loadWindowLimits(r.Context())
		}
		if wr.key == store.ServerBackupTimeKey {
			s.wakeServerBackups()
		}
		value := "default"
		switch {
		case wr.duration != nil:
			value = fmtDuration(*wr.duration)
		case wr.number != nil:
			value = strconv.Itoa(*wr.number)
		case wr.on != nil:
			value = strconv.FormatBool(*wr.on)
		case wr.name != nil:
			value = *wr.name
		}
		s.log.Info("setting changed", "key", wr.key, "value", value, "remote", s.clientIP(r))
		var to any
		switch {
		case wr.duration != nil:
			to = fmtDuration(*wr.duration)
		case wr.number != nil:
			to = *wr.number
		case wr.on != nil:
			to = *wr.on
		case wr.name != nil:
			to = *wr.name
		}
		auditChange(r, wr.key, from, to) // nil: the default
	}
	if pruneBackups {
		s.pruneBackups(r.Context(), st, time.Now())
	}
	return s.getSettings(w, r)
}

// btoi is 1 for true, 0 for false.
func btoi(b bool) int {
	if b {
		return 1
	}
	return 0
}

// checkBackupWindows refuses a change that leaves backups.auto_min above
// backups.auto_max: given reports a key's new value, whether it goes
// back to the default, and whether the request has it.
func checkBackupWindows(r *http.Request, st *store.Store, given func(key string) (n int, clear, ok bool)) error {
	cur, err := st.BackupSettings(r.Context())
	if err != nil {
		return fail(http.StatusInternalServerError, "%v", err)
	}
	value := func(key string, now, def int) int {
		n, clear, ok := given(key)
		switch {
		case !ok:
			return now
		case clear:
			return def
		}
		return n
	}
	lo := value(store.BackupsAutoMinKey, cur.AutoMin, store.DefaultBackupsAutoMin)
	hi := value(store.BackupsAutoMaxKey, cur.AutoMax, store.DefaultBackupsAutoMax)
	if lo > hi {
		return fail(http.StatusBadRequest, "%s (%d) must not be above %s (%d)", store.BackupsAutoMinKey, lo, store.BackupsAutoMaxKey, hi)
	}
	return nil
}

// parseSettingInt reads and checks one whole number, given as a JSON
// number or as a string of digits.
func parseSettingInt(d intSetting, raw json.RawMessage) (int, error) {
	var n int
	if err := json.Unmarshal(raw, &n); err != nil {
		var text string
		if json.Unmarshal(raw, &text) != nil {
			return 0, fail(http.StatusBadRequest, "%s: want a whole number", d.key)
		}
		if n, err = strconv.Atoi(strings.TrimSpace(text)); err != nil {
			return 0, fail(http.StatusBadRequest, "%s %q: want a whole number", d.key, text)
		}
	}
	if n < d.min || n > d.max {
		return 0, fail(http.StatusBadRequest, "%s %d: between %d and %d", d.key, n, d.min, d.max)
	}
	return n, nil
}

// parseSettingDuration reads and checks one duration.
func parseSettingDuration(d durationSetting, text string) (time.Duration, error) {
	text = strings.TrimSpace(text)
	v, err := time.ParseDuration(text)
	if text == "0" {
		v, err = 0, nil
	}
	if err != nil {
		return 0, fail(http.StatusBadRequest, "%s %q: want a duration such as 30m or 24h", d.key, text)
	}
	if v == 0 && d.zero {
		return 0, nil
	}
	if v < d.min || v > d.max {
		off := ""
		if d.zero {
			off = ", or 0 for off"
		}
		return 0, fail(http.StatusBadRequest, "%s %s: between %s and %s%s", d.key, text, fmtDuration(d.min), fmtDuration(d.max), off)
	}
	return v, nil
}
