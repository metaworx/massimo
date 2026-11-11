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
	"time"

	"go.metaworx.ch/massimo/internal/store"
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
	var buf syncBuffer
	_, _, s, st := startStored(t, Options{Logger: slog.New(slog.NewJSONHandler(&buf, nil))})
	s.logServer(systemEvent("backups_pruned", 0, map[string]any{"removed": 2}))
	ctx := context.Background()
	var rows []store.Event
	eventually(t, "the connect burst in the store", func() bool {
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
	time.Sleep(50 * time.Millisecond) // the lines of the last rows
	type key struct{ kind, source, device, level string }
	lines := map[key]int{}
	for _, l := range jsonLines(t, buf.String()) {
		if l["kind"] == nil {
			if m, _ := l["msg"].(string); m == "device state" || m == "cube connected" {
				t.Errorf("a line of %v without kind: %v", l["source"], l)
			}
			continue
		}
		dev, _ := l["device"].(string)
		lines[key{l["kind"].(string), l["source"].(string), dev, strings.ToLower(l["level"].(string))}]++
	}
	for _, r := range rows {
		if r.Level < store.LevelInfo {
			continue
		}
		k := key{r.Kind, r.Source, r.Device, store.LevelName(r.Level)}
		if lines[k] == 0 {
			t.Errorf("no line for the row %+v; lines %v", k, lines)
		}
		lines[k]--
	}
}
