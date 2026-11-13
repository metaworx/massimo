// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

// The OpenID Connect providers and the users' identities at them
// (docs/api-v1.md, "Providers: /api/v1/providers" and "Identities:
// /api/v1/users/{name}/identities"; the sign-in itself is oidc.go):
//
//	GET, POST          /api/v1/providers                     capability auth
//	PUT, DELETE        /api/v1/providers/{key}               capability auth
//	GET                /api/v1/users/{name}/identities       capability users, or its own
//	POST               /api/v1/users/{name}/identities       capability users
//	PUT                /api/v1/users/{name}/identities/{id}  capability users
//	DELETE             /api/v1/users/{name}/identities/{id}  capability users, or its own
//
// A user's own link is oidc.go's (oidcStartLink).
//
// A provider's client secret is written, never answered: has_secret
// says whether it has one. An enabled provider is saved only where its
// issuer's discovery document is reached and names it (checkIssuer).

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"go.metaworx.ch/massimo/internal/store"
)

// providerView is a provider as the API answers it.
type providerView struct {
	Key         string    `json:"key"`
	Name        string    `json:"name"`
	Issuer      string    `json:"issuer"`
	ClientID    string    `json:"client_id"`
	HasSecret   bool      `json:"has_secret"`
	SecretLost  bool      `json:"secret_lost"` // sealed, and the key from opaque.key does not open it
	Scopes      string    `json:"scopes"`
	AutoCreate  bool      `json:"auto_create"`
	AutoRole    string    `json:"auto_role"`
	AutoDomains []string  `json:"auto_domains"`
	GroupsClaim string    `json:"groups_claim"`
	Allowed     []string  `json:"allowed_groups"`
	Enabled     bool      `json:"enabled"`
	Callback    string    `json:"callback"` // the redirect URI's path under the page's address
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func viewProvider(p store.AuthProvider) providerView {
	return providerView{Key: p.Key, Name: p.Name, Issuer: p.Issuer, ClientID: p.ClientID, HasSecret: p.ClientSecret != "", SecretLost: p.SecretLost, Scopes: p.Scopes,
		AutoCreate: p.AutoCreate, AutoRole: cmpOr(p.AutoRole, store.RoleViewer), AutoDomains: p.AutoDomains, GroupsClaim: p.GroupsClaim, Allowed: p.AllowedGroups, Enabled: p.Enabled,
		Callback: oidcCallback, CreatedAt: p.CreatedAt, UpdatedAt: p.UpdatedAt}
}

// providerSubject names a provider in the audit's row.
func providerSubject(p store.AuthProvider) map[string]any {
	return map[string]any{"uuid": p.UUID, "key": p.Key, "name": p.Name}
}

// providerFail maps the store's errors of the providers and identities
// to a status.
func providerFail(err error) error {
	text := strings.TrimPrefix(err.Error(), "store: ")
	switch {
	case errors.Is(err, store.ErrNoProvider), errors.Is(err, store.ErrNoIdentity):
		return fail(http.StatusNotFound, "%s", text)
	case errors.Is(err, store.ErrProviderExists), errors.Is(err, store.ErrIdentityExists):
		return fail(http.StatusConflict, "%s", text)
	case errors.Is(err, store.ErrBadProvider), errors.Is(err, store.ErrBadEmail), errors.Is(err, store.ErrNoRole):
		return fail(http.StatusBadRequest, "%s", text)
	}
	return userFail(err)
}

// providerRequest is the body of POST /api/v1/providers and PUT
// /api/v1/providers/{key}: in a PUT a field left out stays as it is,
// client_secret too, and "" clears it.
type providerRequest struct {
	Key          *string   `json:"key"`
	Name         *string   `json:"name"`
	Issuer       *string   `json:"issuer"`
	ClientID     *string   `json:"client_id"`
	ClientSecret *string   `json:"client_secret"`
	Scopes       *string   `json:"scopes"`
	AutoCreate   *bool     `json:"auto_create"`
	AutoRole     *string   `json:"auto_role"`
	AutoDomains  *[]string `json:"auto_domains"`
	GroupsClaim  *string   `json:"groups_claim"`
	Allowed      *[]string `json:"allowed_groups"`
	Enabled      *bool     `json:"enabled"`
}

// apply writes the fields given onto p.
func (req providerRequest) apply(p *store.AuthProvider) {
	set := func(dst *string, v *string) {
		if v != nil {
			*dst = *v
		}
	}
	set(&p.Name, req.Name)
	set(&p.Issuer, req.Issuer)
	set(&p.ClientID, req.ClientID)
	set(&p.ClientSecret, req.ClientSecret)
	set(&p.Scopes, req.Scopes)
	set(&p.AutoRole, req.AutoRole)
	set(&p.GroupsClaim, req.GroupsClaim)
	if req.Allowed != nil {
		p.AllowedGroups = *req.Allowed
	}
	if req.AutoCreate != nil {
		p.AutoCreate = *req.AutoCreate
	}
	if req.AutoDomains != nil {
		p.AutoDomains = *req.AutoDomains
	}
	if req.Enabled != nil {
		p.Enabled = *req.Enabled
	}
}

func decodeProvider(r *http.Request) (providerRequest, error) {
	var req providerRequest
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		return req, fail(http.StatusBadRequest, "bad body: %v", err)
	}
	return req, nil
}

