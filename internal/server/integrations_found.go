// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

import (
	"context"
	"net/http"
	"slices"
	"sync"
	"time"

	"go.metaworx.ch/massimo/internal/store"
)

// The installations of integrations seen on the LAN (AP IntegrationUsers,
// Analysis A5): Home Assistant announces itself by zeroconf with its
// installation id, so the owner sees which installations could ask to be
// registered, and whether a request's id is one seen on the LAN.

// integrationsBrowseEvery is how often the LAN is asked for integrations,
// with a first run at the start.
const integrationsBrowseEvery = 30 * time.Minute

// FoundIntegration is an installation of an integration seen on the LAN.
type FoundIntegration struct {
	Kind       string `json:"kind"` // homeassistant
	Name       string `json:"name"` // the instance name
	InstanceID string `json:"instance_id"`
	Location   string `json:"location,omitempty"` // the home's name there
	URL        string `json:"url,omitempty"`
	Version    string `json:"version,omitempty"`
	Address    string `json:"address"` // where the answer came from
	// Integration is the uuid of the integration or the request of that
	// installation, the integration first; absent for none.
	Integration string `json:"integration,omitempty"`
}

// foundIntegrations is the last look at the LAN.
type foundIntegrations struct {
	mu    sync.Mutex
	at    time.Time
	found []FoundIntegration
}

// browseIntegrations asks the LAN now and keeps the answer; without
// Options.FindIntegrations it finds nothing.
func (s *Server) browseIntegrations(ctx context.Context) ([]FoundIntegration, time.Time, error) {
	if s.opts.FindIntegrations == nil {
		return []FoundIntegration{}, time.Time{}, nil
	}
	found, err := s.opts.FindIntegrations(ctx)
	if err != nil {
		return nil, time.Time{}, err
	}
	now := time.Now()
	s.integFound.mu.Lock()
	s.integFound.at, s.integFound.found = now, found
	s.integFound.mu.Unlock()
	return found, now, nil
}

// lastFoundIntegrations is the last look's answer.
func (s *Server) lastFoundIntegrations() ([]FoundIntegration, time.Time) {
	s.integFound.mu.Lock()
	defer s.integFound.mu.Unlock()
	return slices.Clone(s.integFound.found), s.integFound.at
}

// instanceSeen says whether the last look saw an installation id.
func (s *Server) instanceSeen(id string) bool {
	if id == "" {
		return false
	}
	found, _ := s.lastFoundIntegrations()
	return slices.ContainsFunc(found, func(f FoundIntegration) bool { return f.InstanceID == id })
}

// integrationsLoop looks at the LAN at the start and every
// integrationsBrowseEvery.
func (s *Server) integrationsLoop(ctx context.Context) {
	for {
		if _, _, err := s.browseIntegrations(ctx); err != nil && ctx.Err() == nil {
			s.log.Warn("integrations on the LAN not looked for", "error", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(integrationsBrowseEvery):
		}
	}
}

// getFoundIntegrations handles GET /api/v1/integrations/found: the
// installations the last look saw, each with the integration or request
// of its id; ?refresh=1 looks now.
func (s *Server) getFoundIntegrations(w http.ResponseWriter, r *http.Request) error {
	st, err := s.userStore()
	if err != nil {
		return err
	}
	found, at := s.lastFoundIntegrations()
	if r.URL.Query().Get("refresh") == "1" {
		if found, at, err = s.browseIntegrations(r.Context()); err != nil {
			return fail(http.StatusBadGateway, "the LAN not asked: %v", err)
		}
	}
	list, err := st.Integrations(r.Context())
	if err != nil {
		return integrationFail(err)
	}
	for n, f := range found {
		for _, i := range list {
			if i.InstanceID != f.InstanceID || i.State == store.IntegrationRefused {
				continue
			}
			if found[n].Integration == "" || i.State != store.IntegrationPending {
				found[n].Integration = i.UUID
			}
		}
	}
	out := struct {
		Found []FoundIntegration `json:"found"`
		At    time.Time          `json:"at,omitzero"`
		Look  bool               `json:"looks"` // the server looks at the LAN
	}{found, at, s.opts.FindIntegrations != nil}
	if out.Found == nil {
		out.Found = []FoundIntegration{}
	}
	writeJSON(w, http.StatusOK, out)
	return nil
}
