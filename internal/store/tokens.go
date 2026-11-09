// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package store

// Tokens (docs/store.md section 6): a session the web UI holds for 30
// days, extended on use, or an API key that never expires. A token is
// 32 random bytes shown once; the store keeps its SHA-256. A revoked
// token keeps its row (revoked_at), so that the audit's token_id still
// points at something.

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"time"
)

// The kinds of a token.
const (
	TokenSession = "session"
	TokenAPI     = "api"
)

// ErrNoToken is the error of a token that does not exist or is revoked.
var ErrNoToken = errors.New("store: no such token")

// Token is one row of tokens, without the hash.
type Token struct {
	ID           int64
	UUID         string
	UserID       int64
	Kind         string
	Label        string
	CreatedAt    time.Time
	ExpiresAt    time.Time // zero: never
	LastUsedAt   time.Time
	LastUsedFrom string
	RevokedAt    time.Time
	Version      int64
}

// Expired reports whether the token's end has passed at now.
func (t Token) Expired(now time.Time) bool { return !t.ExpiresAt.IsZero() && !now.Before(t.ExpiresAt) }

// HashToken returns the hash the store keeps of a token: SHA-256, hex.
func HashToken(secret string) string {
	h := sha256.Sum256([]byte(secret))
	return hex.EncodeToString(h[:])
}

// NewSecret returns a new token: 32 random bytes, hex.
func NewSecret() (string, error) {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("store: token: %w", err)
	}
	return hex.EncodeToString(b[:]), nil
}

const tokenColumns = `t.id, t.uuid, t.user_id, t.kind, t.label, t.created_at, t.expires_at, t.last_used_at, t.last_used_from, t.revoked_at, t.version`

func scanToken(row scanner) (Token, error) {
	var t Token
	var created, expires, used, revoked sql.NullString
	if err := row.Scan(&t.ID, &t.UUID, &t.UserID, &t.Kind, &t.Label, &created, &expires, &used, &t.LastUsedFrom, &revoked, &t.Version); err != nil {
		return Token{}, err
	}
	t.CreatedAt, t.ExpiresAt, t.LastUsedAt, t.RevokedAt = parseTime(created), parseTime(expires), parseTime(used), parseTime(revoked)
	return t, nil
}

// AddToken creates a token of a kind for a user, with a label and an
// end (zero: never), and returns it with the secret, which is shown
// once and never stored.
func (s *Store) AddToken(ctx context.Context, userID int64, kind, label string, expires time.Time) (Token, string, error) {
	secret, err := NewSecret()
	if err != nil {
		return Token{}, "", err
	}
	t, err := s.AddTokenHash(ctx, userID, kind, label, HashToken(secret), expires)
	return t, secret, err
}

// AddTokenHash creates a token whose secret the caller holds, as the
// bootstrap does with MASSIMO_API_TOKEN.
func (s *Store) AddTokenHash(ctx context.Context, userID int64, kind, label, hash string, expires time.Time) (Token, error) {
	if err := s.writable(); err != nil {
		return Token{}, err
	}
	if kind != TokenSession && kind != TokenAPI {
		return Token{}, fmt.Errorf("store: token kind %q", kind)
	}
	res, err := s.db.ExecContext(ctx, `INSERT INTO tokens (uuid, user_id, kind, hash, label, created_at, expires_at, origin) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		newUUID(), userID, kind, hash, label, s.nowText(), nullTime(expires), s.nodeID)
	if err != nil {
		return Token{}, fmt.Errorf("store: add token: %w", err)
	}
	id, _ := res.LastInsertId()
	t, _, err := s.Token(ctx, id)
	return t, err
}

// Token returns a token by identifier, revoked or not.
func (s *Store) Token(ctx context.Context, id int64) (Token, bool, error) {
	t, err := scanToken(s.db.QueryRowContext(ctx, `SELECT `+tokenColumns+` FROM tokens t WHERE t.id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return Token{}, false, nil
	}
	if err != nil {
		return Token{}, false, fmt.Errorf("store: token %d: %w", id, err)
	}
	return t, true, nil
}

// TokenByHash finds a live token by the hash of its secret: not revoked,
// its user not removed. Whether it has expired is the caller's check.
func (s *Store) TokenByHash(ctx context.Context, hash string) (Token, bool, error) {
	t, err := scanToken(s.db.QueryRowContext(ctx, `SELECT `+tokenColumns+` FROM tokens t JOIN users u ON u.id = t.user_id WHERE t.hash = ? AND t.revoked_at IS NULL AND u.deleted_at IS NULL`, hash))
	if errors.Is(err, sql.ErrNoRows) {
		return Token{}, false, nil
	}
	if err != nil {
		return Token{}, false, fmt.Errorf("store: token: %w", err)
	}
	return t, true, nil
}

// Tokens lists a user's live tokens, oldest first.
func (s *Store) Tokens(ctx context.Context, userID int64) ([]Token, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+tokenColumns+` FROM tokens t WHERE t.user_id = ? AND t.revoked_at IS NULL ORDER BY t.id`, userID)
	if err != nil {
		return nil, fmt.Errorf("store: tokens: %w", err)
	}
	defer rows.Close()
	out := []Token{}
	for rows.Next() {
		t, err := scanToken(rows)
		if err != nil {
			return nil, fmt.Errorf("store: tokens: %w", err)
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// RevokeToken revokes a live token; ErrNoToken when there is none.
func (s *Store) RevokeToken(ctx context.Context, id int64) error {
	if err := s.writable(); err != nil {
		return err
	}
	res, err := s.db.ExecContext(ctx, `UPDATE tokens SET revoked_at = ?, version = version + 1 WHERE id = ? AND revoked_at IS NULL`, s.nowText(), id)
	if err != nil {
		return fmt.Errorf("store: token %d: %w", id, err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("%w: %d", ErrNoToken, id)
	}
	// What the token subscribed to push goes with it.
	if err := removePushSubscriptions(ctx, s.db, 0, id); err != nil {
		return fmt.Errorf("store: token %d: %w", id, err)
	}
	return nil
}

// RevokeSessions revokes every live session of a user, as a password
// set by someone else does; API keys stay.
func (s *Store) RevokeSessions(ctx context.Context, userID int64) error {
	if err := s.writable(); err != nil {
		return err
	}
	if _, err := s.db.ExecContext(ctx, `UPDATE tokens SET revoked_at = ?, version = version + 1 WHERE user_id = ? AND kind = ? AND revoked_at IS NULL`, s.nowText(), userID, TokenSession); err != nil {
		return fmt.Errorf("store: sessions of %d: %w", userID, err)
	}
	if _, err := s.db.ExecContext(ctx, `DELETE FROM push_subscriptions WHERE token_id IN (SELECT id FROM tokens WHERE user_id = ? AND kind = ? AND revoked_at IS NOT NULL)`, userID, TokenSession); err != nil {
		return fmt.Errorf("store: sessions of %d: %w", userID, err)
	}
	return nil
}

// TouchToken records a use at now from an address and, for a session,
// moves its end to expires (zero keeps it). The server calls it at most
// once an hour per token.
func (s *Store) TouchToken(ctx context.Context, id int64, from string, expires time.Time) error {
	if err := s.writable(); err != nil {
		return err
	}
	if _, err := s.db.ExecContext(ctx, `UPDATE tokens SET last_used_at = ?, last_used_from = ?, expires_at = coalesce(?, expires_at) WHERE id = ?`, s.nowText(), from, nullTime(expires), id); err != nil {
		return fmt.Errorf("store: token %d: %w", id, err)
	}
	return nil
}
