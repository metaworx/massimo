// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"

	"go.metaworx.ch/massimo/internal/store"
)

// The registration of integrations (docs/api-v1.md, "Integrations";
// store/integrations.go): an integration asks without a key, the owner
// approves it after comparing the short code both sides show, and the
// integration collects its key with that code. The routes that ask and
// collect take no key; they are bounded per address and by the number of
// requests waiting.

// IntegrationRequestsKey is the setting that lets integrations ask to be
// registered: "off" refuses them; anything else, the default, takes them.
const IntegrationRequestsKey = "integrations.requests"

// requestLimit bounds the registration requests of one address: at most
// requestBurst within requestWindow.
type requestLimit struct {
	mu   sync.Mutex
	seen map[string][]time.Time
}

const (
	requestBurst  = 10
	requestWindow = 10 * time.Minute
)

// allow reports whether one more request from addr is taken now.
func (l *requestLimit) allow(addr string, now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.seen == nil {
		l.seen = map[string][]time.Time{}
	}
	var kept []time.Time
	for _, t := range l.seen[addr] {
		if now.Sub(t) < requestWindow {
			kept = append(kept, t)
		}
	}
	if len(kept) >= requestBurst {
		l.seen[addr] = kept
		return false
	}
	l.seen[addr] = append(kept, now)
	return true
}

// codeAlphabet makes the short codes: no 0/O, 1/I/L to confuse.
const codeAlphabet = "ABCDEFGHJKMNPQRSTUVWXYZ23456789"

// newRequestCode is a short code of six characters.
func newRequestCode() (string, error) {
	var b [6]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	for i := range b {
		b[i] = codeAlphabet[int(b[i])%len(codeAlphabet)]
	}
	return string(b[:]), nil
}

// integrationView is an integration as the API shows it.
type integrationView struct {
	UUID            string    `json:"uuid"`
	Kind            string    `json:"kind"`
	Key             string    `json:"key,omitempty"`
	Name            string    `json:"name"`
	State           string    `json:"state"`
	InstanceID      string    `json:"instance_id"`
	PendingInstance string    `json:"pending_instance,omitempty"`
	URL             string    `json:"url,omitempty"`
	Version         string    `json:"version,omitempty"`
	MaxCaps         []string  `json:"max_caps"`
	User            string    `json:"user,omitempty"`
	RequestCode     string    `json:"request_code,omitempty"`
	RequestExpires  time.Time `json:"request_expires,omitzero"`
	RequestRemote   string    `json:"request_remote,omitempty"`
	KeyIssuedAt     time.Time `json:"key_issued_at,omitzero"`
	CreatedAt       time.Time `json:"created_at"`
	// Users are its users, in GET /api/v1/integrations/{uuid} alone.
	Users []integrationUserView `json:"users,omitempty"`
}

// viewIntegration is i as the API shows it; a request shows its code and
// where it came from.
func (s *Server) viewIntegration(r *http.Request, i store.Integration) integrationView {
	v := integrationView{UUID: i.UUID, Kind: i.Kind, Name: i.Name, State: i.State, InstanceID: i.InstanceID, PendingInstance: i.PendingInstance,
		URL: i.URL, Version: i.AppVersion, MaxCaps: i.MaxCaps, KeyIssuedAt: i.KeyIssuedAt, CreatedAt: i.CreatedAt}
	if i.State == store.IntegrationPending {
		v.RequestCode, v.RequestExpires, v.RequestRemote = i.RequestCode, i.RequestExpires, i.RequestRemote
	} else {
		v.Key = i.Key
		if u, ok, err := s.opts.Store.UserByID(r.Context(), i.UserID); err == nil && ok {
			v.User = u.Name
		}
	}
	return v
}

// integrationFail maps the store's errors of integrations to statuses.
func integrationFail(err error) error {
	switch {
	case errors.Is(err, store.ErrNoIntegration):
		return fail(http.StatusNotFound, "%v", err)
	case errors.Is(err, store.ErrNotPending), errors.Is(err, store.ErrRequestGone):
		return fail(http.StatusGone, "%v", err)
	case errors.Is(err, store.ErrTooManyRequests):
		return &httpError{status: http.StatusTooManyRequests, msg: err.Error(), retryAfter: 60}
	case errors.Is(err, store.ErrBadInstance):
		return fail(http.StatusBadRequest, "%v", err)
	}
	return fail(http.StatusInternalServerError, "%v", err)
}

