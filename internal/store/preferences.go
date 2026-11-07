// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package store

// Preferences (docs/store.md section 9 (c)): a user's settings of the
// web UI and colour schemes, a JSON document per key, so that they
// follow the user across browsers. The page uses the keys ui and
// schemes; the store takes any key of the pattern below. A removed
// preference is a tombstone, so that the removal can be replicated; the
// value is a content column, never filtered on.

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"time"
)

// MaxPreferenceBytes is the largest value a preference holds.
const MaxPreferenceBytes = 64 << 10

// Errors of the preferences.
var (
	ErrBadKey        = errors.New("store: bad preference key")
	ErrBadValue      = errors.New("store: a preference is a JSON document")
	ErrValueTooLarge = fmt.Errorf("store: a preference holds at most %d bytes", MaxPreferenceBytes)
)

// preferenceKey is a preference's key: lower case, 1 to 32 of letters,
// digits, '_', '.' and '-', starting with a letter.
var preferenceKey = regexp.MustCompile(`^[a-z][a-z0-9_.-]{0,31}$`)

// ValidPreferenceKey reports whether a key may name a preference.
func ValidPreferenceKey(key string) bool { return preferenceKey.MatchString(key) }

// Preference is one row of preferences.
type Preference struct {
	UserID    int64
	Key       string
	Value     json.RawMessage
	UpdatedAt time.Time
	Version   int64
	Origin    string
}

func scanPreference(row scanner) (Preference, error) {
	var p Preference
	var value string
	var updated sql.NullString
	if err := row.Scan(&p.UserID, &p.Key, &value, &updated, &p.Version, &p.Origin); err != nil {
		return Preference{}, err
	}
	p.Value, p.UpdatedAt = json.RawMessage(value), parseTime(updated)
	return p, nil
}

const preferenceColumns = `user_id, key, value, updated_at, version, origin`

// Preferences returns a user's preferences that are not removed, by key.
func (s *Store) Preferences(ctx context.Context, userID int64) ([]Preference, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+preferenceColumns+` FROM preferences WHERE user_id = ? AND deleted_at IS NULL ORDER BY key`, userID)
	if err != nil {
		return nil, fmt.Errorf("store: preferences: %w", err)
	}
	defer rows.Close()
	out := []Preference{}
	for rows.Next() {
		p, err := scanPreference(rows)
		if err != nil {
			return nil, fmt.Errorf("store: preferences: %w", err)
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// Preference returns one of a user's preferences, and whether it is set.
func (s *Store) Preference(ctx context.Context, userID int64, key string) (Preference, bool, error) {
	p, err := scanPreference(s.db.QueryRowContext(ctx, `SELECT `+preferenceColumns+` FROM preferences WHERE user_id = ? AND key = ? AND deleted_at IS NULL`, userID, key))
	if errors.Is(err, sql.ErrNoRows) {
		return Preference{}, false, nil
	}
	if err != nil {
		return Preference{}, false, fmt.Errorf("store: preference %s: %w", key, err)
	}
	return p, true, nil
}

// SetPreference writes a user's preference: a new row, or the row of
// the key with its version bumped and a tombstone cleared.
func (s *Store) SetPreference(ctx context.Context, userID int64, key string, value json.RawMessage) (Preference, error) {
	if err := s.writable(); err != nil {
		return Preference{}, err
	}
	if !ValidPreferenceKey(key) {
		return Preference{}, fmt.Errorf("%w: %q", ErrBadKey, key)
	}
	if len(value) > MaxPreferenceBytes {
		return Preference{}, ErrValueTooLarge
	}
	if len(value) == 0 || !json.Valid(value) {
		return Preference{}, ErrBadValue
	}
	if _, err := s.db.ExecContext(ctx, `INSERT INTO preferences (uuid, user_id, key, value, updated_at, origin) VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT (user_id, key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at, origin = excluded.origin,
			version = preferences.version + 1, deleted_at = NULL`,
		newUUID(), userID, key, string(value), s.nowText(), s.nodeID); err != nil {
		return Preference{}, fmt.Errorf("store: preference %s: %w", key, err)
	}
	p, _, err := s.Preference(ctx, userID, key)
	return p, err
}

// DeletePreference removes a user's preference, leaving a tombstone; a
// key that is not set is no error.
func (s *Store) DeletePreference(ctx context.Context, userID int64, key string) error {
	if err := s.writable(); err != nil {
		return err
	}
	if !ValidPreferenceKey(key) {
		return fmt.Errorf("%w: %q", ErrBadKey, key)
	}
	now := s.nowText()
	if _, err := s.db.ExecContext(ctx, `UPDATE preferences SET deleted_at = ?, updated_at = ?, origin = ?, version = version + 1 WHERE user_id = ? AND key = ? AND deleted_at IS NULL`,
		now, now, s.nodeID, userID, key); err != nil {
		return fmt.Errorf("store: preference %s: %w", key, err)
	}
	return nil
}

// removePreferences makes every live preference of a user a tombstone,
// within the transaction of the user's removal or takeover.
func (s *Store) removePreferences(ctx context.Context, tx *sql.Tx, userID int64, now string) error {
	if _, err := tx.ExecContext(ctx, `UPDATE preferences SET deleted_at = ?, updated_at = ?, origin = ?, version = version + 1 WHERE user_id = ? AND deleted_at IS NULL`,
		now, now, s.nodeID, userID); err != nil {
		return fmt.Errorf("store: preferences of user %d: %w", userID, err)
	}
	return nil
}
