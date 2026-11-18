// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

import (
	"context"
	"encoding/base64"
	"net/http/httptest"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"go.metaworx.ch/massimo/internal/cubelan/fakecube"
	"go.metaworx.ch/massimo/internal/engine"
	"go.metaworx.ch/massimo/internal/protocol/cube"
	"go.metaworx.ch/massimo/internal/store"
	"go.metaworx.ch/massimo/internal/testkit"
	"go.metaworx.ch/massimo/internal/testkit/cubewatch"
	"go.metaworx.ch/massimo/internal/view"
)

// checkClock is the room checks' clock in the tests: the real time, so
// that the Cube's greeting time moves on from when it was read, plus an
// offset the test moves on, never by sleeping.
type checkClock struct{ off atomic.Int64 }

func (c *checkClock) now() time.Time      { return time.Now().Add(time.Duration(c.off.Load())) }
func (c *checkClock) add(d time.Duration) { c.off.Add(int64(d)) }

// driftRig is a server over the fake Cube's rooms fixture whose room
// checks run only when the test calls check, on the clock of clock.
type driftRig struct {
	t        *testing.T
	fc       *fakecube.Server
	received *testkit.Watch[cube.Message] // the lines fc received
	ts       *httptest.Server
	s        *Server
	rt       *runtime
	st       *store.Store
	clock    *checkClock
}

