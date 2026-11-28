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
// end, and the key's next session ends it. A token whose secret the store
// made keeps the hash of its revoke code (revoke_hash, migration 0100), by
// which a client that forgot the token ends it (RevokeByCode). A key made
// with a key as the bearer names it (made_by, migration 0101), and a
// revoke of that key, by its id or by its code, revokes it, and the keys
// it made (AP KeyLife, D5-A). An API key unused for the days of the
// setting auth.key_idle_days ends, unless it is kept from it
// (idle_exempt, migration 0102) or is the settings file's (AP KeyLife,
// Block 3); an end is a state read at each check, nothing written, so a
// key lives again where the limit is raised or the key excluded (D6-A).
// Every place a token is let in, counted or sent to holds to one rule,
// alive.

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
	// RevokeKept says the row holds the hash of the token's revoke code
	// (revoke_hash); a token made before migration 0100 gets it at its
	// first use after it (TouchToken), one whose secret the store never
	// held none.
	RevokeKept bool
	// IdleExempt keeps an API key from the idle limit (idle_exempt,
	// migration 0102; AP KeyLife, D7-A): it does not end however long it
	// stays unused. An integration's key never has it.
	IdleExempt bool

	// SettingsKey, IdleEndsAt, EndsAt and Ended are the token's life under
	// the rule alive at the check that read it (lifeRule.apply): Tokens,
	// TokenByHash, TokenAlive and SetTokenLife give them; Token gives the
	// row alone, and they are zero there.

	// SettingsKey says the token is the settings file's MASSIMO_API_TOKEN,
	// by the hash Bootstrap last applied (D3-A): the idle limit passes
	// over it.
	SettingsKey bool
	// IdleEndsAt is when an API key ends unused: its last use, or its
	// making where it was never used, plus the idle limit; zero where the
	// limit is off, for a key kept from it, the settings file's and a
	// session.
	IdleEndsAt time.Time
	// EndsAt is the earlier of the token's end date (ExpiresAt) and
	// IdleEndsAt, zero for neither.
	EndsAt time.Time
	// Ended says why the token has ended (EndedRevoked, EndedExpired,
	// EndedIdle), "" while it lives by its own row; a session made from a
	// key is not judged by its key here, Tokens listing such a session
	// only while it lives.
	Ended string

	hash string // the hash, for the settings file's key alone (SettingsKey); never shown
}

// Why a token has ended (Token.Ended); the API shows the last two as a
// key's ended.
const (
	EndedRevoked = "revoked"
	EndedExpired = "expired" // its end date has passed
	EndedIdle    = "idle"    // an API key unused for longer than the idle limit
)

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

const tokenColumns = `t.id, t.uuid, t.user_id, t.kind, t.label, t.created_at, t.expires_at, t.last_used_at, t.last_used_from, t.deleted_at, t.version, t.parent_id, t.made_by, t.revoke_hash IS NOT NULL, t.idle_exempt, t.hash`

// tokenJoins joins a token t to its user u and to the key k a session was
// made from, for the rule alive.
const tokenJoins = ` FROM tokens t LEFT JOIN users u ON u.id = t.user_id LEFT JOIN tokens k ON k.id = t.parent_id`

// alive is the rule of a live token t, joined as tokenJoins joins it
// (AP KeyLife, Blocks 2 and 3): not revoked, not past its own end, not
// ended unused (idleAliveT), its user there and not removed, and for a
// session made from a key, the key there, not revoked, not past its end
// and not ended unused; so a session whose key a binary before migration
// 0025 revoked is refused all the same. It is never NULL, so NOT alive
// is its opposite. It takes lifeRule.args. TokenByHash, TokenAlive,
// TokenEnded, Tokens, PushSubscriptions and AddKeySession hold to it, and
// lifeRule.apply is its form in Go for a token's own row.
const alive = `(t.deleted_at IS NULL AND (t.expires_at IS NULL OR t.expires_at > ?) AND ` + idleAliveT + ` AND u.id IS NOT NULL AND u.deleted_at IS NULL
	AND (t.parent_id IS NULL OR (k.id IS NOT NULL AND k.deleted_at IS NULL AND (k.expires_at IS NULL OR k.expires_at > ?) AND ` + idleAliveK + `)))`

