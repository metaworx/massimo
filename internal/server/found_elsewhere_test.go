// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

import (
	"log/slog"
	"net"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"go.metaworx.ch/massimo/internal/cubelan/fakecube"
	"go.metaworx.ch/massimo/internal/testkit"
)

// elsewhereMsg is the log message of a Cube found at another address.
const elsewhereMsg = "cube answers discovery from another address than configured"

// storedElsewhere starts a server over a stored Cube with a fixed
// address at 127.0.0.1, which a responder at 127.0.0.3 answers
// discovery for, and returns the server, the Cube's configured address
// and the watch of its log, Debug included.
func storedElsewhere(t *testing.T) (*httptest.Server, string, *testkit.Watch[testkit.Record]) {
	t.Helper()
	elsewhere, err := fakecube.StartDiscovery("127.0.0.3:0", "KEQ0000100", 0x0a0107)
	if err != nil {
		testkit.BindOrSkip(t, err)
	}
	t.Cleanup(func() { elsewhere.Close() })
	log, logs := testkit.LogWatch(slog.LevelDebug)
	ts := startServer(t, stored(), withOptions(Options{DiscoveryWait: 200 * time.Millisecond, DiscoveryAddr: "127.0.0.1:0",
		DiscoveryTargets: []string{elsewhere.Addr()}, Logger: log})).ts
	_, out := do(t, ts, "GET", "/api/v1/cubes/KEQ0000100", "")
	addr, _ := out["configured_address"].(string)
	if addr == "" {
		t.Fatalf("no configured address: %v", out)
	}
	return ts, addr, logs
}

// elsewhereOf reads a Cube's found_elsewhere from an answer, nil where
// it has none.
func elsewhereOf(out map[string]any) map[string]any {
	f, _ := out["found_elsewhere"].(map[string]any)
	return f
}

// A Cube with a fixed address that answers discovery at another one
// has found_elsewhere, with the configured port; the log warns once and
// says so at Debug for the runs that repeat it.
func TestAFixedCubeFoundElsewhere(t *testing.T) {
	ts, addr, logs := storedElsewhere(t)
	for range 3 {
		if code, out := postDiscovery(t, ts); code != 200 {
			t.Fatalf("discover %d %v", code, out)
		}
	}
	_, out := do(t, ts, "GET", "/api/v1/cubes/KEQ0000100", "")
	_, port, _ := net.SplitHostPort(addr)
	f := elsewhereOf(out)
	if f == nil || f["address"] != "127.0.0.3:"+port || f["configured"] != addr || f["since"] == nil {
		t.Fatalf("found_elsewhere %v", out["found_elsewhere"])
	}
	warnings := logs.Since(0, func(r testkit.Record) bool {
		return r.Level == slog.LevelWarn && r.Msg == elsewhereMsg && strings.HasPrefix(r.Attrs["found"], "127.0.0.3")
	})
	if n := len(warnings); n != 1 {
		t.Errorf("%d Warn lines found at 127.0.0.3 over three runs, want 1: %v", n, logs.Since(0, testkit.Msg(elsewhereMsg)))
	}
	if n := logs.Count(0, testkit.MsgAt(slog.LevelDebug, elsewhereMsg)); n < 2 {
		t.Errorf("%d Debug lines for the repeats, want 2 or more: %v", n, logs.Since(0, testkit.Msg(elsewhereMsg)))
	}
}

// Taking the found address, or following discovery, clears
// found_elsewhere at once, and the next run does not warn again.
func TestFoundElsewhereClears(t *testing.T) {
	for _, tc := range []struct {
		name string
		body func(found string) string
	}{
		{"the found address", func(found string) string { return `{"address":"` + found + `"}` }},
		{"follow discovery", func(string) string { return `{"follow_discovery":true}` }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ts, _, logs := storedElsewhere(t)
			if code, out := postDiscovery(t, ts); code != 200 {
				t.Fatalf("discover %d %v", code, out)
			}
			_, out := do(t, ts, "GET", "/api/v1/cubes/KEQ0000100", "")
			f := elsewhereOf(out)
			if f == nil {
				t.Fatalf("no found_elsewhere: %v", out)
			}
			code, out := do(t, ts, "PUT", "/api/v1/cubes/KEQ0000100", tc.body(f["address"].(string)))
			if code != 200 || elsewhereOf(out) != nil {
				t.Fatalf("put %d, found_elsewhere %v", code, out["found_elsewhere"])
			}
			if code, out := postDiscovery(t, ts); code != 200 {
				t.Fatalf("discover %d %v", code, out)
			}
			if _, out := do(t, ts, "GET", "/api/v1/cubes/KEQ0000100", ""); elsewhereOf(out) != nil {
				t.Errorf("found_elsewhere after the next run: %v", out["found_elsewhere"])
			}
			if warnings := logs.Since(0, testkit.MsgAt(slog.LevelWarn, elsewhereMsg)); len(warnings) != 1 {
				t.Errorf("%d Warn lines, want the first only: %v", len(warnings), warnings)
			}
		})
	}
}
