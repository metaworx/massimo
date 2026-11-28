// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

// Login in the web UI (docs/store.md section 6): POST /api/v1/login
// takes a name and a password and answers a session token, which the
// page keeps in localStorage and sends as the bearer header; POST
// /api/v1/login/key makes one from an API key, for the apps' login by
// fingerprint; POST /api/v1/logout revokes it; DELETE /api/v1/me/token
// revokes the token that calls, a session or a key; GET /api/v1/me says
// who the caller is and what the caller may do, so that the page can hide
// what the role may not.

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"maps"
	"net/http"
	"regexp"
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
	Name        string `json:"name"`
	DisplayName string `json:"display_name,omitempty"`
	// Display is how the user is shown: its chain, "<the integration's
	// name> › <its display name>" for an integration's user, with ",
	// as <login name>" where it acts as a Massimo user (names.go).
	Display     string     `json:"display,omitempty"`
	Role        string     `json:"role"`
	Disabled    bool       `json:"disabled"`
	HasPassword bool       `json:"has_password"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
	InvitedBy   string     `json:"invited_by,omitempty"`
	AuthVersion int        `json:"auth_version"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	LastLoginAt *time.Time `json:"last_login_at,omitempty"`
	// May is what the caller may do with the user, in the users routes'
	// answers (users_api.go); absent elsewhere.
	May *userMay `json:"may,omitempty"`
	// Integration marks an integration's user, with the integration's
	// uuid, and in GET /api/v1/users with its id and name there and the
	// user it is linked to; Links lists there the integrations' users
	// linked to this one.
	Integration *userIntegration `json:"integration,omitempty"`
	Links       []userLink       `json:"links,omitempty"`
}

// userIntegration is what makes a user an integration's: the integration,
// and for one of its users the id and name there and the Massimo user it
// is linked to. The integration's own user has no id.
type userIntegration struct {
	Key      string `json:"key"` // undocumented, for older clients: its uuid since migration 0024 dropped the key
	UUID     string `json:"uuid"`
	ID       string `json:"id,omitempty"`
	Name     string `json:"name,omitempty"`
	LinkedTo string `json:"linked_to,omitempty"`
}

// userLink is an integration's user linked to a Massimo user.
type userLink struct {
	Integration string `json:"integration"` // the integration's uuid
	ID          string `json:"id"`
	Name        string `json:"name,omitempty"`
	Display     string `json:"display,omitempty"` // its chain, "<the integration's name> › <its display name>"
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
	// KeyID is the key a session was made from (POST /api/v1/login/key),
	// absent for a session of a sign-in and for a key.
	KeyID int64 `json:"key_id,omitempty"`
	// FromSettings marks, in a user's token list, the key that is the
	// settings file's MASSIMO_API_TOKEN: it is changed or removed in the
	// file, and the API does not revoke it (settingsKey).
	FromSettings bool `json:"from_settings,omitempty"`
	// MadeBy is the API key that made a key, as the bearer of its
	// POST /api/v1/users/{name}/tokens (AP KeyLife, D5-A), absent for a
	// key made by a session, the command line, the bootstrap or an
	// integration's approval, and for a session.
	MadeBy int64 `json:"made_by,omitempty"`
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
	// KeptOpen says the server without regular users is kept open on
	// purpose, the setting auth.keep_open: the web UI does not ask for the
	// first owner (AP FirstOwner, D11). Only on an open server.
	KeptOpen bool `json:"kept_open,omitempty"`
	// Token is the one the caller presented.
	Token *tokenView `json:"token,omitempty"`
	// Integration is the registered integration whose key it is, null
	// for any other key; ActsFor the integration's user a call with
	// X-MassimoHA-On-Behalf-Of acts for (onbehalf.go). Capabilities is
	// then what the chain leaves.
	Integration *meIntegration `json:"integration"`
	ActsFor     *meActsFor     `json:"acts_for,omitempty"`
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
	return tokenView{ID: t.ID, Kind: t.Kind, Label: t.Label, CreatedAt: t.CreatedAt, ExpiresAt: optTime(t.ExpiresAt), LastUsedAt: optTime(t.LastUsedAt), LastUsedFrom: t.LastUsedFrom, KeyID: t.ParentID, MadeBy: t.MadeBy}
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
	// Login is a sign-in by OPAQUE for the name that the browser could
	// not finish (opaque_api.go): the password sign-in takes it over, so
	// that the attempt, counted at the start, counts once, and a user
	// still of version 1 gets a proof to upgrade with.
	Login string `json:"login,omitempty"`
}

