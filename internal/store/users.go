// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package store

// Users (docs/store.md section 6): a login name, a display name, a
// password hash or none, one role, and for guests an end date. A
// removed user is a tombstone (deleted_at) that keeps its identifier for
// the audit; adding the name again takes the row over, as AddCube does
// with a Cube.

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
)

// Errors of the users.
var (
	ErrUserExists = errors.New("store: user exists")
	ErrNoUser     = errors.New("store: no such user")
	ErrBadName    = errors.New("store: bad user name")
	ErrLastOwner  = errors.New("store: the last enabled owner cannot be disabled, removed or demoted")
)

// userName is a login name: lower case, 1 to 64 of letters, digits,
// '.', '_', '-' and '@', starting with a letter or digit.
var userName = regexp.MustCompile(`^[a-z0-9][a-z0-9._@-]{0,63}$`)

// NormalName is a login name as the store keeps it: trimmed, lower case.
func NormalName(name string) string { return strings.ToLower(strings.TrimSpace(name)) }

// User is one row of users.
type User struct {
	ID          int64
	UUID        string
	Name        string // the login name, lower case
	DisplayName string
	HasPassword bool
	RoleID      int64
	Role        string // the role's key
	Disabled    bool
	ExpiresAt   time.Time // guests: disabled from then on; zero for none
	InvitedBy   int64     // the user that created this one; 0 for none
	CreatedAt   time.Time
	UpdatedAt   time.Time
	LastLoginAt time.Time
	AuthVersion int // 1: the server hashes the password
	Version     int64
	Origin      string
}

// Expired reports whether the user's end date has passed at now.
func (u User) Expired(now time.Time) bool { return !u.ExpiresAt.IsZero() && !now.Before(u.ExpiresAt) }

const userColumns = `u.id, u.uuid, u.name, u.display_name, u.password_hash <> '', u.role_id, r.key, u.disabled, u.expires_at, u.invited_by,
	u.created_at, u.updated_at, u.last_login_at, u.auth_version, u.version, u.origin`

const userFrom = ` FROM users u JOIN roles r ON r.id = u.role_id `

func scanUser(row scanner) (User, error) {
	var u User
	var expires, created, updated, last sql.NullString
	var invited sql.NullInt64
	if err := row.Scan(&u.ID, &u.UUID, &u.Name, &u.DisplayName, &u.HasPassword, &u.RoleID, &u.Role, &u.Disabled, &expires, &invited,
		&created, &updated, &last, &u.AuthVersion, &u.Version, &u.Origin); err != nil {
		return User{}, err
	}
	u.ExpiresAt, u.CreatedAt, u.UpdatedAt, u.LastLoginAt = parseTime(expires), parseTime(created), parseTime(updated), parseTime(last)
	u.InvitedBy = invited.Int64
	return u, nil
}

// nullTime is a time as the store writes it, NULL for the zero time.
func nullTime(t time.Time) any {
	if t.IsZero() {
		return nil
	}
	return formatTime(t)
}

// zeroTime is the zero time, for a token that never expires.
var zeroTime time.Time

// nullID is a row identifier, NULL for 0.
func nullID(id int64) any {
	if id == 0 {
		return nil
	}
	return id
}

// Users returns every user that is not removed, by name.
func (s *Store) Users(ctx context.Context) ([]User, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+userColumns+userFrom+`WHERE u.deleted_at IS NULL ORDER BY u.name`)
	if err != nil {
		return nil, fmt.Errorf("store: users: %w", err)
	}
	defer rows.Close()
	var out []User
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, fmt.Errorf("store: users: %w", err)
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

// User returns the user of a login name, and whether there is one.
func (s *Store) User(ctx context.Context, name string) (User, bool, error) {
	return s.user(ctx, s.db, name)
}

func (s *Store) user(ctx context.Context, q queryer, name string) (User, bool, error) {
	u, err := scanUser(q.QueryRowContext(ctx, `SELECT `+userColumns+userFrom+`WHERE u.name = ? AND u.deleted_at IS NULL`, NormalName(name)))
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, false, nil
	}
	if err != nil {
		return User{}, false, fmt.Errorf("store: user %s: %w", name, err)
	}
	return u, true, nil
}

