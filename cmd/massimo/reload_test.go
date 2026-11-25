// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package main

import (
	"cmp"
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
	"slices"
	"strings"
	"syscall"
	"testing"
	"time"

	"go.metaworx.ch/massimo/internal/env"
	"go.metaworx.ch/massimo/internal/server"
	"go.metaworx.ch/massimo/internal/store"
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
	if _, err := sighupAction("ignore", false, false); err == nil || err.Error() != `-sighup or MASSIMO_SIGHUP "ignore": want auto, reload or stop` {
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

// Every variable behind a flag of serve is one env.Known knows, so that
// massimo config does not call it a typo and a reload can list a change
// of it as waiting for the next start (settings); MASSIMO_RESTART and
// MASSIMO_SIGHUP were not known.
func TestFlagVariablesAreKnown(t *testing.T) {
	t.Parallel()
	for f, v := range flagVars {
		if !env.Known(v) {
			t.Errorf("-%s: %s unknown to massimo config and to a reload", f, v)
		}
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
	r := newSettingsReloader(p, slog.New(slog.NewTextHandler(io.Discard, nil)), srv, settings(), fixedSettings(map[string]string{"log": "/tmp/x.log"}), nil)
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
	r2 := newSettingsReloader(p, slog.New(slog.NewTextHandler(io.Discard, nil)), srv2, settings(), fixedSettings(map[string]string{"token": "placeholder-token-0"}), nil)
	os.WriteFile(p, []byte(tokenVar+"=placeholder-token-2\n"), 0o600)
	r2.reload(t.Context(), "test")
	if srv2.AuthMode() != "open" || len(r2.pending()) != 0 {
		t.Fatalf("with -token: mode %s, pending %v", srv2.AuthMode(), r2.pending())
	}
}

// Every setting the start refuses (startChecks), changed in the settings
// file to a value the start refuses, makes the reload's settings_read a
// warning with the start's own error after "the next start refuses: ":
// one line, on the process's log as in the server's, where the log's
// warning sign counts it; the start, with the same settings and flags,
// stops with that error after "serve: ". The value still waits for the
// next start, the running level stays, and the reboot switch is off. A
// check runs where a setting it reads differs from the start and no flag
// fixed it, and reads a fixed flag's value; a secret's value is named by
// its variable, and the checks touch nothing. A good change warns of
// nothing and settles the warning. POST /api/v1/reload and GET
// /api/v1/restart answer the same, each refusal with the settings that
// changed and that its check reads, and a good change clears it (AP
// ReloadChecks, D8-B).
func TestReloadWarnsOfARefusedSetting(t *testing.T) {
	clearCubeEnv(t)
	resetLogLevel(t)
	for _, c := range startChecks {
		for _, v := range c.vars {
			if !strings.HasSuffix(v, "_") {
				unsetForTest(t, v)
			}
		}
	}
	unsetForTest(t, tokenVar, "MASSIMO_LOG", "MASSIMO_LISTEN", "MASSIMO_UPDATE_REQUEST", env.ACMEEmail, env.ACMEDirectory)
	data := os.Getenv(env.DataVar) // clearCubeEnv's, which no check may write to
	dir := t.TempDir()
	p := filepath.Join(dir, "env")
	os.WriteFile(p, nil, 0o600)
	t.Cleanup(func() { os.WriteFile(p, nil, 0o600); env.ReloadFile(p) }) // the env package forgets the test's names
	st := storekit.Open(t, "")
	// The log's writer, which the server's Run would start.
	wctx, stopWriter := context.WithCancel(context.Background())
	writerDone := make(chan struct{})
	go func() { st.RunEvents(wctx); close(writerDone) }()
	t.Cleanup(func() { stopWriter(); <-writerDone })
	// One process log, as serve's: the reloader's lines and the server's.
	log, w := testkit.LogWatch(nil)
	sysLog := log.With("source", "system")
	srv := server.New(server.Options{Store: st, Logger: log, Reboot: server.RebootOptions{By: bySystemd, Reboot: func() error { t.Error("rebooted"); return nil }}})
	apiCall := func(method, path string, v any) {
		t.Helper()
		req := httptest.NewRequest(method, path, strings.NewReader("{}"))
		req.RemoteAddr = "127.0.0.1:5000"
		rec := httptest.NewRecorder()
		srv.Handler().ServeHTTP(rec, req)
		if rec.Code != 200 || json.Unmarshal(rec.Body.Bytes(), v) != nil {
			t.Fatalf("%s %s: %d %s", method, path, rec.Code, rec.Body.String())
		}
	}
	apiGet := func(path string, v any) {
		t.Helper()
		apiCall("GET", path, v)
	}
	rebootAvailable := func() bool {
		t.Helper()
		var v struct {
			Available bool `json:"available"`
		}
		apiGet("/api/v1/reboot", &v)
		return v.Available
	}
	reads := 0 // the settings_read lines the reloads wrote, one each
	warned := func(r testkit.Record) bool { return r.Level >= slog.LevelWarn }
	// reload writes the settings file and reads it again with r, through
	// POST /api/v1/reload as serve hands r to the server; then checks the
	// process log, the server's newest settings_read and the log's
	// warning sign: one warning, the event's line, with want after "the
	// next start refuses: " as its error, or for want "" none. The
	// reload's answer and GET /api/v1/restart say the same refusals, whose
	// texts joined are want, and which it returns: none for want "", the
	// reload's answer then [].
	reload := func(r *settingsReloader, what, text, want string) []server.Refusal {
		t.Helper()
		m := w.Mark()
		if err := os.WriteFile(p, []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
		srv.SetReloader(p, r.apiReload(func(context.Context) {}))
		var answer struct {
			Refused *[]server.Refusal `json:"refused"`
		}
		apiCall("POST", "/api/v1/reload", &answer)
		reads++
		var after struct {
			Refused []server.Refusal `json:"refused"`
		}
		apiGet("/api/v1/restart", &after)
		var texts []string
		for _, f := range after.Refused {
			texts = append(texts, f.Error)
		}
		if answer.Refused == nil || !slices.EqualFunc(*answer.Refused, after.Refused, sameRefusal) || strings.Join(texts, "; ") != want {
			t.Fatalf("%s: the reload's refused %+v, GET /api/v1/restart's %+v, want %q", what, answer.Refused, after.Refused, want)
		}
		var lines []struct{ Level, Error string }
		testkit.Eventually(t, what+": the settings_read in the server's log", func() bool {
			lines = nil
			apiGet("/api/v1/log?kind=settings_read", &lines)
			return len(lines) == reads
		})
		var sign struct {
			Level string `json:"level"`
		}
		apiGet("/api/v1/log/alerts", &sign)
		l := lines[len(lines)-1]
		if want == "" {
			w.HasNot(t, m, what+": a warning", warned)
			if l.Level != "info" || l.Error != "" || sign.Level != "" {
				t.Fatalf("%s: the line %+v, the sign %q", what, l, sign.Level)
			}
			return nil
		}
		want = "the next start refuses: " + want
		rec, _ := w.Has(t, m, what+": the settings_read line at warn", testkit.MsgAt(slog.LevelWarn, "settings read", "kind", "settings_read"))
		if rec.Attrs["error"] != want {
			t.Fatalf("%s: the line's error\n%q, want\n%q", what, rec.Attrs["error"], want)
		}
		if n := w.Count(m, warned); n != 1 {
			t.Fatalf("%s: %d warnings, want the event's line alone: %v", what, n, w.Since(m, warned))
		}
		if l.Level != "warn" || l.Error != want || sign.Level != "warn" {
			t.Fatalf("%s: the line %+v, the sign %q", what, l, sign.Level)
		}
		return after.Refused
	}
	// names checks that the refusals name the settings of want, one list
	// each, in the start's order.
	names := func(what string, refused []server.Refusal, want ...[]string) {
		t.Helper()
		if !slices.EqualFunc(refused, want, func(f server.Refusal, set []string) bool { return slices.Equal(f.Settings, set) }) {
			t.Fatalf("%s: the refusals %+v, want the settings %v", what, refused, want)
		}
	}
	// refusedAtStart runs the start in the settings the reload left and
	// with the flags given: it stops with want after "serve: ", before it
	// opens anything. -addr opens no store, and -listen's port is out of
	// range, so that a start the checks let through fails at its listener
	// instead of serving.
	refusedAtStart := func(what string, flags map[string]string, want string) {
		t.Helper()
		args := []string{"-addr", "127.0.0.1:1", "-listen", "127.0.0.1:99999", "-proxy", "off", "-discover-interval", "0"}
		for f, v := range flags {
			args = append(args, "-"+f+"="+v)
		}
		if err := runServe(args); err == nil || err.Error() != "serve: "+want {
			t.Fatalf("%s: the start stopped with\n%v, want\nserve: %s", what, err, want)
		}
	}
	covered := map[string]bool{} // the checks a case refuses, by their first variable
	r := newSettingsReloader(p, sysLog, srv, settings(), nil, nil)
	level := logLevel.Level()

	reload(r, "a good change of every check's, none for stderr alone",
		env.LogLevel+"=Debug\n"+env.LogFormat+"=json\n"+env.LogFileFormat+"=plain\n"+env.LogProcessFormat+"=none\n"+
			env.LogRotateSize+"=20M\n"+env.LogKeep+"=3\n"+env.LogKeepMin+"=1M\n"+env.HSTS+"=30\n"+env.TrustedProxies+"=127.0.0.1\n"+
			restartVar+"=exit\n"+sighupVar+"=reload\n"+rebootVar+"=on\n"+env.Announce+"=off\n"+env.TLS+"=self-signed\n"+
			"MASSIMO_CUBE_KEQ0000000_ADDRESS=192.0.2.1\nMASSIMO_CUBE_KEQ0000000_IGNORE=0a0101\n", "")
	if !rebootAvailable() {
		t.Fatal("a good change: the reboot not switched on")
	}
	reload(r, "the reboot switch's bad value", rebootVar+"=maybe\n", `MASSIMO_REBOOT "maybe": want on or off`)
	if rebootAvailable() {
		t.Fatal("the reboot switch's bad value: the reboot still on")
	}
	refusedAtStart("the reboot switch's bad value", nil, `MASSIMO_REBOOT "maybe": want on or off`)
	covered[rebootVar] = true

	// One bad value per check, the TLS checks with several.
	missingHook := filepath.Join(dir, "none.sh")
	_, missingHookErr := os.Stat(missingHook)
	noExec := filepath.Join(dir, "noexec.sh")
	os.WriteFile(noExec, []byte("#!/bin/sh\n"), 0o644)
	hook := filepath.Join(dir, "hook.sh")
	os.WriteFile(hook, []byte("#!/bin/sh\n"), 0o755)
	cases := []struct{ check, what, text, want string }{
		{env.LogFormat, "a bad format", env.LogFormat + "=xml\n", `-log-format or MASSIMO_LOG_FORMAT: unknown log format "xml", want json or plain`},
		{env.LogFileFormat, "none in the file", env.LogFileFormat + "=none\n", `-log-file-format or MASSIMO_LOG_FILE_FORMAT: unknown log format "none", want json or plain`},
		{env.LogProcessFormat, "a bad format on stderr", env.LogProcessFormat + "=yaml\n", `-log-process-format or MASSIMO_LOG_PROCESS_FORMAT: unknown log format "yaml", want json, plain or none`},
		{env.LogRotateSize, "a bad rotation size", env.LogRotateSize + "=big\n", `-log-rotate-size or MASSIMO_LOG_ROTATE_SIZE: unknown size "big", want bytes or a number with k, M or G`},
		{env.LogKeep, "a bad number of files", env.LogKeep + "=many\n", `-log-keep or MASSIMO_LOG_KEEP: "many" is no number of files`},
		{env.LogKeepMin, "a bad size to keep", env.LogKeepMin + "=-1\n", `-log-keep-min or MASSIMO_LOG_KEEP_MIN: unknown size "-1", want bytes or a number with k, M or G`},
		{env.LogLevel, "a bad level", env.LogLevel + "=deubg\n", `-log-level or MASSIMO_LOG_LEVEL: unknown log level "deubg", want debug, info, warn or error`},
		{env.HSTS, "a bad HSTS", env.HSTS + "=-1\n", `-hsts or MASSIMO_HSTS "-1": want a number of days, 0 or more`},
		{env.TrustedProxies, "a bad proxy", env.TrustedProxies + "=10.0.0.300\n", `-trusted-proxies or MASSIMO_TRUSTED_PROXIES: "10.0.0.300" is neither an address nor a CIDR`},
		{restartVar, "a bad restart", restartVar + "=always\n", `-restart or MASSIMO_RESTART "always": want auto, exit or off`},
		{sighupVar, "a bad SIGHUP", sighupVar + "=ignore\n", `-sighup or MASSIMO_SIGHUP "ignore": want auto, reload or stop`},
		{env.Announce, "a bad announcement", env.Announce + "=maybe\n", `MASSIMO_ANNOUNCE "maybe": want on or off`},
		{env.TLS, "a bad TLS mode", env.TLS + "=yes\n", `-tls or MASSIMO_TLS "yes": want off, files, self-signed or acme`},
		{env.TLS, "files without the files", env.TLS + "=files\n", "-tls files needs -tls-cert and -tls-key (MASSIMO_TLS_CERT and MASSIMO_TLS_KEY)"},
		{env.TLS, "acme without names", env.TLS + "=acme\n", "tls: -tls acme needs the names of -tls-names"},
		{env.TLS, "a wildcard without DNS-01", env.TLS + "=acme\n" + env.TLSNames + "=*.example.net\n", "tls: a wildcard (*.example.net) needs the DNS-01 challenge (-acme-dns-hook)"},
		{env.TLS, "a DNS hook that is not there", env.TLS + "=acme\n" + env.TLSNames + "=*.example.net\n" + env.ACMEDNSHook + "=" + missingHook + "\n",
			"tls: -acme-dns-hook: " + missingHookErr.Error()},
		{env.Prefix, "a Cube's bad _IGNORE", "MASSIMO_CUBE_KEQ0000000_ADDRESS=192.0.2.1\nMASSIMO_CUBE_KEQ0000000_IGNORE=zz\n",
			`MASSIMO_CUBE_KEQ0000000_IGNORE: rf address "zz": want 6 hex digits`},
	}
	if runtime.GOOS != "windows" { // which checks no execute bit
		cases = append(cases, struct{ check, what, text, want string }{env.TLS, "a DNS hook that is not executable",
			env.TLS + "=acme\n" + env.TLSNames + "=*.example.net\n" + env.ACMEDNSHook + "=" + noExec + "\n", "tls: -acme-dns-hook " + noExec + " is not executable"})
	}
	for _, c := range cases {
		refused := reload(r, c.what, c.text, c.want)
		var set []string // the settings of the text, which the start had none of, all read by the check
		for _, line := range strings.Fields(c.text) {
			k, _, _ := strings.Cut(line, "=")
			if !slices.Contains(r.pending(), k) {
				t.Fatalf("%s: %s does not wait for the next start: pending %v", c.what, k, r.pending())
			}
			set = append(set, k)
		}
		slices.Sort(set)
		names(c.what, refused, set)
		refusedAtStart(c.what, nil, c.want)
		covered[c.check] = true
	}
	if logLevel.Level() != level {
		t.Fatalf("the running level %v, want %v", logLevel.Level(), level)
	}
	for _, c := range startChecks {
		if !covered[c.vars[0]] {
			t.Errorf("no case refused by the check of %v", c.vars)
		}
	}
	formats := reload(r, "three bad formats, in the start's order",
		env.LogFileFormat+"=none\n"+env.LogFormat+"=xml\n"+env.LogProcessFormat+"=yaml\n",
		`-log-format or MASSIMO_LOG_FORMAT: unknown log format "xml", want json or plain; `+
			`-log-file-format or MASSIMO_LOG_FILE_FORMAT: unknown log format "none", want json or plain; `+
			`-log-process-format or MASSIMO_LOG_PROCESS_FORMAT: unknown log format "yaml", want json, plain or none`)
	names("three bad formats", formats, []string{env.LogFormat}, []string{env.LogFileFormat}, []string{env.LogProcessFormat})
	reload(r, "good again, which settles the warnings", env.LogLevel+"=warn\n", "")

	// The key's path is named by its variable, as GET /api/v1/config
	// hides its value; the start's own error keeps the path.
	certFile, keyFile := filepath.Join(dir, "cert.pem"), filepath.Join(dir, "missing-key.pem")
	os.WriteFile(certFile, []byte("not read: the key is missing\n"), 0o600)
	_, keyErr := os.Stat(keyFile)
	reload(r, "a key that is not there", env.TLS+"=files\n"+env.TLSCert+"="+certFile+"\n"+env.TLSKey+"="+keyFile+"\n",
		"tls: "+strings.ReplaceAll(keyErr.Error(), keyFile, env.TLSKey))
	refusedAtStart("a key that is not there", nil, "tls: "+keyErr.Error())
	w.HasNot(t, 0, "the key's path in the log", testkit.Contains(keyFile))
	// A short path, which the variable's name holds itself, is named once.
	_, shortErr := os.Stat("KEY")
	reload(r, "a key path the name holds", env.TLS+"=files\n"+env.TLSCert+"="+certFile+"\n"+env.TLSKey+"=KEY\n",
		"tls: "+strings.ReplaceAll(shortErr.Error(), "KEY", env.TLSKey))
	refusedAtStart("a key path the name holds", nil, "tls: "+shortErr.Error())

	// DNS-01 passes without its directory: the checks make nothing.
	reload(r, "a good DNS-01 change", env.TLS+"=acme\n"+env.TLSNames+"=*.example.net\n"+env.ACMEDNSHook+"="+hook+"\n", "")
	if left, err := os.ReadDir(data); err != nil || len(left) != 0 {
		t.Fatalf("the data directory after the checks: %v %v", left, err)
	}

	// A pairing: a MASSIMO_TLS_HTTP the start took beside acme is refused,
	// unchanged, once MASSIMO_TLS changes or leaves the file; a plain
	// _IGNORE the start took without a Cube, once the file names one.
	acme := env.TLSNames + "=massimo.example.net\n" + env.TLSHTTP + "=:80\n"
	startWith := func(text string, flags map[string]string) *settingsReloader {
		t.Helper()
		os.WriteFile(p, []byte(text), 0o600)
		if _, _, err := env.ReloadFile(p); err != nil {
			t.Fatal(err)
		}
		return newSettingsReloader(p, sysLog, srv, settings(), fixedSettings(flags), nil)
	}
	paired := startWith(env.TLS+"=acme\n"+acme+env.Ignore+"=zz\n", nil)
	// Each named by the setting that changed, not by the one the start's text names.
	names("MASSIMO_TLS from acme to off", reload(paired, "MASSIMO_TLS from acme to off", env.TLS+"=off\n"+acme+env.Ignore+"=zz\n",
		"-tls-http goes with -tls acme"), []string{env.TLS})
	refusedAtStart("MASSIMO_TLS from acme to off", nil, "-tls-http goes with -tls acme")
	names("a Cube beside the plain _IGNORE", reload(paired, "a Cube beside the plain _IGNORE",
		env.TLS+"=acme\n"+acme+env.Ignore+"=zz\nMASSIMO_CUBE_KEQ0000000_ADDRESS=192.0.2.1\n",
		`MASSIMO_CUBE_IGNORE: rf address "zz": want 6 hex digits`), []string{"MASSIMO_CUBE_KEQ0000000_ADDRESS"})
	refusedAtStart("a Cube beside the plain _IGNORE", nil, `MASSIMO_CUBE_IGNORE: rf address "zz": want 6 hex digits`)
	names("MASSIMO_TLS=acme dropped", reload(paired, "MASSIMO_TLS=acme dropped", acme+env.Ignore+"=zz\n", "-tls-http goes with -tls acme"),
		[]string{env.TLS})
	refusedAtStart("MASSIMO_TLS=acme dropped", nil, "-tls-http goes with -tls acme")

	// A flag's value where it fixes a setting the check reads: -tls acme
	// given, and the file adds a DNS hook beside its MASSIMO_TLS_HTTP.
	tlsFlag := map[string]string{"tls": "acme"}
	tlsFixed := startWith(acme, tlsFlag)
	names("-tls acme given, a DNS hook added", reload(tlsFixed, "-tls acme given, a DNS hook added", acme+env.ACMEDNSHook+"="+hook+"\n",
		"-tls-http is for the HTTP-01 challenge; with -acme-dns-hook the DNS-01 challenge needs no port"), []string{env.ACMEDNSHook})
	refusedAtStart("-tls acme given, a DNS hook added", tlsFlag,
		"-tls-http is for the HTTP-01 challenge; with -acme-dns-hook the DNS-01 challenge needs no port")

	// The flags' settings are not the file's; one no flag fixes still is.
	fixed := startWith("", map[string]string{"log-level": "info", "log-format": "plain", "allow-reboot": "on"})
	reload(fixed, "-log-level, -log-format and -allow-reboot given", env.LogLevel+"=deubg\n"+env.LogFormat+"=xml\n"+rebootVar+"=maybe\n", "")
	reload(fixed, "-log-file-format not given", env.LogLevel+"=deubg\n"+env.LogFormat+"=xml\n"+env.LogFileFormat+"=xml\n",
		`-log-file-format or MASSIMO_LOG_FILE_FORMAT: unknown log format "xml", want json or plain`)

	// The process environment's value wins over the file's, at the next
	// start as at this one: the file's bad value is not the start's.
	t.Cleanup(func() { // after t.Setenv's: the env package forgets the environment's name too
		os.WriteFile(p, []byte(env.HSTS+"=30\n"), 0o600)
		env.LoadFile(p)
		os.WriteFile(p, nil, 0o600)
		env.ReloadFile(p)
	})
	t.Setenv(env.HSTS, "30")
	environment := startWith("", nil)
	reload(environment, "MASSIMO_HSTS set in the environment", env.HSTS+"=-1\n"+env.TrustedProxies+"=127.0.0.1\n", "")
}

// A settings file that does not parse, read again, changes nothing: the
// process's settings, the token and what waits for the next start stay
// as they were, the file counting as read then, and POST /api/v1/reload
// answers 500 with the error. Its settings_read is a warning with the
// start's own error after "the next start refuses: ": one line, on the
// process's log as in the server's, where the log's warning sign counts
// it, and without the bad line, which may be half a secret wrapped onto
// a line of its own. GET /api/v1/restart says the next start refuses
// the file, by its error alone, naming no setting. The next read that
// parses settles it and clears that, though it changed nothing; a
// missing file is the reloader's warning alone. What the API's read does
// after the read (serve's rotation and checkpoint, apiReload) runs once
// for each read, before its answer, the one that failed included, as
// SIGHUP's does (AP StartConfiguration, D8-A).
func TestReloadWarnsOfAnUnreadableFile(t *testing.T) {
	unsetForTest(t, tokenVar, env.HSTS)
	p := filepath.Join(t.TempDir(), "env")
	put := func(text string) {
		t.Helper()
		if err := os.WriteFile(p, []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	put("")
	t.Cleanup(func() { os.WriteFile(p, nil, 0o600); env.ReloadFile(p) }) // the env package forgets the test's names
	st := storekit.Open(t, "")
	// The log's writer, which the server's Run would start.
	wctx, stopWriter := context.WithCancel(context.Background())
	writerDone := make(chan struct{})
	go func() { st.RunEvents(wctx); close(writerDone) }()
	t.Cleanup(func() { stopWriter(); <-writerDone })
	// One process log, as serve's: the reloader's lines and the server's.
	log, w := testkit.LogWatch(nil)
	srv := server.New(server.Options{Store: st, Logger: log})
	r := newSettingsReloader(p, log.With("source", "system"), srv, settings(), nil, nil)
	// The API's reload as serve hands it over, counting what runs after.
	afters := 0
	srv.SetReloader(p, r.apiReload(func(context.Context) { afters++ }))
	api := func(method, path string, v any) int {
		t.Helper()
		req := httptest.NewRequest(method, path, strings.NewReader("{}"))
		req.RemoteAddr = "127.0.0.1:5000"
		rec := httptest.NewRecorder()
		srv.Handler().ServeHTTP(rec, req)
		if err := json.Unmarshal(rec.Body.Bytes(), v); err != nil {
			t.Fatalf("%s %s: %d %s", method, path, rec.Code, rec.Body.String())
		}
		return rec.Code
	}
	type line struct{ Level, Error string }
	type restartView struct {
		Pending []string         `json:"pending"`
		ReadAt  *time.Time       `json:"read_at"`
		Refused []server.Refusal `json:"refused"`
	}
	type alerts struct {
		Level  string `json:"level"`
		Alerts []struct {
			Kind string `json:"kind"`
		} `json:"alerts"`
	}
	reads := 0 // the settings_read lines written
	// newest waits for the server's newest settings_read, the reads'th,
	// and answers it with the log's warning sign.
	newest := func(what string) (line, alerts) {
		t.Helper()
		var lines []line
		testkit.Eventually(t, what+": the settings_read in the server's log", func() bool {
			lines = nil
			api("GET", "/api/v1/log?kind=settings_read", &lines)
			return len(lines) == reads
		})
		var sign alerts
		api("GET", "/api/v1/log/alerts", &sign)
		return lines[len(lines)-1], sign
	}
	warned := func(rec testkit.Record) bool { return rec.Level >= slog.LevelWarn }
	settingsRead := testkit.Msg("settings read", "kind", "settings_read")

	// A good read first, which leaves MASSIMO_HSTS waiting for the next start.
	good := env.HSTS + "=30\n"
	put(good)
	if _, _, _, err := r.reload(t.Context(), "the settings file changed"); err != nil {
		t.Fatal(err)
	}
	reads++
	newest("a good read")

	// A changed value and a token wrapped onto a line of its own, which
	// does not parse.
	put(env.HSTS + "=60\n" + tokenVar + "=placeholder-token-\nwrapped-secret-9\n")
	m := w.Mark()
	t0 := time.Now()
	var answer struct{ Error string }
	code := api("POST", "/api/v1/reload", &answer)
	reads++
	l, sign := newest("a file that does not parse")
	for _, secret := range []string{"wrapped-secret-9", "placeholder-token-"} {
		w.HasNot(t, 0, "the token's "+secret+" in the log", testkit.Contains(secret))
		if strings.Contains(answer.Error, secret) || strings.Contains(l.Error, secret) {
			t.Fatalf("the token's %s in the answer %q or the server's line %q", secret, answer.Error, l.Error)
		}
	}
	// The start's own error, which every command stops with.
	parseErr := p + ":3: expected KEY=VALUE"
	if _, err := env.LoadFile(p); err == nil || err.Error() != parseErr {
		t.Fatalf("the start's own error: %v, want %s", err, parseErr)
	}
	if code != 500 || answer.Error != "reading the settings file: "+parseErr || afters != 1 {
		t.Fatalf("POST /api/v1/reload: %d %q, what runs after it %d times", code, answer.Error, afters)
	}
	want := "the next start refuses: " + parseErr
	rec, _ := w.Has(t, m, "the settings_read line at warn", testkit.MsgAt(slog.LevelWarn, "settings read", "kind", "settings_read", "why", "the API"))
	if rec.Attrs["error"] != want {
		t.Fatalf("the line's error\n%q, want\n%q", rec.Attrs["error"], want)
	}
	if n := w.Count(m, warned); n != 1 {
		t.Fatalf("%d warnings, want the event's line alone: %v", n, w.Since(m, warned))
	}
	if l.Level != "warn" || l.Error != want || sign.Level != "warn" || len(sign.Alerts) != 1 || sign.Alerts[0].Kind != "settings_read" {
		t.Fatalf("the line %+v, the sign %+v", l, sign)
	}
	// The file counts as read then, as the event's time says in the web
	// UI; nothing of it applies.
	var after restartView
	api("GET", "/api/v1/restart", &after)
	if os.Getenv(env.HSTS) != "30" || os.Getenv(tokenVar) != "" || srv.AuthMode() != "open" ||
		strings.Join(after.Pending, ",") != env.HSTS || strings.Join(r.pending(), ",") != env.HSTS ||
		after.ReadAt == nil || after.ReadAt.Before(t0) {
		t.Fatalf("after the failed read: %s=%q, the token %q, mode %s, pending %v and %v, read at %v, the read asked for at %v",
			env.HSTS, os.Getenv(env.HSTS), os.Getenv(tokenVar), srv.AuthMode(), after.Pending, r.pending(), after.ReadAt, t0)
	}
	// What the next start refuses: the file, before any setting of it.
	if !slices.EqualFunc(after.Refused, []server.Refusal{{Error: parseErr}}, sameRefusal) {
		t.Fatalf("after the failed read: refused %+v, want the file's error alone", after.Refused)
	}

	// The file as it was: nothing changed, and the read settles the
	// warning; a read after it that changes nothing writes no line again.
	put(good)
	m = w.Mark()
	if changed, _, _, err := r.reload(t.Context(), "the settings file changed"); err != nil || len(changed) != 0 {
		t.Fatalf("the file as it was: %v %v", changed, err)
	}
	reads++
	w.HasNot(t, m, "a warning", warned)
	if l, sign := newest("the file as it was"); l.Level != "info" || l.Error != "" || sign.Level != "" {
		t.Fatalf("the file as it was: the line %+v, the sign %+v", l, sign)
	}
	after = restartView{}
	if api("GET", "/api/v1/restart", &after); after.Refused != nil {
		t.Fatalf("the file as it was: refused %+v, want none", after.Refused)
	}
	m = w.Mark()
	r.reload(t.Context(), "the settings file changed")
	w.HasNot(t, m, "a settings_read of an unchanged file", settingsRead)
	// The API's good read, and what runs after it, once more.
	var res server.ReloadResult
	if code := api("POST", "/api/v1/reload", &res); code != 200 || afters != 2 {
		t.Fatalf("the API's good read: %d %+v, what runs after it %d times", code, res, afters)
	}

	// A missing file: an editor may be replacing it, and the next start
	// takes it as none.
	os.Remove(p)
	m = w.Mark()
	if _, _, _, err := r.reload(t.Context(), "SIGHUP"); !os.IsNotExist(err) {
		t.Fatalf("a missing file: %v", err)
	}
	w.Has(t, m, "a missing file: the reloader's warning", testkit.MsgAt(slog.LevelWarn, "settings file not read again", "why", "SIGHUP"))
	w.HasNot(t, m, "a missing file: a settings_read", settingsRead)
}

// A settings file saved again with its settings unchanged, which the
// watcher reads, writes its settings_read where what the next start
// refuses has changed meanwhile on the box: a DNS hook the file names
// made, then removed again. GET /api/v1/restart, the server's newest
// settings_read and the log's warning sign agree at each step: a
// warning, settled, a warning again; a read that finds the same writes
// no line (AP ReloadChecks, D8-B).
func TestReloadWarnsAsTheBoxChanges(t *testing.T) {
	clearCubeEnv(t)
	unsetForTest(t, tokenVar, env.TLS, env.TLSCert, env.TLSKey, env.TLSNames, env.TLSHTTP, env.ACMEDNSHook)
	dir := t.TempDir()
	p := filepath.Join(dir, "env")
	os.WriteFile(p, nil, 0o600)
	t.Cleanup(func() { os.WriteFile(p, nil, 0o600); env.ReloadFile(p) }) // the env package forgets the test's names
	st := storekit.Open(t, "")
	// The log's writer, which the server's Run would start.
	wctx, stopWriter := context.WithCancel(context.Background())
	writerDone := make(chan struct{})
	go func() { st.RunEvents(wctx); close(writerDone) }()
	t.Cleanup(func() { stopWriter(); <-writerDone })
	log, w := testkit.LogWatch(nil)
	srv := server.New(server.Options{Store: st, Logger: log})
	r := newSettingsReloader(p, log.With("source", "system"), srv, settings(), nil, nil)
	apiGet := func(path string, v any) {
		t.Helper()
		req := httptest.NewRequest("GET", path, nil)
		req.RemoteAddr = "127.0.0.1:5000"
		rec := httptest.NewRecorder()
		srv.Handler().ServeHTTP(rec, req)
		if rec.Code != 200 || json.Unmarshal(rec.Body.Bytes(), v) != nil {
			t.Fatalf("GET %s: %d %s", path, rec.Code, rec.Body.String())
		}
	}
	hook := filepath.Join(dir, "hook.sh")
	_, missing := os.Stat(hook)
	text := env.TLS + "=acme\n" + env.TLSNames + "=*.example.net\n" + env.ACMEDNSHook + "=" + hook + "\n"
	refused := []server.Refusal{{Settings: []string{env.ACMEDNSHook, env.TLS, env.TLSNames}, Error: "tls: -acme-dns-hook: " + missing.Error()}}
	reads := 0 // the settings_read lines written
	// save writes the file, the line before it a comment, and has the
	// watcher read it; then the newest settings_read, the sign and GET
	// /api/v1/restart's refused are want's, a warning or none.
	save := func(what, comment string, want []server.Refusal) {
		t.Helper()
		os.WriteFile(p, []byte(comment+text), 0o600)
		if _, _, _, err := r.reload(t.Context(), "the settings file changed"); err != nil {
			t.Fatalf("%s: %v", what, err)
		}
		reads++
		var lines []struct{ Level, Error string }
		testkit.Eventually(t, what+": the settings_read in the server's log", func() bool {
			lines = nil
			apiGet("/api/v1/log?kind=settings_read", &lines)
			return len(lines) == reads
		})
		var sign struct {
			Level string `json:"level"`
		}
		apiGet("/api/v1/log/alerts", &sign)
		var after struct {
			Refused []server.Refusal `json:"refused"`
		}
		apiGet("/api/v1/restart", &after)
		level := "info"
		if len(want) > 0 {
			level = "warn"
		}
		if l := lines[len(lines)-1]; l.Level != level || sign.Level != strings.TrimPrefix(level, "info") ||
			!slices.EqualFunc(after.Refused, want, sameRefusal) {
			t.Fatalf("%s: the line %+v, the sign %q, refused %+v, want %+v", what, l, sign.Level, after.Refused, want)
		}
	}
	save("the hook named, not there", "", refused)
	if err := os.WriteFile(hook, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	save("the hook made, the file saved with a comment", "# a comment\n", nil)
	m := w.Mark()
	os.WriteFile(p, []byte("# another comment\n"+text), 0o600)
	r.reload(t.Context(), "the settings file changed")
	w.HasNot(t, m, "a settings_read of a read that finds the same", testkit.Msg("settings read", "kind", "settings_read"))
	os.Remove(hook)
	save("the hook removed, the file saved again", "# a third comment\n", refused)
}

// A reload that finds notes on the settings file, lines it set aside,
// says them on the process's log, the reloader's warning of its own, and
// its settings_read is a warning of them after "the settings file: ",
// after what the next start refuses where both stand: one line of all
// that the read found. They are no refusals: POST /api/v1/reload and GET
// /api/v1/restart name the refusals alone. A read whose settings did not
// change writes its settings_read where the notes changed, which settles
// the warning once the file is fixed, and none where they did not; a
// note names a secret given twice by its name, never a value (AP
// StartConfiguration, D3-B).
func TestReloadTellsTheNotes(t *testing.T) {
	const cubeKey = "MASSIMO_CUBE_KEQ0000000_KEY"
	unsetForTest(t, tokenVar, env.HSTS, cubeKey)
	p := filepath.Join(t.TempDir(), "env")
	put := func(text string) {
		t.Helper()
		if err := os.WriteFile(p, []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	put("")
	t.Cleanup(func() { os.WriteFile(p, nil, 0o600); env.ReloadFile(p) }) // the env package forgets the test's names
	st := storekit.Open(t, "")
	// The log's writer, which the server's Run would start.
	wctx, stopWriter := context.WithCancel(context.Background())
	writerDone := make(chan struct{})
	go func() { st.RunEvents(wctx); close(writerDone) }()
	t.Cleanup(func() { stopWriter(); <-writerDone })
	// One process log, as serve's: the reloader's lines and the server's.
	log, w := testkit.LogWatch(nil)
	srv := server.New(server.Options{Store: st, Logger: log})
	r := newSettingsReloader(p, log.With("source", "system"), srv, settings(), nil, nil)
	srv.SetReloader(p, r.apiReload(func(context.Context) {}))
	api := func(method, path string, v any) {
		t.Helper()
		req := httptest.NewRequest(method, path, strings.NewReader("{}"))
		req.RemoteAddr = "127.0.0.1:5000"
		rec := httptest.NewRecorder()
		srv.Handler().ServeHTTP(rec, req)
		if rec.Code != 200 || json.Unmarshal(rec.Body.Bytes(), v) != nil {
			t.Fatalf("%s %s: %d %s", method, path, rec.Code, rec.Body.String())
		}
	}
	warned := func(rec testkit.Record) bool { return rec.Level >= slog.LevelWarn }
	settingsRead := testkit.Msg("settings read", "kind", "settings_read")
	setAside := func(why, notes string) func(testkit.Record) bool {
		return testkit.MsgAt(slog.LevelWarn, "settings file lines set aside", "file", p, "why", why, "notes", notes)
	}
	reads := 0 // the settings_read lines written
	// check waits for the server's newest settings_read, the reads'th,
	// and checks it, the log's sign and GET /api/v1/restart's refused:
	// the line at warn with want as its error, or at info for want "",
	// and refused the same as the reload's.
	check := func(what, want string, refused []server.Refusal) {
		t.Helper()
		var lines []struct{ Level, Error string }
		testkit.Eventually(t, what+": the settings_read in the server's log", func() bool {
			lines = nil
			api("GET", "/api/v1/log?kind=settings_read", &lines)
			return len(lines) == reads
		})
		var sign struct {
			Level string `json:"level"`
		}
		api("GET", "/api/v1/log/alerts", &sign)
		var after struct {
			Refused []server.Refusal `json:"refused"`
		}
		api("GET", "/api/v1/restart", &after)
		level := "warn"
		if want == "" {
			level = ""
		}
		if l := lines[len(lines)-1]; l.Error != want || l.Level != cmp.Or(level, "info") || sign.Level != level || !slices.EqualFunc(after.Refused, refused, sameRefusal) {
			t.Fatalf("%s: the line %+v, the sign %q, refused %+v; want the error\n%q, refused %+v", what, l, sign.Level, after.Refused, want, refused)
		}
	}
	hsts := `-hsts or MASSIMO_HSTS "-1": want a number of days, 0 or more`
	twice := env.HSTS + " is given on lines 1 and 2; the first wins"

	// A start that found a note, as serve makes its reloader with the
	// notes its log says after server_started: the file read again as
	// the start read it writes no line, the start's standing; the later
	// line deleted, which changes no value, writes its line at info,
	// which settles the start's warning.
	put(env.HSTS + "=30\n" + env.HSTS + "=60\n")
	found, err := env.LoadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	startNotes := noteTexts(found)
	atStart := newSettingsReloader(p, log.With("source", "system"), srv, settings(), nil, startNotes)
	srv.LogSettingsRead(t.Context(), "start", nil, nil, nil, startNotes) // logStarted's line
	reads++
	check("the start's note", "the settings file: "+twice, nil)
	m := w.Mark()
	atStart.reload(t.Context(), "the settings file changed")
	w.HasNot(t, m, "the start's note read again: a settings_read", settingsRead)
	put(env.HSTS + "=30\n")
	if changed, _, _, err := atStart.reload(t.Context(), "the settings file changed"); err != nil || len(changed) != 0 {
		t.Fatalf("the start's note fixed: %v %v", changed, err)
	}
	reads++
	check("the start's note fixed", "", nil)

	// A note and a refusal: the line holds both, the refusal first, and
	// the API names the refusal alone.
	put(env.HSTS + "=-1\n" + env.HSTS + "=30\n")
	m = w.Mark()
	var answer server.ReloadResult
	api("POST", "/api/v1/reload", &answer)
	reads++
	refused := []server.Refusal{{Settings: []string{env.HSTS}, Error: hsts}}
	if !slices.EqualFunc(answer.Refused, refused, sameRefusal) {
		t.Fatalf("a note and a refusal: the reload's refused %+v, want the refusal alone", answer.Refused)
	}
	want := "the next start refuses: " + hsts + "; the settings file: " + twice
	check("a note and a refusal", want, refused)
	w.Has(t, m, "a note and a refusal: the reloader's warning of the note", setAside("the API", twice))
	rec, _ := w.Has(t, m, "a note and a refusal: the settings_read line at warn", testkit.MsgAt(slog.LevelWarn, "settings read", "kind", "settings_read"))
	if rec.Attrs["error"] != want {
		t.Fatalf("a note and a refusal: the line's error\n%q, want\n%q", rec.Attrs["error"], want)
	}
	if n := w.Count(m, warned); n != 2 {
		t.Fatalf("a note and a refusal: %d warnings, want the reloader's and the event's line: %v", n, w.Since(m, warned))
	}

	// A note alone: a warning of it, nothing refused.
	put(env.HSTS + "=30\n" + env.HSTS + "=-1\n")
	m = w.Mark()
	answer = server.ReloadResult{}
	api("POST", "/api/v1/reload", &answer)
	reads++
	if answer.Refused == nil || len(answer.Refused) != 0 {
		t.Fatalf("a note alone: the reload's refused %+v, want []", answer.Refused)
	}
	check("a note alone", "the settings file: "+twice, nil)
	w.Has(t, m, "a note alone: the reloader's warning", setAside("the API", twice))

	// The file saved again, its settings and notes the same: the
	// reloader's warning, and no settings_read.
	put(env.HSTS + "=30\n" + env.HSTS + "=-1\n# a comment\n")
	m = w.Mark()
	r.reload(t.Context(), "the settings file changed")
	w.Has(t, m, "the same note: the reloader's warning", setAside("the settings file changed", twice))
	w.HasNot(t, m, "the same note: a settings_read", settingsRead)

	// The note fixed, nothing else changed: the read writes its line,
	// which settles the warning.
	put(env.HSTS + "=30\n")
	m = w.Mark()
	if changed, _, _, err := r.reload(t.Context(), "the settings file changed"); err != nil || len(changed) != 0 {
		t.Fatalf("the note fixed: %v %v", changed, err)
	}
	reads++
	check("the note fixed", "", nil)
	w.HasNot(t, m, "the note fixed: a warning", warned)

	// A secret given twice, the token and a Cube key: named, never its
	// value, in the log as in the server's line (which the store holds:
	// the token now asks the API for itself).
	secrets := []string{"placeholder-token-1", "placeholder-token-2", "00112233445566778899aabbccddeeff", "ffeeddccbbaa99887766554433221100"}
	put(tokenVar + "=" + secrets[0] + "\n" + tokenVar + "=" + secrets[1] + "\n" + cubeKey + "=" + secrets[2] + "\n" + cubeKey + "=" + secrets[3] + "\n")
	m = w.Mark()
	if _, _, _, err := r.reload(t.Context(), "SIGHUP"); err != nil {
		t.Fatal(err)
	}
	notes := tokenVar + " is given on lines 1 and 2; the first wins; " + cubeKey + " is given on lines 3 and 4; the first wins"
	w.Has(t, m, "a secret given twice: the reloader's warning", setAside("SIGHUP", notes))
	rec, _ = w.Has(t, m, "a secret given twice: the settings_read line at warn", testkit.MsgAt(slog.LevelWarn, "settings read", "kind", "settings_read"))
	if rec.Attrs["error"] != "the settings file: "+notes {
		t.Fatalf("a secret given twice: the line's error %q", rec.Attrs["error"])
	}
	if err := st.FlushEvents(t.Context()); err != nil {
		t.Fatal(err)
	}
	rows, err := st.Events(t.Context(), store.EventQuery{Kind: "settings_read", Limit: 100})
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range secrets {
		w.HasNot(t, 0, "a secret's value in the log", testkit.Contains(secret))
		for _, row := range rows {
			if strings.Contains(string(row.Data), secret) {
				t.Fatalf("a secret's value in the server's line: %s", row.Data)
			}
		}
	}
	if os.Getenv(tokenVar) != secrets[0] {
		t.Fatal("the token's first line does not win")
	}
}

// A reload removes the keep-open setting where the server has users
// beside it, as a process of an older version, or a store put in place
// by hand, can leave them (AP FirstOwner, D12).
func TestSettingsReloaderDropsKeepOpen(t *testing.T) {
	unsetForTest(t, tokenVar)
	p := filepath.Join(t.TempDir(), "env")
	os.WriteFile(p, nil, 0o600)
	t.Cleanup(func() { os.WriteFile(p, nil, 0o600); env.ReloadFile(p) })
	st := storekit.Open(t, "")
	srv := server.New(server.Options{Store: st, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	r := newSettingsReloader(p, slog.New(slog.NewTextHandler(io.Discard, nil)), srv, settings(), nil, nil)
	if _, err := st.AddUser(t.Context(), store.User{Name: "anna", Role: store.RoleOwner}, "", 0); err != nil {
		t.Fatal(err)
	}
	if err := st.SetSetting(t.Context(), store.KeepOpenKey, true); err != nil {
		t.Fatal(err)
	}
	r.reload(t.Context(), "test")
	if st.KeepOpen(t.Context()) {
		t.Fatal("kept open after the reload")
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
	// The reload's end, its own line, which it writes once it has set
	// what waits for the next start: it sets the token first, so a 401
	// alone may come before the pending settings.
	s.log.Await(t, m, "the settings file read again for the SIGHUP, with what changed",
		testkit.Contains(`msg="settings file read again"`, "source=system", "why=SIGHUP", "changed="), serveWait)
	if code, text := get(t, base+"/api/v1/cubes", ""); code != 401 {
		t.Fatalf("the server does not ask for the token after the SIGHUP: %d %s", code, text) // serve's log follows
	}
	if code, text := get(t, base+"/api/v1/cubes", "placeholder-token-1"); code != 200 {
		t.Fatalf("with the token: %d %s", code, text)
	}
	if code, text := get(t, base+"/api/v1/restart", "placeholder-token-1"); code != 200 || !strings.Contains(text, `"pending": [`) || !strings.Contains(text, env.TrustedProxies) {
		t.Fatalf("GET /api/v1/restart: %d %s", code, text)
	}
	// The SIGHUP's last step, the store's checkpoint, comes after the
	// reload: awaited before the stop, which would cancel it.
	s.log.Await(t, m, "the store checkpointed", testkit.Msg("store checkpointed"), serveWait)
	// Once serve ended, its log is read whole; the token first, which no
	// line shows.
	s.stop(t)
	s.log.HasNot(t, 0, "the token in the log", testkit.Contains("placeholder-token-1"))
	s.log.Has(t, m, "the token applied, its owner created", testkit.Msg("api token applied", "source", "server", "owner_created", "true"))
}

// POST /api/v1/reload does what SIGHUP does whatever the read finds: a
// settings file that does not parse, and one that is there but cannot be
// read, rotate serve's log file and checkpoint its store, and are then
// answered 500 with the read's error; a good read is answered 200 after
// the same. Windows has no SIGHUP, so the API's read is its only one that
// rotates the log file and checkpoints the store (AP StartConfiguration,
// D8-A).
func TestServeReloadFromTheAPIRotatesAndCheckpoints(t *testing.T) {
	t.Parallel()
	skipBuilt(t)
	dir := t.TempDir()
	settingsFile := filepath.Join(dir, "env")
	os.WriteFile(settingsFile, []byte("# nothing yet\n"), 0o600)
	s := startServe(t, settingsFile, t.TempDir(), "-log", filepath.Join(dir, "serve.log"))
	// reload asks for the read and answers its status and body, once
	// serve's log has said the read's rotation and checkpoint.
	reload := func(what string) (int, []byte) {
		t.Helper()
		m := s.log.Mark()
		resp, err := http.Post("http://"+s.addr+"/api/v1/reload", "application/json", strings.NewReader("{}"))
		if err != nil {
			t.Fatal(err)
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		s.log.Await(t, m, what+": the log file rotated", testkit.MsgAt(slog.LevelInfo, "log rotated", "reason", "reload"), serveWait)
		s.log.Await(t, m, what+": the store checkpointed", testkit.Msg("store checkpointed"), serveWait)
		return resp.StatusCode, body
	}
	failed := func(what, want string) {
		t.Helper()
		var answer struct{ Error string }
		if code, body := reload(what); code != 500 || json.Unmarshal(body, &answer) != nil || answer.Error != "reading the settings file: "+want {
			t.Fatalf("%s: %d %s, want 500 with %q", what, code, body, want)
		}
	}

	os.WriteFile(settingsFile, []byte(env.HSTS+"=60\nwrapped\n"), 0o600)
	failed("a file that does not parse", settingsFile+":2: expected KEY=VALUE")
	os.Remove(settingsFile)
	if err := os.Mkdir(settingsFile, 0o700); err != nil {
		t.Fatal(err)
	}
	failed("a file that cannot be read", settingsFile+":1: read "+settingsFile+": is a directory")
	os.Remove(settingsFile)
	os.WriteFile(settingsFile, []byte(env.HSTS+"=60\n"), 0o600)
	var res server.ReloadResult
	if code, body := reload("a good read"); code != 200 || json.Unmarshal(body, &res) != nil || strings.Join(res.Pending, ",") != env.HSTS {
		t.Fatalf("a good read: %d %s", code, body)
	}
	s.stop(t)
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
