// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

// Package server keeps every configured Cube connected, runs a command
// queue per Cube and exposes state and commands over a JSON HTTP API.
//
// Routes (all JSON unless noted):
//
//	GET  /healthz
//	GET  /api/v1/cubes                               every Cube with connection and budget
//	GET  /api/v1/cubes/{sn}                          hello, budget, rooms, configuration
//	GET  /api/v1/cubes/{sn}/rooms                    rooms with their devices and states
//	GET  /api/v1/cubes/{sn}/devices                  all devices
//	GET  /api/v1/cubes/{sn}/devices/{rf}             one device: configuration and state
//	PUT  /api/v1/cubes/{sn}/devices/{rf}/setpoint    {"mode":"manual","temp":21.5} | {"mode":"auto"} |
//	                                                 {"mode":"vacation","temp":17,"until":"2026-11-10 12:00"}
//	PUT  /api/v1/cubes/{sn}/rooms/{id}/setpoint      same body, addressed to the whole room
//	GET  /api/v1/cubes/{sn}/backup?format=yaml|json  a backup file
//	GET  /api/v1/cubes/{sn}/events                   server-sent events: connection and state changes
//
// With a token configured, every /api request needs "Authorization: Bearer <token>".
//
// With proxy listeners configured, the server also answers as a Cube on
// the LAN (TCP 62910 and UDP discovery), see internal/proxy.
package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.metaworx.ch/massimo/internal/backup"
	"go.metaworx.ch/massimo/internal/cubelan"
	"go.metaworx.ch/massimo/internal/engine"
	"go.metaworx.ch/massimo/internal/protocol/cube"
	"go.metaworx.ch/massimo/internal/proxy"
)

// CubeConfig names one Cube to serve.
type CubeConfig struct {
	Serial string // for the URL and the logs; "" takes the one the Cube reports
	Addr   string // host[:port]
	// Ignore hides devices (and the rooms that hold only them) from every
	// view and keeps every write away from them; see backup.Ignored.
	Ignore []cube.RFAddress
}

// Options configure a Server.
type Options struct {
	Cubes        []CubeConfig
	Token        string        // API token; empty disables authentication
	PollInterval time.Duration // L polling; 0 keeps the driver default of one minute
	Logger       *slog.Logger
	Version      string
	Queue        engine.Options
	// Proxy lists the LAN listeners on which the server itself answers
	// as a Cube (see internal/proxy), one per Cube.
	Proxy []ProxyListener
	// ShowGhosts shows the devices of cube.GhostRoom and the ignore
	// lists' devices everywhere instead of hiding them.
	ShowGhosts bool
}

// ProxyListener describes one Cube-protocol listener of the server.
type ProxyListener struct {
	Serial string // which Cube; empty with a single configured Cube
	Addr   string // host:port to listen on, usually ":62910"
	// Discovery answers UDP discovery requests for this Cube; only
	// meaningful for a listener on the Cube's port 62910.
	Discovery bool
	// DiscoverySerial replaces the serial in discovery replies (the
	// original software keeps one answer per serial).
	DiscoverySerial string
}

// Server is the HTTP API over a set of Cubes.
type Server struct {
	opts  Options
	log   *slog.Logger
	mu    sync.RWMutex
	cubes map[string]*runtime // by serial, lower case
	order []string
	mux   *http.ServeMux
}

type runtime struct {
	cfg        CubeConfig
	client     *cubelan.Client
	queue      *engine.Queue
	serial     string
	ignore     backup.Ignored // from the configuration; hidden() adds the ghost room
	showGhosts bool

	subs   map[chan event]struct{}
	subsMu sync.Mutex
	push   map[chan struct{}]struct{} // proxy clients waiting for L pushes
}

// subscribePush returns a channel that gets a signal whenever the Cube
// reported device state, for the LAN proxy.
func (rt *runtime) subscribePush() (<-chan struct{}, func()) {
	ch := make(chan struct{}, 1)
	rt.subsMu.Lock()
	rt.push[ch] = struct{}{}
	rt.subsMu.Unlock()
	return ch, func() {
		rt.subsMu.Lock()
		delete(rt.push, ch)
		rt.subsMu.Unlock()
	}
}

type event struct {
	Kind   string      `json:"kind"`
	Device string      `json:"device,omitempty"`
	Error  string      `json:"error,omitempty"`
	State  interface{} `json:"state,omitempty"`
	Time   time.Time   `json:"time"`
}

