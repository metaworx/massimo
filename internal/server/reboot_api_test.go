// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

import (
	"encoding/json"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"go.metaworx.ch/massimo/internal/store"
	"go.metaworx.ch/massimo/internal/testkit"
	"go.metaworx.ch/massimo/internal/testkit/cubewatch"
	"go.metaworx.ch/massimo/internal/update"
)

// rebootFake is a RebootOptions.Reboot that reports each call on calls
// and fails with err.
type rebootFake struct {
	calls chan struct{}
	err   error
}

func newRebootFake(err error) *rebootFake {
	return &rebootFake{calls: make(chan struct{}, 8), err: err}
}

func (f *rebootFake) reboot() error {
	f.calls <- struct{}{}
	return f.err
}

// called waits for one call, or fails the test after five seconds,
// scaled.
func (f *rebootFake) called(t *testing.T) {
	t.Helper()
	select {
	case <-f.calls:
	case <-time.After(testkit.Scale(5 * time.Second)):
		t.Fatal("the reboot was not asked for")
	}
}

// none checks that no call came.
func (f *rebootFake) none(t *testing.T) {
	t.Helper()
	select {
	case <-f.calls:
		t.Fatal("the reboot was asked for")
	default:
	}
}

func decodeReboot(t *testing.T, text string) rebootView {
	t.Helper()
	var v rebootView
	if err := json.Unmarshal([]byte(text), &v); err != nil {
		t.Fatalf("%v: %s", err, text)
	}
	return v
}

// rebootAnswer is the answer to POST /api/v1/reboot.
type rebootAnswer struct {
	OK bool      `json:"ok"`
	By string    `json:"by"`
	At time.Time `json:"at"`
}

func rebootOptions(f *rebootFake) RebootOptions {
	return RebootOptions{By: "systemd", Allowed: true, Helper: func() string { return "" }, Reboot: f.reboot}
}

// The capability reboot is the owner's: a manager, who may restart the
// server, may not reboot its computer. The owner's request with no delay
// reboots at once: 202, the reboot asked for once, an audit row and an
// acknowledged command, its retry replayed, and the Cube sent nothing;
// GET then says the reboot has begun.
func TestRebootRoutes(t *testing.T) {
	t.Parallel()
	f := newRebootFake(nil)
	opts := storeOptions(t, t.TempDir(), "secret")
	opts.Reboot = rebootOptions(f)
	r := startServer(t, withOptions(opts))
	fc, ts, s := r.fc, r.ts, r.s
	keepStore(t, ts, s.opts.Store)
	manager := userToken(t, s.opts.Store, "manager1", store.RoleManager)
	path := "/api/v1/reboot"
	code, text := callAPI(t, ts, "GET", path, "secret", "")
	if v := decodeReboot(t, text); code != 200 || !v.Available || v.By != "systemd" || v.Blocked != "" || v.At != nil {
		t.Fatalf("GET as owner: %d %s", code, text)
	}
	for _, m := range []string{"GET", "POST", "DELETE"} {
		if code, text := callAPI(t, ts, m, path, manager, "{}"); code != 403 || errorOf(text) != `capability "reboot" required` {
			t.Fatalf("%s as manager: %d %s", m, code, text)
		}
		if code, text := callAPI(t, ts, m, path, "", "{}"); code != 401 {
			t.Fatalf("%s without a key: %d %s", m, code, text)
		}
	}
	received := cubewatch.Lines(fc)
	before := received.Mark()
	headers := map[string]string{"Idempotency-Key": "reboot-1"}
	resp, text := callWith(t, ts, "POST", path, "secret", `{"delay": 0}`, headers)
	var answer rebootAnswer
	if json.Unmarshal([]byte(text), &answer); resp.StatusCode != 202 || !answer.OK || answer.By != "systemd" || time.Since(answer.At) > 5*time.Second {
		t.Fatalf("POST as owner: %d %s", resp.StatusCode, text)
	}
	f.called(t)
	rows := auditRows(t, ts, "secret", "?limit=1")
	if len(rows) != 1 || rows[0].Action != "server.reboot" {
		t.Fatalf("audit %+v", rows)
	}
	if cmd := commandOfRow(t, ts, "secret", rows[0]); cmd.Action != "server.reboot" || cmd.State != store.StateAcknowledged || cmd.Status != 202 {
		t.Fatalf("the reboot's command %+v", cmd)
	}
	if resp, text := callWith(t, ts, "POST", path, "secret", `{"delay": 0}`, headers); resp.StatusCode != 202 || resp.Header.Get("Idempotent-Replayed") != "true" {
		t.Fatalf("the retry: %d %s", resp.StatusCode, text)
	}
	f.none(t)
	code, text = callAPI(t, ts, "GET", path, "secret", "")
	if v := decodeReboot(t, text); code != 200 || v.Available || v.Blocked != rebootBegun || v.At == nil || !v.Begun {
		t.Fatalf("GET after the reboot began: %d %s", code, text)
	}
	if code, text := callAPI(t, ts, "DELETE", path, "secret", ""); code != 409 || errorOf(text) != rebootBegun || !refusalHas(t, text, "reboot_begun", "null") {
		t.Fatalf("taking back a reboot that began: %d %s", code, text)
	}
	if got := received.Since(before, cubewatch.Except('l')); len(got) != 0 {
		t.Fatalf("the reboot sent the Cube %v", got)
	}
}

