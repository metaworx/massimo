// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package cubelan

import (
	"strings"
	"sync"
	"time"

	"go.metaworx.ch/massimo/internal/protocol/cube"
)

// Snapshot is everything the Cube has told us so far: the greeting, the
// room/device metadata, every device configuration and the latest live
// state per device. It is updated by the client and read by everyone else.
type Snapshot struct {
	mu       sync.RWMutex
	hello    *cube.Hello
	metadata *cube.Metadata
	configs  map[cube.RFAddress]*cube.DeviceConfig
	states   map[cube.RFAddress]cube.DeviceState
	times    map[cube.RFAddress]*StateTimes
	since    time.Time        // first message from the Cube
	listed   bool             // an L line has been applied
	order    []cube.RFAddress // devices in the order the Cube first reported them
	mparts   []cube.MetadataPart
	updated  time.Time
	// greeting is set while a new connection's greeting is being read;
	// fresh collects the devices it sent a C line for.
	greeting bool
	fresh    map[cube.RFAddress]bool
	// forgotten are the devices Forget dropped: an L line does not bring
	// one back, a C line for it or a new greeting does.
	forgotten map[cube.RFAddress]bool
	// delisted holds the last entry of a device an M line left out of
	// the room and device list, until Forget takes it or a list names
	// the device again: a removal writes the list without the device
	// before the device is forgotten, and its event still names it.
	delisted map[cube.RFAddress]cube.DeviceMeta
}

func newSnapshot() *Snapshot {
	return &Snapshot{
		configs: map[cube.RFAddress]*cube.DeviceConfig{},
		states:  map[cube.RFAddress]cube.DeviceState{},
		times:   map[cube.RFAddress]*StateTimes{},
	}
}

// SnapshotOf builds a snapshot from a greeting's lines (H, M, C…, L), as
// a connection builds it: a backup's raw lines give the Cube as it was
// when the backup was taken, for the diff of two backups (docs/store.md
// section 9 (e)). now is the time the snapshot records the lines at.
func SnapshotOf(msgs []cube.Message, now time.Time) (*Snapshot, error) {
	s := newSnapshot()
	s.beginGreeting()
	for _, m := range msgs {
		if _, err := s.apply(m, now); err != nil {
			return nil, err
		}
	}
	return s, nil
}

// Event describes a change the client noticed.
type Event struct {
	Kind   EventKind
	Device cube.RFAddress  // for device events
	Err    error           // for EventDisconnected
	New    *cube.NewDevice // for EventPaired
	Gone   *Gone           // for EventForgotten: the device as the snapshot knew it
	// Greeting marks the H, M and C lines of a connection's greeting,
	// which a client receives at every connection, changed or not.
	Greeting bool
	// Cause says, for EventConnected, why the connection began
	// (CauseStart, CauseLoss, CauseReconnect); DownFor how long the Cube
	// was lost before it, for CauseLoss.
	Cause   string
	DownFor time.Duration
}

// The causes of a connection, for EventConnected.
const (
	CauseStart     = "start"     // the client's first connection
	CauseLoss      = "loss"      // after a loss (EventDisconnected)
	CauseReconnect = "reconnect" // a reconnect the client was asked for
)

// Gone is what the snapshot knew of a device it dropped (Forget): its
// entry in the room and device list, or, for a device the list did not
// hold, its configuration's type, serial and room. The event of a
// forgotten device carries it, since the device is no longer there to
// look up.
type Gone struct {
	Type   cube.DeviceType
	Serial string
	Name   string
	RoomID uint8
}

// EventKind enumerates Event types.
type EventKind int

const (
	EventConnected      EventKind = iota // greeting received, snapshot complete
	EventDisconnected                    // connection lost, once per loss (Run); the client reconnects on its own
	EventHello                           // new H line (duty cycle, free slots, clock)
	EventMetadata                        // rooms or devices changed
	EventConfig                          // a device configuration arrived
	EventState                           // a device's live state changed
	EventPairingStarted                  // inclusion mode started
	EventPaired                          // a device joined during inclusion mode
	EventPairingEnded                    // inclusion mode ended
	EventForgotten                       // a device deleted from the Cube was dropped (ForgetDevice)
)

func (k EventKind) String() string {
	switch k {
	case EventConnected:
		return "connected"
	case EventDisconnected:
		return "disconnected"
	case EventHello:
		return "hello"
	case EventMetadata:
		return "metadata"
	case EventConfig:
		return "config"
	case EventState:
		return "state"
	case EventPairingStarted:
		return "pairing_started"
	case EventPaired:
		return "paired"
	case EventPairingEnded:
		return "pairing_ended"
	case EventForgotten:
		return "device_forgotten"
	}
	return "unknown"
}

