// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package store

// The OpenID Connect providers a user may sign in through (docs/store.md
// section 6, "Sign-in through OpenID Connect"; migration 0016): each a
// key, the issuer whose discovery document names its endpoints, the
// client's id and secret, and whether a sign-in without an identity
// creates the user. Which role such a user may get is the server's rule
// (internal/server); the store keeps the role it is given.

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"
)

// Errors of the providers.
var (
	ErrNoProvider     = errors.New("store: no such provider")
	ErrProviderExists = errors.New("store: provider exists")
	ErrBadProvider    = errors.New("store: bad provider")
)

// DefaultScopes are the scopes a provider asks for when none are given.
const DefaultScopes = "openid email profile"

// AuthProvider is one row of auth_providers.
type AuthProvider struct {
	ID           int64
	UUID         string
	Key          string // in the routes, as a role's key
	Name         string // on the sign-in button; "" shows the key
	Issuer       string
	ClientID     string
	ClientSecret string // never answered by the API
	Scopes       string // space-separated, openid among them
	AutoCreate   bool
	AutoRole     string   // the key of a created user's role; "" for viewer
	AutoDomains  []string // the e-mail domains a sign-in creates users for; none: any
	Enabled      bool
	CreatedAt    time.Time
	UpdatedAt    time.Time
	Version      int64
}

const providerColumns = `p.id, p.uuid, p.key, p.name, p.issuer, p.client_id, p.client_secret, p.scopes, p.auto_create,
	coalesce(r.key, ''), p.auto_domains, p.enabled, p.created_at, p.updated_at, p.version`

const providerFrom = ` FROM auth_providers p LEFT JOIN roles r ON r.id = p.auto_role_id `

func scanProvider(row scanner) (AuthProvider, error) {
	var p AuthProvider
	var domains string
	var created, updated sql.NullString
	if err := row.Scan(&p.ID, &p.UUID, &p.Key, &p.Name, &p.Issuer, &p.ClientID, &p.ClientSecret, &p.Scopes, &p.AutoCreate,
		&p.AutoRole, &domains, &p.Enabled, &created, &updated, &p.Version); err != nil {
		return AuthProvider{}, err
	}
	p.AutoDomains = splitDomains(domains)
	p.CreatedAt, p.UpdatedAt = parseTime(created), parseTime(updated)
	return p, nil
}

// splitDomains reads auto_domains: lower case, without blanks or repeats.
func splitDomains(v string) []string {
	out := []string{}
	for _, d := range strings.Split(v, ",") {
		d = strings.ToLower(strings.TrimSpace(d))
		if d != "" && !containsString(out, d) {
			out = append(out, d)
		}
	}
	return out
}

func containsString(list []string, v string) bool {
	for _, s := range list {
		if s == v {
			return true
		}
	}
	return false
}

// AllowsDomain reports whether a sign-in with the e-mail may create a
// user: any domain where the provider names none, else one of them.
func (p AuthProvider) AllowsDomain(email string) bool {
	if len(p.AutoDomains) == 0 {
		return true
	}
	at := strings.LastIndexByte(email, '@')
	return at > 0 && containsString(p.AutoDomains, strings.ToLower(email[at+1:]))
}

