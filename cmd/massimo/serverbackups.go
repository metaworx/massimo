// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package main

// massimo store backups, backup without a file, push and pull: the
// server's own backups (AP ServerBackup v1.0, Block 2; docs/store.md,
// "Server backups"). Each goes through this box's running serve where
// one answers (its API, with MASSIMO_API_TOKEN), so that the job is the
// server's, and acts on the store directly otherwise, under the jobs'
// lock that serve's jobs take too (serverbackup.LockJobs), so that the
// two never run at once. The command line had no API client: serveAPI is
// the smallest one. Its address is the one serve published in the data
// directory (internal/serve, serve.api), else MASSIMO_LISTEN's; the key
// goes only to a server whose /healthz names the node of the store in
// the data directory, this box's serve.

import (
	"bytes"
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"slices"
	"strings"
	"text/tabwriter"
	"time"

	"go.metaworx.ch/massimo/internal/env"
	"go.metaworx.ch/massimo/internal/install"
	"go.metaworx.ch/massimo/internal/serve"
	"go.metaworx.ch/massimo/internal/serverbackup"
	"go.metaworx.ch/massimo/internal/store"
)

// defaultListen is serve's -listen where MASSIMO_LISTEN is unset.
const defaultListen = "127.0.0.1:8099"

// serveAPI is a running serve's API as the command line reaches it.
type serveAPI struct {
	base   string // http(s)://host:port, no slash at its end
	token  string
	client *http.Client
}

// findServe finds this box's running serve of the data directory dir:
// at the address its serve.api names, else at listen (https with tls),
// its /healthz answering within two seconds with the node of the store
// in dir. The token goes to that server alone, never to one that names
// another node or none: other is the address of such a server, "" for
// none. nil where no serve of dir answers, or the store cannot be read.
func findServe(ctx context.Context, dir, listen string, tls bool, token string) (api *serveAPI, other string) {
	node := storeNode(dir)
	if node == "" {
		return nil, ""
	}
	var bases []string
	if f, err := serve.ReadAPIFile(dir); err == nil {
		bases = append(bases, f.URL)
	}
	if health, err := install.HealthURL(listen, tls); err == nil {
		if b := strings.TrimSuffix(health, "/healthz"); !slices.Contains(bases, b) {
			bases = append(bases, b)
		}
	}
	for _, base := range bases {
		client := healthClient(base + "/healthz")
		client.Timeout = 0
		switch got, err := healthNode(ctx, client, base); {
		case err != nil:
		case got == node:
			return &serveAPI{base: base, token: token, client: client}, ""
		default:
			other = base
		}
	}
	return nil, other
}

// storeNode is the node of the store in dir, "" where it cannot be read.
func storeNode(dir string) string {
	if _, err := os.Stat(store.Path(dir)); err != nil {
		return ""
	}
	st, err := store.OpenReadOnly(dir, store.Options{})
	if err != nil {
		return ""
	}
	defer st.Close()
	return st.NodeID()
}

