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
	t.Parallel()
	clock := time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC)
	s, err := Open(copied(t), Options{Logger: quiet(), Now: func() time.Time { return clock }})
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
	if got, ok, _ := s.Token(ctx, key.ID); !ok || got.DeletedAt.IsZero() {
		t.Fatalf("revoked key %+v %v", got, ok)
	}
}

// A session made from a key names it (migration 0025): the key's next
// session revokes the earlier one, revoking the key revokes its session
// and what that subscribed to push, a session cannot make one, and a
// session whose key a binary before 0025 revoked counts no more.
func TestKeySessions(t *testing.T) {
	t.Parallel()
	clock := time.Date(2026, 11, 18, 12, 0, 0, 0, time.UTC)
	s, err := Open(copied(t), Options{Logger: quiet(), Now: func() time.Time { return clock }})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx := context.Background()
	anna, _ := s.AddUser(ctx, User{Name: "anna", Role: RoleOperator}, "pw", 0)
	key, _, _ := s.AddToken(ctx, anna.ID, TokenAPI, "phone, login by fingerprint", zeroTime)
	other, _, _ := s.AddToken(ctx, anna.ID, TokenAPI, "Loxone", zeroTime)
	plain, plainSecret, _ := s.AddToken(ctx, anna.ID, TokenSession, "Firefox", clock.Add(time.Hour))
	end := clock.Add(30 * 24 * time.Hour)

	first, firstSecret, err := s.AddKeySession(ctx, key.ID, "Massimo/1 (Android 17; SM-X)", end)
	if err != nil || first.Kind != TokenSession || first.ParentID != key.ID || first.UserID != anna.ID || !first.ExpiresAt.Equal(end) || len(firstSecret) != 64 {
		t.Fatalf("first session %+v %q %v", first, firstSecret, err)
	}
	if got, ok, _ := s.TokenByHash(ctx, HashToken(firstSecret)); !ok || got.ParentID != key.ID {
		t.Fatalf("first by hash %+v %v", got, ok)
	}
	sub, _, err := s.AddPushSubscription(ctx, PushSubscription{UserID: anna.ID, TokenID: first.ID, Endpoint: "https://push.example.net/a", P256dh: "k", Auth: "a"})
	if err != nil {
		t.Fatal(err)
	}

	// The next session ends the earlier one, and its subscription.
	second, secondSecret, err := s.AddKeySession(ctx, key.ID, "", end)
	if err != nil || second.ParentID != key.ID {
		t.Fatalf("second session %+v %v", second, err)
	}
	if _, ok, _ := s.TokenByHash(ctx, HashToken(firstSecret)); ok {
		t.Fatal("the earlier session lives on beside the next")
	}
	if _, ok, _ := s.PushSubscription(ctx, sub.ID); ok {
		t.Fatal("the earlier session's subscription stayed")
	}
	if _, ok, _ := s.TokenByHash(ctx, HashToken(secondSecret)); !ok {
		t.Fatal("the second session not found")
	}

	// Neither a session nor a revoked key makes one.
	if _, _, err := s.AddKeySession(ctx, plain.ID, "", end); !errors.Is(err, ErrNoToken) {
		t.Fatalf("a session made a session: %v", err)
	}
	if _, _, err := s.AddKeySession(ctx, 9999, "", end); !errors.Is(err, ErrNoToken) {
		t.Fatalf("no key made a session: %v", err)
	}

	// Revoking the key ends its session; the other key and the
	// sign-in's session stay.
	if err := s.RevokeToken(ctx, key.ID); err != nil {
		t.Fatal(err)
	}
	if got, ok, _ := s.Token(ctx, second.ID); !ok || got.DeletedAt.IsZero() {
		t.Fatalf("the key's session after its key %+v", got)
	}
	if _, _, err := s.AddKeySession(ctx, key.ID, "", end); !errors.Is(err, ErrNoToken) {
		t.Fatalf("a revoked key made a session: %v", err)
	}
	list, _ := s.Tokens(ctx, anna.ID)
	if len(list) != 2 || list[0].ID != other.ID || list[1].ID != plain.ID {
		t.Fatalf("live tokens %+v", list)
	}
	if _, ok, _ := s.TokenByHash(ctx, HashToken(plainSecret)); !ok {
		t.Fatal("the sign-in's session went with the key")
	}

	// A binary before 0025 revokes the key alone: its session counts no
	// more all the same, and is not listed.
	third, thirdSecret, err := s.AddKeySession(ctx, other.ID, "", end)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.ExecContext(ctx, `UPDATE tokens SET deleted_at = ? WHERE id = ?`, s.nowText(), other.ID); err != nil {
		t.Fatal(err)
	}
	if _, ok, _ := s.TokenByHash(ctx, HashToken(thirdSecret)); ok {
		t.Fatal("a session whose key an older binary revoked was found")
	}
	if list, _ := s.Tokens(ctx, anna.ID); len(list) != 1 || list[0].ID != plain.ID {
		t.Fatalf("live tokens after the older binary %+v (third %d)", list, third.ID)
	}
}
