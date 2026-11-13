// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package store

// The users' identities at the OpenID Connect providers (docs/store.md
// section 6, "Sign-in through OpenID Connect"; migration 0016). A sign-in
// finds its user by the provider's subject (sub), which does not change;
// an e-mail an owner assigned is a pre-link, bound to the subject at the
// first sign-in with that e-mail verified, and by its subject alone from
// then on, since an address can change hands at a provider. An owner may
// assign the subject itself, the user id at the provider, which is bound
// from the start (AP Oidc v1.1, D26-A).

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Errors of the identities.
var (
	ErrNoIdentity     = errors.New("store: no such identity")
	ErrIdentityExists = errors.New("store: identity exists")
	ErrBadEmail       = errors.New("store: bad e-mail")
	// ErrIdentityTaken is a link of an account another user has.
	ErrIdentityTaken = errors.New("store: the account is another user's")
	// ErrIdentityUsed is a change of an identity a sign-in used: it is
	// removed and assigned again instead.
	ErrIdentityUsed = errors.New("store: the identity was used for a sign-in")
)

// IdentityDetails are what a provider says of an account at a sign-in or
// a link, kept on its identity for the user and the managers to see.
type IdentityDetails struct {
	Email    string
	Name     string
	Username string
	Groups   []string
}

// Identity is one row of user_identities, with its provider's key.
type Identity struct {
	ID         int64
	UUID       string
	UserID     int64
	ProviderID int64
	Provider   string // the provider's key
	Subject    string // "" until bound
	Email      string
	Name       string   // at the provider, from the last sign-in or link
	Username   string   // preferred_username, likewise
	Groups     []string // likewise
	CreatedAt  time.Time
	BoundAt    time.Time
	LastUsedAt time.Time
}

// Bound reports whether the identity has its subject.
func (i Identity) Bound() bool { return i.Subject != "" }

const identityColumns = `i.id, i.uuid, i.user_id, i.provider_id, p.key, coalesce(i.subject, ''), i.email, i.name, i.username, i.groups,
	i.created_at, i.bound_at, i.last_used_at`

const identityFrom = ` FROM user_identities i JOIN auth_providers p ON p.id = i.provider_id `

func scanIdentity(row scanner) (Identity, error) {
	var i Identity
	var created, bound, used sql.NullString
	var groups string
	if err := row.Scan(&i.ID, &i.UUID, &i.UserID, &i.ProviderID, &i.Provider, &i.Subject, &i.Email, &i.Name, &i.Username, &groups,
		&created, &bound, &used); err != nil {
		return Identity{}, err
	}
	if json.Unmarshal([]byte(groups), &i.Groups) != nil || i.Groups == nil {
		i.Groups = []string{}
	}
	i.CreatedAt, i.BoundAt, i.LastUsedAt = parseTime(created), parseTime(bound), parseTime(used)
	return i, nil
}

// normalEmail is an e-mail as kept: trimmed and lower case; ErrBadEmail
// for one without a local part and a domain.
func normalEmail(v string) (string, error) {
	v = strings.ToLower(strings.TrimSpace(v))
	at := strings.LastIndexByte(v, '@')
	if at < 1 || at == len(v)-1 || strings.ContainsAny(v, " \t\r\n,;") || len(v) > 254 {
		return "", fmt.Errorf("%w: %q", ErrBadEmail, v)
	}
	return v, nil
}

// Identities returns a user's identities, by provider.
func (s *Store) Identities(ctx context.Context, userID int64) ([]Identity, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+identityColumns+identityFrom+`WHERE i.user_id = ? AND p.deleted_at IS NULL ORDER BY p.key, i.id`, userID)
	if err != nil {
		return nil, fmt.Errorf("store: identities: %w", err)
	}
	defer rows.Close()
	out := []Identity{}
	for rows.Next() {
		i, err := scanIdentity(rows)
		if err != nil {
			return nil, fmt.Errorf("store: identities: %w", err)
		}
		out = append(out, i)
	}
	return out, rows.Err()
}

