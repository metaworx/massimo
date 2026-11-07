// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package store

import (
	"context"
	"errors"
	"strings"
	"testing"
)

var allCaps = []string{"read", "preferences", "setpoint", "lock", "configure", "rename", "structure", "pair", "discover", "backup", "restore", "guests", "users", "roles", "roles.disable", "capabilities", "cubes", "settings", "update", "restart", "audit"}

// Roles are listed with their sets, created, renamed, given another
// set, disabled and removed under the rules of docs/store.md section 6.
func TestRoles(t *testing.T) {
	s := open(t, t.TempDir())
	ctx := context.Background()
	roles, err := s.Roles(ctx)
	if err != nil || len(roles) != 6 || roles[0].Key != RoleOwner || roles[5].Key != RoleIntegration || !roles[1].Builtin {
		t.Fatalf("roles %+v %v", roles, err)
	}
	if strings.Join(roles[2].Capabilities, ",") != "backup,configure,discover,lock,preferences,read,setpoint" {
		t.Fatalf("operator %v", roles[2].Capabilities)
	}
	night, err := s.AddRole(ctx, Role{Key: "Night", Name: "Night shift", Capabilities: []string{"setpoint", "read", "setpoint", " "}}, 0)
	if err != nil || night.Key != "night" || night.Name != "Night shift" || night.Builtin || strings.Join(night.Capabilities, ",") != "read,setpoint" {
		t.Fatalf("add %+v %v", night, err)
	}
	if _, err := s.AddRole(ctx, Role{Key: "night"}, 0); !errors.Is(err, ErrRoleExists) {
		t.Fatalf("add twice: %v", err)
	}
	if _, err := s.AddRole(ctx, Role{Key: "1bad"}, 0); !errors.Is(err, ErrBadRole) {
		t.Fatalf("bad key: %v", err)
	}
	night, err = s.UpdateRole(ctx, "night", func(r *Role) error { r.Name, r.Capabilities = "Nights", []string{"read", "lock"}; return nil }, 0)
	if err != nil || night.Name != "Nights" || strings.Join(night.Capabilities, ",") != "lock,read" || night.Version != 2 {
		t.Fatalf("update %+v %v", night, err)
	}
	if _, err := s.UpdateRole(ctx, RoleOwner, func(*Role) error { return nil }, 0); !errors.Is(err, ErrImmutable) {
		t.Fatalf("update owner: %v", err)
	}
	if err := s.SetRoleDisabled(ctx, RoleOwner, true, 0); !errors.Is(err, ErrImmutable) {
		t.Fatalf("disable owner: %v", err)
	}
	if err := s.DeleteRole(ctx, RoleViewer, 0); !errors.Is(err, ErrImmutable) {
		t.Fatalf("delete a default role: %v", err)
	}
	if err := s.SetRoleDisabled(ctx, "night", true, 0); err != nil {
		t.Fatal(err)
	}
	if r, _, _ := s.Role(ctx, "night"); !r.Disabled {
		t.Fatal("not disabled")
	}
	if err := s.SetRoleDisabled(ctx, "nope", true, 0); !errors.Is(err, ErrNoRole) {
		t.Fatalf("disable unknown: %v", err)
	}
	u, _ := s.AddUser(ctx, User{Name: "n1", Role: "night"}, "", 0)
	if r, _, _ := s.Role(ctx, "night"); r.Users != 1 {
		t.Fatalf("users %d", r.Users)
	}
	if err := s.DeleteRole(ctx, "night", 0); !errors.Is(err, ErrRoleInUse) {
		t.Fatalf("delete in use: %v", err)
	}
	s.DeleteUser(ctx, u.Name, 0)
	if err := s.DeleteRole(ctx, "night", 0); err != nil {
		t.Fatal(err)
	}
	if _, ok, _ := s.Role(ctx, "night"); ok {
		t.Fatal("removed role still found")
	}
	if _, err := s.AddUser(ctx, User{Name: "n2", Role: "night"}, "", 0); !errors.Is(err, ErrNoRole) {
		t.Fatalf("user with a removed role: %v", err)
	}
	back, err := s.AddRole(ctx, Role{Key: "night", Capabilities: []string{"read"}}, 0)
	if err != nil || back.ID != night.ID || back.Disabled || strings.Join(back.Capabilities, ",") != "read" {
		t.Fatalf("take over %+v %v", back, err)
	}
}

// The effective set: everything for owner, nothing for a disabled role,
// the role's set minus the disabled capabilities otherwise.
func TestEffective(t *testing.T) {
	s := open(t, t.TempDir())
	ctx := context.Background()
	caps, off, err := s.Effective(ctx, RoleOwner, allCaps)
	if err != nil || off || len(caps) != len(allCaps) {
		t.Fatalf("owner %v %v %v", caps, off, err)
	}
	if caps, _, _ := s.Effective(ctx, RoleViewer, allCaps); strings.Join(caps, ",") != "preferences,read" {
		t.Fatalf("viewer %v", caps)
	}
	if err := s.SetCapabilityDisabled(ctx, "pair", true, 0); err != nil {
		t.Fatal(err)
	}
	s.SetCapabilityDisabled(ctx, "pair", true, 0) // twice is fine
	if d, _ := s.DisabledCapabilities(ctx); strings.Join(d, ",") != "pair" {
		t.Fatalf("disabled %v", d)
	}
	if caps, _, _ := s.Effective(ctx, RoleManager, allCaps); strings.Contains(strings.Join(caps, ","), "pair") || len(caps) != 12 {
		t.Fatalf("manager without pair %v", caps)
	}
	if caps, _, _ := s.Effective(ctx, RoleOwner, allCaps); len(caps) != len(allCaps) {
		t.Fatal("a disabled capability touched owner")
	}
	s.SetCapabilityDisabled(ctx, "pair", false, 0)
	if caps, _, _ := s.Effective(ctx, RoleManager, allCaps); len(caps) != 13 {
		t.Fatalf("manager %v", caps)
	}
	s.SetRoleDisabled(ctx, RoleManager, true, 0)
	if caps, off, _ := s.Effective(ctx, RoleManager, allCaps); !off || len(caps) != 0 {
		t.Fatalf("disabled manager %v %v", caps, off)
	}
	if _, _, err := s.Effective(ctx, "nope", allCaps); !errors.Is(err, ErrNoRole) {
		t.Fatalf("unknown role: %v", err)
	}
}
