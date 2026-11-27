// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package main

// massimo store restore <file|id> -scope <list> -key <file> [-identity]
// [-env] [-tls] [-plan]: a server backup restored by scope (AP
// ServerBackup v1.0, Block 5; docs/store.md, "Restore by scope"). Through
// a running serve, its API with MASSIMO_API_TOKEN, where one answers, a
// file uploaded first, so that the restore is that server's, which then
// restarts; on the store otherwise, with serve stopped. The key is read
// into memory and written nowhere.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"go.metaworx.ch/massimo/internal/env"
	"go.metaworx.ch/massimo/internal/serve"
	"go.metaworx.ch/massimo/internal/serverbackup"
	"go.metaworx.ch/massimo/internal/store"
)

// maxKeyFile bounds the key file a restore reads.
const maxKeyFile = 1 << 20

// restoreArgs are store restore's options.
type restoreArgs struct {
	scopes   []string
	keyFile  string
	identity bool
	env      bool
	tls      bool
	plan     bool
}

// cliCube is a Cube of a restore or its plan.
type cliCube struct {
	Serial string `json:"serial"`
	Name   string `json:"name"`
	Live   bool   `json:"live"`
	Backup string `json:"backup"`
}

// cliRestore is a restore's plan or its outcome, as the API answers it
// and the command prints it.
type cliRestore struct {
	Backup struct {
		UUID   string    `json:"uuid"`
		Time   time.Time `json:"time"`
		Label  string    `json:"label"`
		Schema int       `json:"schema"`
	} `json:"backup"`
	Identity struct {
		ThisBox     bool   `json:"this_box"`
		Fingerprint string `json:"fingerprint"`
		Answers     *bool  `json:"answers"`
	} `json:"identity"`
	Migrated bool                       `json:"migrated"`
	Scopes   []string                   `json:"scopes"`
	Serials  []string                   `json:"serials"`
	Cubes    []cliCube                  `json:"cubes"`
	Files    []serverbackup.PlannedFile `json:"files"`
	Report   store.RestoreReport        `json:"report"`
	Refused  string                     `json:"refused"`
	Restart  struct {
		Started bool   `json:"started"`
		By      string `json:"by"`
		Blocked string `json:"blocked"`
	} `json:"restart"`
	Before string `json:"before_backup"`
}

// readKeyFile reads a key file into memory and checks that it holds a
// key; it is never written.
func readKeyFile(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, maxKeyFile+1))
	if err != nil {
		return nil, err
	}
	if len(b) > maxKeyFile {
		return nil, errors.New("the key file is too long")
	}
	if _, err := serverbackup.ParseIdentities(b); err != nil {
		return nil, err
	}
	return b, nil
}

// restore is massimo store restore: ref a backup file, or a backup's id
// as massimo store backups lists it.
func (c *backupCLI) restore(ctx context.Context, ref string, a restoreArgs) error {
	key, err := readKeyFile(a.keyFile)
	if err != nil {
		return fmt.Errorf("store restore: %w", err)
	}
	if _, err := serverbackup.ParseScopes(a.scopes); err != nil {
		return fmt.Errorf("store restore: %w", err)
	}
	file := ""
	if fi, err := os.Stat(ref); err == nil && fi.Mode().IsRegular() {
		file = ref
	}
	if c.api != nil {
		return c.restoreThroughServe(ctx, ref, file, key, a)
	}
	uuid := ""
	if file == "" {
		if uuid, err = c.resolve(ctx, ref); err != nil {
			return fmt.Errorf("store restore: %w", err)
		}
	}
	st, err := c.open(true)
	if err != nil {
		return err
	}
	defer st.Close()
	o := serverbackup.RestoreOptions{Backup: uuid, File: file, Key: key, Scopes: a.scopes, Identity: a.identity, Env: a.env, TLS: a.tls,
		TLSPlace: serve.RestoreTLSPlace(serve.TLS{Mode: os.Getenv(env.TLS), Cert: os.Getenv(env.TLSCert), Key: os.Getenv(env.TLSKey)}, c.dir)}
	cat := c.catalogue(st)
	if a.plan {
		p, err := cat.Plan(ctx, o)
		if err != nil {
			return fmt.Errorf("store restore: %w", err)
		}
		c.printRestore(planOf(p, st), true)
		return nil
	}
	res, err := cat.Restore(ctx, o)
	if errors.Is(err, store.ErrRestoreMissing) {
		c.printMissing(res.Report)
	}
	if err != nil {
		return fmt.Errorf("store restore: %w", err)
	}
	r := cliRestore{Scopes: res.Selection.Scopes, Serials: res.Selection.Serials, Files: res.Files, Report: res.Report, Before: res.Before.UUID}
	for _, cb := range res.Report.Cubes {
		r.Cubes = append(r.Cubes, cliCube{Serial: cb.Serial, Name: cb.Name, Live: cb.Live, Backup: cb.Backup})
	}
	r.Restart.Blocked = "no serve runs; start it"
	c.printRestore(r, false)
	return nil
}