// New creates a server; Run starts the Cube connections.
func New(opts Options) *Server {
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}
	if opts.PollInterval == 0 {
		opts.PollInterval = time.Minute
	}
	s := &Server{opts: opts, log: opts.Logger, cubes: map[string]*runtime{}}
	for _, c := range opts.Cubes {
		rt := &runtime{cfg: c, serial: strings.ToUpper(c.Serial), ignore: backup.IgnoreSet(c.Ignore), showGhosts: opts.ShowGhosts, subs: map[chan event]struct{}{}, push: map[chan struct{}]struct{}{}}
		rt.client = cubelan.New(cubelan.Options{Addr: c.Addr, PollInterval: opts.PollInterval, Logger: opts.Logger})
		rt.queue = engine.NewQueue(rt.client, opts.Queue)
		key := strings.ToLower(c.Serial)
		if key == "" {
			key = strings.ToLower(c.Addr)
		}
		s.cubes[key] = rt
		s.order = append(s.order, key)
	}
	s.routes()
	return s
}

// Run connects every Cube and blocks until ctx ends.
func (s *Server) Run(ctx context.Context) error {
	var wg sync.WaitGroup
	for _, key := range s.order {
		rt := s.cubes[key]
		wg.Add(3)
		go func() { defer wg.Done(); rt.client.Run(ctx) }()
		go func() { defer wg.Done(); rt.queue.Run(ctx) }()
		go func() { defer wg.Done(); s.pump(ctx, rt) }()
	}
	var responder *proxy.Responder
	for _, pl := range s.opts.Proxy {
		rt, err := s.proxyRuntime(pl)
		if err != nil {
			s.log.Error("proxy listener not started", "addr", pl.Addr, "error", err)
			continue
		}
		p := proxy.New(proxy.Options{Client: rt.client, Queue: rt.queue, DiscoverySerial: pl.DiscoverySerial, Ignore: rt.cfg.Ignore, ShowGhosts: rt.showGhosts,
			MaxDutyPct: s.opts.Queue.MaxDutyPct, Subscribe: rt.subscribePush, Logger: s.log})
		if pl.Discovery {
			if responder == nil {
				responder = proxy.NewResponder(s.log)
			}
			host, _, _ := net.SplitHostPort(pl.Addr)
			if err := responder.Add(p, net.ParseIP(host)); err != nil {
				s.log.Error("proxy discovery not started", "addr", pl.Addr, "error", err)
			}
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := p.Run(ctx, pl.Addr); err != nil && ctx.Err() == nil {
				s.log.Error("proxy listener failed", "addr", pl.Addr, "error", err)
			}
		}()
	}
	if responder != nil {
		pc, err := net.ListenPacket("udp4", fmt.Sprintf(":%d", cubelan.DiscoveryPort))
		if err != nil {
			s.log.Warn("proxy: discovery port not available, clients must be given the address", "port", cubelan.DiscoveryPort, "error", err)
		} else {
			wg.Add(1)
			go func() { defer wg.Done(); responder.Serve(ctx, pc) }()
		}
	}
	wg.Wait()
	return ctx.Err()
}

// hidden is the set of devices left out of every view: the configured
// ignore list plus the devices the Cube places in the ghost room.
func (rt *runtime) hidden() backup.Ignored {
	return backup.Hidden(rt.cfg.Ignore, rt.client.Snapshot().Metadata(), rt.showGhosts)
}

// proxyRuntime picks the Cube a proxy listener serves.
func (s *Server) proxyRuntime(pl ProxyListener) (*runtime, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if pl.Serial == "" {
		if len(s.order) != 1 {
			return nil, fmt.Errorf("several cubes configured, name one as <serial>=%s", pl.Addr)
		}
		return s.cubes[s.order[0]], nil
	}
	if rt, ok := s.cubes[strings.ToLower(pl.Serial)]; ok {
		return rt, nil
	}
	return nil, fmt.Errorf("no cube %s configured", pl.Serial)
}

