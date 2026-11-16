// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package store

import (
	"context"
	"strings"
	"testing"
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

// The audit's filter by user finds the rows by the index audit_user, by
// the user's id rather than a join on the name.
func TestAuditUserFilterUsesItsIndex(t *testing.T) {
	s := open(t, t.TempDir())
	if plan := queryPlan(t, s, auditSelect+` WHERE `+auditByUser+` ORDER BY a.id DESC LIMIT ?`, int64(1), "", "", 10); !strings.Contains(plan, "audit_user") {
		t.Fatalf("plan %s", plan)
	}
}
