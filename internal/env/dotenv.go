// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package env

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// sources records where each variable came from: "file" when LoadFile
// set it, "environment, overrides <file>" when it was already set (the
// environment wins).
var sources = map[string]string{}

// Source says where a variable's value came from: "file", "environment,
// overrides <file>", or "environment" for one the file did not mention.
func Source(name string) string {
	if s := sources[name]; s != "" {
		return s
	}
	return "environment"
}

// File returns the env file to read: MASSIMO_ENV_FILE, else ".env" in
// the current directory.
func File() string {
	if f := os.Getenv(FileVar); f != "" {
		return f
	}
	return ".env"
}

// LoadFile reads KEY=VALUE lines from path and sets them in the process
// environment unless the variable is already set. It exists so that secrets
// such as the Cube key never have to be typed on a command line, where they
// would end up in the shell history. Blank lines and # comments are
// ignored; an optional "export " prefix and surrounding quotes are stripped.
func LoadFile(path string) error {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for n := 1; sc.Scan(); n++ {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimPrefix(line, "export ")
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			return fmt.Errorf("%s:%d: expected KEY=VALUE", path, n)
		}
		k, v = strings.TrimSpace(k), strings.TrimSpace(v)
		if len(v) >= 2 && (v[0] == '"' && v[len(v)-1] == '"' || v[0] == '\'' && v[len(v)-1] == '\'') {
			v = v[1 : len(v)-1]
		}
		if os.Getenv(k) == "" {
			os.Setenv(k, v)
			sources[k] = "file"
		} else {
			sources[k] = "environment, overrides " + path
		}
	}
	return sc.Err()
}
