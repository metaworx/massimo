// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"

	"go.metaworx.ch/massimo/internal/cubelan/fakecube"
)

// runFakeCube serves a recorded greeting (a capture or the raw lines of a
// backup) on a TCP port, so the other commands can be tried without a Cube.
func runFakeCube(args []string) error {
	fs := flag.NewFlagSet("fakecube", flag.ContinueOnError)
	listen := fs.String("listen", "127.0.0.1:62910", "address to listen on")
	file := fs.String("greeting", "", "file with the protocol lines to send on connect (a capture, or a backup's lines)")
	duty := fs.Int("duty", 0, "duty cycle percentage to report in S replies")
	fs.Usage = func() {
		fmt.Fprint(os.Stderr, "usage: massimo fakecube -greeting <file> [-listen 127.0.0.1:62910]\n\nBehave like a Cube's LAN interface, using recorded data.\n\n")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *file == "" {
		return fmt.Errorf("fakecube: -greeting is required")
	}
	greeting, err := fakecube.LoadGreeting(*file)
	if err != nil {
		return err
	}
	srv := &fakecube.Server{Greeting: greeting, Duty: *duty}
	addr, err := srv.Start(*listen)
	if err != nil {
		return err
	}
	defer srv.Close()
	fmt.Fprintf(os.Stderr, "fake cube listening on %s (%d lines)\n", addr, len(greeting))
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	<-ctx.Done()
	return nil
}