// checkProvider normalises a provider to write and checks it: the key as
// a role's, an issuer that is an absolute https URL (http on a loopback
// host, for tests and a provider on the same box), a client id, scopes
// with openid, domains without an @.
func checkProvider(p *AuthProvider) error {
	p.Key = strings.ToLower(strings.TrimSpace(p.Key))
	p.Name = strings.TrimSpace(p.Name)
	p.Issuer = strings.TrimSpace(p.Issuer)
	p.ClientID = strings.TrimSpace(p.ClientID)
	p.Scopes = strings.Join(strings.Fields(p.Scopes), " ")
	if p.Scopes == "" {
		p.Scopes = DefaultScopes
	}
	p.AutoDomains = splitDomains(strings.Join(p.AutoDomains, ","))
	switch {
	case !roleKey.MatchString(p.Key):
		return fmt.Errorf("%w: key %q: lower case letters, digits, _ and -, starting with a letter", ErrBadProvider, p.Key)
	case !validIssuer(p.Issuer):
		return fmt.Errorf("%w: issuer %q: an https URL without a query or fragment (http only on loopback)", ErrBadProvider, p.Issuer)
	case p.ClientID == "":
		return fmt.Errorf("%w: a client id is needed", ErrBadProvider)
	case !containsString(strings.Fields(p.Scopes), "openid"):
		return fmt.Errorf("%w: the scopes must include openid", ErrBadProvider)
	case len(p.Name) > 64:
		return fmt.Errorf("%w: name longer than 64 bytes", ErrBadProvider)
	}
	for _, d := range p.AutoDomains {
		if strings.ContainsAny(d, "@ /") {
			return fmt.Errorf("%w: domain %q", ErrBadProvider, d)
		}
	}
	return nil
}

// validIssuer reports whether an issuer is an absolute https URL without
// a query or fragment, or http on a loopback host.
func validIssuer(v string) bool {
	u, err := url.Parse(v)
	if err != nil || u.Host == "" || u.RawQuery != "" || u.Fragment != "" || u.User != nil {
		return false
	}
	switch u.Scheme {
	case "https":
		return true
	case "http":
		host := u.Hostname()
		ip := net.ParseIP(host)
		return host == "localhost" || ip != nil && ip.IsLoopback()
	}
	return false
}

