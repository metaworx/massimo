// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package main

// massimo user: the users of serve's store from the command line
// (docs/store.md section 6, "The commands"). It opens the store
// directly, beside a running server thanks to WAL, and takes -data like
// serve; for the service it runs as the service user. The audit records
// what it does with no user and the remote "cli".

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"golang.org/x/term"

	"go.metaworx.ch/massimo/internal/store"
)

// stdin and stdout are the command's streams, replaced by the tests.
var (
	stdin  io.Reader = os.Stdin
	stdout io.Writer = os.Stdout
)

func userUsage() {
	fmt.Fprint(os.Stderr, `usage: massimo [-data <dir>] user <command> [options] [args]

  add <name> [-role <key>] [-display-name <text>] [-expires <time>] [-no-password | -password-stdin]
                     create a user; the password is asked twice on the terminal
  list               every user with its role, state, password and tokens
  passwd <name> [-password-stdin]
                     set a user's password, which ends its sessions
  disable <name>     refuse the user everything, keeping its rows
  enable <name>
  token <name> -label <text>
                     create an API key for the user and print it once
  tokens <name>      the user's live sessions and keys with their ids
  revoke <name> [<token id>]
                     revoke one session or key of the user (the id of tokens), or every one

The roles are owner, manager, operator, guest, viewer, integration and the
ones created through the API (viewer when -role is absent); -expires takes
an RFC 3339 time, the end of a guest. -password-stdin reads the password
from the first line of standard input, for scripts.
The data directory is -data, else MASSIMO_DATA, else the current directory;
for the service run it as its user, for example
  sudo -u massimo massimo -data /var/lib/massimo user add anna -role operator
`)
}

// runUser handles massimo user.
func runUser(args []string) error {
	if len(args) == 0 {
		userUsage()
		return errors.New("user: which command?")
	}
	ctx := context.Background()
	logger, err := newLogger(os.Stderr)
	if err != nil {
		return err
	}
	st, err := store.Open(dataDir(), store.Options{Version: versionString(), Logger: logger})
	if err != nil {
		return err
	}
	defer st.Close()
	u := userCmd{st: st, ctx: ctx}
	switch args[0] {
	case "add":
		return u.add(args[1:])
	case "list":
		return u.list(args[1:])
	case "passwd":
		return u.passwd(args[1:])
	case "disable", "enable":
		return u.setDisabled(args[1:], args[0] == "disable")
	case "token":
		return u.token(args[1:])
	case "tokens":
		return u.tokens(args[1:])
	case "revoke":
		return u.revoke(args[1:])
	case "help", "-h", "--help":
		userUsage()
		return nil
	}
	userUsage()
	return fmt.Errorf("user: unknown command %q", args[0])
}

type userCmd struct {
	st  *store.Store
	ctx context.Context
}

// flags parses a subcommand's flags, which may come before or after the
// name: "user add anna -role operator" and "user add -role operator anna".
func (u userCmd) flags(cmd string, args []string, fs *flag.FlagSet) ([]string, error) {
	fs.Usage = userUsage
	var names []string
	for len(args) > 0 {
		if err := fs.Parse(args); err != nil {
			return nil, err
		}
		args = fs.Args()
		if len(args) > 0 {
			names = append(names, args[0])
			args = args[1:]
		}
	}
	return names, nil
}

// audit records what the command did.
func (u userCmd) audit(action, target string, detail map[string]any, err error) {
	if detail == nil {
		detail = map[string]any{}
	}
	result := store.ResultOK
	if err != nil {
		result, detail["error"] = store.ResultFailed, err.Error()
	}
	raw, _ := json.Marshal(detail)
	if _, aerr := u.st.Audit(u.ctx, store.AuditEntry{Remote: store.RemoteCLI, Action: action, Target: target, Detail: raw, Result: result}); aerr != nil {
		fmt.Fprintln(os.Stderr, "warning: audit row not written:", aerr)
	}
}

