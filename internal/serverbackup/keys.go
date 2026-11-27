// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package serverbackup

// The keys of a server backup: the recipients it is encrypted for, which
// the server keeps, and the identities that open it, which it never
// keeps (AP ServerBackup, D4-A and D17-C).

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"filippo.io/age"
	"filippo.io/age/agessh"
	"golang.org/x/crypto/ssh"
)

// KeyKind is the kind of a recipient (Kind is a kind of target).
type KeyKind string

// The kinds of recipient. A file's recipients are all post-quantum or
// all classic, as age requires (CheckKinds).
const (
	// KindPostQuantum is age's hybrid of ML-KEM-768 and X25519,
	// age1pq1…, about 2000 characters: it stays closed to a quantum
	// computer.
	KindPostQuantum KeyKind = "pq"
	// KindX25519 is age's classic X25519 key, age1…, 62 characters.
	KindX25519 KeyKind = "x25519"
	// KindSSH is an OpenSSH Ed25519 public key, ssh-ed25519 …, the
	// owner's own; classic, as X25519 is.
	KindSSH KeyKind = "ssh-ed25519"
)

// PostQuantum reports whether a recipient of the kind is post-quantum.
func (k KeyKind) PostQuantum() bool { return k == KindPostQuantum }

// ErrMixedKinds is CheckKinds' error for post-quantum and classic
// recipients together, which age refuses in one file: a classic one
// beside would open what the post-quantum one keeps closed.
var ErrMixedKinds = errors.New("serverbackup: post-quantum and classic recipients do not go into one file; keep the recipients of one kind")

// Recipient is a public key a backup is encrypted for, with the label
// the owner gives it.
type Recipient struct {
	// Label names the key for the owner, in the manifest too; the
	// caller sets it.
	Label string
	// Line is the key in its canonical form: age1pq1…, age1…, or
	// ssh-ed25519 and the key, without the comment.
	Line string
	Kind KeyKind
	// Fingerprint is "SHA256:" and the unpadded base64 of the SHA-256
	// of the key: for ssh-ed25519 of its wire form, which ssh-keygen -l
	// shows; for age of the line.
	Fingerprint string

	r age.Recipient
}

// ParseRecipient reads a recipient line: age1pq1… (post-quantum),
// age1… (X25519) or ssh-ed25519 … (an OpenSSH line, its comment left
// out). Any other line is refused, an age secret key with a warning that
// it is secret, and the error never repeats the line.
func ParseRecipient(line string) (Recipient, error) {
	line = strings.TrimSpace(line)
	switch {
	case strings.HasPrefix(line, "AGE-SECRET-KEY-"), strings.Contains(line, "PRIVATE KEY"):
		return Recipient{}, errors.New("serverbackup: that is a secret key, not a recipient; give its public key (age1…), and keep the secret key away from the server")
	case strings.HasPrefix(line, "age1pq1"):
		r, err := age.ParseHybridRecipient(line)
		if err != nil {
			return Recipient{}, errors.New("serverbackup: not a valid post-quantum recipient (age1pq1…)")
		}
		s := r.String()
		return Recipient{Line: s, Kind: KindPostQuantum, Fingerprint: fingerprint([]byte(s)), r: r}, nil
	case strings.HasPrefix(line, "age1"):
		r, err := age.ParseX25519Recipient(line)
		if err != nil {
			return Recipient{}, errors.New("serverbackup: not a valid X25519 recipient (age1…)")
		}
		s := r.String()
		return Recipient{Line: s, Kind: KindX25519, Fingerprint: fingerprint([]byte(s)), r: r}, nil
	case strings.HasPrefix(line, "ssh-ed25519 "):
		pk, _, _, _, err := ssh.ParseAuthorizedKey([]byte(line))
		if err != nil || pk.Type() != ssh.KeyAlgoED25519 {
			return Recipient{}, errors.New("serverbackup: not a valid ssh-ed25519 line")
		}
		r, err := agessh.NewEd25519Recipient(pk)
		if err != nil {
			return Recipient{}, fmt.Errorf("serverbackup: the ssh-ed25519 key: %w", err)
		}
		s := strings.TrimSpace(string(ssh.MarshalAuthorizedKey(pk)))
		return Recipient{Line: s, Kind: KindSSH, Fingerprint: ssh.FingerprintSHA256(pk), r: r}, nil
	case strings.HasPrefix(line, "ssh-"), strings.HasPrefix(line, "ecdsa-"), strings.HasPrefix(line, "sk-"):
		return Recipient{}, errors.New("serverbackup: of the SSH keys only ssh-ed25519 is taken")
	}
	return Recipient{}, errors.New("serverbackup: not a recipient; give an age1pq1…, age1… or ssh-ed25519 line")
}