// A reboot waits its delay, 60 s when the request names none, and can be
// taken back meanwhile; while it waits the server neither takes another
// nor restarts. A delay outside 0 to 3600 seconds is refused
// (reboot_delay_bad), a body that does not parse with no code.
func TestRebootDelayAndTakeBack(t *testing.T) {
	t.Parallel()
	f := newRebootFake(nil)
	opts := storeOptions(t, t.TempDir(), "secret")
	opts.Reboot = rebootOptions(f)
	opts.Restart = RestartOptions{By: "systemd", Restart: func() error { t.Error("restarted"); return nil }}
	r := startServer(t, withOptions(opts))
	ts, s := r.ts, r.s
	keepStore(t, ts, s.opts.Store)
	path := "/api/v1/reboot"
	for body, code := range map[string]string{`{"delay": -1}`: "reboot_delay_bad", `{"delay": 3601}`: "reboot_delay_bad", `{"delay": "60"}`: "", `{"delay": 1.5}`: "", `[`: ""} {
		params := "null"
		if code != "" {
			params = `{"max":3600}`
		}
		if status, text := callAPI(t, ts, "POST", path, "secret", body); status != 400 || !refusalHas(t, text, code, params) {
			t.Errorf("POST %s: %d %s", body, status, text)
		}
	}
	for _, c := range []struct {
		body string
		want time.Duration
	}{{"", DefaultRebootDelay}, {"{}", DefaultRebootDelay}, {`{"delay": 3600}`, time.Hour}} {
		code, text := callAPI(t, ts, "POST", path, "secret", c.body)
		var answer rebootAnswer
		if json.Unmarshal([]byte(text), &answer); code != 202 || answer.At.Sub(time.Now().Add(c.want)).Abs() > 5*time.Second {
			t.Fatalf("POST %q: %d %s", c.body, code, text)
		}
		code, text = callAPI(t, ts, "GET", path, "secret", "")
		if v := decodeReboot(t, text); code != 200 || v.Available || v.Blocked != rebootWaiting || v.At == nil || !v.At.Equal(answer.At) || v.Begun {
			t.Fatalf("GET while it waits: %d %s", code, text)
		}
		if code, text := callAPI(t, ts, "POST", path, "secret", `{"delay": 0}`); code != 409 || errorOf(text) != rebootWaiting || !refusalHas(t, text, "reboot_waiting", "null") {
			t.Fatalf("a second reboot: %d %s", code, text)
		}
		if code, text := callAPI(t, ts, "POST", "/api/v1/restart", "secret", "{}"); code != 409 || errorOf(text) != "a reboot of the computer is waiting" ||
			!refusalHas(t, text, "reboot_waiting", "null") {
			t.Fatalf("a restart while it waits: %d %s", code, text)
		}
		if code, text := callAPI(t, ts, "DELETE", path, "secret", ""); code != 200 {
			t.Fatalf("DELETE: %d %s", code, text)
		}
		rows := auditRows(t, ts, "secret", "?limit=1")
		if len(rows) != 1 || rows[0].Action != "server.reboot.cancel" {
			t.Fatalf("audit %+v", rows)
		}
		code, text = callAPI(t, ts, "GET", path, "secret", "")
		if v := decodeReboot(t, text); code != 200 || !v.Available || v.At != nil {
			t.Fatalf("GET after taking it back: %d %s", code, text)
		}
	}
	if code, text := callAPI(t, ts, "DELETE", path, "secret", ""); code != 409 || errorOf(text) != "no reboot is waiting" || !refusalHas(t, text, "reboot_not_waiting", "null") {
		t.Fatalf("DELETE with none waiting: %d %s", code, text)
	}
	// A timer that fires after its reboot was taken back does nothing.
	if _, ok := s.planReboot(time.Hour); !ok {
		t.Fatal("not planned")
	}
	s.reboot.mu.Lock()
	gen := s.reboot.gen
	s.reboot.takeBack()
	s.reboot.mu.Unlock()
	s.rebootNow(gen)
	f.none(t)
}

