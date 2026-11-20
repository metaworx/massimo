// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package main

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"

	"go.metaworx.ch/massimo/internal/env"
	"go.metaworx.ch/massimo/internal/server"
	"go.metaworx.ch/massimo/internal/testkit"
	"go.metaworx.ch/massimo/internal/testkit/storekit"
)

// What SIGHUP does: a reload under a service manager and after nohup, a
// stop in a terminal, unless -sighup says.
func TestSighupAction(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		mode             string
		managed, ignored bool
		want             string
	}{
		{"", true, false, sighupReload}, {"auto", false, true, sighupReload}, {"", false, false, sighupStop},
		{"reload", false, false, sighupReload}, {"stop", true, false, sighupStop},
	} {
		if got, err := sighupAction(c.mode, c.managed, c.ignored); err != nil || got != c.want {
			t.Errorf("%q managed %v ignored %v: %q %v, want %q", c.mode, c.managed, c.ignored, got, err, c.want)
		}
	}
	if _, err := sighupAction("ignore", false, false); err == nil || !strings.Contains(err.Error(), "want auto, reload or stop") {
		t.Fatalf("a bad -sighup: %v", err)
	}
}

// A SIGHUP before serve hands over its reload is logged at Warn and
// dropped; afterwards it runs the reload, and logs nothing of its own.
func TestHupReloadBeforeReady(t *testing.T) {
	t.Parallel()
	log, w := testkit.LogWatch(nil)
	h := &hupReload{log: log}
	h.handle()
	w.Has(t, 0, "before ready: the warning, with its reason", testkit.ContainsAt(slog.LevelWarn, "SIGHUP during the start, ignored", "reason="))
	m := w.Mark()
	runs := 0
	h.ready(func() { runs++ })
	h.handle()
	h.handle()
	if runs != 2 {
		t.Fatalf("after ready: %d runs", runs)
	}
	w.HasNot(t, m, "after ready: a line of its own", nil)
}

