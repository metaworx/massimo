// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package install

import (
	"runtime"
	"strings"
)

// debBin is the binary of the Debian package (packaging/nfpm.yaml).
const debBin = "/usr/bin/massimo"

// ServiceCommand is the command line, shown to people and not run, that
// runs bin with args as the service user on this platform: through sudo
// on Linux and macOS; on Windows, where the service runs as LocalSystem
// and an administrator's PowerShell is the way, the call of bin. Name bin
// by its absolute path: sudo's PATH on macOS has no /usr/local/bin.
func ServiceCommand(bin string, args ...string) string {
	if runtime.GOOS == "windows" {
		return winCommand(bin, args...)
	}
	return sudoCommand(bin, args...)
}

// sudoCommand is ServiceCommand on Linux and macOS, whatever this
// platform is: install's messages for them are tested everywhere.
func sudoCommand(bin string, args ...string) string {
	return strings.Join(append([]string{"sudo", "-u", User, shellQuote(bin)}, args...), " ")
}

// winCommand is ServiceCommand on Windows: PowerShell's call operator,
// bin quoted, since it is usually under C:\Program Files.
func winCommand(bin string, args ...string) string {
	return strings.Join(append([]string{`& "` + bin + `"`}, args...), " ")
}

// shellQuote quotes s for a POSIX shell, unless it holds only letters,
// digits and characters no shell treats specially.
func shellQuote(s string) string {
	if s != "" && strings.Trim(s, "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789/._-+:@%=,") == "" {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// ownerLine creates the first owner as the service user with bin, from
// any directory: the service user reads the service's settings file and
// keeps its data in StateDir without -env and -data (cmd/massimo,
// useServiceDefaults), so no ./.env is opened where the line is run.
func ownerLine(bin string) string {
	return sudoCommand(bin, "user", "add", "<name>", "-role", "owner")
}
