// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

// The users routes (docs/store.md section 6): list, create, change and
// remove users, and the sessions and API keys of each. The route is
// registered with users; a guests holder reaches it for users whose
// role is guest, regular ones or, for a call through an integration,
// that integration's own; an integrations holder for an integration's
// users; and a user reaches its own name for its display name, its
// password and its tokens (auth.go, allowed). On an open server, POST
// /api/v1/users from the local network creates the first owner.

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"go.metaworx.ch/massimo/internal/password"
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
	// Record and KSF are a password of version 2 in place of Password:
	// the registration record of opaque_api.go and its key stretching.
	Record *string `json:"record"`
	KSF    string  `json:"ksf,omitempty"`
	// Proof is a proof of the current password from a sign-in by OPAQUE,
	// for one's own change, in place of current_password.
	Proof string `json:"proof,omitempty"`
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
		return req, refuse(http.StatusBadRequest, codeUserDisplayNameLong, maxDisplayName)
	}
	if req.Record != nil && req.Password != nil {
		return req, fail(http.StatusBadRequest, "password or record, not both")
	}
	return req, nil
}

// record reads the registration record of a request.
func (req userRequest) record() ([]byte, error) {
	b, err := opaqueB64.DecodeString(*req.Record)
	if err != nil {
		return nil, fail(http.StatusBadRequest, "record: not base64url: %v", err)
	}
	return b, nil
}

// expiresAt reads the end date of a request: set, cleared (null), or
// absent.
func (req userRequest) expiresAt() (t time.Time, set bool, err error) {
	return expiresOf(req.ExpiresAt)
}

// expiresOf reads an end date, expires_at of a user's or an API key's
// body, decoded apart so that a null is told from an absent field: an
// RFC 3339 time (set), null (set, the zero time: no end), or absent (not
// set); anything else is 400, and so is a time that reads as the zero
// time (0001-01-01T00:00:00Z), which would else be taken for null
// (end_date_year_one).
func expiresOf(raw json.RawMessage) (t time.Time, set bool, err error) {
	if len(raw) == 0 {
		return time.Time{}, false, nil
	}
	if string(raw) == "null" {
		return time.Time{}, true, nil
	}
	var s string
	if json.Unmarshal(raw, &s) != nil {
		return time.Time{}, false, fail(http.StatusBadRequest, "expires_at: want a time such as 2026-12-24T18:00:00Z, or null")
	}
	t, err = time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Time{}, false, fail(http.StatusBadRequest, "expires_at: %v", err)
	}
	if t.IsZero() {
		return time.Time{}, false, refuse(http.StatusBadRequest, codeEndDateYearOne, s)
	}
	return t, true, nil
}

// endFail is the refusal of end, an API key's end date, that has passed
// (store.ErrEndPassed, AP KeyLife, Block 4): 400 key_end_passed, as for
// any bad field; any other error goes on as it is.
func endFail(err error, end time.Time) error {
	if errors.Is(err, store.ErrEndPassed) {
		return refuse(http.StatusBadRequest, codeKeyEndPassed, end.UTC().Format(time.RFC3339))
	}
	return err
}

// userFail maps a store error of the users and roles to a status. A
// request whose client went away before the store answered says so
// (clientGone): only a route that checks or hashes a password says
// that it waited for its turn, through passwordFail (AP MobileApp v1.5,
// D106-A).
func userFail(err error) error {
	var he *httpError
	if errors.As(err, &he) {
		return err
	}
	if errors.Is(err, store.ErrBusy) {
		return passwordBusy(err)
	}
	if gone := clientGone(err); gone != nil {
		return gone
	}
	// The store's "store: " says where the error came from, not what is
	// wrong; a client shows the rest as it is.
	text := strings.TrimPrefix(err.Error(), "store: ")
	switch {
	case errors.Is(err, store.ErrNoUser), errors.Is(err, store.ErrNoRole), errors.Is(err, store.ErrNoToken):
		return fail(http.StatusNotFound, "%s", text)
	case errors.Is(err, store.ErrUserExists), errors.Is(err, store.ErrRoleExists), errors.Is(err, store.ErrLastOwner), errors.Is(err, store.ErrRoleInUse):
		return fail(http.StatusConflict, "%s", text)
	case errors.Is(err, store.ErrBadName):
		// the name's rule, which the page's check of a new name words alike
		return refuseSaying(http.StatusBadRequest, text, codeUserNameBad)
	case errors.Is(err, store.ErrBadRole), errors.Is(err, store.ErrBadMessage):
		return fail(http.StatusBadRequest, "%s", text)
	case errors.Is(err, store.ErrNoOpaque):
		return fail(http.StatusServiceUnavailable, "%s", text)
	case errors.Is(err, store.ErrImmutable), errors.Is(err, store.ErrIntegrationUser):
		return fail(http.StatusForbidden, "%s", text)
	}
	return fail(http.StatusInternalServerError, "%s", text)
}

// noWayIn refuses a key, a password or a provider's identity for an
// integration's user: it acts through its integration alone, whose own
// key is the one way in, so that nobody, on an open server anybody on
// the local network, gives such a user a sign-in of its own.
func noWayIn(u store.User) error {
	if u.IntegrationID != 0 {
		return refuse(http.StatusForbidden, codeUserIntegrationNoWayIn, u.Name)
	}
	return nil
}

// newPassword holds a plain password that a request sets to the rule of
// internal/password, with the user's names and the server's as its
// context: 400 with the rule's words where it fails, which name no word
// of the context. A record registered by OPAQUE is never seen and so
// never checked, and the callers leave out an empty password, which
// means none (AP PasswordRules, D11-A, D50-A).
func (s *Server) newPassword(r *http.Request, pw string, names ...string) error {
	if err := password.Check(pw, append(names[:len(names):len(names)], s.serverNames(r)...)...); err != nil {
		return fail(http.StatusBadRequest, "%v", err)
	}
	return nil
}

// hostName is the host's name, the server's name by default; the tests
// give the server a fixed one, so that none reads the machine's.
var hostName = os.Hostname