// planOf is a plan of the catalogue as the command prints it.
func planOf(p serverbackup.RestorePlan, st *store.Store) cliRestore {
	var r cliRestore
	r.Backup.UUID, r.Backup.Time, r.Backup.Label, r.Backup.Schema = p.Manifest.UUID, p.Manifest.Created, p.Manifest.Label, p.Manifest.Schema.Version
	r.Identity.ThisBox, r.Identity.Fingerprint = p.ThisBox, p.Manifest.Node.Fingerprint
	r.Migrated, r.Scopes, r.Serials, r.Files, r.Report, r.Refused = p.Migrated, p.Selection.Scopes, p.Selection.Serials, p.Files, p.Report, p.Refused
	for _, cb := range p.Cubes {
		r.Cubes = append(r.Cubes, cliCube{Serial: cb.Serial, Name: cb.Name, Live: cb.Live})
	}
	return r
}

// restoreThroughServe restores through the API: a file uploaded first.
func (c *backupCLI) restoreThroughServe(ctx context.Context, ref, file string, key []byte, a restoreArgs) error {
	uuid := ""
	var err error
	if file != "" {
		if uuid, err = c.api.upload(ctx, file); err != nil {
			return fmt.Errorf("store restore: the upload: %w", err)
		}
		fmt.Fprintf(c.out, "uploaded %s as %s\n", file, shortBackupID(uuid))
	} else if uuid, err = c.resolve(ctx, ref); err != nil {
		return fmt.Errorf("store restore: %w", err)
	}
	body := map[string]any{"key_file": string(key), "scopes": a.scopes, "take_identity": a.identity, "env": a.env, "tls": a.tls}
	path := "/api/v1/server-backups/" + url.PathEscape(uuid) + "/restore"
	if a.plan {
		path += "/plan"
	}
	var r cliRestore
	if err := c.api.do(ctx, http.MethodPost, path, body, &r); err != nil {
		return fmt.Errorf("store restore: %w", err)
	}
	c.printRestore(r, a.plan)
	return nil
}

// upload sends a backup file to the server's local target and answers
// its uuid.
func (a *serveAPI) upload(ctx context.Context, path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.base+"/api/v1/server-backups/upload", f)
	if err != nil {
		return "", err
	}
	req.ContentLength = fi.Size()
	req.Header.Set("Content-Type", "application/zip")
	if a.token != "" {
		req.Header.Set("Authorization", "Bearer "+a.token)
	}
	resp, err := a.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("the server at %s: %w", a.base, err)
	}
	defer resp.Body.Close()
	var ans struct {
		Backup struct {
			UUID string `json:"uuid"`
		} `json:"backup"`
		Error string `json:"error"`
	}
	json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&ans)
	if resp.StatusCode/100 != 2 {
		return "", fmt.Errorf("the server at %s answered %d: %s", a.base, resp.StatusCode, ans.Error)
	}
	return ans.Backup.UUID, nil
}

