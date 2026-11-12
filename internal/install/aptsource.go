// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package install

import (
	"errors"
	"fmt"
	"strings"
)

// The files of the package massimo-archive-keyring that a massimo source's
// Signed-By names: the numbered suites' key, and the rolling suites' (devel,
// nightly), one key in each, so that the rolling key signs nothing a box on
// a numbered suite accepts (packaging/README.md, "The apt repository").
const (
	AptKeyring        = "/usr/share/keyrings/massimo-archive-keyring.gpg"
	AptRollingKeyring = "/usr/share/keyrings/massimo-rolling-archive-keyring.gpg"
)

// aptRepositories are the addresses of the project's apt repository a
// source may name: the site's, and the one before 2026-11-09, which
// GitHub redirects to it.
var aptRepositories = []string{"https://massimo.metaworx.ch/apt", "https://metaworx.github.io/massimo/apt"}

// AptSwitch returns the massimo source b with its suite changed to suite
// and its Signed-By to the keyring file of that kind of suite (rolling for
// devel and nightly), every other line as it was. It changes only a source
// as packaging/README.md sets it up: one entry, one suite, the project's
// repository or a local one (file:, which only root can name, as only
// root can write the source), and Signed-By one of the keyring package's
// files; anything else is left to its owner, and the error says why.
func AptSwitch(b []byte, suite string, rolling bool) ([]byte, error) {
	lines := strings.Split(string(b), "\n")
	found := map[string]int{} // the line of each field, by its lower-case name
	entries, inEntry := 0, false
	for i, line := range lines {
		t := strings.TrimSpace(line)
		switch {
		case t == "":
			inEntry = false
			continue
		case strings.HasPrefix(t, "#"):
			continue
		}
		if !inEntry {
			if entries++; entries > 1 {
				return nil, errors.New("it holds more than one entry")
			}
			inEntry = true
		}
		k, _, ok := strings.Cut(t, ":")
		// A continuation line belongs to the field before it.
		if !ok || line[0] == ' ' || line[0] == '\t' {
			continue
		}
		k = strings.ToLower(strings.TrimSpace(k))
		if _, twice := found[k]; twice {
			return nil, fmt.Errorf("it names %s twice", k)
		}
		found[k] = i
	}
	if entries == 0 {
		return nil, errors.New("it holds no entry")
	}
	value := func(k string) ([]string, error) {
		i, ok := found[k]
		if !ok {
			return nil, fmt.Errorf("it has no %s line", k)
		}
		_, v, _ := strings.Cut(lines[i], ":")
		// A value continued on the next lines is not one this changes.
		if i+1 < len(lines) && len(lines[i+1]) > 0 && (lines[i+1][0] == ' ' || lines[i+1][0] == '\t') {
			return nil, fmt.Errorf("its %s line goes on", k)
		}
		return strings.Fields(v), nil
	}
	suites, err := value("suites")
	if err != nil {
		return nil, err
	}
	if len(suites) != 1 {
		return nil, fmt.Errorf("it names %d suites, not one", len(suites))
	}
	uris, err := value("uris")
	if err != nil {
		return nil, err
	}
	if len(uris) != 1 || !aptRepository(uris[0]) {
		return nil, fmt.Errorf("its URIs are not the project's repository (%s)", strings.Join(uris, " "))
	}
	signed, err := value("signed-by")
	if err != nil {
		return nil, err
	}
	if len(signed) != 1 || signed[0] != AptKeyring && signed[0] != AptRollingKeyring {
		return nil, errors.New("its Signed-By is not a file of the package massimo-archive-keyring")
	}
	keyring := AptKeyring
	if rolling {
		keyring = AptRollingKeyring
	}
	set := func(k, v string) {
		i := found[k]
		name, _, _ := strings.Cut(lines[i], ":")
		lines[i] = name + ": " + v
	}
	set("suites", suite)
	set("signed-by", keyring)
	return []byte(strings.Join(lines, "\n")), nil
}

// aptRepository says whether a source's URI is the project's apt
// repository, or a local one.
func aptRepository(uri string) bool {
	if strings.HasPrefix(uri, "file:") {
		return true
	}
	uri = strings.TrimSuffix(uri, "/")
	for _, r := range aptRepositories {
		if uri == r {
			return true
		}
	}
	return false
}
