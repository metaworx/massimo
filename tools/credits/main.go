// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

// Command credits writes the credits and licences of everything the
// massimo binary includes into internal/credits/credits.json, which the
// binary embeds (internal/credits): Massimo itself, Go's runtime and
// standard library, the Go modules that go list -deps finds for
// ./cmd/massimo on every platform the Makefile's cross builds, each
// directory of third_party/ and each file of internal/server/ui/vendor/,
// each with every licence file it has. Go's entry and the modules' carry
// no version: the binary fills in those it was built with, so that a
// module's update needs no new list unless it adds or removes a module,
// a module it requires in turn too, or changes a licence's text. go list
// reads the modules from the module cache and downloads one missing
// there where GOPROXY allows; with GOPROXY=off, go mod download fills
// the cache first. It runs from the repository's top, as make credits
// runs it:
//
//	go run ./tools/credits          # writes the file
//	go run ./tools/credits -check   # exits 1 where the file is not current
//
// An entry without a licence file stops it with an error naming the
// entry and where it looked, and leaves the file as it was.
package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"go.metaworx.ch/massimo/internal/safefile"
)

// outPath is the list, from the repository's top.
const outPath = "internal/credits/credits.json"

func main() {
	check := flag.Bool("check", false, "exit 1 where "+outPath+" is not current, writing nothing")
	flag.Parse()
	root, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(os.Stderr, "credits:", err)
		os.Exit(1)
	}
	if err := run(root, *check, repository(root), os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// repository is the source of the list for the checkout at root: its
// files, the modules go list finds there and Go's own directory.
func repository(root string) source {
	return source{
		root:    root,
		goRoot:  goRoot,
		modules: func() ([]module, error) { return linkedModules(root) },
	}
}

// goRoot is Go's own directory, as go env names it.
func goRoot() (string, error) {
	out, err := exec.Command("go", "env", "GOROOT").Output()
	if err != nil {
		return "", fmt.Errorf("credits: go env GOROOT: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// run writes the list for s into its root's outPath, or with check only
// compares it with the file there, and says which on w.
func run(root string, check bool, s source, w io.Writer) error {
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		return fmt.Errorf("credits: %w (run it from the repository's top)", err)
	}
	out, n, err := generate(s)
	if err != nil {
		return err
	}
	path := filepath.Join(root, filepath.FromSlash(outPath))
	have, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("credits: %w", err)
	}
	if bytes.Equal(have, out) {
		fmt.Fprintf(w, "credits: %s is current (%d entries)\n", outPath, n)
		return nil
	}
	if check {
		return fmt.Errorf("credits: %s is not current: run make credits (go run ./tools/credits)", outPath)
	}
	if err := safefile.Write(path, out, safefile.Options{Mode: 0o644}); err != nil {
		return fmt.Errorf("credits: %w", err)
	}
	fmt.Fprintf(w, "credits: wrote %s (%d entries)\n", outPath, n)
	return nil
}
