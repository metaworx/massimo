// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package cube

import (
	"bytes"
	"encoding/binary"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"
)

// Examples below are taken from https://github.com/Bouni/max-cube-protocol
// unless stated otherwise.

func TestParseLine(t *testing.T) {
	m, err := ParseLine("S:00,0,31\r\n")
	if err != nil {
		t.Fatal(err)
	}
	if m.Type != 'S' || m.Data != "00,0,31" || m.Wire() != "S:00,0,31\r\n" {
		t.Fatalf("got %+v", m)
	}
	if _, err := ParseLine("garbage"); err == nil {
		t.Fatal("want error for non-protocol line")
	}
}

func TestSplitLinesHandlesLFAndPadding(t *testing.T) {
	ms, err := SplitLines("i:000051b1,00000000,ffffffff\r\nb:OK\r\n\x00\x00\x00")
	if err != nil {
		t.Fatal(err)
	}
	if len(ms) != 2 || ms[0].Type != 'i' || ms[1].String() != "b:OK" {
		t.Fatalf("got %v", ms)
	}
	ms, err = SplitLines("H:KMD1016788,099c3e,0113\nB:FOOBAR1,x,1,y")
	if err != nil || len(ms) != 2 || ms[1].Type != 'B' {
		t.Fatalf("got %v, %v", ms, err)
	}
}

func TestReader(t *testing.T) {
	r := NewReader(strings.NewReader("H:a,000001,0113\r\nL:\r\n"))
	m, err := r.Read()
	if err != nil || m.Type != 'H' {
		t.Fatalf("got %v %v", m, err)
	}
	m, err = r.Read()
	if err != nil || m.Type != 'L' || m.Data != "" {
		t.Fatalf("got %v %v", m, err)
	}
}

func TestParseHello(t *testing.T) {
	h, err := ParseHello("KEQ0523864,097f2c,0113,00000000,477719c0,00,32,0d0c09,1404,03,0000")
	if err != nil {
		t.Fatal(err)
	}
	if h.Serial != "KEQ0523864" || h.RFAddress != 0x097f2c || h.FirmwareVersion != "1.1.3" {
		t.Errorf("identity: %+v", h)
	}
	if h.DutyCycle != 0 || h.FreeMemorySlots != 50 || h.CubeDate != "2013-12-09" || h.CubeTime != "20:04" {
		t.Errorf("status: %+v", h)
	}
	// Shortened form sent over the portal connection.
	h, err = ParseHello("KMD1016788,099c3e,0113")
	if err != nil || h.RFAddress != 0x099c3e {
		t.Fatalf("short form: %+v %v", h, err)
	}
}

func TestParseSendResult(t *testing.T) {
	r, err := ParseSendResult("00,0,31")
	if err != nil {
		t.Fatal(err)
	}
	if r.DutyCycle != 0 || r.Discarded || r.FreeMemorySlots != 0x31 {
		t.Fatalf("got %+v", r)
	}
	r, _ = ParseSendResult("64,1,00")
	if r.DutyCycle != 100 || !r.Discarded {
		t.Fatalf("got %+v", r)
	}
}

func TestParseNewDevice(t *testing.T) {
	d, err := ParseNewDevice("Aw4VzExFUTAwMTUzNDD/")
	if err != nil {
		t.Fatal(err)
	}
	if d.Type != DeviceWallThermostat || d.RFAddress != 0x0e15cc || d.Serial != "LEQ0015340" {
		t.Fatalf("got %+v", d)
	}
}

func TestParseNTPServers(t *testing.T) {
	got := ParseNTPServers("ntp.homematic.com,ntp.homematic.com")
	if len(got) != 2 || got[0] != "ntp.homematic.com" {
		t.Fatalf("got %v", got)
	}
}

