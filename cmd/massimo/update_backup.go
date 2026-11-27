// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package main

// The server's backup before massimo update installs a release by hand
// (AP ServerBackup v1.0, Block 10; docs/store.md, "Before an update").
// The update looks at the service's store where nobody names a data
// directory (updateStoreDir), and then at the service's settings too:
// root reads ./.env alone, which holds neither the service's key nor its
// address, so the service's settings file fills what the environment
// leaves unset (updateSettingsFor). This box's running serve takes the
// backup, POST /api/v1/update/backup, found and proved by its node as
// the server backups' commands find it (findServe), so that the key goes
// to it alone. Where none can be asked, no serve answering or the key
// refused, the store's keys decide, read without writing: none, a line
// and the update goes on; a key, the backup is made on the store, to the
// local targets, pinned for 30 days, under the jobs' lock, with the
// service's settings file and TLS files, by a process that may write the
// store: root never writes the store of the service's user, whose files
// it would leave root's, and stops instead, saying how to go on. A
// backup that fails stops the update; -no-backup updates without one.
// The update helper takes none: the server that wrote its request took
// it first.

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"runtime"
	"slices"

	"go.metaworx.ch/massimo/internal/env"
	"go.metaworx.ch/massimo/internal/install"
	"go.metaworx.ch/massimo/internal/serve"
	"go.metaworx.ch/massimo/internal/serverbackup"
	"go.metaworx.ch/massimo/internal/store"
)

// listenVar is the variable of serve's listen address.
const listenVar = "MASSIMO_LISTEN"

// updateSettings are the settings massimo update asks a serve with and
// makes a backup on the store with: the settings file a backup carries,
// the API key, serve's listen address and its TLS.
type updateSettings struct {
	envFile string
	token   string
	listen  string
	tls     serve.TLS
}

// serviceEnvFile is the service's settings file on this host: on
// Windows the installer's under %ProgramData%, elsewhere install's and
// the Debian package's.
func serviceEnvFile() string {
	if runtime.GOOS == "windows" {
		return install.WindowsPaths(os.Getenv("ProgramFiles"), os.Getenv("ProgramData")).Env
	}
	return serviceEnvPath
}

// updateSettingsFor are the settings for the store in dir: the
// environment's, and where dir is the service's because nobody named a
// data directory, and nobody named a settings file, the service's
// settings file's for what the environment leaves unset, and that file
// as the one a backup carries. A file that cannot be read adds nothing.
func updateSettingsFor(dir string) updateSettings {
	s := updateSettings{envFile: env.File(), token: os.Getenv(tokenVar), listen: os.Getenv(listenVar),
		tls: serve.TLS{Mode: os.Getenv(env.TLS), Cert: os.Getenv(env.TLSCert), Key: os.Getenv(env.TLSKey)}}
	if dir != "" && implicitDataDir() && os.Getenv(env.FileVar) == "" {
		if path := serviceEnvFile(); path != s.envFile {
			if vars, _, err := env.ReadFileLenient(path); err == nil {
				get := func(name string) string { return vars[env.NameKey(name)] }
				s.envFile = path
				s.token = cmp.Or(s.token, get(tokenVar))
				s.listen = cmp.Or(s.listen, get(listenVar))
				s.tls.Mode = cmp.Or(s.tls.Mode, get(env.TLS))
				s.tls.Cert = cmp.Or(s.tls.Cert, get(env.TLSCert))
				s.tls.Key = cmp.Or(s.tls.Key, get(env.TLSKey))
			}
		}
	}
	s.listen = cmp.Or(s.listen, defaultListen)
	return s
}

