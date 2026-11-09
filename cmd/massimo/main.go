// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

// Command massimo is the command-line entry point.
package main

import (
	"errors"
	"fmt"
	"os"

	"go.metaworx.ch/massimo/internal/env"
)

func main() {
	args, err := globalOptions(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(2)
	}
	if len(args) < 1 {
		usage()
		os.Exit(2)
	}
	useServiceDefaults()
	if err := env.LoadFile(env.File()); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
	if selectedSN == "" {
		selectedSN = os.Getenv(env.SN)
	}
	switch args[0] {
	case "version", "--version", "-v", "-V", "help", "-h", "--help":
	default:
		// Every run announces its version on stderr, so logs and bug
		// reports say which build produced them. stdout stays data-only.
		fmt.Fprintln(os.Stderr, versionString())
	}
	rest := args[1:]
	switch args[0] {
	case "decode":
		err = runDecode(rest)
	case "portal-crypto":
		err = runPortalCrypto(rest)
	case "portal":
		err = runPortal(rest)
	case "cube":
		err = runCube(rest)
	case "watch":
		err = runWatch(rest)
	case "status":
		err = runStatus(rest)
	case "backup":
		err = runBackup(rest)
	case "restore":
		err = runRestore(rest)
	case "fakecube":
		err = runFakeCube(rest)
	case "discover":
		err = runDiscover(rest)
	case "netconfig":
		err = runNetconfig(rest)
	case "config":
		err = runConfig(rest)
	case "serve":
		err = runServe(rest)
	case "legacy":
		err = runLegacy(rest)
	case "update":
		err = runUpdate(rest)
	case "restart":
		err = runRestart(rest)
	case "store":
		err = runStore(rest)
	case "tls":
		err = runTLS(rest)
	case "user":
		err = runUser(rest)
	case "install":
		err = runInstall(rest)
	case "version", "--version", "-v", "-V":
		fmt.Println(versionString())
	case "help", "-h", "--help":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n", args[0])
		usage()
		os.Exit(2)
	}
	if err != nil && !errors.Is(err, errRestart) {
		fmt.Fprintln(os.Stderr, "error:", err)
	}
	// A Windows service that its manager stopped reports that before the
	// process ends.
	serviceEnd()
	// A serve that stopped for a restart exits with exitRestart, which its
	// service manager answers with a start (restart.go).
	if code := exitCode(err); code != 0 {
		os.Exit(code)
	}
}

func usage() {
	fmt.Fprint(os.Stderr, `usage: massimo [-env <file>] [-data <dir>] [-sn <serial>] [-log-level <level>] [-show-hidden] [-direct | -no-direct] <command> [args]

Secrets such as MASSIMO_CUBE_KEY and MASSIMO_CUBE_IV are read from the
environment, or from a .env file in the current directory (KEY=VALUE lines;
override the path with -env or MASSIMO_ENV_FILE). Keep that file out of git.
On Linux and macOS the service user, which owns /etc/massimo/env, reads
that file alone and keeps its data in /var/lib/massimo, from any directory,
unless -env or MASSIMO_ENV_FILE name another file.
Several Cubes: put the serial into the variable names
(MASSIMO_CUBE_KEQ0123456_KEY, _IV, _ADDRESS) and pick one with -sn or
MASSIMO_CUBE_SN; with -sn and no address the Cube is discovered on the
LAN. Devices in room 255 (the parking room for devices that never
report) and those in MASSIMO_CUBE_<SN>_IGNORE are hidden everywhere;
-show-hidden (older names -show-ghosts, -no-ignore) shows and touches
them for this run. With -addr, the serial the Cube reports selects its
key. -addr defaults to MASSIMO_CUBE_ADDRESS. With MASSIMO_CUBE_<SN>_PROXY
set, commands connect to that running massimo serve; if it does not
answer within two seconds they warn and connect to the Cube itself,
which takes the Cube's only connection while serve is away. -no-direct
makes that an error instead; -direct skips the proxy and connects to the
Cube itself even when the proxy answers. -addr wins over both.
-log-level debug, info, warn or error (or MASSIMO_LOG_LEVEL; default
info) is the level the commands log at, also after serve's name.

serve keeps the Cubes and its settings in massimo.db in the data directory
(-data, MASSIMO_DATA, the service user's /var/lib/massimo, else the current
directory); after its first start
the file's _ADDRESS, _NAME, _PROXY, _PROXY_SERIAL and _IGNORE lines only
add Cubes the store does not know. The other commands keep reading them.

commands:
  decode [file]   decode Cube protocol lines (default: stdin) and print JSON
  portal-crypto   AES helpers for the Cube's portal encryption (see its -h)
  portal          fake portal server that decrypts and logs what a Cube sends
  cube            send protocol lines to a Cube over the LAN
  watch           stay connected to a Cube and print state changes
  status          show every device, its state and which rooms accept commands
  backup          save the Cube's configuration to a JSON or YAML file
  restore         write a backup back to the Cube (only the differences)
  fakecube        serve a recorded greeting as if it were a Cube, for tests
  discover        list the Cubes on the local network (UDP broadcast)
  netconfig       show or set a Cube's network configuration (static or DHCP; see its -h)
  config          show the settings in effect, with keys and IVs redacted
  serve           keep the Cubes connected, serve the JSON API and act as a Cube on the LAN
  legacy          run the original MAX! software with filtered output
  store           back up, migrate or describe serve's store, massimo.db (see its usage)
  tls             show the certificate serve presents, or make a new self-signed one (see its usage)
  user            add, list, disable and enable the users of serve's store, set passwords, make API keys (see its usage)
  install         install massimo serve as a service (systemd), or update or -remove it
  update          install the newest signed release from GitHub (-check only looks)
  restart         restart the installed server (systemd, launchd, the Windows service)
  version         print the version (also --version)
`)
}