// UserByID returns the user of a row identifier, removed or not.
func (s *Store) UserByID(ctx context.Context, id int64) (User, bool, error) {
	u, err := scanUser(s.db.QueryRowContext(ctx, `SELECT `+userColumns+userFrom+`WHERE u.id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, false, nil
	}
	if err != nil {
		return User{}, false, fmt.Errorf("store: user %d: %w", id, err)
	}
	return u, true, nil
}

// CountUsers counts the users that are not removed, disabled ones
// included: with one, every request needs a token.
func (s *Store) CountUsers(ctx context.Context) (int, error) {
	var n int
	if err := s.db.QueryRowContext(ctx, `SELECT count(*) FROM users WHERE deleted_at IS NULL`).Scan(&n); err != nil {
		return 0, fmt.Errorf("store: users: %w", err)
	}
	return n, nil
}

// roleID finds a role by key; ErrNoRole when there is none.
func (s *Store) roleID(ctx context.Context, q queryer, key string) (int64, error) {
	var id int64
	err := q.QueryRowContext(ctx, `SELECT id FROM roles WHERE key = ? AND deleted_at IS NULL`, key).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, fmt.Errorf("%w: %s", ErrNoRole, key)
	}
	if err != nil {
		return 0, fmt.Errorf("store: role %s: %w", key, err)
	}
	return id, nil
}

// AddUser adds a user with u's Name, DisplayName, Role (a key),
// Disabled and ExpiresAt, and the password (none when empty), created
// by the user by (0: nobody, the command line). A removed user of the
// name is taken over: its row gets the new values and keeps its
// identifier.
func (s *Store) AddUser(ctx context.Context, u User, password string, by int64) (User, error) {
	if err := s.writable(); err != nil {
		return User{}, err
	}
	u.Name = NormalName(u.Name)
	if !userName.MatchString(u.Name) {
		return User{}, fmt.Errorf("%w: %q", ErrBadName, u.Name)
	}
	if u.Role == "" {
		u.Role = RoleViewer
	}
	hash := ""
	if password != "" {
		var err error
		if hash, err = HashPassword(password); err != nil {
			return User{}, err
		}
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return User{}, fmt.Errorf("store: add user %s: %w", u.Name, err)
	}
	defer tx.Rollback()
	role, err := s.roleID(ctx, tx, u.Role)
	if err != nil {
		return User{}, err
	}
	var id int64
	var deleted sql.NullString
	now := s.nowText()
	switch err := tx.QueryRowContext(ctx, `SELECT id, deleted_at FROM users WHERE name = ?`, u.Name).Scan(&id, &deleted); {
	case errors.Is(err, sql.ErrNoRows):
		_, err = tx.ExecContext(ctx, `INSERT INTO users (uuid, name, display_name, password_hash, role_id, disabled, expires_at, invited_by, created_at, updated_at, updated_by, origin)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			newUUID(), u.Name, u.DisplayName, hash, role, u.Disabled, nullTime(u.ExpiresAt), nullID(by), now, now, nullID(by), s.nodeID)
	case err != nil:
		return User{}, fmt.Errorf("store: add user %s: %w", u.Name, err)
	case !deleted.Valid:
		return User{}, fmt.Errorf("%w: %s", ErrUserExists, u.Name)
	default:
		_, err = tx.ExecContext(ctx, `UPDATE users SET display_name = ?, password_hash = ?, role_id = ?, disabled = ?, expires_at = ?, invited_by = ?, last_login_at = NULL, auth_version = 1,
			deleted_at = NULL, updated_at = ?, updated_by = ?, origin = ?, version = version + 1 WHERE id = ?`,
			u.DisplayName, hash, role, u.Disabled, nullTime(u.ExpiresAt), nullID(by), now, nullID(by), s.nodeID, id)
	}
	if err != nil {
		return User{}, fmt.Errorf("store: add user %s: %w", u.Name, err)
	}
	if err := tx.Commit(); err != nil {
		return User{}, fmt.Errorf("store: add user %s: %w", u.Name, err)
	}
	out, _, err := s.User(ctx, u.Name)
	return out, err
}

// ownersLeft counts the enabled owners that are not removed, other than
// the user id.
func (s *Store) ownersLeft(ctx context.Context, q queryer, id int64) (int, error) {
	var n int
	err := q.QueryRowContext(ctx, `SELECT count(*) FROM users u JOIN roles r ON r.id = u.role_id WHERE r.key = ? AND u.disabled = 0 AND u.deleted_at IS NULL AND u.id <> ?`, RoleOwner, id).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("store: owners: %w", err)
	}
	return n, nil
}

