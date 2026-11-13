// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package proxy

import (
	"encoding/base64"
	"fmt"
	"log/slog"
	"sort"
	"strings"

	"go.metaworx.ch/massimo/internal/protocol/cube"
	"go.metaworx.ch/massimo/internal/view"
)

// The virtual wall thermostats for proxy clients (docs/proxy.md, "Virtual
// wall thermostats"; docs/api-v1.md, "The virtual wall thermostat"). With
// Options.Virtual set, a client sees the API's view "virtual": every
// room has one wall thermostat, its virtual one, and the real wall
// thermostats are left out as hidden devices are. The original software
// allows one wall thermostat per room and shows a room by it, so it
// shows the room's temperature as Massimo's other clients do.
//
// Nothing of a virtual wall thermostat ever reaches the Cube: a metadata
// write is stripped of them, with the real wall thermostats put back; a
// command to one goes to its room, or to the room's real wall
// thermostat, or is answered by the proxy where there is none; a link
// with one is a link with the real wall thermostat, or answered.

// Virtual is a room's virtual wall thermostat as the proxy shows it.
type Virtual struct {
	Room   cube.RoomMeta
	RF     cube.RFAddress
	Serial string
	// Wall is the room's real wall thermostat, 0 for none; Member a real
	// thermostat of the room, the address a room command carries.
	Wall, Member cube.RFAddress
	// Entry is its L entry, with the length byte; Config its C payload.
	Entry, Config []byte
}

// virtuals is the virtual wall thermostats, by address; nil when they
// are off.
func (p *Proxy) virtuals() map[cube.RFAddress]Virtual {
	if p.opts.Virtual == nil {
		return nil
	}
	list := p.opts.Virtual()
	if len(list) == 0 {
		return nil
	}
	out := make(map[cube.RFAddress]Virtual, len(list))
	for _, v := range list {
		out[v.RF] = v
	}
	return out
}

// clientHidden is what a client does not see: the hidden devices, and
// with virtual wall thermostats the real wall thermostats of their rooms.
func (p *Proxy) clientHidden(md *cube.Metadata, vs map[cube.RFAddress]Virtual) view.Hidden {
	h := p.hidden(md)
	if len(vs) == 0 {
		return h
	}
	out := view.Hidden{}
	for rf := range h {
		out[rf] = true
	}
	for _, v := range vs {
		if v.Wall != 0 {
			out[v.Wall] = true
		}
	}
	return out
}

// virtualMetadata is the metadata a client sees: md without the devices
// it does not see, with the virtual wall thermostats in their rooms.
func virtualMetadata(md *cube.Metadata, skip view.Hidden, skipRooms map[uint8]bool, vs map[cube.RFAddress]Virtual) *cube.Metadata {
	cp := *md
	cp.Devices, cp.Rooms = nil, nil
	for _, r := range md.Rooms {
		if !skipRooms[r.ID] {
			cp.Rooms = append(cp.Rooms, r)
		}
	}
	for _, d := range md.Devices {
		if !skip[d.RFAddress] {
			cp.Devices = append(cp.Devices, d)
		}
	}
	for _, r := range cp.Rooms {
		for _, v := range ordered(vs) {
			if v.Room.ID == r.ID {
				cp.Devices = append(cp.Devices, cube.DeviceMeta{Type: cube.DeviceWallThermostat, RFAddress: v.RF, Serial: v.Serial, Name: r.Name, RoomID: r.ID})
			}
		}
	}
	return &cp
}

// virtualGreeting is the greeting with the virtual wall thermostats: its
// M lines rebuilt, their C lines before the L line, their entries in it.
func (p *Proxy) virtualGreeting(lines []cube.Message, md *cube.Metadata, skip view.Hidden, skipRooms map[uint8]bool, vs map[cube.RFAddress]Virtual) []cube.Message {
	var out []cube.Message
	metaDone := false
	for _, m := range lines {
		switch m.Type {
		case 'M':
			if metaDone {
				continue
			}
			metaDone = true
			cmds, err := cube.WriteMetadataCommands(virtualMetadata(md, skip, skipRooms, vs))
			if err != nil {
				p.log.Warn("proxy: metadata with virtual wall thermostats not encoded, the Cube's sent", "error", err)
				out = append(out, m)
				continue
			}
			for _, c := range cmds {
				idx, data, _ := strings.Cut(c.Data, ",")
				out = append(out, cube.Message{Type: 'M', Data: fmt.Sprintf("%s,%02x,%s", idx, len(cmds), data)})
			}
		case 'L':
			for _, v := range ordered(vs) {
				out = append(out, virtualConfigLine(v))
			}
			out = append(out, withVirtualEntries(m, vs))
		default:
			out = append(out, m)
		}
	}
	return out
}

