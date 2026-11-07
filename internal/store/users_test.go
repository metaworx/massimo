// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package store

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

// A password hashes to a PHC string that verifies, and nothing else
// does.
func TestPasswordHash(t *testing.T) {
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

	// Removal tombstones the row and revokes its tokens; the name comes
	// back over the same row.
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
	if gone, ok, _ := s.UserByID(ctx, bob.ID); !ok || gone.Name != "bob" {
		t.Fatalf("removed user by id: %+v %v", gone, ok)
	}
	if tk, _, _ := s.Token(ctx, tok.ID); tk.RevokedAt.IsZero() {
		t.Fatal("token of a removed user not revoked")
	}
	if err := s.DeleteUser(ctx, "bob", 0); !errors.Is(err, ErrNoUser) {
		t.Fatalf("remove twice: %v", err)
	}
	back, err := s.AddUser(ctx, User{Name: "bob", Role: RoleGuest}, "new", 0)
	if err != nil || back.ID != bob.ID || back.UUID != bob.UUID || back.Role != RoleGuest || !back.HasPassword || back.Version <= bob.Version {
		t.Fatalf("take over %+v %v (was %+v)", back, err, bob)
	}
}

// The last enabled owner cannot be disabled, demoted or removed.
func TestLastOwner(t *testing.T) {
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
