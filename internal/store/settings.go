// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"
)

// Keys of the server's settings. The durations are JSON strings in Go's
// notation ("30m", "24h").
const (
	StaleAfterKey = "stale_after"
	// StaleAfterWallKey is the wall thermostats' stale threshold;
	// StaleAfterKey the radiator thermostats'.
	StaleAfterWallKey   = "stale_after_wall"
	PollIntervalKey     = "poll_interval"
	RefreshIntervalKey  = "refresh_interval"
	DiscoverIntervalKey = "discover_interval"
	// DutyWaitKey is how long radio commands wait at a full duty cycle
	// before the Cube's budget is read again (engine.Options.DutyWait).
	DutyWaitKey = "duty_wait"
	// WindowWarnKey and WindowAlertKey are how long a window may stay
	// open before the web UI shows it amber, then red.
	WindowWarnKey  = "window_warn_after"
	WindowAlertKey = "window_alert_after"
)

// UpdateChannelKey is the update channel the box follows, one of
// update.Flavours as a JSON string; unset, it follows its build's flavour.
const UpdateChannelKey = "update.channel"

// UpdateAutoKey says whether the box installs a release by itself:
// UpdateAutoOff, the default, or UpdateAutoNight, in the window
// UpdateWindowKey names while the radio is quiet (AP AutoUpdate D1, D7).
const UpdateAutoKey = "update.auto"

// The values of UpdateAutoKey.
const (
	UpdateAutoOff   = "off"
	UpdateAutoNight = "night"
)

// UpdateWindowKey is the window of an automatic update, HH:MM-HH:MM in
// the server's time zone, at least 30 minutes, its end past midnight
// allowed; DefaultUpdateWindow where it is unset (AP AutoUpdate D2).
const UpdateWindowKey = "update.auto_window"

// DefaultUpdateWindow is the window of an automatic update where none is
// set: after Europe's switch hour, 02:00 to 03:00, so that a night never
// loses or repeats part of it.
const DefaultUpdateWindow = "03:00-05:00"

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

// SetSetting writes a setting for the server itself (SetSettingBy).
func (s *Store) SetSetting(ctx context.Context, key string, value any) error {
	return s.SetSettingBy(ctx, key, value, 0)
}

// SetSettingBy writes a setting for the user by (0: the server itself);
// value is marshalled to JSON unless it is a json.RawMessage. The row's
// version goes up, this node becomes its origin and by its updated_by.
func (s *Store) SetSettingBy(ctx context.Context, key string, value any, by int64) error {
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
	_, err := s.db.ExecContext(ctx, `INSERT INTO settings (key, value, updated_at, updated_by, origin) VALUES (?, ?, ?, ?, ?)
		ON CONFLICT (key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at, updated_by = excluded.updated_by,
			origin = excluded.origin, version = settings.version + 1, deleted_at = NULL`, key, string(raw), s.nowText(), nullID(by), s.nodeID)
	if err != nil {
		return fmt.Errorf("store: setting %s: %w", key, err)
	}
	return nil
}

// sealedSettings are the settings kept sealed (secret.go; AP
// SealedSecrets v1.0, Block 2): in the file, the JSON string of a sealed
// box, the setting's key its additional data. SecretSetting and
// SetSecretSetting read and write them.
var sealedSettings = []string{VAPIDKey}

// ErrSecretLost is a sealed setting that does not open: opaque.key lost,
// or another store's.
var ErrSecretLost = errors.New("store: a sealed secret does not open")

// SecretSetting returns a sealed setting opened, and whether it is set;
// ErrSecretLost where it does not open. One kept plain, from before the
// sealing, is answered as it is.
func (s *Store) SecretSetting(ctx context.Context, key string) (json.RawMessage, bool, error) {
	raw, ok, err := s.Setting(ctx, key)
	if err != nil || !ok {
		return nil, ok, err
	}
	var kept string
	if json.Unmarshal(raw, &kept) != nil {
		return raw, true, nil // plain: not a string
	}
	plain, lost := s.openSecret(key, kept)
	if lost {
		return nil, true, fmt.Errorf("%w: setting %s", ErrSecretLost, key)
	}
	if plain == kept {
		return raw, true, nil // a plain string
	}
	return json.RawMessage(plain), true, nil
}