// AddIdentity assigns an e-mail at a provider to a user, a pre-link that
// the first sign-in with it binds. An e-mail waiting at the provider for
// another user, or for this one, is ErrIdentityExists.
func (s *Store) AddIdentity(ctx context.Context, userID int64, provider, email string, by int64) (Identity, error) {
	if err := s.writable(); err != nil {
		return Identity{}, err
	}
	email, err := normalEmail(email)
	if err != nil {
		return Identity{}, err
	}
	p, ok, err := s.Provider(ctx, provider)
	if err != nil {
		return Identity{}, err
	}
	if !ok {
		return Identity{}, fmt.Errorf("%w: %s", ErrNoProvider, provider)
	}
	var n int
	if err := s.db.QueryRowContext(ctx, `SELECT count(*) FROM user_identities WHERE provider_id = ? AND subject IS NULL AND email = ?`, p.ID, email).Scan(&n); err != nil {
		return Identity{}, fmt.Errorf("store: identity: %w", err)
	}
	if n > 0 {
		return Identity{}, fmt.Errorf("%w: %s at %s", ErrIdentityExists, email, p.Key)
	}
	res, err := s.db.ExecContext(ctx, `INSERT INTO user_identities (uuid, user_id, provider_id, email, created_at, created_by) VALUES (?, ?, ?, ?, ?, ?)`,
		newUUID(), userID, p.ID, email, s.nowText(), nullID(by))
	if err != nil {
		return Identity{}, fmt.Errorf("store: identity: %w", err)
	}
	id, _ := res.LastInsertId()
	return s.identity(ctx, s.db, id)
}

// AddSubjectIdentity assigns the user id at a provider (its sub) to a
// user, bound from the start: the next sign-in with it finds the user.
// A subject another identity at the provider has is ErrIdentityExists.
func (s *Store) AddSubjectIdentity(ctx context.Context, userID int64, provider, subject string, by int64) (Identity, error) {
	if err := s.writable(); err != nil {
		return Identity{}, err
	}
	subject = strings.TrimSpace(subject)
	if subject == "" || len(subject) > 255 {
		return Identity{}, fmt.Errorf("%w: a user id of 1 to 255 bytes", ErrBadProvider)
	}
	p, ok, err := s.Provider(ctx, provider)
	if err != nil {
		return Identity{}, err
	}
	if !ok {
		return Identity{}, fmt.Errorf("%w: %s", ErrNoProvider, provider)
	}
	var n int
	if err := s.db.QueryRowContext(ctx, `SELECT count(*) FROM user_identities WHERE provider_id = ? AND subject = ?`, p.ID, subject).Scan(&n); err != nil {
		return Identity{}, fmt.Errorf("store: identity: %w", err)
	}
	if n > 0 {
		return Identity{}, fmt.Errorf("%w: user id %s at %s", ErrIdentityExists, subject, p.Key)
	}
	now := s.nowText()
	res, err := s.db.ExecContext(ctx, `INSERT INTO user_identities (uuid, user_id, provider_id, subject, created_at, created_by, bound_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		newUUID(), userID, p.ID, subject, now, nullID(by), now)
	if err != nil {
		return Identity{}, fmt.Errorf("store: identity: %w", err)
	}
	id, _ := res.LastInsertId()
	return s.identity(ctx, s.db, id)
}

// EmailWaits reports whether an e-mail waits at a provider for a user
// that is not removed: a pre-link its first verified sign-in binds.
func (s *Store) EmailWaits(ctx context.Context, providerID int64, email string) (bool, error) {
	email, err := normalEmail(email)
	if err != nil {
		return false, nil
	}
	var n int
	if err := s.db.QueryRowContext(ctx, `SELECT count(*) FROM user_identities i JOIN users u ON u.id = i.user_id
		WHERE i.provider_id = ? AND i.subject IS NULL AND i.email = ? AND u.deleted_at IS NULL`, providerID, email).Scan(&n); err != nil {
		return false, fmt.Errorf("store: identity: %w", err)
	}
	return n > 0, nil
}

func (s *Store) identity(ctx context.Context, q queryer, id int64) (Identity, error) {
	i, err := scanIdentity(q.QueryRowContext(ctx, `SELECT `+identityColumns+identityFrom+`WHERE i.id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return Identity{}, fmt.Errorf("%w: %d", ErrNoIdentity, id)
	}
	if err != nil {
		return Identity{}, fmt.Errorf("store: identity %d: %w", id, err)
	}
	return i, nil
}

