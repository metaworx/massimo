// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

import (
	"encoding/json"
	"strings"
	"testing"

	"go.metaworx.ch/massimo/internal/store"
)

// GET /api/v1/store shows the store as store info does and GET
// /api/v1/config the settings in effect, both for a holder of settings
// alone; a secret's value never leaves the server, whatever the view
// handed to it holds.
func TestStoreAndConfigViews(t *testing.T) {
	opts := storeOptions(t, t.TempDir(), "secret")
	_, ts, s := startWith(t, opts)
	manager := userToken(t, s.opts.Store, "manager1", store.RoleManager)
	code, text := callAPI(t, ts, "GET", "/api/v1/store", "secret", "")
	var sv storeView
	if err := json.Unmarshal([]byte(text), &sv); err != nil || code != 200 || sv.File == "" || sv.Schema < 1 || sv.Binary < sv.Schema ||
		sv.Node == "" || sv.Rows["users"] < 1 || sv.Rows["roles"] < 6 {
		t.Fatalf("GET /api/v1/store as owner: %d %s", code, text)
	}
	for _, path := range []string{"/api/v1/store", "/api/v1/config"} {
		if code, text := callAPI(t, ts, "GET", path, manager, ""); code != 403 || errorOf(text) != `capability "settings" required` {
			t.Fatalf("%s as manager: %d %s", path, code, text)
		}
	}
	if code, text := callAPI(t, ts, "GET", "/api/v1/config", "secret", ""); code != 503 {
		t.Fatalf("GET /api/v1/config without a view: %d %s", code, text)
	}
	s.SetConfigView(func() ConfigView {
		return ConfigView{File: "/etc/massimo/env", FileFound: true, Listen: ":8099", TLS: "off", Variables: []ConfigVariable{
			{Name: "MASSIMO_API_TOKEN", Value: "token-value-1", Source: "file"},
			{Name: "MASSIMO_CUBE_KEQ0000100_KEY", Value: "00112233445566778899aabbccddeeff", Source: "file"},
			{Name: "MASSIMO_LISTEN", Value: ":8099", Source: "file"},
		}}
	})
	code, text = callAPI(t, ts, "GET", "/api/v1/config", "secret", "")
	if strings.Contains(text, "token-value-1") || strings.Contains(text, "00112233") {
		t.Fatalf("a secret left the server: %s", text)
	}
	var cv ConfigView
	if err := json.Unmarshal([]byte(text), &cv); err != nil || code != 200 || cv.Version != s.opts.Version || len(cv.Variables) != 3 ||
		!cv.Variables[0].Secret || !cv.Variables[1].Secret || cv.Variables[2].Value != ":8099" || cv.Variables[2].Secret {
		t.Fatalf("GET /api/v1/config as owner: %d %s", code, text)
	}
}
