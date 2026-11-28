// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package store

// The catalogue of the server's own backups (AP ServerBackup v1.0, Block
// 2; docs/store.md, "Server backups"; migration 0103): the keys a backup
// is encrypted for, the targets its copies go to, the backups and their
// copies. internal/serverbackup makes, reads and copies the files; the
// store keeps what is known of them. The catalogue is this server's: no
// row is replicated, and a row is deleted rather than marked.

import (
	"context"
	"crypto/subtle"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// PendingKeyLifetime is how long a key the server made waits for its
// confirmation, the last six characters of its identity typed back,
// before it goes (AP ServerBackup, D4-A).
const PendingKeyLifetime = time.Hour

// ServerBackupTimeKey is the setting of the daily backup's time, HH:MM in
// the box's local time; DefaultServerBackupTime where it is unset (D11-A).
const ServerBackupTimeKey = "serverbackup.time"

// DefaultServerBackupTime is the daily backup's time where none is set:
// after Europe's switch hour, so that a night never loses or repeats it.
const DefaultServerBackupTime = "03:30"

// The states of a server backup.
const (
	ServerBackupMade   = "made"
	ServerBackupFailed = "failed"
)

// The states of a backup's copy on a target.
const (
	CopyThere   = "there"
	CopyFailed  = "failed"
	CopyMissing = "missing"
)

// The errors of the catalogue.
var (
	ErrNoServerBackupKey        = errors.New("store: no such server backup key")
	ErrServerBackupKeyExists    = errors.New("store: that key is a server backup key already")
	ErrServerBackupKeyConfirm   = errors.New("store: those are not the key's last six characters")
	ErrNoServerBackupTarget     = errors.New("store: no such server backup target")
	ErrServerBackupTargetExists = errors.New("store: a server backup target of that name exists")
	ErrNoServerBackup           = errors.New("store: no such server backup")
)

// ServerBackupKey is a recipient the server's backups are encrypted for.
type ServerBackupKey struct {
	ID   int64
	UUID string
	// Label is the owner's name for it, in plain in every manifest.
	Label string
	// Recipient is the public key, as serverbackup.ParseRecipient writes
	// it; Fingerprint and Kind are its.
	Recipient, Fingerprint, Kind string
	// Pending is a key the server made whose last six characters have not
	// been typed back yet; it encrypts nothing until they are, and goes
	// after PendingKeyLifetime.
	Pending   bool
	CreatedAt time.Time
	CreatedBy int64
}

const serverBackupKeyColumns = `id, uuid, label, recipient, fingerprint, kind, confirm_hash, created_at, coalesce(created_by, 0)`

func scanServerBackupKey(row scanner) (ServerBackupKey, string, error) {
	var k ServerBackupKey
	var hash string
	var created sql.NullString
	if err := row.Scan(&k.ID, &k.UUID, &k.Label, &k.Recipient, &k.Fingerprint, &k.Kind, &hash, &created, &k.CreatedBy); err != nil {
		return ServerBackupKey{}, "", err
	}
	k.CreatedAt, k.Pending = parseTime(created), hash != ""
	return k, hash, nil
}

// dropPendingKeys removes the keys made whose confirmation did not come
// within PendingKeyLifetime.
func (s *Store) dropPendingKeys(ctx context.Context) error {
	if s.readOnly {
		return nil
	}
	_, err := s.db.ExecContext(ctx, `DELETE FROM server_backup_keys WHERE confirm_hash != '' AND created_at < ?`, formatTime(s.now().Add(-PendingKeyLifetime)))
	if err != nil {
		return fmt.Errorf("store: server backup keys: %w", err)
	}
	return nil
}

// ServerBackupKeys lists the keys, the pending ones included, oldest
// first; a pending key past PendingKeyLifetime is removed first.
func (s *Store) ServerBackupKeys(ctx context.Context) ([]ServerBackupKey, error) {
	if err := s.dropPendingKeys(ctx); err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, `SELECT `+serverBackupKeyColumns+` FROM server_backup_keys ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("store: server backup keys: %w", err)
	}
	defer rows.Close()
	out := []ServerBackupKey{}
	for rows.Next() {
		k, _, err := scanServerBackupKey(rows)
		if err != nil {
			return nil, fmt.Errorf("store: server backup keys: %w", err)
		}
		out = append(out, k)
	}
	return out, rows.Err()
}

// serverBackupKey returns a key by uuid, with its confirmation's hash.
func (s *Store) serverBackupKey(ctx context.Context, uuid string) (ServerBackupKey, string, error) {
	k, hash, err := scanServerBackupKey(s.db.QueryRowContext(ctx, `SELECT `+serverBackupKeyColumns+` FROM server_backup_keys WHERE uuid = ?`, uuid))
	if errors.Is(err, sql.ErrNoRows) {
		return ServerBackupKey{}, "", fmt.Errorf("%w: %s", ErrNoServerBackupKey, uuid)
	}
	if err != nil {
		return ServerBackupKey{}, "", fmt.Errorf("store: server backup key %s: %w", uuid, err)
	}
	return k, hash, nil
}

// AddServerBackupKey keeps a recipient, by the user by (0 for none): a
// key at once where confirmHash is "", else a pending key, which
// ConfirmServerBackupKey with the same hash makes a key. A fingerprint
// kept already is ErrServerBackupKeyExists. The caller checks the
// recipient and its kind (serverbackup.ParseRecipient, CheckKinds).
func (s *Store) AddServerBackupKey(ctx context.Context, k ServerBackupKey, confirmHash string, by int64) (ServerBackupKey, error) {
	if err := s.writable(); err != nil {
		return ServerBackupKey{}, err
	}
	if err := s.dropPendingKeys(ctx); err != nil {
		return ServerBackupKey{}, err
	}
	if k.Recipient == "" || k.Fingerprint == "" || k.Kind == "" {
		return ServerBackupKey{}, errors.New("store: a server backup key needs its recipient, fingerprint and kind")
	}
	id := newUUID()
	_, err := s.db.ExecContext(ctx, `INSERT INTO server_backup_keys (uuid, label, recipient, fingerprint, kind, confirm_hash, created_at, created_by) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		id, k.Label, k.Recipient, k.Fingerprint, k.Kind, confirmHash, s.nowText(), nullID(by))
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			return ServerBackupKey{}, fmt.Errorf("%w: %s", ErrServerBackupKeyExists, k.Fingerprint)
		}
		return ServerBackupKey{}, fmt.Errorf("store: add server backup key: %w", err)
	}
	out, _, err := s.serverBackupKey(ctx, id)
	return out, err
}

