// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

// Package server keeps every configured Cube connected, runs a command
// queue per Cube and exposes state and commands over a JSON HTTP API.
//
// Routes (all JSON unless noted):
//
//	GET  /healthz
//	GET  /api/v1/cubes                               every Cube with connection and budget, and the ones found but not configured
//	POST /api/v1/cubes                               serve a Cube from now on, {"serial":"KEQ0000000"} (cubes_api.go)
//	POST /api/v1/cubes/discover                      look for Cubes on the LAN now
//	GET  /api/v1/cubes/{sn}                          hello, budget, rooms, configuration
//	PUT  /api/v1/cubes/{sn}                          name, address, proxy and hidden list, in the store
//	DELETE /api/v1/cubes/{sn}                        stop serving the Cube; its row stays
//	GET  /api/v1/settings                            the server's settings (settings_api.go)
//	PUT  /api/v1/settings                            change them, {"stale_after":"36h"}
//	GET  /api/v1/cubes/{sn}/rooms                    rooms with their devices and states
//	GET  /api/v1/cubes/{sn}/devices                  all devices
//	GET  /api/v1/cubes/{sn}/devices/{rf}             one device: configuration and state
//	PUT  /api/v1/cubes/{sn}/devices/{rf}/setpoint    {"mode":"manual","temp":21.5} | {"mode":"auto"} |
//	                                                 {"mode":"vacation","temp":17,"until":"2026-11-10 12:00"}
//	PUT  /api/v1/cubes/{sn}/rooms/{id}/setpoint      same body, addressed to the whole room
//	GET  /api/v1/cubes/{sn}/backup?format=yaml|json  a backup file
//	POST /api/v1/cubes/{sn}/restore/plan             what restoring the backup in the body would send
//	POST /api/v1/cubes/{sn}/restore?plan=<hash>      send it; progress as restore_step events
//	GET  /api/v1/cubes/{sn}/restore                  how the last restore went
//	GET  /api/v1/cubes/{sn}/events                   server-sent events: connection and state changes
//	POST /api/v1/cubes/{sn}/events/ticket            a single-use ticket that opens the event stream
//	POST /api/v1/cubes/{sn}/pairing                  start pairing (inclusion mode), {"seconds":60}
//	GET  /api/v1/cubes/{sn}/pairing                  pairing state and the devices that joined
//	DELETE /api/v1/cubes/{sn}/pairing                stop pairing
//	GET  /api/v1/update                              newest signed release on the update channel
//	POST /api/v1/update                              ask the update helper to install it (needs a token)
//
// With a token configured, every /api request needs "Authorization: Bearer <token>";
// the event stream also opens with ?ticket=<ticket> from POST …/events/ticket and,
// deprecated, ?token=<token>. Repeated failed token checks lock the client's address
// out for a while (auth.go). A web UI for all of this is served at / (see ui.go).
//
// With Options.TLS set the server terminates TLS itself (the certificate
// from internal/certs, the listener wrapped by cmd/massimo), /healthz
// reports the certificate, and every response carries
// Strict-Transport-Security.
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
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"go.metaworx.ch/massimo/internal/backup"
	"go.metaworx.ch/massimo/internal/certs"
	"go.metaworx.ch/massimo/internal/cubeio"
	"go.metaworx.ch/massimo/internal/cubelan"
	"go.metaworx.ch/massimo/internal/engine"
	"go.metaworx.ch/massimo/internal/protocol/cube"
	"go.metaworx.ch/massimo/internal/proxy"
	"go.metaworx.ch/massimo/internal/store"
	"go.metaworx.ch/massimo/internal/view"
)

