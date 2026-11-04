// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"sort"
	"strings"

	"go.metaworx.ch/massimo/internal/backup"
	"go.metaworx.ch/massimo/internal/cubelan"
	"go.metaworx.ch/massimo/internal/protocol/cube"
)

// statusDevice is one line of `massimo status -json`.
type statusDevice struct {
	RFAddress string          `json:"rf_address"`
	Type      cube.DeviceType `json:"type"`
	Name      string          `json:"name,omitempty"`
	Serial    string          `json:"serial,omitempty"`
	RoomID    uint8           `json:"room_id"`
	Room      string          `json:"room,omitempty"`
	Unlisted  bool            `json:"unlisted,omitempty"`
	Reported  bool            `json:"reported"`
	State     *backup.State   `json:"state,omitempty"`
}

type statusRoom struct {
	ID      uint8          `json:"id"`
	Name    string         `json:"name"`
	Ready   bool           `json:"ready"`
	Devices []statusDevice `json:"devices"`
}

type statusReport struct {
	Serial          string `json:"serial"`
	Firmware        string `json:"firmware"`
	DutyCyclePct    int    `json:"duty_cycle_pct"`
	FreeMemorySlots int    `json:"free_memory_slots"`
	// Pending is the number of radiograms the Cube has sent and not yet
	// seen acknowledged (memory slots in use); their effect is not in
	// the states yet.
	Pending  int            `json:"pending_radiograms"`
	CubeTime string         `json:"cube_time"`
	Rooms    []statusRoom   `json:"rooms"`
	Unlisted []statusDevice `json:"unlisted"`
}

// runStatus connects to a Cube and prints what it knows about every
// device: which ones have reported since the Cube's last restart, their
// mode, setpoint, temperatures and flags, and which rooms accept
// commands.
func runStatus(args []string) error {
	fs := flag.NewFlagSet("status", flag.ContinueOnError)
	addr := fs.String("addr", "", "Cube address, host[:port]")
	asJSON := fs.Bool("json", false, "print JSON instead of a table")
	snFlag(fs)
	fs.Usage = func() {
		fmt.Fprint(os.Stderr, "usage: massimo status [-addr <ip>] [-json]\n\nShow every device the Cube knows, its last reported state and whether the\nCube holds state for it at all. After a Cube restart devices show up as\n\"not reported\" until they transmit again; the Cube discards room commands\nwhile a room has such a member.\n\n")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := defaultAddr(addr, "status"); err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	c, err := connectCube(ctx, *addr)
	if err != nil {
		return err
	}
	useDeviceSerial(c.Snapshot().Hello().Serial)
	ignore, err := ignoredDevices()
	if err != nil {
		return err
	}
	rep := statusFromSnapshot(c.Snapshot(), ignore)
	if *asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(rep)
	}
	printStatus(rep)
	return nil
}

