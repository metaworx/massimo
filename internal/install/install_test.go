// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package install

import (
	"bytes"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"go.metaworx.ch/massimo/internal/env"
)

type recorder struct{ cmds []string }

func (r *recorder) run(name string, args ...string) error {
	r.cmds = append(r.cmds, name+" "+strings.Join(args, " "))
	return nil
}

func (r *recorder) has(prefix string) bool {
	for _, c := range r.cmds {
		if strings.HasPrefix(c, prefix) {
			return true
		}
	}
	return false
}

func setup(t *testing.T) (root, bin string) {
	root = t.TempDir()
	os.MkdirAll(filepath.Join(root, LogrotateDir), 0o755)
	bin = filepath.Join(t.TempDir(), "massimo-linux-armv6")
	os.WriteFile(bin, []byte("binary v1"), 0o755)
	return root, bin
}

func read(t *testing.T, p string) string {
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestInstallFresh(t *testing.T) {
	root, bin := setup(t)
	rec := &recorder{}
	var out bytes.Buffer
	if err := Install(Options{Root: root, Binary: bin, Out: &out, Run: rec.run}); err != nil {
		t.Fatal(err)
	}
	if read(t, filepath.Join(root, BinPath)) != "binary v1" {
		t.Fatal("binary")
	}
	env := filepath.Join(root, EnvPath)
	if !strings.Contains(read(t, env), "MASSIMO_CUBE_<SN>_ADDRESS") {
		t.Fatal("env template")
	}
	if runtime.GOOS != "windows" { // no Unix modes there
		if fi, _ := os.Stat(filepath.Join(root, BinPath)); fi.Mode().Perm() != 0o755 {
			t.Fatalf("binary mode %v", fi.Mode())
		}
		if fi, _ := os.Stat(env); fi.Mode().Perm() != 0o600 {
			t.Fatalf("env mode %v", fi.Mode())
		}
	}
	unit := read(t, filepath.Join(root, UnitDir, "massimo.service"))
	if !strings.Contains(unit, updateFlag) {
		t.Fatal("massimo.service lacks the update request flag")
	}
	// The store lands in the state directory, whatever the working
	// directory.
	if !strings.Contains(unit, " -data /var/lib/massimo serve ") || !strings.Contains(unit, "StateDirectory=massimo") {
		t.Fatal("massimo.service does not pass -data /var/lib/massimo")
	}
	// systemd sets the state directory's mode at every start: 0755 unless
	// the unit says otherwise.
	if !strings.Contains(unit, "\nStateDirectoryMode=0750\n") {
		t.Fatal("massimo.service leaves /var/lib/massimo at systemd's 0755")
	}
	if !strings.Contains(unit, "\nUser=_massimo\nGroup=_massimo\n") {
		t.Fatal("massimo.service does not run as _massimo")
	}
	for _, f := range []string{UnitDir + "/massimo-update.path", UnitDir + "/massimo-update.service", LogrotateDir + "/massimo"} {
		if _, err := os.Stat(filepath.Join(root, f)); err != nil {
			t.Error(err)
		}
	}
	for _, c := range []string{"useradd --system", "chown -h _massimo:_massimo " + filepath.Join(root, EnvPath), "systemctl daemon-reload", "systemctl enable massimo.service", "systemctl enable --now massimo-update.path"} {
		if !rec.has(c) {
			t.Errorf("no %q in %q", c, rec.cmds)
		}
	}
	// Only the template: enabled, not started, and the way to the owner
	// said, in the state directory the install made for it.
	if rec.has("systemctl restart") || !strings.Contains(out.String(), "not started") {
		t.Fatalf("started with the template: %q\n%s", rec.cmds, out.String())
	}
	if !strings.Contains(out.String(), "create the owner first: sudo -u _massimo /usr/local/bin/massimo user add <name> -role owner\n") || !strings.Contains(out.String(), "then start it: sudo systemctl start massimo\n") {
		t.Errorf("the message:\n%s", out.String())
	}
	if !rec.has("chown _massimo:_massimo " + filepath.Join(root, StateDir)) {
		t.Errorf("no state directory: %q", rec.cmds)
	}
	// No group adm here: the log directory is the service's group's.
	if !rec.has("chown _massimo:_massimo "+filepath.Join(root, LogDir)) || !rec.has("chmod 2750 "+filepath.Join(root, LogDir)) {
		t.Errorf("no log directory: %q", rec.cmds)
	}
	if strings.Contains(unit, "\nLogsDirectory=") || !strings.Contains(unit, "\nReadWritePaths=-/var/log/massimo\n") {
		t.Error("massimo.service gives the log directory back to the service's group, or cannot write it")
	}
}

// The log directory is the admin group's where there is one, set-group-id,
// and an older release's logs there are narrowed to 0640 in that group;
// a link among them is left alone.
func TestInstallLogDirAdmin(t *testing.T) {
	root, bin := setup(t)
	dir := filepath.Join(root, LogDir)
	os.MkdirAll(dir, 0o755)
	os.WriteFile(filepath.Join(dir, "serve.log"), nil, 0o644)
	os.WriteFile(filepath.Join(dir, "serve.log.2.gz"), nil, 0o644)
	os.Symlink("/etc/passwd", filepath.Join(dir, "serve.log.9"))
	rec := &recorder{}
	adm := func(name string, args ...string) ([]byte, error) {
		if name == "getent" && len(args) == 2 && args[0] == "group" && args[1] == "adm" {
			return []byte("adm:x:4:\n"), nil
		}
		return nil, nil
	}
	if err := Install(Options{Root: root, Binary: bin, Out: io.Discard, Run: rec.run, Output: adm}); err != nil {
		t.Fatal(err)
	}
	for _, c := range []string{
		"chown _massimo:adm " + dir, "chmod 2750 " + dir,
		"chown -h _massimo:adm " + filepath.Join(dir, "serve.log"), "chmod 0640 " + filepath.Join(dir, "serve.log"),
		"chown -h _massimo:adm " + filepath.Join(dir, "serve.log.2.gz"), "chmod 0640 " + filepath.Join(dir, "serve.log.2.gz"),
	} {
		if !rec.has(c) {
			t.Errorf("no %q in %q", c, rec.cmds)
		}
	}
	if rec.has("chmod 0640 " + filepath.Join(dir, "serve.log.9")) {
		t.Errorf("the link was followed: %q", rec.cmds)
	}
}

// isActive answers systemctl is-active with state.
func isActive(state string) func(string, ...string) ([]byte, error) {
	return func(name string, args ...string) ([]byte, error) {
		if name == "systemctl" && len(args) > 0 && args[0] == "is-active" {
			return []byte(state + "\n"), nil
		}
		return nil, nil
	}
}

func TestOnlyTemplate(t *testing.T) {
	tmpl := []byte(envTemplate())
	for _, c := range []struct {
		name, cur string
		want      bool
	}{
		{"the template", envTemplate(), true},
		{"an older template's comments", "# an older comment\n\nMASSIMO_LOG=/var/log/massimo/serve.log\n\n", true},
		{"CRLF and blanks around", strings.ReplaceAll(envTemplate(), "\n", "  \r\n"), true},
		{"a setting added", envTemplate() + "MASSIMO_API_TOKEN=x\n", false},
		{"a comment uncommented", strings.Replace(envTemplate(), "# MASSIMO_ANNOUNCE=off", "MASSIMO_ANNOUNCE=off", 1), false},
		{"the template's setting taken out", "# nothing here\n", false},
	} {
		if got := onlyTemplate([]byte(c.cur), tmpl); got != c.want {
			t.Errorf("%s: %v, want %v", c.name, got, c.want)
		}
	}
}

// Installing again: the first start stays the owner's while the settings
// are only the template's, and whatever runs is restarted.
func TestInstallStartRule(t *testing.T) {
	for _, c := range []struct {
		name, env, state string
		started          bool
	}{
		{"the template, stopped", envTemplate(), "inactive", false},
		{"the template, failed", envTemplate(), "failed", false},
		{"the template, running", envTemplate(), "active", true},
		{"the template, in its restart pause", envTemplate(), "activating", true},
		{"an older template, stopped", "# an older comment\nMASSIMO_LOG=/var/log/massimo/serve.log\n", "inactive", false},
		{"a setting of its own, stopped", envTemplate() + "MASSIMO_ANNOUNCE=off\n", "inactive", true},
	} {
		t.Run(c.name, func(t *testing.T) {
			root, bin := setup(t)
			os.MkdirAll(filepath.Join(root, EnvDir), 0o750)
			os.WriteFile(filepath.Join(root, EnvPath), []byte(c.env), 0o600)
			rec := &recorder{}
			var out bytes.Buffer
			if err := Install(Options{Root: root, Binary: bin, Out: &out, Run: rec.run, Output: isActive(c.state)}); err != nil {
				t.Fatal(err)
			}
			if got := rec.has("systemctl restart massimo.service"); got != c.started {
				t.Errorf("restarted: %v, want %v; %q", got, c.started, rec.cmds)
			}
			if strings.Contains(out.String(), "not started") == c.started {
				t.Errorf("the message:\n%s", out.String())
			}
		})
	}
}

func TestInstallAgainKeepsSettingsAndEdits(t *testing.T) {
	root, bin := setup(t)
	if err := Install(Options{Root: root, Binary: bin, Out: &bytes.Buffer{}, Run: (&recorder{}).run}); err != nil {
		t.Fatal(err)
	}
	env := filepath.Join(root, EnvPath)
	os.WriteFile(env, []byte("MASSIMO_CUBE_KEQ0000001_ADDRESS=10.0.0.1\n"), 0o600)
	unit := filepath.Join(root, UnitDir, "massimo.service")
	os.WriteFile(unit, []byte("edited by hand\n"), 0o644)
	os.WriteFile(bin, []byte("binary v2"), 0o755)
	settings := filepath.Join(t.TempDir(), ".env")
	os.WriteFile(settings, []byte("OTHER=1\n"), 0o600)

	rec := &recorder{}
	var out bytes.Buffer
	if err := Install(Options{Root: root, Binary: bin, Settings: settings, Out: &out, Run: rec.run}); err != nil {
		t.Fatal(err)
	}
	if read(t, filepath.Join(root, BinPath)) != "binary v2" {
		t.Fatal("binary not updated")
	}
	if !strings.Contains(read(t, env), "10.0.0.1") {
		t.Fatal("existing settings overwritten")
	}
	if read(t, unit+".bak") != "edited by hand\n" || !strings.Contains(read(t, unit), "ExecStart=") {
		t.Fatal("edited unit not kept as .bak")
	}
	if !rec.has("systemctl restart massimo.service") {
		t.Fatalf("not restarted: %q", rec.cmds)
	}
	if !strings.Contains(out.String(), "massimo-update.path is current") {
		t.Fatalf("unchanged unit not reported:\n%s", out.String())
	}
}

func TestInstallWithoutHelperAndWithSettings(t *testing.T) {
	root, bin := setup(t)
	settings := filepath.Join(t.TempDir(), ".env")
	os.WriteFile(settings, []byte("MASSIMO_CUBE_KEQ0000001_ADDRESS=10.0.0.1\n"), 0o600)
	rec := &recorder{}
	if err := Install(Options{Root: root, Binary: bin, Settings: settings, NoUpdateHelper: true, Out: &bytes.Buffer{}, Run: rec.run}); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(read(t, filepath.Join(root, UnitDir, "massimo.service")), updateFlag) {
		t.Fatal("update flag left in without the helper")
	}
	if _, err := os.Stat(filepath.Join(root, UnitDir, "massimo-update.path")); err == nil {
		t.Fatal("helper installed")
	}
	if !strings.Contains(read(t, filepath.Join(root, EnvPath)), "10.0.0.1") || !rec.has("systemctl restart massimo.service") || rec.has("systemctl enable --now massimo-update.path") {
		t.Fatalf("settings or start: %q", rec.cmds)
	}
}

func TestDryRunChangesNothing(t *testing.T) {
	root, bin := setup(t)
	rec := &recorder{}
	var out bytes.Buffer
	if err := Install(Options{Root: root, Binary: bin, DryRun: true, Out: &out, Run: rec.run}); err != nil {
		t.Fatal(err)
	}
	if len(rec.cmds) != 0 {
		t.Fatalf("ran %q", rec.cmds)
	}
	for _, p := range []string{BinPath, EnvPath, UnitDir + "/massimo.service"} {
		if _, err := os.Stat(filepath.Join(root, p)); err == nil {
			t.Errorf("%s written", p)
		}
	}
	if !strings.Contains(out.String(), "would write "+UnitDir+"/massimo.service") {
		t.Fatal(out.String())
	}
}

func TestRemove(t *testing.T) {
	root, bin := setup(t)
	Install(Options{Root: root, Binary: bin, Out: &bytes.Buffer{}, Run: (&recorder{}).run})
	rec := &recorder{}
	if err := Remove(Options{Root: root, Out: &bytes.Buffer{}, Run: rec.run}, false); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{BinPath, UnitDir + "/massimo.service", UnitDir + "/massimo-update.path", LogrotateDir + "/massimo"} {
		if _, err := os.Stat(filepath.Join(root, p)); err == nil {
			t.Errorf("%s still there", p)
		}
	}
	if _, err := os.Stat(filepath.Join(root, EnvPath)); err != nil {
		t.Fatal("settings removed without -purge")
	}
	if !rec.has("systemctl disable --now massimo-update.path massimo-reboot.path massimo.service") || rec.has("userdel") {
		t.Fatalf("%q", rec.cmds)
	}
	rec = &recorder{}
	sys := system{passwd: map[string]string{User: ownEntry(User)}}
	if err := Remove(Options{Root: root, Out: &bytes.Buffer{}, Run: rec.run, Output: sys.output}, true); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, EnvDir)); err == nil || !rec.has("userdel "+User) {
		t.Fatalf("purge: %v %q", err, rec.cmds)
	}
}