// readPassword asks for a password twice on the terminal, or reads it
// from standard input with -password-stdin.
func readPassword(fromStdin bool) (string, error) {
	if fromStdin {
		line, err := bufio.NewReader(stdin).ReadString('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			return "", err
		}
		pw := strings.TrimRight(line, "\r\n")
		if pw == "" {
			return "", errors.New("no password on standard input")
		}
		return pw, nil
	}
	f, ok := stdin.(*os.File)
	if !ok || !term.IsTerminal(int(f.Fd())) {
		return "", errors.New("no terminal to ask for the password on: give -password-stdin, or -no-password for a user with keys only")
	}
	ask := func(prompt string) (string, error) {
		fmt.Fprint(os.Stderr, prompt)
		b, err := term.ReadPassword(int(f.Fd()))
		fmt.Fprintln(os.Stderr)
		return string(b), err
	}
	pw, err := ask("Password: ")
	if err != nil {
		return "", err
	}
	again, err := ask("Again: ")
	if err != nil {
		return "", err
	}
	if pw != again {
		return "", errors.New("the passwords differ")
	}
	if pw == "" {
		return "", errors.New("an empty password; give -no-password for a user with keys only")
	}
	return pw, nil
}

func (u userCmd) add(args []string) error {
	fs := flag.NewFlagSet("user add", flag.ContinueOnError)
	role := fs.String("role", store.RoleViewer, "the user's role")
	display := fs.String("display-name", "", "the name shown")
	expires := fs.String("expires", "", "the end of a guest, RFC 3339")
	noPassword := fs.Bool("no-password", false, "a user that logs in with keys only")
	fromStdin := fs.Bool("password-stdin", false, "read the password from standard input")
	names, err := u.flags("add", args, fs)
	if err != nil {
		return err
	}
	if len(names) != 1 {
		userUsage()
		return errors.New("user add: give the name")
	}
	nu := store.User{Name: names[0], DisplayName: *display, Role: *role}
	if *expires != "" {
		if nu.ExpiresAt, err = time.Parse(time.RFC3339, *expires); err != nil {
			return fmt.Errorf("user add: -expires: %w", err)
		}
	}
	password := ""
	if !*noPassword {
		if password, err = readPassword(*fromStdin); err != nil {
			return fmt.Errorf("user add: %w", err)
		}
	}
	created, err := u.st.AddUser(u.ctx, nu, password, 0)
	u.audit("user.add", "user:"+store.NormalName(names[0]), map[string]any{"role": *role, "password": password != ""}, err)
	if err != nil {
		return fmt.Errorf("user add: %w", err)
	}
	fmt.Fprintf(stdout, "user %s added with the role %s\n", created.Name, created.Role)
	return nil
}

func (u userCmd) list(args []string) error {
	if len(args) > 0 {
		userUsage()
		return errors.New("user list takes no argument")
	}
	users, err := u.st.Users(u.ctx)
	if err != nil {
		return err
	}
	w := tabwriter.NewWriter(stdout, 0, 8, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tROLE\tSTATE\tPASSWORD\tTOKENS\tLAST LOGIN")
	now := time.Now()
	for _, us := range users {
		state := "enabled"
		switch {
		case us.Disabled:
			state = "disabled"
		case us.Expired(now):
			state = "expired " + us.ExpiresAt.Local().Format("2006-01-02")
		case !us.ExpiresAt.IsZero():
			state = "until " + us.ExpiresAt.Local().Format("2006-01-02")
		}
		password := "no"
		if us.HasPassword {
			password = "yes"
		}
		tokens, err := u.st.Tokens(u.ctx, us.ID)
		if err != nil {
			return err
		}
		last := "never"
		if !us.LastLoginAt.IsZero() {
			last = us.LastLoginAt.Local().Format("2006-01-02 15:04")
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%d\t%s\n", us.Name, us.Role, state, password, len(tokens), last)
	}
	return w.Flush()
}

func (u userCmd) passwd(args []string) error {
	fs := flag.NewFlagSet("user passwd", flag.ContinueOnError)
	fromStdin := fs.Bool("password-stdin", false, "read the password from standard input")
	names, err := u.flags("passwd", args, fs)
	if err != nil {
		return err
	}
	if len(names) != 1 {
		userUsage()
		return errors.New("user passwd: give the name")
	}
	us, ok, err := u.st.User(u.ctx, names[0])
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("user passwd: no user %q", names[0])
	}
	password, err := readPassword(*fromStdin)
	if err != nil {
		return fmt.Errorf("user passwd: %w", err)
	}
	err = u.st.SetPassword(u.ctx, us.Name, password, 0)
	if err == nil {
		err = u.st.RevokeSessions(u.ctx, us.ID)
	}
	u.audit("user.update", "user:"+us.Name, map[string]any{"password": true}, err)
	if err != nil {
		return fmt.Errorf("user passwd: %w", err)
	}
	fmt.Fprintf(stdout, "password of %s set; its sessions are ended\n", us.Name)
	return nil
}

func (u userCmd) setDisabled(args []string, disabled bool) error {
	verb := "enable"
	if disabled {
		verb = "disable"
	}
	if len(args) != 1 {
		userUsage()
		return fmt.Errorf("user %s: give the name", verb)
	}
	us, err := u.st.UpdateUser(u.ctx, args[0], func(c *store.User) error { c.Disabled = disabled; return nil }, 0)
	u.audit("user.update", "user:"+store.NormalName(args[0]), map[string]any{"disabled": disabled}, err)
	if err != nil {
		return fmt.Errorf("user %s: %w", verb, err)
	}
	fmt.Fprintf(stdout, "user %s %sd\n", us.Name, verb)
	return nil
}

func (u userCmd) token(args []string) error {
	fs := flag.NewFlagSet("user token", flag.ContinueOnError)
	label := fs.String("label", "", "what the key is for (required)")
	names, err := u.flags("token", args, fs)
	if err != nil {
		return err
	}
	if len(names) != 1 || strings.TrimSpace(*label) == "" {
		userUsage()
		return errors.New("user token: give the name and -label")
	}
	us, ok, err := u.st.User(u.ctx, names[0])
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("user token: no user %q", names[0])
	}
	tok, secret, err := u.st.AddToken(u.ctx, us.ID, store.TokenAPI, strings.TrimSpace(*label), time.Time{})
	target := "user:" + us.Name
	if err == nil {
		target = "token:" + strconv.FormatInt(tok.ID, 10)
	}
	u.audit("token.add", target, map[string]any{"user": us.Name, "label": strings.TrimSpace(*label)}, err)
	if err != nil {
		return fmt.Errorf("user token: %w", err)
	}
	fmt.Fprintf(os.Stderr, "API key %d of %s, shown once; send it as \"Authorization: Bearer <key>\":\n", tok.ID, us.Name)
	fmt.Fprintln(stdout, secret)
	return nil
}

