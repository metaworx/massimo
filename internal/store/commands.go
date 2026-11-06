// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package store

// Commands (docs/server.md, "What version 1 prepares", item 7; NOTE
// StoreUsers v1.0: commands history): every write request is a row, the
// parent, and every engine.Command it enqueues is a row beneath it, a
// step, written by the queue's observer. A parent goes queued → sent
// with its first step and takes its final state from the HTTP outcome;
// a step takes its states from the engine. The client's Idempotency-Key
// is kept per user for 24 hours, so that a retried write answers what
// the first one did; finished rows stay for commands.keep_days, 0 for
// ever.

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// The states of a command, docs/design.md section 5.
const (
	StateQueued       = "queued"
	StateSent         = "sent"
	StateAcknowledged = "acknowledged"
	StateFailed       = "failed"
	StateDropped      = "dropped"
)

// ActionStep is the action of a step row.
const ActionStep = "step"

// CommandsKeepDaysKey is the setting that says how many days finished
// commands are kept; 0 or unset keeps them.
const CommandsKeepDaysKey = "commands.keep_days"

// keyKeep is how long an idempotency key answers a retry.
const keyKeep = 24 * time.Hour

// Errors of the commands.
var (
	ErrNoCommand = errors.New("store: no such command")
	ErrKeyExists = errors.New("store: idempotency key in use")
)

// Command is one row of commands; a parent carries its Steps when read
// by Command or CommandByUUID.
type Command struct {
	ID         int64
	UUID       string
	ParentID   int64 // 0 for a parent
	CubeID     int64
	UserID     int64
	TokenID    int64
	Key        string // the Idempotency-Key; "" for none or cleared
	Action     string
	Target     string
	Request    json.RawMessage
	State      string
	Status     int // the HTTP status answered; 0 while running, and for a step
	Result     json.RawMessage
	CreatedAt  time.Time
	UpdatedAt  time.Time
	FinishedAt time.Time
	Version    int64
	Origin     string
	Steps      []Command
}

// Final reports whether a state is one a command does not leave.
func Final(state string) bool {
	return state == StateAcknowledged || state == StateFailed || state == StateDropped
}

func validState(state string) bool {
	switch state {
	case StateQueued, StateSent, StateAcknowledged, StateFailed, StateDropped:
		return true
	}
	return false
}

// jsonText is a JSON document as the store writes it; empty is {}.
func jsonText(raw json.RawMessage) (string, error) {
	if len(raw) == 0 {
		return "{}", nil
	}
	if !json.Valid(raw) {
		return "", errors.New("store: not JSON")
	}
	return string(raw), nil
}

const commandColumns = `c.id, c.uuid, c.parent_id, c.cube_id, c.user_id, c.token_id, c.idempotency_key, c.action, c.target, c.request, c.state, c.status, c.result,
	c.created_at, c.updated_at, c.finished_at, c.version, c.origin`

func scanCommand(row scanner) (Command, error) {
	var c Command
	var parent, cube, user, token sql.NullInt64
	var request, result string
	var created, updated, finished sql.NullString
	if err := row.Scan(&c.ID, &c.UUID, &parent, &cube, &user, &token, &c.Key, &c.Action, &c.Target, &request, &c.State, &c.Status, &result,
		&created, &updated, &finished, &c.Version, &c.Origin); err != nil {
		return Command{}, err
	}
	c.ParentID, c.CubeID, c.UserID, c.TokenID = parent.Int64, cube.Int64, user.Int64, token.Int64
	c.Request, c.Result = json.RawMessage(request), json.RawMessage(result)
	c.CreatedAt, c.UpdatedAt, c.FinishedAt = parseTime(created), parseTime(updated), parseTime(finished)
	return c, nil
}

