// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package store

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"runtime/metrics"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// The parameters of the first releases, 64 MiB and one pass on four
// lanes: their hashes are in the stores and still verify.
const (
	oldArgonTime    = 1
	oldArgonMemory  = 64 * 1024
	oldArgonThreads = 4
)

// A hash of the old parameters verifies; one asking for more than the
// ceiling is refused without allocating it.
func TestPasswordParameters(t *testing.T) {
	old, err := hashWith("correct horse", oldArgonTime, oldArgonMemory, oldArgonThreads)
	if err != nil || !strings.HasPrefix(old, "$argon2id$v=19$m=65536,t=1,p=4$") {
		t.Fatalf("old hash %q %v", old, err)
	}
	if !VerifyPassword(old, "correct horse") || VerifyPassword(old, "wrong") {
		t.Fatal("the old hash does not verify")
	}
	_, saltKey, _ := strings.Cut(old, "$m=65536,t=1,p=4$")
	for _, params := range []string{"m=262145,t=1,p=1", "m=19456,t=17,p=1", "m=19456,t=2,p=17", "m=0,t=2,p=1", "m=19456,t=2", "m=4294967295,t=1,p=1"} {
		h := "$argon2id$v=19$" + params + "$" + saltKey
		before := allocated()
		if VerifyPassword(h, "correct horse") {
			t.Errorf("%s verifies", params)
		}
		if n := allocated() - before; n > 1<<20 {
			t.Errorf("%s allocated %d bytes", params, n)
		}
	}
}

// One hash allocates the 19 MiB of its memory parameter and little more.
func TestPasswordMemory(t *testing.T) {
	HashPassword("warm up")
	before := allocated()
	if _, err := HashPassword("correct horse"); err != nil {
		t.Fatal(err)
	}
	if n := allocated() - before; n < 18<<20 || n > 21<<20 {
		t.Fatalf("a hash allocated %.1f MiB, want about 19", float64(n)/(1<<20))
	}
}

// allocated is the bytes allocated so far by the process.
func allocated() uint64 {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return m.TotalAlloc
}

// The cost of a hash and of a check, with the parameters of now and
// of the first releases; -benchmem shows the memory each takes.
func BenchmarkPassword(b *testing.B) {
	old, _ := hashWith("correct horse", oldArgonTime, oldArgonMemory, oldArgonThreads)
	now, _ := HashPassword("correct horse")
	for _, c := range []struct {
		name string
		run  func()
	}{
		{"hash", func() { HashPassword("correct horse") }},
		{"verify", func() { VerifyPassword(now, "correct horse") }},
		{"verify-old", func() { VerifyPassword(old, "correct horse") }},
	} {
		b.Run(c.name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				c.run()
			}
		})
	}
}

// The gate hands out its slots, queues as many callers as it has room
// for, refuses the next with ErrBusy, and lets a waiter go when its
// context ends.
func TestHashGate(t *testing.T) {
	g := newHashGate(1, 1)
	ctx := t.Context()
	leave, err := g.enter(ctx)
	if err != nil {
		t.Fatal(err)
	}
	got := make(chan error, 1)
	go func() {
		l, err := g.enter(ctx)
		if err == nil {
			l()
		}
		got <- err
	}()
	for g.waiting.Load() != 1 {
		time.Sleep(time.Millisecond)
	}
	if _, err := g.enter(ctx); !errors.Is(err, ErrBusy) {
		t.Fatalf("a full queue: %v, want ErrBusy", err)
	}
	leave()
	if err := <-got; err != nil {
		t.Fatalf("the waiter: %v", err)
	}

	leave, _ = g.enter(ctx)
	defer leave()
	ended, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := g.enter(ended); !errors.Is(err, context.Canceled) {
		t.Fatalf("an ended context: %v", err)
	}
	if n := g.waiting.Load(); n != 0 {
		t.Fatalf("%d still waiting", n)
	}
}

