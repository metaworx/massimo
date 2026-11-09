// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package store

import (
	"context"
	"slices"
	"testing"
)

// integrationCaps returns the role integration's capabilities, sorted.
func integrationCaps(t *testing.T, s *Store) []string {
	t.Helper()
	rows, err := s.db.Query(`SELECT c.capability FROM role_capabilities c JOIN roles r ON r.id = c.role_id
		WHERE r.key = 'integration' AND r.deleted_at IS NULL ORDER BY c.capability`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var caps []string
	for rows.Next() {
		var c string
		if err := rows.Scan(&c); err != nil {
			t.Fatal(err)
		}
		caps = append(caps, c)
	}
	return caps
}

// Migration 0012 gives the role integration configure where it still
// holds exactly its seeded capabilities, and leaves a role the owner
// narrowed or widened as it is (AP HomeAssistant v1.0, Block 2).
func TestMigration12(t *testing.T) {
	ctx := context.Background()
	for name, tc := range map[string]struct {
		change string
		want   []string
	}{
		"seeded":   {"", []string{"configure", "lock", "read", "setpoint"}},
		"narrowed": {`DELETE FROM role_capabilities WHERE capability = 'lock' AND role_id = (SELECT id FROM roles WHERE key = 'integration')`, []string{"read", "setpoint"}},
		"widened":  {`INSERT INTO role_capabilities (role_id, capability) SELECT id, 'backup' FROM roles WHERE key = 'integration'`, []string{"backup", "lock", "read", "setpoint"}},
		"swapped": {`UPDATE role_capabilities SET capability = 'rename' WHERE capability = 'lock' AND role_id = (SELECT id FROM roles WHERE key = 'integration')`,
			[]string{"read", "rename", "setpoint"}},
	} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			s11 := openAt(t, dir, 11)
			if tc.change != "" {
				if _, err := s11.db.ExecContext(ctx, tc.change); err != nil {
					t.Fatal(err)
				}
			}
			s11.Close()
			s := open(t, dir)
			if got := integrationCaps(t, s); !slices.Equal(got, tc.want) {
				t.Fatalf("integration has %v, want %v", got, tc.want)
			}
		})
	}
}
