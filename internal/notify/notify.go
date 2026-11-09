// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

// Package notify decides, from the Cubes' events, what to tell a user:
// a window open for half an hour, a Cube offline for ten minutes, a low
// battery, frost. Each user who has a push subscription chooses the
// kinds and their delays in the preference notify (Prefs); the Sink runs
// the rules for every such user off the server's goroutines and hands
// each Notification to a deliverer.
//
// A notification is data, no text: its kind and values, which the page
// or the app renders in the user's language.
package notify

import (
	"encoding/json"
	"fmt"
	"sort"
	"time"
)

// The kinds of notification.
const (
	KindCubeOffline = "cube_offline" // disconnected, and not back within the delay
	KindCubeOnline  = "cube_online"  // back after a cube_offline was sent
	KindWindowOpen  = "window_open"  // a window open, still open after the delay
	KindBatteryLow  = "battery_low"  // a device's battery went low
	KindDeviceError = "device_error" // a device reports an error or a link error
	KindFrost       = "frost"        // the measured temperature fell under the threshold
	KindDeviceStale = "device_stale" // a thermostat past the stale threshold
	KindPaired      = "paired"       // a device joined by pairing
	KindCubeFound   = "cube_found"   // discovery found a Cube that is not configured
	KindUpdate      = "update"       // a newer release
)

// PrefKey is the preference that holds a user's choices.
const PrefKey = "notify"

// Rule is one kind's setting: on or off, and its delay or threshold
// where the kind has one.
type Rule struct {
	On       bool     `json:"on"`
	AfterMin *int     `json:"after_min,omitempty"`
	BelowC   *float64 `json:"below_c,omitempty"`
}

// Prefs is a user's notify preference: a Rule per kind.
type Prefs map[string]Rule

// The bounds of the delays and the frost threshold.
const (
	MinAfterMin = 1
	MaxAfterMin = 24 * 60
	MinBelowC   = -10.0
	MaxBelowC   = 20.0
)

func intp(v int) *int           { return &v }
func floatp(v float64) *float64 { return &v }

// Defaults are the rules of a user who has set none: the kinds that
// need a look on, the merely informative off.
func Defaults() Prefs {
	return Prefs{
		KindCubeOffline: {On: true, AfterMin: intp(10)},
		KindCubeOnline:  {On: true},
		KindWindowOpen:  {On: true, AfterMin: intp(30)},
		KindBatteryLow:  {On: true},
		KindDeviceError: {On: true},
		KindFrost:       {On: true, BelowC: floatp(8)},
		KindDeviceStale: {On: false},
		KindPaired:      {On: false},
		KindCubeFound:   {On: false},
		KindUpdate:      {On: false},
	}
}

// Kinds lists every kind, sorted.
func Kinds() []string {
	var out []string
	for k := range Defaults() {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// ParsePrefs reads a notify preference over the defaults: a kind it
// does not name, or a delay or threshold a kind leaves out, keeps the
// default. A kind it does not know, a value of the wrong type, and a
// delay or threshold out of bounds are errors.
func ParsePrefs(raw json.RawMessage) (Prefs, error) {
	out := Defaults()
	if len(raw) == 0 {
		return out, nil
	}
	var in map[string]json.RawMessage
	if err := json.Unmarshal(raw, &in); err != nil {
		return nil, fmt.Errorf("notify: the preference is not an object of kinds: %w", err)
	}
	for kind, v := range in {
		def, known := out[kind]
		if !known {
			return nil, fmt.Errorf("notify: unknown kind %q", kind)
		}
		var r Rule
		if err := json.Unmarshal(v, &r); err != nil {
			return nil, fmt.Errorf("notify: %s: %w", kind, err)
		}
		switch {
		case r.AfterMin != nil && def.AfterMin == nil:
			return nil, fmt.Errorf("notify: %s takes no after_min", kind)
		case r.BelowC != nil && def.BelowC == nil:
			return nil, fmt.Errorf("notify: %s takes no below_c", kind)
		case r.AfterMin != nil && (*r.AfterMin < MinAfterMin || *r.AfterMin > MaxAfterMin):
			return nil, fmt.Errorf("notify: %s: after_min %d: want %d to %d", kind, *r.AfterMin, MinAfterMin, MaxAfterMin)
		case r.BelowC != nil && (*r.BelowC < MinBelowC || *r.BelowC > MaxBelowC):
			return nil, fmt.Errorf("notify: %s: below_c %g: want %g to %g", kind, *r.BelowC, MinBelowC, MaxBelowC)
		}
		if r.AfterMin == nil {
			r.AfterMin = def.AfterMin
		}
		if r.BelowC == nil {
			r.BelowC = def.BelowC
		}
		out[kind] = r
	}
	return out, nil
}

// after is a kind's delay.
func (p Prefs) after(kind string) time.Duration {
	if r := p[kind]; r.AfterMin != nil {
		return time.Duration(*r.AfterMin) * time.Minute
	}
	return 0
}

// on says whether a kind is on.
func (p Prefs) on(kind string) bool { return p[kind].On }

// Notification is what a user is told: data, no text.
type Notification struct {
	Kind string `json:"kind"`
	// Cube is the Cube's serial; for cube_found the one found.
	Cube string `json:"cube,omitempty"`
	// Device is the device's radio address, Serial its S/N.
	Device     string `json:"device,omitempty"`
	Serial     string `json:"serial,omitempty"`
	DeviceName string `json:"device_name,omitempty"`
	Room       int    `json:"room,omitempty"`
	RoomName   string `json:"room_name,omitempty"`
	// Since is when the condition began, for the delayed kinds.
	Since *time.Time `json:"since,omitempty"`
	// Values are the kind's: minutes, actual_c and below_c, version, …
	Values map[string]any `json:"values,omitempty"`
	Time   time.Time      `json:"time"`
	// URL is the page that shows it, relative to the server.
	URL string `json:"url"`
}

// Event is what the server tells the Sink: one of the Cube's events, or
// one of its own (cube_found, update).
type Event struct {
	// Kind is the server's event kind: connected, disconnected, state,
	// paired, cube_found or update; others are ignored.
	Kind string
	// Cube is the serial of the Cube the event is of.
	Cube string
	// The device, for a device's events.
	Device, Serial, DeviceName string
	Room                       int
	RoomName                   string
	// State is the device's state, for a state event.
	State *State
	// Values are the event's: type for paired, address for cube_found,
	// version for update.
	Values map[string]any
	Time   time.Time
}

// State is what the rules read of a device's state.
type State struct {
	WindowOpen *bool
	BatteryLow bool
	Error      bool
	LinkError  bool
	ActualTemp *float64
}

// Device is a device as the stale scan sees it.
type Device struct {
	Cube, Device, Serial, DeviceName string
	Room                             int
	RoomName                         string
	Stale                            bool
}

// User is a user the rules run for, with the preference.
type User struct {
	ID    int64
	Prefs Prefs
}
