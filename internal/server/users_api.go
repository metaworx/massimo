// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

// The users routes (docs/store.md section 6): list, create, change and
// remove users, and the sessions and API keys of each. The route is
// registered with users; a guests holder reaches it for users whose
// role is guest, and a user reaches its own name for its display name,
// its password and its tokens (auth.go, allowed). On an open server,
// POST /api/v1/users from the local network creates the first owner.

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"go.metaworx.ch/massimo/internal/store"
)

// maxDisplayName is the longest display name accepted, in bytes.
const maxDisplayName = 64

// userRequest is the body of POST /api/v1/users and PUT
// /api/v1/users/{name}; absent fields stay as they are.
type userRequest struct {
	Name            string  `json:"name,omitempty"` // POST only
	DisplayName     *string `json:"display_name"`
	Role            *string `json:"role"`
	Disabled        *bool   `json:"disabled"`
	Password        *string `json:"password"`
	CurrentPassword string  `json:"current_password,omitempty"` // a user's own change
	// Record and KSF are a password of version 2 in place of Password:
	// the registration record of opaque_api.go and its key stretching.
	Record *string `json:"record"`
	KSF    string  `json:"ksf,omitempty"`
	// Proof is a proof of the current password from a sign-in by OPAQUE,
	// for one's own change, in place of current_password.
	Proof string `json:"proof,omitempty"`
	// ExpiresAt is a guest's end date; null clears it. Decoded apart,
	// since a null must be told from an absent field.
	ExpiresAt json.RawMessage `json:"expires_at"`
}

func decodeUser(r *http.Request) (userRequest, error) {
	var req userRequest
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		return req, fail(http.StatusBadRequest, "bad body: %v", err)
	}
	if req.DisplayName != nil && len(*req.DisplayName) > maxDisplayName {
		return req, fail(http.StatusBadRequest, "display_name: at most %d bytes", maxDisplayName)
	}
	if req.Record != nil && req.Password != nil {
		return req, fail(http.StatusBadRequest, "password or record, not both")
	}
	return req, nil
}

// record reads the registration record of a request.
func (req userRequest) record() ([]byte, error) {
	b, err := opaqueB64.DecodeString(*req.Record)
	if err != nil {
		return nil, fail(http.StatusBadRequest, "record: not base64url: %v", err)
	}
	return b, nil
}

// expiresAt reads the end date of a request: set, cleared (null), or
// absent.
func (req userRequest) expiresAt() (t time.Time, set bool, err error) {
	if len(req.ExpiresAt) == 0 {
		return time.Time{}, false, nil
	}
	if string(req.ExpiresAt) == "null" {
		return time.Time{}, true, nil
	}
	var s string
	if json.Unmarshal(req.ExpiresAt, &s) != nil {
		return time.Time{}, false, fail(http.StatusBadRequest, "expires_at: want a time such as 2026-12-24T18:00:00Z, or null")
	}
	t, err = time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Time{}, false, fail(http.StatusBadRequest, "expires_at: %v", err)
	}
	return t, true, nil
}

// userFail maps a store error of the users and roles to a status.
func userFail(err error) error {
	var he *httpError
	if errors.As(err, &he) {
		return err
	}
	if busy := passwordBusy(err); busy != nil {
		return busy
	}
	// The store's "store: " says where the error came from, not what is
	// wrong; a client shows the rest as it is.
	text := strings.TrimPrefix(err.Error(), "store: ")
	switch {
	case errors.Is(err, store.ErrNoUser), errors.Is(err, store.ErrNoRole), errors.Is(err, store.ErrNoToken):
		return fail(http.StatusNotFound, "%s", text)
	case errors.Is(err, store.ErrUserExists), errors.Is(err, store.ErrRoleExists), errors.Is(err, store.ErrLastOwner), errors.Is(err, store.ErrRoleInUse):
		return fail(http.StatusConflict, "%s", text)
	case errors.Is(err, store.ErrBadName), errors.Is(err, store.ErrBadRole), errors.Is(err, store.ErrBadMessage):
		return fail(http.StatusBadRequest, "%s", text)
	case errors.Is(err, store.ErrNoOpaque):
		return fail(http.StatusServiceUnavailable, "%s", text)
	case errors.Is(err, store.ErrImmutable):
		return fail(http.StatusForbidden, "%s", text)
	}
	return fail(http.StatusInternalServerError, "%s", text)
}