// DeleteIdentity removes one of a user's identities.
func (s *Store) DeleteIdentity(ctx context.Context, userID, id int64) error {
	if err := s.writable(); err != nil {
		return err
	}
	res, err := s.db.ExecContext(ctx, `DELETE FROM user_identities WHERE id = ? AND user_id = ?`, id, userID)
	if err != nil {
		return fmt.Errorf("store: identity %d: %w", id, err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("%w: %d", ErrNoIdentity, id)
	}
	return nil
}

// UserBySubject finds the user of a provider's subject; false where no
// user that is not removed has it.
func (s *Store) UserBySubject(ctx context.Context, providerID int64, subject string) (User, Identity, bool, error) {
	i, err := scanIdentity(s.db.QueryRowContext(ctx, `SELECT `+identityColumns+identityFrom+`JOIN users u ON u.id = i.user_id
		WHERE i.provider_id = ? AND i.subject = ? AND u.deleted_at IS NULL`, providerID, subject))
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, Identity{}, false, nil
	}
	if err != nil {
		return User{}, Identity{}, false, fmt.Errorf("store: identity: %w", err)
	}
	u, ok, err := s.UserByID(ctx, i.UserID)
	if err != nil || !ok {
		return User{}, Identity{}, false, err
	}
	return u, i, true, nil
}

// BindIdentity binds the pre-link of a provider with the e-mail to the
// subject, and returns its user; false where none waits for the e-mail.
// The caller has checked that the provider verified the e-mail.
func (s *Store) BindIdentity(ctx context.Context, providerID int64, email, subject string) (User, Identity, bool, error) {
	if err := s.writable(); err != nil {
		return User{}, Identity{}, false, err
	}
	email, err := normalEmail(email)
	if err != nil || subject == "" {
		return User{}, Identity{}, false, nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return User{}, Identity{}, false, fmt.Errorf("store: identity: %w", err)
	}
	defer tx.Rollback()
	var id int64
	err = tx.QueryRowContext(ctx, `SELECT i.id FROM user_identities i JOIN users u ON u.id = i.user_id
		WHERE i.provider_id = ? AND i.subject IS NULL AND i.email = ? AND u.deleted_at IS NULL`, providerID, email).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, Identity{}, false, nil
	}
	if err != nil {
		return User{}, Identity{}, false, fmt.Errorf("store: identity: %w", err)
	}
	now := s.nowText()
	if _, err := tx.ExecContext(ctx, `UPDATE user_identities SET subject = ?, bound_at = ?, last_used_at = ? WHERE id = ?`, subject, now, now, id); err != nil {
		return User{}, Identity{}, false, fmt.Errorf("store: identity: %w", err)
	}
	i, err := s.identity(ctx, tx, id)
	if err != nil {
		return User{}, Identity{}, false, err
	}
	if err := tx.Commit(); err != nil {
		return User{}, Identity{}, false, fmt.Errorf("store: identity: %w", err)
	}
	u, ok, err := s.UserByID(ctx, i.UserID)
	return u, i, ok, err
}