// ConfirmServerBackupKey makes a pending key a key, where hash is the
// one it was added with (compared in constant time); a key confirmed
// already stays one. A wrong hash is ErrServerBackupKeyConfirm, a key
// that is not there, or whose hour is over, ErrNoServerBackupKey.
func (s *Store) ConfirmServerBackupKey(ctx context.Context, uuid, hash string) (ServerBackupKey, error) {
	if err := s.writable(); err != nil {
		return ServerBackupKey{}, err
	}
	if err := s.dropPendingKeys(ctx); err != nil {
		return ServerBackupKey{}, err
	}
	k, want, err := s.serverBackupKey(ctx, uuid)
	if err != nil {
		return ServerBackupKey{}, err
	}
	if !k.Pending {
		return k, nil
	}
	if hash == "" || subtle.ConstantTimeCompare([]byte(hash), []byte(want)) != 1 {
		return ServerBackupKey{}, ErrServerBackupKeyConfirm
	}
	if _, err := s.db.ExecContext(ctx, `UPDATE server_backup_keys SET confirm_hash = '' WHERE id = ?`, k.ID); err != nil {
		return ServerBackupKey{}, fmt.Errorf("store: server backup key %s: %w", uuid, err)
	}
	k.Pending = false
	return k, nil
}

// DeleteServerBackupKey removes a key, pending or not: the backups made
// later are not encrypted for it, those made before still are.
func (s *Store) DeleteServerBackupKey(ctx context.Context, uuid string) error {
	if err := s.writable(); err != nil {
		return err
	}
	res, err := s.db.ExecContext(ctx, `DELETE FROM server_backup_keys WHERE uuid = ?`, uuid)
	if err != nil {
		return fmt.Errorf("store: server backup key %s: %w", uuid, err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("%w: %s", ErrNoServerBackupKey, uuid)
	}
	return nil
}

// ServerBackupTarget is a place the copies of the server's backups go.
type ServerBackupTarget struct {
	ID   int64
	UUID string
	// Name is the owner's, unique without regard to case.
	Name string
	// Kind is a serverbackup.Kind's name; Config its fields but the
	// secret one, which is Secret, opened.
	Kind   string
	Config map[string]string
	Secret string
	// SecretLost is a sealed secret that does not open: opaque.key gone,
	// or another store's. It is entered again.
	SecretLost bool
	// Takes is every, daily, weekly or monthly (serverbackup.Takes);
	// Retention the JSON of a serverbackup.Policy.
	Takes     string
	Retention json.RawMessage
	Enabled   bool
	CreatedAt time.Time
	UpdatedAt time.Time
	UpdatedBy int64

	kept string // the secret as kept, sealed
}

const serverBackupTargetColumns = `id, uuid, name, kind, config, secret, takes, retention, enabled, created_at, updated_at, coalesce(updated_by, 0)`

func (s *Store) scanServerBackupTarget(row scanner) (ServerBackupTarget, error) {
	var t ServerBackupTarget
	var config, retention string
	var created, updated sql.NullString
	if err := row.Scan(&t.ID, &t.UUID, &t.Name, &t.Kind, &config, &t.kept, &t.Takes, &retention, &t.Enabled, &created, &updated, &t.UpdatedBy); err != nil {
		return ServerBackupTarget{}, err
	}
	t.Config = map[string]string{}
	if err := json.Unmarshal([]byte(config), &t.Config); err != nil {
		return ServerBackupTarget{}, fmt.Errorf("target %s: config: %w", t.UUID, err)
	}
	t.Retention = json.RawMessage(retention)
	t.CreatedAt, t.UpdatedAt = parseTime(created), parseTime(updated)
	t.Secret, t.SecretLost = s.openSecret(t.UUID, t.kept)
	return t, nil
}

// ServerBackupTargets lists the targets, oldest first.
func (s *Store) ServerBackupTargets(ctx context.Context) ([]ServerBackupTarget, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+serverBackupTargetColumns+` FROM server_backup_targets ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("store: server backup targets: %w", err)
	}
	defer rows.Close()
	out := []ServerBackupTarget{}
	for rows.Next() {
		t, err := s.scanServerBackupTarget(rows)
		if err != nil {
			return nil, fmt.Errorf("store: server backup targets: %w", err)
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// ServerBackupTarget returns a target by uuid.
func (s *Store) ServerBackupTarget(ctx context.Context, uuid string) (ServerBackupTarget, bool, error) {
	t, err := s.scanServerBackupTarget(s.db.QueryRowContext(ctx, `SELECT `+serverBackupTargetColumns+` FROM server_backup_targets WHERE uuid = ?`, uuid))
	if errors.Is(err, sql.ErrNoRows) {
		return ServerBackupTarget{}, false, nil
	}
	if err != nil {
		return ServerBackupTarget{}, false, fmt.Errorf("store: server backup target %s: %w", uuid, err)
	}
	return t, true, nil
}

// checkTarget refuses a target the table cannot take.
func checkTarget(t *ServerBackupTarget) error {
	t.Name = strings.TrimSpace(t.Name)
	switch {
	case t.Name == "":
		return errors.New("store: a server backup target needs a name")
	case t.Kind == "":
		return errors.New("store: a server backup target needs a kind")
	}
	switch t.Takes {
	case "":
		t.Takes = "every"
	case "every", "daily", "weekly", "monthly":
	default:
		return fmt.Errorf("store: a target takes every backup, or the first of each day, week or month: not %q", t.Takes)
	}
	if len(t.Retention) == 0 {
		t.Retention = json.RawMessage(`{}`)
	}
	if !json.Valid(t.Retention) {
		return errors.New("store: a target's retention is not JSON")
	}
	if t.Config == nil {
		t.Config = map[string]string{}
	}
	return nil
}

// AddServerBackupTarget keeps a new target, by the user by, its secret
// sealed with its uuid as the additional data. A name taken, without
// regard to case, is ErrServerBackupTargetExists.
func (s *Store) AddServerBackupTarget(ctx context.Context, t ServerBackupTarget, by int64) (ServerBackupTarget, error) {
	if err := s.writable(); err != nil {
		return ServerBackupTarget{}, err
	}
	if err := checkTarget(&t); err != nil {
		return ServerBackupTarget{}, err
	}
	t.UUID = newUUID()
	sealed, err := s.sealSecret(t.UUID, t.Secret)
	if err != nil {
		return ServerBackupTarget{}, err
	}
	config, _ := json.Marshal(t.Config)
	now := s.nowText()
	_, err = s.db.ExecContext(ctx, `INSERT INTO server_backup_targets (uuid, name, kind, config, secret, takes, retention, enabled, created_at, updated_at, updated_by)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		t.UUID, t.Name, t.Kind, string(config), sealed, t.Takes, string(t.Retention), t.Enabled, now, now, nullID(by))
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			return ServerBackupTarget{}, fmt.Errorf("%w: %s", ErrServerBackupTargetExists, t.Name)
		}
		return ServerBackupTarget{}, fmt.Errorf("store: add server backup target: %w", err)
	}
	out, _, err := s.ServerBackupTarget(ctx, t.UUID)
	return out, err
}

