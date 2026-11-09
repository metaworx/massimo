// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package install

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// PackagedPath is the marker a package ships (the Debian package of
// packaging/nfpm.yaml; the Homebrew formula puts the same file under its
// prefix): its first line names the package manager that owns the
// binary, its second the command that upgrades massimo there, its third
// the one that removes it. A box has one owner of the binary: where a
// package manager is it, `massimo update` and the web UI's button leave
// the install to it, and `massimo install` only configures
// (packaging/README.md, "Debian package").
const PackagedPath = "/usr/share/massimo/packaged"

// AptSource is the apt source of the massimo repository that
// packaging/README.md, "The apt repository", sets up, and the only one
// the update helper fetches from on a box apt owns massimo on.
const AptSource = "/etc/apt/sources.list.d/massimo.sources"

// AptSuite reads the first suite the apt source at path names (its
// Suites field, deb822 as packaging/apt/publish.sh writes it): the update
// channel apt follows there.
func AptSuite(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(string(b), "\n") {
		k, v, ok := strings.Cut(line, ":")
		if !ok || strings.HasPrefix(strings.TrimSpace(k), "#") || !strings.EqualFold(strings.TrimSpace(k), "Suites") {
			continue
		}
		if f := strings.Fields(v); len(f) > 0 {
			return f[0], nil
		}
	}
	return "", fmt.Errorf("%s names no suite", path)
}

// Manager is the package manager a marker names.
type Manager struct {
	Name    string // apt, dnf, brew
	Command string // what upgrades massimo there, for people
	Remove  string // what removes it
}

// String is the reason the updater and the API give: "managed by apt:
// sudo apt update && sudo apt install massimo".
func (m Manager) String() string {
	if m.Command == "" {
		return "managed by " + m.Name
	}
	return "managed by " + m.Name + ": " + m.Command
}

// Configurable reports whether `massimo install` may still write the
// settings and the listen drop-ins on this box: the Linux package
// managers, whose layout is the one install knows; brew's prefix is not.
func (m Manager) Configurable() bool { return m.Name == "apt" || m.Name == "dnf" }

// RemoveHint says how the package is removed, for `install -remove`.
func (m Manager) RemoveHint() string {
	if m.Remove == "" {
		return "remove it with " + m.Name
	}
	return "remove it with: " + m.Remove
}

// Packaged reports the package manager that owns this binary, from the
// marker at PackagedPath below root and, with no root, beside the binary
// (<dir>/../share/massimo/packaged, where brew puts it).
func Packaged(root string) (Manager, bool) {
	exe := ""
	if root == "" {
		if p, err := os.Executable(); err == nil {
			if p, err = filepath.EvalSymlinks(p); err == nil {
				exe = p
			}
		}
	}
	return packagedIn(root, exe)
}

func packagedIn(root, exe string) (Manager, bool) {
	paths := []string{filepath.Join(root, PackagedPath)}
	if exe != "" {
		paths = append(paths, filepath.Join(filepath.Dir(exe), "..", "share", "massimo", "packaged"))
	}
	for _, p := range paths {
		if m, ok := readMarker(p); ok {
			return m, true
		}
	}
	return Manager{}, false
}

// readMarker parses a marker file; a missing or empty one is no marker.
func readMarker(path string) (Manager, bool) {
	b, err := os.ReadFile(path)
	if err != nil {
		return Manager{}, false
	}
	lines := strings.Split(string(b), "\n")
	for len(lines) < 3 {
		lines = append(lines, "")
	}
	m := Manager{Name: strings.TrimSpace(lines[0]), Command: strings.TrimSpace(lines[1]), Remove: strings.TrimSpace(lines[2])}
	if m.Name == "" {
		return Manager{}, false
	}
	return m, true
}
