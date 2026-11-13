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
	fixture := fs.String("fixture", "", "serve a built-in house instead of -greeting: rooms (a wall thermostat with two radiator thermostats, three radiator thermostats, a shutter contact alone)")
	apply := fs.Bool("apply", true, "carry out the radio commands acknowledged: setpoints, modes, boosts, programmes, presets and locks change the devices' lines (false: acknowledge only)")
	applyAfter := fs.Duration("apply-after", 0, "apply a command this long after acknowledging it, as a device confirms it after its wakeup")
	boostFor := fs.Duration("boost-for", 0, "end a boost after this long instead of the device's boost time")
	control := fs.String("control", "", "serve the control on this loopback address, such as 127.0.0.1:62911: set a device's state, remove it from its room and restore it, drop the connections, go silent, release a dead session (see the README)")
	singleClient := fs.Bool("single-client", false, "serve one client at a time, as the Cube does: while a client is connected, or a dead session is held, another client's dial times out (on Linux; elsewhere its connection is closed at once)")
	holdDead := fs.Duration("hold-dead", fakecube.DefaultHoldDead, "with -single-client, how long a session that ended while the fake was silent (the control's POST /silent) is held, as the Cube holds one whose end an outage lost; an R to -discovery frees it at once")
	pair := fs.String("pair", "", "a device that joins pairing mode, <template rf>,<rf>,<serial>[,<delay>]: a copy of the greeting's device <template rf> in no room, joining <delay> (default 2s) after n:, again after a t: deleted it")
	fs.Usage = func() {
		fmt.Fprint(os.Stderr, "usage: massimo fakecube -greeting <file> | -fixture <name> [-listen 127.0.0.1:62910]\n\nBehave like a Cube's LAN interface, using recorded data.\n\n")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return err
	}
	var greeting []cube.Message
	var err error
	switch {
	case (*file == "") == (*fixture == ""):
		return fmt.Errorf("fakecube: one of -greeting and -fixture is required")
	case *fixture != "":
		greeting, err = fakecube.LoadFixture(*fixture)
	default:
		greeting, err = fakecube.LoadGreeting(*file)
	}
	if err != nil {
		return err
	}
	srv := &fakecube.Server{Greeting: greeting, Duty: *duty, Apply: *apply, ApplyAfter: *applyAfter, BoostFor: *boostFor,
		SingleClient: *singleClient, HoldDead: *holdDead}
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
	if *control != "" {
		caddr, err := srv.Control(*control)
		if err != nil {
			return err
		}
		fmt.Fprintf(os.Stderr, "control listening on %s\n", caddr)
	}
	if *discovery != "" {
		d, err := fakecube.StartDiscovery(*discovery, *discoverySerial, 0x0a0b0c)
		if err != nil {
			return fmt.Errorf("fakecube: -discovery: %w", err)
		}
		defer d.Close()
		d.SetOnRestart(srv.Restart) // an R ends the TCP sessions too
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
