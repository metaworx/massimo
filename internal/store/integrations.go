// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package store

// Integrations (docs/store.md section 6, "Integrations"; migration 0021):
// programs such as Home Assistant that act for users of their own. An
// integration asks to be registered without a key (AddIntegrationRequest);
// the owner compares the short code both sides show and approves it
// (ApproveIntegration), which makes its own user <uuid>:, holding its
// key, and its user <uuid>:system, and the integration collects its key
// with the code (CollectIntegrationKey). Each of its users names it in
// users.integration_id (migration 0024). A request from an installation that was
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
	ID   int64
	UUID string
	Kind string
	Name string // as reported
	// DisplayName is the integration's name as the owner gives it: its
	// own user's display name, which follows Name while it is the name
	// reported before (copyReportedName).
	DisplayName     string
	InstanceID      string
	PendingInstance string
	URL             string
	AppVersion      string
	MaxCaps         []string
	UserID          int64 // its own user, <uuid>:, holding its key
	SystemUserID    int64 // its user <uuid>:system
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

// integrationColumns are the columns scanIntegration reads, of
// integrations g: its own user and its system user found by their names
// (integrationUserName), and its own user's display name, first: the
// content-column rule (rule_test.go) reads a query from its first WHERE.
const integrationColumns = `g.id, g.uuid, g.kind, g.name, g.instance_id, g.pending_instance, g.url, g.app_version, g.max_caps,
	coalesce((SELECT u.display_name FROM users u WHERE u.integration_id = g.id AND u.name = g.uuid || ':' AND u.deleted_at IS NULL), ''),
	coalesce((SELECT u.id FROM users u WHERE u.integration_id = g.id AND u.name = g.uuid || ':' AND u.deleted_at IS NULL), 0),
	coalesce((SELECT u.id FROM users u WHERE u.integration_id = g.id AND u.name = g.uuid || ':system' AND u.deleted_at IS NULL), 0),
	g.state, g.request_code, g.request_expires_at, g.request_remote, g.key_issued_at,
	coalesce(g.merged_into, 0), g.created_at, g.updated_at, g.version`

// integrationName matches the name of an integration's user:
// <integrations.uuid>:, its own, <uuid>:system, and <uuid>:<subject>,
// the subject its user's id there, lower case (docs/store.md section 6,
// "Integrations"). Only the store names them so: userName, the rule of
// every other name, has no ':'.
var integrationName = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}:([a-z0-9]{1,64}|system)?$`)

// integrationUserName is the name of integration uuid's user of subject:
// "" its own, "system" its system user.
func integrationUserName(uuid, subject string) string { return uuid + ":" + subject }

func scanIntegration(row scanner) (Integration, error) {
	var i Integration
	var caps string
	var expires, issued, created, updated sql.NullString
	if err := row.Scan(&i.ID, &i.UUID, &i.Kind, &i.Name, &i.InstanceID, &i.PendingInstance, &i.URL, &i.AppVersion, &caps, &i.DisplayName,
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
	if err := s.db.QueryRowContext(ctx, `SELECT count(*) FROM integrations WHERE state = ? AND request_expires_at > ? AND deleted_at IS NULL`,
		IntegrationPending, formatTime(now)).Scan(&waiting); err != nil {
		return Integration{}, fmt.Errorf("store: integration request: %w", err)
	}
	if waiting >= MaxPendingRequests {
		return Integration{}, ErrTooManyRequests
	}
	uuid := newUUID()
	if _, err := s.db.ExecContext(ctx, `INSERT INTO integrations (uuid, kind, name, instance_id, url, app_version, state, request_code, request_expires_at,
		request_remote, created_at, updated_at, origin) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		uuid, r.Kind, strings.TrimSpace(r.Name), r.InstanceID, r.URL, r.AppVersion, IntegrationPending, code,
		formatTime(now.Add(RequestLife)), r.Remote, formatTime(now), formatTime(now), s.nodeID); err != nil {
		return Integration{}, fmt.Errorf("store: integration request: %w", err)
	}
	i, _, err := s.IntegrationByUUID(ctx, uuid)
	return i, err
}