// system answers the read-only questions install asks the system:
// systemctl is-active with active, getent passwd and group from the
// entries; an account or group not listed is not there.
type system struct {
	active string
	passwd map[string]string // the name's passwd line
	groups []string
	owner  string // what stat -c %U says of any file; "" fails
}

func (s system) output(name string, args ...string) ([]byte, error) {
	switch {
	case name == "systemctl" && len(args) > 0 && args[0] == "is-active":
		return []byte(s.active + "\n"), nil
	case name == "getent" && len(args) == 2 && args[0] == "passwd":
		if e, ok := s.passwd[args[1]]; ok {
			return []byte(e + "\n"), nil
		}
		return nil, errors.New("exit status 2")
	case name == "stat" && len(args) == 3 && args[0] == "-c" && args[1] == "%U":
		if s.owner == "" {
			return nil, errors.New("exit status 1")
		}
		return []byte(s.owner + "\n"), nil
	case name == "getent" && len(args) == 2 && args[0] == "group":
		if slices.Contains(s.groups, args[1]) {
			return []byte(args[1] + ":x:101:\n"), nil
		}
		return nil, errors.New("exit status 2")
	}
	return nil, nil
}

// ownEntry is the passwd line of an account an install or the package
// made: its home is the state directory. personEntry is a person's.
func ownEntry(name string) string { return name + ":x:100:101::/var/lib/massimo:/usr/sbin/nologin" }

