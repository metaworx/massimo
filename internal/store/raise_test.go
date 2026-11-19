// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package store

// The store's first min_reader raise, migration 0024 and its down part
// (docs/store.md section 5, "Migrations"; AP StoreRaise v1.2, Block 3,
// D14-A): a store goes up and comes down again as it was, but for the
// losses 0024_raise.down.sql names (L1 to L8), which losses0024 holds
// for the comparisons.

import (
	"bytes"
	"compress/gzip"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"testing"
	"testing/fstest"
	"time"
)

// losses0024 are the columns a store taken up by 0024 and down again may
// hold other values in than before, by table, each with the loss of
// 0024_raise.down.sql that names it. A loss of rows (L2, L6) has no
// entry: TestRaiseDownLosses checks those rows itself.
var losses0024 = map[string]map[string]string{
	"cube_options": {"updated_at": "L7", "updated_by": "L7"},
	"integrations": {"key": "L4"},
	"users":        {"name": "L3, L5"},
	"tokens":       {"deleted_at": "L8"},
	"commands":     {"deleted_at": "L8"},
}

// binaryMinReader is the min_reader of the binary's last migration,
// which a file it made or migrated has.
func binaryMinReader(t *testing.T) int {
	t.Helper()
	ms, err := loadMigrations(embeddedMigrations())
	if err != nil {
		t.Fatal(err)
	}
	return ms[len(ms)-1].minReader
}

// migrationsUpTo is the embedded migrations up to version, their down
// parts with them: a binary built for version.
func migrationsUpTo(t *testing.T, version int) fstest.MapFS {
	t.Helper()
	m := fstest.MapFS{}
	names, err := fs.Glob(embeddedMigrations(), "[0-9]*.sql")
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range names {
		v, _ := strconv.Atoi(n[:4])
		if v > version {
			continue
		}
		b, err := fs.ReadFile(embeddedMigrations(), n)
		if err != nil {
			t.Fatal(err)
		}
		m[n] = &fstest.MapFile{Data: b}
	}
	return m
}

