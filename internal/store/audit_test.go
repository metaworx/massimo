// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package store

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

// Audit rows are written with the user and the Cube joined in on
// reading, listed newest first, paged by time and filtered by user.
func TestAudit(t *testing.T) {
	clock := time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC)
	s, err := Open(t.TempDir(), Options{Logger: quiet(), Now: func() time.Time { return clock }})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx := context.Background()
	anna, _ := s.AddUser(ctx, User{Name: "anna", Role: RoleOperator}, "pw", 0)
	cube, _ := s.AddCube(ctx, Cube{Serial: "KEQ0000001", Configured: true})
	for i, e := range []AuditEntry{
		{UserID: anna.ID, Remote: "10.0.0.5", Action: "setpoint", CubeID: cube.ID, Target: "room:3", Detail: json.RawMessage(`{"temp":21}`)},
		{Remote: RemoteCLI, Action: "user.add", Target: "user:bob"},
		{UserID: anna.ID, Remote: "10.0.0.5", Action: "pairing.start", CubeID: cube.ID, Result: ResultRefused},
	} {
		clock = clock.Add(time.Minute)
		if _, err := s.Audit(ctx, e); err != nil {
			t.Fatalf("row %d: %v", i, err)
		}
	}
	if _, err := s.Audit(ctx, AuditEntry{Action: "x", Result: "maybe"}); err == nil {
		t.Fatal("bad result accepted")
	}
	if _, err := s.Audit(ctx, AuditEntry{Action: "x", Detail: json.RawMessage(`nope`)}); err == nil {
		t.Fatal("bad detail accepted")
	}
	all, err := s.AuditList(ctx, time.Time{}, 0, "")
	if err != nil || len(all) != 3 || all[0].Action != "pairing.start" || all[0].Result != ResultRefused || all[0].User != "anna" || all[0].Cube != "KEQ0000001" || all[2].Action != "setpoint" {
		t.Fatalf("list %+v %v", all, err)
	}
	if string(all[2].Detail) != `{"temp":21}` || string(all[1].Detail) != "{}" || all[1].User != "" || all[1].Remote != RemoteCLI {
		t.Fatalf("details %+v", all)
	}
	if page, _ := s.AuditList(ctx, all[0].Time, 1, ""); len(page) != 1 || page[0].Action != "user.add" {
		t.Fatalf("page %+v", page)
	}
	if mine, _ := s.AuditList(ctx, time.Time{}, 10, "ANNA"); len(mine) != 2 || mine[1].Action != "setpoint" {
		t.Fatalf("by user %+v", mine)
	}
	if none, _ := s.AuditList(ctx, time.Time{}, 10, "bob"); len(none) != 0 {
		t.Fatalf("unknown user %+v", none)
	}
}
