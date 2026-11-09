// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package notify

import (
	"math"
	"time"
)

// coalesce is how long a kind stays quiet for one device after it was
// sent: a window that opens and closes all morning is one notification
// per half hour.
const coalesce = 30 * time.Minute

// frostHysteresis is how far above the threshold the temperature must
// rise before frost can be sent again.
const frostHysteresis = 0.5

// engine is one user's rules: what it saw of each device and Cube, the
// delays running, and when each kind was sent. A restart forgets it,
// which costs at most one repeated notification.
type engine struct {
	s    *Sink
	user User

	prev    map[string]State     // a device's last state, by cube and device
	below   map[string]bool      // a device under the user's frost threshold
	stale   map[string]bool      // a device the stale scan last saw stale
	pending map[string]*pending  // the delays running, by their key
	offline map[string]time.Time // the Cubes a cube_offline was sent for, since when
	sent    map[string]time.Time // when a kind was sent, by kind, cube and device
	version string               // the release an update was last seen for
}

// pending is a delayed notification: sent when its timer fires, unless
// the condition cleared first.
type pending struct {
	seq   uint64
	timer Timer
	n     Notification
}

func newEngine(s *Sink, u User) *engine {
	return &engine{s: s, user: u, prev: map[string]State{}, below: map[string]bool{}, stale: map[string]bool{},
		pending: map[string]*pending{}, offline: map[string]time.Time{}, sent: map[string]time.Time{}}
}

func deviceKey(cube, device string) string { return cube + "|" + device }

// event runs the rules over one event.
func (e *engine) event(ev Event) {
	p := e.user.Prefs
	switch ev.Kind {
	case "disconnected":
		key := "offline|" + ev.Cube
		if p.on(KindCubeOffline) && e.pending[key] == nil {
			e.delay(key, p.after(KindCubeOffline), Notification{Kind: KindCubeOffline, Cube: ev.Cube, Since: timep(ev.Time),
				Values: map[string]any{"minutes": int(p.after(KindCubeOffline) / time.Minute)}})
		}
	case "connected":
		e.cancel("offline|" + ev.Cube)
		if since, ok := e.offline[ev.Cube]; ok {
			delete(e.offline, ev.Cube)
			if p.on(KindCubeOnline) {
				down := int(math.Round(ev.Time.Sub(since).Minutes()))
				e.send(Notification{Kind: KindCubeOnline, Cube: ev.Cube, Since: timep(since), Values: map[string]any{"down_minutes": down}})
			}
		}
	case "state":
		if ev.State != nil {
			e.state(ev, *ev.State)
		}
	case "device_forgotten":
		key := deviceKey(ev.Cube, ev.Device)
		e.cancel("window|" + key)
		delete(e.prev, key)
		delete(e.below, key)
		delete(e.stale, key)
	case "paired":
		if p.on(KindPaired) {
			n := device(ev, KindPaired)
			n.Values = ev.Values
			e.send(n)
		}
	case "cube_found":
		if p.on(KindCubeFound) {
			e.send(Notification{Kind: KindCubeFound, Cube: ev.Cube, Values: ev.Values})
		}
	case "update":
		v, _ := ev.Values["version"].(string)
		if v != "" && v != e.version {
			e.version = v
			if p.on(KindUpdate) {
				e.send(Notification{Kind: KindUpdate, Values: ev.Values})
			}
		}
	}
}

