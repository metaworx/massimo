// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package main

import (
	"bytes"
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"testing"

	"go.metaworx.ch/massimo/internal/env"
	"go.metaworx.ch/massimo/internal/testkit"
	"go.metaworx.ch/massimo/internal/update"
)

// Each of the start's checks reads the variables its entry names and no
// other, so that a reload runs it where one of them changed: the Cubes'
// check, whose prefix stands for every Cube line, reads the selected
// Cube through get and the lines from the environment itself.
func TestStartChecksReadTheirVariables(t *testing.T) {
	t.Parallel()
	for _, c := range startChecks {
		var read []string
		c.check(func(k string) string {
			if !slices.Contains(read, k) {
				read = append(read, k)
			}
			return ""
		})
		slices.Sort(read)
		if len(c.vars) == 1 && strings.HasSuffix(c.vars[0], "_") {
			for _, k := range read {
				if !strings.HasPrefix(k, c.vars[0]) {
					t.Errorf("the check of %s* reads %s", c.vars[0], k)
				}
			}
			continue
		}
		if want := slices.Sorted(slices.Values(c.vars)); !slices.Equal(read, want) {
			t.Errorf("the check of %v reads %v", c.vars, read)
		}
	}
}

// hideSecrets names the value of MASSIMO_TLS_KEY by the variable, as it
// is and as %q escapes it, in one pass: a value the name holds, a letter
// and a Windows path come out named once, and other values stay.
func TestHideSecrets(t *testing.T) {
	t.Parallel()
	for _, c := range []struct{ key, text, want string }{
		{"KEY", "tls: stat KEY: no such file or directory", "tls: stat MASSIMO_TLS_KEY: no such file or directory"},
		{"S", "tls: stat S: no such file or directory", "tls: stat MASSIMO_TLS_KEY: no such file or directory"},
		{`C:\ssl\key.pem`, `tls: open C:\ssl\key.pem: denied; "C:\\ssl\\key.pem"`, `tls: open MASSIMO_TLS_KEY: denied; "MASSIMO_TLS_KEY"`},
		{"/etc/ssl/key.pem", "tls: open /etc/ssl/cert.pem: denied", "tls: open /etc/ssl/cert.pem: denied"},
		{"", "tls: stat key.pem: no such file or directory", "tls: stat key.pem: no such file or directory"},
	} {
		values := map[string]string{env.TLSKey: c.key, env.TLSCert: "/etc/ssl/cert.pem"}
		if got := hideSecrets(c.text, func(k string) string { return values[k] }); got != c.want {
			t.Errorf("key %q: %q, want %q", c.key, got, c.want)
		}
	}
}

