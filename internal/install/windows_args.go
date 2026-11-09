// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package install

import (
	"strings"
)

// ServiceName is the Windows service (and the name `massimo update
// -restart` takes there).
const ServiceName = "massimo"

// DefaultListen is the API and web UI address when -listen is not given.
const DefaultListen = ":8099"

// WinPaths is the layout of a Windows install.
type WinPaths struct {
	Dir     string // %ProgramFiles%\Massimo
	Bin     string // …\massimo.exe
	DataDir string // %ProgramData%\Massimo: settings, store and log, SYSTEM and Administrators only
	Env     string // …\env
	Log     string // …\serve.log
	// The update from the web UI: the request the server writes, the
	// helper's status and the helper's log (the server starts the helper
	// itself on Windows, cmd/massimo/update_helper_windows.go).
	Request   string // …\update-request
	Status    string // …\update-status.json
	UpdateLog string // …\update.log
}

// WindowsPaths derives the layout from %ProgramFiles% and %ProgramData%
// (empty values take the usual C:\ defaults).
func WindowsPaths(programFiles, programData string) WinPaths {
	if programFiles == "" {
		programFiles = `C:\Program Files`
	}
	if programData == "" {
		programData = `C:\ProgramData`
	}
	dir := winJoin(programFiles, "Massimo")
	data := winJoin(programData, "Massimo")
	return WinPaths{Dir: dir, Bin: winJoin(dir, "massimo.exe"), DataDir: data, Env: winJoin(data, "env"), Log: winJoin(data, "serve.log"),
		Request: winJoin(data, "update-request"), Status: winJoin(data, "update-status.json"), UpdateLog: winJoin(data, "update.log")}
}

func winJoin(a, b string) string { return strings.TrimRight(a, `\`) + `\` + b }

// ServiceArgs is the command line the service runs massimo with, after
// the binary: the same serve as the systemd unit's, with the store
// (-data) in the data directory. With the update
// helper it names the request and status files, which lets the web UI
// start updates; noHelper leaves them out and the button hidden.
func ServiceArgs(p WinPaths, listen string, noHelper bool) []string {
	if listen == "" {
		listen = DefaultListen
	}
	args := []string{"-env", p.Env, "-data", p.DataDir, "serve", "-listen", listen, "-log", p.Log}
	if !noHelper {
		args = append(args, "-update-request", p.Request, "-update-status", p.Status)
	}
	return args
}

// ListenFromCommandLine returns the -listen value of a service command
// line written by ServiceArgs, or "" when it has none, so that installing
// again keeps an address chosen earlier.
func ListenFromCommandLine(cmdline string) string {
	f := strings.Fields(cmdline)
	for i := 0; i+1 < len(f); i++ {
		if f[i] == "-listen" {
			return strings.Trim(f[i+1], `"`)
		}
	}
	return ""
}

// windowsEnvTemplate is envTemplate with the log in the data directory.
func windowsEnvTemplate(p WinPaths) string {
	t := strings.Replace(envTemplate(), "MASSIMO_LOG=/var/log/massimo/serve.log", "MASSIMO_LOG="+p.Log, 1)
	return strings.Replace(t, "Readable only by the user "+User+".", "Readable only by SYSTEM and Administrators.", 1)
}
