// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package store

// Users (docs/store.md section 6): a login name, a display name, a
// password hash or none, one role, and for guests an end date. A
// removed user is a tombstone (deleted_at) that keeps its identifier and
// its name for the audit; a name is unique among the users that are not
// removed (migration 0024), so a user added under a removed one's name is
// a row of its own.

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
	ErrLastOwner  = errors.New("store: the last enabled owner without an end date cannot be disabled, removed, demoted or given one")
	// ErrIntegrationUser is a key, a password or a provider's identity
	// asked for an integration's user, which acts through its
	// integration alone: the integration's own key is the one way in.
	ErrIntegrationUser = errors.New("store: an integration's user acts through its integration alone")
)

// regularUser is the condition of a regular user in the table users: one
// that is not removed and is not an integration's, a disabled one
// included. A server is open while it has none (AP LooseEnds, D1).
const regularUser = `deleted_at IS NULL AND integration_id IS NULL`

// integrationUser is regularUser's opposite among the users not
// removed: an integration's user.
const integrationUser = `deleted_at IS NULL AND integration_id IS NOT NULL`

// noWayIn reports ErrIntegrationUser where the user that where and arg
// name (for example "id = ?" and an id) is an integration's.
func noWayIn(ctx context.Context, q queryer, where string, arg any) error {
	var n int
	if err := q.QueryRowContext(ctx, `SELECT count(*) FROM users WHERE `+where+` AND `+integrationUser, arg).Scan(&n); err != nil {
		return fmt.Errorf("store: user: %w", err)
	}
	if n > 0 {
		return ErrIntegrationUser
	}
	return nil
}

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
	// IntegrationID is the integration whose user this is, 0 for none
	// (integrations.go).
	IntegrationID int64
	CreatedAt     time.Time
	UpdatedAt     time.Time
	LastLoginAt   time.Time
	AuthVersion   int // 1: the server hashes the password; 2: OPAQUE (opaque.go)
	Version       int64
	Origin        string
}

// Expired reports whether the user's end date has passed at now.
func (u User) Expired(now time.Time) bool { return !u.ExpiresAt.IsZero() && !now.Before(u.ExpiresAt) }

const userColumns = `u.id, u.uuid, u.name, u.display_name, (u.password_hash <> '' OR u.opaque_record IS NOT NULL), u.role_id, r.key, u.disabled, u.expires_at, u.invited_by,
	coalesce(u.integration_id, 0), u.created_at, u.updated_at, u.last_login_at, u.auth_version, u.version, u.origin`

const userFrom = ` FROM users u JOIN roles r ON r.id = u.role_id `