// SetSecretSetting writes a setting sealed; value is marshalled to JSON
// unless it is a json.RawMessage.
func (s *Store) SetSecretSetting(ctx context.Context, key string, value any) error {
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
	sealed, err := s.sealSecret(key, string(raw))
	if err != nil {
		return err
	}
	return s.SetSetting(ctx, key, sealed)
}

// DeleteSetting removes a setting for the server itself
// (DeleteSettingBy).
func (s *Store) DeleteSetting(ctx context.Context, key string) error {
	return s.DeleteSettingBy(ctx, key, 0)
}

// DeleteSettingBy removes a setting for the user by (0: the server
// itself), leaving a tombstone (deleted_at) so that the removal can be
// replicated.
func (s *Store) DeleteSettingBy(ctx context.Context, key string, by int64) error {
	if err := s.writable(); err != nil {
		return err
	}
	now := s.nowText()
	_, err := s.db.ExecContext(ctx, `UPDATE settings SET deleted_at = ?, updated_at = ?, updated_by = ?, origin = ?, version = version + 1 WHERE key = ? AND deleted_at IS NULL`,
		now, now, nullID(by), s.nodeID, key)
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
	if err != nil && ctx.Err() == nil {
		s.log.Warn("store: the password sign-in setting not read; it stays on", "error", err)
	}
	return !ok || on
}

// PasswordRevealKey says whether the web UI's password fields offer their
// eye, the button that shows the password as it is typed (true by
// default, AP MobileApp D84-A): off, no field shows one. The page learns
// it before anyone signs in, from GET /healthz (docs/api-v1.md).
const PasswordRevealKey = "auth.password_reveal"

// PasswordReveal reports whether the password fields offer their eye; a
// setting that cannot be read counts as the default, on. GET /healthz
// reads it at every call: a request that went away before the read
// (its context cancelled) is no failure of the store's, and goes to no
// log.
func (s *Store) PasswordReveal(ctx context.Context) bool {
	on, ok, err := s.SettingBool(ctx, PasswordRevealKey)
	if err != nil && ctx.Err() == nil {
		s.log.Warn("store: the password reveal setting not read; it stays on", "error", err)
	}
	return !ok || on
}

// AnnounceKey says whether serve announces itself on the LAN by mDNS:
// AnnounceAuto (the default), on unless it listens on a loopback address;
// AnnounceOn or AnnounceOff. It applies at the next start, and -announce
// or MASSIMO_ANNOUNCE wins where given (AP AnnounceSetting).
const AnnounceKey = "announce"

// The values of AnnounceKey.
const (
	AnnounceAuto = "auto"
	AnnounceOn   = "on"
	AnnounceOff  = "off"
)

// ProxiedURLKey keeps the address a trusted proxy serves the server at,
// as the last authorized request through it said: the announcement's url
// where MASSIMO_PUBLIC_URL is not set. The server writes it; no API does.
const ProxiedURLKey = "server.proxied_url"

// IntegrationRequestsKey says whether integrations may ask to be
// registered, POST /api/v1/integrations/requests (true by default); the
// announcement's reg says it on the LAN.
const IntegrationRequestsKey = "integrations.requests"

// IntegrationRequests reports whether integrations may ask to be
// registered; a setting that cannot be read counts as the default, on.
func (s *Store) IntegrationRequests(ctx context.Context) bool {
	on, ok, err := s.SettingBool(ctx, IntegrationRequestsKey)
	if err != nil {
		s.log.Warn("store: the integrations' requests setting not read; it stays on", "error", err)
	}
	return !ok || on
}

// KeepOpenKey says that a server without regular users (CountUsers) is
// kept open on purpose (false by default): the web UI then does not ask
// for the first owner (AP FirstOwner, D11). It counts only while the
// server has no regular user, and goes with the first one: every
// transaction of the store that makes a regular user removes it
// (dropKeepOpen), and the server removes it at its start and at a
// reload where it finds one (DropKeepOpen); an integration's users leave
// it as it is (AP LooseEnds, D1). It changes nothing about who is let
// in.
const KeepOpenKey = "auth.keep_open"

