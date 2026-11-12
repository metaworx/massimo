// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package main

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"

	"go.metaworx.ch/massimo/internal/env"
	"go.metaworx.ch/massimo/internal/install"
	"go.metaworx.ch/massimo/internal/store"
)

// The service's settings file and data directory, as install and the
// Debian package make them; variables so that a test puts them into a
// directory of its own.
var (
	serviceEnvPath  = install.EnvPath
	serviceStateDir = install.StateDir
)

// serviceDefaults is set when this run is the service user's without
// -env or MASSIMO_ENV_FILE (useServiceDefaults): its settings file is
// the service's, and its data directory, unless -data or MASSIMO_DATA
// name another, the service's.
var serviceDefaults bool

// useServiceDefaults decides, after the global options and before the
// settings file is read, whether this run gets the service user's
// defaults. Run as the service user, which owns the service's settings
// file, massimo reads that file alone and keeps its data in the
// service's directory, as `sudo sshd` reads sshd's own configuration:
// store, user and tls then work from any directory, a home the service
// user cannot enter too, and act on what the service uses. -env and
// MASSIMO_ENV_FILE turn both defaults off; root and everyone else keep
// ./.env and the current directory.
func useServiceDefaults() {
	serviceDefaults = os.Getenv(env.FileVar) == "" && serviceUser(serviceEnvPath)
	if serviceDefaults {
		env.SetDefaultFile(serviceEnvPath)
	}
}

// serviceExample is the example line of a usage text that runs this
// binary with args for the service: as the service user on Linux and
// macOS, which then reads the service's settings and store; on Windows,
// where the service runs as LocalSystem, with the service's -env and
// -data in an administrator's PowerShell.
func serviceExample(args ...string) string {
	exe, err := os.Executable()
	if err != nil {
		exe = "massimo"
	}
	if runtime.GOOS == "windows" {
		p := install.WindowsPaths(os.Getenv("ProgramFiles"), os.Getenv("ProgramData"))
		return install.ServiceCommand(exe, append([]string{"-env", p.Env, "-data", p.DataDir}, args...)...)
	}
	return install.ServiceCommand(exe, args...)
}

// dataDirUsage is the paragraph of the usage texts of store, user and
// tls on the data directory, with serviceExample's line for args.
func dataDirUsage(args ...string) string {
	text := "The data directory is -data, else MASSIMO_DATA, else the current directory,\n" +
		"which must have a store already;\n"
	if runtime.GOOS == "windows" {
		text += "for the service's, in an administrator's PowerShell, for example\n"
	} else {
		text += "the service user, without -env, reads the service's settings and store from\n" +
			"any directory, for example\n"
	}
	return text + "  " + serviceExample(args...) + "\n"
}

// implicitDataDir reports whether the data directory is the current
// directory because nobody named one: no -data, no MASSIMO_DATA, not a
// Windows service, not the service user's default.
func implicitDataDir() bool {
	return os.Getenv(env.DataVar) == "" && serviceDataDir() == "" && !serviceDefaults
}

// serviceStoreDir is the service's data directory on this host where it
// exists, else empty: on Windows the installer's under %ProgramData%,
// elsewhere install's and the Debian package's. A directory this user
// may not enter still exists.
func serviceStoreDir() string {
	dir := serviceStateDir
	if runtime.GOOS == "windows" {
		dir = install.WindowsPaths(os.Getenv("ProgramFiles"), os.Getenv("ProgramData")).DataDir
	}
	if _, err := os.Stat(dir); errors.Is(err, fs.ErrNotExist) {
		return ""
	}
	return dir
}

// checkStoreDir refuses a command that would create a store in a data
// directory nobody named: the current directory without massimo.db.
// Such a store is nobody's, and serve never reads what the command
// writes into it. The error names the service's command for args where
// the service is installed, else -data.
func checkStoreDir(args ...string) error {
	dir := dataDir()
	if !implicitDataDir() {
		return nil
	}
	if _, err := os.Stat(store.Path(dir)); !errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		abs = dir
	}
	msg := fmt.Sprintf("no %s in the current directory %s, and neither -data nor %s names another", store.FileName, abs, env.DataVar)
	if svc := serviceStoreDir(); svc != "" {
		return fmt.Errorf("%s; the service's store is in %s, for example\n  %s", msg, svc, serviceExample(args...))
	}
	return fmt.Errorf("%s: give -data, the directory serve keeps its store in", msg)
}

// warnOtherStore warns on w when a command of args works on a store
// other than the service's, where the service is installed on this host
// and this run is not the service user's.
func warnOtherStore(w io.Writer, file string, args ...string) {
	svc := serviceStoreDir()
	if svc == "" || serviceDefaults || serviceDataDir() != "" || sameDir(filepath.Dir(file), svc) {
		return
	}
	fmt.Fprintf(w, "warning: %s is not the service's store, which is in %s; for that one, for example\n  %s\n", file, svc, serviceExample(args...))
}

// sameDir reports whether a and b name the same directory, compared as
// absolute, cleaned paths.
func sameDir(a, b string) bool {
	aa, err1 := filepath.Abs(a)
	bb, err2 := filepath.Abs(b)
	return err1 == nil && err2 == nil && aa == bb
}