const personEntry = "massimo:x:1000:1000:Massimo Rossi,,,:/home/massimo:/bin/bash"

func TestServiceAccount(t *testing.T) {
	for _, c := range []struct {
		name      string
		sys       system
		want, not []string
	}{
		{"none", system{}, []string{"useradd --system --user-group --home-dir /var/lib/massimo --no-create-home --shell"}, []string{"usermod", "groupmod"}},
		{"there", system{passwd: map[string]string{User: ownEntry(User)}}, nil, []string{"useradd", "usermod", "groupmod"}},
		{"an older install's", system{passwd: map[string]string{formerUser: ownEntry(formerUser)}, groups: []string{formerUser}},
			[]string{"usermod -l _massimo massimo", "groupmod -n _massimo massimo"}, []string{"useradd"}},
		{"an older install's, the group taken", system{passwd: map[string]string{formerUser: ownEntry(formerUser)}, groups: []string{formerUser, User}},
			[]string{"usermod -l _massimo massimo"}, []string{"useradd", "groupmod"}},
		{"an older install's, running", system{active: "active", passwd: map[string]string{formerUser: ownEntry(formerUser)}, groups: []string{formerUser}},
			[]string{"systemctl stop massimo.service", "usermod -l _massimo massimo", "systemctl restart massimo.service"}, []string{"useradd"}},
		{"a person's", system{passwd: map[string]string{formerUser: personEntry}, groups: []string{formerUser}},
			[]string{"useradd --system --user-group --home-dir /var/lib/massimo --no-create-home --shell"}, []string{"usermod", "groupmod"}},
	} {
		t.Run(c.name, func(t *testing.T) {
			root, bin := setup(t)
			rec := &recorder{}
			var out bytes.Buffer
			// A stop stops: systemctl answers inactive after it.
			sys := c.sys
			run := func(name string, args ...string) error {
				if name == "systemctl" && len(args) > 0 && args[0] == "stop" {
					sys.active = "inactive"
				}
				return rec.run(name, args...)
			}
			output := func(name string, args ...string) ([]byte, error) { return sys.output(name, args...) }
			if err := Install(Options{Root: root, Binary: bin, Out: &out, Run: run, Output: output}); err != nil {
				t.Fatal(err)
			}
			for _, w := range c.want {
				if !rec.has(w) {
					t.Errorf("no %q in %q", w, rec.cmds)
				}
			}
			for _, n := range c.not {
				if rec.has(n) {
					t.Errorf("%q in %q", n, rec.cmds)
				}
			}
			if rec.has("useradd") && !strings.HasSuffix(rec.cmds[0], " "+User) {
				t.Errorf("useradd makes %q", rec.cmds[0])
			}
			// usermod refuses an account a process runs as: the stop
			// comes first.
			if i, j := slices.IndexFunc(rec.cmds, func(c string) bool { return strings.HasPrefix(c, "systemctl stop") }),
				slices.IndexFunc(rec.cmds, func(c string) bool { return strings.HasPrefix(c, "usermod") }); i > j {
				t.Errorf("stopped after the rename: %q", rec.cmds)
			}
		})
	}
}

