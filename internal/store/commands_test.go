// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package store

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

// A parent command and its steps move through the states; a key answers
// a retry; the prune clears keys and, with a retention, finished rows.
func TestCommands(t *testing.T) {
	clock := time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC)
	s, err := Open(t.TempDir(), Options{Logger: quiet(), Now: func() time.Time { return clock }})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx := context.Background()
	anna, _ := s.AddUser(ctx, User{Name: "anna", Role: RoleOperator}, "pw", 0)
	cube, _ := s.AddCube(ctx, Cube{Serial: "KEQ0000001", Configured: true})
	p, err := s.AddCommand(ctx, Command{CubeID: cube.ID, UserID: anna.ID, Key: "k1", Action: "device.update", Target: "device:0a010d", Request: json.RawMessage(`{"method":"PUT"}`)})
	if err != nil || p.State != StateQueued || p.UUID == "" || p.Key != "k1" || p.CubeID != cube.ID || len(p.Steps) != 0 {
		t.Fatalf("parent %+v %v", p, err)
	}
	if _, err := s.AddCommand(ctx, Command{Request: json.RawMessage(`nope`)}); err == nil {
		t.Fatal("bad request accepted")
	}
	s1, err := s.AddStep(ctx, p.ID, Command{Target: "device:0a010d", Request: json.RawMessage(`{"key":"move:out"}`)})
	if err != nil || s1.ParentID != p.ID || s1.Action != ActionStep || s1.CubeID != cube.ID || s1.UserID != anna.ID || s1.State != StateQueued {
		t.Fatalf("step %+v %v", s1, err)
	}
	s2, _ := s.AddStep(ctx, p.ID, Command{Target: "device:0a010d"})
	if _, err := s.AddStep(ctx, 9999, Command{}); !errors.Is(err, ErrNoCommand) {
		t.Fatalf("step of nothing: %v", err)
	}
	if _, err := s.AddStep(ctx, s1.ID, Command{}); !errors.Is(err, ErrNoCommand) {
		t.Fatalf("step of a step: %v", err)
	}
	if err := s.SetCommandState(ctx, s1.ID, StateSent, 0, nil); err != nil {
		t.Fatal(err)
	}
	if got, _, _ := s.Command(ctx, p.ID); got.State != StateSent || got.Steps[0].State != StateSent || got.Steps[1].State != StateQueued {
		t.Fatalf("after the first step was sent: %+v", got)
	}
	if err := s.SetCommandState(ctx, s1.ID, StateAcknowledged, 0, json.RawMessage(`{"discarded":false}`)); err != nil {
		t.Fatal(err)
	}
	s.SetCommandState(ctx, s2.ID, StateDropped, 0, json.RawMessage(`{"error":"superseded"}`))
	if err := s.SetCommandState(ctx, p.ID, "lost", 0, nil); err == nil {
		t.Fatal("bad state accepted")
	}
	if err := s.SetCommandState(ctx, 9999, StateFailed, 0, nil); !errors.Is(err, ErrNoCommand) {
		t.Fatalf("state of nothing: %v", err)
	}
	clock = clock.Add(time.Second)
	if err := s.SetCommandState(ctx, p.ID, StateAcknowledged, 200, json.RawMessage(`{"ok":true}`)); err != nil {
		t.Fatal(err)
	}
	got, ok, err := s.CommandByUUID(ctx, p.UUID)
	if err != nil || !ok || got.State != StateAcknowledged || got.Status != 200 || string(got.Result) != `{"ok":true}` || !got.FinishedAt.Equal(clock) || len(got.Steps) != 2 || got.Steps[1].State != StateDropped || got.Steps[0].FinishedAt.IsZero() {
		t.Fatalf("finished %+v %v %v", got, ok, err)
	}
	if _, ok, _ := s.CommandByUUID(ctx, "nope"); ok {
		t.Fatal("unknown uuid found")
	}

	// The key answers the same user's retry, not another user's.
	again, err := s.AddCommand(ctx, Command{UserID: anna.ID, Key: "k1", Action: "device.update"})
	if !errors.Is(err, ErrKeyExists) || again.ID != p.ID || len(again.Steps) != 2 {
		t.Fatalf("retry %+v %v", again, err)
	}
	bob, _ := s.AddUser(ctx, User{Name: "bob", Role: RoleOperator}, "pw", 0)
	other, err := s.AddCommand(ctx, Command{UserID: bob.ID, Key: "k1", Action: "setpoint"})
	if err != nil || other.ID == p.ID {
		t.Fatalf("another user's key %+v %v", other, err)
	}
	anon, err := s.AddCommand(ctx, Command{Key: "k1", Action: "setpoint"})
	if err != nil || anon.UserID != 0 {
		t.Fatalf("anonymous key %+v %v", anon, err)
	}
	if _, err := s.AddCommand(ctx, Command{Key: "k1", Action: "setpoint"}); !errors.Is(err, ErrKeyExists) {
		t.Fatalf("anonymous retry: %v", err)
	}

	// Unfinished rows are dropped at a start.
	if pending, _ := s.Pending(ctx); len(pending) != 2 {
		t.Fatalf("pending %+v", pending)
	}
	n, err := s.DropUnfinished(ctx, "server restarted")
	if err != nil || n != 2 {
		t.Fatalf("dropped %d %v", n, err)
	}
	if got, _, _ := s.Command(ctx, other.ID); got.State != StateDropped || string(got.Result) != `{"error":"server restarted"}` || got.FinishedAt.IsZero() {
		t.Fatalf("dropped row %+v", got)
	}
	if pending, _ := s.Pending(ctx); len(pending) != 0 {
		t.Fatalf("pending after the drop %+v", pending)
	}

	// After 24 hours the key is free again; the rows stay.
	clock = clock.Add(25 * time.Hour)
	fresh, err := s.AddCommand(ctx, Command{UserID: anna.ID, Key: "k1", Action: "setpoint"})
	if err != nil || fresh.ID == p.ID {
		t.Fatalf("key after a day %+v %v", fresh, err)
	}
	if old, _, _ := s.Command(ctx, p.ID); old.Key != "" || old.State != StateAcknowledged {
		t.Fatalf("old row %+v", old)
	}

	// A retention removes finished parents with their steps and unlinks
	// their audit rows.
	auditID, _ := s.Audit(ctx, AuditEntry{Action: "device.update", CommandID: p.ID})
	if err := s.SetSetting(ctx, CommandsKeepDaysKey, 1); err != nil {
		t.Fatal(err)
	}
	if d, _ := s.CommandsKeepDays(ctx); d != 1 {
		t.Fatalf("keep days %d", d)
	}
	clock = clock.Add(48 * time.Hour)
	s.AddCommand(ctx, Command{Action: "setpoint"})
	if _, ok, _ := s.Command(ctx, p.ID); ok {
		t.Fatal("finished parent kept past the retention")
	}
	if _, ok, _ := s.Command(ctx, s1.ID); ok {
		t.Fatal("step kept past the retention")
	}
	if _, ok, _ := s.Command(ctx, fresh.ID); !ok {
		t.Fatal("an unfinished row was removed")
	}
	rows, _ := s.AuditList(ctx, time.Time{}, 10, "")
	for _, r := range rows {
		if r.ID == auditID && r.CommandID != 0 {
			t.Fatalf("audit row still points at the removed command: %+v", r)
		}
	}
}

