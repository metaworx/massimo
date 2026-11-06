// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
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
	return updateServerWith(t, token, UpdateOptions{Request: request, Finder: f})
}

func updateServerWith(t *testing.T, token string, o UpdateOptions) (*httptest.Server, string) {
	status := filepath.Join(t.TempDir(), "status.json")
	o.Version, o.Channel, o.Status = "v0.1.0-alpha.9", "alpha", status
	s := New(Options{Token: token, Update: o})
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
	// A helper that cannot be started here.
	ts, _ = updateServerWith(t, "secret", UpdateOptions{Request: req, Finder: &fakeFinder{tag: "v0.1.0-alpha.10"}, Start: func() error { return nil }, Blocked: "in a job"})
	if code, out := call(t, ts, "GET", "secret"); code != 200 || out["can_update"] != false || out["blocked"] != "in a job" {
		t.Fatalf("GET blocked: %d %v", code, out)
	}
	if code, _ := call(t, ts, "POST", "secret"); code != 403 {
		t.Fatalf("POST blocked: %d", code)
	}
}

// Where the server starts the helper itself (Windows): once per accepted
// request; a failed start removes the request, a running one is a 409.
func TestUpdateStartsHelper(t *testing.T) {
	req := filepath.Join(t.TempDir(), "update-request")
	var starts int
	var startErr error
	start := func() error {
		starts++
		if _, err := os.Stat(req); err != nil {
			t.Errorf("started before the request was written: %v", err)
		}
		return startErr
	}
	ts, status := updateServerWith(t, "secret", UpdateOptions{Request: req, Finder: &fakeFinder{tag: "v0.1.0-alpha.10"}, Start: start})

	if code, out := call(t, ts, "POST", "secret"); code != 202 || starts != 1 {
		t.Fatalf("POST: %d %v, %d starts", code, out, starts)
	}
	// The helper took the request and works: refused from the status file
	// without a start.
	os.Remove(req)
	update.WriteStatus(status, update.Status{State: "checking"})
	if code, _ := call(t, ts, "POST", "secret"); code != 409 || starts != 1 {
		t.Fatalf("POST while running: %d, %d starts", code, starts)
	}
	update.WriteStatus(status, update.Status{State: "failed", Message: "boom"})

	// Before the helper's first status: the starter's own guard.
	startErr = update.ErrHelperRunning
	if code, out := call(t, ts, "POST", "secret"); code != 409 || starts != 2 || !strings.Contains(out["error"].(string), "already running") {
		t.Fatalf("POST while starting: %d %v, %d starts", code, out, starts)
	}
	os.Remove(req)

	startErr = errors.New("no such file")
	if code, out := call(t, ts, "POST", "secret"); code != 500 || starts != 3 || !strings.Contains(out["error"].(string), "no such file") {
		t.Fatalf("POST with a failing start: %d %v", code, out)
	}
	if _, err := os.Stat(req); !os.IsNotExist(err) {
		t.Fatalf("request left after a failed start: %v", err)
	}
}

// TestUpdateManaged: where a package manager owns the binary, the API
// still says what is out, names the manager as the reason, and starts
// nothing.
func TestUpdateManaged(t *testing.T) {
	req := filepath.Join(t.TempDir(), "update-request")
	f := &fakeFinder{tag: "v0.1.0-alpha.10"}
	ts, _ := updateServerWith(t, "secret", UpdateOptions{Request: req, Finder: f, Managed: "managed by apt: sudo apt install massimo"})
	code, out := call(t, ts, "GET", "secret")
	if code != 200 || out["available"] != true || out["can_update"] != false || out["blocked"] != "managed by apt: sudo apt install massimo" {
		t.Fatalf("GET: %d %v", code, out)
	}
	if code, out := call(t, ts, "POST", "secret"); code != 403 {
		t.Fatalf("POST: %d %v", code, out)
	}
	if _, err := os.Stat(req); err == nil {
		t.Fatal("a request was written")
	}
}