const metadataExample = "VgIEAQNCYWQK7WkCBEJ1cm8K8wADCldvaG56aW1tZXIK8wwEDFNjaGxhZnppbW1lcgr" +
	"1QAUCCu1pS0VRMDM3ODA0MAZIVCBCYWQBAgrzAEtFUTAzNzk1NDQHSFQgQnVybwICCvMMS0VRMD" +
	"M3OTU1NhlIVCBXb2huemltbWVyIEJhbGtvbnNlaXRlAwIK83lLRVEwMzc5NjY1GkhUIFdvaG56a" +
	"W1tZXIgRmVuc3RlcnNlaXRlAwIK9UBLRVEwMzgwMTIwD0hUIFNjaGxhZnppbW1lcgQB"

func TestParseMetadata(t *testing.T) {
	part, err := ParseMetadataPart("00,01," + metadataExample)
	if err != nil {
		t.Fatal(err)
	}
	m, err := ParseMetadata([]MetadataPart{part})
	if err != nil {
		t.Fatal(err)
	}
	wantRooms := []RoomMeta{
		{1, "Bad", 0x0aed69}, {2, "Buro", 0x0af300}, {3, "Wohnzimmer", 0x0af30c}, {4, "Schlafzimmer", 0x0af540},
	}
	if !reflect.DeepEqual(m.Rooms, wantRooms) {
		t.Errorf("rooms: got %+v", m.Rooms)
	}
	if len(m.Devices) != 5 {
		t.Fatalf("want 5 devices, got %d", len(m.Devices))
	}
	d := m.Devices[0]
	if d.Type != DeviceRadiatorThermostatPlus || d.RFAddress != 0x0aed69 || d.Serial != "KEQ0378040" || d.Name != "HT Bad" || d.RoomID != 1 {
		t.Errorf("device 0: %+v", d)
	}
	if last := m.Devices[4]; last.Name != "HT Schlafzimmer" || last.RoomID != 4 {
		t.Errorf("device 4: %+v", last)
	}

	// Round trip: encoding the decoded metadata reproduces the original bytes.
	raw, err := m.Encode()
	if err != nil {
		t.Fatal(err)
	}
	if got := b64.EncodeToString(raw); got != metadataExample {
		t.Errorf("round trip mismatch:\n got %s\nwant %s", got, metadataExample)
	}
}

func TestParseMetadataMultiPart(t *testing.T) {
	// Split the base64 string at an arbitrary position, as the Cube does.
	a, b := metadataExample[:101], metadataExample[101:]
	m, err := ParseMetadata([]MetadataPart{{Index: 1, Count: 2, Data: b}, {Index: 0, Count: 2, Data: a}})
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Rooms) != 4 || len(m.Devices) != 5 {
		t.Fatalf("got %d rooms, %d devices", len(m.Rooms), len(m.Devices))
	}
	if _, err := ParseMetadata([]MetadataPart{{Index: 1, Count: 2, Data: b}}); err == nil {
		t.Fatal("want error for missing part")
	}
}

func TestWriteMetadataCommands(t *testing.T) {
	part, _ := ParseMetadataPart("00,01," + metadataExample)
	m, _ := ParseMetadata([]MetadataPart{part})
	cmds, err := WriteMetadataCommands(m)
	if err != nil {
		t.Fatal(err)
	}
	if len(cmds) != 1 || cmds[0].String() != "m:00,"+metadataExample {
		t.Fatalf("got %v", cmds)
	}
}

// cubeConfigExample is the Cube C payload from the hexdump in the protocol docs.
const cubeConfigExample = "7QPyXQATAQBKRVEwNTQ0OTIzAAsABEAAAAAAAAAAAP///////////////////////////wsABEAAAAAAAAAAQf//////////////" +
	"/////////////2h0dHA6Ly9tYXguZXEtMy5kZTo4MC9jdWJlADAvbG9va3VwAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA" +
	"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAENFVAAACgADAAAO" +
	"EENFU1QAAwACAAAcIA=="