// idleAliveT and idleAliveK are the idle part of alive, for the token t
// and the key k a session was made from (AP KeyLife, Block 3): a token
// other than an API key, a key kept from the limit (idle_exempt), the
// settings file's key (its hash, D3-A), any token while the limit is off
// (the cutoff ""), or a key last used, or made where it was never used,
// after the cutoff, the time of the check less the limit. Each takes the
// file's hash and the cutoff twice.
const (
	idleAliveT = `(t.kind <> '` + TokenAPI + `' OR t.idle_exempt <> 0 OR t.hash = ? OR ? = '' OR coalesce(t.last_used_at, t.created_at) > ?)`
	idleAliveK = `(k.kind <> '` + TokenAPI + `' OR k.idle_exempt <> 0 OR k.hash = ? OR ? = '' OR coalesce(k.last_used_at, k.created_at) > ?)`
)

// lifeRule is what the rule alive takes at one check (AP KeyLife, Block
// 3): its time, the settings file's hash ("" for none), which the idle
// limit passes over, and the idle limit, 0 where it is off.
type lifeRule struct {
	now  time.Time
	file string
	idle time.Duration
}

// rule reads the rule alive at now: the setting auth.key_idle_days,
// read at each check so that a change applies at once, and the
// settings file's hash as Bootstrap last applied it. A setting that
// cannot be read fails the check, as a token that cannot be read does.
// It reads through s.db, so it is called outside a transaction: the
// store has one connection.
func (s *Store) rule(ctx context.Context, now time.Time) (lifeRule, error) {
	days, _, err := s.SettingInt(ctx, KeyIdleDaysKey)
	if err != nil {
		return lifeRule{}, err
	}
	r := lifeRule{now: now, file: s.settingsHash()}
	if days > 0 {
		r.idle = time.Duration(days) * 24 * time.Hour
	}
	return r, nil
}

// args are alive's arguments under r: the time of the check, then the
// idle part's, for t and again for k.
func (r lifeRule) args() []any {
	at, cutoff := formatTime(r.now), ""
	if r.idle > 0 {
		cutoff = formatTime(r.now.Add(-r.idle))
	}
	return []any{at, r.file, cutoff, cutoff, at, r.file, cutoff, cutoff}
}

// apply gives t, a token as its row has it, its life under r, by the
// rule alive's own parts in Go (TestKeyEndRule holds the two to each
// other): SettingsKey, IdleEndsAt, EndsAt, and Ended, revoked first,
// then whichever of its end date and its idle end came first, the end
// date where both came at once. Its user and the key a session was made
// from are the SQL's alone.
func (r lifeRule) apply(t *Token) {
	t.SettingsKey = r.file != "" && t.hash == r.file
	t.IdleEndsAt = time.Time{}
	if t.Kind == TokenAPI && !t.IdleExempt && !t.SettingsKey && r.idle > 0 {
		from := t.LastUsedAt
		if from.IsZero() {
			from = t.CreatedAt
		}
		t.IdleEndsAt = from.Add(r.idle)
	}
	t.EndsAt = t.ExpiresAt
	if !t.IdleEndsAt.IsZero() && (t.EndsAt.IsZero() || t.IdleEndsAt.Before(t.EndsAt)) {
		t.EndsAt = t.IdleEndsAt
	}
	switch {
	case !t.DeletedAt.IsZero():
		t.Ended = EndedRevoked
	case ended(t.EndsAt, r.now) && t.EndsAt.Equal(t.ExpiresAt):
		t.Ended = EndedExpired
	case ended(t.EndsAt, r.now):
		t.Ended = EndedIdle
	default:
		t.Ended = ""
	}
}

