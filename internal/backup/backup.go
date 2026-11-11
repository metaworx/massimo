// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

// Package backup saves everything a Cube knows about its installation and
// plans how to write a saved configuration back.
//
// A backup file has two parts: the raw protocol lines of the greeting (H,
// M, C…, L), which are lossless and let the data be re-decoded by newer
// versions of this software, and a decoded, human-readable copy (rooms,
// device names, thermostat settings, week programs, NTP servers, time zone
// rules) that can be edited by hand before it is restored.
package backup

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"go.metaworx.ch/massimo/internal/cubelan"
	"go.metaworx.ch/massimo/internal/protocol/cube"
	"go.metaworx.ch/massimo/internal/safefile"
	"go.metaworx.ch/massimo/internal/view"
)

// Version is the format version written into new files.
const Version = 1

// File is one backup of one Cube.
type File struct {
	Version int       `json:"version"`
	TakenAt time.Time `json:"taken_at"`
	Tool    string    `json:"tool,omitempty"` // software that wrote the file

	Cube CubeInfo `json:"cube"`
	// NTP servers (F reply); empty when they were not requested.
	NTP []string `json:"ntp,omitempty"`
	// CubeConfig is the Cube's own configuration (portal, time zone).
	CubeConfig *cube.CubeConfig `json:"cube_config,omitempty"`
	Rooms      []cube.RoomMeta  `json:"rooms"`
	Devices    []Device         `json:"devices"`

	// Lines are the raw protocol lines (H, M, C…, L) as the Cube sent
	// them, without the trailing CRLF. They are the authoritative record.
	Lines []string `json:"lines"`
}

// CubeInfo identifies the Cube the backup was taken from.
type CubeInfo struct {
	Serial    string         `json:"serial"`
	RFAddress cube.RFAddress `json:"rf_address"`
	Firmware  string         `json:"firmware"` // e.g. "1.1.3"
}

// Device is one paired device with its name, room and configuration.
type Device struct {
	RFAddress cube.RFAddress  `json:"rf_address"`
	Type      cube.DeviceType `json:"type"`
	Serial    string          `json:"serial"`
	Name      string          `json:"name"`
	RoomID    uint8           `json:"room_id"`
	Firmware  uint8           `json:"firmware,omitempty"`
	// Unlisted marks a device that is paired but was never assigned to a
	// room: the Cube holds its configuration but it is not in the room
	// and device list, and the original software offers it as a "new
	// device". A restore leaves such devices alone unless the backup
	// gives them a name.
	Unlisted bool `json:"unlisted,omitempty"`

	Thermostat *cube.ThermostatConfig `json:"thermostat,omitempty"`
	Wall       *cube.WallConfig       `json:"wall_thermostat,omitempty"`

	// State is what the device reported last (the L line): mode,
	// setpoint, measured temperature, valve, flags. Informational, except
	// that a restore with the State option re-applies mode and setpoint.
	State *view.State `json:"state,omitempty"`
}

// Take builds a backup from the client's current snapshot. ntp may be nil
// when the servers were not requested; tool names the software version.
// Hidden devices and the rooms that hold only them are left out of the
// rooms and devices sections; the raw lines keep everything.
func Take(snap *cubelan.Snapshot, ntp []string, tool string, now time.Time, hidden view.Hidden) (*File, error) {
	h := snap.Hello()
	if h == nil {
		return nil, fmt.Errorf("backup: no greeting from the cube yet")
	}
	f := &File{
		Version: Version,
		TakenAt: now.Truncate(time.Second),
		Tool:    tool,
		Cube:    CubeInfo{Serial: h.Serial, RFAddress: h.RFAddress, Firmware: h.FirmwareVersion},
		NTP:     append([]string(nil), ntp...),
		Rooms:   []cube.RoomMeta{},
		Devices: []Device{},
	}
	if c := snap.Config(h.RFAddress); c != nil && c.Cube != nil {
		cc := *c.Cube
		f.CubeConfig = &cc
	}
	md := snap.Metadata()
	f.Rooms = append(f.Rooms, hidden.VisibleRooms(md)...)
	// Devices in the order of the Cube's own list, then any the Cube only
	// sent a configuration for.
	meta := map[cube.RFAddress]cube.DeviceMeta{}
	var order []cube.RFAddress
	if md != nil {
		for _, d := range md.Devices {
			meta[d.RFAddress] = d
			order = append(order, d.RFAddress)
		}
	}
	for _, rf := range snap.Devices() {
		if _, ok := meta[rf]; !ok {
			order = append(order, rf)
		}
	}
	for _, rf := range order {
		if rf == h.RFAddress || hidden[rf] {
			continue
		}
		c := snap.Config(rf)
		m, hasMeta := meta[rf]
		if c == nil && !hasMeta {
			continue
		}
		d := Device{RFAddress: rf, Unlisted: !hasMeta}
		if hasMeta {
			d.Type, d.Serial, d.Name, d.RoomID = m.Type, m.Serial, m.Name, m.RoomID
		}
		if c != nil {
			d.Type, d.Firmware = c.Type, c.Firmware
			if c.Thermostat != nil {
				tc := *c.Thermostat
				tc.WeekProgram = cloneWeekProgram(tc.WeekProgram)
				d.Thermostat = &tc
			}
			if c.Wall != nil {
				wc := *c.Wall
				wc.WeekProgram = cloneWeekProgram(wc.WeekProgram)
				wc.Extra = append([]byte(nil), wc.Extra...)
				d.Wall = &wc
			}
			if !hasMeta {
				d.Serial, d.RoomID = c.Serial, c.RoomID
			}
		}
		if st, ok := snap.State(rf); ok {
			d.State = view.StateOf(st)
			if st.Valid && d.Wall != nil {
				d.Wall.DisplayActual = st.DisplayActual
			}
		}
		f.Devices = append(f.Devices, d)
	}
	// A fixed order makes files comparable; the Cube's own order is not.
	sort.Slice(f.Rooms, func(i, j int) bool { return f.Rooms[i].ID < f.Rooms[j].ID })
	sort.Slice(f.Devices, func(i, j int) bool { return f.Devices[i].RFAddress < f.Devices[j].RFAddress })
	for _, m := range snap.Lines() {
		f.Lines = append(f.Lines, m.String())
	}
	return f, nil
}

