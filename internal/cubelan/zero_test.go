// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package cubelan

import (
	"context"
	"encoding/base64"
	"strings"
	"testing"
	"time"

	"go.metaworx.ch/massimo/internal/cubelan/fakecube"
	"go.metaworx.ch/massimo/internal/protocol/cube"
)

// zeroEntry is the L entry one of the owner's Cubes listed at radio
// address 000000 (firmware 1.1.3, docs/cube-protocol.md): no flags.
var zeroEntry = []byte{0x0b, 0x00, 0x00, 0x00, 0x09, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}

// zeroConfig is a C line of the shape the same Cube sent for 000000: 207
// bytes, its length byte 0xce, its address and type bytes zero, too short
// for a Cube's configuration. Its other bytes are zero here; the Cube's
// were not recorded.
func zeroConfig() cube.Message {
	b := make([]byte, 207)
	b[0] = 0xce
	return cube.Message{Type: 'C', Data: "000000," + base64.StdEncoding.EncodeToString(b)}
}

// zeroGreeting is the fixture's greeting with the C line and the L entry
// at 000000, as that Cube sends them.
func zeroGreeting(t *testing.T) []cube.Message {
	t.Helper()
	g, err := fakecube.LoadGreeting(fixture)
	if err != nil {
		t.Fatal(err)
	}
	var out []cube.Message
	for _, m := range g {
		if m.Type == 'L' {
			raw, err := base64.StdEncoding.DecodeString(m.Data)
			if err != nil {
				t.Fatal(err)
			}
			out = append(out, zeroConfig())
			m = cube.Message{Type: 'L', Data: base64.StdEncoding.EncodeToString(append(raw, zeroEntry...))}
		}
		out = append(out, m)
	}
	return out
}

// The entry at 000000 is no device, and the snapshot gives the greeting
// back as the Cube sent it: the C line it cannot read and the L entry,
// byte for byte, for the proxy and a backup; a backup's lines read back,
// and a later greeting without them forgets the kept line.
func TestZeroEntryIsKeptAndNoDevice(t *testing.T) {
	greeting, err := fakecube.LoadGreeting(fixture)
	if err != nil {
		t.Fatal(err)
	}
	plain, err := SnapshotOf(greeting, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	s, err := SnapshotOf(zeroGreeting(t), time.Now())
	if err != nil {
		t.Fatalf("a greeting with the entry at 000000 reads: %v", err)
	}
	for _, rf := range s.Devices() {
		if rf == cube.ZeroAddress {
			t.Fatalf("000000 among the devices: %v", s.Devices())
		}
	}
	if len(s.Devices()) != len(plain.Devices()) {
		t.Fatalf("devices %v, without the entry %v", s.Devices(), plain.Devices())
	}
	var cLine, lLine string
	for _, m := range s.Lines() {
		if m.Type == 'C' && strings.HasPrefix(m.Data, "000000,") {
			cLine = m.String()
		}
		if m.Type == 'L' {
			lLine = m.Data
		}
	}
	if want := zeroConfig().String(); cLine != want {
		t.Fatalf("the C line at 000000 not repeated as received:\n got %q\nwant %q", cLine, want)
	}
	raw, _ := base64.StdEncoding.DecodeString(lLine)
	if !strings.HasSuffix(string(raw), string(zeroEntry)) {
		t.Fatalf("the L line without the entry at 000000: % x", raw)
	}
	again, err := SnapshotOf(s.Lines(), time.Now())
	if err != nil {
		t.Fatalf("the lines read back: %v", err)
	}
	if a, b := len(s.Lines()), len(again.Lines()); a != b {
		t.Fatalf("%d lines, read back %d", a, b)
	}
	s.beginGreeting()
	for _, m := range greeting {
		if err := s.Apply(m, time.Now()); err != nil {
			t.Fatal(err)
		}
	}
	for _, m := range s.Lines() {
		if strings.HasPrefix(m.Data, "000000,") {
			t.Fatalf("the kept line outlives a greeting without it: %s", m)
		}
	}
}

// A greeting with the entry at 000000 is logged once, at Warn: the line
// that cannot be read, kept, and the entry, not shown.
func TestClientWarnsOfZeroEntry(t *testing.T) {
	srv := &fakecube.Server{Greeting: zeroGreeting(t)}
	addr, err := srv.Start("127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { srv.Close() })
	var logs logLines
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c := New(Options{Addr: addr, Logger: logs.logger()})
	go c.Run(ctx)
	const kept, entry = "cube: a line it sent cannot be read; kept as received for the proxy and backups",
		"cube: its device list has an entry at radio address 000000; it is no device, not shown and sent nothing"
	deadline := time.Now().Add(5 * time.Second)
	for len(logs.lines("WARN", entry)) == 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if k, e := logs.lines("WARN", kept), logs.lines("WARN", entry); len(k) != 1 || len(e) != 1 {
		t.Fatalf("warnings: kept %q, entry %q", k, e)
	}
	for _, rf := range c.Snapshot().Devices() {
		if rf == cube.ZeroAddress {
			t.Fatal("000000 among the client's devices")
		}
	}
}
