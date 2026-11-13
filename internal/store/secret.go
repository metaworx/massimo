// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package store

// The secrets kept in massimo.db, sealed (docs/store.md section 6, "Sign-in
// through OpenID Connect"; AP Oidc v1.1, Block 9; AP SealedSecrets v1.0):
// AES-256-GCM with a key derived from opaque.key, so that a copy or a
// backup of massimo.db alone does not hold them. A secret sealed is "v2:"
// and the base64 of the nonce and the box, with additional data of its
// own (a provider's uuid), so that a box copied elsewhere does not open.
// opaque.key lost, or another store's, leaves each secret to be entered
// again, as it leaves each password to be set again.
//
// Before AP SealedSecrets, a build of main sealed with a key file of its
// own, secret.key, as "v1:": such a secret is opened with it once and
// sealed again, and the file then renamed secret.key.retired.

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hkdf"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// SecretKeyFileName is the key file a build of main sealed with before
// AP SealedSecrets, in the data directory; retiredSuffix is added to its
// name once nothing sealed with it is left.
const (
	SecretKeyFileName = "secret.key"
	retiredSuffix     = ".retired"
)

// secretPEM is the PEM block type of secret.key, its bytes the 32 of the
// AES-256 key.
const secretPEM = "MASSIMO SECRET KEY"

// The prefixes of a sealed secret: the version, which says the key.
const (
	sealedPrefix = "v2:" // the key derived from opaque.key
	legacyPrefix = "v1:" // secret.key
)

// sealInfo is the HKDF info of the sealing key, which keeps it apart from
// the OPAQUE keys it is derived from.
const sealInfo = "massimo sealed secrets v2"

// SecretKeyPath returns the retired key file of the sealed secrets in the
// data directory dir.
func SecretKeyPath(dir string) string { return filepath.Join(dir, SecretKeyFileName) }

// deriveSealKey derives the sealing key from the bytes of opaque.key.
func deriveSealKey(opaqueKey []byte) ([]byte, error) {
	key, err := hkdf.Key(sha256.New, opaqueKey, nil, sealInfo, 32)
	if err != nil {
		return nil, fmt.Errorf("store: the sealing key: %w", err)
	}
	return key, nil
}

// sealKey is the sealing key, derived when opaque.key was read; nil in a
// store opened read-only that could not read it.
func (s *Store) sealKey() []byte {
	s.secretMu.Lock()
	defer s.secretMu.Unlock()
	return s.secret
}

// legacyKey reads secret.key, where a build of main made one; nil where
// there is none.
func (s *Store) legacyKey() ([]byte, error) {
	s.secretMu.Lock()
	defer s.secretMu.Unlock()
	if s.legacy != nil {
		return s.legacy, nil
	}
	p := SecretKeyPath(s.dir)
	b, err := os.ReadFile(p)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return nil, nil
	case err != nil:
		return nil, fmt.Errorf("store: %w", err)
	}
	blk, _ := pem.Decode(b)
	if blk == nil || blk.Type != secretPEM || len(blk.Bytes) != 32 {
		return nil, fmt.Errorf("store: %s: not a PEM %s of 32 bytes", p, secretPEM)
	}
	s.legacy = blk.Bytes
	return s.legacy, nil
}

// sealSecret seals a secret with its additional data aad; "" stays "".
func (s *Store) sealSecret(aad, plain string) (string, error) {
	if plain == "" {
		return "", nil
	}
	key := s.sealKey()
	if key == nil {
		return "", errors.New("store: no opaque.key to seal with")
	}
	box, err := sealBox(key, aad, plain)
	if err != nil {
		return "", err
	}
	return sealedPrefix + box, nil
}

// openSecret opens a secret as kept: a sealed one with its key, a plain
// one, left from before the sealing, as it is. lost is a sealed one that
// does not open: its key gone, or another store's.
func (s *Store) openSecret(aad, kept string) (plain string, lost bool) {
	var key []byte
	box, sealed := strings.CutPrefix(kept, sealedPrefix)
	if sealed {
		key = s.sealKey()
	} else if box, sealed = strings.CutPrefix(kept, legacyPrefix); sealed {
		key, _ = s.legacyKey()
	} else {
		return kept, false
	}
	if key == nil {
		return "", true
	}
	out, err := openBox(key, aad, box)
	if err != nil {
		return "", true
	}
	return out, false
}

