// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package install

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"html"
	"io/fs"
	"net"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"go.metaworx.ch/massimo/packaging"
)

// The macOS install: the binary, the settings, the state and the log in
// the same places as on Linux (/usr/local/bin/massimo, /etc/massimo/env,
// /var/lib/massimo, /var/log/massimo; on macOS /etc and /var are
// /private/etc and /private/var, which belong to the system's owner and
// survive system updates), so that the documentation, the defaults of
// serve's -update-status and the update helper's paths are one set. The
// service is the LaunchDaemon ch.metaworx.massimo, run by launchd as the
// hidden system user _massimo; the update helper is a second one,
// ch.metaworx.massimo-update, run as root when the server writes the
// update request. Both property lists are embedded from
// packaging/launchd/. Nothing here needs macOS to build, so the tests
// run everywhere with Options.Root.
const (
	Label          = "ch.metaworx.massimo"
	HelperLabel    = "ch.metaworx.massimo-update"
	DaemonDir      = "/Library/LaunchDaemons"
	DarwinUser     = "_massimo"
	StateDir       = "/var/lib/massimo"
	HelperStateDir = "/var/lib/massimo-update"
	LogDir         = "/var/log/massimo"
	HelperLog      = "/var/log/massimo-update.log"
	DarwinLog      = LogDir + "/serve.log"

	launchctl      = "/bin/launchctl"
	dscl           = "/usr/bin/dscl"
	socketfilterfw = "/usr/libexec/ApplicationFirewall/socketfilterfw"
)

// PlistPath is where the LaunchDaemon with this label is installed.
func PlistPath(label string) string { return DaemonDir + "/" + label + ".plist" }

const (
	listenMark      = "@LISTEN@"
	healthMark      = "@HEALTH@"
	updateArgsServe = "\t\t<string>-update-request</string>\n\t\t<string>/var/lib/massimo/update-request</string>\n"
)

// launchdPlists returns the service's and the helper's property lists for
// a listen address; without the helper the service gets no
// -update-request.
func launchdPlists(listen string, noHelper bool) (serve, helper []byte, err error) {
	host, port, err := net.SplitHostPort(listen)
	if err != nil {
		return nil, nil, fmt.Errorf("-listen %q: %v (want host:port or :port)", listen, err)
	}
	if n, err := strconv.Atoi(port); err != nil || n < 1 || n > 65535 {
		return nil, nil, fmt.Errorf("-listen %q: bad port", listen)
	}
	if serve, err = readPlist(Label); err != nil {
		return nil, nil, err
	}
	serve = bytes.Replace(serve, []byte("<string>"+listenMark+"</string>"), []byte("<string>"+xmlEscape(listen)+"</string>"), 1)
	if noHelper {
		serve = bytes.Replace(serve, []byte(updateArgsServe), nil, 1)
	}
	if helper, err = readPlist(HelperLabel); err != nil {
		return nil, nil, err
	}
	// Twice: in the comment and in the arguments.
	helper = bytes.ReplaceAll(helper, []byte(healthMark), []byte(xmlEscape(healthURL(host, port))))
	return serve, helper, nil
}

// readPlist reads an embedded property list with Unix line ends, which a
// checkout on Windows (autocrlf) would otherwise turn into CRLF.
func readPlist(label string) ([]byte, error) {
	b, err := fs.ReadFile(packaging.Files, "launchd/"+label+".plist")
	return bytes.ReplaceAll(b, []byte("\r\n"), []byte("\n")), err
}

func xmlEscape(s string) string {
	var b strings.Builder
	xml.EscapeText(&b, []byte(s))
	return b.String()
}

var listenArg = regexp.MustCompile(`<string>-listen</string>\s*<string>([^<]*)</string>`)

// ListenFromPlist returns the -listen value of a service property list
// written by install, or "" when it has none, so that installing again
// keeps an address chosen earlier.
func ListenFromPlist(b []byte) string {
	m := listenArg.FindSubmatch(b)
	if m == nil {
		return ""
	}
	return html.UnescapeString(string(m[1]))
}

// CheckDarwin says why this Mac cannot take the install, or nil.
func CheckDarwin(o Options) error {
	if o.Root != "" || o.DryRun {
		return nil
	}
	if os.Geteuid() != 0 {
		return errors.New("installing needs root: run it with sudo")
	}
	if _, err := os.Stat(launchctl); err != nil {
		return errors.New("no launchctl on this system")
	}
	return nil
}

