// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package main

import (
	"bufio"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"go.metaworx.ch/massimo/internal/install"
	"go.metaworx.ch/massimo/internal/update"
)

// channel is the update channel, the branch the build was made from; set
// at build time by the Makefile like version.
var channel = ""

// packaged says whether a package manager owns this binary, in which case
// update leaves the install to it; a variable for the test.
var packaged = func() (install.Manager, bool) { return install.Packaged("") }

// updateChannel is the channel updates come from: MASSIMO_UPDATE_CHANNEL,
// else the one compiled in.
func updateChannel() string {
	if c := os.Getenv("MASSIMO_UPDATE_CHANNEL"); c != "" {
		return c
	}
	return channel
}

func runUpdate(args []string) (err error) {
	fs := flag.NewFlagSet("update", flag.ContinueOnError)
	check := fs.Bool("check", false, "only show whether a newer release is out")
	ch := fs.String("channel", updateChannel(), "update channel: the branch whose releases to follow (default MASSIMO_UPDATE_CHANNEL, else the branch this build came from)")
	want := fs.String("version", "", "install this release tag instead of the newest, even an older one")
	rollback := fs.Bool("rollback", false, "go back to the binary before the last update (again: forward)")
	binary := fs.String("binary", "", "the binary to replace (default: this one)")
	restart := fs.String("restart", "", "restart this service afterwards: a systemd unit such as massimo.service, on macOS the LaunchDaemon "+install.Label+", on Windows the service massimo")
	health := fs.String("health", "", "after the restart, wait until this URL (the server's /healthz) reports the new version, else roll back")
	healthWait := fs.Duration("health-timeout", 2*time.Minute, "how long to wait for -health")
	request := fs.String("request", "", "helper mode: run only when this request file exists, take the channel from it and remove it; where apt owns massimo, run apt for the massimo source instead")
	statusPath := fs.String("status", "", "write the progress to this file, for the server")
	yes := fs.Bool("yes", false, "do not ask before installing")
	force := fs.Bool("force", false, "also replace a development or modified build")
	fs.Usage = func() {
		fmt.Fprint(os.Stderr, "usage: massimo update [-check] [-channel <branch>] [-version <tag>] [-rollback] [-restart <unit> [-health <url>]]\n\nFind the newest signed release on the update channel on GitHub, verify\nit and replace this binary with it (the old one is kept as <binary>.prev).\nA release is installed only when it is newer than the running version,\nunless -version names it. Replacing an installed binary needs root (an\nadministrator on Windows, where the running .exe is renamed aside);\nsee packaging/README.md, also for the update helper that lets the web UI\nstart an update. Where a package manager owns massimo, update leaves the\ninstall to it, and the helper of the Debian package runs apt.\n\n")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return err
	}
	ctx := context.Background()
	last := "" // the state this run last wrote to the status file
	status := func(s update.Status) {
		last = s.State
		if err := update.WriteStatus(*statusPath, s); err != nil {
			fmt.Fprintln(os.Stderr, "update: status:", err)
		}
	}

	if *request != "" {
		c, ok, terr := update.TakeRequest(*request)
		if terr != nil {
			status(update.Status{State: "failed", Message: terr.Error()})
			return terr
		}
		if !ok {
			return nil // the path unit fired for a request already taken
		}
		if c != "" {
			*ch = c
		}
		*yes = true
		// The request is taken: an error that returns without a status of
		// its own (a development build, no channel, the binary not found)
		// is the run's outcome too, so that the UI does not show the
		// previous run's.
		defer func() {
			if err != nil && last != "failed" && last != "rolled-back" {
				status(update.Status{State: "failed", From: version, Channel: *ch, Message: err.Error()})
			}
		}()
	}

	// A package manager that owns the binary keeps owning it: -check still
	// says what is out, nothing is installed or rolled back from here. The
	// helper asks apt where apt is the owner (update_apt.go).
	if !*check {
		if m, ok := packaged(); ok {
			if *request != "" && m.Name == "apt" {
				return aptUpdate(ctx, status, *health, *healthWait)
			}
			err := fmt.Errorf("this massimo is %s", m)
			status(update.Status{State: "failed", From: version, Message: err.Error()})
			return err
		}
	}

	target := *binary
	if target == "" {
		exe, err := os.Executable()
		if err != nil {
			return err
		}
		if target, err = filepath.EvalSymlinks(exe); err != nil {
			return err
		}
	}
	cur, curOK := update.ParseVersion(version)

	if *rollback {
		if err := update.Rollback(target, update.PrevPath(target)); err != nil {
			return err
		}
		fmt.Printf("rolled back %s; the replaced binary is now %s\n", target, update.PrevPath(target))
		return restartService(*restart)
	}

	if !*check {
		if (!curOK || cur.Dirty) && !*force && *want == "" {
			return fmt.Errorf("this is a development or modified build (%s); use -force to replace it anyway", versionOr(version))
		}
	}
	if *want == "" && !update.ValidChannel(*ch) {
		return errors.New("no update channel: set -channel or MASSIMO_UPDATE_CHANNEL (a build without the Makefile has none)")
	}

	status(update.Status{State: "checking", From: version, Channel: *ch})
	c := update.NewClient()
	var rel *update.Release
	if *want != "" {
		rel, err = c.ByTag(ctx, *want)
	} else {
		rel, err = c.Latest(ctx, *ch)
		if err == nil && rel == nil {
			err = fmt.Errorf("no signed release on channel %s", *ch)
		}
	}
	if err != nil {
		status(update.Status{State: "failed", From: version, Channel: *ch, Message: err.Error()})
		return err
	}
	newer := !curOK || update.Compare(rel.Version, cur) > 0
	fmt.Printf("running:  %s (channel %s)\n", versionOr(version), versionOr(channel))
	fmt.Printf("release:  %s (channel %s, %s)\n          %s\n", rel.Tag, rel.Channel, rel.Commit, rel.URL)
	if *want == "" && !newer {
		fmt.Println("up to date")
		status(update.Status{State: "up-to-date", From: version, To: rel.Tag, Channel: *ch})
		return nil
	}
	if *check {
		fmt.Println("a newer release is out; `massimo update` installs it")
		return nil
	}
	asset := update.AssetName()
	if _, ok := rel.Assets[asset]; !ok {
		err := fmt.Errorf("release %s has no binary for this platform (%s)", rel.Tag, asset)
		status(update.Status{State: "failed", From: version, To: rel.Tag, Channel: *ch, Message: err.Error()})
		return err
	}
	if !*yes && !confirm(fmt.Sprintf("replace %s with %s?", target, rel.Tag)) {
		return errors.New("cancelled")
	}

	fail := func(err error) error {
		update.Discard(target)
		status(update.Status{State: "failed", From: version, To: rel.Tag, Channel: *ch, Message: err.Error()})
		return err
	}
	status(update.Status{State: "installing", From: version, To: rel.Tag, Channel: *ch})
	f, err := update.Stage(target)
	if err != nil {
		return fail(err)
	}
	fmt.Printf("downloading %s …\n", asset)
	err = c.Download(ctx, rel, asset, f)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return fail(err)
	}
	if err := update.Smoke(ctx, f.Name(), rel.Tag); err != nil {
		return fail(err)
	}
	if err := update.Swap(target); err != nil {
		return fail(err)
	}
	fmt.Printf("installed %s as %s (the old one is %s)\n", rel.Tag, target, update.PrevPath(target))
	if *restart == "" {
		status(update.Status{State: "done", From: version, To: rel.Tag, Channel: *ch, Message: "installed; restart massimo to run it"})
		fmt.Println("restart massimo to run the new version")
		return nil
	}

	status(update.Status{State: "restarting", From: version, To: rel.Tag, Channel: *ch})
	err = restartService(*restart)
	if err == nil && *health != "" {
		err = waitHealthy(ctx, *health, rel.Tag, *healthWait)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "update:", err, "- rolling back")
		if rerr := update.Rollback(target, update.FailedPath(target)); rerr != nil {
			err = fmt.Errorf("%v; the rollback failed too: %v", err, rerr)
		} else if rerr := restartService(*restart); rerr != nil {
			err = fmt.Errorf("%v; rolled back, but the restart failed: %v", err, rerr)
		}
		status(update.Status{State: "rolled-back", From: version, To: rel.Tag, Channel: *ch, Message: err.Error()})
		return err
	}
	status(update.Status{State: "done", From: version, To: rel.Tag, Channel: *ch})
	fmt.Println("running", rel.Tag)
	return nil
}

