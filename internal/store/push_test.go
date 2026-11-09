// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package store

import (
	"context"
	"errors"
	"testing"
	"time"
)

// A subscription is kept per endpoint: the same endpoint again updates
// its row, which then belongs to whoever subscribed last; each user
// lists and removes only their own.
func TestPushSubscriptions(t *testing.T) {
	ctx := context.Background()
	clock := time.Date(2026, 11, 9, 6, 0, 0, 0, time.UTC)
	s, err := Open(t.TempDir(), Options{Logger: quiet(), Now: func() time.Time { return clock }})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	anna, _ := s.AddUser(ctx, User{Name: "anna", Role: RoleOperator}, "pw", 0)
	bob, _ := s.AddUser(ctx, User{Name: "bob", Role: RoleViewer}, "pw", 0)
	annaTok, _, _ := s.AddToken(ctx, anna.ID, TokenSession, "phone", clock.Add(24*time.Hour))
	bobTok, _, _ := s.AddToken(ctx, bob.ID, TokenSession, "laptop", clock.Add(24*time.Hour))

	sub := PushSubscription{UserID: anna.ID, TokenID: annaTok.ID, Endpoint: "https://push.example.net/a", P256dh: "key-a", Auth: "auth-a", Label: "Firefox"}
	p, created, err := s.AddPushSubscription(ctx, sub)
	if err != nil || !created || p.ID == 0 || p.UUID == "" || p.UserID != anna.ID || p.Label != "Firefox" || !p.CreatedAt.Equal(clock) || !p.LastOKAt.IsZero() {
		t.Fatalf("added %+v %v %v", p, created, err)
	}
	if err := s.NotePushResult(ctx, p.ID, false); err != nil {
		t.Fatal(err)
	}
	if err := s.NotePushResult(ctx, p.ID, false); err != nil {
		t.Fatal(err)
	}
	if got, _, _ := s.PushSubscription(ctx, p.ID); got.Failures != 2 {
		t.Fatalf("failures %d", got.Failures)
	}
	clock = clock.Add(time.Minute)
	if err := s.NotePushResult(ctx, p.ID, true); err != nil {
		t.Fatal(err)
	}
	if got, _, _ := s.PushSubscription(ctx, p.ID); got.Failures != 0 || !got.LastOKAt.Equal(clock) {
		t.Fatalf("after a delivery %+v", got)
	}

	// The same endpoint from Bob: his now, the keys new, the record clear.
	again, created, err := s.AddPushSubscription(ctx, PushSubscription{UserID: bob.ID, TokenID: bobTok.ID, Endpoint: sub.Endpoint, P256dh: "key-b", Auth: "auth-b", Label: "Chrome"})
	if err != nil || created || again.ID != p.ID || again.UserID != bob.ID || again.TokenID != bobTok.ID || again.P256dh != "key-b" || again.Label != "Chrome" || !again.LastOKAt.IsZero() {
		t.Fatalf("again %+v %v %v", again, created, err)
	}
	if list, _ := s.PushSubscriptions(ctx, anna.ID); len(list) != 0 {
		t.Fatalf("Anna's after Bob's: %+v", list)
	}
	if err := s.DeletePushSubscription(ctx, anna.ID, p.ID); !errors.Is(err, ErrNoPushSubscription) {
		t.Fatalf("Anna removed Bob's: %v", err)
	}
	if err := s.DeletePushSubscription(ctx, bob.ID, p.ID); err != nil {
		t.Fatal(err)
	}
	if list, _ := s.PushSubscriptions(ctx, 0); len(list) != 0 {
		t.Fatalf("after the removal: %+v", list)
	}
}

// Every subscription that may be sent to: not one whose token was
// revoked (which removes it) or has expired, nor one whose user was
// disabled or removed (which removes it).
func TestPushSubscriptionsThatMayBeSentTo(t *testing.T) {
	ctx := context.Background()
	clock := time.Date(2026, 11, 9, 6, 0, 0, 0, time.UTC)
	s, err := Open(t.TempDir(), Options{Logger: quiet(), Now: func() time.Time { return clock }})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	add := func(name string) (User, Token) {
		u, err := s.AddUser(ctx, User{Name: name, Role: RoleOperator}, "pw", 0)
		if err != nil {
			t.Fatal(err)
		}
		tok, _, _ := s.AddToken(ctx, u.ID, TokenSession, name, clock.Add(time.Hour))
		if _, _, err := s.AddPushSubscription(ctx, PushSubscription{UserID: u.ID, TokenID: tok.ID, Endpoint: "https://push.example.net/" + name, P256dh: "k", Auth: "a"}); err != nil {
			t.Fatal(err)
		}
		return u, tok
	}
	users := func() map[string]bool {
		list, err := s.PushSubscriptions(ctx, 0)
		if err != nil {
			t.Fatal(err)
		}
		out := map[string]bool{}
		for _, p := range list {
			u, _, _ := s.UserByID(ctx, p.UserID)
			out[u.Name] = true
		}
		return out
	}
	_, revoked := add("revoked")
	sessions, _ := add("sessions")
	add("disabled")
	removed, _ := add("removed")
	add("kept")
	if got := users(); len(got) != 5 {
		t.Fatalf("all five: %v", got)
	}
	if err := s.RevokeToken(ctx, revoked.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.RevokeSessions(ctx, sessions.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.UpdateUser(ctx, "disabled", func(u *User) error { u.Disabled = true; return nil }, 0); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteUser(ctx, "removed", 0); err != nil {
		t.Fatal(err)
	}
	if got := users(); len(got) != 1 || !got["kept"] {
		t.Fatalf("after the revocations: %v", got)
	}
	for _, id := range []int64{revoked.UserID, sessions.ID, removed.ID} {
		if list, _ := s.PushSubscriptions(ctx, id); len(list) != 0 {
			t.Errorf("user %d's subscription kept: %+v", id, list)
		}
	}
	clock = clock.Add(2 * time.Hour) // the sessions' end
	if got := users(); len(got) != 0 {
		t.Fatalf("after the sessions' end: %v", got)
	}
}