// Integrations lists the integrations and the requests still waiting,
// by name; refused requests and expired ones are left out.
func (s *Store) Integrations(ctx context.Context) ([]Integration, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+integrationColumns+` FROM integrations g WHERE g.deleted_at IS NULL
		AND (g.state IN (?, ?) OR g.state = ? AND g.request_expires_at > ?) ORDER BY g.name, g.id`,
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

// IntegrationByID returns an integration by its row, whatever its state.
func (s *Store) IntegrationByID(ctx context.Context, id int64) (Integration, bool, error) {
	return s.integrationBy(ctx, s.db, "id", id)
}

// IntegrationOfUser returns the integration whose own user userID is,
// the holder of its key: its user <uuid>:.
func (s *Store) IntegrationOfUser(ctx context.Context, userID int64) (Integration, bool, error) {
	i, err := scanIntegration(s.db.QueryRowContext(ctx, `SELECT `+integrationColumns+` FROM integrations g JOIN users o ON o.integration_id = g.id
		WHERE o.id = ? AND o.name = g.uuid || ':' AND g.deleted_at IS NULL`, userID))
	if errors.Is(err, sql.ErrNoRows) {
		return Integration{}, false, nil
	}
	if err != nil {
		return Integration{}, false, fmt.Errorf("store: integration: %w", err)
	}
	return i, true, nil
}

// integrationBy returns the integration whose column col is v; col is
// the caller's own text, never input.
func (s *Store) integrationBy(ctx context.Context, q queryer, col string, v any) (Integration, bool, error) {
	i, err := scanIntegration(q.QueryRowContext(ctx, `SELECT `+integrationColumns+` FROM integrations g WHERE g.`+col+` = ? AND g.deleted_at IS NULL`, v))
	if errors.Is(err, sql.ErrNoRows) {
		return Integration{}, false, nil
	}
	if err != nil {
		return Integration{}, false, fmt.Errorf("store: integration: %w", err)
	}
	return i, true, nil
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
		set := `instance_id = ?, pending_instance = '', url = ?, app_version = ?, state = ?, request_code = ?, request_expires_at = ?, key_issued_at = NULL`
		args := []any{req.InstanceID, req.URL, req.AppVersion, IntegrationActive, req.RequestCode, expires}
		if req.Name != "" && req.Name != target.Name {
			// The name the installation reports now; its own user's
			// display name follows while it is the one reported before.
			set += `, name = ?`
			args = append(args, req.Name)
			if err := copyReportedName(ctx, tx, target.UserID, target.Name, req.Name, now); err != nil {
				return Integration{}, err
			}
		}
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
		if _, err := s.addUserTx(ctx, tx, User{Name: integrationUserName(req.UUID, ""), DisplayName: req.Name, Role: RoleIntegration, IntegrationID: req.ID}, by); err != nil {
			return Integration{}, fmt.Errorf("store: approve integration: %w", err)
		}
		sys, err := s.addUserTx(ctx, tx, User{Name: integrationUserName(req.UUID, "system"), DisplayName: "system", Role: RoleIntegration, IntegrationID: req.ID}, by)
		if err != nil {
			return Integration{}, fmt.Errorf("store: approve integration: %w", err)
		}
		if caps == nil {
			caps = []string{}
		}
		raw, _ := json.Marshal(normalCaps(caps))
		if _, err := tx.ExecContext(ctx, `UPDATE integrations SET state = ?, max_caps = ?, request_expires_at = ?,
			updated_at = ?, updated_by = ?, origin = ?, version = version + 1 WHERE id = ?`,
			IntegrationActive, string(raw), expires, formatTime(now), nullID(by), s.nodeID, req.ID); err != nil {
			return Integration{}, fmt.Errorf("store: approve integration: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO user_identities (uuid, effective_user_id, integration_id, subject, name, real_user_id, created_at, created_by,
			bound_at, updated_at, origin) VALUES (?, ?, ?, 'system', 'system', ?, ?, ?, ?, ?, ?)`,
			newUUID(), sys.ID, req.ID, sys.ID, formatTime(now), nullID(by), formatTime(now), formatTime(now), s.nodeID); err != nil {
			return Integration{}, fmt.Errorf("store: approve integration: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return Integration{}, fmt.Errorf("store: approve integration: %w", err)
	}
	i, _, err := s.IntegrationByUUID(ctx, result)
	return i, err
}

// copyReportedName copies a name an integration reports anew into the
// display name of its user userID, as long as that display name is still
// the name reported before (was): a name the owner gave stays.
func copyReportedName(ctx context.Context, tx *sql.Tx, userID int64, was, name string, now time.Time) error {
	if userID == 0 || name == "" || name == was {
		return nil
	}
	// Compared here, not in a WHERE: display_name is a content column
	// (rule_test.go).
	var display string
	if err := tx.QueryRowContext(ctx, `SELECT display_name FROM users WHERE id = ?`, userID).Scan(&display); err != nil {
		return fmt.Errorf("store: integration user %d: %w", userID, err)
	}
	if display != was {
		return nil
	}
	if _, err := tx.ExecContext(ctx, `UPDATE users SET display_name = ?, updated_at = ?, version = version + 1 WHERE id = ?`,
		name, formatTime(now), userID); err != nil {
		return fmt.Errorf("store: integration user %d: %w", userID, err)
	}
	return nil
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
		if t.Kind != TokenAPI || !t.DeletedAt.IsZero() {
			continue
		}
		if !t.LastUsedAt.IsZero() {
			return "", Integration{}, ErrRequestGone
		}
		if _, err := s.RevokeToken(ctx, t.ID); err != nil && !errors.Is(err, ErrNoToken) {
			return "", Integration{}, err
		}
	}
	_, secret, err := s.addToken(ctx, i.UserID, TokenAPI, "integration "+i.Name, time.Time{}, true)
	if err != nil {
		return "", Integration{}, err
	}
	if _, err := s.db.ExecContext(ctx, `UPDATE integrations SET key_issued_at = ?, version = version + 1 WHERE id = ?`, formatTime(now), i.ID); err != nil {
		return "", Integration{}, fmt.Errorf("store: integration key: %w", err)
	}
	i.KeyIssuedAt = now
	return secret, i, nil
}

// SetIntegration changes an integration's DisplayName (its own user's
// display name), MaxCaps and State (active or disabled) with change; Name
// is the one the integration reports, which change does not set.
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
	was := i
	if err := change(&i); err != nil {
		return Integration{}, err
	}
	if i.State != IntegrationActive && i.State != IntegrationDisabled {
		return Integration{}, fmt.Errorf("store: integration state %q: active or disabled", i.State)
	}
	display := strings.TrimSpace(i.DisplayName)
	if display != was.DisplayName && (display == "" || i.UserID == 0) {
		return Integration{}, fmt.Errorf("%w: integration %s: a name of its own user", ErrBadName, uuid)
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Integration{}, fmt.Errorf("store: integration %s: %w", uuid, err)
	}
	defer tx.Rollback()
	now := s.nowText()
	raw, _ := json.Marshal(normalCaps(i.MaxCaps))
	if _, err := tx.ExecContext(ctx, `UPDATE integrations SET max_caps = ?, state = ?, updated_at = ?, updated_by = ?, origin = ?,
		version = version + 1 WHERE id = ?`, string(raw), i.State, now, nullID(by), s.nodeID, i.ID); err != nil {
		return Integration{}, fmt.Errorf("store: integration %s: %w", uuid, err)
	}
	if display != was.DisplayName {
		if _, err := tx.ExecContext(ctx, `UPDATE users SET display_name = ?, updated_at = ?, updated_by = ?, origin = ?, version = version + 1 WHERE id = ?`,
			display, now, nullID(by), s.nodeID, i.UserID); err != nil {
			return Integration{}, fmt.Errorf("store: integration %s: %w", uuid, err)
		}
	}
	if err := tx.Commit(); err != nil {
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
		if t.DeletedAt.IsZero() {
			if _, err := s.RevokeToken(ctx, t.ID); err != nil && !errors.Is(err, ErrNoToken) {
				return err
			}
		}
	}
	return nil
}

// addUserTx adds a user without a password inside tx, for an integration
// and its users: no sign-in, only what acts through the integration. Its
// name is integrationName's, and u.IntegrationID its integration. It is
// not a regular user (regularUser), so the setting KeepOpenKey stays as
// it is (AP LooseEnds, D1).
func (s *Store) addUserTx(ctx context.Context, tx *sql.Tx, u User, by int64) (User, error) {
	if err := s.writable(); err != nil {
		return User{}, err
	}
	u.Name = NormalName(u.Name)
	if !integrationName.MatchString(u.Name) || u.IntegrationID == 0 {
		return User{}, fmt.Errorf("%w: %q", ErrBadName, u.Name)
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
	res, err := tx.ExecContext(ctx, `INSERT INTO users (uuid, name, display_name, role_id, integration_id, created_at, updated_at, updated_by, origin)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`, uuid, u.Name, u.DisplayName, role, u.IntegrationID, now, now, nullID(by), s.nodeID)
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
	EffectiveUserID int64  // the user it acts as: RealUserID, or the Massimo user it is linked to
	RealUserID      int64  // the user made at its first call, <uuid>:<subject>
	SystemGenerated bool
	LastLevel       string
	CreatedAt       time.Time
	LastUsedAt      time.Time
}

// Linked reports whether the identity acts as a Massimo user of its own.
func (i IntegrationIdentity) Linked() bool { return i.EffectiveUserID != i.RealUserID }

const integrationIdentityColumns = `id, uuid, integration_id, coalesce(subject, ''), name, effective_user_id, coalesce(real_user_id, effective_user_id),
	system_generated, last_level, created_at, last_used_at`

// identityUserRemoved is the condition, in a query of user_identities,
// of an identity whose own user or the user it acts as is removed. The
// removal of either removes the identity with it (DeleteUser). An older
// store removed it only with the user it acts as, so that an integration's
// user removed while linked left its identity acting as the linked user.
// One whose linked user was removed only a link racing the removal of the
// user it links to could leave, which LinkIntegrationUser no longer lets
// through; the condition covers it as a guard.
const identityUserRemoved = `EXISTS (SELECT 1 FROM users u WHERE u.id IN (user_identities.effective_user_id, user_identities.real_user_id)
	AND u.deleted_at IS NOT NULL)`

// liveIntegrationIdentity is the condition of an integration's identity
// that acts for someone: not removed, and neither of its users removed
// (identityUserRemoved). One that is not is no one's: no list shows it,
// no link takes it, no stream goes on for it, and the next call of its
// user removes it and makes it anew (IntegrationUser; AP LinkedRemoval,
// D1-A).
const liveIntegrationIdentity = `deleted_at IS NULL AND NOT ` + identityUserRemoved

func scanIntegrationIdentity(row scanner) (IntegrationIdentity, error) {
	var i IntegrationIdentity
	var created, used sql.NullString
	if err := row.Scan(&i.ID, &i.UUID, &i.IntegrationID, &i.Subject, &i.Name, &i.EffectiveUserID, &i.RealUserID, &i.SystemGenerated,
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
// integration, without a password, named <uuid>:<subject>; a subject
// that makes no such name (integrationName), or none, is ErrBadName. Its
// name, level and time of use are kept current, a name only where the
// call brings one, and the display name of its user follows the name
// while that is still the one reported before (copyReportedName). The
// subject "system" is the integration's user for the calls no user
// started.
//
// An identity whose own user was removed before the store removed the
// identity with it, as an older store holds one, is removed here, and the
// call made as a first call (AP LinkedRemoval, D1-A); so is one whose
// linked user was removed, which only a link racing that removal could
// leave, as a guard. A first call takes the integration's user of its
// name again where that user is left without an identity, as after the
// removal of the Massimo user it was linked to, with its role and state
// (D2-A).
func (s *Store) IntegrationUser(ctx context.Context, i Integration, seen IntegrationSeen) (IntegrationIdentity, error) {
	if err := s.writable(); err != nil {
		return IntegrationIdentity{}, err
	}
	if seen.Subject == "" {
		// <uuid>: is the integration's own user, which holds its key.
		return IntegrationIdentity{}, fmt.Errorf("%w: an integration's user without an id", ErrBadName)
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return IntegrationIdentity{}, fmt.Errorf("store: integration user: %w", err)
	}
	defer tx.Rollback()
	now := s.nowText()
	res, err := tx.ExecContext(ctx, `UPDATE user_identities SET deleted_at = ?, updated_at = ?, origin = ?, version = version + 1
		WHERE integration_id = ? AND subject = ? AND deleted_at IS NULL AND `+identityUserRemoved, now, now, s.nodeID, i.ID, seen.Subject)
	if err != nil {
		return IntegrationIdentity{}, fmt.Errorf("store: integration user: %w", err)
	}
	orphan, _ := res.RowsAffected()
	// Every identity of the subject that is not removed is live now: the
	// UPDATE above removed the others in this transaction, so deleted_at
	// alone says it, here and in the read at the end.
	id, err := scanIntegrationIdentity(tx.QueryRowContext(ctx, `SELECT `+integrationIdentityColumns+` FROM user_identities
		WHERE integration_id = ? AND subject = ? AND deleted_at IS NULL`, i.ID, seen.Subject))
	switch {
	case errors.Is(err, sql.ErrNoRows):
		if seen.Subject == "system" {
			return IntegrationIdentity{}, fmt.Errorf("%w: integration %s has no system user", ErrNoIntegration, i.UUID)
		}
		own, err := s.integrationUserTx(ctx, tx, i, seen)
		if err != nil {
			return IntegrationIdentity{}, err
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO user_identities (uuid, effective_user_id, integration_id, subject, name, real_user_id, system_generated,
			last_level, created_at, bound_at, last_used_at, updated_at, origin) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			newUUID(), own, i.ID, seen.Subject, seen.Name, own, seen.SystemGenerated, seen.Level, now, now, now, now, s.nodeID); err != nil {
			return IntegrationIdentity{}, fmt.Errorf("store: integration user: %w", err)
		}
	case err != nil:
		return IntegrationIdentity{}, fmt.Errorf("store: integration user: %w", err)
	default:
		name := id.Name
		if seen.Name != "" && seen.Name != id.Name {
			name = seen.Name
			if err := copyReportedName(ctx, tx, id.RealUserID, id.Name, name, s.now()); err != nil {
				return IntegrationIdentity{}, err
			}
		}
		if _, err := tx.ExecContext(ctx, `UPDATE user_identities SET name = ?, last_level = ?, system_generated = ?, last_used_at = ? WHERE id = ?`,
			name, seen.Level, seen.SystemGenerated || id.SystemGenerated, now, id.ID); err != nil {
			return IntegrationIdentity{}, fmt.Errorf("store: integration user: %w", err)
		}
	}
	out, err := scanIntegrationIdentity(tx.QueryRowContext(ctx, `SELECT `+integrationIdentityColumns+` FROM user_identities
		WHERE integration_id = ? AND subject = ? AND deleted_at IS NULL`, i.ID, seen.Subject))
	if err != nil {
		return IntegrationIdentity{}, fmt.Errorf("store: integration user: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return IntegrationIdentity{}, fmt.Errorf("store: integration user: %w", err)
	}
	if orphan > 0 {
		s.log.Info("store: an integration's user whose own or linked user was removed acted for nobody; its call took it as a first call",
			"integration", i.UUID, "user", seen.Subject)
	}
	return out, nil
}

// integrationUserTx is the id of the user of an integration's user's
// first call, named <uuid>:<subject>, the subject lower case: the user of
// that name of integration i that is not removed and that no identity has
// as its own, as the removal of the Massimo user it was linked to leaves
// it, taken again as the owner left it (AP LinkedRemoval, D2-A), else one
// made with the role integration. A user of that name of no integration
// or of another, one another identity has, or the integration's own user
// or its system user (by id, whatever the case of the subject), is never
// taken: the name is held, ErrUserExists.
func (s *Store) integrationUserTx(ctx context.Context, tx *sql.Tx, i Integration, seen IntegrationSeen) (int64, error) {
	name := integrationUserName(i.UUID, strings.ToLower(seen.Subject))
	var left int64
	err := tx.QueryRowContext(ctx, `SELECT id FROM users WHERE name = ? AND integration_id = ? AND deleted_at IS NULL AND id NOT IN (?, ?)
		AND id NOT IN (SELECT coalesce(real_user_id, effective_user_id) FROM user_identities WHERE integration_id IS NOT NULL AND deleted_at IS NULL)`,
		name, i.ID, i.UserID, i.SystemUserID).Scan(&left)
	switch {
	case err == nil:
		return left, nil
	case !errors.Is(err, sql.ErrNoRows):
		return 0, fmt.Errorf("store: integration user %s: %w", name, err)
	}
	display := seen.Name
	if display == "" {
		display = seen.Subject
	}
	made, err := s.addUserTx(ctx, tx, User{Name: name, DisplayName: display, Role: RoleIntegration, IntegrationID: i.ID}, 0)
	return made.ID, err
}

// ErrBadLink is a link the store refuses: to a user of an integration,
// or of the system user.
var ErrBadLink = errors.New("store: an integration's user links to a Massimo user of its own")

// IntegrationIdentities lists an integration's users, the system user
// first, then by name; one whose own or linked user was removed is none
// (liveIntegrationIdentity).
func (s *Store) IntegrationIdentities(ctx context.Context, integrationID int64) ([]IntegrationIdentity, error) {
	return s.integrationIdentities(ctx, `integration_id = ? AND `+liveIntegrationIdentity, integrationID)
}

// AllIntegrationIdentities lists every integration's users, as
// IntegrationIdentities does.
func (s *Store) AllIntegrationIdentities(ctx context.Context) ([]IntegrationIdentity, error) {
	return s.integrationIdentities(ctx, `integration_id IS NOT NULL AND `+liveIntegrationIdentity)
}

// IntegrationIdentityByID returns the integration's identity of row id
// while it acts for someone (liveIntegrationIdentity), and false where it
// does not: removed, or either of its users removed. It reads and writes
// nothing else, for an event stream that asks at each keep-alive whether
// its caller is still let in.
func (s *Store) IntegrationIdentityByID(ctx context.Context, id int64) (IntegrationIdentity, bool, error) {
	out, err := scanIntegrationIdentity(s.db.QueryRowContext(ctx, `SELECT `+integrationIdentityColumns+` FROM user_identities
		WHERE id = ? AND integration_id IS NOT NULL AND `+liveIntegrationIdentity, id))
	if errors.Is(err, sql.ErrNoRows) {
		return IntegrationIdentity{}, false, nil
	}
	if err != nil {
		return IntegrationIdentity{}, false, fmt.Errorf("store: integration user %d: %w", id, err)
	}
	return out, true, nil
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
// (ErrBadLink). One whose own or linked user was removed is no user to
// link (liveIntegrationIdentity): ErrNoIntegration, as one never seen; a
// removed user, or none, is no user to link to: ErrNoUser. Both are read
// in the transaction that writes the link, so that a removal of either
// does not fall between.
func (s *Store) LinkIntegrationUser(ctx context.Context, integrationID int64, subject string, userID, by int64) (IntegrationIdentity, error) {
	if err := s.writable(); err != nil {
		return IntegrationIdentity{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return IntegrationIdentity{}, fmt.Errorf("store: integration user: %w", err)
	}
	defer tx.Rollback()
	id, err := scanIntegrationIdentity(tx.QueryRowContext(ctx, `SELECT `+integrationIdentityColumns+` FROM user_identities
		WHERE integration_id = ? AND subject = ? AND `+liveIntegrationIdentity, integrationID, subject))
	if errors.Is(err, sql.ErrNoRows) {
		return IntegrationIdentity{}, fmt.Errorf("%w: user %s", ErrNoIntegration, subject)
	}
	if err != nil {
		return IntegrationIdentity{}, fmt.Errorf("store: integration user: %w", err)
	}
	target := id.RealUserID
	if userID != 0 {
		if subject == "system" {
			return IntegrationIdentity{}, fmt.Errorf("%w: the system user", ErrBadLink)
		}
		var ofIntegration bool
		err := tx.QueryRowContext(ctx, `SELECT integration_id IS NOT NULL FROM users WHERE id = ? AND deleted_at IS NULL`, userID).Scan(&ofIntegration)
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return IntegrationIdentity{}, fmt.Errorf("%w: %d", ErrNoUser, userID)
		case err != nil:
			return IntegrationIdentity{}, fmt.Errorf("store: integration user: %w", err)
		case ofIntegration:
			return IntegrationIdentity{}, ErrBadLink
		}
		target = userID
	}
	if _, err := tx.ExecContext(ctx, `UPDATE user_identities SET effective_user_id = ?, created_by = coalesce(created_by, ?), updated_at = ?, origin = ?,
		version = version + 1 WHERE id = ?`, target, nullID(by), s.nowText(), s.nodeID, id.ID); err != nil {
		return IntegrationIdentity{}, fmt.Errorf("store: integration user: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return IntegrationIdentity{}, fmt.Errorf("store: integration user: %w", err)
	}
	id.EffectiveUserID = target
	return id, nil
}
