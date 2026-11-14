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
	if err != nil || i.State != IntegrationActive || i.Key != "casa-leone" || i.UserID == 0 || i.SystemUserID == 0 || strings.Join(i.MaxCaps, ",") != "read,setpoint" {
		t.Fatalf("approved %+v %v", i, err)
	}
	own, _, _ := s.UserByID(ctx, i.UserID)
	sys, _, _ := s.UserByID(ctx, i.SystemUserID)
	if own.Name != "casa-leone" || own.Role != RoleIntegration || own.HasPassword || sys.Name != "casa-leone.system" {
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
	if tok, ok, _ := s.TokenByHash(ctx, HashToken(k1)); ok && tok.RevokedAt.IsZero() {
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
	if tok, ok, _ := s.TokenByHash(ctx, HashToken(k2)); ok && tok.RevokedAt.IsZero() {
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
// gets a key of its own.
func TestIntegrationRequestsBounded(t *testing.T) {
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
	if a.Key != "casa-leone" || b.Key != "casa-leone-2" {
		t.Fatalf("keys %q %q", a.Key, b.Key)
	}
	if _, err := s.AddIntegrationRequest(ctx, IntegrationRequest{Kind: KindHomeAssistant, InstanceID: "short"}, "C"); !errors.Is(err, ErrBadInstance) {
		t.Fatalf("a bad installation id: %v", err)
	}
}

// An integration's users are made at their first call, named by the
// integration's key and their id, longer where two ids begin alike; a
// later call updates the name and the level.
func TestIntegrationUsers(t *testing.T) {
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
	ua, _, _ := s.UserByID(ctx, a.OwnUserID)
	ub, _, _ := s.UserByID(ctx, b.OwnUserID)
	if ua.Name != "casa-leone.12345678" || ub.Name != "casa-leone.12345678bbbb" || ua.Role != RoleIntegration || ua.DisplayName != "Anna" {
		t.Fatalf("names %q %q %+v", ua.Name, ub.Name, ua)
	}
	again, _ := s.IntegrationUser(ctx, i, IntegrationSeen{Subject: "12345678aaaaaaaaaaaaaaaaaaaaaaaa", Name: "Anna R.", Level: "admin"})
	ua, _, _ = s.UserByID(ctx, a.OwnUserID)
	if again.ID != a.ID || again.Name != "Anna R." || again.LastLevel != "admin" || ua.DisplayName != "Anna R." {
		t.Fatalf("again %+v, user %+v", again, ua)
	}
	if sys, err := s.IntegrationUser(ctx, i, IntegrationSeen{Subject: "system", Level: "admin"}); err != nil || sys.UserID != i.SystemUserID {
		t.Fatalf("system %+v %v", sys, err)
	}
}

// An integration's users link to Massimo users, two to the same one, and
// unlink; the system user and a link to an integration's user are refused.
func TestLinkIntegrationUser(t *testing.T) {
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
		if err != nil || !got.Linked() || got.UserID != anna.ID {
			t.Fatalf("link %s: %+v %v", id.Subject, got, err)
		}
	}
	list, err := s.IntegrationIdentities(ctx, i.ID)
	if err != nil || len(list) != 3 || list[0].Subject != "system" || list[1].UserID != anna.ID || list[2].UserID != anna.ID {
		t.Fatalf("listed %+v %v", list, err)
	}
	if _, err := s.LinkIntegrationUser(ctx, i.ID, "system", anna.ID, 0); !errors.Is(err, ErrBadLink) {
		t.Fatalf("linking the system user: %v", err)
	}
	if _, err := s.LinkIntegrationUser(ctx, i.ID, b.Subject, a.OwnUserID, 0); !errors.Is(err, ErrBadLink) {
		t.Fatalf("linking to an integration's user: %v", err)
	}
	if _, err := s.LinkIntegrationUser(ctx, i.ID, b.Subject, i.UserID, 0); !errors.Is(err, ErrBadLink) {
		t.Fatalf("linking to the integration's own user: %v", err)
	}
	if _, err := s.LinkIntegrationUser(ctx, i.ID, "cccccccc000000000000000000000000", anna.ID, 0); !errors.Is(err, ErrNoIntegration) {
		t.Fatalf("an unknown user: %v", err)
	}
	got, err := s.LinkIntegrationUser(ctx, i.ID, a.Subject, 0, 0)
	if err != nil || got.Linked() || got.UserID != a.OwnUserID {
		t.Fatalf("unlinked %+v %v", got, err)
	}
	// A later call keeps the link.
	again, _ := s.IntegrationUser(ctx, i, IntegrationSeen{Subject: b.Subject, Name: "Anna's phone", Level: "control"})
	if again.UserID != anna.ID {
		t.Fatalf("the link lost at a call: %+v", again)
	}
	all, _ := s.AllIntegrationIdentities(ctx)
	if len(all) != 3 {
		t.Fatalf("all %d", len(all))
	}
}
