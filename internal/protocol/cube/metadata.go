// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package cube

import (
	"bytes"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// Metadata is the room and device list stored in the Cube (M messages).
// HiddenRoom is the room id, by convention, of the parking room for
// devices that are paired but never report (hidden devices): the Cube counts
// such a device as a member of the room in its C line and discards every
// command to that room, and it never drops the device. Massimo hides the
// devices of this room and the room itself everywhere unless asked to
// show them. The original software allocates the lowest free id and so
// never reaches 255 on its own.
const HiddenRoom uint8 = 255

type Metadata struct {
	Header   [2]byte      `json:"-"` // observed as 'V', 0x02; preserved for round trips
	Rooms    []RoomMeta   `json:"rooms"`
	Devices  []DeviceMeta `json:"devices"`
	Trailing []byte       `json:"-"` // bytes after the device list (observed: 0x01); preserved
}

// RoomMeta describes one room.
type RoomMeta struct {
	ID        uint8     `json:"id"`
	Name      string    `json:"name"`
	RFAddress RFAddress `json:"rf_address"` // address of the first device in the room
}

// DeviceMeta describes one paired device.
type DeviceMeta struct {
	Type      DeviceType `json:"type"`
	RFAddress RFAddress  `json:"rf_address"`
	Serial    string     `json:"serial"`
	Name      string     `json:"name"`
	RoomID    uint8      `json:"room_id"`
}

// MetadataPart is one M message: index, count and a base64 fragment.
type MetadataPart struct {
	Index, Count int
	Data         string
}

// ParseMetadataPart parses the payload of one M message ("00,01,<base64>").
func ParseMetadataPart(data string) (MetadataPart, error) {
	f := strings.SplitN(data, ",", 3)
	if len(f) != 3 {
		return MetadataPart{}, fmt.Errorf("M: want 3 fields, got %d", len(f))
	}
	idx, err := strconv.ParseUint(f[0], 16, 8)
	if err != nil {
		return MetadataPart{}, fmt.Errorf("M: index %q: %w", f[0], err)
	}
	cnt, err := strconv.ParseUint(f[1], 16, 8)
	if err != nil {
		return MetadataPart{}, fmt.Errorf("M: count %q: %w", f[1], err)
	}
	return MetadataPart{Index: int(idx), Count: int(cnt), Data: f[2]}, nil
}

// ParseMetadata assembles M message parts and decodes them. The base64
// fragments are concatenated before decoding because the Cube splits the
// encoded string, not the binary data.
func ParseMetadata(parts []MetadataPart) (*Metadata, error) {
	if len(parts) == 0 {
		return nil, fmt.Errorf("M: no parts")
	}
	sorted := append([]MetadataPart(nil), parts...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Index < sorted[j].Index })
	var sb strings.Builder
	for i, p := range sorted {
		if p.Index != i {
			return nil, fmt.Errorf("M: part %d missing", i)
		}
		if p.Count != len(sorted) {
			return nil, fmt.Errorf("M: part %d says %d parts, have %d", i, p.Count, len(sorted))
		}
		sb.WriteString(strings.TrimSpace(p.Data))
	}
	b, err := decodeB64("M", sb.String())
	if err != nil {
		return nil, err
	}
	return DecodeMetadata(b)
}

// DecodeMetadata decodes the binary metadata block.
func DecodeMetadata(b []byte) (*Metadata, error) {
	r := byteReader{b: b, what: "M"}
	m := &Metadata{}
	copy(m.Header[:], r.next(2))
	nRooms := int(r.u8())
	for i := 0; i < nRooms && r.err == nil; i++ {
		var room RoomMeta
		room.ID = r.u8()
		room.Name = string(r.next(int(r.u8())))
		room.RFAddress = r.rf()
		m.Rooms = append(m.Rooms, room)
	}
	nDev := int(r.u8())
	for i := 0; i < nDev && r.err == nil; i++ {
		var d DeviceMeta
		d.Type = DeviceType(r.u8())
		d.RFAddress = r.rf()
		d.Serial = string(r.next(10))
		d.Name = string(r.next(int(r.u8())))
		d.RoomID = r.u8()
		m.Devices = append(m.Devices, d)
	}
	if r.err != nil {
		return nil, r.err
	}
	// Empty, not nil, when there are none, so that Encode writes none.
	m.Trailing = append([]byte{}, r.rest()...)
	return m, nil
}

