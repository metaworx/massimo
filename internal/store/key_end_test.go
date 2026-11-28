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

// An API key's end date (AP KeyLife, Block 4; D5-A, D6-A): made with one
// by AddKey, a date not after now refused and nothing made; at its date
// the key ends, refused by TokenByHash, ended for TokenEnded, listed
// Ended expired, and its session with it, while its heir lives on, an
// end being no revoke; moved later by SetTokenLife, the key and its
// session live again; a date that has passed is refused and changes
// nothing, the zero time removes the end, and an integration's key takes
// one too. A session, a revoked key and none take none.
func TestKeyEnd(t *testing.T) {
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
	count := func() int {
		t.Helper()
		var n int
		if err := s.db.QueryRowContext(ctx, `SELECT count(*) FROM tokens WHERE user_id = ?`, anna.ID).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}

	// A date not after now makes no key; a sub-millisecond past it, as the
	// store keeps times, neither.
	before := count()
	for _, c := range []struct {
		what string
		end  time.Time
	}{{"now", t0}, {"an hour ago", t0.Add(-time.Hour)}, {"within now's millisecond", t0.Add(400 * time.Microsecond)}} {
		if _, _, err := s.AddKey(ctx, anna.ID, KeyOptions{Label: "ends " + c.what, Expires: c.end}); !errors.Is(err, ErrEndPassed) {
			t.Errorf("a key ending %s: %v, want ErrEndPassed", c.what, err)
		}
	}
	if n := count(); n != before {
		t.Fatalf("a key with an end that has passed was made: %d tokens, %d before", n, before)
	}

	end := t0.Add(10 * day)
	phone, phoneSecret, err := s.AddKey(ctx, anna.ID, KeyOptions{Label: "phone", Expires: end})
	if err != nil || !phone.ExpiresAt.Equal(end) {
		t.Fatalf("a key made with an end date: %+v %v", phone, err)
	}
	ofPhone, ofPhoneSecret, err := s.AddKeySession(ctx, phone.ID, "phone", t0.Add(30*day))
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.AddPushSubscription(ctx, PushSubscription{UserID: anna.ID, TokenID: ofPhone.ID, Endpoint: "https://push.example.net/a", P256dh: "k", Auth: "a"}); err != nil {
		t.Fatal(err)
	}
	heir, heirSecret, err := s.AddKey(ctx, anna.ID, KeyOptions{Label: "made by the phone's key", MadeBy: phone.ID})
	if err != nil || heir.MadeBy != phone.ID || !heir.ExpiresAt.IsZero() {
		t.Fatalf("an heir without an end date: %+v %v", heir, err)
	}
	alive := func(what, secret string, want bool) {
		t.Helper()
		if _, ok, err := s.TokenByHash(ctx, HashToken(secret)); err != nil || ok != want {
			t.Errorf("%s: alive %v %v, want %v", what, ok, err, want)
		}
		if got, err := s.TokenEnded(ctx, HashToken(secret), clock); err != nil || got == want {
			t.Errorf("%s: ended %v %v, want %v", what, got, err, !want)
		}
	}
	listed := func() map[int64]Token {
		t.Helper()
		list, err := s.Tokens(ctx, anna.ID)
		if err != nil {
			t.Fatal(err)
		}
		m := map[int64]Token{}
		for _, tk := range list {
			m[tk.ID] = tk
		}
		return m
	}

	// Before its date it lives, and says when it ends.
	clock = t0.Add(10*day - time.Millisecond)
	alive("the key a millisecond before its end", phoneSecret, true)
	alive("its session", ofPhoneSecret, true)
	if tk := listed()[phone.ID]; !tk.ExpiresAt.Equal(end) || !tk.EndsAt.Equal(end) || tk.Ended != "" {
		t.Errorf("the key before its end, listed: %+v", tk)
	}

	// At its date it ends, its session with it; its heir lives on.
	clock = end
	alive("the key at its end", phoneSecret, false)
	alive("its session", ofPhoneSecret, false)
	alive("its heir, made without an end", heirSecret, true)
	m := listed()
	if tk, ok := m[phone.ID]; !ok || tk.Ended != EndedExpired || !tk.EndsAt.Equal(end) {
		t.Errorf("the key past its end, listed to be revoked: %v %+v", ok, tk)
	}
	if _, ok := m[ofPhone.ID]; ok {
		t.Error("the session of a key past its end is listed")
	}
	if tk, ok := m[heir.ID]; !ok || tk.Ended != "" {
		t.Errorf("the heir of a key past its end: %v %+v", ok, tk)
	}
	if subs, _ := s.PushSubscriptions(ctx, 0); len(subs) != 0 {
		t.Errorf("the session of a key past its end is sent to: %+v", subs)
	}
	if _, _, err := s.AddKeySession(ctx, phone.ID, "phone", clock.Add(30*day)); !errors.Is(err, ErrNoToken) {
		t.Errorf("a key past its end made a session: %v", err)
	}

	// Moved later, the key and its session live again (D6-A).
	later := t0.Add(20 * day)
	got, err := s.SetTokenLife(ctx, phone.ID, TokenLife{Expires: &later})
	if err != nil || !got.ExpiresAt.Equal(later) || !got.EndsAt.Equal(later) || got.Ended != "" || got.Version != phone.Version+1 {
		t.Fatalf("the end moved later: %+v %v", got, err)
	}
	alive("the key, its end moved later", phoneSecret, true)
	alive("its session", ofPhoneSecret, true)
	if subs, _ := s.PushSubscriptions(ctx, 0); len(subs) != 1 {
		t.Errorf("the session alive again is not sent to: %+v", subs)
	}

	// A date that has passed is refused and changes nothing; no field keeps
	// the end; with the exclusion in one call, both change.
	for _, c := range []struct {
		what string
		end  time.Time
	}{{"now", clock}, {"before now", t0.Add(5 * day)}} {
		passed := c.end
		if _, err := s.SetTokenLife(ctx, phone.ID, TokenLife{Expires: &passed}); !errors.Is(err, ErrEndPassed) {
			t.Errorf("the end moved to %s: %v, want ErrEndPassed", c.what, err)
		}
	}
	if tk, _, _ := s.Token(ctx, phone.ID); !tk.ExpiresAt.Equal(later) || tk.Version != phone.Version+1 {
		t.Errorf("a refused end changed the key: %+v", tk)
	}
	if got, err := s.SetTokenLife(ctx, phone.ID, TokenLife{}); err != nil || !got.ExpiresAt.Equal(later) {
		t.Errorf("no field keeps the end: %+v %v", got, err)
	}
	yes, sooner := true, t0.Add(15*day)
	if got, err := s.SetTokenLife(ctx, phone.ID, TokenLife{IdleExempt: &yes, Expires: &sooner}); err != nil || !got.IdleExempt || !got.ExpiresAt.Equal(sooner) {
		t.Errorf("the exclusion and the end in one call: %+v %v", got, err)
	}

	// The zero time removes the end: past every date, the key lives.
	var none time.Time
	if got, err := s.SetTokenLife(ctx, phone.ID, TokenLife{Expires: &none}); err != nil || !got.ExpiresAt.IsZero() || !got.EndsAt.IsZero() || got.Ended != "" {
		t.Errorf("the end removed: %+v %v", got, err)
	}
	clock = t0.Add(400 * day)
	alive("the key without an end, a year on", phoneSecret, true)

	// An integration's key takes one too; a session, a revoked key and none
	// take none.
	i := integrationOnly(t, s)
	integ, integSecret, err := s.addToken(ctx, i.UserID, TokenAPI, "the integration's", zeroTime, true)
	if err != nil {
		t.Fatal(err)
	}
	integEnd := clock.Add(day)
	if got, err := s.SetTokenLife(ctx, integ.ID, TokenLife{Expires: &integEnd}); err != nil || !got.ExpiresAt.Equal(integEnd) {
		t.Fatalf("an integration's key given an end date: %+v %v", got, err)
	}
	clock = integEnd
	alive("the integration's key at its end", integSecret, false)
	sign, _, _ := s.AddToken(ctx, anna.ID, TokenSession, "Firefox", clock.Add(30*day))
	far := clock.Add(day)
	if _, err := s.SetTokenLife(ctx, sign.ID, TokenLife{Expires: &far}); !errors.Is(err, ErrNoToken) {
		t.Errorf("a session given an end date: %v", err)
	}
	if _, err := s.RevokeToken(ctx, heir.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.SetTokenLife(ctx, heir.ID, TokenLife{Expires: &far}); !errors.Is(err, ErrNoToken) {
		t.Errorf("a revoked key given an end date: %v", err)
	}
	if _, err := s.SetTokenLife(ctx, 9999, TokenLife{Expires: &far}); !errors.Is(err, ErrNoToken) {
		t.Errorf("no key given an end date: %v", err)
	}
}

// The settings file's key never ends by a date (AP KeyLife, Block 4,
// D3-A): a key made with an end date and then put into the file loses it
// at the bootstrap and lives past it; a row of the file's key that holds
// an end date all the same, as an older binary may leave it, is passed
// over by the rule, for the key and for a session made from it, and
// listed as living; SetTokenLife refuses the file's key whatever it
// changes.
func TestSettingsKeyNeverEnds(t *testing.T) {
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
	owner, _ := s.AddUser(ctx, User{Name: "owner", Role: RoleOwner}, "", 0)
	key, secret, err := s.AddKey(ctx, owner.ID, KeyOptions{Label: "laptop", Expires: t0.Add(2 * day)})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Bootstrap(ctx, secret); err != nil {
		t.Fatal(err)
	}
	if got, _, _ := s.Token(ctx, key.ID); !got.ExpiresAt.IsZero() {
		t.Errorf("the bootstrap kept the end date of the key it took in: %v", got.ExpiresAt)
	}
	alive := func(what, secret string) {
		t.Helper()
		if _, ok, err := s.TokenByHash(ctx, HashToken(secret)); err != nil || !ok {
			t.Errorf("%s: alive %v %v, want true", what, ok, err)
		}
		if got, err := s.TokenEnded(ctx, HashToken(secret), clock); err != nil || got {
			t.Errorf("%s: ended %v %v, want false", what, got, err)
		}
	}
	clock = t0.Add(3 * day)
	alive("the settings file's key past the end date it was made with", secret)

	// Its row holding an end date all the same.
	if _, err := s.db.ExecContext(ctx, `UPDATE tokens SET expires_at = ? WHERE id = ?`, formatTime(t0.Add(day)), key.ID); err != nil {
		t.Fatal(err)
	}
	alive("the settings file's key past an end date its row holds", secret)
	session, sessionSecret, err := s.AddKeySession(ctx, key.ID, "phone", clock.Add(30*day))
	if err != nil {
		t.Errorf("a session of the settings file's key past an end date its row holds: %v", err)
	} else {
		alive("a session of the settings file's key past an end date its row holds", sessionSecret)
		if _, ok, err := s.TokenAlive(ctx, session.ID); err != nil || !ok {
			t.Errorf("TokenAlive of that session: %v %v", ok, err)
		}
	}
	list, err := s.Tokens(ctx, owner.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, tk := range list {
		if tk.ID == key.ID && (!tk.SettingsKey || tk.Ended != "" || !tk.EndsAt.IsZero()) {
			t.Errorf("the settings file's key listed: settings key %v, ended %q, ends at %v", tk.SettingsKey, tk.Ended, tk.EndsAt)
		}
	}

	// SetTokenLife refuses it, and changes nothing.
	later, yes := clock.Add(day), true
	var none time.Time
	for _, c := range []struct {
		what string
		life TokenLife
	}{{"an end date", TokenLife{Expires: &later}}, {"no end", TokenLife{Expires: &none}}, {"kept from the idle limit", TokenLife{IdleExempt: &yes}}} {
		if _, err := s.SetTokenLife(ctx, key.ID, c.life); !errors.Is(err, ErrSettingsKey) {
			t.Errorf("the settings file's key given %s: %v, want ErrSettingsKey", c.what, err)
		}
	}
	if got, _, _ := s.Token(ctx, key.ID); got.IdleExempt || !got.ExpiresAt.Equal(t0.Add(day)) {
		t.Errorf("a refused change of the settings file's key changed it: %+v", got)
	}
}

// A key past its end date is no revoked key (AP KeyLife, D6-A): its code
// still revokes it, the keys it made with it, of another user too, and an
// heir's session, and RevokeByCode names the heirs; moved later after
// that, the end brings back no key a code revoked.
func TestEndedKeyRevokedByCode(t *testing.T) {
	t.Parallel()
	t0 := time.Date(2026, 11, 28, 12, 0, 0, 0, time.UTC)
	clock := t0
	s, err := Open(copied(t), Options{Logger: quiet(), Now: func() time.Time { return clock }})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx := context.Background()
	day := 24 * time.Hour
	anna, _ := s.AddUser(ctx, User{Name: "anna", Role: RoleManager}, "", 0)
	bob, _ := s.AddUser(ctx, User{Name: "bob", Role: RoleOperator}, "", 0)
	key, keySecret, err := s.AddKey(ctx, anna.ID, KeyOptions{Label: "a week's key", Expires: t0.Add(7 * day)})
	if err != nil {
		t.Fatal(err)
	}
	heir, heirSecret, err := s.AddKey(ctx, bob.ID, KeyOptions{Label: "heir, without an end", MadeBy: key.ID})
	if err != nil {
		t.Fatal(err)
	}
	_, heirSessSecret, err := s.AddKeySession(ctx, heir.ID, "tablet", t0.Add(30*day))
	if err != nil {
		t.Fatal(err)
	}
	clock = t0.Add(8 * day)
	if _, ok, _ := s.TokenByHash(ctx, HashToken(keySecret)); ok {
		t.Fatal("a key past its end date let in")
	}
	if _, ok, _ := s.TokenByHash(ctx, HashToken(heirSecret)); !ok {
		t.Fatal("the heir of a key past its end date ended with it")
	}
	got, heirs, ok, err := s.RevokeByCode(ctx, RevokeCode(keySecret))
	if err != nil || !ok || got.ID != key.ID {
		t.Fatalf("the code of a key past its end date: %+v %v %v", got, ok, err)
	}
	if fmt.Sprint(heirs) != fmt.Sprint([]int64{heir.ID}) {
		t.Errorf("the heirs: %v, want [%d]", heirs, heir.ID)
	}
	if now, _, _ := s.Token(ctx, key.ID); now.DeletedAt.IsZero() {
		t.Error("the key past its end date is not revoked by its code")
	}
	for what, secret := range map[string]string{"the heir": heirSecret, "the heir's session": heirSessSecret} {
		if _, ok, _ := s.TokenByHash(ctx, HashToken(secret)); ok {
			t.Errorf("%s lives on after the code of a key past its end date", what)
		}
	}
	later := clock.Add(day)
	if _, err := s.SetTokenLife(ctx, key.ID, TokenLife{Expires: &later}); !errors.Is(err, ErrNoToken) {
		t.Errorf("the end of a key its code revoked moved: %v, want ErrNoToken", err)
	}
}
