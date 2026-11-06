// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package main

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"go.metaworx.ch/massimo/internal/env"
	"go.metaworx.ch/massimo/internal/store"
)

// runUserWith runs massimo user with a standard input and returns what
// it printed.
func runUserWith(t *testing.T, input string, args ...string) (string, error) {
	t.Helper()
	var out bytes.Buffer
	stdin, stdout = strings.NewReader(input), &out
	defer func() { stdin, stdout = nil, nil }()
	err := runUser(args)
	return out.String(), err
}

// The user commands against a store in a temporary directory, with the
// password from standard input, and the audit rows they leave.
func TestUserCommands(t *testing.T) {
	clearCubeEnv(t)
	dir := t.TempDir()
	t.Setenv(env.DataVar, dir)
	ctx := context.Background()
	if out, err := runUserWith(t, "pw\n", "add", "Anna", "-role", "operator", "-display-name", "Anna", "-password-stdin"); err != nil || !strings.Contains(out, "user anna added with the role operator") {
		t.Fatalf("add: %q %v", out, err)
	}
	if _, err := runUserWith(t, "", "add", "anna", "-no-password"); err == nil || !strings.Contains(err.Error(), "exists") {
		t.Fatalf("add twice: %v", err)
	}
	if _, err := runUserWith(t, "", "add", "bob", "-role", "nope", "-no-password"); err == nil {
		t.Fatal("unknown role accepted")
	}
	if _, err := runUserWith(t, "", "add", "bob"); err == nil || !strings.Contains(err.Error(), "no terminal") {
		t.Fatalf("add without a terminal: %v", err)
	}
	if _, err := runUserWith(t, "\n", "add", "bob", "-password-stdin"); err == nil {
		t.Fatal("empty password accepted")
	}
	if _, err := runUserWith(t, "", "add", "-no-password", "-role", "guest", "-expires", "2026-12-24T18:00:00Z", "guest1"); err != nil {
		t.Fatalf("add a guest with the flags first: %v", err)
	}
	st, err := store.OpenReadOnly(dir, store.Options{})
	if err != nil {
		t.Fatal(err)
	}
	anna, ok, _ := st.User(ctx, "anna")
	if !ok || anna.Role != store.RoleOperator || !anna.HasPassword || anna.DisplayName != "Anna" {
		t.Fatalf("anna %+v", anna)
	}
	if err := st.CheckPassword(ctx, "anna", "pw"); err != nil {
		t.Fatal(err)
	}
	g, _, _ := st.User(ctx, "guest1")
	if g.Role != store.RoleGuest || g.ExpiresAt.IsZero() || g.HasPassword {
		t.Fatalf("guest %+v", g)
	}
	st.Close()

	out, err := runUserWith(t, "", "list")
	if err != nil || !strings.Contains(out, "anna") || !strings.Contains(out, "operator") || !strings.Contains(out, "until 2026-12-24") || !strings.Contains(out, "NAME") {
		t.Fatalf("list: %q %v", out, err)
	}
	if _, err := runUserWith(t, "new\n", "passwd", "anna", "-password-stdin"); err != nil {
		t.Fatalf("passwd: %v", err)
	}
	if _, err := runUserWith(t, "x\n", "passwd", "nobody", "-password-stdin"); err == nil {
		t.Fatal("passwd of nobody")
	}
	if out, err := runUserWith(t, "", "disable", "anna"); err != nil || !strings.Contains(out, "disabled") {
		t.Fatalf("disable: %q %v", out, err)
	}
	if out, err := runUserWith(t, "", "enable", "anna"); err != nil || !strings.Contains(out, "enabled") {
		t.Fatalf("enable: %q %v", out, err)
	}
	key, err := runUserWith(t, "", "token", "anna", "-label", "Loxone")
	key = strings.TrimSpace(key)
	if err != nil || len(key) != 64 {
		t.Fatalf("token: %q %v", key, err)
	}
	if _, err := runUserWith(t, "", "token", "anna"); err == nil {
		t.Fatal("token without a label")
	}
	runUserWith(t, "", "token", "anna", "-label", "Second")
	st, _ = store.OpenReadOnly(dir, store.Options{})
	tok, ok, _ := st.TokenByHash(ctx, store.HashToken(key))
	if !ok || tok.Label != "Loxone" || tok.Kind != store.TokenAPI {
		t.Fatalf("key %+v %v", tok, ok)
	}
	if err := st.CheckPassword(ctx, "anna", "new"); err != nil {
		t.Fatal("password not set")
	}
	st.Close()
	if out, err := runUserWith(t, "", "revoke", "anna", "999"); err == nil {
		t.Fatalf("revoke unknown: %q", out)
	}
	if out, err := runUserWith(t, "", "revoke", "anna", "1"); err != nil || !strings.Contains(out, "1 token(s)") {
		t.Fatalf("revoke one: %q %v", out, err)
	}
	if out, err := runUserWith(t, "", "revoke", "anna"); err != nil || !strings.Contains(out, "1 token(s)") {
		t.Fatalf("revoke the rest: %q %v", out, err)
	}
	st, _ = store.OpenReadOnly(dir, store.Options{})
	defer st.Close()
	if list, _ := st.Tokens(ctx, anna.ID); len(list) != 0 {
		t.Fatalf("tokens left %+v", list)
	}
	rows, _ := st.AuditList(ctx, time.Time{}, 100, "")
	actions := map[string]int{}
	for _, r := range rows {
		if r.Remote != store.RemoteCLI || r.UserID != 0 {
			t.Fatalf("audit row %+v", r)
		}
		actions[r.Action+":"+r.Result]++
	}
	for _, want := range []string{"user.add:ok", "user.add:failed", "user.update:ok", "token.add:ok", "token.revoke:ok"} {
		if actions[want] == 0 {
			t.Errorf("no audit row %s in %v", want, actions)
		}
	}
	if _, err := runUserWith(t, "", "nope"); err == nil {
		t.Fatal("unknown command")
	}
	if _, err := runUserWith(t, ""); err == nil {
		t.Fatal("no command")
	}
}