// Eight password checks at once on a store whose gate has two slots
// compute at most two hashes at a time, and all of them succeed.
func TestPasswordChecksBounded(t *testing.T) {
	s := open(t, t.TempDir())
	ctx := t.Context()
	if _, err := s.AddUser(ctx, User{Name: "anna", Role: RoleViewer}, "correct horse", 0); err != nil {
		t.Fatal(err)
	}
	s.hashes = newHashGate(2, 16)
	var running, most atomic.Int32
	real := argonKey
	argonKey = func(password, salt []byte, time, memory uint32, threads uint8, keyLen uint32) []byte {
		n := running.Add(1)
		defer running.Add(-1)
		for m := most.Load(); n > m && !most.CompareAndSwap(m, n); m = most.Load() {
		}
		return real(password, salt, time, memory, threads, keyLen)
	}
	t.Cleanup(func() { argonKey = real })
	var wg sync.WaitGroup
	errs := make(chan error, 8)
	for range 8 {
		wg.Go(func() { errs <- s.CheckPassword(ctx, "anna", "correct horse") })
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	if m := most.Load(); m < 1 || m > 2 {
		t.Fatalf("%d hashes at once, want at most 2", m)
	}
}

// The peak heap of eight password checks sent at once, with as many
// slots as the gate of a store on one core, on four cores, and none
// to speak of (eight).
func BenchmarkPasswordChecksAtOnce(b *testing.B) {
	for _, slots := range []int{1, 4, 8} {
		b.Run(fmt.Sprintf("slots=%d", slots), func(b *testing.B) {
			s, err := Open(b.TempDir(), Options{Version: "test", Logger: quiet()})
			if err != nil {
				b.Fatal(err)
			}
			defer s.Close()
			ctx := context.Background()
			s.AddUser(ctx, User{Name: "anna", Role: RoleViewer}, "correct horse", 0)
			s.hashes = newHashGate(slots, 8)
			var peak uint64
			for b.Loop() {
				runtime.GC()
				base := heapObjects()
				stop := make(chan struct{})
				sampled := make(chan uint64)
				go func() {
					var most uint64
					for {
						if h := heapObjects(); h > most {
							most = h
						}
						select {
						case <-stop:
							sampled <- most
							return
						case <-time.After(time.Millisecond):
						}
					}
				}()
				var wg sync.WaitGroup
				for range 8 {
					wg.Go(func() { s.CheckPassword(ctx, "anna", "correct horse") })
				}
				wg.Wait()
				close(stop)
				if most := <-sampled; most > base && most-base > peak {
					peak = most - base
				}
			}
			b.ReportMetric(float64(peak)/(1<<20), "peak-MiB")
		})
	}
}

// heapObjects is the bytes of live and unswept heap objects.
func heapObjects() uint64 {
	m := []metrics.Sample{{Name: "/memory/classes/heap/objects:bytes"}}
	metrics.Read(m)
	return m[0].Value.Uint64()
}

// A sign-in against a hash of the old parameters replaces it by one of
// the current ones, without counting as an edit of the user; a wrong
// password, a current hash, a hash changed meanwhile and a read-only
// store keep the hash they have.
func TestRehashAtSignIn(t *testing.T) {
	dir := t.TempDir()
	s := open(t, dir)
	ctx := t.Context()
	anna, err := s.AddUser(ctx, User{Name: "anna", Role: RoleViewer}, "", 0)
	if err != nil {
		t.Fatal(err)
	}
	stored := func() string {
		var h string
		s.db.QueryRowContext(ctx, `SELECT password_hash FROM users WHERE id = ?`, anna.ID).Scan(&h)
		return h
	}
	old, _ := hashWith("correct horse", oldArgonTime, oldArgonMemory, oldArgonThreads)
	if _, err := s.db.ExecContext(ctx, `UPDATE users SET password_hash = ? WHERE id = ?`, old, anna.ID); err != nil {
		t.Fatal(err)
	}
	before, _, _ := s.User(ctx, "anna")

	if err := s.CheckPassword(ctx, "anna", "wrong"); !errors.Is(err, ErrBadPassword) || stored() != old {
		t.Fatalf("a wrong password: %v, hash changed %v", err, stored() != old)
	}
	ro, err := OpenReadOnly(dir, Options{Version: "test", Logger: quiet()})
	if err != nil {
		t.Fatal(err)
	}
	defer ro.Close()
	if err := ro.CheckPassword(ctx, "anna", "correct horse"); err != nil || stored() != old {
		t.Fatalf("a read-only store: %v, hash changed %v", err, stored() != old)
	}

	if err := s.CheckPassword(ctx, "anna", "correct horse"); err != nil {
		t.Fatal(err)
	}
	fresh := stored()
	if !strings.HasPrefix(fresh, "$argon2id$v=19$m=19456,t=2,p=1$") || !VerifyPassword(fresh, "correct horse") || stale(fresh) {
		t.Fatalf("after the sign-in: %q", fresh)
	}
	if after, _, _ := s.User(ctx, "anna"); after.Version != before.Version || !after.UpdatedAt.Equal(before.UpdatedAt) || after.AuthVersion != 1 {
		t.Fatalf("the rehash is an edit: %+v, before %+v", after, before)
	}
	if err := s.CheckPassword(ctx, "anna", "correct horse"); err != nil || stored() != fresh {
		t.Fatalf("a current hash: %v, rehashed %v", err, stored() != fresh)
	}
	s.rehash(ctx, anna.ID, old, "correct horse")
	if stored() != fresh {
		t.Fatal("a hash changed meanwhile was overwritten")
	}
}

// A user add of a taken name or an unknown role, and a password set for
// a user that is not there, answer without a hash; a removed user's
// name is taken over with one.
func TestNoHashForAKnownAnswer(t *testing.T) {
	s := open(t, t.TempDir())
	ctx := t.Context()
	if _, err := s.AddUser(ctx, User{Name: "anna", Role: RoleViewer}, "pw", 0); err != nil {
		t.Fatal(err)
	}
	s.AddUser(ctx, User{Name: "gone", Role: RoleViewer}, "", 0)
	s.DeleteUser(ctx, "gone", 0)
	var hashes atomic.Int32
	real := argonKey
	argonKey = func(password, salt []byte, time, memory uint32, threads uint8, keyLen uint32) []byte {
		hashes.Add(1)
		return real(password, salt, time, memory, threads, keyLen)
	}
	t.Cleanup(func() { argonKey = real })
	if _, err := s.AddUser(ctx, User{Name: " Anna ", Role: RoleViewer}, "pw", 0); !errors.Is(err, ErrUserExists) {
		t.Fatalf("a taken name: %v", err)
	}
	if _, err := s.AddUser(ctx, User{Name: "bert", Role: "nosuch"}, "pw", 0); !errors.Is(err, ErrNoRole) {
		t.Fatalf("an unknown role: %v", err)
	}
	if err := s.SetPassword(ctx, "nobody", "pw", 0); !errors.Is(err, ErrNoUser) {
		t.Fatalf("a password for nobody: %v", err)
	}
	if err := s.SetPassword(ctx, "gone", "pw", 0); !errors.Is(err, ErrNoUser) {
		t.Fatalf("a password for a removed user: %v", err)
	}
	if n := hashes.Load(); n != 0 {
		t.Fatalf("%d hashes for answers that need none", n)
	}
	if _, err := s.AddUser(ctx, User{Name: "gone", Role: RoleViewer}, "pw", 0); err != nil || hashes.Load() != 1 {
		t.Fatalf("a removed user's name: %v, %d hashes", err, hashes.Load())
	}
}

// A stored string of another argon2 version, which this code would
// compute as version 19 and so never match, or with a salt shorter than
// the PHC format's minimum, is refused as malformed.
func TestParseHashRefusesOtherVersionsAndShortSalts(t *testing.T) {
	h, err := HashPassword("secret")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := parseHash(h); !ok {
		t.Fatalf("a hash of HashPassword refused: %s", h)
	}
	key := "a2V5a2V5a2V5a2V5a2V5a2V5a2V5a2V5a2V5a2U"
	for _, s := range []string{
		"$argon2id$v=16$m=19456,t=2,p=1$c2FsdHNhbHRzYWx0c2FsdA$" + key,
		"$argon2id$$m=19456,t=2,p=1$c2FsdHNhbHRzYWx0c2FsdA$" + key,
		"$argon2id$v=19$m=19456,t=2,p=1$$" + key,
		"$argon2id$v=19$m=19456,t=2,p=1$c2FsdA$" + key,
	} {
		if _, ok := parseHash(s); ok {
			t.Errorf("accepted: %s", s)
		}
	}
}