// serverNames are the server's names a new password must not contain:
// Options.ServerName, or the first label of the host's name, and the
// first label of the request's Host. A Host a reverse proxy or a client
// sets otherwise only adds a word or takes one away from the request's
// own check of a new password: it lets nobody in.
func (s *Server) serverNames(r *http.Request) []string {
	name := s.opts.ServerName
	if name == "" {
		h, _ := hostName()
		name = password.HostWord(h)
	}
	return []string{name, password.HostWord(r.Host)}
}

// busyRetry is the Retry-After, in seconds, of a password check or
// hash refused because the store runs as many as it takes at once.
const busyRetry = 5

// passwordBusy is the answer to a password check or hash that the
// store refused for the load (store.ErrBusy), or that waited until the
// client went away; nil for any other error.
func passwordBusy(err error) error {
	if errors.Is(err, store.ErrBusy) || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return withRetryAfter(refuse(http.StatusServiceUnavailable, codePasswordBusy), busyRetry)
	}
	return nil
}

// passwordFail is userFail for a route that checks or hashes a
// password through the store, a password set, a user added with one, a
// registration: a check or hash that waited until its client went away
// keeps passwordBusy's answer, since it waited for its turn.
func passwordFail(err error) error {
	var he *httpError
	if errors.As(err, &he) {
		return err
	}
	if busy := passwordBusy(err); busy != nil {
		return busy
	}
	return userFail(err)
}

// clientGoneText is the error of a request whose client went away
// before the store answered: the audit's row says it, the result
// failed; the client, gone, reads nothing.
const clientGoneText = "the client went away before the answer"

// clientGone is the answer to a store error of a request whose context
// ended, its client gone or its time up, before the store answered; nil
// for any other error.
func clientGone(err error) error {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return fail(http.StatusServiceUnavailable, clientGoneText)
	}
	return nil
}

// userStore is the store, or the error of a server without one.
func (s *Server) userStore() (*store.Store, error) {
	if s.opts.Store == nil {
		return nil, refuse(http.StatusServiceUnavailable, codeUserNoStore)
	}
	return s.opts.Store, nil
}

// access is what a principal may do with users.
type access struct {
	all       bool // users: every user
	guests    bool // guests: users whose role is guest (guestOf)
	passwords bool // passwords: the password of a user it outranks
	// integrations: an integration's users, whatever their role but
	// owner (integrationsOf; AP AccessEnds, D41-A5).
	integrations bool
	// integration is the id of the integration the principal comes
	// through, 0 for none: its call's, or its user's where that is an
	// integration's and holds a key from before. Its guests holder
	// reaches that integration's guests alone (D41-A4).
	integration int64
	// self is the user's own name, for a principal that comes through no
	// integration: a call for an integration's user linked to a Massimo
	// user acts with that user's role, but has none of the rights of its
	// own name, its key, password, identities and fields, since it
	// manages its own integration's users alone, and a key one of its
	// users holds from before counts as such a call (selfOf; AP
	// AccessEnds, D41-A4).
	self bool
	// outranks is the rank rule: whether the principal's role has every
	// capability of the role named; nil where no rank applies (an open
	// server, the one token of a server without a store).
	outranks func(role string) bool
}

// accessOf is what principal p may do with the user name ("" for none
// in particular). The rank rule compares the roles' own sets, owner
// holding every capability: an owner outranks every role, nobody else
// outranks owner, and otherwise a role outranks one whose set is part
// of its own. A role the store cannot read is outranked by owner alone.
func (s *Server) accessOf(ctx context.Context, p *principal, name string) access {
	a := access{all: p.has("users"), guests: p.has("guests"), passwords: p.has("passwords"), integrations: p.has("integrations"),
		self: selfOf(p, name)}
	if !p.named() || s.opts.Store == nil {
		return a
	}
	if p.integration != nil {
		a.integration = p.integration.ID
	} else {
		a.integration = p.user.IntegrationID
	}
	st, own := s.opts.Store, p.user.Role
	sets := map[string]map[string]bool{}
	set := func(key string) map[string]bool {
		if m, ok := sets[key]; ok {
			return m
		}
		var m map[string]bool
		if r, ok, err := st.Role(ctx, key); err == nil && ok {
			m = map[string]bool{}
			for _, c := range r.Capabilities {
				m[c] = true
			}
		}
		sets[key] = m
		return m
	}
	a.outranks = func(role string) bool {
		switch {
		case own == store.RoleOwner:
			return true
		case role == store.RoleOwner:
			return false
		}
		mine, theirs := set(own), set(role)
		if mine == nil || theirs == nil {
			return false
		}
		for c := range theirs {
			if !mine[c] {
				return false
			}
		}
		return true
	}
	return a
}

// selfOf reports whether name is the principal's own user's, the rights
// of an own name being a regular user's alone: never a call's through
// an integration, nor a key's that an integration's user holds from
// before (access.self); auth.go's allowed admits by it too.
func selfOf(p *principal, name string) bool {
	return p.named() && p.integration == nil && p.user.IntegrationID == 0 && strings.EqualFold(name, p.user.Name)
}

// ranks reports whether the rank rule lets the principal act on a role.
func (a access) ranks(role string) bool {
	return a.outranks == nil || a.outranks(role)
}

// guestOf says whether guests reaches user u (AP AccessEnds, D41-A4): a
// guest, a regular one where the principal comes through no
// integration, one of that integration's where it does, so that a call
// through an integration manages its own integration's guests alone and
// no holder of guests of no integration an integration's.
func (a access) guestOf(u store.User) bool {
	return a.guests && u.Role == store.RoleGuest && u.IntegrationID == a.integration
}

// integrationsOf says whether integrations reaches user u (AP
// AccessEnds, D41-A5): an integration's user, whatever its role but
// owner, which stays an owner's alone as every owner does (D47-A).
func (a access) integrationsOf(u store.User) bool {
	return a.integrations && u.IntegrationID != 0 && u.Role != store.RoleOwner
}

