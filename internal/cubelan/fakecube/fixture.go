// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package fakecube

import (
	"encoding/base64"
	"fmt"
	"strings"

	"go.metaworx.ch/massimo/internal/protocol/cube"
)

// Fixtures are greetings built for tests, beside the recorded ones: the
// house a test needs, from device lines recorded on hardware with other
// addresses, serials and rooms. Every address and serial is a placeholder.

// Fixtures names the fixtures `massimo fakecube -fixture` serves.
var Fixtures = map[string]func() ([]cube.Message, error){
	"rooms": RoomsFixture,
}

// Recorded lines the fixtures copy (internal/protocol/cube/testdata and
// the tests of the protocol package): the Cube's greeting header and its
// own configuration, a radiator thermostat's and a wall thermostat's.
const (
	fixtureHello    = "KEQ0000100,0a0107,0113,00000000,14cf200c,00,32,1a0b03,1333,03,0000"
	fixtureCubeC    = "0a0107,7QoBBwATAf9LRVEwMDAwMTAwAAsABEAAAAAAAAAAAP///////////////////////////wsABEAAAAAAAAAAQf///////////////////////////2h0dHA6Ly9tYXguZXEtMy5kZTo4MC9jdWJlADAvbG9va3VwAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAEdNVAAACgADAAAAAEJTVAAAAwACAAAOEA=="
	fixtureRadiator = "0goBCgEBEKBNRVEwMDAwMTAxLB49CQcKAzAM/wBETlSGRSBFIEUgRSBFIEUgRSBFIEUgRSBFIEROVIZFIEUgRSBFIEUgRSBFIEUgRSBFIEUgMNxU80UKMSAxIDEgMSBFIEUgRSBFIEUgRSAw3FTzRQoxIDEgMSAxIEUgRSBFIEUgRSBFIDDcVPNFCjEgMSAxIDEgRSBFIEUgRSBFIEUgMNxU80UKMSAxIDEgMSBFIEUgRSBFIEUgRSAw3FTzRQoxIDEgMSAxIEUgRSBFIEUgRSBFIA=="
	fixtureWall     = "zgoBDgMAEP9ORVEwMDAwNTA3KiI9CURIVQhFIEUgRSBFIEUgRSBFIEUgRSBFIEUgREhVCEUgRSBFIEUgRSBFIEUgRSBFIEUgRSBESFRsRMxVFEUgRSBFIEUgRSBFIEUgRSBFIERIVGxEzFUURSBFIEUgRSBFIEUgRSBFIEUgREhUbETMVRRFIEUgRSBFIEUgRSBFIEUgRSBESFRsRMxVFEUgRSBFIEUgRSBFIEUgRSBFIERIVGxEzFUURSBFIEUgRSBFIEUgRSBFIEUgBxgw"
)

// fixtureDevice is one device of a fixture with the state it reports.
type fixtureDevice struct {
	rf       cube.RFAddress
	typ      cube.DeviceType
	serial   string
	name     string
	room     uint8
	setpoint float64
	actual   float64 // 0: none reported
	valve    int
	open     bool // a shutter contact's window
}

// RoomsFixture is the house the room thermostat's tests need:
//
//	room 1 "Wohnen":  a wall thermostat at 21.4 °C and two radiator
//	                  thermostats at 20.5 and 22.0 °C (the wall
//	                  thermostat's value is the room's; it can be removed
//	                  and restored to test that the room stays)
//	room 2 "Schlafen": three radiator thermostats at 19.0, 19.5 and
//	                  23.5 °C (median 19.5, mean 20.7)
//	room 3 "Flur":    a shutter contact alone, closed
//
// Every thermostat is in auto at 20 °C. A test makes one stale with the
// server's stale threshold, and boosts one with a setpoint command.
func RoomsFixture() ([]cube.Message, error) {
	rooms := []cube.RoomMeta{{ID: 1, Name: "Wohnen"}, {ID: 2, Name: "Schlafen"}, {ID: 3, Name: "Flur"}}
	devices := []fixtureDevice{
		{rf: 0x0a0201, typ: cube.DeviceWallThermostat, serial: "NEQ0000201", name: "Wandthermostat Wohnen", room: 1, setpoint: 20, actual: 21.4},
		{rf: 0x0a0202, typ: cube.DeviceRadiatorThermostat, serial: "MEQ0000202", name: "Heizung Wohnen Fenster", room: 1, setpoint: 20, actual: 20.5},
		{rf: 0x0a0203, typ: cube.DeviceRadiatorThermostat, serial: "MEQ0000203", name: "Heizung Wohnen Tür", room: 1, setpoint: 20, actual: 22.0, valve: 15},
		{rf: 0x0a0204, typ: cube.DeviceRadiatorThermostat, serial: "MEQ0000204", name: "Heizung Schlafen 1", room: 2, setpoint: 20, actual: 19.0},
		{rf: 0x0a0205, typ: cube.DeviceRadiatorThermostat, serial: "MEQ0000205", name: "Heizung Schlafen 2", room: 2, setpoint: 20, actual: 19.5},
		{rf: 0x0a0206, typ: cube.DeviceRadiatorThermostat, serial: "MEQ0000206", name: "Heizung Schlafen 3", room: 2, setpoint: 20, actual: 23.5},
		{rf: 0x0a0207, typ: cube.DeviceShutterContact, serial: "OEQ0000207", name: "Fenster Flur", room: 3},
	}
	return buildFixture(rooms, devices)
}

