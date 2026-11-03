// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package main

import (
	"bufio"
	"crypto/md5"
	"flag"
	"fmt"
	"net"
	"os"
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
	login := fs.String("login", "", "user:password; sends a B: line with the MD5 hash the Cube expects")
	fs.Usage = func() {
		fmt.Fprint(os.Stderr, "usage: massimo cube -addr <ip> [flags] [line ...]\n\nSend protocol lines to a Cube over the LAN and print the replies.\n\n")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
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

	var serial string
	drain := func(show bool) {
		for {
			conn.SetReadDeadline(time.Now().Add(*wait))
			line, err := r.ReadString('\n')
			if line != "" {
				line = strings.TrimRight(line, "\r\n")
				if strings.HasPrefix(line, "H:") {
					if h, err := cube.ParseHello(line[2:]); err == nil {
						serial = h.Serial
					}
				}
				if show {
					fmt.Println(line)
				}
			}
			if err != nil {
				return
			}
		}
	}
	drain(!*quiet)

	lines := fs.Args()
	if *login != "" {
		user, pass, ok := strings.Cut(*login, ":")
		if !ok || serial == "" {
			return fmt.Errorf("cube: -login needs user:password and a greeting with the serial")
		}
		lines = append(lines, fmt.Sprintf("B:%s,%x,1,%s", user, md5.Sum([]byte(pass+serial)), pass))
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
