// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package store

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"
)

// MASSIMO_API_TOKEN creates the owner at the first start, is added to
// an owner's keys when it changes, and is revoked when it goes.
func TestBootstrap(t *testing.T) {
	var logged bytes.Buffer
	s, err := Open(t.TempDir(), Options{Logger: slog.New(slog.NewTextHandler(&logged, nil))})
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
	logged.Reset()
	res, err = s.Bootstrap(ctx, "secret-one")
	if err != nil || !res.OwnerCreated || !res.TokenAdded || res.Users != 1 || !strings.Contains(logged.String(), "owner created from MASSIMO_API_TOKEN") {
		t.Fatalf("%+v %v %s", res, err, logged.String())
	}
	owner, ok, _ := s.User(ctx, BootstrapUser)
	if !ok || owner.Role != RoleOwner || owner.HasPassword {
		t.Fatalf("owner %+v %v", owner, ok)
	}
	tok, ok, _ := s.TokenByHash(ctx, HashToken("secret-one"))
	if !ok || tok.UserID != owner.ID || tok.Kind != TokenAPI || tok.Label != BootstrapLabel || !tok.ExpiresAt.IsZero() {
		t.Fatalf("token %+v %v", tok, ok)
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