// A final row stays final when sent comes late, taking the status only;
// a status below zero keeps the one the row has.
func TestCommandStateOrder(t *testing.T) {
	s := open(t, t.TempDir())
	ctx := context.Background()
	p, _ := s.AddCommand(ctx, Command{Action: "restore.start"})
	// The run ends before the answer is recorded.
	if err := s.SetCommandState(ctx, p.ID, StateAcknowledged, -1, json.RawMessage(`{"sent":0}`)); err != nil {
		t.Fatal(err)
	}
	if err := s.SetCommandState(ctx, p.ID, StateSent, 202, json.RawMessage(`{"id":"x"}`)); err != nil {
		t.Fatal(err)
	}
	got, _, _ := s.Command(ctx, p.ID)
	if got.State != StateAcknowledged || got.Status != 202 || string(got.Result) != `{"sent":0}` || got.FinishedAt.IsZero() {
		t.Fatalf("late sent: %+v", got)
	}
	// The other order: the answer first, then the run.
	q, _ := s.AddCommand(ctx, Command{Action: "restore.start"})
	s.SetCommandState(ctx, q.ID, StateSent, 202, json.RawMessage(`{"id":"y"}`))
	s.SetCommandState(ctx, q.ID, StateFailed, -1, json.RawMessage(`{"error":"step 2"}`))
	got, _, _ = s.Command(ctx, q.ID)
	if got.State != StateFailed || got.Status != 202 || string(got.Result) != `{"error":"step 2"}` {
		t.Fatalf("run after the answer: %+v", got)
	}
	if err := s.SetCommandState(ctx, 9999, StateSent, 0, nil); !errors.Is(err, ErrNoCommand) {
		t.Fatalf("sent for nothing: %v", err)
	}
}