// The guard of the start's checks (AP ReloadChecks, D4-A): every
// variable serve reads is one its start refuses a bad value of, alone or
// beside others, or one it takes any value of, for a reason. The built
// massimo runs once per bad value, nothing else wrong, and stops with
// the table's own error, the one a reload warns of; a value the start
// alone refuses, on the box's state, the table leaves be (D1-A); and,
// with every variable that takes any value set to an odd one, it starts.
// The table runs here too, over the same settings in the environment,
// where its check of the Cubes reads them (serve.FileCubes). It catches
// a variable nobody classified, a refusal of the start's without its
// entry, a text the start says otherwise than the table, and an entry
// for a value the start takes; not a pairing nobody listed.
func TestStartChecksCoverTheStart(t *testing.T) {
	if testing.Short() {
		t.Skip("runs the built massimo once per refused setting")
	}
	if runtime.GOOS == "windows" {
		t.Skip("the built massimo is no .exe (builtBinary)")
	}
	cases := startRefusalCases(t, t.TempDir())
	refused := map[string]bool{}
	for _, c := range cases {
		refused[c.of] = true
	}

	// Every variable serve reads is classified once, and every one
	// classified is one it reads and env.Known knows.
	named := serveVariables(t)
	for name, where := range named {
		_, anyValue := startTakesAny[name]
		switch {
		case refused[name] && anyValue:
			t.Errorf("%s is both refused and taking any value", name)
		case !refused[name] && !anyValue:
			t.Errorf("%s (%s) is not classified: a refusal of the start's for it (startRefusalCases), or why the start takes any value (startTakesAny)", name, where)
		}
		if !env.Known(name) && !env.Removed(name) {
			t.Errorf("%s (%s) is not known to env.Known: massimo config calls it a typo, and a reload lists no change of it", name, where)
		}
	}
	for name := range refused {
		if _, ok := named[name]; !ok {
			t.Errorf("a refusal of %s, which serve does not read", name)
		}
	}
	for name := range startTakesAny {
		if _, ok := named[name]; !ok {
			t.Errorf("%s takes any value, but serve does not read it", name)
		}
	}
	for _, c := range startChecks {
		for _, v := range c.vars {
			if !strings.HasSuffix(v, "_") && !refused[v] {
				t.Errorf("the check of %v reads %s, which no case refuses", c.vars, v)
			}
		}
	}

	// The table over each case's settings, in the environment as the
	// settings file puts them there, with the flags the run gives; what
	// this process's environment sets, which the built massimo does not
	// get (guardEnv), unset.
	var anyLines []string
	for name, a := range startTakesAny {
		anyLines = append(anyLines, name+"="+a.odd)
	}
	slices.Sort(anyLines)
	clearCubeEnv(t)
	unset := map[string]bool{}
	for _, lines := range append([][]string{anyLines}, linesOf(cases)...) {
		for _, l := range lines {
			k, _, _ := strings.Cut(l, "=")
			unset[k] = true
		}
	}
	for _, c := range startChecks {
		for _, v := range c.vars {
			if !strings.HasSuffix(v, "_") {
				unset[v] = true
			}
		}
	}
	for k := range unset {
		unsetForTest(t, k)
	}
	table := func(lines []string) (errs []error, first int) {
		for _, l := range lines {
			k, v, _ := strings.Cut(l, "=")
			os.Setenv(k, v)
		}
		defer func() {
			for _, l := range lines {
				k, _, _ := strings.Cut(l, "=")
				os.Unsetenv(k)
			}
		}()
		_, given := guardArgs("", "", "", lines)
		get := settingOf(fixedSettings(given))
		first = -1
		for i, c := range startChecks {
			if c.check(get) != nil {
				first = i
				break
			}
		}
		return startRefusals(get, nil), first
	}
	wants := make([]string, len(cases))
	covered := map[int]bool{}
	for i, c := range cases {
		if !slices.ContainsFunc(c.lines, func(l string) bool { return strings.HasPrefix(l, c.of+"=") }) {
			t.Errorf("%s: no line of %s", c.what, c.of)
		}
		errs, first := table(c.lines)
		if len(errs) > 0 {
			wants[i] = errs[0].Error()
		}
		switch {
		case c.alone != "":
			if len(errs) > 0 {
				t.Errorf("%s: the table refuses it, which the start alone was to (%s): %v", c.what, c.alone, errs)
			}
			if c.says == "" {
				t.Errorf("%s: no beginning of the start's own error", c.what)
			}
		case len(errs) == 0:
			t.Errorf("%s: the table refuses nothing of %q", c.what, c.lines)
		case len(errs) > 1:
			t.Errorf("%s: %d refusals, where one value is wrong: %v", c.what, len(errs), errs)
		case !startChecks[first].reads(c.of):
			t.Errorf("%s: refused by the check of %v, which does not read %s", c.what, startChecks[first].vars, c.of)
		default:
			covered[first] = true
		}
	}
	for i, c := range startChecks {
		if !covered[i] {
			t.Errorf("no case is refused by the check of %v", c.vars)
		}
	}
	if errs, _ := table(anyLines); len(errs) > 0 {
		t.Errorf("the table refuses values of those that take any: %v", errs)
	}

	// The start, from the built massimo, side by side.
	base := guardEnv()
	isError := func(r testkit.Record) bool { return strings.HasPrefix(r.Line, "error: ") }
	for i, c := range cases {
		t.Run(c.what, func(t *testing.T) {
			t.Parallel()
			s := guardStart(t, base, c.lines)
			r, _ := s.log.Await(t, 0, "the start's error: line", func(r testkit.Record) bool {
				return isError(r) || !c.listens && r.Msg == "api listening"
			}, serveWait)
			switch {
			case !isError(r) && wants[i] != "":
				t.Errorf("the start takes %q and listens; the table refuses it with %q", c.lines, wants[i])
			case !isError(r):
				t.Errorf("the start takes %q and listens", c.lines)
			case c.alone != "":
				if !strings.HasPrefix(r.Line, "error: "+c.says) {
					t.Errorf("the start stops with\n%s\nnot with its own error, which begins\nerror: %s", r.Line, c.says)
				}
			case wants[i] == "":
				t.Errorf("the start refuses it, the table does not:\n%s", r.Line)
			case r.Line != "error: serve: "+wants[i]:
				t.Errorf("the start stops with\n%s\nthe table refuses it with\nerror: serve: %s", r.Line, wants[i])
			}
		})
	}
	t.Run("an odd value of each that takes any", func(t *testing.T) {
		t.Parallel()
		s := guardStart(t, base, anyLines)
		r, _ := s.log.Await(t, 0, "api listening", func(r testkit.Record) bool { return isError(r) || r.Msg == "api listening" }, serveWait)
		if isError(r) {
			t.Errorf("the start refuses a value of those that take any:\n%s", r.Line)
		}
	})
}

