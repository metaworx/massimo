// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

import (
	"strconv"
	"strings"
	"time"
)

// The event stream's ids and its replay (docs/api-v1.md, "GET …/events").
//
// Every event a Cube's runtime publishes gets the next number of the
// runtime's sequence, and the stream writes it as the event's id,
// "<epoch>-<seq>": the epoch names the runtime's start, so an id from
// before a restart is told apart. The runtime keeps its last replaySize
// events in a ring; a client that reconnects with the last id it read
// gets the events after it that it may see, and one whose id the ring no
// longer holds gets a "resync" event and the current state, as at a
// first connection. A subscriber that cannot keep up is ended instead
// of losing events silently, and its reconnection resumes.

// replaySize is how many of a Cube's events the ring keeps for a
// reconnecting stream: about a quarter of an hour of a busy house.
const replaySize = 1024

// subscriber is an event stream's subscription: whose it is, for a
// user's notifications, and gone, closed when the stream fell behind and
// must end.
type subscriber struct {
	user int64
	// audit says the subscriber may see the events only a user with
	// audit sees (sign-ins, lockouts).
	audit bool
	gone  chan struct{}
}

// sees reports whether the subscriber gets an event published for user
// (0 for every subscriber).
func (sub *subscriber) sees(ev event, user int64) bool {
	return (user == 0 || sub.user == user) && (!ev.audit || sub.audit)
}

// published is an event in the replay ring, with the user it is for (0
// for every subscriber).
type published struct {
	user int64
	ev   event
}

// newEpoch names a runtime's start for its stream ids.
func newEpoch() string {
	return strconv.FormatInt(time.Now().UnixNano(), 36)
}

// streamID is the id the stream writes for an event of the runtime.
func (rt *runtime) streamID(seq uint64) string {
	return rt.epoch + "-" + strconv.FormatUint(seq, 10)
}

// publish numbers an event, keeps it for replay and hands it to the
// Cube's stream subscribers: to every one for user 0, else to that
// user's alone.
func (rt *runtime) publish(ev event, user int64) {
	rt.subsMu.Lock()
	defer rt.subsMu.Unlock()
	rt.publishLocked(ev, user)
}

// publishLocked is publish with subsMu held. A subscriber whose channel
// is full is dropped and told to end, so that it reconnects and resumes
// rather than going on with a gap it cannot see.
func (rt *runtime) publishLocked(ev event, user int64) {
	rt.seq++
	ev.seq = rt.seq
	rt.replay = append(rt.replay, published{user: user, ev: ev})
	if len(rt.replay) > replaySize {
		rt.replay = rt.replay[len(rt.replay)-replaySize:]
	}
	for ch, sub := range rt.subs {
		if !sub.sees(ev, user) {
			continue
		}
		select {
		case ch <- ev:
		default:
			delete(rt.subs, ch)
			close(sub.gone)
		}
	}
}

// since returns the events after the stream id last that sub may see,
// oldest first, and whether the ring still holds all of them: false for
// an id of another epoch, one not issued yet, a malformed one, or one
// whose successors the ring has dropped. subsMu must be held.
func (rt *runtime) since(last string, sub *subscriber) ([]event, bool) {
	epoch, num, ok := strings.Cut(last, "-")
	if !ok || epoch != rt.epoch {
		return nil, false
	}
	seq, err := strconv.ParseUint(num, 10, 64)
	if err != nil || seq > rt.seq {
		return nil, false
	}
	oldest := rt.seq - uint64(len(rt.replay)) + 1 // the ring's first event
	if seq+1 < oldest {
		return nil, false
	}
	var out []event
	for _, p := range rt.replay {
		if p.ev.seq <= seq || !sub.sees(p.ev, p.user) {
			continue
		}
		out = append(out, p.ev)
	}
	return out, true
}