func TestParseCubeConfig(t *testing.T) {
	c, err := ParseConfig("03f25d," + cubeConfigExample)
	if err != nil {
		t.Fatal(err)
	}
	if c.Type != DeviceCube || c.Serial != "JEQ0544923" || c.Cube == nil {
		t.Fatalf("header: %+v", c)
	}
	cc := c.Cube
	if cc.PortalURL != "http://max.eq-3.de:80/cube" || cc.PortalEnabled {
		t.Errorf("portal: %+v", cc)
	}
	if cc.PushButtonUp != 0x00 || cc.PushButtonDown != 0x41 {
		t.Errorf("push buttons: %+v", cc)
	}
	wantW := TimeZoneRule{Name: "CET", Month: 10, Weekday: 0, Hour: 3, OffsetSeconds: 3600}
	wantS := TimeZoneRule{Name: "CEST", Month: 3, Weekday: 0, Hour: 2, OffsetSeconds: 7200}
	if cc.Winter != wantW || cc.Summer != wantS {
		t.Errorf("time zones: %+v / %+v", cc.Winter, cc.Summer)
	}
	if _, err := ParseConfig("03f25e," + cubeConfigExample); err == nil {
		t.Error("want error for mismatching address")
	}
}

// thermostatConfig builds a radiator thermostat C payload using the example
// values from the protocol docs.
func thermostatConfig() []byte {
	b := make([]byte, thermoWeekProgram+WeekProgramSize)
	b[0] = byte(len(b) - 1)
	copy(b[1:4], []byte{0x0f, 0xc3, 0x80})
	b[4], b[5], b[6], b[7] = byte(DeviceRadiatorThermostat), 1, 0x19, 0x01
	copy(b[8:18], "KEQ0565026")
	copy(b[0x12:], []byte{0x24, 0x20, 0x3d, 0x09, 0x07, 0x18, 0x03, 0xf4, 0x0c, 0xff, 0x00})
	// Every day: 16 °C until 06:00, 21 °C until 22:00, 16 °C until 24:00; rest filled with the last point.
	for d := 0; d < 7; d++ {
		pts := []uint16{32<<9 | 72, 42<<9 | 264, 32<<9 | 288}
		for s := 0; s < DaySlots; s++ {
			v := pts[min(s, len(pts)-1)]
			binary.BigEndian.PutUint16(b[thermoWeekProgram+(d*DaySlots+s)*2:], v)
		}
	}
	return b
}

func TestDecodeThermostatConfig(t *testing.T) {
	c, err := DecodeConfig(thermostatConfig())
	if err != nil {
		t.Fatal(err)
	}
	th := c.Thermostat
	if th == nil {
		t.Fatal("no thermostat config")
	}
	want := ThermostatConfig{
		ComfortTemp: 18, EcoTemp: 16, MaxSetpoint: 30.5, MinSetpoint: 4.5, TempOffset: 0,
		WindowOpenTemp: 12, WindowOpenMins: 15, BoostMinutes: 60, BoostValvePct: 100,
		DecalcDay: Saturday, DecalcHour: 12, MaxValvePct: 100, ValveOffsetPct: 0,
	}
	got := *th
	got.WeekProgram = WeekProgram{}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got  %+v\nwant %+v", got, want)
	}
	wantDay := DayProgram{{16, 360}, {21, 1320}, {16, 1440}}
	for d, day := range th.WeekProgram {
		if !reflect.DeepEqual(day, wantDay) {
			t.Errorf("%s: got %+v", Weekday(d), day)
		}
	}
}