func TestRemoveAccounts(t *testing.T) {
	for _, c := range []struct {
		name      string
		passwd    map[string]string
		want, not []string
		say       string
	}{
		{"a person's massimo stays", map[string]string{User: ownEntry(User), formerUser: personEntry},
			[]string{"userdel _massimo"}, []string{"userdel massimo"}, "kept the user massimo, which is not the service's"},
		{"an older install's, never renamed", map[string]string{formerUser: ownEntry(formerUser)},
			[]string{"userdel massimo"}, []string{"userdel _massimo"}, "remove the user massimo"},
	} {
		t.Run(c.name, func(t *testing.T) {
			root, bin := setup(t)
			Install(Options{Root: root, Binary: bin, Out: &bytes.Buffer{}, Run: (&recorder{}).run})
			rec := &recorder{}
			var out bytes.Buffer
			if err := Remove(Options{Root: root, Out: &out, Run: rec.run, Output: system{passwd: c.passwd}.output}, true); err != nil {
				t.Fatal(err)
			}
			for _, w := range c.want {
				if !rec.has(w) {
					t.Errorf("no %q in %q", w, rec.cmds)
				}
			}
			for _, n := range c.not {
				if rec.has(n) {
					t.Errorf("%q in %q", n, rec.cmds)
				}
			}
			if !strings.Contains(out.String(), c.say) {
				t.Errorf("no %q in %s", c.say, out.String())
			}
		})
	}
}

func TestSettingsWarnings(t *testing.T) {
	root, bin := setup(t)
	settings := filepath.Join(t.TempDir(), ".env")
	os.WriteFile(settings, []byte(`MASSIMO_CUBE_JEQ0000001_ADDRESS=192.168.1.118
MASSIMO_CUBE_JEQ0000001_PROXY=192.168.1.50
MASSIMO_CUBE_JEQ0000001_PROXY_SERIAL=JEQ0000009
export MASSIMO_CUBE_KEQ0000001_PROXY="192.168.1.49:62910"
# MASSIMO_CUBE_KEQ0000002_PROXY=192.168.1.51
MASSIMO_CUBE_KEQ0000003_PROXY=:62910
`), 0o600)
	var out bytes.Buffer
	local := func() ([]net.IP, error) {
		return []net.IP{net.ParseIP("192.168.1.48"), net.ParseIP("192.168.1.50")}, nil
	}
	if err := Install(Options{Root: root, Binary: bin, Settings: settings, Out: &out, Run: (&recorder{}).run, LocalAddrs: local}); err != nil {
		t.Fatal(err)
	}
	o := out.String()
	if !strings.Contains(o, "no MASSIMO_API_TOKEN") {
		t.Errorf("no token warning:\n%s", o)
	}
	if !strings.Contains(o, "MASSIMO_CUBE_KEQ0000001_PROXY=192.168.1.49:62910 is not an address") {
		t.Errorf("no warning for .49:\n%s", o)
	}
	for _, quiet := range []string{"192.168.1.50 is not", "KEQ0000002", "KEQ0000003", "PROXY_SERIAL"} {
		if strings.Contains(o, quiet) {
			t.Errorf("unexpected warning about %s:\n%s", quiet, o)
		}
	}
}