// pump forwards client events to the SSE subscribers and learns the
// serial once the Cube is connected.
func (s *Server) pump(ctx context.Context, rt *runtime) {
	for {
		select {
		case <-ctx.Done():
			return
		case e := <-rt.client.Events():
			if e.Kind == cubelan.EventConnected {
				if h := rt.client.Snapshot().Hello(); h != nil && rt.serial == "" {
					rt.serial = h.Serial
					s.mu.Lock()
					s.cubes[strings.ToLower(h.Serial)] = rt
					s.mu.Unlock()
				}
			}
			if e.Device != 0 && rt.hidden()[e.Device] {
				continue
			}
			ev := event{Kind: e.Kind.String(), Time: time.Now()}
			if e.Device != 0 {
				ev.Device = e.Device.String()
				if st, ok := rt.client.Snapshot().State(e.Device); ok {
					ev.State = deviceView(rt.client.Snapshot(), e.Device, st)
				}
			}
			if e.Err != nil {
				ev.Error = e.Err.Error()
			}
			rt.subsMu.Lock()
			for ch := range rt.subs {
				select {
				case ch <- ev:
				default: // slow subscriber: drop
				}
			}
			if e.Kind == cubelan.EventState || e.Kind == cubelan.EventConnected {
				for ch := range rt.push {
					select {
					case ch <- struct{}{}:
					default: // a signal is already pending
					}
				}
			}
			rt.subsMu.Unlock()
		}
	}
}

// Handler returns the HTTP handler.
func (s *Server) Handler() http.Handler { return s.mux }

func (s *Server) routes() {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "version": s.opts.Version})
	})
	api := func(pattern string, h func(http.ResponseWriter, *http.Request) error) {
		mux.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) {
			if s.opts.Token != "" && r.Header.Get("Authorization") != "Bearer "+s.opts.Token {
				writeError(w, http.StatusUnauthorized, "missing or wrong token")
				return
			}
			if err := h(w, r); err != nil {
				var he *httpError
				if errors.As(err, &he) {
					if len(he.noState) > 0 {
						writeJSON(w, he.status, map[string]any{"error": he.msg, "no_state": he.noState})
						return
					}
					writeError(w, he.status, he.msg)
					return
				}
				writeError(w, http.StatusInternalServerError, err.Error())
			}
		})
	}
	api("GET /api/v1/cubes", s.listCubes)
	api("GET /api/v1/cubes/{sn}", s.getCube)
	api("GET /api/v1/cubes/{sn}/rooms", s.getRooms)
	api("GET /api/v1/cubes/{sn}/devices", s.getDevices)
	api("GET /api/v1/cubes/{sn}/devices/{rf}", s.getDevice)
	api("PUT /api/v1/cubes/{sn}/devices/{rf}/setpoint", s.setDeviceSetpoint)
	api("PUT /api/v1/cubes/{sn}/rooms/{id}/setpoint", s.setRoomSetpoint)
	api("DELETE /api/v1/cubes/{sn}/rooms/{id}", s.deleteRoom)
	api("PUT /api/v1/cubes/{sn}/devices/{rf}/lock", s.setDeviceLock)
	api("GET /api/v1/cubes/{sn}/backup", s.getBackup)
	api("GET /api/v1/cubes/{sn}/events", s.events)
	s.mux = mux
}

type httpError struct {
	status int
	msg    string
	// noState lists room members the Cube holds no state for when a
	// room command was discarded for that reason.
	noState []string
}

func (e *httpError) Error() string { return e.msg }

