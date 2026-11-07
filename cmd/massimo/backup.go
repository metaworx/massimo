// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"time"

	"go.metaworx.ch/massimo/internal/backup"
	"go.metaworx.ch/massimo/internal/cubeio"
	"go.metaworx.ch/massimo/internal/cubelan"
	"go.metaworx.ch/massimo/internal/engine"
	"go.metaworx.ch/massimo/internal/protocol/cube"
)

// connectCube connects to a Cube and waits for the greeting.
func connectCube(ctx context.Context, addr string) (*cubelan.Client, error) {
	logger, err := newLogger(os.Stderr)
	if err != nil {
		return nil, err
	}
	c := cubelan.New(cubelan.Options{Addr: addr, Logger: logger})
	go c.Run(ctx)
	select {
	case <-c.Ready():
		return c, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(30 * time.Second):
		return nil, fmt.Errorf("no greeting from %s within 30 s", addr)
	}
}

// fetchNTP asks the Cube for its NTP servers.
func fetchNTP(ctx context.Context, c *cubelan.Client) ([]string, error) {
	reply, err := c.Send(ctx, cube.CmdGetNTP)
	if err != nil {
		return nil, err
	}
	return cube.ParseNTPServers(reply.Data), nil
}

// runBackup saves everything the Cube reports into a file.
func runBackup(args []string) error {
	fs := flag.NewFlagSet("backup", flag.ContinueOnError)
	addr := fs.String("addr", "", "Cube address, host[:port]")
	out := fs.String("out", "", "output file; .yaml/.yml writes YAML, anything else JSON (default: massimo-backup-<serial>-<time>.json)")
	yamlOut := fs.Bool("yaml", false, "write YAML instead of JSON when -out is not given")
	snFlag(fs)
	fs.Usage = func() {
		fmt.Fprint(os.Stderr, "usage: massimo backup -addr <ip> [-out file]\n\nSave the Cube's rooms, device names, thermostat settings, week programs,\nNTP servers and time zone rules, together with the raw protocol lines.\n\n")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := defaultAddr(addr, "backup"); err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	c, err := connectCube(ctx, *addr)
	if err != nil {
		return err
	}
	ntp, err := fetchNTP(ctx, c)
	if err != nil {
		fmt.Fprintln(os.Stderr, "warning: NTP servers not read:", err)
	}
	useDeviceSerial(c.Snapshot().Hello().Serial)
	hidden, err := hiddenSet(c.Snapshot())
	if err != nil {
		return err
	}
	f, err := backup.Take(c.Snapshot(), ntp, versionString(), time.Now(), hidden)
	if err != nil {
		return err
	}
	path := *out
	if path == "" {
		ext := "json"
		if *yamlOut {
			ext = "yaml"
		}
		path = f.DefaultName(ext)
	}
	if err := f.Save(path); err != nil {
		return err
	}
	fmt.Printf("saved %s: cube %s fw %s, %d rooms, %d devices, %d lines\n", path, f.Cube.Serial, f.Cube.Firmware, len(f.Rooms), len(f.Devices), len(f.Lines))
	stop()
	return nil
}

// runRestore compares a backup with the Cube and writes the differences.
func runRestore(args []string) error {
	fs := flag.NewFlagSet("restore", flag.ContinueOnError)
	addr := fs.String("addr", "", "Cube address, host[:port]")
	from := fs.String("from", "", "backup file (JSON or YAML)")
	dry := fs.Bool("dry-run", false, "only print what would be sent")
	yes := fs.Bool("yes", false, "do not ask before sending")
	portal := fs.Bool("portal", false, "also restore the portal URL and whether the Cube polls it")
	state := fs.Bool("state", false, "also re-apply each thermostat's mode and setpoint from the backup's state section")
	skip := fs.String("skip", "", "comma-separated groups to leave alone: names, settings, programs, clock")
	gap := fs.Duration("gap", 200*time.Millisecond, "pause between commands")
	snFlag(fs)
	fs.Usage = func() {
		fmt.Fprint(os.Stderr, "usage: massimo restore -addr <ip> -from <file> [-dry-run] [-yes] [-portal] [-state] [-skip groups]\n\nWrite a backup back to the Cube. Only settings that differ are sent.\nDevices in the backup that are not paired with the Cube are reported;\npairing cannot be restored over the LAN.\n\n")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := defaultAddr(addr, "restore"); err != nil {
		return err
	}
	if *from == "" {
		return fmt.Errorf("restore: -from is required")
	}
	opts := backup.PlanOptions{Portal: *portal, State: *state}
	for _, g := range strings.Split(*skip, ",") {
		switch strings.TrimSpace(g) {
		case "":
		case "names":
			opts.SkipNames = true
		case "settings":
			opts.SkipSettings = true
		case "programs":
			opts.SkipPrograms = true
		case "clock":
			opts.SkipClock = true
		default:
			return fmt.Errorf("restore: unknown -skip group %q", g)
		}
	}
	f, err := backup.Load(*from)
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	c, err := connectCube(ctx, *addr)
	if err != nil {
		return err
	}
	useDeviceSerial(c.Snapshot().Hello().Serial)
	if opts.Hidden, err = hiddenDevices(); err != nil {
		return err
	}
	opts.ShowHidden = showHidden
	cur := backup.Current{Snapshot: c.Snapshot()}
	if !opts.SkipClock {
		if cur.NTP, err = fetchNTP(ctx, c); err != nil {
			fmt.Fprintln(os.Stderr, "warning: NTP servers not read:", err)
		}
	}
	p, err := backup.NewPlan(f, cur, opts)
	if err != nil {
		return err
	}
	h := c.Snapshot().Hello()
	fmt.Printf("backup from %s taken %s; cube %s fw %s, duty cycle %d%%, %d free slots\n",
		f.Cube.Serial, f.TakenAt.Format(time.RFC3339), h.Serial, h.FirmwareVersion, h.DutyCycle, h.FreeMemorySlots)
	for _, n := range p.Notes {
		fmt.Println("note:", n)
	}
	if p.Empty() {
		fmt.Println("nothing to do: the cube matches the backup")
		return nil
	}
	radio := 0
	for i, s := range p.Steps {
		fmt.Printf("%2d. %s\n", i+1, s.What)
		for _, l := range s.Command.Lines {
			if l.Type == 's' {
				radio++
			}
		}
	}
	fmt.Printf("%d steps, %d radio commands\n", len(p.Steps), radio)
	if *dry {
		return nil
	}
	if !*yes {
		fmt.Fprint(os.Stderr, "send these to the cube? [y/N] ")
		line, _ := bufio.NewReader(os.Stdin).ReadString('\n')
		if a := strings.ToLower(strings.TrimSpace(line)); a != "y" && a != "yes" {
			return fmt.Errorf("restore: cancelled")
		}
	}
	logger, err := newLogger(os.Stderr)
	if err != nil {
		return err
	}
	q := engine.NewQueue(c, engine.Options{Gap: *gap, Logger: logger})
	go q.Run(ctx)
	results, err := p.Run(ctx, cubeio.Writer{Client: c, Queue: q}, func(i int, s backup.Step, r engine.Result) {
		status := fmt.Sprintf("ok (duty cycle %d%%, %d free slots)", r.Budget.DutyCyclePct, r.Budget.FreeMemorySlots)
		if r.Err != nil {
			status = "failed: " + r.Err.Error()
		}
		fmt.Printf("%2d. %s: %s\n", i+1, s.What, status)
	})
	if err != nil {
		return err
	}
	fmt.Printf("done: %d steps sent\n", len(results))
	return nil
}