// startRefusal is a refusal of the start's the guard runs: the variable
// whose value it refuses, the settings file's lines, with nothing else
// wrong in them, and, for a value the start alone refuses (D1-A), why,
// and what its error begins with, after "error: ", up to the parts that
// differ between systems; listens says that the start refuses it after
// "api listening".
type startRefusal struct {
	of, what    string
	lines       []string
	alone, says string
	listens     bool
}

// linesOf are the cases' settings files, their lines each.
func linesOf(cases []startRefusal) [][]string {
	var out [][]string
	for _, c := range cases {
		out = append(out, c.lines)
	}
	return out
}

// guardCube is the serial of the guard's per-Cube variables.
const guardCube = "KEQ0000000"

// startRefusalCases are the start's refusals, in its table's order, one
// bad value each or a pairing of values that go together alone, then
// those the start alone makes; dir holds the files they name.
func startRefusalCases(t *testing.T, dir string) []startRefusal {
	t.Helper()
	file := func(name, text string, mode os.FileMode) string {
		t.Helper()
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(text), mode); err != nil {
			t.Fatal(err)
		}
		return p
	}
	cert := file("cert.pem", "not read: the key is missing\n", 0o600)
	key := file("key.pem", "not read: the certificate is missing\n", 0o600)
	hook := file("hook.sh", "#!/bin/sh\n", 0o755)
	noExec := file("noexec.sh", "#!/bin/sh\n", 0o644)
	notDir := file("file", "a file where a directory belongs\n", 0o600)
	logFile := filepath.Join(notDir, "serve.log")
	missing := filepath.Join(dir, "missing")
	acme := []string{env.TLS + "=acme", env.TLSNames + "=massimo.example.net"}
	cubeAddress := env.PerCube(env.Address, guardCube) + "=127.0.0.1:1"
	return []startRefusal{
		{of: env.LogFormat, what: "a bad format", lines: []string{env.LogFormat + "=xml"}},
		{of: env.LogFileFormat, what: "none in the file", lines: []string{env.LogFileFormat + "=none"}},
		{of: env.LogProcessFormat, what: "a bad format on stderr", lines: []string{env.LogProcessFormat + "=yaml"}},
		{of: env.LogRotateSize, what: "a bad rotation size", lines: []string{env.LogRotateSize + "=big"}},
		{of: env.LogKeep, what: "a bad number of files", lines: []string{env.LogKeep + "=many"}},
		{of: env.LogKeepMin, what: "a bad size to keep", lines: []string{env.LogKeepMin + "=-1"}},
		{of: env.LogLevel, what: "a bad level", lines: []string{env.LogLevel + "=deubg"}},
		{of: env.HSTS, what: "a bad HSTS", lines: []string{env.HSTS + "=-1"}},
		{of: env.TrustedProxies, what: "a bad proxy", lines: []string{env.TrustedProxies + "=10.0.0.300"}},
		{of: restartVar, what: "a bad restart", lines: []string{restartVar + "=always"}},
		{of: sighupVar, what: "a bad SIGHUP", lines: []string{sighupVar + "=ignore"}},
		{of: rebootVar, what: "a bad reboot switch", lines: []string{rebootVar + "=maybe"}},
		{of: env.Announce, what: "a bad announcement", lines: []string{env.Announce + "=maybe"}},
		{of: env.TLS, what: "a bad TLS mode", lines: []string{env.TLS + "=yes"}},
		{of: env.TLS, what: "files without the files", lines: []string{env.TLS + "=files"}},
		{of: env.TLSCert, what: "a certificate that is not there", lines: []string{env.TLS + "=files", env.TLSCert + "=" + missing, env.TLSKey + "=" + key}},
		{of: env.TLSKey, what: "a key that is not there", lines: []string{env.TLS + "=files", env.TLSCert + "=" + cert, env.TLSKey + "=" + missing}},
		{of: env.TLSNames, what: "acme without names", lines: []string{env.TLS + "=acme", env.TLSNames + "="}},
		{of: env.TLSNames, what: "a name without a dot", lines: []string{env.TLS + "=acme", env.TLSNames + "=massimo"}},
		{of: env.TLSNames, what: "a wildcard without DNS-01", lines: []string{env.TLS + "=acme", env.TLSNames + "=*.example.net"}},
		{of: env.TLSNames, what: "a DNS-01 name without a dot", lines: []string{env.TLS + "=acme", env.TLSNames + "=massimo", env.ACMEDNSHook + "=" + hook}},
		{of: env.TLSHTTP, what: "the HTTP-01 port without acme", lines: []string{env.TLSHTTP + "=:80"}},
		{of: env.TLSHTTP, what: "the HTTP-01 port beside a DNS hook", lines: append(slices.Clone(acme), env.TLSHTTP+"=:80", env.ACMEDNSHook+"="+hook)},
		{of: env.ACMEDNSHook, what: "a DNS hook without acme", lines: []string{env.ACMEDNSHook + "=" + hook}},
		{of: env.ACMEDNSHook, what: "a DNS hook that is not there", lines: append(slices.Clone(acme), env.ACMEDNSHook+"="+missing)},
		{of: env.ACMEDNSHook, what: "a DNS hook that is not executable", lines: append(slices.Clone(acme), env.ACMEDNSHook+"="+noExec)},
		{of: env.PerCube(env.Ignore, guardCube), what: "a Cube's bad hidden list", lines: []string{cubeAddress, env.PerCube(env.Ignore, guardCube) + "=zz"}},
		{of: env.Ignore, what: "the plain hidden list beside a Cube", lines: []string{cubeAddress, env.Ignore + "=zz"}},

		// The start's own errors, up to the system's error text, such as
		// "not a directory".
		{of: "MASSIMO_LISTEN", what: "a listen address that does not parse", lines: []string{"MASSIMO_LISTEN=127.0.0.1:99999"},
			alone: "a port taken shows at the listen alone, and every packaged service gives -listen",
			says:  "listen tcp: address 99999: invalid port"},
		{of: "MASSIMO_LOG", what: "a log file that does not open", lines: []string{"MASSIMO_LOG=" + logFile},
			alone: "a directory missing or not writable shows at the open alone, and every packaged service gives -log",
			says:  "serve: open " + logFile + ": "},
		{of: env.DataVar, what: "a data directory that is a file", lines: []string{env.DataVar + "=" + notDir},
			alone: "a directory missing or not writable shows at the store's open alone, and every packaged service gives -data",
			says:  "serve: store: mkdir " + notDir + ": "},
		// Refused after the API listens, so the ACME directory is the
		// loopback's, which nothing asks before the start ends.
		{of: env.TLSHTTP, what: "an HTTP-01 port that does not parse", alone: "a port taken shows at the listen alone", listens: true,
			lines: append(slices.Clone(acme), env.TLSHTTP+"=127.0.0.1:99999", env.ACMEDirectory+"=http://127.0.0.1:1/directory"),
			says:  "serve: -tls-http: listen tcp: address 99999: invalid port"},
	}
}

