// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package main

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"sort"

	"go.metaworx.ch/massimo/internal/store"
)

// runStore handles massimo store backup|migrate|info, the commands that
// open serve's store directly (docs/store.md section 7). They take
// -data like serve and run as the service user, beside a running
// server.
func runStore(args []string) error {
	usage := func() {
		fmt.Fprint(os.Stderr, `usage: massimo [-data <dir>] store backup <file> | migrate | info

  backup <file>  write a consistent copy of massimo.db, also while serve runs
  migrate        create massimo.db or apply its pending migrations, without starting serve
  info           the file's schema version, node and row counts

The data directory is -data, else MASSIMO_DATA, else the current directory;
for the service run it as its user, for example
  sudo -u massimo massimo -data /var/lib/massimo store info
`)
	}
	if len(args) == 0 {
		usage()
		return errors.New("store: which command?")
	}
	ctx := context.Background()
	dir := dataDir()
	switch args[0] {
	case "backup":
		if len(args) != 2 {
			usage()
			return errors.New("store backup: give the file to write")
		}
		st, err := store.OpenReadOnly(dir, store.Options{})
		if err != nil {
			return storeError(dir, err)
		}
		defer st.Close()
		if err := st.Backup(ctx, args[1]); err != nil {
			return err
		}
		fmt.Printf("wrote %s\n", args[1])
	case "migrate":
		st, err := store.Open(dir, store.Options{Version: versionString(), Logger: slog.New(slog.NewTextHandler(os.Stderr, nil))})
		if err != nil {
			return err
		}
		defer st.Close()
		v, mr := st.SchemaVersion()
		fmt.Printf("%s: schema version %d (min_reader %d)\n", st.File(), v, mr)
	case "info":
		st, err := store.OpenReadOnly(dir, store.Options{})
		if err != nil {
			return storeError(dir, err)
		}
		defer st.Close()
		in, err := st.Info(ctx)
		if err != nil {
			return err
		}
		fmt.Printf("file:           %s\n", in.Path)
		fmt.Printf("schema version: %d (min_reader %d; this binary: %d)\n", in.Schema, in.MinReader, in.Binary)
		fmt.Printf("node:           %s\n", in.NodeID)
		fmt.Printf("tenant:         %s\n", in.TenantID)
		var tables []string
		for t := range in.Counts {
			tables = append(tables, t)
		}
		sort.Strings(tables)
		for _, t := range tables {
			fmt.Printf("  %-13s %d\n", t, in.Counts[t])
		}
	default:
		usage()
		return fmt.Errorf("store: unknown command %q", args[0])
	}
	return nil
}

// storeError explains a store that cannot be opened.
func storeError(dir string, err error) error {
	if errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("store: no %s in %s (give -data, or run serve once)", store.FileName, dir)
	}
	return err
}