// sealBox is the base64 of a nonce and the AES-256-GCM box of plain.
func sealBox(key []byte, aad, plain string) (string, error) {
	gcm, err := newGCM(key)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	rand.Read(nonce)
	return base64.RawStdEncoding.EncodeToString(gcm.Seal(nonce, nonce, []byte(plain), []byte(aad))), nil
}

// openBox opens what sealBox made.
func openBox(key []byte, aad, box string) (string, error) {
	gcm, err := newGCM(key)
	if err != nil {
		return "", err
	}
	raw, err := base64.RawStdEncoding.DecodeString(box)
	if err != nil || len(raw) < gcm.NonceSize() {
		return "", errors.New("store: not a sealed box")
	}
	out, err := gcm.Open(nil, raw[:gcm.NonceSize()], raw[gcm.NonceSize():], []byte(aad))
	if err != nil {
		return "", err
	}
	return string(out), nil
}

func newGCM(key []byte) (cipher.AEAD, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("store: secret: %w", err)
	}
	return cipher.NewGCM(block)
}

// sealPlainSecrets seals, at the open of a writable store, the client
// secrets kept plain (v0.15.0-alpha.1) or with secret.key (a build of
// main), and warns of the sealed ones that do not open. Once every one
// sealed with secret.key is sealed again, the file is renamed
// secret.key.retired, for the owner to delete.
func (s *Store) sealPlainSecrets(ctx context.Context) error {
	var tables int
	if err := s.db.QueryRowContext(ctx, `SELECT count(*) FROM sqlite_master WHERE type = 'table' AND name = 'auth_providers'`).Scan(&tables); err != nil {
		return fmt.Errorf("store: secrets: %w", err)
	}
	if tables == 0 {
		return nil // a store migrated short of 0016, in the tests
	}
	// every live provider, the secrets compared here: no filter on a
	// content column (TestNoFilterOnContent)
	rows, err := s.db.QueryContext(ctx, `SELECT id, uuid, key, client_secret FROM auth_providers WHERE deleted_at IS NULL`)
	if err != nil {
		return fmt.Errorf("store: secrets: %w", err)
	}
	type row struct {
		id             int64
		uuid, key, val string
	}
	var again []row
	unopened := 0
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.id, &r.uuid, &r.key, &r.val); err != nil {
			rows.Close()
			return fmt.Errorf("store: secrets: %w", err)
		}
		if r.val == "" {
			continue
		}
		plain, lost := s.openSecret(r.uuid, r.val)
		switch {
		case lost:
			if strings.HasPrefix(r.val, legacyPrefix) {
				unopened++
			}
			s.log.Warn("store: a provider's client secret does not open: enter it again", "provider", r.key)
		case !strings.HasPrefix(r.val, sealedPrefix):
			r.val = plain
			again = append(again, r)
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return fmt.Errorf("store: secrets: %w", err)
	}
	for _, r := range again {
		sealed, err := s.sealSecret(r.uuid, r.val)
		if err != nil {
			return err
		}
		if _, err := s.db.ExecContext(ctx, `UPDATE auth_providers SET client_secret = ? WHERE id = ?`, sealed, r.id); err != nil {
			return fmt.Errorf("store: secrets: %w", err)
		}
		s.log.Info("a provider's client secret sealed", "provider", r.key)
	}
	return s.retireLegacyKey(unopened)
}

// retireLegacyKey renames secret.key to secret.key.retired where nothing
// sealed with it is left unopened, and logs it.
func (s *Store) retireLegacyKey(unopened int) error {
	p := SecretKeyPath(s.dir)
	if _, err := os.Stat(p); err != nil || unopened > 0 {
		return nil
	}
	if err := os.Rename(p, p+retiredSuffix); err != nil {
		return fmt.Errorf("store: %w", err)
	}
	s.log.Info("secret.key retired: the secrets are sealed with a key derived from opaque.key; delete the file", "path", p+retiredSuffix)
	return nil
}