// healthNode asks base's /healthz, without a key, for the node it serves.
func healthNode(ctx context.Context, client *http.Client, base string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/healthz", nil)
	if err != nil {
		return "", err
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var h struct {
		Node string `json:"node_id"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<16)).Decode(&h); err != nil {
		return "", err
	}
	return h.Node, nil
}

// do sends a request with a JSON body, nil for none, and reads a JSON
// answer into out, nil to drop it; an answer not 2xx is an error with
// the API's own text.
func (a *serveAPI) do(ctx context.Context, method, path string, body, out any) error {
	var rd io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		rd = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, a.base+path, rd)
	if err != nil {
		return err
	}
	if a.token != "" {
		req.Header.Set("Authorization", "Bearer "+a.token)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := a.client.Do(req)
	if err != nil {
		return fmt.Errorf("the server at %s: %w", a.base, err)
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(io.LimitReader(resp.Body, 64<<20))
	if err != nil {
		return fmt.Errorf("the server at %s: %w", a.base, err)
	}
	if resp.StatusCode/100 != 2 {
		var e struct {
			Error string `json:"error"`
		}
		if json.Unmarshal(b, &e) == nil && e.Error != "" {
			return fmt.Errorf("the server at %s answered %d: %s", a.base, resp.StatusCode, e.Error)
		}
		return fmt.Errorf("the server at %s answered %d", a.base, resp.StatusCode)
	}
	if out == nil {
		return nil
	}
	return json.Unmarshal(b, out)
}

// cliBackup is a backup as the commands print it, from the API or from
// the store.
type cliBackup struct {
	UUID        string     `json:"uuid"`
	Time        time.Time  `json:"time"`
	Trigger     string     `json:"trigger"`
	By          string     `json:"by"`
	State       string     `json:"state"`
	Size        int64      `json:"size"`
	Label       string     `json:"label"`
	PinnedUntil *time.Time `json:"pinned_until"`
	Error       string     `json:"error"`
	Tries       int        `json:"tries"`
	Copies      []cliCopy  `json:"copies"`
}

// cliCopy is a copy as the commands print it.
type cliCopy struct {
	TargetName string `json:"target_name"`
	State      string `json:"state"`
	Pulled     bool   `json:"pulled"`
	Tries      int    `json:"tries"`
	Error      string `json:"error"`
}

// cliStep is a step of a job as the commands print it.
type cliStep struct {
	Step   string `json:"step"`
	Target string `json:"target"`
	OK     bool   `json:"ok"`
	Error  string `json:"error"`
}

// backupCLI is what the server backups' commands run with.
type backupCLI struct {
	dir string    // the data directory
	api *serveAPI // a running serve's, nil for none
	out io.Writer
	// envFile and tls are what a backup made on the store carries.
	envFile string
	tls     []serverbackup.TLSFile
	log     *slog.Logger
	// lockWait is how long a job on the store waits for the jobs' lock
	// that another job holds: serve's, or another command's.
	lockWait time.Duration
}

// defaultLockWait is how long a command's job waits for the jobs' lock.
const defaultLockWait = 30 * time.Second

// newBackupCLI is the commands' setting: the data directory, a running
// serve where one answers, and the settings' env and TLS files.
func newBackupCLI(ctx context.Context, out io.Writer) *backupCLI {
	dir := dataDir()
	o := serve.TLS{Mode: os.Getenv(env.TLS), Cert: os.Getenv(env.TLSCert), Key: os.Getenv(env.TLSKey)}
	log, err := newLogger(os.Stderr)
	if err != nil {
		log = slog.Default()
	}
	listen := cmp.Or(os.Getenv("MASSIMO_LISTEN"), defaultListen)
	api, other := findServe(ctx, dir, listen, serve.TLSMode(o.Mode, o.Cert, o.Key) != "off", os.Getenv(tokenVar))
	if other != "" {
		fmt.Fprintf(os.Stderr, "a server answers at %s, but not the serve of %s: it gets no key, and the command acts on the store\n", other, dir)
	}
	return &backupCLI{dir: dir, api: api, out: out, envFile: env.File(), tls: serve.BackupTLSFiles(o, dir), log: log, lockWait: defaultLockWait}
}

// lockJobs takes the jobs' lock for a job on the store, waiting lockWait
// for another job's end.
func (c *backupCLI) lockJobs(ctx context.Context, what string) (func(), error) {
	if _, err := os.Stat(store.Path(c.dir)); err != nil {
		// no store, no lock file left behind
		return nil, storeError(c.dir, err)
	}
	ctx, cancel := context.WithTimeout(ctx, cmp.Or(c.lockWait, defaultLockWait))
	defer cancel()
	unlock, err := serverbackup.LockJobs(ctx, c.dir)
	if errors.Is(err, serverbackup.ErrJobRunning) {
		return nil, fmt.Errorf("store %s: another job of the server's backups runs on %s (serve's, or another command's); try again once it ends", what, c.dir)
	}
	return unlock, err
}

// open opens the store for a job, a read-only one for a listing; a
// directory without one is an error, never a store made.
func (c *backupCLI) open(write bool) (*store.Store, error) {
	if _, err := os.Stat(store.Path(c.dir)); err != nil {
		return nil, storeError(c.dir, err)
	}
	if !write {
		st, err := store.OpenReadOnly(c.dir, store.Options{})
		if err != nil {
			return nil, storeError(c.dir, err)
		}
		return st, nil
	}
	st, err := store.Open(c.dir, store.Options{Version: versionString(), Logger: c.log})
	if err != nil {
		return nil, storeError(c.dir, err)
	}
	return st, nil
}

// catalogue is the catalogue's jobs over st.
func (c *backupCLI) catalogue(st *store.Store) *serverbackup.Catalogue {
	return &serverbackup.Catalogue{Store: st, EnvFile: c.envFile, TLSFiles: c.tls, Version: versionString(), Log: c.log}
}

// where says which way a command went.
func (c *backupCLI) where() string {
	if c.api != nil {
		return "through the server at " + c.api.base
	}
	return "on the store in " + c.dir
}

// list is massimo store backups: the backups, newest first, each with
// its copies.
func (c *backupCLI) list(ctx context.Context) error {
	var bs []cliBackup
	if c.api != nil {
		var ans struct {
			Backups []cliBackup `json:"backups"`
		}
		if err := c.api.do(ctx, http.MethodGet, "/api/v1/server-backups", nil, &ans); err != nil {
			return fmt.Errorf("store backups: %w", err)
		}
		bs = ans.Backups
	} else {
		st, err := c.open(false)
		if err != nil {
			return err
		}
		defer st.Close()
		if bs, err = storeBackups(ctx, st); err != nil {
			return err
		}
	}
	if len(bs) == 0 {
		fmt.Fprintf(c.out, "no server backups (%s)\n", c.where())
		return nil
	}
	tw := tabwriter.NewWriter(c.out, 2, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "ID\tTIME\tTRIGGER\tSTATE\tSIZE\tCOPIES\tLABEL")
	for _, b := range bs {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n", shortBackupID(b.UUID), b.Time.Local().Format("2006-01-02 15:04"), b.Trigger, stateText(b),
			sizeText(b.Size), copiesText(b.Copies), b.Label)
	}
	return tw.Flush()
}

// storeBackups reads the catalogue as the API lists it.
func storeBackups(ctx context.Context, st *store.Store) ([]cliBackup, error) {
	rows, err := st.ServerBackups(ctx)
	if err != nil {
		return nil, err
	}
	ts, err := st.ServerBackupTargets(ctx)
	if err != nil {
		return nil, err
	}
	names := map[string]string{}
	for _, t := range ts {
		names[t.UUID] = t.Name
	}
	var out []cliBackup
	for _, b := range rows {
		cb := cliBackup{UUID: b.UUID, Time: b.Time, Trigger: b.Trigger, State: b.State, Size: b.Size, Error: b.Error, Tries: b.Tries}
		if len(b.Manifest) > 0 {
			if m, _, err := serverbackup.ParseManifest(serverbackup.Raw{Manifest: b.Manifest, Signature: []byte(b.Signature)}); err == nil {
				cb.Label = m.Label
			}
		}
		for _, cp := range b.Copies {
			cb.Copies = append(cb.Copies, cliCopy{TargetName: names[cp.TargetUUID], State: cp.State, Pulled: cp.Pulled, Tries: cp.Tries, Error: cp.Error})
		}
		out = append(out, cb)
	}
	return out, nil
}

// shortBackupID is a backup's uuid as the list prints it: its last
// twelve digits, the random part of a UUIDv7; the commands take it.
func shortBackupID(uuid string) string {
	if len(uuid) > 12 {
		return uuid[len(uuid)-12:]
	}
	return uuid
}

func stateText(b cliBackup) string {
	if b.State == store.ServerBackupFailed {
		return fmt.Sprintf("failed (try %d)", b.Tries)
	}
	if b.PinnedUntil != nil {
		return "made, pinned"
	}
	return "made"
}

func sizeText(n int64) string {
	switch {
	case n == 0:
		return "–"
	case n < 1<<20:
		return fmt.Sprintf("%.1f KiB", float64(n)/(1<<10))
	}
	return fmt.Sprintf("%.1f MiB", float64(n)/(1<<20))
}

func copiesText(cs []cliCopy) string {
	if len(cs) == 0 {
		return "–"
	}
	var parts []string
	for _, c := range cs {
		s := c.TargetName + ": " + c.State
		if c.Pulled {
			s += ", pulled"
		}
		if c.State == store.CopyFailed && c.Tries > 0 {
			s += fmt.Sprintf(" (try %d)", c.Tries)
		}
		parts = append(parts, s)
	}
	return strings.Join(parts, "; ")
}

// resolve finds the backup a uuid, or a uuid's end of at least eight
// digits as the list prints it, names.
func (c *backupCLI) resolve(ctx context.Context, id string) (string, error) {
	var bs []cliBackup
	if c.api != nil {
		var ans struct {
			Backups []cliBackup `json:"backups"`
		}
		if err := c.api.do(ctx, http.MethodGet, "/api/v1/server-backups", nil, &ans); err != nil {
			return "", err
		}
		bs = ans.Backups
	} else {
		st, err := c.open(false)
		if err != nil {
			return "", err
		}
		defer st.Close()
		if bs, err = storeBackups(ctx, st); err != nil {
			return "", err
		}
	}
	var found []string
	for _, b := range bs {
		if b.UUID == id || len(id) >= 8 && strings.HasSuffix(b.UUID, id) {
			found = append(found, b.UUID)
		}
	}
	switch len(found) {
	case 0:
		return "", fmt.Errorf("no server backup %q (massimo store backups lists them)", id)
	case 1:
		return found[0], nil
	}
	return "", fmt.Errorf("%q names %d server backups; give more of its id", id, len(found))
}

// printJob prints a job's backup and its steps.
func (c *backupCLI) printJob(what string, b cliBackup, steps []cliStep) {
	fmt.Fprintf(c.out, "%s %s (%s, %s), %s\n", what, shortBackupID(b.UUID), b.UUID, sizeText(b.Size), c.where())
	for _, s := range steps {
		switch {
		case !s.OK:
			fmt.Fprintf(c.out, "  %s %s: failed: %s\n", s.Step, s.Target, s.Error)
		case s.Step == "make":
		case s.Step == "prune":
			fmt.Fprintf(c.out, "  pruned %s\n", s.Target)
		default:
			fmt.Fprintf(c.out, "  %s %s: ok\n", s.Step, s.Target)
		}
	}
}

// stepsOf are a job's outcomes as the commands print them.
func stepsOf(outs []serverbackup.Outcome) []cliStep {
	var out []cliStep
	for _, o := range outs {
		s := cliStep{Step: o.Step, Target: o.Target, OK: o.Err == nil}
		if o.Err != nil {
			s.Error = o.Err.Error()
		}
		if o.Step == "prune" && o.Removed == 0 && o.Err == nil {
			continue
		}
		out = append(out, s)
	}
	return out
}

// one is a backup of the catalogue as the commands print it.
func one(ctx context.Context, st *store.Store, uuid string) cliBackup {
	bs, _ := storeBackups(ctx, st)
	for _, b := range bs {
		if b.UUID == uuid {
			return b
		}
	}
	return cliBackup{UUID: uuid}
}

// make is massimo store backup without a file: a backup now, to the
// target named, or to the targets that take it.
func (c *backupCLI) make(ctx context.Context, target, label string) error {
	if c.api != nil {
		var ans struct {
			Backup cliBackup `json:"backup"`
			Steps  []cliStep `json:"steps"`
		}
		if err := c.api.do(ctx, http.MethodPost, "/api/v1/server-backups", map[string]string{"target": target, "label": label}, &ans); err != nil {
			return fmt.Errorf("store backup: %w", err)
		}
		c.printJob("made", ans.Backup, ans.Steps)
		return nil
	}
	unlock, err := c.lockJobs(ctx, "backup")
	if err != nil {
		return err
	}
	defer unlock()
	st, err := c.open(true)
	if err != nil {
		return err
	}
	defer st.Close()
	b, outs, err := c.catalogue(st).Make(ctx, serverbackup.MakeRequest{Trigger: serverbackup.TriggerManual, Label: label, Target: target})
	if err != nil {
		return fmt.Errorf("store backup: %w", err)
	}
	c.printJob("made", one(ctx, st, b.UUID), stepsOf(outs))
	return nil
}

// copy is massimo store push or pull <id> <target>.
func (c *backupCLI) copy(ctx context.Context, id, target string, pull bool) error {
	what := "push"
	if pull {
		what = "pull"
	}
	uuid, err := c.resolve(ctx, id)
	if err != nil {
		return fmt.Errorf("store %s: %w", what, err)
	}
	if c.api != nil {
		method, path := http.MethodPut, "/api/v1/server-backups/"+url.PathEscape(uuid)+"/copies/"+url.PathEscape(target)
		if pull {
			method, path = http.MethodPost, path+"/pull"
		}
		var ans struct {
			Backup cliBackup `json:"backup"`
			Steps  []cliStep `json:"steps"`
		}
		if err := c.api.do(ctx, method, path, nil, &ans); err != nil {
			return fmt.Errorf("store %s: %w", what, err)
		}
		c.printJob(what+"ed", ans.Backup, ans.Steps)
		return nil
	}
	unlock, err := c.lockJobs(ctx, what)
	if err != nil {
		return err
	}
	defer unlock()
	st, err := c.open(true)
	if err != nil {
		return err
	}
	defer st.Close()
	cat := c.catalogue(st)
	var o serverbackup.Outcome
	if pull {
		_, o, err = cat.Pull(ctx, uuid, target)
	} else {
		_, o, err = cat.Push(ctx, uuid, target)
	}
	if err != nil {
		return fmt.Errorf("store %s: %w", what, err)
	}
	c.printJob(what+"ed", one(ctx, st, uuid), stepsOf([]serverbackup.Outcome{o}))
	return nil
}

// errBackupArgs is the error of a server backup command's arguments.
var errBackupArgs = errors.New("give the backup's id, as massimo store backups lists it, and the target's name")
