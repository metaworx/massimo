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
// handed to it holds and whatever the case of its name.
func TestStoreAndConfigViews(t *testing.T) {
	t.Parallel()
	opts := storeOptions(t, t.TempDir(), "secret")
	r := startServer(t, withOptions(opts))
	ts, s := r.ts, r.s
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
			{Name: "MASSIMO_API_Token", Value: "token-value-2", Source: "file"},
		}}
	})
	code, text = callAPI(t, ts, "GET", "/api/v1/config", "secret", "")
	if strings.Contains(text, "token-value-1") || strings.Contains(text, "00112233") || strings.Contains(text, "token-value-2") {
		t.Fatalf("a secret left the server: %s", text)
	}
	var cv ConfigView
	if err := json.Unmarshal([]byte(text), &cv); err != nil || code != 200 || cv.Version != s.opts.Version || len(cv.Variables) != 4 ||
		!cv.Variables[0].Secret || !cv.Variables[1].Secret || cv.Variables[2].Value != ":8099" || cv.Variables[2].Secret || !cv.Variables[3].Secret {
		t.Fatalf("GET /api/v1/config as owner: %d %s", code, text)
	}
}

// A secret is known by its name in any case, on every system: Windows
// takes MASSIMO_API_Token in the settings file as the owner's key (AP
// StartConfiguration, D53-A).
func TestSecretVariableInAnyCase(t *testing.T) {
	t.Parallel()
	for name, want := range map[string]bool{
		"MASSIMO_API_TOKEN": true, "MASSIMO_API_Token": true, "massimo_api_token": true, "massimo_tls_key": true,
		"Massimo_Cube_Iv": true, "MASSIMO_SMTP_Password": true, "massimo_oidc_secret": true,
		"MASSIMO_LISTEN": false, "massimo_listen": false, "MASSIMO_TLS_NAMES": false,
	} {
		if got := SecretVariable(name); got != want {
			t.Errorf("SecretVariable(%q) = %v, want %v", name, got, want)
		}
	}
}
