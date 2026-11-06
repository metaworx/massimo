// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package store

// The lockout (docs/store.md section 6): failed token, ticket and
// password checks counted per address and, for a login, per user name,
// in auth_failures, so that the counters survive a restart. The
// semantics are the server's authLimiter: a fixed window, a limit, a
// lockout; rows outside both are pruned on every write.

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// Lockout counts failures on a store with one set of limits.
type Lockout struct {
	s       *Store
	max     int
	window  time.Duration
	lockout time.Duration
}

// Lockout returns the counter with max failures within window locking
// a key out for lockout.
func (s *Store) Lockout(max int, window, lockout time.Duration) *Lockout {
	return &Lockout{s: s, max: max, window: window, lockout: lockout}
}

// LockoutAddr is the key of a client's address.
func LockoutAddr(ip string) string { return "addr:" + ip }

// LockoutUser is the key of a login name.
func LockoutUser(name string) string { return "user:" + NormalName(name) }

// LockedFor reports how long the key is still locked out, 0 if not.
func (l *Lockout) LockedFor(ctx context.Context, key string) (time.Duration, error) {
	var until sql.NullString
	err := l.s.db.QueryRowContext(ctx, `SELECT locked_until FROM auth_failures WHERE key = ?`, key).Scan(&until)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("store: lockout: %w", err)
	}
	if d := parseTime(until).Sub(l.s.now()); d > 0 {
		return d, nil
	}
	return 0, nil
}

// Fail records a failed check and reports whether it locked the key out.
func (l *Lockout) Fail(ctx context.Context, key string) (bool, error) {
	if err := l.s.writable(); err != nil {
		return false, err
	}
	now := l.s.now()
	tx, err := l.s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, fmt.Errorf("store: lockout: %w", err)
	}
	defer tx.Rollback()
	if err := l.prune(ctx, tx, now); err != nil {
		return false, err
	}
	var start, until sql.NullString
	var count int
	err = tx.QueryRowContext(ctx, `SELECT window_start, count, locked_until FROM auth_failures WHERE key = ?`, key).Scan(&start, &count, &until)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return false, fmt.Errorf("store: lockout: %w", err)
	}
	since := parseTime(start)
	if errors.Is(err, sql.ErrNoRows) || now.Sub(since) > l.window {
		since, count = now, 0
	}
	count++
	locked := false
	var lockedUntil any
	if count >= l.max {
		locked, lockedUntil = true, formatTime(now.Add(l.lockout))
		since, count = now, 0
	} else if parseTime(until).After(now) {
		lockedUntil = until.String
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO auth_failures (key, window_start, count, locked_until) VALUES (?, ?, ?, ?)
		ON CONFLICT (key) DO UPDATE SET window_start = excluded.window_start, count = excluded.count, locked_until = excluded.locked_until`,
		key, formatTime(since), count, lockedUntil); err != nil {
		return false, fmt.Errorf("store: lockout: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return false, fmt.Errorf("store: lockout: %w", err)
	}
	return locked, nil
}

// Succeed forgets the failures of a key that is not locked out.
func (l *Lockout) Succeed(ctx context.Context, key string) error {
	if err := l.s.writable(); err != nil {
		return err
	}
	if _, err := l.s.db.ExecContext(ctx, `DELETE FROM auth_failures WHERE key = ? AND (locked_until IS NULL OR locked_until < ?)`, key, formatTime(l.s.now())); err != nil {
		return fmt.Errorf("store: lockout: %w", err)
	}
	return nil
}

// prune removes the rows whose window and lockout are over.
func (l *Lockout) prune(ctx context.Context, tx *sql.Tx, now time.Time) error {
	if _, err := tx.ExecContext(ctx, `DELETE FROM auth_failures WHERE window_start < ? AND (locked_until IS NULL OR locked_until < ?)`, formatTime(now.Add(-l.window)), formatTime(now)); err != nil {
		return fmt.Errorf("store: lockout: %w", err)
	}
	return nil
}
