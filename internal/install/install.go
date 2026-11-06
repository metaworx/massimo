// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

// Package install sets massimo serve up as a service of the operating
// system: a systemd service on Linux, the steps packaging/README.md
// describes for an install by hand, with the unit files embedded from
// packaging/; a Windows service (windows.go); and LaunchDaemons on macOS
// (darwin.go). Running it again updates the binary and the service
// definition and restarts the service; Remove undoes it.
package install

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"go.metaworx.ch/massimo/packaging"
)

// The installed layout, as packaging/systemd/massimo.service expects it.
const (
	BinPath      = "/usr/local/bin/massimo"
	EnvDir       = "/etc/massimo"
	EnvPath      = EnvDir + "/env"
	UnitDir      = "/etc/systemd/system"
	LogrotateDir = "/etc/logrotate.d"
	User         = "massimo"
)

// updateFlag is how massimo.service enables the web UI's update button;
// without the helper it is left out.
const updateFlag = " -update-request /var/lib/massimo/update-request"

// Options configure an install.
type Options struct {
	// Root prefixes every path; "" is the running system. Set for tests,
	// or to prepare an image; commands are then not run.
	Root string
	// Binary is the file installed as BinPath; "" is the running one.
	Binary string
	// Settings, when set, is copied as EnvPath if there is none yet.
	Settings string
	// NoUpdateHelper leaves out massimo-update.path and .service.
	NoUpdateHelper bool
	// NoStart enables the service without (re)starting it.
	NoStart bool
	// Listen, when set, is the API and web UI address (host:port), kept
	// in drop-ins that later installs leave alone; "" keeps what is there.
	Listen string
	// DryRun only says what would be done.
	DryRun bool
	// Out receives one line per step; nil is os.Stdout.
	Out io.Writer
	// Run runs a command (useradd, chown, systemctl); nil runs it, or
	// with Root set, does nothing.
	Run func(name string, args ...string) error
	// Output runs a command that only reads (dscl on macOS) and returns
	// what it printed, also in a dry run; nil runs it, or with Root set,
	// returns nothing.
	Output func(name string, args ...string) ([]byte, error)
	// LocalAddrs lists this machine's addresses for the proxy check;
	// nil asks the network interfaces.
	LocalAddrs func() ([]net.IP, error)
}

type installer struct {
	Options
	svcUser string // the account the service runs as: User, or DarwinUser
}

func newInstaller(o Options) *installer {
	if o.Out == nil {
		o.Out = os.Stdout
	}
	if o.Run == nil {
		if o.Root != "" {
			o.Run = func(string, ...string) error { return nil }
		} else {
			o.Run = func(name string, args ...string) error {
				out, err := exec.Command(name, args...).CombinedOutput()
				if err != nil {
					return fmt.Errorf("%s %s: %v: %s", name, strings.Join(args, " "), err, strings.TrimSpace(string(out)))
				}
				return nil
			}
		}
	}
	if o.Output == nil {
		if o.Root != "" {
			o.Output = func(string, ...string) ([]byte, error) { return nil, nil }
		} else {
			o.Output = func(name string, args ...string) ([]byte, error) {
				return exec.Command(name, args...).Output()
			}
		}
	}
	return &installer{Options: o, svcUser: User}
}

func (in *installer) path(p string) string { return filepath.Join(in.Root, p) }

func (in *installer) say(format string, args ...any) {
	if in.DryRun {
		format = "would " + format
	}
	fmt.Fprintf(in.Out, format+"\n", args...)
}

func (in *installer) run(name string, args ...string) error {
	if in.DryRun {
		in.say("run %s %s", name, strings.Join(args, " "))
		return nil
	}
	return in.Run(name, args...)
}

// Check says why this system cannot take the install, or nil.
func Check(o Options) error {
	if o.Root != "" || o.DryRun {
		return nil
	}
	if os.Geteuid() != 0 {
		return errors.New("installing needs root: run it with sudo")
	}
	if fi, err := os.Stat("/run/systemd/system"); err != nil || !fi.IsDir() {
		return errors.New("this system does not run systemd; install by hand as packaging/README.md describes")
	}
	return nil
}