// anyValue is a variable the start takes any value of: an odd one, for
// the run that shows it, and why.
type anyValue struct{ odd, why string }

// startTakesAny are the variables serve reads whose every value its start
// takes, those of the Cubes in their plain and their per-Cube forms.
var startTakesAny = map[string]anyValue{
	tokenVar:                 {"an odd token", "any text is a token; one the store cannot keep is the store's error"},
	env.ACMEEmail:            {"not an address", "the ACME order's, after the start, which logs its failure; unread outside acme"},
	env.ACMEDirectory:        {"not a URL", "the ACME order's, after the start, which logs its failure; unread outside acme"},
	env.PublicURL:            {"not a URL", "the announcement's url, as it is"},
	"MASSIMO_UPDATE_REQUEST": {"no/such/request", "the request file an update from the API writes"},
	"MASSIMO_UPDATE_CHANNEL": {"nightlyy", "the channel an update check asks for, which says so of one it does not know"},
	update.APIVar:            {"not a URL", "an update check's, which fails on a bad one and says so"},
	update.TokenVar:          {"not a token", "an update check's, which fails on a bad one and says so"},
	env.FileVar:              {"no such file", "names the settings file, read before its lines: a file not there is none, and one that cannot be read stops every command, the file's refusal, not a setting's"},
	env.SN:                   {"not a serial", "picks a Cube: one neither the file nor the store has is looked for on the LAN"},

	env.Address:                             {"127.0.0.1:99999", "a bad address costs that Cube alone, which warns and tries again"},
	env.PerCube(env.Address, guardCube):     {"127.0.0.1:99999", "a bad address costs that Cube alone, which warns and tries again"},
	env.Name:                                {"an odd name", "a Cube's name, as it is"},
	env.PerCube(env.Name, guardCube):        {"an odd name", "a Cube's name, as it is"},
	env.Proxy:                               {"127.0.0.1:99999", "a bad proxy address costs that Cube's proxy, which warns; the guard's runs give -proxy off"},
	env.PerCube(env.Proxy, guardCube):       {"127.0.0.1:99999", "a bad proxy address costs that Cube's proxy, which warns; the guard's runs give -proxy off"},
	env.ProxySerial:                         {"not a serial", "the serial of the proxy's discovery answers, as it is"},
	env.PerCube(env.ProxySerial, guardCube): {"not a serial", "the serial of the proxy's discovery answers, as it is"},
	env.Key:                                 {"not hex", "read no more, the portal's (env.Removed)"},
	env.PerCube(env.Key, guardCube):         {"not hex", "read no more, the portal's (env.Removed)"},
	env.IV:                                  {"not hex", "read no more, the portal's (env.Removed)"},
	env.PerCube(env.IV, guardCube):          {"not hex", "read no more, the portal's (env.Removed)"},
}

