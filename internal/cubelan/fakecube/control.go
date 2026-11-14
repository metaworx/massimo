// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package fakecube

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"go.metaworx.ch/massimo/internal/protocol/cube"
)

// The control of the fake: what a test does to the devices that the real
// Cube would learn from them by radio. A device reports a new state
// (SetState), leaves its room and comes back (Remove, Restore); the
// connections drop (Drop). The same is offered over HTTP on loopback
// (Control), for tests in another language that run `massimo fakecube`.

// State is a change of a device's reported state; nil fields stay.
type State struct {
	ActualTemp  *float64 `json:"actual_temp,omitempty"`
	Mode        *string  `json:"mode,omitempty"`
	Setpoint    *float64 `json:"setpoint,omitempty"`
	ValvePct    *int     `json:"valve_pct,omitempty"`
	WindowOpen  *bool    `json:"window_open,omitempty"`
	BatteryLow  *bool    `json:"battery_low,omitempty"`
	Error       *bool    `json:"error,omitempty"`
	LinkError   *bool    `json:"link_error,omitempty"`
	PanelLocked *bool    `json:"panel_locked,omitempty"`
	// Reported false makes the Cube hold no state for the device, as
	// after a Cube restart.
	Reported *bool `json:"reported,omitempty"`
}

// SetState changes rf's entry in the L line, as a device's report does.
func (s *Server) SetState(rf cube.RFAddress, st State) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	found := false
	for _, a := range s.addressesLocked() {
		found = found || a == rf
	}
	if !found {
		return fmt.Errorf("fakecube: no device %s", rf)
	}
	var err error
	s.editEntryLocked(rf, func(e []byte, t cube.DeviceType) {
		err = st.apply(e, t)
	})
	return err
}

// apply writes the state's fields into an L entry.
func (st State) apply(e []byte, t cube.DeviceType) error {
	flag := func(b *bool, i int, bit byte) {
		if b == nil {
			return
		}
		e[i] &^= bit
		if *b {
			e[i] |= bit
		}
	}
	flag(st.Reported, 5, 0x10)
	flag(st.Error, 5, 0x08)
	flag(st.BatteryLow, lFlags2, 0x80)
	flag(st.LinkError, lFlags2, 0x40)
	flag(st.PanelLocked, lFlags2, 0x20)
	if st.WindowOpen != nil {
		if t != cube.DeviceShutterContact {
			return fmt.Errorf("window_open: a %s has no window", t)
		}
		flag(st.WindowOpen, lFlags2, 0x02)
	}
	if !t.IsThermostat() {
		if st.Mode != nil || st.Setpoint != nil || st.ActualTemp != nil || st.ValvePct != nil {
			return fmt.Errorf("a %s has no temperatures", t)
		}
		return nil
	}
	if len(e) <= lTime {
		return fmt.Errorf("entry of %d bytes is too short", len(e))
	}
	if st.Mode != nil {
		m, err := cube.ParseMode(*st.Mode)
		if err != nil {
			return err
		}
		e[lFlags2] = e[lFlags2]&^0x03 | byte(m)
	}
	if st.Setpoint != nil {
		v, err := cube.EncodeTemp(*st.Setpoint)
		if err != nil {
			return err
		}
		e[lSetpoint] = e[lSetpoint]&0xc0 | v
	}
	if st.ValvePct != nil {
		if !t.IsRadiatorThermostat() || *st.ValvePct < 0 || *st.ValvePct > 100 {
			return fmt.Errorf("valve_pct: %d for a %s", *st.ValvePct, t)
		}
		e[lValve] = byte(*st.ValvePct)
	}
	if st.ActualTemp != nil {
		return encodeActual(e, t, *st.ActualTemp)
	}
	return nil
}

// Remove takes rf out of the Cube's lists, its room's included: its L
// entry, its C line and its metadata entry are gone until Restore, as a
// device another client removed. A connected client learns of it with
// the next greeting (Drop).
func (s *Server) Remove(rf cube.RFAddress) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.removed == nil {
		s.removed = map[cube.RFAddress]bool{}
	}
	s.removed[rf] = true
}

// Restore brings back a device Remove took out, in its room again.
func (s *Server) Restore(rf cube.RFAddress) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.removed, rf)
}

