// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package store

// Backups (docs/store.md section 9 (e)): the backup files of a Cube as
// the server took or received them, gzip in the row, with when, why and
// for whom. The server takes one after a change of the Cube's
// configuration (auto), by hand (manual), before every restore
// (before_restore), and keeps the file a restore was sent (upload).
// Retention is by reason and age, the newest of each Cube kept whatever
// their age; a deleted backup is a tombstone without its body.

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"time"
)

// The reasons a backup is taken for.
const (
	ReasonAuto          = "auto"           // by the server, after a change of the Cube's configuration
	ReasonManual        = "manual"         // by hand
	ReasonBeforeRestore = "before_restore" // by a restore, before its first step
	ReasonUpload        = "upload"         // the file a restore was sent
)

// The settings of the backups, whole numbers (D4 of AP StoreBackups).
const (
	BackupsAutoMinKey        = "backups.auto_min"         // minutes without a change before the automatic backup
	BackupsAutoMaxKey        = "backups.auto_max"         // minutes after the first change at the latest
	BackupsKeepDaysAutoKey   = "backups.keep_days_auto"   // days an automatic backup is kept; 0 keeps them
	BackupsKeepDaysManualKey = "backups.keep_days_manual" // days the others are kept; 0 keeps them
	BackupsKeepNewestKey     = "backups.keep_newest"      // backups of a Cube kept whatever their age
)

// The settings' defaults.
const (
	DefaultBackupsAutoMin        = 60
	DefaultBackupsAutoMax        = 120
	DefaultBackupsKeepDaysAuto   = 90
	DefaultBackupsKeepDaysManual = 365
	DefaultBackupsKeepNewest     = 10
)

// MaxBackupBytes is the largest backup file the store takes.
const MaxBackupBytes = 1 << 20

// ErrBadBackup is a backup the store refuses: an unknown reason or
// format, no Cube, no body or one too large.
var ErrBadBackup = errors.New("store: bad backup")

// Backup is one row of backups, without its body.
type Backup struct {
	ID       int64
	UUID     string
	CubeID   int64
	TakenAt  time.Time
	Reason   string
	Format   string // json or yaml, as the body is
	Tool     string // the software that wrote the file
	Size     int64  // bytes of the file
	SHA256   string // of the file, hex
	UserID   int64  // who asked; 0 for the server
	UserName string // the user's name, when the user is still there
	Note     string
}

// BackupSettings are the five settings with their defaults applied.
type BackupSettings struct {
	AutoMin, AutoMax             int // minutes
	KeepDaysAuto, KeepDaysManual int // days; 0 keeps
	KeepNewest                   int
}

// BackupSettings reads the backups' settings.
func (s *Store) BackupSettings(ctx context.Context) (BackupSettings, error) {
	out := BackupSettings{}
	for _, v := range []struct {
		key string
		def int
		to  *int
	}{
		{BackupsAutoMinKey, DefaultBackupsAutoMin, &out.AutoMin},
		{BackupsAutoMaxKey, DefaultBackupsAutoMax, &out.AutoMax},
		{BackupsKeepDaysAutoKey, DefaultBackupsKeepDaysAuto, &out.KeepDaysAuto},
		{BackupsKeepDaysManualKey, DefaultBackupsKeepDaysManual, &out.KeepDaysManual},
		{BackupsKeepNewestKey, DefaultBackupsKeepNewest, &out.KeepNewest},
	} {
		n, ok, err := s.SettingInt(ctx, v.key)
		if err != nil {
			return BackupSettings{}, err
		}
		if !ok {
			n = v.def
		}
		*v.to = n
	}
	return out, nil
}

func validReason(r string) bool {
	switch r {
	case ReasonAuto, ReasonManual, ReasonBeforeRestore, ReasonUpload:
		return true
	}
	return false
}

const backupColumns = `b.id, b.uuid, b.cube_id, b.taken_at, b.reason, b.format, b.tool, b.size, b.sha256, COALESCE(b.user_id, 0), COALESCE(u.name, ''), b.note`

const backupFrom = ` FROM backups b LEFT JOIN users u ON u.id = b.user_id`