// serveVariables are the variables serve reads, each with where it is
// named: the MASSIMO_ names the product's code spells, a string literal
// each or env.Prefix and one, in the files massimo is built from
// (serveFiles), every command's, which serve reads too; the variables of
// flagVars and of the start's checks; and the per-Cube form, for
// guardCube, of each Cube variable that has one. env.Known is a switch
// and lists nothing, so the code is the list: a name made otherwise at
// run time is not found, and the platform's variables, such as systemd's
// INVOCATION_ID and Windows's ProgramData, say where serve runs and are
// no settings.
func serveVariables(t *testing.T) map[string]string {
	t.Helper()
	isName := regexp.MustCompile(`^MASSIMO_[A-Z0-9]+(_[A-Z0-9]+)*$`)
	out := map[string]string{}
	add := func(name, where string) {
		if _, ok := out[name]; !ok {
			out[name] = where
		}
	}
	fset := token.NewFileSet()
	for _, path := range serveFiles(t) {
		src, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Contains(src, []byte("MASSIMO_")) && !bytes.Contains(src, []byte("Prefix")) {
			continue
		}
		f, err := parser.ParseFile(fset, path, src, parser.SkipObjectResolution)
		if err != nil {
			t.Fatal(err)
		}
		// prefix reports whether e is env.Prefix, its name in package
		// env and the selector elsewhere, or a sum that ends in it, as
		// "…"+env.Prefix.
		prefix := func(e ast.Expr) bool {
			for {
				b, ok := e.(*ast.BinaryExpr)
				if !ok || b.Op != token.ADD {
					break
				}
				e = b.Y
			}
			switch e := e.(type) {
			case *ast.Ident:
				return f.Name.Name == "env" && e.Name == "Prefix"
			case *ast.SelectorExpr:
				x, ok := e.X.(*ast.Ident)
				return ok && x.Name == "env" && e.Sel.Name == "Prefix"
			}
			return false
		}
		ast.Inspect(f, func(n ast.Node) bool {
			var name string
			var at token.Pos
			switch n := n.(type) {
			case *ast.BasicLit:
				if n.Kind == token.STRING {
					name, _ = strconv.Unquote(n.Value)
					at = n.Pos()
				}
			case *ast.BinaryExpr:
				if lit, ok := n.Y.(*ast.BasicLit); ok && n.Op == token.ADD && lit.Kind == token.STRING && prefix(n.X) {
					s, _ := strconv.Unquote(lit.Value)
					name, at = env.Prefix+s, lit.Pos()
				}
			}
			if isName.MatchString(name) {
				add(name, fset.Position(at).String())
			}
			return true
		})
	}
	for f, v := range flagVars {
		add(v, "flagVars, -"+f)
	}
	for _, c := range startChecks {
		for _, v := range c.vars {
			if !strings.HasSuffix(v, "_") {
				add(v, "startChecks")
			}
		}
	}
	for name := range maps.Clone(out) {
		if pc := env.PerCube(name, guardCube); strings.HasPrefix(name, env.Prefix) && (env.Known(pc) || env.Removed(pc)) {
			add(pc, "the per-Cube form of "+name)
		}
	}
	return out
}