// busyRetry is the Retry-After, in seconds, of a password check or
// hash refused because the store runs as many as it takes at once.
const busyRetry = 5

// passwordBusy is the answer to a password check or hash that the
// store refused for the load (store.ErrBusy), or that waited until the
// client went away; nil for any other error.
func passwordBusy(err error) error {
	if errors.Is(err, store.ErrBusy) || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return &httpError{status: http.StatusServiceUnavailable, msg: "too many password checks at once; try again in a few seconds", retryAfter: busyRetry}
	}
	return nil
}

// userStore is the store, or the error of a server without one.
func (s *Server) userStore() (*store.Store, error) {
	if s.opts.Store == nil {
		return nil, fail(http.StatusServiceUnavailable, "this server has no store, so no users: it takes the API token")
	}
	return s.opts.Store, nil
}

// access is what a principal may do with users.
type access struct {
	all       bool // users: every user
	guests    bool // guests: users whose role is guest
	passwords bool // passwords: the password of a user it outranks
	self      bool // the user's own name
	// outranks is the rank rule: whether the principal's role has every
	// capability of the role named; nil where no rank applies (an open
	// server, the one token of a server without a store).
	outranks func(role string) bool
}

// accessOf is what principal p may do with the user name ("" for none
// in particular). The rank rule compares the roles' own sets, owner
// holding every capability: an owner outranks every role, nobody else
// outranks owner, and otherwise a role outranks one whose set is part
// of its own. A role the store cannot read is outranked by owner alone.
func (s *Server) accessOf(ctx context.Context, p *principal, name string) access {
	a := access{all: p.has("users"), guests: p.has("guests"), passwords: p.has("passwords"), self: p.named() && strings.EqualFold(name, p.user.Name)}
	if !p.named() || s.opts.Store == nil {
		return a
	}
	st, own := s.opts.Store, p.user.Role
	sets := map[string]map[string]bool{}
	set := func(key string) map[string]bool {
		if m, ok := sets[key]; ok {
			return m
		}
		var m map[string]bool
		if r, ok, err := st.Role(ctx, key); err == nil && ok {
			m = map[string]bool{}
			for _, c := range r.Capabilities {
				m[c] = true
			}
		}
		sets[key] = m
		return m
	}
	a.outranks = func(role string) bool {
		switch {
		case own == store.RoleOwner:
			return true
		case role == store.RoleOwner:
			return false
		}
		mine, theirs := set(own), set(role)
		if mine == nil || theirs == nil {
			return false
		}
		for c := range theirs {
			if !mine[c] {
				return false
			}
		}
		return true
	}
	return a
}

// ranks reports whether the rank rule lets the principal act on a role.
func (a access) ranks(role string) bool {
	return a.outranks == nil || a.outranks(role)
}

// mayView says whether a principal sees a user: with users or
// passwords every user, with guests a guest. The own user is the
// handlers' question.
func (a access) mayView(u store.User) bool {
	return a.all || a.passwords || a.guests && u.Role == store.RoleGuest
}

// mayManage says whether a principal may change a user, its name, role,
// state and keys: with users, or with guests for a guest, a user it
// outranks.
func (a access) mayManage(u store.User) bool {
	return (a.all || a.guests && u.Role == store.RoleGuest) && a.ranks(u.Role)
}

// maySetPassword says whether a principal may set a user's password: as
// it may manage it, or with passwords, a user it outranks.
func (a access) maySetPassword(u store.User) bool {
	return (a.all || a.passwords || a.guests && u.Role == store.RoleGuest) && a.ranks(u.Role)
}