// Install installs or updates massimo serve as a systemd service. On a
// box whose package manager owns the binary (Packaged) it only
// configures: the settings and the listen address are still the box's
// own, the binary, the user and the unit are the package's.
func Install(o Options) error {
	if m, ok := Packaged(o.Root); ok {
		return configure(o, m)
	}
	if err := Check(o); err != nil {
		return err
	}
	if o.Listen != "" {
		if _, _, err := listenDropIns(o.Listen); err != nil {
			return err
		}
	}
	in := newInstaller(o)
	if err := in.binary(); err != nil {
		return err
	}
	if err := in.user(); err != nil {
		return err
	}
	startable, err := in.settings()
	if err != nil {
		return err
	}
	if startable {
		in.checkSettings()
	}
	if err := in.units(); err != nil {
		return err
	}
	if err := in.listen(); err != nil {
		return err
	}
	if err := in.run("systemctl", "daemon-reload"); err != nil {
		return err
	}
	if err := in.run("systemctl", "enable", "massimo.service"); err != nil {
		return err
	}
	if !in.NoUpdateHelper {
		if err := in.run("systemctl", "enable", "--now", "massimo-update.path"); err != nil {
			return err
		}
	}
	switch {
	case in.NoStart:
		fmt.Fprintln(in.Out, "not started; start it with: systemctl start massimo")
	case !startable:
		fmt.Fprintf(in.Out, "not started: put the Cubes into %s first (see packaging/README.md), then: systemctl start massimo\n", EnvPath)
	default:
		// restart, so that running install again takes the new binary.
		if err := in.run("systemctl", "restart", "massimo.service"); err != nil {
			return err
		}
		in.say("start massimo.service; follow it with: journalctl -u massimo -f")
	}
	return nil
}

// configure is Install on a packaged box: -settings over a settings file
// the package left as the template, the -listen drop-ins, a restart.
func configure(o Options, m Manager) error {
	if !m.Configurable() {
		return fmt.Errorf("%s; massimo install is for a box without the package", m)
	}
	if err := Check(o); err != nil {
		return err
	}
	if o.Listen != "" {
		if _, _, err := listenDropIns(o.Listen); err != nil {
			return err
		}
	}
	in := newInstaller(o)
	fmt.Fprintf(in.Out, "%s: the binary, the user and the unit are the package's; configuring only\n", m)
	startable, err := in.packagedSettings()
	if err != nil {
		return err
	}
	if startable {
		in.checkSettings()
	}
	if err := in.listen(); err != nil {
		return err
	}
	if err := in.run("systemctl", "daemon-reload"); err != nil {
		return err
	}
	switch {
	case in.NoStart:
		fmt.Fprintln(in.Out, "not restarted; restart it with: systemctl restart massimo")
	case !startable:
		fmt.Fprintf(in.Out, "not started: put the Cubes into %s first (see packaging/README.md), then: systemctl start massimo\n", EnvPath)
	default:
		if err := in.run("systemctl", "restart", "massimo.service"); err != nil {
			return err
		}
		in.say("restart massimo.service; follow it with: journalctl -u massimo -f")
	}
	return nil
}

// packagedSettings copies -settings over a settings file the package
// left as the template (its copy under /usr/share/massimo, else the
// embedded one), keeps one that was filled in, and reports whether the
// file holds settings.
func (in *installer) packagedSettings() (bool, error) {
	env := in.path(EnvPath)
	cur, err := os.ReadFile(env)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return false, err
	}
	tmpl, terr := os.ReadFile(in.path("/usr/share/massimo/env.template"))
	if terr != nil {
		tmpl = []byte(envTemplate())
	}
	isTemplate := err != nil || bytes.Equal(cur, tmpl)
	switch {
	case in.Settings == "" && isTemplate:
		fmt.Fprintf(in.Out, "%s is still the template\n", EnvPath)
		return false, nil
	case in.Settings == "":
		fmt.Fprintf(in.Out, "%s exists; kept it\n", EnvPath)
		return true, nil
	case !isTemplate:
		fmt.Fprintf(in.Out, "%s exists; kept it and did not copy %s\n", EnvPath, in.Settings)
		return true, nil
	}
	b, err := os.ReadFile(in.Settings)
	if err != nil {
		return false, err
	}
	in.say("copy %s to %s", in.Settings, EnvPath)
	if !in.DryRun {
		if err := os.MkdirAll(filepath.Dir(env), 0o750); err != nil {
			return false, err
		}
		if err := writeAtomic(env, b, 0o600); err != nil {
			return false, err
		}
	}
	return true, in.run("chown", in.svcUser+":"+in.svcUser, env)
}