func scanToken(row scanner) (Token, error) {
	var t Token
	var created, expires, used, deleted sql.NullString
	var parent, madeBy sql.NullInt64
	if err := row.Scan(&t.ID, &t.UUID, &t.UserID, &t.Kind, &t.Label, &created, &expires, &used, &t.LastUsedFrom, &deleted, &t.Version, &parent, &madeBy,
		&t.RevokeKept, &t.IdleExempt, &t.hash); err != nil {
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
// it, for the integration's own key. The store makes the secret, so the
// row keeps the hash of its revoke code.
func (s *Store) addToken(ctx context.Context, userID int64, kind, label string, expires time.Time, ofIntegration bool) (Token, string, error) {
	secret, err := NewSecret()
	if err != nil {
		return Token{}, "", err
	}
	t, err := s.addTokenHash(ctx, userID, kind, HashToken(secret), revokeHashOf(secret), expires, KeyOptions{Label: label}, ofIntegration)
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
	// IdleExempt keeps the key from the idle limit (AP KeyLife, D7-A):
	// who may set it is the caller's question, the server's.
	IdleExempt bool
}

// AddKey creates an API key for a user with opts, without an end, and
// returns it with the secret, shown once; AddToken of a key is AddKey
// without a maker or an exclusion. The store makes the secret, so the
// row keeps the hash of its revoke code, as addToken's do. An
// integration's user is ErrIntegrationUser, as with AddToken.
func (s *Store) AddKey(ctx context.Context, userID int64, opts KeyOptions) (Token, string, error) {
	secret, err := NewSecret()
	if err != nil {
		return Token{}, "", err
	}
	t, err := s.addTokenHash(ctx, userID, TokenAPI, HashToken(secret), revokeHashOf(secret), time.Time{}, opts, false)
	return t, secret, err
}

// AddTokenHash creates a token whose secret the caller holds, as the
// bootstrap does with MASSIMO_API_TOKEN; an integration's user is
// ErrIntegrationUser, as with AddToken. The store never sees the
// secret, so the row keeps no revoke code's hash.
func (s *Store) AddTokenHash(ctx context.Context, userID int64, kind, label, hash string, expires time.Time) (Token, error) {
	return s.addTokenHash(ctx, userID, kind, hash, "", expires, KeyOptions{Label: label}, false)
}

// addTokenHash is AddTokenHash, with the hash of the token's revoke code,
// "" for none, its label, the key that made it, 0 for none, and its
// exclusion from the idle limit in opts (KeyOptions; the last two a key's
// alone), and addToken's ofIntegration. A maker that is no live API key
// under the rule alive is ErrNoToken, and nothing is made: the check and
// the insert are one statement, so that a request whose bearer was let in
// before its revoke, and makes its key after it, leaves no live key
// naming a revoked maker that no later revoke would reach.
func (s *Store) addTokenHash(ctx context.Context, userID int64, kind, hash, revokeHash string, expires time.Time, opts KeyOptions, ofIntegration bool) (Token, error) {
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
	madeBy, exempt := opts.MadeBy, opts.IdleExempt
	if kind != TokenAPI {
		madeBy, exempt = 0, false
	}
	r, err := s.rule(ctx, s.now())
	if err != nil {
		return Token{}, err
	}
	args := append([]any{newUUID(), userID, kind, hash, opts.Label, s.nowText(), nullTime(expires), s.nodeID, nullID(madeBy), exempt, nullText(revokeHash), madeBy, madeBy, TokenAPI}, r.args()...)
	res, err := s.db.ExecContext(ctx, `INSERT INTO tokens (uuid, user_id, kind, hash, label, created_at, expires_at, origin, made_by, idle_exempt, revoke_hash)
		SELECT ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ? WHERE ? = 0 OR EXISTS (SELECT 1`+tokenJoins+` WHERE t.id = ? AND t.kind = ? AND `+alive+`)`, args...)
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
// rule alive at the store's now: not revoked, not past its end, not an
// API key ended unused, its user not removed, and for a session made
// from a key, the key neither revoked, past its end nor ended unused;
// the token comes with its life (Token.EndsAt). An ended key is so
// refused before its use is written, and its use never brings it back
// (AP KeyLife, D6-A).
func (s *Store) TokenByHash(ctx context.Context, hash string) (Token, bool, error) {
	return s.liveToken(ctx, `t.hash = ?`, hash)
}

// TokenAlive returns a token by identifier while it lives, under the rule
// TokenByHash holds to, with its life; ok is false for one that does not,
// and for none. The event stream asks it at each keep-alive (the
// server's stillAdmitted).
func (s *Store) TokenAlive(ctx context.Context, id int64) (Token, bool, error) {
	return s.liveToken(ctx, `t.id = ?`, id)
}

// liveToken is the token where (of t, with its one argument) selects,
// while it lives under the rule alive at the store's now, with its life.
func (s *Store) liveToken(ctx context.Context, where string, arg any) (Token, bool, error) {
	r, err := s.rule(ctx, s.now())
	if err != nil {
		return Token{}, false, err
	}
	t, err := scanToken(s.db.QueryRowContext(ctx, `SELECT `+tokenColumns+tokenJoins+` WHERE `+where+` AND `+alive,
		append([]any{arg}, r.args()...)...))
	if errors.Is(err, sql.ErrNoRows) {
		return Token{}, false, nil
	}
	if err != nil {
		return Token{}, false, fmt.Errorf("store: token: %w", err)
	}
	r.apply(&t)
	return t, true, nil
}

// TokenEnded reports whether hash is the hash of a token the store issued
// that has ended by now, the rule alive no longer holding for it: revoked
// (deleted_at: a logout, a revoke, a removed user's tokens), past its end
// (expires_at), an API key ended unused (the idle limit), its user
// removed, or a session made from a key that was revoked, is past its end
// or ended unused. A live token, and a hash no token has, are not. The server asks it of a token it refuses, so that one it issued
// and that ended, which a client may still send, does not count toward
// the address's lockout as a guess does: a guessed secret never hits the
// hash of a token that was issued (docs/api-v1.md, "Authentication").
func (s *Store) TokenEnded(ctx context.Context, hash string, now time.Time) (bool, error) {
	r, err := s.rule(ctx, now)
	if err != nil {
		return false, err
	}
	var ended bool
	err = s.db.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1`+tokenJoins+` WHERE t.hash = ? AND NOT `+alive+`)`,
		append([]any{hash}, r.args()...)...).Scan(&ended)
	if err != nil {
		return false, fmt.Errorf("store: ended token: %w", err)
	}
	return ended, nil
}

// Tokens lists a user's live tokens, oldest first, each with its life
// under the rule alive at the store's now: a session under the rule, so
// one made from a key only while the key lives, and a key while it is not
// revoked, ended or not, so that it can still be revoked; an ended key
// says why (Token.Ended, AP KeyLife, D6-A).
func (s *Store) Tokens(ctx context.Context, userID int64) ([]Token, error) {
	r, err := s.rule(ctx, s.now())
	if err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, `SELECT `+tokenColumns+tokenJoins+` WHERE t.user_id = ? AND t.deleted_at IS NULL AND (t.kind = ? OR `+alive+`) ORDER BY t.id`,
		append([]any{userID, TokenAPI}, r.args()...)...)
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
		r.apply(&t)
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
	heirs, err := s.revokeToken(ctx, tx, id)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("store: token %d: %w", id, err)
	}
	return heirs, nil
}

// revokeToken is RevokeToken within tx, which RevokeByCode shares, so
// that a revoke by code ends what a revoke ends, the heirs included.
func (s *Store) revokeToken(ctx context.Context, tx *sql.Tx, id int64) ([]int64, error) {
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
	return heirs, nil
}

// revokeHeirs revokes in tx the heirs of the tokens roots selects (an SQL
// list or SELECT of ids, with its args): the keys made by them (made_by),
// and theirs, followed from key to key in any state, so that a key past
// its end, or one a binary before migration 0101 revoked alone, passes
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

// RevokeByCode revokes, as RevokeToken does, the token whose revoke
// code is code (RevokeCode), a key's sessions and heirs with it, and
// returns it as it was before with the heirs it revoked, oldest first,
// [] for none; ok false where none matched: no row of that code, one
// revoked already, or the settings file's key, by the hash Bootstrap last
// applied (settingsHash), which no revoke ends. A token past its end, and
// an API key ended unused, which an end leaves unrevoked (AP KeyLife,
// D6-A), are revoked all the same, so that what its revoke ends goes with
// it: the code is matched on deleted_at alone, not under the rule alive. The
// code is looked up by a plain read first, so that one that matches
// nothing, which anyone may post, takes no write transaction; a match is
// read again inside it.
func (s *Store) RevokeByCode(ctx context.Context, code string) (Token, []int64, bool, error) {
	const live = `SELECT ` + tokenColumns + ` FROM tokens t WHERE t.revoke_hash = ? AND t.deleted_at IS NULL AND t.hash <> ?`
	hash, keep := HashToken(code), s.settingsHash()
	if _, err := scanToken(s.db.QueryRowContext(ctx, live, hash, keep)); errors.Is(err, sql.ErrNoRows) {
		return Token{}, nil, false, nil
	} else if err != nil {
		return Token{}, nil, false, fmt.Errorf("store: revoke by code: %w", err)
	}
	if err := s.writable(); err != nil {
		return Token{}, nil, false, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Token{}, nil, false, fmt.Errorf("store: revoke by code: %w", err)
	}
	defer tx.Rollback()
	t, err := scanToken(tx.QueryRowContext(ctx, live, hash, keep))
	if errors.Is(err, sql.ErrNoRows) {
		return Token{}, nil, false, nil // revoked meanwhile
	}
	if err != nil {
		return Token{}, nil, false, fmt.Errorf("store: revoke by code: %w", err)
	}
	heirs, err := s.revokeToken(ctx, tx, t.ID)
	if err != nil {
		return Token{}, nil, false, err
	}
	if err := tx.Commit(); err != nil {
		return Token{}, nil, false, fmt.Errorf("store: token %d: %w", t.ID, err)
	}
	return t, heirs, true, nil
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
// is not a live API key under the rule alive: none, revoked, past its end
// or ended unused; ErrIntegrationUser where its user is an integration's,
// which makes no session.
func (s *Store) AddKeySession(ctx context.Context, keyID int64, label string, expires time.Time) (Token, string, error) {
	if err := s.writable(); err != nil {
		return Token{}, "", err
	}
	secret, err := NewSecret()
	if err != nil {
		return Token{}, "", err
	}
	r, err := s.rule(ctx, s.now()) // before the transaction, which holds the store's one connection
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
		append([]any{keyID, TokenAPI}, r.args()...)...).Scan(&userID)
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

// TokenLife is what SetTokenLife changes of an API key's life; a nil
// field keeps what the key has.
type TokenLife struct {
	// IdleExempt keeps the key from the idle limit (true) or holds it to
	// it (false); an integration's key takes no exclusion (AP
	// MobileApp, D75).
	IdleExempt *bool
}

// SetTokenLife changes the life of the API key id, not revoked, ended or
// not, since a key kept from the idle limit lives again (AP KeyLife,
// D6-A), and returns it with its life under the rule at the store's now.
// ErrNoToken where there is no such key: none, a session or revoked;
// ErrIntegrationUser where an integration's key is to be kept from the
// idle limit. Who may change it is the caller's question (the server's,
// D7-A).
func (s *Store) SetTokenLife(ctx context.Context, id int64, life TokenLife) (Token, error) {
	if err := s.writable(); err != nil {
		return Token{}, err
	}
	var integration sql.NullInt64
	err := s.db.QueryRowContext(ctx, `SELECT u.integration_id FROM tokens t JOIN users u ON u.id = t.user_id WHERE t.id = ? AND t.kind = ? AND t.deleted_at IS NULL`,
		id, TokenAPI).Scan(&integration)
	if errors.Is(err, sql.ErrNoRows) {
		return Token{}, fmt.Errorf("%w: key %d", ErrNoToken, id)
	}
	if err != nil {
		return Token{}, fmt.Errorf("store: token %d: %w", id, err)
	}
	if life.IdleExempt != nil && *life.IdleExempt && integration.Valid {
		return Token{}, fmt.Errorf("%w: its key %d is held to the idle limit", ErrIntegrationUser, id)
	}
	var exempt any
	if life.IdleExempt != nil {
		exempt = *life.IdleExempt
	}
	res, err := s.db.ExecContext(ctx, `UPDATE tokens SET idle_exempt = coalesce(?, idle_exempt), origin = ?, version = version + 1 WHERE id = ? AND kind = ? AND deleted_at IS NULL`,
		exempt, s.nodeID, id, TokenAPI)
	if err != nil {
		return Token{}, fmt.Errorf("store: token %d: %w", id, err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return Token{}, fmt.Errorf("%w: key %d", ErrNoToken, id)
	}
	r, err := s.rule(ctx, s.now())
	if err != nil {
		return Token{}, err
	}
	t, _, err := s.Token(ctx, id)
	if err != nil {
		return Token{}, err
	}
	r.apply(&t)
	return t, nil
}

// TouchToken records a use at now from an address and, for a session,
// moves its end to expires (zero keeps it); revokeHash, the hash of the
// token's revoke code, fills a row without one and leaves one written
// alone ("" writes none). The server calls it at most once an hour per
// token, and at the first use of a token without the hash; for a session
// made from a key it calls it for the key too, whose idle limit counts
// the session's use as its own (AP KeyLife, Block 3).
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