// The commands are pruned at most once an hour, not on every write: a
// row past the retention goes with the first write an hour after the
// last prune.
func TestCommandsPrunedHourly(t *testing.T) {
	clock := time.Date(2026, 11, 7, 12, 0, 0, 0, time.UTC)
	s, err := Open(t.TempDir(), Options{Logger: quiet(), Now: func() time.Time { return clock }})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx := context.Background()
	if err := s.SetSetting(ctx, CommandsKeepDaysKey, 1); err != nil {
		t.Fatal(err)
	}
	s.AddCommand(ctx, Command{Action: "setpoint"}) // the first write prunes
	old, _ := s.AddCommand(ctx, Command{Action: "setpoint"})
	s.SetCommandState(ctx, old.ID, StateAcknowledged, 200, nil)
	if _, err := s.db.ExecContext(ctx, `UPDATE commands SET finished_at = ? WHERE id = ?`, formatTime(clock.Add(-72*time.Hour)), old.ID); err != nil {
		t.Fatal(err)
	}
	// By its uuid: SQLite gives the next row the id a removed last one had.
	clock = clock.Add(time.Minute)
	s.AddCommand(ctx, Command{Action: "setpoint"})
	if _, ok, _ := s.CommandByUUID(ctx, old.UUID); !ok {
		t.Fatal("pruned again a minute after the last prune")
	}
	clock = clock.Add(time.Hour)
	s.AddCommand(ctx, Command{Action: "setpoint"})
	if _, ok, _ := s.CommandByUUID(ctx, old.UUID); ok {
		t.Fatal("not pruned an hour after the last prune")
	}
}

// The Idempotency-Key lookup is served by the index on (user_id,
// idempotency_key), and a key is found for its 24 hours only, whenever
// the prune last ran.
func TestKeyLookup(t *testing.T) {
	clock := time.Date(2026, 11, 7, 12, 0, 0, 0, time.UTC)
	s, err := Open(t.TempDir(), Options{Logger: quiet(), Now: func() time.Time { return clock }})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx := context.Background()
	rows, err := s.db.QueryContext(ctx, `EXPLAIN QUERY PLAN `+commandByKey, nil, "k", formatTime(clock))
	if err != nil {
		t.Fatal(err)
	}
	var plan []string
	for rows.Next() {
		var id, parent, unused int
		var detail string
		rows.Scan(&id, &parent, &unused, &detail)
		plan = append(plan, detail)
	}
	rows.Close()
	if !strings.Contains(strings.Join(plan, "; "), "USING INDEX commands_key") {
		t.Fatalf("the key's lookup: %v", plan)
	}
	if _, err := s.AddCommand(ctx, Command{Key: "k", Action: "setpoint"}); err != nil {
		t.Fatal(err)
	}
	clock = clock.Add(23 * time.Hour)
	if _, err := s.AddCommand(ctx, Command{Key: "k", Action: "setpoint"}); !errors.Is(err, ErrKeyExists) {
		t.Fatalf("within its 24 hours: %v", err)
	}
	clock = clock.Add(90 * time.Minute)
	if _, err := s.AddCommand(ctx, Command{Key: "k", Action: "setpoint"}); err != nil {
		t.Fatalf("after its 24 hours: %v", err)
	}
}