// InstallDarwin installs or updates massimo serve as a LaunchDaemon.
func InstallDarwin(o Options) error {
	if err := CheckDarwin(o); err != nil {
		return err
	}
	if o.Listen != "" {
		if _, _, err := launchdPlists(o.Listen, false); err != nil {
			return err
		}
	}
	in := newInstaller(o)
	in.svcUser = DarwinUser

	listen := in.Listen
	if listen == "" {
		if b, err := os.ReadFile(in.path(PlistPath(Label))); err == nil {
			if listen = ListenFromPlist(b); listen != "" {
				fmt.Fprintf(in.Out, "listen address %s kept; change it with -listen\n", listen)
			}
		}
	}
	if listen == "" {
		listen = DefaultListen
	}
	serve, helper, err := launchdPlists(listen, in.NoUpdateHelper)
	if err != nil {
		return err
	}
	in.checkDarwinListen(listen)

	if err := in.binary(); err != nil {
		return err
	}
	if err := in.darwinUser(); err != nil {
		return err
	}
	startable, err := in.settings()
	if err != nil {
		return err
	}
	if startable {
		in.checkSettings()
		in.checkDarwinSettings()
	}
	owner := DarwinUser + ":" + DarwinUser
	for _, d := range []struct{ path, owner, mode string }{
		{StateDir, owner, "0750"},
		// launchd writes the log, not serve: root's directory, so that
		// the file cannot be swapped for a link elsewhere.
		{LogDir, "root:wheel", "0755"},
		{HelperStateDir, "root:wheel", "0755"},
	} {
		if err := in.dir(d.path, d.owner, d.mode); err != nil {
			return err
		}
	}
	if err := in.logFile(owner); err != nil {
		return err
	}

	if in.NoUpdateHelper {
		in.unload(HelperLabel)
		in.remove(PlistPath(HelperLabel), false)
	} else {
		changed, err := in.plist(HelperLabel, helper)
		if err != nil {
			return err
		}
		// Loaded as soon as it is there: it runs only on a request.
		if err := in.load(HelperLabel, changed, false); err != nil {
			return err
		}
	}
	changed, err := in.plist(Label, serve)
	if err != nil {
		return err
	}
	in.darwinFirewall()

	bootstrap := "launchctl bootstrap system " + PlistPath(Label)
	switch {
	case in.NoStart:
		fmt.Fprintf(in.Out, "not (re)started; it starts at the next boot, or now with: sudo %s\n", bootstrap)
	case !startable:
		fmt.Fprintf(in.Out, "not started: put the Cubes into %s first (see packaging/README.md), then: sudo %s\n", EnvPath, bootstrap)
	default:
		// Restarted when loaded already, so that installing again takes
		// the new binary.
		if err := in.load(Label, changed, true); err != nil {
			return err
		}
		in.say("start %s; follow it with: tail -f %s/serve.log", Label, LogDir)
	}
	return nil
}

// checkDarwinListen warns about an address the service user cannot bind:
// macOS lets a user other than root bind a port below 1024 only on every
// address.
func (in *installer) checkDarwinListen(listen string) {
	host, port, _ := net.SplitHostPort(listen)
	n, _ := strconv.Atoi(port)
	if n >= 1024 || host == "" {
		return
	}
	if ip := net.ParseIP(host); ip != nil && ip.IsUnspecified() {
		return
	}
	fmt.Fprintf(in.Out, "warning: on macOS only root may listen on a port below 1024 on a single address; %s cannot listen on %s, use :%s\n", DarwinUser, listen, port)
}

// darwinEnvTemplate is envTemplate without MASSIMO_LOG: launchd writes
// serve's stderr to the log.
func darwinEnvTemplate() string {
	t := strings.Replace(envTemplate, "MASSIMO_LOG=/var/log/massimo/serve.log\n",
		"# The log is "+DarwinLog+", written by launchd; no MASSIMO_LOG here.\n", 1)
	return strings.Replace(t, "by the user "+User+".", "by the user "+DarwinUser+".", 1)
}

