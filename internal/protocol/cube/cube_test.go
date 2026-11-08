// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package cube

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"os"
	"reflect"
	"regexp"
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
	ms, err = SplitLines("H:KMD0000505,0a0106,0113\nB:FOOBAR1,x,1,y")
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
	h, err = ParseHello("KMD0000505,0a0106,0113")
	if err != nil || h.RFAddress != 0x0a0106 {
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

// TestCountersUpToFFFF: the H and S messages' counters read 16 bits, so
// one past ffff is refused on every platform rather than read as a
// negative number where int has 32 bits.
func TestCountersUpToFFFF(t *testing.T) {
	h, err := ParseHello("KMD0000505,0a0106,0113,00000000,477719c0,ffff,ffff")
	if err != nil || h.DutyCycle != 0xffff || h.FreeMemorySlots != 0xffff {
		t.Fatalf("H with ffff: %+v %v", h, err)
	}
	r, err := ParseSendResult("ffff,0,ffff")
	if err != nil || r.DutyCycle != 0xffff || r.FreeMemorySlots != 0xffff {
		t.Fatalf("S with ffff: %+v %v", r, err)
	}
	for _, line := range []string{
		"KMD0000505,0a0106,0113,00000000,477719c0,10000,32",
		"KMD0000505,0a0106,0113,00000000,477719c0,00,80000000",
	} {
		if _, err := ParseHello(line); err == nil || !strings.Contains(err.Error(), "up to ffff") {
			t.Errorf("ParseHello(%q): %v, want refused", line, err)
		}
	}
	for _, data := range []string{"10000,0,31", "00,0,80000000"} {
		if _, err := ParseSendResult(data); err == nil || !strings.Contains(err.Error(), "up to ffff") {
			t.Errorf("ParseSendResult(%q): %v, want refused", data, err)
		}
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
const cubeConfigExample = "7QPyXQATAQBKRVEwMDAwNTAzAAsABEAAAAAAAAAAAP///////////////////////////wsABEAAAAAAAAAAQf//////////////" +
	"/////////////2h0dHA6Ly9tYXguZXEtMy5kZTo4MC9jdWJlADAvbG9va3VwAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA" +
	"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAENFVAAACgADAAAO" +
	"EENFU1QAAwACAAAcIA=="

func TestParseCubeConfig(t *testing.T) {
	c, err := ParseConfig("03f25d," + cubeConfigExample)
	if err != nil {
		t.Fatal(err)
	}
	if c.Type != DeviceCube || c.Serial != "JEQ0000503" || c.Cube == nil {
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
	copy(b[8:18], "KEQ0000504")
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

	// The cases marked captured are lines of the original software, their
	// radio addresses replaced by placeholders and re-encoded; the rest of
	// each line is as captured.
	cases := []struct {
		name string
		got  Message
		want string
	}{
		{"set temperature", mustMsg(CmdSetTemperature(room(0x0a0203, 1), 19, ModeManual, Until{})), "s:AARAAAAACgIDAWY="},
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
		{"delete", CmdDeleteDevices(true, 0x0a0204), "t:01,1,CgIE"},
		{"portal on", CmdSetPortalInterval(0), "i:00000000,00000000"},
		{"portal off", CmdSetPortalInterval(-1), "i:ffffffff,00000000"},
		{"ntp", CmdSetNTP("nl.pool.ntp.org", "ntp.homematic.com"), "f:nl.pool.ntp.org,ntp.homematic.com"},
		{"time zone (captured)", mustMsg(CmdSetTimeZone(
			TimeZoneRule{"EET", 10, 0, 3, 7200}, TimeZoneRule{"EEST", 3, 0, 2, 10800})), "v:RUVUAAAKAAMAABwgRUVTVAADAAIAACow"},
		{"reset error (captured)", CmdResetError(0x0a010a), "r:01,CgEK"},
		{"manual 15 (captured)", mustMsg(CmdSetTemperature(room(0x0a010a, 1), 15, ModeManual, Until{})), "s:AARAAAAACgEKAV4="},
		{"auto (captured)", mustMsg(CmdSetTemperature(room(0x0a0108, 3), 0, ModeAuto, Until{})), "s:AARAAAAACgEIAwA="},
		{"temporary 6.5 until 2026-11-04 02:00 (captured)", mustMsg(CmdSetTemperature(room(0x0a010a, 1), 6.5, ModeVacation, Until{2026, 11, 4, 120})), "s:AARAAAAACgEKAY2kmgQ="},
		{"temperatures with -2 offset (captured)", mustMsg(CmdSetTemperatures(dev(0x0a010a), TemperatureSettings{
			Comfort: 22, Eco: 15, Max: 30.5, Min: 4.5, Offset: -2, WindowOpen: 5, WindowOpenMinutes: 15,
		})), "s:AAARAAAACgEKACwePQkDCgM="},
		{"valve config (captured)", mustMsg(CmdSetValveConfig(room(0x0a010a, 1), ValveSettings{
			BoostMinutes: 5, BoostValvePct: 80, DecalcDay: Saturday, DecalcHour: 12, MaxValvePct: 100, ValveOffsetPct: 0,
		})), "s:AAQSAAAACgEKATAM/wA="},
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

func TestSetDayProgramCaptured(t *testing.T) {
	// Two day programs written by the original software; programs with
	// fewer than 7 points are padded with the last point.
	room := Target{RFAddress: 0x0a010a, RoomID: 1, Room: true}
	cases := []struct {
		day  Weekday
		prog DayProgram
		want string
	}{
		{Wednesday, DayProgram{{19.5, 1105}, {21, 1215}, {17, 1330}, {12, 1440}}, "s:AAQQAAAACgEKAQRO3VTzRQoxIDEgMSAxIA=="},
		{Sunday, DayProgram{{17, 390}, {21, 480}, {19.5, 965}, {23, 1440}}, "s:AAQQAAAACgEKAQFETlRgTsFdIF0gXSBdIA=="},
	}
	for _, c := range cases {
		msgs, err := CmdSetDayProgram(room, c.day, c.prog)
		if err != nil || len(msgs) != 1 || msgs[0].String() != c.want {
			t.Errorf("%s: got %v %v, want %s", c.day, msgs, err, c.want)
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
	m, err := CmdSetTemperature(Target{RFAddress: 0x0fdaed, RoomID: 1, Room: true}, 18, ModeVacation, Until{2011, 8, 29, 120})
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
	// Captured from a real Cube right after power-up, its address a
	// placeholder: flags are all zero.
	st, err := ParseDeviceList("CwoBDQkAAAAAAAAA", map[RFAddress]DeviceType{0x0a010d: DeviceRadiatorThermostat})
	if err != nil {
		t.Fatal(err)
	}
	if st[0].Valid || st[0].Setpoint != nil || st[0].ValvePct != nil {
		t.Fatalf("got %+v", st[0])
	}
}

// TestRealCubeGreeting decodes a session captured from a Cube with firmware
// 1.1.3 (room and device names anonymised, the serials and radio addresses
// replaced by placeholders, everything else verbatim).
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
	if hello.Serial != "KEQ0000100" || hello.FirmwareVersion != "1.1.3" || hello.FreeMemorySlots != 50 || hello.CubeDate != "2026-11-03" {
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

// placeholderSerial is the form of the placeholders a fixture carries in
// place of a real serial (KEQ0000100, MEQ0000101).
var placeholderSerial = regexp.MustCompile(`^[A-Z]EQ0000[0-9]{3}$`)

// TestGreetingsUsePlaceholders keeps real serials out of the recorded
// greetings: every serial in an H, M or C line is a placeholder.
func TestGreetingsUsePlaceholders(t *testing.T) {
	for _, path := range []string{"testdata/cube-fw0113-greeting.txt", "../../../tools/ui-test/greeting.txt"} {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		msgs, err := SplitLines(string(raw))
		if err != nil {
			t.Fatal(err)
		}
		var serials []string
		for _, m := range msgs {
			switch m.Type {
			case 'H':
				h, err := ParseHello(m.Data)
				if err != nil {
					t.Fatal(err)
				}
				serials = append(serials, h.Serial)
			case 'M':
				p, err := ParseMetadataPart(m.Data)
				if err != nil {
					t.Fatal(err)
				}
				md, err := ParseMetadata([]MetadataPart{p})
				if err != nil {
					t.Fatal(err)
				}
				for _, d := range md.Devices {
					serials = append(serials, d.Serial)
				}
			case 'C':
				c, err := ParseConfig(m.Data)
				if err != nil {
					t.Fatal(err)
				}
				serials = append(serials, c.Serial)
			}
		}
		if len(serials) == 0 {
			t.Fatalf("%s: no serial found", path)
		}
		for i, s := range serials {
			if !placeholderSerial.MatchString(s) {
				t.Errorf("%s: serial %d is not a placeholder of the form KEQ0000123", path, i)
			}
		}
	}
}

func TestWeekProgramJSON(t *testing.T) {
	var w WeekProgram
	w[Monday] = DayProgram{{Temp: 17, Until: 420}, {Temp: 21, Until: 1440}}
	w[Saturday] = DayProgram{{Temp: 18, Until: 1440}}
	js, err := json.Marshal(w)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"monday":[{"temp":17,"until":"07:00"},{"temp":21,"until":"24:00"}],"tuesday":null,"wednesday":null,"thursday":null,"friday":null,"saturday":[{"temp":18,"until":"24:00"}],"sunday":null}`
	if string(js) != want {
		t.Fatalf("got %s", js)
	}
	var back WeekProgram
	if err := json.Unmarshal(js, &back); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(w, back) {
		t.Fatalf("round trip %+v", back)
	}
	// Device-native forms are still read.
	var old WeekProgram
	if err := json.Unmarshal([]byte(`[[{"temp":18,"until_min":1440}],null,[{"temp":17,"until_min":420},{"temp":21,"until_min":1440}],null,null,null,null]`), &old); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(w, old) {
		t.Fatalf("old form %+v", old)
	}
	// Numeric and unquoted HHMM forms.
	var alt WeekProgram
	if err := json.Unmarshal([]byte(`{"monday":[{"temp":17,"until":700},{"temp":21,"until":"2400"}],"saturday":[{"temp":18,"until":"24:00"}]}`), &alt); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(w, alt) {
		t.Fatalf("numeric form %+v", alt)
	}
	var bad SwitchPoint
	if err := json.Unmarshal([]byte(`{"temp":17,"until":"07:60"}`), &bad); err == nil {
		t.Fatal("07:60 accepted")
	}
}

// A wall thermostat seen on hardware in 2026 (fw 16): the three bytes
// after the week program are offset 0, window-open 12.0 °C, boost 5 min
// at 80 %. Its L entry reports auto, 18.0 °C set, 24.4 °C measured.
func TestWallThermostatConfigAndState(t *testing.T) {
	c, err := ParseConfig("0a010e,zgoBDgMAEP9ORVEwMDAwNTA3KiI9CURIVQhFIEUgRSBFIEUgRSBFIEUgRSBFIEUgREhVCEUgRSBFIEUgRSBFIEUgRSBFIEUgRSBESFRsRMxVFEUgRSBFIEUgRSBFIEUgRSBFIERIVGxEzFUURSBFIEUgRSBFIEUgRSBFIEUgREhUbETMVRRFIEUgRSBFIEUgRSBFIEUgRSBESFRsRMxVFEUgRSBFIEUgRSBFIEUgRSBFIERIVGxEzFUURSBFIEUgRSBFIEUgRSBFIEUgBxgw")
	if err != nil {
		t.Fatal(err)
	}
	w := c.Wall
	if c.Type != DeviceWallThermostat || c.Firmware != 16 || c.Serial != "NEQ0000507" || w == nil {
		t.Fatalf("config %+v", c)
	}
	if w.ComfortTemp != 21 || w.EcoTemp != 17 || w.MaxSetpoint != 30.5 || w.MinSetpoint != 4.5 ||
		w.TempOffset != 0 || w.WindowOpenTemp != 12 || w.BoostMinutes != 5 || w.BoostValvePct != 80 || len(w.Extra) != 0 {
		t.Fatalf("wall config %+v", w)
	}
	if got := w.WeekProgram[Monday]; len(got) != 5 || got[1] != (SwitchPoint{Temp: 21, Until: 540}) {
		t.Fatalf("monday %+v", got)
	}
	states, err := ParseDeviceList("DAoBDvYSGAEkAAAA9A==", map[RFAddress]DeviceType{0x0a010e: DeviceWallThermostat})
	if err != nil || len(states) != 1 {
		t.Fatal(err)
	}
	s := states[0]
	if !s.Valid || s.Mode != ModeAuto || s.Setpoint == nil || *s.Setpoint != 18 || s.ActualTemp == nil || *s.ActualTemp != 24.4 || s.ValvePct != nil {
		t.Fatalf("state %+v", s)
	}
	nd, err := ParseNewDevice("AwoBDk5FUTAwMDA1MDf/")
	if err != nil || nd.Type != DeviceWallThermostat || nd.RFAddress != 0x0a010e || nd.Serial != "NEQ0000507" {
		t.Fatalf("new device %+v %v", nd, err)
	}
}

func TestWallDisplayMode(t *testing.T) {
	types := map[RFAddress]DeviceType{0x0a010e: DeviceWallThermostat}
	for _, c := range []struct {
		data string
		want bool
	}{{"DAoBDvYSGAEkAAAA7Q==", false}, {"DAoBDvYSGAQkAAAA7Q==", true}} {
		states, err := ParseDeviceList(c.data, types)
		if err != nil || len(states) != 1 || states[0].DisplayActual == nil || *states[0].DisplayActual != c.want {
			t.Fatalf("%s: %+v %v", c.data, states, err)
		}
	}
	// The original software's radiogram for "show measured temperature".
	if got := CmdLockControls(Target{RFAddress: 0x0a0103}, false).String(); got != "s:AACAAAAACgEDAAA=" {
		t.Fatalf("lock controls: %s", got)
	}
	if got := CmdSetDisplayActual(Target{RFAddress: 0x0a010e}, true).String(); got != "s:AACCAAAACgEOAAQ=" {
		t.Fatalf("display command %s", got)
	}
}

// The headers of s: commands the original software sent, as captured.
func TestParseRadioCommand(t *testing.T) {
	for _, tc := range []struct {
		data string
		want RadioCommand
	}{
		{"AAQQAAAACgEKAQRO3VTzRQoxIDEgMSAxIA==", RadioCommand{RFSetWeekProgram, Target{RFAddress: 0x0a010a, RoomID: 1, Room: true}}},
		{"AAARAAAACgEKACwePQkDCgM=", RadioCommand{RFSetTemperatures, Target{RFAddress: 0x0a010a}}},
		{"AARAAAAACgENBgA=", RadioCommand{RFSetTemperature, Target{RFAddress: 0x0a010d, RoomID: 6, Room: true}}},
	} {
		got, err := ParseRadioCommand(tc.data)
		if err != nil || got != tc.want {
			t.Errorf("%s: %+v %v, want %+v", tc.data, got, err, tc.want)
		}
	}
	for _, bad := range []string{"", "!!", "AARAAAAA"} {
		if _, err := ParseRadioCommand(bad); err == nil {
			t.Errorf("%q accepted", bad)
		}
	}
}

func TestParseDeleteDevices(t *testing.T) {
	force, addrs, err := ParseDeleteDevices(CmdDeleteDevices(true, 0x0a0204, 0x0a010d).Data)
	if err != nil || !force || len(addrs) != 2 || addrs[0] != 0x0a0204 || addrs[1] != 0x0a010d {
		t.Fatalf("%v %v %v", force, addrs, err)
	}
	for _, bad := range []string{"", "01,1", "zz,1,CgIE", "02,1,CgIE", "01,0,!!"} {
		if _, _, err := ParseDeleteDevices(bad); err == nil {
			t.Errorf("%q accepted", bad)
		}
	}
}

// The thermostats are the radiator thermostats and the wall thermostat.
func TestIsThermostat(t *testing.T) {
	for typ, want := range map[DeviceType]bool{DeviceCube: false, DeviceRadiatorThermostat: true, DeviceRadiatorThermostatPlus: true,
		DeviceWallThermostat: true, DeviceShutterContact: false, DeviceEcoSwitch: false} {
		if got := typ.IsThermostat(); got != want {
			t.Errorf("%v: %v, want %v", typ, got, want)
		}
	}
}

// A metadata block without bytes after its device list encodes without
// them: Encode adds the default 0x01 only to metadata built, not read.
func TestMetadataWithoutTrailingBytes(t *testing.T) {
	b := []byte{'V', 2, 0, 0}
	m, err := DecodeMetadata(b)
	if err != nil {
		t.Fatal(err)
	}
	if back, err := m.Encode(); err != nil || !bytes.Equal(back, b) {
		t.Fatalf("encoded as %x, want %x (%v)", back, b, err)
	}
	if back, _ := (&Metadata{}).Encode(); !bytes.Equal(back, []byte{'V', 2, 0, 0, 1}) {
		t.Fatalf("built metadata encoded as %x", back)
	}
}
