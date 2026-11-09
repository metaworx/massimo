// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package env

import (
	"bufio"
	"fmt"
	"os"
	"slices"
	"sort"
	"strings"
	"sync"
)

// sources records where each variable came from: "file" when LoadFile
// or ReloadFile set it, "environment, overrides <file>" when it was
// already set (the environment wins). sourcesMu guards it, since serve
// re-reads the file while it runs.
var (
	sourcesMu sync.Mutex
	sources   = map[string]string{}
)

// Source says where a variable's value came from: "file", "environment,
// overrides <file>", or "environment" for one the file did not mention.
func Source(name string) string {
	sourcesMu.Lock()
	defer sourcesMu.Unlock()
	if s := sources[name]; s != "" {
		return s
	}
	return "environment"
}

// defaultFile is the env file File returns without MASSIMO_ENV_FILE.
var defaultFile = ".env"

// SetDefaultFile makes path the env file File returns without
// MASSIMO_ENV_FILE, in place of ".env" in the current directory: the
// service's settings file for the service user (cmd/massimo). It is
// not MASSIMO_ENV_FILE itself, which config would show as set by the
// environment and child processes would inherit. Call it at the start,
// before the file is read.
func SetDefaultFile(path string) { defaultFile = path }

// File returns the env file to read: MASSIMO_ENV_FILE, else the default
// file, ".env" in the current directory unless SetDefaultFile named
// another.
func File() string {
	if f := os.Getenv(FileVar); f != "" {
		return f
	}
	return defaultFile
}

// pair is one KEY=VALUE line of the file.
type pair struct{ k, v string }

// readFile parses path whole; a missing file is os.ErrNotExist.
func readFile(path string) ([]pair, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var out []pair
	sc := bufio.NewScanner(f)
	for n := 1; sc.Scan(); n++ {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimPrefix(line, "export ")
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			return nil, fmt.Errorf("%s:%d: expected KEY=VALUE", path, n)
		}
		k, v = strings.TrimSpace(k), strings.TrimSpace(v)
		if len(v) >= 2 && (v[0] == '"' && v[len(v)-1] == '"' || v[0] == '\'' && v[len(v)-1] == '\'') {
			v = v[1 : len(v)-1]
		}
		out = append(out, pair{k, v})
	}
	return out, sc.Err()
}

// LoadFile reads KEY=VALUE lines from path and sets them in the process
// environment unless the variable is already set. It exists so that secrets
// such as the Cube key never have to be typed on a command line, where they
// would end up in the shell history. Blank lines and # comments are
// ignored; an optional "export " prefix and surrounding quotes are stripped.
func LoadFile(path string) error {
	pairs, err := readFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	sourcesMu.Lock()
	defer sourcesMu.Unlock()
	for _, p := range pairs {
		if os.Getenv(p.k) == "" {
			os.Setenv(p.k, p.v)
			sources[p.k] = "file"
		} else {
			sources[p.k] = "environment, overrides " + path
		}
	}
	return nil
}

// ReloadFile reads path again while the program runs, as LoadFile
// would at a start: a variable the file set takes its new value, or is
// unset once the file drops it; one the file names now and the process
// environment had not set is set from it; one the environment set before
// the file stays as it is. The file is parsed whole first, so a parse
// error, or a missing file (an editor may be replacing it), changes
// nothing and is returned. It returns the names whose value changed,
// sorted.
func ReloadFile(path string) ([]string, error) {
	pairs, err := readFile(path)
	if err != nil {
		return nil, err
	}
	sourcesMu.Lock()
	defer sourcesMu.Unlock()
	named := map[string]bool{}
	var changed []string
	for _, p := range pairs {
		if named[p.k] {
			continue // the first line of a name wins, as at a start
		}
		named[p.k] = true
		src := sources[p.k]
		if strings.HasPrefix(src, "environment") || src == "" && os.Getenv(p.k) != "" {
			sources[p.k] = "environment, overrides " + path
			continue
		}
		if os.Getenv(p.k) != p.v {
			os.Setenv(p.k, p.v)
			changed = append(changed, p.k)
		}
		sources[p.k] = "file"
	}
	for k, src := range sources {
		if src == "file" && !named[k] {
			os.Unsetenv(k)
			delete(sources, k)
			changed = append(changed, k)
		}
	}
	sort.Strings(changed)
	return slices.Compact(changed), nil
}