// checkDarwinSettings warns about MASSIMO_LOG, which on macOS writes
// every line twice: launchd already appends serve's stderr to the log.
func (in *installer) checkDarwinSettings() {
	env := in.path(EnvPath)
	if _, err := os.Stat(env); err != nil {
		env = in.Settings
	}
	if vars, err := readEnv(env); err == nil && vars["MASSIMO_LOG"] != "" {
		fmt.Fprintf(in.Out, "warning: MASSIMO_LOG in %s: launchd writes the log to %s already; remove it, or every line comes twice\n", EnvPath, DarwinLog)
	}
}

// logFile makes the log, which launchd opens, as the service user's.
func (in *installer) logFile(owner string) error {
	p := in.path(DarwinLog)
	if _, err := os.Lstat(p); err == nil {
		return nil
	}
	in.say("make %s", DarwinLog)
	if !in.DryRun {
		f, err := os.OpenFile(p, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o640)
		if err != nil {
			return err
		}
		f.Close()
	}
	return in.run("chown", owner, p)
}

// darwinUser makes the hidden system user and group _massimo, with the
// lowest ID from 400 on that neither a user nor a group has.
func (in *installer) darwinUser() error {
	if in.Root == "" && in.Run(dscl, ".", "-read", "/Users/"+DarwinUser) == nil { // read-only, also in a dry run
		fmt.Fprintf(in.Out, "user %s exists\n", DarwinUser)
		return nil
	}
	group := "/Groups/" + DarwinUser
	gid := ""
	if in.Root == "" && in.Run(dscl, ".", "-read", group) == nil {
		out, err := in.Output(dscl, ".", "-read", group, "PrimaryGroupID")
		if err != nil {
			return fmt.Errorf("group %s: %v", DarwinUser, err)
		}
		gid = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(string(out)), "PrimaryGroupID:"))
	}
	id, err := in.freeID()
	if err != nil {
		return err
	}
	var cmds [][]string
	if gid == "" {
		gid = id
		cmds = append(cmds,
			[]string{"-create", group},
			[]string{"-create", group, "PrimaryGroupID", gid},
			[]string{"-create", group, "RealName", "Massimo"},
			[]string{"-create", group, "Password", "*"})
	}
	user := "/Users/" + DarwinUser
	cmds = append(cmds,
		[]string{"-create", user},
		[]string{"-create", user, "UniqueID", id},
		[]string{"-create", user, "PrimaryGroupID", gid},
		[]string{"-create", user, "UserShell", "/usr/bin/false"},
		[]string{"-create", user, "NFSHomeDirectory", StateDir},
		[]string{"-create", user, "RealName", "Massimo"},
		[]string{"-create", user, "Password", "*"},
		[]string{"-create", user, "IsHidden", "1"})
	in.say("make the hidden system user %s (ID %s)", DarwinUser, id)
	for _, c := range cmds {
		if err := in.run(dscl, append([]string{"."}, c...)...); err != nil {
			return err
		}
	}
	return nil
}

func (in *installer) freeID() (string, error) {
	used := map[int]bool{}
	for _, l := range [][]string{{"/Users", "UniqueID"}, {"/Groups", "PrimaryGroupID"}} {
		out, err := in.Output(dscl, ".", "-list", l[0], l[1])
		if err != nil {
			return "", fmt.Errorf("dscl . -list %s %s: %v", l[0], l[1], err)
		}
		for _, line := range strings.Split(string(out), "\n") {
			if f := strings.Fields(line); len(f) == 2 {
				if n, err := strconv.Atoi(f[1]); err == nil {
					used[n] = true
				}
			}
		}
	}
	for id := 400; id < 500; id++ {
		if !used[id] {
			return strconv.Itoa(id), nil
		}
	}
	return "", errors.New("no free system user ID between 400 and 499")
}

func (in *installer) dir(path, owner, mode string) error {
	if !in.DryRun {
		if err := os.MkdirAll(in.path(path), 0o755); err != nil {
			return err
		}
	}
	if err := in.run("chown", owner, in.path(path)); err != nil {
		return err
	}
	return in.run("chmod", mode, in.path(path))
}

// plist writes a property list (root, 0644, as launchd requires) and
// reports whether it changed.
func (in *installer) plist(label string, b []byte) (bool, error) {
	p := PlistPath(label)
	dst := in.path(p)
	old, err := os.ReadFile(dst)
	switch {
	case err == nil && bytes.Equal(old, b):
		fmt.Fprintf(in.Out, "%s is current\n", p)
		return false, nil
	case err == nil:
		in.say("replace %s", p)
	default:
		in.say("write %s", p)
	}
	if in.DryRun {
		return true, nil
	}
	if err := os.MkdirAll(in.path(DaemonDir), 0o755); err != nil {
		return false, err
	}
	return true, writeAtomic(dst, b, 0o644)
}