// AddIdentityUser creates a user for a provider's subject, a sign-in
// without an identity at a provider that creates users: the name u
// gives, or with -2, -3 … where it is taken, and the identity bound.
func (s *Store) AddIdentityUser(ctx context.Context, u User, providerID int64, subject, email string) (User, Identity, error) {
	base, err := s.newUser(u)
	if err != nil {
		return User{}, Identity{}, err
	}
	if subject == "" {
		return User{}, Identity{}, fmt.Errorf("%w: no subject", ErrBadProvider)
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return User{}, Identity{}, fmt.Errorf("store: add user %s: %w", base.Name, err)
	}
	defer tx.Rollback()
	role, err := s.roleID(ctx, tx, base.Role)
	if err != nil {
		return User{}, Identity{}, err
	}
	name := base.Name
	for n := 2; s.nameFree(ctx, tx, name) != nil; n++ {
		if n > 99 {
			return User{}, Identity{}, fmt.Errorf("%w: %s", ErrUserExists, base.Name)
		}
		suffix := "-" + strconv.Itoa(n)
		name = base.Name[:min(len(base.Name), 64-len(suffix))] + suffix
	}
	now := s.nowText()
	res, err := tx.ExecContext(ctx, `INSERT INTO users (uuid, name, display_name, role_id, created_at, updated_at, origin) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		newUUID(), name, base.DisplayName, role, now, now, s.nodeID)
	if err != nil {
		return User{}, Identity{}, fmt.Errorf("store: add user %s: %w", name, err)
	}
	userID, _ := res.LastInsertId()
	if email != "" {
		if email, err = normalEmail(email); err != nil {
			email = ""
		}
	}
	res, err = tx.ExecContext(ctx, `INSERT INTO user_identities (uuid, user_id, provider_id, subject, email, created_at, bound_at, last_used_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		newUUID(), userID, providerID, subject, email, now, now, now)
	if err != nil {
		return User{}, Identity{}, fmt.Errorf("store: identity: %w", err)
	}
	id, _ := res.LastInsertId()
	i, err := s.identity(ctx, tx, id)
	if err != nil {
		return User{}, Identity{}, err
	}
	if err := tx.Commit(); err != nil {
		return User{}, Identity{}, fmt.Errorf("store: add user %s: %w", name, err)
	}
	out, _, err := s.User(ctx, name)
	return out, i, err
}

// TouchIdentity records a sign-in through the identity, and what the
// provider said of the account; an e-mail it did not send stays.
func (s *Store) TouchIdentity(ctx context.Context, id int64, d IdentityDetails) error {
	if err := s.writable(); err != nil {
		return err
	}
	d = cleanDetails(d)
	if _, err := s.db.ExecContext(ctx, `UPDATE user_identities SET last_used_at = ?, email = CASE WHEN ? = '' THEN email ELSE ? END,
		name = ?, username = ?, groups = ? WHERE id = ?`, s.nowText(), d.Email, d.Email, d.Name, d.Username, groupsText(d.Groups), id); err != nil {
		return fmt.Errorf("store: identity %d: %w", id, err)
	}
	return nil
}

// cleanDetails bounds what a provider says: an e-mail as kept or none,
// names of at most 128 bytes, at most 100 groups of at most 128 bytes.
func cleanDetails(d IdentityDetails) IdentityDetails {
	if e, err := normalEmail(d.Email); err == nil {
		d.Email = e
	} else {
		d.Email = ""
	}
	cut := func(v string) string {
		v = strings.TrimSpace(v)
		if len(v) > 128 {
			v = v[:128]
		}
		return v
	}
	d.Name, d.Username = cut(d.Name), cut(d.Username)
	groups := []string{}
	for _, g := range d.Groups {
		if g = cut(g); g != "" && len(groups) < 100 && !containsString(groups, g) {
			groups = append(groups, g)
		}
	}
	d.Groups = groups
	return d
}

func groupsText(groups []string) string {
	if groups == nil {
		groups = []string{}
	}
	b, _ := json.Marshal(groups)
	return string(b)
}

// LinkIdentity links an account at a provider to a user, the user's own
// doing: the account another user's is ErrIdentityTaken; the user's own
// already, its details are written; else it replaces the user's other
// identities at the provider, bound from now on.
func (s *Store) LinkIdentity(ctx context.Context, userID, providerID int64, subject string, d IdentityDetails) (Identity, error) {
	if err := s.writable(); err != nil {
		return Identity{}, err
	}
	if subject == "" {
		return Identity{}, fmt.Errorf("%w: no subject", ErrBadProvider)
	}
	d = cleanDetails(d)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Identity{}, fmt.Errorf("store: link: %w", err)
	}
	defer tx.Rollback()
	var id, owner int64
	err = tx.QueryRowContext(ctx, `SELECT id, user_id FROM user_identities WHERE provider_id = ? AND subject = ?`, providerID, subject).Scan(&id, &owner)
	switch {
	case err == nil && owner != userID:
		return Identity{}, ErrIdentityTaken
	case err != nil && !errors.Is(err, sql.ErrNoRows):
		return Identity{}, fmt.Errorf("store: link: %w", err)
	}
	now := s.nowText()
	if _, err := tx.ExecContext(ctx, `DELETE FROM user_identities WHERE user_id = ? AND provider_id = ? AND id != ?`, userID, providerID, id); err != nil {
		return Identity{}, fmt.Errorf("store: link: %w", err)
	}
	if id == 0 {
		res, err := tx.ExecContext(ctx, `INSERT INTO user_identities (uuid, user_id, provider_id, subject, email, name, username, groups, created_at, created_by, bound_at, last_used_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			newUUID(), userID, providerID, subject, d.Email, d.Name, d.Username, groupsText(d.Groups), now, userID, now, now)
		if err != nil {
			return Identity{}, fmt.Errorf("store: link: %w", err)
		}
		id, _ = res.LastInsertId()
	} else if _, err := tx.ExecContext(ctx, `UPDATE user_identities SET email = CASE WHEN ? = '' THEN email ELSE ? END, name = ?, username = ?, groups = ?,
		bound_at = coalesce(bound_at, ?), last_used_at = ? WHERE id = ?`, d.Email, d.Email, d.Name, d.Username, groupsText(d.Groups), now, now, id); err != nil {
		return Identity{}, fmt.Errorf("store: link: %w", err)
	}
	i, err := s.identity(ctx, tx, id)
	if err != nil {
		return Identity{}, err
	}
	if err := tx.Commit(); err != nil {
		return Identity{}, fmt.Errorf("store: link: %w", err)
	}
	return i, nil
}

// UpdateIdentity changes the e-mail, or the user id there, of one of a
// user's identities no sign-in has used yet (ErrIdentityUsed): an e-mail
// makes it a waiting one, a user id a bound one. The same rules as
// AddIdentity and AddSubjectIdentity hold.
func (s *Store) UpdateIdentity(ctx context.Context, userID, id int64, email, subject string) (Identity, error) {
	if err := s.writable(); err != nil {
		return Identity{}, err
	}
	subject = strings.TrimSpace(subject)
	if (email == "") == (subject == "") || len(subject) > 255 {
		return Identity{}, fmt.Errorf("%w: an e-mail or a user id of 1 to 255 bytes", ErrBadProvider)
	}
	if email != "" {
		var err error
		if email, err = normalEmail(email); err != nil {
			return Identity{}, err
		}
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Identity{}, fmt.Errorf("store: identity %d: %w", id, err)
	}
	defer tx.Rollback()
	i, err := s.identity(ctx, tx, id)
	if err != nil {
		return Identity{}, err
	}
	if i.UserID != userID {
		return Identity{}, fmt.Errorf("%w: %d", ErrNoIdentity, id)
	}
	if !i.LastUsedAt.IsZero() {
		return Identity{}, fmt.Errorf("%w: remove it and assign it again", ErrIdentityUsed)
	}
	var n int
	if email != "" {
		err = tx.QueryRowContext(ctx, `SELECT count(*) FROM user_identities WHERE provider_id = ? AND subject IS NULL AND email = ? AND id != ?`, i.ProviderID, email, id).Scan(&n)
	} else {
		err = tx.QueryRowContext(ctx, `SELECT count(*) FROM user_identities WHERE provider_id = ? AND subject = ? AND id != ?`, i.ProviderID, subject, id).Scan(&n)
	}
	if err != nil {
		return Identity{}, fmt.Errorf("store: identity %d: %w", id, err)
	}
	if n > 0 {
		return Identity{}, fmt.Errorf("%w: at %s", ErrIdentityExists, i.Provider)
	}
	if email != "" {
		_, err = tx.ExecContext(ctx, `UPDATE user_identities SET email = ?, subject = NULL, bound_at = NULL WHERE id = ?`, email, id)
	} else {
		_, err = tx.ExecContext(ctx, `UPDATE user_identities SET subject = ?, email = '', bound_at = ? WHERE id = ?`, subject, s.nowText(), id)
	}
	if err != nil {
		return Identity{}, fmt.Errorf("store: identity %d: %w", id, err)
	}
	if i, err = s.identity(ctx, tx, id); err != nil {
		return Identity{}, err
	}
	if err := tx.Commit(); err != nil {
		return Identity{}, fmt.Errorf("store: identity %d: %w", id, err)
	}
	return i, nil
}

// Identity returns one of a user's identities.
func (s *Store) Identity(ctx context.Context, userID, id int64) (Identity, error) {
	i, err := s.identity(ctx, s.db, id)
	if err == nil && i.UserID != userID {
		return Identity{}, fmt.Errorf("%w: %d", ErrNoIdentity, id)
	}
	return i, err
}
