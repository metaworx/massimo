// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package main

// The server's backup before massimo update installs a release by hand
// (AP ServerBackup v1.0, Block 10; docs/store.md, "Server backups"): a
// running serve takes it, POST /api/v1/update/backup with
// MASSIMO_API_TOKEN, as the server's own job; where none answers it is
// made on the store, to the local targets, pinned for 30 days, by a
// process that may write the store: root never writes the store of the
// service's user, whose files it would leave root's. Without a key, or
// without a store, the update goes on with a line; a backup that fails
// stops it, and -no-backup updates without one. The update helper takes
// none: the server that wrote its request took it first.

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"

	"go.metaworx.ch/massimo/internal/serverbackup"
	"go.metaworx.ch/massimo/internal/store"
)

// backupBeforeUpdate takes the backup massimo update takes before it
// installs, through a running serve or on the store of updateStoreDir,
// and says so on out.
func backupBeforeUpdate(ctx context.Context, out io.Writer) error {
	c := newBackupCLI(ctx, out)
	c.dir = updateStoreDir()
	return c.beforeUpdate(ctx)
}

// beforeUpdate is backupBeforeUpdate on c: through c.api where a serve
// answers, else on the store in c.dir, "" for none.
func (c *backupCLI) beforeUpdate(ctx context.Context) error {
	if c.api != nil {
		var ans struct {
			Skipped string    `json:"skipped"`
			Backup  cliBackup `json:"backup"`
			Steps   []cliStep `json:"steps"`
		}
		if err := c.api.do(ctx, http.MethodPost, "/api/v1/update/backup", nil, &ans); err != nil {
			return fmt.Errorf("the backup of the server before the update: %w", err)
		}
		if ans.Skipped != "" {
			fmt.Fprintf(c.out, "no backup of the server: no key is set for its backups (%s); the update goes on\n", c.where())
			return nil
		}
		c.printJob("backup before the update", ans.Backup, ans.Steps)
		return nil
	}
	if c.dir == "" {
		fmt.Fprintln(c.out, "no backup of the server: no store on this host; the update goes on")
		return nil
	}
	file := store.Path(c.dir)
	if _, err := os.Stat(file); errors.Is(err, fs.ErrNotExist) {
		fmt.Fprintf(c.out, "no backup of the server: no store in %s; the update goes on\n", c.dir)
		return nil
	}
	if storeOfAnother(file) {
		return fmt.Errorf("the backup of the server before the update: no server answers to take it, and root does not write the store in %s, which is another user's: start the service", c.dir)
	}
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