func fail(status int, format string, args ...any) error {
	return &httpError{status: status, msg: fmt.Sprintf(format, args...)}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func (s *Server) cube(r *http.Request) (*runtime, error) {
	key := strings.ToLower(r.PathValue("sn"))
	s.mu.RLock()
	rt := s.cubes[key]
	s.mu.RUnlock()
	if rt == nil {
		return nil, fail(http.StatusNotFound, "no cube %q", r.PathValue("sn"))
	}
	return rt, nil
}

// Views.

type cubeView struct {
	Serial    string         `json:"serial"`
	Address   string         `json:"address"`
	Connected bool           `json:"connected"`
	Firmware  string         `json:"firmware,omitempty"`
	RFAddress string         `json:"rf_address,omitempty"`
	Budget    cubelan.Budget `json:"budget"`
	// Pending counts radiograms the Cube sent and no device has
	// acknowledged yet (memory slots in use); their effect is not in
	// the device states yet.
	Pending  int              `json:"pending_radiograms"`
	Updated  time.Time        `json:"updated,omitempty"`
	Rooms    int              `json:"rooms"`
	Devices  int              `json:"devices"`
	Queued   int              `json:"queued"`
	Config   *cube.CubeConfig `json:"config,omitempty"`
	RoomList []roomView       `json:"room_list,omitempty"`
	CubeTime string           `json:"cube_time,omitempty"`
}

type roomView struct {
	ID   uint8  `json:"id"`
	Name string `json:"name"`
	// Ready is true when every device of the room has reported since
	// the Cube's last restart; the Cube discards room commands otherwise.
	Ready   bool          `json:"ready"`
	Devices []deviceState `json:"devices"`
}

type deviceState struct {
	RFAddress string          `json:"rf_address"`
	Type      cube.DeviceType `json:"type"`
	Name      string          `json:"name"`
	RoomID    uint8           `json:"room_id"`
	Serial    string          `json:"serial,omitempty"`
	Unlisted  bool            `json:"unlisted,omitempty"`
	// Reported is true when the Cube holds state for the device, that
	// is, the device has transmitted since the Cube's last restart.
	Reported bool          `json:"reported"`
	State    *backup.State `json:"state,omitempty"`
}

type deviceDetail struct {
	deviceState
	Firmware   uint8                  `json:"firmware,omitempty"`
	Thermostat *cube.ThermostatConfig `json:"thermostat,omitempty"`
	Wall       *cube.WallConfig       `json:"wall_thermostat,omitempty"`
}

func (s *Server) summary(rt *runtime) cubeView {
	snap := rt.client.Snapshot()
	b := rt.client.Budget()
	v := cubeView{Serial: rt.serial, Address: rt.cfg.Addr, Budget: b, Pending: b.Pending(), Queued: rt.queue.Len()}
	if h := snap.Hello(); h != nil {
		v.Connected = !snap.Updated().IsZero()
		v.Firmware, v.RFAddress, v.Updated = h.FirmwareVersion, h.RFAddress.String(), snap.Updated()
		v.CubeTime = strings.TrimSpace(h.CubeDate + " " + h.CubeTime)
		if c := snap.Config(h.RFAddress); c != nil {
			v.Config = c.Cube
		}
	}
	if md := snap.Metadata(); md != nil {
		ig := rt.hidden()
		hidden := ig.HiddenRooms(md)
		v.Rooms = len(md.Rooms) - len(hidden)
		for _, d := range md.Devices {
			if !ig[d.RFAddress] {
				v.Devices++
			}
		}
	}
	return v
}

func deviceView(snap *cubelan.Snapshot, rf cube.RFAddress, st cube.DeviceState) deviceState {
	d := deviceState{RFAddress: rf.String(), Type: st.Type}
	if md := snap.Metadata(); md != nil {
		found := false
		for _, m := range md.Devices {
			if m.RFAddress == rf {
				d.Name, d.RoomID, d.Serial, d.Type, found = m.Name, m.RoomID, m.Serial, m.Type, true
			}
		}
		d.Unlisted = !found
	}
	if c := snap.Config(rf); c != nil {
		d.Type = c.Type
		if d.Serial == "" {
			d.Serial, d.RoomID = c.Serial, c.RoomID
		}
	}
	d.Reported = st.Valid
	if st.Valid {
		d.State = &backup.State{Mode: st.Mode, Setpoint: st.Setpoint, Until: st.Until, ActualTemp: st.ActualTemp, ValvePct: st.ValvePct,
			WindowOpen: st.WindowOpen, BatteryLow: st.BatteryLow, Error: st.Error, LinkError: st.LinkError, PanelLocked: st.PanelLocked}
	}
	return d
}

func (s *Server) devices(rt *runtime) []deviceState {
	snap := rt.client.Snapshot()
	var out []deviceState
	h := snap.Hello()
	ig := rt.hidden()
	for _, rf := range snap.Devices() {
		if h != nil && rf == h.RFAddress || ig[rf] {
			continue
		}
		st, _ := snap.State(rf)
		st.RFAddress = rf
		out = append(out, deviceView(snap, rf, st))
	}
	return out
}

// Handlers.

func (s *Server) listCubes(w http.ResponseWriter, r *http.Request) error {
	var out []cubeView
	seen := map[*runtime]bool{}
	s.mu.RLock()
	for _, key := range s.order {
		rt := s.cubes[key]
		if !seen[rt] {
			seen[rt] = true
			out = append(out, s.summary(rt))
		}
	}
	s.mu.RUnlock()
	writeJSON(w, http.StatusOK, out)
	return nil
}

func (s *Server) getCube(w http.ResponseWriter, r *http.Request) error {
	rt, err := s.cube(r)
	if err != nil {
		return err
	}
	v := s.summary(rt)
	v.RoomList = s.rooms(rt)
	writeJSON(w, http.StatusOK, v)
	return nil
}

func (s *Server) rooms(rt *runtime) []roomView {
	snap := rt.client.Snapshot()
	md := snap.Metadata()
	if md == nil {
		return nil
	}
	devs := s.devices(rt)
	ig := rt.hidden()
	hidden := ig.HiddenRooms(md)
	var out []roomView
	for _, r := range md.Rooms {
		if hidden[r.ID] {
			continue
		}
		rv := roomView{ID: r.ID, Name: r.Name, Ready: true, Devices: []deviceState{}}
		for _, d := range devs {
			if d.RoomID == r.ID && !d.Unlisted {
				rv.Devices = append(rv.Devices, d)
				rv.Ready = rv.Ready && d.Reported
			}
		}
		out = append(out, rv)
	}
	return out
}

func (s *Server) getRooms(w http.ResponseWriter, r *http.Request) error {
	rt, err := s.cube(r)
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, s.rooms(rt))
	return nil
}