// UpdateServerBackupTarget changes a target through change, by the user
// by: its uuid and kind stay. A secret that was lost and is not entered
// again stays as it was kept, which a key found again opens.
func (s *Store) UpdateServerBackupTarget(ctx context.Context, uuid string, change func(*ServerBackupTarget) error, by int64) (ServerBackupTarget, error) {
	if err := s.writable(); err != nil {
		return ServerBackupTarget{}, err
	}
	t, ok, err := s.ServerBackupTarget(ctx, uuid)
	if err != nil {
		return ServerBackupTarget{}, err
	}
	if !ok {
		return ServerBackupTarget{}, fmt.Errorf("%w: %s", ErrNoServerBackupTarget, uuid)
	}
	kind := t.Kind
	if err := change(&t); err != nil {
		return ServerBackupTarget{}, err
	}
	t.UUID, t.Kind = uuid, kind
	if err := checkTarget(&t); err != nil {
		return ServerBackupTarget{}, err
	}
	sealed := t.kept
	if !t.SecretLost || t.Secret != "" {
		if sealed, err = s.sealSecret(t.UUID, t.Secret); err != nil {
			return ServerBackupTarget{}, err
		}
	}
	config, _ := json.Marshal(t.Config)
	_, err = s.db.ExecContext(ctx, `UPDATE server_backup_targets SET name = ?, config = ?, secret = ?, takes = ?, retention = ?, enabled = ?, updated_at = ?, updated_by = ? WHERE id = ?`,
		t.Name, string(config), sealed, t.Takes, string(t.Retention), t.Enabled, s.nowText(), nullID(by), t.ID)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			return ServerBackupTarget{}, fmt.Errorf("%w: %s", ErrServerBackupTargetExists, t.Name)
		}
		return ServerBackupTarget{}, fmt.Errorf("store: server backup target %s: %w", uuid, err)
	}
	out, _, err := s.ServerBackupTarget(ctx, uuid)
	return out, err
}