// beginGreeting starts a new connection's greeting. When it is complete
// (its L line), devices the greeting did not mention any more are
// dropped: configurations without a C line and states without an entry
// in the L line, so a device deleted meanwhile does not linger.
func (s *Snapshot) beginGreeting() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.greeting, s.fresh = true, map[cube.RFAddress]bool{}
	s.forgotten = nil // the greeting says what the Cube holds
	s.mparts = s.mparts[:0]
}

// endGreetingLocked drops what the greeting did not repeat; listed are
// the devices of its L line.
func (s *Snapshot) endGreetingLocked(listed map[cube.RFAddress]bool) {
	s.greeting = false
	for rf := range s.configs {
		if !s.fresh[rf] {
			delete(s.configs, rf)
		}
	}
	for rf := range s.states {
		if !listed[rf] {
			delete(s.states, rf)
			delete(s.times, rf)
		}
	}
	kept := s.order[:0]
	for _, rf := range s.order {
		if _, ok := s.configs[rf]; ok || listed[rf] {
			kept = append(kept, rf)
		}
	}
	s.order = kept
	s.fresh = nil
}

// apply folds one incoming message into the snapshot and returns the
// events it caused. Messages that are replies to commands (S, A, F, E, D,
// N) are not handled here.
func (s *Snapshot) apply(m cube.Message, now time.Time) ([]Event, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.updated = now
	if s.since.IsZero() {
		s.since = now
	}
	switch m.Type {
	case 'H':
		h, err := cube.ParseHello(m.Data)
		if err != nil {
			return nil, err
		}
		s.hello = h
		return []Event{{Kind: EventHello, Greeting: s.greeting}}, nil
	case 'M':
		p, err := cube.ParseMetadataPart(m.Data)
		if err != nil {
			return nil, err
		}
		if p.Index == 0 {
			s.mparts = s.mparts[:0]
		}
		s.mparts = append(s.mparts, p)
		if len(s.mparts) < p.Count {
			return nil, nil
		}
		md, err := cube.ParseMetadata(s.mparts)
		s.mparts = s.mparts[:0]
		if err != nil {
			return nil, err
		}
		s.noteDelistedLocked(md)
		s.metadata = md
		return []Event{{Kind: EventMetadata, Greeting: s.greeting}}, nil
	case 'C':
		c, err := cube.ParseConfig(m.Data)
		if err != nil {
			return nil, err
		}
		s.configs[c.RFAddress] = c
		delete(s.forgotten, c.RFAddress)
		if s.greeting {
			s.fresh[c.RFAddress] = true
		}
		if c.Type != cube.DeviceCube {
			s.noteLocked(c.RFAddress)
		}
		return []Event{{Kind: EventConfig, Device: c.RFAddress, Greeting: s.greeting}}, nil
	case 'L':
		states, err := cube.ParseDeviceList(m.Data, s.typesLocked())
		if err != nil {
			return nil, err
		}
		// The first device list's values were there when the server
		// first heard from the Cube: they get exactly that time, the
		// Cube's observed_since, not the moment the line arrived.
		at := now
		if !s.listed {
			at, s.listed = s.since, true
		}
		var evs []Event
		listed := map[cube.RFAddress]bool{}
		for _, st := range states {
			if s.forgotten[st.RFAddress] {
				continue
			}
			listed[st.RFAddress] = true
			s.noteLocked(st.RFAddress)
			old, had := s.states[st.RFAddress]
			s.noteTimesLocked(old, had, st, at)
			if !had || !sameState(old, st) {
				evs = append(evs, Event{Kind: EventState, Device: st.RFAddress})
			}
			s.states[st.RFAddress] = st
		}
		if s.greeting {
			s.endGreetingLocked(listed)
		}
		return evs, nil
	}
	return nil, nil
}

func sameState(a, b cube.DeviceState) bool { return string(a.Raw) == string(b.Raw) }

// noteDelistedLocked keeps the entries of the devices the new room and
// device list md leaves out (delisted), and drops those it names again.
func (s *Snapshot) noteDelistedLocked(md *cube.Metadata) {
	listed := map[cube.RFAddress]bool{}
	for _, d := range md.Devices {
		listed[d.RFAddress] = true
		delete(s.delisted, d.RFAddress)
	}
	if s.metadata == nil {
		return
	}
	for _, d := range s.metadata.Devices {
		if listed[d.RFAddress] {
			continue
		}
		if s.delisted == nil {
			s.delisted = map[cube.RFAddress]cube.DeviceMeta{}
		}
		s.delisted[d.RFAddress] = d
	}
}

