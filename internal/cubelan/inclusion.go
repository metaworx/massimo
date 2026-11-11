// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package cubelan

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"go.metaworx.ch/massimo/internal/protocol/cube"
)

// Pairing (inclusion mode). n:<seconds> puts the Cube into inclusion
// mode and has no immediate reply; while it lasts, the Cube sends an N
// line with the type, radio address and serial of each device that
// joins. The client tracks the mode so that an N line that arrives long
// after the n: was written still reaches whoever started it, and ends
// it on an empty N line, which the Cube is expected to send when the mode
// ends or is cancelled with x:, or on its own once the time is up.

// ErrInclusionActive is returned when inclusion mode is already running.
var ErrInclusionActive = errors.New("cubelan: pairing is already running")

// inclusionGrace is how long after the requested end the client still
// waits for the Cube's own end of inclusion mode.
const inclusionGrace = 10 * time.Second

// InclusionState is the state of pairing on one Cube.
type InclusionState struct {
	Active  bool      `json:"active"`
	Started time.Time `json:"started,omitempty"`
	// Until is when the mode ends at the latest, as requested.
	Until time.Time `json:"until,omitempty"`
	// Ended is when the last inclusion ended; zero while active or never.
	Ended time.Time `json:"ended,omitempty"`
	// Paired lists the devices that joined during the current or last
	// inclusion, in order.
	Paired []cube.NewDevice `json:"paired"`
}

type inclusion struct {
	mu       sync.Mutex
	state    InclusionState
	timer    *time.Timer
	watchers map[chan cube.Message]struct{}
}

// StartInclusion marks inclusion mode as running for timeout; the caller
// then writes the n: command (through the command queue). It fails with
// ErrInclusionActive while a previous one runs.
func (c *Client) StartInclusion(timeout time.Duration) error {
	inc := &c.inclusion
	inc.mu.Lock()
	if inc.state.Active {
		inc.mu.Unlock()
		return ErrInclusionActive
	}
	now := time.Now()
	inc.state = InclusionState{Active: true, Started: now, Until: now.Add(timeout), Paired: []cube.NewDevice{}}
	inc.timer = time.AfterFunc(timeout+inclusionGrace, func() { c.EndInclusion() })
	inc.mu.Unlock()
	c.emit(Event{Kind: EventPairingStarted})
	c.log.Info("cube pairing started", "addr", c.Address(), "seconds", int(timeout.Seconds()))
	return nil
}

// EndInclusion marks inclusion mode as ended, closes the watchers'
// channels and reports whether it was running. It does not write x:.
func (c *Client) EndInclusion() bool {
	inc := &c.inclusion
	inc.mu.Lock()
	ended, paired := inc.endLocked()
	inc.mu.Unlock()
	if ended {
		c.endedInclusion(paired)
	}
	return ended
}

// endLocked marks inclusion mode as ended and closes the watchers'
// channels, with inc.mu held; it reports whether it was running and how
// many devices joined.
func (inc *inclusion) endLocked() (bool, int) {
	if !inc.state.Active {
		return false, 0
	}
	inc.state.Active = false
	inc.state.Ended = time.Now()
	if inc.timer != nil {
		inc.timer.Stop()
		inc.timer = nil
	}
	for ch := range inc.watchers {
		close(ch)
	}
	inc.watchers = nil
	return true, len(inc.state.Paired)
}

// endedInclusion reports the end of inclusion mode, outside inc.mu.
func (c *Client) endedInclusion(paired int) {
	c.emit(Event{Kind: EventPairingEnded})
	c.log.Info("cube pairing ended", "addr", c.Address(), "paired", paired)
}

// Inclusion returns the pairing state.
func (c *Client) Inclusion() InclusionState {
	inc := &c.inclusion
	inc.mu.Lock()
	defer inc.mu.Unlock()
	st := inc.state
	st.Paired = append([]cube.NewDevice{}, inc.state.Paired...)
	return st
}

// WatchInclusion returns a channel that receives every N line the Cube
// sends during the running inclusion and is closed when it ends, and a
// function that ends the watch early. Without a running inclusion the
// channel is closed at once.
func (c *Client) WatchInclusion() (<-chan cube.Message, func()) {
	inc := &c.inclusion
	inc.mu.Lock()
	defer inc.mu.Unlock()
	ch := make(chan cube.Message, 16)
	if !inc.state.Active {
		close(ch)
		return ch, func() {}
	}
	if inc.watchers == nil {
		inc.watchers = map[chan cube.Message]struct{}{}
	}
	inc.watchers[ch] = struct{}{}
	return ch, func() {
		inc.mu.Lock()
		defer inc.mu.Unlock()
		if _, ok := inc.watchers[ch]; ok {
			delete(inc.watchers, ch)
			close(ch)
		}
	}
}

// handleNewDevice takes an N line: a device that joined, or, empty, the
// end of inclusion mode. The device, or the end, is in the state before
// a watcher can read the line, in one critical section: the proxy relays
// the line to its client, which may ask for the state at once. The end
// closes the watchers' channels after the line, which they read first.
func (c *Client) handleNewDevice(m cube.Message) {
	var nd *cube.NewDevice
	if m.Data != "" {
		var err error
		if nd, err = cube.ParseNewDevice(m.Data); err != nil {
			c.log.Warn("bad N line from cube", "error", err)
		}
	}
	inc := &c.inclusion
	inc.mu.Lock()
	if nd != nil {
		inc.state.Paired = append(inc.state.Paired, *nd)
	}
	for ch := range inc.watchers {
		select {
		case ch <- m:
		default: // a watcher that does not read loses lines
		}
	}
	ended, paired := false, 0
	if m.Data == "" {
		ended, paired = inc.endLocked()
	}
	inc.mu.Unlock()
	if m.Data == "" {
		if ended {
			c.endedInclusion(paired)
		}
		return
	}
	if nd == nil {
		return
	}
	c.log.Log(context.Background(), c.eventLevel(slog.LevelInfo), "device paired", "addr", c.Address(), "device", nd.RFAddress, "type", nd.Type, "serial", nd.Serial)
	c.emit(Event{Kind: EventPaired, Device: nd.RFAddress, New: nd})
}