func (s *Server) getDevices(w http.ResponseWriter, r *http.Request) error {
	rt, err := s.cube(r)
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, s.devices(rt))
	return nil
}

func (s *Server) getDevice(w http.ResponseWriter, r *http.Request) error {
	rt, err := s.cube(r)
	if err != nil {
		return err
	}
	rf, err := cube.ParseRFAddress(r.PathValue("rf"))
	if err != nil {
		return fail(http.StatusBadRequest, "%v", err)
	}
	snap := rt.client.Snapshot()
	st, ok := snap.State(rf)
	c := snap.Config(rf)
	if !ok && c == nil || rt.hidden()[rf] {
		return fail(http.StatusNotFound, "no device %s", rf)
	}
	st.RFAddress = rf
	d := deviceDetail{deviceState: deviceView(snap, rf, st)}
	if c != nil {
		d.Firmware, d.Thermostat, d.Wall = c.Firmware, c.Thermostat, c.Wall
		if d.Wall != nil && st.Valid {
			w := *d.Wall
			w.DisplayActual = st.DisplayActual
			d.Wall = &w
		}
	}
	writeJSON(w, http.StatusOK, d)
	return nil
}

// setpointRequest is the body of the setpoint endpoints.
type setpointRequest struct {
	Mode  string  `json:"mode"`            // auto, manual, vacation
	Temp  float64 `json:"temp,omitempty"`  // °C in 0.5 steps; 0 with auto follows the program
	Until string  `json:"until,omitempty"` // vacation end, "YYYY-MM-DD HH:MM"
}

func (s *Server) setDeviceSetpoint(w http.ResponseWriter, r *http.Request) error {
	rt, err := s.cube(r)
	if err != nil {
		return err
	}
	rf, err := cube.ParseRFAddress(r.PathValue("rf"))
	if err != nil {
		return fail(http.StatusBadRequest, "%v", err)
	}
	snap := rt.client.Snapshot()
	c := snap.Config(rf)
	if c == nil || rt.hidden()[rf] {
		return fail(http.StatusNotFound, "no device %s", rf)
	}
	if !c.Type.IsRadiatorThermostat() && c.Type != cube.DeviceWallThermostat {
		return fail(http.StatusBadRequest, "device %s is a %s and has no setpoint", rf, c.Type)
	}
	return s.setpoint(w, r, rt, cube.SetpointTarget(rf, c.RoomID), rf, "device:"+rf.String())
}

