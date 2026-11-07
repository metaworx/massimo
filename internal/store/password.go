// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package store

// Passwords are hashed with argon2id (docs/store.md section 6) and
// stored as PHC strings, which carry the parameters, so that a release
// can change them and verify the old hashes meanwhile.

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"

	"golang.org/x/crypto/argon2"
)

// The parameters of a new hash: OWASP's argon2id configuration of
// 19 MiB, two passes and one lane, which a Raspberry Pi 1 computes in
// about 60 % of the time the library's 64 MiB and one pass took, with
// a 16-byte salt and a 32-byte key.
const (
	argonTime    = 2
	argonMemory  = 19 * 1024 // KiB
	argonThreads = 1
	argonKeyLen  = 32
	argonSaltLen = 16
)

// The largest parameters a stored hash may ask for: a row beyond them
// is refused rather than allocated. A release that raises the
// parameters above them raises these first.
const (
	maxArgonMemory  = 256 * 1024 // KiB
	maxArgonTime    = 16
	maxArgonThreads = 16
)

// HashPassword returns the argon2id hash of password as a PHC string.
func HashPassword(password string) (string, error) {
	return hashWith(password, argonTime, argonMemory, argonThreads)
}

// hashWith hashes with the parameters given; HashPassword's, or old
// ones for the tests.
func hashWith(password string, time, memory uint32, threads uint8) (string, error) {
	salt := make([]byte, argonSaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("store: salt: %w", err)
	}
	key := argon2.IDKey([]byte(password), salt, time, memory, threads, argonKeyLen)
	enc := base64.RawStdEncoding
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s", argon2.Version, memory, time, threads, enc.EncodeToString(salt), enc.EncodeToString(key)), nil
}

// phc is a parsed argon2id PHC string.
type phc struct {
	memory, time uint32
	threads      uint8
	salt, key    []byte
}

// parseHash reads a PHC string of argon2id; false for an empty or
// malformed one, or one whose parameters exceed the ceiling.
func parseHash(hash string) (phc, bool) {
	var h phc
	parts := strings.Split(hash, "$")
	if len(parts) != 6 || parts[0] != "" || parts[1] != "argon2id" {
		return h, false
	}
	for _, kv := range strings.Split(parts[3], ",") {
		k, v, ok := strings.Cut(kv, "=")
		if !ok {
			return h, false
		}
		n, err := strconv.ParseUint(v, 10, 32)
		if err != nil {
			return h, false
		}
		switch k {
		case "m":
			h.memory = uint32(n)
		case "t":
			h.time = uint32(n)
		case "p":
			if n > maxArgonThreads {
				return h, false
			}
			h.threads = uint8(n)
		}
	}
	if h.memory == 0 || h.time == 0 || h.threads == 0 || h.memory > maxArgonMemory || h.time > maxArgonTime {
		return h, false
	}
	enc := base64.RawStdEncoding
	var err error
	if h.salt, err = enc.DecodeString(parts[4]); err != nil {
		return h, false
	}
	if h.key, err = enc.DecodeString(parts[5]); err != nil || len(h.key) == 0 {
		return h, false
	}
	return h, true
}

// VerifyPassword reports whether password matches the PHC string hash;
// an empty or malformed hash, or one beyond the ceiling, never matches.
func VerifyPassword(hash, password string) bool {
	h, ok := parseHash(hash)
	if !ok {
		return false
	}
	got := argon2.IDKey([]byte(password), h.salt, h.time, h.memory, h.threads, uint32(len(h.key)))
	return subtle.ConstantTimeCompare(got, h.key) == 1
}

var (
	dummyOnce sync.Once
	dummyHash string
)

// DummyVerify spends the time a password check costs, for a login of a
// user that does not exist or has no password, so that the answer's
// timing does not say which.
func DummyVerify(password string) {
	dummyOnce.Do(func() { dummyHash, _ = HashPassword("massimo") })
	VerifyPassword(dummyHash, password)
}

// ErrBadPassword is the error of a wrong or missing password.
var ErrBadPassword = errors.New("store: wrong password")