// listProviders handles GET /api/v1/providers: every provider, by key.
func (s *Server) listProviders(w http.ResponseWriter, r *http.Request) error {
	st, err := s.userStore()
	if err != nil {
		return err
	}
	list, err := st.Providers(r.Context())
	if err != nil {
		return providerFail(err)
	}
	out := []providerView{}
	for _, p := range list {
		out = append(out, viewProvider(p))
	}
	writeJSON(w, http.StatusOK, out)
	return nil
}

// checkAutoRole refuses a role for the users a provider creates that
// holds an administrative capability (autoRoleAllowed).
func (s *Server) checkAutoRole(r *http.Request, st *store.Store, p store.AuthProvider) error {
	if err := autoRoleAllowed(r.Context(), st, p.AutoRole); err != nil {
		if errors.Is(err, store.ErrNoRole) {
			return providerFail(err)
		}
		return fail(http.StatusBadRequest, "auto_role: %v", err)
	}
	return nil
}

// saveIssuer checks the issuer of a provider about to be saved enabled,
// and keeps the document's form of it (checkIssuer); a disabled one is
// saved as it is.
func saveIssuer(r *http.Request, p *store.AuthProvider) error {
	if !p.Enabled {
		return nil
	}
	issuer, err := checkIssuer(r.Context(), strings.TrimSpace(p.Issuer))
	if err != nil {
		return fail(http.StatusBadRequest, "issuer: %v", err)
	}
	p.Issuer = issuer
	return nil
}

// postProvider handles POST /api/v1/providers: a new provider, enabled
// unless enabled is false.
func (s *Server) postProvider(w http.ResponseWriter, r *http.Request) error {
	st, err := s.userStore()
	if err != nil {
		return err
	}
	req, err := decodeProvider(r)
	if err != nil {
		return err
	}
	if req.Key == nil {
		return fail(http.StatusBadRequest, "key is needed")
	}
	auditTarget(r, "provider:"+strings.ToLower(strings.TrimSpace(*req.Key)))
	p := store.AuthProvider{Key: *req.Key, Enabled: true}
	req.apply(&p)
	if err := s.checkAutoRole(r, st, p); err != nil {
		return err
	}
	if err := saveIssuer(r, &p); err != nil {
		return err
	}
	p, err = st.AddProvider(r.Context(), p, principalOf(r).userID())
	if err != nil {
		return providerFail(err)
	}
	auditSubject(r, providerSubject(p))
	s.log.Info("sign-in provider added", "provider", p.Key, "issuer", p.Issuer, "remote", s.clientIP(r))
	writeJSON(w, http.StatusCreated, viewProvider(p))
	return nil
}

