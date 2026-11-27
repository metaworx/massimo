// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package store

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"
)

// restoreBox is a store with two Cubes, each with rooms, a Cube backup
// and a command; an owner who signs in by OPAQUE and an operator by
// password with a key; a role of its own; a provider and the push key
// sealed; a target with a secret; an integration with its user; and the
// audit and the log.
type restoreBox struct {
	s         *Store
	bobKey    string
	cubes     []Cube
	targetUID string
}

func newRestoreBox(t *testing.T) restoreBox {
	t.Helper()
	ctx := context.Background()
	s := open(t, t.TempDir())
	b := restoreBox{s: s}
	if _, err := s.AddUserOpaque(ctx, User{Name: "anna", Role: RoleOwner}, "anna's secret", 0); err != nil {
		t.Fatal(err)
	}
	if _, err := s.AddRole(ctx, Role{Key: "heater", Capabilities: []string{"read", "control"}}, 0); err != nil {
		t.Fatal(err)
	}
	bob, err := s.AddUser(ctx, User{Name: "bob", Role: "heater"}, "bob's secret", 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, b.bobKey, err = s.AddToken(ctx, bob.ID, "api", "bob's key", time.Time{}); err != nil {
		t.Fatal(err)
	}
	session, _, err := s.AddToken(ctx, bob.ID, "session", "bob's browser", time.Now().Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.AddProvider(ctx, AuthProvider{Key: "cloud", Issuer: "https://cloud.example.com", ClientID: "massimo", ClientSecret: "provider secret", Enabled: true}, 0); err != nil {
		t.Fatal(err)
	}
	if err := s.SetSecretSetting(ctx, VAPIDKey, "push key"); err != nil {
		t.Fatal(err)
	}
	if err := s.SetSetting(ctx, "poll_interval", "30s"); err != nil {
		t.Fatal(err)
	}
	tg, err := s.AddServerBackupTarget(ctx, ServerBackupTarget{Name: "nas", Kind: "local", Config: map[string]string{"path": "nas"}, Secret: "target secret",
		Takes: "every", Retention: []byte(`{}`), Enabled: true}, 0)
	if err != nil {
		t.Fatal(err)
	}
	b.targetUID = tg.UUID
	integrationOnly(t, s)
	for i, sn := range []string{"KEQ0000000", "KEQ0000001"} {
		c, err := s.AddCube(ctx, Cube{Serial: sn, Configured: true, Name: "cube " + sn})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := s.SyncRooms(ctx, c.ID, []uint8{1, uint8(2 + i)}); err != nil {
			t.Fatal(err)
		}
		if _, err := s.AddBackup(ctx, Backup{CubeID: c.ID, Reason: "manual", Format: "json", UserID: bob.ID}, []byte(`{"cube":"`+sn+`"}`)); err != nil {
			t.Fatal(err)
		}
		if _, err := s.AddCommand(ctx, Command{CubeID: c.ID, UserID: bob.ID, TokenID: session.ID, Action: "setpoint", State: StateAcknowledged}); err != nil {
			t.Fatal(err)
		}
		b.cubes = append(b.cubes, c)
	}
	if _, err := s.Audit(ctx, AuditEntry{EffectiveUserID: bob.ID, TokenID: session.ID, Action: "setpoint", CubeID: b.cubes[0].ID}); err != nil {
		t.Fatal(err)
	}
	return b
}

// backupDir copies a store and its key files into a new directory, as a
// backup's payload holds them once opened.
func backupDir(t *testing.T, s *Store) string {
	t.Helper()
	dir := t.TempDir()
	if err := s.Backup(context.Background(), Path(dir)); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{OpaqueKeyFileName, KeyFileName} {
		b, err := os.ReadFile(filepath.Join(s.Dir(), name))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), b, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// reopen closes s and opens its directory again, as the restart after a
// restore does, with the backup's opaque.key where from is given.
func reopen(t *testing.T, s *Store, from string) *Store {
	t.Helper()
	s.Close()
	if from != "" {
		b, err := os.ReadFile(OpaqueKeyPath(from))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(OpaqueKeyPath(s.Dir()), b, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return openFresh(t, s.Dir())
}

// rows counts a table's rows.
func rows(t *testing.T, s *Store, table string) int {
	t.Helper()
	var n int
	if err := s.db.QueryRow(`SELECT count(*) FROM ` + table).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

// Every table of the store is placed in a scope, or among the rows never
// restored, and the places of a table's rows do not overlap and leave
// none out: a table added later fails here until it is placed.
func TestEveryTableInAScope(t *testing.T) {
	t.Parallel()
	b := newRestoreBox(t)
	ctx := context.Background()
	for _, tb := range []string{"users", "tokens", "cubes", "rooms", "backups", "commands", "audit", "integrations", "auth_providers", "server_backup_targets"} {
		if rows(t, b.s, tb) == 0 {
			t.Errorf("the box has no %s, so their places are not checked", tb)
		}
	}
	// The store's one connection, held: nothing else may query meanwhile.
	conn, err := b.s.db.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	defer conn.ExecContext(context.Background(), `DROP TABLE IF EXISTS temp.restore_serials`)
	tables, err := tableNames(ctx, conn, "main")
	if err != nil {
		t.Fatal(err)
	}
	placed := map[string]bool{}
	for _, e := range scopeTables {
		if !slices.Contains(tables, e.table) {
			t.Errorf("scopeTables places %s, which the store does not have", e.table)
		}
		if e.scope != "" && !slices.Contains(RestoreScopes, e.scope) {
			t.Errorf("%s: no scope %q", e.table, e.scope)
		}
		placed[e.table] = true
	}
	for _, tb := range tables {
		if !placed[tb] {
			t.Errorf("the table %s is in no scope: place it in scopeTables (restore.go), or a restore leaves it out", tb)
		}
	}
	if _, err := conn.ExecContext(ctx, `CREATE TEMP TABLE restore_serials (serial TEXT PRIMARY KEY)`); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.ExecContext(ctx, `INSERT OR IGNORE INTO temp.restore_serials SELECT serial FROM cubes`); err != nil {
		t.Fatal(err)
	}
	for _, tb := range tables {
		var parts []string
		for _, e := range scopeTables {
			if e.table == tb {
				parts = append(parts, "("+where(e, "main")+")")
			}
		}
		if len(parts) == 0 {
			continue // said above
		}
		var total, sum, over int
		conn.QueryRowContext(ctx, `SELECT count(*) FROM `+q(tb)).Scan(&total)
		if err := conn.QueryRowContext(ctx, `SELECT coalesce(sum(`+strings.Join(parts, " + ")+`), 0), coalesce(sum((`+strings.Join(parts, " + ")+`) > 1), 0) FROM `+q(tb)).Scan(&sum, &over); err != nil {
			t.Fatalf("%s: %v", tb, err)
		}
		if sum != total || over != 0 {
			t.Errorf("%s: %d rows, %d placed, %d placed twice", tb, total, sum, over)
		}
	}
}

// Every scope from another box into a new one: the users sign in with
// their passwords once the store has the backup's opaque.key, the sealed
// secrets open, the Cubes come with their rooms, Cube backups and
// commands, the integration with its user, and a session never comes
// back, the references to it cleared.
func TestRestoreEveryScope(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	a := newRestoreBox(t)
	from := backupDir(t, a.s)
	s := open(t, t.TempDir())
	plan, err := s.Restore(ctx, RestoreSelection{Scopes: RestoreScopes, Source: from, OpaqueKey: true, DryRun: true})
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	if rows(t, s, "cubes") != 0 || rows(t, s, "users") != 0 {
		t.Fatal("the plan changed the store")
	}
	rep, err := s.Restore(ctx, RestoreSelection{Scopes: RestoreScopes, Source: from, OpaqueKey: true})
	if err != nil {
		t.Fatalf("restore: %v %+v", err, rep)
	}
	if len(rep.Tables) != len(plan.Tables) || len(rep.Missing) != 0 || rep.Lost != 0 {
		t.Fatalf("report %+v, plan %+v", rep, plan)
	}
	if len(rep.Cubes) != 2 || rep.Cubes[0].Serial != "KEQ0000000" || rep.Cubes[0].Live || rep.Cubes[0].Backup == "" {
		t.Fatalf("cubes %+v", rep.Cubes)
	}
	cleared := map[string]int{}
	for _, c := range rep.Cleared {
		cleared[c.Table+"."+c.Column] = c.Rows
	}
	if cleared["commands.token_id"] != 2 || cleared["audit.token_id"] != 1 {
		t.Errorf("the session's references: %+v", rep.Cleared)
	}
	s = reopen(t, s, from)
	if start, ke3 := newTestClient(t).login(s, "anna", "anna's secret"); ke3 == nil || s.FinishOpaque(start, ke3) != nil {
		t.Error("anna does not sign in with her password")
	}
	if err := s.CheckPassword(ctx, "bob", "bob's secret"); err != nil {
		t.Errorf("bob: %v", err)
	}
	if tok, ok, err := s.TokenByHash(ctx, HashToken(a.bobKey)); err != nil || !ok || tok.Kind != "api" {
		t.Errorf("bob's key: %+v %v %v", tok, ok, err)
	}
	if n := rows(t, s, "tokens WHERE kind = 'session'"); n != 0 {
		t.Errorf("%d sessions came back", n)
	}
	p, ok, err := s.Provider(ctx, "cloud")
	if err != nil || !ok || p.SecretLost || p.ClientSecret != "provider secret" {
		t.Errorf("provider %+v %v %v", p, ok, err)
	}
	if v, ok, err := s.SecretSetting(ctx, VAPIDKey); err != nil || !ok || string(v) != `"push key"` {
		t.Errorf("push key %s %v %v", v, ok, err)
	}
	tg, ok, err := s.ServerBackupTarget(ctx, a.targetUID)
	if err != nil || !ok || tg.Secret != "target secret" {
		t.Errorf("target %+v %v %v", tg, ok, err)
	}
	cubes, _ := s.Cubes(ctx)
	if len(cubes) != 2 {
		t.Fatalf("cubes %+v", cubes)
	}
	for _, c := range cubes {
		rs, _ := s.Rooms(ctx, c.ID)
		bs, _ := s.Backups(ctx, c.ID)
		if len(rs) != 2 || len(bs) != 1 || bs[0].UserName != "bob" {
			t.Errorf("%s: rooms %+v, backups %+v", c.Serial, rs, bs)
		}
	}
	for tb, want := range map[string]int{"integrations": 1, "users WHERE integration_id IS NOT NULL": rows(t, a.s, "users WHERE integration_id IS NOT NULL"), "commands": 2, "audit": 1, "backup_data": 2} {
		if got := rows(t, s, tb); got != want {
			t.Errorf("%s: %d, want %d", tb, got, want)
		}
	}
	var fk int
	if err := s.db.QueryRow(`SELECT count(*) FROM pragma_foreign_key_check`).Scan(&fk); err != nil || fk != 0 {
		t.Errorf("%d references to missing rows %v", fk, err)
	}
}

// A Cube alone into a box without its users is refused with the list of
// what is missing, and changes nothing; with the users it goes.
func TestRestoreMissingRefused(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	a := newRestoreBox(t)
	from := backupDir(t, a.s)
	s := open(t, t.TempDir())
	rep, err := s.Restore(ctx, RestoreSelection{Scopes: []string{ScopeCubes}, Serials: []string{"keq0000000"}, Source: from})
	if !errors.Is(err, ErrRestoreMissing) {
		t.Fatalf("a Cube without its users: %v", err)
	}
	var names []string
	for _, m := range rep.Missing {
		names = append(names, m.Table+":"+m.Name)
	}
	if !slices.Contains(names, "users:bob") || len(rep.Tables) != 5 {
		t.Fatalf("missing %+v, tables %+v", rep.Missing, rep.Tables)
	}
	if rows(t, s, "cubes") != 0 {
		t.Fatal("a refused restore changed the store")
	}
	if _, err := s.Restore(ctx, RestoreSelection{Scopes: []string{ScopeCubes}, Serials: []string{"KEQ0000009"}, Source: from}); err == nil || !strings.Contains(err.Error(), "KEQ0000009") {
		t.Fatalf("a Cube the backup lacks: %v", err)
	}
	rep, err = s.Restore(ctx, RestoreSelection{Scopes: []string{ScopeUsers, ScopeCubes}, Serials: []string{"KEQ0000000"}, Source: from, OpaqueKey: true})
	if err != nil {
		t.Fatalf("with the users: %v %+v", err, rep.Missing)
	}
	if cubes, _ := s.Cubes(ctx); len(cubes) != 1 || cubes[0].Serial != "KEQ0000000" {
		t.Fatalf("cubes %+v", cubes)
	}
}

// The system settings alone, this box keeping its opaque.key: the
// backup's sealed secrets are sealed again with this box's key, and the
// box's identifiers stay.
func TestRestoreSettingsKeepsTheKey(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	a := newRestoreBox(t)
	from := backupDir(t, a.s)
	s := open(t, t.TempDir())
	s.SetSetting(ctx, "poll_interval", "5s")
	s.SetSetting(ctx, "stale_after", "1h")
	node := s.NodeID()
	if _, err := s.Restore(ctx, RestoreSelection{Scopes: []string{ScopeSettings}, Source: from}); err != nil {
		t.Fatal(err)
	}
	if v, ok, err := s.SecretSetting(ctx, VAPIDKey); err != nil || !ok || string(v) != `"push key"` {
		t.Errorf("push key %s %v %v", v, ok, err)
	}
	if tg, ok, _ := s.ServerBackupTarget(ctx, a.targetUID); !ok || tg.Secret != "target secret" || tg.SecretLost {
		t.Errorf("target %+v", tg)
	}
	if v, _ := s.SettingString(ctx, "poll_interval"); v != "30s" {
		t.Errorf("poll_interval %q", v)
	}
	if _, ok, _ := s.Setting(ctx, "stale_after"); ok {
		t.Error("a setting the backup lacks stayed")
	}
	if v, _ := s.SettingString(ctx, NodeIDKey); v != node {
		t.Error("the node id changed without the identity")
	}
}

// On the same box: a user added since goes, kept removed where the audit
// names him; a Cube added since stays; the Cube restored has its name
// back; the owner's own session stays.
func TestRestoreSameBox(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	a := newRestoreBox(t)
	s := a.s
	from := backupDir(t, s)
	dave, err := s.AddUser(ctx, User{Name: "dave", Role: RoleViewer}, "x", 0)
	if err != nil {
		t.Fatal(err)
	}
	eve, _ := s.AddUser(ctx, User{Name: "eve", Role: RoleViewer}, "x", 0)
	s.Audit(ctx, AuditEntry{EffectiveUserID: dave.ID, Action: "login"})
	anna, _, _ := s.User(ctx, "anna")
	annaSession, annaSecret, _ := s.AddToken(ctx, anna.ID, "session", "anna's browser", time.Now().Add(time.Hour))
	extra, _ := s.AddCube(ctx, Cube{Serial: "KEQ0000002", Configured: true})
	if _, err := s.UpdateCube(ctx, "KEQ0000000", func(c *Cube) error { c.Name = "renamed"; return nil }, 0); err != nil {
		t.Fatal(err)
	}
	rep, err := s.Restore(ctx, RestoreSelection{Scopes: []string{ScopeUsers, ScopeCubes}, Serials: []string{"KEQ0000000"}, Source: from, OpaqueKey: true})
	if err != nil {
		t.Fatalf("%v %+v", err, rep)
	}
	var users RestoreTable
	for _, tb := range rep.Tables {
		if tb.Table == "users" {
			users = tb
		}
	}
	if users.Removed != 2 || users.KeptRemoved != 1 || users.Replaced != 2 {
		t.Errorf("users %+v", users)
	}
	if _, ok, _ := s.User(ctx, "dave"); ok {
		t.Error("dave stayed")
	}
	if n := rows(t, s, "users WHERE name = 'dave' AND deleted_at IS NOT NULL"); n != 1 {
		t.Errorf("dave, whom the audit names, not kept removed: %d", n)
	}
	if n := rows(t, s, "users WHERE id = "+strconv.FormatInt(eve.ID, 10)); n != 0 {
		t.Error("eve, whom nothing names, stayed")
	}
	if c, ok, _ := s.Cube(ctx, "KEQ0000000"); !ok || c.Name != "cube KEQ0000000" || c.ID != a.cubes[0].ID {
		t.Errorf("the Cube restored %+v", c)
	}
	if _, ok, _ := s.Cube(ctx, extra.Serial); !ok {
		t.Error("a Cube added since went")
	}
	if tok, ok, _ := s.TokenByHash(ctx, HashToken(annaSecret)); !ok || tok.ID != annaSession.ID {
		t.Error("the owner's own session went")
	}
}

// A backup without an owner does not take users and sign-in from a box
// with one.
func TestRestoreKeepsAnOwner(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	empty := open(t, t.TempDir())
	from := backupDir(t, empty)
	a := newRestoreBox(t)
	if _, err := a.s.Restore(ctx, RestoreSelection{Scopes: []string{ScopeUsers}, Source: from, OpaqueKey: true}); !errors.Is(err, ErrRestoreNoOwner) {
		t.Fatalf("no owner: %v", err)
	}
	if _, ok, _ := a.s.User(ctx, "anna"); !ok {
		t.Fatal("the refused restore removed the owner")
	}
}