// ErrHasUsers is a change that only a server without regular users
// takes: the setting KeepOpenKey switched on, or the first owner once
// there is one.
var ErrHasUsers = errors.New("store: the server has users")

// KeepOpen reports whether the server without regular users is kept
// open; a setting that cannot be read counts as the default, off.
func (s *Store) KeepOpen(ctx context.Context) bool {
	on, ok, err := s.SettingBool(ctx, KeepOpenKey)
	if err != nil {
		s.log.Warn("store: the keep-open setting not read; it counts as off", "error", err)
	}
	return ok && on
}

// SetKeepOpen writes the setting KeepOpenKey for the user by (0: nobody,
// as on an open server), in one transaction with the count of the
// regular users: on, it is ErrHasUsers where the server has any.
func (s *Store) SetKeepOpen(ctx context.Context, on bool, by int64) error {
	if err := s.writable(); err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("store: setting %s: %w", KeepOpenKey, err)
	}
	defer tx.Rollback()
	if on {
		n, err := countUsers(ctx, tx)
		if err != nil {
			return fmt.Errorf("store: setting %s: %w", KeepOpenKey, err)
		}
		if n > 0 {
			return fmt.Errorf("%w: %s counts only on a server without users", ErrHasUsers, KeepOpenKey)
		}
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO settings (key, value, updated_at, updated_by, origin) VALUES (?, ?, ?, ?, ?)
		ON CONFLICT (key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at, updated_by = excluded.updated_by,
			origin = excluded.origin, version = settings.version + 1, deleted_at = NULL`, KeepOpenKey, strconv.FormatBool(on), s.nowText(), nullID(by), s.nodeID); err != nil {
		return fmt.Errorf("store: setting %s: %w", KeepOpenKey, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("store: setting %s: %w", KeepOpenKey, err)
	}
	return nil
}

// dropKeepOpen removes the setting KeepOpenKey inside tx, a transaction
// that has just made a regular user: a tombstone, written by the server
// itself. An integration's users do not call it (AP LooseEnds, D1).
// It reports whether the setting was there; the caller logs it once tx
// is committed (keepOpenDropped).
func (s *Store) dropKeepOpen(ctx context.Context, tx execer) (bool, error) {
	now := s.nowText()
	res, err := tx.ExecContext(ctx, `UPDATE settings SET deleted_at = ?, updated_at = ?, updated_by = NULL, origin = ?, version = version + 1 WHERE key = ? AND deleted_at IS NULL`,
		now, now, s.nodeID, KeepOpenKey)
	if err != nil {
		return false, fmt.Errorf("store: setting %s: %w", KeepOpenKey, err)
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

// keepOpenDropped logs a removal of dropKeepOpen whose transaction is
// committed: why says what made the user, and who the key-value pairs
// that name it.
func (s *Store) keepOpenDropped(dropped bool, why string, who ...any) {
	if dropped {
		s.log.Info("keep-open setting removed: the server has a user", append([]any{"key", KeepOpenKey, "why", why}, who...)...)
	}
}

// DropKeepOpen removes the setting KeepOpenKey where the server has
// regular users, for a start and a reload: what another process, or an older
// version, left behind. It reports whether it removed it; a store
// opened read-only is left as it is.
func (s *Store) DropKeepOpen(ctx context.Context) (bool, error) {
	if s.writable() != nil {
		return false, nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, fmt.Errorf("store: setting %s: %w", KeepOpenKey, err)
	}
	defer tx.Rollback()
	n, err := countUsers(ctx, tx)
	if err != nil {
		return false, fmt.Errorf("store: setting %s: %w", KeepOpenKey, err)
	}
	if n == 0 {
		return false, nil
	}
	dropped, err := s.dropKeepOpen(ctx, tx)
	if err != nil {
		return false, err
	}
	if err := tx.Commit(); err != nil {
		return false, fmt.Errorf("store: setting %s: %w", KeepOpenKey, err)
	}
	if dropped {
		s.log.Info("keep-open setting removed: the server has users", "key", KeepOpenKey, "users", n)
	}
	return dropped, nil
}
