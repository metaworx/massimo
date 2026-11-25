// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package main

// The settings serve's start refuses, in one table (AP ReloadChecks,
// D3-A): the start runs it right after its flags are parsed and stops
// at the first refusal; a reload of the settings file runs the checks
// whose settings changed (reload.go), so that the log says at once, in
// the start's own words, what the next start would refuse; and massimo
// install runs it on the settings file it installs and on one it keeps
// (installChecks), refusing the first and warning of the second where
// the next start would refuse them. The table
// holds the checks without side effects only (D1-A): MASSIMO_LISTEN,
// MASSIMO_LOG, MASSIMO_DATA and the -tls-http bind fail on the box's
// state, a port taken or a directory not writable, and stay the start's,
// whose errors name the setting as the table's do, "-listen or
// MASSIMO_LISTEN: …" (AP StartConfiguration, D20-A).

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"go.metaworx.ch/massimo/internal/env"
	"go.metaworx.ch/massimo/internal/install"
	"go.metaworx.ch/massimo/internal/serve"
	"go.metaworx.ch/massimo/internal/server"
)

// startCheck is one check of the start's: the variables it reads, one
// ending in "_" standing for every variable it begins, and the check,
// the start's own parser, which reads each setting through get, the
// Cubes' lines, which no list of names holds, from environ, KEY=VALUE
// lines as os.Environ gives them, and returns the error the start stops
// with, without its "serve: ".
type startCheck struct {
	vars  []string
	check func(get func(string) string, environ []string) error
}

// reads reports whether the check reads the variable k.
func (c startCheck) reads(k string) bool {
	for _, v := range c.vars {
		if v == k || strings.HasSuffix(v, "_") && strings.HasPrefix(k, v) {
			return true
		}
	}
	return false
}

// startChecks are the start's checks in its order: the log's formats,
// its rotation and level, HSTS, the trusted proxies, the restart,
// SIGHUP (refused on Windows too, which has none), the reboot switch,
// the announcement, TLS, and the settings file's Cubes, whose _IGNORE
// lists the start refuses even for a Cube the store holds (D11-A). With
// -addr, setupCubes also refuses a plain MASSIMO_CUBE_IGNORE through
// env.HiddenList, which the table does not check, -addr being for one run.
var startChecks = []startCheck{
	{[]string{env.LogFormat}, func(get func(string) string, _ []string) error {
		return formatsCheck(logFormatFlags{general: get(env.LogFormat)})
	}},
	{[]string{env.LogFileFormat}, func(get func(string) string, _ []string) error {
		return formatsCheck(logFormatFlags{file: get(env.LogFileFormat)})
	}},
	{[]string{env.LogProcessFormat}, func(get func(string) string, _ []string) error {
		return formatsCheck(logFormatFlags{process: get(env.LogProcessFormat)})
	}},
	{[]string{env.LogRotateSize}, func(get func(string) string, _ []string) error {
		return rotationCheck(logRotation{size: get(env.LogRotateSize)})
	}},
	{[]string{env.LogKeep}, func(get func(string) string, _ []string) error {
		return rotationCheck(logRotation{keep: get(env.LogKeep)})
	}},
	{[]string{env.LogKeepMin}, func(get func(string) string, _ []string) error {
		return rotationCheck(logRotation{keepMin: get(env.LogKeepMin)})
	}},
	{[]string{env.LogLevel}, func(get func(string) string, _ []string) error {
		if _, err := parseLogLevel(get(env.LogLevel)); err != nil {
			return fmt.Errorf("%s: %w", serveLevelWhat, err)
		}
		return nil
	}},
	{[]string{env.HSTS}, func(get func(string) string, _ []string) error {
		_, err := hstsDays(get(env.HSTS))
		return err
	}},
	{[]string{env.TrustedProxies}, func(get func(string) string, _ []string) error {
		_, err := server.ParseTrustedProxies(get(env.TrustedProxies))
		return err
	}},
	{[]string{restartVar}, func(get func(string) string, _ []string) error {
		_, _, err := restartBy(get(restartVar), currentProcess())
		return err
	}},
	{[]string{sighupVar}, func(get func(string) string, _ []string) error {
		_, err := sighupAction(get(sighupVar), false, false)
		return err
	}},
	{[]string{rebootVar}, func(get func(string) string, _ []string) error {
		_, err := parseSwitch(rebootVar, get(rebootVar))
		return err
	}},
	{[]string{env.Announce}, func(get func(string) string, _ []string) error {
		_, err := parseSwitch(env.Announce, get(env.Announce))
		return err
	}},
	{[]string{env.TLS, env.TLSCert, env.TLSKey, env.TLSNames, env.TLSHTTP, env.ACMEDNSHook}, func(get func(string) string, _ []string) error {
		return serve.CheckTLS(serve.TLS{Mode: get(env.TLS), Cert: get(env.TLSCert), Key: get(env.TLSKey), Names: get(env.TLSNames),
			HTTP: get(env.TLSHTTP), DNSHook: get(env.ACMEDNSHook)})
	}},
	// FileCubes reads the Cubes' lines from environ: no flag fixes one,
	// but -sn the Cube it selects.
	{[]string{env.Prefix}, func(get func(string) string, environ []string) error {
		_, _, err := serve.FileCubes(get(env.SN), environ)
		return err
	}},
}