// statusFromSnapshot builds the report; ignored devices and the rooms
// that hold only them are left out.
func statusFromSnapshot(snap *cubelan.Snapshot, ignore []cube.RFAddress) statusReport {
	var rep statusReport
	ig := backup.IgnoreSet(ignore)
	h := snap.Hello()
	if h != nil {
		rep.Serial, rep.Firmware = h.Serial, h.FirmwareVersion
		rep.DutyCyclePct, rep.FreeMemorySlots = h.DutyCycle, h.FreeMemorySlots
		rep.Pending = cubelan.Budget{FreeMemorySlots: h.FreeMemorySlots}.Pending()
		rep.CubeTime = strings.TrimSpace(h.CubeDate + " " + h.CubeTime)
	}
	md := snap.Metadata()
	roomName := map[uint8]string{}
	byRoom := map[uint8][]statusDevice{}
	if md != nil {
		for _, r := range md.Rooms {
			roomName[r.ID] = r.Name
		}
	}
	hidden := ig.HiddenRooms(md)
	for _, rf := range snap.Devices() {
		if h != nil && rf == h.RFAddress || ig[rf] {
			continue
		}
		d := statusDevice{RFAddress: rf.String(), Unlisted: true}
		if md != nil {
			for _, m := range md.Devices {
				if m.RFAddress == rf {
					d.Name, d.RoomID, d.Serial, d.Type, d.Unlisted = m.Name, m.RoomID, m.Serial, m.Type, false
				}
			}
		}
		if c := snap.Config(rf); c != nil {
			d.Type = c.Type
			if d.Serial == "" {
				d.Serial, d.RoomID = c.Serial, c.RoomID
			}
		}
		d.Room = roomName[d.RoomID]
		if st, ok := snap.State(rf); ok && st.Valid {
			d.Reported = true
			d.State = &backup.State{Mode: st.Mode, Setpoint: st.Setpoint, Until: st.Until, ActualTemp: st.ActualTemp, ValvePct: st.ValvePct,
				WindowOpen: st.WindowOpen, BatteryLow: st.BatteryLow, Error: st.Error, LinkError: st.LinkError, PanelLocked: st.PanelLocked}
		}
		if d.Unlisted {
			rep.Unlisted = append(rep.Unlisted, d)
		} else {
			byRoom[d.RoomID] = append(byRoom[d.RoomID], d)
		}
	}
	if md != nil {
		for _, r := range md.Rooms {
			if hidden[r.ID] {
				continue
			}
			sr := statusRoom{ID: r.ID, Name: r.Name, Ready: true, Devices: byRoom[r.ID]}
			if sr.Devices == nil {
				sr.Devices = []statusDevice{}
			}
			for _, d := range sr.Devices {
				sr.Ready = sr.Ready && d.Reported
			}
			rep.Rooms = append(rep.Rooms, sr)
			delete(byRoom, r.ID)
		}
	}
	// Devices listed with a room the metadata does not have.
	var orphans []uint8
	for id := range byRoom {
		orphans = append(orphans, id)
	}
	sort.Slice(orphans, func(i, j int) bool { return orphans[i] < orphans[j] })
	for _, id := range orphans {
		rep.Unlisted = append(rep.Unlisted, byRoom[id]...)
	}
	if rep.Rooms == nil {
		rep.Rooms = []statusRoom{}
	}
	if rep.Unlisted == nil {
		rep.Unlisted = []statusDevice{}
	}
	return rep
}

func printStatus(rep statusReport) {
	fmt.Printf("cube %s fw %s, duty cycle %d%%, %d free slots, clock %s\n", rep.Serial, rep.Firmware, rep.DutyCyclePct, rep.FreeMemorySlots, rep.CubeTime)
	if rep.Pending > 0 {
		fmt.Printf("%d radiogram(s) sent but not yet acknowledged by a device; the states above may still change\n", rep.Pending)
	}
	for _, r := range rep.Rooms {
		ready := "ready"
		if !r.Ready {
			ready = "NOT READY, room commands are discarded until every device has reported"
		}
		fmt.Printf("\nroom %d %s (%s)\n", r.ID, r.Name, ready)
		for _, d := range r.Devices {
			fmt.Println("  " + statusLine(d))
		}
	}
	if len(rep.Unlisted) > 0 {
		fmt.Printf("\nnot in any room\n")
		for _, d := range rep.Unlisted {
			fmt.Println("  " + statusLine(d))
		}
	}
}

func statusLine(d statusDevice) string {
	name := d.Name
	if name == "" {
		name = "(unnamed)"
	}
	line := fmt.Sprintf("%s %-24s %-24s", d.RFAddress, d.Type, name)
	if !d.Reported {
		return line + " not reported since the cube restarted"
	}
	st := d.State
	switch {
	case st.WindowOpen != nil:
		if *st.WindowOpen {
			line += " window open"
		} else {
			line += " window closed"
		}
	case st.Setpoint != nil:
		line += fmt.Sprintf(" %s set=%.1f°C", st.Mode, *st.Setpoint)
		if st.ActualTemp != nil {
			line += fmt.Sprintf(" actual=%.1f°C", *st.ActualTemp)
		}
		if st.ValvePct != nil {
			line += fmt.Sprintf(" valve=%d%%", *st.ValvePct)
		}
		if st.Until != nil {
			line += " until " + st.Until.String()
		}
	default:
		line += " reported"
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
	if st.PanelLocked {
		line += " LOCKED"
	}
	return line
}
