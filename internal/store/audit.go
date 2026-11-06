// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package store

// The audit (docs/store.md section 5): every write, who, when, what,
// the refused ones included; append-only, and the command it belongs
// to by command_id. Nothing in detail is a secret: the server removes
// passwords and tokens before the row is written.

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
	rows, err := s.db.QueryContext(ctx, `SELECT a.id, a.uuid, a.time, a.user_id, coalesce(u.name, ''), a.token_id, a.remote, a.action, a.cube_id, coalesce(c.serial, ''), a.target, a.detail, a.result, a.command_id
		FROM audit a LEFT JOIN users u ON u.id = a.user_id LEFT JOIN cubes c ON c.id = a.cube_id
		WHERE (? = '' OR a.time < ?) AND (? = '' OR u.name = ?) ORDER BY a.id DESC LIMIT ?`, bound, bound, user, user, limit)
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
