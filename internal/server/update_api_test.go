// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
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

	"go.metaworx.ch/massimo/internal/notify"
	"go.metaworx.ch/massimo/internal/store"
	"go.metaworx.ch/massimo/internal/testkit"
	"go.metaworx.ch/massimo/internal/update"
)

type fakeFinder struct {
	tag     string // "" = no release
	flavour string // the release's flavour; "" = the channel asked for
	err     error  // where set, the check fails with it
	calls   int
	channel string // the channel last asked for
}

func (f *fakeFinder) Latest(ctx context.Context, channel string) (*update.Release, error) {
	f.calls++
	f.channel = channel
	if f.err != nil {
		return nil, f.err
	}
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
	dir := t.TempDir()
	opts := storeOptions(t, t.TempDir(), "secret")
	opts.Update = UpdateOptions{Version: "v0.1.0-alpha.9", Channel: "alpha", Request: filepath.Join(dir, "update-request"), Status: filepath.Join(dir, "status.json"),
		Finder: &fakeFinder{tag: "v0.1.0-alpha.10"}}
	r := startServer(t, withOptions(opts))
	ts, s := r.ts, r.s
	keepStore(t, ts, s.opts.Store)
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
	t.Parallel()
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
	t.Parallel()
	dir := t.TempDir()
	f := &fakeFinder{tag: "v0.2.0-beta.2"}
	opts := storeOptions(t, t.TempDir(), "secret")
	opts.Update = UpdateOptions{Version: "v0.2.0-beta.1", Flavour: "beta", Request: filepath.Join(dir, "update-request"), Status: filepath.Join(dir, "status.json"), Finder: f}
	r := startServer(t, withOptions(opts))
	ts, s := r.ts, r.s
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

// On a box apt owns massimo on, the channel is the suite apt follows,
// read at each check; a stored channel is the one the next update asks
// the helper to switch apt to, and nothing is stored for it.
func TestUpdateChannelFromAptSuite(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	f := &fakeFinder{tag: "v0.2.0-beta.2"}
	suite := "alpha"
	opts := storeOptions(t, t.TempDir(), "secret")
	opts.Update = UpdateOptions{Version: "v0.2.0-alpha.1", Flavour: "alpha", Manager: "apt", Suite: func() string { return suite },
		Request: filepath.Join(dir, "update-request"), Status: filepath.Join(dir, "status.json"), Finder: f}
	r := startServer(t, withOptions(opts))
	ts, s := r.ts, r.s
	get := func() map[string]any {
		t.Helper()
		resp, text := callWith(t, ts, "GET", "/api/v1/update", "secret", "", nil)
		var out map[string]any
		if resp.StatusCode != 200 || json.Unmarshal([]byte(text), &out) != nil {
			t.Fatalf("GET: %d %s", resp.StatusCode, text)
		}
		return out
	}
	if out := get(); out["channel"] != "alpha" || out["suite"] != "alpha" || f.channel != "alpha" {
		t.Fatalf("the suite's channel: %v, asked %q", out, f.channel)
	}
	suite = "devel"
	if out := get(); out["channel"] != "devel" || out["suite"] != "devel" {
		t.Fatalf("the suite read again: %v", out)
	}
	if resp, text := callWith(t, ts, "PUT", "/api/v1/settings", "secret", `{"update.channel": "beta"}`, nil); resp.StatusCode != 200 {
		t.Fatalf("PUT beta: %d %s", resp.StatusCode, text)
	}
	if out := get(); out["channel"] != "beta" || out["suite"] != "devel" || f.channel != "beta" {
		t.Fatalf("the stored channel beside the suite: %v, asked %q", out, f.channel)
	}
	if resp, text := callWith(t, ts, "POST", "/api/v1/update", "secret", "{}", nil); resp.StatusCode != 202 {
		t.Fatalf("POST: %d %s", resp.StatusCode, text)
	}
	if c, ok, err := update.TakeRequest(opts.Update.Request); err != nil || !ok || c != "beta" {
		t.Fatalf("the request: %q %v %v", c, ok, err)
	}
	// A release of another flavour stores nothing: apt's suite decides.
	if resp, text := callWith(t, ts, "PUT", "/api/v1/settings", "secret", `{"update.channel": null}`, nil); resp.StatusCode != 200 {
		t.Fatalf("PUT null: %d %s", resp.StatusCode, text)
	}
	suite = "alpha"
	if resp, text := callWith(t, ts, "POST", "/api/v1/update", "secret", "{}", nil); resp.StatusCode != 202 {
		t.Fatalf("POST on alpha: %d %s", resp.StatusCode, text)
	}
	if c, _ := s.opts.Store.SettingString(context.Background(), store.UpdateChannelKey); c != "" {
		t.Fatalf("stored %q on a box apt owns", c)
	}
}

// No server test reaches GitHub: the finder of a server whose options
// name none panics, and startServer refuses a channel without a Finder.
func init() {
	defaultFinder = func() ReleaseFinder { return githubGuard{} }
}

// githubGuard is the finder of a server test that gave none.
type githubGuard struct{}

// Latest panics: the test gave the server a channel but no Finder, and
// the check would have asked GitHub.
func (githubGuard) Latest(_ context.Context, channel string) (*update.Release, error) {
	panic("a server test checks the channel " + channel + " on GitHub: give it UpdateOptions.Finder")
}

// The guard is the finder of a server without one, and fails loudly.
func TestNoServerTestReachesGitHub(t *testing.T) {
	t.Parallel()
	s := New(Options{Update: UpdateOptions{Channel: "alpha"}})
	if _, ok := s.finder().(githubGuard); !ok {
		t.Fatalf("a server without a Finder asks %T", s.finder())
	}
	defer func() {
		if recover() == nil {
			t.Fatal("the guard let the check through")
		}
	}()
	s.updateRelease(t.Context(), true)
}

// finderFunc is a ReleaseFinder of a function.
type finderFunc func(ctx context.Context, channel string) (*update.Release, error)

// Latest calls f.
func (f finderFunc) Latest(ctx context.Context, channel string) (*update.Release, error) {
	return f(ctx, channel)
}

// selfCheckServer is a server over a store that runs v0.1.0-alpha.9 on
// alpha and finds f's releases, not running; its rules run on clock (nil
// for the real one) for one user who wants update, and told carries the
// version of each update they tell.
func selfCheckServer(t *testing.T, f ReleaseFinder, clock notify.Clock) (s *Server, told <-chan string) {
	t.Helper()
	s = New(Options{Store: testStore(t, ""), Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		Update: UpdateOptions{Version: "v0.1.0-alpha.9", Channel: "alpha", Finder: f}})
	prefs := notify.Defaults()
	prefs[notify.KindUpdate] = notify.Rule{On: true}
	versions := make(chan string, 16)
	s.notify = notify.New(notify.Options{
		Clock: clock,
		Users: func(context.Context) ([]notify.User, error) { return []notify.User{{ID: 1, Prefs: prefs}}, nil },
		Deliver: func(_ int64, n notify.Notification) {
			if n.Kind == notify.KindUpdate {
				v, _ := n.Values["version"].(string)
				versions <- v
			}
		},
	})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { defer close(done); s.notify.Run(ctx) }()
	t.Cleanup(func() { cancel(); <-done })
	return s, versions
}

// The server checks for a release by itself: at its first tick, six
// hours later and not before, whatever else asks; a check that failed
// again ten minutes later, then six hours after the one that answered.
func TestUpdateSelfCheckTimes(t *testing.T) {
	t.Parallel()
	f := &fakeFinder{tag: "v0.1.0-alpha.10"}
	s, _ := selfCheckServer(t, f, nil)
	start := time.Date(2026, 11, 24, 12, 0, 0, 0, time.UTC)
	tick := func(at time.Time, checks int, what string) {
		t.Helper()
		s.updateTick(t.Context(), at)
		if f.calls != checks {
			t.Fatalf("%s: %d checks, want %d", what, f.calls, checks)
		}
	}
	tick(start, 1, "the first tick")
	if f.channel != "alpha" {
		t.Fatalf("the channel checked: %q", f.channel)
	}
	tick(start.Add(time.Minute), 1, "the next tick")
	tick(start.Add(selfCheckEvery-time.Minute), 1, "a minute before six hours")
	tick(start.Add(selfCheckEvery), 2, "six hours on")
	f.err = errors.New("no network")
	failed := start.Add(2 * selfCheckEvery)
	tick(failed, 3, "the check that failed")
	tick(failed.Add(checkEvery-time.Minute), 3, "a minute before ten minutes")
	f.err = nil
	tick(failed.Add(checkEvery), 4, "ten minutes after the failure")
	tick(failed.Add(checkEvery+selfCheckEvery-time.Minute), 4, "a minute before six hours after it")
	tick(failed.Add(checkEvery+selfCheckEvery), 5, "six hours after it")
}

// Each check that finds a newer release pushes update; the rules tell
// each version once.
func TestUpdateSelfCheckTellsOncePerVersion(t *testing.T) {
	t.Parallel()
	f := &fakeFinder{tag: "v0.1.0-alpha.10"}
	start := time.Date(2026, 11, 24, 12, 0, 0, 0, time.UTC)
	clock := testkit.NewClock(start)
	s, told := selfCheckServer(t, f, kitClock{clock})
	expect := func(want string) {
		t.Helper()
		select {
		case v := <-told:
			if v != want {
				t.Fatalf("told %s, want %s", v, want)
			}
		case <-time.After(testkit.Scale(10 * time.Second)):
			t.Fatalf("%s not told", want)
		}
	}
	s.updateTick(t.Context(), start)
	expect("v0.1.0-alpha.10")
	clock.Add(selfCheckEvery)
	s.updateTick(t.Context(), start.Add(selfCheckEvery))
	f.tag = "v0.1.0-alpha.11"
	clock.Add(selfCheckEvery)
	s.updateTick(t.Context(), start.Add(2*selfCheckEvery))
	if f.calls != 3 {
		t.Fatalf("%d checks, want 3", f.calls)
	}
	// the rules take the pushes in order: the next told is alpha.11's
	// where alpha.10's second push was told nothing
	expect("v0.1.0-alpha.11")
}

// Nothing is checked without a store or without a valid channel; a
// channel set later is checked at the next tick.
func TestUpdateSelfCheckNeedsStoreAndChannel(t *testing.T) {
	t.Parallel()
	f := &fakeFinder{tag: "v0.1.0-alpha.10"}
	discard := slog.New(slog.NewTextHandler(io.Discard, nil))
	now := time.Now()
	New(Options{Logger: discard, Update: UpdateOptions{Version: "v0.1.0-alpha.9", Channel: "alpha", Finder: f}}).updateTick(t.Context(), now)
	if f.calls != 0 {
		t.Fatalf("a server without a store checked %d times", f.calls)
	}
	st := testStore(t, "")
	s := New(Options{Store: st, Logger: discard, Update: UpdateOptions{Version: "v0.1.0-alpha.9", Finder: f}})
	s.updateTick(t.Context(), now)
	if f.calls != 0 {
		t.Fatalf("a server without a channel checked %d times", f.calls)
	}
	if err := st.SetSetting(t.Context(), store.UpdateChannelKey, "beta"); err != nil {
		t.Fatal(err)
	}
	s.updateTick(t.Context(), now.Add(time.Minute))
	if f.calls != 1 || f.channel != "beta" {
		t.Fatalf("the channel set later: %d checks, of %q", f.calls, f.channel)
	}
}

// Run starts the server's own check: its first tick asks the finder,
// and the loop ends with the server.
func TestUpdateSelfCheckRuns(t *testing.T) {
	t.Parallel()
	asked := make(chan string, 64)
	opts := storeOptions(t, t.TempDir(), "secret")
	opts.Update = UpdateOptions{Version: "v0.1.0-alpha.9", Channel: "alpha", Finder: finderFunc(func(_ context.Context, channel string) (*update.Release, error) {
		select {
		case asked <- channel:
		default:
		}
		return nil, nil
	})}
	opts.selfCheckTick = time.Millisecond
	r := startServer(t, withOptions(opts))
	select {
	case c := <-asked:
		if c != "alpha" {
			t.Fatalf("checked %q", c)
		}
	case <-time.After(testkit.Scale(10 * time.Second)):
		t.Fatal("Run checked nothing")
	}
	r.stop()
}

// fakePackages is a package source that has tag, or fails with err; calls
// counts its reads, channel names the last one's channel.
type fakePackages struct {
	tag     string
	err     error
	calls   int
	channel string
}

// read is the source's UpdateOptions.Source.
func (f *fakePackages) read(_ context.Context, channel string) (string, error) {
	f.calls++
	f.channel = channel
	return f.tag, f.err
}

// sourceServer is a server over a store that runs v0.1.0-alpha.9 on
// alpha, finds f's releases and reads src as the box's package source,
// apt's; with the helper's status file.
func sourceServer(t *testing.T, f *fakeFinder, src *fakePackages) (*Server, *httptest.Server, string) {
	t.Helper()
	dir := t.TempDir()
	status := filepath.Join(dir, "status.json")
	s := New(Options{Store: testStore(t, ""), Token: "secret", Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), Update: UpdateOptions{
		Version: "v0.1.0-alpha.9", Channel: "alpha", Manager: "apt", Request: filepath.Join(dir, "update-request"), Status: status,
		Finder: f, Source: src.read,
	}})
	ts := httptest.NewServer(s.Handler())
	t.Cleanup(ts.Close)
	return s, ts, status
}

