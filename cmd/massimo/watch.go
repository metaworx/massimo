// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"time"

	"go.metaworx.ch/massimo/internal/cubelan"
	"go.metaworx.ch/massimo/internal/protocol/cube"
)

// runWatch keeps a persistent connection to a Cube and prints state
// changes as they happen. It is the first real use of the LAN driver.
func runWatch(args []string) error {
	fs := flag.NewFlagSet("watch", flag.ContinueOnError)
	addr := fs.String("addr", "", "Cube address, host[:port]")
	poll := fs.Duration("poll", 60*time.Second, "device list poll interval (0 = never)")
	fs.Usage = func() {
		fmt.Fprint(os.Stderr, "usage: massimo watch -addr <ip> [-poll 60s]\n\nStay connected to a Cube and print state changes.\n\n")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *addr == "" {
		return fmt.Errorf("watch: -addr is required")
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	c := cubelan.New(cubelan.Options{Addr: *addr, PollInterval: *poll, Logger: logger})
	go c.Run(ctx)

	snap := c.Snapshot()
	names := map[cube.RFAddress]string{}
	for {
		select {
		case <-ctx.Done():
			return nil
		case e := <-c.Events():
			switch e.Kind {
			case EventConnectedKind:
				md := snap.Metadata()
				for _, d := range md.Devices {
					names[d.RFAddress] = d.Name
				}
				h := snap.Hello()
				fmt.Printf("%s connected: %s fw %s, duty cycle %d%%, %d free slots, %d rooms, %d devices\n",
					stamp(), h.Serial, h.FirmwareVersion, h.DutyCycle, h.FreeMemorySlots, len(md.Rooms), len(md.Devices))
				for _, rf := range snap.Devices() {
					printState(snap, rf, names[rf])
				}
			case cubelan.EventDisconnected:
				fmt.Printf("%s disconnected: %v\n", stamp(), e.Err)
			case cubelan.EventHello:
				h := snap.Hello()
				fmt.Printf("%s cube: duty cycle %d%%, %d free slots, clock %s %s\n", stamp(), h.DutyCycle, h.FreeMemorySlots, h.CubeDate, h.CubeTime)
			case cubelan.EventState:
				printState(snap, e.Device, names[e.Device])
			}
		}
	}
}

// EventConnectedKind is cubelan.EventConnected, aliased to keep the switch readable.
const EventConnectedKind = cubelan.EventConnected

func stamp() string { return time.Now().Format("15:04:05") }

func printState(s *cubelan.Snapshot, rf cube.RFAddress, name string) {
	st, ok := s.State(rf)
	if !ok {
		return
	}
	line := fmt.Sprintf("%s %s %-28s", stamp(), rf, name)
	switch {
	case !st.Valid:
		line += " no data yet"
	case st.WindowOpen != nil:
		line += fmt.Sprintf(" window open=%v", *st.WindowOpen)
	default:
		line += fmt.Sprintf(" mode=%s", st.Mode)
		if st.Setpoint != nil {
			line += fmt.Sprintf(" set=%.1f°C", *st.Setpoint)
		}
		if st.ActualTemp != nil {
			line += fmt.Sprintf(" actual=%.1f°C", *st.ActualTemp)
		}
		if st.ValvePct != nil {
			line += fmt.Sprintf(" valve=%d%%", *st.ValvePct)
		}
	}
	if st.Error {
		line += " ERROR"
	}
	if st.LinkError {
		line += " link-error"
	}
	if st.BatteryLow {
		line += " BATTERY-LOW"
	}
	fmt.Println(line)
}
