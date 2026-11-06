// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

// Package fakecube is a TCP server that behaves like a MAX! Cube's LAN
// interface: it sends a greeting on connect and answers commands. It is
// used for tests and for developing against recorded data without
// hardware.
package fakecube

import (
	"bufio"
	"encoding/base64"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.metaworx.ch/massimo/internal/protocol/cube"
)

// Server is a fake Cube.
type Server struct {
	Greeting []cube.Message // sent on connect; the last one should be L
	NTP      []string
	Duty     int  // duty cycle reported in S replies (0..100)
	Discard  bool // make S replies report the command as discarded
	// DiscardIf, when set, makes the S reply to each s: or r: line it
	// reports report the command as discarded.
	DiscardIf func(cube.Message) bool
	// Pair, when set, is a device that joins during inclusion mode.
	Pair *Pairing
	// Keep names devices that t: does not delete, as the Cube keeps a
	// device that never answers.
	Keep map[cube.RFAddress]bool

	ln       net.Listener
	mu       sync.Mutex
	received []cube.Message
	conns    int
	open     map[net.Conn]bool
	joined   bool                    // the Pair device has joined
	configs  map[string]cube.Message // C lines set with SetConfig, by address
	list     *cube.Message           // the L line set with SetDeviceList
	deleted  map[cube.RFAddress]bool // deleted with t:
}

// paired returns the Pair device once it has joined.
func (s *Server) paired() *Pairing {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.joined {
		return s.Pair
	}
	return nil
}

// Pairing describes the device that joins after n:. Delay after the n:
// the fake sends its N line and from then on answers c: for it with
// Config; the inclusion ends with an empty N line when the n: timeout is
// over or on x:. This is how the real Cube is expected to behave; the
// empty N line in particular still has to be confirmed on hardware.
type Pairing struct {
	Device cube.NewDevice
	Config cube.Message // the device's C line
	Delay  time.Duration
}

// NewDeviceLine is the N line for d: type, address, serial, 0xff.
func NewDeviceLine(d cube.NewDevice) cube.Message {
	rf := d.RFAddress.Bytes()
	b := append([]byte{byte(d.Type)}, rf[:]...)
	b = append(append(b, fmt.Sprintf("%-10.10s", d.Serial)...), 0xff)
	return cube.Message{Type: 'N', Data: base64.StdEncoding.EncodeToString(b)}
}

// SetConfig makes c: for the line's device answer with line from now on,
// as the real Cube does once a device acknowledged a configuration write.
func (s *Server) SetConfig(line cube.Message) {
	rf, _, _ := strings.Cut(strings.ToLower(line.Data), ",")
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.configs == nil {
		s.configs = map[string]cube.Message{}
	}
	s.configs[rf] = line
}

// SetDeviceList makes l: answer with line from now on, as the real Cube
// does once devices reported new states.
func (s *Server) SetDeviceList(line cube.Message) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.list = &line
}

// deviceList returns the L line set with SetDeviceList.
func (s *Server) deviceList() (cube.Message, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.list == nil {
		return cube.Message{}, false
	}
	return *s.list, true
}

// config returns the C line set for rf with SetConfig.
func (s *Server) config(rf string) (cube.Message, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, ok := s.configs[strings.ToLower(strings.TrimSpace(rf))]
	return c, ok
}

// deleteDevices takes a t: (delete devices): every device not in Keep
// is gone from the C lines and the L line the fake answers with and
// from a new connection's greeting, as the Cube drops a device once it
// answered; the Pair device may join again with the next n:.
func (s *Server) deleteDevices(addrs []cube.RFAddress) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, rf := range addrs {
		if s.Keep[rf] {
			continue
		}
		if s.deleted == nil {
			s.deleted = map[cube.RFAddress]bool{}
		}
		s.deleted[rf] = true
		delete(s.configs, rf.String())
		if s.Pair != nil && s.Pair.Device.RFAddress == rf {
			s.joined = false
		}
	}
}

// isDeleted reports whether a t: deleted rf.
func (s *Server) isDeleted(rf cube.RFAddress) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.deleted[rf]
}

// visible returns a line the fake sends without the devices deleted
// with t:: an L line without their entries; false for a C line of one.
func (s *Server) visible(m cube.Message) (cube.Message, bool) {
	switch m.Type {
	case 'C':
		addr, _, _ := strings.Cut(m.Data, ",")
		if rf, err := cube.ParseRFAddress(addr); err == nil && s.isDeleted(rf) {
			return m, false
		}
	case 'L':
		raw, err := base64.StdEncoding.DecodeString(m.Data)
		if err != nil {
			return m, true
		}
		var kept []byte
		for i := 0; i < len(raw); {
			end := min(i+1+int(raw[i]), len(raw))
			if entry := raw[i:end]; len(entry) < 4 || !s.isDeleted(cube.RFAddressFromBytes(entry[1:4])) {
				kept = append(kept, entry...)
			}
			i = end
		}
		return cube.Message{Type: 'L', Data: base64.StdEncoding.EncodeToString(kept)}, true
	}
	return m, true
}

// LoadGreeting reads protocol lines from a capture file.
func LoadGreeting(path string) ([]cube.Message, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return cube.SplitLines(string(raw))
}

// Start listens on addr ("127.0.0.1:0" for a random port).
func (s *Server) Start(addr string) (string, error) {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return "", err
	}
	s.ln = ln
	if s.NTP == nil {
		s.NTP = []string{"ntp.homematic.com", "ntp.homematic.com"}
	}
	go s.accept()
	return ln.Addr().String(), nil
}