// startDrift starts a driftRig: the rooms fixture (room 1 a wall
// thermostat and two radiator thermostats, room 2 three radiator
// thermostats, room 3 a window contact; every thermostat in auto at
// 20 °C; the Cube's clock Tuesday 19:51, when the radiator thermostats'
// programme says 21 °C since 18:20 and 17 °C from 20:15), change
// applied to its lines first, and edit to the server's options. The
// fake Cube applies the commands it takes.
func startDrift(t *testing.T, change func([]cube.Message) []cube.Message, edit ...func(*Options)) *driftRig {
	t.Helper()
	greeting, err := fakecube.RoomsFixture()
	if err != nil {
		t.Fatal(err)
	}
	if change != nil {
		greeting = change(greeting)
	}
	fc := &fakecube.Server{Greeting: greeting, Apply: true}
	addr, err := fc.Start("127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	st := testStore(t, "")
	row, err := st.AddCube(context.Background(), store.Cube{Serial: "KEQ0000100", Configured: true, Address: addr, Source: store.SourceEnv})
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := CubeFromRow(row)
	if err != nil {
		t.Fatal(err)
	}
	clock := &checkClock{}
	opts := Options{Cubes: []CubeConfig{cfg}, Store: st, Queue: engine.Options{Gap: time.Millisecond}, PollInterval: 50 * time.Millisecond,
		driftClock: clock.now, driftEvery: -1}
	for _, e := range edit {
		e(&opts)
	}
	s := New(opts)
	ctx, cancel := context.WithCancel(context.Background())
	go s.Run(ctx)
	ts := httptest.NewServer(s.Handler())
	t.Cleanup(func() { ts.Close(); cancel(); fc.Close() })
	waitConnected(t, ts, "KEQ0000100")
	return &driftRig{t: t, fc: fc, received: cubewatch.Lines(fc), ts: ts, s: s, rt: s.cubes["keq0000100"], st: st, clock: clock}
}

// check runs the room checks once.
func (r *driftRig) check() { r.s.roomChecks(context.Background(), r.rt) }

// set changes a device's reported state and waits until the server's
// snapshot shows it.
func (r *driftRig) set(rf cube.RFAddress, mode string, setpoint float64) {
	r.t.Helper()
	if err := r.fc.SetState(rf, fakecube.State{Mode: &mode, Setpoint: &setpoint}); err != nil {
		r.t.Fatal(err)
	}
	r.waitSetting(rf, mode, setpoint)
}

// waitSetting waits until the snapshot shows rf at mode and setpoint.
func (r *driftRig) waitSetting(rf cube.RFAddress, mode string, setpoint float64) {
	r.t.Helper()
	testkit.Eventually(r.t, rf.String()+" at "+mode, func() bool {
		st, ok := r.rt.client.Snapshot().State(rf)
		return ok && st.Mode.String() == mode && st.Setpoint != nil && *st.Setpoint == setpoint
	})
}

// confirmed waits until the pump has handled the states the snapshot
// shows (pumpSync), and checks that they confirmed the writes to room's
// thermostats. The snapshot shows a new state before the pump handles
// its state event and confirms the write; a check before that would find
// the room confirming and pass over it, dropping a disagreement just
// begun.
func (r *driftRig) confirmed(room uint8) {
	r.t.Helper()
	r.rt.pumpSync(r.t)
	if c := r.rt.roomConfirmation(room); c != nil {
		r.t.Fatalf("room %d's writes not confirmed: %+v", room, c)
	}
}

// events are the log's events of a kind, oldest first, once the store
// has written what came before.
func (r *driftRig) events(kind string) []event {
	r.t.Helper()
	if err := r.st.FlushEvents(context.Background()); err != nil {
		r.t.Fatal(err)
	}
	var evs []event
	getJSON(r.t, r.ts, "", "/api/v1/cubes/KEQ0000100/log?limit=200&kind="+kind, &evs)
	return evs
}

// newest is the log's newest event of a kind.
func (r *driftRig) newest(kind string) event {
	r.t.Helper()
	evs := r.events(kind)
	if len(evs) == 0 {
		r.t.Fatalf("no %s event", kind)
	}
	return evs[len(evs)-1]
}

// outcomes are the outcomes of the log's events of a kind, oldest first.
func (r *driftRig) outcomes(kind string) []string {
	r.t.Helper()
	var out []string
	for _, ev := range r.events(kind) {
		out = append(out, summaryText(&ev, "outcome"))
	}
	return out
}

// radioSince are the s: and r: lines the fake Cube received from the
// cursor on.
func (r *driftRig) radioSince(from testkit.Cursor) []string {
	return lineStrings(r.received.Since(from, cubewatch.Type('s', 'r')))
}

// setpointLine is the room command that sets a room to mode and temp,
// named to its master.
func setpointLine(t *testing.T, master cube.RFAddress, room uint8, temp float64, mode cube.Mode) string {
	t.Helper()
	m, err := cube.CmdSetTemperature(cube.Target{RFAddress: master, RoomID: room, Room: true}, temp, mode, cube.Until{})
	if err != nil {
		t.Fatal(err)
	}
	return m.String()
}

// Members of a room that differ for ten minutes are reported; ten
// minutes later the room's state as Massimo last set it is sent once
// more, one room command; agreeing again, the report is cleared. A
// second disagreement the same day is reported, and nothing is sent.
func TestDisagreeResendsMassimosSetpoint(t *testing.T) {
	r := startDrift(t, nil)
	if code, out := do(t, r.ts, "PUT", "/api/v1/cubes/KEQ0000100/rooms/2/setpoint", `{"mode":"manual","temp":21}`); code != 200 {
		t.Fatalf("room setpoint: %d %v", code, out)
	}
	for _, rf := range []cube.RFAddress{0x0a0204, 0x0a0205, 0x0a0206} {
		r.waitSetting(rf, "manual", 21)
	}
	r.confirmed(2)
	sent := r.received.Mark()
	r.set(0x0a0205, "manual", 18)
	r.check()
	r.clock.add(9 * time.Minute)
	r.check()
	if got := r.outcomes("disagree"); len(got) != 0 {
		t.Fatalf("reported before ten minutes: %v", got)
	}
	r.clock.add(2 * time.Minute)
	r.check()
	evs := r.events("disagree")
	if len(evs) != 1 || evs[0].Level != "warn" || summaryText(&evs[0], "outcome") != "differ" || summaryText(&evs[0], "room") != "2" ||
		summaryText(&evs[0], "differs") != "[setpoint]" || !strings.Contains(summaryText(&evs[0], "members"), "rf:0a0205 setpoint:18") {
		t.Fatalf("the report: %+v", evs)
	}
	r.clock.add(9 * time.Minute)
	r.check()
	if got := r.radioSince(sent); len(got) != 0 {
		t.Fatalf("sent before the second check: %v", got)
	}
	r.clock.add(2 * time.Minute)
	r.check()
	want := setpointLine(t, 0x0a0204, 2, 21, cube.ModeManual)
	if got := r.radioSince(sent); !slices.Equal(got, []string{want}) {
		t.Fatalf("the re-send: %v, want %s", got, want)
	}
	if ev := r.newest("disagree"); ev.Level != "info" || summaryText(&ev, "outcome") != "resent" || summaryText(&ev, "from") != "massimo" ||
		summaryText(&ev, "mode") != "manual" || summaryText(&ev, "setpoint") != "21" {
		t.Fatalf("the re-send's event: %+v", ev)
	}
	r.waitSetting(0x0a0205, "manual", 21)
	r.confirmed(2)
	r.clock.add(time.Minute)
	r.check()
	if got := r.outcomes("disagree"); !slices.Equal(got, []string{"differ", "resent", "agree"}) {
		t.Fatalf("outcomes %v", got)
	}
	if alerts := alertKinds(t, r.ts); slices.Contains(alerts, "disagree") {
		t.Fatalf("the report not settled: %v", alerts)
	}

	// The same day: reported, not sent.
	r.set(0x0a0206, "manual", 19)
	r.check()
	r.clock.add(11 * time.Minute)
	r.check()
	r.clock.add(11 * time.Minute)
	r.check()
	r.clock.add(11 * time.Minute)
	r.check()
	if got := r.radioSince(sent); len(got) != 1 {
		t.Fatalf("a second re-send the same day: %v", got)
	}
	last := r.newest("disagree")
	if got := r.outcomes("disagree"); !slices.Equal(got, []string{"differ", "resent", "agree", "differ", "not_resent"}) || summaryText(&last, "why") != "today" {
		t.Fatalf("outcomes %v, %+v", got, last)
	}
	if alerts := alertKinds(t, r.ts); !slices.Contains(alerts, "disagree") {
		t.Fatalf("the second report not an alert: %v", alerts)
	}

	// After a restart, the day's re-send is read from the log.
	d := &r.rt.drift
	d.mu.Lock()
	d.rooms, d.members, d.resent, d.warned, d.seeded = nil, nil, nil, nil, false
	d.mu.Unlock()
	for range 3 {
		r.check()
		r.clock.add(11 * time.Minute)
	}
	if got := r.radioSince(sent); len(got) != 1 {
		t.Fatalf("a re-send after the restart: %v", got)
	}
	if got, last := r.outcomes("disagree"), r.newest("disagree"); !slices.Equal(got[len(got)-2:], []string{"differ", "not_resent"}) || summaryText(&last, "why") != "today" {
		t.Fatalf("after the restart: %v, %+v", got, last)
	}
}

// alertKinds are the kinds of the log's unsettled warnings and errors.
func alertKinds(t *testing.T, ts *httptest.Server) []string {
	t.Helper()
	var v logAlertsView
	getJSON(t, ts, "", "/api/v1/log/alerts", &v)
	var out []string
	for _, a := range v.Alerts {
		out = append(out, a.Kind)
	}
	return out
}

// disagreeAndResend runs the checks of a disagreement from its start to
// its re-send: the first check, the Warn eleven minutes later, the
// second check eleven minutes after it.
func (r *driftRig) disagreeAndResend() {
	r.check()
	r.clock.add(11 * time.Minute)
	r.check()
	r.clock.add(11 * time.Minute)
	r.check()
}

// wantResend checks the one room command sent from the cursor sent on and
// the re-send's event: its source (from) and mode.
func (r *driftRig) wantResend(sent testkit.Cursor, master cube.RFAddress, room uint8, temp float64, mode cube.Mode, from string) {
	r.t.Helper()
	want := setpointLine(r.t, master, room, temp, mode)
	if got := r.radioSince(sent); !slices.Equal(got, []string{want}) {
		r.t.Fatalf("the re-send: %v, want %s", got, want)
	}
	ev := r.newest("disagree")
	if summaryText(&ev, "outcome") != "resent" || summaryText(&ev, "from") != from || summaryText(&ev, "mode") != mode.String() {
		r.t.Fatalf("the re-send's event: %+v", ev.Summary)
	}
	if _, ok := ev.Summary["setpoint"]; ok != (temp != 0) {
		r.t.Fatalf("the re-send's setpoint: %+v", ev.Summary)
	}
}

// Without a recent setpoint of Massimo's, the state more than half of
// the room's thermostats show is sent once more, before the wall
// thermostat's (D188); in a room without a wall thermostat too.
func TestDisagreeResendsTheMajoritys(t *testing.T) {
	t.Run("over the wall thermostat's", func(t *testing.T) {
		r := startDrift(t, nil)
		r.set(0x0a0201, "manual", 19)
		r.set(0x0a0202, "manual", 22)
		r.set(0x0a0203, "manual", 22)
		sent := r.received.Mark()
		r.disagreeAndResend()
		r.wantResend(sent, 0x0a0201, 1, 22, cube.ModeManual, "majority")
	})
	t.Run("without a wall thermostat", func(t *testing.T) {
		r := startDrift(t, nil)
		r.set(0x0a0204, "manual", 18)
		r.set(0x0a0205, "manual", 23)
		r.set(0x0a0206, "manual", 18)
		sent := r.received.Mark()
		r.disagreeAndResend()
		r.wantResend(sent, 0x0a0204, 2, 18, cube.ModeManual, "majority")
	})
}

// Without a recent setpoint of Massimo's and without a majority, the
// room's wall thermostat's state is sent once more; without that too,
// nothing, the report alone.
func TestDisagreeResendsTheWallThermostats(t *testing.T) {
	r := startDrift(t, nil)
	r.set(0x0a0201, "manual", 19)
	r.set(0x0a0202, "manual", 20)
	r.set(0x0a0203, "manual", 22)
	sent := r.received.Mark()
	r.disagreeAndResend()
	r.wantResend(sent, 0x0a0201, 1, 19, cube.ModeManual, "wall_thermostat")

	r.set(0x0a0204, "manual", 18)
	r.set(0x0a0205, "manual", 19)
	r.set(0x0a0206, "manual", 23)
	sent = r.received.Mark()
	r.disagreeAndResend()
	if got := r.radioSince(sent); len(got) != 0 {
		t.Fatalf("sent without a state to take: %v", got)
	}
	if ev := r.newest("disagree"); summaryText(&ev, "outcome") != "not_resent" || summaryText(&ev, "why") != "no_state" || summaryText(&ev, "room") != "2" {
		t.Fatalf("the report alone: %+v", ev.Summary)
	}
}

// Massimo's setpoint in auto with a temperature is sent again as it was
// while no switch point of the programme has passed since; past one,
// auto on the programme, which takes its value now (D188). The fixture's
// programme: 21 °C from 18:20, 17 °C from 20:15.
func TestDisagreeResendsMassimosAuto(t *testing.T) {
	for _, c := range []struct {
		name string
		hhmm string  // the Cube's clock at the start, "" for 19:51
		temp float64 // the re-send's, 0 for the programme
	}{
		{"before a switch point", "121e", 23}, // 18:30, the re-send at 19:02
		{"past a switch point", "", 0},        // 19:51, the re-send at 20:23
	} {
		t.Run(c.name, func(t *testing.T) {
			r := startDrift(t, func(lines []cube.Message) []cube.Message {
				if c.hhmm == "" {
					return lines
				}
				return withCubeTime(t, lines, c.hhmm)
			})
			if code, out := do(t, r.ts, "PUT", "/api/v1/cubes/KEQ0000100/rooms/2/setpoint", `{"mode":"auto","temp":23}`); code != 200 {
				t.Fatalf("room setpoint: %d %v", code, out)
			}
			for _, rf := range []cube.RFAddress{0x0a0204, 0x0a0205, 0x0a0206} {
				r.waitSetting(rf, "auto", 23)
			}
			r.confirmed(2)
			r.clock.add(10 * time.Minute) // a person's change ten minutes later
			r.set(0x0a0205, "manual", 18)
			sent := r.received.Mark()
			r.disagreeAndResend()
			r.wantResend(sent, 0x0a0204, 2, c.temp, cube.ModeAuto, "massimo")
		})
	}
}

// withDeviceIn moves rf into room in the greeting: its metadata entry
// and its C line.
func withDeviceIn(t *testing.T, lines []cube.Message, rf cube.RFAddress, room uint8) []cube.Message {
	t.Helper()
	out := append([]cube.Message(nil), lines...)
	for i, g := range out {
		switch {
		case g.Type == 'M':
			p, err := cube.ParseMetadataPart(g.Data)
			if err != nil {
				t.Fatal(err)
			}
			md, err := cube.ParseMetadata([]cube.MetadataPart{p})
			if err != nil {
				t.Fatal(err)
			}
			for j := range md.Devices {
				if md.Devices[j].RFAddress == rf {
					md.Devices[j].RoomID = room
				}
			}
			enc, err := md.Encode()
			if err != nil {
				t.Fatal(err)
			}
			out[i] = cube.Message{Type: 'M', Data: "00,01," + base64.StdEncoding.EncodeToString(enc)}
		case g.Type == 'C' && strings.HasPrefix(g.Data, rf.String()+","):
			raw := rawConfig(t, lines, rf)
			out[i] = configLine(raw, rf, room, string(raw[8:18]))
		}
	}
	return out
}

// Members whose difference is no one's mistake are left out: a member
// in boost, a hidden one, the room while its window is open (its
// thermostats hold their own window-open temperature) and while a
// structure change of Massimo's concerns it. A window closed again
// starts the ten minutes anew.
func TestDisagreeLeavesOut(t *testing.T) {
	for _, c := range []struct {
		name  string
		apart func(r *driftRig)
	}{
		{"boost", func(r *driftRig) { r.set(0x0a0205, "boost", 21) }},
		{"hidden", func(r *driftRig) {
			r.rt.setConfig(func(c *CubeConfig) { c.Hidden = []cube.RFAddress{0x0a0205} })
			r.set(0x0a0205, "manual", 18)
		}},
		{"window", func(r *driftRig) {
			open := true
			if err := r.fc.SetState(0x0a0207, fakecube.State{WindowOpen: &open}); err != nil {
				r.t.Fatal(err)
			}
			testkit.Eventually(r.t, "the open window", func() bool {
				st, _ := r.rt.client.Snapshot().State(0x0a0207)
				return st.WindowOpen != nil && *st.WindowOpen
			})
			r.set(0x0a0205, "manual", 12)
		}},
		{"structure", func(r *driftRig) {
			f := r.rt.follow
			f.mu.Lock()
			f.watches = append(f.watches, &structureWatch{action: "move", device: 0x0a0204, rooms: []uint8{2}, next: time.Now().Add(time.Hour)})
			f.mu.Unlock()
			r.set(0x0a0205, "manual", 18)
		}},
	} {
		t.Run(c.name, func(t *testing.T) {
			r := startDrift(t, func(lines []cube.Message) []cube.Message { return withDeviceIn(t, lines, 0x0a0207, 2) })
			for _, rf := range []cube.RFAddress{0x0a0204, 0x0a0205, 0x0a0206} {
				r.set(rf, "manual", 21)
			}
			c.apart(r)
			sent := r.received.Mark()
			for range 3 {
				r.check()
				r.clock.add(11 * time.Minute)
			}
			if got := r.outcomes("disagree"); len(got) != 0 {
				t.Fatalf("reported: %v", got)
			}
			if got := r.radioSince(sent); len(got) != 0 {
				t.Fatalf("sent: %v", got)
			}
			if c.name != "window" {
				return
			}
			closed := false
			if err := r.fc.SetState(0x0a0207, fakecube.State{WindowOpen: &closed}); err != nil {
				t.Fatal(err)
			}
			testkit.Eventually(t, "the closed window", func() bool {
				st, _ := r.rt.client.Snapshot().State(0x0a0207)
				return st.WindowOpen != nil && !*st.WindowOpen
			})
			r.check()
			r.clock.add(9 * time.Minute)
			r.check()
			if got := r.outcomes("disagree"); len(got) != 0 {
				t.Fatalf("reported within ten minutes of the window's closing: %v", got)
			}
			r.clock.add(2 * time.Minute)
			r.check()
			if got := r.outcomes("disagree"); !slices.Equal(got, []string{"differ"}) {
				t.Fatalf("after the window: %v", got)
			}
		})
	}
}

// withCubeTime sets the Cube's clock in the greeting's H line, hhmm in
// hex as the Cube sends it ("121e" is 18:30).
func withCubeTime(t *testing.T, lines []cube.Message, hhmm string) []cube.Message {
	t.Helper()
	out := append([]cube.Message(nil), lines...)
	for i, g := range out {
		if g.Type == 'H' {
			f := strings.Split(g.Data, ",")
			f[8] = hhmm
			out[i].Data = strings.Join(f, ",")
			return out
		}
	}
	t.Fatal("no H line")
	return nil
}

// A member in auto that does not follow its programme while the others
// do is reported once it has not for five minutes, its clock or
// programme suspect, with the radiogram that failed and its setpoint
// the programme's an hour before; nothing is sent, and its members'
// difference is not a disagreement. Following again, the report is
// cleared. The Cube's clock starts at 18:30, the programme at 21 °C
// from 18:20 to 20:15 and at 12 °C before.
func TestDriftReported(t *testing.T) {
	r := startDrift(t, func(lines []cube.Message) []cube.Message {
		withErrorFlag(t, lines, 0x0a0206, byte(cube.RadiogramTimeInformation))
		return withCubeTime(t, lines, "121e")
	})
	r.set(0x0a0204, "auto", 21)
	r.set(0x0a0205, "auto", 21)
	r.set(0x0a0206, "auto", 12)
	sent := r.received.Mark()
	r.check()
	r.clock.add(4 * time.Minute)
	r.check()
	if got := r.outcomes("drift"); len(got) != 0 {
		t.Fatalf("reported within five minutes: %v", got)
	}
	r.clock.add(2 * time.Minute)
	r.check()
	evs := r.events("drift")
	if len(evs) != 1 {
		t.Fatalf("drift events %+v", evs)
	}
	ev := evs[0]
	fr, _ := ev.Summary["failed_radiogram"].(map[string]any)
	if ev.Level != "warn" || ev.Device != "0a0206" || ev.Serial != "MEQ0000206" || summaryText(&ev, "outcome") != "not_following" ||
		summaryText(&ev, "setpoint") != "12" || summaryText(&ev, "expected") != "21" || summaryText(&ev, "switch_at") != "18:20" ||
		summaryText(&ev, "hour_off") != "true" || summaryText(&ev, "auto_dst") != "true" || summaryText(&ev, "others") != "[0a0204 0a0205]" ||
		fr["name"] != "TimeInformation" || fr["code"] != float64(3) {
		t.Fatalf("the drift report: %+v", ev)
	}
	for range 3 {
		r.clock.add(11 * time.Minute)
		r.check()
	}
	if got := r.outcomes("disagree"); len(got) != 0 {
		t.Fatalf("a drift taken for a disagreement: %v", got)
	}
	if got := r.outcomes("drift"); len(got) != 1 {
		t.Fatalf("reported twice: %v", got)
	}
	// The flagged member's report may bring its automatic reset (AP
	// RadioHealth v1.2, Block 4), an r: of its own; a correction would be
	// an s: line.
	var corrections []string
	for _, l := range r.radioSince(sent) {
		if !strings.HasPrefix(l, "r:") {
			corrections = append(corrections, l)
		}
	}
	if len(corrections) != 0 {
		t.Fatalf("a drift corrected: %v", corrections)
	}
	r.set(0x0a0206, "auto", 21)
	r.check()
	if got := r.outcomes("drift"); !slices.Equal(got, []string{"not_following", "follows"}) {
		t.Fatalf("outcomes %v", got)
	}
	if alerts := alertKinds(t, r.ts); slices.Contains(alerts, "drift") {
		t.Fatalf("the drift report not settled: %v", alerts)
	}
}

// A switch point just past is no drift: a member still at the
// temperature before it follows for five minutes; then the one member
// left behind is reported, the others having followed.
func TestDriftAfterASwitchPoint(t *testing.T) {
	r := startDrift(t, nil)
	for _, rf := range []cube.RFAddress{0x0a0204, 0x0a0205, 0x0a0206} {
		r.set(rf, "auto", 21)
	}
	r.clock.add(26 * time.Minute) // the Cube's 20:17, two minutes after the switch to 17 °C
	r.check()
	r.clock.add(2 * time.Minute)
	r.check()
	r.clock.add(4 * time.Minute) // 20:23: none has followed, so none is the odd one
	r.check()
	r.clock.add(6 * time.Minute)
	r.check()
	if got := r.outcomes("drift"); len(got) != 0 {
		t.Fatalf("reported: %v", got)
	}
	r.set(0x0a0204, "auto", 17)
	r.set(0x0a0205, "auto", 17)
	r.check()
	r.clock.add(6 * time.Minute)
	r.check()
	evs := r.events("drift")
	if len(evs) != 1 || evs[0].Device != "0a0206" || summaryText(&evs[0], "expected") != "17" || summaryText(&evs[0], "switch_at") != "20:15" {
		t.Fatalf("the drift report: %+v", evs)
	}
	if _, ok := evs[0].Summary["failed_radiogram"]; ok {
		t.Fatalf("a failed radiogram without the flag: %+v", evs[0])
	}
}

// The rules of the comparison: two members in auto that follow their
// own programmes agree whatever their setpoints; one that does not is
// the room's odd one only while every other follows; a command to a
// room is found by its room, its target or a line to a member.
func TestRoomCheckRules(t *testing.T) {
	sp := func(v float64) *float64 { return &v }
	member := func(rf cube.RFAddress, mode cube.Mode, set float64, judged, follows bool) roomMember {
		st := cube.DeviceState{RFAddress: rf, Valid: true, Mode: mode, Setpoint: sp(set)}
		return roomMember{rf: rf, st: st, state: view.StateOf(st), judged: judged, follows: follows}
	}
	a := member(1, cube.ModeAuto, 21, true, true)
	b := member(2, cube.ModeAuto, 19, true, true)
	c := member(3, cube.ModeAuto, 17, true, false)
	m := member(4, cube.ModeManual, 21, false, false)
	if disagrees([]roomMember{a, b}) {
		t.Error("two members on their own programmes disagree")
	}
	if !disagrees([]roomMember{a, m}) || !disagrees([]roomMember{a, c}) {
		t.Error("a manual member, or one off its programme, agrees")
	}
	if odd := drifting([]roomMember{a, b, c}); odd == nil || odd.rf != 3 {
		t.Errorf("the odd one: %+v", odd)
	}
	c2 := member(5, cube.ModeAuto, 23, true, false)
	if drifting([]roomMember{a, c, c2}) != nil || drifting([]roomMember{a, m, c}) != nil || drifting([]roomMember{c}) != nil {
		t.Error("an odd one where not every other member follows")
	}
	// The majority (D188): more than half, two on their own programmes
	// as one, auto on the programme; none of two that differ.
	m2 := member(6, cube.ModeManual, 21, false, false)
	for _, c := range []struct {
		ms   []roomMember
		ok   bool
		mode cube.Mode
		temp float64
	}{
		{[]roomMember{a, b, m}, true, cube.ModeAuto, 0},
		{[]roomMember{m, c, m2}, true, cube.ModeManual, 21},
		{[]roomMember{a, m}, false, 0, 0},
		{[]roomMember{a, c, m}, false, 0, 0},
	} {
		r, ok := majority(2, c.ms)
		if ok != c.ok || ok && (r.mode != c.mode || r.temp != c.temp || r.from != fromMajority) {
			t.Errorf("the majority of %d: %+v %v", len(c.ms), r, ok)
		}
	}
	// Massimo's auto with a temperature is shown by a member on its
	// programme too; a switch point passed is one of a member's
	// programme, or a new day.
	in := roomIntent{mode: cube.ModeAuto, temp: 23}
	if !in.shownBy(a) || in.shownBy(c) || !in.shownBy(member(7, cube.ModeAuto, 23, true, false)) {
		t.Error("auto 23 shown by the wrong members")
	}
	var w cube.WeekProgram
	for d := range w {
		w[d] = cube.DayProgram{{Temp: 17, Until: 6 * 60}, {Temp: 21, Until: 22 * 60}, {Temp: 17, Until: 24 * 60}}
	}
	p := member(8, cube.ModeAuto, 21, true, true)
	p.program = &w
	at := func(hh, mm int) time.Time { return time.Date(2026, 11, 17, hh, mm, 0, 0, time.UTC) }
	for _, c := range []struct {
		then, now time.Time
		want      bool
	}{
		{at(21, 40), at(21, 59), false},
		{at(21, 50), at(22, 5), true},
		{at(23, 50), at(23, 59).Add(10 * time.Minute), true},
		{at(22, 5), at(21, 50), false},
	} {
		if got := switchedSince([]roomMember{a, p}, c.then, c.now); got != c.want {
			t.Errorf("switched between %s and %s: %v", c.then.Format("15:04"), c.now.Format("Jan 2 15:04"), got)
		}
	}

	members := map[cube.RFAddress]bool{0x0a0204: true}
	match := toRoom(2, members)
	line, _ := cube.CmdSetTemperature(cube.Target{RFAddress: 0x0a0301, RoomID: 2, Room: true}, 21, cube.ModeManual, cube.Until{})
	other, _ := cube.CmdSetTemperature(cube.Target{RFAddress: 0x0a0301, RoomID: 3, Room: true}, 21, cube.ModeManual, cube.Until{})
	for _, cmd := range []engine.Command{{Lines: []cube.Message{line}}, {Target: 0x0a0204}, {RoomID: 2, Wakeup: true}} {
		if !match(cmd) {
			t.Errorf("not to the room: %+v", cmd)
		}
	}
	for _, cmd := range []engine.Command{{Lines: []cube.Message{other}}, {Reconnect: true}, {Target: 0x0a0301}} {
		if match(cmd) {
			t.Errorf("to the room: %+v", cmd)
		}
	}
}
