// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"go.metaworx.ch/massimo/internal/certs"
	"go.metaworx.ch/massimo/internal/env"
	"go.metaworx.ch/massimo/internal/store"
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
	fmt.Printf("data dir:      %s (%s)\n", dataDir(), storeState())
	fmt.Printf("log level:     %s\n", logLevelState())
	fmt.Printf("selected cube: %s (from %s)\n", orNone(selectedSN), src)
	fmt.Println("for the commands (serve takes these from its store, see below):")
	fmt.Printf("key in effect: %s\n", redact(cubeEnv(env.Key)))
	fmt.Printf("iv in effect:  %s\n", redact(cubeEnv(env.IV)))
	fmt.Printf("address:       %s\n", orNone(cubeEnv(env.Address)))
	fmt.Printf("hidden:        %s\n", orNone(cubeEnv(env.Ignore)))
	fmt.Printf("proxy:         %s\n", orNone(cubeEnv(env.Proxy)))
	fmt.Printf("proxy serial:  %s\n", orNone(cubeEnv(env.ProxySerial)))
	fmt.Printf("name:          %s\n", orNone(cubeEnv(env.Name)))
	printTLS(os.Stdout)

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
	printStore(os.Stdout)
	if sns, _ := env.Serials(env.Key); len(sns) > 0 {
		fmt.Printf("\ncubes with a key: %s\n", strings.Join(sns, ", "))
		for _, sn := range sns {
			missing := []string{}
			for _, v := range []string{env.IV, env.Address} {
				if _, set := env.PerCubeValue(v, sn); set == "" {
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

// openStoreToRead opens the data directory's store read-only, or says
// why it cannot.
func openStoreToRead() (*store.Store, error) {
	return store.OpenReadOnly(dataDir(), store.Options{Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
}

// logLevelState says which level the commands log at and where it
// came from, or why it is refused.
func logLevelState() string {
	v := os.Getenv(env.LogLevel)
	l, err := parseLogLevel(v)
	switch {
	case err != nil:
		return "invalid: " + err.Error()
	case v == "":
		return "info (default)"
	}
	return strings.ToLower(l.String()) + " (" + env.LogLevel + ", " + env.Source(env.LogLevel) + ")"
}

// storeState says whether the data directory has a store and its
// version.
func storeState() string {
	st, err := openStoreToRead()
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return "no " + store.FileName + " yet; serve creates it"
	case err != nil:
		return store.FileName + ": " + err.Error()
	}
	defer st.Close()
	v, _ := st.SchemaVersion()
	return fmt.Sprintf("%s, schema version %d, node %s", store.FileName, v, st.NodeID())
}

// printStore prints, for each Cube serve would serve, every value of the
// settings file's kind and where it comes from: the store (which wins),
// or the file (a Cube the store does not know yet, added at serve's next
// start), with the file's line where it differs.
func printStore(w io.Writer) {
	file, plain, err := fileCubes()
	if err != nil {
		fmt.Fprintf(w, "\nsettings file: %v\n", err)
		return
	}
	var rows []store.Cube
	if st, err := openStoreToRead(); err == nil {
		rows, err = st.Cubes(context.Background())
		if sn, _ := st.PlainSerial(context.Background()); sn != "" && plain != nil {
			plain.Serial = sn
			file, plain = append(file, *plain), nil
		}
		st.Close()
		if err != nil {
			fmt.Fprintf(w, "\nstore: %v\n", err)
		}
	}
	byFile := map[string]store.FileCube{}
	for _, fc := range file {
		byFile[store.NormalSerial(fc.Serial)] = fc
	}
	fmt.Fprintln(w, "\ncubes for serve (the store wins; the file's lines add a Cube the store does not know):")
	shown := map[string]bool{}
	value := func(field, v, src string) {
		fmt.Fprintf(w, "    %-14s %-30s (%s)\n", field, orNone(v), src)
	}
	for _, row := range rows {
		if !row.Configured && row.Source == store.SourceDiscovery {
			fmt.Fprintf(w, "  %s  found on the LAN at %s, not configured\n", row.Serial, orNone(row.DiscoveredAddress))
			if _, ok := byFile[row.Serial]; !ok {
				continue
			}
		}
		shown[row.Serial] = true
		fc, inFile := byFile[row.Serial]
		state := "configured"
		if !row.Configured {
			state = "removed, not served"
			if row.Source == store.SourceDiscovery {
				state = "found; added from the file at serve's next start"
			}
		}
		fmt.Fprintf(w, "  %s  in the store (source %s), %s\n", row.Serial, row.Source, state)
		diffs := map[string]store.Difference{}
		for _, d := range store.Differences(row, fc) {
			diffs[d.Field] = d
		}
		for _, f := range store.Fields {
			src := "store"
			if fv, ok := fc.Values[f]; inFile && ok {
				if d, differs := diffs[f]; differs {
					src = fmt.Sprintf("store; the file's %s=%s differs and is ignored", d.Variable, d.File)
				} else {
					src = "store, same as " + fv.Variable
				}
			}
			v := store.StoreValue(row, f)
			if f == store.FieldAddress && row.FollowDiscovery {
				v = strings.TrimSpace(v + " (follows discovery)")
			}
			value(f, v, src)
		}
	}
	for _, fc := range file {
		if shown[store.NormalSerial(fc.Serial)] {
			continue
		}
		fmt.Fprintf(w, "  %s  in the settings file only; added to the store at serve's next start\n", fc.Serial)
		for _, f := range store.Fields {
			if fv, ok := fc.Values[f]; ok {
				value(f, fv.Value, "file, "+fv.Variable)
			}
		}
	}
	if plain != nil {
		fmt.Fprintln(w, "  (serial not known yet)  the plain lines; added to the store once the Cube reports its serial")
		for _, f := range store.Fields {
			if fv, ok := plain.Values[f]; ok {
				value(f, fv.Value, "file, "+fv.Variable)
			}
		}
	}
	if len(rows) == 0 && len(file) == 0 && plain == nil {
		fmt.Fprintln(w, "  (none)")
	}
}

// printTLS says how serve serves: plain, or over TLS from which files,
// with the certificate's names, expiry and fingerprint when the files
// can be read here (they belong to the service user).
func printTLS(w io.Writer) {
	certPath, keyPath := os.Getenv(tlsCertVar), os.Getenv(tlsKeyVar)
	mode := tlsMode(os.Getenv(tlsVar), certPath, keyPath)
	if mode == "off" {
		fmt.Fprintf(w, "tls:           off (serve has no TLS)\n")
		return
	}
	days, err := hstsDays(os.Getenv(hstsVar))
	hsts := fmt.Sprintf("%d days", days)
	if err != nil {
		hsts = "invalid: " + os.Getenv(hstsVar)
	}
	if mode == "acme" {
		fmt.Fprintf(w, "tls:           acme (hsts %s)\n", hsts)
		printACME(w)
		return
	}
	if mode == "self-signed" {
		certPath, keyPath = certs.SelfSignedPaths(dataDir())
		fmt.Fprintf(w, "tls:           self-signed (%s; hsts %s)\n", filepath.Dir(certPath), hsts)
		if names := os.Getenv(tlsNamesVar); names != "" {
			fmt.Fprintf(w, "               names given: %s\n", names)
		}
		if _, err := os.Stat(certPath); err != nil {
			fmt.Fprintf(w, "               no certificate in %s yet: serve makes one at its start, or make it now with: massimo tls self-signed\n", filepath.Dir(certPath))
			return
		}
		printCertificate(w, mode, certPath, keyPath)
		return
	}
	fmt.Fprintf(w, "tls:           %s (cert %s, key %s; hsts %s)\n", mode, orNone(certPath), orNone(keyPath), hsts)
	if mode != "files" || certPath == "" || keyPath == "" {
		return
	}
	printCertificate(w, mode, certPath, keyPath)
}

// printACME prints, for config and tls show, the certificate autocert
// cached for each name of MASSIMO_TLS_NAMES, or that there is none yet.
func printACME(w io.Writer) {
	dir, _ := certs.ACMEPaths(dataDir(), "")
	names := certs.ParseNames(os.Getenv(tlsNamesVar))
	if hook := os.Getenv(acmeDNSHookVar); hook != "" {
		fmt.Fprintf(w, "source:        acme by DNS-01 (%s; names %s; directory %s; hook %s)\n", dir, orNone(strings.Join(names, ", ")), orNone(os.Getenv(acmeDirectoryVar)), hook)
		if len(names) == 0 {
			return
		}
		certPath, keyPath := certs.DNS01Paths(dataDir(), names[0])
		if _, err := os.Stat(certPath); err != nil {
			fmt.Fprintf(w, "               no certificate yet (serve orders one at its start)\n")
			return
		}
		printCertificate(w, "acme", certPath, keyPath)
		return
	}
	fmt.Fprintf(w, "source:        acme (%s; names %s; directory %s; http %s)\n", dir, orNone(strings.Join(names, ", ")), orNone(os.Getenv(acmeDirectoryVar)), orNone(os.Getenv(tlsHTTPVar)))
	for _, n := range names {
		st, err := certs.LoadACMECache(dataDir(), n)
		if err != nil {
			fmt.Fprintf(w, "               %s: no certificate yet (serve orders one at its start)\n", n)
			continue
		}
		fmt.Fprintf(w, "               %s: expires %s; fingerprint %s\n", n, st.NotAfter.Format("2006-01-02"), st.Fingerprint)
	}
}

// printCertificate prints a certificate's names, expiry and fingerprint
// under a line of config or tls show.
func printCertificate(w io.Writer, mode, certPath, keyPath string) {
	_, st, err := certs.Load(mode, certPath, keyPath)
	if err != nil {
		fmt.Fprintf(w, "               cannot read the certificate here: %v\n", err)
		return
	}
	fmt.Fprintf(w, "               names %s; expires %s; fingerprint %s\n", strings.Join(st.Names, ", "), st.NotAfter.Format("2006-01-02"), st.Fingerprint)
	if st.SelfSigned {
		fmt.Fprintf(w, "               self-signed: browsers warn once per device; compare this fingerprint there\n")
	}
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
