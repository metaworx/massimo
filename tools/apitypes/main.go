// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

// Command apitypes writes the API's shapes as TypeScript: it reads the
// schemas of docs/api-v1-openapi.yaml (components.schemas) and writes
// internal/server/ui/src/api.d.ts, one exported declaration per schema in
// the file's order, which make ui-check holds the web UI's parts to
// (docs/ui.md, "How the page is built"). It runs from the repository's
// top, as make api-types runs it:
//
//	go run ./tools/apitypes          # writes the file
//	go run ./tools/apitypes -check   # exits 1 where the file is not current
//
// A keyword of the OpenAPI file it does not map stops it with an error
// naming the schema and the keyword, and leaves the file as it was.
package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"os"

	"go.metaworx.ch/massimo/internal/safefile"
)

// The OpenAPI file and the declarations, from the repository's top.
const (
	specPath = "docs/api-v1-openapi.yaml"
	outPath  = "internal/server/ui/src/api.d.ts"
)

func main() {
	check := flag.Bool("check", false, "exit 1 where "+outPath+" is not current, writing nothing")
	flag.Parse()
	if err := run(*check); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// run writes the declarations, or with check only compares them with
// the file.
func run(check bool) error {
	spec, err := os.ReadFile(specPath)
	if err != nil {
		return fmt.Errorf("apitypes: %w (run it from the repository's top)", err)
	}
	out, n, err := generate(spec)
	if err != nil {
		return err
	}
	have, err := os.ReadFile(outPath)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("apitypes: %w", err)
	}
	if bytes.Equal(have, out) {
		fmt.Printf("apitypes: %s is current (%d declarations)\n", outPath, n)
		return nil
	}
	if check {
		return fmt.Errorf("apitypes: %s is not current: run make api-types (go run ./tools/apitypes)", outPath)
	}
	if err := safefile.Write(outPath, out, safefile.Options{Mode: 0o644}); err != nil {
		return fmt.Errorf("apitypes: %w", err)
	}
	fmt.Printf("apitypes: wrote %s (%d declarations)\n", outPath, n)
	return nil
}