// CheckKinds refuses no recipient at all, a recipient not parsed by
// ParseRecipient, and a mix of post-quantum and classic recipients
// (ErrMixedKinds).
func CheckKinds(rs []Recipient) error {
	if len(rs) == 0 {
		return errors.New("serverbackup: no recipient; a backup needs at least one key to be encrypted for")
	}
	for _, r := range rs {
		if r.r == nil {
			return errors.New("serverbackup: a recipient not read by ParseRecipient")
		}
		if r.Kind.PostQuantum() != rs[0].Kind.PostQuantum() {
			return ErrMixedKinds
		}
	}
	return nil
}

// fingerprint is "SHA256:" and the unpadded base64 of b's SHA-256, as
// ssh-keygen -l writes one.
func fingerprint(b []byte) string {
	sum := sha256.Sum256(b)
	return "SHA256:" + base64.RawStdEncoding.EncodeToString(sum[:])
}

// Key is a key GenerateKey made: its secret identity, to be kept by the
// owner and never by the server, and its recipient.
type Key struct {
	// Identity is the secret key, AGE-SECRET-KEY-PQ-1… or
	// AGE-SECRET-KEY-1…; never logged.
	Identity  string
	Recipient Recipient
}

// GenerateKey makes a post-quantum (KindPostQuantum) or a classic X25519
// (KindX25519) identity; no other kind.
func GenerateKey(kind KeyKind) (Key, error) {
	var id, rec string
	switch kind {
	case KindPostQuantum:
		k, err := age.GenerateHybridIdentity()
		if err != nil {
			return Key{}, fmt.Errorf("serverbackup: %w", err)
		}
		id, rec = k.String(), k.Recipient().String()
	case KindX25519:
		k, err := age.GenerateX25519Identity()
		if err != nil {
			return Key{}, fmt.Errorf("serverbackup: %w", err)
		}
		id, rec = k.String(), k.Recipient().String()
	default:
		return Key{}, fmt.Errorf("serverbackup: a key of kind %q is not made here; %q or %q", kind, KindPostQuantum, KindX25519)
	}
	r, err := ParseRecipient(rec)
	if err != nil {
		return Key{}, err
	}
	return Key{Identity: id, Recipient: r}, nil
}

// File is the key as age-keygen writes it: two comment lines, the time
// and the public key, then the secret key; age -d -i reads it.
func (k Key) File(now time.Time) []byte {
	return fmt.Appendf(nil, "# created: %s\n# public key: %s\n%s\n", now.Format(time.RFC3339), k.Recipient.Line, k.Identity)
}

// maxIdentities bounds a key file ParseIdentities reads.
const maxIdentities = 1 << 20

// ParseIdentities reads the secret keys that open a backup: an age key
// file (AGE-SECRET-KEY-PQ-1… and AGE-SECRET-KEY-1… lines, "#" comments,
// as age-keygen and GenerateKey write it) or an OpenSSH Ed25519 private
// key without a passphrase. The error never repeats the key.
func ParseIdentities(b []byte) ([]age.Identity, error) {
	if len(b) > maxIdentities {
		return nil, errors.New("serverbackup: the key file is too long")
	}
	if !utf8.Valid(b) {
		return nil, errors.New("serverbackup: the key file is not text")
	}
	if bytes.Contains(b, []byte("-----BEGIN OPENSSH PRIVATE KEY-----")) {
		id, err := agessh.ParseIdentity(b)
		var missing *ssh.PassphraseMissingError
		switch {
		case errors.As(err, &missing):
			return nil, errors.New("serverbackup: the SSH key has a passphrase; give an unprotected copy, or open the file with age -d -i, which asks for it")
		case err != nil:
			return nil, fmt.Errorf("serverbackup: the SSH key: %w", err)
		}
		if _, ok := id.(*agessh.Ed25519Identity); !ok {
			return nil, errors.New("serverbackup: of the SSH keys only ssh-ed25519 is taken")
		}
		return []age.Identity{id}, nil
	}
	ids, err := age.ParseIdentities(bytes.NewReader(b))
	if err != nil {
		return nil, fmt.Errorf("serverbackup: the key file: %w", err)
	}
	return ids, nil
}