// The API says why the computer cannot be rebooted, in order, each with
// its code: the system, the owner's switch, the helper, an update at
// work; a reboot that fails
// is reported, and the next one may be asked for. Switching the reboot
// off takes back one that waits.
func TestRebootBlocked(t *testing.T) {
	t.Parallel()
	path := "/api/v1/reboot"
	refused := func(o RebootOptions, update UpdateOptions, want, wantCode string) {
		t.Helper()
		opts := storeOptions(t, t.TempDir(), "secret")
		opts.Reboot, opts.Update = o, update
		ts := startServer(t, withOptions(opts)).ts
		code, text := callAPI(t, ts, "GET", path, "secret", "")
		if v := decodeReboot(t, text); code != 200 || v.Available || v.Blocked != want {
			t.Fatalf("GET, want %q: %d %s", want, code, text)
		}
		if code, text := callAPI(t, ts, "POST", path, "secret", `{"delay": 0}`); code != 409 || errorOf(text) != want || !refusalHas(t, text, wantCode, "null") {
			t.Fatalf("POST, want %q, %s: %d %s", want, wantCode, code, text)
		}
	}
	f := newRebootFake(nil)
	refused(RebootOptions{}, UpdateOptions{}, "this server does not reboot the computer it runs on", "reboot_unavailable")
	terminal := "only a server the system runs as a service reboots its computer"
	refused(RebootOptions{Blocked: terminal, Allowed: true, Reboot: f.reboot}, UpdateOptions{}, terminal, "reboot_unavailable")
	o := rebootOptions(f)
	o.Allowed = false
	o.Helper = func() string { t.Error("the helper looked at while switched off"); return "" }
	refused(o, UpdateOptions{}, rebootSwitchedOff, "reboot_switched_off")
	o = rebootOptions(f)
	missing := "no reboot helper installed: sudo massimo install -reboot-helper"
	o.Helper = func() string { return missing }
	refused(o, UpdateOptions{}, missing, "reboot_no_helper")
	status := filepath.Join(t.TempDir(), "status.json")
	if err := update.WriteStatus(status, update.Status{State: "installing", To: "v0.1.0-alpha.10", Time: time.Now()}); err != nil {
		t.Fatal(err)
	}
	refused(rebootOptions(f), UpdateOptions{Status: status}, "an update is running", "update_running")
	f.none(t)

	failing := newRebootFake(errors.New("the reboot helper did not take the request within 30s"))
	opts := storeOptions(t, t.TempDir(), "secret")
	opts.Reboot = rebootOptions(failing)
	opts.rebootStuck = 50 * time.Millisecond // a taken reboot counts as failed after it
	r := startServer(t, withOptions(opts))
	ts, s := r.ts, r.s
	if code, text := callAPI(t, ts, "POST", path, "secret", `{"delay": 0}`); code != 202 {
		t.Fatalf("POST: %d %s", code, text)
	}
	failing.called(t)
	var answer string
	if !testkit.Within(5*time.Second, func() bool {
		_, answer = callAPI(t, ts, "GET", path, "secret", "")
		v := decodeReboot(t, answer)
		return v.Error == failing.err.Error() && v.Available && v.At == nil
	}) {
		t.Fatalf("GET after the failure: %s", answer)
	}

	// A reboot the helper took but the computer did not carry out.
	failing.err = nil
	if code, text := callAPI(t, ts, "POST", path, "secret", `{"delay": 0}`); code != 202 {
		t.Fatalf("POST: %d %s", code, text)
	}
	failing.called(t)
	if !testkit.Within(5*time.Second, func() bool {
		_, answer = callAPI(t, ts, "GET", path, "secret", "")
		v := decodeReboot(t, answer)
		return v.Error == "the computer has not rebooted within 50ms; the reboot helper's log says why" && v.Available && !v.Begun
	}) {
		t.Fatalf("GET after a reboot that did not happen: %s", answer)
	}

	if code, text := callAPI(t, ts, "POST", path, "secret", `{"delay": 3600}`); code != 202 {
		t.Fatalf("POST: %d %s", code, text)
	}
	s.SetRebootAllowed(false)
	_, text := callAPI(t, ts, "GET", path, "secret", "")
	if v := decodeReboot(t, text); v.Available || v.Blocked != rebootSwitchedOff || v.At != nil || v.Error != "" {
		t.Fatalf("GET once switched off: %s", text)
	}
	s.SetRebootAllowed(true)
	if _, text := callAPI(t, ts, "GET", path, "secret", ""); !decodeReboot(t, text).Available {
		t.Fatalf("GET once switched on again: %s", text)
	}
	failing.none(t)
}

