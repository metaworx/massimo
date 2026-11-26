// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

// What an open server leaves to its first regular user (AP LooseEnds,
// D47-A, with a log entry; AP AccessEnds, Block 3): where that user is
// made, by the first owner's request (postUser), from MASSIMO_API_TOKEN
// at a start or a reload of the settings file (store.Bootstrap), or by
// massimo user add on the box, what was set up before it and still works
// (store.OpenSetup), as a rule while the server was open, else left by
// an owner since removed, is said once, where there is any: a
// line of the log, open_setup at warn, which stays a log alert until a
// person marks it seen, with its line in serve's log; and for the first
// owner's request the answer's open_setup, which the page's toast says.
// Names only, never a key, a token, a password or a secret.

import (
	"context"
	"log/slog"
	"strings"

	"go.metaworx.ch/massimo/internal/store"
)

// openSetupMessage is the message of the open_setup line in serve's log.
const openSetupMessage = "the first regular user was made; what was set up before it still works: check it"

// openSetupView is what an open server left, as the answer of the first
// owner's POST /api/v1/users and the open_setup line say it; an empty
// list is left out.
type openSetupView struct {
	Integrations    openIntegrations `json:"integrations,omitempty"`
	Providers       openProviders    `json:"providers,omitempty"`
	Roles           openRoles        `json:"roles,omitempty"`
	CapabilitiesOff []string         `json:"capabilities_off,omitempty"`
}

// openIntegration is an integration an open server left: its uuid and
// its name as the owner sees it.
type openIntegration struct {
	UUID string `json:"uuid"`
	Name string `json:"name"`
}

// openIntegrations are the integrations an open server left.
type openIntegrations []openIntegration

// LogValue names the integrations in a line of serve's log, by their
// names, as a list of strings is written there.
func (l openIntegrations) LogValue() slog.Value {
	names := make([]string, len(l))
	for i, x := range l {
		names[i] = x.Name
	}
	return slog.StringValue(strings.Join(names, ","))
}

// openProvider is a sign-in provider an open server left: its key, the
// name on its button, and whether a sign-in creates users.
type openProvider struct {
	Key        string `json:"key"`
	Name       string `json:"name,omitempty"`
	AutoCreate bool   `json:"auto_create,omitempty"`
}

// openProviders are the sign-in providers an open server left.
type openProviders []openProvider

// LogValue names the providers in a line of serve's log by their keys,
// one whose sign-in creates users said so.
func (l openProviders) LogValue() slog.Value {
	keys := make([]string, len(l))
	for i, p := range l {
		keys[i] = p.Key
		if p.AutoCreate {
			keys[i] += " (creates users)"
		}
	}
	return slog.StringValue(strings.Join(keys, ","))
}

// openRole is a role an open server left: its key and its display name.
type openRole struct {
	Key  string `json:"key"`
	Name string `json:"name,omitempty"`
}

// openRoles are the roles an open server left.
type openRoles []openRole

// LogValue names the roles in a line of serve's log by their keys.
func (l openRoles) LogValue() slog.Value {
	keys := make([]string, len(l))
	for i, r := range l {
		keys[i] = r.Key
	}
	return slog.StringValue(strings.Join(keys, ","))
}

// openSetupOf is set as the API and the log say it.
func openSetupOf(set store.OpenSetup) *openSetupView {
	v := &openSetupView{CapabilitiesOff: set.CapabilitiesOff}
	for _, r := range set.Roles {
		v.Roles = append(v.Roles, openRole{Key: r.Key, Name: r.Name})
	}
	for _, i := range set.Integrations {
		v.Integrations = append(v.Integrations, openIntegration{UUID: i.UUID, Name: i.Name})
	}
	for _, p := range set.Providers {
		v.Providers = append(v.Providers, openProvider{Key: p.Key, Name: p.Name, AutoCreate: p.AutoCreate})
	}
	return v
}

// OpenSetupSummary is the summary of the log's open_setup line: the
// name and the role of the first regular user, what the open server left
// (store.OpenSetup), each list where it holds any, and via, "cli" for a
// user massimo user add made ("" leaves it out), which writes the line
// into the store itself.
func OpenSetupSummary(user, role string, set store.OpenSetup, via string) map[string]any {
	v := openSetupOf(set)
	summary := map[string]any{"user": user, "role": role}
	if len(v.Integrations) > 0 {
		summary["integrations"] = v.Integrations
	}
	if len(v.Providers) > 0 {
		summary["providers"] = v.Providers
	}
	if len(v.Roles) > 0 {
		summary["roles"] = v.Roles
	}
	if len(v.CapabilitiesOff) > 0 {
		summary["capabilities_off"] = v.CapabilitiesOff
	}
	if via != "" {
		summary["via"] = via
	}
	return summary
}

// tellOpenSetup says once what the open server left to its first
// regular user, user with role, made now: the log's open_setup line at
// warn, a log alert until a person marks it seen, and its line in
// serve's log. It returns what it said, nil where the server left
// nothing or the store could not say.
func (s *Server) tellOpenSetup(ctx context.Context, user, role string) *openSetupView {
	st := s.opts.Store
	if st == nil {
		return nil
	}
	set, err := st.OpenSetup(ctx)
	if err != nil {
		s.log.Error("store: what the open server left not read", "error", err)
		return nil
	}
	if set.Empty() {
		return nil
	}
	ev := systemEvent("open_setup", 0, OpenSetupSummary(user, role, set, ""))
	ev.Level = "warn"
	s.logServer(ev)
	return openSetupOf(set)
}