// CubeConfig names one Cube to serve.
type CubeConfig struct {
	Serial string // for the URL and the logs; "" takes the one the Cube reports
	Addr   string // host[:port]
	Name   string // a user-friendly name shown next to the serial, optional
	// Hidden hides devices (and the rooms that hold only them) from every
	// view and keeps every write away from them, in addition to those of
	// cube.HiddenRoom; see view.Hidden. From the store's cubes.hidden,
	// seeded from MASSIMO_CUBE_<SN>_IGNORE.
	Hidden []cube.RFAddress
	// FollowDiscovery moves the connection to the address the Cube
	// answers discovery from, also when Addr is set; a Cube without Addr
	// always follows discovery.
	FollowDiscovery bool
	// Source is where the configuration came from, as the store's
	// cubes.source ("env", "ui", "flag"); "" without a store.
	Source string
	// ProxyAddress and ProxySerial are the store's proxy values, shown in
	// the API; the listeners themselves are Options.Proxy.
	ProxyAddress string
	ProxySerial  string
	// SeedFromFile adds the Cube to the store, as seeded from the
	// settings file, once it reports its serial: the plain
	// MASSIMO_CUBE_ADDRESS without a serial.
	SeedFromFile bool
}

// Options configure a Server.
type Options struct {
	Cubes []CubeConfig
	Token string // API token; empty disables authentication
	// AuthFailures failed token checks from one address within
	// AuthWindow lock it out for AuthLockout (answered 429); zero values
	// take DefaultAuthFailures, DefaultAuthWindow and DefaultAuthLockout.
	AuthFailures int
	AuthWindow   time.Duration
	AuthLockout  time.Duration
	PollInterval time.Duration // L polling; 0 keeps the driver default of one minute
	// RefreshInterval is how often each Cube's snapshot is refreshed in
	// full by reconnecting, so that a write the Cube acknowledged but did
	// not apply does not leave it wrong until the next restart; 0 never.
	RefreshInterval time.Duration
	// FollowUpDelays are the pauses between the re-reads of a device's
	// C line after a configuration write, FollowUpReconnectGap the least
	// time between two reconnects for the budget after writes (only with
	// RefreshInterval set); see followup.go. Zero values take the
	// defaults.
	FollowUpDelays       []time.Duration
	FollowUpReconnectGap time.Duration
	Logger               *slog.Logger
	Version              string
	Queue                engine.Options
	// Proxy lists the LAN listeners on which the server itself answers
	// as a Cube (see internal/proxy), one per Cube.
	Proxy []ProxyListener
	// ShowHidden shows the hidden devices (those of cube.HiddenRoom and
	// of the Hidden lists) everywhere instead of hiding them.
	ShowHidden bool
	// Update configures the update endpoints (update_api.go).
	Update UpdateOptions
	// DiscoverInterval is how often the server looks for Cubes on the LAN
	// (discovery.go), with a first run at the start; 0 runs it only on
	// request, and while a Cube configured by serial has no address.
	DiscoverInterval time.Duration
	// DiscoveryWait is how long a run waits for answers (default
	// DefaultDiscoveryWait). DiscoveryAddr, the local address of the
	// discovery socket (default ":23272"), and DiscoveryTargets, where
	// requests go as host:port (default the broadcast addresses), exist
	// for tests.
	DiscoveryWait    time.Duration
	DiscoveryAddr    string
	DiscoveryTargets []string
	// CubePort is the TCP port of a Cube whose address comes from
	// discovery (default 62910), for tests.
	CubePort int
	// Store is the persistent store (internal/store): the found Cubes are
	// kept there, and /healthz names its node. nil keeps everything in
	// memory, as before the store.
	Store *store.Store
	// FileCubes is what the settings file says about each Cube, by
	// serial in upper case: a line that differs from the store is shown
	// on the Cube in GET /api/v1/cubes (store.Differences).
	FileCubes map[string]store.FileCube
	// SettingsFile names the settings file, for the log.
	SettingsFile string
	// ProxyFixed says the proxy listeners came from the command line
	// (-proxy): a proxy address changed through the API is stored, and
	// takes effect at the next start without -proxy.
	ProxyFixed bool
	// StaleAfter is the stale threshold of the device views (0:
	// view.StaleAfter); SetStaleAfter changes it while the server runs.
	StaleAfter time.Duration
	// TLS, when set, is the certificate source of a server that
	// terminates TLS itself (cmd/massimo wraps the listener with
	// certs.Config): /healthz then reports the certificate as "tls", and
	// every response carries Strict-Transport-Security for HSTSDays
	// days, max-age=0 with zero, which clears what a browser remembered.
	// nil is plain HTTP, with no HSTS header: behind a reverse proxy the
	// proxy terminates TLS and sets it.
	TLS      certs.Source
	HSTSDays int
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
	opts    Options
	log     *slog.Logger
	started time.Time // when the server was created, for /healthz
	mu      sync.RWMutex
	cubes   map[string]*runtime // by serial, lower case
	order   []string
	mux     *http.ServeMux
	handler http.Handler // the mux, behind the HSTS header with TLS

	limiter *authLimiter
	tickets *ticketStore
	updates updateCheck

	// Discovery, see discovery.go.
	responder *proxy.Responder // the proxies' discovery socket, when serving
	discMu    sync.Mutex
	discRun   *discoveryRun   // the run going on, if any
	ctx       context.Context // Run's, for work that outlives a request
	foundMu   sync.Mutex
	found     map[string]*foundCube // by serial, upper case

	// The runtimes' and the proxies' goroutines, see runtimes.go.
	wg       sync.WaitGroup
	cubeMu   sync.Mutex    // serialises adding, changing and removing Cubes (cubes_api.go)
	discWake chan struct{} // wakeDiscovery
	proxyMu  sync.Mutex
	proxies  []*proxyRun
	// staleAfter is the stale threshold in effect, in nanoseconds.
	staleAfter atomic.Int64

	// fileCubes is Options.FileCubes, by serial in upper case; "" is the
	// plain MASSIMO_CUBE_ADDRESS Cube until it reports its serial.
	fileMu    sync.Mutex
	fileCubes map[string]store.FileCube
	// diffs are the file's differing lines per serial (noteRow).
	diffs map[string][]fileDifference
}