// withoutRemoved is an M line without the devices Remove took out; a
// room that named one as its first device names its next one.
func (s *Server) withoutRemoved(m cube.Message) cube.Message {
	s.mu.Lock()
	removed := len(s.removed) > 0
	gone := map[cube.RFAddress]bool{}
	for rf := range s.removed {
		gone[rf] = true
	}
	s.mu.Unlock()
	if !removed {
		return m
	}
	part, err := cube.ParseMetadataPart(m.Data)
	if err != nil || part.Count != 1 {
		return m
	}
	md, err := cube.ParseMetadata([]cube.MetadataPart{part})
	if err != nil {
		return m
	}
	var kept []cube.DeviceMeta
	for _, d := range md.Devices {
		if !gone[d.RFAddress] {
			kept = append(kept, d)
		}
	}
	md.Devices = kept
	for i, r := range md.Rooms {
		if !gone[r.RFAddress] {
			continue
		}
		for _, d := range kept {
			if d.RoomID == r.ID {
				md.Rooms[i].RFAddress = d.RFAddress
				break
			}
		}
	}
	raw, err := md.Encode()
	if err != nil {
		return m
	}
	return cube.Message{Type: 'M', Data: fmt.Sprintf("00,01,%s", base64.StdEncoding.EncodeToString(raw))}
}

// Control serves the control over HTTP on addr, which must be a loopback
// address, and returns the address it listens on. The routes:
//
//	GET  /devices                the devices' states, as the L line holds them
//	PUT  /devices/{rf}/state     a State as JSON
//	POST /devices/{rf}/remove    Remove
//	POST /devices/{rf}/restore   Restore
//	POST /drop                   Drop: the clients connect again
//	POST /silent                 SetSilent(true): the cable is out
//	DELETE /silent               SetSilent(false): the cable is back
//	POST /release                Release: a held dead session is forgotten
//	GET  /connections            the connections so far, open now, a dead session held, the restarts
//
// It stops with the fake.
func (s *Server) Control(addr string) (string, error) {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return "", fmt.Errorf("fakecube: control address: %w", err)
	}
	if ip := net.ParseIP(host); host != "localhost" && (ip == nil || !ip.IsLoopback()) {
		return "", fmt.Errorf("fakecube: control address %s is not a loopback address", addr)
	}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return "", err
	}
	srv := &http.Server{Handler: s.controlHandler(), ReadHeaderTimeout: 5 * time.Second}
	s.mu.Lock()
	s.control = srv
	s.mu.Unlock()
	go srv.Serve(ln)
	return ln.Addr().String(), nil
}

// controlHandler is the control's HTTP routes.
func (s *Server) controlHandler() http.Handler {
	mux := http.NewServeMux()
	device := func(w http.ResponseWriter, r *http.Request) (cube.RFAddress, bool) {
		rf, err := cube.ParseRFAddress(r.PathValue("rf"))
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return 0, false
		}
		return rf, true
	}
	mux.HandleFunc("GET /devices", func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		raw := s.listLocked()
		types := map[cube.RFAddress]cube.DeviceType{}
		for _, rf := range s.addressesLocked() {
			if c := s.configLocked(rf); c != nil {
				types[rf] = c.Type
			}
		}
		s.mu.Unlock()
		states, err := cube.ParseDeviceList(base64.StdEncoding.EncodeToString(raw), types)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(states)
	})
	mux.HandleFunc("PUT /devices/{rf}/state", func(w http.ResponseWriter, r *http.Request) {
		rf, ok := device(w, r)
		if !ok {
			return
		}
		var st State
		dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&st); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := s.SetState(rf, st); err != nil {
			code := http.StatusBadRequest
			if strings.Contains(err.Error(), "no device") {
				code = http.StatusNotFound
			}
			http.Error(w, err.Error(), code)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("POST /devices/{rf}/remove", func(w http.ResponseWriter, r *http.Request) {
		if rf, ok := device(w, r); ok {
			s.Remove(rf)
			w.WriteHeader(http.StatusNoContent)
		}
	})
	mux.HandleFunc("POST /devices/{rf}/restore", func(w http.ResponseWriter, r *http.Request) {
		if rf, ok := device(w, r); ok {
			s.Restore(rf)
			w.WriteHeader(http.StatusNoContent)
		}
	})
	mux.HandleFunc("POST /drop", func(w http.ResponseWriter, r *http.Request) {
		s.Drop()
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("POST /silent", func(w http.ResponseWriter, r *http.Request) {
		s.SetSilent(true)
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("DELETE /silent", func(w http.ResponseWriter, r *http.Request) {
		s.SetSilent(false)
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("POST /release", func(w http.ResponseWriter, r *http.Request) {
		s.Release()
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("GET /connections", func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		v := struct {
			Total    int        `json:"total"`
			Open     int        `json:"open"`
			Dead     bool       `json:"dead"`
			Until    *time.Time `json:"dead_until,omitempty"`
			Restarts int        `json:"restarts"`
		}{Total: s.conns, Open: len(s.open), Dead: !s.deadUntil.IsZero(), Restarts: s.restarts}
		if v.Dead {
			u := s.deadUntil
			v.Until = &u
		}
		s.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(v)
	})
	return mux
}