// loginMode refuses a sign-in where there are no users to sign in as.
func (s *Server) loginMode(ctx context.Context) error {
	switch s.authMode(ctx) {
	case modeLegacy:
		return refuse(http.StatusConflict, codeLoginNoStore)
	case modeOpen:
		return refuse(http.StatusConflict, codeLoginNoUsers)
	}
	return nil
}

// login handles POST /api/v1/login: a session for a name and a
// password. A wrong pair counts against the address and the name; an
// address signs in once at a time. With login, the sign-in by OPAQUE it
// follows has counted the attempt and holds the address; a user still of
// version 1 is answered a proof, upgrade, for its record.
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
	if !st.PasswordLogin(ctx) {
		return refuse(http.StatusForbidden, codeLoginPasswordOff)
	}
	addrKey, userKey := store.LockoutAddr(ip), store.LockoutUser(name)
	var started *pendingLogin
	if req.Login != "" {
		p, ok := s.logins.take(req.Login, time.Now())
		switch {
		case ok && (p.name != name || p.addrKey != addrKey):
			s.signIns.release(p.addrKey)
			return fail(http.StatusBadRequest, "login was started for another name or from another address")
		case ok:
			// The start's claim of the address, now this request's.
			started = p
			s.signIns.hold(addrKey, time.Time{})
		}
	}
	if started == nil {
		if err := s.lockedOut(w, ctx, addrKey, userKey); err != nil {
			return err
		}
		switch ok, full := s.signIns.claim(addrKey); {
		case full:
			return passwordBusy(store.ErrBusy)
		case !ok:
			return withRetryAfter(refuse(http.StatusTooManyRequests, codeLoginRunning), 1)
		}
	}
	defer s.signIns.release(addrKey)
	if err := st.CheckPassword(ctx, name, req.Password); err != nil {
		if busy := passwordBusy(err); busy != nil {
			return busy
		}
		if !errors.Is(err, store.ErrBadPassword) {
			return fail(http.StatusInternalServerError, "%v", err)
		}
		if started == nil {
			s.limiter.fail(ctx, userKey)
			if s.limiter.fail(ctx, addrKey) {
				s.logLockout(ip, "logins")
			}
		}
		auditAction(r, "login.failed")
		s.logSignIn("login_failed", name, 0, ip)
		return refuse(http.StatusUnauthorized, codeLoginWrong)
	}
	u, ok, err := st.User(ctx, name)
	if err != nil || !ok {
		return fail(http.StatusInternalServerError, "user %s: %v", name, err)
	}
	if started == nil {
		return s.openSession(w, r, u, req.Label, nil, userKey, addrKey)
	}
	var extra map[string]any
	if u.AuthVersion == 1 {
		if proof, _ := s.logins.issueProof(u.ID, time.Now()); proof != "" {
			extra = map[string]any{"upgrade": proof}
		}
	}
	if err := s.openSession(w, r, u, req.Label, extra); err != nil {
		return err
	}
	s.cleared(ctx, started)
	return nil
}

// openSession answers a sign-in whose password matched: a user that may
// sign in gets a session, with the extra fields, and the counters of the
// keys are cleared. An integration's user gets none, by whatever it
// signed in (noWayIn).
func (s *Server) openSession(w http.ResponseWriter, r *http.Request, u store.User, label string, extra map[string]any, keys ...string) error {
	return s.openSessionFrom(w, r, u, 0, label, extra, keys...)
}