// Providers returns the providers, by key.
func (s *Store) Providers(ctx context.Context) ([]AuthProvider, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+providerColumns+providerFrom+`WHERE p.deleted_at IS NULL ORDER BY p.key`)
	if err != nil {
		return nil, fmt.Errorf("store: providers: %w", err)
	}
	defer rows.Close()
	out := []AuthProvider{}
	for rows.Next() {
		p, err := scanProvider(rows)
		if err != nil {
			return nil, fmt.Errorf("store: providers: %w", err)
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// Provider returns a provider by key.
func (s *Store) Provider(ctx context.Context, key string) (AuthProvider, bool, error) {
	return s.provider(ctx, s.db, key)
}

func (s *Store) provider(ctx context.Context, q queryer, key string) (AuthProvider, bool, error) {
	p, err := scanProvider(q.QueryRowContext(ctx, `SELECT `+providerColumns+providerFrom+`WHERE p.key = ? AND p.deleted_at IS NULL`, strings.ToLower(strings.TrimSpace(key))))
	if errors.Is(err, sql.ErrNoRows) {
		return AuthProvider{}, false, nil
	}
	if err != nil {
		return AuthProvider{}, false, fmt.Errorf("store: provider %s: %w", key, err)
	}
	return p, true, nil
}

// autoRoleID is the row of a created user's role, NULL for viewer.
func (s *Store) autoRoleID(ctx context.Context, q queryer, key string) (any, error) {
	if key == "" {
		return nil, nil
	}
	id, err := s.roleID(ctx, q, key)
	if err != nil {
		return nil, err
	}
	return id, nil
}

// AddProvider adds a provider, by the user by (0: the command line).
func (s *Store) AddProvider(ctx context.Context, p AuthProvider, by int64) (AuthProvider, error) {
	if err := s.writable(); err != nil {
		return AuthProvider{}, err
	}
	if err := checkProvider(&p); err != nil {
		return AuthProvider{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return AuthProvider{}, fmt.Errorf("store: add provider %s: %w", p.Key, err)
	}
	defer tx.Rollback()
	if _, ok, err := s.provider(ctx, tx, p.Key); err != nil {
		return AuthProvider{}, err
	} else if ok {
		return AuthProvider{}, fmt.Errorf("%w: %s", ErrProviderExists, p.Key)
	}
	role, err := s.autoRoleID(ctx, tx, p.AutoRole)
	if err != nil {
		return AuthProvider{}, err
	}
	now := s.nowText()
	if _, err := tx.ExecContext(ctx, `INSERT INTO auth_providers (uuid, key, name, issuer, client_id, client_secret, scopes, auto_create, auto_role_id, auto_domains, enabled, created_at, updated_at, updated_by, origin)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		newUUID(), p.Key, p.Name, p.Issuer, p.ClientID, p.ClientSecret, p.Scopes, p.AutoCreate, role, strings.Join(p.AutoDomains, ","), p.Enabled, now, now, nullID(by), s.nodeID); err != nil {
		return AuthProvider{}, fmt.Errorf("store: add provider %s: %w", p.Key, err)
	}
	if err := tx.Commit(); err != nil {
		return AuthProvider{}, fmt.Errorf("store: add provider %s: %w", p.Key, err)
	}
	out, _, err := s.Provider(ctx, p.Key)
	return out, err
}

// UpdateProvider changes a provider with change, in one transaction; its
// key stays.
func (s *Store) UpdateProvider(ctx context.Context, key string, change func(*AuthProvider) error, by int64) (AuthProvider, error) {
	if err := s.writable(); err != nil {
		return AuthProvider{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return AuthProvider{}, fmt.Errorf("store: provider %s: %w", key, err)
	}
	defer tx.Rollback()
	p, ok, err := s.provider(ctx, tx, key)
	if err != nil {
		return AuthProvider{}, err
	}
	if !ok {
		return AuthProvider{}, fmt.Errorf("%w: %s", ErrNoProvider, key)
	}
	k := p.Key
	if err := change(&p); err != nil {
		return AuthProvider{}, err
	}
	p.Key = k
	if err := checkProvider(&p); err != nil {
		return AuthProvider{}, err
	}
	role, err := s.autoRoleID(ctx, tx, p.AutoRole)
	if err != nil {
		return AuthProvider{}, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE auth_providers SET name = ?, issuer = ?, client_id = ?, client_secret = ?, scopes = ?, auto_create = ?, auto_role_id = ?, auto_domains = ?, enabled = ?,
		updated_at = ?, updated_by = ?, origin = ?, version = version + 1 WHERE id = ?`,
		p.Name, p.Issuer, p.ClientID, p.ClientSecret, p.Scopes, p.AutoCreate, role, strings.Join(p.AutoDomains, ","), p.Enabled,
		s.nowText(), nullID(by), s.nodeID, p.ID); err != nil {
		return AuthProvider{}, fmt.Errorf("store: provider %s: %w", p.Key, err)
	}
	if err := tx.Commit(); err != nil {
		return AuthProvider{}, fmt.Errorf("store: provider %s: %w", p.Key, err)
	}
	out, _, err := s.Provider(ctx, p.Key)
	return out, err
}

// DeleteProvider removes a provider: a tombstone, whose key is free
// again, and every identity at it deleted, so that no user signs in
// through it any more.
func (s *Store) DeleteProvider(ctx context.Context, key string, by int64) error {
	if err := s.writable(); err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("store: provider %s: %w", key, err)
	}
	defer tx.Rollback()
	p, ok, err := s.provider(ctx, tx, key)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("%w: %s", ErrNoProvider, key)
	}
	now := s.nowText()
	if _, err := tx.ExecContext(ctx, `DELETE FROM user_identities WHERE provider_id = ?`, p.ID); err != nil {
		return fmt.Errorf("store: provider %s: %w", p.Key, err)
	}
	if _, err := tx.ExecContext(ctx, `UPDATE auth_providers SET deleted_at = ?, client_secret = '', updated_at = ?, updated_by = ?, origin = ?, version = version + 1 WHERE id = ?`,
		now, now, nullID(by), s.nodeID, p.ID); err != nil {
		return fmt.Errorf("store: provider %s: %w", p.Key, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("store: provider %s: %w", p.Key, err)
	}
	return nil
}