// mayAssign says whether a principal may give a role: any it outranks
// with users, guest alone with guests.
func (a access) mayAssign(role string) bool {
	if role == store.RoleIntegration {
		return false // an integration's and its users' alone (integrations.go)
	}
	return (a.all || a.guests && role == store.RoleGuest) && a.ranks(role)
}

// rankFail is the refusal of a user or role the principal does not
// outrank, or the capability it lacks.
func (a access) rankFail(role, capability string) error {
	switch {
	case !a.ranks(role) && role == store.RoleOwner:
		return fail(http.StatusForbidden, "only an owner changes an owner")
	case !a.ranks(role):
		return fail(http.StatusForbidden, "the role %s has capabilities yours lacks", role)
	}
	return fail(http.StatusForbidden, "capability %q required", capability)
}

// assignFail is the refusal of a role the principal may not give.
func (a access) assignFail(role string) error {
	switch {
	case role == store.RoleIntegration:
		return fail(http.StatusForbidden, "the role integration is an integration's and its users' alone")
	case !a.ranks(role) && role == store.RoleOwner:
		return fail(http.StatusForbidden, "only an owner gives the role owner")
	case !a.ranks(role):
		return fail(http.StatusForbidden, "the role %s has capabilities yours lacks", role)
	}
	return fail(http.StatusForbidden, "capability %q required to give the role %s", "users", role)
}

// userMay is what the caller may do with a user (userView.May): change
// it, or set its password alone.
type userMay struct {
	Manage   bool `json:"manage"`
	Password bool `json:"password"`
}

func (s *Server) viewUserOf(r *http.Request, st *store.Store, u store.User) userView {
	invitedBy := ""
	if u.InvitedBy != 0 {
		if by, ok, _ := st.UserByID(r.Context(), u.InvitedBy); ok {
			invitedBy = by.Name
		}
	}
	return viewUser(u, invitedBy)
}

// listUsers handles GET /api/v1/users: every user with users, the
// guests and the own user with guests alone.
func (s *Server) listUsers(w http.ResponseWriter, r *http.Request) error {
	st, err := s.userStore()
	if err != nil {
		return err
	}
	p := principalOf(r)
	a := s.accessOf(r.Context(), p, "")
	users, err := st.Users(r.Context())
	if err != nil {
		return userFail(err)
	}
	out := []userView{}
	for _, u := range users {
		self := p.named() && u.ID == p.user.ID
		if a.mayView(u) || self {
			v := s.viewUserOf(r, st, u)
			v.May = &userMay{Manage: a.mayManage(u), Password: a.maySetPassword(u) || self}
			out = append(out, v)
		}
	}
	writeJSON(w, http.StatusOK, out)
	return nil
}