// Encode returns the binary metadata block. Header and trailing bytes are
// written back as they were read; for newly built metadata the observed
// defaults ('V', 0x02 and 0x01) are used. A header of two zero bytes, never
// observed, reads as none and so encodes as the default.
func (m *Metadata) Encode() ([]byte, error) {
	var buf bytes.Buffer
	hdr := m.Header
	if hdr == [2]byte{} {
		hdr = [2]byte{'V', 0x02}
	}
	buf.Write(hdr[:])
	if len(m.Rooms) > 255 || len(m.Devices) > 255 {
		return nil, fmt.Errorf("M: too many rooms or devices")
	}
	buf.WriteByte(byte(len(m.Rooms)))
	for _, r := range m.Rooms {
		if len(r.Name) > 255 {
			return nil, fmt.Errorf("M: room name %q too long", r.Name)
		}
		buf.WriteByte(r.ID)
		buf.WriteByte(byte(len(r.Name)))
		buf.WriteString(r.Name)
		a := r.RFAddress.Bytes()
		buf.Write(a[:])
	}
	buf.WriteByte(byte(len(m.Devices)))
	for _, d := range m.Devices {
		if len(d.Serial) != 10 {
			return nil, fmt.Errorf("M: device %s: serial %q must have 10 characters", d.RFAddress, d.Serial)
		}
		if len(d.Name) > 255 {
			return nil, fmt.Errorf("M: device name %q too long", d.Name)
		}
		buf.WriteByte(byte(d.Type))
		a := d.RFAddress.Bytes()
		buf.Write(a[:])
		buf.WriteString(d.Serial)
		buf.WriteByte(byte(len(d.Name)))
		buf.WriteString(d.Name)
		buf.WriteByte(d.RoomID)
	}
	if m.Trailing != nil {
		buf.Write(m.Trailing)
	} else {
		buf.WriteByte(0x01)
	}
	return buf.Bytes(), nil
}

// MetadataBlockSize is the maximum length of the base64 data in one m
// command, according to the protocol docs.
const MetadataBlockSize = 1900

// WriteMetadataCommands returns the m commands that store m in the Cube.
//
// Not yet verified against hardware: the format follows the protocol docs.
func WriteMetadataCommands(m *Metadata) ([]Message, error) {
	raw, err := m.Encode()
	if err != nil {
		return nil, err
	}
	enc := b64.EncodeToString(raw)
	var out []Message
	for i := 0; len(enc) > 0; i++ {
		n := min(MetadataBlockSize, len(enc))
		out = append(out, Message{Type: 'm', Data: fmt.Sprintf("%02x,%s", i, enc[:n])})
		enc = enc[n:]
	}
	return out, nil
}

// byteReader is a small cursor that records the first error and then
// returns zero values, so decoders can be written linearly.
type byteReader struct {
	b    []byte
	off  int
	err  error
	what string
}

func (r *byteReader) next(n int) []byte {
	if r.err != nil {
		return make([]byte, n)
	}
	if r.off+n > len(r.b) {
		r.err = fmt.Errorf("%s: data truncated at offset %d (need %d bytes, have %d)", r.what, r.off, n, len(r.b)-r.off)
		return make([]byte, n)
	}
	s := r.b[r.off : r.off+n]
	r.off += n
	return s
}

func (r *byteReader) u8() uint8     { return r.next(1)[0] }
func (r *byteReader) rf() RFAddress { return RFAddressFromBytes(r.next(3)) }
func (r *byteReader) rest() []byte {
	if r.err != nil {
		return nil
	}
	return r.b[r.off:]
}
