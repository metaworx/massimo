// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package store

import (
	"context"
	"slices"
	"testing"
)

// Migration 0022 gives repair to manager, also where the owner narrowed
// its set, and to the role integration with its ceiling; no other role
// gains it, a role of the owner's and integration-v0 included (AP
// RadioHealth v1.2, Block 10, D166-A).
func TestMigration22(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	dir := t.TempDir()
	s21 := openAt(t, dir, 21)
	if _, err := s21.db.ExecContext(ctx, `DELETE FROM role_capabilities WHERE capability = 'pair' AND role_id = (SELECT id FROM roles WHERE key = 'manager')`); err != nil {
		t.Fatal(err)
	}
	// As a binary of version 21 wrote it: this one's writes the uuid of 0024.
	for _, q := range []string{
		`INSERT INTO roles (key, created_at, updated_at) VALUES ('house', '2026-11-15T00:00:00.000Z', '2026-11-15T00:00:00.000Z')`,
		`INSERT INTO role_capabilities (role_id, capability) SELECT id, c FROM roles, (SELECT 'read' AS c UNION ALL SELECT 'structure') WHERE key = 'house'`,
	} {
		if _, err := s21.db.ExecContext(ctx, q); err != nil {
			t.Fatal(err)
		}
	}
	s21.Close()
	s := openFresh(t, dir)
	for key, want := range map[string]bool{RoleManager: true, RoleIntegration: true, RoleOperator: false, RoleGuest: false, RoleViewer: false, "house": false} {
		r, ok, err := s.Role(ctx, key)
		if err != nil || !ok {
			t.Fatalf("role %s: %v %v", key, ok, err)
		}
		if got := slices.Contains(r.Capabilities, "repair"); got != want {
			t.Errorf("role %s has repair %v, want %v: %v", key, got, want, r.Capabilities)
		}
		if key == RoleManager && slices.Contains(r.Capabilities, "pair") {
			t.Errorf("manager got pair back: %v", r.Capabilities)
		}
	}
}
