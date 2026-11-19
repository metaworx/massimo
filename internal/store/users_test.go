// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package store

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"testing"
	"time"
)

// A password hashes to a PHC string that verifies, and nothing else
// does.
func TestPasswordHash(t *testing.T) {
	t.Parallel()
	h, err := HashPassword("correct horse")
	if err != nil || !strings.HasPrefix(h, "$argon2id$v=19$m=19456,t=2,p=1$") {
		t.Fatalf("%q %v", h, err)
	}
	if !VerifyPassword(h, "correct horse") || VerifyPassword(h, "wrong") || VerifyPassword("", "correct horse") || VerifyPassword("$argon2id$v=19$m=65536,t=1,p=4$x$y", "correct horse") {
		t.Fatal("verify")
	}
	h2, _ := HashPassword("correct horse")
	if h2 == h {
		t.Fatal("the salt repeats")
	}
	DummyVerify("anything")
}

// Users are added with a role and a password, listed, changed, and
// removed to a tombstone that keeps its identifier and is taken over
// when the name comes back.
func TestUsers(t *testing.T) {
	t.Parallel()
	s := open(t, t.TempDir())
	ctx := context.Background()
	anna, err := s.AddUser(ctx, User{Name: " Anna ", DisplayName: "Anna", Role: RoleOperator}, "secret", 0)
	if err != nil || anna.Name != "anna" || anna.Role != RoleOperator || !anna.HasPassword || anna.UUID == "" || anna.Version != 1 || anna.AuthVersion != 1 || anna.Disabled {
		t.Fatalf("add %+v %v", anna, err)
	}
	if _, err := s.AddUser(ctx, User{Name: "anna"}, "", 0); !errors.Is(err, ErrUserExists) {
		t.Fatalf("add twice: %v", err)
	}
	for _, name := range []string{"", "Anna Smith", "-anna", strings.Repeat("a", 65)} {
		if _, err := s.AddUser(ctx, User{Name: name}, "", 0); !errors.Is(err, ErrBadName) {
			t.Fatalf("name %q: %v", name, err)
		}
	}
	if _, err := s.AddUser(ctx, User{Name: "bob", Role: "nope"}, "", 0); !errors.Is(err, ErrNoRole) {
		t.Fatalf("unknown role: %v", err)
	}
	bob, err := s.AddUser(ctx, User{Name: "bob"}, "", anna.ID)
	if err != nil || bob.Role != RoleViewer || bob.HasPassword || bob.InvitedBy != anna.ID {
		t.Fatalf("bob %+v %v", bob, err)
	}
	if n, _ := s.CountUsers(ctx); n != 2 {
		t.Fatalf("count %d", n)
	}
	all, _ := s.Users(ctx)
	if len(all) != 2 || all[0].Name != "anna" || all[1].Name != "bob" {
		t.Fatalf("users %+v", all)
	}
	if err := s.CheckPassword(ctx, "ANNA", "secret"); err != nil {
		t.Fatalf("right password: %v", err)
	}
	for _, c := range []struct{ name, pw string }{{"anna", "wrong"}, {"bob", "secret"}, {"nobody", "secret"}} {
		if err := s.CheckPassword(ctx, c.name, c.pw); !errors.Is(err, ErrBadPassword) {
			t.Fatalf("%s/%s: %v", c.name, c.pw, err)
		}
	}
	u, err := s.UpdateUser(ctx, "anna", func(u *User) error { u.DisplayName, u.Role = "Anna S.", RoleManager; return nil }, bob.ID)
	if err != nil || u.DisplayName != "Anna S." || u.Role != RoleManager || u.Version != 2 {
		t.Fatalf("update %+v %v", u, err)
	}
	if _, err := s.UpdateUser(ctx, "nobody", func(*User) error { return nil }, 0); !errors.Is(err, ErrNoUser) {
		t.Fatalf("update nobody: %v", err)
	}
	if err := s.SetPassword(ctx, "bob", "pw", 0); err != nil {
		t.Fatal(err)
	}
	if err := s.CheckPassword(ctx, "bob", "pw"); err != nil {
		t.Fatal(err)
	}
	if err := s.SetPassword(ctx, "bob", "", 0); err != nil {
		t.Fatal(err)
	}
	if b, _, _ := s.User(ctx, "bob"); b.HasPassword {
		t.Fatal("password not removed")
	}
	if err := s.NoteLogin(ctx, anna.ID); err != nil {
		t.Fatal(err)
	}
	if a, _, _ := s.User(ctx, "anna"); a.LastLoginAt.IsZero() {
		t.Fatal("login not noted")
	}

	// Removal tombstones the row, which keeps its name, and revokes its
	// tokens; the name is free for a user of its own (migration 0024).
	tok, _, err := s.AddToken(ctx, bob.ID, TokenAPI, "k", zeroTime)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteUser(ctx, "bob", anna.ID); err != nil {
		t.Fatal(err)
	}
	if _, ok, _ := s.User(ctx, "bob"); ok {
		t.Fatal("removed user still found by name")
	}
	gone, ok, _ := s.UserByID(ctx, bob.ID)
	if !ok || gone.Name != "bob" {
		t.Fatalf("removed user by id: %+v %v", gone, ok)
	}
	if tk, _, _ := s.Token(ctx, tok.ID); tk.DeletedAt.IsZero() {
		t.Fatal("token of a removed user not revoked")
	}
	if err := s.DeleteUser(ctx, "bob", 0); !errors.Is(err, ErrNoUser) {
		t.Fatalf("remove twice: %v", err)
	}
	back, err := s.AddUser(ctx, User{Name: "bob", Role: RoleGuest}, "new", 0)
	if err != nil || back.ID == bob.ID || back.UUID == bob.UUID || back.Role != RoleGuest || !back.HasPassword || back.Version != 1 {
		t.Fatalf("a new bob %+v %v (was %+v)", back, err, bob)
	}
	if again, ok, _ := s.UserByID(ctx, bob.ID); !ok || again.Name != "bob" {
		t.Fatalf("the removed bob: %+v", again)
	}
	if id, ok, _ := s.UserIDByName(ctx, "bob"); !ok || id != back.ID {
		t.Fatalf("bob's id by name: %d, want the live one's %d", id, back.ID)
	}
	if _, err := s.AddUser(ctx, User{Name: "bob"}, "", 0); !errors.Is(err, ErrUserExists) {
		t.Fatalf("a live user's name: %v", err)
	}
	// Removed again: two removed rows of the name, the newest found by it.
	if err := s.DeleteUser(ctx, "bob", 0); err != nil {
		t.Fatal(err)
	}
	if second, _, _ := s.UserByID(ctx, back.ID); second.Name != "bob" {
		t.Fatalf("the second removed bob: %q", second.Name)
	}
	if id, ok, _ := s.UserIDByName(ctx, "bob"); !ok || id != back.ID {
		t.Fatalf("bob's id by name: %d, want the last removed %d", id, back.ID)
	}
}