// mayView says whether a principal sees a user: with users or
// passwords every user, with guests or integrations those it reaches
// (guestOf, integrationsOf; AP AccessEnds, D45-A). The own user is the
// handlers' question.
func (a access) mayView(u store.User) bool {
	return a.all || a.passwords || a.guestOf(u) || a.integrationsOf(u)
}

// mayManage says whether a principal may change a user, its name, role,
// state and keys: with users, or with guests for a guest it reaches, a
// user it outranks; with integrations an integration's user it reaches,
// whatever its rank (AP AccessEnds, D47-A), the role given held to the
// rank rule by mayAssign.
func (a access) mayManage(u store.User) bool {
	return (a.all || a.guestOf(u)) && a.ranks(u.Role) || a.integrationsOf(u)
}

// maySetPassword says whether a principal may set a user's password: with
// users, passwords, or guests for a guest it reaches, a user it
// outranks. integrations sets none: an integration's user has no
// password of its own (noWayIn).
func (a access) maySetPassword(u store.User) bool {
	return (a.all || a.passwords || a.guestOf(u)) && a.ranks(u.Role)
}

// mayAdd says whether a principal may add a user, which is a regular
// one: with users, or with guests where it comes through no integration
// (AP AccessEnds, D41-A4).
func (a access) mayAdd() bool {
	return a.all || a.guests && a.integration == 0
}

// mayAssign says whether a principal may give user u the role r, maxCaps
// the maximum of u's integration: any role it outranks with users (a
// role beyond the maximum taken, AP AccessEnds, D6-A), guest alone with
// guests, to a regular user or, through an integration, to one of that
// integration's (a new user is regular, so a call through an
// integration makes none), and with integrations, for an integration's
// user it reaches, a role it outranks within the role integration and
// the maximum, never owner (D47-A, D48-A). The role
// integration is an integration's and its users' alone
// (integrations.go): an integration's user is given it back by the rank
// rule where it was given another (D5), and no other user is given it.
func (a access) mayAssign(r store.Role, u store.User, maxCaps []string) bool {
	switch {
	case r.Key == store.RoleIntegration && u.IntegrationID == 0:
		return false
	case a.all || a.guests && r.Key == store.RoleGuest && u.IntegrationID == a.integration:
		return a.ranks(r.Key)
	case a.integrationsOf(u) && a.ranks(r.Key):
		return r.Key == store.RoleIntegration || r.Key != store.RoleOwner && allIn(r.Capabilities, within(maxCaps))
	}
	return false
}

// within is what an integration's user may act with, the role
// integration and maxCaps, its integration's maximum.
func within(maxCaps []string) map[string]bool {
	return intersect(setOf(maxCaps), integrationCeiling)
}

// roleOf is the role key among roles, the store's; a key it does not
// know comes back with no capabilities, for the store to refuse.
func roleOf(roles []store.Role, key string) store.Role {
	for _, r := range roles {
		if r.Key == key {
			return r
		}
	}
	return store.Role{Key: key}
}

// assignable are the keys of the roles the principal may give user u,
// in the order of roles, the store's, a disabled one among them as the
// users list shows it: none where it may not manage u; else those
// mayAssign lets it give, and for an integration's user only those
// within the role integration and maxCaps, its integration's maximum,
// and integration itself (AP AccessEnds, D3): every call of such a user
// is held there, whatever its role (principalFor), so a role beyond it
// would promise what it never gives. The owner has no set, holding every
// capability by code, and is within no maximum. It is never nil, so that
// "none" is said as an empty list.
func (a access) assignable(u store.User, roles []store.Role, maxCaps []string) []string {
	out := []string{}
	if !a.mayManage(u) {
		return out
	}
	in := within(maxCaps)
	for _, r := range roles {
		if !a.mayAssign(r, u, maxCaps) {
			continue
		}
		if u.IntegrationID != 0 && r.Key != store.RoleIntegration && (r.Key == store.RoleOwner || !allIn(r.Capabilities, in)) {
			continue
		}
		out = append(out, r.Key)
	}
	return out
}

// allIn reports whether every capability of caps is in set.
func allIn(caps []string, set map[string]bool) bool {
	for _, c := range caps {
		if !set[c] {
			return false
		}
	}
	return true
}

// rankFail is the refusal of a user or role the principal does not
// outrank, or the capability it lacks.
func (a access) rankFail(role, capability string) error {
	switch {
	case !a.ranks(role) && role == store.RoleOwner:
		return refuse(http.StatusForbidden, codeUserOwnerOnly)
	case !a.ranks(role):
		return refuse(http.StatusForbidden, codeRoleBeyondYours, role)
	}
	return refuse(http.StatusForbidden, codeCapabilityRequired, capability)
}

// throughIntegration is the refusal of a regular user, or another
// integration's, to a call through an integration (AP AccessEnds,
// D41-A4).
func throughIntegration() error {
	return refuse(http.StatusForbidden, codeIntegrationOwnUsersOnly)
}

// manageFail is the refusal of user u to a principal that may not change
// it. A guest that guests does not reach (guestOf) is refused with why:
// a call through an integration changes its own integration's users
// alone, and an integration's guest is changed by its integration and by
// holders of integrations or of users. Else the refusal is the rank
// rule's or the capability's (rankFail).
func (a access) manageFail(u store.User, capability string) error {
	switch {
	case a.guests && u.Role == store.RoleGuest && a.integration != 0 && u.IntegrationID != a.integration:
		return throughIntegration()
	case a.guests && !a.all && u.Role == store.RoleGuest && u.IntegrationID != a.integration:
		return refuse(http.StatusForbidden, codeUserIntegrationGuest, u.Name)
	}
	return a.rankFail(u.Role, capability)
}

