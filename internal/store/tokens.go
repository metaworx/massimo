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
// session ends it. A token whose secret the store made keeps the hash of
// its revoke code (revoke_hash, migration 0100), by which a client that
// forgot the token ends it (RevokeByCode).

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
	// RevokeKept says the row holds the hash of the token's revoke code
	// (revoke_hash); a token made before migration 0100 gets it at its
	// first use after it (TouchToken), one whose secret the store never
	// held none.
	RevokeKept bool
}

// Expired reports whether the token's end has passed at now.
func (t Token) Expired(now time.Time) bool { return !t.ExpiresAt.IsZero() && !now.Before(t.ExpiresAt) }

// HashToken returns the hash the store keeps of a token: SHA-256, hex.
func HashToken(secret string) string {
	h := sha256.Sum256([]byte(secret))
	return hex.EncodeToString(h[:])
}

// revokeLabel is what RevokeCode puts before the secret: the label
// holds a version, so that another derivation could follow beside it.
const revokeLabel = "massimo revoke v1"

// RevokeCode returns a token's revoke code: the hex of SHA-256 of
// "massimo revoke v1" followed by the secret as the server gave it, 64
// hex digits (AP MobileApp v1.6, D112-A). The store keeps HashToken of
// the code (revoke_hash), from which no code can be had back, so that
// only a client that once held the secret ends the token by its code,
// and neither the store nor a backup of it can. The code lets nothing
// in: no request is authenticated by it.
func RevokeCode(secret string) string {
	h := sha256.Sum256([]byte(revokeLabel + secret))
	return hex.EncodeToString(h[:])
}

// revokeHashOf is the revoke_hash of a token's secret.
func revokeHashOf(secret string) string { return HashToken(RevokeCode(secret)) }

// nullText is a text as the store writes it, NULL for "".
func nullText(v string) any {
	if v == "" {
		return nil
	}
	return v
}

// NewSecret returns a new token: 32 random bytes, hex.
func NewSecret() (string, error) {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("store: token: %w", err)
	}
	return hex.EncodeToString(b[:]), nil
}

const tokenColumns = `t.id, t.uuid, t.user_id, t.kind, t.label, t.created_at, t.expires_at, t.last_used_at, t.last_used_from, t.deleted_at, t.version, t.parent_id, t.revoke_hash IS NOT NULL`

// liveKey is the condition on a token t joined to its key p (LEFT JOIN
// tokens p ON p.id = t.parent_id): a session made from a key counts only
// while the key does, so that one whose key a binary before migration
// 0025 revoked is refused all the same.
const liveKey = `(t.parent_id IS NULL OR (p.id IS NOT NULL AND p.deleted_at IS NULL))`