// A rename moves the user and its password to the new name: the record
// is registered under it, the old name signs in no more; keys and
// preferences stay with the row. A name taken by a live user, a bad
// one and an unknown or removed user are refused; no password leaves
// none.
func TestRenameUser(t *testing.T) {
	t.Parallel()
	s := open(t, t.TempDir())
	ctx := context.Background()
	anna, err := s.AddUserOpaque(ctx, User{Name: "anna", Role: RoleOperator, DisplayName: "Anna"}, "pw", 0)
	if err != nil {
		t.Fatal(err)
	}
	tok, _, _ := s.AddToken(ctx, anna.ID, TokenAPI, "k", zeroTime)
	s.SetPreference(ctx, anna.ID, "ui", json.RawMessage(`{"theme":"dark"}`))
	s.AddUser(ctx, User{Name: "bob"}, "", 0)
	s.AddUser(ctx, User{Name: "gone"}, "", 0)
	s.DeleteUser(ctx, "gone", 0)

	for _, c := range []struct {
		from, to string
		want     error
	}{
		{"anna", "bob", ErrUserExists},
		{"anna", "Bad Name", ErrBadName}, {"nobody", "carl", ErrNoUser}, {"gone", "carl", ErrNoUser},
	} {
		if _, err := s.RenameUser(ctx, c.from, c.to, "x", 0); !errors.Is(err, c.want) {
			t.Errorf("rename %s to %s: %v, want %v", c.from, c.to, err, c.want)
		}
	}
	got, err := s.RenameUser(ctx, "Anna", "Annabel", "new-pw", anna.ID)
	if err != nil || got.ID != anna.ID || got.Name != "annabel" || got.DisplayName != "Anna" || got.Role != RoleOperator || got.AuthVersion != 2 || got.Version <= anna.Version {
		t.Fatalf("renamed: %+v %v", got, err)
	}
	if !opaqueSignsIn(t, s, "annabel", "new-pw") || opaqueSignsIn(t, s, "anna", "pw") || opaqueSignsIn(t, s, "annabel", "pw") {
		t.Fatal("the password under the new name only")
	}
	if _, ok, _ := s.User(ctx, "anna"); ok {
		t.Fatal("the old name still found")
	}
	if tk, ok, _ := s.Token(ctx, tok.ID); !ok || !tk.DeletedAt.IsZero() || tk.UserID != anna.ID {
		t.Fatalf("the key: %+v", tk)
	}
	if list, _ := s.Preferences(ctx, anna.ID); len(list) != 1 {
		t.Fatalf("the preferences: %+v", list)
	}
	if _, err := s.AddUser(ctx, User{Name: "anna"}, "", 0); err != nil {
		t.Fatalf("the old name is free: %v", err)
	}
	none, err := s.RenameUser(ctx, "annabel", "bel", "", 0)
	if err != nil || none.HasPassword || none.AuthVersion != 1 || opaqueSignsIn(t, s, "bel", "new-pw") {
		t.Fatalf("no password: %+v %v", none, err)
	}
}

