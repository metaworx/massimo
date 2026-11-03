// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

// Command massimo is the command-line entry point. For now it only offers
// "decode", which turns a captured Cube session into JSON.
package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	var err error
	switch os.Args[1] {
	case "decode":
		err = runDecode(os.Args[2:])
	case "portal-crypto":
		err = runPortalCrypto(os.Args[2:])
	case "portal":
		err = runPortal(os.Args[2:])
	case "cube":
		err = runCube(os.Args[2:])
	case "help", "-h", "--help":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n", os.Args[1])
		usage()
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprint(os.Stderr, `usage: massimo <command> [args]

commands:
  decode [file]   decode Cube protocol lines (default: stdin) and print JSON
  portal-crypto   AES helpers for the Cube's portal encryption (see its -h)
  portal          fake portal server that decrypts and logs what a Cube sends
  cube            send protocol lines to a Cube over the LAN
`)
}
