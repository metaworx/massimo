// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"testing"
	"time"
)

// Migration 0102 is additive (AP KeyLife, Block 3): a key written as a
// binary for 101 writes one, without the column, is held to the idle
// limit, one kept from it reads so, binaries for 26, 100 and 101 open the
// file, and the down part takes the schema back to the one of 101, losing
// which keys were kept from the limit and only that.
func TestTokenIdleMigration(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	clock := fixedClock()
	at := formatTime(clock())
	dir := additiveMigration(t, 101, 102, "0102_token_idle", func(ctx context.Context, db *sql.DB) {
		exec := func(q string, args ...any) {
			t.Helper()
			if _, err := db.ExecContext(ctx, q, args...); err != nil {
				t.Fatal(err)
			}
		}
		exec(`INSERT INTO users (uuid, name, role_id, created_at, updated_at) SELECT ?, 'anna', id, ?, ? FROM roles WHERE key = ?`, newUUID(), at, at, RoleOperator)
		exec(`INSERT INTO tokens (uuid, user_id, kind, hash, label, created_at, origin) SELECT ?, id, 'api', ?, ?, ?, '' FROM users WHERE name = 'anna'`,
			newUUID(), HashToken("placeholder-old-binary"), "written as 101 writes it", at)
		exec(`INSERT INTO tokens (uuid, user_id, kind, hash, label, created_at, origin, idle_exempt) SELECT ?, id, 'api', ?, ?, ?, '', 1 FROM users WHERE name = 'anna'`,
			newUUID(), HashToken("placeholder-kept"), "kept from the limit", at)
	}, 26, 100)
	s, err := Open(dir, Options{Logger: quiet(), Now: clock})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if got, ok, err := s.TokenByHash(ctx, HashToken("placeholder-old-binary")); err != nil || !ok || got.IdleExempt {
		t.Fatalf("a key written as 101 writes it: %+v %v %v", got, ok, err)
	}
	if got, ok, err := s.TokenByHash(ctx, HashToken("placeholder-kept")); err != nil || !ok || !got.IdleExempt {
		t.Fatalf("a key kept from the limit: %+v %v %v", got, ok, err)
	}
}