// assignFail is the refusal of the role r the principal may not give
// user u.
func (a access) assignFail(r store.Role, u store.User) error {
	switch {
	case r.Key == store.RoleIntegration && u.IntegrationID == 0:
		return refuse(http.StatusForbidden, codeRoleIntegrationOnly)
	case !a.ranks(r.Key) && r.Key == store.RoleOwner:
		return refuse(http.StatusForbidden, codeRoleOwnerOnly)
	case !a.ranks(r.Key):
		return refuse(http.StatusForbidden, codeRoleBeyondYours, r.Key)
	case a.integrationsOf(u):
		return refuse(http.StatusForbidden, codeRoleBeyondIntegration, r.Key)
	}
	return refuse(http.StatusForbidden, codeCapabilityRequiredToGive, "users", r.Key)
}

// userMay is what the caller may do with a user (userView.May): change
// it, or set its password alone, and the roles it may give it.
type userMay struct {
	Manage   bool `json:"manage"`
	Password bool `json:"password"`
	// Roles are the keys of the roles the caller may give the user
	// (assignable), empty where it may give none.
	Roles []string `json:"roles"`
	// Beyond says that the user's role, as stored, lies beyond the role
	// integration and its integration's maximum, within which it acts
	// all the same (beyond; AP AccessEnds, D4): what Roles leaves out may
	// lie within and be one the caller does not outrank.
	Beyond bool `json:"beyond"`
}

// mayOf is what the principal may do with u, self where u is its own
// user, of the store's roles and u's integration's maximum.
func (a access) mayOf(u store.User, self bool, roles []store.Role, maxCaps []string) *userMay {
	return &userMay{Manage: a.mayManage(u), Password: a.maySetPassword(u) || self, Roles: a.assignable(u, roles, maxCaps), Beyond: beyond(u, roles, maxCaps)}
}

// beyond reports whether u is an integration's user whose stored role
// lies beyond the role integration and maxCaps, its integration's
// maximum: owner always, integration never, another where its set does
// not lie within both.
func beyond(u store.User, roles []store.Role, maxCaps []string) bool {
	if u.IntegrationID == 0 || u.Role == store.RoleIntegration {
		return false
	}
	return u.Role == store.RoleOwner || !allIn(roleOf(roles, u.Role).Capabilities, within(maxCaps))
}

// maxOf is the maximum of user u's integration, none for a regular user
// or one whose integration is gone.
func maxOf(ctx context.Context, st *store.Store, u store.User) ([]string, error) {
	if u.IntegrationID == 0 {
		return nil, nil
	}
	i, ok, err := st.IntegrationByID(ctx, u.IntegrationID)
	if err != nil || !ok {
		return nil, err
	}
	return i.MaxCaps, nil
}

// viewUserOf is u as the users routes show it, its chain and its
// integration named by n, a Namer of st (nil for a new one).
func (s *Server) viewUserOf(r *http.Request, st *store.Store, n *store.Namer, u store.User) userView {
	if n == nil {
		n = st.Namer()
	}
	invitedBy := ""
	if u.InvitedBy != 0 {
		if by, ok := n.User(r.Context(), u.InvitedBy); ok {
			invitedBy = by.Name
		}
	}
	v := viewUser(u, invitedBy)
	v.Display, v.Integration = userDisplay(r.Context(), n, u)
	return v
}

