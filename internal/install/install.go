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
	"slices"
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
	// User is the service account, as on macOS: a name no person has,
	// since adduser refuses a user's name that starts with "_".
	User = "_massimo"
	// formerUser is the account's name before it took macOS's: a name a
	// person may have, Massimo being a common one.
	formerUser = "massimo"
)

// updateFlag is how massimo.service enables the web UI's update button;
// without the helper it is left out.
const updateFlag = " -update-request /var/lib/massimo/update-request"

// UpdatePathWant is the link that enabling massimo-update.path makes, by
// which serve knows that the Debian package's update helper is on.
const UpdatePathWant = "/etc/systemd/system/paths.target.wants/massimo-update.path"

// The reboot helper: the request file the server writes and the helper
// watches, on Linux and macOS alike, and on Linux the link that enabling
// massimo-reboot.path makes, by which serve knows the helper is there.
const (
	RebootRequest  = "/var/lib/massimo/reboot-request"
	RebootPathWant = "/etc/systemd/system/paths.target.wants/massimo-reboot.path"
	rebootPath     = "massimo-reboot.path"
)

// rebootWanted says whether the reboot helper stays or comes: asked for,
// taken out, or as present says it is.
func (in *installer) rebootWanted(present bool) bool {
	switch {
	case in.NoRebootHelper:
		return false
	case in.RebootHelper:
		return true
	}
	return present
}

// rebootHint says what else the reboot needs, once the helper was asked for.
func (in *installer) rebootHint(settings string) {
	if in.RebootHelper {
		fmt.Fprintf(in.Out, "the web UI offers the reboot once MASSIMO_REBOOT=on is in %s\n", settings)
	}
}

