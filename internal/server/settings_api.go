// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

// The server's settings in the store (GET and PUT /api/v1/settings): the
// stale threshold, which applies at once, and the poll, refresh and
// discovery intervals, which apply at the next start; the command-line
// flags win over them for one run. node.id and tenant.id are shown and
// cannot be changed.

import (
	"encoding/json"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"go.metaworx.ch/massimo/internal/store"
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

// Defaults of the settings, as serve's flags have them.
const (
	defaultPoll     = time.Minute
	defaultRefresh  = 6 * time.Hour
	defaultDiscover = 30 * time.Minute
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
}

var durationSettings = []durationSetting{
	{key: store.StaleAfterKey, def: view.StaleAfter, min: time.Hour, max: 30 * 24 * time.Hour, now: true,
		inEffect: func(s *Server) time.Duration { return s.StaleAfter() }},
	{key: store.PollIntervalKey, def: defaultPoll, min: 10 * time.Second, max: time.Hour,
		inEffect: func(s *Server) time.Duration { return s.opts.PollInterval }},
	{key: store.RefreshIntervalKey, def: defaultRefresh, min: 10 * time.Minute, max: 7 * 24 * time.Hour, zero: true,
		inEffect: func(s *Server) time.Duration { return s.opts.RefreshInterval }},
	{key: store.DiscoverIntervalKey, def: defaultDiscover, min: time.Minute, max: 24 * time.Hour, zero: true,
		inEffect: func(s *Server) time.Duration { return s.opts.DiscoverInterval }},
}

// settingView is one setting as GET /api/v1/settings shows it.
type settingView struct {
	Value    string `json:"value"`               // the store's value, or the default
	Default  string `json:"default,omitempty"`   // "" for the read-only ones
	Set      bool   `json:"set"`                 // the store holds a value
	InEffect string `json:"in_effect,omitempty"` // what this run uses
	// Applies is "now" or "restart"; "" for the read-only ones.
	Applies  string `json:"applies,omitempty"`
	ReadOnly bool   `json:"read_only,omitempty"`
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
		sv := settingView{Value: fmtDuration(v), Default: fmtDuration(d.def), Set: ok, InEffect: fmtDuration(d.inEffect(s)), Applies: "restart"}
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
	tenant, err := st.TenantID()
	if err != nil {
		return nil, fail(http.StatusInternalServerError, "%v", err)
	}
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
// a whole number as a number or a string, or null to go back to the
// default. Every value is checked before any is written.
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
		clear    bool
		d        *durationSetting
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
		var d *durationSetting
		for i := range durationSettings {
			if durationSettings[i].key == k {
				d = &durationSettings[i]
			}
		}
		if d != nil {
			if isNull {
				writes = append(writes, write{key: k, clear: true, d: d})
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
			writes = append(writes, write{key: k, duration: &v, d: d})
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
	for _, wr := range writes {
		switch {
		case wr.clear:
			err = st.DeleteSetting(r.Context(), wr.key)
		case wr.duration != nil:
			err = st.SetSetting(r.Context(), wr.key, fmtDuration(*wr.duration))
		default:
			err = st.SetSetting(r.Context(), wr.key, *wr.number)
		}
		if err != nil {
			return fail(http.StatusInternalServerError, "%v", err)
		}
		if wr.key == store.StaleAfterKey {
			if wr.duration == nil {
				s.SetStaleAfter(0)
			} else {
				s.SetStaleAfter(*wr.duration)
			}
		}
		value := "default"
		switch {
		case wr.duration != nil:
			value = fmtDuration(*wr.duration)
		case wr.number != nil:
			value = strconv.Itoa(*wr.number)
		}
		s.log.Info("setting changed", "key", wr.key, "value", value, "remote", r.RemoteAddr)
	}
	return s.getSettings(w, r)
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
