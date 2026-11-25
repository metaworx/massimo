// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package install

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"unicode/utf16"

	"go.metaworx.ch/massimo/internal/env"
)

// readEnv reads the settings file at path as serve reads it, for the
// tests that look at what an install left there: the value of each name.
func readEnv(path string) (map[string]string, error) {
	vars, _, err := env.ReadFile(path)
	return vars, err
}

// utf16File writes text as UTF-16LE after the UTF-8 lines head, with
// the byte order mark where bom is set: as PowerShell 5.1's > writes a
// file, or its >> appends to a UTF-8 one. It returns the file's path.
func utf16File(t *testing.T, head, text string, bom bool) string {
	t.Helper()
	units := utf16.Encode([]rune(text))
	if bom {
		units = append([]uint16{0xFEFF}, units...)
	}
	b := []byte(head)
	for _, u := range units {
		b = binary.LittleEndian.AppendUint16(b, u)
	}
	p := filepath.Join(t.TempDir(), "settings.env")
	if err := os.WriteFile(p, b, 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

// An install reads the settings file as serve reads it (env.ReadFile):
// a UTF-8 byte order mark before the first setting is skipped, quotes
// are stripped, the first line of a name wins, and the lines the read
// sets aside are each said once, by their numbers and never a value,
// however often the install reads the file. The copy is the file as it
// was.
func TestInstallReadsSettingsAsServe(t *testing.T) {
	root, bin := setup(t)
	text := "\xEF\xBB\xBFMASSIMO_API_TOKEN=placeholder-secret\n" +
		"MASSIMO_TLS='self-signed'\n" +
		"MASSIMO_TLS=off\n" +
		"export MASSIMO_CUBE_KEQ0000000_PROXY=\"192.0.2.9\"\n" +
		"=placeholder-secret\n" +
		"MASSIMO_ENV_FILE=/elsewhere.env\n"
	settings := filepath.Join(t.TempDir(), ".env")
	os.WriteFile(settings, []byte(text), 0o600)
	var out bytes.Buffer
	local := func() ([]net.IP, error) { return []net.IP{net.ParseIP("192.0.2.1")}, nil }
	if err := Install(Options{Root: root, Binary: bin, Settings: settings, Out: &out, Run: (&recorder{}).run, LocalAddrs: local}); err != nil {
		t.Fatal(err)
	}
	o := out.String()
	if strings.Contains(o, "no MASSIMO_API_TOKEN") {
		t.Errorf("the token after the byte order mark not seen:\n%s", o)
	}
	if !strings.Contains(o, "warning: MASSIMO_CUBE_KEQ0000000_PROXY=192.0.2.9 is not an address of this machine") {
		t.Errorf("the quoted proxy address not seen:\n%s", o)
	}
	// The first MASSIMO_TLS: the helper checks the server over https.
	if b, _ := os.ReadFile(filepath.Join(root, HealthDropIn)); !strings.Contains(string(b), "MASSIMO_HEALTH=https://") {
		t.Errorf("the health drop-in does not follow the first MASSIMO_TLS: %q", b)
	}
	for _, note := range []string{
		"warning: " + EnvPath + ": MASSIMO_TLS is given on lines 2 and 3; the first wins\n",
		"warning: " + EnvPath + ": line 5 has no name before its =, ignored\n",
		"warning: " + EnvPath + ": line 6 sets MASSIMO_ENV_FILE, ignored: name the file with -env\n",
	} {
		if n := strings.Count(o, note); n != 1 {
			t.Errorf("%q said %d times:\n%s", note, n, o)
		}
	}
	if strings.Contains(o, "placeholder-secret") {
		t.Errorf("a value in the output:\n%s", o)
	}
	if read(t, filepath.Join(root, EnvPath)) != text {
		t.Error("the copy is not the file as it was")
	}
}

// A dry run reads the -settings file it would copy, and names it in the
// notes; MASSIMO_TLS_HTTP is the first line's, the capability for its
// port below 1024 following it.
func TestInstallDryRunReadsSettingsAsServe(t *testing.T) {
	root, bin := setup(t)
	settings := filepath.Join(t.TempDir(), ".env")
	os.WriteFile(settings, []byte("MASSIMO_API_TOKEN=t\nMASSIMO_TLS=acme\nMASSIMO_TLS_NAMES=massimo.example.net\nMASSIMO_TLS_HTTP=:80\nMASSIMO_TLS_HTTP=:8080\n"), 0o600)
	var out bytes.Buffer
	if err := Install(Options{Root: root, Binary: bin, Settings: settings, DryRun: true, Out: &out, Run: (&recorder{}).run}); err != nil {
		t.Fatal(err)
	}
	o := out.String()
	if n := strings.Count(o, "warning: "+settings+": MASSIMO_TLS_HTTP is given on lines 4 and 5; the first wins\n"); n != 1 {
		t.Errorf("the note said %d times:\n%s", n, o)
	}
	if !strings.Contains(o, "would write "+ListenDropIn) {
		t.Errorf("no listen drop-in for the first MASSIMO_TLS_HTTP's port below 1024:\n%s", o)
	}
}

// On macOS the warning of MASSIMO_LOG, which the plist's -log overrides,
// reads the file as serve does: after a byte order mark, and not where
// the first of two lines is empty.
func TestDarwinSettingsReadAsServe(t *testing.T) {
	for _, c := range []struct {
		what, text string
		warn       bool
	}{
		{"after a byte order mark", "\xEF\xBB\xBFMASSIMO_LOG=/tmp/x.log\nMASSIMO_API_TOKEN=t\n", true},
		{"its first line empty", "MASSIMO_API_TOKEN=t\nMASSIMO_LOG=\nMASSIMO_LOG=/tmp/x.log\n", false},
	} {
		root, bin := setup(t)
		settings := filepath.Join(t.TempDir(), ".env")
		os.WriteFile(settings, []byte(c.text), 0o600)
		var out bytes.Buffer
		o := darwinOpts(root, bin, &recorder{}, &out)
		o.Settings = settings
		if err := InstallDarwin(o); err != nil {
			t.Fatal(err)
		}
		if got := strings.Contains(out.String(), "warning: MASSIMO_LOG in "+EnvPath+" is not read"); got != c.warn {
			t.Errorf("%s: warned %v, want %v:\n%s", c.what, got, c.warn, out.String())
		}
	}
}

// tree is what is under root: each file's content and each directory,
// "dir", by its path.
func tree(t *testing.T, root string) map[string]string {
	t.Helper()
	m := map[string]string{}
	filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		switch {
		case err != nil:
			t.Fatal(err)
		case d.IsDir():
			m[p] = "dir"
		default:
			b, _ := os.ReadFile(p)
			m[p] = string(b)
		}
		return nil
	})
	return m
}