// enableReboot removes a request left from before, so that the helper
// does not take it at once, and enables the path unit, whose link serve
// looks for.
func (in *installer) enableReboot() error {
	in.remove(RebootRequest, false)
	if err := in.run("systemctl", "enable", "--now", rebootPath); err != nil {
		return err
	}
	in.rebootHint(EnvPath)
	return nil
}

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
	// RebootHelper installs the reboot helper (massimo-reboot.path and
	// .service; on macOS its LaunchDaemon), NoRebootHelper takes it out;
	// neither keeps what is there, brought up to date.
	RebootHelper, NoRebootHelper bool
	// NoStart enables the service without (re)starting it.
	NoStart bool
	// Listen, when set, is the API and web UI address (host:port), kept
	// in drop-ins that later installs leave alone; "" keeps what is there.
	Listen string
	// TLS is -tls: "" keeps what the settings file says, "off" switches
	// it off there, "files" (implied by TLSCert) installs TLSCert and
	// TLSKey beside the settings and names them in the settings file,
	// "self-signed" has the server make a certificate for the box's
	// names and TLSNames (tls.go).
	TLS             string
	TLSCert, TLSKey string
	TLSNames        string
	// ACMEEmail and TLSHTTP go with "acme": the account's contact
	// address, and the plain helper address (:80) for HTTP-01.
	ACMEEmail string
	TLSHTTP   string
	// ACMEDirectory, with "acme", is the CA's directory URL, such as Let's
	// Encrypt's staging one for a first try; empty is the server's default.
	ACMEDirectory string
	// ACMEDNSHook, with "acme", is the executable of the DNS-01 challenge.
	ACMEDNSHook string
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
	darwin  bool   // macOS, whose settings template has no MASSIMO_LOG
	stopped bool   // stopped for the account's rename: it ran, so it is started again
	reboot  bool   // the reboot helper stays or comes (rebootWanted)
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
		if _, _, err := listenDropIns(o.Listen, false, ""); err != nil {
			return err
		}
	}
	if err := o.checkTLS(); err != nil {
		return err
	}
	in := newInstaller(o)
	if err := in.binary(); err != nil {
		return err
	}
	if err := in.user(); err != nil {
		return err
	}
	if err := in.stateDir(); err != nil {
		return err
	}
	startable, err := in.settings()
	if err != nil {
		return err
	}
	if err := in.tlsFiles(EnvDir+"/"+certName, EnvDir+"/"+keyName, EnvPath, true); err != nil {
		return err
	}
	if startable {
		in.checkSettings()
	}
	_, err = os.Lstat(in.path(UnitDir + "/" + rebootPath))
	in.reboot = in.rebootWanted(err == nil)
	if err := in.units(); err != nil {
		return err
	}
	if err := in.listen(in.tlsEffective(in.path(EnvPath)), in.tlsHTTPEffective(in.path(EnvPath))); err != nil {
		return err
	}
	if in.NoUpdateHelper {
		in.removeHelper()
	}
	if in.NoRebootHelper {
		in.removeRebootHelper()
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
	if in.reboot {
		if err := in.enableReboot(); err != nil {
			return err
		}
	}
	switch {
	case in.NoStart:
		fmt.Fprintln(in.Out, "not started; start it with: systemctl start massimo")
	case !startable && !in.active() && !in.stopped:
		in.notStarted(ownerLine(BinPath), linuxStart)
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
// the package left as the template, the TLS options into the settings as
// on a hand install, the listen drop-in, the reboot helper's path unit
// enabled or disabled, a restart. The Debian package's update helper,
// which runs apt, is enabled as a hand install's is, with its health
// drop-in, unless -no-update-helper turns it off.
func configure(o Options, m Manager) error {
	if !m.Configurable() {
		return fmt.Errorf("%s; massimo install is for a box without the package", m)
	}
	if err := Check(o); err != nil {
		return err
	}
	if o.Listen != "" {
		if _, _, err := listenDropIns(o.Listen, false, ""); err != nil {
			return err
		}
	}
	if err := o.checkTLS(); err != nil {
		return err
	}
	// Only the Debian package ships an update helper, and only apt is
	// what it runs.
	helper := m.Name == "apt"
	if !helper {
		o.NoUpdateHelper = true
	}
	in := newInstaller(o)
	fmt.Fprintf(in.Out, "%s: the binary, the user and the unit are the package's; configuring only\n", m)
	startable, err := in.packagedSettings()
	if err != nil {
		return err
	}
	if err := in.tlsFiles(EnvDir+"/"+certName, EnvDir+"/"+keyName, EnvPath, true); err != nil {
		return err
	}
	if startable {
		in.checkSettings()
	}
	if err := in.listen(in.tlsEffective(in.path(EnvPath)), in.tlsHTTPEffective(in.path(EnvPath))); err != nil {
		return err
	}
	if helper && in.NoUpdateHelper {
		in.remove(HealthDropIn, false)
		if !in.DryRun {
			os.Remove(in.path(filepath.Dir(HealthDropIn))) // only when empty
		}
	}
	if err := in.run("systemctl", "daemon-reload"); err != nil {
		return err
	}
	// The package ships the update helper's path unit enabled.
	switch {
	case !helper:
	case in.NoUpdateHelper:
		in.run("systemctl", "disable", "--now", "massimo-update.path") // fails for a unit not enabled; that is fine
	default:
		if err := in.run("systemctl", "enable", "--now", "massimo-update.path"); err != nil {
			return err
		}
	}
	// The package ships the reboot helper's units disabled.
	switch {
	case in.RebootHelper:
		if err := in.enableReboot(); err != nil {
			return err
		}
	case in.NoRebootHelper:
		in.run("systemctl", "disable", "--now", rebootPath) // fails for a unit not enabled; that is fine
	}
	switch {
	case in.NoStart:
		fmt.Fprintln(in.Out, "not restarted; restart it with: systemctl restart massimo")
	case !startable && !in.active():
		in.notStarted(ownerLine(debBin), linuxStart)
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
	isTemplate := err != nil || onlyTemplate(cur, tmpl)
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

// user makes the service account. The one an older install or package
// made, formerUser with the home StateDir, is renamed in place: usermod
// keeps its ID, so its files follow. usermod refuses an account that a
// process runs as, so a running service is stopped first and started
// again by the start rule. Any other formerUser is a person's, and is
// left alone.
func (in *installer) user() error {
	if in.passwd(User) != "" {
		fmt.Fprintf(in.Out, "user %s exists\n", User)
		return nil
	}
	if ownAccount(in.passwd(formerUser)) {
		if in.active() {
			if err := in.run("systemctl", "stop", "massimo.service"); err != nil {
				return err
			}
			in.stopped = true
		}
		in.say("rename the user %s to %s, keeping its ID", formerUser, User)
		if err := in.run("usermod", "-l", User, formerUser); err != nil {
			return fmt.Errorf("rename the user %s to %s: %w; stop what runs as %s and install again", formerUser, User, err, formerUser)
		}
		if in.group(formerUser) && !in.group(User) {
			return in.run("groupmod", "-n", User, formerUser)
		}
		return nil
	}
	nologin := "/usr/sbin/nologin"
	if _, err := os.Stat(nologin); err != nil {
		nologin = "/sbin/nologin"
	}
	return in.run("useradd", "--system", "--user-group", "--home-dir", StateDir, "--no-create-home", "--shell", nologin, User)
}

// passwd is the passwd entry of the account name, "" when there is none;
// read-only, also in a dry run.
func (in *installer) passwd(name string) string {
	out, err := in.Output("getent", "passwd", name)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// group reports whether the group name exists; read-only.
func (in *installer) group(name string) bool {
	out, err := in.Output("getent", "group", name)
	return err == nil && len(bytes.TrimSpace(out)) > 0
}

// ownAccount reports whether a passwd entry is an account that an install
// or the package made for the service: its home is StateDir. A person's
// home is elsewhere.
func ownAccount(entry string) bool {
	f := strings.Split(entry, ":")
	return len(f) >= 7 && f[5] == StateDir
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

// settingLines is what a settings file sets: its lines, trimmed, without
// the comments and blank lines.
func settingLines(b []byte) []string {
	var out []string
	for _, l := range strings.Split(string(b), "\n") {
		if l = strings.TrimSpace(l); l != "" && !strings.HasPrefix(l, "#") {
			out = append(out, l)
		}
	}
	return out
}

// onlyTemplate reports whether a settings file sets what the template
// sets and nothing more. Comments and blank lines do not count, so a
// file left from a template whose comments a later release changed is
// the template still, as packaging/deb/postinst sees it.
func onlyTemplate(cur, tmpl []byte) bool {
	return slices.Equal(settingLines(cur), settingLines(tmpl))
}

// The service's first start is the owner's while the settings are only
// the template's: a server without users answers every device on the
// local network (README.md, "Server"). Whatever runs is restarted, so
// that it takes the new binary, also on the template's settings.
const linuxStart = "sudo systemctl start massimo"

// notStarted says why the service was left stopped and the two ways to
// its first owner: userAdd before the start (ownerLine), or the web UI
// after it.
func (in *installer) notStarted(userAdd, start string) {
	fmt.Fprintln(in.Out, "not started: until it has an owner, every device on the local network may use it")
	fmt.Fprintf(in.Out, "create the owner first: %s\n", userAdd)
	fmt.Fprintf(in.Out, "then start it: %s\n", start)
	fmt.Fprintln(in.Out, "or start it and create the owner in the web UI; the Cubes are added there (System settings → Cubes)")
}

// active asks systemd whether massimo.service runs, or waits to run
// again after a failure; read-only, also in a dry run.
func (in *installer) active() bool {
	out, _ := in.Output("systemctl", "is-active", "massimo.service")
	switch strings.TrimSpace(string(out)) {
	case "active", "activating", "reloading":
		return true
	}
	return false
}

// stateDir makes StateDir for the service user when it is missing:
// systemd makes it only at the first start (StateDirectory=), and
// massimo user add creates the owner there before that.
func (in *installer) stateDir() error {
	if _, err := os.Stat(in.path(StateDir)); !errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return in.dir(StateDir, in.svcUser+":"+in.svcUser, "0750")
}

// settings makes EnvDir and EnvPath; it reports whether the file holds
// settings (one copied, or one that sets more than the template)
// rather than only the template's.
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
	tmpl := []byte(envTemplate())
	if in.darwin {
		tmpl = []byte(darwinEnvTemplate())
	}
	if cur, err := os.ReadFile(env); err == nil {
		if in.Settings != "" {
			fmt.Fprintf(in.Out, "%s exists; kept it and did not copy %s\n", EnvPath, in.Settings)
		} else {
			fmt.Fprintf(in.Out, "%s exists; kept it\n", EnvPath)
		}
		return !onlyTemplate(cur, tmpl), nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return false, err
	}
	content, startable := tmpl, false
	if in.Settings != "" {
		b, err := os.ReadFile(in.Settings)
		if err != nil {
			return false, err
		}
		content, startable = b, true
		in.say("copy %s to %s", in.Settings, EnvPath)
	} else {
		in.say("write the commented template as %s", EnvPath)
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
		fmt.Fprintln(in.Out, "warning: no MASSIMO_API_TOKEN: until it has a user (massimo user add, or the web UI), every device on the local network may use it")
	}
	in.checkTLSSettings(vars)
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
	if in.reboot {
		files = append(files,
			[2]string{"systemd/" + rebootPath, UnitDir + "/" + rebootPath},
			[2]string{"systemd/massimo-reboot.service", UnitDir + "/massimo-reboot.service"})
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

// removeHelper takes out the update helper of an earlier install, as
// InstallDarwin unloads its own: the path unit disabled and stopped, its
// units and the health drop-in removed, so that a request file no longer
// starts a root update.
func (in *installer) removeHelper() {
	units := []string{UnitDir + "/massimo-update.path", UnitDir + "/massimo-update.service", HealthDropIn}
	found := false
	for _, p := range units {
		if _, err := os.Lstat(in.path(p)); err == nil {
			found = true
		}
	}
	if !found {
		return
	}
	// Disabling fails for a unit systemd does not know; that is fine.
	in.run("systemctl", "disable", "--now", "massimo-update.path")
	for _, p := range units {
		in.remove(p, false)
	}
	if !in.DryRun {
		os.Remove(in.path(filepath.Dir(HealthDropIn))) // only when empty
	}
}

// removeRebootHelper takes out the reboot helper of an earlier install:
// the path unit disabled and stopped, so that serve no longer offers the
// reboot, and its units removed.
func (in *installer) removeRebootHelper() {
	units := []string{UnitDir + "/" + rebootPath, UnitDir + "/massimo-reboot.service"}
	if _, err := os.Lstat(in.path(units[0])); err != nil {
		return
	}
	in.run("systemctl", "disable", "--now", rebootPath) // fails for a unit systemd does not know; that is fine
	for _, p := range units {
		in.remove(p, false)
	}
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
	in.run("systemctl", "disable", "--now", "massimo-update.path", rebootPath, "massimo.service")
	for _, p := range []string{UnitDir + "/massimo.service", UnitDir + "/massimo-update.path", UnitDir + "/massimo-update.service",
		UnitDir + "/" + rebootPath, UnitDir + "/massimo-reboot.service",
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
	return in.removeUsers()
}

// removeUsers removes the service account, and formerUser where an older
// install made it and no install since renamed it; a person's formerUser
// stays.
func (in *installer) removeUsers() error {
	for _, name := range []string{User, formerUser} {
		e := in.passwd(name)
		switch {
		case e == "":
			continue
		case name == formerUser && !ownAccount(e):
			fmt.Fprintf(in.Out, "kept the user %s, which is not the service's\n", name)
			continue
		}
		in.say("remove the user %s", name)
		if err := in.run("userdel", name); err != nil {
			return err
		}
	}
	return nil
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
