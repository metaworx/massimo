// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package store

// Roles and capabilities (docs/store.md section 6): a role is a set of
// capability keys, seeded by the first migration for the six default
// roles; owner has every capability by code and cannot be changed,
// disabled or removed. integration is fixed as well, at the ceiling of
// every call through an integration (migration 0021): it is the role of
// integrations and their users, and never changes. The capability keys themselves are the server's
// (internal/server/auth.go); the store keeps what it is given.

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"
)

// The keys of the default roles.
const (
	RoleOwner       = "owner"
	RoleManager     = "manager"
	RoleOperator    = "operator"
	RoleGuest       = "guest"
	RoleViewer      = "viewer"
	RoleIntegration = "integration"
)

// Fixed reports whether a role can never change: owner, and integration,
// the ceiling of the calls through an integration.
func Fixed(key string) bool { return key == RoleOwner || key == RoleIntegration }

// RoleIntegrationV0 is the role migration 0021 gave the users the role
// integration had before it became fixed: an ordinary role with their
// set, "integration (before 1.0)".
const RoleIntegrationV0 = "integration-v0"

// Errors of the roles.
var (
	ErrNoRole     = errors.New("store: no such role")
	ErrRoleExists = errors.New("store: role exists")
	ErrBadRole    = errors.New("store: bad role key")
	ErrImmutable  = errors.New("store: the role cannot be changed")
	ErrRoleInUse  = errors.New("store: the role still has users")
)

// roleKey is a role's key: lower case, 1 to 32 of letters, digits, '_'
// and '-', starting with a letter.
var roleKey = regexp.MustCompile(`^[a-z][a-z0-9_-]{0,31}$`)

// Role is one row of roles with its set.
type Role struct {
	ID           int64
	Key          string
	Name         string // display name; "" shows the key's translated default
	Builtin      bool
	Disabled     bool
	Capabilities []string // sorted; empty for owner, which has every one
	Users        int      // users with this role, removed ones not counted
	CreatedAt    time.Time
	UpdatedAt    time.Time
	Version      int64
}

// normalCaps sorts a set and drops repeats and blanks.
func normalCaps(list []string) []string {
	out := []string{}
	seen := map[string]bool{}
	for _, c := range list {
		c = strings.TrimSpace(c)
		if c != "" && !seen[c] {
			seen[c] = true
			out = append(out, c)
		}
	}
	sort.Strings(out)
	return out
}

const roleColumns = `r.id, r.key, r.name, r.builtin, r.disabled, r.created_at, r.updated_at, r.version,
	coalesce((SELECT group_concat(c.capability) FROM role_capabilities c WHERE c.role_id = r.id), ''),
	(SELECT count(*) FROM users u WHERE u.role_id = r.id AND u.deleted_at IS NULL)`

func scanRole(row scanner) (Role, error) {
	var r Role
	var created, updated sql.NullString
	var caps string
	if err := row.Scan(&r.ID, &r.Key, &r.Name, &r.Builtin, &r.Disabled, &created, &updated, &r.Version, &caps, &r.Users); err != nil {
		return Role{}, err
	}
	r.CreatedAt, r.UpdatedAt = parseTime(created), parseTime(updated)
	if caps != "" {
		r.Capabilities = normalCaps(strings.Split(caps, ","))
	} else {
		r.Capabilities = []string{}
	}
	return r, nil
}

