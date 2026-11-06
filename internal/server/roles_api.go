// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

// The roles and capabilities routes (docs/store.md section 6): a role's
// set and name, created roles, disabling a role, and disabling a
// capability for every role but owner.

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"go.metaworx.ch/massimo/internal/store"
)

// maxRoleName is the longest display name of a role, in bytes.
const maxRoleName = 64

// roleView is a role as the API shows it.
type roleView struct {
	Key          string    `json:"key"`
	Name         string    `json:"name,omitempty"`
	Builtin      bool      `json:"builtin"`
	Disabled     bool      `json:"disabled"`
	Capabilities []string  `json:"capabilities"`
	Users        int       `json:"users"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

func viewRole(r store.Role) roleView {
	caps := r.Capabilities
	if r.Key == store.RoleOwner {
		caps = append([]string(nil), capabilities...)
	}
	return roleView{Key: r.Key, Name: r.Name, Builtin: r.Builtin, Disabled: r.Disabled, Capabilities: caps, Users: r.Users, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt}
}

// capabilityView is one capability as GET /api/v1/capabilities shows it.
type capabilityView struct {
	Key      string `json:"key"`
	Disabled bool   `json:"disabled"`
}

// checkCapabilities refuses a set with an unknown key.
func checkCapabilities(list []string) error {
	for _, c := range list {
		if !validCapability(strings.TrimSpace(c)) {
			return fail(http.StatusBadRequest, "unknown capability %q", c)
		}
	}
	return nil
}

// roleRequest is the body of POST /api/v1/roles and PUT
// /api/v1/roles/{key}.
type roleRequest struct {
	Key          string    `json:"key,omitempty"` // POST only
	Name         *string   `json:"name"`
	Capabilities *[]string `json:"capabilities"`
}

func decodeRole(r *http.Request) (roleRequest, error) {
	var req roleRequest
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		return req, fail(http.StatusBadRequest, "bad body: %v", err)
	}
	if req.Name != nil && len(*req.Name) > maxRoleName {
		return req, fail(http.StatusBadRequest, "name: at most %d bytes", maxRoleName)
	}
	if req.Capabilities != nil {
		if err := checkCapabilities(*req.Capabilities); err != nil {
			return req, err
		}
	}
	return req, nil
}

func byOf(r *http.Request) int64 {
	if p := principalOf(r); p.named() {
		return p.user.ID
	}
	return 0
}

// listRoles handles GET /api/v1/roles: every role with its set, and the
// capabilities with the disabled ones, for a roles editor.
func (s *Server) listRoles(w http.ResponseWriter, r *http.Request) error {
	st, err := s.userStore()
	if err != nil {
		return err
	}
	roles, err := st.Roles(r.Context())
	if err != nil {
		return userFail(err)
	}
	caps, err := s.capabilityViews(r, st)
	if err != nil {
		return err
	}
	out := []roleView{}
	for _, ro := range roles {
		out = append(out, viewRole(ro))
	}
	writeJSON(w, http.StatusOK, map[string]any{"roles": out, "capabilities": caps})
	return nil
}

// postRole handles POST /api/v1/roles.
func (s *Server) postRole(w http.ResponseWriter, r *http.Request) error {
	st, err := s.userStore()
	if err != nil {
		return err
	}
	req, err := decodeRole(r)
	if err != nil {
		return err
	}
	ro := store.Role{Key: req.Key}
	if req.Name != nil {
		ro.Name = strings.TrimSpace(*req.Name)
	}
	if req.Capabilities != nil {
		ro.Capabilities = *req.Capabilities
	}
	auditTarget(r, "role:"+strings.ToLower(strings.TrimSpace(ro.Key)))
	created, err := st.AddRole(r.Context(), ro, byOf(r))
	if err != nil {
		return userFail(err)
	}
	s.log.Info("role added", "role", created.Key, "remote", clientIP(r))
	writeJSON(w, http.StatusCreated, viewRole(created))
	return nil
}

// putRole handles PUT /api/v1/roles/{key}: the name and the set.
func (s *Server) putRole(w http.ResponseWriter, r *http.Request) error {
	st, err := s.userStore()
	if err != nil {
		return err
	}
	req, err := decodeRole(r)
	if err != nil {
		return err
	}
	if req.Key != "" {
		return fail(http.StatusBadRequest, "the key is the path's")
	}
	changed, err := st.UpdateRole(r.Context(), r.PathValue("key"), func(ro *store.Role) error {
		if req.Name != nil {
			ro.Name = strings.TrimSpace(*req.Name)
		}
		if req.Capabilities != nil {
			ro.Capabilities = *req.Capabilities
		}
		return nil
	}, byOf(r))
	if err != nil {
		return userFail(err)
	}
	s.log.Info("role changed", "role", changed.Key, "remote", clientIP(r))
	writeJSON(w, http.StatusOK, viewRole(changed))
	return nil
}

// deleteRole handles DELETE /api/v1/roles/{key}: a created role no user
// has.
func (s *Server) deleteRole(w http.ResponseWriter, r *http.Request) error {
	st, err := s.userStore()
	if err != nil {
		return err
	}
	key := r.PathValue("key")
	if err := st.DeleteRole(r.Context(), key, byOf(r)); err != nil {
		return userFail(err)
	}
	s.log.Info("role removed", "role", key, "remote", clientIP(r))
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "key": key})
	return nil
}

// putRoleDisabled handles PUT /api/v1/roles/{key}/disabled with
// {"disabled": true|false}.
func (s *Server) putRoleDisabled(w http.ResponseWriter, r *http.Request) error {
	st, err := s.userStore()
	if err != nil {
		return err
	}
	disabled, err := decodeDisabled(r)
	if err != nil {
		return err
	}
	key := r.PathValue("key")
	if err := st.SetRoleDisabled(r.Context(), key, disabled, byOf(r)); err != nil {
		return userFail(err)
	}
	ro, _, err := st.Role(r.Context(), key)
	if err != nil {
		return userFail(err)
	}
	s.log.Info("role disabled set", "role", key, "disabled", disabled, "remote", clientIP(r))
	writeJSON(w, http.StatusOK, viewRole(ro))
	return nil
}

func decodeDisabled(r *http.Request) (bool, error) {
	var req struct {
		Disabled *bool `json:"disabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Disabled == nil {
		return false, fail(http.StatusBadRequest, "body must be {\"disabled\": true|false}")
	}
	return *req.Disabled, nil
}

