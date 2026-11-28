// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package store

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"
)

// A key's new secret (AP KeyLife, Block 5; MobileApp.D76-A): RekeyToken
// keeps the key's row, its id, label, end date, exclusion, maker, last use
// and making, and its heirs; the new secret lets in, the old one is an
// ended token (TokenEnded), kept in a revoked row of its own, an API key
// of the key's user and label, made and revoked now, listed nowhere; the
// key's session is revoked with what it subscribed to push, but for keep,
// the caller's, which stays; what the key itself subscribed goes; a
// revoke of the key still takes its heirs. A session, a revoked key, a
// key past its end date or ended unused, none and an integration's key
// take none, and nothing changes.
func TestRekey(t *testing.T) {
	t.Parallel()
	t0 := time.Date(2026, 11, 27, 12, 0, 0, 0, time.UTC)
	clock := t0
	s, err := Open(copied(t), Options{Logger: quiet(), Now: func() time.Time { return clock }})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx := context.Background()
	day := 24 * time.Hour
	anna, _ := s.AddUser(ctx, User{Name: "anna", Role: RoleOperator}, "pw", 0)
	maker, _, err := s.AddKey(ctx, anna.ID, KeyOptions{Label: "laptop"})
	if err != nil {
		t.Fatal(err)
	}
	end := t0.Add(40 * day)
	phone, oldSecret, err := s.AddKey(ctx, anna.ID, KeyOptions{Label: "SM-X, login by fingerprint", MadeBy: maker.ID, IdleExempt: true, Expires: end})
	if err != nil {
		t.Fatal(err)
	}
	heir, heirSecret, err := s.AddKey(ctx, anna.ID, KeyOptions{Label: "made by the phone's key", MadeBy: phone.ID})
	if err != nil {
		t.Fatal(err)
	}
	clock = t0.Add(day)
	if err := s.TouchToken(ctx, phone.ID, "192.0.2.10", zeroTime, ""); err != nil {
		t.Fatal(err)
	}
	session, sessionSecret, err := s.AddKeySession(ctx, phone.ID, "phone", clock.Add(30*day))
	if err != nil {
		t.Fatal(err)
	}
	subscribe := func(token int64, endpoint string) {
		t.Helper()
		if _, _, err := s.AddPushSubscription(ctx, PushSubscription{UserID: anna.ID, TokenID: token, Endpoint: endpoint, P256dh: "k", Auth: "a"}); err != nil {
			t.Fatal(err)
		}
	}
	subscribe(session.ID, "https://push.example.net/session")
	subscribe(phone.ID, "https://push.example.net/key")
	before, _, _ := s.Token(ctx, phone.ID)
	alive := func(what, secret string, want bool) {
		t.Helper()
		if _, ok, err := s.TokenByHash(ctx, HashToken(secret)); err != nil || ok != want {
			t.Errorf("%s: alive %v %v, want %v", what, ok, err, want)
		}
		if got, err := s.TokenEnded(ctx, HashToken(secret), clock); err != nil || got == want {
			t.Errorf("%s: ended %v %v, want %v", what, got, err, !want)
		}
	}
	endpoints := func() map[string]bool {
		t.Helper()
		subs, err := s.PushSubscriptions(ctx, anna.ID)
		if err != nil {
			t.Fatal(err)
		}
		m := map[string]bool{}
		for _, p := range subs {
			m[p.Endpoint] = true
		}
		return m
	}
	count := func() int {
		t.Helper()
		var n int
		if err := s.db.QueryRowContext(ctx, `SELECT count(*) FROM tokens WHERE user_id = ?`, anna.ID).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}

	// The new secret: the same row, the old secret ended, the key's
	// session revoked with what it and the key subscribed to push.
	clock = t0.Add(2 * day)
	n := count()
	got, newSecret, err := s.RekeyToken(ctx, phone.ID, 0)
	if err != nil {
		t.Fatal(err)
	}
	if newSecret == "" || newSecret == oldSecret || got.ID != phone.ID || got.UUID != phone.UUID || got.Label != phone.Label || !got.ExpiresAt.Equal(end) ||
		!got.IdleExempt || got.MadeBy != maker.ID || got.Kind != TokenAPI || !got.CreatedAt.Equal(phone.CreatedAt) ||
		!got.LastUsedAt.Equal(before.LastUsedAt) || got.LastUsedFrom != "192.0.2.10" || got.Version != before.Version+1 || got.Ended != "" || !got.EndsAt.Equal(end) {
		t.Fatalf("the key with its new secret: %+v, before %+v", got, before)
	}
	alive("the new secret", newSecret, true)
	alive("the old secret", oldSecret, false)
	alive("the key's session", sessionSecret, false)
	alive("its heir", heirSecret, true)
	if m := endpoints(); m["https://push.example.net/session"] || m["https://push.example.net/key"] {
		t.Errorf("the old secret's holder is still sent to: %v", m)
	}
	if c := count(); c != n+1 {
		t.Errorf("tokens: %d, want %d, one row for the old secret", c, n+1)
	}
	var old Token
	if err := func() error {
		var id int64
		if err := s.db.QueryRowContext(ctx, `SELECT id FROM tokens WHERE hash = ?`, HashToken(oldSecret)).Scan(&id); err != nil {
			return err
		}
		old, _, err = s.Token(ctx, id)
		return err
	}(); err != nil {
		t.Fatalf("the old secret's row: %v", err)
	}
	if old.ID == phone.ID || old.Kind != TokenAPI || old.UserID != anna.ID || old.Label != phone.Label || !old.CreatedAt.Equal(clock) || !old.DeletedAt.Equal(clock) ||
		old.MadeBy != 0 || old.ParentID != 0 || !old.ExpiresAt.IsZero() || old.IdleExempt {
		t.Errorf("the old secret's row: %+v", old)
	}
	list, err := s.Tokens(ctx, anna.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, tk := range list {
		if tk.ID == old.ID || tk.ID == session.ID {
			t.Errorf("listed: %+v", tk)
		}
	}

	// The caller's own session, made from the key, stays, and its push
	// subscription with it; a second new secret keeps the first one's
	// old secret ended too.
	mine, mineSecret, err := s.AddKeySession(ctx, phone.ID, "phone", clock.Add(30*day))
	if err != nil {
		t.Fatal(err)
	}
	subscribe(mine.ID, "https://push.example.net/mine")
	got2, newer, err := s.RekeyToken(ctx, phone.ID, mine.ID)
	if err != nil || got2.ID != phone.ID {
		t.Fatalf("a second new secret: %+v %v", got2, err)
	}
	alive("the caller's session", mineSecret, true)
	alive("the newer secret", newer, true)
	alive("the first new secret", newSecret, false)
	alive("the old secret", oldSecret, false)
	if m := endpoints(); !m["https://push.example.net/mine"] {
		t.Errorf("the caller's session is sent to no more: %v", m)
	}

	// Its heirs stay its heirs: a revoke takes them.
	if heirs, err := s.RevokeToken(ctx, phone.ID); err != nil || len(heirs) != 1 || heirs[0] != heir.ID {
		t.Fatalf("the key's revoke after its new secrets: heirs %v %v, want [%d]", heirs, err, heir.ID)
	}
	alive("the newer secret, revoked", newer, false)

	// Refused, nothing changes: a revoked key, a session, none, a key past
	// its end date or ended unused, and an integration's key.
	other, otherSecret, err := s.AddKey(ctx, anna.ID, KeyOptions{Label: "Loxone", IdleExempt: true, Expires: clock.Add(day)})
	if err != nil {
		t.Fatal(err)
	}
	idle, idleSecret, err := s.AddKey(ctx, anna.ID, KeyOptions{Label: "unused"})
	if err != nil {
		t.Fatal(err)
	}
	sign, _, _ := s.AddToken(ctx, anna.ID, TokenSession, "Firefox", clock.Add(60*day))
	i := integrationOnly(t, s)
	integ, integSecret, err := s.addToken(ctx, i.UserID, TokenAPI, "the integration's", zeroTime, true)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SetSetting(ctx, KeyIdleDaysKey, 3); err != nil {
		t.Fatal(err)
	}
	if err := s.TouchToken(ctx, integ.ID, "", zeroTime, ""); err != nil {
		t.Fatal(err)
	}
	clock = clock.Add(2 * day) // the integration's key used two days ago, under the limit
	if err := s.TouchToken(ctx, integ.ID, "", zeroTime, ""); err != nil {
		t.Fatal(err)
	}
	clock = clock.Add(2 * day) // the Loxone key past its end date, the unused one past the limit
	n = count()
	for _, c := range []struct {
		what string
		id   int64
		want error
	}{
		{"a revoked key", phone.ID, ErrNoToken},
		{"a session", sign.ID, ErrNoToken},
		{"none", 9999, ErrNoToken},
		{"a key past its end date", other.ID, ErrNoToken},
		{"a key ended unused", idle.ID, ErrNoToken},
		{"an integration's key", integ.ID, ErrIntegrationUser},
	} {
		if _, secret, err := s.RekeyToken(ctx, c.id, 0); !errors.Is(err, c.want) || secret != "" {
			t.Errorf("%s: %q %v, want %v", c.what, secret, err, c.want)
		}
	}
	if c := count(); c != n {
		t.Errorf("a refused new secret made a row: %d tokens, %d before", c, n)
	}
	if err := s.DeleteSetting(ctx, KeyIdleDaysKey); err != nil {
		t.Fatal(err)
	}
	alive("the key ended unused, the limit off again: its secret unchanged", idleSecret, true)
	alive("the integration's key: its secret unchanged", integSecret, true)
	later := clock.Add(day)
	if _, err := s.SetTokenLife(ctx, other.ID, TokenLife{Expires: &later}); err != nil {
		t.Fatal(err)
	}
	alive("the key past its end, moved later: its secret unchanged", otherSecret, true)
}

// The settings file's key takes no new secret in the store either (AP
// KeyLife, Block 5; D3-A): known by the hash Bootstrap last applied,
// RekeyToken refuses it with ErrSettingsKey, as SetTokenLife does, and
// nothing changes: its secret still lets in, its session stays, and no
// row of an old secret is written. Once the file names another token,
// the key it named before is revoked by the bootstrap, and takes none as
// any revoked key.
func TestRekeySettingsKey(t *testing.T) {
	t.Parallel()
	s, err := Open(copied(t), Options{Logger: quiet()})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx := context.Background()
	owner, _ := s.AddUser(ctx, User{Name: "owner", Role: RoleOwner}, "", 0)
	key, secret, err := s.AddKey(ctx, owner.ID, KeyOptions{Label: BootstrapLabel})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Bootstrap(ctx, secret); err != nil {
		t.Fatal(err)
	}
	_, sessionSecret, err := s.AddKeySession(ctx, key.ID, "phone", time.Now().Add(24*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	count := func() int {
		t.Helper()
		var n int
		if err := s.db.QueryRowContext(ctx, `SELECT count(*) FROM tokens WHERE user_id = ?`, owner.ID).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	n := count()
	if _, got, err := s.RekeyToken(ctx, key.ID, 0); !errors.Is(err, ErrSettingsKey) || got != "" {
		t.Fatalf("the settings file's key given a new secret: %q %v, want ErrSettingsKey", got, err)
	}
	for _, c := range []struct{ what, secret string }{{"the file's key", secret}, {"its session", sessionSecret}} {
		if _, ok, err := s.TokenByHash(ctx, HashToken(c.secret)); err != nil || !ok {
			t.Errorf("%s, refused a new secret: alive %v %v", c.what, ok, err)
		}
	}
	if c := count(); c != n {
		t.Errorf("a refused new secret wrote a row: %d tokens, %d before", c, n)
	}
	if _, err := s.Bootstrap(ctx, "another placeholder token of the file"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.RekeyToken(ctx, key.ID, 0); !errors.Is(err, ErrNoToken) {
		t.Errorf("the file's former key, revoked by the bootstrap, given a new secret: %v, want ErrNoToken", err)
	}
}

// A request let in with a key's old secret just before a new secret, and
// writing just after it (AP KeyLife, Block 5): the session it would make
// from the key (AddKeySessionAs) and the key it would make with the key
// as its maker (KeyOptions.Maker) are refused as for a key revoked
// meanwhile, ErrNoToken, and nothing is made; the event stream's check of
// its caller (TokenAliveAs) lets it in no more. Let in with the new
// secret, each goes through. A key without its secret, as a caller could
// make one by its id alone, is let in by none of them.
func TestRekeyAdmittedBefore(t *testing.T) {
	t.Parallel()
	s, err := Open(copied(t), Options{Logger: quiet()})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx := context.Background()
	anna, _ := s.AddUser(ctx, User{Name: "anna", Role: RoleOperator}, "pw", 0)
	key, oldSecret, err := s.AddKey(ctx, anna.ID, KeyOptions{Label: "phone"})
	if err != nil {
		t.Fatal(err)
	}
	before, ok, err := s.TokenByHash(ctx, HashToken(oldSecret)) // the request's admission
	if err != nil || !ok {
		t.Fatalf("the key let in: %v %v", ok, err)
	}
	_, newSecret, err := s.RekeyToken(ctx, key.ID, 0)
	if err != nil {
		t.Fatal(err)
	}
	count := func() int {
		t.Helper()
		var n int
		if err := s.db.QueryRowContext(ctx, `SELECT count(*) FROM tokens WHERE user_id = ?`, anna.ID).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	n := count()
	day := time.Now().Add(24 * time.Hour)
	for _, c := range []struct {
		what string
		key  Token
	}{{"the key let in with its old secret", before}, {"the key by its id alone", Token{ID: key.ID}}} {
		if _, secret, err := s.AddKeySessionAs(ctx, c.key, "phone", day); !errors.Is(err, ErrNoToken) || secret != "" {
			t.Errorf("a session made from %s: %q %v, want ErrNoToken", c.what, secret, err)
		}
		if tok, _, err := s.AddKey(ctx, anna.ID, KeyOptions{Label: "heir", Maker: c.key}); !errors.Is(err, ErrNoToken) {
			t.Errorf("a key made by %s: %+v %v, want ErrNoToken", c.what, tok, err)
		}
		if _, ok, err := s.TokenAliveAs(ctx, c.key); err != nil || ok {
			t.Errorf("%s, alive as it: %v %v, want false", c.what, ok, err)
		}
	}
	if c := count(); c != n {
		t.Errorf("a refused write made a row: %d tokens, %d before", c, n)
	}

	after, ok, err := s.TokenByHash(ctx, HashToken(newSecret))
	if err != nil || !ok {
		t.Fatalf("the key let in with its new secret: %v %v", ok, err)
	}
	if got, ok, err := s.TokenAliveAs(ctx, after); err != nil || !ok || got.ID != key.ID {
		t.Errorf("the key let in with its new secret, alive as it: %+v %v %v", got, ok, err)
	}
	if session, _, err := s.AddKeySessionAs(ctx, after, "phone", day); err != nil || session.ParentID != key.ID {
		t.Errorf("a session made from the key let in with its new secret: %+v %v", session, err)
	}
	if heir, _, err := s.AddKey(ctx, anna.ID, KeyOptions{Label: "heir", Maker: after}); err != nil || heir.MadeBy != key.ID {
		t.Errorf("a key made by the key let in with its new secret: %+v %v", heir, err)
	}
}

// A key's new secret takes its revoke code with it (AP KeyLife, Block 5;
// MobileApp.D112-A): the key's row holds the new secret's code, so that
// code revokes the key and its heirs, and the revoked row of the old
// secret holds the old one, so that the old secret's code matches nothing
// and the key stays; a key whose row held no code, made before the store
// kept codes, gets the new secret's.
func TestRekeyRevokeCode(t *testing.T) {
	t.Parallel()
	s := open(t, t.TempDir())
	ctx := context.Background()
	anna, _ := s.AddUser(ctx, User{Name: "anna", Role: RoleManager}, "", 0)
	key, oldSecret, err := s.AddKey(ctx, anna.ID, KeyOptions{Label: "phone"})
	if err != nil {
		t.Fatal(err)
	}
	heir, heirSecret, err := s.AddKey(ctx, anna.ID, KeyOptions{Label: "made by the phone's key", MadeBy: key.ID})
	if err != nil {
		t.Fatal(err)
	}
	got, newSecret, err := s.RekeyToken(ctx, key.ID, 0)
	if err != nil || !got.RevokeKept {
		t.Fatalf("the new secret: %+v %v", got, err)
	}
	if _, heirs, ok, err := s.RevokeByCode(ctx, RevokeCode(oldSecret)); ok || err != nil || len(heirs) != 0 {
		t.Fatalf("the old secret's code: %v %v %v, want no match", heirs, ok, err)
	}
	if _, ok, _ := s.TokenByHash(ctx, HashToken(newSecret)); !ok {
		t.Fatal("the key ended at its old secret's code")
	}
	revoked, heirs, ok, err := s.RevokeByCode(ctx, RevokeCode(newSecret))
	if err != nil || !ok || revoked.ID != key.ID {
		t.Fatalf("the new secret's code: %+v %v %v", revoked, ok, err)
	}
	if fmt.Sprint(heirs) != fmt.Sprint([]int64{heir.ID}) {
		t.Errorf("the heirs: %v, want [%d]", heirs, heir.ID)
	}
	if _, ok, _ := s.TokenByHash(ctx, HashToken(heirSecret)); ok {
		t.Error("the heir lives on after its maker's new secret's code")
	}

	old, oldLegacy, _ := s.AddKey(ctx, anna.ID, KeyOptions{Label: "made before the codes"})
	if _, err := s.db.ExecContext(ctx, `UPDATE tokens SET revoke_hash = NULL WHERE id = ?`, old.ID); err != nil {
		t.Fatal(err)
	}
	got, newLegacy, err := s.RekeyToken(ctx, old.ID, 0)
	if err != nil || !got.RevokeKept {
		t.Fatalf("the new secret of a key without a code: %+v %v", got, err)
	}
	if _, _, ok, _ := s.RevokeByCode(ctx, RevokeCode(oldLegacy)); ok {
		t.Error("the old secret of a key without a code matched")
	}
	if _, _, ok, err := s.RevokeByCode(ctx, RevokeCode(newLegacy)); !ok || err != nil {
		t.Errorf("the new secret's code of a key that had none: %v %v", ok, err)
	}
}

// A session AddKeySessionAs makes keeps its revoke code, as one of
// AddKeySession does (AP KeyLife, Block 5; MobileApp.D112-A): its code
// ends it, and the key stays.
func TestKeySessionAsRevokeCode(t *testing.T) {
	t.Parallel()
	s := open(t, t.TempDir())
	ctx := context.Background()
	anna, _ := s.AddUser(ctx, User{Name: "anna", Role: RoleOperator}, "", 0)
	_, keySecret, _ := s.AddKey(ctx, anna.ID, KeyOptions{Label: "phone"})
	key, ok, err := s.TokenByHash(ctx, HashToken(keySecret))
	if err != nil || !ok {
		t.Fatalf("the key: %v %v", ok, err)
	}
	sess, sessSecret, err := s.AddKeySessionAs(ctx, key, "phone", time.Now().Add(time.Hour))
	if err != nil || !sess.RevokeKept {
		t.Fatalf("a session of the key as its bearer: %+v %v", sess, err)
	}
	if got, _, ok, err := s.RevokeByCode(ctx, RevokeCode(sessSecret)); err != nil || !ok || got.ID != sess.ID {
		t.Fatalf("the session's code: %+v %v %v", got, ok, err)
	}
	if _, ok, _ := s.TokenByHash(ctx, HashToken(keySecret)); !ok {
		t.Error("the key ended at its session's code")
	}
}