// noteLocked records a device address the first time it is seen.
func (s *Snapshot) noteLocked(rf cube.RFAddress) {
	for _, o := range s.order {
		if o == rf {
			return
		}
	}
	s.order = append(s.order, rf)
}

func (s *Snapshot) typesLocked() map[cube.RFAddress]cube.DeviceType {
	types := map[cube.RFAddress]cube.DeviceType{}
	if s.metadata != nil {
		for _, d := range s.metadata.Devices {
			types[d.RFAddress] = d.Type
		}
	}
	for rf, c := range s.configs {
		types[rf] = c.Type
	}
	return types
}

// Hello returns the latest H line, or nil before the first greeting.
func (s *Snapshot) Hello() *cube.Hello {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.hello
}

// Metadata returns the rooms and devices, or nil before the first greeting.
func (s *Snapshot) Metadata() *cube.Metadata {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.metadata
}

// Config returns a device's configuration, or nil.
func (s *Snapshot) Config(rf cube.RFAddress) *cube.DeviceConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.configs[rf]
}

// Serial returns a device's serial number: its entry in the room and
// device list, else its configuration's; "" for a device the snapshot
// does not know.
func (s *Snapshot) Serial(rf cube.RFAddress) string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.metadata != nil {
		for _, d := range s.metadata.Devices {
			if d.RFAddress == rf && d.Serial != "" {
				return d.Serial
			}
		}
	}
	if c := s.configs[rf]; c != nil {
		return c.Serial
	}
	return ""
}

// BySerial finds a device by its serial number, in any case: in the
// room and device list, else among the configurations. The Cube's own
// configuration is one of them, under the Cube's serial.
func (s *Snapshot) BySerial(serial string) (cube.RFAddress, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if serial == "" {
		return 0, false
	}
	if s.metadata != nil {
		for _, d := range s.metadata.Devices {
			if strings.EqualFold(d.Serial, serial) {
				return d.RFAddress, true
			}
		}
	}
	for rf, c := range s.configs {
		if strings.EqualFold(c.Serial, serial) {
			return rf, true
		}
	}
	return 0, false
}

// State returns a device's latest live state.
func (s *Snapshot) State(rf cube.RFAddress) (cube.DeviceState, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	st, ok := s.states[rf]
	return st, ok
}

// Devices returns all known device addresses in the order the Cube
// reported them; devices known only from the metadata come last.
func (s *Snapshot) Devices() []cube.RFAddress {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := append([]cube.RFAddress(nil), s.order...)
	if s.metadata != nil {
	next:
		for _, d := range s.metadata.Devices {
			for _, o := range out {
				if o == d.RFAddress {
					continue next
				}
			}
			out = append(out, d.RFAddress)
		}
	}
	return out
}

// Forget drops a device the Cube deleted (t:): its configuration, its
// state, its place in the Cube's order and its entry in the metadata,
// so that Devices no longer names it. The Cube's own metadata keeps the
// entry until a client writes it without the device, which the original
// software does right after t:. A device the Cube keeps, one that never
// answers its reset, stays in the Cube's L lines: they do not bring it
// back, but a C line for it does, and a new greeting brings back
// whatever the Cube still holds. It returns what the snapshot knew of
// the device, taken before it is dropped, and whether it knew it.
func (s *Snapshot) Forget(rf cube.RFAddress) (Gone, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.forgotten == nil {
		s.forgotten = map[cube.RFAddress]bool{}
	}
	s.forgotten[rf] = true
	var gone Gone
	c, known := s.configs[rf]
	if known {
		gone = Gone{Type: c.Type, Serial: c.Serial, RoomID: c.RoomID}
	}
	if st, ok := s.states[rf]; ok {
		if !known {
			gone.Type = st.Type
		}
		known = true
	}
	delete(s.configs, rf)
	delete(s.states, rf)
	delete(s.times, rf)
	if s.fresh != nil {
		delete(s.fresh, rf)
	}
	kept := s.order[:0]
	for _, o := range s.order {
		if o == rf {
			known = true
			continue
		}
		kept = append(kept, o)
	}
	s.order = kept
	// The list's entry names the device as the UI showed it: the current
	// list's, else the last one before a list left the device out.
	named := func(d cube.DeviceMeta) {
		gone.Type, gone.Name, gone.RoomID = d.Type, d.Name, d.RoomID
		if d.Serial != "" {
			gone.Serial = d.Serial
		}
	}
	if d, ok := s.delisted[rf]; ok {
		named(d)
		delete(s.delisted, rf)
	}
	if md := s.metadata; md != nil {
		for i, d := range md.Devices {
			if d.RFAddress != rf {
				continue
			}
			named(d)
			// A new value: readers hold the old one.
			cp := *md
			cp.Devices = append(append([]cube.DeviceMeta(nil), md.Devices[:i]...), md.Devices[i+1:]...)
			s.metadata = &cp
			known = true
			break
		}
	}
	return gone, known
}

