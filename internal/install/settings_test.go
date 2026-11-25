// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package install

import (
	"bufio"
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

// utf16File writes text as UTF-16 in the byte order order after the
// UTF-8 lines head, with the byte order mark where bom is set: as
// PowerShell 5.1's > writes a file (UTF-16LE), or its >> appends to a
// UTF-8 one. It returns the file's path.
func utf16File(t *testing.T, order binary.AppendByteOrder, head, text string, bom bool) string {
	t.Helper()
	units := utf16.Encode([]rune(text))
	if bom {
		units = append([]uint16{0xFEFF}, units...)
	}
	b := []byte(head)
	for _, u := range units {
		b = order.AppendUint16(b, u)
	}
	p := filepath.Join(t.TempDir(), "settings.env")
	if err := os.WriteFile(p, b, 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

// A refusedFile is a settings file serve's start cannot read, with the
// parser's error for it: the file's path, at the line where one is
// named (at, ":2"), then text.
type refusedFile struct {
	what, path, at, text string
	is                   error // what the error wraps, nil for nothing
}

// refusal is the error install -settings refuses f with.
func (f refusedFile) refusal() string {
	return "-settings: " + f.path + f.at + ": " + f.text + "; nothing was written"
}

// refusedFiles writes a settings file of each kind serve's start
// cannot read (env.ReadFile): a line that is not KEY=VALUE, a line too
// long to read, UTF-16 text in both byte orders, whole as PowerShell
// 5.1's > writes it, with a byte order mark or without one, or appended
// to UTF-8 lines as its >> does, and a directory, which opens but does
// not read: its error is the system's (*fs.PathError), which only a
// missing file's may pass.
func refusedFiles(t *testing.T) []refusedFile {
	t.Helper()
	dir := t.TempDir()
	d, err := os.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	_, dirErr := d.Read(make([]byte, 1)) // "read <dir>: is a directory", on Windows "…: Incorrect function."
	d.Close()
	if dirErr == nil {
		t.Fatal("a directory read")
	}
	write := func(text string) string {
		p := filepath.Join(t.TempDir(), "settings.env")
		if err := os.WriteFile(p, []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
		return p
	}
	const utf16Text, utf16Err = "# Settings\r\nMASSIMO_API_TOKEN=t\r\n", "UTF-16 text, save it as UTF-8"
	return []refusedFile{
		{"a line that is not KEY=VALUE", write("MASSIMO_API_TOKEN=t\nnot a setting\n"), ":2", "expected KEY=VALUE", nil},
		{"a line too long to read", write("MASSIMO_API_TOKEN=t\nMASSIMO_X=" + strings.Repeat("x", bufio.MaxScanTokenSize) + "\n"), ":2", "bufio.Scanner: token too long", bufio.ErrTooLong},
		{"UTF-16LE", utf16File(t, binary.LittleEndian, "", utf16Text, true), "", utf16Err, env.ErrUTF16},
		{"UTF-16BE", utf16File(t, binary.BigEndian, "", utf16Text, true), "", utf16Err, env.ErrUTF16},
		{"UTF-16BE without a byte order mark", utf16File(t, binary.BigEndian, "", utf16Text, false), "", utf16Err, env.ErrUTF16},
		{"UTF-16LE after UTF-8 lines", utf16File(t, binary.LittleEndian, "# head\r\nMASSIMO_API_TOKEN=t\r\n", "MASSIMO_LISTEN=:80\r\n", false), ":3", utf16Err, env.ErrUTF16},
		{"a directory", dir, ":1", dirErr.Error(), nil},
	}
}

// refusedAs reports whether err is install's refusal of f.
func refusedAs(err error, f refusedFile) bool {
	return err != nil && err.Error() == f.refusal() && (f.is == nil || errors.Is(err, f.is))
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

// A -settings file serve's start cannot read, for a line that is not
// KEY=VALUE, a line too long to read, UTF-16 text in either byte order,
// or as a directory, is refused with the parser's error, its file and
// line, and the word that nothing was written, before the install
// writes, runs or says anything: in a dry run too, by a hand install, by
// one on a packaged box and by one on macOS, also where the settings
// file there is kept and it would not be copied. No binary, no user, no
// directory, no unit, no copy.
func TestInstallRefusesSettingsServeCannotRead(t *testing.T) {
	for _, k := range []struct {
		what             string
		packaged, darwin bool
	}{{"a hand install", false, false}, {"a packaged box", true, false}, {"macOS", false, true}} {
		for _, f := range refusedFiles(t) {
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
				what := fmt.Sprintf("%s, %s, dry run %v, kept %v", k.what, f.what, c.dry, c.kept)
				if !refusedAs(err, f) {
					t.Errorf("%s: %v, want %s", what, err, f.refusal())
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

// A -settings file that is not there is no refusal of serve's start,
// and install takes it as before: refused at the copy, os.ErrNotExist,
// where there is no settings file yet, and not copied where one is kept.
func TestInstallSettingsNotThere(t *testing.T) {
	none := filepath.Join(t.TempDir(), "none.env")
	root, bin := setup(t)
	if err := Install(Options{Root: root, Binary: bin, Settings: none, Out: &bytes.Buffer{}, Run: (&recorder{}).run}); !errors.Is(err, os.ErrNotExist) || strings.Contains(err.Error(), "nothing was written") {
		t.Errorf("no settings file yet: %v", err)
	}
	root, bin = setup(t)
	os.MkdirAll(filepath.Join(root, EnvDir), 0o750)
	os.WriteFile(filepath.Join(root, EnvPath), []byte("MASSIMO_API_TOKEN=mine\n"), 0o600)
	var out bytes.Buffer
	if err := Install(Options{Root: root, Binary: bin, Settings: none, Out: &out, Run: (&recorder{}).run}); err != nil || !strings.Contains(out.String(), EnvPath+" exists; kept it and did not copy "+none+"\n") {
		t.Errorf("a kept settings file: %v\n%s", err, out.String())
	}
}

// A settings file a re-install keeps, which serve refuses for one line
// that is not KEY=VALUE, is read leniently by install's checks
// (env.ReadFileLenient), so that an upgrade does not fail on it: the
// install goes through and starts the service, the line is a warning,
// and the other lines' settings count, so that the health drop-in, the
// capability for MASSIMO_TLS_HTTP's port and the macOS helper's health
// URL are https, not plain http. So without -settings, and with one
// that is not copied over the kept file, by a hand install, by one on a
// packaged box and by one on macOS.
func TestInstallKeepsAFileServeRefuses(t *testing.T) {
	text := "MASSIMO_API_TOKEN=t\nMASSIMO_TLS=acme\nMASSIMO_TLS_NAMES=massimo.example.net\nnot a setting\nMASSIMO_TLS_HTTP=:80\n"
	good := filepath.Join(t.TempDir(), ".env")
	os.WriteFile(good, []byte("MASSIMO_API_TOKEN=u\n"), 0o600)
	const note = "warning: " + EnvPath + ": line 4 is not KEY=VALUE: serve refuses the file until it is fixed\n"
	for _, k := range []struct {
		what             string
		packaged, darwin bool
	}{{"a hand install", false, false}, {"a packaged box", true, false}, {"macOS", false, true}} {
		for _, settings := range []string{"", good} {
			root, bin := setup(t)
			if k.packaged {
				root = packagedRoot(t)
			}
			os.MkdirAll(filepath.Join(root, EnvDir), 0o750)
			os.WriteFile(filepath.Join(root, EnvPath), []byte(text), 0o600)
			var out bytes.Buffer
			rec := &recorder{}
			o, install := Options{Root: root, Binary: bin, Out: &out, Run: rec.run}, Install
			if k.darwin {
				o, install = darwinOpts(root, bin, rec, &out), InstallDarwin
			}
			o.Settings = settings
			what := fmt.Sprintf("%s, -settings %q", k.what, settings)
			if err := install(o); err != nil {
				t.Errorf("%s: %v", what, err)
				continue
			}
			s := out.String()
			if n := strings.Count(s, note); n != 1 || strings.Contains(s, "cannot check") || strings.Contains(s, "no MASSIMO_API_TOKEN") {
				t.Errorf("%s: the warning said %d times, or the file not read:\n%s", what, n, s)
			}
			if read(t, filepath.Join(root, EnvPath)) != text {
				t.Errorf("%s: the kept file changed", what)
			}
			if k.darwin {
				if !strings.Contains(read(t, filepath.Join(root, PlistPath(HelperLabel))), "<string>https://127.0.0.1:8099/healthz</string>") || !strings.Contains(s, "start "+Label) {
					t.Errorf("%s: the helper's health URL not https, or not started:\n%s", what, s)
				}
				continue
			}
			if !rec.has("systemctl restart massimo.service") {
				t.Errorf("%s: not started: %q", what, rec.cmds)
			}
			if b, _ := os.ReadFile(filepath.Join(root, HealthDropIn)); !strings.Contains(string(b), "MASSIMO_HEALTH=https://") {
				t.Errorf("%s: health drop-in: %q", what, b)
			}
			if b, _ := os.ReadFile(filepath.Join(root, ListenDropIn)); !strings.Contains(string(b), "AmbientCapabilities=CAP_NET_BIND_SERVICE") {
				t.Errorf("%s: listen drop-in: %q", what, b)
			}
		}
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
