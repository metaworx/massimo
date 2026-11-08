// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package cube

// Fuzz targets for every parser of bytes that arrive from the Cube or,
// through the proxy, from any LAN client: none may panic, and what has
// an encoder comes back as it went in. `go test` runs the seeds, the
// recorded greeting among them; `make fuzz` runs each target for
// FUZZTIME (GUIDELINES/project/CI.md).

import (
	"bytes"
	"os"
	"testing"
)

// greeting is the recorded session, the seeds' source.
func greeting(f *testing.F) []Message {
	f.Helper()
	raw, err := os.ReadFile("testdata/cube-fw0113-greeting.txt")
	if err != nil {
		f.Fatal(err)
	}
	msgs, err := SplitLines(string(raw))
	if err != nil {
		f.Fatal(err)
	}
	return msgs
}

// FuzzReader reads a stream as the client and the proxy do: at most a
// line for each byte and a line no longer than the input.
func FuzzReader(f *testing.F) {
	raw, _ := os.ReadFile("testdata/cube-fw0113-greeting.txt")
	f.Add(raw)
	f.Add([]byte("H:x\r\n\r\nM:\n\x00\xff"))
	f.Fuzz(func(t *testing.T, b []byte) {
		r := NewReader(bytes.NewReader(b))
		for n := 0; ; n++ {
			if n > len(b)+1 {
				t.Fatalf("%d lines from %d bytes", n, len(b))
			}
			line, err := r.ReadRaw()
			if err != nil {
				return
			}
			if len(line) > len(b) {
				t.Fatalf("a line of %d bytes from %d", len(line), len(b))
			}
			_, _ = ParseLine(line)
		}
	})
}

// FuzzLine parses a line as the type its letter says: the Cube's
// messages, and the commands a LAN client sends through the proxy.
func FuzzLine(f *testing.F) {
	for _, m := range greeting(f) {
		f.Add(m.String())
	}
	for _, l := range []string{
		"s:AARAAAAACgEKAV4=", "t:01,1,CgEN", "c:0a010e", "l:", "m:00,VgIGAQ==", "z:1e,G,01", "n:003c",
		"S:00,0,31", "N:Aw8dVEtFUTAwMDAwMDEB", "F:ntp.homematic.com,ntp.homematic.com", "A:", "E:AAAA", "t:01,2,CgENCgEO",
	} {
		f.Add(l)
	}
	f.Fuzz(func(t *testing.T, line string) {
		m, err := ParseLine(line)
		if err != nil {
			return
		}
		switch m.Type {
		case 'H':
			_, _ = ParseHello(m.Data)
		case 'L':
			_, _ = ParseDeviceList(m.Data, map[RFAddress]DeviceType{0x0a010d: DeviceRadiatorThermostat})
			_, _ = ParseDeviceList(m.Data, nil)
		case 'M':
			if p, err := ParseMetadataPart(m.Data); err == nil {
				_, _ = ParseMetadata([]MetadataPart{p})
			}
		case 'C':
			_, _ = ParseConfig(m.Data)
		case 'N':
			_, _ = ParseNewDevice(m.Data)
		case 'S':
			_, _ = ParseSendResult(m.Data)
		case 'F':
			_ = ParseNTPServers(m.Data)
		case 'E', 'D':
			_, _ = ParseCryptoReply(m.Data)
		case 's':
			_, _ = ParseRadioCommand(m.Data)
		case 't':
			force, addrs, err := ParseDeleteDevices(m.Data)
			if err == nil {
				back := CmdDeleteDevices(force, addrs...)
				if f2, a2, err := ParseDeleteDevices(back.Data); err != nil || f2 != force || len(a2) != len(addrs) {
					t.Fatalf("t: %q came back as %q", m.Data, back.Data)
				}
			}
		case 'c':
			_, _ = ParseRFAddress(m.Data)
		}
	})
}

// FuzzMetadata decodes a metadata block; one that decodes encodes to the
// same bytes, so that a proxy's rewrite of a client's m: changes nothing
// it did not mean to.
func FuzzMetadata(f *testing.F) {
	var parts []MetadataPart
	for _, m := range greeting(f) {
		if m.Type == 'M' {
			p, err := ParseMetadataPart(m.Data)
			if err != nil {
				f.Fatal(err)
			}
			parts = append(parts, p)
		}
	}
	md, err := ParseMetadata(parts)
	if err != nil {
		f.Fatal(err)
	}
	b, err := md.Encode()
	if err != nil {
		f.Fatal(err)
	}
	f.Add(b)
	f.Add([]byte{'V', 2, 0, 0, 1})
	f.Fuzz(func(t *testing.T, b []byte) {
		md, err := DecodeMetadata(b)
		if err != nil || len(b) >= 2 && b[0] == 0 && b[1] == 0 {
			return // a header of zeros encodes as the default (Encode)
		}
		back, err := md.Encode()
		if err != nil {
			t.Fatalf("decoded but does not encode: %v", err)
		}
		if !bytes.Equal(back, b) {
			t.Fatalf("encoded as\n%x\nnot\n%x", back, b)
		}
	})
}

// FuzzConfig decodes a device's configuration, and a week programme of
// the size it takes.
func FuzzConfig(f *testing.F) {
	for _, m := range greeting(f) {
		if m.Type == 'C' {
			if _, err := ParseConfig(m.Data); err != nil {
				f.Fatal(err)
			}
			if _, data, ok := bytes.Cut([]byte(m.Data), []byte(",")); ok {
				b, err := b64.DecodeString(string(data))
				if err != nil {
					f.Fatal(err)
				}
				f.Add(b)
			}
		}
	}
	f.Fuzz(func(t *testing.T, b []byte) {
		_, _ = DecodeConfig(b)
		if len(b) >= WeekProgramSize { // its contract: 182 bytes, as DecodeConfig cuts them
			_ = DecodeWeekProgram(b[:WeekProgramSize])
		}
	})
}

// FuzzRFAddress parses an address; one that parses prints as itself.
func FuzzRFAddress(f *testing.F) {
	for _, s := range []string{"0a010d", "0A010D", "0x0a010d", "", "1", "ffffff", "1000000", " 0a010d "} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		a, err := ParseRFAddress(s)
		if err != nil {
			return
		}
		if b, err := ParseRFAddress(a.String()); err != nil || b != a {
			t.Fatalf("%q parsed as %v, which parses as %v (%v)", s, a, b, err)
		}
	})
}

// FuzzTemp: a temperature the protocol can carry encodes to the units it
// came from.
func FuzzTemp(f *testing.F) {
	for _, v := range []uint8{0, 9, 10, 40, 61, 62, 255} {
		f.Add(v)
	}
	f.Fuzz(func(t *testing.T, v uint8) {
		if back, err := EncodeTemp(DecodeTemp(v)); err == nil && back != v {
			t.Fatalf("%d decodes to %v, which encodes to %d", v, DecodeTemp(v), back)
		}
	})
}
