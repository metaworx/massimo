// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package store

import (
	"runtime"
	"strings"
	"testing"
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