type runtime struct {
	cfgMu sync.RWMutex
	cfg   CubeConfig // through config and setConfig: the API changes it
	// runMu guards cancel and done, which stop the runtime's goroutines
	// (runtimes.go).
	runMu      sync.Mutex
	cancel     context.CancelFunc
	done       chan struct{}
	client     *cubelan.Client
	queue      *engine.Queue
	serial     string
	showHidden bool
	follow     *followUp // re-reads after writes, see followup.go

	// last is each device's state as the last state event had it, for
	// the changes of the next one; only the pump uses it.
	last map[cube.RFAddress]*view.State

	subs   map[chan event]struct{}
	subsMu sync.Mutex
	push   map[chan struct{}]struct{} // proxy clients waiting for L pushes
	logMu  sync.Mutex
	events []event // the last logSize events, for GET …/log

	// restore is the Cube's last restore, running or over, see
	// restore_api.go.
	restoreMu sync.Mutex
	restore   *restoreRun
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
	// Changes says, for a "state" event, which values differ from the
	// device's previous state event and how; absent on the first one.
	Changes map[string]view.Change `json:"changes,omitempty"`
	// Paired is the device that joined, for a "paired" event.
	Paired *cube.NewDevice `json:"paired,omitempty"`
	// Cube is the Cube discovery found, for a "cube_found" event.
	Cube *foundCube `json:"cube,omitempty"`
	// Restore is the progress of a restore, for the "restore_step" and
	// "restore_done" events.
	Restore *restoreProgress `json:"restore,omitempty"`
	Time    time.Time        `json:"time"`
}

// logSize is how many events a Cube's log keeps for GET …/log.
const logSize = 500

// remember appends an event to the runtime's log ring.
func (rt *runtime) remember(ev event) {
	rt.logMu.Lock()
	defer rt.logMu.Unlock()
	rt.events = append(rt.events, ev)
	if len(rt.events) > logSize {
		rt.events = rt.events[len(rt.events)-logSize:]
	}
}

// emit sends an event to the Cube's stream subscribers and remembers it
// for the log.
func (rt *runtime) emit(ev event) {
	rt.remember(ev)
	rt.subsMu.Lock()
	for ch := range rt.subs {
		select {
		case ch <- ev:
		default: // slow subscriber: drop
		}
	}
	rt.subsMu.Unlock()
}

// recent returns the last n events, oldest first.
func (rt *runtime) recent(n int) []event {
	rt.logMu.Lock()
	defer rt.logMu.Unlock()
	if n <= 0 || n > len(rt.events) {
		n = len(rt.events)
	}
	return append([]event(nil), rt.events[len(rt.events)-n:]...)
}

