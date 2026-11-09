// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package store

// Push subscriptions (docs/store.md section 5): the browsers and
// apps a user's notifications go to by Web Push, each tied to the
// session or API key that subscribed, so that revoking it, or removing
// the user, removes the subscription. A row is removed, not marked: a
// subscription is bound to this server's VAPID key and is not
// replicated.

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// VAPIDKey is the setting that holds the server's VAPID key pair (RFC
// 8292), made at its first use; GET /api/v1/settings does not show it.
const VAPIDKey = "push.vapid"

// ErrNoPushSubscription is the error of a subscription that does not
// exist, or is another user's.
var ErrNoPushSubscription = errors.New("store: no such push subscription")

// PushSubscription is one row of push_subscriptions.
type PushSubscription struct {
	ID       int64
	UUID     string
	UserID   int64
	TokenID  int64
	Endpoint string
	// P256dh and Auth are the subscriber's keys, base64url (RFC 8291).
	P256dh, Auth string
	Label        string
	CreatedAt    time.Time
	LastOKAt     time.Time // zero: none yet
	Failures     int
}

const pushColumns = `p.id, p.uuid, p.user_id, p.token_id, p.endpoint, p.p256dh, p.auth, p.label, p.created_at, p.last_ok_at, p.failures`

func scanPush(row scanner) (PushSubscription, error) {
	var p PushSubscription
	var created, ok sql.NullString
	if err := row.Scan(&p.ID, &p.UUID, &p.UserID, &p.TokenID, &p.Endpoint, &p.P256dh, &p.Auth, &p.Label, &created, &ok, &p.Failures); err != nil {
		return PushSubscription{}, err
	}
	p.CreatedAt, p.LastOKAt = parseTime(created), parseTime(ok)
	return p, nil
}

// AddPushSubscription keeps a subscription, and reports whether it is
// new: a new row, or the row of its endpoint, which then belongs to the
// user and token given, with the keys and label given, its delivery
// record cleared.
func (s *Store) AddPushSubscription(ctx context.Context, p PushSubscription) (PushSubscription, bool, error) {
	if err := s.writable(); err != nil {
		return PushSubscription{}, false, err
	}
	var id int64
	err := s.db.QueryRowContext(ctx, `SELECT id FROM push_subscriptions WHERE endpoint = ?`, p.Endpoint).Scan(&id)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		res, err := s.db.ExecContext(ctx, `INSERT INTO push_subscriptions (uuid, user_id, token_id, endpoint, p256dh, auth, label, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			newUUID(), p.UserID, p.TokenID, p.Endpoint, p.P256dh, p.Auth, p.Label, s.nowText())
		if err != nil {
			return PushSubscription{}, false, fmt.Errorf("store: add push subscription: %w", err)
		}
		id, _ = res.LastInsertId()
		got, _, err := s.PushSubscription(ctx, id)
		return got, true, err
	case err != nil:
		return PushSubscription{}, false, fmt.Errorf("store: add push subscription: %w", err)
	}
	if _, err := s.db.ExecContext(ctx, `UPDATE push_subscriptions SET user_id = ?, token_id = ?, p256dh = ?, auth = ?, label = ?, last_ok_at = NULL, failures = 0 WHERE id = ?`,
		p.UserID, p.TokenID, p.P256dh, p.Auth, p.Label, id); err != nil {
		return PushSubscription{}, false, fmt.Errorf("store: push subscription %d: %w", id, err)
	}
	got, _, err := s.PushSubscription(ctx, id)
	return got, false, err
}

// PushSubscription returns a subscription by identifier.
func (s *Store) PushSubscription(ctx context.Context, id int64) (PushSubscription, bool, error) {
	p, err := scanPush(s.db.QueryRowContext(ctx, `SELECT `+pushColumns+` FROM push_subscriptions p WHERE p.id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return PushSubscription{}, false, nil
	}
	if err != nil {
		return PushSubscription{}, false, fmt.Errorf("store: push subscription %d: %w", id, err)
	}
	return p, true, nil
}