func versionOr(s string) string {
	if s == "" {
		return "unknown"
	}
	return s
}

func confirm(q string) bool {
	fmt.Fprint(os.Stderr, q+" [y/N] ")
	line, _ := bufio.NewReader(os.Stdin).ReadString('\n')
	a := strings.ToLower(strings.TrimSpace(line))
	return a == "y" || a == "yes"
}

// restartService restarts the systemd unit, or on Windows the service.
func restartService(name string) error {
	if name == "" {
		return nil
	}
	switch runtime.GOOS {
	case "windows":
		return install.RestartService(name)
	case "darwin":
		return install.RestartLaunchd(name)
	}
	out, err := exec.Command("systemctl", "restart", name).CombinedOutput()
	if err != nil {
		return fmt.Errorf("systemctl restart %s: %v: %s", name, err, strings.TrimSpace(string(out)))
	}
	return nil
}

// healthClient is the client for a -health URL. An https:// URL on a
// host of this box, its loopback or an address one of its interfaces
// has when the check starts (install writes a specific -listen address
// into the URL), is not verified: the server's certificate may be
// self-signed or for a name, a connection to the box's own address
// never leaves it, and the peer's version is no secret; any other host
// is verified as usual.
func healthClient(rawURL string) *http.Client {
	c := &http.Client{Timeout: 5 * time.Second}
	if u, err := url.Parse(rawURL); err == nil && u.Scheme == "https" && ownHost(u.Hostname()) {
		c.Transport = &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}
	}
	return c
}