// buildFixture builds a greeting: H, M, the Cube's C line and each
// device's, and the L line.
func buildFixture(rooms []cube.RoomMeta, devices []fixtureDevice) ([]cube.Message, error) {
	md := &cube.Metadata{}
	for _, r := range rooms {
		for _, d := range devices {
			if d.room == r.ID {
				r.RFAddress = d.rf
				break
			}
		}
		md.Rooms = append(md.Rooms, r)
	}
	out := []cube.Message{{Type: 'H', Data: fixtureHello}}
	var list []byte
	var configs []cube.Message
	for _, d := range devices {
		md.Devices = append(md.Devices, cube.DeviceMeta{Type: d.typ, RFAddress: d.rf, Serial: d.serial, Name: d.name, RoomID: d.room})
		c, err := fixtureConfig(d)
		if err != nil {
			return nil, err
		}
		configs = append(configs, c)
		e, err := fixtureEntry(d)
		if err != nil {
			return nil, err
		}
		list = append(list, e...)
	}
	raw, err := md.Encode()
	if err != nil {
		return nil, err
	}
	out = append(out, cube.Message{Type: 'M', Data: "00,01," + base64.StdEncoding.EncodeToString(raw)})
	out = append(out, cube.Message{Type: 'C', Data: fixtureCubeC})
	out = append(out, configs...)
	return append(out, cube.Message{Type: 'L', Data: base64.StdEncoding.EncodeToString(list)}), nil
}

// fixtureConfig is a device's C line: a copy of the recorded one of its
// type with its address, room and serial.
func fixtureConfig(d fixtureDevice) (cube.Message, error) {
	var raw []byte
	switch {
	case d.typ.IsRadiatorThermostat():
		raw, _ = base64.StdEncoding.DecodeString(fixtureRadiator)
	case d.typ == cube.DeviceWallThermostat:
		raw, _ = base64.StdEncoding.DecodeString(fixtureWall)
	case d.typ == cube.DeviceShutterContact:
		// A shutter contact's C payload is its header alone.
		raw = append([]byte{0x11, 0, 0, 0, byte(d.typ), 0, 0x10, 0xa0}, make([]byte, 10)...)
	default:
		return cube.Message{}, fmt.Errorf("fakecube: no recorded C line for a %s", d.typ)
	}
	a := d.rf.Bytes()
	copy(raw[1:4], a[:])
	raw[5] = d.room
	copy(raw[8:18], fmt.Sprintf("%-10.10s", d.serial))
	return cube.Message{Type: 'C', Data: d.rf.String() + "," + base64.StdEncoding.EncodeToString(raw)}, nil
}

// fixtureEntry is a device's L entry, reported, in auto mode, as the
// recorded entries are laid out.
func fixtureEntry(d fixtureDevice) ([]byte, error) {
	a := d.rf.Bytes()
	var e []byte
	switch {
	case d.typ == cube.DeviceShutterContact:
		e = []byte{0x06, a[0], a[1], a[2], 0x00, 0x12, 0x18}
		if d.open {
			e[lFlags2] |= 0x02
		}
		return e, nil
	case d.typ.IsRadiatorThermostat():
		e = []byte{0x0b, a[0], a[1], a[2], 0x00, 0x12, 0x18, byte(d.valve), 0, 0, 0, 0}
	case d.typ == cube.DeviceWallThermostat:
		e = []byte{0x0c, a[0], a[1], a[2], 0x00, 0x12, 0x18, 0x01, 0, 0, 0, 0, 0}
	default:
		return nil, fmt.Errorf("fakecube: no L entry for a %s", d.typ)
	}
	sp, err := cube.EncodeTemp(d.setpoint)
	if err != nil {
		return nil, err
	}
	e[lSetpoint] = sp
	if d.actual != 0 {
		if err := encodeActual(e, d.typ, d.actual); err != nil {
			return nil, err
		}
	}
	return e, nil
}

// LoadFixture builds the fixture of that name.
func LoadFixture(name string) ([]cube.Message, error) {
	if f, ok := Fixtures[name]; ok {
		return f()
	}
	var names []string
	for n := range Fixtures {
		names = append(names, n)
	}
	return nil, fmt.Errorf("fakecube: no fixture %q (there are: %s)", name, strings.Join(names, ", "))
}
