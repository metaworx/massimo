// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package store

import "context"

// ChainSeparator joins an integration's name and the display name of one
// of its users in a user's chain (Namer.Chain).
const ChainSeparator = " › "

// Namer names users as the page, the command line and the server's own
// log lines show them (docs/store.md section 6, "Integrations"): a user
// by its display name, else its login name; an integration's user by the
// chain "<the integration's name> › <its display name>", the
// integration's name being its own user's display name; and, where a
// user acts as another, ", as <that user's login name>" after it. The
// chain comes from users.integration_id: nothing reads it from a name.
// A Namer reads each user and integration once, for one request or one
// command; it is not safe for concurrent use.
type Namer struct {
	s      *Store
	users  map[int64]*User
	integs map[int64]*Integration
	links  map[int64]int64 // the linked identities: their real user to the user they act as
}

// Namer returns a Namer of the store's users as they are now.
func (s *Store) Namer() *Namer {
	return &Namer{s: s, users: map[int64]*User{}, integs: map[int64]*Integration{}}
}

// Seed gives the Namer users already read, removed ones too, so that it
// does not read them again.
func (n *Namer) Seed(users ...User) {
	for _, u := range users {
		n.users[u.ID] = &u
	}
}

// User returns the user of a row, removed or not; false for none.
func (n *Namer) User(ctx context.Context, id int64) (User, bool) {
	if id == 0 {
		return User{}, false
	}
	u, seen := n.users[id]
	if !seen {
		if got, ok, err := n.s.UserByID(ctx, id); err == nil && ok {
			u = &got
		}
		n.users[id] = u
	}
	if u == nil {
		return User{}, false
	}
	return *u, true
}

// Integration returns the integration whose user u is; false for a user
// of none, or of an integration removed.
func (n *Namer) Integration(ctx context.Context, u User) (Integration, bool) {
	if u.IntegrationID == 0 {
		return Integration{}, false
	}
	i, seen := n.integs[u.IntegrationID]
	if !seen {
		if got, ok, err := n.s.IntegrationByID(ctx, u.IntegrationID); err == nil && ok {
			i = &got
		}
		n.integs[u.IntegrationID] = i
	}
	if i == nil {
		return Integration{}, false
	}
	return *i, true
}

// IntegrationName is an integration's name as it is shown: its own
// user's display name, else the name it reports, else its uuid.
func IntegrationName(i Integration) string {
	switch {
	case i.DisplayName != "":
		return i.DisplayName
	case i.Name != "":
		return i.Name
	}
	return i.UUID
}

// plainName is a user's display name, else its login name.
func plainName(u User) string {
	if u.DisplayName != "" {
		return u.DisplayName
	}
	return u.Name
}

// Chain is how u is shown, without whom it acts as: its display name,
// else its login name; for an integration's own user the integration's
// name; for another of its users "<the integration's name> › <its
// display name>".
func (n *Namer) Chain(ctx context.Context, u User) string {
	i, ok := n.Integration(ctx, u)
	if !ok {
		return plainName(u)
	}
	head := IntegrationName(i)
	if u.ID == i.UserID || u.Name == integrationUserName(i.UUID, "") {
		return head
	}
	return head + ChainSeparator + plainName(u)
}

// Display is how a row made by real with the rights of effective is
// shown, the log's and the audit's: real's chain, and ", as <effective's
// login name>" where the two differ. Either may be 0, the other then
// standing for both; "" where both are.
func (n *Namer) Display(ctx context.Context, real, effective int64) string {
	if real == 0 {
		real = effective
	}
	u, ok := n.User(ctx, real)
	if !ok {
		return ""
	}
	out := n.Chain(ctx, u)
	if effective != 0 && effective != real {
		if as, ok := n.User(ctx, effective); ok {
			out += ", as " + as.Name
		}
	}
	return out
}

// Current is how u is shown in the users list: its chain, and ", as
// <login name>" where its identity at its integration is linked to that
// Massimo user now.
func (n *Namer) Current(ctx context.Context, u User) string {
	if u.IntegrationID == 0 {
		return plainName(u)
	}
	if n.links == nil {
		n.links = map[int64]int64{}
		if ids, err := n.s.AllIntegrationIdentities(ctx); err == nil {
			for _, id := range ids {
				if id.Linked() {
					n.links[id.RealUserID] = id.EffectiveUserID
				}
			}
		}
	}
	return n.Display(ctx, u.ID, n.links[u.ID])
}
