// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package store

import (
	"context"
	"testing"

	"go.metaworx.ch/massimo/internal/testkit"
)

// MASSIMO_API_TOKEN creates the owner at the first start, is added to
// an owner's keys when it changes, and is revoked when it goes.
func TestBootstrap(t *testing.T) {
	t.Parallel()
	log, logged := testkit.LogWatch(nil)
	s, err := Open(copied(t), Options{Logger: log})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx := context.Background()

	// No token, no users: nothing.
	res, err := s.Bootstrap(ctx, "")
	if err != nil || res.Users != 0 || res.OwnerCreated {
		t.Fatalf("%+v %v", res, err)
	}
	// The first start with a token.
	m := logged.Mark()
	res, err = s.Bootstrap(ctx, "secret-one")
	if err != nil || !res.OwnerCreated || !res.TokenAdded || res.Users != 1 || logged.Count(m, testkit.Msg("owner created from MASSIMO_API_TOKEN")) == 0 {
		t.Fatalf("%+v %v %v", res, err, logged.Since(m, nil))
	}
	owner, ok, _ := s.User(ctx, BootstrapUser)
	if !ok || owner.Role != RoleOwner || owner.HasPassword {
		t.Fatalf("owner %+v %v", owner, ok)
	}
	tok, ok, _ := s.TokenByHash(ctx, HashToken("secret-one"))
	if !ok || tok.UserID != owner.ID || tok.Kind != TokenAPI || tok.Label != BootstrapLabel || !tok.ExpiresAt.IsZero() {
		t.Fatalf("token %+v %v", tok, ok)
	}
	if tok.RevokeKept {
		// the store never held the file's secret, and no revoke ends it
		t.Fatal("the settings file's key has a revoke code")
	}
	// The same token again: nothing changes.
	res, err = s.Bootstrap(ctx, "secret-one")
	if err != nil || res.OwnerCreated || res.TokenAdded || res.Revoked != 0 {
		t.Fatalf("second start %+v %v", res, err)
	}
	if list, _ := s.Tokens(ctx, owner.ID); len(list) != 1 {
		t.Fatalf("tokens %+v", list)
	}
	// A changed token: the old one is revoked, the new one added.
	res, err = s.Bootstrap(ctx, "secret-two")
	if err != nil || res.OwnerCreated || !res.TokenAdded || res.Revoked != 1 {
		t.Fatalf("changed token %+v %v", res, err)
	}
	if _, ok, _ := s.TokenByHash(ctx, HashToken("secret-one")); ok {
		t.Fatal("old token still works")
	}
	if _, ok, _ := s.TokenByHash(ctx, HashToken("secret-two")); !ok {
		t.Fatal("new token not added")
	}
	// A key of the owner's own is not touched by a removed token.
	own, _, _ := s.AddToken(ctx, owner.ID, TokenAPI, "Loxone", zeroTime)
	res, err = s.Bootstrap(ctx, "")
	if err != nil || res.Revoked != 1 {
		t.Fatalf("removed token %+v %v", res, err)
	}
	if _, ok, _ := s.TokenByHash(ctx, HashToken("secret-two")); ok {
		t.Fatal("removed token still works")
	}
	if list, _ := s.Tokens(ctx, owner.ID); len(list) != 1 || list[0].ID != own.ID {
		t.Fatalf("the owner's own key: %+v", list)
	}
	// With the owner disabled and another enabled, the token goes to
	// the enabled one.
	anna, _ := s.AddUser(ctx, User{Name: "anna", Role: RoleOwner}, "pw", 0)
	s.UpdateUser(ctx, BootstrapUser, func(u *User) error { u.Disabled = true; return nil }, 0)
	res, _ = s.Bootstrap(ctx, "secret-three")
	tok, _, _ = s.TokenByHash(ctx, HashToken("secret-three"))
	if !res.TokenAdded || tok.UserID != anna.ID {
		t.Fatalf("token for the enabled owner: %+v %+v", res, tok)
	}
}

