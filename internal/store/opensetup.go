// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package store

// What an open server leaves to its first regular user (AP LooseEnds,
// D47-A; AP AccessEnds, Block 3): a store without regular users lets
// every request from the local network do everything, so what such a
// request stored and what still gives access or sends data once the
// server has users is listed when the first one is made, for the owner
// to check, with what an owner since removed left (docs/store.md
// section 6).

import (
	"context"
	"fmt"
	"time"
)

// OpenSetup is what gives access once a server has regular users and
// may have been set up while it had none: read at the first regular
// user, it is what was set up while the server was open, or what an
// earlier owner, since removed, left behind. Names only, never a key or
// a secret.
type OpenSetup struct {
	// Integrations are the integrations active with a live key, or one
	// still to be collected with the approval's code (RequestLife).
	Integrations []OpenIntegration
	// Providers are the sign-in providers enabled.
	Providers []OpenProvider
	// Roles are the roles added, and the default ones changed or
	// disabled since the store was made; owner and integration, which
	// never change, are not among them.
	Roles []OpenRole
	// CapabilitiesOff are the capabilities turned off for every role but
	// owner (DisabledCapabilities).
	CapabilitiesOff []string
}

// OpenIntegration is an integration OpenSetup lists: its uuid and its
// name as the owner sees it, the one given, else the one reported.
type OpenIntegration struct {
	UUID string
	Name string
}

// OpenRole is a role OpenSetup lists: its key and its display name
// ("" shows the key's translated default).
type OpenRole struct {
	Key  string
	Name string
}

// OpenProvider is a sign-in provider OpenSetup lists: its key, the name
// on its button ("" shows the key), and whether a sign-in without an
// identity creates the user (AutoCreate).
type OpenProvider struct {
	Key        string
	Name       string
	AutoCreate bool
}

// Empty reports whether OpenSetup lists nothing.
func (o OpenSetup) Empty() bool {
	return len(o.Integrations) == 0 && len(o.Providers) == 0 && len(o.Roles) == 0 && len(o.CapabilitiesOff) == 0
}

// OpenSetup reads what OpenSetup lists, as the store holds it now.
// Registration requests still waiting are not listed: they give nothing
// until the owner approves them, which the Integrations card asks for
// (AP AccessEnds, D15).
func (s *Store) OpenSetup(ctx context.Context) (OpenSetup, error) {
	var out OpenSetup
	integs, err := s.Integrations(ctx)
	if err != nil {
		return out, err
	}
	now := s.now()
	for _, i := range integs {
		if i.State != IntegrationActive {
			continue
		}
		live, err := s.liveIntegrationKey(ctx, i, now)
		if err != nil {
			return out, err
		}
		if !live {
			continue
		}
		name := i.DisplayName
		if name == "" {
			name = i.Name
		}
		out.Integrations = append(out.Integrations, OpenIntegration{UUID: i.UUID, Name: name})
	}
	if out.Providers, err = s.enabledProviders(ctx); err != nil {
		return out, err
	}
	roles, err := s.Roles(ctx)
	if err != nil {
		return out, err
	}
	for _, r := range roles {
		if roleSetUp(r) {
			out.Roles = append(out.Roles, OpenRole{Key: r.Key, Name: r.Name})
		}
	}
	off, err := s.DisabledCapabilities(ctx)
	if err != nil {
		return out, err
	}
	if len(off) > 0 {
		out.CapabilitiesOff = off
	}
	return out, nil
}

// liveIntegrationKey reports whether an active integration holds a live
// key, or may still collect one with its approval's code: a key never
// collected is collected until RequestLife after the approval
// (CollectIntegrationKey).
func (s *Store) liveIntegrationKey(ctx context.Context, i Integration, now time.Time) (bool, error) {
	if now.Before(i.RequestExpires) {
		return true, nil
	}
	if i.UserID == 0 {
		return false, nil
	}
	tokens, err := s.Tokens(ctx, i.UserID)
	if err != nil {
		return false, err
	}
	for _, t := range tokens {
		if t.Kind == TokenAPI && !t.Expired(now) {
			return true, nil
		}
	}
	return false, nil
}

// enabledProviders lists the sign-in providers enabled, by key, without
// their client's secret, which it does not read.
func (s *Store) enabledProviders(ctx context.Context) ([]OpenProvider, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT p.key, p.name, p.auto_create FROM auth_providers p WHERE p.deleted_at IS NULL AND p.enabled = 1 ORDER BY p.key`)
	if err != nil {
		return nil, fmt.Errorf("store: providers: %w", err)
	}
	defer rows.Close()
	var out []OpenProvider
	for rows.Next() {
		var p OpenProvider
		if err := rows.Scan(&p.Key, &p.Name, &p.AutoCreate); err != nil {
			return nil, fmt.Errorf("store: providers: %w", err)
		}
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: providers: %w", err)
	}
	return out, nil
}

// roleSetUp reports whether a role was added, or a default one changed
// or disabled, since the store was made: a role that is not a default
// one, or a default one past its first version. Owner and integration
// never change (Fixed); migration 0021 raised integration's version. A
// role integration-v0, which that migration made for the users the role
// integration had before, is listed as any other: a request may make
// one of that key as well.
func roleSetUp(r Role) bool {
	switch {
	case Fixed(r.Key):
		return false
	case !r.Builtin:
		return true
	}
	return r.Version > 1
}
