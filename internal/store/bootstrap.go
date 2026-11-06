// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package store

// Bootstrap and compatibility (docs/store.md section 6): the API token
// of the settings file becomes an owner's API key, so that every client
// that sends it today keeps working with full rights, and the file stays
// the way to recover an owner who lost the password.

import (
	"context"
	"fmt"
)

// BootstrapLabel is the label of the token MASSIMO_API_TOKEN becomes.
const BootstrapLabel = "MASSIMO_API_TOKEN"

// BootstrapUser is the name of the owner the first start creates.
const BootstrapUser = "owner"

// BootstrapResult says what Bootstrap did.
type BootstrapResult struct {
	Users        int  // users after the call, removed ones not counted
	OwnerCreated bool // the user owner was created from the token
	TokenAdded   bool // the token was added to an owner's keys
	Revoked      int  // tokens with the label whose secret is not the token any more
}

// Bootstrap applies MASSIMO_API_TOKEN (token, "" for none) to the
// store at a start: with no user and a token, the user owner is created
// with the token as its API key; with users, a token whose hash is
// unknown is added to the first enabled owner, and a key with the
// token's label that is not the token any more is revoked.
func (s *Store) Bootstrap(ctx context.Context, token string) (BootstrapResult, error) {
	var res BootstrapResult
	if err := s.writable(); err != nil {
		return res, err
	}
	n, err := s.CountUsers(ctx)
	if err != nil {
		return res, err
	}
	res.Users = n
	hash := ""
	if token != "" {
		hash = HashToken(token)
	}
	if n == 0 {
		if token == "" {
			return res, nil
		}
		u, err := s.AddUser(ctx, User{Name: BootstrapUser, Role: RoleOwner}, "", 0)
		if err != nil {
			return res, err
		}
		if _, err := s.AddTokenHash(ctx, u.ID, TokenAPI, BootstrapLabel, hash, zeroTime); err != nil {
			return res, err
		}
		s.log.Info("owner created from MASSIMO_API_TOKEN", "user", u.Name)
		res.Users, res.OwnerCreated, res.TokenAdded = 1, true, true
		return res, nil
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id, hash, label FROM tokens WHERE kind = ? AND revoked_at IS NULL`, TokenAPI)
	if err != nil {
		return res, fmt.Errorf("store: bootstrap: %w", err)
	}
	type key struct {
		id          int64
		hash, label string
	}
	var keys []key
	for rows.Next() {
		var k key
		if err := rows.Scan(&k.id, &k.hash, &k.label); err != nil {
			rows.Close()
			return res, fmt.Errorf("store: bootstrap: %w", err)
		}
		keys = append(keys, k)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return res, fmt.Errorf("store: bootstrap: %w", err)
	}
	known := false
	for _, k := range keys {
		// The label is a content column, so it is read here, not in SQL.
		if k.label == BootstrapLabel && k.hash != hash {
			if err := s.RevokeToken(ctx, k.id); err != nil {
				return res, err
			}
			res.Revoked++
			s.log.Info("api token revoked: not MASSIMO_API_TOKEN any more", "token", k.id)
			continue
		}
		if hash != "" && k.hash == hash {
			known = true
		}
	}
	if hash == "" || known {
		return res, nil
	}
	var owner int64
	err = s.db.QueryRowContext(ctx, `SELECT u.id FROM users u JOIN roles r ON r.id = u.role_id WHERE r.key = ? AND u.disabled = 0 AND u.deleted_at IS NULL ORDER BY u.id LIMIT 1`, RoleOwner).Scan(&owner)
	if err != nil {
		s.log.Warn("MASSIMO_API_TOKEN is set but no enabled owner can hold it", "error", err)
		return res, nil
	}
	if _, err := s.AddTokenHash(ctx, owner, TokenAPI, BootstrapLabel, hash, zeroTime); err != nil {
		return res, err
	}
	s.log.Info("api token added to the owner from MASSIMO_API_TOKEN", "user", owner)
	res.TokenAdded = true
	return res, nil
}