func (in *installer) binary() error {
	src := in.Binary
	if src == "" {
		exe, err := os.Executable()
		if err != nil {
			return err
		}
		if src, err = filepath.EvalSymlinks(exe); err != nil {
			return err
		}
	}
	dst := in.path(BinPath)
	if a, err := os.Stat(src); err == nil {
		if b, err := os.Stat(dst); err == nil && os.SameFile(a, b) {
			fmt.Fprintf(in.Out, "%s is already the installed binary\n", BinPath)
			return nil
		}
	}
	in.say("install %s as %s", src, BinPath)
	if in.DryRun {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	b, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return writeAtomic(dst, b, 0o755)
}

func (in *installer) user() error {
	if in.Root == "" && in.Run("id", "-u", User) == nil { // read-only, also in a dry run
		fmt.Fprintf(in.Out, "user %s exists\n", User)
		return nil
	}
	nologin := "/usr/sbin/nologin"
	if _, err := os.Stat(nologin); err != nil {
		nologin = "/sbin/nologin"
	}
	return in.run("useradd", "--system", "--user-group", "--home-dir", "/var/lib/massimo", "--no-create-home", "--shell", nologin, User)
}

// envTemplate is the commented settings file a fresh install gets,
// packaging/env.template, which the Debian package ships as well.
func envTemplate() string {
	b, err := fs.ReadFile(packaging.Files, "env.template")
	if err != nil {
		panic("install: env.template not embedded: " + err.Error())
	}
	return string(b)
}

// settings makes EnvDir and EnvPath; it reports whether the file holds
// settings (an existing or copied one) rather than the fresh template.
func (in *installer) settings() (bool, error) {
	dir, env := in.path(EnvDir), in.path(EnvPath)
	if !in.DryRun {
		if err := os.MkdirAll(dir, 0o750); err != nil {
			return false, err
		}
	}
	if err := in.run("chown", in.svcUser+":"+in.svcUser, dir); err != nil {
		return false, err
	}
	if err := in.run("chmod", "0750", dir); err != nil {
		return false, err
	}
	if _, err := os.Stat(env); err == nil {
		if in.Settings != "" {
			fmt.Fprintf(in.Out, "%s exists; kept it and did not copy %s\n", EnvPath, in.Settings)
		} else {
			fmt.Fprintf(in.Out, "%s exists; kept it\n", EnvPath)
		}
		return true, nil
	}
	content, startable := []byte(envTemplate()), false
	if in.svcUser == DarwinUser {
		content = []byte(darwinEnvTemplate())
	}
	if in.Settings != "" {
		b, err := os.ReadFile(in.Settings)
		if err != nil {
			return false, err
		}
		content, startable = b, true
		in.say("copy %s to %s", in.Settings, EnvPath)
	} else {
		in.say("write a commented %s to fill in", EnvPath)
	}
	if !in.DryRun {
		if err := writeAtomic(env, content, 0o600); err != nil {
			return false, err
		}
	}
	return startable, in.run("chown", in.svcUser+":"+in.svcUser, env)
}

// checkSettings warns about settings the service will trip over. A proxy
// address must be one of this machine's (port 62910 is bound once per
// address), or serve fails to bind and restarts every ten seconds; the
// install adds no addresses, that is the network's owner's choice.
func (in *installer) checkSettings() {
	env := in.path(EnvPath)
	if _, err := os.Stat(env); err != nil {
		env = in.Settings // a dry run: the file that would be copied
	}
	if env == "" {
		return
	}
	vars, err := readEnv(env)
	if err != nil {
		fmt.Fprintf(in.Out, "warning: cannot check %s: %v\n", EnvPath, err)
		return
	}
	if vars["MASSIMO_API_TOKEN"] == "" {
		fmt.Fprintln(in.Out, "warning: no MASSIMO_API_TOKEN: the API is open to the LAN and the web UI cannot start updates")
	}
	lookup := in.LocalAddrs
	if lookup == nil {
		lookup = localAddrs
	}
	local, err := lookup()
	if err != nil {
		return
	}
	for k, v := range vars {
		if !strings.HasPrefix(k, "MASSIMO_CUBE_") || !strings.HasSuffix(k, "_PROXY") {
			continue
		}
		host := v
		if h, _, err := net.SplitHostPort(v); err == nil {
			host = h
		}
		ip := net.ParseIP(host)
		if ip == nil || ip.IsUnspecified() {
			continue // every interface, or a name
		}
		found := false
		for _, l := range local {
			found = found || l.Equal(ip)
		}
		if !found {
			fmt.Fprintf(in.Out, "warning: %s=%s is not an address of this machine; serve cannot listen there (add it, see packaging/README.md)\n", k, v)
		}
	}
}

func localAddrs() ([]net.IP, error) {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return nil, err
	}
	var ips []net.IP
	for _, a := range addrs {
		if n, ok := a.(*net.IPNet); ok {
			ips = append(ips, n.IP)
		}
	}
	return ips, nil
}

