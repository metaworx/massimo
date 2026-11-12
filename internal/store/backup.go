// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package store

import (
	"context"
	"fmt"
)

// Backup writes a consistent copy of the database to dst (which must not
// exist), mode 0600, while the server may be running: a copy of
// massimo.db alone can miss the transactions still in the WAL.
func (s *Store) Backup(ctx context.Context, dst string) error {
	if err := s.copyTo(ctx, dst); err != nil {
		return fmt.Errorf("store: backup to %s: %w", dst, err)
	}
	return nil
}

// Info is what massimo store info prints.
type Info struct {
	Path      string
	Schema    int // the file's schema version
	MinReader int
	Binary    int // the schema version this binary is built for
	NodeID    string
	TenantID  string
	Counts    map[string]int // rows per table
}

// infoTables are the tables Info counts.
var infoTables = []string{"cubes", "settings", "roles", "users", "tokens", "preferences", "log", "audit", "backups", "commands"}

// Info reads the file's version, identity and row counts.
func (s *Store) Info(ctx context.Context) (Info, error) {
	in := Info{Path: s.path, Schema: s.schema, MinReader: s.minReader, Binary: SchemaVersion(), NodeID: s.nodeID, Counts: map[string]int{}}
	var err error
	if in.TenantID, err = s.TenantID(); err != nil {
		return in, err
	}
	for _, t := range infoTables {
		var n int
		// The names come from infoTables, not from input.
		if err := s.db.QueryRowContext(ctx, `SELECT count(*) FROM `+t).Scan(&n); err != nil {
			return in, fmt.Errorf("store: count %s: %w", t, err)
		}
		in.Counts[t] = n
	}
	return in, nil
}
