// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

// Login in the web UI (docs/store.md section 6): POST /api/v1/login
// takes a name and a password and answers a session token, which the
// page keeps in localStorage and sends as the bearer header; POST
// /api/v1/logout revokes it; GET /api/v1/me says who the caller is and
// what the caller may do, so that the page can hide what the role may
// not.

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.metaworx.ch/massimo/internal/store"
)

// maxLabel is the longest token label kept.
const maxLabel = 64

// userView is a user as the API shows it.
type userView struct {
	Name        string     `json:"name"`
	DisplayName string     `json:"display_name,omitempty"`
	Role        string     `json:"role"`
	Disabled    bool       `json:"disabled"`
	HasPassword bool       `json:"has_password"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
	InvitedBy   string     `json:"invited_by,omitempty"`
	AuthVersion int        `json:"auth_version"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	LastLoginAt *time.Time `json:"last_login_at,omitempty"`
}

// roleRef is the role of a user as GET /api/v1/me shows it.
type roleRef struct {
	Key      string `json:"key"`
	Name     string `json:"name,omitempty"`
	Disabled bool   `json:"disabled"`
}

// tokenView is a token as the API shows it, without its secret.
type tokenView struct {
	ID           int64      `json:"id"`
	Kind         string     `json:"kind"`
	Label        string     `json:"label,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	ExpiresAt    *time.Time `json:"expires_at,omitempty"`
	LastUsedAt   *time.Time `json:"last_used_at,omitempty"`
	LastUsedFrom string     `json:"last_used_from,omitempty"`
}

// meView is the answer of GET /api/v1/me.
type meView struct {
	// Mode is how the server authenticates: users, open (a store without
	// users), token or none (no store).
	Mode string `json:"mode"`
	// User and Role are the caller's; null outside users mode.
	User *userView `json:"user"`
	Role *roleRef  `json:"role"`
	// Capabilities is what the caller may do: the role's effective set,
	// or every capability outside users mode.
	Capabilities []string `json:"capabilities"`
	// Setup says the first owner can be created from here: the server is
	// open and the caller is on the local network.
	Setup bool `json:"setup,omitempty"`
	// Token is the one the caller presented.
	Token *tokenView `json:"token,omitempty"`
}

func optTime(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	return &t
}

func viewUser(u store.User, invitedBy string) userView {
	return userView{Name: u.Name, DisplayName: u.DisplayName, Role: u.Role, Disabled: u.Disabled, HasPassword: u.HasPassword, ExpiresAt: optTime(u.ExpiresAt),
		InvitedBy: invitedBy, AuthVersion: u.AuthVersion, CreatedAt: u.CreatedAt, UpdatedAt: u.UpdatedAt, LastLoginAt: optTime(u.LastLoginAt)}
}

func viewToken(t store.Token) tokenView {
	return tokenView{ID: t.ID, Kind: t.Kind, Label: t.Label, CreatedAt: t.CreatedAt, ExpiresAt: optTime(t.ExpiresAt), LastUsedAt: optTime(t.LastUsedAt), LastUsedFrom: t.LastUsedFrom}
}

// signIns holds the addresses with a login running, so that an address
// checks one password at a time and the lockout counts each of its
// attempts before the next one starts. A sign-in by OPAQUE holds its
// address from its start to its finish, at most until the start ends
// (hold).
type signIns struct {
	mu sync.Mutex
	// keys are the addresses signing in: the zero time while a request
	// runs, else the end of a sign-in started.
	keys map[string]time.Time
}

// claim marks a key as signing in: false when it already is, and full
// when maxTracked keys are.
func (s *signIns) claim(key string) (ok, full bool) {
	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()
	if until, held := s.keys[key]; held && (until.IsZero() || now.Before(until)) {
		return false, false
	}
	if len(s.keys) >= maxTracked {
		for k, until := range s.keys {
			if !until.IsZero() && !now.Before(until) {
				delete(s.keys, k)
			}
		}
		if len(s.keys) >= maxTracked {
			return false, true
		}
	}
	if s.keys == nil {
		s.keys = map[string]time.Time{}
	}
	s.keys[key] = time.Time{}
	return true, false
}

// hold keeps a claimed key signing in past its request, until a time.
func (s *signIns) hold(key string, until time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.keys[key] = until
}

// release ends a key's sign-in.
func (s *signIns) release(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.keys, key)
}

// loginRequest is the body of POST /api/v1/login.
type loginRequest struct {
	User     string `json:"user"`
	Password string `json:"password"`
	// Label names the session under the user's tokens ("Firefox on the
	// laptop"); the User-Agent, shortened, when absent.
	Label string `json:"label,omitempty"`
}

// loginMode refuses a sign-in where there are no users to sign in as.
func (s *Server) loginMode(ctx context.Context) error {
	switch s.authMode(ctx) {
	case modeLegacy:
		return fail(http.StatusConflict, "this server has no store: use the API token")
	case modeOpen:
		return fail(http.StatusConflict, "no users yet: create the first owner with POST /api/v1/users, or massimo user add")
	}
	return nil
}

// login handles POST /api/v1/login: a session for a name and a
// password. A wrong pair counts against the address and the name; an
// address signs in once at a time.
func (s *Server) login(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	if err := s.loginMode(ctx); err != nil {
		return err
	}
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.User == "" || req.Password == "" {
		return fail(http.StatusBadRequest, "body must have \"user\" and \"password\"")
	}
	st := s.opts.Store
	name := store.NormalName(req.User)
	ip := s.clientIP(r)
	auditTarget(r, "user:"+name)
	addrKey, userKey := store.LockoutAddr(ip), store.LockoutUser(name)
	if d := s.limiter.lockedFor(ctx, userKey); d > 0 {
		w.Header().Set("Retry-After", strconv.Itoa(int(d.Round(time.Second)/time.Second)))
		return fail(http.StatusTooManyRequests, "too many failed logins for this user; try again later")
	}
	switch ok, full := s.signIns.claim(addrKey); {
	case full:
		return passwordBusy(store.ErrBusy)
	case !ok:
		return &httpError{status: http.StatusTooManyRequests, msg: "a sign-in from this address is already running; try again when it is done", retryAfter: 1}
	}
	defer s.signIns.release(addrKey)
	if err := st.CheckPassword(ctx, name, req.Password); err != nil {
		if busy := passwordBusy(err); busy != nil {
			return busy
		}
		if !errors.Is(err, store.ErrBadPassword) {
			return fail(http.StatusInternalServerError, "%v", err)
		}
		s.limiter.fail(ctx, userKey)
		if s.limiter.fail(ctx, addrKey) {
			s.log.Warn("api: address locked out after failed logins", "remote", ip, "for", s.authLockout)
		}
		auditAction(r, "login.failed")
		s.log.Warn("api: login failed", "user", name, "remote", ip)
		return fail(http.StatusUnauthorized, "wrong user or password")
	}
	u, ok, err := st.User(ctx, name)
	if err != nil || !ok {
		return fail(http.StatusInternalServerError, "user %s: %v", name, err)
	}
	return s.openSession(w, r, u, req.Label, userKey, addrKey)
}

// openSession answers a sign-in whose password matched: a user that may
// sign in gets a session, and the counters of the keys are cleared.
func (s *Server) openSession(w http.ResponseWriter, r *http.Request, u store.User, label string, keys ...string) error {
	ctx := r.Context()
	st := s.opts.Store
	ip := s.clientIP(r)
	now := time.Now()
	switch {
	case u.Disabled:
		return fail(http.StatusForbidden, "user %s is disabled", u.Name)
	case u.Expired(now):
		return fail(http.StatusForbidden, "user %s has expired", u.Name)
	}
	label = strings.TrimSpace(label)
	if label == "" {
		label = r.UserAgent()
	}
	if len(label) > maxLabel {
		label = label[:maxLabel]
	}
	tok, secret, err := st.AddToken(ctx, u.ID, store.TokenSession, label, now.Add(SessionLifetime))
	if err != nil {
		return fail(http.StatusInternalServerError, "%v", err)
	}
	if err := st.TouchToken(ctx, tok.ID, ip, time.Time{}); err != nil {
		s.log.Warn("api: session not touched", "error", err)
	}
	if err := st.NoteLogin(ctx, u.ID); err != nil {
		s.log.Warn("api: login not noted", "error", err)
	} else if again, ok, err := st.User(ctx, u.Name); err == nil && ok {
		u = again
	}
	for _, k := range keys {
		s.limiter.succeed(ctx, k)
	}
	s.log.Info("api: login", "user", u.Name, "remote", ip)
	me, err := s.meOf(r, &principal{mode: modeUsers, user: u, token: tok})
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, map[string]any{"token": secret, "expires_at": tok.ExpiresAt, "user": me.User, "role": me.Role, "capabilities": me.Capabilities})
	return nil
}

// logout handles POST /api/v1/logout: the session that made the call
// is revoked. An API key is not a session; it is revoked under the
// user's tokens.
func (s *Server) logout(w http.ResponseWriter, r *http.Request) error {
	p := principalOf(r)
	if !p.named() {
		return fail(http.StatusBadRequest, "no session to end")
	}
	if p.token.Kind != store.TokenSession {
		return fail(http.StatusBadRequest, "the token is an API key, not a session: revoke it under the user's tokens")
	}
	auditTarget(r, "user:"+p.user.Name)
	if err := s.opts.Store.RevokeToken(r.Context(), p.token.ID); err != nil {
		return fail(http.StatusInternalServerError, "%v", err)
	}
	s.log.Info("api: logout", "user", p.user.Name, "remote", s.clientIP(r))
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	return nil
}

// me handles GET /api/v1/me.
func (s *Server) me(w http.ResponseWriter, r *http.Request) error {
	me, err := s.meOf(r, principalOf(r))
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, me)
	return nil
}

// meOf builds the answer of GET /api/v1/me for a principal.
func (s *Server) meOf(r *http.Request, p *principal) (meView, error) {
	me := meView{Mode: s.AuthMode(), Capabilities: append([]string(nil), capabilities...)}
	if !p.named() {
		me.Setup = p != nil && p.mode == modeOpen && localPeer(s.clientIP(r))
		return me, nil
	}
	st := s.opts.Store
	ctx := r.Context()
	role, ok, err := st.Role(ctx, p.user.Role)
	if err != nil {
		return me, fail(http.StatusInternalServerError, "%v", err)
	}
	if !ok {
		return me, fail(http.StatusInternalServerError, "role %s of user %s is gone", p.user.Role, p.user.Name)
	}
	caps, _, err := st.Effective(ctx, p.user.Role, capabilities)
	if err != nil {
		return me, fail(http.StatusInternalServerError, "%v", err)
	}
	invitedBy := ""
	if p.user.InvitedBy != 0 {
		if by, ok, _ := st.UserByID(ctx, p.user.InvitedBy); ok {
			invitedBy = by.Name
		}
	}
	u := viewUser(p.user, invitedBy)
	tok := viewToken(p.token)
	me.User, me.Role, me.Capabilities, me.Token = &u, &roleRef{Key: role.Key, Name: role.Name, Disabled: role.Disabled}, caps, &tok
	return me, nil
}