// postUser handles POST /api/v1/users: a new user, or, on an open
// server, the first owner from the local network.
func (s *Server) postUser(w http.ResponseWriter, r *http.Request) error {
	st, err := s.userStore()
	if err != nil {
		return err
	}
	req, err := decodeUser(r)
	if err != nil {
		return err
	}
	ctx := r.Context()
	p := principalOf(r)
	u := store.User{Name: req.Name}
	if req.DisplayName != nil {
		u.DisplayName = strings.TrimSpace(*req.DisplayName)
	}
	if req.Role != nil {
		u.Role = *req.Role
	}
	if req.Disabled != nil {
		u.Disabled = *req.Disabled
	}
	if t, set, err := req.expiresAt(); err != nil {
		return err
	} else if set {
		u.ExpiresAt = t
	}
	password := ""
	if req.Password != nil {
		password = *req.Password
	}
	var by int64
	switch {
	case p.mode == modeOpen:
		// The first owner: the form of docs/store.md section 10, item 4.
		if !localPeer(s.clientIP(r)) {
			return fail(http.StatusForbidden, "the first owner is created from the local network, or on the box with massimo user add")
		}
		if password == "" && req.Record == nil {
			return fail(http.StatusBadRequest, "the first owner needs a password")
		}
		if u.Role != "" && u.Role != store.RoleOwner {
			return fail(http.StatusBadRequest, "the first user is the owner")
		}
		u.Role, u.Disabled, u.ExpiresAt = store.RoleOwner, false, time.Time{}
	case p.named():
		by = p.user.ID
		fallthrough
	default:
		if u.Role == "" {
			u.Role = store.RoleViewer
		}
		if a := s.accessOf(ctx, p, ""); !a.mayAssign(u.Role) {
			if !a.ranks(u.Role) || u.Role == store.RoleIntegration {
				return a.assignFail(u.Role)
			}
			return fail(http.StatusForbidden, "capability %q required to create a user with the role %s", "users", u.Role)
		}
	}
	auditTarget(r, "user:"+store.NormalName(u.Name))
	var created store.User
	if req.Record != nil {
		var rec []byte
		if rec, err = req.record(); err != nil {
			return err
		}
		created, err = st.AddOpaqueUser(ctx, u, rec, req.KSF, by)
	} else {
		created, err = st.AddUser(ctx, u, password, by)
	}
	if errors.Is(err, store.ErrNoRole) {
		return fail(http.StatusBadRequest, "%v", err)
	}
	if err != nil {
		return userFail(err)
	}
	auditSubject(r, userSubject(created))
	if p.mode == modeOpen {
		s.log.Info("first owner created", "user", created.Name, "remote", s.clientIP(r))
	} else {
		s.log.Info("user added", "user", created.Name, "role", created.Role, "remote", s.clientIP(r))
	}
	writeJSON(w, http.StatusCreated, s.viewUserOf(r, st, created))
	return nil
}

// What a route needs of the user of its path (targetUser).
const (
	needView     = iota // see it: mayView, or its own
	needManage          // change it: mayManage, or (with selfOK) its own
	needPassword        // change it or its password alone, or its own
)

// targetUser finds the user of the path and checks that the principal
// may do what need asks of it, or (selfOK) that it is its own.
func (s *Server) targetUser(r *http.Request, st *store.Store, selfOK bool, need ...int) (store.User, access, error) {
	name := r.PathValue("name")
	u, ok, err := st.User(r.Context(), name)
	if err != nil {
		return store.User{}, access{}, userFail(err)
	}
	if !ok {
		return store.User{}, access{}, fail(http.StatusNotFound, "no user %q", name)
	}
	a := s.accessOf(r.Context(), principalOf(r), u.Name)
	n := needManage
	if len(need) > 0 {
		n = need[0]
	}
	ok = map[int]bool{needView: a.mayView(u), needManage: a.mayManage(u), needPassword: a.mayManage(u) || a.maySetPassword(u)}[n]
	if !ok && !(selfOK && a.self) {
		if n == needView {
			return store.User{}, access{}, fail(http.StatusForbidden, "capability %q required", "users")
		}
		return store.User{}, access{}, a.rankFail(u.Role, "users")
	}
	return u, a, nil
}

// getUser handles GET /api/v1/users/{name}.
func (s *Server) getUser(w http.ResponseWriter, r *http.Request) error {
	st, err := s.userStore()
	if err != nil {
		return err
	}
	u, _, err := s.targetUser(r, st, true, needView)
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, s.viewUserOf(r, st, u))
	return nil
}

