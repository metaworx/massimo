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
	"fmt"
	"net/http"
	"sort"
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

var durationSettings = []durationSetting{
	{key: store.StaleAfterKey, def: view.StaleAfter, min: time.Hour, max: 30 * 24 * time.Hour, now: true,
		inEffect: func(*Server) time.Duration { return view.StaleAfterNow() }},
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
// values, a duration as a string ("36h", "0" to switch an interval off)
// or null to go back to the default. Every value is checked before any
// is written.
func (s *Server) putSettings(w http.ResponseWriter, r *http.Request) error {
	st, err := s.cubeStore()
	if err != nil {
		return err
	}
	var req map[string]*string
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return fail(http.StatusBadRequest, "bad body: %v (want {\"stale_after\": \"36h\", …}, null for the default)", err)
	}
	if len(req) == 0 {
		return fail(http.StatusBadRequest, "no setting given")
	}
	type write struct {
		d     durationSetting
		value *time.Duration
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
		var d *durationSetting
		for i := range durationSettings {
			if durationSettings[i].key == k {
				d = &durationSettings[i]
			}
		}
		if d == nil {
			return fail(http.StatusBadRequest, "unknown setting %q", k)
		}
		if req[k] == nil {
			writes = append(writes, write{d: *d})
			continue
		}
		v, err := parseSettingDuration(*d, *req[k])
		if err != nil {
			return err
		}
		writes = append(writes, write{d: *d, value: &v})
	}
	for _, wr := range writes {
		if wr.value == nil {
			err = st.DeleteSetting(r.Context(), wr.d.key)
		} else {
			err = st.SetSetting(r.Context(), wr.d.key, fmtDuration(*wr.value))
		}
		if err != nil {
			return fail(http.StatusInternalServerError, "%v", err)
		}
		if wr.d.key == store.StaleAfterKey {
			if wr.value == nil {
				view.SetStaleAfter(0)
			} else {
				view.SetStaleAfter(*wr.value)
			}
		}
		s.log.Info("setting changed", "key", wr.d.key, "value", settingLogValue(wr.value), "remote", r.RemoteAddr)
	}
	return s.getSettings(w, r)
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

func settingLogValue(v *time.Duration) string {
	if v == nil {
		return "default"
	}
	return fmt.Sprint(fmtDuration(*v))
}
