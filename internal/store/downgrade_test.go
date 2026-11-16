// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package store

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"testing"
	"testing/fstest"
)

// laterMigrations is the embedded migrations and extra, as a later
// binary would have them.
func laterMigrations(t *testing.T, extra map[string]string) fstest.MapFS {
	t.Helper()
	m := fstest.MapFS{}
	names, err := fs.Glob(embeddedMigrations(), "*.sql")
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range names {
		b, err := fs.ReadFile(embeddedMigrations(), n)
		if err != nil {
			t.Fatal(err)
		}
		m[n] = &fstest.MapFile{Data: b}
	}
	for n, q := range extra {
		m[n] = &fstest.MapFile{Data: []byte(q)}
	}
	return m
}

// A test-only raise after the embedded migrations: the next version
// raises min_reader, moving the Cubes' names into a table of their own
// and renaming the emptied column, and the one after it adds an index to
// that table, additively; both with their down parts.
func raiseMigrations(t *testing.T, withDowns bool) (fstest.MapFS, int) {
	t.Helper()
	next := SchemaVersion() + 1
	raise := padVersion(next) + "_names"
	index := padVersion(next+1) + "_names_index"
	extra := map[string]string{
		raise + ".sql": "-- min_reader: " + strconv.Itoa(next) + "\n" +
			"CREATE TABLE cube_names (cube_id INTEGER PRIMARY KEY REFERENCES cubes (id), name TEXT NOT NULL);\n" +
			"INSERT INTO cube_names (cube_id, name) SELECT id, name FROM cubes;\n" +
			"UPDATE cubes SET name = '';\n" +
			"ALTER TABLE cubes RENAME COLUMN name TO old_name;\n",
		index + ".sql": "-- min_reader: " + strconv.Itoa(next) + "\n" +
			"CREATE INDEX cube_names_name ON cube_names (name);\n",
	}
	if withDowns {
		extra[raise+".down.sql"] = "UPDATE cubes SET old_name = (SELECT name FROM cube_names WHERE cube_id = cubes.id) WHERE id IN (SELECT cube_id FROM cube_names);\n" +
			"ALTER TABLE cubes RENAME COLUMN old_name TO name;\n" +
			"DROP TABLE cube_names;\n"
		extra[index+".down.sql"] = "DROP INDEX cube_names_name;\n"
	}
	return laterMigrations(t, extra), next
}

// padVersion is a version as a migration's file name begins with it.
func padVersion(v int) string { return fmt.Sprintf("%04d", v) }

// schemaOfFresh is the schema an empty file gets from this binary.
func schemaOfFresh(t *testing.T) string {
	t.Helper()
	s := open(t, t.TempDir())
	q, err := schemaSQL(context.Background(), s)
	if err != nil {
		t.Fatal(err)
	}
	return q
}