// The last enabled owner cannot be disabled, demoted or removed.
func TestLastOwner(t *testing.T) {
	t.Parallel()
	s := open(t, t.TempDir())
	ctx := context.Background()
	o1, _ := s.AddUser(ctx, User{Name: "o1", Role: RoleOwner}, "", 0)
	disable := func(u *User) error { u.Disabled = true; return nil }
	demote := func(u *User) error { u.Role = RoleViewer; return nil }
	if _, err := s.UpdateUser(ctx, "o1", disable, 0); !errors.Is(err, ErrLastOwner) {
		t.Fatalf("disable the last owner: %v", err)
	}
	if _, err := s.UpdateUser(ctx, "o1", demote, 0); !errors.Is(err, ErrLastOwner) {
		t.Fatalf("demote the last owner: %v", err)
	}
	if err := s.DeleteUser(ctx, "o1", 0); !errors.Is(err, ErrLastOwner) {
		t.Fatalf("remove the last owner: %v", err)
	}
	s.AddUser(ctx, User{Name: "o2", Role: RoleOwner}, "", 0)
	if _, err := s.UpdateUser(ctx, "o1", disable, 0); err != nil {
		t.Fatalf("disable one of two owners: %v", err)
	}
	if err := s.DeleteUser(ctx, "o2", 0); !errors.Is(err, ErrLastOwner) {
		t.Fatalf("remove the last enabled owner: %v", err)
	}
	if _, err := s.UpdateUser(ctx, "o1", func(u *User) error { u.Disabled = false; return nil }, 0); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteUser(ctx, "o2", 0); err != nil {
		t.Fatalf("remove an owner while another is enabled: %v", err)
	}
	if u, _, _ := s.User(ctx, "o1"); u.ID != o1.ID || u.Disabled {
		t.Fatalf("o1 %+v", u)
	}
}

// A guest's end date is kept and compared.
func TestGuestEndDate(t *testing.T) {
	t.Parallel()
	s := open(t, t.TempDir())
	ctx := context.Background()
	end := time.Date(2026, 12, 24, 18, 0, 0, 0, time.UTC)
	g, err := s.AddUser(ctx, User{Name: "guest1", Role: RoleGuest, ExpiresAt: end}, "pw", 0)
	if err != nil || !g.ExpiresAt.Equal(end) {
		t.Fatalf("%+v %v", g, err)
	}
	if g.Expired(end.Add(-time.Minute)) || !g.Expired(end) || !g.Expired(end.Add(time.Hour)) {
		t.Fatal("expired")
	}
	g, _ = s.UpdateUser(ctx, "guest1", func(u *User) error { u.ExpiresAt = time.Time{}; return nil }, 0)
	if !g.ExpiresAt.IsZero() || g.Expired(end.Add(time.Hour)) {
		t.Fatalf("end date not cleared: %+v", g)
	}
}

