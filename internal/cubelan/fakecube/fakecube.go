// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

// Package fakecube is a TCP server that behaves like a MAX! Cube's LAN
// interface: it sends a greeting on connect and answers commands. It is
// used for tests and for developing against recorded data without
// hardware.
package fakecube

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"strings"
	"sync"

	"go.metaworx.ch/massimo/internal/protocol/cube"
)

// Server is a fake Cube.
type Server struct {
	Greeting []cube.Message // sent on connect; the last one should be L
	NTP      []string
	Duty     int  // duty cycle reported in S replies (0..100)
	Discard  bool // make S replies report the command as discarded

	ln       net.Listener
	mu       sync.Mutex
	received []cube.Message
	conns    int
	open     map[net.Conn]bool
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
	for _, m := range s.Greeting {
		if _, err := conn.Write([]byte(m.Wire())); err != nil {
			return
		}
	}
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
			reply = last.Wire()
		case 'c':
			reply = "A:\r\n"
			for _, g := range s.Greeting {
				if g.Type == 'C' && strings.HasPrefix(strings.ToLower(g.Data), strings.ToLower(strings.TrimSpace(m.Data))+",") {
					reply = g.Wire()
				}
			}
		case 's', 'r':
			d := 0
			if s.Discard {
				d = 1
			}
			reply = fmt.Sprintf("S:%02x,%d,31\r\n", s.Duty, d)
		case 'f':
			if m.Data != "" {
				s.NTP = strings.Split(m.Data, ",")
			}
			reply = "F:" + strings.Join(s.NTP, ",") + "\r\n"
		case 'e', 'd':
			reply = fmt.Sprintf("%c:%s\r\n", m.Type-32, m.Data)
		case 'n', 'x':
			reply = "N:\r\n"
		default:
			reply = "A:\r\n"
		}
		if _, err := conn.Write([]byte(reply)); err != nil {
			return
		}
	}
}
