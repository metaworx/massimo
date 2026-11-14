// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

import (
	"log/slog"
	"net"
	"net/http/httptest"
	"testing"
	"time"

	"go.metaworx.ch/massimo/internal/cubelan/fakecube"
)

// elsewhereMsg is the log line of a Cube found at another address.
const elsewhereMsg = `msg="cube answers discovery from another address than configured"`

// storedElsewhere starts a server over a stored Cube with a fixed
// address at 127.0.0.1, which a responder at 127.0.0.3 answers
// discovery for, and returns the server, the Cube's configured address
// and the log, Debug included.
func storedElsewhere(t *testing.T) (*httptest.Server, string, *syncBuffer) {
	t.Helper()
	elsewhere, err := fakecube.StartDiscovery("127.0.0.3:0", "KEQ0000100", 0x0a0107)
	if err != nil {
		t.Skip(err)
	}
	t.Cleanup(func() { elsewhere.Close() })
	logs := &syncBuffer{}
	_, ts, _, _ := startStored(t, Options{DiscoveryWait: 200 * time.Millisecond, DiscoveryAddr: "127.0.0.1:0",
		DiscoveryTargets: []string{elsewhere.Addr()},
		Logger:           slog.New(slog.NewTextHandler(logs, &slog.HandlerOptions{Level: slog.LevelDebug}))})
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
	log := logs.String()
	if n := countLines(log, "level=WARN", elsewhereMsg, "found=127.0.0.3"); n != 1 {
		t.Errorf("%d Warn lines over three runs, want 1:\n%s", n, log)
	}
	if n := countLines(log, "level=DEBUG", elsewhereMsg); n < 2 {
		t.Errorf("%d Debug lines for the repeats, want 2 or more:\n%s", n, log)
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
			if n := countLines(logs.String(), "level=WARN", elsewhereMsg); n != 1 {
				t.Errorf("%d Warn lines, want the first only:\n%s", n, logs.String())
			}
		})
	}
}