// printMissing prints the rows a refused restore misses.
func (c *backupCLI) printMissing(rep store.RestoreReport) {
	if len(rep.Missing) == 0 {
		return
	}
	fmt.Fprintln(c.out, "missing here, named by the backup's rows; restore their scopes too:")
	for _, m := range rep.Missing {
		fmt.Fprintf(c.out, "  %s %s (%s), named by %s\n", m.Table, m.Name, m.UUID, m.By)
	}
}

// printRestore prints a plan or a restore's outcome.
func (c *backupCLI) printRestore(r cliRestore, plan bool) {
	if plan {
		who := "this box"
		if !r.Identity.ThisBox {
			who = "another box (" + r.Identity.Fingerprint + ")"
			if r.Identity.Answers != nil && *r.Identity.Answers {
				who += ", which still answers on the network"
			}
		}
		fmt.Fprintf(c.out, "backup %s of %s, made by %s, schema %d", shortBackupID(r.Backup.UUID), r.Backup.Time.Local().Format("2006-01-02 15:04"), who, r.Backup.Schema)
		if r.Migrated {
			fmt.Fprint(c.out, ", migrated to this one's")
		}
		fmt.Fprintf(c.out, " (%s)\n", c.where())
	} else {
		fmt.Fprintf(c.out, "restored %s (%s)\n", strings.Join(r.Scopes, ", "), c.where())
		if r.Before != "" {
			fmt.Fprintf(c.out, "  the store before it is backup %s, pinned for 30 days\n", shortBackupID(r.Before))
		}
	}
	if len(r.Serials) > 0 {
		fmt.Fprintf(c.out, "  Cubes: %s\n", strings.Join(r.Serials, ", "))
	}
	for _, tb := range r.Report.Tables {
		if tb.Restored == 0 && tb.Removed == 0 {
			continue
		}
		fmt.Fprintf(c.out, "  %-13s %-22s %d from the backup, %d replaced, %d removed", tb.Scope, tb.Table, tb.Restored, tb.Replaced, tb.Removed)
		if tb.KeptRemoved > 0 {
			fmt.Fprintf(c.out, " (%d kept, marked removed)", tb.KeptRemoved)
		}
		fmt.Fprintln(c.out)
	}
	for _, cl := range r.Report.Cleared {
		fmt.Fprintf(c.out, "  %s.%s cleared in %d rows\n", cl.Table, cl.Column, cl.Rows)
	}
	for _, rm := range r.Report.Removed {
		fmt.Fprintf(c.out, "  %s: %d rows removed with what they belonged to\n", rm.Table, rm.Rows)
	}
	if r.Report.Lost > 0 {
		fmt.Fprintf(c.out, "  %d sealed secrets did not open; enter them again\n", r.Report.Lost)
	}
	for _, f := range r.Files {
		fmt.Fprintf(c.out, "  %s → %s", f.Name, f.To)
		if f.Before != "" {
			fmt.Fprintf(c.out, " (the file before: %s)", f.Before)
		}
		fmt.Fprintln(c.out)
	}
	for _, cb := range r.Cubes {
		switch {
		case plan:
			here := ""
			if cb.Live {
				here = ", here too"
			}
			fmt.Fprintf(c.out, "  Cube %s %q%s\n", cb.Serial, cb.Name, here)
		case cb.Backup != "":
			fmt.Fprintf(c.out, "  Cube %s: restored in the store, not on the Cube; the Cube restore writes its configuration there, with its radio cost\n", cb.Serial)
		}
	}
	c.printMissing(r.Report)
	if plan {
		if r.Refused != "" {
			fmt.Fprintf(c.out, "refused: %s\n", r.Refused)
		}
		return
	}
	switch {
	case r.Restart.Started:
		fmt.Fprintf(c.out, "the server restarts (%s)\n", r.Restart.By)
	case r.Restart.Blocked != "":
		fmt.Fprintf(c.out, "the server does not restart by itself: %s\n", r.Restart.Blocked)
	}
}