// formatsCheck is the start's check of the log's formats, its error.
func formatsCheck(g logFormatFlags) error {
	_, _, err := g.formats()
	return err
}

// rotationCheck is the start's check of the log's rotation, its error.
func rotationCheck(g logRotation) error {
	_, _, _, err := g.options()
	return err
}

// checkRefusal is a check of the start's that refuses: the variables of
// only (startRefusals) it reads, sorted, and its error.
type checkRefusal struct {
	vars []string
	err  error
}

// startRefusals runs the start's checks in its order, each reading the
// settings through get and the Cubes' lines from environ, and returns
// those that refuse; a nil only runs every check, another only those
// that read one of its variables, each refusal naming those it reads.
func startRefusals(get func(string) string, environ []string, only map[string]bool) []checkRefusal {
	var out []checkRefusal
	for _, c := range startChecks {
		vars := readOf(c, only)
		if only != nil && len(vars) == 0 {
			continue
		}
		if err := c.check(get, environ); err != nil {
			out = append(out, checkRefusal{vars: vars, err: err})
		}
	}
	return out
}

// readOf are the variables of only that c reads, sorted.
func readOf(c startCheck, only map[string]bool) []string {
	var vars []string
	for k := range only {
		if c.reads(k) {
			vars = append(vars, k)
		}
	}
	sort.Strings(vars)
	return vars
}

// settingOf is how the start reads a setting: the value of its flag
// where the flag was given (fixed, fixedSettings), else the variable.
func settingOf(fixed map[string]string) func(string) string {
	return func(k string) string {
		if v, ok := fixed[k]; ok {
			return v
		}
		return os.Getenv(k)
	}
}

// installChecks are the start's checks for massimo install
// (install.Options.StartChecks), run on a settings file's values alone,
// vars, as install says the next start reads them: each setting read
// from vars, the Cubes' lines too, and nothing from install's own
// environment (AP StartConfiguration, D46-A, D47-A). It runs the checks
// that read one of vars, as a reload runs those of the settings that
// changed: the others read the defaults alone, which the start takes.
// Each refusal names the variables of vars its check reads, in the
// start's order. The list's order decides between two spellings of a
// Cube's line (env.PerCubeValue): sorted here, the file's order at the
// start.
func installChecks(vars map[string]string) []install.StartRefusal {
	environ := make([]string, 0, len(vars))
	only := make(map[string]bool, len(vars))
	for k, v := range vars {
		environ = append(environ, k+"="+v)
		only[k] = true
	}
	sort.Strings(environ)
	var out []install.StartRefusal
	for _, r := range startRefusals(func(k string) string { return vars[k] }, environ, only) {
		out = append(out, install.StartRefusal{Vars: r.vars, Err: r.err})
	}
	return out
}