// loaded asks launchd whether the job is loaded; read-only, also in a
// dry run.
func (in *installer) loaded(label string) bool {
	return in.Root == "" && in.Run(launchctl, "print", "system/"+label) == nil
}

// load (re)loads a LaunchDaemon: a new or changed property list is
// booted out and in again, an unchanged one only restarted (kickstart -k)
// when restart is set.
func (in *installer) load(label string, changed, restart bool) error {
	target := "system/" + label
	loaded := in.loaded(label)
	if loaded && !changed {
		if !restart {
			return nil
		}
		return in.run(launchctl, "kickstart", "-k", target)
	}
	if loaded {
		in.unload(label)
	}
	// A job disabled with launchctl disable does not bootstrap.
	if err := in.run(launchctl, "enable", target); err != nil {
		return err
	}
	return in.run(launchctl, "bootstrap", "system", PlistPath(label))
}

// unload boots a job out and waits until launchd has let go of it, since
// bootstrapping it again before fails.
func (in *installer) unload(label string) {
	if !in.loaded(label) {
		return
	}
	in.run(launchctl, "bootout", "system/"+label)
	if in.DryRun {
		return
	}
	for i := 0; i < 50 && in.loaded(label); i++ {
		time.Sleep(200 * time.Millisecond)
	}
}

// darwinFirewall lets the LAN reach the binary when the application
// firewall is on; with it off there is nothing to do.
func (in *installer) darwinFirewall() {
	// "Firewall is enabled. (State = 1)", or "is disabled. (State = 0)".
	out, err := in.Output(socketfilterfw, "--getglobalstate")
	if err != nil || !bytes.Contains(out, []byte("is enabled")) {
		return
	}
	for _, a := range []string{"--add", "--unblockapp"} {
		if err := in.run(socketfilterfw, a, BinPath); err != nil {
			fmt.Fprintf(in.Out, "warning: firewall: %v\n", err)
		}
	}
}

// RemoveDarwin unloads and removes both LaunchDaemons and the binary;
// with purge also the settings, the state, the logs and the user.
func RemoveDarwin(o Options, purge bool) error {
	if err := CheckDarwin(o); err != nil {
		return err
	}
	in := newInstaller(o)
	in.svcUser = DarwinUser
	for _, l := range []string{HelperLabel, Label} {
		in.unload(l)
		in.remove(PlistPath(l), false)
	}
	if in.Root == "" {
		if _, err := os.Stat(socketfilterfw); err == nil {
			in.run(socketfilterfw, "--remove", BinPath) // fails when there is no rule
		}
	}
	for _, p := range []string{BinPath, BinPath + ".prev", BinPath + ".failed"} {
		in.remove(p, false)
	}
	if !purge {
		fmt.Fprintf(in.Out, "kept %s, %s, %s and the user %s (remove them with -purge)\n", EnvDir, StateDir, LogDir, DarwinUser)
		return nil
	}
	for _, p := range []string{EnvDir, StateDir, HelperStateDir, LogDir, HelperLog} {
		in.remove(p, true)
	}
	if in.Root == "" && in.Run(dscl, ".", "-read", "/Users/"+DarwinUser) != nil {
		return nil
	}
	in.say("remove the user and group %s", DarwinUser)
	if err := in.run(dscl, ".", "-delete", "/Users/"+DarwinUser); err != nil {
		return err
	}
	return in.run(dscl, ".", "-delete", "/Groups/"+DarwinUser)
}

// RestartLaunchd restarts a loaded LaunchDaemon (kickstart -k: the
// running instance is stopped first); for `massimo update -restart`. The
// label may carry its domain ("system/…").
func RestartLaunchd(label string) error {
	target := label
	if !strings.Contains(target, "/") {
		target = "system/" + target
	}
	out, err := exec.Command(launchctl, "kickstart", "-k", target).CombinedOutput()
	if err != nil {
		return fmt.Errorf("launchctl kickstart -k %s: %v: %s", target, err, strings.TrimSpace(string(out)))
	}
	return nil
}
