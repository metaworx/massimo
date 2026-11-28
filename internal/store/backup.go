// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package store

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"path/filepath"
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

// Snapshot is what Store.Snapshot says of the copy it wrote: its schema
// version and its rows per scope, counted in the copy itself, so that
// they are the copy's, not the live store's a moment later.
type Snapshot struct {
	Schema    int // the copy's schema version, the store's
	MinReader int
	Counts    SnapshotCounts
}

// SnapshotCounts are a snapshot's rows per scope, numbers alone: a
// server backup's manifest carries them in plain (docs/store.md,
// "Server backups"). The removed rows are left out.
type SnapshotCounts struct {
	Users        int `json:"users"` // the users that are not an integration's
	Integrations int `json:"integrations"`
	Cubes        int `json:"cubes"`
	Rooms        int `json:"rooms"`
	Audit        int `json:"audit"`
	Log          int `json:"log"`
}

// snapshotCounts are the queries of SnapshotCounts, in its order.
var snapshotCounts = []string{
	`SELECT count(*) FROM users WHERE ` + regularUser,
	`SELECT count(*) FROM integrations WHERE deleted_at IS NULL`,
	`SELECT count(*) FROM cubes WHERE deleted_at IS NULL`,
	`SELECT count(*) FROM rooms WHERE deleted_at IS NULL`,
	`SELECT count(*) FROM audit`,
	`SELECT count(*) FROM log`,
}

// Snapshot writes a consistent copy of the database to dst, as Backup
// does (copyTo), for a server backup (internal/serverbackup), and counts
// its rows per scope in the copy.
func (s *Store) Snapshot(ctx context.Context, dst string) (Snapshot, error) {
	if err := s.copyTo(ctx, dst); err != nil {
		return Snapshot{}, fmt.Errorf("store: snapshot to %s: %w", dst, err)
	}
	sn := Snapshot{Schema: s.schema, MinReader: s.minReader}
	if err := countSnapshot(ctx, dst, &sn.Counts); err != nil {
		return Snapshot{}, fmt.Errorf("store: snapshot %s: %w", dst, err)
	}
	return sn, nil
}

// countSnapshot counts the rows of the copy at path, opened read-only
// and immutable: nothing writes it, so no lock and no WAL is needed.
func countSnapshot(ctx context.Context, path string, c *SnapshotCounts) error {
	db, err := openImmutable(path)
	if err != nil {
		return err
	}
	defer db.Close()
	for i, n := range []*int{&c.Users, &c.Integrations, &c.Cubes, &c.Rooms, &c.Audit, &c.Log} {
		if err := db.QueryRowContext(ctx, snapshotCounts[i]).Scan(n); err != nil {
			return err
		}
	}
	return nil
}

// openImmutable opens the store file at path read-only and immutable, a
// copy nothing writes.
func openImmutable(path string) (*sql.DB, error) {
	q := url.Values{}
	q.Set("mode", "ro")
	q.Set("immutable", "1")
	return sql.Open("sqlite", "file:"+filepath.ToSlash(path)+"?"+q.Encode())
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
