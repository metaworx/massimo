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
	*rig
	clock *checkClock
}

// startDrift starts a driftRig: the rooms fixture (room 1 a wall
// thermostat and two radiator thermostats, room 2 three radiator
// thermostats, room 3 a window contact; every thermostat in auto at
// 20 °C; the Cube's clock Tuesday 19:51, when the radiator thermostats'
// programme says 21 °C since 18:20 and 17 °C from 20:15), change
// applied to its lines first, and edit to the server's options, with a
// store, polled often. The fake Cube applies the commands it takes.
func startDrift(t *testing.T, change func([]cube.Message) []cube.Message, edit ...func(*Options)) *driftRig {
	t.Helper()
	return startDriftWith(t, change, nil, edit...)
}

// startDriftWith is startDrift with the fake Cube's states set before
// it starts, after change: values there at the first connection, whose
// age the snapshot does not know (cubelan.StateTimes).
func startDriftWith(t *testing.T, change func([]cube.Message) []cube.Message, states map[cube.RFAddress]fakecube.State, edit ...func(*Options)) *driftRig {
	t.Helper()
	clock := &checkClock{}
	opts := []startOpt{rooms(), stored(), withOptions(Options{PollInterval: 50 * time.Millisecond, driftClock: clock.now, driftEvery: -1})}
	if change != nil {
		opts = append(opts, changeGreeting(change))
	}
	if states != nil {
		opts = append(opts, withFake(func(fc *fakecube.Server) {
			for rf, st := range states {
				if err := fc.SetState(rf, st); err != nil {
					t.Fatal(err)
				}
			}
		}))
	}
	for _, e := range edit {
		opts = append(opts, editOptions(e))
	}
	return &driftRig{rig: startServer(t, opts...), clock: clock}
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
	return lineStrings(r.lines.Since(from, cubewatch.Type('s', 'r')))
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
	t.Parallel()
	r := startDrift(t, nil)
	if code, out := do(t, r.ts, "PUT", "/api/v1/cubes/KEQ0000100/rooms/2/setpoint", `{"mode":"manual","temp":21}`); code != 200 {
		t.Fatalf("room setpoint: %d %v", code, out)
	}
	for _, rf := range []cube.RFAddress{0x0a0204, 0x0a0205, 0x0a0206} {
		r.waitSetting(rf, "manual", 21)
	}
	r.confirmed(2)
	sent := r.lines.Mark()
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

// A disagreement not yet reported starts again while a write to the
// room confirms (AP TestInfrastructure, D19-A): roomChecks leaves a room
// out while a member has not shown a write to it (roomConfirmation), and
// pause drops what the checks had counted of a disagreement not yet
// reported, so that its ten minutes start at the first check after every
// member showed the write.
// Here a member differs before a room setpoint of Massimo's and does not
// take it; the members differ for eleven minutes while the write
// confirms, and nothing is reported. Once the write is confirmed they
// still differ, and the Warn and the re-send of Massimo's setpoint come
// ten and twenty minutes after that check, as in
// TestDisagreeResendsMassimosSetpoint.
func TestDisagreeStartsAgainOnceTheWriteConfirms(t *testing.T) {
	t.Parallel()
	r := startDrift(t, nil)
	r.set(0x0a0205, "manual", 18)
	r.check() // the disagreement begins

	r.fc.SetDeaf(0x0a0205, true) // 0a0205 does not take the write
	r.setRoom2("21")
	r.waitSetting(0x0a0204, "manual", 21)
	r.waitSetting(0x0a0206, "manual", 21)
	r.rt.pumpSync(t)
	if c := r.rt.roomConfirmation(2); c == nil || c.State != confirmPending || !slices.Equal(c.Devices, []string{"0a0205"}) {
		t.Fatalf("the room's confirmation: %+v", c)
	}
	sent := r.lines.Mark()
	r.check()
	r.clock.add(6 * time.Minute)
	r.check()
	r.clock.add(5 * time.Minute)
	r.check()
	if got := r.outcomes("disagree"); len(got) != 0 {
		t.Fatalf("reported while the write confirms, eleven minutes after the disagreement began: %v", got)
	}

	// 0a0206 is set to 18 by hand, and 0a0205 takes the write at last:
	// the members still differ, the write is confirmed.
	r.set(0x0a0206, "manual", 18)
	r.fc.SetDeaf(0x0a0205, false)
	r.set(0x0a0205, "manual", 21)
	r.confirmed(2)
	r.check() // the disagreement starts again
	r.clock.add(9 * time.Minute)
	r.check()
	if got := r.outcomes("disagree"); len(got) != 0 {
		t.Fatalf("reported nine minutes after the write was confirmed: %v", got)
	}
	r.clock.add(2 * time.Minute)
	r.check()
	evs := r.events("disagree")
	if len(evs) != 1 || evs[0].Level != "warn" || summaryText(&evs[0], "outcome") != "differ" || summaryText(&evs[0], "room") != "2" ||
		summaryText(&evs[0], "minutes") != "11" || summaryText(&evs[0], "differs") != "[setpoint]" ||
		!strings.Contains(summaryText(&evs[0], "members"), "rf:0a0206 setpoint:18") {
		t.Fatalf("the report: %+v", evs)
	}
	r.clock.add(9 * time.Minute)
	r.check()
	if got := r.radioSince(sent); len(got) != 0 {
		t.Fatalf("sent before the second check: %v", got)
	}
	r.clock.add(2 * time.Minute)
	r.check()
	r.wantResend(sent, 0x0a0204, 2, 21, cube.ModeManual, fromMassimo)
	r.waitSetting(0x0a0206, "manual", 21)
	r.confirmed(2)
	r.clock.add(time.Minute)
	r.check()
	if got := r.outcomes("disagree"); !slices.Equal(got, []string{"differ", "resent", "agree"}) {
		t.Fatalf("outcomes %v", got)
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

// wantResend checks the one room command sent from the cursor sent on,
// named to master, and the re-send's event: its source (from) and mode,
// and no before_change.
func (r *driftRig) wantResend(sent testkit.Cursor, master cube.RFAddress, room uint8, temp float64, mode cube.Mode, from string) {
	r.t.Helper()
	r.wantResendOf(sent, master, room, temp, mode, from, false)
}

// wantResendOf is wantResend of a state held before a structure change
// where held says so: its event then carries before_change.
func (r *driftRig) wantResendOf(sent testkit.Cursor, master cube.RFAddress, room uint8, temp float64, mode cube.Mode, from string, held bool) {
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
	if v, ok := ev.Summary["before_change"]; ok != held || ok && v != true {
		r.t.Fatalf("the re-send's before_change: %+v", ev.Summary)
	}
}

// Without a recent setpoint of Massimo's, the state more than half of
// the room's thermostats show is sent once more, before the wall
// thermostat's (D188); in a room without a wall thermostat too.
func TestDisagreeResendsTheMajoritys(t *testing.T) {
	t.Parallel()
	t.Run("over the wall thermostat's", func(t *testing.T) {
		t.Parallel()
		r := startDrift(t, nil)
		r.set(0x0a0201, "manual", 19)
		r.set(0x0a0202, "manual", 22)
		r.set(0x0a0203, "manual", 22)
		sent := r.lines.Mark()
		r.disagreeAndResend()
		r.wantResend(sent, 0x0a0201, 1, 22, cube.ModeManual, "majority")
	})
	t.Run("without a wall thermostat", func(t *testing.T) {
		t.Parallel()
		r := startDrift(t, nil)
		r.set(0x0a0204, "manual", 18)
		r.set(0x0a0205, "manual", 23)
		r.set(0x0a0206, "manual", 18)
		sent := r.lines.Mark()
		r.disagreeAndResend()
		r.wantResend(sent, 0x0a0204, 2, 18, cube.ModeManual, "majority")
	})
}

// Without a recent setpoint of Massimo's, without a majority and with
// the members' states from before the first connection, whose age is
// not known, the state of the first of the room's order is sent once
// more (RoomMajority.D9): its master's, here room 2's last radiator
// thermostat; with the master dead, passed over, its wall thermostat's;
// with neither, its first radiator thermostat's. The room command names
// the first of the order too, the dead passed over.
func TestDisagreeResendsByTheRoomsOrder(t *testing.T) {
	t.Parallel()
	manual := func(set float64) fakecube.State {
		mode := "manual"
		return fakecube.State{Mode: &mode, Setpoint: &set}
	}
	off := false
	dead := fakecube.State{Reported: &off}
	for _, c := range []struct {
		name   string
		master cube.RFAddress // the room's master in the metadata, 0 to keep the fixture's
		states map[cube.RFAddress]fakecube.State
		room   uint8
		to     cube.RFAddress // the addressee
		temp   float64
		from   string
	}{
		{"the master's", 0x0a0206, map[cube.RFAddress]fakecube.State{0x0a0204: manual(18), 0x0a0205: manual(19), 0x0a0206: manual(23)},
			2, 0x0a0206, 23, "master"},
		{"the wall thermostat's, the master dead", 0x0a0203, map[cube.RFAddress]fakecube.State{0x0a0201: manual(19), 0x0a0202: manual(22), 0x0a0203: dead},
			1, 0x0a0201, 19, "wall_thermostat"},
		{"a radiator thermostat's, the master dead", 0, map[cube.RFAddress]fakecube.State{0x0a0204: dead, 0x0a0205: manual(19), 0x0a0206: manual(23)},
			2, 0x0a0205, 19, "radiator_thermostat"},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			var change func([]cube.Message) []cube.Message
			if c.master != 0 {
				change = func(lines []cube.Message) []cube.Message { withRoomMaster(t, lines, c.room, c.master); return lines }
			}
			r := startDriftWith(t, change, c.states)
			for rf, st := range c.states {
				if st.Setpoint != nil {
					r.waitSetting(rf, "manual", *st.Setpoint)
				}
			}
			sent := r.lines.Mark()
			r.disagreeAndResend()
			r.wantResend(sent, c.to, c.room, c.temp, cube.ModeManual, c.from)
		})
	}
}

// Without a recent setpoint of Massimo's and without a majority, the
// state of the member whose mode or setpoint changed last is sent once
// more (RoomMajority.D3-A), before the master's: a change by hand at a
// device is the newest wish.
func TestDisagreeResendsTheNewestChange(t *testing.T) {
	t.Parallel()
	r := startDrift(t, nil)
	r.set(0x0a0204, "manual", 18)
	r.set(0x0a0205, "manual", 19)
	r.set(0x0a0206, "manual", 23)
	sent := r.lines.Mark()
	r.disagreeAndResend()
	r.wantResend(sent, 0x0a0204, 2, 23, cube.ModeManual, "newest")
}

// The newest change as D7-C counts it: in auto a setpoint set by hand
// holds only until the programme's next switch point, so a member
// whose setpoint a switch point changed later wins over one set by
// hand in auto, and its programme is sent; against a member in manual
// a member on its programme counts its mode's time alone, so the
// manual setpoint wins although the switch point came after it. The
// fixture's programme says 21 °C at the Cube's 19:51, the members are
// in auto at 20 °C since the first connection.
func TestDisagreeNewestByTheProgramme(t *testing.T) {
	t.Parallel()
	t.Run("a switch point beats a hand change in auto", func(t *testing.T) {
		t.Parallel()
		r := startDrift(t, nil)
		r.set(0x0a0204, "auto", 23) // by hand, in auto
		r.set(0x0a0205, "auto", 22) // by hand, in auto
		r.set(0x0a0206, "auto", 21) // the programme's
		sent := r.lines.Mark()
		r.disagreeAndResend()
		r.wantResend(sent, 0x0a0204, 2, 0, cube.ModeAuto, "newest")
	})
	t.Run("not one in manual", func(t *testing.T) {
		t.Parallel()
		r := startDrift(t, func(lines []cube.Message) []cube.Message { return withDeviceIn(t, lines, 0x0a0206, 3) })
		r.set(0x0a0205, "manual", 18) // by hand, in manual
		r.set(0x0a0204, "auto", 21)   // the programme's, later; its mode from the first connection
		sent := r.lines.Mark()
		r.disagreeAndResend()
		r.wantResend(sent, 0x0a0204, 2, 18, cube.ModeManual, "newest")
	})
}

// A state held for a room before a structure change brought a newcomer
// into it (holdReference, RoomMajority.D8-A) is sent once more in place
// of the members' majority, marked before_change: here the 2026-11-23
// case, a room of a wall thermostat and a radiator thermostat in manual
// at 5 °C, whose wall thermostat then went to auto, the newest change
// and the room's master. Massimo's own setpoint still comes first
// (D4-A). Members that agree on any state drop the held one, as does a
// day (resendGap).
func TestDisagreeResendsTheStateBeforeAChange(t *testing.T) {
	t.Parallel()
	held := resend{mode: cube.ModeManual, temp: 5, from: fromMajority}
	t.Run("a room of two that differ", func(t *testing.T) {
		t.Parallel()
		r := startDrift(t, func(lines []cube.Message) []cube.Message { return withDeviceIn(t, lines, 0x0a0203, 3) })
		r.set(0x0a0201, "manual", 5)
		r.set(0x0a0202, "manual", 5)
		ref, ok := r.rt.roomReference(r.rt.client.Snapshot(), 1, r.s.roomNow())
		if !ok || ref.mode != cube.ModeManual || ref.temp != 5 || ref.from != fromMajority || ref.room != 1 {
			t.Fatalf("the room's reference: %+v %v", ref, ok)
		}
		r.rt.holdReference(1, ref, r.s.roomNow())
		r.set(0x0a0201, "auto", 21)
		sent := r.lines.Mark()
		r.disagreeAndResend()
		r.wantResendOf(sent, 0x0a0201, 1, 5, cube.ModeManual, "majority", true)
	})
	t.Run("Massimo's setpoint first", func(t *testing.T) {
		t.Parallel()
		r := startDrift(t, nil)
		r.setRoom2("21")
		for _, rf := range []cube.RFAddress{0x0a0204, 0x0a0205, 0x0a0206} {
			r.waitSetting(rf, "manual", 21)
		}
		r.confirmed(2)
		r.clock.add(time.Second)
		r.rt.holdReference(2, held, r.s.roomNow()) // after the setpoint, which keeps it
		r.set(0x0a0205, "manual", 18)
		sent := r.lines.Mark()
		r.disagreeAndResend()
		r.wantResend(sent, 0x0a0204, 2, 21, cube.ModeManual, "massimo")
	})
	t.Run("dropped once the members agree", func(t *testing.T) {
		t.Parallel()
		r := startDrift(t, nil)
		r.rt.holdReference(2, held, r.s.roomNow())
		r.clock.add(time.Second)
		r.check() // they agree, in auto at 20 °C
		r.set(0x0a0204, "manual", 18)
		r.set(0x0a0205, "manual", 19)
		r.set(0x0a0206, "manual", 23)
		sent := r.lines.Mark()
		r.disagreeAndResend()
		r.wantResend(sent, 0x0a0204, 2, 23, cube.ModeManual, "newest")
	})
	// Held auto with a temperature, which lasts until the next switch
	// point (20:15 here), is sent as it was before one, and as auto on
	// the programme once one has passed since it was held.
	for _, c := range []struct {
		name  string
		after time.Duration // from the hold to the first check
		temp  float64
	}{{"auto with a temperature before a switch point", 0, 23}, {"auto with a temperature past a switch point", 5 * time.Minute, 0}} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			r := startDrift(t, nil)
			r.set(0x0a0204, "manual", 18)
			r.set(0x0a0205, "manual", 19)
			r.rt.holdReference(2, resend{mode: cube.ModeAuto, temp: 23, from: fromMajority}, r.s.roomNow())
			r.clock.add(c.after)
			sent := r.lines.Mark()
			r.disagreeAndResend()
			r.wantResendOf(sent, 0x0a0204, 2, c.temp, cube.ModeAuto, "majority", true)
		})
	}
	t.Run("dropped after a day", func(t *testing.T) {
		t.Parallel()
		r := startDrift(t, nil)
		r.set(0x0a0204, "manual", 18)
		r.set(0x0a0205, "manual", 19)
		r.set(0x0a0206, "manual", 23)
		r.rt.holdReference(2, held, r.s.roomNow())
		r.clock.add(resendGap)
		sent := r.lines.Mark()
		r.disagreeAndResend()
		r.wantResend(sent, 0x0a0204, 2, 23, cube.ModeManual, "newest")
	})
}