// putProvider handles PUT /api/v1/providers/{key}: the fields given
// changed, its key staying.
func (s *Server) putProvider(w http.ResponseWriter, r *http.Request) error {
	st, err := s.userStore()
	if err != nil {
		return err
	}
	key := strings.ToLower(r.PathValue("key"))
	auditTarget(r, "provider:"+key)
	req, err := decodeProvider(r)
	if err != nil {
		return err
	}
	if req.Key != nil && strings.ToLower(strings.TrimSpace(*req.Key)) != key {
		return fail(http.StatusBadRequest, "a provider's key cannot be changed")
	}
	// the role checked before the store's transaction, which holds its
	// one connection
	now, ok, err := st.Provider(r.Context(), key)
	if err != nil {
		return providerFail(err)
	}
	if !ok {
		return fail(http.StatusNotFound, "no provider %q", key)
	}
	before := now
	req.apply(&now)
	if err := s.checkAutoRole(r, st, now); err != nil {
		return err
	}
	// the issuer checked where it changes or the provider is enabled,
	// outside the transaction too
	if now.Issuer != before.Issuer || now.Enabled && !before.Enabled {
		if err := saveIssuer(r, &now); err != nil {
			return err
		}
	}
	p, err := st.UpdateProvider(r.Context(), key, func(p *store.AuthProvider) error {
		req.apply(p)
		p.Issuer = now.Issuer
		return nil
	}, principalOf(r).userID())
	if err != nil {
		return providerFail(err)
	}
	auditSubject(r, providerSubject(p))
	s.log.Info("sign-in provider changed", "provider", p.Key, "remote", s.clientIP(r))
	writeJSON(w, http.StatusOK, viewProvider(p))
	return nil
}

// deleteProvider handles DELETE /api/v1/providers/{key}: the provider
// removed with every identity at it.
func (s *Server) deleteProvider(w http.ResponseWriter, r *http.Request) error {
	st, err := s.userStore()
	if err != nil {
		return err
	}
	key := strings.ToLower(r.PathValue("key"))
	auditTarget(r, "provider:"+key)
	p, ok, err := st.Provider(r.Context(), key)
	if err != nil {
		return providerFail(err)
	}
	if !ok {
		return fail(http.StatusNotFound, "no provider %q", key)
	}
	if err := st.DeleteProvider(r.Context(), key, principalOf(r).userID()); err != nil {
		return providerFail(err)
	}
	auditSubject(r, providerSubject(p))
	s.log.Info("sign-in provider removed", "provider", p.Key, "remote", s.clientIP(r))
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "key": p.Key})
	return nil
}

