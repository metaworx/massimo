// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/bytemare/opaque"

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

// signsIn runs a sign-in by OPAQUE against the store with the browser's
// key stretching, as the web UI's client would: whether the password
// opens the user's record.
func signsIn(t *testing.T, st *store.Store, name, password string) bool {
	t.Helper()
	c, err := opaque.DefaultConfiguration().Client()
	if err != nil {
		t.Fatal(err)
	}
	ke1, err := c.GenerateKE1([]byte(password))
	if err != nil {
		t.Fatal(err)
	}
	start, err := st.StartOpaque(context.Background(), name, ke1.Serialize())
	if err != nil {
		t.Fatal(err)
	}
	ke2, err := c.Deserialize.KE2(start.KE2)
	if err != nil {
		t.Fatal(err)
	}
	m, tt, p, _ := store.ParseKSF(start.KSF)
	ke3, _, _, err := c.GenerateKE3(ke2, nil, nil, store.BrowserKSF(m, tt, p))
	return err == nil && st.FinishOpaque(start, ke3.Serialize()) == nil
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
	if !signsIn(t, st, "anna", "pw") || anna.AuthVersion != 2 {
		t.Fatalf("anna does not sign in by OPAQUE: auth_version %d", anna.AuthVersion)
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
	if !signsIn(t, st, "anna", "new") || signsIn(t, st, "anna", "pw") {
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

// user tokens lists a user's live sessions and keys with the ids that
// user revoke takes, so that one leaked key can go alone.
func TestUserTokens(t *testing.T) {
	clearCubeEnv(t)
	t.Setenv(env.DataVar, t.TempDir())
	if _, err := runUserWith(t, "", "add", "anna", "-no-password"); err != nil {
		t.Fatal(err)
	}
	var keys []string
	for _, label := range []string{"Loxone", "laptop"} {
		out, err := runUserWith(t, "", "token", "anna", "-label", label)
		if err != nil {
			t.Fatal(err)
		}
		keys = append(keys, strings.TrimSpace(out))
	}
	out, err := runUserWith(t, "", "tokens", "anna")
	if err != nil {
		t.Fatalf("tokens: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) != 3 || !strings.HasPrefix(lines[0], "ID") || !strings.Contains(lines[1], "Loxone") || !strings.Contains(lines[2], "laptop") ||
		!strings.Contains(lines[1], "api") || !strings.Contains(lines[1], "never") {
		t.Fatalf("tokens:\n%s", out)
	}
	for _, k := range keys {
		if strings.Contains(out, k) {
			t.Fatal("a key's secret listed")
		}
	}
	id := strings.Fields(lines[1])[0]
	if out, err := runUserWith(t, "", "revoke", "anna", id); err != nil || !strings.Contains(out, "1 token(s)") {
		t.Fatalf("revoke %s: %q %v", id, out, err)
	}
	if out, _ := runUserWith(t, "", "tokens", "anna"); strings.Contains(out, "Loxone") || !strings.Contains(out, "laptop") {
		t.Fatalf("after the revoke:\n%s", out)
	}
	if _, err := runUserWith(t, "", "tokens", "nobody"); err == nil {
		t.Fatal("tokens of nobody")
	}
}

// Each user command that changed something writes a change line of the
// server's, by the command line, naming the user as they are then; the
// key a token command prints is in none (AP Logging v1.1, Block 6).
func TestUserCommandsLog(t *testing.T) {
	clearCubeEnv(t)
	dir := t.TempDir()
	t.Setenv(env.DataVar, dir)
	if _, err := runUserWith(t, "", "add", "anna", "-no-password", "-display-name", "Anna"); err != nil {
		t.Fatal(err)
	}
	if _, err := runUserWith(t, "", "disable", "anna"); err != nil {
		t.Fatal(err)
	}
	out, err := runUserWith(t, "", "token", "anna", "-label", "phone")
	if err != nil {
		t.Fatal(err)
	}
	st, err := store.OpenReadOnly(dir, store.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	rows, err := st.Events(context.Background(), store.EventQuery{Kind: "change", Limit: 10})
	if err != nil || len(rows) != 3 {
		t.Fatalf("%d change lines, %v", len(rows), err)
	}
	var actions []string
	for i := len(rows) - 1; i >= 0; i-- {
		r := rows[i]
		var ev struct {
			Source  string
			Summary map[string]any
		}
		if err := json.Unmarshal(r.Data, &ev); err != nil {
			t.Fatal(err)
		}
		if r.CubeID != 0 || r.Source != "config" || ev.Source != "config" || ev.Summary["via"] != "cli" || r.UserID != 0 {
			t.Fatalf("line %d: %+v %s", i, r, r.Data)
		}
		actions = append(actions, ev.Summary["action"].(string))
		if ev.Summary["action"] == "user.add" {
			subject, _ := ev.Summary["subject"].(map[string]any)
			if subject["name"] != "anna" || subject["display_name"] != "Anna" || subject["uuid"] == nil {
				t.Fatalf("the user added: %v", ev.Summary)
			}
		}
		for _, line := range strings.Fields(out) {
			if len(line) > 20 && strings.Contains(string(r.Data), line) {
				t.Fatalf("the printed key %q in the log: %s", line, r.Data)
			}
		}
	}
	if strings.Join(actions, ",") != "user.add,user.update,token.add" {
		t.Fatalf("the actions: %v", actions)
	}
}

// Help, and a command with wrong arguments, print the usage without
// opening the store, which would create it with its keys: here in the
// current directory, the data directory when no -data is given.
func TestHelpCreatesNoStore(t *testing.T) {
	clearCubeEnv(t)
	dir := t.TempDir()
	t.Chdir(dir)
	t.Setenv(env.DataVar, "")
	run := map[string]func([]string) error{
		"user":  func(args []string) error { _, err := runUserWith(t, "", args...); return err },
		"store": runStore,
	}
	for _, c := range []struct {
		help bool
		args string
	}{
		{true, "user -h"}, {true, "user help"}, {true, "user --help"},
		{true, "user add -h"}, {true, "user add anna -h"}, {true, "user list -h"},
		{true, "user passwd -h"}, {true, "user disable -h"}, {true, "user enable -h"},
		{true, "user token -h"}, {true, "user tokens -h"}, {true, "user revoke -h"},
		{true, "store -h"}, {true, "store help"}, {true, "store backup -h"},
		{true, "store migrate -h"}, {true, "store info -h"},
		{false, "user"}, {false, "user bogus"}, {false, "user add"},
		{false, "user add -x anna"}, {false, "user add anna -no-password -expires tomorrow"},
		{false, "user list extra"}, {false, "user passwd"}, {false, "user disable"},
		{false, "user enable anna bob"}, {false, "user token anna"}, {false, "user tokens"},
		{false, "user revoke"}, {false, "user revoke anna x"},
		{false, "store"}, {false, "store bogus"}, {false, "store backup"},
		{false, "store migrate extra"}, {false, "store info extra"},
	} {
		f := strings.Fields(c.args)
		err := run[f[0]](f[1:])
		switch {
		case c.help && err != nil && !errors.Is(err, flag.ErrHelp):
			t.Errorf("%s: %v", c.args, err)
		case !c.help && err == nil:
			t.Errorf("%s: no error", c.args)
		}
		left, err := os.ReadDir(dir)
		if err != nil {
			t.Fatal(err)
		}
		if len(left) > 0 {
			t.Fatalf("%s left %s in the directory", c.args, left[0].Name())
		}
	}
}

// massimo user and store migrate create no store in a directory nobody
// named: the current directory without massimo.db. They name the
// service's command where the service is installed, -data where not;
// a change says which store it was written to, and a store other than
// the service's draws a warning. The service's directory is the test's
// (serviceFiles), which Windows does not use.
func TestUserStoreDir(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the service's store is under %ProgramData% on Windows")
	}
	clearCubeEnv(t)
	_, stateDir := serviceFiles(t)
	cwd := t.TempDir()
	t.Chdir(cwd)
	var errOut bytes.Buffer
	stderr = &errOut
	t.Cleanup(func() { stderr = os.Stderr })
	empty := func(what string) {
		t.Helper()
		if left, _ := os.ReadDir(cwd); len(left) > 0 {
			t.Fatalf("%s left %s in the current directory", what, left[0].Name())
		}
	}

	_, err := runUserWith(t, "pw\n", "add", "anna", "-password-stdin")
	if err == nil || !strings.Contains(err.Error(), "user add: no massimo.db in the current directory") ||
		!strings.Contains(err.Error(), stateDir) || !strings.Contains(err.Error(), "sudo -u _massimo") {
		t.Fatalf("add with the service installed: %v", err)
	}
	empty("add")
	if err := runStore([]string{"migrate"}); err == nil || !strings.Contains(err.Error(), "store migrate: no massimo.db") {
		t.Fatalf("migrate: %v", err)
	}
	empty("migrate")
	if err := os.Remove(stateDir); err != nil {
		t.Fatal(err)
	}
	if _, err := runUserWith(t, "", "list"); err == nil || !strings.Contains(err.Error(), "give -data") {
		t.Fatalf("list without a service: %v", err)
	}
	empty("list")
	if err := os.Mkdir(stateDir, 0o750); err != nil {
		t.Fatal(err)
	}

	other := t.TempDir()
	t.Setenv(env.DataVar, other)
	out, err := runUserWith(t, "pw\n", "add", "anna", "-password-stdin")
	if want := "(store " + filepath.Join(other, store.FileName) + ")"; err != nil || !strings.Contains(out, want) {
		t.Fatalf("add with MASSIMO_DATA: %q %v, want %q", out, err, want)
	}
	if w := errOut.String(); !strings.Contains(w, "warning: "+filepath.Join(other, store.FileName)+" is not the service's store, which is in "+stateDir) {
		t.Fatalf("the warning: %q", w)
	}
	errOut.Reset()
	if _, err := runUserWith(t, "", "token", "anna", "-label", "x"); err != nil || !strings.Contains(errOut.String(), "API key 1 of anna (store ") {
		t.Fatalf("token: %v %q", err, errOut.String())
	}

	// A store already in the current directory is used, as a
	// developer's checkout has one; the service's own draws no warning.
	t.Setenv(env.DataVar, cwd)
	if _, err := runUserWith(t, "", "add", "bob", "-no-password"); err != nil {
		t.Fatal(err)
	}
	t.Setenv(env.DataVar, "")
	if out, err := runUserWith(t, "", "disable", "bob"); err != nil || !strings.Contains(out, string(filepath.Separator)+store.FileName+")") {
		t.Fatalf("disable in the current directory's store: %q %v", out, err)
	}
	t.Setenv(env.DataVar, stateDir)
	errOut.Reset()
	if _, err := runUserWith(t, "", "add", "carl", "-no-password"); err != nil || errOut.Len() > 0 {
		t.Fatalf("the service's store: %v %q", err, errOut.String())
	}
}