// Roles returns every role that is not removed, the default ones first
// in their order, then the created ones by key.
func (s *Store) Roles(ctx context.Context) ([]Role, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+roleColumns+` FROM roles r WHERE r.deleted_at IS NULL ORDER BY r.builtin DESC, CASE WHEN r.builtin THEN r.id END, r.key`)
	if err != nil {
		return nil, fmt.Errorf("store: roles: %w", err)
	}
	defer rows.Close()
	var out []Role
	for rows.Next() {
		r, err := scanRole(rows)
		if err != nil {
			return nil, fmt.Errorf("store: roles: %w", err)
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// Role returns the role of a key, and whether there is one.
func (s *Store) Role(ctx context.Context, key string) (Role, bool, error) {
	return s.role(ctx, s.db, key)
}

func (s *Store) role(ctx context.Context, q queryer, key string) (Role, bool, error) {
	r, err := scanRole(q.QueryRowContext(ctx, `SELECT `+roleColumns+` FROM roles r WHERE r.key = ? AND r.deleted_at IS NULL`, key))
	if errors.Is(err, sql.ErrNoRows) {
		return Role{}, false, nil
	}
	if err != nil {
		return Role{}, false, fmt.Errorf("store: role %s: %w", key, err)
	}
	return r, true, nil
}

// writeCaps replaces a role's set.
func writeCaps(ctx context.Context, tx *sql.Tx, roleID int64, caps []string) error {
	if _, err := tx.ExecContext(ctx, `DELETE FROM role_capabilities WHERE role_id = ?`, roleID); err != nil {
		return err
	}
	for _, c := range caps {
		if _, err := tx.ExecContext(ctx, `INSERT INTO role_capabilities (role_id, capability) VALUES (?, ?)`, roleID, c); err != nil {
			return err
		}
	}
	return nil
}

// AddRole creates a role with r's Key, Name and Capabilities, created
// by the user by. A removed role of the key is taken over.
func (s *Store) AddRole(ctx context.Context, r Role, by int64) (Role, error) {
	if err := s.writable(); err != nil {
		return Role{}, err
	}
	r.Key = strings.ToLower(strings.TrimSpace(r.Key))
	if !roleKey.MatchString(r.Key) {
		return Role{}, fmt.Errorf("%w: %q", ErrBadRole, r.Key)
	}
	caps := normalCaps(r.Capabilities)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Role{}, fmt.Errorf("store: add role %s: %w", r.Key, err)
	}
	defer tx.Rollback()
	var id int64
	var deleted sql.NullString
	now := s.nowText()
	switch err := tx.QueryRowContext(ctx, `SELECT id, deleted_at FROM roles WHERE key = ?`, r.Key).Scan(&id, &deleted); {
	case errors.Is(err, sql.ErrNoRows):
		res, err := tx.ExecContext(ctx, `INSERT INTO roles (key, name, builtin, created_at, updated_at, updated_by, created_by, origin) VALUES (?, ?, 0, ?, ?, ?, ?, ?)`,
			r.Key, r.Name, now, now, nullID(by), nullID(by), s.nodeID)
		if err != nil {
			return Role{}, fmt.Errorf("store: add role %s: %w", r.Key, err)
		}
		id, _ = res.LastInsertId()
	case err != nil:
		return Role{}, fmt.Errorf("store: add role %s: %w", r.Key, err)
	case !deleted.Valid:
		return Role{}, fmt.Errorf("%w: %s", ErrRoleExists, r.Key)
	default:
		if _, err := tx.ExecContext(ctx, `UPDATE roles SET name = ?, disabled = 0, deleted_at = NULL, updated_at = ?, updated_by = ?, origin = ?, version = version + 1 WHERE id = ?`,
			r.Name, now, nullID(by), s.nodeID, id); err != nil {
			return Role{}, fmt.Errorf("store: add role %s: %w", r.Key, err)
		}
	}
	if err := writeCaps(ctx, tx, id, caps); err != nil {
		return Role{}, fmt.Errorf("store: add role %s: %w", r.Key, err)
	}
	if err := tx.Commit(); err != nil {
		return Role{}, fmt.Errorf("store: add role %s: %w", r.Key, err)
	}
	out, _, err := s.Role(ctx, r.Key)
	return out, err
}

// UpdateRole changes a role's Name and Capabilities with change, in one
// transaction; owner and integration are ErrImmutable.
func (s *Store) UpdateRole(ctx context.Context, key string, change func(*Role) error, by int64) (Role, error) {
	if err := s.writable(); err != nil {
		return Role{}, err
	}
	if Fixed(key) {
		return Role{}, fmt.Errorf("%w: %s", ErrImmutable, key)
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Role{}, fmt.Errorf("store: role %s: %w", key, err)
	}
	defer tx.Rollback()
	r, ok, err := s.role(ctx, tx, key)
	if err != nil {
		return Role{}, err
	}
	if !ok {
		return Role{}, fmt.Errorf("%w: %s", ErrNoRole, key)
	}
	if err := change(&r); err != nil {
		return Role{}, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE roles SET name = ?, updated_at = ?, updated_by = ?, origin = ?, version = version + 1 WHERE id = ?`,
		r.Name, s.nowText(), nullID(by), s.nodeID, r.ID); err != nil {
		return Role{}, fmt.Errorf("store: role %s: %w", key, err)
	}
	if err := writeCaps(ctx, tx, r.ID, normalCaps(r.Capabilities)); err != nil {
		return Role{}, fmt.Errorf("store: role %s: %w", key, err)
	}
	if err := tx.Commit(); err != nil {
		return Role{}, fmt.Errorf("store: role %s: %w", key, err)
	}
	out, _, err := s.Role(ctx, key)
	return out, err
}

// SetRoleDisabled disables or enables a role; owner and integration are
// ErrImmutable. A
// disabled role's users keep their login and are refused everything.
func (s *Store) SetRoleDisabled(ctx context.Context, key string, disabled bool, by int64) error {
	if err := s.writable(); err != nil {
		return err
	}
	if Fixed(key) {
		return fmt.Errorf("%w: %s", ErrImmutable, key)
	}
	res, err := s.db.ExecContext(ctx, `UPDATE roles SET disabled = ?, updated_at = ?, updated_by = ?, origin = ?, version = version + 1 WHERE key = ? AND deleted_at IS NULL`,
		disabled, s.nowText(), nullID(by), s.nodeID, key)
	if err != nil {
		return fmt.Errorf("store: role %s: %w", key, err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("%w: %s", ErrNoRole, key)
	}
	return nil
}

// DeleteRole removes a created role that no user has: a tombstone that
// keeps its set. A default role is ErrImmutable, one in use ErrRoleInUse.
func (s *Store) DeleteRole(ctx context.Context, key string, by int64) error {
	if err := s.writable(); err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("store: role %s: %w", key, err)
	}
	defer tx.Rollback()
	r, ok, err := s.role(ctx, tx, key)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("%w: %s", ErrNoRole, key)
	}
	if r.Builtin {
		return fmt.Errorf("%w: %s is a default role", ErrImmutable, key)
	}
	if r.Users > 0 {
		return fmt.Errorf("%w: %s", ErrRoleInUse, key)
	}
	now := s.nowText()
	if _, err := tx.ExecContext(ctx, `UPDATE roles SET deleted_at = ?, updated_at = ?, updated_by = ?, origin = ?, version = version + 1 WHERE id = ?`, now, now, nullID(by), s.nodeID, r.ID); err != nil {
		return fmt.Errorf("store: role %s: %w", key, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("store: role %s: %w", key, err)
	}
	return nil
}

// DisabledCapabilities lists the capabilities disabled for every role
// but owner, sorted.
func (s *Store) DisabledCapabilities(ctx context.Context) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT capability FROM capabilities_disabled ORDER BY capability`)
	if err != nil {
		return nil, fmt.Errorf("store: capabilities: %w", err)
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var c string
		if err := rows.Scan(&c); err != nil {
			return nil, fmt.Errorf("store: capabilities: %w", err)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// SetCapabilityDisabled disables a capability for every role but owner,
// or enables it again.
func (s *Store) SetCapabilityDisabled(ctx context.Context, capability string, disabled bool, by int64) error {
	if err := s.writable(); err != nil {
		return err
	}
	var err error
	if disabled {
		_, err = s.db.ExecContext(ctx, `INSERT INTO capabilities_disabled (capability, disabled_at, disabled_by) VALUES (?, ?, ?) ON CONFLICT (capability) DO NOTHING`, capability, s.nowText(), nullID(by))
	} else {
		_, err = s.db.ExecContext(ctx, `DELETE FROM capabilities_disabled WHERE capability = ?`, capability)
	}
	if err != nil {
		return fmt.Errorf("store: capability %s: %w", capability, err)
	}
	return nil
}

// Effective returns the capabilities a role grants: every one of all for
// owner, none for a disabled role, else the role's set minus the
// disabled capabilities. The second result says whether the role is
// disabled.
func (s *Store) Effective(ctx context.Context, roleKey string, all []string) (caps []string, roleDisabled bool, err error) {
	r, ok, err := s.Role(ctx, roleKey)
	if err != nil {
		return nil, false, err
	}
	if !ok {
		return nil, false, fmt.Errorf("%w: %s", ErrNoRole, roleKey)
	}
	if r.Key == RoleOwner {
		return normalCaps(all), false, nil
	}
	if r.Disabled {
		return []string{}, true, nil
	}
	off, err := s.DisabledCapabilities(ctx)
	if err != nil {
		return nil, false, err
	}
	disabled := map[string]bool{}
	for _, c := range off {
		disabled[c] = true
	}
	out := []string{}
	for _, c := range r.Capabilities {
		if !disabled[c] {
			out = append(out, c)
		}
	}
	return out, false, nil
}