// A -settings file in UTF-16 text, as PowerShell 5.1's > writes it or
// its >> appends to a UTF-8 file, is refused with the parser's text
// before the install writes, runs or says anything, in a dry run too, by
// a hand install, by one on a packaged box and by one on macOS, also
// where the settings file there is kept and it would not be copied: no
// binary, no user, no directory, no settings file.
func TestInstallRefusesUTF16Settings(t *testing.T) {
	whole := utf16File(t, "", "# Settings\r\nMASSIMO_API_TOKEN=t\r\n", true)
	appended := utf16File(t, "# head\r\nnot a setting\r\n", "MASSIMO_LISTEN=:80\r\n", false)
	for _, k := range []struct {
		what             string
		packaged, darwin bool
	}{{"a hand install", false, false}, {"a packaged box", true, false}, {"macOS", false, true}} {
		for _, f := range []struct{ path, at string }{{whole, ""}, {appended, ":3"}} {
			for _, c := range []struct {
				dry, kept bool
			}{{false, false}, {true, false}, {false, true}} {
				root, bin := setup(t)
				if k.packaged {
					root = packagedRoot(t)
				}
				if c.kept {
					os.MkdirAll(filepath.Join(root, EnvDir), 0o750)
					os.WriteFile(filepath.Join(root, EnvPath), []byte("MASSIMO_API_TOKEN=mine\n"), 0o600)
				}
				before := tree(t, root)
				var out bytes.Buffer
				rec := &recorder{}
				o, install := Options{Root: root, Binary: bin, Out: &out, Run: rec.run}, Install
				if k.darwin {
					o, install = darwinOpts(root, bin, rec, &out), InstallDarwin
				}
				o.Settings, o.DryRun = f.path, c.dry
				err := install(o)
				what := fmt.Sprintf("%s, %s%s, dry run %v, kept %v", k.what, f.path, f.at, c.dry, c.kept)
				if want := f.path + f.at + ": UTF-16 text, save it as UTF-8"; !errors.Is(err, env.ErrUTF16) || err.Error() != want {
					t.Errorf("%s: %v, want %s", what, err, want)
				}
				if after := tree(t, root); !maps.Equal(after, before) {
					t.Errorf("%s: written before the refusal: %q, before %q", what, after, before)
				}
				if out.Len() != 0 || len(rec.cmds) != 0 {
					t.Errorf("%s: before the refusal said %q, ran %q", what, out.String(), rec.cmds)
				}
			}
		}
	}
}

