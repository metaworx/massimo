// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

// Preferences round-trip per user; a write bumps the version, a removal
// leaves a tombstone, which the next write clears; the key and the value
// are checked.
func TestPreferences(t *testing.T) {
	s := open(t, t.TempDir())
	ctx := context.Background()
	anna, _ := s.AddUser(ctx, User{Name: "anna", Role: RoleOperator}, "pw", 0)
	bob, _ := s.AddUser(ctx, User{Name: "bob", Role: RoleViewer}, "pw", 0)
	if list, err := s.Preferences(ctx, anna.ID); err != nil || len(list) != 0 {
		t.Fatalf("none yet: %v %v", list, err)
	}
	ui := json.RawMessage(`{"poll":5,"theme":"dark"}`)
	p, err := s.SetPreference(ctx, anna.ID, "ui", ui)
	if err != nil || p.Key != "ui" || string(p.Value) != string(ui) || p.Version != 1 || p.Origin != s.NodeID() || p.UpdatedAt.IsZero() {
		t.Fatalf("set %+v %v", p, err)
	}
	p, _ = s.SetPreference(ctx, anna.ID, "ui", json.RawMessage(`{"poll":10}`))
	if string(p.Value) != `{"poll":10}` || p.Version != 2 {
		t.Fatalf("second set %+v", p)
	}
	s.SetPreference(ctx, anna.ID, "schemes", json.RawMessage(`{"schemes":[]}`))
	s.SetPreference(ctx, bob.ID, "ui", json.RawMessage(`{"poll":30}`))
	list, _ := s.Preferences(ctx, anna.ID)
	if len(list) != 2 || list[0].Key != "schemes" || list[1].Key != "ui" {
		t.Fatalf("anna's %+v", list)
	}
	if b, ok, _ := s.Preference(ctx, bob.ID, "ui"); !ok || string(b.Value) != `{"poll":30}` {
		t.Fatalf("bob's %+v %v", b, ok)
	}
	if _, ok, _ := s.Preference(ctx, bob.ID, "schemes"); ok {
		t.Fatal("anna's key found for bob")
	}

	if err := s.DeletePreference(ctx, anna.ID, "ui"); err != nil {
		t.Fatal(err)
	}
	if _, ok, _ := s.Preference(ctx, anna.ID, "ui"); ok {
		t.Fatal("removed preference found")
	}
	var deleted sql.NullString
	var version int
	s.db.QueryRow(`SELECT deleted_at, version FROM preferences WHERE user_id = ? AND key = 'ui'`, anna.ID).Scan(&deleted, &version)
	if !deleted.Valid || version != 3 {
		t.Fatalf("tombstone %v version %d", deleted, version)
	}
	if err := s.DeletePreference(ctx, anna.ID, "ui"); err != nil {
		t.Fatalf("remove twice: %v", err)
	}
	if err := s.DeletePreference(ctx, anna.ID, "never"); err != nil {
		t.Fatalf("remove an unset key: %v", err)
	}
	p, _ = s.SetPreference(ctx, anna.ID, "ui", json.RawMessage(`{"poll":2}`))
	if p.Version != 4 || string(p.Value) != `{"poll":2}` {
		t.Fatalf("set after the removal %+v", p)
	}

	for _, key := range []string{"", "UI", "1ui", "ui/x", strings.Repeat("k", 33)} {
		if _, err := s.SetPreference(ctx, anna.ID, key, json.RawMessage(`{}`)); !errors.Is(err, ErrBadKey) {
			t.Errorf("key %q: %v", key, err)
		}
	}
	for _, v := range []string{"", "nope", "{"} {
		if _, err := s.SetPreference(ctx, anna.ID, "ui", json.RawMessage(v)); !errors.Is(err, ErrBadValue) {
			t.Errorf("value %q: %v", v, err)
		}
	}
	big := json.RawMessage(`"` + strings.Repeat("x", MaxPreferenceBytes) + `"`)
	if _, err := s.SetPreference(ctx, anna.ID, "ui", big); !errors.Is(err, ErrValueTooLarge) {
		t.Fatalf("a large value: %v", err)
	}
	if err := s.DeletePreference(ctx, anna.ID, "Bad"); !errors.Is(err, ErrBadKey) {
		t.Fatalf("remove a bad key: %v", err)
	}
}

// A user removed and added again under the name is another person, in
// a row of its own: the removed user's preferences stay behind with its
// row, as tombstones.
func TestRemovedUserPreferencesGo(t *testing.T) {
	s := open(t, t.TempDir())
	ctx := context.Background()
	anna, _ := s.AddUser(ctx, User{Name: "anna", Role: RoleOperator}, "pw", 0)
	s.SetPreference(ctx, anna.ID, "ui", json.RawMessage(`{"theme":"dark"}`))
	s.SetPreference(ctx, anna.ID, "schemes", json.RawMessage(`{"schemes":[{"name":"Mine"}]}`))
	if err := s.DeleteUser(ctx, "anna", 0); err != nil {
		t.Fatal(err)
	}
	if list, err := s.Preferences(ctx, anna.ID); err != nil || len(list) != 0 {
		t.Fatalf("the removed anna keeps %+v %v", list, err)
	}
	again, err := s.AddUser(ctx, User{Name: "anna", Role: RoleViewer}, "pw2", 0)
	if err != nil || again.ID == anna.ID {
		t.Fatalf("a new row: %+v %v", again, err)
	}
	if list, err := s.Preferences(ctx, again.ID); err != nil || len(list) != 0 {
		t.Fatalf("the new anna inherits %+v %v", list, err)
	}
}