// putUser handles PUT /api/v1/users/{name}: the display name, the
// role, disabled, the end date and the password, of version 1 or 2. A
// user changes its own display name and password, the latter with
// current_password or a proof from a sign-in by OPAQUE, a user with keys
// only its first one without; a password set by someone else ends the
// user's sessions.
func (s *Server) putUser(w http.ResponseWriter, r *http.Request) error {
	st, err := s.userStore()
	if err != nil {
		return err
	}
	u, a, err := s.targetUser(r, st, true, needPassword)
	if err != nil {
		return err
	}
	req, err := decodeUser(r)
	if err != nil {
		return err
	}
	if req.Name != "" {
		return fail(http.StatusBadRequest, "the name is the path's")
	}
	ctx := r.Context()
	p := principalOf(r)
	// Without mayManage a principal changes its own name and password,
	// or with passwords another's password alone.
	manage := a.mayManage(u)
	if !manage && (req.Role != nil || req.Disabled != nil || len(req.ExpiresAt) > 0 || req.DisplayName != nil && !a.self) {
		return a.rankFail(u.Role, "users")
	}
	if req.Role != nil && !a.mayAssign(*req.Role) {
		return a.assignFail(*req.Role)
	}
	setsPassword := req.Password != nil || req.Record != nil
	if setsPassword && !a.self && !a.maySetPassword(u) {
		return a.rankFail(u.Role, "passwords")
	}
	if setsPassword && a.self && !a.all && u.HasPassword {
		// One's own password: the current one proves the caller is the
		// user, not someone at an unlocked browser. A user with keys
		// only has none, and sets its first one as itself: its key could
		// as well make another key.
		if req.Proof != "" {
			if !s.logins.redeemProof(req.Proof, u.ID, time.Now()) {
				return fail(http.StatusForbidden, "proof is wrong, used or too old: sign in again to prove the current password")
			}
		} else if err := st.CheckPassword(ctx, u.Name, req.CurrentPassword); err != nil {
			if errors.Is(err, store.ErrBadPassword) {
				return fail(http.StatusForbidden, "current_password is wrong")
			}
			return userFail(err)
		}
	}
	end, setEnd, err := req.expiresAt()
	if err != nil {
		return err
	}
	var by int64
	if p.named() {
		by = p.user.ID
	}
	if req.DisplayName != nil || req.Role != nil || req.Disabled != nil || setEnd {
		u, err = st.UpdateUser(ctx, u.Name, func(c *store.User) error {
			if req.DisplayName != nil {
				c.DisplayName = strings.TrimSpace(*req.DisplayName)
			}
			if req.Role != nil {
				c.Role = *req.Role
			}
			if req.Disabled != nil {
				c.Disabled = *req.Disabled
			}
			if setEnd {
				c.ExpiresAt = end
			}
			return nil
		}, by)
		if errors.Is(err, store.ErrNoRole) {
			return fail(http.StatusBadRequest, "%v", err)
		}
		if err != nil {
			return userFail(err)
		}
	}
	if setsPassword {
		if req.Record != nil {
			rec, err := req.record()
			if err != nil {
				return err
			}
			err = st.SetOpaqueRecord(ctx, u.Name, rec, req.KSF, by)
			if err != nil {
				return userFail(err)
			}
		} else if err := st.SetPassword(ctx, u.Name, *req.Password, by); err != nil {
			return userFail(err)
		}
		if !a.self {
			if err := st.RevokeSessions(ctx, u.ID); err != nil {
				return userFail(err)
			}
		}
		s.log.Info("password set", "user", u.Name, "by_self", a.self, "remote", s.clientIP(r))
		if u, _, err = st.User(ctx, u.Name); err != nil {
			return userFail(err)
		}
	}
	s.log.Info("user changed", "user", u.Name, "remote", s.clientIP(r))
	auditSubject(r, userSubject(u))
	v := s.viewUserOf(r, st, u)
	v.May = &userMay{Manage: a.mayManage(u), Password: a.maySetPassword(u) || a.self}
	writeJSON(w, http.StatusOK, v)
	return nil
}

// userSubject is a user as a change line names them: ids, login name,
// display name and role as they are then, never a secret.
func userSubject(u store.User) map[string]any {
	return map[string]any{"id": u.ID, "uuid": u.UUID, "name": u.Name, "display_name": u.DisplayName, "role": u.Role}
}