// PushSubscriptions lists a user's subscriptions, oldest first; with
// userID 0, every subscription that may be sent to: its token neither
// revoked nor expired, its user neither removed, disabled nor expired.
func (s *Store) PushSubscriptions(ctx context.Context, userID int64) ([]PushSubscription, error) {
	q := `SELECT ` + pushColumns + `, t.revoked_at, t.expires_at, u.deleted_at, u.disabled, u.expires_at
		FROM push_subscriptions p JOIN tokens t ON t.id = p.token_id JOIN users u ON u.id = p.user_id`
	args := []any{}
	if userID != 0 {
		q += ` WHERE p.user_id = ?`
		args = append(args, userID)
	}
	rows, err := s.db.QueryContext(ctx, q+` ORDER BY p.id`, args...)
	if err != nil {
		return nil, fmt.Errorf("store: push subscriptions: %w", err)
	}
	defer rows.Close()
	now := s.now()
	out := []PushSubscription{}
	for rows.Next() {
		var p PushSubscription
		var created, ok, revoked, tokenEnd, deleted, userEnd sql.NullString
		var disabled bool
		if err := rows.Scan(&p.ID, &p.UUID, &p.UserID, &p.TokenID, &p.Endpoint, &p.P256dh, &p.Auth, &p.Label, &created, &ok, &p.Failures,
			&revoked, &tokenEnd, &deleted, &disabled, &userEnd); err != nil {
			return nil, fmt.Errorf("store: push subscriptions: %w", err)
		}
		p.CreatedAt, p.LastOKAt = parseTime(created), parseTime(ok)
		live := !revoked.Valid && !deleted.Valid && !disabled && !ended(parseTime(tokenEnd), now) && !ended(parseTime(userEnd), now)
		if userID != 0 || live {
			out = append(out, p)
		}
	}
	return out, rows.Err()
}

// ended says whether an end, zero for none, has passed at now.
func ended(end, now time.Time) bool { return !end.IsZero() && !now.Before(end) }

// DeletePushSubscription removes one of a user's subscriptions;
// ErrNoPushSubscription when the user has none of that identifier.
func (s *Store) DeletePushSubscription(ctx context.Context, userID, id int64) error {
	if err := s.writable(); err != nil {
		return err
	}
	res, err := s.db.ExecContext(ctx, `DELETE FROM push_subscriptions WHERE id = ? AND user_id = ?`, id, userID)
	if err != nil {
		return fmt.Errorf("store: push subscription %d: %w", id, err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("%w: %d", ErrNoPushSubscription, id)
	}
	return nil
}

// RemovePushSubscription removes a subscription whoever's it is, as a
// push service that answers 404 or 410 asks.
func (s *Store) RemovePushSubscription(ctx context.Context, id int64) error {
	if err := s.writable(); err != nil {
		return err
	}
	if _, err := s.db.ExecContext(ctx, `DELETE FROM push_subscriptions WHERE id = ?`, id); err != nil {
		return fmt.Errorf("store: push subscription %d: %w", id, err)
	}
	return nil
}

// NotePushResult records a delivery: one the push service took clears
// the failures and sets last_ok_at, one it did not counts a failure.
func (s *Store) NotePushResult(ctx context.Context, id int64, ok bool) error {
	if err := s.writable(); err != nil {
		return err
	}
	q, args := `UPDATE push_subscriptions SET failures = failures + 1 WHERE id = ?`, []any{id}
	if ok {
		q, args = `UPDATE push_subscriptions SET last_ok_at = ?, failures = 0 WHERE id = ?`, []any{s.nowText(), id}
	}
	if _, err := s.db.ExecContext(ctx, q, args...); err != nil {
		return fmt.Errorf("store: push subscription %d: %w", id, err)
	}
	return nil
}

// removePushSubscriptions removes subscriptions within a transaction:
// those of a token, or with token 0, every one of a user.
func removePushSubscriptions(ctx context.Context, tx execer, userID, tokenID int64) error {
	q, args := `DELETE FROM push_subscriptions WHERE user_id = ?`, []any{userID}
	if tokenID != 0 {
		q, args = `DELETE FROM push_subscriptions WHERE token_id = ?`, []any{tokenID}
	}
	_, err := tx.ExecContext(ctx, q, args...)
	return err
}

// execer is what runs a statement: the database or a transaction.
type execer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}