func scanToken(row scanner) (Token, error) {
	var t Token
	var created, expires, used, deleted sql.NullString
	var parent sql.NullInt64
	if err := row.Scan(&t.ID, &t.UUID, &t.UserID, &t.Kind, &t.Label, &created, &expires, &used, &t.LastUsedFrom, &deleted, &t.Version, &parent, &t.RevokeKept); err != nil {
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
// it, for the integration's own key. The store makes the secret, so the
// row keeps the hash of its revoke code.
func (s *Store) addToken(ctx context.Context, userID int64, kind, label string, expires time.Time, ofIntegration bool) (Token, string, error) {
	secret, err := NewSecret()
	if err != nil {
		return Token{}, "", err
	}
	t, err := s.addTokenHash(ctx, userID, kind, label, HashToken(secret), revokeHashOf(secret), expires, ofIntegration)
	return t, secret, err
}

// AddTokenHash creates a token whose secret the caller holds, as the
// bootstrap does with MASSIMO_API_TOKEN; an integration's user is
// ErrIntegrationUser, as with AddToken. The store never sees the
// secret, so the row keeps no revoke code's hash.
func (s *Store) AddTokenHash(ctx context.Context, userID int64, kind, label, hash string, expires time.Time) (Token, error) {
	return s.addTokenHash(ctx, userID, kind, label, hash, "", expires, false)
}

// addTokenHash is AddTokenHash, with addToken's ofIntegration and the
// hash of the token's revoke code, "" for none.
func (s *Store) addTokenHash(ctx context.Context, userID int64, kind, label, hash, revokeHash string, expires time.Time, ofIntegration bool) (Token, error) {
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
	res, err := s.db.ExecContext(ctx, `INSERT INTO tokens (uuid, user_id, kind, hash, label, created_at, expires_at, origin, revoke_hash) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		newUUID(), userID, kind, hash, label, s.nowText(), nullTime(expires), s.nodeID, nullText(revokeHash))
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
	if err := s.revokeToken(ctx, tx, id); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("store: token %d: %w", id, err)
	}
	return nil
}

// revokeToken is RevokeToken within tx, which RevokeByCode shares, so
// that a revoke by code ends what a revoke ends.
func (s *Store) revokeToken(ctx context.Context, tx *sql.Tx, id int64) error {
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
	return nil
}

// RevokeByCode revokes, as RevokeToken does, the token whose revoke
// code is code (RevokeCode), a key's sessions with it, and returns it as
// it was before, ok false where none matched: no row of that code, one
// revoked already, or the token whose hash is keep, the settings file's
// key, which no revoke ends ("" keeps none). A token past its end is
// revoked all the same, so that what its revoke ends goes with it. The
// code is looked up by a plain read first, so that one that matches
// nothing, which anyone may post, takes no write transaction; a match is
// read again inside it.
func (s *Store) RevokeByCode(ctx context.Context, code, keep string) (Token, bool, error) {
	const live = `SELECT ` + tokenColumns + ` FROM tokens t WHERE t.revoke_hash = ? AND t.deleted_at IS NULL AND t.hash <> ?`
	hash := HashToken(code)
	if _, err := scanToken(s.db.QueryRowContext(ctx, live, hash, keep)); errors.Is(err, sql.ErrNoRows) {
		return Token{}, false, nil
	} else if err != nil {
		return Token{}, false, fmt.Errorf("store: revoke by code: %w", err)
	}
	if err := s.writable(); err != nil {
		return Token{}, false, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Token{}, false, fmt.Errorf("store: revoke by code: %w", err)
	}
	defer tx.Rollback()
	t, err := scanToken(tx.QueryRowContext(ctx, live, hash, keep))
	if errors.Is(err, sql.ErrNoRows) {
		return Token{}, false, nil // revoked meanwhile
	}
	if err != nil {
		return Token{}, false, fmt.Errorf("store: revoke by code: %w", err)
	}
	if err := s.revokeToken(ctx, tx, t.ID); err != nil {
		return Token{}, false, err
	}
	if err := tx.Commit(); err != nil {
		return Token{}, false, fmt.Errorf("store: token %d: %w", t.ID, err)
	}
	return t, true, nil
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
	res, err := tx.ExecContext(ctx, `INSERT INTO tokens (uuid, user_id, kind, hash, label, created_at, expires_at, origin, parent_id, revoke_hash) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		newUUID(), userID, TokenSession, HashToken(secret), label, now, nullTime(expires), s.nodeID, keyID, revokeHashOf(secret))
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
// moves its end to expires (zero keeps it); revokeHash, the hash of the
// token's revoke code, fills a row without one and leaves one written
// alone ("" writes none). The server calls it at most once an hour per
// token, and at the first use of a token without the hash.
func (s *Store) TouchToken(ctx context.Context, id int64, from string, expires time.Time, revokeHash string) error {
	if err := s.writable(); err != nil {
		return err
	}
	if _, err := s.db.ExecContext(ctx, `UPDATE tokens SET last_used_at = ?, last_used_from = ?, expires_at = coalesce(?, expires_at), revoke_hash = coalesce(revoke_hash, ?) WHERE id = ?`,
		s.nowText(), from, nullTime(expires), nullText(revokeHash), id); err != nil {
		return fmt.Errorf("store: token %d: %w", id, err)
	}
	return nil
}