func TestParseDeviceList(t *testing.T) {
	// From the protocol docs. Decodes to three 11-byte thermostat entries.
	types := map[RFAddress]DeviceType{0x0fdaed: DeviceRadiatorThermostat}
	st, err := ParseDeviceList("Cw/a7QkSGBgoAMwACw/DcwkSGBgoAM8ACw/DgAkSGBgoAM4A", types)
	if err != nil {
		t.Fatal(err)
	}
	if len(st) != 3 {
		t.Fatalf("want 3 entries, got %d", len(st))
	}
	s := st[0]
	if s.RFAddress != 0x0fdaed || !s.Valid || s.Error || !s.Initialized || s.BatteryLow || !s.GatewayKnown || !s.DST || s.Mode != ModeAuto {
		t.Errorf("flags: %+v", s)
	}
	if s.ValvePct == nil || *s.ValvePct != 24 || *s.Setpoint != 20 || s.ActualTemp == nil || *s.ActualTemp != 20.4 {
		t.Errorf("values: valve=%v setpoint=%v actual=%v", deref(s.ValvePct), deref(s.Setpoint), deref(s.ActualTemp))
	}
	if st[1].RFAddress != 0x0fc373 || *st[1].ActualTemp != 20.7 {
		t.Errorf("entry 1: %+v", st[1])
	}
}

func TestDeviceListVacationAndWall(t *testing.T) {
	// Wall thermostat in vacation mode until 2011-08-29 02:00, measured 29.2 °C, setpoint 25 °C.
	e := []byte{12, 0x01, 0x02, 0x03, 0x00, 0x12, 0x1a, 0x04, 0xb2, 0x9d, 0x0b, 0x04, 0x24}
	st, err := ParseDeviceList(b64.EncodeToString(e), map[RFAddress]DeviceType{0x010203: DeviceWallThermostat})
	if err != nil {
		t.Fatal(err)
	}
	s := st[0]
	if s.Mode != ModeVacation || s.Until == nil || *s.Until != (Until{2011, 8, 29, 120}) {
		t.Errorf("until: %+v", s.Until)
	}
	if *s.Setpoint != 25 || *s.ActualTemp != 29.2 {
		t.Errorf("temps: %v %v", *s.Setpoint, *s.ActualTemp)
	}
	// Shutter contact, window open.
	e = []byte{6, 0x0a, 0x0b, 0x0c, 0x00, 0x12, 0x12}
	st, _ = ParseDeviceList(b64.EncodeToString(e), map[RFAddress]DeviceType{0x0a0b0c: DeviceShutterContact})
	if st[0].WindowOpen == nil || !*st[0].WindowOpen {
		t.Errorf("window: %+v", st[0])
	}
}

func deref[T any](p *T) any {
	if p == nil {
		return nil
	}
	return *p
}

func TestUntilDateRoundTrip(t *testing.T) {
	b0, b1, err := encodeUntilDate(2011, 8, 29)
	if err != nil || b0 != 0x9d || b1 != 0x0b {
		t.Fatalf("got %02x %02x %v", b0, b1, err)
	}
	for m := 1; m <= 12; m++ {
		b0, b1, _ := encodeUntilDate(2026, m, 15)
		y, mm, d := decodeUntilDate(b0, b1)
		if y != 2026 || mm != m || d != 15 {
			t.Errorf("month %d: got %d-%d-%d", m, y, mm, d)
		}
	}
}

