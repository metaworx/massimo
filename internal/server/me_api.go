// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

// The signed-in user's preferences (docs/store.md section 9 (c)): the
// web UI's settings and colour schemes as a JSON document per key, so
// that they follow the user across browsers. The page uses ui and
// schemes. Reading one's own needs a signed-in user only; writing needs
// the capability preferences, which a guest has not, so that its
// settings stay in the browser. Without a signed-in user (an open
// server, the API token without a store) there is no one to keep them
// for, and the page keeps them in the browser.

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"go.metaworx.ch/massimo/internal/store"
)

// preferenceView is how a preference was stored, the answer of PUT.
type preferenceView struct {
	Key       string          `json:"key"`
	Value     json.RawMessage `json:"value"`
	UpdatedAt time.Time       `json:"updated_at"`
	Version   int64           `json:"version"`
}

// prefUser is the signed-in user of a preferences route, or the error
// of a request that has none.
func (s *Server) prefUser(r *http.Request) (*store.Store, *principal, error) {
	st, err := s.userStore()
	if err != nil {
		return nil, nil, err
	}
	p := principalOf(r)
	if !p.named() {
		return nil, nil, fail(http.StatusConflict, "no user signed in: the preferences stay in the browser")
	}
	return st, p, nil
}

// prefKey reads and checks the path's key.
func prefKey(r *http.Request) (string, error) {
	key := r.PathValue("key")
	if !store.ValidPreferenceKey(key) {
		return "", fail(http.StatusBadRequest, "bad preference key %q: 1 to 32 of lower-case letters, digits, _ . -", key)
	}
	return key, nil
}

// getPreferences handles GET /api/v1/me/preferences: every preference
// of the signed-in user, by key.
func (s *Server) getPreferences(w http.ResponseWriter, r *http.Request) error {
	st, p, err := s.prefUser(r)
	if err != nil {
		return err
	}
	list, err := st.Preferences(r.Context(), p.user.ID)
	if err != nil {
		return fail(http.StatusInternalServerError, "%v", err)
	}
	out := map[string]json.RawMessage{}
	for _, pr := range list {
		out[pr.Key] = pr.Value
	}
	writeJSON(w, http.StatusOK, out)
	return nil
}

// getPreference handles GET /api/v1/me/preferences/{key}: the value.
func (s *Server) getPreference(w http.ResponseWriter, r *http.Request) error {
	st, p, err := s.prefUser(r)
	if err != nil {
		return err
	}
	key, err := prefKey(r)
	if err != nil {
		return err
	}
	pr, ok, err := st.Preference(r.Context(), p.user.ID, key)
	if err != nil {
		return fail(http.StatusInternalServerError, "%v", err)
	}
	if !ok {
		return fail(http.StatusNotFound, "no preference %q", key)
	}
	writeJSON(w, http.StatusOK, pr.Value)
	return nil
}

// putPreference handles PUT /api/v1/me/preferences/{key}: the body is
// the value, a JSON document of at most store.MaxPreferenceBytes.
func (s *Server) putPreference(w http.ResponseWriter, r *http.Request) error {
	st, p, err := s.prefUser(r)
	if err != nil {
		return err
	}
	key, err := prefKey(r)
	if err != nil {
		return err
	}
	auditTarget(r, "preference:"+key)
	body, err := io.ReadAll(io.LimitReader(r.Body, store.MaxPreferenceBytes+1))
	if err != nil {
		return fail(http.StatusBadRequest, "bad body: %v", err)
	}
	pr, err := st.SetPreference(r.Context(), p.user.ID, key, json.RawMessage(body))
	switch {
	case errors.Is(err, store.ErrValueTooLarge):
		return fail(http.StatusRequestEntityTooLarge, "%v", err)
	case errors.Is(err, store.ErrBadValue), errors.Is(err, store.ErrBadKey):
		return fail(http.StatusBadRequest, "%v", err)
	case err != nil:
		return fail(http.StatusInternalServerError, "%v", err)
	}
	writeJSON(w, http.StatusOK, preferenceView{Key: pr.Key, Value: pr.Value, UpdatedAt: pr.UpdatedAt, Version: pr.Version})
	return nil
}

// deletePreference handles DELETE /api/v1/me/preferences/{key}: the
// preference is removed, also when it was not set.
func (s *Server) deletePreference(w http.ResponseWriter, r *http.Request) error {
	st, p, err := s.prefUser(r)
	if err != nil {
		return err
	}
	key, err := prefKey(r)
	if err != nil {
		return err
	}
	auditTarget(r, "preference:"+key)
	if err := st.DeletePreference(r.Context(), p.user.ID, key); err != nil {
		return fail(http.StatusInternalServerError, "%v", err)
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "key": key})
	return nil
}