// -tls over a settings file that starts with a UTF-8 byte order mark,
// as Windows' editors save it, replaces the line serve takes, the first
// setting's, and keeps the mark; a name given twice has each of its
// lines replaced, the first, which serve takes, too.
func TestInstallTLSOverByteOrderMark(t *testing.T) {
	const added = "MASSIMO_TLS_CERT=/etc/massimo/cert.pem\nMASSIMO_TLS_KEY=/etc/massimo/key.pem\n"
	for _, c := range []struct{ what, text, want string }{
		{"after a byte order mark", utf8BOM + "MASSIMO_TLS=off\nMASSIMO_API_TOKEN=t\n", utf8BOM + "MASSIMO_TLS=files\nMASSIMO_API_TOKEN=t\n" + added},
		{"given twice", "MASSIMO_API_TOKEN=t\nMASSIMO_TLS=off\nexport MASSIMO_TLS=self-signed\n", "MASSIMO_API_TOKEN=t\nMASSIMO_TLS=files\nMASSIMO_TLS=files\n" + added},
	} {
		root, bin := setup(t)
		settings := filepath.Join(root, EnvPath)
		os.MkdirAll(filepath.Dir(settings), 0o750)
		os.WriteFile(settings, []byte(c.text), 0o600)
		cert, key := filepath.Join(t.TempDir(), "c.pem"), filepath.Join(t.TempDir(), "k.pem")
		os.WriteFile(cert, []byte("CERT"), 0o644)
		os.WriteFile(key, []byte("KEY"), 0o600)
		if err := Install(Options{Root: root, Binary: bin, TLSCert: cert, TLSKey: key, Out: &bytes.Buffer{}, Run: (&recorder{}).run}); err != nil {
			t.Fatal(err)
		}
		vars, _, err := env.ReadFile(settings)
		if err != nil || vars[tlsVar] != "files" {
			t.Errorf("%s: serve reads %s=%q, %v", c.what, tlsVar, vars[tlsVar], err)
		}
		if got := read(t, settings); got != c.want {
			t.Errorf("%s: %q, want %q", c.what, got, c.want)
		}
	}
}

// The notes on the command's own settings file are its start's
// (cmd/massimo), which said them: install says none of its own for it,
// in a dry run of "install -settings .env" in the directory of that
// .env, with -env naming it, by a link to it, nor for the copy a run
// makes of it; another file's it says once, the kept settings file's
// too.
func TestInstallNotesOnceWithTheCommandsFile(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	const note = ": MASSIMO_API_TOKEN is given on lines 1 and 2; the first wins\n"
	text := []byte("MASSIMO_API_TOKEN=t\nMASSIMO_API_TOKEN=u\n")
	own := filepath.Join(dir, ".env")
	os.WriteFile(own, text, 0o600)
	other := filepath.Join(t.TempDir(), "other.env")
	os.WriteFile(other, text, 0o600)
	link := filepath.Join(t.TempDir(), "link.env")
	if err := os.Link(own, link); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		what, envFile, settings string
		dry, kept               bool
		said                    int
	}{
		{"a dry run, the default .env", "", "./.env", true, false, 0},
		{"a dry run, -env naming it", own, ".env", true, false, 0},
		{"a dry run, a hard link to it", "", link, true, false, 0},
		{"a dry run, another file", own, other, true, false, 1},
		{"its copy", "", ".env", false, false, 0},
		{"another file's copy", "", other, false, false, 1},
		{"the kept settings file", "", ".env", false, true, 1},
	}
	if runtime.GOOS != "windows" { // a symbolic link needs a privilege there
		sym := filepath.Join(t.TempDir(), "sym.env")
		if err := os.Symlink(own, sym); err != nil {
			t.Fatal(err)
		}
		cases = append(cases, struct {
			what, envFile, settings string
			dry, kept               bool
			said                    int
		}{"a dry run, a symbolic link to it", "", sym, true, false, 0})
	}
	for _, c := range cases {
		t.Setenv(env.FileVar, c.envFile)
		root, bin := setup(t)
		if c.kept {
			os.MkdirAll(filepath.Join(root, EnvDir), 0o750)
			os.WriteFile(filepath.Join(root, EnvPath), text, 0o600)
		}
		var out bytes.Buffer
		if err := Install(Options{Root: root, Binary: bin, Settings: c.settings, DryRun: c.dry, Out: &out, Run: (&recorder{}).run}); err != nil {
			t.Fatal(err)
		}
		if n := strings.Count(out.String(), note); n != c.said {
			t.Errorf("%s: the note said %d times, want %d:\n%s", c.what, n, c.said, out.String())
		}
	}
}