// identityView is an identity as the API answers it: the provider's
// subject, the user id there, which is no secret, and whether it is
// bound.
type identityView struct {
	ID         int64      `json:"id"`
	Provider   string     `json:"provider"`
	Email      string     `json:"email"`
	Subject    string     `json:"subject"`
	Bound      bool       `json:"bound"`
	CreatedAt  time.Time  `json:"created_at"`
	Name       string     `json:"name"`
	Username   string     `json:"username"`
	Groups     []string   `json:"groups"`
	BoundAt    *time.Time `json:"bound_at,omitempty"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
}

func viewIdentity(i store.Identity) identityView {
	return identityView{ID: i.ID, Provider: i.Provider, Email: i.Email, Subject: i.Subject, Bound: i.Bound(), Name: i.Name, Username: i.Username, Groups: i.Groups, CreatedAt: i.CreatedAt, BoundAt: optTime(i.BoundAt), LastUsedAt: optTime(i.LastUsedAt)}
}

func identitySubject(i store.Identity, u store.User) map[string]any {
	m := map[string]any{"id": i.ID, "uuid": i.UUID, "provider": i.Provider, "email": i.Email, "user": u.Name}
	if i.Subject != "" {
		m["subject"] = i.Subject
	}
	return m
}

// lastWayIn reports whether removing the identity would leave the user
// no way to sign in on the card but API keys (AP Oidc v1.1, D21-A): no
// password and no other bound identity. A user is refused that; a
// manager is not, who can set a password.
func lastWayIn(r *http.Request, st *store.Store, u store.User, gone store.Identity) (bool, error) {
	if u.HasPassword || !gone.Bound() {
		return false, nil
	}
	list, err := st.Identities(r.Context(), u.ID)
	if err != nil {
		return false, err
	}
	for _, i := range list {
		if i.ID != gone.ID && i.Bound() {
			return false, nil
		}
	}
	return true, nil
}

// listIdentities handles GET /api/v1/users/{name}/identities: a user's
// own too.
func (s *Server) listIdentities(w http.ResponseWriter, r *http.Request) error {
	st, err := s.userStore()
	if err != nil {
		return err
	}
	u, _, err := s.targetUser(r, st, true)
	if err != nil {
		return err
	}
	list, err := st.Identities(r.Context(), u.ID)
	if err != nil {
		return providerFail(err)
	}
	out := []identityView{}
	for _, i := range list {
		out = append(out, viewIdentity(i))
	}
	writeJSON(w, http.StatusOK, out)
	return nil
}

// postIdentity handles POST /api/v1/users/{name}/identities: an e-mail
// at a provider assigned to the user, bound at its first sign-in, or the
// user id there (subject), bound at once.
func (s *Server) postIdentity(w http.ResponseWriter, r *http.Request) error {
	st, err := s.userStore()
	if err != nil {
		return err
	}
	u, _, err := s.targetUser(r, st, false)
	if err != nil {
		return err
	}
	auditTarget(r, "user:"+u.Name)
	var req struct {
		Provider string `json:"provider"`
		Email    string `json:"email"`
		Subject  string `json:"subject"`
	}
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil || req.Provider == "" || (req.Email == "") == (req.Subject == "") {
		return fail(http.StatusBadRequest, "body must have \"provider\" and either \"email\" or \"subject\"")
	}
	var i store.Identity
	if req.Subject != "" {
		i, err = st.AddSubjectIdentity(r.Context(), u.ID, req.Provider, req.Subject, principalOf(r).userID())
	} else {
		i, err = st.AddIdentity(r.Context(), u.ID, req.Provider, req.Email, principalOf(r).userID())
	}
	if err != nil {
		return providerFail(err)
	}
	auditSubject(r, identitySubject(i, u))
	s.log.Info("identity assigned", "user", u.Name, "provider", i.Provider, "remote", s.clientIP(r))
	writeJSON(w, http.StatusCreated, viewIdentity(i))
	return nil
}

// putIdentity handles PUT /api/v1/users/{name}/identities/{id}: the
// e-mail or the user id of an identity no sign-in has used yet.
func (s *Server) putIdentity(w http.ResponseWriter, r *http.Request) error {
	st, err := s.userStore()
	if err != nil {
		return err
	}
	u, _, err := s.targetUser(r, st, false)
	if err != nil {
		return err
	}
	auditTarget(r, "user:"+u.Name)
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		return fail(http.StatusBadRequest, "bad identity id %q", r.PathValue("id"))
	}
	var req struct {
		Email   string `json:"email"`
		Subject string `json:"subject"`
	}
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil || (req.Email == "") == (req.Subject == "") {
		return fail(http.StatusBadRequest, "body must have either \"email\" or \"subject\"")
	}
	i, err := st.UpdateIdentity(r.Context(), u.ID, id, req.Email, req.Subject)
	if errors.Is(err, store.ErrIdentityUsed) {
		return fail(http.StatusConflict, "%s", strings.TrimPrefix(err.Error(), "store: "))
	}
	if err != nil {
		return providerFail(err)
	}
	auditSubject(r, identitySubject(i, u))
	s.log.Info("identity changed", "user", u.Name, "provider", i.Provider, "remote", s.clientIP(r))
	writeJSON(w, http.StatusOK, viewIdentity(i))
	return nil
}

// deleteIdentity handles DELETE /api/v1/users/{name}/identities/{id}: a
// user's own too, but not the last way it signs in (lastWayIn).
func (s *Server) deleteIdentity(w http.ResponseWriter, r *http.Request) error {
	st, err := s.userStore()
	if err != nil {
		return err
	}
	u, a, err := s.targetUser(r, st, true)
	if err != nil {
		return err
	}
	auditTarget(r, "user:"+u.Name)
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		return fail(http.StatusBadRequest, "bad identity id %q", r.PathValue("id"))
	}
	gone, err := st.Identity(r.Context(), u.ID, id)
	if err != nil {
		return providerFail(err)
	}
	if a.self {
		if last, err := lastWayIn(r, st, u, gone); err != nil {
			return providerFail(err)
		} else if last {
			return fail(http.StatusConflict, "this is the last way you sign in: set a password or link another account first")
		}
	}
	if err := st.DeleteIdentity(r.Context(), u.ID, id); err != nil {
		return providerFail(err)
	}
	auditSubject(r, identitySubject(gone, u))
	s.log.Info("identity removed", "user", u.Name, "provider", gone.Provider, "remote", s.clientIP(r))
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "id": id})
	return nil
}