func TestCommandExamples(t *testing.T) {
	mustMsg := func(m Message, err error) Message {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
		return m
	}
	room := func(rf RFAddress, id uint8) Target { return Target{RFAddress: rf, RoomID: id, Room: true} }
	dev := func(rf RFAddress) Target { return Target{RFAddress: rf} }

	cases := []struct {
		name string
		got  Message
		want string
	}{
		{"set temperature", mustMsg(CmdSetTemperature(room(0x079100, 1), 19, ModeManual, Until{})), "s:AARAAAAAB5EAAWY="},
		{"set temperatures", mustMsg(CmdSetTemperatures(dev(0x0fc380), TemperatureSettings{
			Comfort: 21.5, Eco: 16.5, Max: 30.5, Min: 4.5, Offset: 0, WindowOpen: 12, WindowOpenMinutes: 15,
		})), "s:AAARAAAAD8OAACshPQkHGAM="},
		{"valve config", mustMsg(CmdSetValveConfig(room(0x0fc380, 1), ValveSettings{
			BoostMinutes: 5, BoostValvePct: 90, DecalcDay: Saturday, DecalcHour: 12, MaxValvePct: 100, ValveOffsetPct: 0,
		})), "s:AAQSAAAAD8OAATIM/wA="},
		{"add link partner", CmdLinkPartner(dev(0x0fc373), true, 0x0fdaed, DeviceRadiatorThermostat), "s:AAAgAAAAD8NzAA/a7QE="},
		{"remove link partner", CmdLinkPartner(dev(0x0fc373), false, 0x0fdaed, DeviceRadiatorThermostat), "s:AAAhAAAAD8NzAA/a7QE="},
		{"set group", CmdSetGroup(0x0fc380, 1, false), "s:AAAiAAAAD8OAAAE="},
		{"remove group", CmdSetGroup(0x0fc380, 1, true), "s:AAAjAAAAD8OAAAE="},
		{"inclusion", CmdInclusion(60 * time.Second), "n:003c"},
		{"wakeup", CmdWakeup(0x1e, WakeupRoom, "01"), "z:1e,G,01"},
		{"delete", CmdDeleteDevices(true, 0x0f1d54), "t:01,1,Dx1U"},
		{"portal on", CmdSetPortalInterval(0), "i:00000000,00000000"},
		{"portal off", CmdSetPortalInterval(-1), "i:ffffffff,00000000"},
		{"ntp", CmdSetNTP("nl.pool.ntp.org", "ntp.homematic.com"), "f:nl.pool.ntp.org,ntp.homematic.com"},
		{"set time", mustMsg(CmdSetTime(
			TimeZoneRule{"CET", 10, 0, 3, 3600}, TimeZoneRule{"CEST", 3, 0, 2, 7200},
			cubeEpoch.Add(0x1fdda096*time.Second))), "v:Q0VUAAAKAAMAAA4QQ0VTVAADAAIAABwg,1fdda096"},
	}
	for _, c := range cases {
		if c.got.String() != c.want {
			t.Errorf("%s: got %s want %s", c.name, c.got, c.want)
		}
	}
}

func TestSetDayProgramExample(t *testing.T) {
	prog := DayProgram{{16, 365}, {19, 550}, {16, 1015}, {19, 1440}}
	msgs, err := CmdSetDayProgram(Target{RFAddress: 0x0fc380, RoomID: 1, Room: true}, Monday, prog)
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 1 || msgs[0].String() != "s:AAQQAAAAD8OAAQJASUxuQMtNIE0gTSBNIA==" {
		t.Fatalf("got %v", msgs)
	}
	// More than 7 points need a second telegram with bit 3 of the day byte set.
	long := DayProgram{}
	for i := 1; i <= 8; i++ {
		long = append(long, SwitchPoint{Temp: 17, Until: i * 180})
	}
	msgs, err = CmdSetDayProgram(Target{RFAddress: 1}, Sunday, long)
	if err != nil || len(msgs) != 2 {
		t.Fatalf("got %v %v", msgs, err)
	}
	raw, _ := b64.DecodeString(msgs[1].Data)
	if raw[10] != byte(Sunday)|0x08 || len(raw) != 11+2 {
		t.Errorf("second telegram: % x", raw)
	}
}

func TestSetTemperatureVacation(t *testing.T) {
	m, err := CmdSetTemperature(Target{RFAddress: 0x0fdaed, RoomID: 1}, 18, ModeVacation, Until{2011, 8, 29, 120})
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := b64.DecodeString(m.Data)
	if !bytes.Equal(raw[10:], []byte{0x80 | 36, 0x9d, 0x0b, 0x04}) {
		t.Fatalf("got % x", raw)
	}
	if _, err := CmdSetTemperature(Target{}, 18, ModeVacation, Until{}); err == nil {
		t.Error("want error for vacation without end")
	}
	if _, err := CmdSetTemperature(Target{}, 18.3, ModeManual, Until{}); err == nil {
		t.Error("want error for 18.3 °C")
	}
}