// capabilityViews lists every capability with whether it is disabled.
func (s *Server) capabilityViews(r *http.Request, st *store.Store) ([]capabilityView, error) {
	off, err := st.DisabledCapabilities(r.Context())
	if err != nil {
		return nil, userFail(err)
	}
	disabled := map[string]bool{}
	for _, c := range off {
		disabled[c] = true
	}
	out := []capabilityView{}
	for _, c := range capabilities {
		out = append(out, capabilityView{Key: c, Disabled: disabled[c]})
	}
	return out, nil
}

// listCapabilities handles GET /api/v1/capabilities.
func (s *Server) listCapabilities(w http.ResponseWriter, r *http.Request) error {
	st, err := s.userStore()
	if err != nil {
		return err
	}
	out, err := s.capabilityViews(r, st)
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, out)
	return nil
}

// putCapability handles PUT /api/v1/capabilities/{key} with
// {"disabled": true|false}: denied to every role but owner.
func (s *Server) putCapability(w http.ResponseWriter, r *http.Request) error {
	st, err := s.userStore()
	if err != nil {
		return err
	}
	key := r.PathValue("key")
	if !validCapability(key) {
		return fail(http.StatusNotFound, "no capability %q", key)
	}
	disabled, err := decodeDisabled(r)
	if err != nil {
		return err
	}
	if err := st.SetCapabilityDisabled(r.Context(), key, disabled, byOf(r)); err != nil {
		return userFail(err)
	}
	s.log.Info("capability disabled set", "capability", key, "disabled", disabled, "remote", clientIP(r))
	writeJSON(w, http.StatusOK, capabilityView{Key: key, Disabled: disabled})
	return nil
}
