// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

import (
	"context"
	"reflect"
	"slices"
	"testing"
	"time"

	"go.metaworx.ch/massimo/internal/cubelan/fakecube"
	"go.metaworx.ch/massimo/internal/engine"
	"go.metaworx.ch/massimo/internal/protocol/cube"
	"go.metaworx.ch/massimo/internal/testkit"
)

// The re-send of an unconfirmed room command (AP RadioHealth v1.3,
// Block 8), on the drift rig (drift_test.go): room 2's thermostats are
// 0a0204, the member its room commands name, 0a0205 and 0a0206.

const room2 = "/api/v1/cubes/KEQ0000100/rooms/2/setpoint"

// resendOptions gives a write's devices within to show it, and the
// queue slotWait for free memory slots (0 the default).
func resendOptions(within, slotWait time.Duration) func(*Options) {
	return func(o *Options) {
		o.confirmWithin = within
		o.Queue.SlotWait = slotWait
	}
}

// unconfirmedOutcomes are the log's unconfirmed events with outcome,
// oldest first.
func (r *driftRig) unconfirmedOutcomes(outcome string) []event {
	r.t.Helper()
	return logLines(r.t, logStart(r.ts), unconfirmedOf(outcome))
}

// waitOutcome waits until the log has n unconfirmed events with outcome,
// and no more, and returns the newest.
func (r *driftRig) waitOutcome(outcome string, n int) event {
	r.t.Helper()
	evs, next := logAfterN(r.t, logStart(r.ts), "an unconfirmed event, "+outcome, n, unconfirmedOf(outcome))
	if more := logLines(r.t, next, unconfirmedOf(outcome)); len(more) != 0 {
		r.t.Fatalf("%d %s events, want %d: %+v", n+len(more), outcome, n, append(evs, more...))
	}
	return evs[n-1]
}

// flag sets rf's communication error and waits until the snapshot
// shows it.
func (r *driftRig) flag(rf cube.RFAddress) {
	r.t.Helper()
	on := true
	if err := r.fc.SetState(rf, fakecube.State{Error: &on}); err != nil {
		r.t.Fatal(err)
	}
	testkit.Eventually(r.t, rf.String()+" flagged", func() bool {
		st, ok := r.rt.client.Snapshot().State(rf)
		return ok && st.Error
	})
}

// setRoom2 sets room 2 to manual at temp through the API.
func (r *driftRig) setRoom2(temp string) {
	r.t.Helper()
	if code, out := do(r.t, r.ts, "PUT", room2, `{"mode":"manual","temp":`+temp+`}`); code != 200 || out["confirmation"] != "pending" {
		r.t.Fatalf("room setpoint %s: %d %v", temp, code, out)
	}
}

// resetDrift forgets the room checks' and the re-sends' days, as a
// restart does; the store's log keeps them.
func (r *driftRig) resetDrift() {
	r.t.Helper()
	if err := r.st.FlushEvents(context.Background()); err != nil {
		r.t.Fatal(err)
	}
	d := &r.rt.drift
	d.mu.Lock()
	d.rooms, d.members, d.resent, d.warned, d.seeded = nil, nil, nil, nil, false
	d.mu.Unlock()
}

