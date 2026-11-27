// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package store

import (
	"context"
	"errors"
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
	if err := s.TouchToken(ctx, phone.ID, "192.0.2.10", zeroTime); err != nil {
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
	if err := s.TouchToken(ctx, integ.ID, "", zeroTime); err != nil {
		t.Fatal(err)
	}
	clock = clock.Add(2 * day) // the integration's key used two days ago, under the limit
	if err := s.TouchToken(ctx, integ.ID, "", zeroTime); err != nil {
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
