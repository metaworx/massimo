// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package main

import (
	"context"
	"crypto/ed25519"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"time"

	"go.metaworx.ch/massimo/internal/serverbackup"
	"go.metaworx.ch/massimo/internal/store"
)

// runStore handles massimo store backup|migrate|info|downgrade, the
// commands that open serve's store directly (docs/store.md section 7),
// and keygen|inspect|decrypt, the server backups' keys and files
// (docs/store.md, "Server backups"), which open no store. They take
// -data like serve and run as the service user, beside a running server,
// but for downgrade, which needs the store alone. Each checks its
// arguments before it opens the store, which migrate creates, so that
// its help and a mistake leave nothing behind.
func runStore(args []string) error {
	usage := func() {
		fmt.Fprint(os.Stderr, `usage: massimo [-data <dir>] store <command> [options] [args]

  backup <file>  write a consistent copy of massimo.db, also while serve runs
  migrate        create massimo.db or apply its pending migrations, without starting serve;
                 a new one only where -data or MASSIMO_DATA names the directory
  info           the file's schema version, node and row counts
  downgrade -to <version>
                 take massimo.db back to what a binary for that schema version opens
                 (its store info names it), for a rollback to an older release;
                 with the service stopped, a copy massimo.db.before-downgrade-<version> first
  keygen -kind pq|classic [-out <file>]
                 make a key for the server's backups: pq is post-quantum (age1pq1…,
                 about 2000 characters), classic is X25519 (age1…, 62 characters,
                 beside an ssh-ed25519 key); a server's keys are of one kind. The
                 secret key goes to <file> (mode 0600, not replaced) or to standard
                 output; the public key, which the server takes, is printed
  inspect <file> a server backup's manifest, who signed it and its payload's hash,
                 read without the key
  decrypt <file> -key <file> -out <dir>
                 open a server backup with its secret key (age's, or an OpenSSH
                 ssh-ed25519 key) into <dir>, empty or new: massimo.db, opaque.key,
                 node.key, env and tls/; age -d -i opens it as well

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
	case "downgrade":
		set := flag.NewFlagSet("store downgrade", flag.ContinueOnError)
		set.Usage = usage
		to := set.Int("to", 0, "the schema version of the binary to go back to")
		if err := set.Parse(args[1:]); err != nil {
			return err
		}
		if len(set.Args()) > 0 || *to < 1 {
			usage()
			return errors.New("store downgrade: give -to <version>, the schema version of the binary to go back to")
		}
		logger, err := newLogger(os.Stderr)
		if err != nil {
			return err
		}
		d, err := store.Downgrade(dir, *to, store.Options{Version: versionString(), Logger: logger})
		if err != nil {
			return storeError(dir, err)
		}
		if f, err := filepath.Abs(d.Path); err == nil {
			warnOtherStore(os.Stderr, f, "store", "downgrade", "-to", strconv.Itoa(*to))
		}
		if d.Copy == "" {
			fmt.Printf("%s: schema version %d (min_reader %d), which a binary for version %d opens; nothing to do\n", d.Path, d.From, d.MinReader, *to)
			break
		}
		fmt.Printf("%s: schema version %d, now %d (min_reader %d), which a binary for version %d opens; the file as it was is %s\n",
			d.Path, d.From, d.To, d.MinReader, *to, d.Copy)
	case "keygen":
		return storeKeygen(args[1:], usage, os.Stdout, os.Stderr)
	case "inspect":
		return storeInspect(args[1:], usage, dir, os.Stdout)
	case "decrypt":
		return storeDecrypt(args[1:], usage, dir, os.Stdout, os.Stderr)
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

// parseMixed parses set over args whose flags may come before and after
// the arguments, as in "decrypt <file> -key <file>", and returns the
// arguments.
func parseMixed(set *flag.FlagSet, args []string) ([]string, error) {
	var rest []string
	for {
		if err := set.Parse(args); err != nil {
			return nil, err
		}
		if set.NArg() == 0 {
			return rest, nil
		}
		rest = append(rest, set.Arg(0))
		args = set.Args()[1:]
	}
}

// storeKeygen is massimo store keygen -kind pq|classic [-out <file>]:
// the kind has no default (AP ServerBackup, D17-C). The secret key goes
// to the file, which is not replaced, or to stdout with a warning; the
// public key is printed.
func storeKeygen(args []string, usage func(), stdout, stderr io.Writer) error {
	set := flag.NewFlagSet("store keygen", flag.ContinueOnError)
	set.Usage = usage
	kind := set.String("kind", "", "pq (post-quantum) or classic (X25519)")
	out := set.String("out", "", "the file for the secret key, mode 0600; standard output without")
	if err := set.Parse(args); err != nil {
		return err
	}
	var k serverbackup.KeyKind
	switch *kind {
	case "pq":
		k = serverbackup.KindPostQuantum
	case "classic":
		k = serverbackup.KindX25519
	default:
		usage()
		return errors.New("store keygen: give -kind pq, post-quantum and about 2000 characters, or -kind classic, X25519, 62 characters and beside an ssh-ed25519 key")
	}
	if set.NArg() > 0 {
		usage()
		return errors.New("store keygen takes no argument; the file is -out <file>")
	}
	key, err := serverbackup.GenerateKey(k)
	if err != nil {
		return err
	}
	file := key.File(time.Now())
	if *out == "" {
		fmt.Fprintln(stderr, "store keygen: the secret key is on standard output; keep it safe and away from the server: without it no backup opens")
		fmt.Fprintf(stderr, "public key: %s\n", key.Recipient.Line)
		_, err := stdout.Write(file)
		return err
	}
	if err := writeNewFile(*out, file); err != nil {
		return fmt.Errorf("store keygen: %w", err)
	}
	fmt.Fprintf(stderr, "store keygen: the secret key is in %s; keep it safe and away from the server: without it no backup opens\n", *out)
	fmt.Fprintln(stdout, key.Recipient.Line)
	return nil
}

// writeNewFile writes b to path, which must not exist, mode 0600, and
// removes it where the write fails.
func writeNewFile(path string, b []byte) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	_, err = f.Write(b)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		os.Remove(path)
	}
	return err
}

// storeInspect is massimo store inspect <file>: the manifest of a server
// backup, the signature's verdict, this box being the one whose node.key
// is in dir, and the payload checked against the manifest. A bad
// signature or a changed payload is printed, then an error.
func storeInspect(args []string, usage func(), dir string, stdout io.Writer) error {
	set := flag.NewFlagSet("store inspect", flag.ContinueOnError)
	set.Usage = usage
	rest, err := parseMixed(set, args)
	if err != nil {
		return err
	}
	if len(rest) != 1 {
		usage()
		return errors.New("store inspect: give the backup's file")
	}
	f, size, err := openSized(rest[0])
	if err != nil {
		return fmt.Errorf("store inspect: %w", err)
	}
	defer f.Close()
	m, sig, _, err := serverbackup.ReadManifest(f, size)
	if err != nil {
		return fmt.Errorf("store inspect: %s: %w", rest[0], err)
	}
	local, lerr := store.ReadNodePublicKey(dir)
	if sig.Verdict(local) == serverbackup.Bad {
		// Nothing of a manifest its signature does not verify is shown.
		fmt.Fprintf(stdout, "file:        %s\n", rest[0])
		fmt.Fprintln(stdout, signatureLine(sig, local, lerr))
		return fmt.Errorf("store inspect: %w", serverbackup.ErrBadSignature)
	}
	printManifest(stdout, rest[0], m)
	fmt.Fprintln(stdout, signatureLine(sig, local, lerr))
	if _, _, err := serverbackup.Verify(f, size); err != nil {
		fmt.Fprintln(stdout, "payload:     not the one the manifest names")
		return fmt.Errorf("store inspect: %w", err)
	}
	fmt.Fprintln(stdout, "payload:     as the manifest names it")
	return nil
}

// signatureLine is store inspect's and store decrypt's line of who
// signed a backup, this box being the one whose node key is local; lerr
// is why it could not be read.
func signatureLine(sig serverbackup.Signature, local ed25519.PublicKey, lerr error) string {
	switch v := sig.Verdict(local); {
	case v == serverbackup.Bad:
		return "signature:   bad: the manifest was changed since it was made"
	case v == serverbackup.OtherBox && lerr != nil:
		return fmt.Sprintf("signature:   good, by the box %s (this box's node.key was not read: whether it is this one is not known)", sig.Fingerprint)
	default:
		return fmt.Sprintf("signature:   good, by %s (%s)", v, sig.Fingerprint)
	}
}

// openSized opens path and returns its size.
func openSized(path string) (*os.File, int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, 0, err
	}
	fi, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, 0, err
	}
	return f, fi.Size(), nil
}

// printManifest prints a backup's manifest for store inspect.
func printManifest(w io.Writer, path string, m serverbackup.Manifest) {
	fmt.Fprintf(w, "file:        %s\n", path)
	fmt.Fprintf(w, "backup:      %s (%s, format %d)\n", m.UUID, m.Kind, m.Format)
	fmt.Fprintf(w, "made:        %s, %s\n", m.Created.Format(time.RFC3339), m.Trigger)
	if m.Label != "" {
		fmt.Fprintf(w, "label:       %s\n", m.Label)
	}
	fmt.Fprintf(w, "massimo:     %s, schema version %d (min_reader %d)\n", m.Version, m.Schema.Version, m.Schema.MinReader)
	c := m.Counts
	fmt.Fprintf(w, "holds:       %d users, %d integrations, %d Cubes, %d rooms, %d audit rows, %d log rows\n", c.Users, c.Integrations, c.Cubes, c.Rooms, c.Audit, c.Log)
	files := "massimo.db, opaque.key, node.key"
	if m.Files.Env {
		files += ", env"
	}
	if m.Files.TLS > 0 {
		files += fmt.Sprintf(", %d TLS files", m.Files.TLS)
	}
	fmt.Fprintf(w, "files:       %s\n", files)
	for i, r := range m.Recipients {
		head := "for:        "
		if i > 0 {
			head = "            "
		}
		fmt.Fprintf(w, "%s %s (%s, %s)\n", head, r.Label, r.Kind, r.Fingerprint)
	}
	fmt.Fprintf(w, "payload.age: %d bytes, SHA-256 %s\n", m.Payload.Size, m.Payload.SHA256)
}

// storeDecrypt is massimo store decrypt <file> -key <file> -out <dir>:
// the backup opened with the secret key into an empty or new directory,
// and who signed it, this box being the one whose node.key is in dir; a
// warning where that was another box.
func storeDecrypt(args []string, usage func(), dir string, stdout, stderr io.Writer) error {
	set := flag.NewFlagSet("store decrypt", flag.ContinueOnError)
	set.Usage = usage
	keyFile := set.String("key", "", "the secret key's file")
	out := set.String("out", "", "the directory to write into, empty or new")
	rest, err := parseMixed(set, args)
	if err != nil {
		return err
	}
	if len(rest) != 1 || *keyFile == "" || *out == "" {
		usage()
		return errors.New("store decrypt: give the backup's file, -key <file> and -out <dir>")
	}
	kb, err := os.ReadFile(*keyFile)
	if err != nil {
		return fmt.Errorf("store decrypt: %w", err)
	}
	ids, err := serverbackup.ParseIdentities(kb)
	if err != nil {
		return fmt.Errorf("store decrypt: %s: %w", *keyFile, err)
	}
	f, size, err := openSized(rest[0])
	if err != nil {
		return fmt.Errorf("store decrypt: %w", err)
	}
	defer f.Close()
	o, err := serverbackup.Open(f, size, ids, *out)
	if err != nil {
		return fmt.Errorf("store decrypt: %s: %w", rest[0], err)
	}
	local, lerr := store.ReadNodePublicKey(dir)
	fmt.Fprintf(stdout, "opened the backup %s of %s into %s\n", o.Manifest.UUID, o.Manifest.Created.Format(time.RFC3339), *out)
	fmt.Fprintln(stdout, signatureLine(o.Signature, local, lerr))
	if o.Signature.Verdict(local) == serverbackup.OtherBox {
		fmt.Fprintf(stderr, "store decrypt: warning: the backup was made by the box %s, not by this one as far as its node.key tells; check that it is the box you meant\n", o.Signature.Fingerprint)
	}
	return nil
}