// listUsers handles GET /api/v1/users: every user with users or
// passwords; else the own user and those the caller may manage, with
// guests the guests it reaches, with integrations an integration's users
// (mayView); each with what the caller may do with it. The roles and the
// integrations are read once.
func (s *Server) listUsers(w http.ResponseWriter, r *http.Request) error {
	st, err := s.userStore()
	if err != nil {
		return err
	}
	p := principalOf(r)
	a := s.accessOf(r.Context(), p, "")
	users, err := st.Users(r.Context())
	if err != nil {
		return userFail(err)
	}
	roles, err := st.Roles(r.Context())
	if err != nil {
		return userFail(err)
	}
	integs, err := st.Integrations(r.Context())
	if err != nil {
		return userFail(err)
	}
	marks, links, err := integrationMarks(r.Context(), st, users, integs)
	if err != nil {
		return userFail(err)
	}
	maxima := map[int64][]string{}
	for _, i := range integs {
		maxima[i.ID] = i.MaxCaps
	}
	out := []userView{}
	n := st.Namer()
	n.Seed(users...)
	for _, u := range users {
		self := selfOf(p, u.Name)
		if a.mayView(u) || self {
			v := s.viewUserOf(r, st, n, u)
			v.May = a.mayOf(u, self, roles, maxima[u.IntegrationID])
			if m := marks[u.ID]; m != nil {
				v.Integration = m
			}
			v.Links = links[u.ID]
			out = append(out, v)
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
		if !localPeer(s.clientIP(r)) {
			return refuse(http.StatusForbidden, codeFirstOwnerNotLocal)
		}
		if password == "" && req.Record == nil {
			return refuse(http.StatusBadRequest, codeFirstOwnerPasswordNeeded)
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
		// A new user is regular: no maximum, and its role's set is asked
		// of an integration's user alone (mayAssign).
		if a, r := s.accessOf(ctx, p, ""), (store.Role{Key: u.Role}); !a.mayAssign(r, u, nil) {
			switch {
			case !a.mayAdd() && a.guests:
				return throughIntegration()
			case !a.ranks(u.Role) || u.Role == store.RoleIntegration:
				return a.assignFail(r, u)
			}
			return refuse(http.StatusForbidden, codeCapabilityRequiredToCreate, "users", u.Role)
		}
	}
	auditTarget(r, "user:"+store.NormalName(u.Name))
	if password != "" {
		if err := s.newPassword(r, password, u.Name, u.DisplayName); err != nil {
			return err
		}
	}
	var created store.User
	var rec []byte
	if req.Record != nil {
		if rec, err = req.record(); err != nil {
			return err
		}
	}
	switch {
	case p.mode == modeOpen && req.Record != nil:
		// The first owner once (AP FirstOwner, D15): the mode was read
		// when the request came, so the store counts the users again as
		// it writes the owner.
		created, err = st.AddFirstOpaqueOwner(ctx, u, rec, req.KSF)
	case p.mode == modeOpen:
		created, err = st.AddFirstOwner(ctx, u, password)
	case req.Record != nil:
		created, err = st.AddOpaqueUser(ctx, u, rec, req.KSF, by)
	default:
		created, err = st.AddUser(ctx, u, password, by)
	}
	if errors.Is(err, store.ErrHasUsers) {
		return refuse(http.StatusConflict, codeFirstOwnerExists)
	}
	if errors.Is(err, store.ErrNoRole) {
		return fail(http.StatusBadRequest, "%v", err)
	}
	if err != nil {
		return passwordFail(err)
	}
	auditSubject(r, userSubject(created))
	out := userCreated{userView: s.viewUserOf(r, st, nil, created)}
	if p.mode == modeOpen {
		s.log.Info("first owner created", "user", created.Name, "remote", s.clientIP(r))
		// the owner is written: a client gone now leaves the line to be said all the same
		out.OpenSetup = s.tellOpenSetup(context.WithoutCancel(ctx), created.Name, created.Role)
	} else {
		s.log.Info("user added", "user", created.Name, "role", created.Role, "remote", s.clientIP(r))
	}
	writeJSON(w, http.StatusCreated, out)
	return nil
}

// userCreated is the answer of POST /api/v1/users: the user, and for the
// first owner what the open server left, where it left anything
// (tellOpenSetup, AP AccessEnds, Block 3).
type userCreated struct {
	userView
	OpenSetup *openSetupView `json:"open_setup,omitempty"`
}

// What a route needs of the user of its path (targetUser).
const (
	needView     = iota // see it: mayView, or its own
	needManage          // change it: mayManage, or (with selfOK) its own
	needPassword        // change it or its password alone, or its own
)

// targetUser finds the user of the path and checks that the principal
// may do what need asks of it, or (selfOK) that it is its own.
func (s *Server) targetUser(r *http.Request, st *store.Store, selfOK bool, need ...int) (store.User, access, error) {
	name := r.PathValue("name")
	u, ok, err := st.User(r.Context(), name)
	if err != nil {
		return store.User{}, access{}, userFail(err)
	}
	if !ok {
		return store.User{}, access{}, refuse(http.StatusNotFound, codeUserUnknown, name)
	}
	a := s.accessOf(r.Context(), principalOf(r), u.Name)
	n := needManage
	if len(need) > 0 {
		n = need[0]
	}
	ok = map[int]bool{needView: a.mayView(u), needManage: a.mayManage(u), needPassword: a.mayManage(u) || a.maySetPassword(u)}[n]
	if !ok && !(selfOK && a.self) {
		if n == needView {
			return store.User{}, access{}, refuse(http.StatusForbidden, codeCapabilityRequired, "users")
		}
		return store.User{}, access{}, a.manageFail(u, "users")
	}
	return u, a, nil
}

// getUser handles GET /api/v1/users/{name}.
func (s *Server) getUser(w http.ResponseWriter, r *http.Request) error {
	st, err := s.userStore()
	if err != nil {
		return err
	}
	u, _, err := s.targetUser(r, st, true, needView)
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, s.viewUserOf(r, st, nil, u))
	return nil
}

// putUser handles PUT /api/v1/users/{name}: the display name, the
// role, disabled, the end date and the password, of version 1 or 2. A
// user changes its own display name and password, the latter with
// current_password or a proof from a sign-in by OPAQUE, a user with keys
// only its first one without; a password set by someone else ends the
// user's sessions.
func (s *Server) putUser(w http.ResponseWriter, r *http.Request) error {
	st, err := s.userStore()
	if err != nil {
		return err
	}
	u, a, err := s.targetUser(r, st, true, needPassword)
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
	// Without mayManage a principal changes its own name and password,
	// or with passwords another's password alone.
	manage := a.mayManage(u)
	if !manage && (req.Role != nil || req.Disabled != nil || len(req.ExpiresAt) > 0 || req.DisplayName != nil && !a.self) {
		return a.manageFail(u, "users")
	}
	// The roles' sets and the user's maximum, which a role given by a
	// holder of integrations is held to and the answer's may names, read
	// before anything is written or a proof used up; neither changes with
	// this request.
	roles, err := st.Roles(ctx)
	if err != nil {
		return userFail(err)
	}
	maxCaps, err := maxOf(ctx, st, u)
	if err != nil {
		return userFail(err)
	}
	if req.Role != nil {
		if r := roleOf(roles, *req.Role); !a.mayAssign(r, u, maxCaps) {
			return a.assignFail(r, u)
		}
	}
	setsPassword := req.Password != nil || req.Record != nil
	if req.Password != nil && *req.Password != "" || req.Record != nil {
		// Before any other change of the request is written.
		if err := noWayIn(u); err != nil {
			return err
		}
	}
	if setsPassword && !a.self && !a.maySetPassword(u) {
		return a.manageFail(u, "passwords")
	}
	if req.Password != nil && *req.Password != "" {
		// Before the current password is checked or a proof used up.
		names := []string{u.Name, u.DisplayName}
		if req.DisplayName != nil {
			names = append(names, *req.DisplayName)
		}
		if err := s.newPassword(r, *req.Password, names...); err != nil {
			return err
		}
	}
	if setsPassword && a.self && !a.all && u.HasPassword {
		// One's own password: the current one proves the caller is the
		// user, not someone at an unlocked browser. A user with keys
		// only has none, and sets its first one as itself: its key could
		// as well make another key.
		if req.Proof != "" {
			if !s.logins.redeemProof(req.Proof, u.ID, time.Now()) {
				return refuse(http.StatusForbidden, codePasswordProofWrong)
			}
		} else if err := st.CheckPassword(ctx, u.Name, req.CurrentPassword); err != nil {
			if errors.Is(err, store.ErrBadPassword) {
				return refuse(http.StatusForbidden, codePasswordCurrentWrong)
			}
			return passwordFail(err)
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
			// Asked again of the user as the write reads it: another change
			// since the check above, an owner's making it an owner, say,
			// leaves what the caller may change of it smaller.
			if (req.Role != nil || req.Disabled != nil || setEnd || req.DisplayName != nil && !a.self) && !a.mayManage(*c) {
				return a.manageFail(*c, "users")
			}
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
	if setsPassword {
		if req.Record != nil {
			rec, err := req.record()
			if err != nil {
				return err
			}
			err = st.SetOpaqueRecord(ctx, u.Name, rec, req.KSF, by)
			if err != nil {
				return passwordFail(err)
			}
		} else if err := st.SetPassword(ctx, u.Name, *req.Password, by); err != nil {
			return passwordFail(err)
		}
		if !a.self {
			if err := st.RevokeSessions(ctx, u.ID); err != nil {
				return userFail(err)
			}
		}
		s.log.Info("password set", "user", u.Name, "by_self", a.self, "remote", s.clientIP(r))
		if u, _, err = st.User(ctx, u.Name); err != nil {
			return userFail(err)
		}
	}
	s.log.Info("user changed", "user", s.logUserName(r.Context(), u), "remote", s.clientIP(r))
	auditSubject(r, userSubject(u))
	v := s.viewUserOf(r, st, nil, u)
	v.May = a.mayOf(u, a.self, roles, maxCaps)
	writeJSON(w, http.StatusOK, v)
	return nil
}

// integrationOwn reports whether u is an integration's own user, <uuid>:,
// which holds its key, or its system user, <uuid>:system, and which
// integration's. Removed, the integration's key went with the first and
// a request attached to the integration again could not collect a key;
// the integration's own removal (DELETE /api/v1/integrations/{uuid})
// keeps both, so that it can.
func integrationOwn(ctx context.Context, st *store.Store, u store.User) (store.Integration, bool, error) {
	if u.IntegrationID == 0 {
		return store.Integration{}, false, nil
	}
	i, ok, err := st.IntegrationByID(ctx, u.IntegrationID)
	if err != nil || !ok {
		return store.Integration{}, false, err
	}
	return i, u.ID == i.UserID || u.ID == i.SystemUserID, nil
}

// userSubject is a user as a change line names them: ids, login name,
// display name and role as they are then, never a secret.
func userSubject(u store.User) map[string]any {
	return map[string]any{"id": u.ID, "uuid": u.UUID, "name": u.Name, "display_name": u.DisplayName, "role": u.Role}
}

// tokenSubject is a key as a change line names it: its ids, its label and
// its user's name; never the key.
func tokenSubject(tok store.Token, u store.User) map[string]any {
	return map[string]any{"id": tok.ID, "uuid": tok.UUID, "name": tok.Label, "user": u.Name}
}

// deleteUser handles DELETE /api/v1/users/{name}: the user is removed
// and its tokens revoked; the last enabled owner stays, and so does the
// user that holds the settings file's key, 409 (settingsKey). An
// integration's own user and its system user stay, 403, for everyone
// (integrationOwn). Where the removal of a regular user turns the server
// open, the log warns.
func (s *Server) deleteUser(w http.ResponseWriter, r *http.Request) error {
	st, err := s.userStore()
	if err != nil {
		return err
	}
	u, _, err := s.targetUser(r, st, false)
	if err != nil {
		return err
	}
	if i, own, err := integrationOwn(r.Context(), st, u); err != nil {
		return userFail(err)
	} else if own {
		return refuse(http.StatusForbidden, codeUserIntegrationOwn, u.Name, i.UUID)
	}
	if file, ok, err := s.settingsKey(r.Context()); err != nil {
		return userFail(err)
	} else if ok && file.UserID == u.ID {
		return refuse(http.StatusConflict, codeUserHoldsSettingsKey, u.Name)
	}
	var by int64
	p := principalOf(r)
	if p.named() {
		by = p.user.ID
	}
	wasUsers := s.authMode(r.Context()) == modeUsers
	if err := st.DeleteUser(r.Context(), u.Name, by); err != nil {
		return userFail(err)
	}
	s.log.Info("user removed", "user", s.logUserName(r.Context(), u), "remote", s.clientIP(r))
	if wasUsers && u.IntegrationID == 0 && s.authMode(r.Context()) == modeOpen {
		// The last regular user gone, an owner with an end date removing
		// itself among the ways (AP LooseEnds, D1): the server is open.
		who := "nobody"
		if p.named() {
			who = s.logUserName(r.Context(), p.user)
		}
		s.log.Warn("the server is open: its last regular user was removed; create the owner", "user", s.logUserName(r.Context(), u), "by", who, "remote", s.clientIP(r))
	}
	auditSubject(r, userSubject(u)) // as the user was
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "name": u.Name})
	return nil
}

// listTokens handles GET /api/v1/users/{name}/tokens: the live sessions
// and keys of the user, the settings file's key marked from_settings.
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
	file, held, err := s.settingsKey(r.Context())
	if err != nil {
		return userFail(err)
	}
	out := []tokenView{}
	for _, t := range tokens {
		v := viewToken(t)
		v.FromSettings = held && t.ID == file.ID
		v.IdleExempt = v.IdleExempt || v.FromSettings // the store passes over it by the hash Bootstrap applied (D3-A)
		out = append(out, v)
	}
	writeJSON(w, http.StatusOK, out)
	return nil
}

