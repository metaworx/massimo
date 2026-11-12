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
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"go.metaworx.ch/massimo/internal/store"
	"go.metaworx.ch/massimo/internal/update"
)

type fakeFinder struct {
	tag     string // "" = no release
	flavour string // the release's flavour; "" = the channel asked for
	calls   int
	channel string // the channel last asked for
}

func (f *fakeFinder) Latest(ctx context.Context, channel string) (*update.Release, error) {
	f.calls++
	f.channel = channel
	if f.tag == "" {
		return nil, nil
	}
	v, _ := update.ParseVersion(f.tag)
	flavour := f.flavour
	if flavour == "" {
		flavour = channel
	}
	return &update.Release{Manifest: update.Manifest{Tag: f.tag, Flavour: flavour}, Version: v, URL: "https://example/" + f.tag}, nil
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

// TestUpdateThroughApt: where apt owns the binary and the package's
// helper is on, the button asks it as on a hand install, and the API
// names apt; with the helper off, the reason is that.
func TestUpdateThroughApt(t *testing.T) {
	req := filepath.Join(t.TempDir(), "update-request")
	off := ""
	ts, _ := updateServerWith(t, "secret", UpdateOptions{Request: req, Finder: &fakeFinder{tag: "v0.1.0-alpha.10"}, Manager: "apt", Helper: func() string { return off }})
	code, out := call(t, ts, "GET", "secret")
	if code != 200 || out["manager"] != "apt" || out["can_update"] != true || out["blocked"] != nil {
		t.Fatalf("GET: %d %v", code, out)
	}
	if code, out := call(t, ts, "POST", "secret"); code != 202 {
		t.Fatalf("POST: %d %v", code, out)
	}
	if b, err := os.ReadFile(req); err != nil || string(b) != "alpha\n" {
		t.Fatalf("request %q, %v", b, err)
	}
	os.Remove(req)
	off = "the update helper is off"
	code, out = call(t, ts, "GET", "secret")
	if code != 200 || out["manager"] != "apt" || out["can_update"] != false || out["blocked"] != off {
		t.Fatalf("GET with the helper off: %d %v", code, out)
	}
	if code, _ := call(t, ts, "POST", "secret"); code != 403 {
		t.Fatalf("POST with the helper off: %d", code)
	}
	if _, err := os.Stat(req); err == nil {
		t.Fatal("a request was written with the helper off")
	}
}

// An update request's 202 has no run that reports back to its command:
// the command is acknowledged with the answer, and a retry with its
// Idempotency-Key gets the 202 again instead of "still running".
func TestUpdateCommandIsAcknowledged(t *testing.T) {
	dir := t.TempDir()
	opts := storeOptions(t, t.TempDir(), "secret")
	opts.Update = UpdateOptions{Version: "v0.1.0-alpha.9", Channel: "alpha", Request: filepath.Join(dir, "update-request"), Status: filepath.Join(dir, "status.json"),
		Finder: &fakeFinder{tag: "v0.1.0-alpha.10"}}
	_, ts, s := startWith(t, opts)
	stores[ts] = s.opts.Store
	headers := map[string]string{"Idempotency-Key": "update-1"}
	if resp, text := callWith(t, ts, "POST", "/api/v1/update", "secret", "{}", headers); resp.StatusCode != 202 {
		t.Fatalf("update: %d %s", resp.StatusCode, text)
	}
	rows := auditRows(t, ts, "secret", "?limit=1")
	if c := commandOfRow(t, ts, "secret", rows[0]); c.Action != "update.start" || c.State != store.StateAcknowledged || c.Status != 202 || c.FinishedAt == nil {
		t.Fatalf("the update's command %+v", c)
	}
	os.Remove(opts.Update.Request)
	if resp, text := callWith(t, ts, "POST", "/api/v1/update", "secret", "{}", headers); resp.StatusCode != 202 || resp.Header.Get("Idempotent-Replayed") != "true" {
		t.Fatalf("the retry: %d %s", resp.StatusCode, text)
	}
	if _, err := os.Stat(opts.Update.Request); err == nil {
		t.Fatal("the retry asked the helper again")
	}
}

// Two POSTs at once: the second waits for the first's request and start,
// and its starter, whose helper is still at work, answers it with 409.
// Both used to write update-request.tmp together, and the second rename
// could answer 500.
func TestUpdatePostsOneAtATime(t *testing.T) {
	req := filepath.Join(t.TempDir(), "update-request")
	var active, overlaps, starts atomic.Int32
	start := func() error {
		defer active.Add(-1)
		if active.Add(1) > 1 {
			overlaps.Add(1)
		}
		time.Sleep(200 * time.Millisecond)
		if starts.Add(1) > 1 {
			return update.ErrHelperRunning
		}
		return nil
	}
	ts, _ := updateServerWith(t, "secret", UpdateOptions{Request: req, Finder: &fakeFinder{tag: "v0.1.0-alpha.10"}, Start: start})
	codes := make([]int, 2)
	var wg sync.WaitGroup
	for i := range codes {
		wg.Go(func() {
			r, _ := http.NewRequest("POST", ts.URL+"/api/v1/update", strings.NewReader("{}"))
			r.Header.Set("Authorization", "Bearer secret")
			resp, err := http.DefaultClient.Do(r)
			if err != nil {
				t.Error(err)
				return
			}
			resp.Body.Close()
			codes[i] = resp.StatusCode
		})
	}
	wg.Wait()
	slices.Sort(codes)
	if overlaps.Load() != 0 || !slices.Equal(codes, []int{202, 409}) {
		t.Fatalf("two POSTs at once: %v, %d starts overlapping", codes, overlaps.Load())
	}
}

// The channel is MASSIMO_UPDATE_CHANNEL where set, else the store's
// update.channel, else the build's flavour; an update to a release of
// another flavour stores the channel first, so that the box stays on it.
func TestUpdateChannelFromTheStore(t *testing.T) {
	dir := t.TempDir()
	f := &fakeFinder{tag: "v0.2.0-beta.2"}
	opts := storeOptions(t, t.TempDir(), "secret")
	opts.Update = UpdateOptions{Version: "v0.2.0-beta.1", Flavour: "beta", Request: filepath.Join(dir, "update-request"), Status: filepath.Join(dir, "status.json"), Finder: f}
	_, ts, s := startWith(t, opts)
	ctx := context.Background()
	get := func() map[string]any {
		t.Helper()
		resp, text := callWith(t, ts, "GET", "/api/v1/update", "secret", "", nil)
		var out map[string]any
		if resp.StatusCode != 200 || json.Unmarshal([]byte(text), &out) != nil {
			t.Fatalf("GET: %d %s", resp.StatusCode, text)
		}
		return out
	}
	put := func(body string) (int, string) {
		t.Helper()
		resp, text := callWith(t, ts, "PUT", "/api/v1/settings", "secret", body, nil)
		return resp.StatusCode, text
	}

	if out := get(); out["channel"] != "beta" || out["flavour"] != "beta" || f.channel != "beta" {
		t.Fatalf("the flavour's channel: %v, asked %q", out, f.channel)
	}
	code, text := put(`{"update.channel": "alpha"}`)
	var settings map[string]settingView
	if code != 200 || json.Unmarshal([]byte(text), &settings) != nil ||
		settings[store.UpdateChannelKey] != (settingView{Value: "alpha", Default: "beta", Set: true, InEffect: "alpha", Applies: "now"}) {
		t.Fatalf("PUT alpha: %d %s", code, text)
	}
	if out := get(); out["channel"] != "alpha" || f.channel != "alpha" {
		t.Fatalf("the stored channel: %v, asked %q", out, f.channel)
	}
	for _, bad := range []string{`{"update.channel": "main"}`, `{"update.channel": 3}`} {
		if code, text := put(bad); code != 400 || !strings.Contains(text, "want one of devel, nightly, alpha, beta, rc, stable") {
			t.Fatalf("PUT %s: %d %s", bad, code, text)
		}
	}
	if code, text := put(`{"update.channel": null}`); code != 200 {
		t.Fatalf("PUT null: %d %s", code, text)
	}
	if out := get(); out["channel"] != "beta" {
		t.Fatalf("back to the flavour's: %v", out)
	}

	// A release of the beta flavour leaves the store as it is.
	if resp, text := callWith(t, ts, "POST", "/api/v1/update", "secret", "{}", nil); resp.StatusCode != 202 {
		t.Fatalf("POST: %d %s", resp.StatusCode, text)
	}
	if c, _ := s.opts.Store.SettingString(ctx, store.UpdateChannelKey); c != "" {
		t.Fatalf("stored %q for a release of the box's own flavour", c)
	}
	os.Remove(opts.Update.Request)
	// A stable release: the channel beta is stored before the request.
	f.tag, f.flavour = "v0.2.0", "stable"
	if resp, text := callWith(t, ts, "POST", "/api/v1/update", "secret", "{}", nil); resp.StatusCode != 202 {
		t.Fatalf("POST stable: %d %s", resp.StatusCode, text)
	}
	if c, _ := s.opts.Store.SettingString(ctx, store.UpdateChannelKey); c != "beta" {
		t.Fatalf("stored %q before installing a stable release, want beta", c)
	}
	if c, ok, err := update.TakeRequest(opts.Update.Request); err != nil || !ok || c != "beta" {
		t.Fatalf("the request: %q %v %v", c, ok, err)
	}

	// MASSIMO_UPDATE_CHANNEL wins over the store.
	s.opts.Update.Channel = "devel"
	if out := get(); out["channel"] != "devel" || f.channel != "devel" {
		t.Fatalf("the variable's channel: %v, asked %q", out, f.channel)
	}
}
