// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package store

// Bootstrap and compatibility (docs/store.md section 6): the API token
// of the settings file becomes an owner's API key, so that every client
// that sends it today keeps working with full rights, and the file stays
// the way to recover an owner who lost the password.

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// BootstrapLabel is the label of the token MASSIMO_API_TOKEN becomes.
const BootstrapLabel = "MASSIMO_API_TOKEN"

// BootstrapUser is the name of the owner the first start creates.
const BootstrapUser = "owner"

// BootstrapResult says what Bootstrap did.
type BootstrapResult struct {
	Users        int  // regular users after the call (CountUsers): removed ones and an integration's not counted
	OwnerCreated bool // the user owner was created from the token
	TokenAdded   bool // the token was added to an owner's keys
	Revoked      int  // tokens with the label whose secret is not the token any more
	Moved        bool // the token's key was taken from an integration's user (heldByIntegration)
	Heirs        int  // the keys the revoked tokens made, and theirs, revoked with them (AP KeyLife, D8-A)
}

// Bootstrap applies MASSIMO_API_TOKEN (token, "" for none) to the
// store at a start: with no regular user (CountUsers, an integration's
// users beside it or not) and a token, the user owner is created with
// the token as its API key; with regular users, a token whose hash is
// unknown is added to the first enabled owner that is not an
// integration's, a known one loses its maker, and a key with the token's
// label that is not the token any more is revoked, with the keys it made
// (AP KeyLife, D8-A); the store keeps the token's hash in memory, so
// that no revoke takes that key as an heir (revokeHeirs) and the idle
// limit passes over it (lifeRule, AP KeyLife, D3-A). The
// token's key that an older start gave to an integration's user with the
// role owner moves to that owner, the one made or the first regular one
// (AP LooseEnds, D1): such a user holds no key of its own, and the
// file's token is the owner's way in.
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
	s.fileHash.Store(&hash)
	held, holder, err := s.heldByIntegration(ctx, hash)
	if err != nil {
		return res, err
	}
	if n == 0 {
		if token == "" {
			return res, nil
		}
		// The owner and its key in one transaction: a key that cannot be
		// given leaves no owner without one.
		u, err := s.newUser(User{Name: BootstrapUser, Role: RoleOwner})
		if err != nil {
			return res, err
		}
		u, err = s.addUser(ctx, u, credential{}, 0, false, func(tx *sql.Tx, id int64) error {
			return s.keyIn(ctx, tx, held, id, hash)
		})
		if err != nil {
			return res, err
		}
		s.keyMoved(held, holder, u.Name)
		s.log.Info("owner created from MASSIMO_API_TOKEN", "user", u.Name)
		res.Users, res.OwnerCreated, res.TokenAdded, res.Moved = 1, true, true, held != 0
		return res, nil
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id, hash, label FROM tokens WHERE kind = ? AND deleted_at IS NULL`, TokenAPI)
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
		if hash != "" && k.hash == hash && k.id != held {
			known = true
			// A key the bootstrap takes in has no maker (AP KeyLife, D5-A),
			// so that the revoke of the file's old key below, which may
			// have made it, leaves it; and no end date (D3-A), which the
			// rule passes over for it all the same (alive).
			if _, err := s.db.ExecContext(ctx, `UPDATE tokens SET made_by = NULL, expires_at = NULL, origin = ?, version = version + 1 WHERE id = ? AND (made_by IS NOT NULL OR expires_at IS NOT NULL)`, s.nodeID, k.id); err != nil {
				return res, fmt.Errorf("store: bootstrap: %w", err)
			}
		}
	}
	for _, k := range keys {
		// The label is a content column, so it is read here, not in SQL.
		if k.label == BootstrapLabel && k.hash != hash {
			// A revoke, so the keys it made go with it (AP KeyLife, D8-A):
			// the file is how the owner ends a token that may have leaked.
			heirs, err := s.RevokeToken(ctx, k.id)
			if errors.Is(err, ErrNoToken) {
				continue // an heir of a key revoked before it in this loop
			}
			if err != nil {
				return res, err
			}
			res.Revoked++
			res.Heirs += len(heirs)
			s.log.Info("api token revoked: not MASSIMO_API_TOKEN any more", "token", k.id, "heirs", len(heirs))
		}
	}
	if hash == "" || known {
		return res, nil
	}
	var owner int64
	// A regular owner: an integration's user with the role owner holds
	// no key (ErrIntegrationUser).
	err = s.db.QueryRowContext(ctx, `SELECT u.id FROM users u JOIN roles r ON r.id = u.role_id WHERE r.key = ? AND u.disabled = 0
		AND u.id IN (SELECT id FROM users WHERE `+regularUser+`) ORDER BY u.id LIMIT 1`, RoleOwner).Scan(&owner)
	if err != nil {
		s.log.Warn("MASSIMO_API_TOKEN is set but no enabled owner can hold it", "error", err)
		return res, nil
	}
	u, _, err := s.UserByID(ctx, owner)
	if err != nil {
		return res, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return res, fmt.Errorf("store: bootstrap: %w", err)
	}
	defer tx.Rollback()
	if err := s.keyIn(ctx, tx, held, u.ID, hash); err != nil {
		return res, err
	}
	if err := tx.Commit(); err != nil {
		return res, fmt.Errorf("store: bootstrap: %w", err)
	}
	s.keyMoved(held, holder, u.Name)
	s.log.Info("api token added to the owner from MASSIMO_API_TOKEN", "user", owner)
	res.TokenAdded, res.Moved = true, held != 0
	return res, nil
}

// heldByIntegration finds the key of hash ("" for none) that an
// integration's user holds, as an older start could give it, live or
// revoked (a revoke of the integration revokes its users' keys): its id
// and that user's name, 0 where there is none. The hash is unique over
// revoked rows too, so a revoked one blocks the key as a live one does.
func (s *Store) heldByIntegration(ctx context.Context, hash string) (int64, string, error) {
	if hash == "" {
		return 0, "", nil
	}
	var id int64
	var name string
	err := s.db.QueryRowContext(ctx, `SELECT t.id, u.name FROM tokens t JOIN users u ON u.id = t.user_id
		WHERE t.hash = ? AND u.integration_id IS NOT NULL`, hash).Scan(&id, &name)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, "", nil
	}
	if err != nil {
		return 0, "", fmt.Errorf("store: bootstrap: %w", err)
	}
	return id, name, nil
}

// keyIn gives the owner of the id the key of hash in tx: the one held by
// an integration's user, moved to it and live again, as the file's token
// it is, its sessions ended, since the hash stays with its row; else a
// new one.
func (s *Store) keyIn(ctx context.Context, tx *sql.Tx, held, owner int64, hash string) error {
	now := s.nowText()
	if held == 0 {
		if _, err := tx.ExecContext(ctx, `INSERT INTO tokens (uuid, user_id, kind, hash, label, created_at, origin) VALUES (?, ?, ?, ?, ?, ?, ?)`,
			newUUID(), owner, TokenAPI, hash, BootstrapLabel, now, s.nodeID); err != nil {
			return fmt.Errorf("store: bootstrap: %w", err)
		}
		return nil
	}
	if _, err := tx.ExecContext(ctx, `UPDATE tokens SET user_id = ?, kind = ?, label = ?, expires_at = NULL, deleted_at = NULL, made_by = NULL, origin = ?, version = version + 1 WHERE id = ?`,
		owner, TokenAPI, BootstrapLabel, s.nodeID, held); err != nil {
		return fmt.Errorf("store: bootstrap: %w", err)
	}
	if err := revokeSessionsOf(ctx, tx, held, now, s.nodeID); err != nil {
		return fmt.Errorf("store: bootstrap: %w", err)
	}
	return nil
}

// keyMoved logs a key keyIn moved, once its transaction is committed.
func (s *Store) keyMoved(held int64, holder, owner string) {
	if held != 0 {
		s.log.Info("the settings file's key moved from an integration's user", "from", holder, "user", owner)
	}
}
