// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

import (
	"context"

	"go.metaworx.ch/massimo/internal/store"
)

// How users are named (AP StoreRaise v1.2, "The display"): a user the
// API shows carries display, the chain "<the integration's name> › <its
// display name>" for an integration's user (store.Namer), its display
// name or its login name for any other, and ", as <login name>" where it
// acts as another user; and integration, the integration it is a user
// of. A row of the log and the audit names both the user whose rights
// it had (effective_user) and who made it (real_user).

// namerOf is a Namer of the server's store, nil without one.
func (s *Server) namerOf() *store.Namer {
	if s.opts.Store == nil {
		return nil
	}
	return s.opts.Store.Namer()
}

// rowUser is the user of a row as an eventUser, its chain in Display:
// with as, the user it acted as ("" for none or the same).
func rowUser(ctx context.Context, n *store.Namer, id, as int64) *eventUser {
	if n == nil {
		return nil
	}
	u, ok := n.User(ctx, id)
	if !ok {
		return nil
	}
	out := &eventUser{Name: u.Name, DisplayName: u.DisplayName, Display: n.Display(ctx, id, as)}
	if i, ok := n.Integration(ctx, u); ok {
		out.Integration = i.UUID
	}
	return out
}

// rowUsers are a row's effective and real users (log.effective_user_id
// and log.real_user_id); a row of before the real user, or of no
// integration, has real the effective one.
func rowUsers(ctx context.Context, n *store.Namer, effective, real int64) (eff, rl *eventUser) {
	if real == 0 {
		real = effective
	}
	if effective == 0 && real == 0 {
		return nil, nil
	}
	return rowUser(ctx, n, effective, 0), rowUser(ctx, n, real, effective)
}

// chainName is how the server's own log lines name a row's user: the
// real user's chain where it is an integration's or acted as another,
// else the effective user's login name, as before.
func chainName(ev event) string {
	if r := ev.RealUser; r != nil && (r.Integration != "" || ev.By == nil || r.Name != ev.By.Name) {
		return r.Display
	}
	if ev.By != nil {
		return ev.By.Name
	}
	return ""
}

// userDisplay is how the users list and /me show u: its chain, and the
// user its identity at its integration acts as now; with the integration
// of an integration's user.
func userDisplay(ctx context.Context, n *store.Namer, u store.User) (string, *userIntegration) {
	if n == nil {
		return "", nil
	}
	display := n.Current(ctx, u)
	if i, ok := n.Integration(ctx, u); ok {
		return display, &userIntegration{Key: i.UUID, UUID: i.UUID}
	}
	return display, nil
}

// logUserName is how the server's own log lines name u: its login name,
// an integration's user by its chain.
func (s *Server) logUserName(ctx context.Context, u store.User) string {
	if u.IntegrationID == 0 || s.opts.Store == nil {
		return u.Name
	}
	return s.opts.Store.Namer().Chain(ctx, u)
}
