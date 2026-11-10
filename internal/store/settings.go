// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// Keys of the server's settings. The durations are JSON strings in Go's
// notation ("30m", "24h").
const (
	StaleAfterKey       = "stale_after"
	PollIntervalKey     = "poll_interval"
	RefreshIntervalKey  = "refresh_interval"
	DiscoverIntervalKey = "discover_interval"
)

// UpdateChannelKey is the update channel the box follows, one of
// update.Flavours as a JSON string; unset, it follows its build's flavour.
const UpdateChannelKey = "update.channel"

// Setting is one row of settings.
type Setting struct {
	Key       string
	Value     json.RawMessage // JSON
	UpdatedAt time.Time
	Version   int64
	Origin    string // node.id of the node that changed it last
}

// Settings returns every setting that is not deleted, by key.
func (s *Store) Settings(ctx context.Context) ([]Setting, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT key, value, updated_at, version, origin FROM settings WHERE deleted_at IS NULL ORDER BY key`)
	if err != nil {
		return nil, fmt.Errorf("store: settings: %w", err)
	}
	defer rows.Close()
	var out []Setting
	for rows.Next() {
		var st Setting
		var v string
		var at sql.NullString
		if err := rows.Scan(&st.Key, &v, &at, &st.Version, &st.Origin); err != nil {
			return nil, fmt.Errorf("store: settings: %w", err)
		}
		st.Value, st.UpdatedAt = json.RawMessage(v), parseTime(at)
		out = append(out, st)
	}
	return out, rows.Err()
}

// Setting returns the value of one setting, and whether it is set.
func (s *Store) Setting(ctx context.Context, key string) (json.RawMessage, bool, error) {
	var v string
	err := s.db.QueryRowContext(ctx, `SELECT value FROM settings WHERE key = ? AND deleted_at IS NULL`, key).Scan(&v)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("store: setting %s: %w", key, err)
	}
	return json.RawMessage(v), true, nil
}

// SettingString returns a setting that holds a JSON string, "" when it
// is not set.
func (s *Store) SettingString(ctx context.Context, key string) (string, error) {
	v, ok, err := s.Setting(ctx, key)
	if err != nil || !ok {
		return "", err
	}
	var str string
	if err := json.Unmarshal(v, &str); err != nil {
		return "", fmt.Errorf("store: setting %s: %w", key, err)
	}
	return str, nil
}

// SettingDuration returns a duration setting, and whether it is set.
func (s *Store) SettingDuration(ctx context.Context, key string) (time.Duration, bool, error) {
	str, err := s.SettingString(ctx, key)
	if err != nil || str == "" {
		return 0, false, err
	}
	d, err := time.ParseDuration(str)
	if err != nil {
		return 0, false, fmt.Errorf("store: setting %s: %w", key, err)
	}
	return d, true, nil
}

// SetSetting writes a setting; value is marshalled to JSON unless it is
// a json.RawMessage. The row's version goes up and this node becomes
// its origin.
func (s *Store) SetSetting(ctx context.Context, key string, value any) error {
	if err := s.writable(); err != nil {
		return err
	}
	raw, ok := value.(json.RawMessage)
	if !ok {
		b, err := json.Marshal(value)
		if err != nil {
			return fmt.Errorf("store: setting %s: %w", key, err)
		}
		raw = b
	}
	if !json.Valid(raw) {
		return fmt.Errorf("store: setting %s: not JSON", key)
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO settings (key, value, updated_at, origin) VALUES (?, ?, ?, ?)
		ON CONFLICT (key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at, origin = excluded.origin,
			version = settings.version + 1, deleted_at = NULL`, key, string(raw), s.nowText(), s.nodeID)
	if err != nil {
		return fmt.Errorf("store: setting %s: %w", key, err)
	}
	return nil
}

// DeleteSetting removes a setting, leaving a tombstone (deleted_at) so
// that the removal can be replicated.
func (s *Store) DeleteSetting(ctx context.Context, key string) error {
	if err := s.writable(); err != nil {
		return err
	}
	now := s.nowText()
	_, err := s.db.ExecContext(ctx, `UPDATE settings SET deleted_at = ?, updated_at = ?, origin = ?, version = version + 1 WHERE key = ? AND deleted_at IS NULL`, now, now, s.nodeID, key)
	if err != nil {
		return fmt.Errorf("store: setting %s: %w", key, err)
	}
	return nil
}

// SettingInt returns a setting that holds a JSON number, and whether it
// is set.
func (s *Store) SettingInt(ctx context.Context, key string) (int, bool, error) {
	v, ok, err := s.Setting(ctx, key)
	if err != nil || !ok {
		return 0, false, err
	}
	var n int
	if err := json.Unmarshal(v, &n); err != nil {
		return 0, false, fmt.Errorf("store: setting %s: %w", key, err)
	}
	return n, true, nil
}

// SettingBool returns a setting that holds a JSON boolean, and whether
// it is set.
func (s *Store) SettingBool(ctx context.Context, key string) (bool, bool, error) {
	v, ok, err := s.Setting(ctx, key)
	if err != nil || !ok {
		return false, false, err
	}
	var b bool
	if err := json.Unmarshal(v, &b); err != nil {
		return false, false, fmt.Errorf("store: setting %s: %w", key, err)
	}
	return b, true, nil
}

// PasswordLoginKey says whether the server takes the password sign-in,
// POST /api/v1/login (true by default): off, every user signs in by
// OPAQUE, and no password crosses (docs/store.md section 6).
const PasswordLoginKey = "auth.password_login"

// PasswordLogin reports whether the server takes the password sign-in;
// a setting that cannot be read counts as the default, on.
func (s *Store) PasswordLogin(ctx context.Context) bool {
	on, ok, err := s.SettingBool(ctx, PasswordLoginKey)
	if err != nil {
		s.log.Warn("store: the password sign-in setting not read; it stays on", "error", err)
	}
	return !ok || on
}