// backupBeforeUpdate takes the backup massimo update takes before it
// installs the release to, and says so on out: through this box's
// running serve, else on the store of updateStoreDir.
func backupBeforeUpdate(ctx context.Context, out io.Writer, to string) error {
	dir := updateStoreDir()
	set := updateSettingsFor(dir)
	log, err := newLogger(os.Stderr)
	if err != nil {
		log = slog.Default()
	}
	c := &backupCLI{dir: dir, out: out, envFile: set.envFile, log: log, lockWait: defaultLockWait}
	if dir != "" {
		c.tls = serve.BackupTLSFiles(set.tls, dir)
		api, other := findServe(ctx, dir, set.listen, serve.TLSMode(set.tls.Mode, set.tls.Cert, set.tls.Key) != "off", set.token)
		if other != "" {
			fmt.Fprintf(os.Stderr, "a server answers at %s, but not the serve of %s: it gets no key\n", other, dir)
		}
		c.api = api
	}
	return c.beforeUpdate(ctx, to)
}

// beforeUpdate is backupBeforeUpdate on c: through c.api where a serve
// answers and takes the key, else on the store in c.dir, "" for none,
// where a key is set and this process may write it.
func (c *backupCLI) beforeUpdate(ctx context.Context, to string) error {
	if c.dir == "" {
		fmt.Fprintln(c.out, "no backup of the server: no store on this host; the update goes on")
		return nil
	}
	if _, err := os.Stat(store.Path(c.dir)); errors.Is(err, fs.ErrNotExist) {
		fmt.Fprintf(c.out, "no backup of the server: no store in %s; the update goes on\n", c.dir)
		return nil
	}
	why := "no serve of " + c.dir + " answers"
	if c.api != nil {
		var ans struct {
			Skipped string    `json:"skipped"`
			Backup  cliBackup `json:"backup"`
			Steps   []cliStep `json:"steps"`
		}
		err := c.api.do(ctx, http.MethodPost, "/api/v1/update/backup", map[string]string{"to": to}, &ans)
		var refused *apiError
		switch {
		case err == nil && ans.Skipped != "":
			fmt.Fprintf(c.out, "no backup of the server: no key is set for its backups (%s); the update goes on\n", c.where())
			return nil
		case err == nil:
			c.printJob("backup before the update", ans.Backup, ans.Steps)
			return nil
		case errors.As(err, &refused) && (refused.Status == http.StatusUnauthorized || refused.Status == http.StatusForbidden):
			why = err.Error()
		default:
			return fmt.Errorf("the backup of the server before the update: %w", err)
		}
	}
	keyed, err := c.keySet(ctx)
	if err != nil {
		return fmt.Errorf("the backup of the server before the update: %w", err)
	}
	if !keyed {
		fmt.Fprintf(c.out, "no backup of the server: no key is set for its backups (the store in %s); the update goes on\n", c.dir)
		return nil
	}
	if storeOfAnother(store.Path(c.dir)) {
		return fmt.Errorf("the backup of the server before the update: a key is set, but %s, and root does not write the store in %s, which is another user's: start the service, or give its key (MASSIMO_API_TOKEN)", why, c.dir)
	}
	unlock, err := c.lockJobs(ctx, "backup")
	if err != nil {
		return fmt.Errorf("the backup of the server before the update: %w", err)
	}
	defer unlock()
	st, err := c.open(true)
	if err != nil {
		return fmt.Errorf("the backup of the server before the update: %w", err)
	}
	defer st.Close()
	b, outs, err := c.catalogue(st).Make(ctx, serverbackup.MakeRequest{Trigger: serverbackup.TriggerBeforeUpdate, Local: true, PinFor: serverbackup.PinBefore})
	switch {
	case errors.Is(err, serverbackup.ErrNoKey):
		fmt.Fprintf(c.out, "no backup of the server: no key is set for its backups (%s); the update goes on\n", c.where())
		return nil
	case err != nil:
		return fmt.Errorf("the backup of the server before the update: %w", err)
	}
	c.printJob("backup before the update", one(ctx, st, b.UUID), stepsOf(outs))
	return nil
}

// keySet reports whether the store in c.dir holds a key the backups are
// encrypted for, read without writing.
func (c *backupCLI) keySet(ctx context.Context) (bool, error) {
	st, err := c.open(false)
	if err != nil {
		return false, err
	}
	defer st.Close()
	keys, err := st.ServerBackupKeys(ctx)
	if err != nil {
		return false, err
	}
	return slices.ContainsFunc(keys, func(k store.ServerBackupKey) bool { return !k.Pending }), nil
}