// Updated is the time of the last message from the Cube.
func (s *Snapshot) Updated() time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.updated
}

// DeviceListLine returns the L message a fresh connection would receive
// now, or false before any device state is known.
func (s *Snapshot) DeviceListLine() (cube.Message, bool) { return s.DeviceListLineExcluding(nil) }

// DeviceListLineExcluding is DeviceListLine without the devices in skip.
func (s *Snapshot) DeviceListLineExcluding(skip map[cube.RFAddress]bool) (cube.Message, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var raw []byte
	for _, rf := range s.order {
		if st, ok := s.states[rf]; ok && !skip[rf] {
			raw = append(raw, st.Raw...)
		}
	}
	if raw == nil {
		return cube.Message{}, false
	}
	return cube.Message{Type: 'L', Data: b64(raw)}, true
}

// ConfigLine returns the C message for one device, or false if its
// configuration is unknown.
func (s *Snapshot) ConfigLine(rf cube.RFAddress) (cube.Message, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	c, ok := s.configs[rf]
	if !ok {
		return cube.Message{}, false
	}
	return cube.Message{Type: 'C', Data: rf.String() + "," + b64(c.Raw)}, true
}

// Lines reproduces the snapshot as protocol lines (H, M, C…, L), i.e. the
// same text a fresh connection would receive. Used for backups and for the
// simulator.
func (s *Snapshot) Lines() []cube.Message { return s.LinesExcluding(nil, nil) }

// LinesExcluding is Lines without the devices in skip, left out of the
// metadata, the C lines and the L line, and without the rooms in
// skipRooms. Rooms of skipped devices stay, empty, unless listed.
func (s *Snapshot) LinesExcluding(skip map[cube.RFAddress]bool, skipRooms map[uint8]bool) []cube.Message {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []cube.Message
	if s.hello != nil {
		out = append(out, cube.Message{Type: 'H', Data: joinFields(s.hello.Fields)})
	}
	if s.metadata != nil {
		md := s.metadata
		if len(skip) > 0 || len(skipRooms) > 0 {
			cp := *md
			cp.Devices, cp.Rooms = nil, nil
			for _, d := range md.Devices {
				if !skip[d.RFAddress] {
					cp.Devices = append(cp.Devices, d)
				}
			}
			for _, r := range md.Rooms {
				if !skipRooms[r.ID] {
					cp.Rooms = append(cp.Rooms, r)
				}
			}
			md = &cp
		}
		if cmds, err := cube.WriteMetadataCommands(md); err == nil {
			for _, c := range cmds {
				// m:NN,<data> → M:NN,<count>,<data>
				idx, data, _ := cutComma(c.Data)
				out = append(out, cube.Message{Type: 'M', Data: idx + "," + twoHex(len(cmds)) + "," + data})
			}
		}
	}
	for rf, c := range s.configs {
		if c.Type == cube.DeviceCube {
			out = append(out, cube.Message{Type: 'C', Data: rf.String() + "," + b64(c.Raw)})
		}
	}
	for _, rf := range s.order {
		if c, ok := s.configs[rf]; ok && !skip[rf] {
			out = append(out, cube.Message{Type: 'C', Data: rf.String() + "," + b64(c.Raw)})
		}
	}
	var raw []byte
	for _, rf := range s.order {
		if st, ok := s.states[rf]; ok && !skip[rf] {
			raw = append(raw, st.Raw...)
		}
	}
	if raw != nil {
		out = append(out, cube.Message{Type: 'L', Data: b64(raw)})
	}
	return out
}

// SnapshotFromLines builds a snapshot from recorded protocol lines (a
// greeting or a backup's raw lines), for tests and offline tools.
func SnapshotFromLines(lines []cube.Message) (*Snapshot, error) {
	s := newSnapshot()
	now := time.Now()
	for _, m := range lines {
		if err := s.Apply(m, now); err != nil {
			return nil, err
		}
	}
	return s, nil
}

// Apply folds a recorded line into the snapshot as if the Cube had sent
// it at the given time, for tests and offline tools.
func (s *Snapshot) Apply(m cube.Message, at time.Time) error {
	_, err := s.apply(m, at)
	return err
}