// New creates a server; Run starts the Cube connections.
func New(opts Options) *Server {
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}
	if opts.PollInterval == 0 {
		opts.PollInterval = time.Minute
	}
	s := &Server{opts: opts, log: opts.Logger, started: time.Now(), cubes: map[string]*runtime{},
		limiter: newAuthLimiter(opts.AuthFailures, opts.AuthWindow, opts.AuthLockout, nil), tickets: newTicketStore(nil),
		found: map[string]*foundCube{}, discWake: make(chan struct{}, 1)}
	s.SetStaleAfter(opts.StaleAfter)
	s.loadFound()
	for _, c := range opts.Cubes {
		if err := s.register(s.newRuntime(c, c.Addr)); err != nil {
			s.log.Error("cube not served", "serial", c.Serial, "addr", c.Addr, "error", err)
		}
	}
	s.routes()
	return s
}

// Run connects every Cube and blocks until ctx ends. Cubes added
// through the API while it runs are started the same way (runtimes.go).
func (s *Server) Run(ctx context.Context) error {
	s.mu.Lock()
	s.ctx = ctx
	s.mu.Unlock()
	for _, rt := range s.runtimes() {
		s.startRuntime(rt)
	}
	for _, pl := range s.opts.Proxy {
		if pl.Discovery {
			s.ensureResponder()
			break
		}
	}
	for _, pl := range s.opts.Proxy {
		s.startProxy(pl)
	}
	s.wg.Add(1)
	go func() { defer s.wg.Done(); s.discoveryLoop(ctx) }()
	<-ctx.Done()
	// Once ctx is done nothing new is tracked (track); taking the lock
	// waits for a track that saw ctx alive, so Wait cannot race an Add.
	s.mu.Lock()
	s.mu.Unlock()
	s.wg.Wait()
	return ctx.Err()
}

// refresh reconnects to the Cube every RefreshInterval, through the
// queue at low priority so that no command is cut off and user commands
// go first. The new greeting replaces the snapshot.
func (s *Server) refresh(ctx context.Context, rt *runtime) {
	t := time.NewTicker(s.opts.RefreshInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
		res := rt.queue.Enqueue(engine.Command{Key: "refresh", Priority: engine.PriorityLow, Reconnect: true}).Wait(ctx)
		if res.Err != nil && ctx.Err() == nil {
			s.log.Warn("periodic refresh failed", "cube", rt.serial, "error", res.Err)
			continue
		}
		s.log.Info("snapshot refreshed", "cube", rt.serial)
	}
}

// writer sends commands to the runtime's Cube and keeps its snapshot in
// step.
func (rt *runtime) writer() cubeio.Writer { return cubeio.Writer{Client: rt.client, Queue: rt.queue} }

// hidden is the set of devices left out of every view: the configured
// hidden list plus the devices the Cube places in room 255.
func (rt *runtime) hidden() view.Hidden {
	return view.NewHidden(rt.config().Hidden, rt.client.Snapshot().Metadata(), rt.showHidden)
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
					s.seedLearned(ctx, rt)
				}
			}
			if e.Device != 0 && rt.hidden()[e.Device] {
				continue
			}
			ev := event{Kind: e.Kind.String(), Time: time.Now()}
			if e.Device != 0 {
				ev.Device = e.Device.String()
				if _, ok := rt.client.Snapshot().State(e.Device); ok {
					dv := view.DeviceOf(rt.client.Snapshot(), e.Device, s.StaleAfter())
					ev.State = dv
					if e.Kind == cubelan.EventState {
						if prev, had := rt.last[e.Device]; had {
							ev.Changes = view.Changes(prev, dv.State)
						}
						rt.last[e.Device] = dv.State
						attrs := stateAttrs(rt.serial, dv)
						if len(ev.Changes) > 0 {
							attrs = append(attrs, "changed", changedNames(ev.Changes))
						}
						s.log.Info("device state", attrs...)
					}
				}
			}
			if e.Err != nil {
				ev.Error = e.Err.Error()
			}
			if e.Kind == cubelan.EventPaired {
				// The metadata does not change by pairing; the device's C
				// line puts it into the device list, as unlisted.
				ev.Paired = e.New
				go rt.writer().Reread(ctx, e.Device)
			}
			rt.remember(ev)
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

