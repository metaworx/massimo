// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"go.metaworx.ch/massimo/internal/store"
)

// Acting for an integration's users (docs/api-v1.md, "Acting for an
// integration's users"): a registered integration's key, with the header
// X-MassimoHA-On-Behalf-Of, acts for one of the integration's users, the
// one the header names, at the level of that call. What the request may do
// is what every link of the chain allows:
//
//  1. the integration user's role (made with the role integration at its
//     first call, narrowed by the owner where wanted),
//  2. the role integration, the ceiling of every call through an
//     integration,
//  3. the Massimo user the integration user is linked to, where it is,
//  4. the integration's maximum, set when the owner approved it,
//  5. the call's level: view reads, control also sets, locks and
//     configures, admin adds nothing of its own.
//
// Without the header the key acts as the integration itself: its role
// integration and its maximum. What the set the chain leaves includes
// (auth.go, includes) is taken at the check, after every link, so that
// a maximum with update and without restart passes restart's check.

// OnBehalfHeader names the user an integration's call acts for.
const OnBehalfHeader = "X-MassimoHA-On-Behalf-Of"

// The levels of a call through an integration.
const (
	LevelView    = "view"
	LevelControl = "control"
	LevelAdmin   = "admin"
)

// levelCaps is what each level allows; admin is not bounded here.
var levelCaps = map[string][]string{
	LevelView:    {"read", "preferences"},
	LevelControl: {"read", "preferences", "setpoint", "lock", "configure"},
}

// onBehalf is what the header says.
type onBehalf struct {
	id, level, name, instance string
	systemGenerated           bool
}

// subjectID is a user's id at Home Assistant: 32 hex digits.
var subjectID = regexp.MustCompile(`^[0-9a-f]{32}$`)

// parseOnBehalf reads the header: parameters separated by ";", each
// key=value, a value quoted or not; name* in the extended notation of
// RFC 8187 (UTF-8”percent-encoded); system_generated alone.
func parseOnBehalf(h string) (onBehalf, error) {
	var ob onBehalf
	for _, part := range strings.Split(h, ";") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		key, value, hasValue := strings.Cut(part, "=")
		key = strings.ToLower(strings.TrimSpace(key))
		value = strings.TrimSpace(value)
		if len(value) >= 2 && value[0] == '"' && value[len(value)-1] == '"' {
			value = strings.ReplaceAll(value[1:len(value)-1], `\"`, `"`)
		}
		switch key {
		case "id":
			ob.id = strings.ToLower(value)
		case "level":
			ob.level = strings.ToLower(value)
		case "instance":
			ob.instance = strings.ToLower(value)
		case "name":
			ob.name = value
		case "name*":
			charset, rest, ok := strings.Cut(value, "'")
			if !ok || !strings.EqualFold(charset, "utf-8") {
				return onBehalf{}, errors.New("name* must be UTF-8''<percent-encoded>")
			}
			_, encoded, ok := strings.Cut(rest, "'")
			if !ok {
				return onBehalf{}, errors.New("name* must be UTF-8''<percent-encoded>")
			}
			name, err := url.PathUnescape(encoded)
			if err != nil {
				return onBehalf{}, errors.New("name*: bad percent-encoding")
			}
			ob.name = name
		case "system_generated":
			ob.systemGenerated = !hasValue || value == "1" || strings.EqualFold(value, "true")
		}
	}
	switch {
	case ob.id != "system" && !subjectID.MatchString(ob.id):
		return onBehalf{}, errors.New("id must be the user's id (32 hex digits) or system")
	case ob.level != LevelView && ob.level != LevelControl && ob.level != LevelAdmin:
		return onBehalf{}, errors.New("level must be view, control or admin")
	case !store.ValidInstanceID(ob.instance):
		return onBehalf{}, errors.New("instance must be the installation id (32 hex digits)")
	case len(ob.name) > 128:
		return onBehalf{}, errors.New("name is at most 128 characters")
	}
	return ob, nil
}

// onBehalfRefusal is a call through an integration refused, with a code the
// integration can tell apart (errcodes.go): bad_header, not_integration,
// integration_disabled, instance_pending, user_disabled; nil for an error
// of ours.
type onBehalfRefusal struct {
	status int
	code   *errCode
	msg    string
}