// Close stops the server and drops every client connection.
func (s *Server) Close() error {
	err := s.ln.Close()
	s.mu.Lock()
	for c := range s.open {
		c.Close()
	}
	s.mu.Unlock()
	return err
}

// Received returns every command received so far.
func (s *Server) Received() []cube.Message {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]cube.Message(nil), s.received...)
}

// Connections returns how many clients connected so far.
func (s *Server) Connections() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.conns
}

func (s *Server) accept() {
	for {
		conn, err := s.ln.Accept()
		if err != nil {
			return
		}
		s.mu.Lock()
		s.conns++
		if s.open == nil {
			s.open = map[net.Conn]bool{}
		}
		s.open[conn] = true
		s.mu.Unlock()
		go s.serve(conn)
	}
}

func (s *Server) serve(conn net.Conn) {
	defer func() {
		conn.Close()
		s.mu.Lock()
		delete(s.open, conn)
		s.mu.Unlock()
	}()
	var wmu sync.Mutex
	write := func(line string) error {
		wmu.Lock()
		defer wmu.Unlock()
		_, err := conn.Write([]byte(line))
		return err
	}
	for _, m := range s.Greeting {
		m, ok := s.visible(m)
		if !ok {
			continue
		}
		if err := write(m.Wire()); err != nil {
			return
		}
	}
	var inclusion *time.Timer // ends the running inclusion
	defer func() {
		if inclusion != nil {
			inclusion.Stop()
		}
	}()
	var last cube.Message
	for _, m := range s.Greeting {
		if m.Type == 'L' {
			last = m
		}
	}
	r := bufio.NewReader(conn)
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return
		}
		m, err := cube.ParseLine(line)
		if err != nil {
			continue
		}
		s.mu.Lock()
		s.received = append(s.received, m)
		s.mu.Unlock()
		var reply string
		switch m.Type {
		case 'q':
			return
		case 'l':
			l := last
			if set, ok := s.deviceList(); ok {
				l = set
			}
			l, _ = s.visible(l)
			reply = l.Wire()
		case 'c':
			reply = "A:\r\n"
			if rf, err := cube.ParseRFAddress(m.Data); err == nil && s.isDeleted(rf) {
				break
			}
			if p := s.paired(); p != nil && strings.EqualFold(strings.TrimSpace(m.Data), p.Device.RFAddress.String()) {
				reply = p.Config.Wire()
			}
			for _, g := range s.Greeting {
				if g.Type == 'C' && strings.HasPrefix(strings.ToLower(g.Data), strings.ToLower(strings.TrimSpace(m.Data))+",") {
					reply = g.Wire()
				}
			}
			if c, ok := s.config(m.Data); ok {
				reply = c.Wire()
			}
		case 's', 'r':
			d := 0
			if s.Discard || s.DiscardIf != nil && s.DiscardIf(m) {
				d = 1
			}
			reply = fmt.Sprintf("S:%02x,%d,31\r\n", s.Duty, d)
		case 'f':
			if m.Data != "" {
				s.NTP = strings.Split(m.Data, ",")
			}
			reply = "F:" + strings.Join(s.NTP, ",") + "\r\n"
		case 't':
			// One A line per address, as the Cube answered on hardware.
			reply = "A:\r\n"
			if _, addrs, err := cube.ParseDeleteDevices(m.Data); err == nil && len(addrs) > 0 {
				s.deleteDevices(addrs)
				reply = strings.Repeat("A:\r\n", len(addrs))
			}
		case 'e', 'd':
			reply = fmt.Sprintf("%c:%s\r\n", m.Type-32, m.Data)
		case 'n':
			// No immediate reply: N lines come when devices join.
			secs, _ := strconv.ParseUint(m.Data, 16, 16)
			if secs == 0 {
				secs = 60
			}
			if inclusion != nil {
				inclusion.Stop()
			}
			if p := s.Pair; p != nil {
				time.AfterFunc(p.Delay, func() {
					s.mu.Lock()
					s.joined = true
					delete(s.deleted, p.Device.RFAddress)
					s.mu.Unlock()
					write(NewDeviceLine(p.Device).Wire())
				})
			}
			inclusion = time.AfterFunc(time.Duration(secs)*time.Second, func() { write("N:\r\n") })
			continue
		case 'x':
			if inclusion != nil {
				inclusion.Stop()
				inclusion = nil
			}
			reply = "N:\r\n"
		default:
			reply = "A:\r\n"
		}
		if err := write(reply); err != nil {
			return
		}
	}
}

// PairingFrom builds a Pairing for a new device that is a copy of the
// device template in greeting with another address and serial, in no
// room, joining delay after n:.
func PairingFrom(greeting []cube.Message, template, rf cube.RFAddress, serial string, delay time.Duration) (*Pairing, error) {
	for _, g := range greeting {
		if g.Type != 'C' || !strings.HasPrefix(strings.ToLower(g.Data), template.String()+",") {
			continue
		}
		raw, err := base64.StdEncoding.DecodeString(g.Data[7:])
		if err != nil {
			return nil, err
		}
		b := rf.Bytes()
		copy(raw[1:4], b[:])
		raw[5] = 0 // no room
		copy(raw[8:18], fmt.Sprintf("%-10.10s", serial))
		c, err := cube.ParseConfig(rf.String() + "," + base64.StdEncoding.EncodeToString(raw))
		if err != nil {
			return nil, err
		}
		return &Pairing{
			Device: cube.NewDevice{Type: c.Type, RFAddress: rf, Serial: c.Serial},
			Config: cube.Message{Type: 'C', Data: rf.String() + "," + base64.StdEncoding.EncodeToString(raw)},
			Delay:  delay,
		}, nil
	}
	return nil, fmt.Errorf("fakecube: no C line for %s in the greeting", template)
}