// postToken handles POST /api/v1/users/{name}/tokens: a new API key,
// shown once. Where the bearer is an API key, the new key names it as
// its maker, and a revoke of that key revokes it (AP KeyLife, D5-A); a
// key made by a session of any kind, one made from a key by
// POST /api/v1/login/key among them, names none. idle_exempt true keeps
// the key from the idle limit, for who manages its user alone (AP
// KeyLife, D7-A; keptFail); expires_at, an RFC 3339 time in the future,
// gives it an end date, null or absent none (Block 4; endFail).
func (s *Server) postToken(w http.ResponseWriter, r *http.Request) error {
	st, err := s.userStore()
	if err != nil {
		return err
	}
	u, a, err := s.targetUser(r, st, true)
	if err != nil {
		return err
	}
	var req struct {
		Label      string          `json:"label"`
		IdleExempt bool            `json:"idle_exempt"`
		ExpiresAt  json.RawMessage `json:"expires_at"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return fail(http.StatusBadRequest, "bad body: %v", err)
	}
	req.Label = strings.TrimSpace(req.Label)
	if req.Label == "" || len(req.Label) > maxLabel {
		return refuse(http.StatusBadRequest, codeTokenLabelLength, maxLabel)
	}
	end, _, err := expiresOf(req.ExpiresAt)
	if err != nil {
		return err
	}
	if err := noWayIn(u); err != nil {
		return err
	}
	if req.IdleExempt && !a.mayManage(u) {
		return keptFail(u)
	}
	opts := store.KeyOptions{Label: req.Label, IdleExempt: req.IdleExempt, Expires: end}
	if p := principalOf(r); p.named() && p.token.Kind == store.TokenAPI {
		opts.Maker = p.token // held to the secret it was let in with
	}
	tok, secret, err := st.AddKey(r.Context(), u.ID, opts)
	if opts.Maker.ID != 0 && errors.Is(err, store.ErrNoToken) {
		// The bearer, let in a moment before, was revoked, ended or given
		// a new secret since: no key is made, as none would have been made
		// after it; refused in errBadToken's words, as a key revoked
		// meanwhile makes no session (openSessionFrom).
		return refuse(http.StatusUnauthorized, codeTokenWrong)
	}
	if err != nil {
		return userFail(endFail(err, end))
	}
	auditTarget(r, "token:"+strconv.FormatInt(tok.ID, 10))
	auditSubject(r, tokenSubject(tok, u))
	line := []any{"user", s.logUserName(r.Context(), u), "label", req.Label, "remote", s.clientIP(r)}
	if tok.MadeBy != 0 {
		line = append(line, "made_by", tok.MadeBy)
	}
	if opts.IdleExempt {
		line = append(line, "idle_exempt", true)
	}
	if !tok.ExpiresAt.IsZero() {
		line = append(line, "expires_at", tok.ExpiresAt)
	}
	s.log.Info("api key created", line...)
	v := viewToken(tok)
	writeJSON(w, http.StatusCreated, map[string]any{"token": secret, "id": v.ID, "kind": v.Kind, "label": v.Label, "created_at": v.CreatedAt})
	return nil
}

// deleteToken handles DELETE /api/v1/users/{name}/tokens/{id}: the
// session or key is revoked, a key's heirs with it, which its answer, its
// log line and its change line name (revoked). The settings file's key
// is 409, as DELETE /api/v1/me/token answers it (settingsKey).
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
	if !ok || tok.UserID != u.ID || !tok.DeletedAt.IsZero() {
		return refuse(http.StatusNotFound, codeTokenUnknown, u.Name, id)
	}
	file, held, err := s.settingsKey(r.Context())
	if err != nil {
		return userFail(err)
	}
	if held && id == file.ID {
		return refuse(http.StatusConflict, codeTokenSettingsKey)
	}
	heirs, err := st.RevokeToken(r.Context(), id)
	if err != nil {
		return userFail(err)
	}
	s.revoked(w, r, tok, u, heirs, "user", s.logUserName(r.Context(), u), "token", id, "kind", tok.Kind, "remote", s.clientIP(r))
	return nil
}

// putToken handles PUT /api/v1/users/{name}/tokens/{id}: an API key's
// life, {"idle_exempt": true|false} (AP KeyLife, Block 3) and
// {"expires_at": "<RFC 3339>"|null} (Block 4), answered with the key's
// view; a field left out keeps what the key has, and a body of none is
// 400. A session is 400; the settings file's key 409, its life being the
// file's (D3-A); the bearer that very key 403, since a key changes
// nothing of its own life, a session made from it not being the key
// (D4-A); idle_exempt from a caller that does not manage the key's user
// 403 (D7-A, keptFail), and true for an integration's key 400 (AP
// MobileApp, D75); expires_at from the key's user or who manages it, as
// targetUser lets in, an integration's key's from a holder of users or
// integrations, a time that has passed 400 (endFail), null removing the
// end date; another user's token 404. Audited as token.update, its
// change line from and to.
func (s *Server) putToken(w http.ResponseWriter, r *http.Request) error {
	st, err := s.userStore()
	if err != nil {
		return err
	}
	u, a, err := s.targetUser(r, st, true)
	if err != nil {
		return err
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		return fail(http.StatusBadRequest, "bad token id %q", r.PathValue("id"))
	}
	var req struct {
		IdleExempt *bool           `json:"idle_exempt"`
		ExpiresAt  json.RawMessage `json:"expires_at"`
	}
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		return fail(http.StatusBadRequest, "bad body: %v (want {\"idle_exempt\": true} or false, {\"expires_at\": \"2026-12-24T18:00:00Z\"} or null)", err)
	}
	end, setEnd, err := expiresOf(req.ExpiresAt)
	if err != nil {
		return err
	}
	if req.IdleExempt == nil && !setEnd {
		return fail(http.StatusBadRequest, "nothing to change: want idle_exempt or expires_at")
	}
	tok, ok, err := st.Token(r.Context(), id)
	if err != nil {
		return userFail(err)
	}
	if !ok || tok.UserID != u.ID || !tok.DeletedAt.IsZero() {
		return refuse(http.StatusNotFound, codeTokenUnknown, u.Name, id)
	}
	if tok.Kind != store.TokenAPI {
		return refuse(http.StatusBadRequest, codeKeyLifeSession, id)
	}
	file, held, err := s.settingsKey(r.Context())
	if err != nil {
		return userFail(err)
	}
	if held && id == file.ID {
		return refuse(http.StatusConflict, codeTokenSettingsKey)
	}
	if p := principalOf(r); p.named() && p.token.ID == id {
		return refuse(http.StatusForbidden, codeKeyLifeOwnKey)
	}
	// Each field's own rule; the exclusion is who manages the user's alone.
	if req.IdleExempt != nil {
		if !a.mayManage(u) {
			return keptFail(u)
		}
		if *req.IdleExempt && u.IntegrationID != 0 {
			return refuse(http.StatusBadRequest, codeKeyLifeIntegrationKept)
		}
	}
	// The end date is the key's user's or who manages it, as targetUser let
	// in, an integration's key's a holder of users' or integrations'
	// (D4-A): nothing more to ask of the caller.
	life := store.TokenLife{IdleExempt: req.IdleExempt}
	if setEnd {
		life.Expires = &end
	}
	auditTarget(r, "token:"+strconv.FormatInt(id, 10))
	auditSubject(r, tokenSubject(tok, u))
	got, err := st.SetTokenLife(r.Context(), id, life)
	if errors.Is(err, store.ErrSettingsKey) {
		return refuse(http.StatusConflict, codeTokenSettingsKey) // the file read again since the check above
	}
	if err != nil {
		return userFail(endFail(err, end))
	}
	line := []any{"user", s.logUserName(r.Context(), u), "token", id, "remote", s.clientIP(r)}
	if req.IdleExempt != nil {
		auditChange(r, "idle_exempt", tok.IdleExempt, got.IdleExempt)
		line = append(line, "idle_exempt", got.IdleExempt)
	}
	if setEnd {
		auditChange(r, "expires_at", optTime(tok.ExpiresAt), optTime(got.ExpiresAt))
		if got.ExpiresAt.IsZero() {
			line = append(line, "expires_at", "none")
		} else {
			line = append(line, "expires_at", got.ExpiresAt)
		}
	}
	s.log.Info("token changed", line...)
	writeJSON(w, http.StatusOK, viewToken(got))
	return nil
}

// rekeyToken handles POST /api/v1/users/{name}/tokens/{id}/rekey: a live
// API key gets a new secret, shown once in the answer, as POST …/tokens
// answers, {"token", "id", "kind", "label", "created_at"} (AP KeyLife,
// Block 5; MobileApp.D76-A, D81-A); its id, label, end date, exclusion,
// maker and heirs stay (store.RekeyToken). The old secret is answered as
// an ended token, not counted toward the lockout, and the key's session
// ends unless it is the caller's own, made from that key. Who may: the
// key's user or who manages it, as targetUser lets in, as for its end
// date. A session is 400; the settings file's key 409, since the next
// start would not add the file's token back, its hash being taken; an
// integration's key 403, the integration learning its key by its
// approval's code alone; the bearer that very key 403, a key changing
// nothing of its own life (D4-A); an ended key 409; another user's token
// 404. The command keeps the answer without token (withoutSecrets), and a
// retry with its Idempotency-Key answers without it and gives no other.
// Audited as token.rekey, with a change line and an Info line.
func (s *Server) rekeyToken(w http.ResponseWriter, r *http.Request) error {
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
	if !ok || tok.UserID != u.ID || !tok.DeletedAt.IsZero() {
		return refuse(http.StatusNotFound, codeTokenUnknown, u.Name, id)
	}
	if tok.Kind != store.TokenAPI {
		return refuse(http.StatusBadRequest, codeNewSecretSession, id)
	}
	file, held, err := s.settingsKey(r.Context())
	if err != nil {
		return userFail(err)
	}
	if held && id == file.ID {
		return refuse(http.StatusConflict, codeTokenSettingsKey)
	}
	if u.IntegrationID != 0 {
		return refuse(http.StatusForbidden, codeNewSecretIntegration)
	}
	p := principalOf(r)
	if p.named() && p.token.ID == id {
		return refuse(http.StatusForbidden, codeNewSecretOwnKey)
	}
	if _, live, err := st.TokenAlive(r.Context(), id); err != nil {
		return userFail(err)
	} else if !live {
		return refuse(http.StatusConflict, codeNewSecretEnded, id)
	}
	// The caller's own session, made from this key, stays.
	var keep int64
	if p.named() && p.token.Kind == store.TokenSession && p.token.ParentID == id {
		keep = p.token.ID
	}
	auditTarget(r, "token:"+strconv.FormatInt(id, 10))
	auditSubject(r, tokenSubject(tok, u))
	got, secret, err := st.RekeyToken(r.Context(), id, keep)
	if errors.Is(err, store.ErrSettingsKey) {
		return refuse(http.StatusConflict, codeTokenSettingsKey) // the file read again since the check above
	}
	if errors.Is(err, store.ErrNoToken) {
		// Ended or revoked since it was read.
		return refuse(http.StatusConflict, codeNewSecretEnded, id)
	}
	if err != nil {
		return userFail(err)
	}
	line := []any{"user", s.logUserName(r.Context(), u), "token", id, "remote", s.clientIP(r)}
	if keep != 0 {
		line = append(line, "kept_session", keep)
	}
	s.log.Info("api key given a new secret", line...)
	v := viewToken(got)
	writeJSON(w, http.StatusOK, map[string]any{"token": secret, "id": v.ID, "kind": v.Kind, "label": v.Label, "created_at": v.CreatedAt})
	return nil
}

// keptFail is the refusal of idle_exempt, a key kept from the idle limit
// or held to it again, to a caller that does not manage the key's user u
// (AP KeyLife, D7-A): the limit is the policy of who sets it, and a user
// who manages nobody keeps it on its own keys.
func keptFail(u store.User) error {
	return refuse(http.StatusForbidden, codeKeyKeepForbidden, u.Name)
}

// revoked answers a revoke of tok, a token of u, that took heirs with it
// (store.RevokeToken): {"ok", "id", "heirs"}, the heirs [] for none, and
// its Info line of line's attributes, the heirs added where it took any;
// the change line's subject names them too (AP KeyLife, D5-A).
func (s *Server) revoked(w http.ResponseWriter, r *http.Request, tok store.Token, u store.User, heirs []int64, line ...any) {
	subject := tokenSubject(tok, u)
	if len(heirs) > 0 {
		line = append(line, "heirs", heirs)
		subject["heirs"] = heirs
	}
	s.log.Info("token revoked", line...)
	auditSubject(r, subject)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "id": tok.ID, "heirs": heirs})
}
