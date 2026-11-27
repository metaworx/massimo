// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package store

// Tokens (docs/store.md section 6): a session the web UI holds for 30
// days, extended on use, or an API key that never expires. A token is
// 32 random bytes shown once; the store keeps its SHA-256. A revoked
// token is a tombstone (deleted_at, revoked_at before migration 0024)
// that keeps its row, so that the audit's token_id still
// points at something. A session made from a key names it (parent_id,
// migration 0025): it lives while the key is neither revoked nor past its
// end, and the key's next session ends it. A key made with a key as the
// bearer names it (made_by, migration 0100), and a revoke of that key
// revokes it, and the keys it made (AP KeyLife, D5-A). Every place a
// token is let in, counted or sent to holds to one rule, alive.

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
	// MadeBy is the API key that made this key, the bearer of the
	// request that made it (AddKey, AP KeyLife, D5-A): a revoke of that
	// key revokes this one. 0 for a key made by a session, the command
	// line, the bootstrap or an integration's approval, and for a session.
	MadeBy int64
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

const tokenColumns = `t.id, t.uuid, t.user_id, t.kind, t.label, t.created_at, t.expires_at, t.last_used_at, t.last_used_from, t.deleted_at, t.version, t.parent_id, t.made_by`

// tokenJoins joins a token t to its user u and to the key k a session was
// made from, for the rule alive.
const tokenJoins = ` FROM tokens t LEFT JOIN users u ON u.id = t.user_id LEFT JOIN tokens k ON k.id = t.parent_id`

// alive is the rule of a live token t, joined as tokenJoins joins it
// (AP KeyLife, Block 2): not revoked, not past its own end, its user
// there and not removed, and for a session made from a key, the key
// there, not revoked and not past its end; so a session whose key a
// binary before migration 0025 revoked is refused all the same. It is
// never NULL, so NOT alive is its opposite. It takes the time of the
// check twice (aliveArgs). TokenByHash, TokenAlive, TokenEnded, Tokens,
// PushSubscriptions and AddKeySession hold to it.
const alive = `(t.deleted_at IS NULL AND (t.expires_at IS NULL OR t.expires_at > ?) AND u.id IS NOT NULL AND u.deleted_at IS NULL
	AND (t.parent_id IS NULL OR (k.id IS NOT NULL AND k.deleted_at IS NULL AND (k.expires_at IS NULL OR k.expires_at > ?))))`

// aliveArgs are alive's arguments at now.
func aliveArgs(now time.Time) []any {
	at := formatTime(now)
	return []any{at, at}
}

func scanToken(row scanner) (Token, error) {
	var t Token
	var created, expires, used, deleted sql.NullString
	var parent, madeBy sql.NullInt64
	if err := row.Scan(&t.ID, &t.UUID, &t.UserID, &t.Kind, &t.Label, &created, &expires, &used, &t.LastUsedFrom, &deleted, &t.Version, &parent, &madeBy); err != nil {
		return Token{}, err
	}
	t.CreatedAt, t.ExpiresAt, t.LastUsedAt, t.DeletedAt = parseTime(created), parseTime(expires), parseTime(used), parseTime(deleted)
	t.ParentID, t.MadeBy = parent.Int64, madeBy.Int64
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
	t, err := s.addTokenHash(ctx, userID, kind, label, HashToken(secret), expires, 0, ofIntegration)
	return t, secret, err
}

// KeyOptions are what AddKey makes an API key with.
type KeyOptions struct {
	// Label names the key in its user's list.
	Label string
	// MadeBy is the API key that was the bearer of the request that makes
	// this one (AP KeyLife, D5-A), 0 for none: a key made by a session of
	// any kind, by the command line, the bootstrap or an integration's
	// approval. A revoke of that key revokes this one.
	MadeBy int64
}

// AddKey creates an API key for a user with opts, without an end, and
// returns it with the secret, shown once; AddToken of a key is AddKey
// without a maker. An integration's user is ErrIntegrationUser, as with
// AddToken.
func (s *Store) AddKey(ctx context.Context, userID int64, opts KeyOptions) (Token, string, error) {
	secret, err := NewSecret()
	if err != nil {
		return Token{}, "", err
	}
	t, err := s.addTokenHash(ctx, userID, TokenAPI, opts.Label, HashToken(secret), time.Time{}, opts.MadeBy, false)
	return t, secret, err
}

// AddTokenHash creates a token whose secret the caller holds, as the
// bootstrap does with MASSIMO_API_TOKEN; an integration's user is
// ErrIntegrationUser, as with AddToken.
func (s *Store) AddTokenHash(ctx context.Context, userID int64, kind, label, hash string, expires time.Time) (Token, error) {
	return s.addTokenHash(ctx, userID, kind, label, hash, expires, 0, false)
}