// MASSIMO_API_TOKEN on a store whose only users are an integration's
// makes the owner with the token as its key, as on a store without
// users (AP LooseEnds, D1).
func TestBootstrapBesideIntegrationUsers(t *testing.T) {
	t.Parallel()
	s := open(t, t.TempDir())
	ctx := context.Background()
	integrationOnly(t, s)
	res, err := s.Bootstrap(ctx, "secret-one")
	if err != nil || !res.OwnerCreated || !res.TokenAdded || res.Users != 1 {
		t.Fatalf("%+v %v", res, err)
	}
	owner, ok, _ := s.User(ctx, BootstrapUser)
	tok, found, _ := s.TokenByHash(ctx, HashToken("secret-one"))
	if !ok || owner.Role != RoleOwner || !found || tok.UserID != owner.ID {
		t.Fatalf("owner %+v %v, token %+v %v", owner, ok, tok, found)
	}
}

// MASSIMO_API_TOKEN goes to the first regular owner, where an
// integration's user with the role owner came before it: such a user
// holds no key.
func TestBootstrapBesideAnIntegrationsOwner(t *testing.T) {
	t.Parallel()
	s := open(t, t.TempDir())
	ctx := context.Background()
	i := integrationOnly(t, s)
	if _, err := s.UpdateUser(ctx, i.UUID+":", func(u *User) error { u.Role = RoleOwner; return nil }, 0); err != nil {
		t.Fatal(err)
	}
	anna, err := s.AddUser(ctx, User{Name: "anna", Role: RoleOwner}, "", 0)
	if err != nil {
		t.Fatal(err)
	}
	res, err := s.Bootstrap(ctx, "secret-one")
	tok, ok, _ := s.TokenByHash(ctx, HashToken("secret-one"))
	if err != nil || !res.TokenAdded || !ok || tok.UserID != anna.ID {
		t.Fatalf("%+v %v, the token %+v %v", res, err, tok, ok)
	}
}

// The settings file's key that an older start gave to an integration's
// user with the role owner moves at the start to a regular owner, the
// first enabled one, or the owner it makes where there is none: then the
// server is in users mode and the token is its way in. Its sessions go,
// and the log says from whom it was taken (AP LooseEnds, D1).
func TestBootstrapTakesTheKeyFromAnIntegrationsUser(t *testing.T) {
	t.Parallel()
	for _, regular := range []bool{true, false} {
		t.Run(map[bool]string{true: "beside a regular owner", false: "no regular owner"}[regular], func(t *testing.T) {
			t.Parallel()
			s := open(t, t.TempDir())
			log, logged := testkit.LogWatch(nil)
			s.log = log
			ctx := context.Background()
			i := integrationOnly(t, s)
			own, err := s.UpdateUser(ctx, i.UUID+":", func(u *User) error { u.Role = RoleOwner; return nil }, 0)
			if err != nil {
				t.Fatal(err)
			}
			// as an older start gave it
			held, err := s.addTokenHash(ctx, own.ID, TokenAPI, HashToken("file"), "", zeroTime, KeyOptions{Label: BootstrapLabel}, true)
			if err != nil {
				t.Fatal(err)
			}
			session, _, err := s.addToken(ctx, own.ID, TokenSession, "x", zeroTime, true)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := s.db.ExecContext(ctx, `UPDATE tokens SET parent_id = ? WHERE id = ?`, held.ID, session.ID); err != nil {
				t.Fatal(err)
			}
			want := BootstrapUser
			if regular {
				if _, err := s.AddUser(ctx, User{Name: "anna", Role: RoleOwner}, "", 0); err != nil {
					t.Fatal(err)
				}
				want = "anna"
			}
			m := logged.Mark()
			res, err := s.Bootstrap(ctx, "file")
			if err != nil || !res.TokenAdded || res.OwnerCreated == regular {
				t.Fatalf("%+v %v", res, err)
			}
			tok, ok, _ := s.TokenByHash(ctx, HashToken("file"))
			u, _, _ := s.UserByID(ctx, tok.UserID)
			if !ok || tok.ID != held.ID || u.Name != want {
				t.Fatalf("the key %+v %v on %s", tok, ok, u.Name)
			}
			if tok.RevokeKept {
				t.Fatal("the settings file's key taken from the integration's user has a revoke code")
			}
			if got, _, _ := s.Token(ctx, session.ID); got.DeletedAt.IsZero() {
				t.Fatal("a session made from the key on the integration's user stays")
			}
			if n, _ := s.CountUsers(ctx); n != 1 {
				t.Fatalf("%d regular users", n)
			}
			if logged.Count(m, testkit.Msg("the settings file's key moved from an integration's user", "from", own.Name, "user", want)) != 1 {
				t.Fatalf("not in the log: %v", logged.Since(m, nil))
			}
		})
	}
}