// The watch reads a changed file once it held still for one more look,
// once; a missing file and an unchanged one ask for nothing.
func TestWatchSettings(t *testing.T) {
	t.Parallel()
	p := filepath.Join(t.TempDir(), "env")
	// put writes the file whole, renamed into place: os.WriteFile
	// truncates first and writes after, and a busy machine that pauses
	// between the two for two of the test's 20 ms looks shows the watch
	// an empty file that held still, then the content, two reloads.
	put := func(text string) {
		t.Helper()
		if err := os.WriteFile(p+".new", []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.Rename(p+".new", p); err != nil {
			t.Fatal(err)
		}
	}
	put("MASSIMO_LOG=a\n")
	calls := make(chan struct{}, 10)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go watchSettings(ctx, p, 20*time.Millisecond, func() { calls <- struct{}{} })
	quiet := func(what string) {
		t.Helper()
		select {
		case <-calls:
			t.Fatalf("%s: a reload", what)
		case <-time.After(150 * time.Millisecond):
		}
	}
	quiet("an unchanged file")
	put("MASSIMO_LOG=bb\n")
	select {
	case <-calls:
	case <-time.After(testkit.Scale(2 * time.Second)):
		t.Fatal("no reload after a change")
	}
	quiet("after the reload")
	os.Remove(p)
	quiet("a missing file")
	put("MASSIMO_LOG=ccc\n")
	select {
	case <-calls:
	case <-time.After(testkit.Scale(2 * time.Second)):
		t.Fatal("no reload after the file came back changed")
	}
}

// unsetForTest unsets variables for the test and restores them after it.
func unsetForTest(t *testing.T, names ...string) {
	t.Helper()
	for _, n := range names {
		t.Setenv(n, "")
		os.Unsetenv(n)
	}
}

// The reloader applies a token the file gained, unless -token fixed it,
// and lists what else applies at the next start: not the Cube keys, not
// what a flag fixed.
func TestSettingsReloader(t *testing.T) {
	unsetForTest(t, tokenVar, env.TrustedProxies, "MASSIMO_LOG", "MASSIMO_CUBE_KEQ0000100_KEY")
	p := filepath.Join(t.TempDir(), "env")
	os.WriteFile(p, nil, 0o600)
	t.Cleanup(func() { os.WriteFile(p, nil, 0o600); env.ReloadFile(p) }) // the env package forgets the test's names
	st := storekit.Open(t, "")
	srv := server.New(server.Options{Store: st, Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), Restart: server.RestartOptions{By: "systemd", Restart: func() error { return nil }}})
	r := newSettingsReloader(p, slog.New(slog.NewTextHandler(io.Discard, nil)), srv, settings(), map[string]bool{"log": true})
	if srv.AuthMode() != "open" {
		t.Fatalf("mode %s", srv.AuthMode())
	}
	os.WriteFile(p, []byte(tokenVar+"=placeholder-token-1\n"+env.TrustedProxies+"=127.0.0.1\nMASSIMO_LOG=/tmp/x.log\nMASSIMO_CUBE_KEQ0000100_KEY=00\n"), 0o600)
	r.reload(t.Context(), "test")
	if srv.AuthMode() != "users" {
		t.Fatalf("the token was not applied: mode %s", srv.AuthMode())
	}
	if got := strings.Join(r.pending(), ","); got != env.TrustedProxies {
		t.Fatalf("pending %q", got)
	}
	req := httptest.NewRequest("GET", "/api/v1/restart", nil)
	req.RemoteAddr = "127.0.0.1:5000"
	req.Header.Set("Authorization", "Bearer placeholder-token-1")
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	var v struct {
		Pending []string `json:"pending"`
	}
	if json.Unmarshal(rec.Body.Bytes(), &v); rec.Code != 200 || strings.Join(v.Pending, ",") != env.TrustedProxies {
		t.Fatalf("GET /api/v1/restart: %d %s", rec.Code, rec.Body.String())
	}

	// With -token, the file's token does not count.
	st2 := storekit.Open(t, "")
	srv2 := server.New(server.Options{Store: st2, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	os.WriteFile(p, nil, 0o600)
	env.ReloadFile(p)
	r2 := newSettingsReloader(p, slog.New(slog.NewTextHandler(io.Discard, nil)), srv2, settings(), map[string]bool{"token": true})
	os.WriteFile(p, []byte(tokenVar+"=placeholder-token-2\n"), 0o600)
	r2.reload(t.Context(), "test")
	if srv2.AuthMode() != "open" || len(r2.pending()) != 0 {
		t.Fatalf("with -token: mode %s, pending %v", srv2.AuthMode(), r2.pending())
	}
}

// startServe runs serve from the built massimo with the settings file and
// the arguments, a store of its own; it returns once the API listens.
func startServe(t *testing.T, settingsFile, data string, args ...string) *builtServe {
	t.Helper()
	cmd := exec.Command(builtBinary(t), append([]string{"-env", settingsFile, "-data", data, "serve", "-listen", "127.0.0.1:0", "-discover-interval", "0"}, args...)...)
	for _, kv := range os.Environ() {
		if !strings.HasPrefix(kv, "MASSIMO_") && !strings.HasPrefix(kv, "INVOCATION_ID=") {
			cmd.Env = append(cmd.Env, kv)
		}
	}
	s := runBuilt(t, cmd)
	s.data = data
	r, _ := s.log.Await(t, 0, "the start: api listening", testkit.Msg("api listening", "source", "system"), serveWait)
	s.addr = r.Attrs["addr"]
	return s
}

func get(t *testing.T, url, token string) (int, string) {
	t.Helper()
	req, _ := http.NewRequest("GET", url, nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(b)
}

// The case on jaguar, end to end: an open server whose settings file
// gains MASSIMO_API_TOKEN asks for it after a SIGHUP, without a restart,
// lists a setting that waits for the next start, and checkpoints its
// store.
func TestServeAppliesTheTokenOnSIGHUP(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("no SIGHUP")
	}
	if testing.Short() {
		t.Skip("builds the binary")
	}
	settingsFile := filepath.Join(t.TempDir(), "env")
	os.WriteFile(settingsFile, []byte("# no token yet\n"), 0o600)
	s := startServe(t, settingsFile, t.TempDir(), "-sighup", "reload")
	base := "http://" + s.addr
	if code, text := get(t, base+"/api/v1/cubes", ""); code != 200 {
		t.Fatalf("the open server: %d %s", code, text)
	}
	os.WriteFile(settingsFile, []byte(tokenVar+"=placeholder-token-1\n"+env.TrustedProxies+"=127.0.0.1\n"), 0o600)
	m := s.log.Mark()
	s.cmd.Process.Signal(syscall.SIGHUP)
	if !testkit.Within(5*time.Second, func() bool {
		code, _ := get(t, base+"/api/v1/cubes", "")
		return code == 401
	}) {
		t.Fatal("the server does not ask for the token after the SIGHUP") // serve's log follows
	}
	if code, text := get(t, base+"/api/v1/cubes", "placeholder-token-1"); code != 200 {
		t.Fatalf("with the token: %d %s", code, text)
	}
	if code, text := get(t, base+"/api/v1/restart", "placeholder-token-1"); code != 200 || !strings.Contains(text, `"pending": [`) || !strings.Contains(text, env.TrustedProxies) {
		t.Fatalf("GET /api/v1/restart: %d %s", code, text)
	}
	// Once serve ended, its log is read whole; the token first, which no
	// line shows.
	s.stop(t)
	s.log.HasNot(t, 0, "the token in the log", testkit.Contains("placeholder-token-1"))
	s.log.Has(t, m, "the settings file read again for the SIGHUP, with what changed",
		testkit.Contains(`msg="settings file read again"`, "source=system", "why=SIGHUP", "changed="))
	s.log.Has(t, m, "the token applied, its owner created", testkit.Msg("api token applied", "source", "server", "owner_created", "true"))
	s.log.Has(t, m, "the store checkpointed", testkit.Msg("store checkpointed"))
}

// With -sighup stop, a SIGHUP stops serve as SIGTERM does: exit 0 and
// the store closed.
func TestServeStopsOnSIGHUP(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("no SIGHUP")
	}
	if testing.Short() {
		t.Skip("builds the binary")
	}
	data := t.TempDir()
	s := startServe(t, os.DevNull, data, "-sighup", "stop")
	s.cmd.Process.Signal(syscall.SIGHUP)
	select {
	case <-s.exited:
		if s.err != nil {
			t.Fatalf("serve ended on SIGHUP with %v", s.err)
		}
	case <-time.After(testkit.Scale(20 * time.Second)):
		t.Fatal("serve did not stop on SIGHUP")
	}
	if _, err := os.Stat(filepath.Join(data, "massimo.db-wal")); err == nil {
		t.Fatal("the store was not closed") // serve's log follows
	}
}