// UpdateUser changes a user with change, in one transaction, and
// writes back DisplayName, Role, Disabled and ExpiresAt. A change that
// would leave no enabled owner is ErrLastOwner.
func (s *Store) UpdateUser(ctx context.Context, name string, change func(*User) error, by int64) (User, error) {
	if err := s.writable(); err != nil {
		return User{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return User{}, fmt.Errorf("store: user %s: %w", name, err)
	}
	defer tx.Rollback()
	u, ok, err := s.user(ctx, tx, name)
	if err != nil {
		return User{}, err
	}
	if !ok {
		return User{}, fmt.Errorf("%w: %s", ErrNoUser, NormalName(name))
	}
	was := u
	if err := change(&u); err != nil {
		return User{}, err
	}
	role, err := s.roleID(ctx, tx, u.Role)
	if err != nil {
		return User{}, err
	}
	if was.Role == RoleOwner && !was.Disabled && (u.Role != RoleOwner || u.Disabled) {
		if n, err := s.ownersLeft(ctx, tx, u.ID); err != nil {
			return User{}, err
		} else if n == 0 {
			return User{}, ErrLastOwner
		}
	}
	if _, err := tx.ExecContext(ctx, `UPDATE users SET display_name = ?, role_id = ?, disabled = ?, expires_at = ?, updated_at = ?, updated_by = ?, origin = ?, version = version + 1 WHERE id = ?`,
		u.DisplayName, role, u.Disabled, nullTime(u.ExpiresAt), s.nowText(), nullID(by), s.nodeID, u.ID); err != nil {
		return User{}, fmt.Errorf("store: user %s: %w", u.Name, err)
	}
	if err := tx.Commit(); err != nil {
		return User{}, fmt.Errorf("store: user %s: %w", u.Name, err)
	}
	out, _, err := s.User(ctx, u.Name)
	return out, err
}

// SetPassword sets a user's password, or removes it when empty. Its
// sessions are the caller's to revoke (RevokeSessions).
func (s *Store) SetPassword(ctx context.Context, name, password string, by int64) error {
	if err := s.writable(); err != nil {
		return err
	}
	hash := ""
	if password != "" {
		var err error
		if hash, err = HashPassword(password); err != nil {
			return err
		}
	}
	res, err := s.db.ExecContext(ctx, `UPDATE users SET password_hash = ?, auth_version = 1, updated_at = ?, updated_by = ?, origin = ?, version = version + 1 WHERE name = ? AND deleted_at IS NULL`,
		hash, s.nowText(), nullID(by), s.nodeID, NormalName(name))
	if err != nil {
		return fmt.Errorf("store: user %s: %w", name, err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("%w: %s", ErrNoUser, NormalName(name))
	}
	return nil
}

// CheckPassword verifies a user's password; ErrBadPassword when it is
// wrong or the user has none. The check costs the same time either way.
func (s *Store) CheckPassword(ctx context.Context, name, password string) error {
	var hash string
	err := s.db.QueryRowContext(ctx, `SELECT password_hash FROM users WHERE name = ? AND deleted_at IS NULL`, NormalName(name)).Scan(&hash)
	if errors.Is(err, sql.ErrNoRows) || err == nil && hash == "" {
		DummyVerify(password)
		return ErrBadPassword
	}
	if err != nil {
		return fmt.Errorf("store: user %s: %w", name, err)
	}
	if !VerifyPassword(hash, password) {
		return ErrBadPassword
	}
	return nil
}

// NoteLogin records a login at now.
func (s *Store) NoteLogin(ctx context.Context, id int64) error {
	if err := s.writable(); err != nil {
		return err
	}
	if _, err := s.db.ExecContext(ctx, `UPDATE users SET last_login_at = ? WHERE id = ?`, s.nowText(), id); err != nil {
		return fmt.Errorf("store: user %d: %w", id, err)
	}
	return nil
}

// DeleteUser removes a user: a tombstone, with every token of the user
// revoked. The last enabled owner is ErrLastOwner.
func (s *Store) DeleteUser(ctx context.Context, name string, by int64) error {
	if err := s.writable(); err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("store: user %s: %w", name, err)
	}
	defer tx.Rollback()
	u, ok, err := s.user(ctx, tx, name)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("%w: %s", ErrNoUser, NormalName(name))
	}
	if u.Role == RoleOwner && !u.Disabled {
		if n, err := s.ownersLeft(ctx, tx, u.ID); err != nil {
			return err
		} else if n == 0 {
			return ErrLastOwner
		}
	}
	now := s.nowText()
	if _, err := tx.ExecContext(ctx, `UPDATE users SET deleted_at = ?, updated_at = ?, updated_by = ?, origin = ?, version = version + 1 WHERE id = ?`, now, now, nullID(by), s.nodeID, u.ID); err != nil {
		return fmt.Errorf("store: user %s: %w", u.Name, err)
	}
	if _, err := tx.ExecContext(ctx, `UPDATE tokens SET revoked_at = ?, version = version + 1 WHERE user_id = ? AND revoked_at IS NULL`, now, u.ID); err != nil {
		return fmt.Errorf("store: user %s: %w", u.Name, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("store: user %s: %w", u.Name, err)
	}
	return nil
}
