// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	goruntime "runtime"
	"strings"
	"testing"

	"go.metaworx.ch/massimo/internal/update"
)

type fakeFinder struct {
	tag   string // "" = no release
	calls int
}

func (f *fakeFinder) Latest(ctx context.Context, channel string) (*update.Release, error) {
	f.calls++
	if f.tag == "" {
		return nil, nil
	}
	v, _ := update.ParseVersion(f.tag)
	return &update.Release{Manifest: update.Manifest{Tag: f.tag, Channel: channel}, Version: v, URL: "https://example/" + f.tag}, nil
}

func updateServer(t *testing.T, token, request string, f *fakeFinder) (*httptest.Server, string) {
	status := filepath.Join(t.TempDir(), "status.json")
	s := New(Options{Token: token, Update: UpdateOptions{Version: "v0.1.0-alpha.9", Channel: "alpha", Request: request, Status: status, Finder: f}})
	ts := httptest.NewServer(s.Handler())
	t.Cleanup(ts.Close)
	return ts, status
}

func call(t *testing.T, ts *httptest.Server, method, token string) (int, map[string]any) {
	t.Helper()
	req, _ := http.NewRequest(method, ts.URL+"/api/v1/update", strings.NewReader("{}"))
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var out map[string]any
	json.NewDecoder(resp.Body).Decode(&out)
	return resp.StatusCode, out
}

func TestUpdateEndpoints(t *testing.T) {
	if goruntime.GOOS == "windows" {
		t.Skip("updates are refused on Windows")
	}
	req := filepath.Join(t.TempDir(), "update-request")
	f := &fakeFinder{tag: "v0.1.0-alpha.10"}
	ts, status := updateServer(t, "secret", req, f)

	if code, _ := call(t, ts, "GET", ""); code != 401 {
		t.Fatalf("GET without token: %d", code)
	}
	code, out := call(t, ts, "GET", "secret")
	if code != 200 || out["latest"] != "v0.1.0-alpha.10" || out["available"] != true || out["can_update"] != true || out["channel"] != "alpha" {
		t.Fatalf("GET: %d %v", code, out)
	}
	call(t, ts, "GET", "secret")
	if f.calls != 1 {
		t.Fatalf("release checked %d times, want the cached one", f.calls)
	}

	code, out = call(t, ts, "POST", "secret")
	if code != 202 || out["requested"] != "v0.1.0-alpha.10" {
		t.Fatalf("POST: %d %v", code, out)
	}
	if b, err := os.ReadFile(req); err != nil || strings.TrimSpace(string(b)) != "alpha" {
		t.Fatalf("request file: %q %v", b, err)
	}

	// While the helper works, a second request is refused.
	update.WriteStatus(status, update.Status{State: "installing", To: "v0.1.0-alpha.10"})
	if code, out := call(t, ts, "POST", "secret"); code != 409 {
		t.Fatalf("POST while running: %d %v", code, out)
	}
	update.WriteStatus(status, update.Status{State: "failed", To: "v0.1.0-alpha.10", Message: "boom"})
	code, out = call(t, ts, "GET", "secret")
	if h, _ := out["helper"].(map[string]any); code != 200 || h["state"] != "failed" || h["message"] != "boom" {
		t.Fatalf("GET helper: %d %v", code, out)
	}

	// Nothing newer: up to date.
	f.tag = "v0.1.0-alpha.9"
	if code, out := call(t, ts, "POST", "secret"); code != 409 || !strings.Contains(out["error"].(string), "up to date") {
		t.Fatalf("POST up to date: %d %v", code, out)
	}
}

func TestUpdateNeedsTokenAndHelper(t *testing.T) {
	req := filepath.Join(t.TempDir(), "update-request")
	// No token: the API is open, but updates are not.
	ts, _ := updateServer(t, "", req, &fakeFinder{tag: "v0.1.0-alpha.10"})
	code, out := call(t, ts, "GET", "")
	if code != 200 || out["available"] != true || out["can_update"] != false || out["blocked"] == "" {
		t.Fatalf("GET: %d %v", code, out)
	}
	if code, _ := call(t, ts, "POST", ""); code != 403 {
		t.Fatalf("POST without a token configured: %d", code)
	}
	if _, err := os.Stat(req); !os.IsNotExist(err) {
		t.Fatal("request written")
	}
	// A token but no helper.
	ts, _ = updateServer(t, "secret", "", &fakeFinder{tag: "v0.1.0-alpha.10"})
	if code, _ := call(t, ts, "POST", "secret"); code != 403 {
		t.Fatalf("POST without a helper: %d", code)
	}
}