// The settings file's key that an older start gave to the integration's
// own user, revoked with the integration since, moves at the start too,
// live again as the file's token, beside a regular owner and with none:
// the hash is unique over revoked rows, so a new key could not be made
// (AP LooseEnds, D1).
func TestBootstrapTakesARevokedKeyFromAnIntegrationsUser(t *testing.T) {
	t.Parallel()
	for _, regular := range []bool{true, false} {
		t.Run(map[bool]string{true: "beside a regular owner", false: "no regular owner"}[regular], func(t *testing.T) {
			t.Parallel()
			s := open(t, t.TempDir())
			ctx := context.Background()
			i := integrationOnly(t, s)
			own, err := s.UpdateUser(ctx, i.UUID+":", func(u *User) error { u.Role = RoleOwner; return nil }, 0)
			if err != nil {
				t.Fatal(err)
			}
			held, err := s.addTokenHash(ctx, own.ID, TokenAPI, HashToken("file"), "", zeroTime, KeyOptions{Label: BootstrapLabel}, true)
			if err != nil {
				t.Fatal(err)
			}
			if err := s.RevokeIntegration(ctx, i.UUID, 0); err != nil {
				t.Fatal(err)
			}
			if got, _, _ := s.Token(ctx, held.ID); got.DeletedAt.IsZero() {
				t.Fatal("the revoke left the key live")
			}
			want := BootstrapUser
			if regular {
				if _, err := s.AddUser(ctx, User{Name: "anna", Role: RoleOwner}, "", 0); err != nil {
					t.Fatal(err)
				}
				want = "anna"
			}
			res, err := s.Bootstrap(ctx, "file")
			if err != nil || !res.TokenAdded || !res.Moved {
				t.Fatalf("%+v %v", res, err)
			}
			tok, ok, _ := s.TokenByHash(ctx, HashToken("file"))
			u, _, _ := s.UserByID(ctx, tok.UserID)
			if !ok || tok.ID != held.ID || u.Name != want {
				t.Fatalf("the key %+v live %v on %s", tok, ok, u.Name)
			}
		})
	}
}

// Where the owner's key cannot be made at the start, the owner is not
// made either: the two are one transaction, so that no owner without a
// key is left on a server nobody can then get into. A removed regular
// user's revoked key of the file's hash is such a case.
func TestBootstrapLeavesNoOwnerWithoutItsKey(t *testing.T) {
	t.Parallel()
	s := open(t, t.TempDir())
	ctx := context.Background()
	old, err := s.AddUser(ctx, User{Name: "old", Role: RoleViewer}, "", 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.AddTokenHash(ctx, old.ID, TokenAPI, BootstrapLabel, HashToken("file"), zeroTime); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteUser(ctx, "old", 0); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Bootstrap(ctx, "file"); err == nil {
		t.Fatal("a key of a hash taken was made")
	}
	if _, ok, _ := s.User(ctx, BootstrapUser); ok {
		t.Fatal("an owner without its key is left")
	}
}
