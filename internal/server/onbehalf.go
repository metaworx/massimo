// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"regexp"
	"slices"
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
// integration and its maximum.

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
// integration can tell apart: bad_header, not_integration,
// integration_disabled, instance_pending, user_disabled.
type onBehalfRefusal struct {
	status int
	code   string
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

// integrationPrincipal is the principal of a request with an
// integration's key: the integration itself, or with the header the user
// it acts for, with the capabilities the chain leaves. A key that is no
// integration's comes back unchanged, unless the request names a user.
func (s *Server) integrationPrincipal(ctx context.Context, r *http.Request, p *principal) (*principal, *onBehalfRefusal) {
	h := r.Header.Get(OnBehalfHeader)
	st := s.opts.Store
	if st == nil || !p.named() {
		if h != "" {
			return nil, &onBehalfRefusal{http.StatusForbidden, "not_integration", "this key is no registered integration's; it acts for nobody"}
		}
		return p, nil
	}
	integ, ok, err := st.IntegrationOfUser(ctx, p.user.ID)
	if err != nil {
		return nil, &onBehalfRefusal{http.StatusInternalServerError, "", err.Error()}
	}
	if !ok {
		if h != "" {
			return nil, &onBehalfRefusal{http.StatusForbidden, "not_integration", "this key is no registered integration's; it acts for nobody"}
		}
		return p, nil
	}
	if integ.State != store.IntegrationActive {
		return nil, &onBehalfRefusal{http.StatusForbidden, "integration_disabled", "the integration " + integ.Key + " is disabled"}
	}
	caps := intersect(p.caps, integ.MaxCaps)
	if h == "" {
		q := *p
		q.caps, q.integration = caps, &integ
		return &q, nil
	}
	ob, err := parseOnBehalf(h)
	if err != nil {
		return nil, &onBehalfRefusal{http.StatusBadRequest, "bad_header", OnBehalfHeader + ": " + err.Error()}
	}
	if ob.instance != integ.InstanceID {
		if err := st.NotePendingInstance(ctx, integ.ID, ob.instance); err != nil {
			s.log.Error("api: an integration's new installation id not recorded", "integration", integ.Key, "error", err)
		}
		return nil, &onBehalfRefusal{http.StatusForbidden, "instance_pending", "the call came from another installation than the one accepted; the owner accepts it in the integration's settings"}
	}
	id, err := st.IntegrationUser(ctx, integ, store.IntegrationSeen{Subject: ob.id, Name: ob.name, SystemGenerated: ob.systemGenerated, Level: ob.level})
	if err != nil {
		return nil, &onBehalfRefusal{http.StatusInternalServerError, "", err.Error()}
	}
	now := time.Now()
	own, ok, err := st.UserByID(ctx, id.OwnUserID)
	if err != nil || !ok {
		return nil, &onBehalfRefusal{http.StatusInternalServerError, "", "the integration's user is missing"}
	}
	if own.Disabled || own.Expired(now) {
		return nil, &onBehalfRefusal{http.StatusForbidden, "user_disabled", "the integration's user " + own.Name + " is disabled"}
	}
	ownCaps, off, err := st.Effective(ctx, own.Role, capabilities)
	if err != nil {
		return nil, &onBehalfRefusal{http.StatusInternalServerError, "", err.Error()}
	}
	if off {
		return nil, &onBehalfRefusal{http.StatusForbidden, "user_disabled", "the role " + own.Role + " is disabled"}
	}
	// 1 and 2: the user's role within the ceiling; 4 is in caps already.
	caps = intersect(caps, ownCaps)
	acting := own
	if id.Linked() {
		linked, ok, err := st.UserByID(ctx, id.UserID)
		if err != nil || !ok {
			return nil, &onBehalfRefusal{http.StatusInternalServerError, "", "the linked user is missing"}
		}
		if linked.Disabled || linked.Expired(now) {
			return nil, &onBehalfRefusal{http.StatusForbidden, "user_disabled", "the linked user " + linked.Name + " is disabled"}
		}
		linkedCaps, off, err := st.Effective(ctx, linked.Role, capabilities)
		if err != nil {
			return nil, &onBehalfRefusal{http.StatusInternalServerError, "", err.Error()}
		}
		if off {
			return nil, &onBehalfRefusal{http.StatusForbidden, "user_disabled", "the role " + linked.Role + " is disabled"}
		}
		caps = intersect(caps, linkedCaps) // 3
		acting = linked
	}
	if allowed, bounded := levelCaps[ob.level]; bounded {
		caps = intersect(caps, allowed) // 5
	}
	q := &principal{mode: p.mode, user: acting, token: p.token, caps: caps, integration: &integ, actsFor: &id}
	return q, nil
}

// integrationCaps are the capabilities left in a principal's set, sorted:
// for GET /api/v1/me.
func integrationCaps(p *principal) []string {
	var out []string
	for c, ok := range p.caps {
		if ok {
			out = append(out, c)
		}
	}
	slices.Sort(out)
	return out
}