func TestListenDropIns(t *testing.T) {
	for _, c := range []struct {
		addr, listen, health string
		tls, cap             bool
		http                 string
	}{
		{":80", "MASSIMO_LISTEN=:80", "http://127.0.0.1:80/healthz", false, true, ""},
		{"0.0.0.0:8080", "MASSIMO_LISTEN=0.0.0.0:8080", "http://127.0.0.1:8080/healthz", false, false, ""},
		{"192.168.1.200:80", "MASSIMO_LISTEN=192.168.1.200:80", "http://192.168.1.200:80/healthz", false, true, ""},
		{"[::]:443", "MASSIMO_LISTEN=[::]:443", "http://[::1]:443/healthz", false, true, ""},
		{"[::]:443", "MASSIMO_LISTEN=[::]:443", "https://[::1]:443/healthz", true, true, ""},
		{"192.168.1.200:8443", "MASSIMO_LISTEN=192.168.1.200:8443", "https://192.168.1.200:8443/healthz", true, false, ""},
		{":8099", "MASSIMO_LISTEN=:8099", "https://127.0.0.1:8099/healthz", true, true, ":80"},
		{":8099", "MASSIMO_LISTEN=:8099", "https://127.0.0.1:8099/healthz", true, false, ":8080"},
	} {
		serve, health, err := listenDropIns(c.addr, c.tls, c.http)
		if err != nil || !strings.Contains(serve, c.listen) || !strings.Contains(health, c.health) || strings.Contains(serve, "CAP_NET_BIND_SERVICE") != c.cap {
			t.Errorf("%s: %v\n%s\n%s", c.addr, err, serve, health)
		}
		if listenFromDropIn([]byte(serve)) != c.addr {
			t.Errorf("%s: read back %q", c.addr, listenFromDropIn([]byte(serve)))
		}
	}
	for _, bad := range []string{"80", ":http", ":0", ":70000", "host"} {
		if _, _, err := listenDropIns(bad, false, ""); err == nil {
			t.Errorf("%q accepted", bad)
		}
	}
}

func TestSetEnvLines(t *testing.T) {
	p := filepath.Join(t.TempDir(), "env")
	os.WriteFile(p, []byte("# settings\nexport MASSIMO_TLS=off\nMASSIMO_API_TOKEN=t\n"), 0o640)
	if err := setEnvLines(p, [][2]string{{"MASSIMO_TLS", "files"}, {"MASSIMO_TLS_CERT", "/etc/massimo/cert.pem"}}, nil); err != nil {
		t.Fatal(err)
	}
	if got := read(t, p); got != "# settings\nMASSIMO_TLS=files\nMASSIMO_API_TOKEN=t\nMASSIMO_TLS_CERT=/etc/massimo/cert.pem\n" {
		t.Fatalf("%q", got)
	}
	// A variable dropped goes, an "export " line of it too; a comment stays.
	os.WriteFile(p, []byte("# MASSIMO_TLS_HTTP=:80 was here\nexport MASSIMO_TLS_HTTP=:80\nMASSIMO_TLS=acme\n"), 0o640)
	if err := setEnvLines(p, [][2]string{{"MASSIMO_TLS", "off"}}, []string{"MASSIMO_TLS_HTTP", "MASSIMO_TLS_NAMES"}); err != nil {
		t.Fatal(err)
	}
	if got := read(t, p); got != "# MASSIMO_TLS_HTTP=:80 was here\nMASSIMO_TLS=off\n" {
		t.Fatalf("dropped: %q", got)
	}
	if fi, _ := os.Stat(p); runtime.GOOS != "windows" && fi.Mode().Perm() != 0o640 {
		t.Fatalf("mode %v", fi.Mode())
	}
	missing := filepath.Join(t.TempDir(), "new")
	if err := setEnvLines(missing, [][2]string{{"A", "1"}}, nil); err != nil || read(t, missing) != "A=1\n" {
		t.Fatalf("new file: %v %q", err, read(t, missing))
	}
}

