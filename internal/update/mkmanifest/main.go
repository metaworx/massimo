// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

// Command mkmanifest writes the release manifest (internal/update) for a
// set of built binaries to stdout: `make release` runs it, the release
// workflow signs its output.
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"go.metaworx.ch/massimo/internal/update"
)

func main() {
	tag := flag.String("tag", "", "release tag, v1.2.3 or v1.2.3-alpha.4")
	commit := flag.String("commit", "", "commit the tag points at")
	channel := flag.String("channel", "", "update channel: the branch the tag was made on")
	dir := flag.String("dir", ".", "directory holding the binaries")
	flag.Parse()
	if err := run(*tag, *commit, *channel, *dir, flag.Args()); err != nil {
		fmt.Fprintln(os.Stderr, "mkmanifest:", err)
		os.Exit(1)
	}
}

func run(tag, commit, channel, dir string, files []string) error {
	v, ok := update.ParseVersion(tag)
	if !ok || !v.Tag() {
		return fmt.Errorf("%q is not a release tag", tag)
	}
	if !update.ValidChannel(channel) {
		return fmt.Errorf("invalid channel %q", channel)
	}
	if len(files) == 0 {
		return fmt.Errorf("no binaries given")
	}
	m := update.Manifest{
		Tag:     tag,
		Commit:  commit,
		Channel: channel,
		// 0.x is pre-release whatever the suffix, like a -suffix on 1.x.
		Prerelease: v.Major == 0 || len(v.Pre) > 0,
		Assets:     map[string]string{},
	}
	for _, name := range files {
		f, err := os.Open(filepath.Join(dir, name))
		if err != nil {
			return err
		}
		h := sha256.New()
		_, err = io.Copy(h, f)
		f.Close()
		if err != nil {
			return err
		}
		m.Assets[strings.TrimPrefix(name, dir+"/")] = hex.EncodeToString(h.Sum(nil))
	}
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	_, err = os.Stdout.Write(append(b, '\n'))
	return err
}
