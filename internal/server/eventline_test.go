// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"

	"go.metaworx.ch/massimo/internal/engine"
	"go.metaworx.ch/massimo/internal/store"
	"go.metaworx.ch/massimo/internal/testkit"
)

// jsonLines reads a JSON log's lines.
func jsonLines(t *testing.T, text string) []map[string]any {
	t.Helper()
	var out []map[string]any
	for _, l := range strings.Split(strings.TrimSpace(text), "\n") {
		if l == "" {
			continue
		}
		var m map[string]any
		if err := json.Unmarshal([]byte(l), &m); err != nil {
			t.Fatalf("not JSON: %q", l)
		}
		out = append(out, m)
	}
	return out
}

// An event's line says its level, its source and kind, its Cube, device,
// serial, command and who started it, then its summary; a server's job
// that failed says so in its message; a line below the logger's level is
// not written.
func TestEventLine(t *testing.T) {
	var buf bytes.Buffer
	log := slog.New(slog.NewJSONHandler(&buf, nil))
	ev := event{Kind: "backup", Source: "system", Level: "info", Device: "0a0101", Serial: "MEQ0000101", Command: 7,
		By: &eventUser{Name: "anna"}, Summary: map[string]any{"reason": "manual", "id": 3}}
	eventLine(log, "KEQ0000100", ev)
	ev.Level, ev.Error, ev.By, ev.userID = "warn", "disk full", nil, 4
	eventLine(log, "KEQ0000100", ev)
	eventLine(log, "", event{Kind: "hello", Source: "cube", Level: "debug"})
	eventLine(log, "", event{Kind: "server_started", Source: "system", Level: "info", Summary: map[string]any{"version": "v1"}})
	lines := jsonLines(t, buf.String())
	if len(lines) != 3 {
		t.Fatalf("%d lines:\n%s", len(lines), buf.String())
	}
	want := map[string]any{"level": "INFO", "msg": "backup taken", "source": "system", "kind": "backup", "cube": "KEQ0000100", "device": "0a0101",
		"serial": "MEQ0000101", "command": float64(7), "by": "anna", "reason": "manual", "id": float64(3)}
	for k, v := range want {
		if lines[0][k] != v {
			t.Errorf("the backup's line: %s = %v, want %v", k, lines[0][k], v)
		}
	}
	if l := lines[1]; l["level"] != "WARN" || l["msg"] != "backup failed" || l["error"] != "disk full" || l["user_id"] != float64(4) {
		t.Errorf("the failed backup's line: %v", l)
	}
	if l := lines[2]; l["msg"] != "server started" || l["cube"] != nil || l["version"] != "v1" {
		t.Errorf("the server's line: %v", l)
	}
}

// Every event the server keeps is one line of the process log, and the
// line and the row say the same: the Cube's connection and its devices'
// states, each once, with no second line of the server's or the Cube
// client's saying it again.
func TestEventLinesMatchTheRows(t *testing.T) {
	log, logs := testkit.LogWatch(nil)
	_, _, s, st := startStored(t, Options{Logger: log})
	s.logServer(systemEvent("backups_pruned", 0, map[string]any{"removed": 2}))
	ctx := context.Background()
	var rows []store.Event
	testkit.Eventually(t, "the connect burst in the store", func() bool {
		st.FlushEvents(ctx)
		var err error
		rows, err = st.Events(ctx, store.EventQuery{CubeID: 1, Server: true})
		if err != nil {
			t.Fatal(err)
		}
		states := 0
		for _, r := range rows {
			if r.Kind == "state" {
				states++
			}
		}
		return states >= 6
	})
	// A line of its own for each row: the lines of the last rows may come
	// after the rows.
	type key struct{ kind, source, device, level string }
	var lines []func(testkit.Record) bool
	for _, r := range rows {
		if r.Level < store.LevelInfo {
			continue
		}
		k := key{r.Kind, r.Source, r.Device, store.LevelName(r.Level)}
		lines = append(lines, func(l testkit.Record) bool {
			kind, ok := l.Attrs["kind"]
			return ok && key{kind, l.Attrs["source"], l.Attrs["device"], strings.ToLower(l.Level.String())} == k
		})
	}
	logs.AwaitAll(t, 0, "a line for each row", lines...)
	for _, l := range logs.Since(0, nil) {
		if _, ok := l.Attrs["kind"]; !ok && (l.Msg == "device state" || l.Msg == "cube connected") {
			t.Errorf("a line of %v without kind: %v", l.Attrs["source"], l)
		}
	}
}

// A step's line says its state, a hold's what the radio waits for.
func TestEventMessages(t *testing.T) {
	for _, c := range []struct {
		ev   event
		want string
	}{
		{event{Kind: "command", Summary: map[string]any{"state": "acknowledged"}}, "command acknowledged"},
		{event{Kind: "hold", Summary: map[string]any{"reason": engine.HoldDutyCycle}}, "radio duty cycle exhausted, holding radio commands until the budget is read again"},
		{event{Kind: "hold", Summary: map[string]any{"reason": engine.HoldMemorySlots}}, "few free memory slots on the cube, holding radio commands"},
		{event{Kind: "login_failed"}, "login failed"},
		{event{Kind: "restart", Error: "no"}, "restart not started"},
	} {
		if got := eventMessage(c.ev); got != c.want {
			t.Errorf("%s: %q, want %q", c.ev.Kind, got, c.want)
		}
	}
}
