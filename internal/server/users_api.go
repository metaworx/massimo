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
	return req, nil
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
	switch {
	case errors.As(err, &he):
		return err
	case errors.Is(err, store.ErrNoUser), errors.Is(err, store.ErrNoRole), errors.Is(err, store.ErrNoToken):
		return fail(http.StatusNotFound, "%v", err)
	case errors.Is(err, store.ErrUserExists), errors.Is(err, store.ErrRoleExists), errors.Is(err, store.ErrLastOwner), errors.Is(err, store.ErrRoleInUse):
		return fail(http.StatusConflict, "%v", err)
	case errors.Is(err, store.ErrBadName), errors.Is(err, store.ErrBadRole):
		return fail(http.StatusBadRequest, "%v", err)
	case errors.Is(err, store.ErrImmutable):
		return fail(http.StatusForbidden, "%v", err)
	}
	return fail(http.StatusInternalServerError, "%v", err)
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
	all    bool // users: every user
	guests bool // guests: users whose role is guest
	self   bool // the user's own name
}

func (s *Server) accessOf(p *principal, name string) access {
	return access{all: p.has("users"), guests: p.has("guests"), self: p.named() && strings.EqualFold(name, p.user.Name)}
}

// mayManage says whether a principal may change a user: with users,
// or with guests for a guest. The own user is the handlers' question.
func (a access) mayManage(u store.User) bool {
	return a.all || a.guests && u.Role == store.RoleGuest
}

// mayAssign says whether a principal may give a role: any with users,
// guest alone with guests.
func (a access) mayAssign(role string) bool {
	return a.all || a.guests && role == store.RoleGuest
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
	a := s.accessOf(p, "")
	users, err := st.Users(r.Context())
	if err != nil {
		return userFail(err)
	}
	out := []userView{}
	for _, u := range users {
		if a.mayManage(u) || p.named() && u.ID == p.user.ID {
			out = append(out, s.viewUserOf(r, st, u))
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
		if !localPeer(clientIP(r)) {
			return fail(http.StatusForbidden, "the first owner is created from the local network, or on the box with massimo user add")
		}
		if password == "" {
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
		if !s.accessOf(p, "").mayAssign(u.Role) {
			return fail(http.StatusForbidden, "capability %q required to create a user with the role %s", "users", u.Role)
		}
	}
	auditTarget(r, "user:"+store.NormalName(u.Name))
	created, err := st.AddUser(ctx, u, password, by)
	if errors.Is(err, store.ErrNoRole) {
		return fail(http.StatusBadRequest, "%v", err)
	}
	if err != nil {
		return userFail(err)
	}
	if p.mode == modeOpen {
		s.log.Info("first owner created", "user", created.Name, "remote", clientIP(r))
	} else {
		s.log.Info("user added", "user", created.Name, "role", created.Role, "remote", clientIP(r))
	}
	writeJSON(w, http.StatusCreated, s.viewUserOf(r, st, created))
	return nil
}

// targetUser finds the user of the path and checks that the principal
// may touch it: through users, guests for a guest, or (selfOK) its own
// name.
func (s *Server) targetUser(r *http.Request, st *store.Store, selfOK bool) (store.User, access, error) {
	name := r.PathValue("name")
	u, ok, err := st.User(r.Context(), name)
	if err != nil {
		return store.User{}, access{}, userFail(err)
	}
	if !ok {
		return store.User{}, access{}, fail(http.StatusNotFound, "no user %q", name)
	}
	a := s.accessOf(principalOf(r), u.Name)
	if !a.mayManage(u) && !(selfOK && a.self) {
		return store.User{}, access{}, fail(http.StatusForbidden, "capability %q required", "users")
	}
	return u, a, nil
}

// getUser handles GET /api/v1/users/{name}.
func (s *Server) getUser(w http.ResponseWriter, r *http.Request) error {
	st, err := s.userStore()
	if err != nil {
		return err
	}
	u, _, err := s.targetUser(r, st, true)
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, s.viewUserOf(r, st, u))
	return nil
}

// putUser handles PUT /api/v1/users/{name}: the display name, the
// role, disabled, the end date and the password. A user changes its own
// display name and password, the latter with current_password; a
// password set by someone else ends the user's sessions.
func (s *Server) putUser(w http.ResponseWriter, r *http.Request) error {
	st, err := s.userStore()
	if err != nil {
		return err
	}
	u, a, err := s.targetUser(r, st, true)
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
	selfOnly := !a.mayManage(u)
	if selfOnly && (req.Role != nil || req.Disabled != nil || len(req.ExpiresAt) > 0) {
		return fail(http.StatusForbidden, "capability %q required", "users")
	}
	if req.Role != nil && !a.mayAssign(*req.Role) {
		return fail(http.StatusForbidden, "capability %q required to give the role %s", "users", *req.Role)
	}
	if req.Password != nil && a.self && !a.all {
		// One's own password: the current one proves the caller is the
		// user, not someone at an unlocked browser.
		if err := st.CheckPassword(ctx, u.Name, req.CurrentPassword); err != nil {
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
	if req.Password != nil {
		if err := st.SetPassword(ctx, u.Name, *req.Password, by); err != nil {
			return userFail(err)
		}
		if !a.self {
			if err := st.RevokeSessions(ctx, u.ID); err != nil {
				return userFail(err)
			}
		}
		s.log.Info("password set", "user", u.Name, "by_self", a.self, "remote", clientIP(r))
		if u, _, err = st.User(ctx, u.Name); err != nil {
			return userFail(err)
		}
	}
	s.log.Info("user changed", "user", u.Name, "remote", clientIP(r))
	writeJSON(w, http.StatusOK, s.viewUserOf(r, st, u))
	return nil
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
	s.log.Info("user removed", "user", u.Name, "remote", clientIP(r))
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
	s.log.Info("api key created", "user", u.Name, "label", req.Label, "remote", clientIP(r))
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
	s.log.Info("token revoked", "user", u.Name, "token", id, "kind", tok.Kind, "remote", clientIP(r))
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "id": id})
	return nil
}
