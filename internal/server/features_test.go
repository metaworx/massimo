// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// featuresAt is GET /healthz's features map.
func featuresAt(t *testing.T, ts *httptest.Server) map[string]string {
	t.Helper()
	resp, err := http.Get(ts.URL + "/healthz")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var h struct {
		Features map[string]string `json:"features"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&h); err != nil || resp.StatusCode != 200 {
		t.Fatalf("healthz: %d %v", resp.StatusCode, err)
	}
	return h.Features
}

// GET /healthz names what this server can do: with a store all of it,
// without one the features that need it left out, as their routes are.
func TestHealthzFeatures(t *testing.T) {
	_, ts, _, _ := startRooms(t) // with a store
	with := featuresAt(t, ts)
	for _, name := range []string{"event_ids", "idempotency", "commands", "vwt", "program_etag", "boost", "proxy_virtual", "integration"} {
		if with[name] != "1.0" {
			t.Errorf("with a store: %s = %q, want 1.0", name, with[name])
		}
	}
	if len(with) != 8 {
		t.Errorf("with a store: %v", with)
	}

	_, bare, _ := start(t, "") // without one
	without := featuresAt(t, bare)
	for _, name := range []string{"idempotency", "commands", "proxy_virtual", "integration"} {
		if _, ok := without[name]; ok {
			t.Errorf("without a store: %s listed", name)
		}
	}
	if without["vwt"] != "1.0" || without["event_ids"] != "1.0" {
		t.Errorf("without a store: %v", without)
	}
	// What the map leaves out answers as such: no commands without a store.
	if code, _ := callAPI(t, bare, "GET", "/api/v1/commands/1", "", ""); code == 200 {
		t.Error("commands answer without a store")
	}
}