// AddCommand inserts a parent row in state queued with c's CubeID,
// UserID, TokenID, Key, Action, Target and Request. A Key the same user
// holds already answers that row with ErrKeyExists, so that a retried
// request is answered rather than run again.
func (s *Store) AddCommand(ctx context.Context, c Command) (Command, error) {
	if err := s.writable(); err != nil {
		return Command{}, err
	}
	request, err := jsonText(c.Request)
	if err != nil {
		return Command{}, fmt.Errorf("store: command request: %w", err)
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Command{}, fmt.Errorf("store: add command: %w", err)
	}
	defer tx.Rollback()
	now := s.now()
	if err := s.pruneCommands(ctx, tx, now); err != nil {
		return Command{}, err
	}
	if c.Key != "" {
		have, err := scanCommand(tx.QueryRowContext(ctx, `SELECT `+commandColumns+` FROM commands c WHERE coalesce(c.user_id, 0) = ? AND c.idempotency_key = ? AND c.parent_id IS NULL ORDER BY c.id DESC LIMIT 1`, c.UserID, c.Key))
		if err == nil {
			tx.Rollback()
			out, _, err := s.Command(ctx, have.ID)
			if err != nil {
				return Command{}, err
			}
			return out, ErrKeyExists
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return Command{}, fmt.Errorf("store: add command: %w", err)
		}
	}
	at := formatTime(now)
	res, err := tx.ExecContext(ctx, `INSERT INTO commands (uuid, cube_id, user_id, token_id, idempotency_key, action, target, request, state, created_at, updated_at, origin)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		newUUID(), nullID(c.CubeID), nullID(c.UserID), nullID(c.TokenID), c.Key, c.Action, c.Target, request, StateQueued, at, at, s.nodeID)
	if err != nil {
		return Command{}, fmt.Errorf("store: add command: %w", err)
	}
	id, _ := res.LastInsertId()
	if err := tx.Commit(); err != nil {
		return Command{}, fmt.Errorf("store: add command: %w", err)
	}
	out, _, err := s.Command(ctx, id)
	return out, err
}

// AddStep inserts a step beneath a parent, in state queued, with the
// parent's Cube, user and token and c's Target and Request.
func (s *Store) AddStep(ctx context.Context, parentID int64, c Command) (Command, error) {
	if err := s.writable(); err != nil {
		return Command{}, err
	}
	request, err := jsonText(c.Request)
	if err != nil {
		return Command{}, fmt.Errorf("store: step request: %w", err)
	}
	at := s.nowText()
	res, err := s.db.ExecContext(ctx, `INSERT INTO commands (uuid, parent_id, cube_id, user_id, token_id, action, target, request, state, created_at, updated_at, origin)
		SELECT ?, id, cube_id, user_id, token_id, ?, ?, ?, ?, ?, ?, ? FROM commands WHERE id = ? AND parent_id IS NULL`,
		newUUID(), ActionStep, c.Target, request, StateQueued, at, at, s.nodeID, parentID)
	if err != nil {
		return Command{}, fmt.Errorf("store: add step: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return Command{}, fmt.Errorf("%w: parent %d", ErrNoCommand, parentID)
	}
	id, _ := res.LastInsertId()
	out, _, err := s.Command(ctx, id)
	return out, err
}

// SetCommandState moves a command to a state with the status answered
// (0 for a step, below 0 to keep the one it has) and the result; a final
// state sets finished_at. A step going sent takes its parent from
// queued to sent. A row that is final stays so when sent comes late,
// as a restore's answer may after its run; it then takes the status
// only.
func (s *Store) SetCommandState(ctx context.Context, id int64, state string, status int, result json.RawMessage) error {
	if err := s.writable(); err != nil {
		return err
	}
	if !validState(state) {
		return fmt.Errorf("store: command state %q", state)
	}
	text, err := jsonText(result)
	if err != nil {
		return fmt.Errorf("store: command result: %w", err)
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("store: command %d: %w", id, err)
	}
	defer tx.Rollback()
	now := s.nowText()
	var cur string
	var curStatus int
	switch err := tx.QueryRowContext(ctx, `SELECT state, status FROM commands WHERE id = ?`, id).Scan(&cur, &curStatus); {
	case errors.Is(err, sql.ErrNoRows):
		return fmt.Errorf("%w: %d", ErrNoCommand, id)
	case err != nil:
		return fmt.Errorf("store: command %d: %w", id, err)
	}
	if status < 0 {
		status = curStatus
	}
	if state == StateSent && Final(cur) {
		if status != curStatus {
			if _, err := tx.ExecContext(ctx, `UPDATE commands SET status = ?, updated_at = ?, version = version + 1 WHERE id = ?`, status, now, id); err != nil {
				return fmt.Errorf("store: command %d: %w", id, err)
			}
		}
		return tx.Commit()
	}
	var finished any
	if Final(state) {
		finished = now
	}
	if _, err := tx.ExecContext(ctx, `UPDATE commands SET state = ?, status = ?, result = ?, updated_at = ?, finished_at = coalesce(?, finished_at), origin = ?, version = version + 1 WHERE id = ?`,
		state, status, text, now, finished, s.nodeID, id); err != nil {
		return fmt.Errorf("store: command %d: %w", id, err)
	}
	if state == StateSent {
		if _, err := tx.ExecContext(ctx, `UPDATE commands SET state = ?, updated_at = ?, version = version + 1 WHERE id = (SELECT parent_id FROM commands WHERE id = ?) AND state = ?`,
			StateSent, now, id, StateQueued); err != nil {
			return fmt.Errorf("store: command %d: %w", id, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("store: command %d: %w", id, err)
	}
	return nil
}

// Command returns a command by identifier, a parent with its steps.
func (s *Store) Command(ctx context.Context, id int64) (Command, bool, error) {
	return s.commandWhere(ctx, `c.id = ?`, id)
}

// CommandByUUID returns a command by its uuid, a parent with its steps.
func (s *Store) CommandByUUID(ctx context.Context, uuid string) (Command, bool, error) {
	return s.commandWhere(ctx, `c.uuid = ?`, uuid)
}

func (s *Store) commandWhere(ctx context.Context, where string, arg any) (Command, bool, error) {
	c, err := scanCommand(s.db.QueryRowContext(ctx, `SELECT `+commandColumns+` FROM commands c WHERE `+where, arg))
	if errors.Is(err, sql.ErrNoRows) {
		return Command{}, false, nil
	}
	if err != nil {
		return Command{}, false, fmt.Errorf("store: command: %w", err)
	}
	if c.ParentID == 0 {
		if c.Steps, err = s.steps(ctx, c.ID); err != nil {
			return Command{}, false, err
		}
	}
	return c, true, nil
}

// steps lists a parent's steps in order.
func (s *Store) steps(ctx context.Context, parentID int64) ([]Command, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+commandColumns+` FROM commands c WHERE c.parent_id = ? ORDER BY c.id`, parentID)
	if err != nil {
		return nil, fmt.Errorf("store: steps: %w", err)
	}
	defer rows.Close()
	out := []Command{}
	for rows.Next() {
		c, err := scanCommand(rows)
		if err != nil {
			return nil, fmt.Errorf("store: steps: %w", err)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// Pending lists the parents that are queued or sent, oldest first.
func (s *Store) Pending(ctx context.Context) ([]Command, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+commandColumns+` FROM commands c WHERE c.state IN (?, ?) AND c.parent_id IS NULL ORDER BY c.id`, StateQueued, StateSent)
	if err != nil {
		return nil, fmt.Errorf("store: pending: %w", err)
	}
	defer rows.Close()
	out := []Command{}
	for rows.Next() {
		c, err := scanCommand(rows)
		if err != nil {
			return nil, fmt.Errorf("store: pending: %w", err)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// DropUnfinished moves every queued or sent row to dropped with the
// reason as its error, at a start: the queue they were in is gone.
func (s *Store) DropUnfinished(ctx context.Context, reason string) (int, error) {
	if err := s.writable(); err != nil {
		return 0, err
	}
	result, _ := json.Marshal(map[string]string{"error": reason})
	now := s.nowText()
	res, err := s.db.ExecContext(ctx, `UPDATE commands SET state = ?, result = ?, updated_at = ?, finished_at = ?, origin = ?, version = version + 1 WHERE state IN (?, ?)`,
		StateDropped, string(result), now, now, s.nodeID, StateQueued, StateSent)
	if err != nil {
		return 0, fmt.Errorf("store: commands: %w", err)
	}
	n, _ := res.RowsAffected()
	return int(n), nil
}

// CommandsKeepDays returns the retention of finished commands in days;
// 0 keeps them.
func (s *Store) CommandsKeepDays(ctx context.Context) (int, error) {
	return s.settingInt(ctx, s.db, CommandsKeepDaysKey)
}

// settingInt reads a setting that holds a JSON number, 0 when unset.
func (s *Store) settingInt(ctx context.Context, q queryer, key string) (int, error) {
	var v string
	err := q.QueryRowContext(ctx, `SELECT value FROM settings WHERE key = ? AND deleted_at IS NULL`, key).Scan(&v)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("store: setting %s: %w", key, err)
	}
	var n int
	if err := json.Unmarshal([]byte(v), &n); err != nil {
		return 0, fmt.Errorf("store: setting %s: %w", key, err)
	}
	return n, nil
}

// pruneCommands clears the keys older than keyKeep and, with a
// retention set, removes the finished parents older than it, their
// steps with them (ON DELETE CASCADE) and their audit rows unlinked
// (ON DELETE SET NULL).
func (s *Store) pruneCommands(ctx context.Context, tx *sql.Tx, now time.Time) error {
	if _, err := tx.ExecContext(ctx, `UPDATE commands SET idempotency_key = '' WHERE idempotency_key <> '' AND created_at < ?`, formatTime(now.Add(-keyKeep))); err != nil {
		return fmt.Errorf("store: commands: %w", err)
	}
	days, err := s.settingInt(ctx, tx, CommandsKeepDaysKey)
	if err != nil {
		return err
	}
	if days <= 0 {
		return nil
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM commands WHERE parent_id IS NULL AND finished_at IS NOT NULL AND finished_at < ?`, formatTime(now.Add(-time.Duration(days)*24*time.Hour))); err != nil {
		return fmt.Errorf("store: commands: %w", err)
	}
	return nil
}