func TestDayProgramValidate(t *testing.T) {
	bad := []DayProgram{
		{},
		{{20, 600}},                        // does not end at 24:00
		{{20, 900}, {18, 600}, {16, 1440}}, // not increasing
		{{50, 1440}},                       // temperature out of range
	}
	for i, p := range bad {
		if p.Validate() == nil {
			t.Errorf("case %d: want error", i)
		}
	}
}

func TestDeviceListInvalidEntryHasNoValues(t *testing.T) {
	// Captured from a real Cube right after power-up: flags are all zero.
	st, err := ParseDeviceList("CxUUtgkAAAAAAAAA", map[RFAddress]DeviceType{0x1514b6: DeviceRadiatorThermostat})
	if err != nil {
		t.Fatal(err)
	}
	if st[0].Valid || st[0].Setpoint != nil || st[0].ValvePct != nil {
		t.Fatalf("got %+v", st[0])
	}
}

// TestRealCubeGreeting decodes a session captured from a Cube with firmware
// 1.1.3 (room and device names anonymised, everything else verbatim).
func TestRealCubeGreeting(t *testing.T) {
	raw, err := os.ReadFile("testdata/cube-fw0113-greeting.txt")
	if err != nil {
		t.Fatal(err)
	}
	msgs, err := SplitLines(string(raw))
	if err != nil {
		t.Fatal(err)
	}
	var (
		hello   *Hello
		md      *Metadata
		configs []*DeviceConfig
		states  []DeviceState
	)
	types := map[RFAddress]DeviceType{}
	for _, m := range msgs {
		switch m.Type {
		case 'H':
			if hello, err = ParseHello(m.Data); err != nil {
				t.Fatal(err)
			}
		case 'M':
			p, err := ParseMetadataPart(m.Data)
			if err != nil {
				t.Fatal(err)
			}
			if md, err = ParseMetadata([]MetadataPart{p}); err != nil {
				t.Fatal(err)
			}
			for _, d := range md.Devices {
				types[d.RFAddress] = d.Type
			}
		case 'C':
			c, err := ParseConfig(m.Data)
			if err != nil {
				t.Fatal(err)
			}
			configs = append(configs, c)
		case 'L':
			if states, err = ParseDeviceList(m.Data, types); err != nil {
				t.Fatal(err)
			}
		default:
			t.Fatalf("unexpected message type %c", m.Type)
		}
	}
	if hello.Serial != "KEQ0537249" || hello.FirmwareVersion != "1.1.3" || hello.FreeMemorySlots != 50 || hello.CubeDate != "2026-11-03" {
		t.Errorf("hello: %+v", hello)
	}
	if len(md.Rooms) != 6 || len(md.Devices) != 6 || md.Rooms[1].Name != "Küche" {
		t.Errorf("metadata: %+v", md)
	}
	if len(configs) != 7 || configs[0].Type != DeviceCube {
		t.Fatalf("configs: %d", len(configs))
	}
	cc := configs[0].Cube
	if cc.PortalEnabled || cc.PortalURL != "http://max.eq-3.de:80/cube" || cc.Winter.Name != "GMT" || cc.Summer.Name != "BST" || cc.Summer.OffsetSeconds != 3600 {
		t.Errorf("cube config: %+v", cc)
	}
	th := configs[1].Thermostat
	if th == nil || th.ComfortTemp != 21.5 || th.EcoTemp != 16.5 || th.BoostMinutes != 5 || th.BoostValvePct != 80 {
		t.Errorf("thermostat config: %+v", th)
	}
	wantDay := DayProgram{{5, 720}, {13, 1260}, {18, 1290}, {5, 1440}}
	if !reflect.DeepEqual(th.WeekProgram[Monday], wantDay) {
		t.Errorf("week program: %+v", th.WeekProgram[Monday])
	}
	if len(states) != 6 || states[0].Valid || states[0].Type != DeviceRadiatorThermostat {
		t.Errorf("states: %+v", states)
	}
}
