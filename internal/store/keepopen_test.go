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
	"time"
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

// Every transaction of the store that makes a regular user removes the
// setting, and says so in the log (D12); an integration's users leave it
// (TestKeepOpenStaysWithIntegrationUsers).
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
}

// An integration's users are not regular users: its approval and a user
// made at its first call leave the setting as it is and the count at 0,
// the setting can be switched on beside them, and the
// first owner is made once, taking the setting with it (AP LooseEnds,
// D1).
func TestKeepOpenStaysWithIntegrationUsers(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := open(t, t.TempDir())
	var log bytes.Buffer
	s.log = slog.New(slog.NewTextHandler(&log, nil))
	keptOpen(t, s)
	integrationOnly(t, s)
	if !s.KeepOpen(ctx) {
		t.Fatal("the setting went with an integration's users")
	}
	if strings.Contains(log.String(), "keep-open setting removed") {
		t.Fatalf("a removal in the log: %s", log.String())
	}
	if n, err := s.CountUsers(ctx); n != 0 || err != nil {
		t.Fatalf("%d regular users, %v", n, err)
	}
	if n, err := s.CountIntegrationUsers(ctx); n != 3 || err != nil {
		t.Fatalf("%d integration users, %v", n, err)
	}
	if err := s.SetKeepOpen(ctx, false, 0); err != nil {
		t.Fatal(err)
	}
	keptOpen(t, s)
	root, err := s.AddFirstOwner(ctx, User{Name: "root"}, "correct horse")
	if err != nil || root.Role != RoleOwner || s.KeepOpen(ctx) {
		t.Fatalf("the first owner: %+v %v %v", root, err, s.KeepOpen(ctx))
	}
	if _, err := s.AddFirstOwner(ctx, User{Name: "anna"}, ""); !errors.Is(err, ErrHasUsers) {
		t.Fatalf("a second: %v", err)
	}
	if err := s.SetKeepOpen(ctx, true, 0); !errors.Is(err, ErrHasUsers) {
		t.Fatalf("on beside the owner: %v", err)
	}
}

// An owner that is removed, with an integration's users left, leaves a
// server without a regular user: the setting may be switched on again,
// and the first owner made again (D1; FirstOwner, D12).
func TestKeepOpenAfterTheOwnerIsRemoved(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := open(t, t.TempDir())
	// An owner with an end date may be removed: it is not an owner left.
	if _, err := s.AddUser(ctx, User{Name: "anna", Role: RoleOwner, ExpiresAt: s.now().Add(time.Hour)}, "", 0); err != nil {
		t.Fatal(err)
	}
	integrationOnly(t, s)
	if err := s.SetKeepOpen(ctx, true, 0); !errors.Is(err, ErrHasUsers) {
		t.Fatalf("on beside the owner: %v", err)
	}
	if err := s.DeleteUser(ctx, "anna", 0); err != nil {
		t.Fatal(err)
	}
	if n, _ := s.CountUsers(ctx); n != 0 {
		t.Fatalf("%d regular users", n)
	}
	keptOpen(t, s)
	if _, err := s.AddFirstOwner(ctx, User{Name: "root"}, ""); err != nil || s.KeepOpen(ctx) {
		t.Fatalf("the first owner again: %v %v", err, s.KeepOpen(ctx))
	}
}

