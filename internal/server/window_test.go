// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"go.metaworx.ch/massimo/internal/store"
	"go.metaworx.ch/massimo/internal/view"
)

// The window's two limits: 5 and 10 minutes unset, a change applies at
// once and is stored, the alert never under the warning, null goes back
// to the default.
func TestWindowSettings(t *testing.T) {
	_, ts, srv, st := startStored(t, Options{})
	lim := func(out map[string]any, key string) map[string]any { m, _ := out[key].(map[string]any); return m }
	code, out := do(t, ts, "GET", "/api/v1/settings", "")
	if w, a := lim(out, "window_warn_after"), lim(out, "window_alert_after"); code != 200 || w["value"] != "5m" || a["value"] != "10m" || w["applies"] != "now" || w["set"] != false {
		t.Fatalf("defaults: %d %v %v", code, w, a)
	}
	code, out = do(t, ts, "PUT", "/api/v1/settings", `{"window_warn_after":"2m","window_alert_after":"4m"}`)
	if code != 200 || lim(out, "window_warn_after")["in_effect"] != "2m" || lim(out, "window_alert_after")["in_effect"] != "4m" {
		t.Fatalf("put: %d %v", code, out)
	}
	if w, a := srv.windowLimits(); w != 2*time.Minute || a != 4*time.Minute {
		t.Fatalf("in effect %v %v", w, a)
	}
	if d, ok, _ := st.SettingDuration(context.Background(), store.WindowAlertKey); !ok || d != 4*time.Minute {
		t.Fatalf("stored %v %v", d, ok)
	}
	for _, bad := range []string{`{"window_alert_after":"1m"}`, `{"window_warn_after":"5m"}`, `{"window_warn_after":"30s"}`, `{"window_alert_after":"25h"}`} {
		if code, out := do(t, ts, "PUT", "/api/v1/settings", bad); code != 400 {
			t.Errorf("%s: %d %v", bad, code, out)
		}
	}
	if code, out := do(t, ts, "PUT", "/api/v1/settings", `{"window_warn_after":null,"window_alert_after":null}`); code != 200 || lim(out, "window_warn_after")["set"] != false {
		t.Fatalf("back to the defaults: %d %v", code, out)
	}
	if w, a := srv.windowLimits(); w != view.WindowWarnAfter || a != view.WindowAlertAfter {
		t.Fatalf("defaults in effect %v %v", w, a)
	}
}

// An open window's device carries its times by the limits in effect;
// a closed one and one without the time it opened none.
func TestWindowTimes(t *testing.T) {
	srv := New(Options{})
	srv.SetWindowLimits(3*time.Minute, 7*time.Minute)
	at := time.Date(2026, 11, 14, 12, 0, 0, 0, time.UTC)
	yes, no := true, false
	devs := srv.withWindow([]view.Device{
		{RFAddress: "0a0101", State: &view.State{WindowOpen: &yes}, Received: map[string]time.Time{"window_open": at}},
		{RFAddress: "0a0102", State: &view.State{WindowOpen: &no}, Received: map[string]time.Time{"window_open": at}},
		{RFAddress: "0a0103", State: &view.State{WindowOpen: &yes}},
		{RFAddress: "0a0104"},
	})
	if w := devs[0].Window; w == nil || !w.OpenSince.Equal(at) || !w.WarnAt.Equal(at.Add(3*time.Minute)) || !w.AlertAt.Equal(at.Add(7*time.Minute)) {
		t.Fatalf("open window: %+v", w)
	}
	for _, d := range devs[1:] {
		if d.Window != nil {
			t.Errorf("%s: times %+v", d.RFAddress, d.Window)
		}
	}
	b, _ := json.Marshal(devs[0])
	var back map[string]any
	json.Unmarshal(b, &back)
	if w, _ := back["window"].(map[string]any); w["warn_at"] != "2026-11-14T12:03:00Z" || w["alert_at"] != "2026-11-14T12:07:00Z" {
		t.Fatalf("the API's window: %s", b)
	}
}