// ordered is the virtual wall thermostats by address, so that every
// greeting lists them alike.
func ordered(vs map[cube.RFAddress]Virtual) []Virtual {
	out := make([]Virtual, 0, len(vs))
	for _, v := range vs {
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].RF < out[j].RF })
	return out
}

// virtualConfigLine is a virtual wall thermostat's C line.
func virtualConfigLine(v Virtual) cube.Message {
	return cube.Message{Type: 'C', Data: v.RF.String() + "," + base64.StdEncoding.EncodeToString(v.Config)}
}

// withVirtualEntries is an L line with the virtual wall thermostats'
// entries after the real devices'.
func withVirtualEntries(l cube.Message, vs map[cube.RFAddress]Virtual) cube.Message {
	raw, _ := base64.StdEncoding.DecodeString(l.Data)
	for _, v := range ordered(vs) {
		raw = append(raw, v.Entry...)
	}
	return cube.Message{Type: 'L', Data: base64.StdEncoding.EncodeToString(raw)}
}

// Offsets of an s: command's header.
const (
	sFlags  = 1
	sCmd    = 2
	sTarget = 6 // three bytes
	sRoom   = 9
	sBody   = 10
)

// routeVirtual is where a client's radio command naming a virtual wall
// thermostat goes: the line to send instead, or ok false where the proxy
// answers it and sends nothing. A command naming none comes back as it
// is.
func (p *Proxy) routeVirtual(m cube.Message, vs map[cube.RFAddress]Virtual, remote string) (cube.Message, bool) {
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(m.Data))
	if err != nil || len(raw) < sBody {
		return m, true
	}
	target := cube.RFAddressFromBytes(raw[sTarget : sTarget+3])
	v, isTarget := vs[target]
	var partner Virtual
	isPartner := false
	if (raw[sCmd] == cube.RFAddLinkPartner || raw[sCmd] == cube.RFRemoveLink) && len(raw) >= sBody+3 {
		partner, isPartner = vs[cube.RFAddressFromBytes(raw[sBody:sBody+3])]
	}
	if !isTarget && !isPartner {
		return m, true
	}
	put := func(at int, rf cube.RFAddress) {
		b := rf.Bytes()
		copy(raw[at:at+3], b[:])
	}
	answered := func(why string) (cube.Message, bool) {
		p.log.Info("proxy: a command to a virtual wall thermostat answered, not sent", "remote", remote, "command", m.String(), "why", why)
		return cube.Message{}, false
	}
	if isPartner {
		// A link with the virtual wall thermostat is one with the room's
		// real wall thermostat; without one there is nothing to link.
		if partner.Wall == 0 {
			return answered("no real wall thermostat to link")
		}
		put(sBody, partner.Wall)
		if !isTarget {
			return cube.Message{Type: 's', Data: base64.StdEncoding.EncodeToString(raw)}, true
		}
	}
	switch raw[sCmd] {
	case cube.RFSetTemperature, cube.RFSetWeekProgram:
		// To its room, as a real wall thermostat's reach the room.
		if v.Member == 0 {
			return answered("room without a thermostat")
		}
		raw[sFlags] |= 0x04
		raw[sRoom] = v.Room.ID
		put(sTarget, v.Member)
	default:
		// The settings, the display, the lock, the links and the group
		// are a real device's.
		if v.Wall == 0 {
			return answered("no real wall thermostat")
		}
		put(sTarget, v.Wall)
	}
	return cube.Message{Type: 's', Data: base64.StdEncoding.EncodeToString(raw)}, true
}

// stripVirtual takes the virtual wall thermostats out of a client's
// metadata and returns how many.
func stripVirtual(md *cube.Metadata, vs map[cube.RFAddress]Virtual) int {
	if len(vs) == 0 {
		return 0
	}
	var kept []cube.DeviceMeta
	for _, d := range md.Devices {
		if _, ok := vs[d.RFAddress]; !ok {
			kept = append(kept, d)
		}
	}
	n := len(md.Devices) - len(kept)
	md.Devices = kept
	return n
}

// refuseMetadata notes a metadata write the proxy cannot strip of the
// virtual wall thermostats: it is not sent, since the Cube must never
// store one.
func (p *Proxy) refuseMetadata(what, remote string, err error) {
	p.note(Event{Kind: EventWarning, Level: slog.LevelWarn, Remote: remote, Summary: map[string]any{"what": what, "virtual": true}, Err: err},
		"proxy: metadata write refused: with virtual wall thermostats on, a write the proxy cannot strip of them is not sent", "remote", remote, "what", what)
}
