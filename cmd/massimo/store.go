// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"

	"go.metaworx.ch/massimo/internal/store"
)

// runStore handles massimo store backup|migrate|info, the commands that
// open serve's store directly (docs/store.md section 7). They take
// -data like serve and run as the service user, beside a running
// server. Each checks its arguments before it opens the store, which
// migrate creates, so that its help and a mistake leave nothing behind.
func runStore(args []string) error {
	usage := func() {
		fmt.Fprint(os.Stderr, `usage: massimo [-data <dir>] store backup <file> | migrate | info

  backup <file>  write a consistent copy of massimo.db, also while serve runs
  migrate        create massimo.db or apply its pending migrations, without starting serve;
                 a new one only where -data or MASSIMO_DATA names the directory
  info           the file's schema version, node and row counts

`+dataDirUsage("store", "info"))
	}
	if len(args) == 0 {
		usage()
		return errors.New("store: which command?")
	}
	// parse answers -h with the usage and refuses a flag: the commands
	// take none.
	parse := func() ([]string, error) {
		set := flag.NewFlagSet("store "+args[0], flag.ContinueOnError)
		set.Usage = usage
		if err := set.Parse(args[1:]); err != nil {
			return nil, err
		}
		return set.Args(), nil
	}
	ctx := context.Background()
	dir := dataDir()
	switch args[0] {
	case "backup":
		rest, err := parse()
		if err != nil {
			return err
		}
		if len(rest) != 1 {
			usage()
			return errors.New("store backup: give the file to write")
		}
		st, err := store.OpenReadOnly(dir, store.Options{})
		if err != nil {
			return storeError(dir, err)
		}
		defer st.Close()
		if err := st.Backup(ctx, rest[0]); err != nil {
			return err
		}
		fmt.Printf("wrote %s\n", rest[0])
	case "migrate":
		rest, err := parse()
		if err != nil {
			return err
		}
		if len(rest) > 0 {
			usage()
			return errors.New("store migrate takes no argument")
		}
		if err := checkStoreDir("store", "migrate"); err != nil {
			return fmt.Errorf("store migrate: %w", err)
		}
		logger, err := newLogger(os.Stderr)
		if err != nil {
			return err
		}
		st, err := store.Open(dir, store.Options{Version: versionString(), Logger: logger})
		if err != nil {
			return err
		}
		defer st.Close()
		if f, err := filepath.Abs(st.File()); err == nil {
			warnOtherStore(os.Stderr, f, "store", "migrate")
		}
		v, mr := st.SchemaVersion()
		fmt.Printf("%s: schema version %d (min_reader %d)\n", st.File(), v, mr)
	case "info":
		rest, err := parse()
		if err != nil {
			return err
		}
		if len(rest) > 0 {
			usage()
			return errors.New("store info takes no argument")
		}
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
	case "help", "-h", "--help":
		usage()
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
