// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package store

// The audit (docs/store.md section 5): every write, who, when, what,
// the refused ones included; append-only, and the command it belongs
// to by command_id. Nothing in detail is a secret: the server removes
// passwords and tokens before the row is written. The events' writer
// prunes the rows older than audit.keep_days beside the log (events.go).

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

// The results of an audited request.
const (
	ResultOK      = "ok"
	ResultRefused = "refused"
	ResultFailed  = "failed"
)

// RemoteCLI is the remote of a row written by a massimo command rather
// than a client.
const RemoteCLI = "cli"

// AuditKeepDaysKey is the setting that says how many days the audit's
// rows are kept; 0 keeps them, unset is DefaultAuditKeepDays.
const AuditKeepDaysKey = "audit.keep_days"

// DefaultAuditKeepDays is the audit's retention when the setting is
// unset (AP OneLog v1.1, D7-A): a year.
const DefaultAuditKeepDays = 365

// AuditKeepDays returns the audit's retention in days; 0 keeps it.
func (s *Store) AuditKeepDays(ctx context.Context) (int, error) {
	n, ok, err := s.SettingInt(ctx, AuditKeepDaysKey)
	if err != nil {
		return 0, err
	}
	if !ok {
		return DefaultAuditKeepDays, nil
	}
	return n, nil
}

// PruneAudit removes the audit's rows older than audit.keep_days before
// now, in chunks, and returns how many it removed.
func (s *Store) PruneAudit(ctx context.Context, now time.Time) (int, error) {
	return s.pruneAudit(ctx, now, nil)
}

// pruneAudit is PruneAudit with between, when given, run after each
// chunk that removed rows; the prune stops where it returns false. A
// chunk walks audit_time from its start.
func (s *Store) pruneAudit(ctx context.Context, now time.Time, between func() bool) (int, error) {
	if err := s.writable(); err != nil {
		return 0, err
	}
	days, err := s.AuditKeepDays(ctx)
	if err != nil || days <= 0 {
		return 0, err
	}
	cutoff := formatTime(now.Add(-time.Duration(days) * 24 * time.Hour))
	removed, stopped := 0, false
	for {
		res, err := s.db.ExecContext(ctx, `DELETE FROM audit WHERE id IN (SELECT id FROM audit WHERE time < ? LIMIT ?)`, cutoff, pruneChunk)
		if err != nil {
			return removed, fmt.Errorf("store: prune audit: %w", err)
		}
		n, _ := res.RowsAffected()
		removed += int(n)
		if n == 0 {
			break
		}
		if between != nil && !between() {
			stopped = true
			break
		}
	}
	if removed > 0 || stopped {
		s.log.Info("audit pruned", "removed", removed, "keep_days", days, "stopped", stopped)
	}
	if removed > 0 {
		data, _ := json.Marshal(map[string]any{"kind": "audit_pruned", "source": "system", "level": "info",
			"summary": map[string]any{"removed": removed, "keep_days": days}, "time": now})
		s.AddEvent(Event{Kind: "audit_pruned", Source: "system", Level: LevelInfo, Time: now, Data: data})
	}
	return removed, nil
}

// AuditEntry is one row of audit, with the user's name and the Cube's
// serial joined in.
type AuditEntry struct {
	ID        int64
	UUID      string
	Time      time.Time
	UserID    int64  // 0: no authentication, or the command line
	User      string // the login name, "" for none
	TokenID   int64
	Remote    string
	Action    string
	CubeID    int64
	Cube      string // the serial, "" for none
	Target    string
	Detail    json.RawMessage
	Result    string
	CommandID int64
}

// Audit writes a row; the time is e.Time, or now.
func (s *Store) Audit(ctx context.Context, e AuditEntry) (int64, error) {
	if err := s.writable(); err != nil {
		return 0, err
	}
	switch e.Result {
	case ResultOK, ResultRefused, ResultFailed:
	case "":
		e.Result = ResultOK
	default:
		return 0, fmt.Errorf("store: audit result %q", e.Result)
	}
	at := e.Time
	if at.IsZero() {
		at = s.now()
	}
	detail := "{}"
	if len(e.Detail) > 0 {
		if !json.Valid(e.Detail) {
			return 0, fmt.Errorf("store: audit detail is not JSON")
		}
		detail = string(e.Detail)
	}
	res, err := s.db.ExecContext(ctx, `INSERT INTO audit (uuid, time, user_id, token_id, remote, action, cube_id, target, detail, result, command_id) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		newUUID(), formatTime(at), nullID(e.UserID), nullID(e.TokenID), e.Remote, e.Action, nullID(e.CubeID), e.Target, detail, e.Result, nullID(e.CommandID))
	if err != nil {
		return 0, fmt.Errorf("store: audit: %w", err)
	}
	id, _ := res.LastInsertId()
	return id, nil
}

// AuditList returns the newest rows first: those before a time (zero:
// from the newest), at most limit (at most 1000), and those of one
// user's name when given.
func (s *Store) AuditList(ctx context.Context, before time.Time, limit int, user string) ([]AuditEntry, error) {
	if limit <= 0 || limit > 1000 {
		limit = 1000
	}
	bound := ""
	if !before.IsZero() {
		bound = formatTime(before)
	}
	user = NormalName(user)
	return s.auditRows(ctx, `(? = '' OR a.time < ?) AND (? = '' OR u.name = ?)`, bound, bound, user, user, limit)
}

// AuditOfCommand returns the audit's row of a command, the request that
// is the command; false for none (a proxy client's, or one pruned).
func (s *Store) AuditOfCommand(ctx context.Context, commandID int64) (AuditEntry, bool, error) {
	rows, err := s.auditRows(ctx, `a.command_id = ?`, commandID, 1)
	if err != nil || len(rows) == 0 {
		return AuditEntry{}, false, err
	}
	return rows[0], true, nil
}

// auditRows reads the audit's rows of a WHERE, with the user's name and
// the Cube's serial joined in, newest first; args end with the limit.
func (s *Store) auditRows(ctx context.Context, where string, args ...any) ([]AuditEntry, error) {
	// where is the callers' own text, never input.
	rows, err := s.db.QueryContext(ctx, `SELECT a.id, a.uuid, a.time, a.user_id, coalesce(u.name, ''), a.token_id, a.remote, a.action, a.cube_id, coalesce(c.serial, ''), a.target, a.detail, a.result, a.command_id
		FROM audit a LEFT JOIN users u ON u.id = a.user_id LEFT JOIN cubes c ON c.id = a.cube_id
		WHERE `+where+` ORDER BY a.id DESC LIMIT ?`, args...)
	if err != nil {
		return nil, fmt.Errorf("store: audit: %w", err)
	}
	defer rows.Close()
	out := []AuditEntry{}
	for rows.Next() {
		var e AuditEntry
		var at sql.NullString
		var userID, tokenID, cubeID, commandID sql.NullInt64
		var detail string
		if err := rows.Scan(&e.ID, &e.UUID, &at, &userID, &e.User, &tokenID, &e.Remote, &e.Action, &cubeID, &e.Cube, &e.Target, &detail, &e.Result, &commandID); err != nil {
			return nil, fmt.Errorf("store: audit: %w", err)
		}
		e.Time, e.UserID, e.TokenID, e.CubeID, e.CommandID = parseTime(at), userID.Int64, tokenID.Int64, cubeID.Int64, commandID.Int64
		e.Detail = json.RawMessage(detail)
		out = append(out, e)
	}
	return out, rows.Err()
}
