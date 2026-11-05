// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

//go:build windows

package install

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"
)

// The Windows install: the binary in %ProgramFiles%\Massimo, settings
// and log in %ProgramData%\Massimo (SYSTEM and Administrators only, since
// the settings hold the Cube keys), and the service "massimo", started
// automatically as LocalSystem and restarted by the service manager when
// it fails. The binary answers the service manager by itself
// (cmd/massimo/service_windows.go). A firewall rule lets the LAN reach
// it on private and domain networks.

func winPaths() WinPaths {
	return WindowsPaths(os.Getenv("ProgramFiles"), os.Getenv("ProgramData"))
}

const firewallRule = "Massimo"

func connect() (*mgr.Mgr, error) {
	m, err := mgr.Connect()
	if errors.Is(err, windows.ERROR_ACCESS_DENIED) {
		return nil, errors.New("installing needs an administrator: run it from a command prompt opened with \"Run as administrator\"")
	}
	return m, err
}

// InstallWindows installs or updates massimo serve as a Windows service.
func InstallWindows(o Options) error {
	if o.Listen != "" {
		if _, _, err := listenDropIns(o.Listen); err != nil { // the same validation as on Linux
			return err
		}
	}
	in := newInstaller(o)
	p := winPaths()
	m, err := connect()
	if err != nil && !in.DryRun {
		return err
	}
	if m != nil {
		defer m.Disconnect()
	}
	var s *mgr.Service
	if m != nil {
		if s, err = m.OpenService(ServiceName); err == nil {
			defer s.Close()
		} else {
			s = nil
		}
	}

	listen := in.Listen
	if s != nil {
		if cfg, err := s.Config(); err == nil && listen == "" {
			listen = ListenFromCommandLine(cfg.BinaryPathName)
		}
		// The running binary cannot be overwritten.
		if err := stopService(in, s); err != nil {
			return err
		}
	}
	if listen == "" {
		listen = DefaultListen
	}

	if err := in.winBinary(p); err != nil {
		return err
	}
	startable, err := in.winSettings(p)
	if err != nil {
		return err
	}

	args := ServiceArgs(p, listen)
	cfg := mgr.Config{
		DisplayName: "Massimo",
		Description: "Keeps the MAX! Cubes connected, serves the API and web UI and answers as a Cube on the LAN.",
		StartType:   mgr.StartAutomatic,
	}
	if s == nil {
		in.say("create the service %s: %s %s", ServiceName, p.Bin, strings.Join(args, " "))
		if !in.DryRun {
			if s, err = m.CreateService(ServiceName, p.Bin, cfg, args...); err != nil {
				return fmt.Errorf("create the service: %w", err)
			}
			defer s.Close()
		}
	} else {
		in.say("update the service %s: %s %s", ServiceName, p.Bin, strings.Join(args, " "))
		if !in.DryRun {
			old, err := s.Config()
			if err != nil {
				return err
			}
			old.DisplayName, old.Description, old.StartType = cfg.DisplayName, cfg.Description, cfg.StartType
			old.BinaryPathName = commandLine(p.Bin, args)
			if err := s.UpdateConfig(old); err != nil {
				return fmt.Errorf("update the service: %w", err)
			}
		}
	}
	if !in.DryRun {
		// Like Restart=on-failure: again after 10 s, for the first three
		// failures of a day.
		actions := []mgr.RecoveryAction{{Type: mgr.ServiceRestart, Delay: 10 * time.Second}, {Type: mgr.ServiceRestart, Delay: 10 * time.Second}, {Type: mgr.ServiceRestart, Delay: 10 * time.Second}}
		if err := s.SetRecoveryActions(actions, 24*60*60); err != nil {
			return fmt.Errorf("recovery actions: %w", err)
		}
	}

	// One rule for the program covers the API, the Cube proxy (TCP 62910)
	// and discovery (UDP 23272); not on public networks.
	in.run("netsh", "advfirewall", "firewall", "delete", "rule", "name="+firewallRule)
	if err := in.run("netsh", "advfirewall", "firewall", "add", "rule", "name="+firewallRule, "dir=in", "action=allow",
		"program="+p.Bin, "enable=yes", "profile=private,domain"); err != nil {
		fmt.Fprintf(in.Out, "warning: firewall rule: %v\n", err)
	}

	switch {
	case in.NoStart:
		fmt.Fprintln(in.Out, "not started; start it with: sc start massimo")
	case !startable:
		fmt.Fprintf(in.Out, "not started: put the Cubes into %s first (see packaging/README.md), then: sc start massimo\n", p.Env)
	default:
		in.say("start the service %s", ServiceName)
		if !in.DryRun {
			if err := s.Start(); err != nil {
				return fmt.Errorf("start the service: %w", err)
			}
			if err := waitState(s, svc.Running, time.Minute); err != nil {
				return err
			}
		}
		fmt.Fprintf(in.Out, "log: %s\n", p.Log)
	}
	return nil
}