func scanUser(row scanner) (User, error) {
	var u User
	var expires, created, updated, last sql.NullString
	var invited sql.NullInt64
	if err := row.Scan(&u.ID, &u.UUID, &u.Name, &u.DisplayName, &u.HasPassword, &u.RoleID, &u.Role, &u.Disabled, &expires, &invited,
		&u.IntegrationID, &created, &updated, &last, &u.AuthVersion, &u.Version, &u.Origin); err != nil {
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

// UserIDByName returns the row identifier of a login name: the user's
// that is not removed, else the one removed last of that name.
func (s *Store) UserIDByName(ctx context.Context, name string) (int64, bool, error) {
	var id int64
	err := s.db.QueryRowContext(ctx, `SELECT id FROM users WHERE name = ? ORDER BY deleted_at IS NOT NULL, deleted_at DESC, id DESC LIMIT 1`, NormalName(name)).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, fmt.Errorf("store: user %s: %w", name, err)
	}
	return id, true, nil
}

// countUsers counts the regular users (regularUser) through q, the
// store or a transaction.
func countUsers(ctx context.Context, q queryer) (int, error) {
	var n int
	if err := q.QueryRowContext(ctx, `SELECT count(*) FROM users WHERE `+regularUser).Scan(&n); err != nil {
		return 0, fmt.Errorf("store: users: %w", err)
	}
	return n, nil
}

// CountUsers counts the regular users, disabled ones included and an
// integration's not: with one, every request needs a token.
func (s *Store) CountUsers(ctx context.Context) (int, error) {
	return countUsers(ctx, s.db)
}

// CountIntegrationUsers counts the integrations' users that are not
// removed (integrationUser), which CountUsers leaves out.
func (s *Store) CountIntegrationUsers(ctx context.Context) (int, error) {
	var n int
	if err := s.db.QueryRowContext(ctx, `SELECT count(*) FROM users WHERE `+integrationUser).Scan(&n); err != nil {
		return 0, fmt.Errorf("store: integration users: %w", err)
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

// credential is what a new user signs in with: a hash of version 1, a
// record of version 2 (opaque.go), or neither, for keys only.
type credential struct {
	hash   string
	record []byte
	ksf    string
}

// authVersion is the auth_version of a credential.
func (c credential) authVersion() int {
	if c.record != nil {
		return 2
	}
	return 1
}

// newUser normalises a user to add: the name checked, the role viewer
// by default.
func (s *Store) newUser(u User) (User, error) {
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
	return u, nil
}

// AddUser adds a user with u's Name, DisplayName, Role (a key),
// Disabled and ExpiresAt, and the password (none when empty), created
// by the user by (0: nobody, the command line). A removed user's name is
// free, and a user added under it is a row of its own.
func (s *Store) AddUser(ctx context.Context, u User, password string, by int64) (User, error) {
	u, err := s.newUser(u)
	if err != nil {
		return User{}, err
	}
	hash := ""
	if password != "" {
		if err := s.addable(ctx, u); err != nil {
			return User{}, err
		}
		var err error
		if hash, err = s.hashPassword(ctx, password); err != nil {
			return User{}, err
		}
	}
	return s.addUser(ctx, u, credential{hash: hash}, by, false)
}

// AddFirstOwner adds the first owner, as AddUser does with the role
// owner and nobody as its creator, only while the server has no regular
// user (CountUsers): ErrHasUsers where it has one, also one that another
// request added since its caller looked, since the count is read again
// in the transaction that writes the user (AP FirstOwner, D15).
func (s *Store) AddFirstOwner(ctx context.Context, u User, password string) (User, error) {
	u.Role = RoleOwner
	u, err := s.newUser(u)
	if err != nil {
		return User{}, err
	}
	if err := s.noUsers(ctx, s.db); err != nil {
		return User{}, err // before the cost of a hash
	}
	hash := ""
	if password != "" {
		if err := s.addable(ctx, u); err != nil {
			return User{}, err
		}
		if hash, err = s.hashPassword(ctx, password); err != nil {
			return User{}, err
		}
	}
	return s.addUser(ctx, u, credential{hash: hash}, 0, true)
}

// AddFirstOpaqueOwner adds the first owner as AddFirstOwner does, with
// a password of version 2, as AddOpaqueUser takes it.
func (s *Store) AddFirstOpaqueOwner(ctx context.Context, u User, record []byte, ksf string) (User, error) {
	u.Role = RoleOwner
	u, err := s.newUser(u)
	if err != nil {
		return User{}, err
	}
	if err := s.checkRecord(record, ksf); err != nil {
		return User{}, err
	}
	return s.addUser(ctx, u, credential{record: record, ksf: ksf}, 0, true)
}

// noUsers reports ErrHasUsers where a regular user exists
// (regularUser); an integration's users do not count.
func (s *Store) noUsers(ctx context.Context, q queryer) error {
	n, err := countUsers(ctx, q)
	if err != nil {
		return err
	}
	if n > 0 {
		return fmt.Errorf("%w: the first owner exists already", ErrHasUsers)
	}
	return nil
}

// addable answers what a user add would refuse, before the cost of a
// hash: an unknown role, a taken name. The transaction checks again for
// a race.
func (s *Store) addable(ctx context.Context, u User) error {
	if _, err := s.roleID(ctx, s.db, u.Role); err != nil {
		return err
	}
	return s.nameFree(ctx, s.db, u.Name)
}

// nameFree reports ErrUserExists where a user that is not removed has
// the name.
func (s *Store) nameFree(ctx context.Context, q queryer, name string) error {
	var n int
	if err := q.QueryRowContext(ctx, `SELECT count(*) FROM users WHERE name = ? AND deleted_at IS NULL`, name).Scan(&n); err != nil {
		return fmt.Errorf("store: user %s: %w", name, err)
	}
	if n > 0 {
		return fmt.Errorf("%w: %s", ErrUserExists, name)
	}
	return nil
}

// AddUserOpaque adds a user as AddUser does, with a password of version 2
// whose registration runs here, both halves (opaqueRecordFor): for the
// command line, which has the password and the store at hand.
func (s *Store) AddUserOpaque(ctx context.Context, u User, password string, by int64) (User, error) {
	u, c, err := s.opaqueCredential(ctx, u, password)
	if err != nil {
		return User{}, err
	}
	return s.addUser(ctx, u, c, by, false)
}

// opaqueCredential checks u as a user add does and registers its
// password of version 2 here, both halves (opaqueRecordFor): the user
// and the credential for addUser.
func (s *Store) opaqueCredential(ctx context.Context, u User, password string) (User, credential, error) {
	u, err := s.newUser(u)
	if err != nil {
		return User{}, credential{}, err
	}
	if err := s.addable(ctx, u); err != nil {
		return User{}, credential{}, err
	}
	record, err := s.opaqueRecordFor(ctx, u.Name, password)
	if err != nil {
		return User{}, credential{}, err
	}
	return u, credential{record: record, ksf: OpaqueKSF}, nil
}

// commandStep runs in AddCommandUser once the user is checked and its
// password registered, before the transaction that writes it; nil but
// in the tests, which add a user from another process there.
var commandStep func()

// AddCommandUser adds a user as the command line does: with a password
// of version 2 registered here where one is given (AddUserOpaque), else
// with keys only (AddUser). It reports whether the user is the store's
// first regular user (CountUsers), counted in the transaction that
// writes it, so that of a first owner another process makes at the same
// moment, while the registration runs, and this user one alone is first
// (AP AccessEnds, D12).
func (s *Store) AddCommandUser(ctx context.Context, u User, password string, by int64) (User, bool, error) {
	var c credential
	var err error
	if password != "" {
		u, c, err = s.opaqueCredential(ctx, u, password)
	} else {
		u, err = s.newUser(u)
	}
	if err != nil {
		return User{}, false, err
	}
	if commandStep != nil {
		commandStep()
	}
	first := false
	counted := func(tx *sql.Tx, _ int64) error {
		n, err := countUsers(ctx, tx)
		first = n == 1
		return err
	}
	created, err := s.addUser(ctx, u, c, by, false, counted)
	if err != nil {
		return User{}, false, err
	}
	return created, first, nil
}

// SetPasswordOpaque sets a user's password of version 2, registered here
// as AddUserOpaque does. Its sessions are the caller's to revoke.
func (s *Store) SetPasswordOpaque(ctx context.Context, name, password string, by int64) error {
	if err := s.writable(); err != nil {
		return err
	}
	var live int
	if err := s.db.QueryRowContext(ctx, `SELECT count(*) FROM users WHERE name = ? AND deleted_at IS NULL`, NormalName(name)).Scan(&live); err != nil {
		return fmt.Errorf("store: user %s: %w", name, err)
	}
	if live == 0 {
		return fmt.Errorf("%w: %s", ErrNoUser, NormalName(name))
	}
	record, err := s.opaqueRecordFor(ctx, name, password)
	if err != nil {
		return err
	}
	return s.SetOpaqueRecord(ctx, name, record, OpaqueKSF, by)
}

// AddOpaqueUser adds a user as AddUser does, with a password of
// version 2: the client's registration record and its key stretching
// (OpaqueRegistration, SetOpaqueRecord).
func (s *Store) AddOpaqueUser(ctx context.Context, u User, record []byte, ksf string, by int64) (User, error) {
	u, err := s.newUser(u)
	if err != nil {
		return User{}, err
	}
	if err := s.checkRecord(record, ksf); err != nil {
		return User{}, err
	}
	return s.addUser(ctx, u, credential{record: record, ksf: ksf}, by, false)
}

// addUser writes a user that newUser checked, with its credential, and
// removes the setting KeepOpenKey in the same transaction; first, only
// while the server has no regular user (AddFirstOwner). The transaction begins
// IMMEDIATE (the store's _txlock), so it holds the database's one write
// lock from its start: a second one waits for the first to commit, and
// its count then sees the first one's user. then, where given, runs in
// the same transaction with the new user's id, before it commits: what
// must not be left undone beside the user (the owner's key at a start,
// Bootstrap).
func (s *Store) addUser(ctx context.Context, u User, c credential, by int64, first bool, then ...func(tx *sql.Tx, id int64) error) (User, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return User{}, fmt.Errorf("store: add user %s: %w", u.Name, err)
	}
	defer tx.Rollback()
	if first {
		if err := s.noUsers(ctx, tx); err != nil {
			return User{}, err
		}
	}
	role, err := s.roleID(ctx, tx, u.Role)
	if err != nil {
		return User{}, err
	}
	if err := s.nameFree(ctx, tx, u.Name); err != nil {
		return User{}, err
	}
	now := s.nowText()
	res, err := tx.ExecContext(ctx, `INSERT INTO users (uuid, name, display_name, password_hash, opaque_record, opaque_ksf, auth_version, role_id, disabled, expires_at, invited_by, created_at, updated_at, updated_by, origin)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		newUUID(), u.Name, u.DisplayName, c.hash, c.record, c.ksf, c.authVersion(), role, u.Disabled, nullTime(u.ExpiresAt), nullID(by), now, now, nullID(by), s.nodeID)
	if err != nil {
		return User{}, fmt.Errorf("store: add user %s: %w", u.Name, err)
	}
	id, _ := res.LastInsertId()
	for _, f := range then {
		if err := f(tx, id); err != nil {
			return User{}, err
		}
	}
	dropped, err := s.dropKeepOpen(ctx, tx)
	if err != nil {
		return User{}, err
	}
	if err := tx.Commit(); err != nil {
		return User{}, fmt.Errorf("store: add user %s: %w", u.Name, err)
	}
	s.keepOpenDropped(dropped, "user added", "user", u.Name)
	out, _, err := s.User(ctx, u.Name)
	return out, err
}

// ownerLeft reports whether a user counts as an owner that is left: an
// enabled owner without an end date, who can sign in for good. An
// integration's user is none, whatever its role: it signs in nowhere.
func ownerLeft(u User) bool {
	return u.Role == RoleOwner && !u.Disabled && u.ExpiresAt.IsZero() && u.IntegrationID == 0
}

// ownersLeft counts the owners left (ownerLeft) among the regular users
// (regularUser), other than the user id.
func (s *Store) ownersLeft(ctx context.Context, q queryer, id int64) (int, error) {
	var n int
	err := q.QueryRowContext(ctx, `SELECT count(*) FROM users u JOIN roles r ON r.id = u.role_id
		WHERE r.key = ? AND u.disabled = 0 AND u.expires_at IS NULL AND u.id <> ?
		AND u.id IN (SELECT id FROM users WHERE `+regularUser+`)`, RoleOwner, id).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("store: owners: %w", err)
	}
	return n, nil
}

// UpdateUser changes a user with change, in one transaction, and
// writes back DisplayName, Role, Disabled and ExpiresAt. A change that
// would leave no owner without an end date is ErrLastOwner.
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
	if ownerLeft(was) && !ownerLeft(u) {
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

// SetPassword sets a user's password of version 1, or removes it when
// empty, a record of version 2 with it. Its sessions are the caller's to
// revoke (RevokeSessions). An integration's user gets none
// (ErrIntegrationUser); one it holds may be removed.
func (s *Store) SetPassword(ctx context.Context, name, password string, by int64) error {
	if err := s.writable(); err != nil {
		return err
	}
	hash := ""
	if password != "" {
		if err := noWayIn(ctx, s.db, "name = ?", NormalName(name)); err != nil {
			return err
		}
		// No hash for a user that is not there.
		var live int
		if err := s.db.QueryRowContext(ctx, `SELECT count(*) FROM users WHERE name = ? AND deleted_at IS NULL`, NormalName(name)).Scan(&live); err != nil {
			return fmt.Errorf("store: user %s: %w", name, err)
		}
		if live == 0 {
			return fmt.Errorf("%w: %s", ErrNoUser, NormalName(name))
		}
		var err error
		if hash, err = s.hashPassword(ctx, password); err != nil {
			return err
		}
	}
	res, err := s.db.ExecContext(ctx, `UPDATE users SET password_hash = ?, opaque_record = NULL, opaque_ksf = '', auth_version = 1, updated_at = ?, updated_by = ?, origin = ?, version = version + 1 WHERE name = ? AND deleted_at IS NULL`,
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
// wrong or the user has none. The check costs the same time either way,
// in a slot of the store's gate (ErrBusy when there is none). A match
// against a stale hash rehashes the password (rehash).
func (s *Store) CheckPassword(ctx context.Context, name, password string) error {
	leave, err := s.hashes.enter(ctx)
	if err != nil {
		return err
	}
	defer leave()
	var id int64
	var hash string
	err = s.db.QueryRowContext(ctx, `SELECT id, password_hash FROM users WHERE name = ? AND deleted_at IS NULL`, NormalName(name)).Scan(&id, &hash)
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
	s.rehash(ctx, id, hash, password)
	return nil
}

// rehash replaces a stale hash, whose password has just matched, by one
// of the current parameters, in the caller's slot. Only the hash is
// written, as the last login is: the password is the same, so the row
// keeps its version and update time. A hash changed meanwhile is left
// alone, and so is the hash of a read-only store; a failure is logged,
// since the old hash still verifies.
func (s *Store) rehash(ctx context.Context, id int64, old, password string) {
	if !stale(old) || s.writable() != nil {
		return
	}
	fresh, err := HashPassword(password)
	var res sql.Result
	if err == nil {
		res, err = s.db.ExecContext(context.WithoutCancel(ctx), `UPDATE users SET password_hash = ? WHERE id = ? AND password_hash = ?`, fresh, id, old)
	}
	if err != nil {
		s.log.Warn("store: password not rehashed", "user", id, "error", err)
		return
	}
	if n, _ := res.RowsAffected(); n == 1 {
		s.log.Info("store: password rehashed with the current parameters", "user", id)
	}
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

// DeleteUser removes a user: a tombstone that keeps its name, with every
// token of the user revoked, and the keys they made, and theirs, of any
// user (revokeHeirs, AP KeyLife, D5-A), every preference and every
// identity a tombstone too. The name is free again, and a user added under it later
// is a row of its own, with none of the removed one's history. The last
// owner without an end date is ErrLastOwner.
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
	if ownerLeft(u) {
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
	// The keys its tokens made, and theirs, of any user (AP KeyLife, D5-A):
	// its tokens are revoked, so their heirs go with them.
	heirs, err := revokeHeirs(ctx, tx, `SELECT id FROM tokens WHERE user_id = ? AND deleted_at IS NULL`, []any{u.ID}, now, s.nodeID)
	if err != nil {
		return fmt.Errorf("store: user %s: %w", u.Name, err)
	}
	if _, err := tx.ExecContext(ctx, `UPDATE tokens SET deleted_at = ?, origin = ?, version = version + 1 WHERE user_id = ? AND deleted_at IS NULL`, now, s.nodeID, u.ID); err != nil {
		return fmt.Errorf("store: user %s: %w", u.Name, err)
	}
	if err := s.removePreferences(ctx, tx, u.ID, now); err != nil {
		return fmt.Errorf("store: user %s: %w", u.Name, err)
	}
	if err := removePushSubscriptions(ctx, tx, u.ID, 0); err != nil {
		return fmt.Errorf("store: user %s: %w", u.Name, err)
	}
	// its identities: no sign-in finds it any more
	if _, err := tx.ExecContext(ctx, `UPDATE user_identities SET deleted_at = ?, updated_at = ?, origin = ?, version = version + 1
		WHERE effective_user_id = ? AND deleted_at IS NULL`, now, now, s.nodeID, u.ID); err != nil {
		return fmt.Errorf("store: user %s: %w", u.Name, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("store: user %s: %w", u.Name, err)
	}
	if len(heirs) > 0 {
		s.log.Info("keys made by a removed user's keys revoked", "user", u.ID, "heirs", heirs)
	}
	return nil
}

// RenameUser gives a user the login name to, with the password password
// under it, or none when empty. The password goes with the name: a
// record of version 2 holds the name it was registered under (the
// credential identifier, credentialID), so it is registered here again,
// both halves, as AddUserOpaque does. Sessions are the caller's to
// revoke, as with SetPassword; keys, preferences and push subscriptions
// stay with the row. A name taken by a user that is not removed is
// ErrUserExists.
func (s *Store) RenameUser(ctx context.Context, from, to, password string, by int64) (User, error) {
	if err := s.writable(); err != nil {
		return User{}, err
	}
	to = NormalName(to)
	if !userName.MatchString(to) {
		return User{}, fmt.Errorf("%w: %q", ErrBadName, to)
	}
	if _, ok, err := s.User(ctx, from); err != nil {
		return User{}, err
	} else if !ok {
		return User{}, fmt.Errorf("%w: %s", ErrNoUser, NormalName(from))
	}
	if err := s.nameFree(ctx, s.db, to); err != nil {
		return User{}, err
	}
	c := credential{}
	if password != "" {
		record, err := s.opaqueRecordFor(ctx, to, password)
		if err != nil {
			return User{}, err
		}
		c = credential{record: record, ksf: OpaqueKSF}
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return User{}, fmt.Errorf("store: rename user %s: %w", from, err)
	}
	defer tx.Rollback()
	u, ok, err := s.user(ctx, tx, from)
	if err != nil {
		return User{}, err
	}
	if !ok {
		return User{}, fmt.Errorf("%w: %s", ErrNoUser, NormalName(from))
	}
	if err := s.nameFree(ctx, tx, to); err != nil {
		return User{}, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE users SET name = ?, password_hash = '', opaque_record = ?, opaque_ksf = ?, auth_version = ?, updated_at = ?, updated_by = ?, origin = ?, version = version + 1 WHERE id = ?`,
		to, c.record, c.ksf, c.authVersion(), s.nowText(), nullID(by), s.nodeID, u.ID); err != nil {
		return User{}, fmt.Errorf("store: rename user %s: %w", u.Name, err)
	}
	if err := tx.Commit(); err != nil {
		return User{}, fmt.Errorf("store: rename user %s: %w", u.Name, err)
	}
	out, _, err := s.User(ctx, to)
	return out, err
}