// changedNames lists the names of the changed values, sorted.
func changedNames(c map[string]view.Change) string {
	names := make([]string, 0, len(c))
	for k := range c {
		names = append(names, k)
	}
	sort.Strings(names)
	return strings.Join(names, ",")
}

// stateAttrs builds the log attributes of a device state change: the
// device, then whatever the device reported.
func stateAttrs(serial string, d view.Device) []any {
	attrs := []any{"cube", serial, "device", d.RFAddress, "name", d.Name}
	if d.State == nil {
		return append(attrs, "reported", false)
	}
	st := d.State
	if d.Type.IsRadiatorThermostat() || d.Type == cube.DeviceWallThermostat {
		attrs = append(attrs, "mode", st.Mode.String())
	}
	if st.Setpoint != nil {
		attrs = append(attrs, "setpoint", *st.Setpoint)
	}
	if st.ActualTemp != nil {
		attrs = append(attrs, "actual", *st.ActualTemp)
	}
	if st.ValvePct != nil {
		attrs = append(attrs, "valve_pct", *st.ValvePct)
	}
	if st.WindowOpen != nil {
		attrs = append(attrs, "window_open", *st.WindowOpen)
	}
	if st.Until != nil {
		attrs = append(attrs, "until", st.Until.String())
	}
	if st.BatteryLow {
		attrs = append(attrs, "battery_low", true)
	}
	if st.Error {
		attrs = append(attrs, "error", true)
	}
	if st.LinkError {
		attrs = append(attrs, "link_error", true)
	}
	if st.PanelLocked {
		attrs = append(attrs, "locked", true)
	}
	return attrs
}

// SetStaleAfter sets the stale threshold of the device views; 0 or less
// goes back to view.StaleAfter.
func (s *Server) SetStaleAfter(d time.Duration) {
	if d <= 0 {
		d = view.StaleAfter
	}
	s.staleAfter.Store(int64(d))
}

// StaleAfter returns the stale threshold in effect.
func (s *Server) StaleAfter() time.Duration { return time.Duration(s.staleAfter.Load()) }

// Handler returns the HTTP handler.
func (s *Server) Handler() http.Handler { return s.handler }

