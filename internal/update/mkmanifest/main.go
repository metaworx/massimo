// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

// Command mkmanifest writes the release manifest (internal/update) for a
// set of built binaries to stdout: `make build-release` runs it, the release
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
	tag := flag.String("tag", "", "version: the release tag, v1.2.3 or v1.2.3-alpha.4, or for a rolling build git describe's v1.2.3-alpha.4-5-gabc1234")
	commit := flag.String("commit", "", "commit the version names")
	flavour := flag.String("flavour", "", "what the build is released as: the tag's label, stable without one, or devel or nightly")
	release := flag.String("release", "", "the GitHub release's tag where it is not -tag: release/<flavour> for a rolling build")
	dir := flag.String("dir", ".", "directory holding the binaries")
	flag.Parse()
	if err := run(*tag, *commit, *flavour, *release, *dir, flag.Args()); err != nil {
		fmt.Fprintln(os.Stderr, "mkmanifest:", err)
		os.Exit(1)
	}
}

func run(tag, commit, flavour, release, dir string, files []string) error {
	v, ok := update.ParseVersion(tag)
	rolling := update.Rolling(flavour)
	switch {
	case !ok || v.Dirty:
		return fmt.Errorf("%q is not a release's version", tag)
	case !v.Tag() && !rolling:
		return fmt.Errorf("%q is some commits after a tag, which only a rolling flavour releases, not %q", tag, flavour)
	case !update.ValidChannel(flavour):
		return fmt.Errorf("invalid flavour %q", flavour)
	case rolling && release != update.RollingPrefix+flavour:
		return fmt.Errorf("a %s build is released as %s%s, not %q", flavour, update.RollingPrefix, flavour, release)
	case !rolling && release != "" && release != tag:
		return fmt.Errorf("a %s release is released under its tag %s, not %q", flavour, tag, release)
	case len(files) == 0:
		return fmt.Errorf("no binaries given")
	}
	if release == tag {
		release = ""
	}
	m := update.Manifest{
		Tag:     tag,
		Release: release,
		Commit:  commit,
		Flavour: flavour,
		// 0.x is pre-release whatever the suffix, like a -suffix on 1.x,
		// and so is every rolling build.
		Prerelease: v.Major == 0 || len(v.Pre) > 0 || rolling,
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