// A release is available only where the box's package source has it:
// latest is the source's newest, never newer than GitHub's, and
// released GitHub's where the source has it not yet; a source that
// cannot be read makes nothing available, and says why. The source is
// read for the channel, and not where GitHub has nothing newer.
func TestUpdateFromTheSource(t *testing.T) {
	t.Parallel()
	f, src := &fakeFinder{tag: "v0.1.0-alpha.11"}, &fakePackages{}
	_, ts, _ := sourceServer(t, f, src)
	get := func() map[string]any {
		t.Helper()
		var out map[string]any
		if code := getJSON(t, ts, "secret", "/api/v1/update?refresh=1", &out); code != 200 {
			t.Fatalf("GET: %d %v", code, out)
		}
		return out
	}
	for _, c := range []struct {
		source, latest, released string
		available                bool
	}{
		{"v0.1.0-alpha.11", "v0.1.0-alpha.11", "", true},                // level with GitHub
		{"v0.1.0-alpha.10", "v0.1.0-alpha.10", "v0.1.0-alpha.11", true}, // behind, but newer than the running one
		{"v0.1.0-alpha.12", "v0.1.0-alpha.11", "", true},                // never newer than GitHub's
		{"v0.1.0-alpha.9", "v0.1.0-alpha.9", "v0.1.0-alpha.11", false},  // the running one
		{"", "", "v0.1.0-alpha.11", false},                              // none in the index
	} {
		src.tag = c.source
		out := get()
		latest, _ := out["latest"].(string)
		released, _ := out["released"].(string)
		if latest != c.latest || released != c.released || out["available"] != c.available || out["can_update"] != c.available || out["source_error"] != nil {
			t.Errorf("the source at %q: %v", c.source, out)
		}
	}
	if src.channel != "alpha" {
		t.Errorf("the source read for %q", src.channel)
	}
	if code, out := call(t, ts, "POST", "secret"); code != 409 || !strings.Contains(out["error"].(string), "v0.1.0-alpha.11 is out, but apt has it not yet") {
		t.Errorf("POST while apt has nothing newer: %d %v", code, out)
	}

	src.err = errors.New("GET https://massimo.example.invalid/apt/dists/alpha/main/binary-armhf/Packages.gz: 503 Service Unavailable")
	out := get()
	if out["source_error"] != src.err.Error() || out["latest"] != nil || out["available"] != false || out["can_update"] != false {
		t.Errorf("an unreadable source: %v", out)
	}
	if code, out := call(t, ts, "POST", "secret"); code != 502 || !strings.Contains(out["error"].(string), "reading what apt has failed") {
		t.Errorf("POST with an unreadable source: %d %v", code, out)
	}

	src.err, f.tag = nil, "v0.1.0-alpha.9"
	calls := src.calls
	if out := get(); out["latest"] != "v0.1.0-alpha.9" || out["available"] != false || src.calls != calls {
		t.Errorf("nothing newer on GitHub: %v, %d reads", out, src.calls-calls)
	}
}

