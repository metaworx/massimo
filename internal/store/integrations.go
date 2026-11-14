// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package store

// Integrations (docs/store.md section 6, "Integrations"; migration 0021):
// programs such as Home Assistant that act for users of their own. An
// integration asks to be registered without a key (AddIntegrationRequest);
// the owner compares the short code both sides show and approves it
// (ApproveIntegration), which makes its own user, holding its key, and
// its user "system", and the integration collects its key with the code
// (CollectIntegrationKey). A request from an installation that was
// registered before can be attached to that integration instead, keeping
// its users and links. Revoking deletes the key and disables the
// integration; its rows stay.

import (
	"context"
	"crypto/subtle"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
)

// The kinds of integration.
const KindHomeAssistant = "homeassistant"

// The states of an integration.
const (
	IntegrationPending  = "pending"  // a request waiting for the owner
	IntegrationActive   = "active"   // registered, its key valid
	IntegrationDisabled = "disabled" // revoked or switched off
	IntegrationRefused  = "refused"  // a request refused, expired or attached to another
)

// RequestLife is how long a registration request waits for the owner,
// and how long an approved one waits for its key to be collected.
const RequestLife = 10 * time.Minute

// MaxPendingRequests is how many registration requests may wait at once.
const MaxPendingRequests = 5

// Errors of the integrations.
var (
	ErrNoIntegration   = errors.New("store: no such integration")
	ErrTooManyRequests = errors.New("store: too many registration requests waiting")
	ErrRequestPending  = errors.New("store: the request waits for the owner")
	ErrRequestGone     = errors.New("store: the request was refused, expired, or its key collected and used")
	ErrNotPending      = errors.New("store: the integration is not a pending request")
	ErrBadInstance     = errors.New("store: bad installation id")
)

// instanceID is an installation id: 32 hex digits, lower case.
var instanceID = regexp.MustCompile(`^[0-9a-f]{32}$`)

// ValidInstanceID reports whether s is an installation id.
func ValidInstanceID(s string) bool { return instanceID.MatchString(s) }

// Integration is one row of integrations.
type Integration struct {
	ID              int64
	UUID            string
	Kind            string
	Key             string // lower case; its users' names begin with it
	Name            string
	InstanceID      string
	PendingInstance string
	URL             string
	AppVersion      string
	MaxCaps         []string
	UserID          int64 // its own user, holding its key
	SystemUserID    int64 // its user "system"
	State           string
	RequestCode     string
	RequestExpires  time.Time
	RequestRemote   string
	KeyIssuedAt     time.Time
	MergedInto      int64
	CreatedAt       time.Time
	UpdatedAt       time.Time
	Version         int64
}

const integrationColumns = `id, uuid, kind, key, name, instance_id, pending_instance, url, app_version, max_caps,
	coalesce(user_id, 0), coalesce(system_user_id, 0), state, request_code, request_expires, request_remote, key_issued_at,
	coalesce(merged_into, 0), created_at, updated_at, version`

func scanIntegration(row scanner) (Integration, error) {
	var i Integration
	var caps string
	var expires, issued, created, updated sql.NullString
	if err := row.Scan(&i.ID, &i.UUID, &i.Kind, &i.Key, &i.Name, &i.InstanceID, &i.PendingInstance, &i.URL, &i.AppVersion, &caps,
		&i.UserID, &i.SystemUserID, &i.State, &i.RequestCode, &expires, &i.RequestRemote, &issued,
		&i.MergedInto, &created, &updated, &i.Version); err != nil {
		return Integration{}, err
	}
	if json.Unmarshal([]byte(caps), &i.MaxCaps) != nil || i.MaxCaps == nil {
		i.MaxCaps = []string{}
	}
	i.RequestExpires, i.KeyIssuedAt, i.CreatedAt, i.UpdatedAt = parseTime(expires), parseTime(issued), parseTime(created), parseTime(updated)
	return i, nil
}

// IntegrationRequest is what an integration says of itself when it asks
// to be registered.
type IntegrationRequest struct {
	Kind       string
	Name       string
	InstanceID string
	URL        string
	AppVersion string
	Remote     string // the address it asked from
}