// DeleteServerBackupTarget removes a target and its copies from the
// catalogue; the files stay where they are. A backup left without a copy
// goes with it.
func (s *Store) DeleteServerBackupTarget(ctx context.Context, uuid string) error {
	if err := s.writable(); err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("store: server backup target %s: %w", uuid, err)
	}
	defer tx.Rollback()
	res, err := tx.ExecContext(ctx, `DELETE FROM server_backup_targets WHERE uuid = ?`, uuid)
	if err != nil {
		return fmt.Errorf("store: server backup target %s: %w", uuid, err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("%w: %s", ErrNoServerBackupTarget, uuid)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM server_backups WHERE state = 'made' AND id NOT IN (SELECT backup_id FROM server_backup_copies)`); err != nil {
		return fmt.Errorf("store: server backup target %s: %w", uuid, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("store: server backup target %s: %w", uuid, err)
	}
	return nil
}

// ServerBackup is one of the server's backups: a file made once, the
// same on every target, or a backup that failed, with its retries.
type ServerBackup struct {
	ID int64
	// UUID is the manifest's, the backup's key everywhere.
	UUID string
	// Time is the manifest's created; a failed one's last try.
	Time time.Time
	// Trigger is the manifest's, or uploaded for a file that came in
	// by upload (serverbackup.Trigger).
	Trigger string
	// UserID is who asked, 0 for the schedule, a scan or the server.
	UserID int64
	State  string
	// Size and SHA256 are the file's, hex; SHA256 is "" for a backup a
	// scan found, until a check has read it whole.
	Size   int64
	SHA256 string
	// Manifest is manifest.json as it was signed, and Signature
	// manifest.sig (serverbackup.Raw); both empty for a failed one.
	Manifest  json.RawMessage
	Signature string
	// PinnedUntil keeps it from every retention until then; zero: not
	// pinned.
	PinnedUntil time.Time
	// Error, Tries and NextTry are a failed one's: its last error, its
	// tries, and when it is tried again, zero for never.
	Error   string
	Tries   int
	NextTry time.Time
	Copies  []ServerBackupCopy
}

// Pinned reports whether the backup is pinned at now.
func (b ServerBackup) Pinned(now time.Time) bool {
	return !b.PinnedUntil.IsZero() && now.Before(b.PinnedUntil)
}

// Copy returns the backup's copy on the target of a uuid.
func (b ServerBackup) Copy(targetUUID string) (ServerBackupCopy, bool) {
	for _, c := range b.Copies {
		if c.TargetUUID == targetUUID {
			return c, true
		}
	}
	return ServerBackupCopy{}, false
}

// ServerBackupCopy is a backup's copy on a target.
type ServerBackupCopy struct {
	TargetID   int64
	TargetUUID string
	// Path is the file's name on the target.
	Path string
	// State is there, failed or missing (CopyThere, …).
	State string
	// Pulled is a copy pulled to the target by hand: it stays until it is
	// deleted, whatever the retention (D12-A).
	Pulled    bool
	PushedAt  time.Time
	CheckedAt time.Time
	Error     string
	// Tries and NextTry are a failed copy's.
	Tries   int
	NextTry time.Time
}

const serverBackupColumns = `id, uuid, time, trigger, coalesce(user_id, 0), state, size, sha256, manifest, signature, pinned_until, error, tries, next_try`

func scanServerBackup(row scanner) (ServerBackup, error) {
	var b ServerBackup
	var at, pinned, next sql.NullString
	var manifest string
	if err := row.Scan(&b.ID, &b.UUID, &at, &b.Trigger, &b.UserID, &b.State, &b.Size, &b.SHA256, &manifest, &b.Signature, &pinned, &b.Error, &b.Tries, &next); err != nil {
		return ServerBackup{}, err
	}
	b.Time, b.PinnedUntil, b.NextTry = parseTime(at), parseTime(pinned), parseTime(next)
	if manifest != "" {
		b.Manifest = json.RawMessage(manifest)
	}
	b.Copies = []ServerBackupCopy{}
	return b, nil
}

// ServerBackups lists the backups, newest first, each with its copies in
// the order of their targets.
func (s *Store) ServerBackups(ctx context.Context) ([]ServerBackup, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+serverBackupColumns+` FROM server_backups ORDER BY time DESC, id DESC`)
	if err != nil {
		return nil, fmt.Errorf("store: server backups: %w", err)
	}
	out := []ServerBackup{}
	byID := map[int64]int{}
	for rows.Next() {
		b, err := scanServerBackup(rows)
		if err != nil {
			rows.Close()
			return nil, fmt.Errorf("store: server backups: %w", err)
		}
		byID[b.ID] = len(out)
		out = append(out, b)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: server backups: %w", err)
	}
	cs, err := s.copies(ctx, 0)
	if err != nil {
		return nil, err
	}
	for id, list := range cs {
		if i, ok := byID[id]; ok {
			out[i].Copies = list
		}
	}
	return out, nil
}

// ServerBackup returns a backup by uuid, with its copies.
func (s *Store) ServerBackup(ctx context.Context, uuid string) (ServerBackup, bool, error) {
	b, err := scanServerBackup(s.db.QueryRowContext(ctx, `SELECT `+serverBackupColumns+` FROM server_backups WHERE uuid = ?`, uuid))
	if errors.Is(err, sql.ErrNoRows) {
		return ServerBackup{}, false, nil
	}
	if err != nil {
		return ServerBackup{}, false, fmt.Errorf("store: server backup %s: %w", uuid, err)
	}
	cs, err := s.copies(ctx, b.ID)
	if err != nil {
		return ServerBackup{}, false, err
	}
	if list := cs[b.ID]; list != nil {
		b.Copies = list
	}
	return b, true, nil
}

// copies reads the copies of one backup, or of every one with id 0, by
// backup.
func (s *Store) copies(ctx context.Context, id int64) (map[int64][]ServerBackupCopy, error) {
	q := `SELECT c.backup_id, c.target_id, t.uuid, c.path, c.state, c.pulled, c.pushed_at, c.checked_at, c.error, c.tries, c.next_try
		FROM server_backup_copies c JOIN server_backup_targets t ON t.id = c.target_id`
	args := []any{}
	if id != 0 {
		q += ` WHERE c.backup_id = ?`
		args = append(args, id)
	}
	rows, err := s.db.QueryContext(ctx, q+` ORDER BY c.target_id`, args...)
	if err != nil {
		return nil, fmt.Errorf("store: server backup copies: %w", err)
	}
	defer rows.Close()
	out := map[int64][]ServerBackupCopy{}
	for rows.Next() {
		var c ServerBackupCopy
		var backup int64
		var pushed, checked, next sql.NullString
		if err := rows.Scan(&backup, &c.TargetID, &c.TargetUUID, &c.Path, &c.State, &c.Pulled, &pushed, &checked, &c.Error, &c.Tries, &next); err != nil {
			return nil, fmt.Errorf("store: server backup copies: %w", err)
		}
		c.PushedAt, c.CheckedAt, c.NextTry = parseTime(pushed), parseTime(checked), parseTime(next)
		out[backup] = append(out[backup], c)
	}
	return out, rows.Err()
}

// SaveServerBackup keeps a backup by its uuid: a new row, or the row of
// that uuid replaced, its copies kept. A backup that is made needs its
// size, hash and manifest.
func (s *Store) SaveServerBackup(ctx context.Context, b ServerBackup) (ServerBackup, error) {
	if err := s.writable(); err != nil {
		return ServerBackup{}, err
	}
	if b.UUID == "" {
		return ServerBackup{}, errors.New("store: a server backup needs its uuid")
	}
	if b.State == "" {
		b.State = ServerBackupMade
	}
	if b.State == ServerBackupMade && (b.Size <= 0 || len(b.Manifest) == 0) {
		return ServerBackup{}, fmt.Errorf("store: server backup %s: a backup made needs its size and manifest", b.UUID)
	}
	if b.Time.IsZero() {
		b.Time = s.now()
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO server_backups (uuid, time, trigger, user_id, state, size, sha256, manifest, signature, pinned_until, error, tries, next_try)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (uuid) DO UPDATE SET time = excluded.time, trigger = excluded.trigger, user_id = excluded.user_id, state = excluded.state,
			size = excluded.size, sha256 = excluded.sha256, manifest = excluded.manifest, signature = excluded.signature,
			pinned_until = excluded.pinned_until, error = excluded.error, tries = excluded.tries, next_try = excluded.next_try`,
		b.UUID, formatTime(b.Time), b.Trigger, nullID(b.UserID), b.State, b.Size, b.SHA256, string(b.Manifest), b.Signature, nullTime(b.PinnedUntil), b.Error, b.Tries, nullTime(b.NextTry))
	if err != nil {
		return ServerBackup{}, fmt.Errorf("store: server backup %s: %w", b.UUID, err)
	}
	out, _, err := s.ServerBackup(ctx, b.UUID)
	return out, err
}

