// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

// Command massimo is the command-line entry point.
package main

import (
	"fmt"
	"os"
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
	envFile := os.Getenv("MASSIMO_ENV_FILE")
	if envFile == "" {
		envFile = ".env"
	}
	if err := loadDotEnv(envFile); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
	if selectedSN == "" {
		selectedSN = os.Getenv(snEnv)
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
	case "config":
		err = runConfig(rest)
	case "serve":
		err = runServe(rest)
	case "legacy":
		err = runLegacy(rest)
	case "version", "--version", "-v", "-V":
		fmt.Println(versionString())
	case "help", "-h", "--help":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n", args[0])
		usage()
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprint(os.Stderr, `usage: massimo [-env <file>] [-sn <serial>] <command> [args]

Secrets such as MASSIMO_CUBE_KEY and MASSIMO_CUBE_IV are read from the
environment, or from a .env file in the current directory (KEY=VALUE lines;
override the path with -env or MASSIMO_ENV_FILE). Keep that file out of git.
Several Cubes: put the serial into the variable names
(MASSIMO_CUBE_KEQ0123456_KEY, _IV, _ADDRESS) and pick one with -sn or
MASSIMO_CUBE_SN; with -sn and no address the Cube is discovered on the
LAN. With -addr, the serial the Cube reports selects its key. -addr
defaults to MASSIMO_CUBE_ADDRESS.

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
  config          show the settings in effect, with keys and IVs redacted
  serve           keep the Cubes connected, serve the JSON API and act as a Cube on the LAN
  legacy          run the original MAX! software with filtered output
  version         print the version (also --version)
`)
}