func cloneWeekProgram(w cube.WeekProgram) cube.WeekProgram {
	for i := range w {
		w[i] = append(cube.DayProgram(nil), w[i]...)
	}
	return w
}

// Metadata rebuilds the Cube's room and device list from the decoded part.
func (f *File) Metadata() *cube.Metadata {
	md := &cube.Metadata{Rooms: append([]cube.RoomMeta(nil), f.Rooms...)}
	for _, d := range f.Devices {
		md.Devices = append(md.Devices, cube.DeviceMeta{Type: d.Type, RFAddress: d.RFAddress, Serial: d.Serial, Name: d.Name, RoomID: d.RoomID})
	}
	return md
}

// Device returns the device with the given address.
func (f *File) Device(rf cube.RFAddress) (Device, bool) {
	for _, d := range f.Devices {
		if d.RFAddress == rf {
			return d, true
		}
	}
	return Device{}, false
}

// RoomName returns the name of a room, or "" when unknown.
func (f *File) RoomName(id uint8) string {
	for _, r := range f.Rooms {
		if r.ID == id {
			return r.Name
		}
	}
	return ""
}

// ConfigHash is the SHA-256, hex, of the Cube's configuration in the
// file: its own configuration, the rooms and the devices without their
// live state, and without the wall thermostat's display choice, which
// the file takes from the live state. The time, the tool, the NTP servers
// and the raw lines are left out, so that two backups of a Cube whose
// configuration did not change have the same hash: the server takes its
// automatic backup when the hash leaves the newest backup's
// (docs/store.md section 9 (e)).
func ConfigHash(f *File) string {
	devices := make([]Device, len(f.Devices))
	for i, d := range f.Devices {
		d.State = nil
		if d.Wall != nil {
			w := *d.Wall
			w.DisplayActual = nil
			d.Wall = &w
		}
		devices[i] = d
	}
	rooms := f.Rooms
	if rooms == nil {
		rooms = []cube.RoomMeta{}
	}
	data, _ := json.Marshal(struct {
		CubeConfig *cube.CubeConfig `json:"cube_config"`
		Rooms      []cube.RoomMeta  `json:"rooms"`
		Devices    []Device         `json:"devices"`
	}{f.CubeConfig, rooms, devices})
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// Messages re-parses the raw lines.
func (f *File) Messages() ([]cube.Message, error) {
	return cube.SplitLines(strings.Join(f.Lines, "\r\n") + "\r\n")
}

// DefaultName is the file name used when none is given.
func (f *File) DefaultName(ext string) string {
	return fmt.Sprintf("massimo-backup-%s-%s.%s", f.Cube.Serial, f.TakenAt.Format("20060102-150405"), ext)
}

// IsYAML reports whether a path should be read or written as YAML.
func IsYAML(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".yaml", ".yml":
		return true
	}
	return false
}

// Marshal encodes the file as JSON, or as YAML when yamlFormat is set. The
// YAML uses the same field names as the JSON.
func (f *File) Marshal(yamlFormat bool) ([]byte, error) {
	if !yamlFormat {
		return json.MarshalIndent(f, "", "  ")
	}
	js, err := json.Marshal(f)
	if err != nil {
		return nil, err
	}
	return jsonToYAML(js)
}

// Unmarshal decodes JSON or YAML.
func Unmarshal(data []byte, yamlFormat bool) (*File, error) {
	if yamlFormat {
		var generic any
		if err := yaml.Unmarshal(data, &generic); err != nil {
			return nil, err
		}
		js, err := json.Marshal(generic)
		if err != nil {
			return nil, err
		}
		data = js
	}
	f := &File{}
	if err := json.Unmarshal(data, f); err != nil {
		return nil, err
	}
	if f.Version > Version {
		return nil, fmt.Errorf("backup: file version %d is newer than this software supports (%d)", f.Version, Version)
	}
	return f, nil
}

// Save writes the file, whole (internal/safefile): a backup cut short by
// a kill or a full disk never replaces a good one. A new file gets mode
// 0600, an existing one keeps its own. The format follows the extension
// (.yaml/.yml or JSON).
func (f *File) Save(path string) error {
	data, err := f.Marshal(IsYAML(path))
	if err != nil {
		return err
	}
	return safefile.Write(path, data, safefile.Options{})
}

// Load reads a backup; the format follows the extension.
func Load(path string) (*File, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	f, err := Unmarshal(data, IsYAML(path))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return f, nil
}