// The idle limit (AP KeyLife, Block 3; D2-A, D3-A, D6-A, D7-A): off by
// default, nothing ends; on, an API key unused for its days, counted
// from its last use or its making, is refused by TokenByHash, has ended
// for TokenEnded, is listed Ended idle, and its session ends with it; a
// key kept from it and the settings file's key stay, an integration's
// key ends and takes no exclusion. The limit raised, off, or the key
// kept from it, the key lives again, its session with it.
func TestKeyIdle(t *testing.T) {
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
	anna, _ := s.AddUser(ctx, User{Name: "anna", Role: RoleOperator}, "pw", 0)
	phone, phoneSecret, _ := s.AddKey(ctx, anna.ID, KeyOptions{Label: "phone"})
	ofPhone, ofPhoneSecret, err := s.AddKeySession(ctx, phone.ID, "phone", t0.Add(30*day))
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.AddPushSubscription(ctx, PushSubscription{UserID: anna.ID, TokenID: ofPhone.ID, Endpoint: "https://push.example.net/a", P256dh: "k", Auth: "a"}); err != nil {
		t.Fatal(err)
	}
	never, neverSecret, _ := s.AddKey(ctx, anna.ID, KeyOptions{Label: "never used"})
	kept, keptSecret, err := s.AddKey(ctx, anna.ID, KeyOptions{Label: "kept", IdleExempt: true})
	if err != nil || !kept.IdleExempt {
		t.Fatalf("a key made kept from the limit: %+v %v", kept, err)
	}
	if _, err := s.Bootstrap(ctx, "placeholder-file-token"); err != nil {
		t.Fatal(err)
	}
	file, ok, _ := s.TokenByHash(ctx, HashToken("placeholder-file-token"))
	if !ok || file.UserID != owner.ID || !file.SettingsKey {
		t.Fatalf("the settings file's key: %+v %v", file, ok)
	}
	i := integrationOnly(t, s)
	integ, integSecret, err := s.addToken(ctx, i.UserID, TokenAPI, "the integration's", zeroTime, true)
	if err != nil {
		t.Fatal(err)
	}
	sign, signSecret, _ := s.AddToken(ctx, anna.ID, TokenSession, "Firefox", t0.Add(30*day))
	alive := func(what, secret string, want bool) {
		t.Helper()
		if _, ok, err := s.TokenByHash(ctx, HashToken(secret)); err != nil || ok != want {
			t.Errorf("%s: alive %v %v, want %v", what, ok, err, want)
		}
		if got, err := s.TokenEnded(ctx, HashToken(secret), clock); err != nil || got == want {
			t.Errorf("%s: ended %v %v, want %v", what, got, err, !want)
		}
	}
	listed := func(user int64) map[int64]Token {
		t.Helper()
		list, err := s.Tokens(ctx, user)
		if err != nil {
			t.Fatal(err)
		}
		m := map[int64]Token{}
		for _, tk := range list {
			m[tk.ID] = tk
		}
		return m
	}
	setLimit := func(days any) {
		t.Helper()
		if days == nil {
			if err := s.DeleteSetting(ctx, KeyIdleDaysKey); err != nil {
				t.Fatal(err)
			}
			return
		}
		if err := s.SetSetting(ctx, KeyIdleDaysKey, days); err != nil {
			t.Fatal(err)
		}
	}

	// Off by default: 20 days unused end no key.
	clock = t0.Add(20 * day)
	for _, c := range []struct{ what, secret string }{{"a key used", phoneSecret}, {"its session", ofPhoneSecret}, {"a key never used", neverSecret},
		{"a key kept", keptSecret}, {"the integration's", integSecret}, {"a sign-in's session", signSecret}} {
		alive(c.what+", the limit unset", c.secret, true)
	}
	if err := s.TouchToken(ctx, phone.ID, "192.0.2.10", zeroTime, ""); err != nil {
		t.Fatal(err)
	}
	for id, tk := range listed(anna.ID) {
		if !tk.IdleEndsAt.IsZero() || tk.Ended != "" {
			t.Errorf("token %d with the limit off: idle end %v, ended %q", id, tk.IdleEndsAt, tk.Ended)
		}
	}

	// On at 7 days, at day 20: the key used at day 20 lives until day 27;
	// the one never used ended at day 7, its making's; the kept one, the
	// settings file's and the sign-in's session stay; the integration's
	// key ends too.
	setLimit(7)
	alive("a key used at day 20", phoneSecret, true)
	alive("its session", ofPhoneSecret, true)
	alive("a key never used, made at day 0", neverSecret, false)
	alive("a key kept from the limit", keptSecret, true)
	alive("the settings file's key, never used", "placeholder-file-token", true)
	alive("the integration's key, never used", integSecret, false)
	alive("a sign-in's session", signSecret, true)
	m := listed(anna.ID)
	if tk := m[phone.ID]; !tk.IdleEndsAt.Equal(t0.Add(27*day)) || !tk.EndsAt.Equal(tk.IdleEndsAt) || tk.Ended != "" {
		t.Errorf("the key used: %+v", tk)
	}
	if tk, ok := m[never.ID]; !ok || !tk.IdleEndsAt.Equal(t0.Add(7*day)) || tk.Ended != EndedIdle {
		t.Errorf("the key never used, listed to be revoked: %v %+v", ok, tk)
	}
	if tk := m[kept.ID]; !tk.IdleExempt || !tk.IdleEndsAt.IsZero() || tk.Ended != "" {
		t.Errorf("the key kept: %+v", tk)
	}
	if tk := m[sign.ID]; !tk.IdleEndsAt.IsZero() || tk.Ended != "" {
		t.Errorf("a sign-in's session: %+v", tk)
	}
	if tk := listed(owner.ID)[file.ID]; !tk.SettingsKey || !tk.IdleEndsAt.IsZero() || tk.Ended != "" {
		t.Errorf("the settings file's key: %+v", tk)
	}
	if tk := listed(i.UserID)[integ.ID]; tk.Ended != EndedIdle {
		t.Errorf("the integration's key: %+v", tk)
	}

	// At day 27 the key used at day 20 ends, its session with it: refused,
	// not listed, not sent to, and no new one is made.
	clock = t0.Add(27 * day)
	alive("the key used at day 20, at day 27", phoneSecret, false)
	alive("its session", ofPhoneSecret, false)
	if _, ok := listed(anna.ID)[ofPhone.ID]; ok {
		t.Error("the session of a key ended unused is listed")
	}
	if subs, _ := s.PushSubscriptions(ctx, 0); len(subs) != 0 {
		t.Errorf("the session of a key ended unused is sent to: %+v", subs)
	}
	if _, _, err := s.AddKeySession(ctx, phone.ID, "phone", clock.Add(30*day)); !errors.Is(err, ErrNoToken) {
		t.Errorf("a key ended unused made a session: %v", err)
	}
	if _, ok, _ := s.TokenAlive(ctx, ofPhone.ID); ok {
		t.Error("TokenAlive gave the session of a key ended unused")
	}

	// The limit raised to 30 days: both keys live again, the session with
	// its key, its own end at day 30; turned off, every key lives.
	setLimit(30)
	alive("the key used at day 20, the limit raised", phoneSecret, true)
	alive("its session", ofPhoneSecret, true)
	alive("the key never used, made at day 0", neverSecret, true)
	if subs, _ := s.PushSubscriptions(ctx, 0); len(subs) != 1 {
		t.Errorf("the session alive again is not sent to: %+v", subs)
	}
	clock = t0.Add(60 * day)
	alive("the key used at day 20, at day 60", phoneSecret, false)
	setLimit(0)
	alive("the key used at day 20, the limit 0", phoneSecret, true)
	alive("the integration's key, the limit 0", integSecret, true)
	setLimit(nil)
	alive("the key never used, the limit unset again", neverSecret, true)

	// Kept from the limit, an ended key lives again; held to it again, it
	// has ended again (D6-A).
	setLimit(7)
	alive("the key never used, at day 60", neverSecret, false)
	yes, no := true, false
	got, err := s.SetTokenLife(ctx, never.ID, TokenLife{IdleExempt: &yes})
	if err != nil || !got.IdleExempt || got.Ended != "" || !got.IdleEndsAt.IsZero() || got.Version != never.Version+1 {
		t.Fatalf("kept from the limit: %+v %v", got, err)
	}
	alive("the key never used, kept", neverSecret, true)
	if got, err = s.SetTokenLife(ctx, never.ID, TokenLife{}); err != nil || !got.IdleExempt {
		t.Errorf("no change keeps the exclusion: %+v %v", got, err)
	}
	if got, err = s.SetTokenLife(ctx, never.ID, TokenLife{IdleExempt: &no}); err != nil || got.IdleExempt || got.Ended != EndedIdle {
		t.Errorf("held to the limit again: %+v %v", got, err)
	}
	alive("the key never used, held to the limit again", neverSecret, false)

	// An integration's key takes no exclusion; a session, a revoked key and
	// none take no life.
	if _, err := s.SetTokenLife(ctx, integ.ID, TokenLife{IdleExempt: &yes}); !errors.Is(err, ErrIntegrationUser) {
		t.Errorf("an integration's key kept from the limit: %v", err)
	}
	if got, err := s.SetTokenLife(ctx, integ.ID, TokenLife{IdleExempt: &no}); err != nil || got.IdleExempt {
		t.Errorf("an integration's key held to the limit, as it is: %+v %v", got, err)
	}
	if _, err := s.SetTokenLife(ctx, sign.ID, TokenLife{IdleExempt: &yes}); !errors.Is(err, ErrNoToken) {
		t.Errorf("a session given a life: %v", err)
	}
	if _, err := s.RevokeToken(ctx, kept.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.SetTokenLife(ctx, kept.ID, TokenLife{IdleExempt: &no}); !errors.Is(err, ErrNoToken) {
		t.Errorf("a revoked key given a life: %v", err)
	}
	if _, err := s.SetTokenLife(ctx, 9999, TokenLife{IdleExempt: &no}); !errors.Is(err, ErrNoToken) {
		t.Errorf("no key given a life: %v", err)
	}
}

// An integration's key ended unused counts as no live key for what the
// store leaves to its first regular user (OpenSetup, AP KeyLife, Block
// 3): its integration is listed while the key lives, not once it ended.
func TestOpenSetupIdleKey(t *testing.T) {
	t.Parallel()
	t0 := time.Date(2026, 11, 27, 12, 0, 0, 0, time.UTC)
	clock := t0
	s, err := Open(copied(t), Options{Logger: quiet(), Now: func() time.Time { return clock }})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx := context.Background()
	req, err := s.AddIntegrationRequest(ctx, haRequest(instanceA), "K7Q2PX")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.ApproveIntegration(ctx, req.UUID, []string{"read"}, "", 0); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.CollectIntegrationKey(ctx, req.UUID, req.RequestCode); err != nil {
		t.Fatal(err)
	}
	if err := s.SetSetting(ctx, KeyIdleDaysKey, 2); err != nil {
		t.Fatal(err)
	}
	clock = t0.Add(RequestLife + time.Hour)
	if set, err := s.OpenSetup(ctx); err != nil || len(set.Integrations) != 1 {
		t.Fatalf("the integration with a live key: %+v %v", set.Integrations, err)
	}
	clock = t0.Add(48 * time.Hour)
	if set, err := s.OpenSetup(ctx); err != nil || len(set.Integrations) != 0 {
		t.Fatalf("the integration whose key ended unused: %+v %v", set.Integrations, err)
	}
}

// The rule alive in SQL and its form in Go for a token's own row
// (lifeRule.apply) say the same on one table of cases (AP KeyLife,
// Block 3): a token lives in SQL exactly where Go finds it not ended,
// and Go names why it ended. The clock stands between two milliseconds,
// as the store's times do not, at each edge.
func TestKeyEndRule(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 11, 27, 12, 0, 0, 400_000, time.UTC) // 0.4 ms past the second
	clock := now
	s, err := Open(copied(t), Options{Logger: quiet(), Now: func() time.Time { return clock }})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx := context.Background()
	anna, _ := s.AddUser(ctx, User{Name: "anna", Role: RoleOperator}, "pw", 0)
	file := HashToken("placeholder-file")
	s.fileHash.Store(&file)
	day, ms := 24*time.Hour, time.Millisecond
	cut := now.Add(-7 * day).Truncate(ms) // the limit's edge at 7 days, as the store keeps times
	var never time.Time
	for n, c := range []struct {
		what               string
		kind               string
		days               int // the limit, 0 for off
		exempt, ofFile     bool
		created, used, end time.Time
		revoked            bool
		ended              string
	}{
		{what: "a key, the limit off", kind: TokenAPI, created: now.Add(-400 * day), ended: ""},
		{what: "a key used a millisecond after the edge", kind: TokenAPI, days: 7, created: now.Add(-30 * day), used: cut.Add(ms)},
		{what: "a key used at the edge", kind: TokenAPI, days: 7, created: now.Add(-30 * day), used: cut, ended: EndedIdle},
		{what: "a key used before the edge", kind: TokenAPI, days: 7, created: now.Add(-30 * day), used: cut.Add(-day), ended: EndedIdle},
		{what: "a key never used, made after the edge", kind: TokenAPI, days: 7, created: cut.Add(ms), used: never},
		{what: "a key never used, made at the edge", kind: TokenAPI, days: 7, created: cut, ended: EndedIdle},
		{what: "a key kept from the limit", kind: TokenAPI, days: 7, exempt: true, created: now.Add(-30 * day)},
		{what: "the settings file's key", kind: TokenAPI, days: 7, ofFile: true, created: now.Add(-30 * day)},
		{what: "a session unused for long", kind: TokenSession, days: 7, created: now.Add(-30 * day), end: now.Add(day)},
		{what: "a key past its end date, used", kind: TokenAPI, days: 7, created: now.Add(-30 * day), used: now.Add(-day), end: now.Truncate(ms), ended: EndedExpired},
		{what: "a key a millisecond before its end date", kind: TokenAPI, days: 7, created: now.Add(-30 * day), used: now.Add(-day), end: now.Truncate(ms).Add(ms)},
		{what: "a key ended unused before its end date", kind: TokenAPI, days: 7, created: now.Add(-30 * day), used: cut.Add(-day), end: now.Add(-12 * time.Hour), ended: EndedIdle},
		{what: "a key past its end date before it was unused too long", kind: TokenAPI, days: 7, created: now.Add(-30 * day), used: cut.Add(-day), end: cut.Add(-2 * day), ended: EndedExpired},
		{what: "a key ended unused on the day its end date came", kind: TokenAPI, days: 7, created: now.Add(-30 * day), used: cut.Add(-day), end: cut.Add(-day).Add(7 * day), ended: EndedExpired},
		{what: "a revoked key", kind: TokenAPI, created: now.Add(-day), revoked: true, ended: EndedRevoked},
	} {
		if c.days > 0 {
			if err := s.SetSetting(ctx, KeyIdleDaysKey, c.days); err != nil {
				t.Fatal(err)
			}
		} else if err := s.DeleteSetting(ctx, KeyIdleDaysKey); err != nil {
			t.Fatal(err)
		}
		hash := HashToken(fmt.Sprintf("placeholder-%d", n))
		if c.ofFile {
			hash = file
		}
		var used, end, revoked any
		if !c.used.IsZero() {
			used = formatTime(c.used)
		}
		if !c.end.IsZero() {
			end = formatTime(c.end)
		}
		if c.revoked {
			revoked = formatTime(now.Add(-time.Hour))
		}
		res, err := s.db.ExecContext(ctx, `INSERT INTO tokens (uuid, user_id, kind, hash, label, created_at, expires_at, last_used_at, deleted_at, origin, idle_exempt) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, '', ?)`,
			newUUID(), anna.ID, c.kind, hash, c.what, formatTime(c.created), end, used, revoked, c.exempt)
		if err != nil {
			t.Fatal(err)
		}
		id, _ := res.LastInsertId()
		_, inSQL, err := s.TokenAlive(ctx, id)
		if err != nil {
			t.Fatal(err)
		}
		row, _, _ := s.Token(ctx, id)
		r, err := s.rule(ctx, clock)
		if err != nil {
			t.Fatal(err)
		}
		r.apply(&row)
		if inSQL != (row.Ended == "") || row.Ended != c.ended {
			t.Errorf("%s: alive in SQL %v, ended in Go %q, want %q (%+v)", c.what, inSQL, row.Ended, c.ended, row)
		}
	}
}