// After the helper's "nothing newer yet" for a version, it is held back:
// not available, no Update, POST refused, until a check made after the
// helper's answer finds it in the source; a newer release is not held.
func TestUpdateHeldAfterNothingNewer(t *testing.T) {
	t.Parallel()
	f, src := &fakeFinder{tag: "v0.1.0-alpha.10"}, &fakePackages{tag: "v0.1.0-alpha.10"}
	s, ts, status := sourceServer(t, f, src)
	if code, out := call(t, ts, "POST", "secret"); code != 202 {
		t.Fatalf("POST: %d %v", code, out)
	}
	get := func(path string) map[string]any {
		t.Helper()
		var out map[string]any
		if code := getJSON(t, ts, "secret", path, &out); code != 200 {
			t.Fatalf("GET: %d %v", code, out)
		}
		return out
	}
	// apt had nothing newer; the check the POST made is older than that
	ended := time.Now()
	for at := s.updateHistory(t.Context())[0].At; !ended.After(at); ended = time.Now() {
	}
	if err := update.WriteStatus(status, update.Status{State: "up-to-date", Channel: "alpha", Message: "apt has nothing newer yet", Time: ended}); err != nil {
		t.Fatal(err)
	}
	if out := get("/api/v1/update"); out["held"] != "v0.1.0-alpha.10" || out["available"] != false || out["can_update"] != false || out["latest"] != "v0.1.0-alpha.10" {
		t.Fatalf("held: %v", out)
	}
	// a later check that does not find it keeps it held, and POST, which
	// checks again, is refused
	src.tag = "v0.1.0-alpha.9"
	if out := get("/api/v1/update?refresh=1"); out["held"] != "v0.1.0-alpha.10" || out["available"] != false {
		t.Fatalf("held after a check without it: %v", out)
	}
	if code, out := call(t, ts, "POST", "secret"); code != 409 || !strings.Contains(out["error"].(string), "v0.1.0-alpha.10 is held back") {
		t.Fatalf("POST while held: %d %v", code, out)
	}
	// a check that finds it ends the hold
	src.tag = "v0.1.0-alpha.10"
	if out := get("/api/v1/update?refresh=1"); out["held"] != nil || out["available"] != true || out["can_update"] != true {
		t.Fatalf("after a check that finds it: %v", out)
	}
	// a newer release is not held by an older version's hold
	s.holdVersion("v0.1.0-alpha.10", time.Now().Add(time.Hour))
	f.tag, src.tag = "v0.1.0-alpha.11", "v0.1.0-alpha.11"
	if out := get("/api/v1/update?refresh=1"); out["held"] != nil || out["latest"] != "v0.1.0-alpha.11" || out["available"] != true {
		t.Fatalf("a newer release: %v", out)
	}
}

