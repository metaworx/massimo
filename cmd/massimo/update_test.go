// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"go.metaworx.ch/massimo/internal/env"
	"go.metaworx.ch/massimo/internal/update"
)

func TestHealthClient(t *testing.T) {
	if healthClient("http://127.0.0.1:8099/healthz").Transport != nil {
		t.Fatal("plain http given a transport of its own")
	}
	if healthClient("https://192.0.2.10:8099/healthz").Transport != nil {
		t.Fatal("a LAN host not verified")
	}
	for _, u := range []string{"https://127.0.0.1:8099/healthz", "https://[::1]:8099/healthz", "https://localhost:8099/healthz"} {
		tr, ok := healthClient(u).Transport.(*http.Transport)
		if !ok || tr.TLSClientConfig == nil || !tr.TLSClientConfig.InsecureSkipVerify {
			t.Fatalf("%s verified", u)
		}
	}
}

// TestWaitHealthyOverTLS checks the helper's health check against a
// server with a certificate nothing trusts, on the loopback.
func TestWaitHealthyOverTLS(t *testing.T) {
	ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"ok":true,"version":"massimo v1.2.3 (go)"}`)
	}))
	defer ts.Close()
	if err := waitHealthy(context.Background(), ts.URL+"/healthz", "v1.2.3", 5*time.Second); err != nil {
		t.Fatal(err)
	}
	if err := waitHealthy(context.Background(), ts.URL+"/healthz", "v1.2.4", time.Second); err == nil {
		t.Fatal("wrong version passed")
	}
}

// TestUpdateVariablesAreKnown keeps massimo config from flagging the
// variables massimo update reads as typos.
func TestUpdateVariablesAreKnown(t *testing.T) {
	for _, k := range []string{update.TokenVar, "MASSIMO_UPDATE_CHANNEL"} {
		if !env.Known(k) {
			t.Errorf("%s unknown to massimo config", k)
		}
	}
}

// TestSmokeRunsRealBinary builds massimo and runs the update's smoke test
// on it: the stand-in scripts of internal/update cannot show how the real
// binary starts (it loads its .env before every command).
func TestSmokeRunsRealBinary(t *testing.T) {
	if testing.Short() {
		t.Skip("builds the binary")
	}
	bin := filepath.Join(t.TempDir(), "massimo")
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}
	out, err := exec.Command("go", "build", "-ldflags", "-X main.version=v9.8.7", "-o", bin, ".").CombinedOutput()
	if err != nil {
		t.Fatalf("go build: %v\n%s", err, out)
	}
	if err := update.Smoke(context.Background(), bin, "v9.8.7"); err != nil {
		t.Fatal(err)
	}
	if err := update.Smoke(context.Background(), bin, "v9.8.8"); err == nil {
		t.Fatal("wrong version passed")
	}
}