// AddIntegrationRequest keeps a registration request with its short code
// for RequestLife. More than MaxPendingRequests waiting is
// ErrTooManyRequests.
func (s *Store) AddIntegrationRequest(ctx context.Context, r IntegrationRequest, code string) (Integration, error) {
	if err := s.writable(); err != nil {
		return Integration{}, err
	}
	if r.Kind != KindHomeAssistant {
		return Integration{}, fmt.Errorf("store: integration kind %q: %s only", r.Kind, KindHomeAssistant)
	}
	if !ValidInstanceID(r.InstanceID) {
		return Integration{}, fmt.Errorf("%w: %q", ErrBadInstance, r.InstanceID)
	}
	now := s.now()
	var waiting int
	if err := s.db.QueryRowContext(ctx, `SELECT count(*) FROM integrations WHERE state = ? AND request_expires > ? AND deleted_at IS NULL`,
		IntegrationPending, formatTime(now)).Scan(&waiting); err != nil {
		return Integration{}, fmt.Errorf("store: integration request: %w", err)
	}
	if waiting >= MaxPendingRequests {
		return Integration{}, ErrTooManyRequests
	}
	uuid := newUUID()
	if _, err := s.db.ExecContext(ctx, `INSERT INTO integrations (uuid, kind, key, name, instance_id, url, app_version, state, request_code, request_expires,
		request_remote, created_at, updated_at, origin) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		uuid, r.Kind, "request-"+uuid, strings.TrimSpace(r.Name), r.InstanceID, r.URL, r.AppVersion, IntegrationPending, code,
		formatTime(now.Add(RequestLife)), r.Remote, formatTime(now), formatTime(now), s.nodeID); err != nil {
		return Integration{}, fmt.Errorf("store: integration request: %w", err)
	}
	i, _, err := s.IntegrationByUUID(ctx, uuid)
	return i, err
}

// Integrations lists the integrations and the requests still waiting,
// by name; refused requests and expired ones are left out.
func (s *Store) Integrations(ctx context.Context) ([]Integration, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+integrationColumns+` FROM integrations WHERE deleted_at IS NULL
		AND (state IN (?, ?) OR state = ? AND request_expires > ?) ORDER BY name, id`,
		IntegrationActive, IntegrationDisabled, IntegrationPending, s.nowText())
	if err != nil {
		return nil, fmt.Errorf("store: integrations: %w", err)
	}
	defer rows.Close()
	out := []Integration{}
	for rows.Next() {
		i, err := scanIntegration(rows)
		if err != nil {
			return nil, fmt.Errorf("store: integrations: %w", err)
		}
		out = append(out, i)
	}
	return out, rows.Err()
}

// IntegrationByUUID returns an integration or a request, whatever its
// state.
func (s *Store) IntegrationByUUID(ctx context.Context, uuid string) (Integration, bool, error) {
	return s.integrationBy(ctx, s.db, "uuid", uuid)
}

// IntegrationOfUser returns the integration whose own user userID is,
// the holder of its key.
func (s *Store) IntegrationOfUser(ctx context.Context, userID int64) (Integration, bool, error) {
	return s.integrationBy(ctx, s.db, "user_id", userID)
}

func (s *Store) integrationBy(ctx context.Context, q queryer, col string, v any) (Integration, bool, error) {
	i, err := scanIntegration(q.QueryRowContext(ctx, `SELECT `+integrationColumns+` FROM integrations WHERE `+col+` = ? AND deleted_at IS NULL`, v))
	if errors.Is(err, sql.ErrNoRows) {
		return Integration{}, false, nil
	}
	if err != nil {
		return Integration{}, false, fmt.Errorf("store: integration: %w", err)
	}
	return i, true, nil
}

// integrationKey is the key an integration of name gets: lower case, the
// characters of a user name, "integration" for a name without any.
func integrationKey(name string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(name) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case b.Len() > 0 && !strings.HasSuffix(b.String(), "-"):
			b.WriteByte('-')
		}
	}
	k := strings.Trim(b.String(), "-")
	if len(k) > 40 {
		k = strings.Trim(k[:40], "-")
	}
	if k == "" {
		k = "integration"
	}
	return k
}

// ApproveIntegration registers a pending request with the maximum caps
// (a subset of the role integration, the caller's to check). With attach,
// the uuid of an integration of the same kind, the request is attached to
// it instead: that integration takes the request's installation id,
// address and version and is active again, with its users, links and
// maximum (caps, where given, replacing it). Either way the key is
// collected with the request's code within RequestLife from now.
func (s *Store) ApproveIntegration(ctx context.Context, uuid string, caps []string, attach string, by int64) (Integration, error) {
	if err := s.writable(); err != nil {
		return Integration{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Integration{}, fmt.Errorf("store: approve integration: %w", err)
	}
	defer tx.Rollback()
	req, ok, err := s.integrationBy(ctx, tx, "uuid", uuid)
	if err != nil {
		return Integration{}, err
	}
	now := s.now()
	if !ok || req.State != IntegrationPending || !now.Before(req.RequestExpires) {
		return Integration{}, ErrNotPending
	}
	expires := formatTime(now.Add(RequestLife))
	result := uuid
	if attach != "" {
		target, ok, err := s.integrationBy(ctx, tx, "uuid", attach)
		if err != nil {
			return Integration{}, err
		}
		if !ok || target.Kind != req.Kind || (target.State != IntegrationActive && target.State != IntegrationDisabled) {
			return Integration{}, fmt.Errorf("%w: %s", ErrNoIntegration, attach)
		}
		set := `instance_id = ?, pending_instance = '', url = ?, app_version = ?, state = ?, request_code = ?, request_expires = ?, key_issued_at = NULL`
		args := []any{req.InstanceID, req.URL, req.AppVersion, IntegrationActive, req.RequestCode, expires}
		if caps != nil {
			raw, _ := json.Marshal(normalCaps(caps))
			set += `, max_caps = ?`
			args = append(args, string(raw))
		}
		if _, err := tx.ExecContext(ctx, `UPDATE integrations SET `+set+`, updated_at = ?, updated_by = ?, origin = ?, version = version + 1 WHERE id = ?`,
			append(args, formatTime(now), nullID(by), s.nodeID, target.ID)...); err != nil {
			return Integration{}, fmt.Errorf("store: approve integration: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `UPDATE integrations SET state = ?, merged_into = ?, updated_at = ?, updated_by = ?, version = version + 1 WHERE id = ?`,
			IntegrationRefused, target.ID, formatTime(now), nullID(by), req.ID); err != nil {
			return Integration{}, fmt.Errorf("store: approve integration: %w", err)
		}
		if err := enableUsers(ctx, tx, target.UserID, target.SystemUserID); err != nil {
			return Integration{}, err
		}
		result = target.UUID
	} else {
		key, err := s.freeIntegrationKey(ctx, tx, integrationKey(req.Name))
		if err != nil {
			return Integration{}, err
		}
		own, err := s.addUserTx(ctx, tx, User{Name: key, DisplayName: req.Name, Role: RoleIntegration}, by)
		if err != nil {
			return Integration{}, fmt.Errorf("store: approve integration: %w", err)
		}
		sys, err := s.addUserTx(ctx, tx, User{Name: key + ".system", DisplayName: "system", Role: RoleIntegration}, by)
		if err != nil {
			return Integration{}, fmt.Errorf("store: approve integration: %w", err)
		}
		if caps == nil {
			caps = []string{}
		}
		raw, _ := json.Marshal(normalCaps(caps))
		if _, err := tx.ExecContext(ctx, `UPDATE integrations SET key = ?, state = ?, max_caps = ?, user_id = ?, system_user_id = ?, request_expires = ?,
			updated_at = ?, updated_by = ?, origin = ?, version = version + 1 WHERE id = ?`,
			key, IntegrationActive, string(raw), own.ID, sys.ID, expires, formatTime(now), nullID(by), s.nodeID, req.ID); err != nil {
			return Integration{}, fmt.Errorf("store: approve integration: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO user_identities (uuid, user_id, integration_id, subject, name, own_user_id, created_at, created_by, bound_at)
			VALUES (?, ?, ?, 'system', 'system', ?, ?, ?, ?)`, newUUID(), sys.ID, req.ID, sys.ID, formatTime(now), nullID(by), formatTime(now)); err != nil {
			return Integration{}, fmt.Errorf("store: approve integration: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return Integration{}, fmt.Errorf("store: approve integration: %w", err)
	}
	i, _, err := s.IntegrationByUUID(ctx, result)
	return i, err
}

// freeIntegrationKey is base, or base-2, base-3 …, the first that no
// integration and no user has.
func (s *Store) freeIntegrationKey(ctx context.Context, tx *sql.Tx, base string) (string, error) {
	for n := 1; n < 1000; n++ {
		key := base
		if n > 1 {
			key = fmt.Sprintf("%s-%d", base, n)
		}
		var taken int
		if err := tx.QueryRowContext(ctx, `SELECT (SELECT count(*) FROM integrations WHERE key = ? AND deleted_at IS NULL)
			+ (SELECT count(*) FROM users WHERE name IN (?, ?))`, key, key, key+".system").Scan(&taken); err != nil {
			return "", fmt.Errorf("store: integration key: %w", err)
		}
		if taken == 0 {
			return key, nil
		}
	}
	return "", fmt.Errorf("store: no free integration key for %q", base)
}

// enableUsers enables the users given, those of an integration attached
// again.
func enableUsers(ctx context.Context, tx *sql.Tx, ids ...int64) error {
	for _, id := range ids {
		if id == 0 {
			continue
		}
		if _, err := tx.ExecContext(ctx, `UPDATE users SET disabled = 0, version = version + 1 WHERE id = ? AND disabled = 1`, id); err != nil {
			return fmt.Errorf("store: integration user %d: %w", id, err)
		}
	}
	return nil
}

// RefuseIntegration refuses a pending request.
func (s *Store) RefuseIntegration(ctx context.Context, uuid string, by int64) error {
	if err := s.writable(); err != nil {
		return err
	}
	res, err := s.db.ExecContext(ctx, `UPDATE integrations SET state = ?, updated_at = ?, updated_by = ?, version = version + 1 WHERE uuid = ? AND state = ?`,
		IntegrationRefused, s.nowText(), nullID(by), uuid, IntegrationPending)
	if err != nil {
		return fmt.Errorf("store: refuse integration: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotPending
	}
	return nil
}

// CollectIntegrationKey hands out the key of an approved request, found
// by its uuid and its code: a new API key of the integration's own user.
// It does so again, revoking the key before, as long as that key was
// never used and RequestLife since the approval has not passed, so that
// an answer lost on the way does not spend the approval. A request still
// waiting is ErrRequestPending; one refused or expired, or whose key was
// used, ErrRequestGone; an unknown uuid or a wrong code ErrNoIntegration.
func (s *Store) CollectIntegrationKey(ctx context.Context, uuid, code string) (string, Integration, error) {
	if err := s.writable(); err != nil {
		return "", Integration{}, err
	}
	req, ok, err := s.IntegrationByUUID(ctx, uuid)
	if err != nil {
		return "", Integration{}, err
	}
	if !ok || code == "" || subtle.ConstantTimeCompare([]byte(code), []byte(req.RequestCode)) != 1 {
		return "", Integration{}, ErrNoIntegration
	}
	i := req
	if req.MergedInto != 0 {
		if i, ok, err = s.integrationBy(ctx, s.db, "id", req.MergedInto); err != nil || !ok {
			return "", Integration{}, ErrRequestGone
		}
	}
	now := s.now()
	switch {
	case i.State == IntegrationPending && now.Before(i.RequestExpires):
		return "", Integration{}, ErrRequestPending
	case i.State != IntegrationActive || !now.Before(i.RequestExpires) || subtle.ConstantTimeCompare([]byte(code), []byte(i.RequestCode)) != 1:
		return "", Integration{}, ErrRequestGone
	}
	tokens, err := s.Tokens(ctx, i.UserID)
	if err != nil {
		return "", Integration{}, err
	}
	for _, t := range tokens {
		if t.Kind != TokenAPI || !t.RevokedAt.IsZero() {
			continue
		}
		if !t.LastUsedAt.IsZero() {
			return "", Integration{}, ErrRequestGone
		}
		if err := s.RevokeToken(ctx, t.ID); err != nil && !errors.Is(err, ErrNoToken) {
			return "", Integration{}, err
		}
	}
	_, secret, err := s.AddToken(ctx, i.UserID, TokenAPI, "integration "+i.Name, time.Time{})
	if err != nil {
		return "", Integration{}, err
	}
	if _, err := s.db.ExecContext(ctx, `UPDATE integrations SET key_issued_at = ?, version = version + 1 WHERE id = ?`, formatTime(now), i.ID); err != nil {
		return "", Integration{}, fmt.Errorf("store: integration key: %w", err)
	}
	i.KeyIssuedAt = now
	return secret, i, nil
}

// SetIntegration changes an integration's Name, MaxCaps and State
// (active or disabled) with change.
func (s *Store) SetIntegration(ctx context.Context, uuid string, change func(*Integration) error, by int64) (Integration, error) {
	if err := s.writable(); err != nil {
		return Integration{}, err
	}
	i, ok, err := s.IntegrationByUUID(ctx, uuid)
	if err != nil {
		return Integration{}, err
	}
	if !ok || i.State == IntegrationPending || i.State == IntegrationRefused {
		return Integration{}, fmt.Errorf("%w: %s", ErrNoIntegration, uuid)
	}
	if err := change(&i); err != nil {
		return Integration{}, err
	}
	if i.State != IntegrationActive && i.State != IntegrationDisabled {
		return Integration{}, fmt.Errorf("store: integration state %q: active or disabled", i.State)
	}
	raw, _ := json.Marshal(normalCaps(i.MaxCaps))
	if _, err := s.db.ExecContext(ctx, `UPDATE integrations SET name = ?, max_caps = ?, state = ?, updated_at = ?, updated_by = ?, origin = ?,
		version = version + 1 WHERE id = ?`, strings.TrimSpace(i.Name), string(raw), i.State, s.nowText(), nullID(by), s.nodeID, i.ID); err != nil {
		return Integration{}, fmt.Errorf("store: integration %s: %w", uuid, err)
	}
	out, _, err := s.IntegrationByUUID(ctx, uuid)
	return out, err
}

// NotePendingInstance records an installation id other than the accepted
// one that a call came with, for the owner to accept.
func (s *Store) NotePendingInstance(ctx context.Context, id int64, instance string) error {
	if !ValidInstanceID(instance) {
		return fmt.Errorf("%w: %q", ErrBadInstance, instance)
	}
	_, err := s.db.ExecContext(ctx, `UPDATE integrations SET pending_instance = ? WHERE id = ? AND instance_id <> ?`, instance, id, instance)
	return err
}

// AcceptInstance makes the installation id last seen the accepted one.
func (s *Store) AcceptInstance(ctx context.Context, uuid string, by int64) (Integration, error) {
	if err := s.writable(); err != nil {
		return Integration{}, err
	}
	res, err := s.db.ExecContext(ctx, `UPDATE integrations SET instance_id = pending_instance, pending_instance = '', updated_at = ?, updated_by = ?,
		version = version + 1 WHERE uuid = ? AND pending_instance <> ''`, s.nowText(), nullID(by), uuid)
	if err != nil {
		return Integration{}, fmt.Errorf("store: integration %s: %w", uuid, err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return Integration{}, fmt.Errorf("%w: %s has no installation id to accept", ErrNoIntegration, uuid)
	}
	i, _, err := s.IntegrationByUUID(ctx, uuid)
	return i, err
}

// RevokeIntegration revokes an integration's keys and disables it; its
// users, links and maximum stay, for a request from the same installation
// to be attached to it later.
func (s *Store) RevokeIntegration(ctx context.Context, uuid string, by int64) error {
	i, err := s.SetIntegration(ctx, uuid, func(i *Integration) error { i.State = IntegrationDisabled; return nil }, by)
	if err != nil {
		return err
	}
	tokens, err := s.Tokens(ctx, i.UserID)
	if err != nil {
		return err
	}
	for _, t := range tokens {
		if t.RevokedAt.IsZero() {
			if err := s.RevokeToken(ctx, t.ID); err != nil && !errors.Is(err, ErrNoToken) {
				return err
			}
		}
	}
	return nil
}

// addUserTx adds a user without a password inside tx, for an integration
// and its users: no sign-in, only what acts through the integration.
func (s *Store) addUserTx(ctx context.Context, tx *sql.Tx, u User, by int64) (User, error) {
	u, err := s.newUser(u)
	if err != nil {
		return User{}, err
	}
	role, err := s.roleID(ctx, tx, u.Role)
	if err != nil {
		return User{}, err
	}
	if err := s.nameFree(ctx, tx, u.Name); err != nil {
		return User{}, err
	}
	now := s.nowText()
	uuid := newUUID()
	res, err := tx.ExecContext(ctx, `INSERT INTO users (uuid, name, display_name, role_id, created_at, updated_at, updated_by, origin)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`, uuid, u.Name, u.DisplayName, role, now, now, nullID(by), s.nodeID)
	if err != nil {
		return User{}, fmt.Errorf("store: add user %s: %w", u.Name, err)
	}
	u.ID, _ = res.LastInsertId()
	u.UUID = uuid
	return u, nil
}

// IntegrationIdentity is one of an integration's users: an identity of
// user_identities whose subject is the user's id at the integration.
type IntegrationIdentity struct {
	ID              int64
	UUID            string
	IntegrationID   int64
	Subject         string // the user's id at the integration, or "system"
	Name            string // its name there, from its last call
	UserID          int64  // the user it acts as: OwnUserID, or the Massimo user it is linked to
	OwnUserID       int64  // the user made at its first call
	SystemGenerated bool
	LastLevel       string
	CreatedAt       time.Time
	LastUsedAt      time.Time
}

// Linked reports whether the identity acts as a Massimo user of its own.
func (i IntegrationIdentity) Linked() bool { return i.UserID != i.OwnUserID }

const integrationIdentityColumns = `id, uuid, integration_id, coalesce(subject, ''), name, user_id, coalesce(own_user_id, user_id), system_generated,
	last_level, created_at, last_used_at`

func scanIntegrationIdentity(row scanner) (IntegrationIdentity, error) {
	var i IntegrationIdentity
	var created, used sql.NullString
	if err := row.Scan(&i.ID, &i.UUID, &i.IntegrationID, &i.Subject, &i.Name, &i.UserID, &i.OwnUserID, &i.SystemGenerated,
		&i.LastLevel, &created, &used); err != nil {
		return IntegrationIdentity{}, err
	}
	i.CreatedAt, i.LastUsedAt = parseTime(created), parseTime(used)
	return i, nil
}

// IntegrationSeen is what a call through an integration says of its
// user: its id there, its name, whether it is the integration's own
// system user, and the level of the call.
type IntegrationSeen struct {
	Subject         string
	Name            string
	SystemGenerated bool
	Level           string
}

// IntegrationUser finds the user a call through integration i acts for,
// making it at its first call: an identity with a user of the role
// integration, without a password, named "<key>.<the id's first 8
// characters>" (longer where that name is taken). Its name, level and
// time of use are kept current; a name only where the call brings one.
// The subject "system" is the integration's user for the calls no user
// started.
func (s *Store) IntegrationUser(ctx context.Context, i Integration, seen IntegrationSeen) (IntegrationIdentity, error) {
	if err := s.writable(); err != nil {
		return IntegrationIdentity{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return IntegrationIdentity{}, fmt.Errorf("store: integration user: %w", err)
	}
	defer tx.Rollback()
	now := s.nowText()
	id, err := scanIntegrationIdentity(tx.QueryRowContext(ctx, `SELECT `+integrationIdentityColumns+` FROM user_identities WHERE integration_id = ? AND subject = ?`,
		i.ID, seen.Subject))
	switch {
	case errors.Is(err, sql.ErrNoRows):
		if seen.Subject == "system" {
			return IntegrationIdentity{}, fmt.Errorf("%w: integration %s has no system user", ErrNoIntegration, i.Key)
		}
		own, err := s.integrationUserTx(ctx, tx, i, seen)
		if err != nil {
			return IntegrationIdentity{}, err
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO user_identities (uuid, user_id, integration_id, subject, name, own_user_id, system_generated,
			last_level, created_at, bound_at, last_used_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			newUUID(), own.ID, i.ID, seen.Subject, seen.Name, own.ID, seen.SystemGenerated, seen.Level, now, now, now); err != nil {
			return IntegrationIdentity{}, fmt.Errorf("store: integration user: %w", err)
		}
	case err != nil:
		return IntegrationIdentity{}, fmt.Errorf("store: integration user: %w", err)
	default:
		name := id.Name
		if seen.Name != "" && seen.Name != id.Name {
			name = seen.Name
			if _, err := tx.ExecContext(ctx, `UPDATE users SET display_name = ?, updated_at = ?, version = version + 1 WHERE id = ?`, name, now, id.OwnUserID); err != nil {
				return IntegrationIdentity{}, fmt.Errorf("store: integration user: %w", err)
			}
		}
		if _, err := tx.ExecContext(ctx, `UPDATE user_identities SET name = ?, last_level = ?, system_generated = ?, last_used_at = ? WHERE id = ?`,
			name, seen.Level, seen.SystemGenerated || id.SystemGenerated, now, id.ID); err != nil {
			return IntegrationIdentity{}, fmt.Errorf("store: integration user: %w", err)
		}
	}
	out, err := scanIntegrationIdentity(tx.QueryRowContext(ctx, `SELECT `+integrationIdentityColumns+` FROM user_identities WHERE integration_id = ? AND subject = ?`,
		i.ID, seen.Subject))
	if err != nil {
		return IntegrationIdentity{}, fmt.Errorf("store: integration user: %w", err)
	}
	return out, tx.Commit()
}

// integrationUserTx makes the user of an integration's user's first call,
// its name the integration's key and as much of the id as makes it free.
func (s *Store) integrationUserTx(ctx context.Context, tx *sql.Tx, i Integration, seen IntegrationSeen) (User, error) {
	short := strings.ToLower(seen.Subject)
	for n := 8; ; n += 4 {
		name := i.Key + "." + short[:min(n, len(short))]
		if err := s.nameFree(ctx, tx, name); err == nil {
			display := seen.Name
			if display == "" {
				display = seen.Subject
			}
			return s.addUserTx(ctx, tx, User{Name: name, DisplayName: display, Role: RoleIntegration}, 0)
		} else if !errors.Is(err, ErrUserExists) {
			return User{}, err
		}
		if n >= len(short) {
			return User{}, fmt.Errorf("%w: no free name for %s's user %s", ErrUserExists, i.Key, seen.Subject)
		}
	}
}

// ErrBadLink is a link the store refuses: to a user of an integration,
// or of the system user.
var ErrBadLink = errors.New("store: an integration's user links to a Massimo user of its own")

// IntegrationIdentities lists an integration's users, the system user
// first, then by name.
func (s *Store) IntegrationIdentities(ctx context.Context, integrationID int64) ([]IntegrationIdentity, error) {
	return s.integrationIdentities(ctx, `integration_id = ?`, integrationID)
}

// AllIntegrationIdentities lists every integration's users.
func (s *Store) AllIntegrationIdentities(ctx context.Context) ([]IntegrationIdentity, error) {
	return s.integrationIdentities(ctx, `integration_id IS NOT NULL`)
}

func (s *Store) integrationIdentities(ctx context.Context, where string, args ...any) ([]IntegrationIdentity, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+integrationIdentityColumns+` FROM user_identities WHERE `+where+`
		ORDER BY subject <> 'system', lower(name), id`, args...)
	if err != nil {
		return nil, fmt.Errorf("store: integration users: %w", err)
	}
	defer rows.Close()
	out := []IntegrationIdentity{}
	for rows.Next() {
		i, err := scanIntegrationIdentity(rows)
		if err != nil {
			return nil, fmt.Errorf("store: integration users: %w", err)
		}
		out = append(out, i)
	}
	return out, rows.Err()
}

// LinkIntegrationUser links an integration's user to a Massimo user, its
// calls then acting as that user; userID 0 unlinks it, its calls acting
// as the user made at its first call again. The system user stays its
// own, and a user of an integration is no Massimo user to link to
// (ErrBadLink).
func (s *Store) LinkIntegrationUser(ctx context.Context, integrationID int64, subject string, userID, by int64) (IntegrationIdentity, error) {
	if err := s.writable(); err != nil {
		return IntegrationIdentity{}, err
	}
	id, err := scanIntegrationIdentity(s.db.QueryRowContext(ctx, `SELECT `+integrationIdentityColumns+` FROM user_identities WHERE integration_id = ? AND subject = ?`,
		integrationID, subject))
	if errors.Is(err, sql.ErrNoRows) {
		return IntegrationIdentity{}, fmt.Errorf("%w: user %s", ErrNoIntegration, subject)
	}
	if err != nil {
		return IntegrationIdentity{}, fmt.Errorf("store: integration user: %w", err)
	}
	target := id.OwnUserID
	if userID != 0 {
		if subject == "system" {
			return IntegrationIdentity{}, fmt.Errorf("%w: the system user", ErrBadLink)
		}
		var ofIntegration int
		if err := s.db.QueryRowContext(ctx, `SELECT (SELECT count(*) FROM user_identities WHERE own_user_id = ?) + (SELECT count(*) FROM integrations WHERE user_id = ?)`,
			userID, userID).Scan(&ofIntegration); err != nil {
			return IntegrationIdentity{}, fmt.Errorf("store: integration user: %w", err)
		}
		if ofIntegration > 0 {
			return IntegrationIdentity{}, ErrBadLink
		}
		target = userID
	}
	if _, err := s.db.ExecContext(ctx, `UPDATE user_identities SET user_id = ?, created_by = coalesce(created_by, ?) WHERE id = ?`, target, nullID(by), id.ID); err != nil {
		return IntegrationIdentity{}, fmt.Errorf("store: integration user: %w", err)
	}
	id.UserID = target
	return id, nil
}
