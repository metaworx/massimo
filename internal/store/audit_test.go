// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package store

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"
)

// Audit rows are written with the user and the Cube joined in on
// reading, listed newest first, paged by time and filtered by user.
func TestAudit(t *testing.T) {
	t.Parallel()
	clock := time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC)
	s, err := Open(copied(t), Options{Logger: quiet(), Now: func() time.Time { return clock }})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx := context.Background()
	anna, _ := s.AddUser(ctx, User{Name: "anna", Role: RoleOperator}, "pw", 0)
	cube, _ := s.AddCube(ctx, Cube{Serial: "KEQ0000001", Configured: true})
	for i, e := range []AuditEntry{
		{EffectiveUserID: anna.ID, Remote: "10.0.0.5", Action: "setpoint", CubeID: cube.ID, Target: "room:3", Detail: json.RawMessage(`{"temp":21}`)},
		{Remote: RemoteCLI, Action: "user.add", Target: "user:bob"},
		{EffectiveUserID: anna.ID, Remote: "10.0.0.5", Action: "pairing.start", CubeID: cube.ID, Result: ResultRefused},
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

// Paging by time lists every row once, newest first by time, also a row
// written after newer ones with an older time, as the server writes a
// row when its request is answered with the time it came, and a row of
// refused requests at its minute's end with the time of the minute's
// first: each minute here has three rows and then such a row, at each
// page size.
func TestAuditPagesListEveryRowOnce(t *testing.T) {
	t.Parallel()
	s, err := Open(copied(t), Options{Logger: quiet()})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx := context.Background()
	start := time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC)
	var ids []int64
	for minute := range 4 {
		at := start.Add(time.Duration(minute) * time.Minute)
		for _, e := range []AuditEntry{
			{Time: at.Add(10 * time.Second), Remote: "10.0.0.5", Action: "setpoint"},
			{Time: at.Add(20 * time.Second), Remote: "10.0.0.5", Action: "setpoint"},
			{Time: at.Add(40 * time.Second), Remote: "10.0.0.5", Action: "setpoint"},
			{Time: at.Add(5 * time.Second), Remote: "192.0.2.10", Action: "setpoint", Result: ResultRefused},
		} {
			id, err := s.Audit(ctx, e)
			if err != nil {
				t.Fatal(err)
			}
			ids = append(ids, id)
		}
	}
	all, err := s.AuditList(ctx, time.Time{}, 0, "")
	if err != nil || len(all) != len(ids) {
		t.Fatalf("%d rows, %v", len(all), err)
	}
	for i := 1; i < len(all); i++ {
		if !all[i].Time.Before(all[i-1].Time) {
			t.Errorf("row %d at %v listed after row %d at %v", all[i].ID, all[i].Time.Format(time.TimeOnly), all[i-1].ID, all[i-1].Time.Format(time.TimeOnly))
			break
		}
	}
	for limit := 1; limit <= 5; limit++ {
		seen := map[int64]int{}
		for before, pages := (time.Time{}), 0; ; pages++ {
			if pages > len(ids) {
				t.Fatalf("limit %d: more pages than rows", limit)
			}
			page, err := s.AuditList(ctx, before, limit, "")
			if err != nil {
				t.Fatal(err)
			}
			if len(page) == 0 {
				break
			}
			for _, e := range page {
				seen[e.ID]++
			}
			before = page[len(page)-1].Time
		}
		var wrong []string
		for _, id := range ids {
			if seen[id] != 1 {
				wrong = append(wrong, fmt.Sprintf("row %d %d times", id, seen[id]))
			}
		}
		if len(wrong) > 0 {
			t.Errorf("limit %d: %d of %d rows not listed once: %s", limit, len(wrong), len(ids), strings.Join(wrong, ", "))
		}
	}
}
