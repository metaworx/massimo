// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package store

// The node identity (docs/server.md, "What version 1 prepares", item 3):
// a UUID in settings as node.id, written by the first migration together
// with tenant.id, and an Ed25519 key pair in node.key next to the
// database, created at the first start. The node's id is the origin of
// every row this node changes; the key will sign what the node sends
// once sync exists.

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"database/sql"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// Settings keys of the identity.
const (
	NodeIDKey   = "node.id"
	TenantIDKey = "tenant.id"
)

// KeyFileName is the node's private key in the data directory.
const KeyFileName = "node.key"

// nodeKey is the node's key pair.
type nodeKey struct {
	private ed25519.PrivateKey
	public  ed25519.PublicKey
}

// seedIdentifiers writes node.id and tenant.id, two UUIDv7, and makes
// the node the origin of the seeded roles; run by the first migration.
func seedIdentifiers(ctx context.Context, s *Store, tx *sql.Tx) error {
	node, tenant := newUUID(), newUUID()
	now := s.nowText()
	for _, kv := range [][2]string{{NodeIDKey, node}, {TenantIDKey, tenant}} {
		v, _ := json.Marshal(kv[1])
		if _, err := tx.ExecContext(ctx, `INSERT INTO settings (key, value, updated_at, origin) VALUES (?, ?, ?, ?)`, kv[0], string(v), now, node); err != nil {
			return err
		}
	}
	_, err := tx.ExecContext(ctx, `UPDATE roles SET origin = ?`, node)
	return err
}

// KeyPath returns the node key's file in the data directory dir.
func KeyPath(dir string) string { return filepath.Join(dir, KeyFileName) }

// loadIdentity reads node.id and the key pair, creating the key file
// (mode 0600) when there is none.
func (s *Store) loadIdentity() error {
	id, err := s.SettingString(context.Background(), NodeIDKey)
	if err != nil {
		return fmt.Errorf("store: node.id: %w", err)
	}
	if id == "" {
		return fmt.Errorf("store: %s has no node.id", s.path)
	}
	s.nodeID = id
	p := KeyPath(s.dir)
	b, err := os.ReadFile(p)
	switch {
	case err == nil:
		k, err := parseKey(b)
		if err != nil {
			return fmt.Errorf("store: %s: %w", p, err)
		}
		s.key = k
		return nil
	case !errors.Is(err, fs.ErrNotExist):
		return fmt.Errorf("store: %w", err)
	}
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return fmt.Errorf("store: node key: %w", err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		return fmt.Errorf("store: node key: %w", err)
	}
	f, err := os.OpenFile(p, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return fmt.Errorf("store: %w", err)
	}
	if err := pem.Encode(f, &pem.Block{Type: "PRIVATE KEY", Bytes: der}); err != nil {
		f.Close()
		os.Remove(p)
		return fmt.Errorf("store: %s: %w", p, err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("store: %s: %w", p, err)
	}
	s.key = nodeKey{private: priv, public: pub}
	s.log.Info("node key created", "path", p, "node", id)
	return nil
}

// parseKey reads a PEM PKCS #8 Ed25519 private key.
func parseKey(b []byte) (nodeKey, error) {
	blk, _ := pem.Decode(b)
	if blk == nil || blk.Type != "PRIVATE KEY" {
		return nodeKey{}, errors.New("not a PEM private key")
	}
	k, err := x509.ParsePKCS8PrivateKey(blk.Bytes)
	if err != nil {
		return nodeKey{}, err
	}
	priv, ok := k.(ed25519.PrivateKey)
	if !ok {
		return nodeKey{}, errors.New("not an Ed25519 key")
	}
	return nodeKey{private: priv, public: priv.Public().(ed25519.PublicKey)}, nil
}

// NodeID returns the node's identifier, node.id.
func (s *Store) NodeID() string { return s.nodeID }

// TenantID returns the identifier of the tenant the file belongs to,
// tenant.id.
func (s *Store) TenantID() (string, error) { return s.SettingString(context.Background(), TenantIDKey) }

// NodePublicKey returns the node's Ed25519 public key; nil on a store
// opened read-only.
func (s *Store) NodePublicKey() ed25519.PublicKey { return s.key.public }

// Sign signs msg with the node's private key.
func (s *Store) Sign(msg []byte) ([]byte, error) {
	if s.key.private == nil {
		return nil, errors.New("store: no node key")
	}
	return ed25519.Sign(s.key.private, msg), nil
}