// tokens lists a user's live sessions and keys, oldest first, with the
// ids revoke takes; never their secrets, which the store does not have.
func (u userCmd) tokens(args []string) error {
	if len(args) != 1 {
		userUsage()
		return errors.New("user tokens: give the name")
	}
	us, ok, err := u.st.User(u.ctx, args[0])
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("user tokens: no user %q", args[0])
	}
	tokens, err := u.st.Tokens(u.ctx, us.ID)
	if err != nil {
		return err
	}
	when := func(t time.Time) string {
		if t.IsZero() {
			return "never"
		}
		return t.Local().Format("2006-01-02 15:04")
	}
	w := tabwriter.NewWriter(stdout, 0, 8, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tKIND\tLABEL\tCREATED\tLAST USED\tFROM\tEXPIRES")
	for _, t := range tokens {
		from := t.LastUsedFrom
		if from == "" {
			from = "-"
		}
		fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%s\t%s\t%s\n", t.ID, t.Kind, t.Label, when(t.CreatedAt), when(t.LastUsedAt), from, when(t.ExpiresAt))
	}
	return w.Flush()
}

func (u userCmd) revoke(args []string) error {
	if len(args) < 1 || len(args) > 2 {
		userUsage()
		return errors.New("user revoke: give the name, and a token id to revoke one")
	}
	us, ok, err := u.st.User(u.ctx, args[0])
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("user revoke: no user %q", args[0])
	}
	tokens, err := u.st.Tokens(u.ctx, us.ID)
	if err != nil {
		return err
	}
	if len(args) == 2 {
		id, err := strconv.ParseInt(args[1], 10, 64)
		if err != nil {
			return fmt.Errorf("user revoke: bad token id %q", args[1])
		}
		found := false
		for _, t := range tokens {
			found = found || t.ID == id
		}
		if !found {
			return fmt.Errorf("user revoke: %s has no live token %d", us.Name, id)
		}
		tokens = []store.Token{{ID: id}}
	}
	for _, t := range tokens {
		err := u.st.RevokeToken(u.ctx, t.ID)
		u.audit("token.revoke", "token:"+strconv.FormatInt(t.ID, 10), map[string]any{"user": us.Name}, err)
		if err != nil {
			return fmt.Errorf("user revoke: %w", err)
		}
	}
	fmt.Fprintf(stdout, "%d token(s) of %s revoked\n", len(tokens), us.Name)
	return nil
}