// postIntegrationRequest handles POST /api/v1/integrations/requests: an
// integration asks to be registered, without a key.
func (s *Server) postIntegrationRequest(w http.ResponseWriter, r *http.Request) error {
	st, err := s.userStore()
	if err != nil {
		return err
	}
	if v, _ := st.SettingString(r.Context(), IntegrationRequestsKey); v == "off" {
		return fail(http.StatusForbidden, "this server takes no registration requests (setting %s)", IntegrationRequestsKey)
	}
	if !s.integrationRequests.allow(s.clientIP(r), time.Now()) {
		return &httpError{status: http.StatusTooManyRequests, msg: "too many registration requests from this address", retryAfter: 60}
	}
	var req struct {
		Kind       string `json:"kind"`
		Name       string `json:"name"`
		InstanceID string `json:"instance_id"`
		URL        string `json:"url"`
		Version    string `json:"version"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return fail(http.StatusBadRequest, "bad body: %v", err)
	}
	if req.Kind != store.KindHomeAssistant {
		return fail(http.StatusBadRequest, "kind must be %q", store.KindHomeAssistant)
	}
	name := strings.TrimSpace(req.Name)
	if name == "" || len(name) > 64 || len(req.URL) > 256 || len(req.Version) > 32 {
		return fail(http.StatusBadRequest, "name is 1 to 64 characters, url at most 256, version at most 32")
	}
	code, err := newRequestCode()
	if err != nil {
		return fail(http.StatusInternalServerError, "%v", err)
	}
	i, err := st.AddIntegrationRequest(r.Context(), store.IntegrationRequest{Kind: req.Kind, Name: name, InstanceID: strings.ToLower(req.InstanceID),
		URL: req.URL, AppVersion: req.Version, Remote: s.clientIP(r)}, code)
	if err != nil {
		return integrationFail(err)
	}
	auditTarget(r, "integration:"+i.UUID)
	writeJSON(w, http.StatusCreated, map[string]any{"id": i.UUID, "code": i.RequestCode, "expires_at": i.RequestExpires})
	return nil
}

// getIntegrationRequest handles GET /api/v1/integrations/requests/{id}:
// the integration collects its key with its code, once the owner approved
// it.
func (s *Server) getIntegrationRequest(w http.ResponseWriter, r *http.Request) error {
	st, err := s.userStore()
	if err != nil {
		return err
	}
	key, i, err := st.CollectIntegrationKey(r.Context(), r.PathValue("id"), strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("code"))))
	switch {
	case errors.Is(err, store.ErrRequestPending):
		writeJSON(w, http.StatusAccepted, map[string]any{"state": store.IntegrationPending})
		return nil
	case err != nil:
		return integrationFail(err)
	}
	writeJSON(w, http.StatusOK, map[string]any{"key": key, "integration": s.viewIntegration(r, i)})
	return nil
}

// getIntegrations handles GET /api/v1/integrations: the integrations and
// the requests waiting.
func (s *Server) getIntegrations(w http.ResponseWriter, r *http.Request) error {
	st, err := s.userStore()
	if err != nil {
		return err
	}
	list, err := st.Integrations(r.Context())
	if err != nil {
		return integrationFail(err)
	}
	out := []integrationView{}
	for _, i := range list {
		out = append(out, s.viewIntegration(r, i))
	}
	writeJSON(w, http.StatusOK, out)
	return nil
}

// checkMaxCaps refuses a maximum beyond the role integration.
func checkMaxCaps(caps []string) error {
	for _, c := range caps {
		if !slices.Contains(integrationCeiling, c) {
			return fail(http.StatusBadRequest, "max_caps: %q is not an integration's to have", c)
		}
	}
	return nil
}

// approveIntegration handles POST /api/v1/integrations/{uuid}/approve:
// {"max_caps": [...], "attach": "<uuid>"}, both optional; the maximum is
// the whole role integration by default.
func (s *Server) approveIntegration(w http.ResponseWriter, r *http.Request) error {
	st, err := s.userStore()
	if err != nil {
		return err
	}
	var req struct {
		MaxCaps *[]string `json:"max_caps"`
		Attach  string    `json:"attach"`
	}
	if r.ContentLength != 0 {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			return fail(http.StatusBadRequest, "bad body: %v", err)
		}
	}
	var caps []string
	if req.MaxCaps != nil {
		if err := checkMaxCaps(*req.MaxCaps); err != nil {
			return err
		}
		caps = *req.MaxCaps
	} else if req.Attach == "" {
		caps = slices.Clone(integrationCeiling)
	}
	i, err := st.ApproveIntegration(r.Context(), r.PathValue("uuid"), caps, req.Attach, principalOf(r).userID())
	if err != nil {
		return integrationFail(err)
	}
	writeJSON(w, http.StatusOK, s.viewIntegration(r, i))
	return nil
}

// refuseIntegration handles POST /api/v1/integrations/{uuid}/refuse.
func (s *Server) refuseIntegration(w http.ResponseWriter, r *http.Request) error {
	st, err := s.userStore()
	if err != nil {
		return err
	}
	if err := st.RefuseIntegration(r.Context(), r.PathValue("uuid"), principalOf(r).userID()); err != nil {
		return integrationFail(err)
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	return nil
}

// acceptInstance handles POST /api/v1/integrations/{uuid}/accept-instance:
// the installation id last seen becomes the accepted one.
func (s *Server) acceptInstance(w http.ResponseWriter, r *http.Request) error {
	st, err := s.userStore()
	if err != nil {
		return err
	}
	i, err := st.AcceptInstance(r.Context(), r.PathValue("uuid"), principalOf(r).userID())
	if err != nil {
		return integrationFail(err)
	}
	writeJSON(w, http.StatusOK, s.viewIntegration(r, i))
	return nil
}

// putIntegration handles PUT /api/v1/integrations/{uuid}: {"name",
// "max_caps", "state"}, each optional.
func (s *Server) putIntegration(w http.ResponseWriter, r *http.Request) error {
	st, err := s.userStore()
	if err != nil {
		return err
	}
	var req struct {
		Name    *string   `json:"name"`
		MaxCaps *[]string `json:"max_caps"`
		State   *string   `json:"state"`
	}
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		return fail(http.StatusBadRequest, "bad body: %v", err)
	}
	if req.MaxCaps != nil {
		if err := checkMaxCaps(*req.MaxCaps); err != nil {
			return err
		}
	}
	if req.State != nil && *req.State != store.IntegrationActive && *req.State != store.IntegrationDisabled {
		return fail(http.StatusBadRequest, "state is active or disabled")
	}
	if req.Name != nil && (strings.TrimSpace(*req.Name) == "" || len(*req.Name) > 64) {
		return fail(http.StatusBadRequest, "name is 1 to 64 characters")
	}
	i, err := st.SetIntegration(r.Context(), r.PathValue("uuid"), func(i *store.Integration) error {
		if req.Name != nil {
			i.Name = *req.Name
		}
		if req.MaxCaps != nil {
			i.MaxCaps = *req.MaxCaps
		}
		if req.State != nil {
			i.State = *req.State
		}
		return nil
	}, principalOf(r).userID())
	if err != nil {
		return integrationFail(err)
	}
	writeJSON(w, http.StatusOK, s.viewIntegration(r, i))
	return nil
}

// deleteIntegration handles DELETE /api/v1/integrations/{uuid}: its key
// revoked, the integration disabled; its users and links stay.
func (s *Server) deleteIntegration(w http.ResponseWriter, r *http.Request) error {
	st, err := s.userStore()
	if err != nil {
		return err
	}
	if err := st.RevokeIntegration(r.Context(), r.PathValue("uuid"), principalOf(r).userID()); err != nil {
		return integrationFail(err)
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	return nil
}

// integrationUserView is one of an integration's users as the API shows
// it: who it acts as, and a proposal by name for a link.
type integrationUserView struct {
	ID              string    `json:"id"` // its id at the integration, or system
	Name            string    `json:"name"`
	User            string    `json:"user"`     // the user its calls act as
	OwnUser         string    `json:"own_user"` // the user made at its first call
	Linked          bool      `json:"linked"`
	SystemGenerated bool      `json:"system_generated,omitempty"`
	LastLevel       string    `json:"last_level,omitempty"`
	LastUsedAt      time.Time `json:"last_used_at,omitzero"`
	// Proposal is a Massimo user of the same name, for the owner to link
	// it to (D1-C); absent where linked or none matches.
	Proposal string `json:"proposal,omitempty"`
}

// integrationOf is the integration of a request's path, a 404 for none
// or a request.
func (s *Server) integrationOf(r *http.Request, st *store.Store) (store.Integration, error) {
	i, ok, err := st.IntegrationByUUID(r.Context(), r.PathValue("uuid"))
	if err != nil {
		return store.Integration{}, integrationFail(err)
	}
	if !ok || i.State == store.IntegrationPending || i.State == store.IntegrationRefused {
		return store.Integration{}, fail(http.StatusNotFound, "no integration %s", r.PathValue("uuid"))
	}
	return i, nil
}

// getIntegration handles GET /api/v1/integrations/{uuid}: the
// integration with its users, each with a proposal for a link by name.
func (s *Server) getIntegration(w http.ResponseWriter, r *http.Request) error {
	st, err := s.userStore()
	if err != nil {
		return err
	}
	i, err := s.integrationOf(r, st)
	if err != nil {
		return err
	}
	ids, err := st.IntegrationIdentities(r.Context(), i.ID)
	if err != nil {
		return integrationFail(err)
	}
	users, err := st.Users(r.Context())
	if err != nil {
		return userFail(err)
	}
	// Proposals leave out every integration's users.
	marks, _, err := integrationMarks(r.Context(), st, users)
	if err != nil {
		return integrationFail(err)
	}
	byID := map[int64]store.User{}
	for _, u := range users {
		byID[u.ID] = u
	}
	out := []integrationUserView{}
	for _, id := range ids {
		v := integrationUserView{ID: id.Subject, Name: id.Name, User: byID[id.UserID].Name, OwnUser: byID[id.OwnUserID].Name, Linked: id.Linked(),
			SystemGenerated: id.SystemGenerated, LastLevel: id.LastLevel, LastUsedAt: id.LastUsedAt}
		if !id.Linked() && id.Subject != "system" && id.Name != "" {
			for _, u := range users {
				if marks[u.ID] == nil && (strings.EqualFold(u.DisplayName, id.Name) || strings.EqualFold(u.Name, id.Name)) {
					v.Proposal = u.Name
					break
				}
			}
		}
		out = append(out, v)
	}
	v := s.viewIntegration(r, i)
	v.Users = out
	writeJSON(w, http.StatusOK, v)
	return nil
}

// putIntegrationUser handles PUT /api/v1/integrations/{uuid}/users/{id}:
// {"user": "<name>"} links the integration's user to a Massimo user,
// {"user": null} unlinks it. The caller must outrank the user linked to,
// as for changing that user.
func (s *Server) putIntegrationUser(w http.ResponseWriter, r *http.Request) error {
	st, err := s.userStore()
	if err != nil {
		return err
	}
	i, err := s.integrationOf(r, st)
	if err != nil {
		return err
	}
	var req struct {
		User *string `json:"user"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return fail(http.StatusBadRequest, "bad body: %v", err)
	}
	var to int64
	if req.User != nil {
		u, ok, err := st.User(r.Context(), *req.User)
		if err != nil {
			return userFail(err)
		}
		if !ok {
			return fail(http.StatusNotFound, "no user %s", *req.User)
		}
		if a := s.accessOf(r.Context(), principalOf(r), ""); !a.ranks(u.Role) {
			return a.rankFail(u.Role, "integrations")
		}
		to = u.ID
	}
	id, err := st.LinkIntegrationUser(r.Context(), i.ID, strings.ToLower(r.PathValue("id")), to, principalOf(r).userID())
	switch {
	case errors.Is(err, store.ErrBadLink):
		return fail(http.StatusBadRequest, "%v", err)
	case err != nil:
		return integrationFail(err)
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "linked": id.Linked()})
	return nil
}