// intersect keeps the capabilities of set that keep holds.
func intersect(set map[string]bool, keep []string) map[string]bool {
	out := map[string]bool{}
	for _, c := range keep {
		if set[c] {
			out[c] = true
		}
	}
	return out
}

// withinIntegration is what links 2 and 4 of the chain leave of caps: the
// role integration, the ceiling of every call through integ whatever role
// its user was given, and integ's maximum. Every principal that comes
// through an integration is bounded by it, a request's and a stream's at
// each keep-alive alike (principalFor, integrationPrincipal, stillActsFor).
func withinIntegration(caps map[string]bool, integ store.Integration) map[string]bool {
	return intersect(intersect(caps, integrationCeiling), integ.MaxCaps)
}

// integrationPrincipal is the principal of a request with an
// integration's key: the integration itself, or with the header the user
// it acts for, with the capabilities the chain leaves and the
// installation id the call came from. A key that is no integration's
// comes back unchanged, unless the request names a user.
func (s *Server) integrationPrincipal(ctx context.Context, r *http.Request, p *principal) (*principal, *onBehalfRefusal) {
	h := r.Header.Get(OnBehalfHeader)
	st := s.opts.Store
	if st == nil || !p.named() {
		if h != "" {
			return nil, &onBehalfRefusal{http.StatusForbidden, codeNotIntegration, "this key is no registered integration's; it acts for nobody"}
		}
		return p, nil
	}
	integ, ok, err := st.IntegrationOfUser(ctx, p.user.ID)
	if err != nil {
		return nil, &onBehalfRefusal{http.StatusInternalServerError, nil, err.Error()}
	}
	if !ok {
		if h != "" {
			return nil, &onBehalfRefusal{http.StatusForbidden, codeNotIntegration, "this key is no registered integration's; it acts for nobody"}
		}
		return p, nil
	}
	if integ.State != store.IntegrationActive {
		return nil, &onBehalfRefusal{http.StatusForbidden, codeIntegrationDisabled, "the integration " + integ.UUID + " is disabled"}
	}
	// 2 and 4: the role integration bounds every call through the
	// integration, whatever role its own user was given, and so does its
	// maximum.
	caps := withinIntegration(p.caps, integ)
	if h == "" {
		q := *p
		q.caps, q.integration = caps, &integ
		return &q, nil
	}
	ob, err := parseOnBehalf(h)
	if err != nil {
		return nil, &onBehalfRefusal{http.StatusBadRequest, codeBadHeader, OnBehalfHeader + ": " + err.Error()}
	}
	if ob.instance != integ.InstanceID {
		if err := st.NotePendingInstance(ctx, integ.ID, ob.instance); err != nil {
			s.log.Error("api: an integration's new installation id not recorded", "integration", integ.UUID, "error", err)
		}
		return nil, &onBehalfRefusal{http.StatusForbidden, codeInstancePending, "the call came from another installation than the one accepted; the owner accepts it in the integration's settings"}
	}
	id, err := st.IntegrationUser(ctx, integ, store.IntegrationSeen{Subject: ob.id, Name: ob.name, SystemGenerated: ob.systemGenerated, Level: ob.level})
	if err != nil {
		return nil, &onBehalfRefusal{http.StatusInternalServerError, nil, err.Error()}
	}
	q, refused := s.actingFor(ctx, p, caps, &integ, &id, ob.level, time.Now())
	if refused != nil {
		return nil, refused
	}
	q.instance = ob.instance
	return q, nil
}

