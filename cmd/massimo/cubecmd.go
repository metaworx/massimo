// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package main

import (
	"bufio"
	"flag"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"go.metaworx.ch/massimo/internal/protocol/cube"
)

// runCube is a minimal LAN client: it connects to the Cube, prints the
// greeting, sends each argument as one protocol line and prints the
// replies. It is meant for experiments, not for continuous operation.
func runCube(args []string) error {
	fs := flag.NewFlagSet("cube", flag.ContinueOnError)
	addr := fs.String("addr", "", "Cube address, host[:port] (default port 62910)")
	wait := fs.Duration("wait", 2*time.Second, "how long to wait for replies after each command")
	quiet := fs.Bool("quiet", false, "do not print the greeting")
	portal := fs.String("portal", "", "'on', 'off' or a number of seconds: control the Cube's own portal polling (sends the i: command)")
	fs.Usage = func() {
		fmt.Fprint(os.Stderr, "usage: massimo cube -addr <ip> [flags] [line ...]\n\nSend protocol lines to a Cube over the LAN and print the replies.\n\n")
		fs.PrintDefaults()
	}
	if err := fs.Parse(reorderFlags(fs, args)); err != nil {
		return err
	}
	if *addr == "" {
		return fmt.Errorf("cube: -addr is required")
	}
	if !strings.Contains(*addr, ":") {
		*addr += ":62910"
	}
	conn, err := net.DialTimeout("tcp", *addr, 5*time.Second)
	if err != nil {
		return err
	}
	defer conn.Close()
	r := bufio.NewReaderSize(conn, 16<<10)

	drain := func(show bool) {
		for {
			conn.SetReadDeadline(time.Now().Add(*wait))
			line, err := r.ReadString('\n')
			if line != "" && show {
				fmt.Println(strings.TrimRight(line, "\r\n"))
			}
			if err != nil {
				return
			}
		}
	}
	drain(!*quiet)

	lines := fs.Args()
	switch *portal {
	case "":
	case "on":
		lines = append(lines, cube.CmdSetPortalInterval(0).String())
	case "off":
		lines = append(lines, cube.CmdSetPortalInterval(-1).String())
	default:
		n, err := strconv.ParseInt(*portal, 10, 32)
		if err != nil || n < 0 {
			return fmt.Errorf("cube: -portal must be 'on', 'off' or a number of seconds")
		}
		lines = append(lines, cube.CmdSetPortalInterval(int32(n)).String())
	}
	for _, l := range lines {
		if _, err := cube.ParseLine(l); err != nil {
			return err
		}
		fmt.Println("> " + l)
		conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
		if _, err := conn.Write([]byte(l + "\r\n")); err != nil {
			return err
		}
		drain(true)
	}
	conn.Write([]byte("q:\r\n"))
	return nil
}

// reorderFlags moves flags (and their values) in front of positional
// arguments so that "cube -addr x 'u:...' -login a:b" works; the standard
// flag package stops parsing at the first positional argument.
func reorderFlags(fs *flag.FlagSet, args []string) []string {
	var flags, pos []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			pos = append(pos, args[i+1:]...)
			break
		}
		name, _, hasValue := strings.Cut(strings.TrimLeft(a, "-"), "=")
		f := fs.Lookup(name)
		if !strings.HasPrefix(a, "-") || f == nil {
			pos = append(pos, a)
			continue
		}
		flags = append(flags, a)
		if bf, ok := f.Value.(interface{ IsBoolFlag() bool }); (!ok || !bf.IsBoolFlag()) && !hasValue && i+1 < len(args) {
			i++
			flags = append(flags, args[i])
		}
	}
	return append(flags, pos...)
}