// serveFiles are the files massimo is built from on Linux, macOS and
// Windows, of this module's packages alone: those go list -deps names
// for each system, which leaves out the tests' files and the test kit,
// as no file of the program imports it (TestOnlyTestsImportTheKit). A
// path is relative to this directory, as a failure names it.
func serveFiles(t *testing.T) []string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	files := map[string]bool{}
	for _, goos := range []string{"linux", "darwin", "windows"} {
		cmd := exec.Command("go", "list", "-deps", "-json=Dir,GoFiles,CgoFiles,Module", ".")
		cmd.Env = append(os.Environ(), "GOOS="+goos)
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		out, err := cmd.Output()
		if err != nil {
			t.Fatalf("go list for %s: %v\n%s", goos, err, stderr.Bytes())
		}
		for d := json.NewDecoder(bytes.NewReader(out)); d.More(); {
			var p struct {
				Dir               string
				GoFiles, CgoFiles []string
				Module            *struct{ Main bool }
			}
			if err := d.Decode(&p); err != nil {
				t.Fatalf("go list for %s: %v", goos, err)
			}
			if p.Module == nil || !p.Module.Main {
				continue
			}
			for _, name := range append(p.GoFiles, p.CgoFiles...) {
				path := filepath.Join(p.Dir, name)
				if rel, err := filepath.Rel(wd, path); err == nil {
					path = rel
				}
				files[path] = true
			}
		}
	}
	return slices.Sorted(maps.Keys(files))
}

// guardArgs are the built massimo's arguments for a settings file and a
// data directory: loopback only, a search on the LAN only at target, a
// socket the test holds, and no proxy; -data and -listen where the lines
// do not set their variables, which the flags would fix. given are
// serve's flags given, as fixedSettings takes them.
func guardArgs(file, data, target string, lines []string) (args []string, given map[string]string) {
	sets := func(k string) bool {
		return slices.ContainsFunc(lines, func(l string) bool { return strings.HasPrefix(l, k+"=") })
	}
	args = []string{"-env", file}
	if !sets(env.DataVar) {
		args = append(args, "-data", data)
	}
	given = map[string]string{"discover-interval": "0", "discover-target": target, "discover-listen": "127.0.0.1:0", "proxy": "off"}
	if !sets("MASSIMO_LISTEN") {
		given["listen"] = "127.0.0.1:0"
	}
	args = append(args, "serve")
	for _, f := range slices.Sorted(maps.Keys(given)) {
		args = append(args, "-"+f, given[f])
	}
	return args, given
}

// guardEnv is the built massimo's environment: this process's, without
// its MASSIMO_ variables and the marks of a systemd service, and with no
// settings file but the one -env names.
func guardEnv() []string {
	var out []string
	for _, kv := range os.Environ() {
		if !strings.HasPrefix(kv, "MASSIMO_") && !strings.HasPrefix(kv, "INVOCATION_ID=") && !strings.HasPrefix(kv, "SYSTEMD_EXEC_PID=") {
			out = append(out, kv)
		}
	}
	return append(out, env.FileVar+"="+os.DevNull)
}

// guardStart starts serve from the built massimo over a settings file of
// lines, with a data directory of its own (guardArgs) and base for its
// environment.
func guardStart(t *testing.T, base, lines []string) *builtServe {
	t.Helper()
	settingsFile := filepath.Join(t.TempDir(), "env")
	if err := os.WriteFile(settingsFile, []byte(strings.Join(lines, "\n")+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	args, _ := guardArgs(settingsFile, t.TempDir(), testkit.ListenPacket(t).LocalAddr().String(), lines)
	cmd := exec.Command(builtBinary(t), args...)
	cmd.Env = base
	return runBuilt(t, cmd)
}
