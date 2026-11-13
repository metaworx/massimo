// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package store

// The providers' client secrets sealed (docs/store.md section 6, "Sign-in
// through OpenID Connect"; AP Oidc v1.1, Block 9, D20-A): AES-256-GCM
// with a key of its own, secret.key beside the store's file, so that a
// copy or a backup of massimo.db alone does not hold them. The key is
// made with the first secret sealed; a store opened read-only never makes
// it. A secret sealed is "v1:" and the base64 of the nonce and the box,
// the provider's uuid its additional data, so that a box copied to
// another row does not open. Lost, the key leaves each secret to be
// entered again.

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// SecretKeyFileName is the key of the sealed secrets in the data
// directory.
const SecretKeyFileName = "secret.key"

// secretPEM is the PEM block type of the key file, its bytes the 32 of
// the AES-256 key.
const secretPEM = "MASSIMO SECRET KEY"

// sealedPrefix marks a sealed secret, and its version.
const sealedPrefix = "v1:"

// SecretKeyPath returns the key file of the sealed secrets in the data
// directory dir.
func SecretKeyPath(dir string) string { return filepath.Join(dir, SecretKeyFileName) }

// secretKey reads the key, making it where there is none and create is
// true and the store is writable; nil where there is none.
func (s *Store) secretKey(create bool) ([]byte, error) {
	s.secretMu.Lock()
	defer s.secretMu.Unlock()
	if s.secret != nil {
		return s.secret, nil
	}
	p := SecretKeyPath(s.dir)
	b, err := os.ReadFile(p)
	switch {
	case err == nil:
		blk, _ := pem.Decode(b)
		if blk == nil || blk.Type != secretPEM || len(blk.Bytes) != 32 {
			return nil, fmt.Errorf("store: %s: not a PEM %s of 32 bytes", p, secretPEM)
		}
		s.secret = blk.Bytes
		return s.secret, nil
	case !errors.Is(err, fs.ErrNotExist):
		return nil, fmt.Errorf("store: %w", err)
	case !create || s.readOnly:
		return nil, nil
	}
	key := make32()
	f, err := os.OpenFile(p, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return nil, fmt.Errorf("store: %w", err)
	}
	if err := pem.Encode(f, &pem.Block{Type: secretPEM, Bytes: key}); err != nil {
		f.Close()
		os.Remove(p)
		return nil, fmt.Errorf("store: %s: %w", p, err)
	}
	if err := f.Close(); err != nil {
		os.Remove(p)
		return nil, fmt.Errorf("store: %s: %w", p, err)
	}
	s.secret = key
	s.log.Info("secret key created", "path", p)
	return key, nil
}

// make32 is 32 random bytes.
func make32() []byte {
	b := make([]byte, 32)
	rand.Read(b)
	return b
}

// sealSecret seals a secret for the row of the uuid; "" stays "".
func (s *Store) sealSecret(uuid, plain string) (string, error) {
	if plain == "" {
		return "", nil
	}
	key, err := s.secretKey(true)
	if err != nil {
		return "", err
	}
	if key == nil {
		return "", fmt.Errorf("store: no %s to seal with", SecretKeyFileName)
	}
	gcm, err := newGCM(key)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	rand.Read(nonce)
	box := gcm.Seal(nonce, nonce, []byte(plain), []byte(uuid))
	return sealedPrefix + base64.RawStdEncoding.EncodeToString(box), nil
}

// openSecret opens a secret of the row of the uuid as kept: a sealed one
// with the key, a plain one, left from before the sealing, as it is.
// lost is a sealed one that does not open: the key gone, or another's.
func (s *Store) openSecret(uuid, kept string) (plain string, lost bool) {
	box, sealed := strings.CutPrefix(kept, sealedPrefix)
	if !sealed {
		return kept, false
	}
	key, err := s.secretKey(false)
	if err != nil || key == nil {
		return "", true
	}
	gcm, err := newGCM(key)
	if err != nil {
		return "", true
	}
	raw, err := base64.RawStdEncoding.DecodeString(box)
	if err != nil || len(raw) < gcm.NonceSize() {
		return "", true
	}
	out, err := gcm.Open(nil, raw[:gcm.NonceSize()], raw[gcm.NonceSize():], []byte(uuid))
	if err != nil {
		return "", true
	}
	return string(out), false
}

func newGCM(key []byte) (cipher.AEAD, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("store: secret: %w", err)
	}
	return cipher.NewGCM(block)
}

// sealPlainSecrets seals the client secrets a store of v0.15.0-alpha.1
// kept plain, at the open of a writable store; it warns of the sealed
// ones that do not open.
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
	var plain []row
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.id, &r.uuid, &r.key, &r.val); err != nil {
			rows.Close()
			return fmt.Errorf("store: secrets: %w", err)
		}
		if r.val == "" {
			continue
		}
		if !strings.HasPrefix(r.val, sealedPrefix) {
			plain = append(plain, r)
		} else if _, lost := s.openSecret(r.uuid, r.val); lost {
			s.log.Warn("store: a provider's client secret does not open: enter it again", "provider", r.key, "key", SecretKeyPath(s.dir))
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return fmt.Errorf("store: secrets: %w", err)
	}
	for _, r := range plain {
		sealed, err := s.sealSecret(r.uuid, r.val)
		if err != nil {
			return err
		}
		if _, err := s.db.ExecContext(ctx, `UPDATE auth_providers SET client_secret = ? WHERE id = ?`, sealed, r.id); err != nil {
			return fmt.Errorf("store: secrets: %w", err)
		}
		s.log.Info("a provider's client secret sealed", "provider", r.key)
	}
	return nil
}