// A key ended unused is no revoked key (AP KeyLife, D6-A): its code
// still revokes it, the keys it made with it and an heir's session, as a
// revoke by its id would, and RevokeByCode names the heirs. Its use after
// the end is refused by the rule and writes nothing, so a key made before
// the store kept codes, ended before its first use since, gets no revoke
// hash then, and its code matches nothing.
func TestIdleKeyRevokedByCode(t *testing.T) {
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
	key, keySecret, _ := s.AddKey(ctx, anna.ID, KeyOptions{Label: "phone"})
	heir, heirSecret, err := s.AddKey(ctx, anna.ID, KeyOptions{Label: "heir, kept from the limit", MadeBy: key.ID, IdleExempt: true})
	if err != nil {
		t.Fatal(err)
	}
	_, heirSessSecret, err := s.AddKeySession(ctx, heir.ID, "tablet", t0.Add(30*day))
	if err != nil {
		t.Fatal(err)
	}
	old, oldSecret, _ := s.AddKey(ctx, anna.ID, KeyOptions{Label: "made before 0100"})
	if _, err := s.db.ExecContext(ctx, `UPDATE tokens SET revoke_hash = NULL WHERE id = ?`, old.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.SetSetting(ctx, KeyIdleDaysKey, 7); err != nil {
		t.Fatal(err)
	}
	clock = t0.Add(8 * day)
	list, err := s.Tokens(ctx, anna.ID)
	if err != nil {
		t.Fatal(err)
	}
	ended := map[int64]string{}
	for _, tok := range list {
		ended[tok.ID] = tok.Ended
	}
	if ended[key.ID] != EndedIdle || ended[old.ID] != EndedIdle || ended[heir.ID] != "" {
		t.Fatalf("the ends before the codes: %v", ended)
	}

	// The use after the end: refused, and nothing written.
	before, _, _ := s.Token(ctx, old.ID)
	if _, ok, _ := s.TokenByHash(ctx, HashToken(oldSecret)); ok {
		t.Fatal("a key ended unused let in")
	}
	if after, _, _ := s.Token(ctx, old.ID); after.RevokeKept || after.Version != before.Version || !after.LastUsedAt.Equal(before.LastUsedAt) {
		t.Fatalf("the ended key's use was written: %+v, before %+v", after, before)
	}
	if _, _, ok, err := s.RevokeByCode(ctx, RevokeCode(oldSecret)); ok || err != nil {
		t.Fatalf("the code of an ended key without a revoke hash: %v %v", ok, err)
	}

	got, heirs, ok, err := s.RevokeByCode(ctx, RevokeCode(keySecret))
	if err != nil || !ok || got.ID != key.ID {
		t.Fatalf("the code of a key ended unused: %+v %v %v", got, ok, err)
	}
	if fmt.Sprint(heirs) != fmt.Sprint([]int64{heir.ID}) {
		t.Errorf("the heirs: %v, want [%d]", heirs, heir.ID)
	}
	if now, _, _ := s.Token(ctx, key.ID); now.DeletedAt.IsZero() {
		t.Error("the key ended unused is not revoked by its code")
	}
	for what, secret := range map[string]string{"the heir": heirSecret, "the heir's session": heirSessSecret} {
		if _, ok, _ := s.TokenByHash(ctx, HashToken(secret)); ok {
			t.Errorf("%s lives on after the code of a key ended unused", what)
		}
	}
	// Raised again, the limit brings back no key a code revoked.
	if err := s.SetSetting(ctx, KeyIdleDaysKey, 0); err != nil {
		t.Fatal(err)
	}
	if _, ok, _ := s.TokenByHash(ctx, HashToken(keySecret)); ok {
		t.Error("the key revoked by its code lives again with the limit off")
	}
	if _, ok, _ := s.TokenByHash(ctx, HashToken(oldSecret)); !ok {
		t.Error("the key that only ended unused does not live again with the limit off")
	}
}
