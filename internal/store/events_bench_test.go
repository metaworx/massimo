// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package store

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"sync"
	"testing"
	"time"
)

// The benchmarks of the log's writer and pages (docs/store.md section 5,
// "Measuring on a box"; AP SuiteGaps v1.0, Block 3), on one store made
// once per run: a copy of a box's store when MASSIMO_STORE_BENCH_COPY
// names one, grown to MASSIMO_STORE_BENCH_ROWS events (90,000) with the
// newest 1000 events of its busiest Cube, cycled; else rows of their own
// as TestEventsPageQuery fills them, 3000 under -short. No writer runs,
// so no prune either. make store-bench builds them for the Raspberry
// Pis; CI runs each once (GUIDELINES/project/CI.md, section 2).
var bench struct {
	once sync.Once
	dir  string
	s    *Store
	cube int64
	tmpl []Event
}

// TestMain closes the benchmarks' store and removes it after the run.
func TestMain(m *testing.M) {
	code := m.Run()
	if bench.s != nil {
		bench.s.Close()
	}
	if bench.dir != "" {
		os.RemoveAll(bench.dir)
	}
	os.Exit(code)
}

// benchStore makes the benchmarks' store on the first call.
func benchStore(b *testing.B) (*Store, int64, []Event) {
	b.Helper()
	bench.once.Do(func() { makeBenchStore(b) })
	if bench.s == nil || len(bench.tmpl) == 0 {
		b.Fatal("the benchmarks' store was not made")
	}
	return bench.s, bench.cube, bench.tmpl
}

func makeBenchStore(b *testing.B) {
	ctx := context.Background()
	dir, err := os.MkdirTemp("", "massimo-store-bench-")
	if err != nil {
		b.Fatal(err)
	}
	bench.dir = dir
	rows := 90000
	if v := os.Getenv("MASSIMO_STORE_BENCH_ROWS"); v != "" {
		if rows, err = strconv.Atoi(v); err != nil {
			b.Fatalf("MASSIMO_STORE_BENCH_ROWS: %v", err)
		}
	}
	copied := os.Getenv("MASSIMO_STORE_BENCH_COPY")
	if copied != "" {
		data, err := os.ReadFile(copied)
		if err != nil {
			b.Fatal(err)
		}
		if err := os.WriteFile(Path(dir), data, 0o600); err != nil {
			b.Fatal(err)
		}
	}
	began := time.Now()
	s, err := Open(dir, Options{Version: "bench", Logger: quiet()})
	if err != nil {
		b.Fatal(err)
	}
	bench.s = s
	opened := time.Since(began)
	if copied != "" {
		if err := s.db.QueryRowContext(ctx, `SELECT cube_id FROM log WHERE cube_id IS NOT NULL GROUP BY cube_id ORDER BY count(*) DESC LIMIT 1`).Scan(&bench.cube); err != nil {
			b.Fatalf("the copy's busiest Cube: %v", err)
		}
		if bench.tmpl, err = s.Events(ctx, EventQuery{CubeID: bench.cube, Limit: MaxEventsPage}); err != nil {
			b.Fatal(err)
		}
	} else {
		if testing.Short() {
			rows = 3000
		}
		cube, err := s.AddCube(ctx, Cube{Serial: "KEQ0000001", Configured: true})
		if err != nil {
			b.Fatal(err)
		}
		bench.cube = cube.ID
		start := time.Now().UTC().Add(-29 * 24 * time.Hour).Truncate(time.Second)
		step := 28 * 24 * time.Hour / time.Duration(rows)
		fill(b, s, cube.ID, rows, start, step)
		fill(b, s, 0, rows/30, start, 30*step)
		for i := range 300 {
			level := LevelInfo
			if i%3 == 0 {
				level = LevelWarn
			}
			bench.tmpl = append(bench.tmpl, Event{CubeID: cube.ID, Kind: "state", Source: "device", Level: level,
				Device: "0abc01", Serial: "MEQ0000001", Data: json.RawMessage(stateData)})
		}
	}
	began = time.Now()
	next, have := 0, benchCount(b, s)
	for have < rows {
		batch := benchBatch(min(1000, rows-have), &next)
		if err := s.writeEvents(ctx, batch); err != nil {
			b.Fatal(err)
		}
		have += len(batch)
	}
	fmt.Printf("store-bench: %s, opened in %v, %d events, %v to grow, the levels%s\n",
		map[bool]string{true: "a copy of " + copied, false: "rows of its own"}[copied != ""],
		opened.Round(time.Millisecond), benchCount(b, s), time.Since(began).Round(time.Millisecond), benchLevels(b, s))
}

// benchBatch is n events from the templates, cycled from *next, at the
// time now, as the writer receives them.
func benchBatch(n int, next *int) []Event {
	out := make([]Event, n)
	for i := range out {
		e := bench.tmpl[*next%len(bench.tmpl)]
		*next++
		e.ID, e.Time = 0, time.Now()
		out[i] = e
	}
	return out
}

func benchCount(b *testing.B, s *Store) int {
	var n int
	if err := s.db.QueryRow(`SELECT count(*) FROM log`).Scan(&n); err != nil {
		b.Fatal(err)
	}
	return n
}

func benchLevels(b *testing.B, s *Store) string {
	rows, err := s.db.Query(`SELECT level, count(*) FROM log GROUP BY level ORDER BY level`)
	if err != nil {
		b.Fatal(err)
	}
	defer rows.Close()
	out := ""
	for rows.Next() {
		var level, n int
		rows.Scan(&level, &n)
		out += fmt.Sprintf(" %s %d", LevelName(level), n)
	}
	return out
}

// The writer's batch of 100 events in one transaction (flushAt).
func BenchmarkWriteEvents(b *testing.B) {
	s, _, _ := benchStore(b)
	ctx := context.Background()
	next := 0
	for b.Loop() {
		if err := s.writeEvents(ctx, benchBatch(flushAt, &next)); err != nil {
			b.Fatal(err)
		}
	}
}

// A page of 200 of the busiest Cube's events: all of them, its warnings
// and above, its errors, and with the server's rows.
func BenchmarkEventsPage(b *testing.B) {
	s, cube, _ := benchStore(b)
	ctx := context.Background()
	warn, errs := LevelWarn, LevelError
	for _, c := range []struct {
		name string
		q    EventQuery
	}{
		{"all", EventQuery{CubeID: cube}},
		{"warnings", EventQuery{CubeID: cube, MinLevel: &warn}},
		{"errors", EventQuery{CubeID: cube, MinLevel: &errs}},
		{"server", EventQuery{CubeID: cube, Server: true}},
	} {
		c.q.Limit = 200
		b.Run(c.name, func(b *testing.B) {
			n := 0
			for b.Loop() {
				page, err := s.Events(ctx, c.q)
				if err != nil {
					b.Fatal(err)
				}
				n = len(page)
			}
			b.ReportMetric(float64(n), "events/page")
		})
	}
}
