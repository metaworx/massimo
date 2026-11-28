// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package store

import (
	"context"
	"database/sql"
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
	if err := s.TouchToken(ctx, sess.ID, "10.0.0.5", clock.Add(30*24*time.Hour), ""); err != nil {
		t.Fatal(err)
	}
	got, _, _ = s.Token(ctx, sess.ID)
	if !got.LastUsedAt.Equal(clock) || got.LastUsedFrom != "10.0.0.5" || !got.ExpiresAt.Equal(clock.Add(30*24*time.Hour)) {
		t.Fatalf("touched %+v", got)
	}
	s.TouchToken(ctx, key.ID, "10.0.0.6", zeroTime, "")
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

// TokenEnded tells a token the store issued and that ended, which the
// server refuses without counting it toward the lockout (AP MobileApp
// v1.4, D64-A), from a live one and from a hash no token has: revoked,
// logged out, past its end, its user removed, a session whose key was
// revoked, by this binary or alone by one before migration 0025.
func TestTokenEnded(t *testing.T) {
	t.Parallel()
	clock := time.Date(2026, 11, 23, 12, 0, 0, 0, time.UTC)
	s, err := Open(copied(t), Options{Logger: quiet(), Now: func() time.Time { return clock }})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx := context.Background()
	anna, _ := s.AddUser(ctx, User{Name: "anna", Role: RoleOperator}, "pw", 0)
	bob, _ := s.AddUser(ctx, User{Name: "bob", Role: RoleOperator}, "pw", 0)
	end := clock.Add(30 * 24 * time.Hour)
	ended := func(what, secret string, want bool) {
		t.Helper()
		got, err := s.TokenEnded(ctx, HashToken(secret), clock)
		if err != nil || got != want {
			t.Errorf("%s: ended %v %v, want %v", what, got, err, want)
		}
	}

	_, live, _ := s.AddToken(ctx, anna.ID, TokenSession, "Firefox", end)
	_, key, _ := s.AddToken(ctx, anna.ID, TokenAPI, "Loxone", zeroTime)
	ended("a live session", live, false)
	ended("a live key", key, false)
	ended("a hash no token has", "nope", false)

	loggedOut, loggedOutSecret, _ := s.AddToken(ctx, anna.ID, TokenSession, "Firefox", end)
	if err := s.RevokeToken(ctx, loggedOut.ID); err != nil {
		t.Fatal(err)
	}
	ended("a session logged out", loggedOutSecret, true)
	revokedKey, revokedKeySecret, _ := s.AddToken(ctx, anna.ID, TokenAPI, "phone, login by fingerprint", zeroTime)
	_, keySession, _ := s.AddKeySession(ctx, revokedKey.ID, "phone", end)
	if err := s.RevokeToken(ctx, revokedKey.ID); err != nil {
		t.Fatal(err)
	}
	ended("a revoked key", revokedKeySecret, true)
	ended("the session of a revoked key", keySession, true)
	_, expired, _ := s.AddToken(ctx, anna.ID, TokenSession, "Firefox", clock.Add(-time.Second))
	_, atItsEnd, _ := s.AddToken(ctx, anna.ID, TokenSession, "Firefox", clock)
	ended("a session past its end", expired, true)
	ended("a session at its end", atItsEnd, true)
	bobsTok, bobs, err := s.AddToken(ctx, bob.ID, TokenAPI, "k", zeroTime)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteUser(ctx, "bob", 0); err != nil {
		t.Fatal(err)
	}
	ended("a removed user's key", bobs, true)
	// The user's tombstone alone, the key's revocation undone by hand:
	// ended all the same, by the user removed.
	if _, err := s.db.ExecContext(ctx, `UPDATE tokens SET deleted_at = NULL WHERE id = ?`, bobsTok.ID); err != nil {
		t.Fatal(err)
	}
	if got, ok, _ := s.Token(ctx, bobsTok.ID); !ok || !got.DeletedAt.IsZero() {
		t.Fatalf("the removed user's key still revoked: %+v", got)
	}
	ended("the key of a removed user, the key itself not revoked", bobs, true)

	// A binary before 0025 revoked the key alone: its session counts no
	// more, so it has ended.
	older, _, _ := s.AddToken(ctx, anna.ID, TokenAPI, "phone, login by fingerprint", zeroTime)
	_, olderSession, _ := s.AddKeySession(ctx, older.ID, "phone", end)
	if _, err := s.db.ExecContext(ctx, `UPDATE tokens SET deleted_at = ? WHERE id = ?`, s.nowText(), older.ID); err != nil {
		t.Fatal(err)
	}
	ended("a session whose key an older binary revoked", olderSession, true)

	// A password set by someone else ends the sessions; the keys live.
	_, plain, _ := s.AddToken(ctx, anna.ID, TokenSession, "Firefox", end)
	if err := s.RevokeSessions(ctx, anna.ID); err != nil {
		t.Fatal(err)
	}
	ended("a session a new password ended", plain, true)
	ended("a key beside it", key, false)
}

// revokeVector is a secret and its revoke code, the vector the app's
// RevokeCodeTest.kt shares, so that both work the code out alike (AP
// MobileApp v1.6, D112-A).
var revokeVector = struct{ secret, code string }{
	"00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff",
	"1a797b292ec9355f7ac4484c667557d8471cb11c4e47c4321a1328d4eb33cebb",
}

// A token's revoke code is the hex of SHA-256 of "massimo revoke v1" and
// the secret, and differs from the hash the store keeps; the store
// keeps the hash of the code for a token whose secret it made, a
// session, a key and a key's session, and none for one made from a hash
// it was handed; a touch fills a row without one and keeps one written.
func TestRevokeCode(t *testing.T) {
	t.Parallel()
	if got := RevokeCode(revokeVector.secret); got != revokeVector.code {
		t.Fatalf("RevokeCode = %s, want %s", got, revokeVector.code)
	}
	if RevokeCode(revokeVector.secret) == HashToken(revokeVector.secret) {
		t.Fatal("the code is the hash the store keeps")
	}
	s := open(t, t.TempDir())
	ctx := context.Background()
	anna, _ := s.AddUser(ctx, User{Name: "anna", Role: RoleOperator}, "pw", 0)
	revokeHash := func(id int64) string {
		t.Helper()
		var h sql.NullString
		if err := s.db.QueryRowContext(ctx, `SELECT revoke_hash FROM tokens WHERE id = ?`, id).Scan(&h); err != nil {
			t.Fatal(err)
		}
		return h.String
	}
	sess, sessSecret, _ := s.AddToken(ctx, anna.ID, TokenSession, "Firefox", zeroTime)
	key, keySecret, _ := s.AddToken(ctx, anna.ID, TokenAPI, "phone, login by fingerprint", zeroTime)
	keySess, keySessSecret, err := s.AddKeySession(ctx, key.ID, "phone", zeroTime)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		what   string
		tok    Token
		secret string
	}{{"a session", sess, sessSecret}, {"a key", key, keySecret}, {"a key's session", keySess, keySessSecret}} {
		if !c.tok.RevokeKept || revokeHash(c.tok.ID) != HashToken(RevokeCode(c.secret)) {
			t.Errorf("%s: kept %v, hash %q", c.what, c.tok.RevokeKept, revokeHash(c.tok.ID))
		}
	}
	handed, err := s.AddTokenHash(ctx, anna.ID, TokenAPI, "handed", HashToken("handed"), zeroTime)
	if err != nil || handed.RevokeKept || revokeHash(handed.ID) != "" {
		t.Fatalf("a token of a hash handed: %+v %v", handed, err)
	}
	if err := s.TouchToken(ctx, handed.ID, "10.0.0.5", zeroTime, HashToken(RevokeCode("handed"))); err != nil {
		t.Fatal(err)
	}
	if got, _, _ := s.Token(ctx, handed.ID); !got.RevokeKept || revokeHash(handed.ID) != HashToken(RevokeCode("handed")) {
		t.Fatalf("filled at a touch: %+v", got)
	}
	if err := s.TouchToken(ctx, handed.ID, "10.0.0.5", zeroTime, HashToken("another")); err != nil {
		t.Fatal(err)
	}
	if revokeHash(handed.ID) != HashToken(RevokeCode("handed")) {
		t.Fatal("a touch replaced a revoke hash written")
	}
}

