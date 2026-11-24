// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package store

import (
	"context"
	"strings"
	"testing"
	"time"
)

// queryPlan is SQLite's EXPLAIN QUERY PLAN of a statement, its details
// joined by "; ".
func queryPlan(t *testing.T, s *Store, query string, args ...any) string {
	t.Helper()
	rows, err := s.db.QueryContext(context.Background(), `EXPLAIN QUERY PLAN `+query, args...)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var details []string
	for rows.Next() {
		var id, parent, unused int
		var detail string
		if err := rows.Scan(&id, &parent, &unused, &detail); err != nil {
			t.Fatal(err)
		}
		details = append(details, detail)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return strings.Join(details, "; ")
}

// The pending commands are found by the partial index commands_pending,
// which SQLite takes only where the query names the states as its WHERE
// does, literally; the drop at a start too (AP StoreRaise v1.2, Block 2).
func TestPendingUsesItsIndex(t *testing.T) {
	t.Parallel()
	s := open(t, t.TempDir())
	if StateQueued != "queued" || StateSent != "sent" {
		t.Fatal("the states differ from those pendingWhere and commands_pending name")
	}
	for name, q := range map[string]string{
		"Pending":        pendingQuery,
		"DropUnfinished": `UPDATE commands SET state = 'dropped' WHERE ` + pendingWhere,
	} {
		if plan := queryPlan(t, s, q); !strings.Contains(plan, "commands_pending") {
			t.Errorf("%s: %s", name, plan)
		}
	}
}

// A page of the audit walks audit_time from its newest end in the page's
// order, by time, then id, without a sort of its own, from the newest
// and before a time. The filter by user finds the rows by the indexes
// audit_user and audit_real_user, by the user's id rather than a join on
// the name, and sorts what the two walks found.
func TestAuditPagesUseTheirIndexes(t *testing.T) {
	t.Parallel()
	s := open(t, t.TempDir())
	at := formatTime(time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC))
	for _, sh := range []struct {
		name  string
		where string
		args  []any
		want  []string
		sorts bool
	}{
		{"from the newest", auditBefore, []any{"", "", 10}, []string{"USING INDEX audit_time"}, false},
		{"before a time", auditBefore, []any{at, at, 10}, []string{"USING INDEX audit_time"}, false},
		{"a user", auditByUser, []any{int64(1), int64(1), "", "", 10}, []string{"audit_user", "audit_real_user"}, true},
		{"a user, before a time", auditByUser, []any{int64(1), int64(1), at, at, 10}, []string{"audit_user", "audit_real_user"}, true},
	} {
		plan := queryPlan(t, s, auditQuery(sh.where, auditPage), sh.args...)
		for _, ix := range sh.want {
			if !strings.Contains(plan, ix) {
				t.Errorf("%s: not by %s: %s", sh.name, ix, plan)
			}
		}
		if !sh.sorts && strings.Contains(plan, "TEMP B-TREE") {
			t.Errorf("%s: sorted: %s", sh.name, plan)
		}
		t.Logf("%-22s %s", sh.name, plan)
	}
}