// A room setpoint a flagged member does not show is sent once more at
// its deadline, the same room command after the member's r:; still not
// shown, a Warn names the member and what it shows, and the room checks
// leave the room alone. A newer write not shown either is not sent again
// that day, also after a restart, which reads the day's re-send from the
// log; a day later it is.
func TestUnconfirmedRoomCommandResent(t *testing.T) {
	t.Parallel()
	r := startDrift(t, nil, resendOptions(400*time.Millisecond, 0))
	r.fc.SetDeaf(0x0a0205, true)
	r.flag(0x0a0205)
	reset := cube.CmdResetError(0x0a0205).String()
	sent := r.lines.Mark()
	r.setRoom2("22")
	r.waitOutcome("not_confirmed", 1)
	resent := r.waitOutcome("resent", 1)
	write := setpointLine(t, 0x0a0204, 2, 22, cube.ModeManual)
	if got := r.radioSince(sent); !slices.Equal(got, []string{reset, write, reset, write}) {
		t.Fatalf("the lines: %v, want the write and its re-send, each after %s", got, reset)
	}
	if resent.Level != "info" || resent.Summary["target"] != "room:2" || resent.Summary["source"] != "api" || resent.Summary["duty_pct"] == nil ||
		resent.Summary["free_slots"] == nil || !reflect.DeepEqual(resent.Summary["expected"], map[string]any{"mode": "manual", "setpoint": float64(22)}) {
		t.Fatalf("the re-send's line: %+v", resent)
	}

	warn := r.waitOutcome("resend_not_confirmed", 1)
	want := []any{map[string]any{"rf_address": "0a0205", "serial": "MEQ0000205", "fields": []any{"mode", "setpoint"},
		"shows": map[string]any{"mode": "auto", "setpoint": float64(20)}, "error": true, "link_error": false, "out_of_date": false}}
	if warn.Level != "warn" || warn.Summary["target"] != "room:2" || warn.Summary["confirmed"] != float64(2) || !reflect.DeepEqual(warn.Summary["devices"], want) {
		t.Fatalf("the Warn: %+v", warn)
	}
	if c := r.rt.roomConfirmation(2); c == nil || c.State != confirmUnconfirmed {
		t.Fatalf("the room's confirmation: %+v", c)
	}
	if alerts := alertKinds(t, r.ts); !slices.Contains(alerts, "unconfirmed") {
		t.Fatalf("the Warn not an alert: %v", alerts)
	}

	// The room checks leave the room alone while its write is not
	// confirmed: its members differ, and nothing is reported or sent.
	for range 3 {
		r.check()
		r.clock.add(11 * time.Minute)
	}
	if got := r.outcomes("disagree"); len(got) != 0 {
		t.Fatalf("the room checks reported %v", got)
	}

	// The same day: a newer write, not shown either, is not sent again.
	r.setRoom2("23")
	r.waitOutcome("not_confirmed", 2)
	if ev := r.waitOutcome("not_resent", 1); ev.Summary["why"] != whyToday || ev.Level != "info" {
		t.Fatalf("the same day: %+v", ev)
	}

	// After a restart, the day's re-send is read from the log.
	r.resetDrift()
	r.setRoom2("24")
	r.waitOutcome("not_confirmed", 3)
	if ev := r.waitOutcome("not_resent", 2); ev.Summary["why"] != whyToday {
		t.Fatalf("after the restart: %+v", ev)
	}
	if got := r.radioSince(sent); len(got) != 8 {
		t.Fatalf("sent again the same day: %v", got)
	}

	// A day later, it is sent again.
	r.clock.add(25 * time.Hour)
	n := r.lines.Mark()
	r.setRoom2("21")
	r.waitOutcome("resent", 2)
	write = setpointLine(t, 0x0a0204, 2, 21, cube.ModeManual)
	if got := r.radioSince(n); !slices.Equal(got, []string{reset, write, reset, write}) {
		t.Fatalf("a day later: %v", got)
	}
}

// A re-send waits behind the queue's hold; every member showing it, an
// Info says so and settles the Warn. The room checks send nothing to
// the room for the rest of the day.
func TestUnconfirmedRoomCommandResentBehindTheHold(t *testing.T) {
	t.Parallel()
	r := startDrift(t, nil, resendOptions(400*time.Millisecond, 1500*time.Millisecond))
	r.fc.SetDeaf(0x0a0205, true)
	r.fc.SetFreeSlots(2) // the write's answer: 2 free, so that its re-send waits
	sent, m := r.lines.Mark(), logMark(t, r.ts)
	r.setRoom2("22")
	r.waitOutcome("not_confirmed", 1)
	logAfter(t, m, "the queue's hold", logLine("hold", ""))
	if got := r.radioSince(sent); len(got) != 1 {
		t.Fatalf("sent before the hold ended: %v", got)
	}
	r.fc.SetDeaf(0x0a0205, false) // the member hears again
	r.fc.SetFreeSlots(49)
	r.waitOutcome("resent", 1)
	ok := r.waitOutcome("resend_confirmed", 1)
	if ok.Level != "info" || ok.Summary["target"] != "room:2" || ok.Summary["confirmed"] != float64(3) {
		t.Fatalf("the Info: %+v", ok)
	}
	write := setpointLine(t, 0x0a0204, 2, 22, cube.ModeManual)
	if got := r.radioSince(sent); !slices.Equal(got, []string{write, write}) {
		t.Fatalf("the lines: %v", got)
	}
	if c := r.rt.roomConfirmation(2); c != nil {
		t.Fatalf("the room's confirmation: %+v", c)
	}
	if alerts := alertKinds(t, r.ts); slices.Contains(alerts, "unconfirmed") {
		t.Fatalf("the Warn not settled: %v", alerts)
	}
	if got := r.unconfirmedOutcomes("resend_not_confirmed"); len(got) != 0 {
		t.Fatalf("a Warn after the Info: %+v", got)
	}

	// The room checks: the members differ, are reported, and nothing is
	// sent, the day's re-send taken.
	r.set(0x0a0206, "manual", 18)
	n := r.lines.Mark()
	r.disagreeAndResend()
	last := r.newest("disagree")
	if got := r.outcomes("disagree"); !slices.Equal(got, []string{"differ", "not_resent"}) || summaryText(&last, "why") != "today" {
		t.Fatalf("the room checks: %v, %+v", got, last)
	}
	if got := r.radioSince(n); len(got) != 0 {
		t.Fatalf("the room checks sent %v", got)
	}
}

