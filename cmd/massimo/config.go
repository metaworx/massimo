// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package main

import (
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"

	"go.metaworx.ch/massimo/internal/env"
)

// runConfig prints the MASSIMO_* settings in effect. Keys and IVs are
// redacted to their length, so the output can be pasted into a bug
// report.
func runConfig(args []string) error {
	fs := flag.NewFlagSet("config", flag.ContinueOnError)
	snFlag(fs)
	fs.Usage = func() {
		fmt.Fprint(os.Stderr, "usage: massimo config\n\nShow the settings in effect (env file, selected Cube, variables), with\nkeys and IVs redacted.\n\n")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return err
	}
	envFile := env.File()
	state := "not found"
	if _, err := os.Stat(envFile); err == nil {
		state = "loaded"
	}
	fmt.Printf("env file:      %s (%s)\n", envFile, state)
	src := "none"
	switch {
	case snExplicit:
		src = "-sn"
	case selectedSN != "":
		src = env.SN
	}
	fmt.Printf("selected cube: %s (from %s)\n", orNone(selectedSN), src)
	fmt.Printf("key in effect: %s\n", redact(cubeEnv(env.Key)))
	fmt.Printf("iv in effect:  %s\n", redact(cubeEnv(env.IV)))
	fmt.Printf("address:       %s\n", orNone(cubeEnv(env.Address)))
	fmt.Printf("hidden:        %s\n", orNone(cubeEnv(env.Ignore)))
	fmt.Printf("proxy:         %s\n", orNone(cubeEnv(env.Proxy)))
	fmt.Printf("proxy serial:  %s\n", orNone(cubeEnv(env.ProxySerial)))
	fmt.Printf("name:          %s\n", orNone(cubeEnv(env.Name)))

	var names []string
	for _, kv := range os.Environ() {
		if k, _, _ := strings.Cut(kv, "="); strings.HasPrefix(k, "MASSIMO_") {
			names = append(names, k)
		}
	}
	sort.Strings(names)
	fmt.Println("\nvariables:")
	for _, k := range names {
		v := os.Getenv(k)
		if strings.HasSuffix(k, "_KEY") || strings.HasSuffix(k, "_IV") || strings.HasSuffix(k, "_TOKEN") {
			v = redact(v)
		}
		src := env.Source(k)
		note := ""
		if !env.Known(k) {
			note = "   <- unknown name, typo?"
		}
		fmt.Printf("  %-40s %-45s (%s)%s\n", k, v, src, note)
	}
	if sns, _ := env.Serials(env.Key); len(sns) > 0 {
		fmt.Printf("\ncubes with a key: %s\n", strings.Join(sns, ", "))
		for _, sn := range sns {
			missing := []string{}
			for _, v := range []string{env.IV, env.Address} {
				if os.Getenv(env.PerCube(v, sn)) == "" {
					missing = append(missing, strings.TrimPrefix(v, env.Prefix))
				}
			}
			if len(missing) > 0 {
				fmt.Printf("  %s: no %s\n", sn, strings.Join(missing, ", "))
			}
		}
	}
	return nil
}

func orNone(s string) string {
	if s == "" {
		return "(none)"
	}
	return s
}

// redact shows only whether a secret is set and how long it is.
func redact(s string) string {
	if s == "" {
		return "(unset)"
	}
	hexDigits := 0
	for _, c := range strings.TrimLeft(s, "kK") {
		if strings.ContainsRune("0123456789abcdefABCDEF", c) {
			hexDigits++
		}
	}
	return fmt.Sprintf("set (%d characters, %d hex digits)", len(s), hexDigits)
}
