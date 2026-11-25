// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package store

// Tokens (docs/store.md section 6): a session the web UI holds for 30
// days, extended on use, or an API key that never expires. A token is
// 32 random bytes shown once; the store keeps its SHA-256. A revoked
// token is a tombstone (deleted_at, revoked_at before migration 0024)
// that keeps its row, so that the audit's token_id still
// points at something. A session made from a key names it (parent_id,
// migration 0025): it lives while the key does, and the key's next
// session ends it.

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
	DeletedAt    time.Time // when it was revoked; zero while it lives
	Version      int64
	// ParentID is the key a session was made from (AddKeySession), 0
	// for a session of a sign-in and for a key.
	ParentID int64
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

const tokenColumns = `t.id, t.uuid, t.user_id, t.kind, t.label, t.created_at, t.expires_at, t.last_used_at, t.last_used_from, t.deleted_at, t.version, t.parent_id`

// liveKey is the condition on a token t joined to its key p (LEFT JOIN
// tokens p ON p.id = t.parent_id): a session made from a key counts only
// while the key does, so that one whose key a binary before migration
// 0025 revoked is refused all the same.
const liveKey = `(t.parent_id IS NULL OR (p.id IS NOT NULL AND p.deleted_at IS NULL))`

func scanToken(row scanner) (Token, error) {
	var t Token
	var created, expires, used, deleted sql.NullString
	var parent sql.NullInt64
	if err := row.Scan(&t.ID, &t.UUID, &t.UserID, &t.Kind, &t.Label, &created, &expires, &used, &t.LastUsedFrom, &deleted, &t.Version, &parent); err != nil {
		return Token{}, err
	}
	t.CreatedAt, t.ExpiresAt, t.LastUsedAt, t.DeletedAt = parseTime(created), parseTime(expires), parseTime(used), parseTime(deleted)
	t.ParentID = parent.Int64
	return t, nil
}

// AddToken creates a token of a kind for a user, with a label and an
// end (zero: never), and returns it with the secret, which is shown
// once and never stored. An integration's user is ErrIntegrationUser:
// its integration's key is CollectIntegrationKey's.
func (s *Store) AddToken(ctx context.Context, userID int64, kind, label string, expires time.Time) (Token, string, error) {
	return s.addToken(ctx, userID, kind, label, expires, false)
}

// addToken is AddToken; ofIntegration lets an integration's user have
// it, for the integration's own key.
func (s *Store) addToken(ctx context.Context, userID int64, kind, label string, expires time.Time, ofIntegration bool) (Token, string, error) {
	secret, err := NewSecret()
	if err != nil {
		return Token{}, "", err
	}
	t, err := s.addTokenHash(ctx, userID, kind, label, HashToken(secret), expires, ofIntegration)
	return t, secret, err
}

// AddTokenHash creates a token whose secret the caller holds, as the
// bootstrap does with MASSIMO_API_TOKEN; an integration's user is
// ErrIntegrationUser, as with AddToken.
func (s *Store) AddTokenHash(ctx context.Context, userID int64, kind, label, hash string, expires time.Time) (Token, error) {
	return s.addTokenHash(ctx, userID, kind, label, hash, expires, false)
}