// The room checks leave a room alone while a write to it is pending
// (D199-A): a member that has not taken it yet differs, and nothing is
// reported or sent.
func TestRoomChecksWaitForAPendingWrite(t *testing.T) {
	t.Parallel()
	r := startDrift(t, nil)
	r.fc.SetDeaf(0x0a0205, true)
	r.setRoom2("22")
	sent := r.lines.Mark()
	r.waitSetting(0x0a0206, "manual", 22)
	for range 3 {
		r.check()
		r.clock.add(11 * time.Minute)
	}
	if c := r.rt.roomConfirmation(2); c == nil || c.State != confirmPending {
		t.Fatalf("the room's confirmation: %+v", c)
	}
	if got := r.outcomes("disagree"); len(got) != 0 {
		t.Fatalf("the room checks reported %v", got)
	}
	if got := r.radioSince(sent); len(got) != 0 {
		t.Fatalf("the room checks sent %v", got)
	}
}

// A room the room checks sent its state once more today is not sent an
// unconfirmed write again (D199-A).
func TestRoomChecksResendTakesTheDay(t *testing.T) {
	t.Parallel()
	r := startDrift(t, nil, resendOptions(400*time.Millisecond, 0))
	r.set(0x0a0205, "manual", 18)
	r.disagreeAndResend()
	if got := r.outcomes("disagree"); !slices.Equal(got, []string{"differ", "resent"}) {
		t.Fatalf("the room checks: %v", got)
	}
	r.fc.SetDeaf(0x0a0205, true)
	sent := r.lines.Mark()
	r.setRoom2("22")
	r.waitOutcome("not_confirmed", 1)
	if ev := r.waitOutcome("not_resent", 1); ev.Summary["why"] != whyToday {
		t.Fatalf("not re-sent: %+v", ev)
	}
	if got := r.radioSince(sent); len(got) != 1 {
		t.Fatalf("sent: %v", got)
	}
}