// openSessionFrom is openSession for a session made from the API key
// keyID, which then names the key and ends the key's earlier session; 0
// for a sign-in.
func (s *Server) openSessionFrom(w http.ResponseWriter, r *http.Request, u store.User, keyID int64, label string, extra map[string]any, keys ...string) error {
	ctx := r.Context()
	st := s.opts.Store
	ip := s.clientIP(r)
	now := time.Now()
	switch {
	case u.IntegrationID != 0:
		return noWayIn(u)
	case u.Disabled:
		return refuse(http.StatusForbidden, codeUserAccountDisabled, u.Name)
	case u.Expired(now):
		return refuse(http.StatusForbidden, codeUserAccountExpired, u.Name)
	}
	label = strings.TrimSpace(label)
	if label == "" {
		label = r.UserAgent()
	}
	if len(label) > maxLabel {
		label = label[:maxLabel]
	}
	var tok store.Token
	var secret string
	var err error
	if keyID != 0 {
		tok, secret, err = st.AddKeySession(ctx, keyID, label, now.Add(SessionLifetime))
	} else {
		tok, secret, err = st.AddToken(ctx, u.ID, store.TokenSession, label, now.Add(SessionLifetime))
	}
	if errors.Is(err, store.ErrNoToken) {
		return refuse(http.StatusUnauthorized, codeTokenWrong) // the key was revoked meanwhile: errBadToken's words
	}
	if err != nil {
		return fail(http.StatusInternalServerError, "%v", err)
	}
	if err := st.TouchToken(ctx, tok.ID, ip, time.Time{}, ""); err != nil {
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
	s.logSignIn("login", u.Name, u.ID, ip)
	me, err := s.meOf(r, &principal{mode: modeUsers, user: u, token: tok})
	if err != nil {
		return err
	}
	answer := map[string]any{"token": secret, "expires_at": tok.ExpiresAt, "user": me.User, "role": me.Role, "capabilities": me.Capabilities}
	maps.Copy(answer, extra)
	writeJSON(w, http.StatusOK, answer)
	return nil
}

// loginKeyRequest is the optional body of POST /api/v1/login/key.
type loginKeyRequest struct {
	// Label names the session as Label of loginRequest does.
	Label string `json:"label,omitempty"`
}

// loginKey handles POST /api/v1/login/key: a session from the API key
// that makes the call, for the apps' login by fingerprint (docs/app.md),
// answered as POST /api/v1/login answers. The session names the key, and
// the key's earlier session ends, so that a phone holds one per key.
// The wrapper has refused a wrong key, 401 and counted against the
// address as every failed token check is, a revoked one, 401 and not
// counted, as no token that ended is (D64-A), and a user or role that
// may not use the API, 403. A session as the bearer is 400, a
// registered integration's key 403. It sends no password, so it works
// where the password sign-in is off.
func (s *Server) loginKey(w http.ResponseWriter, r *http.Request) error {
	if err := s.loginMode(r.Context()); err != nil {
		return err
	}
	p := principalOf(r)
	if !p.named() {
		return refuse(http.StatusConflict, codeLoginNoUsers)
	}
	auditTarget(r, "user:"+p.user.Name)
	if p.token.Kind != store.TokenAPI {
		return fail(http.StatusBadRequest, "the bearer is a session, not an API key: a session is made from a key")
	}
	if p.integration != nil {
		return refuse(http.StatusForbidden, codeLoginIntegrationKey)
	}
	var req loginKeyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
		return fail(http.StatusBadRequest, "bad body: %v", err)
	}
	return s.openSessionFrom(w, r, p.user, p.token.ID, req.Label, nil)
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
	if _, err := s.opts.Store.RevokeToken(r.Context(), p.token.ID); err != nil {
		return fail(http.StatusInternalServerError, "%v", err)
	}
	s.log.Info("api: logout", "user", p.user.Name, "remote", s.clientIP(r))
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	return nil
}

// settingsKey is the live API key that is the settings file's
// MASSIMO_API_TOKEN, with its user's id, ok false where the file has none
// or the store holds no live key of it. The store keeps each token's
// hash, unique, so the hash of the token the server holds from the file
// (legacyToken, which SetToken follows) finds that one row; its label is
// no mark, any key may carry it. The API ends that key neither by a
// revoke (AP MobileApp v1.4, D86-A) nor with its user's removal (D89-A):
// the next start would not add it back, its hash being taken
// (store.Bootstrap), so it is changed or removed in the file.
func (s *Server) settingsKey(ctx context.Context) (store.Token, bool, error) {
	file := s.legacyToken()
	if file == "" || s.opts.Store == nil {
		return store.Token{}, false, nil
	}
	tok, ok, err := s.opts.Store.TokenByHash(ctx, store.HashToken(file))
	if err != nil || !ok || tok.Kind != store.TokenAPI {
		return store.Token{}, false, err
	}
	return tok, true, nil
}