// Massimo's setpoint in auto with a temperature is sent again as it was
// while no switch point of the programme has passed since; past one,
// auto on the programme, which takes its value now (D188). The fixture's
// programme: 21 °C from 18:20, 17 °C from 20:15.
func TestDisagreeResendsMassimosAuto(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name string
		hhmm string  // the Cube's clock at the start, "" for 19:51
		temp float64 // the re-send's, 0 for the programme
	}{
		{"before a switch point", "121e", 23}, // 18:30, the re-send at 19:02
		{"past a switch point", "", 0},        // 19:51, the re-send at 20:23
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
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
			sent := r.lines.Mark()
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
	t.Parallel()
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
			t.Parallel()
			r := startDrift(t, func(lines []cube.Message) []cube.Message { return withDeviceIn(t, lines, 0x0a0207, 2) })
			for _, rf := range []cube.RFAddress{0x0a0204, 0x0a0205, 0x0a0206} {
				r.set(rf, "manual", 21)
			}
			c.apart(r)
			sent := r.lines.Mark()
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
	t.Parallel()
	r := startDrift(t, func(lines []cube.Message) []cube.Message {
		withErrorFlag(t, lines, 0x0a0206, byte(cube.RadiogramTimeInformation))
		return withCubeTime(t, lines, "121e")
	})
	r.set(0x0a0204, "auto", 21)
	r.set(0x0a0205, "auto", 21)
	r.set(0x0a0206, "auto", 12)
	sent := r.lines.Mark()
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
	t.Parallel()
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
	t.Parallel()
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

// The newest change's rules (RoomMajority.D3-A, D7-C), on members made
// by hand: a member's time is the later of its mode's and its
// setpoint's; against a member in manual or on holiday, one that
// follows its programme counts its mode's time alone; a time not known
// is none; the newest time shared by members that agree is theirs, by
// members that differ none's.
func TestNewestRules(t *testing.T) {
	t.Parallel()
	sp := func(v float64) *float64 { return &v }
	t0 := time.Date(2026, 11, 23, 12, 0, 0, 0, time.UTC)
	at := func(m int) time.Time { return t0.Add(time.Duration(m) * time.Minute) }
	var none time.Time
	until := cube.Until{Year: 2026, Month: 11, Day: 30, Minutes: 12 * 60}
	member := func(rf cube.RFAddress, mode cube.Mode, set float64, follows bool, modeAt, setAt time.Time) roomMember {
		st := cube.DeviceState{RFAddress: rf, Valid: true, Mode: mode, Setpoint: sp(set)}
		if mode == cube.ModeVacation {
			st.Until = &until
		}
		return roomMember{rf: rf, typ: cube.DeviceRadiatorThermostat, st: st, state: view.StateOf(st), judged: mode == cube.ModeAuto,
			follows: follows, modeAt: modeAt, setpointAt: setAt}
	}
	for _, c := range []struct {
		name string
		ms   []roomMember
		ok   bool
		mode cube.Mode
		temp float64 // 0 with auto: the programme
	}{
		{"the later of mode and setpoint", []roomMember{member(1, cube.ModeManual, 18, false, at(1), at(1)), member(2, cube.ModeAuto, 23, false, none, at(2))},
			true, cube.ModeAuto, 23},
		{"a switch point against a hand change in auto", []roomMember{member(1, cube.ModeAuto, 23, false, none, at(1)), member(2, cube.ModeAuto, 21, true, none, at(2))},
			true, cube.ModeAuto, 0},
		{"a switch point against manual: its mode's time alone", []roomMember{member(1, cube.ModeManual, 18, false, at(1), at(1)), member(2, cube.ModeAuto, 21, true, none, at(2))},
			true, cube.ModeManual, 18},
		{"against a holiday too", []roomMember{member(1, cube.ModeVacation, 15, false, at(1), at(1)), member(2, cube.ModeAuto, 21, true, none, at(2))},
			true, cube.ModeVacation, 15},
		{"its mode set later", []roomMember{member(1, cube.ModeManual, 18, false, at(1), at(1)), member(2, cube.ModeAuto, 21, true, at(3), at(2))},
			true, cube.ModeAuto, 0},
		{"no time known", []roomMember{member(1, cube.ModeManual, 18, false, none, none), member(2, cube.ModeManual, 19, false, none, none)},
			false, 0, 0},
		{"the newest shared by two that agree", []roomMember{member(1, cube.ModeManual, 18, false, at(2), at(2)), member(2, cube.ModeManual, 18, false, at(2), at(2)),
			member(3, cube.ModeManual, 20, false, at(1), at(1))}, true, cube.ModeManual, 18},
		{"the newest shared by two that differ", []roomMember{member(1, cube.ModeManual, 18, false, at(2), at(2)), member(2, cube.ModeManual, 19, false, at(2), at(2)),
			member(3, cube.ModeManual, 20, false, at(1), at(1))}, false, 0, 0},
	} {
		r, ok := newest(2, c.ms)
		if ok != c.ok || ok && (r.mode != c.mode || r.temp != c.temp || r.from != fromNewest || r.room != 2) {
			t.Errorf("%s: %+v %v, want %s %v", c.name, r, ok, c.mode, c.temp)
		}
	}
}

// hold is a copy of the state held for room, nil for none.
func (r *driftRig) hold(room uint8) *heldReference {
	d := &r.rt.drift
	d.mu.Lock()
	defer d.mu.Unlock()
	if h := d.holds[room]; h != nil {
		cp := *h
		return &cp
	}
	return nil
}

// pauseRoom leaves room out of the room checks for an hour, as a
// structure change of Massimo's that is read back does (structureRooms).
func (r *driftRig) pauseRoom(room uint8) {
	f := r.rt.follow
	f.mu.Lock()
	defer f.mu.Unlock()
	f.watches = append(f.watches, &structureWatch{action: "move", device: 0x0a0204, rooms: []uint8{room}, next: time.Now().Add(time.Hour)})
}

// How long a state held for a room lasts (holdReference): a check that
// finds the room's thermostats agreeing drops it where two or more are
// compared or every thermostat of the room is, one included, and keeps
// it where a thermostat is left out; roomReference gives it again only
// while no check has judged the room since it was held, and its time
// stays; a newer setpoint of Massimo's drops it, but for the held state
// itself, and so does a change by hand at a member after the first
// check that judged it, but for one to the held state; a check that
// scanned before the hold leaves it alone.
func TestHeldReferenceLifetime(t *testing.T) {
	t.Parallel()
	held := resend{mode: cube.ModeManual, temp: 5, from: fromMajority}
	t.Run("a room of one thermostat drops it once it agrees", func(t *testing.T) {
		t.Parallel()
		r := startDrift(t, func(lines []cube.Message) []cube.Message {
			return withDeviceIn(t, withDeviceIn(t, lines, 0x0a0202, 3), 0x0a0203, 3)
		})
		r.rt.holdReference(1, held, r.s.roomNow())
		r.clock.add(time.Second)
		r.check()
		if h := r.hold(1); h != nil {
			t.Fatalf("kept in a room of one thermostat: %+v", h)
		}
	})
	t.Run("a thermostat left out keeps it, and a judged one is not given again", func(t *testing.T) {
		t.Parallel()
		off := false
		r := startDriftWith(t, func(lines []cube.Message) []cube.Message { return withDeviceIn(t, lines, 0x0a0203, 3) },
			map[cube.RFAddress]fakecube.State{0x0a0202: {Reported: &off}})
		r.set(0x0a0201, "manual", 5)
		ref, ok := r.rt.roomReference(r.rt.client.Snapshot(), 1, r.s.roomNow())
		if !ok || ref.mode != cube.ModeManual || ref.temp != 5 {
			t.Fatalf("the room's reference: %+v %v", ref, ok)
		}
		r.rt.holdReference(1, ref, r.s.roomNow())
		for _, d := range []time.Duration{time.Second, 2 * time.Hour} {
			r.clock.add(d)
			r.check()
		}
		if h := r.hold(1); h == nil || !h.judged {
			t.Fatalf("a hold with a thermostat left out: %+v", h)
		}
		r.set(0x0a0201, "manual", 21) // by hand
		ref, ok = r.rt.roomReference(r.rt.client.Snapshot(), 1, r.s.roomNow())
		if !ok || ref.temp != 21 || ref.beforeChange {
			t.Fatalf("the room's reference after a check judged the hold: %+v %v", ref, ok)
		}
	})
	t.Run("given again while the room is left out, its time kept", func(t *testing.T) {
		t.Parallel()
		r := startDrift(t, nil)
		at := r.s.roomNow()
		r.rt.holdReference(2, held, at)
		r.pauseRoom(2)
		r.clock.add(time.Hour)
		r.check()
		ref, ok := r.rt.roomReference(r.rt.client.Snapshot(), 2, r.s.roomNow())
		if !ok || ref.mode != cube.ModeManual || ref.temp != 5 || !ref.beforeChange {
			t.Fatalf("the reference of a room left out: %+v %v", ref, ok)
		}
		r.rt.holdReference(2, ref, r.s.roomNow())
		if h := r.hold(2); h == nil || !h.at.Equal(at) || h.judged {
			t.Fatalf("the hold given again: %+v, held at %v", h, at)
		}
	})
	t.Run("a newer setpoint of Massimo's drops it, the held state's own keeps it", func(t *testing.T) {
		t.Parallel()
		r := startDrift(t, nil)
		r.rt.holdReference(2, held, r.s.roomNow())
		r.clock.add(time.Second)
		r.setRoom2("21")
		if h := r.hold(2); h != nil {
			t.Fatalf("kept after a newer setpoint: %+v", h)
		}
		for _, rf := range []cube.RFAddress{0x0a0204, 0x0a0205, 0x0a0206} {
			r.waitSetting(rf, "manual", 21)
		}
		r.confirmed(2)
		r.rt.holdReference(2, held, r.s.roomNow())
		r.clock.add(time.Second)
		r.setRoom2("5")
		if h := r.hold(2); h == nil {
			t.Fatal("the held state sent as a setpoint dropped it")
		}
	})
	t.Run("a change by hand after it was judged drops it, one to the held state keeps it", func(t *testing.T) {
		t.Parallel()
		r := startDrift(t, nil)
		r.set(0x0a0204, "manual", 18)
		r.rt.holdReference(2, held, r.s.roomNow())
		r.clock.add(time.Second)
		r.check() // judged: they differ
		r.set(0x0a0205, "manual", 5)
		r.set(0x0a0206, "manual", 5)
		r.clock.add(time.Minute)
		r.check()
		if h := r.hold(2); h == nil {
			t.Fatal("members that took the held state dropped it")
		}
		r.set(0x0a0204, "manual", 7)
		r.clock.add(time.Minute)
		r.check()
		if h := r.hold(2); h != nil {
			t.Fatalf("kept after a change by hand: %+v", h)
		}
	})
	t.Run("a later change by hand wins over it, a thermostat left out back", func(t *testing.T) {
		t.Parallel()
		off, on := false, true
		r := startDriftWith(t, func(lines []cube.Message) []cube.Message { return withDeviceIn(t, lines, 0x0a0203, 3) },
			map[cube.RFAddress]fakecube.State{0x0a0202: {Reported: &off}})
		r.set(0x0a0201, "manual", 5)
		ref, ok := r.rt.roomReference(r.rt.client.Snapshot(), 1, r.s.roomNow())
		if !ok {
			t.Fatal("no reference")
		}
		r.rt.holdReference(1, ref, r.s.roomNow())
		for _, d := range []time.Duration{time.Second, 3 * time.Hour} {
			r.clock.add(d)
			r.check()
		}
		r.set(0x0a0201, "manual", 21) // by hand
		if err := r.fc.SetState(0x0a0202, fakecube.State{Reported: &on}); err != nil {
			t.Fatal(err)
		}
		testkit.Eventually(t, "0a0202 back", func() bool {
			st, ok := r.rt.client.Snapshot().State(0x0a0202)
			return ok && st.Valid
		})
		sent := r.lines.Mark()
		r.disagreeAndResend()
		r.wantResend(sent, 0x0a0201, 1, 21, cube.ModeManual, "newest")
	})
	t.Run("a check that scanned before it leaves it", func(t *testing.T) {
		t.Parallel()
		r := startDrift(t, nil)
		r.rt.holdReference(2, held, r.s.roomNow().Add(time.Minute)) // held after the check's start
		r.check()                                                   // they agree, in auto at 20 °C
		if h := r.hold(2); h == nil || h.judged {
			t.Fatalf("the hold of a check that came before it: %+v", h)
		}
	})
}

// A held state's re-send goes out once even where the room had a
// re-send that day, the one radiogram its structure change costs; a
// second one is held to the day's limit. 0a0205 does not take the held
// re-send, so that the room still disagrees after it, with no change by
// hand that would drop the hold, and no check can find it agreeing.
func TestHeldResendPastTheDaysLimit(t *testing.T) {
	t.Parallel()
	r := startDrift(t, nil)
	r.set(0x0a0204, "manual", 18)
	r.set(0x0a0205, "manual", 19)
	r.set(0x0a0206, "manual", 23)
	sent := r.lines.Mark()
	r.disagreeAndResend()
	r.wantResend(sent, 0x0a0204, 2, 23, cube.ModeManual, "newest")
	for _, rf := range []cube.RFAddress{0x0a0204, 0x0a0205, 0x0a0206} {
		r.waitSetting(rf, "manual", 23)
	}
	r.clock.add(time.Minute)
	r.check() // they agree
	r.rt.holdReference(2, resend{mode: cube.ModeManual, temp: 5, from: fromMajority}, r.s.roomNow())
	r.set(0x0a0205, "manual", 18)
	r.fc.SetDeaf(0x0a0205, true)
	sent = r.lines.Mark()
	r.disagreeAndResend()
	r.wantResendOf(sent, 0x0a0204, 2, 5, cube.ModeManual, "majority", true)
	r.waitSetting(0x0a0204, "manual", 5)
	r.waitSetting(0x0a0206, "manual", 5)
	if h := r.hold(2); h == nil || !h.resent {
		t.Fatalf("the hold after its re-send: %+v", h)
	}
	// A disagreement begun again while the hold stands is held to the
	// day's limit.
	d := &r.rt.drift
	d.mu.Lock()
	delete(d.rooms, 2)
	d.mu.Unlock()
	sent = r.lines.Mark()
	r.disagreeAndResend()
	if got := r.radioSince(sent); len(got) != 0 {
		t.Fatalf("a second held re-send the same day: %v", got)
	}
	if ev := r.newest("disagree"); summaryText(&ev, "outcome") != "not_resent" || summaryText(&ev, "why") != "today" {
		t.Fatalf("the second re-send's event: %+v", ev.Summary)
	}
}

// A held holiday is sent as it was while its end has not passed at the
// Cube's time (2026-11-03 19:51 in the fixture), and dropped once it
// has, the next step taking the room.
func TestHeldHoliday(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name  string
		until cube.Until
		ended bool
	}{
		{"before its end", cube.Until{Year: 2026, Month: 11, Day: 10, Minutes: 12 * 60}, false},
		{"past its end", cube.Until{Year: 2026, Month: 11, Day: 3, Minutes: 12 * 60}, true},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			r := startDrift(t, nil)
			r.set(0x0a0204, "manual", 18)
			r.set(0x0a0205, "manual", 19)
			r.set(0x0a0206, "manual", 23)
			r.rt.holdReference(2, resend{mode: cube.ModeVacation, temp: 15, until: c.until, from: fromMajority}, r.s.roomNow())
			sent := r.lines.Mark()
			r.disagreeAndResend()
			if c.ended {
				r.wantResend(sent, 0x0a0204, 2, 23, cube.ModeManual, "newest")
				if h := r.hold(2); h != nil {
					t.Fatalf("a holiday past its end kept: %+v", h)
				}
				return
			}
			m, err := cube.CmdSetTemperature(cube.Target{RFAddress: 0x0a0204, RoomID: 2, Room: true}, 15, cube.ModeVacation, c.until)
			if err != nil {
				t.Fatal(err)
			}
			if got := r.radioSince(sent); !slices.Equal(got, []string{m.String()}) {
				t.Fatalf("the re-send: %v, want %s", got, m)
			}
			if ev := r.newest("disagree"); summaryText(&ev, "before_change") != "true" || summaryText(&ev, "until") != c.until.String() {
				t.Fatalf("the re-send's event: %+v", ev.Summary)
			}
		})
	}
}