// interfaceAddrs lists the box's addresses; a variable for the test.
var interfaceAddrs = net.InterfaceAddrs

// ownHost says whether a URL's host is this box: its loopback, or an
// address of one of its interfaces. A name is not looked up.
func ownHost(host string) bool {
	if loopbackHost(host) {
		return true
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	addrs, err := interfaceAddrs()
	if err != nil {
		return false
	}
	for _, a := range addrs {
		var own net.IP
		switch v := a.(type) {
		case *net.IPNet:
			own = v.IP
		case *net.IPAddr:
			own = v.IP
		}
		if own != nil && own.Equal(ip) {
			return true
		}
	}
	return false
}

// loopbackHost says whether a URL's host is this box's loopback.
func loopbackHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// healthEvery is how often waitHealthy asks; a variable for the tests.
var healthEvery = 2 * time.Second

// waitHealthy polls the server's /healthz until it reports tag.
func waitHealthy(ctx context.Context, url, tag string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	client := healthClient(url)
	last := "no answer"
	for time.Now().Before(deadline) {
		if v, err := healthVersion(ctx, client, url); err != nil {
			last = err.Error()
		} else if f := strings.Fields(v); len(f) >= 2 && f[1] == tag {
			return nil
		} else {
			last = "it reports " + v
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(healthEvery):
		}
	}
	return fmt.Errorf("the server did not come up with %s within %s (%s)", tag, timeout, last)
}

func healthVersion(ctx context.Context, client *http.Client, url string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var h struct {
		OK      bool   `json:"ok"`
		Version string `json:"version"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&h); err != nil {
		return "", err
	}
	if !h.OK {
		return "", errors.New("not ok")
	}
	return h.Version, nil
}