// addTokenHash is AddTokenHash, with the key that made it, 0 for none
// (KeyOptions.MadeBy), and addToken's ofIntegration. A maker that is no
// live API key under the rule alive is ErrNoToken, and nothing is made:
// the check and the insert are one statement, so that a request whose
// bearer was let in before its revoke, and makes its key after it, leaves
// no live key naming a revoked maker that no later revoke would reach.
func (s *Store) addTokenHash(ctx context.Context, userID int64, kind, label, hash string, expires time.Time, madeBy int64, ofIntegration bool) (Token, error) {
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
	args := append([]any{newUUID(), userID, kind, hash, label, s.nowText(), nullTime(expires), s.nodeID, nullID(madeBy), madeBy, madeBy, TokenAPI}, aliveArgs(s.now())...)
	res, err := s.db.ExecContext(ctx, `INSERT INTO tokens (uuid, user_id, kind, hash, label, created_at, expires_at, origin, made_by)
		SELECT ?, ?, ?, ?, ?, ?, ?, ?, ? WHERE ? = 0 OR EXISTS (SELECT 1`+tokenJoins+` WHERE t.id = ? AND t.kind = ? AND `+alive+`)`, args...)
	if err != nil {
		return Token{}, fmt.Errorf("store: add token: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return Token{}, fmt.Errorf("%w: the maker %d is no live API key", ErrNoToken, madeBy)
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

// TokenByHash finds a live token by the hash of its secret, under the
// rule alive at the store's now: not revoked, not past its end, its user
// not removed, and for a session made from a key, the key neither revoked
// nor past its end.
func (s *Store) TokenByHash(ctx context.Context, hash string) (Token, bool, error) {
	t, err := scanToken(s.db.QueryRowContext(ctx, `SELECT `+tokenColumns+tokenJoins+` WHERE t.hash = ? AND `+alive,
		append([]any{hash}, aliveArgs(s.now())...)...))
	if errors.Is(err, sql.ErrNoRows) {
		return Token{}, false, nil
	}
	if err != nil {
		return Token{}, false, fmt.Errorf("store: token: %w", err)
	}
	return t, true, nil
}

// TokenAlive returns a token by identifier while it lives, under the rule
// TokenByHash holds to; ok is false for one that does not, and for none.
// The event stream asks it at each keep-alive (the server's
// stillAdmitted).
func (s *Store) TokenAlive(ctx context.Context, id int64) (Token, bool, error) {
	t, err := scanToken(s.db.QueryRowContext(ctx, `SELECT `+tokenColumns+tokenJoins+` WHERE t.id = ? AND `+alive,
		append([]any{id}, aliveArgs(s.now())...)...))
	if errors.Is(err, sql.ErrNoRows) {
		return Token{}, false, nil
	}
	if err != nil {
		return Token{}, false, fmt.Errorf("store: token %d: %w", id, err)
	}
	return t, true, nil
}

// TokenEnded reports whether hash is the hash of a token the store issued
// that has ended by now, the rule alive no longer holding for it: revoked
// (deleted_at: a logout, a revoke, a removed user's tokens), past its end
// (expires_at), its user removed, or a session made from a key that was
// revoked or is past its end. A live token, and a hash no token has, are
// not. The server asks it of a token it refuses, so that one it issued
// and that ended, which a client may still send, does not count toward
// the address's lockout as a guess does: a guessed secret never hits the
// hash of a token that was issued (docs/api-v1.md, "Authentication").
func (s *Store) TokenEnded(ctx context.Context, hash string, now time.Time) (bool, error) {
	var ended bool
	err := s.db.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1`+tokenJoins+` WHERE t.hash = ? AND NOT `+alive+`)`,
		append([]any{hash}, aliveArgs(now)...)...).Scan(&ended)
	if err != nil {
		return false, fmt.Errorf("store: ended token: %w", err)
	}
	return ended, nil
}

// Tokens lists a user's live tokens, oldest first: a session under the
// rule alive, so one made from a key only while the key is neither
// revoked nor past its end, and a key while it is not revoked, past its
// end or not, so that it can still be revoked; whether a key has ended is
// the caller's (Token.Expired).
func (s *Store) Tokens(ctx context.Context, userID int64) ([]Token, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+tokenColumns+tokenJoins+` WHERE t.user_id = ? AND t.deleted_at IS NULL AND (t.kind = ? OR `+alive+`) ORDER BY t.id`,
		append([]any{userID, TokenAPI}, aliveArgs(s.now())...)...)
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

// RevokeToken revokes a token not revoked yet, and for a key the live
// sessions made from it and its heirs, the keys it made and theirs
// (revokeHeirs); what each subscribed to push goes with it. It returns
// the heirs it revoked, oldest first, [] for none; ErrNoToken when there
// is no such token, or it is revoked already.
func (s *Store) RevokeToken(ctx context.Context, id int64) ([]int64, error) {
	if err := s.writable(); err != nil {
		return nil, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("store: token %d: %w", id, err)
	}
	defer tx.Rollback()
	now := s.nowText()
	res, err := tx.ExecContext(ctx, `UPDATE tokens SET deleted_at = ?, origin = ?, version = version + 1 WHERE id = ? AND deleted_at IS NULL`, now, s.nodeID, id)
	if err != nil {
		return nil, fmt.Errorf("store: token %d: %w", id, err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return nil, fmt.Errorf("%w: %d", ErrNoToken, id)
	}
	if err := revokeSessionsOf(ctx, tx, id, now, s.nodeID); err != nil {
		return nil, fmt.Errorf("store: token %d: %w", id, err)
	}
	// What the token subscribed to push goes with it.
	if err := removePushSubscriptions(ctx, tx, 0, id); err != nil {
		return nil, fmt.Errorf("store: token %d: %w", id, err)
	}
	heirs, err := s.revokeHeirs(ctx, tx, `?`, []any{id}, now)
	if err != nil {
		return nil, fmt.Errorf("store: token %d: %w", id, err)
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("store: token %d: %w", id, err)
	}
	return heirs, nil
}

// revokeHeirs revokes in tx the heirs of the tokens roots selects (an SQL
// list or SELECT of ids, with its args): the keys made by them (made_by),
// and theirs, followed from key to key in any state, so that a key past
// its end, or one a binary before migration 0100 revoked alone, passes
// the revoke on (AP KeyLife, D5-A); each not revoked yet is revoked with
// its sessions and what they and it subscribed to push. The settings
// file's key, by the hash Bootstrap last applied, is passed over, and so
// are the keys it made: whoever made it, it goes with no other key's
// revoke nor its maker's user's removal, as the API revokes it neither
// (MobileApp.D86-A, D89-A). It returns those it revoked, oldest first,
// [] for none.
func (s *Store) revokeHeirs(ctx context.Context, tx *sql.Tx, roots string, args []any, now string) ([]int64, error) {
	file := s.settingsHash()
	rows, err := tx.QueryContext(ctx, `WITH RECURSIVE heirs(id) AS (SELECT id FROM tokens WHERE made_by IN (`+roots+`) AND hash <> ?
		UNION SELECT t.id FROM tokens t JOIN heirs h ON t.made_by = h.id WHERE t.hash <> ?)
		SELECT t.id FROM tokens t JOIN heirs h ON h.id = t.id WHERE t.deleted_at IS NULL ORDER BY t.id`, append(args, file, file)...)
	if err != nil {
		return nil, fmt.Errorf("heirs: %w", err)
	}
	heirs := []int64{}
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return nil, fmt.Errorf("heirs: %w", err)
		}
		heirs = append(heirs, id)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("heirs: %w", err)
	}
	for _, id := range heirs {
		if _, err := tx.ExecContext(ctx, `UPDATE tokens SET deleted_at = ?, origin = ?, version = version + 1 WHERE id = ? AND deleted_at IS NULL`, now, s.nodeID, id); err != nil {
			return nil, fmt.Errorf("heir %d: %w", id, err)
		}
		if err := revokeSessionsOf(ctx, tx, id, now, s.nodeID); err != nil {
			return nil, fmt.Errorf("heir %d: %w", id, err)
		}
		if err := removePushSubscriptions(ctx, tx, 0, id); err != nil {
			return nil, fmt.Errorf("heir %d: %w", id, err)
		}
	}
	return heirs, nil
}

// settingsHash is the hash of the settings file's token that Bootstrap
// last applied, "" for none and before it ran (the command line runs it
// never).
func (s *Store) settingsHash() string {
	if h := s.fileHash.Load(); h != nil {
		return *h
	}
	return ""
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
// is not a live API key under the rule alive: none, revoked or past its
// end; ErrIntegrationUser where its user is an integration's, which makes
// no session.
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
	err = tx.QueryRowContext(ctx, `SELECT t.user_id`+tokenJoins+` WHERE t.id = ? AND t.kind = ? AND `+alive,
		append([]any{keyID, TokenAPI}, aliveArgs(s.now())...)...).Scan(&userID)
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
