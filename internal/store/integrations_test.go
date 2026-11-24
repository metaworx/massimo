// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package store

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

const (
	instanceA = "0123456789abcdef0123456789abcdef"
	instanceB = "fedcba9876543210fedcba9876543210"
)

func haRequest(instance string) IntegrationRequest {
	return IntegrationRequest{Kind: KindHomeAssistant, Name: "Casa Leone", InstanceID: instance, URL: "http://ha.example:8123", AppVersion: "2026.11.1", Remote: "192.0.2.7"}
}

// A request waits, is approved with its maximum, and its key is collected
// with its code: again while unused, never after its first use.
func TestIntegrationRegistration(t *testing.T) {
	t.Parallel()
	s := open(t, t.TempDir())
	ctx := context.Background()
	req, err := s.AddIntegrationRequest(ctx, haRequest(instanceA), "K7Q2PX")
	if err != nil || req.State != IntegrationPending {
		t.Fatalf("request %+v %v", req, err)
	}
	if _, _, err := s.CollectIntegrationKey(ctx, req.UUID, "K7Q2PX"); !errors.Is(err, ErrRequestPending) {
		t.Fatalf("collect while waiting: %v", err)
	}
	if _, _, err := s.CollectIntegrationKey(ctx, req.UUID, "WRONG1"); !errors.Is(err, ErrNoIntegration) {
		t.Fatalf("a wrong code: %v", err)
	}
	i, err := s.ApproveIntegration(ctx, req.UUID, []string{"setpoint", "read"}, "", 0)
	if err != nil || i.State != IntegrationActive || i.UserID == 0 || i.SystemUserID == 0 || strings.Join(i.MaxCaps, ",") != "read,setpoint" {
		t.Fatalf("approved %+v %v", i, err)
	}
	own, _, _ := s.UserByID(ctx, i.UserID)
	sys, _, _ := s.UserByID(ctx, i.SystemUserID)
	if own.Name != i.UUID+":" || own.DisplayName != "Casa Leone" || own.Role != RoleIntegration || own.HasPassword || own.IntegrationID != i.ID ||
		sys.Name != i.UUID+":system" || sys.IntegrationID != i.ID {
		t.Fatalf("its users %+v %+v", own, sys)
	}
	k1, got, err := s.CollectIntegrationKey(ctx, req.UUID, "K7Q2PX")
	if err != nil || k1 == "" || got.UUID != i.UUID {
		t.Fatalf("collect %q %+v %v", k1, got, err)
	}
	// The answer lost: the same code gives a new key, the first revoked.
	k2, _, err := s.CollectIntegrationKey(ctx, req.UUID, "K7Q2PX")
	if err != nil || k2 == k1 {
		t.Fatalf("collect again %v", err)
	}
	if tok, ok, _ := s.TokenByHash(ctx, HashToken(k1)); ok && tok.DeletedAt.IsZero() {
		t.Fatal("the first key still valid")
	}
	tok, _, _ := s.TokenByHash(ctx, HashToken(k2))
	if err := s.TouchToken(ctx, tok.ID, "192.0.2.7", time.Time{}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.CollectIntegrationKey(ctx, req.UUID, "K7Q2PX"); !errors.Is(err, ErrRequestGone) {
		t.Fatalf("collect after the key's use: %v", err)
	}
	if byUser, ok, _ := s.IntegrationOfUser(ctx, i.UserID); !ok || byUser.UUID != i.UUID {
		t.Fatal("the integration by its user")
	}

	// A second request from the same installation, attached to the first
	// after a revoke: the same integration, users and maximum.
	if err := s.RevokeIntegration(ctx, i.UUID, 0); err != nil {
		t.Fatal(err)
	}
	if tok, ok, _ := s.TokenByHash(ctx, HashToken(k2)); ok && tok.DeletedAt.IsZero() {
		t.Fatal("the key outlived the revoke")
	}
	again, _ := s.AddIntegrationRequest(ctx, haRequest(instanceA), "Z9Z9Z9")
	back, err := s.ApproveIntegration(ctx, again.UUID, nil, i.UUID, 0)
	if err != nil || back.UUID != i.UUID || back.State != IntegrationActive || back.UserID != i.UserID || strings.Join(back.MaxCaps, ",") != "read,setpoint" {
		t.Fatalf("attached %+v %v", back, err)
	}
	if k, got, err := s.CollectIntegrationKey(ctx, again.UUID, "Z9Z9Z9"); err != nil || k == "" || got.UUID != i.UUID {
		t.Fatalf("the attached request's key: %+v %v", got, err)
	}
	list, _ := s.Integrations(ctx)
	if len(list) != 1 || list[0].UUID != i.UUID {
		t.Fatalf("integrations %+v", list)
	}

	// Another installation id seen, then accepted.
	if err := s.NotePendingInstance(ctx, i.ID, instanceB); err != nil {
		t.Fatal(err)
	}
	if acc, err := s.AcceptInstance(ctx, i.UUID, 0); err != nil || acc.InstanceID != instanceB || acc.PendingInstance != "" {
		t.Fatalf("accepted %+v %v", acc, err)
	}
}

// Refused and too many requests; a second integration of the same name
// gets users of its own, named by its uuid.
func TestIntegrationRequestsBounded(t *testing.T) {
	t.Parallel()
	s := open(t, t.TempDir())
	ctx := context.Background()
	r, _ := s.AddIntegrationRequest(ctx, haRequest(instanceA), "AAAAAA")
	if err := s.RefuseIntegration(ctx, r.UUID, 0); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.CollectIntegrationKey(ctx, r.UUID, "AAAAAA"); !errors.Is(err, ErrRequestGone) {
		t.Fatalf("a refused request: %v", err)
	}
	if _, err := s.ApproveIntegration(ctx, r.UUID, nil, "", 0); !errors.Is(err, ErrNotPending) {
		t.Fatalf("approving a refused one: %v", err)
	}
	for n := 0; n < MaxPendingRequests; n++ {
		if _, err := s.AddIntegrationRequest(ctx, haRequest(instanceB), "BBBBBB"); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.AddIntegrationRequest(ctx, haRequest(instanceB), "BBBBBB"); !errors.Is(err, ErrTooManyRequests) {
		t.Fatalf("one too many: %v", err)
	}
	list, _ := s.Integrations(ctx)
	a, _ := s.ApproveIntegration(ctx, list[0].UUID, nil, "", 0)
	b, _ := s.ApproveIntegration(ctx, list[1].UUID, nil, "", 0)
	ua, _, _ := s.UserByID(ctx, a.UserID)
	ub, _, _ := s.UserByID(ctx, b.UserID)
	if a.Name != b.Name || ua.Name != a.UUID+":" || ub.Name != b.UUID+":" {
		t.Fatalf("users %q %q", ua.Name, ub.Name)
	}
	if _, err := s.AddIntegrationRequest(ctx, IntegrationRequest{Kind: KindHomeAssistant, InstanceID: "short"}, "C"); !errors.Is(err, ErrBadInstance) {
		t.Fatalf("a bad installation id: %v", err)
	}
}

// An integration's users are made at their first call, named by the
// integration's uuid and their id, which must make such a name; a later
// call updates the name and the level, and the display name of its user
// while that is the name reported before (the copy rule).
func TestIntegrationUsers(t *testing.T) {
	t.Parallel()
	s := open(t, t.TempDir())
	ctx := context.Background()
	req, _ := s.AddIntegrationRequest(ctx, haRequest(instanceA), "AAAAAA")
	i, _ := s.ApproveIntegration(ctx, req.UUID, nil, "", 0)
	a, err := s.IntegrationUser(ctx, i, IntegrationSeen{Subject: "12345678aaaaaaaaaaaaaaaaaaaaaaaa", Name: "Anna", Level: "view"})
	if err != nil || a.Linked() || a.Name != "Anna" {
		t.Fatalf("Anna %+v %v", a, err)
	}
	b, err := s.IntegrationUser(ctx, i, IntegrationSeen{Subject: "12345678bbbbbbbbbbbbbbbbbbbbbbbb", Name: "Bruno", Level: "control", SystemGenerated: true})
	if err != nil || !b.SystemGenerated {
		t.Fatalf("Bruno %+v %v", b, err)
	}
	ua, _, _ := s.UserByID(ctx, a.RealUserID)
	ub, _, _ := s.UserByID(ctx, b.RealUserID)
	if ua.Name != i.UUID+":12345678aaaaaaaaaaaaaaaaaaaaaaaa" || ub.Name != i.UUID+":12345678bbbbbbbbbbbbbbbbbbbbbbbb" || ua.Role != RoleIntegration ||
		ua.DisplayName != "Anna" || ua.IntegrationID != i.ID {
		t.Fatalf("names %q %q %+v", ua.Name, ub.Name, ua)
	}
	again, _ := s.IntegrationUser(ctx, i, IntegrationSeen{Subject: "12345678aaaaaaaaaaaaaaaaaaaaaaaa", Name: "Anna R.", Level: "admin"})
	ua, _, _ = s.UserByID(ctx, a.RealUserID)
	if again.ID != a.ID || again.Name != "Anna R." || again.LastLevel != "admin" || ua.DisplayName != "Anna R." {
		t.Fatalf("again %+v, user %+v", again, ua)
	}
	// A display name given by hand stays when the name reported changes.
	if _, err := s.UpdateUser(ctx, ua.Name, func(u *User) error { u.DisplayName = "Anna (kitchen)"; return nil }, 0); err != nil {
		t.Fatal(err)
	}
	again, _ = s.IntegrationUser(ctx, i, IntegrationSeen{Subject: "12345678aaaaaaaaaaaaaaaaaaaaaaaa", Name: "Anna Rossi", Level: "admin"})
	ua, _, _ = s.UserByID(ctx, a.RealUserID)
	if again.Name != "Anna Rossi" || ua.DisplayName != "Anna (kitchen)" {
		t.Fatalf("a name given by hand: %+v, user %+v", again, ua)
	}
	if sys, err := s.IntegrationUser(ctx, i, IntegrationSeen{Subject: "system", Level: "admin"}); err != nil || sys.EffectiveUserID != i.SystemUserID {
		t.Fatalf("system %+v %v", sys, err)
	}
	if _, err := s.IntegrationUser(ctx, i, IntegrationSeen{Subject: "not-a-plain-id", Level: "view"}); !errors.Is(err, ErrBadName) {
		t.Fatalf("a subject that makes no name: %v", err)
	}
	if _, err := s.AddUser(ctx, User{Name: i.UUID + ":x"}, "", 0); !errors.Is(err, ErrBadName) {
		t.Fatalf("a regular user under an integration's name: %v", err)
	}
}

// A request attached to an integration brings the installation's name:
// the integration takes it, and its own user's display name follows while
// it is the name reported before.
func TestIntegrationReportedName(t *testing.T) {
	t.Parallel()
	s := open(t, t.TempDir())
	ctx := context.Background()
	req, _ := s.AddIntegrationRequest(ctx, haRequest(instanceA), "AAAAAA")
	i, _ := s.ApproveIntegration(ctx, req.UUID, nil, "", 0)
	attach := func(name, code string) (Integration, User) {
		t.Helper()
		r := haRequest(instanceA)
		r.Name = name
		again, err := s.AddIntegrationRequest(ctx, r, code)
		if err != nil {
			t.Fatal(err)
		}
		back, err := s.ApproveIntegration(ctx, again.UUID, nil, i.UUID, 0)
		if err != nil {
			t.Fatal(err)
		}
		own, _, _ := s.UserByID(ctx, back.UserID)
		return back, own
	}
	if back, own := attach("Casa Rossa", "BBBBBB"); back.Name != "Casa Rossa" || own.DisplayName != "Casa Rossa" {
		t.Fatalf("a new name reported: %+v, %+v", back, own)
	}
	if _, err := s.UpdateUser(ctx, i.UUID+":", func(u *User) error { u.DisplayName = "Home"; return nil }, 0); err != nil {
		t.Fatal(err)
	}
	if back, own := attach("Casa Verde", "CCCCCC"); back.Name != "Casa Verde" || own.DisplayName != "Home" {
		t.Fatalf("a name given by hand: %+v, %+v", back, own)
	}
}

// An integration's users link to Massimo users, two to the same one, and
// unlink; the system user and a link to an integration's user are refused.
func TestLinkIntegrationUser(t *testing.T) {
	t.Parallel()
	s := open(t, t.TempDir())
	ctx := context.Background()
	req, _ := s.AddIntegrationRequest(ctx, haRequest(instanceA), "AAAAAA")
	i, _ := s.ApproveIntegration(ctx, req.UUID, nil, "", 0)
	anna, err := s.AddUser(ctx, User{Name: "anna", Role: RoleGuest}, "", 0)
	if err != nil {
		t.Fatal(err)
	}
	a, _ := s.IntegrationUser(ctx, i, IntegrationSeen{Subject: "aaaaaaaa000000000000000000000000", Name: "Anna", Level: "view"})
	b, _ := s.IntegrationUser(ctx, i, IntegrationSeen{Subject: "bbbbbbbb000000000000000000000000", Name: "Anna's phone", Level: "view"})
	if _, err := s.IntegrationUser(ctx, i, IntegrationSeen{Subject: "system", Level: "admin"}); err != nil {
		t.Fatal(err)
	}
	for _, id := range []IntegrationIdentity{a, b} {
		got, err := s.LinkIntegrationUser(ctx, i.ID, id.Subject, anna.ID, 0)
		if err != nil || !got.Linked() || got.EffectiveUserID != anna.ID {
			t.Fatalf("link %s: %+v %v", id.Subject, got, err)
		}
	}
	list, err := s.IntegrationIdentities(ctx, i.ID)
	if err != nil || len(list) != 3 || list[0].Subject != "system" || list[1].EffectiveUserID != anna.ID || list[2].EffectiveUserID != anna.ID {
		t.Fatalf("listed %+v %v", list, err)
	}
	if _, err := s.LinkIntegrationUser(ctx, i.ID, "system", anna.ID, 0); !errors.Is(err, ErrBadLink) {
		t.Fatalf("linking the system user: %v", err)
	}
	if _, err := s.LinkIntegrationUser(ctx, i.ID, b.Subject, a.RealUserID, 0); !errors.Is(err, ErrBadLink) {
		t.Fatalf("linking to an integration's user: %v", err)
	}
	if _, err := s.LinkIntegrationUser(ctx, i.ID, b.Subject, i.UserID, 0); !errors.Is(err, ErrBadLink) {
		t.Fatalf("linking to the integration's own user: %v", err)
	}
	if _, err := s.LinkIntegrationUser(ctx, i.ID, "cccccccc000000000000000000000000", anna.ID, 0); !errors.Is(err, ErrNoIntegration) {
		t.Fatalf("an unknown user: %v", err)
	}
	got, err := s.LinkIntegrationUser(ctx, i.ID, a.Subject, 0, 0)
	if err != nil || got.Linked() || got.EffectiveUserID != a.RealUserID {
		t.Fatalf("unlinked %+v %v", got, err)
	}
	// A later call keeps the link.
	again, _ := s.IntegrationUser(ctx, i, IntegrationSeen{Subject: b.Subject, Name: "Anna's phone", Level: "control"})
	if again.EffectiveUserID != anna.ID {
		t.Fatalf("the link lost at a call: %+v", again)
	}
	all, _ := s.AllIntegrationIdentities(ctx)
	if len(all) != 3 {
		t.Fatalf("all %d", len(all))
	}
}

// integrationOnly approves an integration on s and makes one of its
// users at that user's first call: two users of the integration's and
// a third, none of them a regular user.
func integrationOnly(t *testing.T, s *Store) Integration {
	t.Helper()
	ctx := context.Background()
	req, err := s.AddIntegrationRequest(ctx, haRequest(instanceA), "K7Q2PX")
	if err != nil {
		t.Fatal(err)
	}
	i, err := s.ApproveIntegration(ctx, req.UUID, []string{"read"}, "", 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.IntegrationUser(ctx, i, IntegrationSeen{Subject: "abc123", Name: "Anna"}); err != nil {
		t.Fatal(err)
	}
	return i
}

// An integration's user acts through its integration alone: whatever
// its role, the store gives it no key, no session, no password of
// either version and no identity at a provider; the integration's own
// key is still collected, and a password it holds may be removed.
func TestIntegrationUserHasNoWayIn(t *testing.T) {
	t.Parallel()
	s := open(t, t.TempDir())
	ctx := context.Background()
	if _, err := s.AddUser(ctx, User{Name: "anna", Role: RoleOwner}, "", 0); err != nil {
		t.Fatal(err)
	}
	i := integrationOnly(t, s)
	p, err := s.AddProvider(ctx, AuthProvider{Key: "cloud", Name: "Cloud", Issuer: "https://cloud.example.com", ClientID: "massimo", Enabled: true}, 0)
	if err != nil {
		t.Fatal(err)
	}
	record, err := s.opaqueRecordFor(ctx, "x", "correct horse")
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{i.UUID + ":", i.UUID + ":system", i.UUID + ":abc123"} {
		u, err := s.UpdateUser(ctx, name, func(u *User) error { u.Role = RoleOwner; return nil }, 0)
		if err != nil {
			t.Fatalf("%s: the role owner: %v", name, err)
		}
		if _, _, err := s.AddToken(ctx, u.ID, TokenAPI, "x", time.Time{}); !errors.Is(err, ErrIntegrationUser) {
			t.Errorf("%s: a key: %v", name, err)
		}
		if _, _, err := s.AddToken(ctx, u.ID, TokenSession, "x", time.Time{}); !errors.Is(err, ErrIntegrationUser) {
			t.Errorf("%s: a session: %v", name, err)
		}
		if _, err := s.AddTokenHash(ctx, u.ID, TokenAPI, BootstrapLabel, HashToken("secret"), time.Time{}); !errors.Is(err, ErrIntegrationUser) {
			t.Errorf("%s: a key by its hash: %v", name, err)
		}
		if err := s.SetPassword(ctx, name, "correct horse", 0); !errors.Is(err, ErrIntegrationUser) {
			t.Errorf("%s: a password: %v", name, err)
		}
		if err := s.SetPassword(ctx, name, "", 0); err != nil {
			t.Errorf("%s: no password: %v", name, err)
		}
		if err := s.SetOpaqueRecord(ctx, name, record, OpaqueKSF, 0); !errors.Is(err, ErrIntegrationUser) {
			t.Errorf("%s: a record: %v", name, err)
		}
		if _, err := s.AddIdentity(ctx, u.ID, p.Key, "anna@example.com", 0); !errors.Is(err, ErrIntegrationUser) {
			t.Errorf("%s: an e-mail at a provider: %v", name, err)
		}
		if _, err := s.AddSubjectIdentity(ctx, u.ID, p.Key, "sub-"+u.UUID, 0); !errors.Is(err, ErrIntegrationUser) {
			t.Errorf("%s: a user id at a provider: %v", name, err)
		}
		if _, err := s.UpdateIdentity(ctx, u.ID, 1, "anna@example.com", ""); !errors.Is(err, ErrIntegrationUser) {
			t.Errorf("%s: an identity changed: %v", name, err)
		}
		if _, err := s.LinkIdentity(ctx, u.ID, p.ID, "sub-"+u.UUID, IdentityDetails{}); !errors.Is(err, ErrIntegrationUser) {
			t.Errorf("%s: a link: %v", name, err)
		}
		if got, _, _ := s.UserByID(ctx, u.ID); got.HasPassword {
			t.Errorf("%s: a password kept", name)
		}
	}
	key, _, err := s.CollectIntegrationKey(ctx, i.UUID, "K7Q2PX")
	if err != nil {
		t.Fatalf("the integration's own key: %v", err)
	}
	tok, ok, _ := s.TokenByHash(ctx, HashToken(key))
	if !ok || tok.UserID != i.UserID {
		t.Fatalf("the own key %+v %v", tok, ok)
	}
	if _, _, err := s.AddKeySession(ctx, tok.ID, "x", time.Time{}); !errors.Is(err, ErrIntegrationUser) {
		t.Errorf("a session from the integration's key: %v", err)
	}
}

// The last regular owner stays, beside an integration's user given the
// role owner: it is no owner left, and its own change of role is not
// held back (AP LooseEnds, D1).
func TestLastOwnerBesideAnIntegrationsOwner(t *testing.T) {
	t.Parallel()
	s := open(t, t.TempDir())
	ctx := context.Background()
	if _, err := s.AddUser(ctx, User{Name: "anna", Role: RoleOwner}, "", 0); err != nil {
		t.Fatal(err)
	}
	i := integrationOnly(t, s)
	system := i.UUID + ":system"
	if _, err := s.UpdateUser(ctx, system, func(u *User) error { u.Role = RoleOwner; return nil }, 0); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteUser(ctx, "anna", 0); !errors.Is(err, ErrLastOwner) {
		t.Fatalf("the last regular owner removed: %v", err)
	}
	if _, err := s.UpdateUser(ctx, "anna", func(u *User) error { u.Role = RoleViewer; return nil }, 0); !errors.Is(err, ErrLastOwner) {
		t.Fatalf("the last regular owner demoted: %v", err)
	}
	if _, err := s.UpdateUser(ctx, "anna", func(u *User) error { u.Disabled = true; return nil }, 0); !errors.Is(err, ErrLastOwner) {
		t.Fatalf("the last regular owner disabled: %v", err)
	}
	if _, err := s.UpdateUser(ctx, system, func(u *User) error { u.Role = RoleIntegration; return nil }, 0); err != nil {
		t.Fatalf("the integration's user narrowed: %v", err)
	}
	if err := s.DeleteUser(ctx, i.UUID+":abc123", 0); err != nil {
		t.Fatalf("an integration's user removed: %v", err)
	}
}
