// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package main

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"go.metaworx.ch/massimo/internal/env"
	"go.metaworx.ch/massimo/internal/server"
	"go.metaworx.ch/massimo/internal/store"
)

// What SIGHUP does: a reload under a service manager and after nohup, a
// stop in a terminal, unless -sighup says.
func TestSighupAction(t *testing.T) {
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

// The watch reads a changed file once it held still for one more look,
// once; a missing file and an unchanged one ask for nothing.
func TestWatchSettings(t *testing.T) {
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
	case <-time.After(2 * time.Second):
		t.Fatal("no reload after a change")
	}
	quiet("after the reload")
	os.Remove(p)
	quiet("a missing file")
	put("MASSIMO_LOG=ccc\n")
	select {
	case <-calls:
	case <-time.After(2 * time.Second):
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
	st, err := store.Open(t.TempDir(), store.Options{Version: "test", Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
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
	st2, err := store.Open(t.TempDir(), store.Options{Version: "test", Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	if err != nil {
		t.Fatal(err)
	}
	defer st2.Close()
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

// startServe builds massimo and runs serve with the settings file and
// the arguments, a store of its own; it returns the command, the API's
// base URL and the log so far.
func startServe(t *testing.T, settingsFile, data string, args ...string) (*exec.Cmd, string, func() string, chan struct{}) {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "massimo")
	if out, err := exec.Command("go", "build", "-o", bin, ".").CombinedOutput(); err != nil {
		t.Fatalf("go build: %v\n%s", err, out)
	}
	cmd := exec.Command(bin, append([]string{"-env", settingsFile, "-data", data, "serve", "-listen", "127.0.0.1:0", "-discover-interval", "0"}, args...)...)
	for _, kv := range os.Environ() {
		if !strings.HasPrefix(kv, "MASSIMO_") && !strings.HasPrefix(kv, "INVOCATION_ID=") {
			cmd.Env = append(cmd.Env, kv)
		}
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cmd.Process.Kill() })
	addr := make(chan string, 1)
	var mu sync.Mutex
	var log strings.Builder
	scanned := make(chan struct{})
	go func() {
		defer close(scanned)
		sc := bufio.NewScanner(stderr)
		for sc.Scan() {
			mu.Lock()
			log.WriteString(sc.Text() + "\n")
			mu.Unlock()
			if m := regexp.MustCompile(`msg="api listening" source=system addr=(\S+)`).FindStringSubmatch(sc.Text()); m != nil {
				addr <- m[1]
			}
		}
	}()
	select {
	case a := <-addr:
		return cmd, "http://" + a, func() string { mu.Lock(); defer mu.Unlock(); return log.String() }, scanned
	case <-time.After(10 * time.Second):
		t.Fatal("serve did not start")
	}
	return nil, "", nil, nil
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
	if runtime.GOOS == "windows" {
		t.Skip("no SIGHUP")
	}
	if testing.Short() {
		t.Skip("builds the binary")
	}
	settingsFile := filepath.Join(t.TempDir(), "env")
	os.WriteFile(settingsFile, []byte("# no token yet\n"), 0o600)
	cmd, base, log, scanned := startServe(t, settingsFile, t.TempDir(), "-sighup", "reload")
	if code, text := get(t, base+"/api/v1/cubes", ""); code != 200 {
		t.Fatalf("the open server: %d %s", code, text)
	}
	os.WriteFile(settingsFile, []byte(tokenVar+"=placeholder-token-1\n"+env.TrustedProxies+"=127.0.0.1\n"), 0o600)
	cmd.Process.Signal(syscall.SIGHUP)
	deadline := time.Now().Add(5 * time.Second)
	for {
		if code, _ := get(t, base+"/api/v1/cubes", ""); code == 401 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("the server does not ask for the token after the SIGHUP; log:\n%s", log())
		}
		time.Sleep(50 * time.Millisecond)
	}
	if code, text := get(t, base+"/api/v1/cubes", "placeholder-token-1"); code != 200 {
		t.Fatalf("with the token: %d %s", code, text)
	}
	if code, text := get(t, base+"/api/v1/restart", "placeholder-token-1"); code != 200 || !strings.Contains(text, `"pending": [`) || !strings.Contains(text, env.TrustedProxies) {
		t.Fatalf("GET /api/v1/restart: %d %s", code, text)
	}
	cmd.Process.Signal(syscall.SIGTERM)
	if err := cmd.Wait(); err != nil {
		t.Fatalf("serve ended with %v", err)
	}
	<-scanned
	for _, want := range []string{`msg="settings file read again" source=system why=SIGHUP changed=`, `msg="api token applied" source=server owner_created=true`, `msg="store checkpointed"`} {
		if !strings.Contains(log(), want) {
			t.Errorf("the log lacks %s:\n%s", want, log())
		}
	}
	if strings.Contains(log(), "placeholder-token-1") {
		t.Error("the log shows the token")
	}
}

// With -sighup stop, a SIGHUP stops serve as SIGTERM does: exit 0 and
// the store closed.
func TestServeStopsOnSIGHUP(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("no SIGHUP")
	}
	if testing.Short() {
		t.Skip("builds the binary")
	}
	data := t.TempDir()
	cmd, _, log, _ := startServe(t, os.DevNull, data, "-sighup", "stop")
	cmd.Process.Signal(syscall.SIGHUP)
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("serve ended on SIGHUP with %v", err)
		}
	case <-time.After(20 * time.Second):
		t.Fatal("serve did not stop on SIGHUP")
	}
	if _, err := os.Stat(filepath.Join(data, "massimo.db-wal")); err == nil {
		t.Fatalf("the store was not closed; log:\n%s", log())
	}
}
