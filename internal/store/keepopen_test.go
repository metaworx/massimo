// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package store

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"
)

// keptOpen sets the keep-open setting on a store without users.
func keptOpen(t *testing.T, s *Store) {
	t.Helper()
	if err := s.SetKeepOpen(context.Background(), true, 0); err != nil {
		t.Fatal(err)
	}
	if !s.KeepOpen(context.Background()) {
		t.Fatal("not kept open")
	}
}

// The keep-open setting is off by default, set and reset on a store
// without users, and refused on one with users, where false still goes
// through (AP FirstOwner, D11, D12).
func TestKeepOpenSetting(t *testing.T) {
	t.Parallel()
	s := open(t, t.TempDir())
	ctx := context.Background()
	if s.KeepOpen(ctx) {
		t.Fatal("kept open by default")
	}
	keptOpen(t, s)
	if err := s.SetKeepOpen(ctx, false, 0); err != nil || s.KeepOpen(ctx) {
		t.Fatalf("off: %v %v", err, s.KeepOpen(ctx))
	}
	if err := s.SetSetting(ctx, KeepOpenKey, "yes"); err != nil {
		t.Fatal(err)
	}
	if s.KeepOpen(ctx) {
		t.Fatal("a value that is not a switch keeps it open")
	}
	if _, err := s.AddUser(ctx, User{Name: "anna", Role: RoleOwner}, "", 0); err != nil {
		t.Fatal(err)
	}
	if err := s.SetKeepOpen(ctx, true, 0); !errors.Is(err, ErrHasUsers) {
		t.Fatalf("on with users: %v", err)
	}
	if _, set, _ := s.Setting(ctx, KeepOpenKey); set {
		t.Fatal("set with users")
	}
	if err := s.SetKeepOpen(ctx, false, 0); err != nil {
		t.Fatalf("off with users: %v", err)
	}
}

// Every transaction of the store that makes a user removes the setting,
// and says so in the log; a server that loses its users again may be
// kept open again (D12).
func TestKeepOpenGoesWithTheFirstUser(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	for name, add := range map[string]func(t *testing.T, s *Store){
		"user": func(t *testing.T, s *Store) {
			if _, err := s.AddUser(ctx, User{Name: "anna", Role: RoleOwner}, "", 0); err != nil {
				t.Fatal(err)
			}
		},
		"opaque user": func(t *testing.T, s *Store) {
			if _, err := s.AddUserOpaque(ctx, User{Name: "anna", Role: RoleOwner}, "correct horse", 0); err != nil {
				t.Fatal(err)
			}
		},
		"bootstrap": func(t *testing.T, s *Store) {
			if res, err := s.Bootstrap(ctx, "secret"); err != nil || !res.OwnerCreated {
				t.Fatalf("%+v %v", res, err)
			}
		},
		"provider": func(t *testing.T, s *Store) {
			p, err := s.AddProvider(ctx, AuthProvider{Key: "cloud", Name: "Cloud", Issuer: "https://cloud.example.com", ClientID: "massimo", Enabled: true}, 0)
			if err != nil {
				t.Fatal(err)
			}
			if _, _, err := s.AddIdentityUser(ctx, User{Name: "anna", Role: RoleViewer}, p.ID, "sub-anna", ""); err != nil {
				t.Fatal(err)
			}
		},
		"integration": func(t *testing.T, s *Store) {
			req, err := s.AddIntegrationRequest(ctx, haRequest(instanceA), "K7Q2PX")
			if err != nil {
				t.Fatal(err)
			}
			if _, err := s.ApproveIntegration(ctx, req.UUID, []string{"read"}, "", 0); err != nil {
				t.Fatal(err)
			}
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			s := open(t, t.TempDir())
			var log bytes.Buffer
			s.log = slog.New(slog.NewTextHandler(&log, nil))
			keptOpen(t, s)
			add(t, s)
			if s.KeepOpen(ctx) {
				t.Fatal("still kept open with a user")
			}
			if !strings.Contains(log.String(), "keep-open setting removed") {
				t.Fatalf("not in the log: %s", log.String())
			}
		})
	}

	// An integration's user made at its first call, once every other
	// user is gone.
	s := open(t, t.TempDir())
	req, _ := s.AddIntegrationRequest(ctx, haRequest(instanceA), "K7Q2PX")
	i, err := s.ApproveIntegration(ctx, req.UUID, []string{"read"}, "", 0)
	if err != nil {
		t.Fatal(err)
	}
	us, _ := s.Users(ctx)
	for _, u := range us {
		if err := s.DeleteUser(ctx, u.Name, 0); err != nil {
			t.Fatal(err)
		}
	}
	keptOpen(t, s)
	if _, err := s.IntegrationUser(ctx, i, IntegrationSeen{Subject: "abc123", Name: "Anna"}); err != nil {
		t.Fatal(err)
	}
	if s.KeepOpen(ctx) {
		t.Fatal("still kept open with an integration's user")
	}
}

// At a start or a reload, a setting left beside users is removed; with
// no users it stays.
func TestDropKeepOpen(t *testing.T) {
	t.Parallel()
	s := open(t, t.TempDir())
	ctx := context.Background()
	keptOpen(t, s)
	if dropped, err := s.DropKeepOpen(ctx); dropped || err != nil || !s.KeepOpen(ctx) {
		t.Fatalf("without users: %v %v", dropped, err)
	}
	// As a process that does not know the setting leaves it: a user and
	// the setting both.
	if _, err := s.AddUser(ctx, User{Name: "anna", Role: RoleOwner}, "", 0); err != nil {
		t.Fatal(err)
	}
	if err := s.SetSetting(ctx, KeepOpenKey, true); err != nil {
		t.Fatal(err)
	}
	if dropped, err := s.DropKeepOpen(ctx); !dropped || err != nil || s.KeepOpen(ctx) {
		t.Fatalf("with users: %v %v", dropped, err)
	}
	if dropped, err := s.DropKeepOpen(ctx); dropped || err != nil {
		t.Fatalf("again: %v %v", dropped, err)
	}
}
