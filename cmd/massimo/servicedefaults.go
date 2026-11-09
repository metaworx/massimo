// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package main

import (
	"os"
	"runtime"

	"go.metaworx.ch/massimo/internal/env"
	"go.metaworx.ch/massimo/internal/install"
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
	text := "The data directory is -data, else MASSIMO_DATA, else the current directory;\n"
	if runtime.GOOS == "windows" {
		text += "for the service's, in an administrator's PowerShell, for example\n"
	} else {
		text += "the service user, without -env, reads the service's settings and store from\n" +
			"any directory, for example\n"
	}
	return text + "  " + serviceExample(args...) + "\n"
}
