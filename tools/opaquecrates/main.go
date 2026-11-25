// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

// Command opaquecrates writes the licence texts of the Rust crates in the
// WebAssembly of the vendored OPAQUE client into
// internal/server/ui/vendor/opaque.LICENSE-3RD-PARTY, which System info's
// credits take up beside opaque.LICENSE (make credits). The file's head,
// written by hand, says how its crates were had and lists them, each by
// name, version and the SHA-256 of its archive on crates.io; the program
// keeps the head, fetches each archive from static.crates.io, refuses
// one whose SHA-256 differs, and writes after the head the licence files
// at each archive's root. It is no part of the build: it is run by hand,
// online, from the repository's top, and git keeps its file:
//
//	go run ./tools/opaquecrates                  # writes the file
//	go run ./tools/opaquecrates -check           # exits 1 where the file is not what the archives give
//	go run ./tools/opaquecrates -crates <dir>    # the archives from <dir>/<name>-<version>.crate
//
// An error leaves the file as it was. No test runs it against the
// network: internal/server's TestVendoredOpaqueCrates holds the
// committed file to its list and to the crates the WebAssembly names.
package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"go.metaworx.ch/massimo/internal/safefile"
)

// outPath is the file, from the repository's top.
const outPath = "internal/server/ui/vendor/opaque.LICENSE-3RD-PARTY"

// cratesIO is where crates.io serves a crate's archive:
// <cratesIO><name>/<name>-<version>.crate.
const cratesIO = "https://static.crates.io/crates/"

func main() {
	check := flag.Bool("check", false, "exit 1 where "+outPath+" is not what the archives give, writing nothing")
	dir := flag.String("crates", "", "read each archive from `dir`/<name>-<version>.crate instead of crates.io")
	flag.Parse()
	fetch := fromCratesIO(&http.Client{Timeout: 2 * time.Minute})
	if *dir != "" {
		fetch = fromDir(*dir)
	}
	if err := run(outPath, *check, fetch, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// run writes the file at path anew from its head and the archives, or
// with check only compares it with what is there, and says which on w.
func run(path string, check bool, fetch func(crate) ([]byte, error), w io.Writer) error {
	have, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("opaquecrates: %w (run it from the repository's top)", err)
	}
	out, n, err := generate(have, fetch)
	if err != nil {
		return err
	}
	if bytes.Equal(have, out) {
		fmt.Fprintf(w, "opaquecrates: %s is current (%d crates)\n", path, n)
		return nil
	}
	if check {
		return fmt.Errorf("opaquecrates: %s is not what the archives give: run go run ./tools/opaquecrates", path)
	}
	if err := safefile.Write(path, out, safefile.Options{Mode: 0o644}); err != nil {
		return fmt.Errorf("opaquecrates: %w", err)
	}
	fmt.Fprintf(w, "opaquecrates: wrote %s (%d crates, %d bytes)\n", path, n, len(out))
	return nil
}

// fromCratesIO fetches a crate's archive from crates.io with client: a
// plain GET with Go's own User-Agent and no credential.
func fromCratesIO(client *http.Client) func(crate) ([]byte, error) {
	return func(c crate) ([]byte, error) {
		url := cratesIO + c.name + "/" + c.name + "-" + c.version + ".crate"
		resp, err := client.Get(url)
		if err != nil {
			return nil, fmt.Errorf("opaquecrates: %w", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("opaquecrates: %s: %s", url, resp.Status)
		}
		b, err := io.ReadAll(io.LimitReader(resp.Body, maxArchive+1))
		if err != nil {
			return nil, fmt.Errorf("opaquecrates: %s: %w", url, err)
		}
		if len(b) > maxArchive {
			return nil, fmt.Errorf("opaquecrates: %s: more than %d bytes", url, maxArchive)
		}
		return b, nil
	}
}

// fromDir reads a crate's archive from dir, as <name>-<version>.crate.
func fromDir(dir string) func(crate) ([]byte, error) {
	return func(c crate) ([]byte, error) {
		b, err := os.ReadFile(filepath.Join(dir, c.name+"-"+c.version+".crate"))
		if err != nil {
			return nil, fmt.Errorf("opaquecrates: %w", err)
		}
		return b, nil
	}
}