// An owner with an end date is no owner left: the last owner without one
// cannot be given one, and while the only other owner has one, the last
// cannot be demoted, disabled or removed.
func TestLastOwnerWithEndDate(t *testing.T) {
	t.Parallel()
	s := open(t, t.TempDir())
	ctx := context.Background()
	s.AddUser(ctx, User{Name: "o1", Role: RoleOwner}, "", 0)
	tomorrow := time.Now().Add(24 * time.Hour).UTC().Truncate(time.Second)
	ends := func(u *User) error { u.ExpiresAt = tomorrow; return nil }
	if _, err := s.UpdateUser(ctx, "o1", ends, 0); !errors.Is(err, ErrLastOwner) {
		t.Fatalf("an end date for the last owner: %v", err)
	}
	if _, err := s.AddUser(ctx, User{Name: "o2", Role: RoleOwner, ExpiresAt: tomorrow}, "", 0); err != nil {
		t.Fatal(err)
	}
	if _, err := s.UpdateUser(ctx, "o1", func(u *User) error { u.Role = RoleViewer; return nil }, 0); !errors.Is(err, ErrLastOwner) {
		t.Fatalf("demote the last owner beside one that ends: %v", err)
	}
	if _, err := s.UpdateUser(ctx, "o1", func(u *User) error { u.Disabled = true; return nil }, 0); !errors.Is(err, ErrLastOwner) {
		t.Fatalf("disable the last owner beside one that ends: %v", err)
	}
	if err := s.DeleteUser(ctx, "o1", 0); !errors.Is(err, ErrLastOwner) {
		t.Fatalf("remove the last owner beside one that ends: %v", err)
	}
	if _, err := s.UpdateUser(ctx, "o2", func(u *User) error { u.ExpiresAt = time.Time{}; return nil }, 0); err != nil {
		t.Fatal(err)
	}
	if _, err := s.UpdateUser(ctx, "o1", ends, 0); err != nil {
		t.Fatalf("an end date beside an owner without one: %v", err)
	}
	if err := s.DeleteUser(ctx, "o1", 0); err != nil {
		t.Fatalf("remove an owner that ends: %v", err)
	}
}

// opaqueSignsIn reports whether a sign-in by OPAQUE for the name with the
// password goes through, as the browser's client runs it.
func opaqueSignsIn(t *testing.T, s *Store, name, password string) bool {
	t.Helper()
	start, ke3 := newTestClient(t).login(s, name, password)
	return ke3 != nil && s.FinishOpaque(start, ke3) == nil
}

// Migration 0015 renamed the users removed before it as DeleteUser did
// then, -<id> added where a user had that name already, and left the
// live ones (AP UserCli v1.0, Block 2); 0024 takes the suffixes off
// again, a name being unique among the users not removed from then on.
func TestMigration15(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	dir := t.TempDir()
	s14 := openAt(t, dir, 14)
	for _, q := range []string{
		`INSERT INTO users (uuid, name, role_id, created_at, updated_at, deleted_at) SELECT 'u1', 'anna', id, '2026-11-01T10:00:00.000Z', '2026-11-02T08:05:09.123Z', '2026-11-02T08:05:09.123Z' FROM roles WHERE key = 'viewer'`,
		`INSERT INTO users (uuid, name, role_id, created_at, updated_at, deleted_at) SELECT 'u2', 'bob', id, '2026-11-01T10:00:00.000Z', '2026-11-03T23:59:00.000Z', '2026-11-03T23:59:00.000Z' FROM roles WHERE key = 'viewer'`,
		`INSERT INTO users (uuid, name, role_id, created_at, updated_at) SELECT 'u3', 'bob-del-20261103-2359', id, '2026-11-04T10:00:00.000Z', '2026-11-04T10:00:00.000Z' FROM roles WHERE key = 'viewer'`,
		`INSERT INTO users (uuid, name, role_id, created_at, updated_at) SELECT 'u4', 'carl', id, '2026-11-04T10:00:00.000Z', '2026-11-04T10:00:00.000Z' FROM roles WHERE key = 'viewer'`,
	} {
		if _, err := s14.db.ExecContext(ctx, q); err != nil {
			t.Fatal(err)
		}
	}
	s14.Close()
	names := func(s *Store) (map[string]string, int64) {
		t.Helper()
		out := map[string]string{}
		rows, err := s.db.QueryContext(ctx, `SELECT uuid, name, id FROM users WHERE uuid LIKE 'u_'`)
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		var bobID int64
		for rows.Next() {
			var uuid, name string
			var id int64
			rows.Scan(&uuid, &name, &id)
			out[uuid] = name
			if uuid == "u2" {
				bobID = id
			}
		}
		return out, bobID
	}
	s15 := openAt(t, dir, 15)
	got, bobID := names(s15)
	want := map[string]string{"u1": "anna-del-20261102-0805", "u2": "bob-del-20261103-2359-" + strconv.FormatInt(bobID, 10), "u3": "bob-del-20261103-2359", "u4": "carl"}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("0015, %s: %q, want %q", k, got[k], v)
		}
	}
	s15.Close()
	s := open(t, dir)
	got, _ = names(s)
	want = map[string]string{"u1": "anna", "u2": "bob", "u3": "bob-del-20261103-2359", "u4": "carl"}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("0024, %s: %q, want %q", k, got[k], v)
		}
	}
	if _, err := s.AddUser(ctx, User{Name: "anna"}, "", 0); err != nil {
		t.Fatalf("the name of a user removed before: %v", err)
	}
}