// An open server reboots only for a request from the box itself, and
// takes a reboot back only for one; anyone else gets the reason.
func TestRebootOpenServer(t *testing.T) {
	t.Parallel()
	f := newRebootFake(nil)
	trusted, err := ParseTrustedProxies("127.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	opts := storeOptions(t, t.TempDir(), "")
	opts.Reboot = rebootOptions(f)
	opts.TrustedProxies = trusted
	ts := startServer(t, withOptions(opts)).ts
	path := "/api/v1/reboot"
	h := map[string]string{"X-Forwarded-For": "192.168.1.20"}
	if code, text := sendWith(t, ts, "GET", path, "", h); code != 200 || decodeReboot(t, text).Available || decodeReboot(t, text).Blocked != openRebootRefused {
		t.Fatalf("GET through the proxy: %d %s", code, text)
	}
	if code, text := sendWith(t, ts, "POST", path, "{}", h); code != 403 || errorOf(text) != openRebootRefused || !refusalHas(t, text, "reboot_open", "null") {
		t.Fatalf("POST through the proxy: %d %s", code, text)
	}
	if code, text := sendWith(t, ts, "POST", path, `{"delay": 3600}`, nil); code != 202 {
		t.Fatalf("POST from the box: %d %s", code, text)
	}
	if code, text := sendWith(t, ts, "DELETE", path, "", h); code != 403 || errorOf(text) != openRebootRefused || !refusalHas(t, text, "reboot_open", "null") {
		t.Fatalf("DELETE through the proxy: %d %s", code, text)
	}
	if code, text := sendWith(t, ts, "DELETE", path, "", nil); code != 200 {
		t.Fatalf("DELETE from the box: %d %s", code, text)
	}
	f.none(t)
}