// addTokenHash is AddTokenHash, with addToken's ofIntegration.
func (s *Store) addTokenHash(ctx context.Context, userID int64, kind, label, hash string, expires time.Time, ofIntegration bool) (Token, error) {
	if err := s.writable(); err != nil {
		return Token{}, err
	}
	if kind != TokenSession && kind != TokenAPI {
		return Token{}, fmt.Errorf("store: token kind %q", kind)
	}
	if !ofIntegration {
		if err := noWayIn(ctx, s.db, "id = ?", userID); err != nil {
			return Token{}, err
		}
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
// its user not removed, and for a session made from a key, the key not
// revoked. Whether it has expired is the caller's check.
func (s *Store) TokenByHash(ctx context.Context, hash string) (Token, bool, error) {
	t, err := scanToken(s.db.QueryRowContext(ctx, `SELECT `+tokenColumns+` FROM tokens t JOIN users u ON u.id = t.user_id LEFT JOIN tokens p ON p.id = t.parent_id
		WHERE t.hash = ? AND t.deleted_at IS NULL AND u.deleted_at IS NULL AND `+liveKey, hash))
	if errors.Is(err, sql.ErrNoRows) {
		return Token{}, false, nil
	}
	if err != nil {
		return Token{}, false, fmt.Errorf("store: token: %w", err)
	}
	return t, true, nil
}

// TokenEnded reports whether hash is the hash of a token the store issued
// that has ended by now: revoked (deleted_at: a logout, a revoke, a
// removed user's tokens), past its end (expires_at), its user removed, or
// a session made from a key that was revoked. A live token, and a hash
// no token has, are not. The server asks it of a token it refuses, so
// that one it issued and that ended, which a client may still send, does
// not count toward the address's lockout as a guess does: a guessed
// secret never hits the hash of a token that was issued (docs/api-v1.md,
// "Authentication").
func (s *Store) TokenEnded(ctx context.Context, hash string, now time.Time) (bool, error) {
	var ended bool
	err := s.db.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM tokens t LEFT JOIN users u ON u.id = t.user_id LEFT JOIN tokens p ON p.id = t.parent_id
		WHERE t.hash = ? AND (t.deleted_at IS NOT NULL OR t.expires_at <= ? OR u.id IS NULL OR u.deleted_at IS NOT NULL OR NOT `+liveKey+`))`,
		hash, formatTime(now)).Scan(&ended)
	if err != nil {
		return false, fmt.Errorf("store: ended token: %w", err)
	}
	return ended, nil
}

// Tokens lists a user's live tokens, oldest first: a session made from a
// key only while the key lives.
func (s *Store) Tokens(ctx context.Context, userID int64) ([]Token, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+tokenColumns+` FROM tokens t LEFT JOIN tokens p ON p.id = t.parent_id
		WHERE t.user_id = ? AND t.deleted_at IS NULL AND `+liveKey+` ORDER BY t.id`, userID)
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

// RevokeToken revokes a live token, and for a key the live sessions
// made from it; what each subscribed to push goes with it. ErrNoToken
// when there is none.
func (s *Store) RevokeToken(ctx context.Context, id int64) error {
	if err := s.writable(); err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("store: token %d: %w", id, err)
	}
	defer tx.Rollback()
	now := s.nowText()
	res, err := tx.ExecContext(ctx, `UPDATE tokens SET deleted_at = ?, origin = ?, version = version + 1 WHERE id = ? AND deleted_at IS NULL`, now, s.nodeID, id)
	if err != nil {
		return fmt.Errorf("store: token %d: %w", id, err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("%w: %d", ErrNoToken, id)
	}
	if err := revokeSessionsOf(ctx, tx, id, now, s.nodeID); err != nil {
		return fmt.Errorf("store: token %d: %w", id, err)
	}
	// What the token subscribed to push goes with it.
	if err := removePushSubscriptions(ctx, tx, 0, id); err != nil {
		return fmt.Errorf("store: token %d: %w", id, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("store: token %d: %w", id, err)
	}
	return nil
}

// revokeSessionsOf revokes the live sessions made from the key keyID,
// and what they subscribed to push.
func revokeSessionsOf(ctx context.Context, tx execer, keyID int64, now, node string) error {
	if _, err := tx.ExecContext(ctx, `DELETE FROM push_subscriptions WHERE token_id IN (SELECT id FROM tokens WHERE parent_id = ? AND deleted_at IS NULL)`, keyID); err != nil {
		return err
	}
	_, err := tx.ExecContext(ctx, `UPDATE tokens SET deleted_at = ?, origin = ?, version = version + 1 WHERE parent_id = ? AND deleted_at IS NULL`, now, node, keyID)
	return err
}

// AddKeySession makes a session from the live API key keyID, with a
// label and an end, and returns it with the secret, shown once: the
// session names the key, and the key's earlier sessions are revoked,
// so that a key has one live session at a time. ErrNoToken where the key
// is not a live API key; ErrIntegrationUser where its user is an
// integration's, which makes no session.
func (s *Store) AddKeySession(ctx context.Context, keyID int64, label string, expires time.Time) (Token, string, error) {
	if err := s.writable(); err != nil {
		return Token{}, "", err
	}
	secret, err := NewSecret()
	if err != nil {
		return Token{}, "", err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Token{}, "", fmt.Errorf("store: session of key %d: %w", keyID, err)
	}
	defer tx.Rollback()
	var userID int64
	err = tx.QueryRowContext(ctx, `SELECT user_id FROM tokens WHERE id = ? AND kind = ? AND deleted_at IS NULL`, keyID, TokenAPI).Scan(&userID)
	if errors.Is(err, sql.ErrNoRows) {
		return Token{}, "", fmt.Errorf("%w: key %d", ErrNoToken, keyID)
	}
	if err != nil {
		return Token{}, "", fmt.Errorf("store: session of key %d: %w", keyID, err)
	}
	if err := noWayIn(ctx, tx, "id = ?", userID); err != nil {
		return Token{}, "", err
	}
	now := s.nowText()
	if err := revokeSessionsOf(ctx, tx, keyID, now, s.nodeID); err != nil {
		return Token{}, "", fmt.Errorf("store: session of key %d: %w", keyID, err)
	}
	res, err := tx.ExecContext(ctx, `INSERT INTO tokens (uuid, user_id, kind, hash, label, created_at, expires_at, origin, parent_id) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		newUUID(), userID, TokenSession, HashToken(secret), label, now, nullTime(expires), s.nodeID, keyID)
	if err != nil {
		return Token{}, "", fmt.Errorf("store: session of key %d: %w", keyID, err)
	}
	if err := tx.Commit(); err != nil {
		return Token{}, "", fmt.Errorf("store: session of key %d: %w", keyID, err)
	}
	id, _ := res.LastInsertId()
	t, _, err := s.Token(ctx, id)
	return t, secret, err
}

// RevokeSessions revokes every live session of a user, as a password
// set by someone else does; API keys stay.
func (s *Store) RevokeSessions(ctx context.Context, userID int64) error {
	if err := s.writable(); err != nil {
		return err
	}
	if _, err := s.db.ExecContext(ctx, `UPDATE tokens SET deleted_at = ?, origin = ?, version = version + 1 WHERE user_id = ? AND kind = ? AND deleted_at IS NULL`, s.nowText(), s.nodeID, userID, TokenSession); err != nil {
		return fmt.Errorf("store: sessions of %d: %w", userID, err)
	}
	if _, err := s.db.ExecContext(ctx, `DELETE FROM push_subscriptions WHERE token_id IN (SELECT id FROM tokens WHERE user_id = ? AND kind = ? AND deleted_at IS NOT NULL)`, userID, TokenSession); err != nil {
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