// setDeviceLock locks or unlocks the buttons of one thermostat with
// radiogram 0x80, sent directly to the device after a wakeup of its
// room, like the other per-device settings.
func (s *Server) setDeviceLock(w http.ResponseWriter, r *http.Request) error {
	rt, err := s.cube(r)
	if err != nil {
		return err
	}
	rf, err := cube.ParseRFAddress(r.PathValue("rf"))
	if err != nil {
		return fail(http.StatusBadRequest, "%v", err)
	}
	c := rt.client.Snapshot().Config(rf)
	if c == nil || rt.hidden()[rf] {
		return fail(http.StatusNotFound, "no device %s", rf)
	}
	if !c.Type.IsRadiatorThermostat() && c.Type != cube.DeviceWallThermostat {
		return fail(http.StatusBadRequest, "device %s is a %s and has no key lock", rf, c.Type)
	}
	var req struct {
		Locked *bool `json:"locked"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Locked == nil {
		return fail(http.StatusBadRequest, "body must be {\"locked\": true|false}")
	}
	m := cube.CmdLockControls(cube.Target{RFAddress: rf}, *req.Locked)
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Minute)
	defer cancel()
	res := rt.queue.Enqueue(engine.Command{Key: "lock:" + rf.String(), Priority: engine.PriorityHigh, Target: rf, RoomID: c.RoomID, Wakeup: c.RoomID != 0, Lines: []cube.Message{m}}).Wait(ctx)
	if res.Err != nil {
		status := http.StatusBadGateway
		if errors.Is(res.Err, engine.ErrSuperseded) {
			status = http.StatusConflict
		}
		return fail(status, "%v", res.Err)
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "discarded": res.Discarded, "budget": res.Budget})
	return nil
}

// deleteRoom removes an empty room from the Cube's metadata. A room
// with devices, visible or hidden, is refused: the devices would have to
// be moved first, which the API does not do yet.
func (s *Server) deleteRoom(w http.ResponseWriter, r *http.Request) error {
	rt, err := s.cube(r)
	if err != nil {
		return err
	}
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil || id < 1 || id > 255 {
		return fail(http.StatusBadRequest, "bad room id %q", r.PathValue("id"))
	}
	md := rt.client.Snapshot().Metadata()
	if md == nil {
		return fail(http.StatusServiceUnavailable, "cube not connected yet")
	}
	hidden := rt.hidden().HiddenRooms(md)
	found := false
	for _, room := range md.Rooms {
		if int(room.ID) == id && !hidden[room.ID] {
			found = true
		}
	}
	if !found {
		return fail(http.StatusNotFound, "no room %d", id)
	}
	for _, d := range md.Devices {
		if int(d.RoomID) == id {
			return fail(http.StatusConflict, "room %d still has device %s; move it first", id, d.RFAddress)
		}
	}
	cp := *md
	cp.Rooms = nil
	for _, room := range md.Rooms {
		if int(room.ID) != id {
			cp.Rooms = append(cp.Rooms, room)
		}
	}
	lines, err := cube.WriteMetadataCommands(&cp)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Minute)
	defer cancel()
	res := rt.queue.Enqueue(engine.Command{Key: "metadata", Priority: engine.PriorityHigh, Lines: lines}).Wait(ctx)
	if res.Err != nil {
		return fail(http.StatusBadGateway, "%v", res.Err)
	}
	for _, m := range lines {
		idx, data, _ := strings.Cut(m.Data, ",")
		rt.client.ApplyLocal(cube.Message{Type: 'M', Data: idx + "," + fmt.Sprintf("%02x", len(lines)) + "," + data})
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "rooms": len(cp.Rooms)})
	return nil
}

func (s *Server) setRoomSetpoint(w http.ResponseWriter, r *http.Request) error {
	rt, err := s.cube(r)
	if err != nil {
		return err
	}
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil || id < 1 || id > 255 {
		return fail(http.StatusBadRequest, "bad room id %q", r.PathValue("id"))
	}
	md := rt.client.Snapshot().Metadata()
	if md == nil {
		return fail(http.StatusServiceUnavailable, "cube not connected yet")
	}
	ig := rt.hidden()
	hidden := ig.HiddenRooms(md)
	for _, room := range md.Rooms {
		if int(room.ID) == id && !hidden[room.ID] {
			return s.setpoint(w, r, rt, cube.Target{RFAddress: room.RFAddress, RoomID: room.ID, Room: true}, 0, fmt.Sprintf("room:%d", id))
		}
	}
	return fail(http.StatusNotFound, "no room %d", id)
}

func (s *Server) setpoint(w http.ResponseWriter, r *http.Request, rt *runtime, t cube.Target, target cube.RFAddress, key string) error {
	var req setpointRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return fail(http.StatusBadRequest, "bad body: %v", err)
	}
	mode, err := cube.ParseMode(req.Mode)
	if err != nil {
		return fail(http.StatusBadRequest, "mode must be auto, manual or vacation")
	}
	if mode == cube.ModeBoost {
		return fail(http.StatusBadRequest, "boost is started on the device, not over the air")
	}
	var until cube.Until
	if mode == cube.ModeVacation {
		if err := until.UnmarshalText([]byte(req.Until)); err != nil {
			return fail(http.StatusBadRequest, "%v", err)
		}
	}
	m, err := cube.CmdSetTemperature(t, req.Temp, mode, until)
	if err != nil {
		return fail(http.StatusBadRequest, "%v", err)
	}
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Minute)
	defer cancel()
	res := rt.queue.Enqueue(engine.Command{Key: "setpoint:" + key, Priority: engine.PriorityHigh, Target: target, Lines: []cube.Message{m}}).Wait(ctx)
	if res.Err != nil {
		status := http.StatusBadGateway
		if errors.Is(res.Err, engine.ErrSuperseded) {
			status = http.StatusConflict
		}
		if errors.Is(res.Err, engine.ErrDiscarded) && t.Room {
			if silent := neverReported(rt.client.Snapshot(), t.RoomID, rt.hidden()); len(silent) > 0 {
				he := fail(status, "%v: the cube holds no state for room %d members %s (not reported since the cube's last restart); the cube refuses room commands until they report or are removed",
					res.Err, t.RoomID, strings.Join(silent, ", ")).(*httpError)
				he.noState = silent
				return he
			}
		}
		return fail(status, "%v", res.Err)
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "discarded": res.Discarded, "budget": res.Budget})
	return nil
}

// neverReported lists the devices the Cube places in a room that have
// never sent a state. Observed on hardware: the Cube discards room
// commands while such a member exists.
func neverReported(snap *cubelan.Snapshot, room uint8, ignore backup.Ignored) []string {
	var out []string
	for _, rf := range snap.Devices() {
		c := snap.Config(rf)
		if c == nil || c.Type == cube.DeviceCube || c.RoomID != room || ignore[rf] {
			continue
		}
		if st, ok := snap.State(rf); !ok || !st.Valid {
			out = append(out, rf.String())
		}
	}
	return out
}

func (s *Server) getBackup(w http.ResponseWriter, r *http.Request) error {
	rt, err := s.cube(r)
	if err != nil {
		return err
	}
	var ntp []string
	if reply, err := rt.client.Send(r.Context(), cube.CmdGetNTP); err == nil {
		ntp = cube.ParseNTPServers(reply.Data)
	}
	f, err := backup.Take(rt.client.Snapshot(), ntp, s.opts.Version, time.Now(), rt.hidden())
	if err != nil {
		return fail(http.StatusServiceUnavailable, "%v", err)
	}
	yaml := r.URL.Query().Get("format") == "yaml"
	data, err := f.Marshal(yaml)
	if err != nil {
		return err
	}
	if yaml {
		w.Header().Set("Content-Type", "application/yaml")
		w.Header().Set("Content-Disposition", "attachment; filename="+f.DefaultName("yaml"))
	} else {
		w.Header().Set("Content-Type", "application/json")
	}
	w.Write(data)
	return nil
}

func (s *Server) events(w http.ResponseWriter, r *http.Request) error {
	rt, err := s.cube(r)
	if err != nil {
		return err
	}
	fl, ok := w.(http.Flusher)
	if !ok {
		return fail(http.StatusNotImplemented, "streaming not supported")
	}
	ch := make(chan event, 64)
	rt.subsMu.Lock()
	rt.subs[ch] = struct{}{}
	rt.subsMu.Unlock()
	defer func() {
		rt.subsMu.Lock()
		delete(rt.subs, ch)
		rt.subsMu.Unlock()
	}()
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.WriteHeader(http.StatusOK)
	// Current state first, so a client needs no separate fetch.
	for _, d := range s.devices(rt) {
		writeEvent(w, event{Kind: "state", Device: d.RFAddress, State: d, Time: time.Now()})
	}
	fl.Flush()
	keep := time.NewTicker(30 * time.Second)
	defer keep.Stop()
	for {
		select {
		case <-r.Context().Done():
			return nil
		case ev := <-ch:
			writeEvent(w, ev)
			fl.Flush()
		case <-keep.C:
			fmt.Fprint(w, ": keep-alive\n\n")
			fl.Flush()
		}
	}
}

func writeEvent(w http.ResponseWriter, ev event) {
	data, _ := json.Marshal(ev)
	fmt.Fprintf(w, "event: %s\ndata: %s\n\n", ev.Kind, data)
}