// A later binary's raise and the additive migration after it are undone
// by their down parts, newest first: the schema is this binary's again,
// the rows are kept (the Cube's name too, which the raise had moved), a
// copy of the file as it was is left, and the later binary migrates the
// file again.
func TestDowngrade(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	later, raise := raiseMigrations(t, true)
	s := open(t, dir)
	if _, err := s.AddCube(ctx, Cube{Serial: "KEQ0000001", Name: "Hall", Configured: true}); err != nil {
		t.Fatal(err)
	}
	s.Close()
	s, err := Open(dir, Options{Version: "later", Logger: quiet(), migrations: later})
	if err != nil {
		t.Fatal(err)
	}
	if v, mr := s.SchemaVersion(); v != raise+1 || mr != raise {
		t.Fatalf("later binary: version %d min_reader %d", v, mr)
	}
	s.Close()
	if _, err := Open(dir, Options{Logger: quiet()}); !errors.Is(err, ErrRefused) {
		t.Fatalf("this binary opened the raised file: %v", err)
	}
	if v, mr, err := FileVersion(dir); err != nil || v != raise+1 || mr != raise {
		t.Fatalf("FileVersion: %d %d %v", v, mr, err)
	}

	d, err := Downgrade(dir, SchemaVersion(), Options{Logger: quiet(), migrations: later})
	if err != nil {
		t.Fatal(err)
	}
	if d.From != raise+1 || d.To != raise-1 || d.MinReader != binaryMinReader(t) || !slices.Equal(d.Undone, []int{raise + 1, raise}) {
		t.Fatalf("downgraded %+v", d)
	}
	if d.Copy != Path(dir)+".before-downgrade-"+strconv.Itoa(raise+1) {
		t.Fatalf("copy %q", d.Copy)
	}
	if fi, err := os.Stat(d.Copy); err != nil || (runtime.GOOS != "windows" && fi.Mode().Perm() != 0o600) {
		t.Fatalf("copy: %v %v", fi, err)
	}

	s = open(t, dir)
	if v, _ := s.SchemaVersion(); v != SchemaVersion() {
		t.Fatalf("version %d after the downgrade", v)
	}
	c, ok, err := s.Cube(ctx, "KEQ0000001")
	if err != nil || !ok || c.Name != "Hall" {
		t.Fatalf("the Cube after the downgrade: %+v %v %v", c, ok, err)
	}
	got, err := schemaSQL(ctx, s)
	if err != nil {
		t.Fatal(err)
	}
	if got != schemaOfFresh(t) {
		t.Fatalf("schema after the downgrade differs from a fresh file's:\n%s", got)
	}
	s.Close()

	// The later binary migrates it again, and a second downgrade from the
	// same version leaves a copy of its own.
	s, err = Open(dir, Options{Logger: quiet(), migrations: later})
	if err != nil {
		t.Fatalf("migrating again: %v", err)
	}
	s.Close()
	d, err = Downgrade(dir, SchemaVersion(), Options{Logger: quiet(), migrations: later})
	if err != nil || d.Copy != Path(dir)+".before-downgrade-"+strconv.Itoa(raise+1)+"-2" {
		t.Fatalf("second downgrade: %+v %v", d, err)
	}
}

// A binary that knows neither version undoes them by the down parts the
// later binary wrote into the file; a row without one gets it at the
// later binary's next start.
func TestDowngradeFromTheStoresDownParts(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	later, raise := raiseMigrations(t, true)
	s, err := Open(dir, Options{Logger: quiet(), migrations: later})
	if err != nil {
		t.Fatal(err)
	}
	var down string
	if err := s.db.QueryRowContext(ctx, `SELECT down_sql FROM schema_version WHERE version = ?`, raise).Scan(&down); err != nil || !strings.Contains(down, "DROP TABLE cube_names") {
		t.Fatalf("down part in the store: %q %v", down, err)
	}
	if _, err := s.db.ExecContext(ctx, `UPDATE schema_version SET down_sql = NULL WHERE version = ?`, raise+1); err != nil {
		t.Fatal(err)
	}
	s.Close()
	s, err = Open(dir, Options{Logger: quiet(), migrations: later})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.db.QueryRowContext(ctx, `SELECT coalesce(down_sql, '') FROM schema_version WHERE version = ?`, raise+1).Scan(&down); err != nil || !strings.Contains(down, "DROP INDEX") {
		t.Fatalf("down part not written again: %q %v", down, err)
	}
	s.Close()

	d, err := Downgrade(dir, SchemaVersion(), Options{Logger: quiet()})
	if err != nil {
		t.Fatal(err)
	}
	if d.To != raise-1 {
		t.Fatalf("downgraded %+v", d)
	}
	s = open(t, dir)
	got, err := schemaSQL(ctx, s)
	if err != nil {
		t.Fatal(err)
	}
	if got != schemaOfFresh(t) {
		t.Fatal("schema after the downgrade by the store's down parts differs from a fresh file's")
	}
}

// A file that a binary for the target version reads already is left
// alone: no copy, no change.
func TestDowngradeNothingToDo(t *testing.T) {
	dir := t.TempDir()
	later, raise := raiseMigrations(t, true)
	s, err := Open(dir, Options{Logger: quiet(), migrations: later})
	if err != nil {
		t.Fatal(err)
	}
	s.Close()
	for _, to := range []int{raise, raise + 1, raise + 5} {
		d, err := Downgrade(dir, to, Options{Logger: quiet(), migrations: later})
		if err != nil || d.From != raise+1 || d.To != raise+1 || d.Copy != "" || d.Undone != nil {
			t.Fatalf("to %d: %+v %v", to, d, err)
		}
	}
	if _, err := Downgrade(dir, 0, Options{Logger: quiet()}); err == nil {
		t.Fatal("downgrade to version 0")
	}
	if _, err := Downgrade(t.TempDir(), 1, Options{Logger: quiet()}); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("no file: %v", err)
	}
}