// A hand install has no source: GitHub's release is offered as it is,
// and a hold ends at the next check after it, which finds it there.
func TestUpdateWithoutSource(t *testing.T) {
	t.Parallel()
	f := &fakeFinder{tag: "v0.1.0-alpha.10"}
	ts, _ := updateServer(t, "secret", filepath.Join(t.TempDir(), "update-request"), f)
	code, out := call(t, ts, "GET", "secret")
	if code != 200 || out["latest"] != "v0.1.0-alpha.10" || out["released"] != nil || out["source_error"] != nil || out["available"] != true {
		t.Fatalf("GET: %d %v", code, out)
	}
}

// A check whose source could not be read is tried again after
// checkEvery, as one that failed.
func TestUpdateSelfCheckSourceFailed(t *testing.T) {
	t.Parallel()
	src := &fakePackages{err: errors.New("no network")}
	s := New(Options{Store: testStore(t, ""), Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		Update: UpdateOptions{Version: "v0.1.0-alpha.9", Channel: "alpha", Manager: "apt", Finder: &fakeFinder{tag: "v0.1.0-alpha.10"}, Source: src.read}})
	start := time.Date(2026, 11, 24, 12, 0, 0, 0, time.UTC)
	s.updateTick(t.Context(), start)
	if src.calls != 1 || !s.updateNext.Equal(start.Add(checkEvery)) {
		t.Fatalf("%d reads, next check at %v", src.calls, s.updateNext)
	}
}