func TestInstallTLSFiles(t *testing.T) {
	root, bin := setup(t)
	src := t.TempDir()
	cert, key := filepath.Join(src, "fullchain.pem"), filepath.Join(src, "privkey.pem")
	os.WriteFile(cert, []byte("CERT"), 0o644)
	os.WriteFile(key, []byte("KEY"), 0o600)
	rec := &recorder{}
	var out bytes.Buffer
	if err := Install(Options{Root: root, Binary: bin, TLSCert: cert, TLSKey: key, Out: &out, Run: rec.run}); err != nil {
		t.Fatal(err)
	}
	certDst, keyDst := filepath.Join(root, EnvDir, certName), filepath.Join(root, EnvDir, keyName)
	if read(t, certDst) != "CERT" || read(t, keyDst) != "KEY" {
		t.Fatal("files not copied")
	}
	if fi, _ := os.Stat(keyDst); runtime.GOOS != "windows" && fi.Mode().Perm() != 0o600 {
		t.Fatalf("key mode %v", fi.Mode())
	}
	if !rec.has("chown -h _massimo:_massimo "+certDst) || !rec.has("chown -h _massimo:_massimo "+filepath.Join(root, EnvPath)) {
		t.Fatalf("not owned by the service user: %q", rec.cmds)
	}
	env := read(t, filepath.Join(root, EnvPath))
	for _, want := range []string{"MASSIMO_TLS=files\n", "MASSIMO_TLS_CERT=/etc/massimo/cert.pem\n", "MASSIMO_TLS_KEY=/etc/massimo/key.pem\n"} {
		if strings.Count(env, want) != 1 {
			t.Errorf("settings lack %q:\n%s", want, env)
		}
	}
	if !strings.HasPrefix(env, "# Settings of massimo serve") {
		t.Error("template lost")
	}
	if !strings.Contains(read(t, filepath.Join(root, HealthDropIn)), "MASSIMO_HEALTH=https://127.0.0.1:8099/healthz") {
		t.Fatal("the helper does not check https")
	}
	if _, err := os.Stat(filepath.Join(root, ListenDropIn)); err == nil {
		t.Fatal("a listen drop-in without -listen")
	}

	// A later install with -listen keeps TLS from the settings.
	out.Reset()
	if err := Install(Options{Root: root, Binary: bin, Listen: ":443", Out: &out, Run: rec.run}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(read(t, filepath.Join(root, ListenDropIn)), "CAP_NET_BIND_SERVICE") ||
		!strings.Contains(read(t, filepath.Join(root, HealthDropIn)), "https://127.0.0.1:443/healthz") {
		t.Fatalf("drop-ins after -listen:\n%s", out.String())
	}
	if read(t, certDst) != "CERT" || strings.Count(read(t, filepath.Join(root, EnvPath)), "MASSIMO_TLS=files\n") != 1 {
		t.Fatal("TLS not kept")
	}

	// -tls off: the settings say so, the helper goes back to http.
	if err := Install(Options{Root: root, Binary: bin, TLS: "off", Out: &out, Run: rec.run}); err != nil {
		t.Fatal(err)
	}
	env = read(t, filepath.Join(root, EnvPath))
	if strings.Count(env, "MASSIMO_TLS=off\n") != 1 || strings.Contains(env, "MASSIMO_TLS=files") || strings.Contains(env, "MASSIMO_TLS_CERT=") {
		t.Fatalf("settings after -tls off:\n%s", env)
	}
	if !strings.Contains(read(t, filepath.Join(root, HealthDropIn)), "http://127.0.0.1:443/healthz") {
		t.Fatal("the helper still checks https")
	}

	// -tls self-signed: the settings say so, the server makes the pair.
	if err := Install(Options{Root: root, Binary: bin, TLS: "self-signed", TLSNames: "massimo.example.net", Out: &out, Run: rec.run}); err != nil {
		t.Fatal(err)
	}
	env = read(t, filepath.Join(root, EnvPath))
	if strings.Count(env, "MASSIMO_TLS=self-signed\n") != 1 || !strings.Contains(env, "MASSIMO_TLS_NAMES=massimo.example.net\n") {
		t.Fatalf("settings after -tls self-signed:\n%s", env)
	}
	if !strings.Contains(read(t, filepath.Join(root, HealthDropIn)), "https://127.0.0.1:443/healthz") {
		t.Fatal("the helper does not check https for self-signed")
	}

	// -tls acme with its names, the contact and the port-80 helper, which
	// needs the capability although the listener does not.
	staging := "https://acme-staging-v02.api.letsencrypt.org/directory"
	if err := Install(Options{Root: root, Binary: bin, TLS: "acme", TLSNames: "massimo.example.net", ACMEEmail: "owner@example.net", ACMEDirectory: staging, TLSHTTP: ":80", Out: &out, Run: rec.run}); err != nil {
		t.Fatal(err)
	}
	env = read(t, filepath.Join(root, EnvPath))
	for _, want := range []string{"MASSIMO_TLS=acme\n", "MASSIMO_TLS_NAMES=massimo.example.net\n", "MASSIMO_ACME_EMAIL=owner@example.net\n", "MASSIMO_ACME_DIRECTORY=" + staging + "\n", "MASSIMO_TLS_HTTP=:80\n"} {
		if strings.Count(env, want) != 1 {
			t.Errorf("settings after -tls acme lack %q:\n%s", want, env)
		}
	}
	if l := read(t, filepath.Join(root, ListenDropIn)); !strings.Contains(l, "MASSIMO_LISTEN=:443") || !strings.Contains(l, "CAP_NET_BIND_SERVICE") {
		t.Fatalf("listen drop-in after -tls acme:\n%s", l)
	}
	// A later install keeps the helper address and the capability.
	if err := Install(Options{Root: root, Binary: bin, Out: &out, Run: rec.run}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(read(t, filepath.Join(root, ListenDropIn)), "CAP_NET_BIND_SERVICE") {
		t.Fatal("capability lost")
	}
	if !strings.Contains(read(t, filepath.Join(root, EnvPath)), "MASSIMO_ACME_DIRECTORY="+staging+"\n") {
		t.Fatal("the ACME directory lost by an install without -tls")
	}

	// -tls acme with a DNS-01 hook: the variable set, the hook checked.
	hook := filepath.Join(src, "hook.sh")
	os.WriteFile(hook, []byte("#!/bin/sh\n"), 0o755)
	if err := Install(Options{Root: root, Binary: bin, TLS: "acme", TLSNames: "*.example.net,example.net", ACMEDNSHook: hook, Out: &out, Run: rec.run}); err != nil {
		t.Fatal(err)
	}
	if env = read(t, filepath.Join(root, EnvPath)); strings.Count(env, "MASSIMO_ACME_DNS_HOOK="+hook+"\n") != 1 || !strings.Contains(env, "MASSIMO_TLS_NAMES=*.example.net,example.net\n") {
		t.Fatalf("settings after -acme-dns-hook:\n%s", env)
	}
	noExec := filepath.Join(src, "noexec.sh")
	os.WriteFile(noExec, []byte("#!/bin/sh\n"), 0o644)

	// Refused: a missing file, -tls off with files, acme without names, a
	// hook that is not there, a hook with the port-80 helper, a directory
	// that is no http or https URL.
	for _, o := range []Options{{TLSCert: filepath.Join(src, "none.pem"), TLSKey: key}, {TLS: "off", TLSCert: cert, TLSKey: key}, {TLS: "files"}, {TLS: "self-signed", TLSCert: cert, TLSKey: key}, {TLS: "acme"}, {TLS: "acme", TLSNames: "a.example.net", TLSHTTP: "80"},
		{TLS: "acme", TLSNames: "a.example.net", ACMEDNSHook: filepath.Join(src, "none.sh")}, {TLS: "acme", TLSNames: "a.example.net", ACMEDNSHook: hook, TLSHTTP: ":80"}, {TLS: "yes"},
		{TLS: "acme", TLSNames: "a.example.net", ACMEDirectory: "staging"}, {TLS: "acme", TLSNames: "a.example.net", ACMEDirectory: "ftp://acme.example.net/directory"}, {TLS: "acme", TLSNames: "a.example.net", ACMEDirectory: "https://"}} {
		o.Root, o.Binary, o.Out, o.Run = root, bin, &bytes.Buffer{}, rec.run
		if err := Install(o); err == nil {
			t.Errorf("%+v accepted", o)
		}
	}
	if runtime.GOOS != "windows" {
		if err := Install(Options{Root: root, Binary: bin, TLS: "acme", TLSNames: "a.example.net", ACMEDNSHook: noExec, Out: &bytes.Buffer{}, Run: rec.run}); err == nil {
			t.Error("a hook that is not executable accepted")
		}
	}

	// A dry run says what it would do and writes nothing.
	root2, bin2 := setup(t)
	out.Reset()
	if err := Install(Options{Root: root2, Binary: bin2, TLSCert: cert, TLSKey: key, DryRun: true, Out: &out, Run: rec.run}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "would copy "+cert) || !strings.Contains(out.String(), "would set MASSIMO_TLS=files") {
		t.Fatalf("dry run:\n%s", out.String())
	}
	if _, err := os.Stat(filepath.Join(root2, EnvDir, certName)); err == nil {
		t.Fatal("dry run copied")
	}
}

func TestTLSSettingsWarnings(t *testing.T) {
	root, bin := setup(t)
	settings := filepath.Join(t.TempDir(), ".env")
	os.WriteFile(settings, []byte("MASSIMO_API_TOKEN=t\nMASSIMO_TLS=files\nMASSIMO_TLS_CERT=/etc/massimo/cert.pem\n"), 0o600)
	var out bytes.Buffer
	if err := Install(Options{Root: root, Binary: bin, Settings: settings, Out: &out, Run: (&recorder{}).run}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "warning: MASSIMO_TLS_CERT=/etc/massimo/cert.pem") || !strings.Contains(out.String(), "warning: no MASSIMO_TLS_KEY") {
		t.Fatalf("no warning about the TLS files:\n%s", out.String())
	}
	if !strings.Contains(read(t, filepath.Join(root, HealthDropIn)), "https://") {
		t.Fatal("the settings' TLS not seen by the helper")
	}
}

func TestInstallListenKeptAcrossInstalls(t *testing.T) {
	root, bin := setup(t)
	run := (&recorder{}).run
	if err := Install(Options{Root: root, Binary: bin, Listen: ":80", Out: &bytes.Buffer{}, Run: run}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(read(t, filepath.Join(root, ListenDropIn)), "MASSIMO_LISTEN=:80") ||
		!strings.Contains(read(t, filepath.Join(root, HealthDropIn)), "127.0.0.1:80/healthz") {
		t.Fatal("drop-ins")
	}
	unit := read(t, filepath.Join(root, UnitDir, "massimo.service"))
	if !strings.Contains(unit, "-listen ${MASSIMO_LISTEN}") || !strings.Contains(unit, "Environment=MASSIMO_LISTEN=:8099") {
		t.Fatal("massimo.service does not take the address from MASSIMO_LISTEN")
	}
	if !strings.Contains(read(t, filepath.Join(root, UnitDir, "massimo-update.service")), "-health ${MASSIMO_HEALTH}") {
		t.Fatal("massimo-update.service does not take the URL from MASSIMO_HEALTH")
	}
	// A later install without -listen keeps them.
	var out bytes.Buffer
	if err := Install(Options{Root: root, Binary: bin, Out: &out, Run: run}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(read(t, filepath.Join(root, ListenDropIn)), "MASSIMO_LISTEN=:80") || !strings.Contains(out.String(), "kept") {
		t.Fatalf("drop-in not kept:\n%s", out.String())
	}
	if err := Install(Options{Root: root, Binary: bin, Listen: "80", Out: &bytes.Buffer{}, Run: run}); err == nil {
		t.Fatal("bad -listen accepted")
	}
	if err := Remove(Options{Root: root, Out: &bytes.Buffer{}, Run: run}, false); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, filepath.Dir(ListenDropIn))); err == nil {
		t.Fatal("drop-in directory left behind")
	}
}

// A reinstall with -no-update-helper takes the helper out: its units and
// their drop-in go, and the path unit is disabled, so that a request
// file no longer starts a root update.
func TestReinstallWithoutHelperRemovesIt(t *testing.T) {
	root, bin := setup(t)
	if err := Install(Options{Root: root, Binary: bin, Listen: "0.0.0.0:8099", Out: &bytes.Buffer{}, Run: (&recorder{}).run}); err != nil {
		t.Fatal(err)
	}
	units := []string{UnitDir + "/massimo-update.path", UnitDir + "/massimo-update.service", HealthDropIn}
	for _, f := range units {
		if _, err := os.Stat(filepath.Join(root, f)); err != nil {
			t.Fatalf("first install: %v", err)
		}
	}
	rec := &recorder{}
	var out bytes.Buffer
	if err := Install(Options{Root: root, Binary: bin, NoUpdateHelper: true, Out: &out, Run: rec.run}); err != nil {
		t.Fatal(err)
	}
	for _, f := range units {
		if _, err := os.Stat(filepath.Join(root, f)); err == nil {
			t.Errorf("%s left after -no-update-helper", f)
		}
	}
	if _, err := os.Stat(filepath.Join(root, filepath.Dir(HealthDropIn))); err == nil {
		t.Errorf("the empty drop-in directory %s left", filepath.Dir(HealthDropIn))
	}
	if !rec.has("systemctl disable --now massimo-update.path") {
		t.Fatalf("the path unit is not disabled: %q", rec.cmds)
	}
	if _, err := os.Stat(filepath.Join(root, ListenDropIn)); err != nil {
		t.Errorf("the serve drop-in went with the helper: %v", err)
	}
	// The order: the units are disabled before the reload forgets them.
	disable, reload := -1, -1
	for i, c := range rec.cmds {
		switch {
		case strings.HasPrefix(c, "systemctl disable --now massimo-update.path") && disable < 0:
			disable = i
		case c == "systemctl daemon-reload" && reload < 0:
			reload = i
		}
	}
	if disable < 0 || reload < disable {
		t.Errorf("disable at %d, daemon-reload at %d: %q", disable, reload, rec.cmds)
	}
}

// A new -tls mode takes the TLS variables of the old one out of the
// settings, so that serve, which refuses -tls-http or a DNS hook outside
// acme, starts after the switch; the owner's other lines stay.
func TestInstallSwitchesTLSMode(t *testing.T) {
	root, bin := setup(t)
	hook := filepath.Join(t.TempDir(), "hook.sh")
	os.WriteFile(hook, []byte("#!/bin/sh\n"), 0o755)
	cert, key := filepath.Join(t.TempDir(), "c.pem"), filepath.Join(t.TempDir(), "k.pem")
	os.WriteFile(cert, []byte("CERT"), 0o644)
	os.WriteFile(key, []byte("KEY"), 0o600)
	env := filepath.Join(root, EnvPath)
	install := func(o Options) map[string]string {
		t.Helper()
		o.Root, o.Binary, o.Out, o.Run = root, bin, &bytes.Buffer{}, (&recorder{}).run
		if err := Install(o); err != nil {
			t.Fatal(err)
		}
		vars, err := readEnv(env)
		if err != nil {
			t.Fatal(err)
		}
		return vars
	}
	install(Options{TLS: "acme", TLSNames: "massimo.example.net", ACMEEmail: "owner@example.net", ACMEDirectory: "https://acme.example.net/directory", TLSHTTP: ":80"})
	f, _ := os.OpenFile(env, os.O_APPEND|os.O_WRONLY, 0)
	f.WriteString("MASSIMO_API_TOKEN=kept\n")
	f.Close()
	for _, c := range []struct {
		o          Options
		want, gone []string
	}{
		{Options{TLS: "off"}, []string{"MASSIMO_TLS=off"}, []string{"MASSIMO_TLS_HTTP", "MASSIMO_TLS_NAMES", "MASSIMO_ACME_EMAIL", "MASSIMO_ACME_DIRECTORY"}},
		{Options{TLS: "acme", TLSNames: "massimo.example.net", ACMEDirectory: "https://acme.example.net/directory", ACMEDNSHook: hook}, []string{"MASSIMO_TLS=acme", "MASSIMO_ACME_DIRECTORY=https://acme.example.net/directory", "MASSIMO_ACME_DNS_HOOK=" + hook}, []string{"MASSIMO_TLS_HTTP"}},
		{Options{TLS: "acme", TLSNames: "massimo.example.net", TLSHTTP: ":80"}, []string{"MASSIMO_TLS_HTTP=:80"}, []string{"MASSIMO_ACME_DNS_HOOK"}},
		{Options{TLSCert: cert, TLSKey: key}, []string{"MASSIMO_TLS=files", "MASSIMO_TLS_CERT=/etc/massimo/cert.pem"}, []string{"MASSIMO_TLS_HTTP", "MASSIMO_TLS_NAMES"}},
		{Options{TLS: "self-signed", TLSNames: "nas.lan"}, []string{"MASSIMO_TLS=self-signed", "MASSIMO_TLS_NAMES=nas.lan"}, []string{"MASSIMO_TLS_CERT", "MASSIMO_TLS_KEY"}},
		{Options{}, []string{"MASSIMO_TLS=self-signed", "MASSIMO_TLS_NAMES=nas.lan"}, nil}, // no -tls: the settings stay
	} {
		vars := install(c.o)
		for _, w := range c.want {
			k, v, _ := strings.Cut(w, "=")
			if vars[k] != v {
				t.Errorf("-tls %q: %s=%q, want %q", c.o.tlsMode(), k, vars[k], v)
			}
		}
		for _, g := range c.gone {
			if _, ok := vars[g]; ok {
				t.Errorf("-tls %q: %s left from the mode before", c.o.tlsMode(), g)
			}
		}
		if vars["MASSIMO_API_TOKEN"] != "kept" {
			t.Fatalf("-tls %q: the owner's line went", c.o.tlsMode())
		}
	}
}

// Every TLS variable install writes is one serve reads, as massimo
// config knows them.
func TestTLSVariablesAreKnown(t *testing.T) {
	for _, k := range modeVars {
		if !env.Known(k) {
			t.Errorf("%s is not a variable serve knows", k)
		}
	}
}

func TestTakenOver(t *testing.T) {
	const chown = "hand the service its files: sudo chown -R _massimo:_massimo /etc/massimo /var/lib/massimo /var/log/massimo\n"
	for _, c := range []struct {
		name, owner string
		want        bool
	}{
		{"a person's massimo owns the settings", formerUser, true},
		{"the service's", User, false},
		{"no answer", "", false},
	} {
		t.Run(c.name, func(t *testing.T) {
			root, bin := setup(t)
			sys := system{passwd: map[string]string{formerUser: personEntry}, owner: c.owner}
			var out bytes.Buffer
			if err := Install(Options{Root: root, Binary: bin, Out: &out, Run: (&recorder{}).run, Output: sys.output}); err != nil {
				t.Fatal(err)
			}
			if got := strings.Contains(out.String(), chown); got != c.want {
				t.Errorf("chown named %v, want %v: %s", got, c.want, out.String())
			}
			if c.want && !strings.Contains(out.String(), "warning: /etc/massimo/env belongs to the user massimo, which an older release ran the service as") {
				t.Errorf("no warning: %s", out.String())
			}
		})
	}
}