func (s *Server) routes() {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		h := map[string]any{"ok": true, "version": s.opts.Version, "started_at": s.started}
		if s.opts.Store != nil {
			h["node_id"] = s.opts.Store.NodeID()
		}
		if s.opts.TLS != nil {
			h["tls"] = s.opts.TLS.Status()
		}
		writeJSON(w, http.StatusOK, h)
	})
	api := func(pattern string, h func(http.ResponseWriter, *http.Request) error) {
		mux.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) {
			if !s.authorize(w, r) {
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
				// Every handler error should come through fail; one that
				// does not is a bug, logged so that it gets noticed.
				s.log.Error("api: error not mapped to a status", "method", r.Method, "path", r.URL.Path, "error", err)
				writeError(w, http.StatusInternalServerError, err.Error())
			}
		})
	}
	api("GET /api/v1/cubes", s.listCubes)
	api("POST /api/v1/cubes", s.postCube)
	api("PUT /api/v1/cubes/{sn}", s.putCube)
	api("DELETE /api/v1/cubes/{sn}", s.deleteCube)
	api("GET /api/v1/settings", s.getSettings)
	api("PUT /api/v1/settings", s.putSettings)
	api("POST /api/v1/cubes/discover", s.postDiscover)
	api("GET /api/v1/cubes/{sn}", s.getCube)
	api("GET /api/v1/cubes/{sn}/rooms", s.getRooms)
	api("GET /api/v1/cubes/{sn}/devices", s.getDevices)
	api("GET /api/v1/cubes/{sn}/devices/{rf}", s.getDevice)
	api("PUT /api/v1/cubes/{sn}/devices/{rf}/setpoint", s.setDeviceSetpoint)
	api("PUT /api/v1/cubes/{sn}/rooms/{id}/setpoint", s.setRoomSetpoint)
	api("DELETE /api/v1/cubes/{sn}/rooms/{id}", s.deleteRoom)
	api("PUT /api/v1/cubes/{sn}/devices/{rf}/lock", s.setDeviceLock)
	api("GET /api/v1/cubes/{sn}/backup", s.getBackup)
	api("POST /api/v1/cubes/{sn}/restore/plan", s.planRestore)
	api("POST /api/v1/cubes/{sn}/restore", s.startRestore)
	api("GET /api/v1/cubes/{sn}/restore", s.getRestore)
	api("GET /api/v1/cubes/{sn}/events", s.events)
	api("POST /api/v1/cubes/{sn}/events/ticket", s.issueTicket)
	api("GET /api/v1/cubes/{sn}/log", s.getLog)
	api("POST /api/v1/cubes/{sn}/pairing", s.startPairing)
	api("GET /api/v1/cubes/{sn}/pairing", s.getPairing)
	api("DELETE /api/v1/cubes/{sn}/pairing", s.stopPairing)
	api("GET /api/v1/update", s.getUpdate)
	api("POST /api/v1/update", s.startUpdate)
	s.configRoutes(api)
	s.uiRoutes(mux)
	s.mux = mux
	s.handler = mux
	if s.opts.TLS != nil {
		// HSTS on every response, the API's included: a browser that saw
		// it refuses http:// to this host for that long, so it goes out
		// only from a server that terminates TLS itself, and max-age=0
		// (-hsts 0) is how a box going back to plain HTTP clears it.
		hsts := "max-age=" + strconv.Itoa(max(s.opts.HSTSDays, 0)*24*60*60)
		s.handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Strict-Transport-Security", hsts)
			mux.ServeHTTP(w, r)
		})
	}
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
	Serial  string `json:"serial"`
	Name    string `json:"name,omitempty"`
	Address string `json:"address"`
	// Configured is true for the Cubes the server serves; GET /cubes
	// also lists Cubes discovery found that are not (foundCube).
	Configured bool `json:"configured"`
	// Source is where the Cube's configuration comes from: "env" (the
	// settings file, seeded into the store), "ui" (the API), "flag"
	// (-addr, for this run); absent without a store.
	Source string `json:"source,omitempty"`
	// FileDiffers lists the settings file's lines for this Cube whose
	// values differ from the store's, which wins.
	FileDiffers []fileDifference `json:"file_differs,omitempty"`
	// The configuration PUT /api/v1/cubes/{sn} changes: whether the
	// address follows discovery, the proxy listener's address and the
	// serial of its discovery replies, and the hidden radio addresses.
	ConfiguredAddress string   `json:"configured_address"` // "" follows discovery; Address is where it connects now
	FollowDiscovery   bool     `json:"follow_discovery"`
	ProxyAddress      string   `json:"proxy_address,omitempty"`
	ProxySerial       string   `json:"proxy_serial,omitempty"`
	Hidden            []string `json:"hidden"`
	// ProxyListening lists the addresses the Cube's proxy listens on now.
	ProxyListening []string `json:"proxy_listening,omitempty"`
	// MAC and LastSeen come from discovery: the Cube's MAC address and
	// when it last answered.
	MAC       string         `json:"mac,omitempty"`
	LastSeen  *time.Time     `json:"last_seen,omitempty"`
	Connected bool           `json:"connected"`
	Firmware  string         `json:"firmware,omitempty"`
	RFAddress string         `json:"rf_address,omitempty"`
	Budget    cubelan.Budget `json:"budget"`
	// Pending counts radiograms the Cube sent and no device has
	// acknowledged yet (memory slots in use); their effect is not in
	// the device states yet.
	Pending int       `json:"pending_radiograms"`
	Updated time.Time `json:"updated,omitempty"`
	// ObservedSince is when the server first heard from the Cube; a
	// device value received at that time was already there and may be
	// older.
	ObservedSince time.Time        `json:"observed_since,omitempty"`
	Rooms         int              `json:"rooms"`
	Devices       int              `json:"devices"`
	Queued        int              `json:"queued"`
	Config        *cube.CubeConfig `json:"config,omitempty"`
	RoomList      []view.Room      `json:"room_list,omitempty"`
	CubeTime      string           `json:"cube_time,omitempty"`
}