// PinServerBackup keeps a backup from every retention until until, zero
// to unpin it.
func (s *Store) PinServerBackup(ctx context.Context, uuid string, until time.Time) error {
	if err := s.writable(); err != nil {
		return err
	}
	res, err := s.db.ExecContext(ctx, `UPDATE server_backups SET pinned_until = ? WHERE uuid = ?`, nullTime(until), uuid)
	if err != nil {
		return fmt.Errorf("store: server backup %s: %w", uuid, err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("%w: %s", ErrNoServerBackup, uuid)
	}
	return nil
}

// DeleteServerBackup removes a backup and its copies from the catalogue.
func (s *Store) DeleteServerBackup(ctx context.Context, uuid string) error {
	if err := s.writable(); err != nil {
		return err
	}
	res, err := s.db.ExecContext(ctx, `DELETE FROM server_backups WHERE uuid = ?`, uuid)
	if err != nil {
		return fmt.Errorf("store: server backup %s: %w", uuid, err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("%w: %s", ErrNoServerBackup, uuid)
	}
	return nil
}

// SaveServerBackupCopy keeps a backup's copy on a target, both by uuid:
// a new row, or the row replaced.
func (s *Store) SaveServerBackupCopy(ctx context.Context, backupUUID string, c ServerBackupCopy) error {
	if err := s.writable(); err != nil {
		return err
	}
	switch c.State {
	case CopyThere, CopyFailed, CopyMissing:
	default:
		return fmt.Errorf("store: a copy's state %q", c.State)
	}
	var backup, target int64
	if err := s.db.QueryRowContext(ctx, `SELECT id FROM server_backups WHERE uuid = ?`, backupUUID).Scan(&backup); errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("%w: %s", ErrNoServerBackup, backupUUID)
	} else if err != nil {
		return fmt.Errorf("store: server backup %s: %w", backupUUID, err)
	}
	if err := s.db.QueryRowContext(ctx, `SELECT id FROM server_backup_targets WHERE uuid = ?`, c.TargetUUID).Scan(&target); errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("%w: %s", ErrNoServerBackupTarget, c.TargetUUID)
	} else if err != nil {
		return fmt.Errorf("store: server backup target %s: %w", c.TargetUUID, err)
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO server_backup_copies (backup_id, target_id, path, state, pulled, pushed_at, checked_at, error, tries, next_try)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (backup_id, target_id) DO UPDATE SET path = excluded.path, state = excluded.state, pulled = excluded.pulled,
			pushed_at = excluded.pushed_at, checked_at = excluded.checked_at, error = excluded.error, tries = excluded.tries, next_try = excluded.next_try`,
		backup, target, c.Path, c.State, c.Pulled, nullTime(c.PushedAt), nullTime(c.CheckedAt), c.Error, c.Tries, nullTime(c.NextTry))
	if err != nil {
		return fmt.Errorf("store: server backup %s on %s: %w", backupUUID, c.TargetUUID, err)
	}
	return nil
}

// DeleteServerBackupCopy removes a backup's copy on a target from the
// catalogue, and the backup with it where that was its last copy and it
// is made. It reports whether there was such a copy.
func (s *Store) DeleteServerBackupCopy(ctx context.Context, backupUUID, targetUUID string) (bool, error) {
	if err := s.writable(); err != nil {
		return false, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, fmt.Errorf("store: server backup %s on %s: %w", backupUUID, targetUUID, err)
	}
	defer tx.Rollback()
	res, err := tx.ExecContext(ctx, `DELETE FROM server_backup_copies
		WHERE backup_id = (SELECT id FROM server_backups WHERE uuid = ?) AND target_id = (SELECT id FROM server_backup_targets WHERE uuid = ?)`, backupUUID, targetUUID)
	if err != nil {
		return false, fmt.Errorf("store: server backup %s on %s: %w", backupUUID, targetUUID, err)
	}
	n, _ := res.RowsAffected()
	if _, err := tx.ExecContext(ctx, `DELETE FROM server_backups WHERE uuid = ? AND state = 'made' AND id NOT IN (SELECT backup_id FROM server_backup_copies)`, backupUUID); err != nil {
		return false, fmt.Errorf("store: server backup %s: %w", backupUUID, err)
	}
	if err := tx.Commit(); err != nil {
		return false, fmt.Errorf("store: server backup %s on %s: %w", backupUUID, targetUUID, err)
	}
	return n > 0, nil
}
