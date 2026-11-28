// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

import (
	"encoding/json"
	"log/slog"
	"slices"
	"strings"
	"testing"
	"time"

	"go.metaworx.ch/massimo/internal/cubelan"
	"go.metaworx.ch/massimo/internal/cubelan/fakecube"
	"go.metaworx.ch/massimo/internal/protocol/cube"
	"go.metaworx.ch/massimo/internal/store"
	"go.metaworx.ch/massimo/internal/testkit"
)

// A refresh asked for: refused without restart, then a second
// connection to the Cube, whose greeting has replaced the snapshot by
// the answer, its budget in the answer; the Cube is sent its quit and
// nothing else, and the log names who asked.
func TestRefreshACube(t *testing.T) {
	t.Parallel()
	r := startServer(t, quiet(), withLog(slog.LevelInfo), withOptions(storeOptions(t, t.TempDir(), "secret")))
	fc, ts, rt, st := r.fc, r.ts, r.rt, r.s.opts.Store
	manager := userToken(t, st, "manager1", store.RoleManager)
	operator := userToken(t, st, "operator1", store.RoleOperator)
	path := "/api/v1/cubes/KEQ0000100/refresh"
	if code, text := callAPI(t, ts, "POST", path, operator, ""); code != 403 || errorOf(text) != `capability "restart" required` {
		t.Fatalf("as operator: %d %s", code, text)
	}
	if n := fc.Connections(); n != 1 {
		t.Fatalf("%d connections before the refresh, want 1", n)
	}

	// What only a new greeting brings while the server polls nothing: the
	// H line's free slots, and a battery flag in the L line.
	rf := cube.RFAddress(0x0a010d)
	if s, ok := rt.client.Snapshot().State(rf); !ok || s.BatteryLow {
		t.Fatalf("state of %s before: %+v %v", rf, s, ok)
	}
	fc.SetFreeSlots(33)
	low := true
	if err := fc.SetState(rf, fakecube.State{BatteryLow: &low}); err != nil {
		t.Fatal(err)
	}
	sent, logged := r.lines.Mark(), r.log.Mark()
	asked := time.Now()
	code, text := callAPI(t, ts, "POST", path, manager, "")
	if code != 200 {
		t.Fatalf("refresh: %d %s", code, text)
	}
	var answer struct {
		OK     bool           `json:"ok"`
		Budget cubelan.Budget `json:"budget"`
	}
	if err := json.Unmarshal([]byte(text), &answer); err != nil {
		t.Fatal(err)
	}
	if !answer.OK || answer.Budget.FreeMemorySlots != 33 || answer.Budget.UpdatedAt.Before(asked) {
		t.Errorf("answer %s, want ok and the new greeting's 33 free slots", text)
	}
	if n := fc.Connections(); n != 2 {
		t.Errorf("%d connections after the refresh, want 2", n)
	}
	snap := rt.client.Snapshot()
	if h := snap.Hello(); h == nil || h.FreeMemorySlots != 33 || snap.HelloAt().Before(asked) {
		t.Errorf("hello %+v at %v, want the new greeting's", h, snap.HelloAt())
	}
	if s, _ := snap.State(rf); !s.BatteryLow {
		t.Errorf("state of %s after: %+v, want its battery flag", rf, s)
	}
	testkit.Eventually(t, "the quit", func() bool { return r.lines.Count(sent, nil) > 0 })
	if got := lineStrings(r.lines.Since(sent, nil)); !slices.Equal(got, []string{"q:"}) {
		t.Errorf("sent %q, want only the quit", got)
	}
	if n := r.log.Count(logged, testkit.Msg("snapshot refreshed", "cube", "KEQ0000100", "asked", "true", "user", "manager1")); n != 1 {
		t.Errorf("%d lines of the refresh, want 1:\n%v", n, r.log.Since(logged, nil))
	}
	if code, _ := callAPI(t, ts, "POST", "/api/v1/cubes/KEQ0000999/refresh", manager, ""); code != 404 {
		t.Errorf("another Cube: %d", code)
	}
}

// While the Cube reboots its greeting after the reboot reads it again:
// a refresh is refused, and makes no connection.
func TestRefreshRefusedWhileTheCubeReboots(t *testing.T) {
	t.Parallel()
	r := startServer(t, quiet())
	fc, ts, rt := r.fc, r.ts, r.rt
	fc.SetSilent(true) // the Cube restarts: no greeting for a while
	defer fc.SetSilent(false)
	rt.client.Rebooting(cubelan.RebootWindow)
	testkit.Eventually(t, "the client's dial", func() bool { return fc.Connections() == 2 })
	code, body := do(t, ts, "POST", "/api/v1/cubes/KEQ0000100/refresh", "")
	if code != 409 || !strings.Contains(body["error"].(string), "rebooting") {
		t.Fatalf("refresh while rebooting: %d %v", code, body)
	}
	refusedWithCode(t, "refresh while rebooting", code, body, 409, "cube_rebooting", "null")
	if n := fc.Connections(); n != 2 {
		t.Errorf("%d connections, want the client's own 2", n)
	}
}

// A Cube whose address or serial is not known yet is refused, as its
// reboot is: there is nothing to reconnect to, or nothing reached yet
// (cube_address_not_known_yet, cube_serial_not_known_yet).
func TestRefreshRefusedWithoutAddressOrSerial(t *testing.T) {
	t.Parallel()
	// Discovery only towards a loopback port that nobody answers on.
	nowhere := discovering(Options{DiscoveryAddr: "127.0.0.1:0", DiscoveryTargets: []string{"127.0.0.1:9"}})
	ts := startServer(t, nowhere, withCubes(CubeConfig{Serial: "KEQ0000200"})).ts
	code, body := do(t, ts, "POST", "/api/v1/cubes/KEQ0000200/refresh", "")
	if code != 409 || body["error"] != "the Cube's address is not known yet" {
		t.Errorf("without an address: %d %v", code, body)
	}
	refusedWithCode(t, "refresh without an address", code, body, 409, "cube_address_not_known_yet", "null")
	code, body = do(t, ts, "POST", "/api/v1/cubes/KEQ0000200/reboot", "{}")
	refusedWithCode(t, "reboot without an address", code, body, 409, "cube_address_not_known_yet", "null")
	ts = startServer(t, nowhere, withCubes(CubeConfig{Addr: "127.0.0.1:9"})).ts
	code, body = do(t, ts, "POST", "/api/v1/cubes/127.0.0.1:9/refresh", "")
	if code != 409 || body["error"] != "the Cube's serial is not known yet" {
		t.Errorf("without a serial: %d %v", code, body)
	}
	refusedWithCode(t, "refresh without a serial", code, body, 409, "cube_serial_not_known_yet", "null")
	code, body = do(t, ts, "POST", "/api/v1/cubes/127.0.0.1:9/reboot", "{}")
	refusedWithCode(t, "reboot without a serial", code, body, 409, "cube_serial_not_known_yet", "null")
}