// A value's first sighting is no one's change (cubelan.StateTimes):
// a thermostat whose entry gets its values only after the first
// connection does not win as the newest change, and the room's order
// takes the master's state; nor does the mode with which a boost ends.
func TestNewestPassesOverNoOnesChanges(t *testing.T) {
	t.Parallel()
	manual := func(set float64) fakecube.State {
		mode := "manual"
		return fakecube.State{Mode: &mode, Setpoint: &set}
	}
	t.Run("values seen late", func(t *testing.T) {
		t.Parallel()
		on := true
		r := startDriftWith(t, func(lines []cube.Message) []cube.Message { return withoutValues(t, lines, 0x0a0206) },
			map[cube.RFAddress]fakecube.State{0x0a0204: manual(18), 0x0a0205: manual(19)})
		if st, ok := r.rt.client.Snapshot().State(0x0a0206); !ok || st.Valid || st.OutOfDate {
			t.Fatalf("0a0206 with values at the first connection: %+v", st)
		}
		r.waitSetting(0x0a0204, "manual", 18)
		r.waitSetting(0x0a0205, "manual", 19)
		mode, set := "manual", 23.0
		if err := r.fc.SetState(0x0a0206, fakecube.State{Reported: &on, Mode: &mode, Setpoint: &set}); err != nil {
			t.Fatal(err)
		}
		r.waitSetting(0x0a0206, "manual", 23)
		sent := r.lines.Mark()
		r.disagreeAndResend()
		r.wantResend(sent, 0x0a0204, 2, 18, cube.ModeManual, "master")
	})
	t.Run("a boost's end", func(t *testing.T) {
		t.Parallel()
		r := startDrift(t, nil)
		r.set(0x0a0204, "manual", 18)
		r.set(0x0a0205, "manual", 19)
		for _, mode := range []string{"boost", "auto"} { // 0a0206 boosts, then goes back to auto at 20 °C
			if err := r.fc.SetState(0x0a0206, fakecube.State{Mode: &mode}); err != nil {
				t.Fatal(err)
			}
			r.waitSetting(0x0a0206, mode, 20)
		}
		sent := r.lines.Mark()
		r.disagreeAndResend()
		r.wantResend(sent, 0x0a0204, 2, 19, cube.ModeManual, "newest")
	})
}

// withoutValues gives rf's entry in the greeting's L line no values, as
// after a restart of the Cube: neither its valid bit (0x10) nor its
// initialized bit (0x02) set.
func withoutValues(t *testing.T, lines []cube.Message, rf cube.RFAddress) []cube.Message {
	t.Helper()
	out := append([]cube.Message(nil), lines...)
	for i, m := range out {
		if m.Type != 'L' {
			continue
		}
		raw, err := base64.StdEncoding.DecodeString(m.Data)
		if err != nil {
			t.Fatal(err)
		}
		for j := 0; j < len(raw); j += int(raw[j]) + 1 {
			if cube.RFAddressFromBytes(raw[j+1:j+4]) == rf {
				raw[j+5] &^= 0x12
			}
		}
		out[i].Data = base64.StdEncoding.EncodeToString(raw)
	}
	return out
}