// actingFor is the principal of a call through integration integ for its
// user id, at level, the rest of the chain once the identity is known:
// p is the key's principal and caps what links 2 and 4 left of it; the
// integration user's own user (1) and the Massimo user it is linked to
// (3), each enabled, not past its end and of a role that is not disabled,
// narrow it, and then the level (5). It writes nothing, so that an event
// stream asks it again at each keep-alive (stillAdmitted).
func (s *Server) actingFor(ctx context.Context, p *principal, caps map[string]bool, integ *store.Integration, id *store.IntegrationIdentity,
	level string, now time.Time) (*principal, *onBehalfRefusal) {
	st := s.opts.Store
	own, ok, err := st.UserByID(ctx, id.RealUserID)
	if err != nil || !ok {
		return nil, &onBehalfRefusal{http.StatusInternalServerError, nil, "the integration's user is missing"}
	}
	if own.Disabled || own.Expired(now) {
		return nil, &onBehalfRefusal{http.StatusForbidden, codeUserDisabled, "the integration's user " + own.Name + " is disabled"}
	}
	ownCaps, ownOff, roleOff, err := st.Effective(ctx, own.Role, capabilities)
	if err != nil {
		return nil, &onBehalfRefusal{http.StatusInternalServerError, nil, err.Error()}
	}
	if roleOff {
		return nil, &onBehalfRefusal{http.StatusForbidden, codeUserDisabled, "the role " + own.Role + " is disabled"}
	}
	// 1: the user's role; 2 and 4 are in caps already. The capabilities
	// disabled for every role that any role of the chain left out are
	// never included.
	caps = intersect(caps, ownCaps)
	off := union(p.off, ownOff)
	acting := own
	if id.Linked() {
		linked, ok, err := st.UserByID(ctx, id.EffectiveUserID)
		if err != nil || !ok {
			return nil, &onBehalfRefusal{http.StatusInternalServerError, nil, "the linked user is missing"}
		}
		if linked.Disabled || linked.Expired(now) {
			return nil, &onBehalfRefusal{http.StatusForbidden, codeUserDisabled, "the linked user " + linked.Name + " is disabled"}
		}
		linkedCaps, linkedOff, roleOff, err := st.Effective(ctx, linked.Role, capabilities)
		if err != nil {
			return nil, &onBehalfRefusal{http.StatusInternalServerError, nil, err.Error()}
		}
		if roleOff {
			return nil, &onBehalfRefusal{http.StatusForbidden, codeUserDisabled, "the role " + linked.Role + " is disabled"}
		}
		caps = intersect(caps, linkedCaps) // 3
		off = union(off, linkedOff)
		acting = linked
	}
	if allowed, bounded := levelCaps[level]; bounded {
		caps = intersect(caps, allowed) // 5
	}
	q := &principal{mode: p.mode, user: acting, token: p.token, caps: caps, off: off, integration: integ, actsFor: id}
	return q, nil
}

// stillActsFor reports whether a call through an integration that p, its
// principal, was let in with would be let in now, read again without a
// write: np is the principal of its key now (principalFor). Its
// integration is still the key's and active, the installation id the
// call came from still the accepted one, its identity still acts for
// someone (store.IntegrationIdentityByID), as the same user and of the
// same own user, and the chain at the call's level still holds read, so
// that the removal, a disabling or the end of either user, a role
// disabled, an unlink or a link end the stream (AP LinkedRemoval,
// Block 1), and so does another installation id accepted (D4-A).
func (s *Server) stillActsFor(ctx context.Context, p, np *principal, now time.Time) bool {
	st := s.opts.Store
	integ, ok, err := st.IntegrationOfUser(ctx, np.user.ID)
	if err != nil || !ok || integ.State != store.IntegrationActive || p.integration == nil || integ.ID != p.integration.ID {
		return false
	}
	if p.instance != integ.InstanceID {
		// As integrationPrincipal refuses a request of that installation,
		// without recording it pending: a keep-alive writes nothing.
		return false
	}
	id, ok, err := st.IntegrationIdentityByID(ctx, p.actsFor.ID)
	if err != nil || !ok || id.EffectiveUserID != p.user.ID || id.RealUserID != p.actsFor.RealUserID {
		return false
	}
	caps := withinIntegration(np.caps, integ)
	q, refused := s.actingFor(ctx, np, caps, &integ, &id, p.actsFor.LastLevel, now)
	return refused == nil && q.has("read")
}

// union is a set of capabilities with those of more added.
func union(set map[string]bool, more []string) map[string]bool {
	out := setOf(more)
	for c, ok := range set {
		if ok {
			out[c] = true
		}
	}
	return out
}