// readEnv reads KEY=VALUE lines as massimo's .env loader does, enough to
// check them: comments and blank lines skipped, quotes removed.
func readEnv(path string) (map[string]string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	vars := map[string]string{}
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "export "))
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		v = strings.TrimSpace(v)
		if len(v) >= 2 && (v[0] == '"' || v[0] == '\'') && v[len(v)-1] == v[0] {
			v = v[1 : len(v)-1]
		}
		vars[strings.TrimSpace(k)] = v
	}
	return vars, nil
}

// unitFiles lists what goes where: embedded name → installed path.
func (in *installer) unitFiles() [][2]string {
	files := [][2]string{{"systemd/massimo.service", UnitDir + "/massimo.service"}}
	if !in.NoUpdateHelper {
		files = append(files,
			[2]string{"systemd/massimo-update.path", UnitDir + "/massimo-update.path"},
			[2]string{"systemd/massimo-update.service", UnitDir + "/massimo-update.service"})
	}
	if fi, err := os.Stat(in.path(LogrotateDir)); err == nil && fi.IsDir() {
		files = append(files, [2]string{"logrotate/massimo", LogrotateDir + "/massimo"})
	}
	return files
}

func (in *installer) units() error {
	for _, f := range in.unitFiles() {
		b, err := fs.ReadFile(packaging.Files, f[0])
		if err != nil {
			return err
		}
		if f[0] == "systemd/massimo.service" && in.NoUpdateHelper {
			b = bytes.Replace(b, []byte(updateFlag), nil, 1)
		}
		dst := in.path(f[1])
		old, err := os.ReadFile(dst)
		switch {
		case err == nil && bytes.Equal(old, b):
			fmt.Fprintf(in.Out, "%s is current\n", f[1])
			continue
		case err == nil:
			// Edited by hand (systemctl edit --full) or from an older
			// release: keep it next to the new one.
			in.say("replace %s, keeping the old one as %s.bak", f[1], f[1])
			if !in.DryRun {
				if err := writeAtomic(dst+".bak", old, 0o644); err != nil {
					return err
				}
			}
		default:
			in.say("write %s", f[1])
		}
		if in.DryRun {
			continue
		}
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return err
		}
		if err := writeAtomic(dst, b, 0o644); err != nil {
			return err
		}
	}
	return nil
}

// Remove stops and removes the service, the units and the binary. With
// purge it removes the settings, the state, the logs and the user too.
// A packaged box is left to its package manager.
func Remove(o Options, purge bool) error {
	if m, ok := Packaged(o.Root); ok {
		return fmt.Errorf("%s; %s", m, m.RemoveHint())
	}
	if err := Check(o); err != nil {
		return err
	}
	in := newInstaller(o)
	// Stopping fails for units that are not there; that is fine.
	in.run("systemctl", "disable", "--now", "massimo-update.path", "massimo.service")
	for _, p := range []string{UnitDir + "/massimo.service", UnitDir + "/massimo-update.path", UnitDir + "/massimo-update.service",
		LogrotateDir + "/massimo", BinPath, BinPath + ".prev", BinPath + ".failed", ListenDropIn, HealthDropIn} {
		in.remove(p, false)
	}
	for _, d := range []string{ListenDropIn, HealthDropIn} {
		if !in.DryRun {
			os.Remove(in.path(filepath.Dir(d))) // only when empty
		}
	}
	if err := in.run("systemctl", "daemon-reload"); err != nil {
		return err
	}
	if !purge {
		fmt.Fprintf(in.Out, "kept %s, /var/lib/massimo, /var/log/massimo and the user %s (remove them with -purge)\n", EnvDir, User)
		return nil
	}
	for _, p := range []string{EnvDir, "/var/lib/massimo", "/var/lib/massimo-update", "/var/log/massimo"} {
		in.remove(p, true)
	}
	in.say("remove the user %s", User)
	return in.run("userdel", User)
}

func (in *installer) remove(p string, all bool) {
	if _, err := os.Lstat(in.path(p)); err != nil {
		return
	}
	in.say("remove %s", p)
	if in.DryRun {
		return
	}
	var err error
	if all {
		err = os.RemoveAll(in.path(p))
	} else {
		err = os.Remove(in.path(p))
	}
	if err != nil {
		fmt.Fprintf(in.Out, "could not remove %s: %v\n", p, err)
	}
}

// writeAtomic writes to a temporary file beside path and renames it over.
func writeAtomic(path string, b []byte, mode os.FileMode) error {
	tmp := filepath.Join(filepath.Dir(path), "."+filepath.Base(path)+".tmp")
	os.Remove(tmp)
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_EXCL|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	if _, err := f.Write(b); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	if err := os.Chmod(tmp, mode); err != nil { // umask
		os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, path)
}
