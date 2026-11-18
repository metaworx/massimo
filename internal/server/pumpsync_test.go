// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

import (
	"encoding/base64"
	"testing"
	"time"

	"go.metaworx.ch/massimo/internal/protocol/cube"
	"go.metaworx.ch/massimo/internal/testkit"
)

// pumpSync waits until the runtime's event pump has handled every event
// its Cube client had sent out at the call, the events of the Cube's
// lines the client was handling then included: each published to the
// stream, written to the process log, kept for GET …/log (which writes
// the store's pending events before it reads), and whatever else the
// pump does in its turn, such as a room's confirmation or a reset it
// queues. A test calls it after it saw what a Cube event changed where
// the client shows it - the API's devices, rooms and Cubes are the
// client's snapshot - and before it reads what the pump makes of the
// event (the log, the stream, the alerts) or changes the Cube again.
// docs/testing.md, "The pump".
func (rt *runtime) pumpSync(t testing.TB) {
	t.Helper()
	var sent uint64
	testkit.Eventually(t, "the cube client's lines handled", func() bool {
		var handling int64
		handling, sent = rt.client.EventCounts()
		return handling == 0
	})
	if !testkit.Within(testkit.Wait, func() bool { return rt.pumped.Load() >= sent }) {
		t.Fatalf("timed out after %v waiting for the pump to handle the cube client's event %d: it has handled up to %d",
			testkit.Scale(testkit.Wait), sent, rt.pumped.Load())
	}
}

// pumpSync is rt.pumpSync for each of the server's Cubes, one after the
// other.
func pumpSync(t testing.TB, s *Server) {
	t.Helper()
	for _, rt := range s.runtimes() {
		rt.pumpSync(t)
	}
}

// Once the API shows a state the Cube reported, the pump's line of it is
// in the log after pumpSync, without a wait of its own.
func TestPumpSync(t *testing.T) {
	r := startServer(t, stored(), withOptions(Options{PollInterval: 100 * time.Millisecond}))
	fc, ts, s := r.fc, r.ts, r.s
	pumpSync(t, s)
	greeting := fc.Greeting
	raw, _ := base64.StdEncoding.DecodeString(greeting[len(greeting)-1].Data)
	copy(raw, []byte{0x0b, 0x0a, 0x01, 0x0d, 0x09, 0x12, 0x01, 30, 42, 0, 0, 0}) // 0a010d: manual, 21 °C, valve 30 %
	fc.SetDeviceList(cube.Message{Type: 'L', Data: base64.StdEncoding.EncodeToString(raw)})
	testkit.Eventually(t, "the report in the API", func() bool {
		var d map[string]any
		getJSON(t, ts, "", "/api/v1/cubes/KEQ0000100/devices/0a010d", &d)
		st, _ := d["state"].(map[string]any)
		return st != nil && st["valve_pct"] == 30.0
	})
	pumpSync(t, s)
	var evs []event
	getJSON(t, ts, "", "/api/v1/cubes/KEQ0000100/log?kind=state&device=0a010d", &evs)
	if len(evs) == 0 || evs[len(evs)-1].Changes["valve_pct"].To != 30.0 {
		t.Fatalf("the newest state line of 0a010d: %+v", evs)
	}
}
