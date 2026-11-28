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
	if _, err := s.RevokeToken(ctx, sess.ID); !errors.Is(err, ErrNoToken) {
		t.Fatalf("revoke twice: %v", err)
	}
	if _, err := s.RevokeToken(ctx, key.ID); err != nil {
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
	if _, err := s.RevokeToken(ctx, key.ID); err != nil {
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

	// A key past its end takes its session with it (AP KeyLife, Block 2):
	// the session, alive by its own end, is refused, has ended, is not
	// listed, is not sent to, and the key makes no other. The key itself
	// is refused too, and stays listed, to be revoked.
	ending, endingSecret, _ := s.AddToken(ctx, anna.ID, TokenAPI, "phone with an end", clock.Add(30*time.Minute))
	fourth, fourthSecret, err := s.AddKeySession(ctx, ending.ID, "phone", end)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.AddPushSubscription(ctx, PushSubscription{UserID: anna.ID, TokenID: fourth.ID, Endpoint: "https://push.example.net/b", P256dh: "k", Auth: "a"}); err != nil {
		t.Fatal(err)
	}
	clock = clock.Add(30 * time.Minute) // the key's end; the session's own lies 30 days on, the sign-in's half an hour
	if _, ok, _ := s.TokenByHash(ctx, HashToken(fourthSecret)); ok {
		t.Error("the session of a key past its end was found")
	}
	if _, ok, _ := s.TokenByHash(ctx, HashToken(endingSecret)); ok {
		t.Error("a key past its end was found")
	}
	if got, err := s.TokenEnded(ctx, HashToken(fourthSecret), clock); err != nil || !got {
		t.Errorf("the session of a key past its end has not ended: %v %v", got, err)
	}
	list, _ = s.Tokens(ctx, anna.ID)
	if len(list) != 2 || list[0].ID != plain.ID || list[1].ID != ending.ID {
		t.Errorf("live tokens with a key past its end %+v (its session %d)", list, fourth.ID)
	}
	if subs, _ := s.PushSubscriptions(ctx, 0); len(subs) != 0 {
		t.Errorf("the session of a key past its end is sent to: %+v", subs)
	}
	if _, _, err := s.AddKeySession(ctx, ending.ID, "", end); !errors.Is(err, ErrNoToken) {
		t.Errorf("a key past its end made a session: %v", err)
	}
}

// TokenAlive gives a token by its id under the rule TokenByHash holds
// to (AP KeyLife, Block 2): not revoked, not past its end, its user not
// removed, and a session made from a key only while the key is neither
// revoked nor past its end.
func TestTokenAlive(t *testing.T) {
	t.Parallel()
	clock := time.Date(2026, 11, 27, 12, 0, 0, 0, time.UTC)
	s, err := Open(copied(t), Options{Logger: quiet(), Now: func() time.Time { return clock }})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx := context.Background()
	anna, _ := s.AddUser(ctx, User{Name: "anna", Role: RoleOperator}, "pw", 0)
	bob, _ := s.AddUser(ctx, User{Name: "bob", Role: RoleOperator}, "pw", 0)
	month := clock.Add(30 * 24 * time.Hour)
	alive := func(what string, id int64, want bool) {
		t.Helper()
		got, ok, err := s.TokenAlive(ctx, id)
		if err != nil || ok != want || (ok && got.ID != id) {
			t.Errorf("%s: alive %v %+v %v, want %v", what, ok, got, err, want)
		}
	}

	session, _, _ := s.AddToken(ctx, anna.ID, TokenSession, "Firefox", month)
	key, _, _ := s.AddToken(ctx, anna.ID, TokenAPI, "Loxone", zeroTime)
	ofKey, _, _ := s.AddKeySession(ctx, key.ID, "phone", month)
	alive("a session", session.ID, true)
	alive("a key", key.ID, true)
	alive("a key's session", ofKey.ID, true)
	alive("no token", 9999, false)

	revoked, _, _ := s.AddToken(ctx, anna.ID, TokenSession, "Firefox", month)
	if _, err := s.RevokeToken(ctx, revoked.ID); err != nil {
		t.Fatal(err)
	}
	alive("a revoked session", revoked.ID, false)
	past, _, _ := s.AddToken(ctx, anna.ID, TokenSession, "Firefox", clock)
	alive("a session at its end", past.ID, false)

	revokedKey, _, _ := s.AddToken(ctx, anna.ID, TokenAPI, "phone", zeroTime)
	ofRevoked, _, _ := s.AddKeySession(ctx, revokedKey.ID, "phone", month)
	if _, err := s.db.ExecContext(ctx, `UPDATE tokens SET deleted_at = ? WHERE id = ?`, s.nowText(), revokedKey.ID); err != nil {
		t.Fatal(err)
	}
	alive("the session of a key an older binary revoked", ofRevoked.ID, false)

	endingKey, _, _ := s.AddToken(ctx, anna.ID, TokenAPI, "phone", clock.Add(time.Minute))
	ofEnding, _, _ := s.AddKeySession(ctx, endingKey.ID, "phone", month)
	alive("the session of a key before its end", ofEnding.ID, true)
	clock = clock.Add(time.Minute)
	alive("a key at its end", endingKey.ID, false)
	alive("the session of a key at its end", ofEnding.ID, false)

	bobs, _, _ := s.AddToken(ctx, bob.ID, TokenAPI, "k", zeroTime)
	if err := s.DeleteUser(ctx, "bob", 0); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.ExecContext(ctx, `UPDATE tokens SET deleted_at = NULL WHERE id = ?`, bobs.ID); err != nil {
		t.Fatal(err)
	}
	alive("the key of a removed user, the key itself not revoked", bobs.ID, false)
	alive("a session, after the others", session.ID, true)
}

// A key made by a key names it (made_by, migration 0101), and a revoke
// takes the keys it made, and theirs, of any user, with their sessions
// and what those subscribed to push (AP KeyLife, Block 2, D5-A, D8-A);
// the makers are followed from key to key in any state, so a key past
// its end, or one an older binary revoked alone, passes the revoke on.
// A key's end takes none of them, and nor does a key made by no key.
func TestKeyHeirs(t *testing.T) {
	t.Parallel()
	clock := time.Date(2026, 11, 27, 12, 0, 0, 0, time.UTC)
	s, err := Open(copied(t), Options{Logger: quiet(), Now: func() time.Time { return clock }})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx := context.Background()
	anna, _ := s.AddUser(ctx, User{Name: "anna", Role: RoleManager}, "pw", 0)
	bob, _ := s.AddUser(ctx, User{Name: "bob", Role: RoleOperator}, "pw", 0)
	month := clock.Add(30 * 24 * time.Hour)
	key := func(u User, label string, madeBy int64) (Token, string) {
		t.Helper()
		tok, secret, err := s.AddKey(ctx, u.ID, KeyOptions{Label: label, MadeBy: madeBy})
		if err != nil || tok.Kind != TokenAPI || tok.Label != label || tok.MadeBy != madeBy || !tok.ExpiresAt.IsZero() || len(secret) != 64 {
			t.Fatalf("key %s: %+v %v", label, tok, err)
		}
		if got, _, _ := s.Token(ctx, tok.ID); got.MadeBy != madeBy {
			t.Fatalf("key %s read again: made by %d, want %d", label, got.MadeBy, madeBy)
		}
		return tok, secret
	}
	alive := func(what, secret string, want bool) {
		t.Helper()
		if _, ok, _ := s.TokenByHash(ctx, HashToken(secret)); ok != want {
			t.Errorf("%s: alive %v, want %v", what, ok, want)
		}
	}

	root, rootSecret := key(anna, "root", 0)
	child, childSecret := key(anna, "child", root.ID)
	sibling, siblingSecret := key(anna, "sibling", root.ID)
	grandchild, grandchildSecret := key(bob, "grandchild, another user's", child.ID)
	great, greatSecret := key(bob, "great-grandchild", grandchild.ID)
	_, otherSecret := key(anna, "other", 0)
	session, sessionSecret, err := s.AddKeySession(ctx, grandchild.ID, "phone", month)
	if err != nil || session.MadeBy != 0 {
		t.Fatalf("a key's session: %+v %v", session, err)
	}
	sub, _, err := s.AddPushSubscription(ctx, PushSubscription{UserID: bob.ID, TokenID: session.ID, Endpoint: "https://push.example.net/c", P256dh: "k", Auth: "a"})
	if err != nil {
		t.Fatal(err)
	}

	// The child ends by its end date: its heirs stay, and pass the
	// revoke on all the same.
	if _, err := s.db.ExecContext(ctx, `UPDATE tokens SET expires_at = ? WHERE id = ?`, formatTime(clock), child.ID); err != nil {
		t.Fatal(err)
	}
	alive("a key past its end", childSecret, false)
	alive("the heir of a key past its end", grandchildSecret, true)
	alive("the session of that heir", sessionSecret, true)

	heirs, err := s.RevokeToken(ctx, root.ID)
	if err != nil {
		t.Fatal(err)
	}
	if want := []int64{child.ID, sibling.ID, grandchild.ID, great.ID}; fmt.Sprint(heirs) != fmt.Sprint(want) {
		t.Errorf("heirs %v, want %v", heirs, want)
	}
	for what, secret := range map[string]string{"the root": rootSecret, "its child": childSecret, "its sibling": siblingSecret,
		"the grandchild": grandchildSecret, "the great-grandchild": greatSecret, "the grandchild's session": sessionSecret} {
		alive(what+" after the root's revoke", secret, false)
		if ended, _ := s.TokenEnded(ctx, HashToken(secret), clock); !ended {
			t.Errorf("%s after the root's revoke has not ended", what)
		}
	}
	if got, _, _ := s.Token(ctx, child.ID); got.DeletedAt.IsZero() {
		t.Error("the child past its end not revoked")
	}
	if _, ok, _ := s.PushSubscription(ctx, sub.ID); ok {
		t.Error("the heir's session's subscription stayed")
	}
	alive("a key no key made", otherSecret, true)
	if heirs, err := s.RevokeToken(ctx, root.ID); !errors.Is(err, ErrNoToken) || heirs != nil {
		t.Errorf("a revoke again: %v %v", heirs, err)
	}

	// A key that made none has no heirs; one an older binary revoked
	// alone passes the revoke on to the keys it made.
	lone, _ := key(anna, "lone", 0)
	if heirs, err := s.RevokeToken(ctx, lone.ID); err != nil || heirs == nil || len(heirs) != 0 {
		t.Errorf("a key without heirs: %v %v", heirs, err)
	}
	first, _ := key(anna, "first", 0)
	second, _ := key(anna, "second", first.ID)
	third, thirdSecret := key(bob, "third", second.ID)
	if _, err := s.db.ExecContext(ctx, `UPDATE tokens SET deleted_at = ? WHERE id = ?`, s.nowText(), second.ID); err != nil {
		t.Fatal(err)
	}
	alive("the heir of a key an older binary revoked", thirdSecret, true)
	if heirs, err := s.RevokeToken(ctx, first.ID); err != nil || fmt.Sprint(heirs) != fmt.Sprint([]int64{third.ID}) {
		t.Errorf("through a key revoked before: %v %v", heirs, err)
	}
	alive("the heir of a key an older binary revoked, after its maker's revoke", thirdSecret, false)

	// A user's removal revokes its keys, and their heirs with them.
	carol, _ := s.AddUser(ctx, User{Name: "carol", Role: RoleManager}, "pw", 0)
	carols, _ := key(carol, "carol's", 0)
	_, bobsOfCarol := key(bob, "made with carol's key", carols.ID)
	_, bobsOwn := key(bob, "bob's own", 0)
	if err := s.DeleteUser(ctx, "carol", 0); err != nil {
		t.Fatal(err)
	}
	alive("the heir of a removed user's key", bobsOfCarol, false)
	alive("a key of the heir's user beside it", bobsOwn, true)
}

// The bootstrap's revoke of a key that is no longer the settings file's
// is a revoke: the keys it made go with it (AP KeyLife, D8-A), and the
// result counts them.
func TestBootstrapRevokeTakesHeirs(t *testing.T) {
	t.Parallel()
	s := open(t, t.TempDir())
	ctx := context.Background()
	if res, err := s.Bootstrap(ctx, "file-one"); err != nil || !res.OwnerCreated {
		t.Fatalf("bootstrap: %+v %v", res, err)
	}
	owner, _, _ := s.User(ctx, BootstrapUser)
	file, ok, err := s.TokenByHash(ctx, HashToken("file-one"))
	if err != nil || !ok {
		t.Fatalf("the file's key: %v %v", ok, err)
	}
	heir, heirSecret, err := s.AddKey(ctx, owner.ID, KeyOptions{Label: "made with the file's key", MadeBy: file.ID})
	if err != nil {
		t.Fatal(err)
	}
	_, otherSecret, _ := s.AddKey(ctx, owner.ID, KeyOptions{Label: "made by a session"})
	res, err := s.Bootstrap(ctx, "file-two")
	if err != nil || res.Revoked != 1 || res.Heirs != 1 || !res.TokenAdded {
		t.Fatalf("the file's token changed: %+v %v", res, err)
	}
	if _, ok, _ := s.TokenByHash(ctx, HashToken(heirSecret)); ok {
		t.Errorf("the heir %d of the file's old key lives", heir.ID)
	}
	if _, ok, _ := s.TokenByHash(ctx, HashToken(otherSecret)); !ok {
		t.Error("a key no key made went with the file's old key")
	}
}

// A key made with the settings file's key and then put into the file is
// no heir once the bootstrap takes it: a key the bootstrap brings in has
// no maker (AP KeyLife, D5-A). The revoke of the file's old key leaves it,
// and the next start finds it as the file's.
func TestBootstrapTakesAKeyTheOldOneMade(t *testing.T) {
	t.Parallel()
	s := open(t, t.TempDir())
	ctx := context.Background()
	if res, err := s.Bootstrap(ctx, "file-one"); err != nil || !res.OwnerCreated {
		t.Fatalf("bootstrap: %+v %v", res, err)
	}
	owner, _, _ := s.User(ctx, BootstrapUser)
	file, _, _ := s.TokenByHash(ctx, HashToken("file-one"))
	_, keySecret, err := s.AddKey(ctx, owner.ID, KeyOptions{Label: "laptop", MadeBy: file.ID})
	if err != nil {
		t.Fatal(err)
	}
	res, err := s.Bootstrap(ctx, keySecret)
	if err != nil || res.Revoked != 1 || res.Heirs != 0 || res.TokenAdded {
		t.Errorf("the key put into the file: %+v %v", res, err)
	}
	if got, ok, _ := s.TokenByHash(ctx, HashToken(keySecret)); !ok || got.MadeBy != 0 {
		t.Errorf("the file's new key: %+v %v, want it live and of no maker", got, ok)
	}
	if res, err := s.Bootstrap(ctx, keySecret); err != nil || res.Revoked != 0 || res.TokenAdded {
		t.Errorf("the next start: %+v %v", res, err)
	}
}

// The settings file's key goes with no other key's revoke, whoever made
// it (MobileApp.D86-A, D89-A): the revoke of its maker and the removal of
// its maker's user pass over it, and so over the keys it made.
func TestHeirsPassOverTheFilesKey(t *testing.T) {
	t.Parallel()
	s := open(t, t.TempDir())
	ctx := context.Background()
	olga, _ := s.AddUser(ctx, User{Name: "olga", Role: RoleOwner}, "", 0)
	anna, _ := s.AddUser(ctx, User{Name: "anna", Role: RoleManager}, "", 0)
	maker, _, _ := s.AddKey(ctx, anna.ID, KeyOptions{Label: "maker"})
	file, fileSecret, _ := s.AddKey(ctx, olga.ID, KeyOptions{Label: "to be the file's", MadeBy: maker.ID})
	_, ofFileSecret, _ := s.AddKey(ctx, olga.ID, KeyOptions{Label: "made with the file's", MadeBy: file.ID})
	if _, err := s.Bootstrap(ctx, fileSecret); err != nil {
		t.Fatal(err)
	}
	// Whatever brought it there, the file's key named its maker.
	madeBy := func(id int64) {
		t.Helper()
		if _, err := s.db.ExecContext(ctx, `UPDATE tokens SET made_by = ? WHERE id = ?`, id, file.ID); err != nil {
			t.Fatal(err)
		}
	}
	alive := func(what, secret string) {
		t.Helper()
		if _, ok, _ := s.TokenByHash(ctx, HashToken(secret)); !ok {
			t.Errorf("%s went", what)
		}
	}
	madeBy(maker.ID)
	heirs, err := s.RevokeToken(ctx, maker.ID)
	if err != nil || len(heirs) != 0 {
		t.Errorf("the revoke of the file's key's maker: heirs %v %v", heirs, err)
	}
	alive("the file's key, at its maker's revoke", fileSecret)
	alive("a key the file's key made, at the revoke of the file's key's maker", ofFileSecret)
	other, _, _ := s.AddKey(ctx, anna.ID, KeyOptions{Label: "another maker"})
	madeBy(other.ID)
	if err := s.DeleteUser(ctx, "anna", 0); err != nil {
		t.Fatal(err)
	}
	alive("the file's key, at its maker's user's removal", fileSecret)
}

// The bootstrap's revoke of the file's old key takes an heir of it that
// carries the file's label too, which the loop then finds revoked: it
// goes on, and the file's new token is added (AP KeyLife, D8-A).
func TestBootstrapRevokeTakesAnHeirOfTheFilesLabel(t *testing.T) {
	t.Parallel()
	s := open(t, t.TempDir())
	ctx := context.Background()
	if _, err := s.Bootstrap(ctx, "file-one"); err != nil {
		t.Fatal(err)
	}
	owner, _, _ := s.User(ctx, BootstrapUser)
	file, _, _ := s.TokenByHash(ctx, HashToken("file-one"))
	_, heirSecret, err := s.AddKey(ctx, owner.ID, KeyOptions{Label: BootstrapLabel, MadeBy: file.ID})
	if err != nil {
		t.Fatal(err)
	}
	res, err := s.Bootstrap(ctx, "file-two")
	if err != nil || !res.TokenAdded || res.Revoked != 1 || res.Heirs != 1 {
		t.Fatalf("the file's token changed: %+v %v", res, err)
	}
	if _, ok, _ := s.TokenByHash(ctx, HashToken("file-two")); !ok {
		t.Error("the file's new token was not added")
	}
	if _, ok, _ := s.TokenByHash(ctx, HashToken(heirSecret)); ok {
		t.Error("the heir of the file's label lives")
	}
}

// A key is made with a maker only while the maker lives (AP KeyLife,
// D5-A): a request that passed the check of its bearer before the
// bearer's revoke, and makes its key after it, makes none, so that no
// live key names a revoked maker that no later revoke would reach.
func TestAddKeyNeedsALiveMaker(t *testing.T) {
	t.Parallel()
	clock := time.Date(2026, 11, 27, 12, 0, 0, 0, time.UTC)
	s, err := Open(copied(t), Options{Logger: quiet(), Now: func() time.Time { return clock }})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx := context.Background()
	anna, _ := s.AddUser(ctx, User{Name: "anna", Role: RoleManager}, "", 0)
	revoked, _, _ := s.AddKey(ctx, anna.ID, KeyOptions{Label: "revoked"})
	if _, err := s.RevokeToken(ctx, revoked.ID); err != nil {
		t.Fatal(err)
	}
	ending, _, _ := s.AddToken(ctx, anna.ID, TokenAPI, "ending", clock.Add(time.Minute))
	clock = clock.Add(time.Minute)
	for what, maker := range map[string]int64{"a revoked maker": revoked.ID, "a maker past its end": ending.ID, "no maker that exists": 9999} {
		if tok, _, err := s.AddKey(ctx, anna.ID, KeyOptions{Label: "late", MadeBy: maker}); !errors.Is(err, ErrNoToken) {
			t.Errorf("%s: %+v %v", what, tok, err)
		}
	}
	live, _, _ := s.AddKey(ctx, anna.ID, KeyOptions{Label: "live"})
	if tok, _, err := s.AddKey(ctx, anna.ID, KeyOptions{Label: "made with a live one", MadeBy: live.ID}); err != nil || tok.MadeBy != live.ID {
		t.Errorf("a live maker: %+v %v", tok, err)
	}
	list, _ := s.Tokens(ctx, anna.ID)
	for _, tk := range list {
		if tk.Label == "late" {
			t.Errorf("a key of an ended maker was made: %+v", tk)
		}
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
	if _, err := s.RevokeToken(ctx, loggedOut.ID); err != nil {
		t.Fatal(err)
	}
	ended("a session logged out", loggedOutSecret, true)
	revokedKey, revokedKeySecret, _ := s.AddToken(ctx, anna.ID, TokenAPI, "phone, login by fingerprint", zeroTime)
	_, keySession, _ := s.AddKeySession(ctx, revokedKey.ID, "phone", end)
	if _, err := s.RevokeToken(ctx, revokedKey.ID); err != nil {
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
// token that ended by a revoke, an unknown code, or the settings file's
// key, which the store knows by the hash its bootstrap last applied.
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
	got, heirs, ok, err := s.RevokeByCode(ctx, RevokeCode(sessSecret))
	if err != nil || !ok || got.ID != sess.ID || got.UserID != anna.ID || !got.DeletedAt.IsZero() || heirs == nil || len(heirs) != 0 {
		t.Fatalf("a session by its code: %+v %v %v %v", got, heirs, ok, err)
	}
	if _, ok, _ := s.TokenByHash(ctx, HashToken(sessSecret)); ok {
		t.Fatal("the session lives on after its code")
	}
	if _, _, ok, err := s.RevokeByCode(ctx, RevokeCode(sessSecret)); ok || err != nil {
		t.Fatalf("a code taken already: %v %v", ok, err)
	}
	if _, _, ok, err := s.RevokeByCode(ctx, RevokeCode("nobody's")); ok || err != nil {
		t.Fatalf("an unknown code: %v %v", ok, err)
	}

	key, keySecret, _ := s.AddToken(ctx, anna.ID, TokenAPI, "phone, login by fingerprint", zeroTime)
	keySess, _, _ := s.AddKeySession(ctx, key.ID, "phone", clock.Add(time.Hour))
	sub, _, err := s.AddPushSubscription(ctx, PushSubscription{UserID: anna.ID, TokenID: keySess.ID, Endpoint: "https://push.example.net/a", P256dh: "k", Auth: "a"})
	if err != nil {
		t.Fatal(err)
	}
	if got, _, ok, err := s.RevokeByCode(ctx, RevokeCode(keySecret)); err != nil || !ok || got.ID != key.ID {
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
	if _, _, ok, err := s.RevokeByCode(ctx, RevokeCode(oldSecret)); !ok || err != nil {
		t.Fatalf("a key past its end: %v %v", ok, err)
	}
	if got, _, _ := s.Token(ctx, oldSess.ID); got.DeletedAt.IsZero() {
		t.Fatal("the session of a key past its end lives on")
	}

	// The settings file's key, named by its hash, is left live.
	file, fileSecret, _ := s.AddToken(ctx, anna.ID, TokenAPI, "named in the file later", zeroTime)
	if _, err := s.Bootstrap(ctx, fileSecret); err != nil {
		t.Fatal(err)
	}
	if _, _, ok, err := s.RevokeByCode(ctx, RevokeCode(fileSecret)); ok || err != nil {
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
	if _, _, ok, err := s.RevokeByCode(ctx, RevokeCode("nobody's")); ok || err != nil {
		t.Fatalf("a code that matches nothing: %v %v", ok, err)
	}
	if _, _, ok, err := s.RevokeByCode(ctx, RevokeCode(secret)); ok || err == nil {
		t.Fatalf("a match reaches the write: %v %v", ok, err)
	}
}

// A key's code revokes it as its revoke does (AP KeyLife, D5-A): the
// keys it made, and theirs, of any user, go with it, with their sessions
// and what those subscribed to push, and RevokeByCode returns them as
// RevokeToken does; its own session goes too, another key stays. An
// heir's own code then matches nothing.
func TestRevokeByCodeTakesTheHeirs(t *testing.T) {
	t.Parallel()
	s := open(t, t.TempDir())
	ctx := context.Background()
	anna, _ := s.AddUser(ctx, User{Name: "anna", Role: RoleManager}, "", 0)
	bob, _ := s.AddUser(ctx, User{Name: "bob", Role: RoleOperator}, "", 0)
	hour := time.Now().Add(time.Hour)
	root, rootSecret, _ := s.AddToken(ctx, anna.ID, TokenAPI, "root", zeroTime)
	child, childSecret, _ := s.AddKey(ctx, anna.ID, KeyOptions{Label: "child", MadeBy: root.ID})
	grandchild, grandchildSecret, _ := s.AddKey(ctx, bob.ID, KeyOptions{Label: "grandchild, another user's", MadeBy: child.ID})
	_, otherSecret, _ := s.AddKey(ctx, anna.ID, KeyOptions{Label: "other"})
	_, rootSessSecret, _ := s.AddKeySession(ctx, root.ID, "phone", hour)
	heirSess, heirSessSecret, err := s.AddKeySession(ctx, grandchild.ID, "tablet", hour)
	if err != nil {
		t.Fatal(err)
	}
	sub, _, err := s.AddPushSubscription(ctx, PushSubscription{UserID: bob.ID, TokenID: heirSess.ID, Endpoint: "https://push.example.net/h", P256dh: "k", Auth: "a"})
	if err != nil {
		t.Fatal(err)
	}
	got, heirs, ok, err := s.RevokeByCode(ctx, RevokeCode(rootSecret))
	if err != nil || !ok || got.ID != root.ID {
		t.Fatalf("the key by its code: %+v %v %v", got, ok, err)
	}
	if want := []int64{child.ID, grandchild.ID}; fmt.Sprint(heirs) != fmt.Sprint(want) {
		t.Errorf("the heirs: %v, want %v", heirs, want)
	}
	for what, secret := range map[string]string{
		"the key": rootSecret, "its session": rootSessSecret, "the child": childSecret,
		"the grandchild": grandchildSecret, "the grandchild's session": heirSessSecret,
	} {
		if _, ok, _ := s.TokenByHash(ctx, HashToken(secret)); ok {
			t.Errorf("%s lives on after the key's code", what)
		}
	}
	if _, ok, _ := s.PushSubscription(ctx, sub.ID); ok {
		t.Error("the heir's session's subscription stayed")
	}
	if _, ok, _ := s.TokenByHash(ctx, HashToken(otherSecret)); !ok {
		t.Error("another key went with the key's code")
	}
	if _, _, ok, err := s.RevokeByCode(ctx, RevokeCode(childSecret)); ok || err != nil {
		t.Errorf("an heir's code after its maker's: %v %v", ok, err)
	}
}

// The settings file's key is ended by no code (MobileApp.D86-A), and the
// store knows it by itself, by the hash its bootstrap last applied, as
// the heirs of a revoke do: its code is no match and changes nothing,
// neither the key nor its session nor the key it made. Once the file
// names another token, the key is an ordinary one again.
func TestRevokeByCodeKeepsTheFilesKey(t *testing.T) {
	t.Parallel()
	s := open(t, t.TempDir())
	ctx := context.Background()
	olga, _ := s.AddUser(ctx, User{Name: "olga", Role: RoleOwner}, "", 0)
	file, fileSecret, _ := s.AddToken(ctx, olga.ID, TokenAPI, "laptop", zeroTime)
	heir, _, _ := s.AddKey(ctx, olga.ID, KeyOptions{Label: "made with the file's", MadeBy: file.ID})
	sess, _, err := s.AddKeySession(ctx, file.ID, "phone", time.Now().Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Bootstrap(ctx, fileSecret); err != nil {
		t.Fatal(err)
	}
	before := map[int64]Token{}
	for _, id := range []int64{file.ID, heir.ID, sess.ID} {
		before[id], _, _ = s.Token(ctx, id)
	}
	if got, heirs, ok, err := s.RevokeByCode(ctx, RevokeCode(fileSecret)); ok || err != nil || len(heirs) != 0 {
		t.Fatalf("the file's key's code: %+v %v %v %v, want no match", got, heirs, ok, err)
	}
	for id, was := range before {
		if now, _, _ := s.Token(ctx, id); now.Version != was.Version || !now.DeletedAt.IsZero() {
			t.Errorf("token %d changed at the file's key's code: %+v, before %+v", id, now, was)
		}
	}
	if _, err := s.Bootstrap(ctx, "another-file-token"); err != nil {
		t.Fatal(err)
	}
	if _, heirs, ok, err := s.RevokeByCode(ctx, RevokeCode(fileSecret)); !ok || err != nil || fmt.Sprint(heirs) != fmt.Sprint([]int64{heir.ID}) {
		t.Fatalf("the key's code once the file names another: %v %v %v", heirs, ok, err)
	}
}

// A key AddKey makes keeps the hash of its revoke code, as AddToken's do
// (AP MobileApp v1.6, D112-A): the store made the secret, so the key's
// code ends it, with a maker or without.
func TestAddKeyKeepsItsRevokeCode(t *testing.T) {
	t.Parallel()
	s := open(t, t.TempDir())
	ctx := context.Background()
	anna, _ := s.AddUser(ctx, User{Name: "anna", Role: RoleManager}, "", 0)
	maker, makerSecret, _ := s.AddKey(ctx, anna.ID, KeyOptions{Label: "maker"})
	heir, heirSecret, _ := s.AddKey(ctx, anna.ID, KeyOptions{Label: "heir", MadeBy: maker.ID})
	for _, c := range []struct {
		what   string
		tok    Token
		secret string
	}{{"the heir", heir, heirSecret}, {"the maker", maker, makerSecret}} {
		if !c.tok.RevokeKept {
			t.Errorf("%s keeps no revoke code: %+v", c.what, c.tok)
		}
		got, heirs, ok, err := s.RevokeByCode(ctx, RevokeCode(c.secret))
		if err != nil || !ok || got.ID != c.tok.ID || len(heirs) != 0 {
			t.Errorf("%s by its code: %+v %v %v %v", c.what, got, heirs, ok, err)
		}
		if _, ok, _ := s.TokenByHash(ctx, HashToken(c.secret)); ok {
			t.Errorf("%s lives on after its code", c.what)
		}
	}
}