// deleteMyToken handles DELETE /api/v1/me/token: the token that makes the
// call is revoked, a session or an API key, a key's sessions with it (AP
// MobileApp, D80-B) and its heirs, which the answer names as
// deleteToken's does (AP KeyLife, D5-A), so that a client revokes what
// it holds without its user's name or the token's id, as the apps' login
// by fingerprint does with the key alone. A caller without a token of a
// user (an open server, the one token of a server without users) is 400,
// and so is a call an integration's key makes on a user's behalf, which
// would revoke the integration's own key in that user's name. The settings file's
// token (MASSIMO_API_TOKEN) is 409: the next start would not bring the
// owner's key back, its hash being taken, so it is changed or removed in
// the settings file.
func (s *Server) deleteMyToken(w http.ResponseWriter, r *http.Request) error {
	p := principalOf(r)
	if !p.named() || p.token.ID == 0 {
		return fail(http.StatusBadRequest, "no token of a user to revoke")
	}
	if p.actsFor != nil {
		return fail(http.StatusBadRequest, "a call on a user's behalf revokes nothing: revoke the integration's key without the %s header", OnBehalfHeader)
	}
	if file := s.legacyToken(); file != "" && tokenMatches(file, strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")) {
		return refuse(http.StatusConflict, codeTokenSettingsKey)
	}
	auditTarget(r, "token:"+strconv.FormatInt(p.token.ID, 10))
	auditSubject(r, tokenSubject(p.token, p.user))
	heirs, err := s.opts.Store.RevokeToken(r.Context(), p.token.ID)
	if err != nil {
		return userFail(err)
	}
	s.revoked(w, r, p.token, p.user, heirs, "user", s.logUserName(r.Context(), p.user), "token", p.token.ID, "kind", p.token.Kind, "by", "itself", "remote", s.clientIP(r))
	return nil
}

// revokeCodeRequest is the body of POST /api/v1/tokens/revoke.
type revokeCodeRequest struct {
	Code string `json:"code"`
}

// revokeCodeForm is a revoke code as store.RevokeCode makes it: 64
// lower-case hex digits.
var revokeCodeForm = regexp.MustCompile(`^[0-9a-f]{64}$`)

// revokeByCode handles POST /api/v1/tokens/revoke, without a token: the
// live token whose revoke code the body names is revoked, a key's
// sessions and heirs with it (AP KeyLife, D5-A), for a client that forgot
// the token without its server answering (AP MobileApp v1.6, D112-A;
// docs/app.md). It answers 204 whether the code matched or not, so that
// it tells nobody which codes are live; a malformed body is 400 with
// revoke_code_malformed; a locked-out address 429. The settings file's
// key is left live, as no revoke ends it (settingsKey): the store knows
// it by the hash its bootstrap last applied, which SetToken follows
// (store.RevokeByCode). A code never counts toward the lockout,
// matched or not: one that matches nothing is a code taken already, a
// token that ended by itself or one the server has not seen since its
// update, and an app's codes would lock its own address out. A match is
// a row of the audit, a change line and an Info line, which name the
// heirs where it took any, as a revoke's do (revoked); anything else is
// counted per address and minute as a refused request is (quietAudit),
// since anyone may call it. No row nor line carries the code. A server
// without a store keeps no token to revoke: 204 at once.
func (s *Server) revokeByCode(w http.ResponseWriter, r *http.Request) error {
	st := s.opts.Store
	if st == nil {
		w.WriteHeader(http.StatusNoContent)
		return nil
	}
	ctx := r.Context()
	ip := s.clientIP(r)
	if err := s.lockedOut(w, ctx, store.LockoutAddr(ip), ""); err != nil {
		s.quietAudit(r, ip, http.StatusTooManyRequests)
		return err
	}
	var req revokeCodeRequest
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil || !revokeCodeForm.MatchString(req.Code) {
		s.quietAudit(r, ip, http.StatusBadRequest)
		return refuse(http.StatusBadRequest, codeRevokeCodeMalformed)
	}
	tok, heirs, ok, err := st.RevokeByCode(ctx, req.Code)
	if err != nil {
		return userFail(err)
	}
	if !ok {
		s.quietAudit(r, ip, http.StatusNoContent)
		w.WriteHeader(http.StatusNoContent)
		return nil
	}
	// The token is revoked: its user is read for the row and the line
	// even where the client went away meanwhile.
	u, _, err := st.UserByID(context.WithoutCancel(ctx), tok.UserID)
	if err != nil {
		s.log.Warn("api: user of a token revoked by code not read", "token", tok.ID, "error", err)
	}
	auditTarget(r, "token:"+strconv.FormatInt(tok.ID, 10))
	subject := tokenSubject(tok, u)
	line := []any{"user", s.logUserName(ctx, u), "token", tok.ID, "kind", tok.Kind, "by", "code", "remote", ip}
	if len(heirs) > 0 {
		line = append(line, "heirs", heirs)
		subject["heirs"] = heirs
	}
	auditSubject(r, subject)
	s.log.Info("token revoked", line...)
	w.WriteHeader(http.StatusNoContent)
	return nil
}

