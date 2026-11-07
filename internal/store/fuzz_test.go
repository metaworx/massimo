// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package store

// The store's parsers of what a row or a request holds: a PHC string from
// users.password_hash, and the key stretching a client sends with its
// OPAQUE record. Neither may panic, and what they accept stays within the
// ceiling, so that a row cannot make a sign-in allocate more than it.
// `go test` runs the seeds; `make fuzz` runs each target for FUZZTIME.

import (
	"testing"
)

func FuzzParseHash(f *testing.F) {
	h, err := HashPassword("secret")
	if err != nil {
		f.Fatal(err)
	}
	f.Add(h)
	f.Add("$argon2id$v=19$m=65536,t=1,p=4$c2FsdHNhbHRzYWx0c2FsdA$a2V5a2V5a2V5a2V5a2V5a2V5a2V5a2V5a2V5a2U")
	f.Add("$argon2id$v=19$m=4294967295,t=1,p=1$AAAA$AAAA")
	f.Add("$argon2i$v=19$m=1,t=1,p=1$$")
	f.Add("")
	f.Fuzz(func(t *testing.T, s string) {
		p, ok := parseHash(s)
		if !ok {
			return
		}
		if p.memory > maxArgonMemory || p.time > maxArgonTime || uint32(p.threads) > maxArgonThreads || p.time == 0 || p.threads == 0 {
			t.Fatalf("%q accepted with m=%d t=%d p=%d", s, p.memory, p.time, p.threads)
		}
		if len(p.key) == 0 || len(p.salt) < minSaltLen {
			t.Fatalf("%q accepted with a salt of %d bytes and a key of %d", s, len(p.salt), len(p.key))
		}
	})
}

func FuzzParseKSF(f *testing.F) {
	for _, s := range []string{OpaqueKSF, "m=12288,t=4,p=1", "m=1,t=1,p=1", "m=524288,t=2,p=1", "t=2,m=19456,p=1", "m=19456,t=2,p=0", "m=,t=,p=", ""} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		m, tm, p, err := ParseKSF(s)
		if err != nil {
			return
		}
		if m > maxArgonMemory || tm > maxArgonTime || uint32(p) > maxArgonThreads || uint64(m)*uint64(tm) < argonMemory*argonTime {
			t.Fatalf("%q accepted as m=%d t=%d p=%d", s, m, tm, p)
		}
		if m2, t2, p2, err := ParseKSF(formatKSF(m, tm, p)); err != nil || m2 != m || t2 != tm || p2 != p {
			t.Fatalf("%q does not come back from %q (%v)", s, formatKSF(m, tm, p), err)
		}
	})
}