func (in *installer) winBinary(p WinPaths) error {
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
	if a, err := os.Stat(src); err == nil {
		if b, err := os.Stat(p.Bin); err == nil && os.SameFile(a, b) {
			fmt.Fprintf(in.Out, "%s is already the installed binary\n", p.Bin)
			return nil
		}
	}
	in.say("install %s as %s", src, p.Bin)
	if in.DryRun {
		return nil
	}
	if err := os.MkdirAll(p.Dir, 0o755); err != nil {
		return err
	}
	b, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return writeAtomic(p.Bin, b, 0o755)
}

// winSettings makes the data directory, readable only by SYSTEM and the
// Administrators, and the settings file in it; like settings() it
// reports whether the file holds settings rather than the template.
func (in *installer) winSettings(p WinPaths) (bool, error) {
	if !in.DryRun {
		if err := os.MkdirAll(p.DataDir, 0o700); err != nil {
			return false, err
		}
	}
	// S-1-5-18 is SYSTEM, S-1-5-32-544 the Administrators group; the
	// inherited rights (Users may read in ProgramData) are dropped.
	if err := in.run("icacls", p.DataDir, "/inheritance:r", "/grant:r", "*S-1-5-18:(OI)(CI)F", "*S-1-5-32-544:(OI)(CI)F"); err != nil {
		return false, err
	}
	if _, err := os.Stat(p.Env); err == nil {
		fmt.Fprintf(in.Out, "%s exists; kept it\n", p.Env)
		return true, nil
	}
	content, startable := []byte(windowsEnvTemplate(p)), false
	if in.Settings != "" {
		b, err := os.ReadFile(in.Settings)
		if err != nil {
			return false, err
		}
		content, startable = b, true
		in.say("copy %s to %s", in.Settings, p.Env)
	} else {
		in.say("write a commented %s to fill in", p.Env)
	}
	if in.DryRun {
		return startable, nil
	}
	return startable, writeAtomic(p.Env, content, 0o600)
}

// RemoveWindows stops and deletes the service, the firewall rule and the
// program directory; with purge also the settings and the log.
func RemoveWindows(o Options, purge bool) error {
	in := newInstaller(o)
	p := winPaths()
	m, err := connect()
	if err != nil && !in.DryRun {
		return err
	}
	if m != nil {
		defer m.Disconnect()
		if s, err := m.OpenService(ServiceName); err == nil {
			if err := stopService(in, s); err != nil {
				s.Close()
				return err
			}
			in.say("delete the service %s", ServiceName)
			if !in.DryRun {
				err = s.Delete()
			}
			s.Close()
			if err != nil {
				return err
			}
		}
	}
	in.run("netsh", "advfirewall", "firewall", "delete", "rule", "name="+firewallRule)
	in.remove(p.Dir, true)
	if !purge {
		fmt.Fprintf(in.Out, "kept %s (remove it with -purge)\n", p.DataDir)
		return nil
	}
	in.remove(p.DataDir, true)
	return nil
}

func stopService(in *installer, s *mgr.Service) error {
	st, err := s.Query()
	if err != nil || st.State == svc.Stopped {
		return nil
	}
	in.say("stop the service %s", ServiceName)
	if in.DryRun {
		return nil
	}
	if _, err := s.Control(svc.Stop); err != nil {
		return fmt.Errorf("stop the service: %w", err)
	}
	return waitState(s, svc.Stopped, time.Minute)
}

func waitState(s *mgr.Service, want svc.State, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		st, err := s.Query()
		if err != nil {
			return err
		}
		if st.State == want {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("the service did not reach state %d within %s (now %d)", want, timeout, st.State)
		}
		time.Sleep(300 * time.Millisecond)
	}
}

// commandLine quotes the binary and the arguments as the service manager
// stores them, as mgr.CreateService does.
func commandLine(bin string, args []string) string {
	s := syscall.EscapeArg(bin)
	for _, a := range args {
		s += " " + syscall.EscapeArg(a)
	}
	return s
}

// RestartService stops the service if it runs and starts it again; for
// `massimo update -restart`.
func RestartService(name string) error {
	m, err := connect()
	if err != nil {
		return err
	}
	defer m.Disconnect()
	s, err := m.OpenService(name)
	if err != nil {
		return fmt.Errorf("service %s: %w", name, err)
	}
	defer s.Close()
	if err := stopService(newInstaller(Options{Out: os.Stderr}), s); err != nil {
		return err
	}
	if err := s.Start(); err != nil {
		return fmt.Errorf("start the service %s: %w", name, err)
	}
	return waitState(s, svc.Running, time.Minute)
}
