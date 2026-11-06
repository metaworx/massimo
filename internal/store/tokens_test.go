// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package store

import (
	"context"
	"errors"
	"testing"
	"time"
)

// A token is found by the hash of its secret while it is live, touched
// on use, and revoked one at a time or by kind.
func TestTokens(t *testing.T) {
	clock := time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC)
	s, err := Open(t.TempDir(), Options{Logger: quiet(), Now: func() time.Time { return clock }})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx := context.Background()
	anna, _ := s.AddUser(ctx, User{Name: "anna", Role: RoleOperator}, "pw", 0)
	end := clock.Add(30 * 24 * time.Hour)
	sess, secret, err := s.AddToken(ctx, anna.ID, TokenSession, "Firefox", end)
	if err != nil || len(secret) != 64 || sess.Kind != TokenSession || !sess.ExpiresAt.Equal(end) || sess.UUID == "" {
		t.Fatalf("session %+v %q %v", sess, secret, err)
	}
	key, keySecret, _ := s.AddToken(ctx, anna.ID, TokenAPI, "Loxone", zeroTime)
	if key.Expired(clock.Add(100 * 365 * 24 * time.Hour)) {
		t.Fatal("an api key expires")
	}
	if _, _, err := s.AddToken(ctx, anna.ID, "other", "", zeroTime); err == nil {
		t.Fatal("unknown kind accepted")
	}
	got, ok, err := s.TokenByHash(ctx, HashToken(secret))
	if err != nil || !ok || got.ID != sess.ID || got.UserID != anna.ID {
		t.Fatalf("by hash %+v %v %v", got, ok, err)
	}
	if _, ok, _ := s.TokenByHash(ctx, HashToken("nope")); ok {
		t.Fatal("unknown hash found")
	}
	if list, _ := s.Tokens(ctx, anna.ID); len(list) != 2 || list[0].ID != sess.ID {
		t.Fatalf("tokens %+v", list)
	}
	clock = clock.Add(time.Hour)
	if err := s.TouchToken(ctx, sess.ID, "10.0.0.5", clock.Add(30*24*time.Hour)); err != nil {
		t.Fatal(err)
	}
	got, _, _ = s.Token(ctx, sess.ID)
	if !got.LastUsedAt.Equal(clock) || got.LastUsedFrom != "10.0.0.5" || !got.ExpiresAt.Equal(clock.Add(30*24*time.Hour)) {
		t.Fatalf("touched %+v", got)
	}
	s.TouchToken(ctx, key.ID, "10.0.0.6", zeroTime)
	if got, _, _ := s.Token(ctx, key.ID); !got.ExpiresAt.IsZero() {
		t.Fatal("touch gave the key an end")
	}
	if !got.Expired(got.ExpiresAt) || got.Expired(got.ExpiresAt.Add(-time.Second)) {
		t.Fatal("expired")
	}

	// A password set by someone else revokes the sessions, not the keys.
	if err := s.RevokeSessions(ctx, anna.ID); err != nil {
		t.Fatal(err)
	}
	if _, ok, _ := s.TokenByHash(ctx, HashToken(secret)); ok {
		t.Fatal("revoked session found")
	}
	if _, ok, _ := s.TokenByHash(ctx, HashToken(keySecret)); !ok {
		t.Fatal("api key revoked with the sessions")
	}
	if err := s.RevokeToken(ctx, sess.ID); !errors.Is(err, ErrNoToken) {
		t.Fatalf("revoke twice: %v", err)
	}
	if err := s.RevokeToken(ctx, key.ID); err != nil {
		t.Fatal(err)
	}
	if list, _ := s.Tokens(ctx, anna.ID); len(list) != 0 {
		t.Fatalf("live tokens %+v", list)
	}
	if got, ok, _ := s.Token(ctx, key.ID); !ok || got.RevokedAt.IsZero() {
		t.Fatalf("revoked key %+v %v", got, ok)
	}
}