// tokenSubject is a key as a change line names it: its ids, its label and
// its user's name; never the key.
func tokenSubject(tok store.Token, u store.User) map[string]any {
	return map[string]any{"id": tok.ID, "uuid": tok.UUID, "name": tok.Label, "user": u.Name}
}

// deleteUser handles DELETE /api/v1/users/{name}: the user is removed
// and its tokens revoked; the last enabled owner stays.
func (s *Server) deleteUser(w http.ResponseWriter, r *http.Request) error {
	st, err := s.userStore()
	if err != nil {
		return err
	}
	u, _, err := s.targetUser(r, st, false)
	if err != nil {
		return err
	}
	var by int64
	if p := principalOf(r); p.named() {
		by = p.user.ID
	}
	if err := st.DeleteUser(r.Context(), u.Name, by); err != nil {
		return userFail(err)
	}
	s.log.Info("user removed", "user", u.Name, "remote", s.clientIP(r))
	auditSubject(r, userSubject(u)) // as the user was
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "name": u.Name})
	return nil
}

// listTokens handles GET /api/v1/users/{name}/tokens: the live sessions
// and keys of the user.
func (s *Server) listTokens(w http.ResponseWriter, r *http.Request) error {
	st, err := s.userStore()
	if err != nil {
		return err
	}
	u, _, err := s.targetUser(r, st, true)
	if err != nil {
		return err
	}
	tokens, err := st.Tokens(r.Context(), u.ID)
	if err != nil {
		return userFail(err)
	}
	out := []tokenView{}
	for _, t := range tokens {
		out = append(out, viewToken(t))
	}
	writeJSON(w, http.StatusOK, out)
	return nil
}

// postToken handles POST /api/v1/users/{name}/tokens: a new API key,
// shown once.
func (s *Server) postToken(w http.ResponseWriter, r *http.Request) error {
	st, err := s.userStore()
	if err != nil {
		return err
	}
	u, _, err := s.targetUser(r, st, true)
	if err != nil {
		return err
	}
	var req struct {
		Label string `json:"label"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return fail(http.StatusBadRequest, "bad body: %v", err)
	}
	req.Label = strings.TrimSpace(req.Label)
	if req.Label == "" || len(req.Label) > maxLabel {
		return fail(http.StatusBadRequest, "label: 1 to %d bytes", maxLabel)
	}
	tok, secret, err := st.AddToken(r.Context(), u.ID, store.TokenAPI, req.Label, time.Time{})
	if err != nil {
		return userFail(err)
	}
	auditTarget(r, "token:"+strconv.FormatInt(tok.ID, 10))
	auditSubject(r, tokenSubject(tok, u))
	s.log.Info("api key created", "user", u.Name, "label", req.Label, "remote", s.clientIP(r))
	v := viewToken(tok)
	writeJSON(w, http.StatusCreated, map[string]any{"token": secret, "id": v.ID, "kind": v.Kind, "label": v.Label, "created_at": v.CreatedAt})
	return nil
}

// deleteToken handles DELETE /api/v1/users/{name}/tokens/{id}: the
// session or key is revoked.
func (s *Server) deleteToken(w http.ResponseWriter, r *http.Request) error {
	st, err := s.userStore()
	if err != nil {
		return err
	}
	u, _, err := s.targetUser(r, st, true)
	if err != nil {
		return err
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		return fail(http.StatusBadRequest, "bad token id %q", r.PathValue("id"))
	}
	tok, ok, err := st.Token(r.Context(), id)
	if err != nil {
		return userFail(err)
	}
	if !ok || tok.UserID != u.ID || !tok.RevokedAt.IsZero() {
		return fail(http.StatusNotFound, "user %s has no token %d", u.Name, id)
	}
	if err := st.RevokeToken(r.Context(), id); err != nil {
		return userFail(err)
	}
	s.log.Info("token revoked", "user", u.Name, "token", id, "kind", tok.Kind, "remote", s.clientIP(r))
	auditSubject(r, tokenSubject(tok, u))
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "id": id})
	return nil
}
