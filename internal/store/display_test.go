// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package store

import (
	"context"
	"errors"
	"testing"
)

// The name the owner gives an integration is its own user's display name,
// set by SetIntegration beside the name it reports; a name it then
// reports anew leaves it (AP StoreRaise v1.2, Block 4). A Namer shows
// its users by the chain, a linked one with the user it acts as.
func TestIntegrationDisplayName(t *testing.T) {
	t.Parallel()
	s := open(t, t.TempDir())
	ctx := context.Background()
	req, _ := s.AddIntegrationRequest(ctx, haRequest(instanceA), "AAAAAA")
	i, err := s.ApproveIntegration(ctx, req.UUID, nil, "", 0)
	if err != nil {
		t.Fatal(err)
	}
	if i.DisplayName != i.Name || IntegrationName(i) != i.Name {
		t.Fatalf("approved: %q %q", i.Name, i.DisplayName)
	}
	reported := i.Name
	i, err = s.SetIntegration(ctx, i.UUID, func(i *Integration) error { i.DisplayName = " Home "; i.Name = "ignored"; return nil }, 0)
	if err != nil || i.DisplayName != "Home" || i.Name != reported || IntegrationName(i) != "Home" {
		t.Fatalf("named: %+v %v", i, err)
	}
	if _, err := s.SetIntegration(ctx, i.UUID, func(i *Integration) error { i.DisplayName = " "; return nil }, 0); !errors.Is(err, ErrBadName) {
		t.Fatalf("an empty name: %v", err)
	}
	r := haRequest(instanceA)
	r.Name = "Casa Verde"
	again, _ := s.AddIntegrationRequest(ctx, r, "BBBBBB")
	if i, err = s.ApproveIntegration(ctx, again.UUID, nil, i.UUID, 0); err != nil || i.Name != "Casa Verde" || i.DisplayName != "Home" {
		t.Fatalf("a name reported anew: %+v %v", i, err)
	}

	anna, _ := s.AddUser(ctx, User{Name: "anna", DisplayName: "Anna"}, "", 0)
	bob, _ := s.AddUser(ctx, User{Name: "bob"}, "", 0)
	id, err := s.IntegrationUser(ctx, i, IntegrationSeen{Subject: "aaaaaaaa11112222333344445555aaaa", Name: "Anna's phone", Level: "view"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.LinkIntegrationUser(ctx, i.ID, id.Subject, anna.ID, 0); err != nil {
		t.Fatal(err)
	}
	n := s.Namer()
	own, _ := n.User(ctx, i.UserID)
	sys, _ := n.User(ctx, i.SystemUserID)
	phone, _ := n.User(ctx, id.RealUserID)
	for _, c := range []struct{ got, want string }{
		{n.Chain(ctx, own), "Home"},
		{n.Chain(ctx, sys), "Home › system"},
		{n.Chain(ctx, phone), "Home › Anna's phone"},
		{n.Current(ctx, phone), "Home › Anna's phone, as anna"},
		{n.Current(ctx, anna), "Anna"},
		{n.Current(ctx, bob), "bob"},
		{n.Display(ctx, phone.ID, anna.ID), "Home › Anna's phone, as anna"},
		{n.Display(ctx, 0, anna.ID), "Anna"},
		{n.Display(ctx, bob.ID, bob.ID), "bob"},
		{n.Display(ctx, 0, 0), ""},
	} {
		if c.got != c.want {
			t.Errorf("%q, want %q", c.got, c.want)
		}
	}
}