// state runs the device rules over a state event, against the device's
// previous state; a device seen for the first time counts as having had
// none of the conditions.
func (e *engine) state(ev Event, st State) {
	p := e.user.Prefs
	key := deviceKey(ev.Cube, ev.Device)
	prev, had := e.prev[key]
	e.prev[key] = st

	open := st.WindowOpen != nil && *st.WindowOpen
	wasOpen := had && prev.WindowOpen != nil && *prev.WindowOpen
	switch {
	case !open:
		e.cancel("window|" + key)
	case !wasOpen && p.on(KindWindowOpen):
		n := device(ev, KindWindowOpen)
		n.Since = timep(ev.Time)
		n.Values = map[string]any{"minutes": int(p.after(KindWindowOpen) / time.Minute)}
		e.delay("window|"+key, p.after(KindWindowOpen), n)
	}

	if st.BatteryLow && !(had && prev.BatteryLow) && p.on(KindBatteryLow) {
		e.send(device(ev, KindBatteryLow))
	}

	if (st.Error || st.LinkError) && !(had && (prev.Error || prev.LinkError)) && p.on(KindDeviceError) {
		n := device(ev, KindDeviceError)
		n.Values = map[string]any{"error": st.Error, "link_error": st.LinkError}
		e.send(n)
	}

	if t := st.ActualTemp; t != nil && p[KindFrost].BelowC != nil {
		limit := *p[KindFrost].BelowC
		switch {
		case *t < limit && !e.below[key]:
			e.below[key] = true
			if p.on(KindFrost) {
				n := device(ev, KindFrost)
				n.Values = map[string]any{"actual_c": *t, "below_c": limit}
				e.send(n)
			}
		case *t >= limit+frostHysteresis:
			e.below[key] = false
		}
	}
}

// scan runs device_stale over the stale scan's devices.
func (e *engine) scan(devices []Device) {
	for _, d := range devices {
		key := deviceKey(d.Cube, d.Device)
		if d.Stale && !e.stale[key] && e.user.Prefs.on(KindDeviceStale) {
			e.send(Notification{Kind: KindDeviceStale, Cube: d.Cube, Device: d.Device, Serial: d.Serial, DeviceName: d.DeviceName, Room: d.Room, RoomName: d.RoomName})
		}
		e.stale[key] = d.Stale
	}
}

// device is a notification of kind about an event's device.
func device(ev Event, kind string) Notification {
	return Notification{Kind: kind, Cube: ev.Cube, Device: ev.Device, Serial: ev.Serial, DeviceName: ev.DeviceName, Room: ev.Room, RoomName: ev.RoomName}
}

// delay sends n after d, unless cancel comes first.
func (e *engine) delay(key string, d time.Duration, n Notification) {
	e.cancel(key)
	p := &pending{seq: e.s.nextSeq(), n: n}
	user, seq := e.user.ID, p.seq
	p.timer = e.s.o.Clock.AfterFunc(d, func() { e.s.enqueue(item{fire: &fire{user: user, key: key, seq: seq}}) })
	e.pending[key] = p
}

// cancel stops a delay.
func (e *engine) cancel(key string) {
	if p := e.pending[key]; p != nil {
		p.timer.Stop()
		delete(e.pending, key)
	}
}

// fired sends a delay's notification, if it still runs and its kind is
// still on.
func (e *engine) fired(key string, seq uint64) {
	p := e.pending[key]
	if p == nil || p.seq != seq {
		return // cancelled, or replaced, before its timer's item was taken
	}
	delete(e.pending, key)
	if !e.user.Prefs.on(p.n.Kind) {
		return
	}
	if e.send(p.n) && p.n.Kind == KindCubeOffline {
		e.offline[p.n.Cube] = *p.n.Since
	}
}

// stop cancels every delay, for a user whose rules end.
func (e *engine) stop() {
	for key := range e.pending {
		e.cancel(key)
	}
}

// send hands n to the deliverer, unless the same kind was sent for the
// same device within coalesce; it reports whether it did.
func (e *engine) send(n Notification) bool {
	now := e.s.o.Clock.Now()
	key := n.Kind + "|" + n.Cube + "|" + n.Device
	if last, ok := e.sent[key]; ok && now.Sub(last) < coalesce {
		return false
	}
	e.sent[key] = now
	n.Time, n.URL = now, "/"
	e.s.o.Deliver(e.user.ID, n)
	return true
}

func timep(t time.Time) *time.Time { return &t }