// migrateOnly applies the migrations of fsys (nil: the embedded ones) to
// the store in dir and closes it, with nothing else of Open: no node
// identity, no OPAQUE keys, no sealing of secrets, which a binary for
// another version would do its own way.
func migrateOnly(t *testing.T, dir string, fsys fs.FS, now func() time.Time) {
	t.Helper()
	s := newStore(dir, Options{Version: "test", Logger: quiet(), Now: now})
	if err := s.open(false); err != nil {
		t.Fatal(err)
	}
	if err := s.migrate(context.Background(), fsys, 0); err != nil {
		s.db.Close()
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
}

// fixedClock is a clock that stands still, so that two runs of the same
// migrations write the same times.
func fixedClock() func() time.Time {
	at := time.Date(2026, 11, 16, 9, 0, 0, 0, time.UTC)
	return func() time.Time { return at }
}

// copyStore copies the closed store of dir to a new directory.
func copyStore(t *testing.T, dir string) string {
	t.Helper()
	b, err := os.ReadFile(Path(dir))
	if err != nil {
		t.Fatal(err)
	}
	to := t.TempDir()
	if err := os.WriteFile(Path(to), b, 0o600); err != nil {
		t.Fatal(err)
	}
	return to
}

// rawDB opens the file of dir without the store, for reading it whole.
func rawDB(t *testing.T, dir string) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", "file:"+filepath.ToSlash(Path(dir))+"?mode=ro")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

// fileSchema is sqlite_master of db, each object as type, table and its
// SQL text, by name.
func fileSchema(t *testing.T, db *sql.DB) map[string]string {
	t.Helper()
	rows, err := db.Query(`SELECT type, name, tbl_name, coalesce(sql, '') FROM sqlite_master`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	out := map[string]string{}
	for rows.Next() {
		var typ, name, table, text string
		if err := rows.Scan(&typ, &name, &table, &text); err != nil {
			t.Fatal(err)
		}
		out[name] = typ + " " + table + "\n" + text
	}
	return out
}

// fileRows is every row of every table of db, sqlite_sequence included,
// each as the text of its values, sorted; the columns that ignore names
// for a table read "~".
func fileRows(t *testing.T, db *sql.DB, ignore map[string]map[string]string) map[string][]string {
	t.Helper()
	tables, err := db.Query(`SELECT name FROM sqlite_master WHERE type = 'table'`)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for tables.Next() {
		var n string
		tables.Scan(&n)
		names = append(names, n)
	}
	tables.Close()
	out := map[string][]string{}
	for _, table := range names {
		rows, err := db.Query(`SELECT * FROM "` + table + `"`)
		if err != nil {
			t.Fatal(err)
		}
		cols, _ := rows.Columns()
		var list []string
		for rows.Next() {
			vals := make([]any, len(cols))
			ptrs := make([]any, len(cols))
			for i := range vals {
				ptrs[i] = &vals[i]
			}
			if err := rows.Scan(ptrs...); err != nil {
				t.Fatal(err)
			}
			var b strings.Builder
			for i, v := range vals {
				if _, skip := ignore[table][cols[i]]; skip {
					v = "~"
				}
				if bs, ok := v.([]byte); ok {
					v = fmt.Sprintf("x'%x'", bs)
				}
				fmt.Fprintf(&b, "%s=%#v ", cols[i], v)
			}
			list = append(list, b.String())
		}
		rows.Close()
		sort.Strings(list)
		out[table] = list
	}
	return out
}

// sameFile fails where the files of two directories differ in
// sqlite_master or in their rows, but for the columns of ignore.
func sameFile(t *testing.T, want, got string, ignore map[string]map[string]string) {
	t.Helper()
	ws, gs := fileSchema(t, rawDB(t, want)), fileSchema(t, rawDB(t, got))
	for name, w := range ws {
		if g, ok := gs[name]; !ok {
			t.Errorf("sqlite_master: %s missing", name)
		} else if g != w {
			t.Errorf("sqlite_master: %s is\n%s\nwant\n%s", name, g, w)
		}
	}
	for name := range gs {
		if _, ok := ws[name]; !ok {
			t.Errorf("sqlite_master: %s is new", name)
		}
	}
	wr, gr := fileRows(t, rawDB(t, want), ignore), fileRows(t, rawDB(t, got), ignore)
	// The AUTOINCREMENT counters are rows too (D17-A): a file of backups
	// or commands has them, and they are compared as every table is.
	if len(wr["backups"])+len(wr["commands"]) > 0 && len(wr["sqlite_sequence"]) == 0 {
		t.Errorf("sqlite_sequence: no counters, though backups or commands have rows")
	}
	for table, w := range wr {
		if g := gr[table]; !slices.Equal(g, w) {
			t.Errorf("rows of %s:\n%s\nwant\n%s", table, strings.Join(g, "\n"), strings.Join(w, "\n"))
		}
	}
}

// oldCube adds a configured Cube with SQL that every schema version
// takes, for a store of an earlier version, whose columns this binary's
// AddCube does not write; it returns the row's id.
func oldCube(t *testing.T, s *Store, serial string) int64 {
	t.Helper()
	now := s.nowText()
	res, err := s.db.Exec(`INSERT INTO cubes (uuid, serial, configured, created_at, updated_at) VALUES (?, ?, 1, ?, ?)`, newUUID(), serial, now, now)
	if err != nil {
		t.Fatal(err)
	}
	id, _ := res.LastInsertId()
	return id
}

// oldUser adds a user without a password with SQL that every schema
// version takes, and returns its id.
func oldUser(t *testing.T, s *Store, name, role string) int64 {
	t.Helper()
	now := s.nowText()
	res, err := s.db.Exec(`INSERT INTO users (uuid, name, role_id, created_at, updated_at) SELECT ?, ?, id, ?, ? FROM roles WHERE key = ?`, newUUID(), name, now, now, role)
	if err != nil {
		t.Fatal(err)
	}
	id, _ := res.LastInsertId()
	return id
}

// oldBackup adds a backup with its file in the row, as a store before
// 0024 kept it, and returns its id.
func oldBackup(t *testing.T, s *Store, cubeID int64, reason, format string, at time.Time, body []byte) int64 {
	t.Helper()
	var z bytes.Buffer
	zw := gzip.NewWriter(&z)
	zw.Write(body)
	zw.Close()
	res, err := s.db.Exec(`INSERT INTO backups (uuid, cube_id, taken_at, reason, format, size, sha256, body) VALUES (?, ?, ?, ?, ?, ?, 'sum', ?)`,
		newUUID(), cubeID, formatTime(at), reason, format, len(body), z.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	id, _ := res.LastInsertId()
	return id
}

// raiseVersion is 0024's number.
const raiseVersion = 24

// undoneFromNewest is what a downgrade to 23 undoes: every version from
// the newest down to the raise, newest first, the additive ones after it
// included (0025).
func undoneFromNewest() []int {
	var out []int
	for v := SchemaVersion(); v >= raiseVersion; v-- {
		out = append(out, v)
	}
	return out
}

// D14-A: each release's store, migrated to the newest version and
// downgraded for its own release, equals the same store migrated only as
// far as that downgrade stops, in sqlite_master and in every row, but
// for losses0024. The migrations up to the stop are the same in both;
// they run once and the file is copied, so that the times the SQL of
// earlier migrations writes (strftime('now'), 0021) are the same too.
func TestReleasedStoresDownAgain(t *testing.T) {
	t.Parallel()
	dumps, err := filepath.Glob(filepath.Join("testdata", "releases", "v*.sql"))
	if err != nil || len(dumps) < 6 {
		t.Fatalf("the releases' dumps: %v, %v", dumps, err)
	}
	ms, err := loadMigrations(embeddedMigrations())
	if err != nil {
		t.Fatal(err)
	}
	for _, dump := range dumps {
		t.Run(strings.TrimSuffix(filepath.Base(dump), ".sql"), func(t *testing.T) {
			below := t.TempDir()
			_, release := loadDump(t, dump, Path(below))
			// Where Downgrade stops for that release: before the first
			// migration whose min_reader is above it.
			stop := ms[len(ms)-1].version
			for _, m := range ms {
				if m.minReader > release {
					stop = m.version - 1
					break
				}
			}
			if stop == ms[len(ms)-1].version {
				t.Skipf("schema %d reads the newest file; nothing to take down", release)
			}
			clock := fixedClock()
			migrateOnly(t, below, migrationsUpTo(t, stop), clock)
			down := copyStore(t, below)
			migrateOnly(t, down, nil, clock)
			if v, _, _ := FileVersion(down); v != SchemaVersion() {
				t.Fatalf("migrated to %d, want %d", v, SchemaVersion())
			}
			d, err := Downgrade(down, release, Options{Logger: quiet(), Now: clock})
			if err != nil {
				t.Fatal(err)
			}
			if d.To != stop {
				t.Fatalf("downgraded to %d, want %d: %+v", d.To, stop, d)
			}
			sameFile(t, below, down, losses0024)
		})
	}
}

// v23Store makes a store of version 23 in dir, as a binary of that
// version leaves it, with a row of every kind 0024 changes: an
// integration with its users (one linked) and a request waiting, removed
// users (two in one minute), a removed role, an option of a Cube, a
// revoked token, a backup and a tombstone, the audit of a call through
// the integration, a row of the log without a source, and a provider
// with domains. Its ids: the integration's uuid and the users'.
func v23Store(t *testing.T, dir string, clock func() time.Time) (integ string, users map[string]int64) {
	t.Helper()
	migrateOnly(t, dir, migrationsUpTo(t, 23), clock)
	s := newStore(dir, Options{Logger: quiet(), Now: clock})
	if err := s.open(false); err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	integ = "01a1e000-0000-7000-8000-000000000001"
	req := "01a1e000-0000-7000-8000-000000000002"
	cube := oldCube(t, s, "KEQ0000001")
	const at = "2026-11-10T09:00:12.000Z"
	q := []string{
		`INSERT INTO users (uuid, name, role_id, created_at, updated_at) SELECT 'u-anna', 'anna', id, '` + at + `', '` + at + `' FROM roles WHERE key = 'operator'`,
		`INSERT INTO users (uuid, name, role_id, created_at, updated_at, deleted_at) SELECT 'u-bob', 'bob-del-20261110-0900', id, '` + at + `', '` + at + `', '` + at + `' FROM roles WHERE key = 'viewer'`,
		`INSERT INTO users (uuid, name, role_id, created_at, updated_at, deleted_at) SELECT 'u-carl1', 'carl-del-20261110-0900', id, '` + at + `', '` + at + `', '2026-11-10T09:00:20.000Z' FROM roles WHERE key = 'viewer'`,
		`INSERT INTO users (uuid, name, role_id, created_at, updated_at, deleted_at) SELECT 'u-carl2', 'carl-del-20261110-0900-' || (SELECT max(id) + 1 FROM users), id, '` + at + `', '` + at + `', '2026-11-10T09:00:40.000Z' FROM roles WHERE key = 'viewer'`,
		`INSERT INTO users (uuid, name, display_name, role_id, created_at, updated_at) SELECT 'u-own', 'casa-leone', 'Casa Leone', id, '` + at + `', '` + at + `' FROM roles WHERE key = 'integration'`,
		`INSERT INTO users (uuid, name, display_name, role_id, created_at, updated_at) SELECT 'u-sys', 'casa-leone.system', 'system', id, '` + at + `', '` + at + `' FROM roles WHERE key = 'integration'`,
		`INSERT INTO users (uuid, name, display_name, role_id, created_at, updated_at) SELECT 'u-ha', 'casa-leone.0123abcd', 'Anna', id, '` + at + `', '` + at + `' FROM roles WHERE key = 'integration'`,
		`INSERT INTO integrations (uuid, kind, key, name, instance_id, max_caps, user_id, system_user_id, state, request_expires, created_at, updated_at)
			SELECT '` + integ + `', 'homeassistant', 'casa-leone', 'Casa Leone', '0123456789abcdef0123456789abcdef', '["read"]',
			(SELECT id FROM users WHERE uuid = 'u-own'), (SELECT id FROM users WHERE uuid = 'u-sys'), 'active', '` + at + `', '` + at + `', '` + at + `'`,
		`INSERT INTO integrations (uuid, kind, key, name, state, request_code, request_expires, created_at, updated_at)
			VALUES ('` + req + `', 'homeassistant', 'request-` + req + `', 'Other', 'pending', 'K7Q2PX', '2026-11-16T09:10:00.000Z', '` + at + `', '` + at + `')`,
		`INSERT INTO user_identities (uuid, user_id, integration_id, subject, name, own_user_id, created_at, bound_at)
			SELECT 'i-sys', u.id, g.id, 'system', 'system', u.id, '` + at + `', '` + at + `' FROM users u, integrations g WHERE u.uuid = 'u-sys' AND g.uuid = '` + integ + `'`,
		`INSERT INTO user_identities (uuid, user_id, integration_id, subject, name, own_user_id, last_level, created_at, bound_at)
			SELECT 'i-ha', a.id, g.id, '0123abcd0123abcd0123abcd0123abcd', 'Anna', o.id, 'control', '` + at + `', '` + at + `'
			FROM users a, users o, integrations g WHERE a.uuid = 'u-anna' AND o.uuid = 'u-ha' AND g.uuid = '` + integ + `'`,
		`INSERT INTO auth_providers (uuid, key, issuer, client_id, auto_domains, created_at, updated_at) VALUES ('p-cloud', 'cloud', 'https://id.example.com', 'massimo', 'a.example,b.example', '` + at + `', '` + at + `')`,
		`INSERT INTO user_identities (uuid, user_id, provider_id, subject, email, created_at, bound_at)
			SELECT 'i-oidc', u.id, p.id, 'sub-1', 'anna@example.com', '` + at + `', '` + at + `' FROM users u, auth_providers p WHERE u.uuid = 'u-anna' AND p.uuid = 'p-cloud'`,
		`INSERT INTO roles (key, name, created_at, updated_at, deleted_at) VALUES ('night', 'Night', '` + at + `', '` + at + `', '` + at + `')`,
		`INSERT INTO cube_options (cube_id, proxy_virtual, updated_at) VALUES (` + strconv.FormatInt(cube, 10) + `, 1, '` + at + `')`,
		`INSERT INTO tokens (uuid, user_id, kind, hash, created_at, revoked_at) SELECT 't-old', id, 'session', 'h1', '` + at + `', '` + at + `' FROM users WHERE uuid = 'u-anna'`,
		`INSERT INTO audit (uuid, time, user_id, action, detail) SELECT 'a-plain', '` + at + `', id, 'setpoint', '{}' FROM users WHERE uuid = 'u-anna'`,
		`INSERT INTO audit (uuid, time, user_id, action, detail) SELECT 'a-via', '` + at + `', id, 'setpoint',
			'{"via":"casa-leone","for":{"id":"0123abcd0123abcd0123abcd0123abcd","name":"Anna","level":"control"}}' FROM users WHERE uuid = 'u-anna'`,
		`INSERT INTO log (uuid, cube_id, time, kind, source, user_id, data) SELECT 'l-user', ` + strconv.FormatInt(cube, 10) + `, '` + at + `', 'change', 'config', id, '{}' FROM users WHERE uuid = 'u-anna'`,
		`INSERT INTO log (uuid, cube_id, time, kind, device, data) VALUES ('l-old', ` + strconv.FormatInt(cube, 10) + `, '` + at + `', 'state', '0a0101', '{"kind":"state","state":{"serial":"MEQ0000101"}}')`,
		`INSERT INTO commands (uuid, cube_id, action, state, created_at, updated_at) VALUES ('c-1', ` + strconv.FormatInt(cube, 10) + `, 'setpoint', 'acknowledged', '` + at + `', '` + at + `')`,
	}
	for _, stmt := range q {
		if _, err := s.db.Exec(stmt); err != nil {
			t.Fatalf("%v: %s", err, stmt)
		}
	}
	oldBackup(t, s, cube, ReasonManual, "json", clock().Add(-time.Hour), []byte(`{"version":1}`))
	gone := oldBackup(t, s, cube, ReasonAuto, "json", clock(), []byte(`{"version":2}`))
	if _, err := s.db.Exec(`UPDATE backups SET deleted_at = ?, body = x'' WHERE id = ?`, at, gone); err != nil {
		t.Fatal(err)
	}
	// The newest backup purged, so that the sequence is above the ids.
	oldBackup(t, s, cube, ReasonAuto, "json", clock(), []byte(`{"version":3}`))
	if _, err := s.db.Exec(`DELETE FROM backups WHERE id = (SELECT max(id) FROM backups)`); err != nil {
		t.Fatal(err)
	}
	var top, seq int64
	if err := s.db.QueryRow(`SELECT (SELECT max(id) FROM backups), (SELECT seq FROM sqlite_sequence WHERE name = 'backups')`).Scan(&top, &seq); err != nil || seq <= top {
		t.Fatalf("the backups' counter %d, the highest id %d, %v: the purge should leave it above", seq, top, err)
	}
	users = map[string]int64{}
	rows, err := s.db.Query(`SELECT uuid, id FROM users`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var u string
		var id int64
		rows.Scan(&u, &id)
		users[u] = id
	}
	return integ, users
}

// 0024 on a store of version 23: no table loses a row, an integration's
// users are named by its uuid and name it, the log and the audit have
// who made each row (the audit row of the call through the integration
// its user's own), the suffixes of the removed users are gone, the
// files are in backup_data, the option in the Cube's row, the domains a
// JSON list, every role has a uuid, and every foreign key holds. Its
// down part makes the file of version 23 again, but for losses0024; of
// those, the time and the user of the Cube's option (L7) differ here.
func TestRaiseOfAFilledStore(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	clock := fixedClock()
	before := t.TempDir()
	integ, ids := v23Store(t, before, clock)
	beforeRows := tableRows(t, rawDB(t, before))
	dir := copyStore(t, before)
	s, err := Open(dir, Options{Logger: quiet(), Now: clock})
	if err != nil {
		t.Fatal(err)
	}
	if v, mr := s.SchemaVersion(); v != SchemaVersion() || mr != raiseVersion {
		t.Fatalf("schema %d min_reader %d", v, mr)
	}
	if _, err := os.Stat(Path(dir) + ".before-24"); err != nil {
		t.Fatalf("no copy before the raise: %v", err)
	}
	after := tableRows(t, s.db)
	for table, n := range beforeRows {
		switch {
		case table == "cube_options":
			if _, ok := after[table]; ok {
				t.Error("cube_options is still there")
			}
		case after[table] < n:
			t.Errorf("%s: %d rows after the raise, %d before", table, after[table], n)
		}
	}
	if after["backup_data"] != 1 {
		t.Errorf("backup_data: %d rows, want the live backup's", after["backup_data"])
	}
	var fk int
	if err := s.db.QueryRow(`SELECT count(*) FROM pragma_foreign_key_check`).Scan(&fk); err != nil || fk != 0 {
		t.Fatalf("foreign_key_check: %d, %v", fk, err)
	}

	names := map[string]string{}
	for u, id := range ids {
		var name string
		var integID sql.NullInt64
		if err := s.db.QueryRow(`SELECT name, integration_id FROM users WHERE id = ?`, id).Scan(&name, &integID); err != nil {
			t.Fatal(err)
		}
		names[u] = name
		if strings.HasPrefix(u, "u-own") || u == "u-sys" || u == "u-ha" {
			if !integID.Valid {
				t.Errorf("%s: no integration_id", u)
			}
		} else if integID.Valid {
			t.Errorf("%s: integration_id %d", u, integID.Int64)
		}
	}
	want := map[string]string{"u-anna": "anna", "u-bob": "bob", "u-carl1": "carl", "u-carl2": "carl",
		"u-own": integ + ":", "u-sys": integ + ":system", "u-ha": integ + ":0123abcd0123abcd0123abcd0123abcd"}
	for u, n := range want {
		if names[u] != n {
			t.Errorf("%s: name %q, want %q", u, names[u], n)
		}
	}
	i, ok, err := s.IntegrationByUUID(ctx, integ)
	if err != nil || !ok || i.UserID != ids["u-own"] || i.SystemUserID != ids["u-sys"] || i.RequestExpires.IsZero() {
		t.Fatalf("the integration: %+v %v %v", i, ok, err)
	}
	if own, ok, _ := s.IntegrationOfUser(ctx, ids["u-own"]); !ok || own.UUID != integ {
		t.Fatal("the integration of its own user")
	}
	if _, ok, _ := s.IntegrationOfUser(ctx, ids["u-ha"]); ok {
		t.Fatal("an integration's user taken for its own")
	}
	var real, realLog, unsourced int64
	if err := s.db.QueryRow(`SELECT real_user_id FROM audit WHERE uuid = 'a-via'`).Scan(&real); err != nil || real != ids["u-ha"] {
		t.Errorf("the audit row through the integration: real user %d, %v; want %d", real, err, ids["u-ha"])
	}
	if err := s.db.QueryRow(`SELECT real_user_id FROM audit WHERE uuid = 'a-plain'`).Scan(&real); err != nil || real != ids["u-anna"] {
		t.Errorf("the plain audit row: real user %d, %v", real, err)
	}
	if err := s.db.QueryRow(`SELECT count(*) FROM log WHERE real_user_id IS NOT effective_user_id`).Scan(&realLog); err != nil || realLog != 0 {
		t.Errorf("log rows whose real user is not the effective one: %d, %v", realLog, err)
	}
	if err := s.db.QueryRow(`SELECT count(*) FROM log WHERE source = ''`).Scan(&unsourced); err != nil || unsourced != 0 {
		t.Errorf("rows without a source: %d, %v", unsourced, err)
	}
	if page, _ := s.Events(ctx, EventQuery{CubeID: 1, Kind: "state"}); len(page) != 1 || page[0].Source != "device" || page[0].Serial != "MEQ0000101" {
		t.Errorf("the row an older binary wrote: %+v", page)
	}
	ha, err := s.IntegrationIdentities(ctx, i.ID)
	if err != nil || len(ha) != 2 || ha[1].RealUserID != ids["u-ha"] || ha[1].EffectiveUserID != ids["u-anna"] || !ha[1].Linked() {
		t.Fatalf("the integration's users: %+v %v", ha, err)
	}
	c, _, _ := s.Cube(ctx, "KEQ0000001")
	if !c.ProxyVirtual {
		t.Error("proxy_virtual not carried from cube_options")
	}
	if p, _, _ := s.Provider(ctx, "cloud"); strings.Join(p.AutoDomains, ",") != "a.example,b.example" {
		t.Errorf("auto_domains %v", p.AutoDomains)
	}
	var noUUID int
	if err := s.db.QueryRow(`SELECT count(*) FROM roles WHERE length(uuid) <> 36`).Scan(&noUUID); err != nil || noUUID != 0 {
		t.Errorf("%d roles without a uuid, %v", noUUID, err)
	}
	if tok, _, _ := s.Token(ctx, 1); tok.DeletedAt.IsZero() {
		t.Error("the revoked token lost its revocation")
	}
	if list, _ := s.Backups(ctx, c.ID); len(list) != 1 {
		t.Errorf("backups %+v", list)
	} else if body, ok, err := s.BackupBody(ctx, c.ID, list[0].ID); err != nil || !ok || string(body) != `{"version":1}` {
		t.Errorf("the backup's file: %q %v %v", body, ok, err)
	}
	s.Close()

	if _, err := Downgrade(dir, 23, Options{Logger: quiet(), Now: clock}); err != nil {
		t.Fatal(err)
	}
	// What the up part's fill gave the row of an older binary stays, as
	// the down part's header says.
	filled := copyStore(t, before)
	ws := newStore(filled, Options{Logger: quiet(), Now: clock})
	if err := ws.open(false); err != nil {
		t.Fatal(err)
	}
	if _, err := ws.db.Exec(`UPDATE log SET source = 'device', serial = 'MEQ0000101' WHERE uuid = 'l-old'`); err != nil {
		t.Fatal(err)
	}
	ws.Close()
	sameFile(t, filled, dir, losses0024)
	var updated string
	if err := rawDB(t, dir).QueryRow(`SELECT updated_at FROM cube_options`).Scan(&updated); err != nil || updated == "2026-11-10T09:00:12.000Z" {
		t.Errorf("cube_options.updated_at %q, %v: L7 says it is the Cube's", updated, err)
	}
}

// What the raised schema holds and schema 23 cannot is lost by the down
// part as 0024_raise.down.sql names it: a removed identity (L2), the
// second row of a role's key (L6), the name of a user removed after the
// raise suffixed again (L5), the key of an integration renamed made from
// its new name (L4); a second row of a Cube's serial is kept under
// <serial>~<id> with everything of it (D15-C); the file is that of schema
// 23 to the byte, and every foreign key holds.
func TestRaiseDownLosses(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	clock := fixedClock()
	dir := t.TempDir()
	integ, ids := v23Store(t, dir, clock)
	s, err := Open(dir, Options{Logger: quiet(), Now: clock})
	if err != nil {
		t.Fatal(err)
	}
	// L2
	ida, _ := s.Identities(ctx, ids["u-anna"])
	if len(ida) != 1 {
		t.Fatalf("anna's identities %+v", ida)
	}
	if err := s.DeleteIdentity(ctx, ids["u-anna"], ida[0].ID); err != nil {
		t.Fatal(err)
	}
	// L6
	night, err := s.AddRole(ctx, Role{Key: "night", Capabilities: []string{"read"}}, 0)
	if err != nil {
		t.Fatal(err)
	}
	// L7: the Cube removed, and added again with a log row and a backup.
	old, _, _ := s.Cube(ctx, "KEQ0000001")
	if _, err := s.db.Exec(`UPDATE cubes SET deleted_at = ? WHERE id = ?`, s.nowText(), old.ID); err != nil {
		t.Fatal(err)
	}
	fresh, err := s.AddCube(ctx, Cube{Serial: "KEQ0000001", Configured: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.InsertEvent(ctx, Event{CubeID: fresh.ID, Kind: "connected", Source: "cube", Data: []byte(`{}`)}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.AddBackup(ctx, Backup{CubeID: fresh.ID, Reason: ReasonManual, Format: "json"}, []byte(`{"version":4}`)); err != nil {
		t.Fatal(err)
	}
	// L5
	dora, err := s.AddUser(ctx, User{Name: "dora"}, "", 0)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteUser(ctx, "dora", 0); err != nil {
		t.Fatal(err)
	}
	if _, err := s.AddUser(ctx, User{Name: "dora"}, "", 0); err != nil {
		t.Fatal(err)
	}
	// L4: a name the integration reports anew, as an attached request's.
	if _, err := s.db.Exec(`UPDATE integrations SET name = 'Villa Rossa' WHERE uuid = ?`, integ); err != nil {
		t.Fatal(err)
	}
	// L1: a row through the integration, its real user its own.
	if _, err := s.Audit(ctx, AuditEntry{EffectiveUserID: ids["u-anna"], RealUserID: ids["u-ha"], Action: "setpoint"}); err != nil {
		t.Fatal(err)
	}
	s.Close()

	d, err := Downgrade(dir, 23, Options{Logger: quiet(), Now: clock})
	if err != nil || d.To != 23 || !slices.Equal(d.Undone, undoneFromNewest()) {
		t.Fatalf("downgrade %+v %v", d, err)
	}
	if got, want := fileSchema(t, rawDB(t, dir)), schemaAt(t, 23); fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatal("the schema after the downgrade is not that of version 23")
	}
	db := rawDB(t, dir)
	count := func(q string, args ...any) int {
		t.Helper()
		var n int
		if err := db.QueryRow(q, args...).Scan(&n); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
		return n
	}
	if n := count(`SELECT count(*) FROM pragma_foreign_key_check`); n != 0 {
		t.Fatalf("foreign_key_check: %d", n)
	}
	if n := count(`SELECT count(*) FROM user_identities WHERE provider_id IS NOT NULL`); n != 0 {
		t.Errorf("L2: %d removed identities left", n)
	}
	if n := count(`SELECT count(*) FROM roles WHERE key = 'night' AND id = ? AND deleted_at IS NULL`, night.ID); n != 1 {
		t.Errorf("L6: the live night not kept")
	}
	if n := count(`SELECT count(*) FROM roles WHERE key = 'night'`); n != 1 {
		t.Errorf("L6: %d rows of night", n)
	}
	if n := count(`SELECT count(*) FROM cubes WHERE serial = 'KEQ0000001' AND id = ?`, fresh.ID); n != 1 {
		t.Errorf("D15-C: the live Cube's row does not keep the serial")
	}
	if n := count(`SELECT count(*) FROM cubes WHERE serial = ? AND id = ?`, "KEQ0000001~"+strconv.FormatInt(old.ID, 10), old.ID); n != 1 {
		t.Errorf("D15-C: the removed row not named KEQ0000001~%d", old.ID)
	}
	if n := count(`SELECT count(*) FROM log WHERE cube_id = ?`, old.ID); n != 2 {
		t.Errorf("D15-C: %d log rows of the removed Cube's row, want its 2", n)
	}
	if n := count(`SELECT count(*) FROM commands WHERE cube_id = ? AND uuid = 'c-1'`, old.ID); n != 1 {
		t.Errorf("D15-C: the removed row's command not kept with it")
	}
	// L7: the option of the removed row, on, is a row again; the live
	// row's, off, is none.
	if n := count(`SELECT count(*) FROM cube_options WHERE cube_id = ?`, old.ID); n != 1 || count(`SELECT count(*) FROM cube_options`) != 1 {
		t.Errorf("L7: cube_options %d", count(`SELECT count(*) FROM cube_options`))
	}
	if n := count(`SELECT count(*) FROM users WHERE id = ? AND name = 'dora-del-20261116-0900'`, dora.ID); n != 1 {
		t.Errorf("L5: the removed dora not suffixed")
	}
	if n := count(`SELECT count(*) FROM integrations WHERE uuid = ? AND key = 'villa-rossa'`, integ); n != 1 {
		t.Errorf("L4: the key not made from the new name")
	}
	for name, want := range map[string]string{"u-own": "villa-rossa", "u-sys": "villa-rossa.system", "u-ha": "villa-rossa.0123abcd"} {
		if n := count(`SELECT count(*) FROM users WHERE id = ? AND name = ?`, ids[name], want); n != 1 {
			t.Errorf("L3: %s not named %s", name, want)
		}
	}
	if n := count(`SELECT count(*) FROM integrations WHERE key = 'request-01a1e000-0000-7000-8000-000000000002'`); n != 1 {
		t.Errorf("the waiting request's key")
	}
	s = open(t, dir)
	if v, _ := s.SchemaVersion(); v != SchemaVersion() {
		t.Fatalf("migrated again to %d", v)
	}
}

// Two rows of one serial (D15-C): the down part keeps both with
// everything that names them, the removed one under <serial>~<id> in
// cubes.serial alone, the log byte for byte; 0024 takes the suffix off
// again, and both rows have the plain serial and all their rows.
func TestRaiseDownKeepsBothRowsOfASerial(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	clock := fixedClock()
	dir := t.TempDir()
	s, err := Open(dir, Options{Logger: quiet(), Now: clock})
	if err != nil {
		t.Fatal(err)
	}
	fill := func(c Cube, n int) {
		t.Helper()
		if err := s.InsertEvent(ctx, Event{CubeID: c.ID, Kind: "state", Source: "device", Serial: "MEQ000010" + strconv.Itoa(n),
			Data: []byte(`{"kind":"state","cube_serial":"KEQ0000001","n":` + strconv.Itoa(n) + `}`)}); err != nil {
			t.Fatal(err)
		}
		if _, err := s.AddBackup(ctx, Backup{CubeID: c.ID, Reason: ReasonManual, Format: "json"}, []byte(`{"version":`+strconv.Itoa(n)+`}`)); err != nil {
			t.Fatal(err)
		}
		if _, err := s.AddCommand(ctx, Command{CubeID: c.ID, Action: "setpoint"}); err != nil {
			t.Fatal(err)
		}
		if _, err := s.Audit(ctx, AuditEntry{CubeID: c.ID, Action: "setpoint", Target: "cube:KEQ0000001"}); err != nil {
			t.Fatal(err)
		}
		if _, err := s.db.Exec(`INSERT INTO rooms (uuid, cube_id, room_id, created_at, updated_at) VALUES (?, ?, 1, ?, ?)`, newUUID(), c.ID, s.nowText(), s.nowText()); err != nil {
			t.Fatal(err)
		}
	}
	old, err := s.AddCube(ctx, Cube{Serial: "KEQ0000001", Name: "Old", Configured: true})
	if err != nil {
		t.Fatal(err)
	}
	fill(old, 1)
	if _, err := s.db.Exec(`UPDATE cubes SET deleted_at = ? WHERE id = ?`, s.nowText(), old.ID); err != nil {
		t.Fatal(err)
	}
	fresh, err := s.AddCube(ctx, Cube{Serial: "KEQ0000001", Name: "New", Configured: true})
	if err != nil {
		t.Fatal(err)
	}
	fill(fresh, 2)
	s.Close()
	logRows := func() []string {
		t.Helper()
		// Each read closes its connection, which Downgrade's lock needs.
		db := rawDB(t, dir)
		defer db.Close()
		// The columns both schemas have; the user's are none of these rows'.
		rows, err := db.Query(`SELECT quote(id) || ' ' || quote(uuid) || ' ' || quote(cube_id) || ' ' || quote(time) || ' ' || quote(kind) || ' ' || quote(source) || ' ' ||
			quote(level) || ' ' || quote(device) || ' ' || quote(serial) || ' ' || quote(data) || ' ' || quote(command_id) || ' ' || quote(visibility) FROM log ORDER BY id`)
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		var out []string
		for rows.Next() {
			var r string
			rows.Scan(&r)
			out = append(out, r)
		}
		return out
	}
	children := func() string {
		t.Helper()
		db := rawDB(t, dir)
		defer db.Close()
		var b strings.Builder
		for _, table := range []string{"log", "backups", "rooms", "commands", "audit"} {
			for _, id := range []int64{old.ID, fresh.ID} {
				var n int
				// The names are the test's own.
				if err := db.QueryRow(`SELECT count(*) FROM `+table+` WHERE cube_id = ?`, id).Scan(&n); err != nil {
					t.Fatal(err)
				}
				fmt.Fprintf(&b, "%s %d: %d; ", table, id, n)
			}
		}
		return b.String()
	}
	serials := func() map[int64]string {
		t.Helper()
		db := rawDB(t, dir)
		defer db.Close()
		rows, err := db.Query(`SELECT id, serial FROM cubes`)
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		out := map[int64]string{}
		for rows.Next() {
			var id int64
			var sn string
			rows.Scan(&id, &sn)
			out[id] = sn
		}
		return out
	}
	wantLog, wantChildren := logRows(), children()
	if len(wantLog) != 2 || strings.Contains(wantChildren, ": 0;") {
		t.Fatalf("the rows made: %v, %s", wantLog, wantChildren)
	}

	if _, err := Downgrade(dir, 23, Options{Logger: quiet(), Now: clock}); err != nil {
		t.Fatal(err)
	}
	if got := serials(); len(got) != 2 || got[fresh.ID] != "KEQ0000001" || got[old.ID] != "KEQ0000001~"+strconv.FormatInt(old.ID, 10) {
		t.Fatalf("serials down: %v", got)
	}
	if got := children(); got != wantChildren {
		t.Errorf("down: %s, want %s", got, wantChildren)
	}
	if got := logRows(); !slices.Equal(got, wantLog) {
		t.Errorf("the log down:\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(wantLog, "\n"))
	}

	migrateOnly(t, dir, nil, clock)
	if got := serials(); len(got) != 2 || got[fresh.ID] != "KEQ0000001" || got[old.ID] != "KEQ0000001" {
		t.Fatalf("serials up again: %v", got)
	}
	if got := children(); got != wantChildren {
		t.Errorf("up again: %s, want %s", got, wantChildren)
	}
	if got := logRows(); !slices.Equal(got, wantLog) {
		t.Errorf("the log up again:\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(wantLog, "\n"))
	}
	s = open(t, dir)
	if removed, err := s.RemovedCubes(ctx, "KEQ0000001"); err != nil || len(removed) != 1 || removed[0].ID != old.ID {
		t.Fatalf("the removed row up again: %+v %v", removed, err)
	}
	if c, ok, _ := s.Cube(ctx, "KEQ0000001"); !ok || c.ID != fresh.ID {
		t.Fatalf("the live row up again: %+v", c)
	}
}

// schemaAt is sqlite_master of a new file the migrations up to version
// make.
func schemaAt(t *testing.T, version int) map[string]string {
	t.Helper()
	dir := t.TempDir()
	migrateOnly(t, dir, migrationsUpTo(t, version), fixedClock())
	return fileSchema(t, rawDB(t, dir))
}

// A new file taken up and down again has the schema of version 23, and
// a binary for 23 refuses the raised file, naming the downgrade.
func TestRaiseRefusedAndDownAgain(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	s := open(t, dir)
	s.Close()
	_, err := Open(dir, Options{Logger: quiet(), migrations: migrationsUpTo(t, 23), readerVersion: 23})
	if !errors.Is(err, ErrRefused) || !strings.Contains(err.Error(), "massimo store downgrade -to 23") {
		t.Fatalf("a binary for 23: %v", err)
	}
	if _, err := Downgrade(dir, 23, Options{Logger: quiet()}); err != nil {
		t.Fatal(err)
	}
	if got, want := fileSchema(t, rawDB(t, dir)), schemaAt(t, 23); fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatal("the schema after the downgrade is not that of version 23")
	}
	s, err = Open(dir, Options{Logger: quiet(), migrations: migrationsUpTo(t, 23), readerVersion: 23})
	if err != nil {
		t.Fatalf("a binary for 23 after the downgrade: %v", err)
	}
	if v, mr := s.SchemaVersion(); v != 23 || mr != 1 {
		t.Fatalf("schema %d min_reader %d", v, mr)
	}
	s.Close()
}

// The times of the raise and of its down part on a log of
// MASSIMO_STORE_RAISE_ROWS rows (none: skipped), a tenth of them with a
// user, as the NOTE of Block 3 records them for 500 000.
func TestRaiseTimes(t *testing.T) {
	t.Parallel()
	n, _ := strconv.Atoi(os.Getenv("MASSIMO_STORE_RAISE_ROWS"))
	if n <= 0 {
		t.Skip("MASSIMO_STORE_RAISE_ROWS not set")
	}
	clock := fixedClock()
	dir := t.TempDir()
	_, ids := v23Store(t, dir, clock)
	s := newStore(dir, Options{Logger: quiet(), Now: clock})
	if err := s.open(false); err != nil {
		t.Fatal(err)
	}
	began := time.Now()
	if _, err := s.db.Exec(`WITH RECURSIVE n(i) AS (SELECT 1 UNION ALL SELECT i + 1 FROM n WHERE i < ?)
		INSERT INTO log (uuid, cube_id, time, kind, source, level, device, serial, user_id, data)
		SELECT 'r-' || i, 1, strftime('%Y-%m-%dT%H:%M:%fZ', '2026-01-01', '+' || (i / 10) || ' seconds'), 'state', 'device', 0, '0a0101', 'MEQ0000101',
		       CASE WHEN i % 10 = 0 THEN ? END, '{"kind":"state","device":"0a0101","state":{"setpoint":21.5,"actual":20.9,"mode":"auto","valve":40}}'
		FROM n`, n, ids["u-anna"]); err != nil {
		t.Fatal(err)
	}
	s.Close()
	fi, _ := os.Stat(Path(dir))
	t.Logf("filled %d rows in %v, %d MB", n, time.Since(began).Round(time.Millisecond), fi.Size()>>20)
	began = time.Now()
	migrateOnly(t, dir, nil, clock)
	t.Logf("0024 up: %v", time.Since(began).Round(time.Millisecond))
	began = time.Now()
	if _, err := Downgrade(dir, 23, Options{Logger: quiet(), Now: clock}); err != nil {
		t.Fatal(err)
	}
	t.Logf("0024 down: %v (the copy before it included)", time.Since(began).Round(time.Millisecond))
}