type deviceDetail struct {
	view.Device
	Firmware   uint8                  `json:"firmware,omitempty"`
	Thermostat *cube.ThermostatConfig `json:"thermostat,omitempty"`
	Wall       *cube.WallConfig       `json:"wall_thermostat,omitempty"`
}

func (s *Server) summary(rt *runtime) cubeView {
	snap := rt.client.Snapshot()
	b := rt.client.Budget()
	cfg := rt.config()
	v := cubeView{Serial: rt.serial, Name: cfg.Name, Address: rt.client.Address(), Configured: true, Source: cfg.Source, ConfiguredAddress: cfg.Addr, FollowDiscovery: cfg.Addr == "" || cfg.FollowDiscovery,
		ProxyAddress: s.proxyAddress(rt), ProxySerial: cfg.ProxySerial, Hidden: hiddenStrings(cfg.Hidden), Budget: b, Pending: view.Pending(snap, b), Queued: rt.queue.Len()}
	v.FileDiffers = s.fileDiffers(rt.serial)
	v.ProxyListening = s.proxyListening(rt)
	if f, ok := s.foundInfo(rt.serial); ok {
		v.MAC, v.LastSeen = f.MAC, &f.LastSeen
	}
	if h := snap.Hello(); h != nil {
		v.Connected = rt.client.Connected()
		v.Firmware, v.RFAddress, v.Updated, v.ObservedSince = h.FirmwareVersion, h.RFAddress.String(), snap.Updated(), snap.Since()
		v.CubeTime = strings.TrimSpace(h.CubeDate + " " + h.CubeTime)
		if c := snap.Config(h.RFAddress); c != nil {
			v.Config = c.Cube
		}
	}
	v.Rooms, v.Devices = view.Counts(snap, rt.hidden())
	return v
}

// Handlers.

// getLog returns the last events of a Cube (the same objects the event
// stream sends), oldest first; ?limit=N caps the number (default 200).
func (s *Server) getLog(w http.ResponseWriter, r *http.Request) error {
	rt, err := s.cube(r)
	if err != nil {
		return err
	}
	n := 200
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err = strconv.Atoi(v); err != nil || n < 1 {
			return fail(http.StatusBadRequest, "bad limit %q", v)
		}
	}
	evs := rt.recent(n)
	if evs == nil {
		evs = []event{}
	}
	writeJSON(w, http.StatusOK, evs)
	return nil
}

// listCubes handles GET /cubes: the configured Cubes, then the ones
// discovery found that are not configured.
func (s *Server) listCubes(w http.ResponseWriter, r *http.Request) error {
	out := []any{}
	for _, rt := range s.runtimes() {
		out = append(out, s.summary(rt))
	}
	for _, f := range s.unconfigured() {
		out = append(out, f)
	}
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

// rooms is the room list of a Cube as the API shows it.
func (s *Server) rooms(rt *runtime) []view.Room {
	return view.Rooms(rt.client.Snapshot(), rt.hidden(), s.StaleAfter())
}

// devices is the device list of a Cube as the API shows it.
func (s *Server) devices(rt *runtime) []view.Device {
	return view.Devices(rt.client.Snapshot(), rt.hidden(), s.StaleAfter())
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
	d := deviceDetail{Device: view.DeviceOf(snap, rf, s.StaleAfter())}
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
	rt.afterWrite()
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
	if _, ok := rt.hidden().Room(md, uint8(id)); !ok {
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
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Minute)
	defer cancel()
	if err := s.writeMetadata(ctx, rt, &cp); err != nil {
		return err
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
	room, ok := rt.hidden().Room(md, uint8(id))
	if !ok {
		return fail(http.StatusNotFound, "no room %d", id)
	}
	return s.setpoint(w, r, rt, cube.Target{RFAddress: room.RFAddress, RoomID: room.ID, Room: true}, 0, fmt.Sprintf("room:%d", id))
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
	var roomCmd uint8
	if t.Room {
		roomCmd = t.RoomID
	}
	if err := s.commandError(rt, res, roomCmd); err != nil {
		return err
	}
	rt.afterWrite()
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "discarded": res.Discarded, "budget": res.Budget})
	return nil
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
		return fail(http.StatusInternalServerError, "backup: %v", err)
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