// A version without a down part, in the binary or in the file, stops
// the downgrade before anything changes, naming the copy made before the
// raise.
func TestDowngradeWithoutDownPart(t *testing.T) {
	dir := t.TempDir()
	later, raise := raiseMigrations(t, false)
	s, err := Open(dir, Options{Logger: quiet(), migrations: later})
	if err != nil {
		t.Fatal(err)
	}
	s.Close()
	_, err = Downgrade(dir, SchemaVersion(), Options{Logger: quiet(), migrations: later})
	if err == nil || !strings.Contains(err.Error(), ".before-"+strconv.Itoa(raise)) {
		t.Fatalf("downgrade without down parts: %v", err)
	}
	if v, _, _ := FileVersion(dir); v != raise+1 {
		t.Fatalf("file changed to version %d", v)
	}
	if m, _ := fs.Glob(os.DirFS(dir), "*.before-downgrade-*"); len(m) != 0 {
		t.Fatalf("copies left: %v", m)
	}
}

// A down part that fails leaves the file as it was and no copy.
func TestDowngradeFailingDownPart(t *testing.T) {
	dir := t.TempDir()
	later, raise := raiseMigrations(t, true)
	s, err := Open(dir, Options{Logger: quiet(), migrations: later})
	if err != nil {
		t.Fatal(err)
	}
	s.Close()
	later[padVersion(raise)+"_names.down.sql"] = &fstest.MapFile{Data: []byte("DROP TABLE no_such_table;\n")}
	if _, err := Downgrade(dir, SchemaVersion(), Options{Logger: quiet(), migrations: later}); err == nil {
		t.Fatal("a failing down part passed")
	}
	if v, mr, _ := FileVersion(dir); v != raise+1 || mr != raise {
		t.Fatalf("file at %d %d", v, mr)
	}
	if m, _ := fs.Glob(os.DirFS(dir), "*.before-downgrade-*"); len(m) != 0 {
		t.Fatalf("copies left: %v", m)
	}
}

// A store that a serve has open is refused with ErrInUse, and is not
// changed.
func TestDowngradeRefusesAnOpenStore(t *testing.T) {
	dir := t.TempDir()
	later, raise := raiseMigrations(t, true)
	s, err := Open(dir, Options{Logger: quiet(), migrations: later})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if _, err := Downgrade(dir, SchemaVersion(), Options{Logger: quiet(), migrations: later}); !errors.Is(err, ErrInUse) {
		t.Fatalf("downgrade beside an open store: %v", err)
	}
	if v, _ := s.SchemaVersion(); v != raise+1 {
		t.Fatalf("version %d", v)
	}
}

// A down part without its migration is refused with the set.
func TestDownPartWithoutMigration(t *testing.T) {
	if _, err := loadMigrations(testMigrations(t, map[string]string{"0002_x.down.sql": "SELECT 1;"})); err == nil {
		t.Fatal("a down part without its migration was accepted")
	}
	ms, err := loadMigrations(testMigrations(t, map[string]string{"0002_x.sql": "-- min_reader: 2\nSELECT 1;", "0002_x.down.sql": "SELECT 2;"}))
	if err != nil || ms[1].down != "SELECT 2;" {
		t.Fatalf("down part not read: %v", err)
	}
}

// Every migration from the first that raises min_reader on has a down
// part: undoing a raise undoes every migration after it as well, the
// additive ones included, since they were made on the raised schema
// (docs/store.md section 5, "Migrations").
func TestRaisingMigrationsHaveDownParts(t *testing.T) {
	ms, err := loadMigrations(embeddedMigrations())
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range ms {
		if m.minReader > ms[0].minReader && m.down == "" {
			t.Errorf("%s (min_reader %d) has no %s.down.sql", m.name, m.minReader, strings.TrimSuffix(m.name, ".sql"))
		}
	}
}