// A settings file serve refuses for one line that is not KEY=VALUE is
// read leniently by install's checks (env.ReadFileLenient): the line is
// a note, the others' settings count as readEnv's did, so that the
// health drop-in, the capability for MASSIMO_TLS_HTTP's port and the
// macOS helper's health URL are as before, not plain http.
func TestInstallReadsAFileServeRefuses(t *testing.T) {
	text := "MASSIMO_API_TOKEN=t\nMASSIMO_TLS=acme\nMASSIMO_TLS_NAMES=massimo.example.net\nnot a setting\nMASSIMO_TLS_HTTP=:80\n"
	settings := filepath.Join(t.TempDir(), ".env")
	os.WriteFile(settings, []byte(text), 0o600)
	const note = ": line 4 is not KEY=VALUE: serve refuses the file until it is fixed\n"

	root, bin := setup(t)
	var out bytes.Buffer
	if err := Install(Options{Root: root, Binary: bin, Settings: settings, Out: &out, Run: (&recorder{}).run}); err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(out.String(), "warning: "+EnvPath+note); n != 1 || strings.Contains(out.String(), "cannot check") || strings.Contains(out.String(), "no MASSIMO_API_TOKEN") {
		t.Errorf("the note said %d times, or the file not read:\n%s", n, out.String())
	}
	if b, _ := os.ReadFile(filepath.Join(root, HealthDropIn)); !strings.Contains(string(b), "MASSIMO_HEALTH=https://") {
		t.Errorf("health drop-in: %q", b)
	}
	if b, _ := os.ReadFile(filepath.Join(root, ListenDropIn)); !strings.Contains(string(b), "AmbientCapabilities=CAP_NET_BIND_SERVICE") {
		t.Errorf("listen drop-in: %q", b)
	}

	root, bin = setup(t)
	out.Reset()
	o := darwinOpts(root, bin, &recorder{}, &out)
	o.Settings = settings
	if err := InstallDarwin(o); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(read(t, filepath.Join(root, PlistPath(HelperLabel))), "<string>https://127.0.0.1:8099/healthz</string>") || !strings.Contains(out.String(), "warning: "+EnvPath+note) {
		t.Errorf("macOS: the helper's health URL not https, or no note:\n%s", out.String())
	}
}

// A settings file left as the template, saved with a UTF-8 byte order
// mark, is the template still, as serve skips the mark: the first start
// stays the owner's.
func TestTemplateWithByteOrderMark(t *testing.T) {
	if !onlyTemplate([]byte(utf8BOM+envTemplate()), []byte(envTemplate())) {
		t.Error("the template with a byte order mark is not the template")
	}
	root, bin := setup(t)
	os.MkdirAll(filepath.Join(root, EnvDir), 0o750)
	os.WriteFile(filepath.Join(root, EnvPath), []byte(utf8BOM+envTemplate()), 0o600)
	rec := &recorder{}
	var out bytes.Buffer
	if err := Install(Options{Root: root, Binary: bin, Out: &out, Run: rec.run}); err != nil {
		t.Fatal(err)
	}
	if rec.has("systemctl restart massimo.service") || !strings.Contains(out.String(), "not started") {
		t.Errorf("started on the template: %q\n%s", rec.cmds, out.String())
	}
}

// A dry run on a packaged box, whose settings file is still the
// template, checks the -settings file it would copy over it, not the
// template: no warning of a missing token, and that file's notes.
func TestInstallPackagedDryRunReadsSettings(t *testing.T) {
	root := packagedRoot(t)
	settings := filepath.Join(t.TempDir(), ".env")
	os.WriteFile(settings, []byte("MASSIMO_API_TOKEN=t\nMASSIMO_API_TOKEN=u\n"), 0o600)
	var out bytes.Buffer
	if err := Install(Options{Root: root, Settings: settings, DryRun: true, Out: &out, Run: (&recorder{}).run}); err != nil {
		t.Fatal(err)
	}
	o := out.String()
	if strings.Contains(o, "no MASSIMO_API_TOKEN") {
		t.Errorf("the template checked:\n%s", o)
	}
	if n := strings.Count(o, "warning: "+settings+": MASSIMO_API_TOKEN is given on lines 1 and 2; the first wins\n"); n != 1 {
		t.Errorf("the note said %d times:\n%s", n, o)
	}
	if read(t, filepath.Join(root, EnvPath)) != envTemplate() {
		t.Error("a dry run wrote the settings")
	}
}