// integrationMarks are the users list's marks: which users are an
// integration's, and which integration users are linked to a user.
func integrationMarks(ctx context.Context, st *store.Store, users []store.User) (map[int64]*userIntegration, map[int64][]userLink, error) {
	integs, err := st.Integrations(ctx)
	if err != nil {
		return nil, nil, err
	}
	ids, err := st.AllIntegrationIdentities(ctx)
	if err != nil {
		return nil, nil, err
	}
	names := map[int64]string{}
	for _, u := range users {
		names[u.ID] = u.Name
	}
	byID := map[int64]store.Integration{}
	marks := map[int64]*userIntegration{}
	for _, i := range integs {
		byID[i.ID] = i
		if i.UserID != 0 {
			marks[i.UserID] = &userIntegration{Key: i.Key, UUID: i.UUID}
		}
	}
	links := map[int64][]userLink{}
	for _, id := range ids {
		i := byID[id.IntegrationID]
		m := &userIntegration{Key: i.Key, UUID: i.UUID, ID: id.Subject, Name: id.Name}
		if id.Linked() {
			m.LinkedTo = names[id.UserID]
			links[id.UserID] = append(links[id.UserID], userLink{Integration: i.Key, ID: id.Subject, Name: id.Name})
		}
		marks[id.OwnUserID] = m
	}
	return marks, links, nil
}