func scanBackup(row scanner) (Backup, error) {
	var b Backup
	var taken sql.NullString
	if err := row.Scan(&b.ID, &b.UUID, &b.CubeID, &taken, &b.Reason, &b.Format, &b.Tool, &b.Size, &b.SHA256, &b.UserID, &b.UserName, &b.Note); err != nil {
		return Backup{}, err
	}
	b.TakenAt = parseTime(taken)
	return b, nil
}

// AddBackup stores a backup file of a Cube as given, gzip; b gives the
// Cube, the reason, the format, the tool, who asked and a note, and the
// time it was taken (now when zero).
func (s *Store) AddBackup(ctx context.Context, b Backup, body []byte) (Backup, error) {
	if err := s.writable(); err != nil {
		return Backup{}, err
	}
	if b.CubeID == 0 || !validReason(b.Reason) || (b.Format != "json" && b.Format != "yaml") || len(body) == 0 {
		return Backup{}, fmt.Errorf("%w: cube %d, reason %q, format %q, %d bytes", ErrBadBackup, b.CubeID, b.Reason, b.Format, len(body))
	}
	if len(body) > MaxBackupBytes {
		return Backup{}, fmt.Errorf("%w: %d bytes, at most %d", ErrBadBackup, len(body), MaxBackupBytes)
	}
	var z bytes.Buffer
	zw := gzip.NewWriter(&z)
	if _, err := zw.Write(body); err != nil {
		return Backup{}, fmt.Errorf("store: backup: %w", err)
	}
	if err := zw.Close(); err != nil {
		return Backup{}, fmt.Errorf("store: backup: %w", err)
	}
	sum := sha256.Sum256(body)
	at := b.TakenAt
	if at.IsZero() {
		at = s.now()
	}
	res, err := s.db.ExecContext(ctx, `INSERT INTO backups (uuid, cube_id, taken_at, reason, format, tool, size, sha256, body, user_id, note, origin)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		newUUID(), b.CubeID, formatTime(at), b.Reason, b.Format, b.Tool, len(body), hex.EncodeToString(sum[:]), z.Bytes(), nullID(b.UserID), b.Note, s.nodeID)
	if err != nil {
		return Backup{}, fmt.Errorf("store: backup: %w", err)
	}
	id, _ := res.LastInsertId()
	out, _, err := s.StoredBackup(ctx, b.CubeID, id)
	return out, err
}

// Backups returns a Cube's backups, newest first, without tombstones.
func (s *Store) Backups(ctx context.Context, cubeID int64) ([]Backup, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+backupColumns+backupFrom+` WHERE b.cube_id = ? AND b.deleted_at IS NULL ORDER BY b.taken_at DESC, b.id DESC`, cubeID)
	if err != nil {
		return nil, fmt.Errorf("store: backups: %w", err)
	}
	defer rows.Close()
	out := []Backup{}
	for rows.Next() {
		b, err := scanBackup(rows)
		if err != nil {
			return nil, fmt.Errorf("store: backups: %w", err)
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

// StoredBackup returns one of a Cube's backups, and whether it is there.
func (s *Store) StoredBackup(ctx context.Context, cubeID, id int64) (Backup, bool, error) {
	b, err := scanBackup(s.db.QueryRowContext(ctx, `SELECT `+backupColumns+backupFrom+` WHERE b.cube_id = ? AND b.id = ? AND b.deleted_at IS NULL`, cubeID, id))
	if errors.Is(err, sql.ErrNoRows) {
		return Backup{}, false, nil
	}
	if err != nil {
		return Backup{}, false, fmt.Errorf("store: backup %d: %w", id, err)
	}
	return b, true, nil
}

// NewestBackup returns a Cube's newest backup, and whether it has one.
func (s *Store) NewestBackup(ctx context.Context, cubeID int64) (Backup, bool, error) {
	b, err := scanBackup(s.db.QueryRowContext(ctx, `SELECT `+backupColumns+backupFrom+` WHERE b.cube_id = ? AND b.deleted_at IS NULL ORDER BY b.taken_at DESC, b.id DESC LIMIT 1`, cubeID))
	if errors.Is(err, sql.ErrNoRows) {
		return Backup{}, false, nil
	}
	if err != nil {
		return Backup{}, false, fmt.Errorf("store: newest backup: %w", err)
	}
	return b, true, nil
}

// BackupBody returns the file of one of a Cube's backups as it was
// given, and whether it is there.
func (s *Store) BackupBody(ctx context.Context, cubeID, id int64) ([]byte, bool, error) {
	var z []byte
	err := s.db.QueryRowContext(ctx, `SELECT body FROM backups WHERE cube_id = ? AND id = ? AND deleted_at IS NULL`, cubeID, id).Scan(&z)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("store: backup %d: %w", id, err)
	}
	zr, err := gzip.NewReader(bytes.NewReader(z))
	if err != nil {
		return nil, false, fmt.Errorf("store: backup %d: %w", id, err)
	}
	body, err := io.ReadAll(io.LimitReader(zr, MaxBackupBytes+1))
	if err != nil {
		return nil, false, fmt.Errorf("store: backup %d: %w", id, err)
	}
	return body, true, nil
}

// DeleteBackup removes one of a Cube's backups, leaving a tombstone
// without its body, and reports whether there was one.
func (s *Store) DeleteBackup(ctx context.Context, cubeID, id int64) (bool, error) {
	if err := s.writable(); err != nil {
		return false, err
	}
	res, err := s.db.ExecContext(ctx, `UPDATE backups SET deleted_at = ?, body = x'', origin = ?, version = version + 1 WHERE cube_id = ? AND id = ? AND deleted_at IS NULL`,
		s.nowText(), s.nodeID, cubeID, id)
	if err != nil {
		return false, fmt.Errorf("store: delete backup %d: %w", id, err)
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

// PruneBackups removes, per Cube, the backups past the newest
// backups.keep_newest that are older than their reason's days
// (backups.keep_days_auto for auto, backups.keep_days_manual for the
// others; 0 keeps them), and tombstones older than
// backups.keep_days_manual. It returns how many rows it removed.
func (s *Store) PruneBackups(ctx context.Context, now time.Time) (int, error) {
	if err := s.writable(); err != nil {
		return 0, err
	}
	set, err := s.BackupSettings(ctx)
	if err != nil {
		return 0, err
	}
	cutoff := func(days int) string {
		if days <= 0 {
			return ""
		}
		return formatTime(now.Add(-time.Duration(days) * 24 * time.Hour))
	}
	auto, manual := cutoff(set.KeepDaysAuto), cutoff(set.KeepDaysManual)
	rows, err := s.db.QueryContext(ctx, `SELECT id, cube_id, taken_at, reason FROM backups WHERE deleted_at IS NULL ORDER BY cube_id, taken_at DESC, id DESC`)
	if err != nil {
		return 0, fmt.Errorf("store: prune backups: %w", err)
	}
	var gone []int64
	seen := map[int64]int{}
	for rows.Next() {
		var id, cube int64
		var taken, reason string
		if err := rows.Scan(&id, &cube, &taken, &reason); err != nil {
			rows.Close()
			return 0, fmt.Errorf("store: prune backups: %w", err)
		}
		seen[cube]++
		if seen[cube] <= set.KeepNewest {
			continue
		}
		limit := manual
		if reason == ReasonAuto {
			limit = auto
		}
		if limit != "" && taken < limit {
			gone = append(gone, id)
		}
	}
	rows.Close()
	removed := 0
	for _, id := range gone {
		if _, err := s.db.ExecContext(ctx, `DELETE FROM backups WHERE id = ?`, id); err != nil {
			return removed, fmt.Errorf("store: prune backups: %w", err)
		}
		removed++
	}
	if manual != "" {
		res, err := s.db.ExecContext(ctx, `DELETE FROM backups WHERE deleted_at IS NOT NULL AND deleted_at < ?`, manual)
		if err != nil {
			return removed, fmt.Errorf("store: prune backups: %w", err)
		}
		n, _ := res.RowsAffected()
		removed += int(n)
	}
	if removed > 0 {
		s.log.Info("backups pruned", "removed", removed)
	}
	return removed, nil
}
