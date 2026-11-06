// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package store

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"strings"
	"testing"
	"time"
)

// Discovery answers for a configured Cube are no configuration change:
// version, updated_at and origin stay; an unchanged answer writes at most
// last_seen, once per run.
func TestDiscoveryKeepsProvenance(t *testing.T) {
	s := open(t, t.TempDir())
	ctx := context.Background()
	t0 := time.Date(2026, 11, 6, 8, 0, 0, 0, time.UTC)
	added, err := s.AddCube(ctx, Cube{Serial: "KEQ0000001", Configured: true, Address: "10.0.0.1", Source: SourceEnv})
	if err != nil {
		t.Fatal(err)
	}
	d := Discovered{Serial: "KEQ0000001", Address: "10.0.0.1", MAC: "00:1a:22:00:00:01", RFAddress: "0aa001", Firmware: "1.1.3", At: t0}
	for i := 0; i < 3; i++ {
		d.At = t0.Add(time.Duration(i) * 30 * time.Minute)
		c, isNew, err := s.NoteDiscovered(ctx, d)
		if err != nil || isNew || !c.LastSeen.Equal(d.At) {
			t.Fatalf("answer %d: %+v %v %v", i, c, isNew, err)
		}
	}
	row, _, _ := s.Cube(ctx, "KEQ0000001")
	if row.Version != added.Version || !row.UpdatedAt.Equal(added.UpdatedAt) || row.Origin != added.Origin || !row.Configured || row.Source != SourceEnv {
		t.Fatalf("provenance changed by discovery: %+v, was %+v", row, added)
	}
	if row.MAC != d.MAC || !row.FirstSeen.Equal(t0) || !row.LastSeen.Equal(t0) {
		t.Fatalf("discovery columns %+v (last_seen is written once per run)", row)
	}
	// A changed answer writes the discovery columns, still without a
	// version.
	d.Address, d.MAC, d.At = "10.0.0.9", "", t0.Add(2*time.Hour)
	if _, _, err := s.NoteDiscovered(ctx, d); err != nil {
		t.Fatal(err)
	}
	row, _, _ = s.Cube(ctx, "KEQ0000001")
	if row.DiscoveredAddress != "10.0.0.9" || row.MAC != "00:1a:22:00:00:01" || !row.LastSeen.Equal(d.At) || row.Version != added.Version {
		t.Fatalf("changed answer %+v", row)
	}
}

// tombstone marks a row deleted, as sync will.
func tombstone(t *testing.T, s *Store, serial string) {
	t.Helper()
	if _, err := s.db.Exec(`UPDATE cubes SET deleted_at = ? WHERE serial = ?`, s.nowText(), serial); err != nil {
		t.Fatal(err)
	}
}

// A tombstoned row comes back as found through discovery, and as the
// new values through AddCube, keeping its identifier.
func TestTombstonedCubeComesBack(t *testing.T) {
	s := open(t, t.TempDir())
	ctx := context.Background()
	first, err := s.AddCube(ctx, Cube{Serial: "KEQ0000001", Configured: true, Name: "Old"})
	if err != nil {
		t.Fatal(err)
	}
	tombstone(t, s, "KEQ0000001")
	if all, _ := s.Cubes(ctx); len(all) != 0 {
		t.Fatalf("tombstoned row listed: %+v", all)
	}
	c, isNew, err := s.NoteDiscovered(ctx, Discovered{Serial: "KEQ0000001", Address: "10.0.0.1", RFAddress: "0aa001"})
	if err != nil || !isNew || c.Serial != "KEQ0000001" || c.Configured || c.Source != SourceDiscovery || c.UUID != first.UUID || c.Version != first.Version+1 {
		t.Fatalf("discovery of a tombstoned row: %+v %v %v", c, isNew, err)
	}

	tombstone(t, s, "KEQ0000001")
	c, err = s.AddCube(ctx, Cube{Serial: "KEQ0000001", Configured: true, Name: "New", Source: SourceUI})
	if err != nil || c.Name != "New" || !c.Configured || c.UUID != first.UUID || c.Version != first.Version+2 {
		t.Fatalf("add over a tombstoned row: %+v %v", c, err)
	}
	if _, err := s.AddCube(ctx, Cube{Serial: "KEQ0000001"}); !errors.Is(err, ErrExists) {
		t.Fatalf("add over a live row: %v", err)
	}
}

// A copy left by an earlier attempt does not keep the store from
// starting; a failed migration removes its copy; a file that has moved
// on is not copied again.
func TestCopyBeforeLeftovers(t *testing.T) {
	dir := t.TempDir()
	// The file starts at version 1, as a box updated from v0.7.0-alpha.1 has it.
	s, err := Open(dir, Options{Logger: quiet(), migrations: testMigrations(t, nil), readerVersion: 1})
	if err != nil {
		t.Fatal(err)
	}
	s.Close()
	copyPath := Path(dir) + ".before-2"

	// A breaking migration that fails: the file stays at 1, no copy.
	bad := testMigrations(t, map[string]string{"0002_bad.sql": "-- min_reader: 2\nALTER TABLE nowhere ADD COLUMN x TEXT;\n"})
	if _, err := Open(dir, Options{Logger: quiet(), migrations: bad}); err == nil {
		t.Fatal("bad migration applied")
	}
	if _, err := os.Stat(copyPath); err == nil {
		t.Fatal("copy left behind by a failed migration")
	}

	// A leftover copy (a crash after the copy) is replaced.
	os.WriteFile(copyPath, []byte("leftover"), 0o600)
	good := testMigrations(t, map[string]string{"0002_good.sql": "-- min_reader: 2\nALTER TABLE cubes ADD COLUMN x TEXT NOT NULL DEFAULT '';\n"})
	s, err = Open(dir, Options{Logger: quiet(), migrations: good})
	if err != nil {
		t.Fatalf("leftover copy: %v", err)
	}
	if v, _ := s.SchemaVersion(); v != 2 {
		t.Fatalf("version %d", v)
	}
	db, err := sql.Open("sqlite", copyPath)
	if err != nil {
		t.Fatal(err)
	}
	var v int
	if err := db.QueryRow(`SELECT max(version) FROM schema_version`).Scan(&v); err != nil || v != 1 {
		t.Fatalf("the copy is not the file at version 1: %d %v", v, err)
	}
	db.Close()

	// The file has moved on: no copy of version 2 is made from it.
	if _, err := s.copyBefore(context.Background(), 2, 2); err == nil || !strings.Contains(err.Error(), "not made again") {
		t.Fatalf("copy of a file that moved on: %v", err)
	}
	s.Close()
}