// RevokeByCode revokes the live token of a code as RevokeToken does, a
// key's sessions with it, even past its end; it matches nothing for a
// token that ended by a revoke, an unknown code, or the token whose hash
// it is told to keep, the settings file's key.
func TestRevokeByCode(t *testing.T) {
	t.Parallel()
	clock := time.Date(2026, 11, 28, 12, 0, 0, 0, time.UTC)
	s, err := Open(copied(t), Options{Logger: quiet(), Now: func() time.Time { return clock }})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx := context.Background()
	anna, _ := s.AddUser(ctx, User{Name: "anna", Role: RoleOperator}, "pw", 0)
	sess, sessSecret, _ := s.AddToken(ctx, anna.ID, TokenSession, "Firefox", clock.Add(time.Hour))
	got, ok, err := s.RevokeByCode(ctx, RevokeCode(sessSecret), "")
	if err != nil || !ok || got.ID != sess.ID || got.UserID != anna.ID || !got.DeletedAt.IsZero() {
		t.Fatalf("a session by its code: %+v %v %v", got, ok, err)
	}
	if _, ok, _ := s.TokenByHash(ctx, HashToken(sessSecret)); ok {
		t.Fatal("the session lives on after its code")
	}
	if _, ok, err := s.RevokeByCode(ctx, RevokeCode(sessSecret), ""); ok || err != nil {
		t.Fatalf("a code taken already: %v %v", ok, err)
	}
	if _, ok, err := s.RevokeByCode(ctx, RevokeCode("nobody's"), ""); ok || err != nil {
		t.Fatalf("an unknown code: %v %v", ok, err)
	}

	key, keySecret, _ := s.AddToken(ctx, anna.ID, TokenAPI, "phone, login by fingerprint", zeroTime)
	keySess, _, _ := s.AddKeySession(ctx, key.ID, "phone", clock.Add(time.Hour))
	sub, _, err := s.AddPushSubscription(ctx, PushSubscription{UserID: anna.ID, TokenID: keySess.ID, Endpoint: "https://push.example.net/a", P256dh: "k", Auth: "a"})
	if err != nil {
		t.Fatal(err)
	}
	if got, ok, err := s.RevokeByCode(ctx, RevokeCode(keySecret), ""); err != nil || !ok || got.ID != key.ID {
		t.Fatalf("a key by its code: %+v %v %v", got, ok, err)
	}
	if got, _, _ := s.Token(ctx, keySess.ID); got.DeletedAt.IsZero() {
		t.Fatal("the key's session lives on after the key's code")
	}
	if _, ok, _ := s.PushSubscription(ctx, sub.ID); ok {
		t.Fatal("the key's session's subscription stayed")
	}

	// Past its end it is still revoked, its sessions with it.
	old, oldSecret, _ := s.AddToken(ctx, anna.ID, TokenAPI, "old", zeroTime)
	oldSess, _, _ := s.AddKeySession(ctx, old.ID, "phone", clock.Add(time.Hour))
	if _, err := s.db.ExecContext(ctx, `UPDATE tokens SET expires_at = ? WHERE id = ?`, formatTime(clock.Add(-time.Minute)), old.ID); err != nil {
		t.Fatal(err)
	}
	if _, ok, err := s.RevokeByCode(ctx, RevokeCode(oldSecret), ""); !ok || err != nil {
		t.Fatalf("a key past its end: %v %v", ok, err)
	}
	if got, _, _ := s.Token(ctx, oldSess.ID); got.DeletedAt.IsZero() {
		t.Fatal("the session of a key past its end lives on")
	}

	// The settings file's key, named by its hash, is left live.
	file, fileSecret, _ := s.AddToken(ctx, anna.ID, TokenAPI, "named in the file later", zeroTime)
	if _, ok, err := s.RevokeByCode(ctx, RevokeCode(fileSecret), HashToken(fileSecret)); ok || err != nil {
		t.Fatalf("the kept key: %v %v", ok, err)
	}
	if got, _, _ := s.Token(ctx, file.ID); !got.DeletedAt.IsZero() {
		t.Fatal("the kept key was revoked")
	}
}

// A code that matches nothing, which anyone may post, takes no write
// transaction: on a store that takes no writes it is no match, not the
// store's refusal, and only a match reaches the write.
func TestRevokeByCodeReadsFirst(t *testing.T) {
	t.Parallel()
	s := open(t, t.TempDir())
	ctx := context.Background()
	anna, _ := s.AddUser(ctx, User{Name: "anna", Role: RoleOperator}, "", 0)
	_, secret, _ := s.AddToken(ctx, anna.ID, TokenAPI, "phone", zeroTime)
	s.readOnly = true
	defer func() { s.readOnly = false }()
	if _, ok, err := s.RevokeByCode(ctx, RevokeCode("nobody's"), ""); ok || err != nil {
		t.Fatalf("a code that matches nothing: %v %v", ok, err)
	}
	if _, ok, err := s.RevokeByCode(ctx, RevokeCode(secret), ""); ok || err == nil {
		t.Fatalf("a match reaches the write: %v %v", ok, err)
	}
}