// At a start or a reload, a setting left beside users is removed; with
// no regular users it stays, an integration's users beside it (AP
// LooseEnds, D1).
func TestDropKeepOpen(t *testing.T) {
	t.Parallel()
	s := open(t, t.TempDir())
	ctx := context.Background()
	keptOpen(t, s)
	if dropped, err := s.DropKeepOpen(ctx); dropped || err != nil || !s.KeepOpen(ctx) {
		t.Fatalf("without users: %v %v", dropped, err)
	}
	integrationOnly(t, s)
	if dropped, err := s.DropKeepOpen(ctx); dropped || err != nil || !s.KeepOpen(ctx) {
		t.Fatalf("beside an integration's users: %v %v", dropped, err)
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

// The first owner is added once: a second is ErrHasUsers, and so is one
// that many goroutines try at once, of which one wins; the setting goes
// with the one that wins (AP FirstOwner, D15).
func TestAddFirstOwnerOnce(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := open(t, t.TempDir())
	keptOpen(t, s)
	root, err := s.AddFirstOwner(ctx, User{Name: "root", Role: RoleViewer}, "correct horse")
	if err != nil || root.Role != RoleOwner || !root.HasPassword || s.KeepOpen(ctx) {
		t.Fatalf("the first: %+v %v", root, err)
	}
	if _, err := s.AddFirstOwner(ctx, User{Name: "anna"}, ""); !errors.Is(err, ErrHasUsers) {
		t.Fatalf("a second: %v", err)
	}
	record, err := s.opaqueRecordFor(ctx, "anna", "correct horse")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.AddFirstOpaqueOwner(ctx, User{Name: "anna"}, record, OpaqueKSF); !errors.Is(err, ErrHasUsers) {
		t.Fatalf("a second by OPAQUE: %v", err)
	}
	if n, _ := s.CountUsers(ctx); n != 1 {
		t.Fatalf("%d users", n)
	}

	for round := range 20 {
		s := open(t, t.TempDir())
		const tries = 8
		errs := make(chan error, tries)
		start := make(chan struct{})
		for i := range tries {
			go func() {
				<-start
				_, err := s.AddFirstOwner(ctx, User{Name: "owner" + string(rune('a'+i))}, "")
				errs <- err
			}()
		}
		close(start)
		won := 0
		for range tries {
			switch err := <-errs; {
			case err == nil:
				won++
			case !errors.Is(err, ErrHasUsers):
				t.Fatalf("round %d: %v", round, err)
			}
		}
		if n, _ := s.CountUsers(ctx); won != 1 || n != 1 {
			t.Fatalf("round %d: %d won, %d users", round, won, n)
		}
	}
}

// The first owner is still counted in the transaction that writes it
// while an integration's users are made at the same moment: of the
// first owners tried at once, one wins, every integration's user is
// made, and the count is the one owner (AP FirstOwner, D15; AP
// LooseEnds, D1).
func TestAddFirstOwnerBesideAnIntegration(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	for round := range 10 {
		s := open(t, t.TempDir())
		i := integrationOnly(t, s)
		req, err := s.AddIntegrationRequest(ctx, haRequest(instanceB), "M4R8TW")
		if err != nil {
			t.Fatal(err)
		}
		const owners, callers = 4, 4
		errs := make(chan error, owners+callers+1)
		won := make(chan struct{}, owners)
		start := make(chan struct{})
		for n := range owners {
			go func() {
				<-start
				_, err := s.AddFirstOwner(ctx, User{Name: "owner" + string(rune('a'+n))}, "")
				if err == nil {
					won <- struct{}{}
				} else if errors.Is(err, ErrHasUsers) {
					err = nil
				}
				errs <- err
			}()
		}
		for n := range callers {
			go func() {
				<-start
				_, err := s.IntegrationUser(ctx, i, IntegrationSeen{Subject: "user" + string(rune('a'+n)), Name: "Anna"})
				errs <- err
			}()
		}
		go func() {
			<-start
			_, err := s.ApproveIntegration(ctx, req.UUID, []string{"read"}, "", 0)
			errs <- err
		}()
		close(start)
		for range owners + callers + 1 {
			if err := <-errs; err != nil {
				t.Fatalf("round %d: %v", round, err)
			}
		}
		if n, _ := s.CountUsers(ctx); len(won) != 1 || n != 1 {
			t.Fatalf("round %d: %d won, %d regular users", round, len(won), n)
		}
		if n, _ := s.CountIntegrationUsers(ctx); n != 3+callers+2 {
			t.Fatalf("round %d: %d integration users", round, n)
		}
	}
}