// quietAudit takes a request out of the audit's rows of their own and
// counts it per address and minute as a refused request is
// (refusalAudit), in rows apart from the other refused requests, with no
// change line: a revoke by code that matched no token that may be
// revoked, or was refused.
func (s *Server) quietAudit(r *http.Request, ip string, status int) {
	a := auditOf(r)
	if a == nil {
		return
	}
	a.skip = true
	s.codeRefusals.note(r, a.pattern, a.action, ip, status)
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
		// An open server has a store (authMode).
		me.KeptOpen = p != nil && p.mode == modeOpen && s.opts.Store.KeepOpen(r.Context())
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
	caps, off, _, err := st.Effective(ctx, p.user.Role, capabilities)
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
	u.Display, u.Integration = userDisplay(ctx, st.Namer(), p.user)
	tok := viewToken(p.token)
	// The role's set with what it includes, as every check takes it
	// (principal.has).
	me.User, me.Role, me.Capabilities, me.Token = &u, &roleRef{Key: role.Key, Name: role.Name, Disabled: role.Disabled}, heldCaps(setOf(caps), setOf(off)), &tok
	if p.integration != nil || p.user.IntegrationID != 0 {
		// What the chain leaves, or for an integration's user's own key
		// the role integration and the maximum (principalFor), with what
		// that includes.
		me.Capabilities = heldCaps(p.caps, p.off)
	}
	if i := p.integration; i != nil {
		me.Integration = &meIntegration{UUID: i.UUID, Key: i.UUID, Name: store.IntegrationName(*i), ReportedName: i.Name, State: i.State, InstanceID: i.InstanceID}
		if f := p.actsFor; f != nil {
			me.ActsFor = &meActsFor{ID: f.Subject, Name: f.Name, Level: f.LastLevel, Linked: f.Linked(), SystemGenerated: f.SystemGenerated}
		}
	}
	return me, nil
}

// meIntegration is a key's integration in GET /api/v1/me.
type meIntegration struct {
	UUID string `json:"uuid"`
	Key  string `json:"key"` // undocumented, for older clients: its uuid since migration 0024 dropped the key
	// Name is the integration's name as the owner gives it, its own
	// user's display name; ReportedName the one it reports.
	Name         string `json:"name"`
	ReportedName string `json:"reported_name"`
	State        string `json:"state"`
	InstanceID   string `json:"instance_id"`
}

// meActsFor is the integration's user a call acts for in GET /api/v1/me.
type meActsFor struct {
	ID              string `json:"id"`
	Name            string `json:"name,omitempty"`
	Level           string `json:"level"`
	Linked          bool   `json:"linked"`
	SystemGenerated bool   `json:"system_generated,omitempty"`
}
