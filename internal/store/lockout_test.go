// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package store

import (
	"context"
	"testing"
	"time"
)

// The lockout keeps the authLimiter's rules on auth_failures and the
// counters survive a restart of the store.
func TestLockout(t *testing.T) {
	clock := time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC)
	now := func() time.Time { return clock }
	dir := t.TempDir()
	s, err := Open(dir, Options{Logger: quiet(), Now: now})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	l := s.Lockout(3, time.Minute, 5*time.Minute)
	a := LockoutAddr("10.0.0.5")
	fail := func(key string) bool {
		t.Helper()
		locked, err := l.Fail(ctx, key)
		if err != nil {
			t.Fatal(err)
		}
		return locked
	}
	if fail(a) || fail(a) {
		t.Fatal("locked too early")
	}
	// Failures in an earlier window do not count.
	clock = clock.Add(2 * time.Minute)
	if fail(a) || fail(a) {
		t.Fatal("old failures counted")
	}
	if !fail(a) {
		t.Fatal("third failure in the window did not lock")
	}
	if d, _ := l.LockedFor(ctx, a); d != 5*time.Minute {
		t.Fatalf("locked for %v", d)
	}
	if d, _ := l.LockedFor(ctx, LockoutUser("Anna")); d != 0 {
		t.Fatal("other key locked")
	}
	l.Succeed(ctx, a) // a success during the lockout does not lift it
	if d, _ := l.LockedFor(ctx, a); d != 5*time.Minute {
		t.Fatal("success lifted the lockout")
	}

	// A restart keeps the lockout.
	s.Close()
	s, err = Open(dir, Options{Logger: quiet(), Now: now})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	l = s.Lockout(3, time.Minute, 5*time.Minute)
	clock = clock.Add(4 * time.Minute)
	if d, _ := l.LockedFor(ctx, a); d != time.Minute {
		t.Fatalf("after the restart: locked for %v", d)
	}
	clock = clock.Add(time.Minute + time.Second)
	if d, _ := l.LockedFor(ctx, a); d != 0 {
		t.Fatal("still locked after the lockout")
	}
	u := LockoutUser("anna")
	fail(u)
	if err := l.Succeed(ctx, u); err != nil {
		t.Fatal(err)
	}
	var n int
	s.db.QueryRow(`SELECT count(*) FROM auth_failures WHERE key = ?`, u).Scan(&n)
	if n != 0 {
		t.Fatal("success did not forget the failures")
	}
	// Rows outside the window and the lockout are pruned on a write.
	fail(u)
	clock = clock.Add(10 * time.Minute)
	fail(LockoutAddr("10.0.0.6"))
	s.db.QueryRow(`SELECT count(*) FROM auth_failures`).Scan(&n)
	if n != 1 {
		t.Fatalf("%d rows after the prune", n)
	}
}
