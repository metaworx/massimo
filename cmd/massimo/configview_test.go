// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package main

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"

	"go.metaworx.ch/massimo/internal/server"
	"go.metaworx.ch/massimo/internal/testkit/storekit"
)

// configView, GET /api/v1/config's settings in effect, marks a key, an
// IV and the token as set and leaves their values out, and shows the
// others with where they came from.
func TestConfigViewHidesSecrets(t *testing.T) {
	t.Setenv("MASSIMO_API_TOKEN", "token-value-1")
	t.Setenv("MASSIMO_CUBE_KEQ0000100_KEY", "00112233445566778899aabbccddeeff")
	t.Setenv("MASSIMO_CUBE_KEQ0000100_IV", "ffeeddccbbaa99887766554433221100")
	t.Setenv("MASSIMO_LISTEN", ":8099")
	v := configView(":8099", "off")
	if v.Listen != ":8099" || v.TLS != "off" || v.File == "" {
		t.Fatalf("view %+v", v)
	}
	seen := map[string]bool{}
	for _, cv := range v.Variables {
		seen[cv.Name] = true
		switch cv.Name {
		case "MASSIMO_API_TOKEN", "MASSIMO_CUBE_KEQ0000100_KEY", "MASSIMO_CUBE_KEQ0000100_IV":
			if !cv.Secret || cv.Value != "" {
				t.Errorf("%s shown: %+v", cv.Name, cv)
			}
		case "MASSIMO_LISTEN":
			if cv.Secret || cv.Value != ":8099" || cv.Source == "" {
				t.Errorf("MASSIMO_LISTEN: %+v", cv)
			}
		}
	}
	for _, k := range []string{"MASSIMO_API_TOKEN", "MASSIMO_CUBE_KEQ0000100_KEY", "MASSIMO_LISTEN"} {
		if !seen[k] {
			t.Errorf("%s not listed", k)
		}
	}
}

// MASSIMO_API_Token, which Windows' start takes from the settings file
// as the owner's key, shows no value in GET /api/v1/config, on every
// system (AP StartConfiguration, D53-A). The environment holds it, as
// the file's line sets it on Windows; elsewhere the file's line is set
// aside (TestKnownNameInAnotherCase of internal/env).
func TestConfigHidesASecretInAnyCase(t *testing.T) {
	unsetForTest(t, tokenVar)
	t.Setenv("MASSIMO_API_Token", "placeholder-token-1")
	srv := server.New(server.Options{Store: storekit.Open(t, ""), Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	srv.SetConfigView(func() server.ConfigView { return configView(":8099", "off") })
	req := httptest.NewRequest("GET", "/api/v1/config", nil)
	req.RemoteAddr = "127.0.0.1:5000" // the open server answers its local network
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	text := rec.Body.String()
	if rec.Code != 200 || strings.Contains(text, "placeholder-token-1") {
		t.Fatalf("GET /api/v1/config: %d %s", rec.Code, text)
	}
	var v server.ConfigView
	if err := json.Unmarshal(rec.Body.Bytes(), &v); err != nil {
		t.Fatal(err)
	}
	for _, cv := range v.Variables {
		if cv.Name == "MASSIMO_API_Token" {
			if !cv.Secret {
				t.Fatalf("MASSIMO_API_Token not marked as a secret: %+v", cv)
			}
			return
		}
	}
	t.Fatalf("MASSIMO_API_Token not listed: %s", text)
}

// massimo config redacts a secret's value whatever the case of its name,
// as GET /api/v1/config leaves it out: MASSIMO_API_Token, which Windows'
// start takes as the owner's key, and a password's (AP
// StartConfiguration, D53-A).
func TestConfigRedactsASecretInAnyCase(t *testing.T) {
	unsetForTest(t, tokenVar)
	t.Setenv("MASSIMO_API_Token", "placeholder-token-1")
	t.Setenv("MASSIMO_SMTP_Password", "placeholder-value-2")
	var out strings.Builder
	printVariables(&out)
	redacted := 0
	for _, line := range strings.Split(out.String(), "\n") {
		if f := strings.Fields(line); len(f) > 1 && (f[0] == "MASSIMO_API_Token" || f[0] == "MASSIMO_SMTP_Password") && f[1] == "set" {
			redacted++
		}
	}
	if strings.Contains(out.String(), "placeholder-") || redacted != 2 {
		t.Fatalf("massimo config's variables:\n%s", out.String())
	}
}