// Not sent once more: a write a newer write to a member superseded, one
// superseded while its re-send waited behind the hold, a write while a
// command of Massimo's to the room waits, a boost, and a LAN proxy
// client's write, which is only reported.
func TestUnconfirmedRoomCommandNotResent(t *testing.T) {
	t.Parallel()
	t.Run("superseded", func(t *testing.T) {
		t.Parallel()
		r := startDrift(t, nil, resendOptions(400*time.Millisecond, 0))
		r.fc.SetDeaf(0x0a0205, true)
		sent := r.lines.Mark()
		r.setRoom2("22")
		if code, out := do(t, r.ts, "PUT", "/api/v1/cubes/KEQ0000100/devices/0a0204/setpoint", `{"mode":"manual","temp":19}`); code != 200 {
			t.Fatalf("device setpoint: %d %v", code, out)
		}
		r.waitOutcome("not_confirmed", 1)
		if ev := r.waitOutcome("not_resent", 1); ev.Summary["why"] != whySuperseded || ev.Summary["target"] != "room:2" {
			t.Fatalf("not re-sent: %+v", ev)
		}
		if got := r.radioSince(sent); len(got) != 2 {
			t.Fatalf("sent: %v", got)
		}
	})
	t.Run("superseded behind the hold", func(t *testing.T) {
		t.Parallel()
		r := startDrift(t, nil, resendOptions(400*time.Millisecond, 800*time.Millisecond))
		r.fc.SetDeaf(0x0a0205, true)
		r.fc.SetFreeSlots(2)
		sent := r.lines.Mark()
		r.setRoom2("22")
		r.waitOutcome("not_confirmed", 1)
		testkit.Eventually(t, "the re-send in the queue", func() bool {
			return r.rt.queue.Pending(func(c engine.Command) bool { return c.Key == resendKey(2) })
		})
		r.fc.SetDeaf(0x0a0205, false) // the newer write is shown, and not sent again
		if code, out := do(t, r.ts, "PUT", room2, `{"mode":"manual","temp":23}`); code != 200 {
			t.Fatalf("the newer write: %d %v", code, out)
		}
		if ev := r.waitOutcome("not_resent", 1); ev.Summary["why"] != whySuperseded {
			t.Fatalf("not re-sent: %+v", ev)
		}
		want := []string{setpointLine(t, 0x0a0204, 2, 22, cube.ModeManual), setpointLine(t, 0x0a0204, 2, 23, cube.ModeManual)}
		if got := r.radioSince(sent); !slices.Equal(got, want) {
			t.Fatalf("sent: %v", got)
		}
		// Nothing was sent: the room's day is not taken.
		r.rt.drift.mu.Lock()
		_, taken := r.rt.drift.resent[2]
		r.rt.drift.mu.Unlock()
		if taken {
			t.Fatal("the dropped re-send took the room's day")
		}
	})
	t.Run("busy", func(t *testing.T) {
		t.Parallel()
		r := startDrift(t, nil, resendOptions(400*time.Millisecond, 2*time.Second))
		r.fc.SetDeaf(0x0a0205, true)
		r.fc.SetFreeSlots(2)
		r.setRoom2("22")
		// A command of Massimo's to the room that waits behind the hold.
		other := r.rt.queue.Enqueue(engine.Command{Key: "test:room:2", Priority: engine.PriorityLow, Lines: []cube.Message{setpointLine2(t)}})
		if ev := r.waitOutcome("not_resent", 1); ev.Summary["why"] != whyBusy {
			t.Fatalf("not re-sent: %+v", ev)
		}
		other.Wait(context.Background())
	})
	t.Run("boost", func(t *testing.T) {
		t.Parallel()
		r := startDrift(t, nil, resendOptions(400*time.Millisecond, 0))
		r.fc.SetDeaf(0x0a0205, true)
		if code, out := do(t, r.ts, "PUT", room2, `{"mode":"boost"}`); code != 200 {
			t.Fatalf("boost: %d %v", code, out)
		}
		if ev := r.waitOutcome("not_resent", 1); ev.Summary["why"] != whyBoost {
			t.Fatalf("not re-sent: %+v", ev)
		}
	})
	t.Run("proxy", func(t *testing.T) {
		t.Parallel()
		r := startDrift(t, nil, resendOptions(300*time.Millisecond, 0))
		r.fc.SetDeaf(0x0a0205, true)
		m, err := cube.CmdSetTemperature(cube.Target{RFAddress: 0x0a0204, RoomID: 2, Room: true}, 21.5, cube.ModeManual, cube.Until{})
		if err != nil {
			t.Fatal(err)
		}
		c, err := cube.ParseRadioCommand(m.Data)
		if err != nil {
			t.Fatal(err)
		}
		sent := r.lines.Mark()
		r.rt.proxyWrote(c) // as if the Cube had taken it
		if ev := r.waitOutcome("not_confirmed", 1); ev.Summary["source"] != "proxy" {
			t.Fatalf("the report: %+v", ev)
		}
		time.Sleep(200 * time.Millisecond)
		for _, ev := range r.events("unconfirmed") {
			if ev.Summary["outcome"] != "not_confirmed" {
				t.Fatalf("a line of a re-send: %+v", ev)
			}
		}
		if got := r.radioSince(sent); len(got) != 0 {
			t.Fatalf("sent: %v", got)
		}
	})
}

// setpointLine2 is a room command to room 2, manual at 18 °C.
func setpointLine2(t *testing.T) cube.Message {
	t.Helper()
	m, err := cube.CmdSetTemperature(cube.Target{RFAddress: 0x0a0204, RoomID: 2, Room: true}, 18, cube.ModeManual, cube.Until{})
	if err != nil {
		t.Fatal(err)
	}
	return m
}

// A setpoint asked for supersedes by its room: a room's, a device's by
// its room or its C line's; a device without a room none.
func TestSetpointAsked(t *testing.T) {
	t.Parallel()
	r := startDrift(t, nil)
	at := time.Now()
	if r.rt.confirms.askedSince(2, at) || r.rt.confirms.askedSince(1, at) {
		t.Fatal("asked before any")
	}
	r.rt.setpointAsked(cube.Target{RFAddress: 0x0a0205}) // direct, by its C line: room 2
	if !r.rt.confirms.askedSince(2, at) || r.rt.confirms.askedSince(1, at) {
		t.Fatal("a direct setpoint not by its C line's room")
	}
	r.rt.setpointAsked(cube.Target{RFAddress: 0x0a0201, RoomID: 1, Room: true})
	if !r.rt.confirms.askedSince(1, at) {
		t.Fatal("a room setpoint not by its room")
	}
}
