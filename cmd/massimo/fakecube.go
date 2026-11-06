// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"time"

	"go.metaworx.ch/massimo/internal/cubelan/fakecube"
	"go.metaworx.ch/massimo/internal/protocol/cube"
)

// runFakeCube serves a recorded greeting (a capture or the raw lines of a
// backup) on a TCP port, so the other commands can be tried without a Cube.
func runFakeCube(args []string) error {
	fs := flag.NewFlagSet("fakecube", flag.ContinueOnError)
	listen := fs.String("listen", "127.0.0.1:62910", "address to listen on")
	file := fs.String("greeting", "", "file with the protocol lines to send on connect (a capture, or a backup's lines)")
	duty := fs.Int("duty", 0, "duty cycle percentage to report in S replies")
	discovery := fs.String("discovery", "", "also answer UDP discovery on this address, such as 127.0.0.1:23273, as a Cube that is not configured (serve finds it with -discover-target <that address>)")
	discoverySerial := fs.String("discovery-serial", "KEQ0000000", "serial of the Cube that answers -discovery")
	pair := fs.String("pair", "", "a device that joins pairing mode, <template rf>,<rf>,<serial>[,<delay>]: a copy of the greeting's device <template rf> in no room, joining <delay> (default 2s) after n:, again after a t: deleted it")
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
	if *pair != "" {
		if srv.Pair, err = parsePair(greeting, *pair); err != nil {
			return fmt.Errorf("fakecube: -pair: %w", err)
		}
	}
	addr, err := srv.Start(*listen)
	if err != nil {
		return err
	}
	defer srv.Close()
	fmt.Fprintf(os.Stderr, "fake cube listening on %s (%d lines)\n", addr, len(greeting))
	if *discovery != "" {
		d, err := fakecube.StartDiscovery(*discovery, *discoverySerial, 0x0a0b0c)
		if err != nil {
			return fmt.Errorf("fakecube: -discovery: %w", err)
		}
		defer d.Close()
		fmt.Fprintf(os.Stderr, "answering discovery on %s as %s\n", d.Addr(), *discoverySerial)
	}
	if srv.Pair != nil {
		fmt.Fprintf(os.Stderr, "a %s joins pairing as %s (%s) %s after n:\n", srv.Pair.Device.Type, srv.Pair.Device.RFAddress, srv.Pair.Device.Serial, srv.Pair.Delay)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	<-ctx.Done()
	return nil
}

// parsePair reads -pair, <template rf>,<rf>,<serial>[,<delay>], into the
// device that joins: a copy of the greeting's device <template rf>.
func parsePair(greeting []cube.Message, v string) (*fakecube.Pairing, error) {
	f := strings.Split(v, ",")
	if len(f) < 3 || len(f) > 4 {
		return nil, fmt.Errorf("want <template rf>,<rf>,<serial>[,<delay>], got %q", v)
	}
	template, err := cube.ParseRFAddress(f[0])
	if err != nil {
		return nil, err
	}
	rf, err := cube.ParseRFAddress(f[1])
	if err != nil {
		return nil, err
	}
	serial := strings.TrimSpace(f[2])
	if serial == "" || len(serial) > 10 {
		return nil, fmt.Errorf("serial %q: 1 to 10 characters", serial)
	}
	delay := 2 * time.Second
	if len(f) == 4 {
		if delay, err = time.ParseDuration(strings.TrimSpace(f[3])); err != nil || delay < 0 {
			return nil, fmt.Errorf("delay %q: a duration such as 2s", f[3])
		}
	}
	return fakecube.PairingFrom(greeting, template, rf, serial, delay)
}
