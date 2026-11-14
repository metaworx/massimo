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
//	GET  /api/v1/cubes/{sn}/backups                  the stored backups (backups_api.go)
//	POST /api/v1/cubes/{sn}/backups                  take one now
//	GET  /api/v1/cubes/{sn}/backups/{id}             a stored backup's file
//	DELETE /api/v1/cubes/{sn}/backups/{id}           remove one
//	GET  /api/v1/cubes/{sn}/backups/{id}/diff        what would turn another into it
//	POST /api/v1/cubes/{sn}/restore/plan             what restoring the backup in the body (or backup=<id>) would send
//	POST /api/v1/cubes/{sn}/restore?plan=<hash>      send it; progress as restore_step events
//	GET  /api/v1/cubes/{sn}/restore                  how the last restore went
//	GET  /api/v1/cubes/{sn}/events                   server-sent events: connection and state changes
//	POST /api/v1/cubes/{sn}/events/ticket            a single-use ticket that opens the event stream
//	POST /api/v1/cubes/{sn}/pairing                  start pairing (inclusion mode), {"seconds":60}
//	GET  /api/v1/cubes/{sn}/pairing                  pairing state and the devices that joined
//	DELETE /api/v1/cubes/{sn}/pairing                stop pairing
//	POST /api/v1/cubes/{sn}/reboot                   restart the Cube with a UDP R (cube_reboot.go)
//	GET  /api/v1/system                              how the server is installed, the computer, the caller's address (system_api.go)
//	GET  /api/v1/update                              newest signed release on the update channel
//	POST /api/v1/update                              ask the update helper to install it (needs a token)
//	GET  /api/v1/restart                             whether the server can be restarted from here (restart_api.go)
//	POST /api/v1/restart                             restart it through its service manager
//	POST /api/v1/reload                              read the settings file again at once, as SIGHUP does
//	GET  /api/v1/reboot                              whether the server can reboot its computer from here (reboot_api.go)
//	POST /api/v1/reboot                              reboot it after a delay, through its root helper
//	DELETE /api/v1/reboot                            take back a reboot that waits
//	GET  /api/v1/store                               the store's file, schema and rows, as store info shows (serverinfo_api.go)
//	GET  /api/v1/config                              the settings in effect, as massimo config shows, secrets as set
//	POST /api/v1/login                               a session for a name and a password (login_api.go)
//	POST /api/v1/login/start, /login/finish          a session by OPAQUE, no password sent (opaque_api.go)
//	POST /api/v1/logout                              end the session
//	GET  /api/v1/me                                  who the caller is and what the caller may do
//	GET|POST /api/v1/users, /users/{name}, /users/{name}/tokens    the users and their tokens (users_api.go)
//	POST /api/v1/users/{name}/register               a registration by OPAQUE, its first step (opaque_api.go)
//	GET|POST /api/v1/roles, /roles/{key}, /roles/{key}/disabled   the roles and their sets (roles_api.go)
//	GET|PUT /api/v1/capabilities, /capabilities/{key}              the capabilities, disabled or not
//	GET  /api/v1/audit                               every request that changed something (audit.go)
//	GET  /api/v1/commands/{id}                       a write request with its steps on the queue (commands.go)
//
// Every route is registered with the capability it needs (auth.go): with users in
// the store, every /api request carries "Authorization: Bearer <token>", a session or
// an API key that names a user whose role grants the capabilities; without a store
// the one token of Options.Token is checked as before. The event stream also opens
// with ?ticket=<ticket> from POST …/events/ticket and, deprecated, ?token=<token>.
// Repeated failed checks lock the client's address out for a while. Every request
// that changes something is an audit row and a command row (audit.go, commands.go).
// A web UI for all of this is served at / (see ui.go).
//
// With Options.TLS set the server terminates TLS itself (the certificate
// from internal/certs, the listener wrapped by internal/serve), /healthz
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
	"net"
	"net/http"
	"net/netip"
	"regexp"
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
	"go.metaworx.ch/massimo/internal/notify"
	"go.metaworx.ch/massimo/internal/protocol/cube"
	"go.metaworx.ch/massimo/internal/proxy"
	"go.metaworx.ch/massimo/internal/push"
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
	// StoreID is the id of the Cube's row in the store, 0 without one:
	// the log of a Cube with a row is kept in the store (log.go).
	StoreID int64
}

// RestartOptions say who starts the server again after a restart from
// the API, and how it stops for one; cmd/massimo finds them.
type RestartOptions struct {
	// By names who starts the server again: "systemd", "launchd",
	// "windows" (the service manager), "exit" (whatever runs it, told by
	// the exit status); "" when nobody would, and Blocked says why.
	By      string
	Blocked string
	// Restart stops the server for the restart, or asks the service
	// manager to; it is called once the answer to the request is written.
	Restart func() error
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
	// CubeHold is how long a Cube's lost connection is held open, so that
	// it resumes when the network is back (cubelan.Options.Hold): 0 is
	// cubelan.DefaultHold, a negative value holds nothing.
	CubeHold time.Duration
	// cubeOptions, set by a test, changes the options of each Cube's
	// client before it is made: a short reply timeout, a quick dialer.
	cubeOptions func(*cubelan.Options)
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
	// One that cannot listen is tried again until it does.
	Proxy []ProxyListener
	// ShowHidden shows the hidden devices (those of cube.HiddenRoom and
	// of the Hidden lists) everywhere instead of hiding them.
	ShowHidden bool
	// Update configures the update endpoints (update_api.go).
	Update UpdateOptions
	// Restart configures the restart from the API.
	Restart RestartOptions
	// Reboot configures the reboot of the computer (reboot_api.go).
	Reboot RebootOptions
	// Install says how the server was installed, for GET /api/v1/system
	// (system_api.go): the package manager that owns the binary ("apt",
	// "brew"), else what runs it (InstallWindowsService, InstallSystemd,
	// InstallLaunchd); empty is InstallOther.
	Install string
	// DiscoverInterval is how often the server looks for Cubes on the LAN
	// (discovery.go), with a first run at the start; 0 runs it only on
	// request, and while a Cube configured by serial has no address or a
	// Cube following discovery is lost. Above 0, a lost Cube with a fixed
	// address is searched for too (searchLost).
	DiscoverInterval time.Duration
	// AddressRetry is how often discovery runs while a Cube needs it
	// (needSearch); 0 is a minute. The tests shorten it.
	AddressRetry time.Duration
	// BackupMinute is the unit of backups.auto_min and backups.auto_max
	// and how often a Cube's configuration is looked at for its
	// automatic backup (backups.go); 0 is a minute. The tests shorten it.
	BackupMinute time.Duration
	// DiscoveryWait is how long a run waits for answers (default
	// DefaultDiscoveryWait). DiscoveryAddr, the local address of the
	// discovery socket (default ":23272"), and DiscoveryTargets, where
	// requests go as host:port (default the broadcast addresses), exist
	// for tests.
	DiscoveryWait time.Duration
	// FindIntegrations asks the LAN for the installations of integrations
	// (integrations_found.go), at the start and every 30 minutes; nil
	// looks for none.
	FindIntegrations func(ctx context.Context) ([]FoundIntegration, error)
	DiscoveryAddr    string
	DiscoveryTargets []string
	// LookupMAC gives the MAC of an address that answered discovery
	// (default cubelan.LookupMAC, the ARP cache), for tests: the loopback
	// has no ARP entries.
	LookupMAC func(net.IP) string
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
	// notifyClock is the notification rules' clock, for the tests; nil
	// is the real one.
	notifyClock notify.Clock
	// Deliverers take a user's notifications beside Web Push: an app
	// embedding the server shows them itself (push.Local).
	Deliverers []push.Deliverer
	// pushClient and pushRetry are Web Push's client and waits, for the
	// tests; nil for push.Sender's defaults.
	pushClient *http.Client
	pushRetry  []time.Duration
	// proxyRetry is the first and the longest wait between a proxy
	// listener's tries, for the tests; zero for 1 s and 30 s.
	proxyRetry [2]time.Duration
	// proxyListen opens a proxy listener in place of net.Listen, for
	// the tests.
	proxyListen func(addr string) (net.Listener, error)
	// TLS, when set, is the certificate source of a server that
	// terminates TLS itself (internal/serve wraps the listener with
	// certs.Config): /healthz then reports the certificate as "tls", and
	// every response carries Strict-Transport-Security for HSTSDays
	// days, max-age=0 with zero, which clears what a browser remembered.
	// nil is plain HTTP, with no HSTS header: behind a reverse proxy the
	// proxy terminates TLS and sets it.
	TLS      certs.Source
	HSTSDays int
	// TrustedProxies are the reverse proxies whose X-Forwarded-For and
	// X-Real-IP name the client (clientIP); empty, the headers are
	// ignored.
	TrustedProxies []netip.Prefix
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
	stopWhy atomic.Value // string: why the server will stop (noteStop)
	started time.Time    // when the server was created, for /healthz
	// notify runs the notification rules (push_api.go), nil without a
	// store; vapidMu makes the VAPID key once; webPush delivers what the
	// rules decide, and pushSlots bound the deliveries under way.
	notify    *notify.Sink
	vapidMu   sync.Mutex
	webPush   *push.WebPush
	pushSlots chan struct{}
	mu        sync.RWMutex
	cubes     map[string]*runtime // by serial, lower case
	order     []string
	mux       *http.ServeMux
	handler   http.Handler // the mux, behind the HSTS header with TLS

	limiter     failureLimiter
	authLockout time.Duration // the lockout in effect, for the log
	signIns     signIns       // the addresses with a login running
	// integrationRequests bounds the registration requests per address
	// (integrations_api.go).
	integrationRequests requestLimit
	integFound          foundIntegrations // integrations_found.go
	logins              opaqueLogins      // the sign-ins by OPAQUE started, and the proofs given
	oidc                oidcFlows         // the sign-ins through a provider started, and their codes (oidc.go)
	tickets             *ticketStore
	refusals            *refusalAudit // nil without a store
	// steps writes the queues' step rows in order (commands.go).
	steps   serialWorker
	updates updateCheck
	// updateMu holds one update request at a time (startUpdate).
	updateMu sync.Mutex
	// restartGate lets one restart through (restart_api.go), and
	// pending lists what applies only at the next one.
	restartGate restartGate
	// reboot is the reboot that waits (reboot_api.go).
	reboot    rebootPlan
	pendingMu sync.Mutex
	pending   []string
	// the settings file, when it was last read, and what reads it again
	// (restart_api.go, SetReloader)
	settingsFile string
	settingsRead time.Time
	reloader     func(context.Context) (ReloadResult, error)
	configView   func() ConfigView // serverinfo_api.go, SetConfigView
	// token is the one token without a store (auth.go, legacyToken).
	tokenMu sync.RWMutex
	token   string
	// streamKeepAlive is the event streams' keep-alive in nanoseconds,
	// at which each also asks whether its caller is still let in.
	streamKeepAlive atomic.Int64

	// apiAddr is where the API listens (SetAPIAddr), under mu, for GET
	// /api/v1/system; proxyCheck the proxy's certificate (proxied.go).
	apiAddr    net.Addr
	proxyCheck proxyCheck

	// Discovery, see discovery.go.
	responder *proxy.Responder // the proxies' discovery socket, when serving
	discMu    sync.Mutex
	discRun   *discoveryRun   // the run going on, if any
	discNext  *discoveryRun   // the run to start when discRun ends, if any
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
	// logRef is what the log compares the Cube's lines with; only the
	// pump touches it (logdiff.go).
	logRef logRef

	cfgMu sync.RWMutex
	cfg   CubeConfig // through config and setConfig: the API changes it
	// storeID is the Cube's row in the store, from CubeConfig.StoreID or
	// learned at its first connection; 0 keeps the log in the ring.
	storeID atomic.Int64
	// store is the server's store, for the log; nil without one.
	store *store.Store
	// runMu guards cancel and done, which stop the runtime's goroutines
	// (runtimes.go).
	runMu      sync.Mutex
	cancel     context.CancelFunc
	done       chan struct{}
	client     *cubelan.Client
	queue      *engine.Queue
	serialMu   sync.Mutex
	serial     string // through cubeSerial and learnSerial: the pump learns it
	showHidden bool
	follow     *followUp // re-reads after writes, see followup.go

	// last is each device's state as the last state event had it, for
	// the changes of the next one; only the pump uses it.
	last map[cube.RFAddress]*view.State
	// vwt is the rooms' uids and settings, and each room's virtual wall
	// thermostat as its last event had it (vwt.go).
	vwt vwtState
	// programs holds the week programmes written and not yet confirmed
	// by the devices' configurations (etag.go).
	programs pendingPrograms
	// proxyVirtual is the Cube's option proxy_virtual: its proxy shows
	// clients the virtual wall thermostats (proxy_virtual.go).
	proxyVirtual atomic.Bool

	subs   map[chan event]*subscriber // the stream subscribers (resume.go)
	subsMu sync.Mutex
	// epoch, seq and replay number the stream's events and keep the last
	// of them for a reconnecting client, under subsMu (resume.go).
	epoch  string
	seq    uint64
	replay []published
	logMu  sync.Mutex
	events []event // the last logSize events, for GET …/log

	// bk is the Cube's configuration against its newest backup, for the
	// automatic backup (backups.go).
	bk backupWatch

	// restore is the Cube's last restore, running or over, see
	// restore_api.go.
	restoreMu sync.Mutex
	restore   *restoreRun

	// structure is the lock of a change of rooms, devices and their
	// metadata, see lockStructure.
	structure chan struct{}

	// evlog takes the events' lines (eventline.go): the server's root
	// logger, each line saying its own source.
	evlog *slog.Logger
	// sentTo names, per device, the command last sent to it, for the
	// device's next state line (proxylog.go).
	sentTo stateCommands
}

// lockStructure takes the Cube's structure lock, so that one change of
// rooms and devices at a time reads the metadata, sends its radiograms
// and writes the metadata: two built from the same reading would lose
// one of them, the more so as the queue lets a metadata write supersede
// one still waiting. It waits as long as ctx lets it and returns the
// function that releases the lock.
func (rt *runtime) lockStructure(ctx context.Context) (func(), error) {
	select {
	case rt.structure <- struct{}{}:
		return func() { <-rt.structure }, nil
	case <-ctx.Done():
		return nil, fail(http.StatusServiceUnavailable, "another change of the cube's rooms and devices is still running")
	}
}

type event struct {
	Kind   string `json:"kind"`
	Device string `json:"device,omitempty"`
	// Serial is the S/N of the device the event concerns, also when the
	// device has no state yet or is gone; for the Cube's own
	// configuration, the Cube's.
	Serial string `json:"serial,omitempty"`
	// Source says whose the event is: "device" for a device's, "cube"
	// for the Cube's own (its configuration, connection, rooms,
	// pairing), "system" for the server's own jobs, "config" for a change
	// of the configuration (settle gives each kind its own).
	Source string `json:"source,omitempty"`
	// Level is how much the event matters: debug, info, warn or error,
	// as the server's text log names them.
	Level string      `json:"level,omitempty"`
	Error string      `json:"error,omitempty"`
	State interface{} `json:"state,omitempty"`
	// Changes says, for a "state" event, which values differ from the
	// device's previous state event and how; absent on the first one.
	// For a "config" event, which values of the configuration differ
	// from the one held before (logdiff.go).
	Changes map[string]view.Change `json:"changes,omitempty"`
	// ListChanges says, for a "metadata" event, what differs in the room
	// and device list.
	ListChanges []listChange `json:"list_changes,omitempty"`
	// Summary is what a line holds, in short, where it says no
	// differences: a hello's budget, a list's rooms and devices, a
	// configuration's main values, a connection's devices, rooms,
	// firmware, budget, cause and down_for_s.
	Summary map[string]any `json:"summary,omitempty"`
	// Missed marks the differences a greeting found: what changed while
	// the Cube was disconnected, or the server down.
	Missed bool `json:"while_disconnected,omitempty"`
	// Paired is the device that joined, for a "paired" event.
	Paired *cube.NewDevice `json:"paired,omitempty"`
	// Cube is the Cube discovery found, for a "cube_found" event.
	Cube *foundCube `json:"cube,omitempty"`
	// Restore is the progress of a restore, for the "restore_step" and
	// "restore_done" events.
	Restore *restoreProgress `json:"restore,omitempty"`
	// Notification is what a user is told, for a "notification" event,
	// which only that user's streams get (push_api.go).
	Notification *notify.Notification `json:"notification,omitempty"`
	Time         time.Time            `json:"time"`
	// ID is the event's row in the store, for GET …/log's paging; the
	// stream's events and the ring's have none.
	ID int64 `json:"id,omitempty"`
	// By is who started what the event says, as the users table names
	// them now; absent for the Cube, the server's own jobs and a server
	// without users (the bootstrap token is a key of the first owner).
	By *eventUser `json:"by,omitempty"`
	// Changed says, for a "config" event of a thermostat or a virtual
	// wall thermostat, what of its configuration changed: program, temps
	// (the presets, boost and valve settings) or settings (a virtual
	// wall thermostat's own); Etag is the programme's new tag where it
	// changed (etag.go).
	Changed []string `json:"changed,omitempty"`
	Etag    string   `json:"etag,omitempty"`
	// Command is the command the event belongs to (its attempt, a
	// discard and a retry, its outcome), GET /api/v1/commands/{id}; 0
	// for none, and once the command is pruned.
	Command int64 `json:"command,omitempty"`
	// CubeSerial is the S/N of the Cube whose event it is, in a page of
	// every Cube's log (GET /api/v1/log); absent elsewhere, and for the
	// server's own events.
	CubeSerial string `json:"cube_serial,omitempty"`
	// userID is By's row in the store, kept in log.user_id.
	userID int64
	// audit marks an event only a user with audit sees (the log's
	// visibility audit).
	audit bool
	// server marks the server's own event (logServer), no Cube's, which
	// every Cube's ring keeps without a store.
	server bool
	// virtual marks an event of a virtual wall thermostat, wallRoom one
	// whose room has a real wall thermostat, and realWall an event of a
	// real wall thermostat in a room: the stream's view (?devices=) says
	// which of them a subscriber gets (vwt.go).
	virtual, wallRoom, realWall bool
	// seq is the event's number on the Cube's stream, its id there
	// (resume.go); 0 for an event the stream did not publish.
	seq uint64
}

// eventUser names the user an event says started it.
type eventUser struct {
	Name        string `json:"name"`
	DisplayName string `json:"display_name,omitempty"`
}

// settle gives an event the source and the level its kind has where it
// says none: the Cube's for the driver's events of no device and for
// cube_found, the system's for a restore's; warn for a lost connection,
// debug for the H line, which only a greeting brings; info for the rest.
func (ev *event) settle() {
	if ev.Source == "" {
		switch {
		case strings.HasPrefix(ev.Kind, "restore_"):
			ev.Source = "system"
		case ev.Device != "":
			ev.Source = "device"
		default:
			ev.Source = "cube"
		}
	}
	if ev.Level == "" {
		switch ev.Kind {
		case "disconnected":
			ev.Level = "warn"
		case "hello":
			ev.Level = "debug"
		default:
			ev.Level = "info"
		}
	}
}

// logSize is how many events the ring keeps for GET …/log, the log of a
// Cube without a row in the store (log.go).
const logSize = 500

// remember keeps an event for the log: in the store for a Cube with a
// row there, else in the runtime's ring. It never waits for the store.
func (rt *runtime) remember(ev event) {
	if id := rt.storeID.Load(); id != 0 && rt.store != nil {
		rt.store.AddEvent(storedEvent(id, ev))
		return
	}
	rt.logMu.Lock()
	defer rt.logMu.Unlock()
	rt.events = append(rt.events, ev)
	if len(rt.events) > logSize {
		rt.events = rt.events[len(rt.events)-logSize:]
	}
}

// emit sends an event to the Cube's stream subscribers and remembers it
// for the log, settled.
func (rt *runtime) emit(ev event) {
	ev.settle()
	rt.line(ev)
	rt.remember(ev)
	rt.send(ev)
}

// send hands an event to the Cube's stream subscribers.
func (rt *runtime) send(ev event) {
	rt.publish(ev, 0)
}

// sendUser hands an event to the Cube's stream subscribers that are the
// user's: a notification is for its user alone.
func (rt *runtime) sendUser(userID int64, ev event) {
	rt.publish(ev, userID)
}

// logServer keeps an event of the server's, no Cube's: once in the
// store, with no Cube (cube_id NULL), which every Cube's log shows, and
// on every Cube's stream; a Cube without a row in the store keeps it in
// its ring, its log being the ring's.
func (s *Server) logServer(ev event) {
	ev.settle()
	ev.server = true
	s.nameUser(context.Background(), &ev)
	eventLine(s.opts.Logger, "", ev)
	if st := s.opts.Store; st != nil {
		st.AddEvent(storedEvent(0, ev))
	}
	for _, rt := range s.runtimes() {
		if rt.storeID.Load() == 0 || rt.store == nil {
			rt.remember(ev)
		}
		rt.send(ev)
	}
}

// nameUser sets an event's By from its user, as the store names them
// now; a removed user keeps the names it had.
func (s *Server) nameUser(ctx context.Context, ev *event) {
	if ev.userID == 0 || s.opts.Store == nil {
		return
	}
	if u, ok, err := s.opts.Store.UserByID(ctx, ev.userID); err == nil && ok {
		ev.By = &eventUser{Name: u.Name, DisplayName: u.DisplayName}
	}
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

// partLog is the logger of a part the server starts: its source, and the
// Cube it serves where it serves one.
func (s *Server) partLog(source, serial string) *slog.Logger {
	l := s.opts.Logger.With("source", source)
	if serial != "" {
		l = l.With("cube", serial)
	}
	return l
}

// New creates a server; Run starts the Cube connections.
func New(opts Options) *Server {
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}
	if opts.PollInterval == 0 {
		opts.PollInterval = time.Minute
	}
	// The server's own lines are source server; the parts it starts get
	// the logger with their own (cube, queue, proxy, push).
	s := &Server{opts: opts, log: opts.Logger.With("source", "server"), started: time.Now(), cubes: map[string]*runtime{}, token: opts.Token,
		limiter: newLimiter(opts, opts.Logger.With("source", "server")), authLockout: opts.AuthLockout, tickets: newTicketStore(nil),
		found: map[string]*foundCube{}, discWake: make(chan struct{}, 1)}
	if s.authLockout <= 0 {
		s.authLockout = DefaultAuthLockout
	}
	s.streamKeepAlive.Store(int64(30 * time.Second))
	s.pushSlots = make(chan struct{}, maxDeliveries)
	s.reboot.allowed = opts.Reboot.Allowed
	s.SetStaleAfter(opts.StaleAfter)
	s.loadFound()
	if opts.Store != nil {
		s.notify = s.newNotify()
		s.webPush = s.newWebPush()
		s.refusals = newRefusalAudit(s.writeRefusal)
		// MASSIMO_API_TOKEN becomes the owner's key, or follows the file (docs/store.md section 6).
		if _, err := opts.Store.Bootstrap(context.Background(), opts.Token); err != nil {
			s.log.Error("store: bootstrap failed", "error", err)
		}
		if n, err := opts.Store.DropUnfinished(context.Background(), "server restarted"); err != nil {
			s.log.Error("store: unfinished commands not dropped", "error", err)
		} else if n > 0 {
			s.log.Info("unfinished commands dropped", "count", n)
		}
	}
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
	// The log's writer (log.go), before the Cubes connect, on a context
	// of its own: it ends once the server's goroutines have, so that what
	// they emit as they stop is written, and Run waits for it.
	writerDone, stopWriter := make(chan struct{}), func() {}
	if st := s.opts.Store; st != nil {
		wctx, cancel := context.WithCancel(context.WithoutCancel(ctx))
		stopWriter = cancel
		go func() { defer close(writerDone); st.RunEvents(wctx) }()
		s.wg.Add(1)
		go func() { defer s.wg.Done(); s.pruneBackupsDaily(ctx, st) }()
	} else {
		close(writerDone)
	}
	s.logStarted(ctx)
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
	if s.opts.FindIntegrations != nil {
		s.wg.Add(1)
		go func() { defer s.wg.Done(); s.integrationsLoop(ctx) }()
	}
	if s.notify != nil {
		s.wg.Add(1)
		go func() { defer s.wg.Done(); s.notify.Run(ctx) }()
	}
	<-ctx.Done()
	s.logStopping() // before the writer stops, which writes it
	// Once ctx is done nothing new is tracked (track); taking the lock
	// waits for a track that saw ctx alive, so Wait cannot race an Add.
	s.mu.Lock()
	s.mu.Unlock()
	s.wg.Wait()
	s.steps.flush(5 * time.Second) // the drained commands' steps, before the store closes
	s.refusals.close()
	stopWriter()
	<-writerDone
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
			s.log.Warn("periodic refresh failed", "cube", rt.cubeSerial(), "error", res.Err)
			continue
		}
		s.log.Info("snapshot refreshed", "cube", rt.cubeSerial())
	}
}

// cubeSerial is the Cube's serial, "" while a Cube served by its
// address has not reported it.
func (rt *runtime) cubeSerial() string {
	rt.serialMu.Lock()
	defer rt.serialMu.Unlock()
	return rt.serial
}

// learnSerial sets the serial the Cube reported, and reports whether
// the runtime did not know it yet.
func (rt *runtime) learnSerial(sn string) bool {
	rt.serialMu.Lock()
	defer rt.serialMu.Unlock()
	if rt.serial != "" || sn == "" {
		return false
	}
	rt.serial = sn
	return true
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
				if h := rt.client.Snapshot().Hello(); h != nil && rt.learnSerial(h.Serial) {
					key := strings.ToLower(h.Serial)
					s.mu.Lock()
					other, taken := s.cubes[key]
					if !taken {
						s.cubes[key] = rt
					}
					s.mu.Unlock()
					if taken && other != rt {
						// Another runtime serves the serial, one added by
						// serial meanwhile: two clients would compete for a
						// one-client Cube, so this one stops, as register
						// refuses a second runtime for a serial.
						s.log.Error("cube served twice: its serial is served already, the connection by address stops", "serial", h.Serial, "addr", rt.client.Address())
						go s.removeCube(rt)
						return
					}
					s.seedLearned(ctx, rt)
				}
				s.learnStoreID(ctx, rt)
			}
			switch e.Kind {
			case cubelan.EventConnected, cubelan.EventMetadata, cubelan.EventConfig, cubelan.EventPaired, cubelan.EventForgotten:
				rt.bk.touch() // the configuration may have changed (backups.go)
			}
			if e.Kind == cubelan.EventDisconnected && s.searchLost(rt) {
				s.wakeDiscovery() // it may have taken another address (discovery.go)
			}
			if e.Kind == cubelan.EventConnected || e.Kind == cubelan.EventMetadata {
				s.syncRooms(ctx, rt) // the rooms' uids (vwt.go)
			}
			if e.Device != 0 && rt.hidden()[e.Device] {
				continue
			}
			ev := event{Kind: e.Kind.String(), Time: time.Now()}
			if e.Device != 0 {
				ev.Device = e.Device.String()
				snap := rt.client.Snapshot()
				ev.Source, ev.Serial = "device", snap.Serial(e.Device)
				ev.realWall = realWall(snap, e.Device)
				if c := snap.Config(e.Device); c != nil && c.Type == cube.DeviceCube {
					ev.Source = "cube"
					if sn := rt.cubeSerial(); sn != "" {
						ev.Serial = sn
					}
				}
				if g := e.Gone; g != nil {
					// The snapshot has dropped the device: its view is what
					// the snapshot knew of it.
					ev.Serial = g.Serial
					ev.State = view.Device{RFAddress: ev.Device, Type: g.Type, Name: g.Name, RoomID: g.RoomID, Serial: g.Serial}
				}
				if _, ok := rt.client.Snapshot().State(e.Device); ok {
					dv := view.DeviceOf(rt.client.Snapshot(), e.Device, s.StaleAfter())
					ev.State = dv
					if e.Kind == cubelan.EventState {
						if prev, had := rt.last[e.Device]; had {
							ev.Changes = view.Changes(prev, dv.State)
						}
						rt.last[e.Device] = dv.State
						ev.Command = rt.sentTo.take(e.Device)
					}
				}
			}
			if e.Err != nil {
				ev.Error = e.Err.Error()
			}
			if e.Kind == cubelan.EventForgotten {
				delete(rt.last, e.Device)
				rt.follow.forget(e.Device)
			}
			if e.Kind == cubelan.EventPaired {
				// The metadata does not change by pairing; the device's C
				// line puts it into the device list, as unlisted.
				ev.Paired = e.New
				go rt.writer().Reread(ctx, e.Device)
			}
			if e.Kind == cubelan.EventConfig && e.Device != 0 {
				ev.Changed, ev.Etag = rt.configChanged(e.Device)
			}
			after := s.detail(ctx, rt, e, &ev)
			ev.settle()
			rt.line(ev)
			rt.remember(ev)
			s.notifyEvent(rt, ev)
			rt.subsMu.Lock()
			rt.publishLocked(ev, 0)
			rt.subsMu.Unlock()
			for _, a := range after {
				rt.emit(a)
			}
			switch e.Kind {
			case cubelan.EventState, cubelan.EventMetadata, cubelan.EventConnected, cubelan.EventConfig, cubelan.EventForgotten:
				s.publishVWTs(rt, ev.Command)
			}
			if e.Kind == cubelan.EventConfig && len(ev.Changed) > 0 {
				s.publishVirtualConfig(rt, e.Device, ev.Changed, ev.Etag)
			}
		}
	}
}

// detail says what a hello, metadata, config or connected event holds
// (logdiff.go). A greeting's own lines go to the log at debug with their
// summaries, and what they found changed against the reference is held
// until the connected line, which returns it, to follow at info. Outside
// a greeting, a line with differences says them at info, one without is
// at debug, and a device's first configuration says its summary.
func (s *Server) detail(ctx context.Context, rt *runtime, e cubelan.Event, ev *event) []event {
	snap := rt.client.Snapshot()
	hidden := rt.hidden()
	switch e.Kind {
	case cubelan.EventHello:
		ev.Summary = helloSummary(snap.Hello())
		if e.Greeting {
			ev.Level = "debug"
		}
	case cubelan.EventMetadata:
		ref := s.logReference(ctx, rt)
		md := snap.Metadata()
		changes := listChanges(ref.meta, md, hidden)
		if md != nil {
			ref.meta = md
		}
		var one listChange
		if len(changes) > 0 && sameDevice(changes) {
			one = changes[0]
		}
		switch {
		case e.Greeting:
			ev.Level, ev.Summary = "debug", listSummary(md, hidden)
			if len(changes) > 0 {
				ref.held = append(ref.held, event{Kind: "metadata", Source: "cube", Device: one.Device, Serial: one.Serial, ListChanges: changes, Missed: true, Time: ev.Time})
			}
		case len(changes) > 0:
			ev.ListChanges, ev.Device, ev.Serial = changes, one.Device, one.Serial
		default:
			ev.Level, ev.Summary = "debug", listSummary(md, hidden)
		}
	case cubelan.EventConfig:
		ref := s.logReference(ctx, rt)
		cfg := snap.Config(e.Device)
		prev := ref.configs[e.Device]
		if cfg != nil {
			ref.configs[e.Device] = cfg
		}
		changes := configChanges(prev, cfg)
		switch {
		case e.Greeting:
			ev.Level, ev.Summary = "debug", configSummary(cfg)
			if len(changes) > 0 {
				ref.held = append(ref.held, event{Kind: "config", Source: ev.Source, Device: ev.Device, Serial: ev.Serial, Changes: changes, Missed: true, Time: ev.Time})
			}
		case prev == nil:
			ev.Summary = configSummary(cfg)
		case len(changes) > 0:
			ev.Changes = changes
		default:
			ev.Level, ev.Summary = "debug", configSummary(cfg)
		}
	case cubelan.EventDisconnected:
		if !e.HeldUntil.IsZero() {
			// The connection is held open (cubelan, hold.go).
			ev.Summary = map[string]any{"held_until": e.HeldUntil.Format(time.RFC3339)}
		}
	case cubelan.EventConnected:
		ref := s.logReference(ctx, rt)
		ev.Summary = connectedSummary(snap, hidden, e)
		if e.Cause == cubelan.CauseReconnect {
			ev.Level = "debug"
		}
		held := ref.held
		ref.held = nil
		return held
	}
	return nil
}

// sameDevice reports whether every change of a list concerns one device.
func sameDevice(changes []listChange) bool {
	for _, c := range changes {
		if c.Device == "" || c.Device != changes[0].Device {
			return false
		}
	}
	return true
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
	if d.Type.IsThermostat() {
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
		h := map[string]any{"ok": true, "version": s.opts.Version, "started_at": s.started, "features": s.featuresOf()}
		if s.opts.Store != nil {
			h["node_id"] = s.opts.Store.NodeID()
		}
		if s.opts.TLS != nil {
			h["tls"] = s.opts.TLS.Status()
		}
		writeJSON(w, http.StatusOK, h)
	})
	// api registers a route with the capability it needs (auth.go): the
	// wrapper authenticates, checks the capability, and maps the handler's
	// error to a status.
	api := func(pattern, capability string, h func(http.ResponseWriter, *http.Request) error) {
		if capability != capNone && capability != capAny && !validCapability(capability) {
			panic(fmt.Sprintf("route %s: unknown capability %q", pattern, capability))
		}
		action, audited := actions[pattern]
		if strings.HasPrefix(pattern, "GET ") {
			audited = false
		}
		mux.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) {
			// Authorized first (audit.go): a request refused before
			// anyone is known is counted per address and minute, its body
			// unread; any other is a row of its own.
			var sw *statusWriter
			if audited && action != "" && s.opts.Store != nil {
				sw = &statusWriter{ResponseWriter: w}
				w = sw
			}
			p, ok := s.authorize(w, r, capability)
			if sw != nil && p == nil {
				s.refusals.note(r, pattern, action, s.clientIP(r), sw.status)
				return
			}
			var au *auditInfo
			if sw != nil {
				au, r = s.beginAudit(sw, r, pattern, action, ok)
				au.p = p
				defer func() { s.endAudit(r.Context(), au, s.clientIP(r)) }()
			}
			if !ok {
				return
			}
			if au != nil && !noCommand[action] && s.beginCommand(w, r, au) {
				return
			}
			r = withPrincipal(r, p)
			if err := h(w, r); err != nil {
				var he *httpError
				if errors.As(err, &he) {
					if he.retryAfter > 0 {
						w.Header().Set("Retry-After", strconv.Itoa(he.retryAfter))
					}
					if len(he.noState) > 0 {
						writeJSON(w, he.status, map[string]any{"error": he.msg, "no_state": he.noState})
						return
					}
					if he.etag != "" {
						w.Header().Set("ETag", quoteTag(he.etag))
						writeJSON(w, he.status, map[string]any{"error": he.msg, "etag": he.etag})
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
	api("GET /api/v1/cubes", "read", s.listCubes)
	api("POST /api/v1/login", "none", s.login)
	api("POST /api/v1/login/start", "none", s.loginStart)
	api("POST /api/v1/login/finish", "none", s.loginFinish)
	api("GET /api/v1/login/providers", "none", s.loginProviders)
	api("POST /api/v1/login/oidc/finish", "none", s.oidcFinish)
	api("GET /api/v1/login/oidc/callback", "none", s.oidcCallback)
	api("POST /api/v1/login/oidc/{key}", "none", s.oidcStartLogin)
	api("POST /api/v1/logout", "any", s.logout)
	api("GET /api/v1/me", "any", s.me)
	api("GET /api/v1/audit", "audit", s.getAudit)
	api("GET /api/v1/commands/{id}", "read", s.getCommand)
	api("GET /api/v1/me/preferences", "any", s.getPreferences)
	api("GET /api/v1/me/preferences/{key}", "any", s.getPreference)
	api("PUT /api/v1/me/preferences/{key}", "preferences", s.putPreference)
	api("DELETE /api/v1/me/preferences/{key}", "preferences", s.deletePreference)
	api("GET /api/v1/push/vapid", "read", s.getVAPID)
	api("GET /api/v1/me/push", "preferences", s.listPush)
	api("POST /api/v1/me/push", "preferences", s.addPush)
	api("DELETE /api/v1/me/push/{id}", "preferences", s.deletePush)
	api("GET /api/v1/users", "users", s.listUsers)
	api("POST /api/v1/users", "users", s.postUser)
	api("GET /api/v1/users/{name}", "users", s.getUser)
	api("PUT /api/v1/users/{name}", "users", s.putUser)
	api("DELETE /api/v1/users/{name}", "users", s.deleteUser)
	api("GET /api/v1/users/{name}/tokens", "users", s.listTokens)
	api("POST /api/v1/users/{name}/tokens", "users", s.postToken)
	api("DELETE /api/v1/users/{name}/tokens/{id}", "users", s.deleteToken)
	api("POST /api/v1/users/{name}/register", "users", s.registerOpaque)
	api("GET /api/v1/users/{name}/identities", "users", s.listIdentities)
	api("POST /api/v1/users/{name}/identities", "users", s.postIdentity)
	api("PUT /api/v1/users/{name}/identities/{id}", "users", s.putIdentity)
	api("DELETE /api/v1/users/{name}/identities/{id}", "users", s.deleteIdentity)
	api("POST /api/v1/users/{name}/identities/link/{key}", "users", s.oidcStartLink)
	api("GET /api/v1/users/{name}/identities/link/pending/{code}", "users", s.getPendingLink)
	api("POST /api/v1/users/{name}/identities/link/pending/{code}", "users", s.confirmPendingLink)
	api("DELETE /api/v1/users/{name}/identities/link/pending/{code}", "users", s.cancelPendingLink)
	api("POST /api/v1/integrations/requests", "none", s.postIntegrationRequest)
	api("GET /api/v1/integrations/requests/{id}", "none", s.getIntegrationRequest)
	api("GET /api/v1/integrations", "integrations", s.getIntegrations)
	api("POST /api/v1/integrations/{uuid}/approve", "integrations", s.approveIntegration)
	api("POST /api/v1/integrations/{uuid}/refuse", "integrations", s.refuseIntegration)
	api("POST /api/v1/integrations/{uuid}/accept-instance", "integrations", s.acceptInstance)
	api("PUT /api/v1/integrations/{uuid}", "integrations", s.putIntegration)
	api("GET /api/v1/integrations/{uuid}", "integrations", s.getIntegration)
	api("GET /api/v1/integrations/found", "integrations", s.getFoundIntegrations)
	api("PUT /api/v1/integrations/{uuid}/users/{id}", "integrations", s.putIntegrationUser)
	api("DELETE /api/v1/integrations/{uuid}", "integrations", s.deleteIntegration)
	api("GET /api/v1/providers", "auth", s.listProviders)
	api("POST /api/v1/providers", "auth", s.postProvider)
	api("PUT /api/v1/providers/{key}", "auth", s.putProvider)
	api("DELETE /api/v1/providers/{key}", "auth", s.deleteProvider)
	api("GET /api/v1/roles", "roles", s.listRoles)
	api("POST /api/v1/roles", "roles", s.postRole)
	api("PUT /api/v1/roles/{key}", "roles", s.putRole)
	api("DELETE /api/v1/roles/{key}", "roles", s.deleteRole)
	api("PUT /api/v1/roles/{key}/disabled", "roles.disable", s.putRoleDisabled)
	api("GET /api/v1/capabilities", "capabilities", s.listCapabilities)
	api("PUT /api/v1/capabilities/{key}", "capabilities", s.putCapability)
	api("POST /api/v1/cubes", "cubes", s.postCube)
	api("PUT /api/v1/cubes/{sn}", "cubes", s.putCube)
	api("DELETE /api/v1/cubes/{sn}", "cubes", s.deleteCube)
	api("POST /api/v1/cubes/{sn}/reboot", "cubes.reboot", s.rebootCube)
	api("GET /api/v1/settings", "settings", s.getSettings)
	api("PUT /api/v1/settings", "settings", s.putSettings)
	api("POST /api/v1/cubes/discover", "discover", s.postDiscover)
	api("GET /api/v1/cubes/{sn}", "read", s.getCube)
	api("GET /api/v1/cubes/{sn}/rooms", "read", s.getRooms)
	api("GET /api/v1/cubes/{sn}/devices", "read", s.getDevices)
	api("GET /api/v1/cubes/{sn}/devices/{rf}", "read", s.getDevice)
	api("PUT /api/v1/cubes/{sn}/devices/{rf}/setpoint", "setpoint", s.setDeviceSetpoint)
	api("PUT /api/v1/cubes/{sn}/rooms/{id}/setpoint", "setpoint", s.setRoomSetpoint)
	api("DELETE /api/v1/cubes/{sn}/rooms/{id}", "structure", s.deleteRoom)
	api("PUT /api/v1/cubes/{sn}/devices/{rf}/lock", "lock", s.setDeviceLock)
	api("GET /api/v1/cubes/{sn}/backup", "backup", s.getBackup)
	api("GET /api/v1/cubes/{sn}/backups", "backup", s.listBackups)
	api("POST /api/v1/cubes/{sn}/backups", "backup", s.postBackup)
	api("GET /api/v1/cubes/{sn}/backups/{id}", "backup", s.getStoredBackup)
	api("DELETE /api/v1/cubes/{sn}/backups/{id}", "restore", s.deleteBackup)
	api("GET /api/v1/cubes/{sn}/backups/{id}/diff", "backup", s.diffBackups)
	api("POST /api/v1/cubes/{sn}/restore/plan", "restore", s.planRestore)
	api("POST /api/v1/cubes/{sn}/restore", "restore", s.startRestore)
	api("GET /api/v1/cubes/{sn}/restore", "read", s.getRestore)
	api("GET /api/v1/cubes/{sn}/events", "read", s.events)
	api("POST /api/v1/cubes/{sn}/events/ticket", "read", s.issueTicket)
	api("GET /api/v1/cubes/{sn}/log", "read", s.getLog)
	api("GET /api/v1/log", "read", s.getAllLog)
	api("POST /api/v1/cubes/{sn}/pairing", "pair", s.startPairing)
	api("GET /api/v1/cubes/{sn}/pairing", "read", s.getPairing)
	api("DELETE /api/v1/cubes/{sn}/pairing", "pair", s.stopPairing)
	api("GET /api/v1/system", "read", s.getSystem)
	api("GET /api/v1/update", "read", s.getUpdate)
	api("POST /api/v1/update", "update", s.startUpdate)
	api("GET /api/v1/restart", "restart", s.getRestart)
	api("POST /api/v1/restart", "restart", s.startRestart)
	api("POST /api/v1/reload", "restart", s.reloadSettings)
	api("GET /api/v1/reboot", "reboot", s.getReboot)
	api("POST /api/v1/reboot", "reboot", s.startReboot)
	api("DELETE /api/v1/reboot", "reboot", s.cancelReboot)
	api("GET /api/v1/store", "settings", s.getStore)
	api("GET /api/v1/config", "settings", s.getConfig)
	s.configRoutes(api)
	s.uiRoutes(mux)
	s.mux = mux
	// A write that another site's page sent is refused before it is
	// routed (docs/api-v1.md, "Authentication"): a browser sends a POST
	// with a plain-text body without asking the server first, and a
	// server that asks for no key would serve it.
	cop := http.NewCrossOriginProtection()
	cop.SetDenyHandler(http.HandlerFunc(s.crossOriginRefused))
	inner := cop.Handler(mux)
	s.handler = inner
	if s.opts.TLS != nil {
		// HSTS on every response, the API's included: a browser that saw
		// it refuses http:// to this host for that long, so it goes out
		// only from a server that terminates TLS itself, and max-age=0
		// (-hsts 0) is how a box going back to plain HTTP clears it.
		hsts := "max-age=" + strconv.Itoa(max(s.opts.HSTSDays, 0)*24*60*60)
		s.handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Strict-Transport-Security", hsts)
			inner.ServeHTTP(w, r)
		})
	}
}

// crossOriginRefused answers a write another site's page sent, with 403;
// on an audited route it is counted with the other refusals of its
// address (audit.go).
func (s *Server) crossOriginRefused(w http.ResponseWriter, r *http.Request) {
	if s.opts.Store != nil {
		if _, pattern := s.mux.Handler(r); pattern != "" {
			if action := actions[pattern]; action != "" {
				s.refusals.note(r, pattern, action, s.clientIP(r), http.StatusForbidden)
			}
		}
	}
	writeError(w, http.StatusForbidden, "cross-origin request refused")
}

type httpError struct {
	status int
	msg    string
	// noState lists room members the Cube holds no state for when a
	// room command was discarded for that reason.
	noState []string
	// retryAfter is the Retry-After header's seconds, none when 0.
	retryAfter int
	// etag is the current tag of what a 412 refused a write to.
	etag string
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

// deviceSerial is the form of a device's serial number: three letters
// and seven digits, as printed on the device.
var deviceSerial = regexp.MustCompile(`^[A-Za-z]{3}[0-9]{7}$`)

// deviceOf resolves the device a path or a query names on a Cube: by
// its radio address, six hex digits, or by its serial number, which the
// Cube's snapshot holds while the Cube has the device. The two cannot be
// confused. A serial the snapshot does not know is a 404, anything else
// that is neither a 400.
func deviceOf(rt *runtime, name string) (cube.RFAddress, error) {
	if !deviceSerial.MatchString(name) {
		rf, err := cube.ParseRFAddress(name)
		if err != nil {
			return 0, fail(http.StatusBadRequest, "bad device %q: a radio address of six hex digits, or a serial number", name)
		}
		return rf, nil
	}
	rf, ok := rt.client.Snapshot().BySerial(name)
	if !ok {
		return 0, fail(http.StatusNotFound, "no device %s", strings.ToUpper(name))
	}
	return rf, nil
}

// deviceTarget is a device's audit target: "device:" and its serial
// number, whichever form the request named it by; its radio address
// for a device whose serial the Cube's snapshot does not hold.
func (s *Server) deviceTarget(sn, name string) string {
	s.mu.RLock()
	rt := s.cubes[strings.ToLower(sn)]
	s.mu.RUnlock()
	if rt == nil {
		return "device:" + strings.ToLower(name)
	}
	if room, ok := virtualOf(rt, name); ok {
		// A virtual wall thermostat's commands are its room's.
		return "room:" + strconv.Itoa(int(room.ID))
	}
	rf, err := deviceOf(rt, name)
	if err != nil {
		return "device:" + strings.ToLower(name)
	}
	if serial := rt.client.Snapshot().Serial(rf); serial != "" {
		return "device:" + serial
	}
	return "device:" + rf.String()
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
	ConfiguredAddress string `json:"configured_address"` // "" follows discovery; Address is where it connects now
	FollowDiscovery   bool   `json:"follow_discovery"`
	ProxyAddress      string `json:"proxy_address,omitempty"`
	ProxySerial       string `json:"proxy_serial,omitempty"`
	// ProxyVirtual is the option that makes the proxy show the virtual
	// wall thermostats.
	ProxyVirtual bool     `json:"proxy_virtual"`
	Hidden       []string `json:"hidden"`
	// ProxyListening lists the addresses the Cube's proxy listens on now.
	ProxyListening []string `json:"proxy_listening,omitempty"`
	// ProxyFailing lists the Cube's proxy listeners that do not listen,
	// and are tried again (runtimes.go).
	ProxyFailing []proxyFailure `json:"proxy_failing,omitempty"`
	// MAC and LastSeen come from discovery: the Cube's MAC address and
	// when it last answered.
	MAC       string     `json:"mac,omitempty"`
	LastSeen  *time.Time `json:"last_seen,omitempty"`
	Connected bool       `json:"connected"`
	// Link says how the server stands with the Cube beyond connected:
	// a lost connection held open, a Cube that refuses a new one while it
	// holds the server's old one (stale) or another client's (busy).
	Link      *cubelan.Link  `json:"link,omitempty"`
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
	// CubeTimeAt is when the server read CubeTime, the Cube's clock in
	// its greeting at the last connect, so that a client can move it on.
	CubeTimeAt *time.Time `json:"cube_time_at,omitempty"`
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
	v := cubeView{Serial: rt.cubeSerial(), Name: cfg.Name, Address: rt.client.Address(), Configured: true, Source: cfg.Source, ConfiguredAddress: cfg.Addr, FollowDiscovery: cfg.Addr == "" || cfg.FollowDiscovery,
		ProxyAddress: s.proxyAddress(rt), ProxySerial: cfg.ProxySerial, ProxyVirtual: rt.proxyVirtual.Load(), Hidden: hiddenStrings(cfg.Hidden), Budget: b, Pending: view.Pending(snap, b), Queued: rt.queue.Len()}
	v.FileDiffers = s.fileDiffers(rt.cubeSerial())
	link := rt.client.Link()
	v.Link = &link
	v.ProxyListening = s.proxyListening(rt)
	v.ProxyFailing = s.proxyFailing(rt)
	if f, ok := s.foundInfo(rt.cubeSerial()); ok {
		v.MAC, v.LastSeen = f.MAC, &f.LastSeen
	}
	if h := snap.Hello(); h != nil {
		v.Connected = rt.client.Connected()
		v.Firmware, v.RFAddress, v.Updated, v.ObservedSince = h.FirmwareVersion, h.RFAddress.String(), snap.Updated(), snap.Since()
		v.CubeTime = strings.TrimSpace(h.CubeDate + " " + h.CubeTime)
		if at := snap.HelloAt(); !at.IsZero() && v.CubeTime != "" {
			v.CubeTimeAt = &at
		}
		if c := snap.Config(h.RFAddress); c != nil {
			v.Config = c.Cube
		}
	}
	v.Rooms, v.Devices = view.Counts(snap, rt.hidden())
	return v
}

// Handlers.

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
	kind, err := kindOf(r)
	if err != nil {
		return err
	}
	v := s.summary(rt)
	v.RoomList = s.roomsOf(rt, kind)
	writeJSON(w, http.StatusOK, v)
	return nil
}

func (s *Server) getRooms(w http.ResponseWriter, r *http.Request) error {
	rt, err := s.cube(r)
	if err != nil {
		return err
	}
	kind, err := kindOf(r)
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, s.roomsOf(rt, kind))
	return nil
}

func (s *Server) getDevices(w http.ResponseWriter, r *http.Request) error {
	rt, err := s.cube(r)
	if err != nil {
		return err
	}
	kind, err := kindOf(r)
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, s.devicesOf(rt, kind))
	return nil
}

func (s *Server) getDevice(w http.ResponseWriter, r *http.Request) error {
	rt, err := s.cube(r)
	if err != nil {
		return err
	}
	if room, ok := virtualOf(rt, r.PathValue("rf")); ok {
		writeJSON(w, http.StatusOK, s.virtualDetail(rt, room))
		return nil
	} else if roomAlias.MatchString(r.PathValue("rf")) {
		return fail(http.StatusNotFound, "no room %s", strings.TrimPrefix(strings.ToLower(r.PathValue("rf")), "room-"))
	}
	rf, err := deviceOf(rt, r.PathValue("rf"))
	if err != nil {
		return err
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
	Mode  string  `json:"mode"`            // auto, manual, vacation, boost
	Temp  float64 `json:"temp,omitempty"`  // °C in 0.5 steps; 0 with auto follows the program
	Until string  `json:"until,omitempty"` // vacation end, "YYYY-MM-DD HH:MM"
}

func (s *Server) setDeviceSetpoint(w http.ResponseWriter, r *http.Request) error {
	rt, err := s.cube(r)
	if err != nil {
		return err
	}
	if room, ok := virtualOf(rt, r.PathValue("rf")); ok {
		return s.onVirtual(w, r, rt, room, vwtSetpoint)
	}
	rf, err := deviceOf(rt, r.PathValue("rf"))
	if err != nil {
		return err
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
	if room, ok := virtualOf(rt, r.PathValue("rf")); ok {
		return s.onVirtual(w, r, rt, room, vwtLock)
	}
	rf, err := deviceOf(rt, r.PathValue("rf"))
	if err != nil {
		return err
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
	res := rt.enqueue(ctx, engine.Command{Key: "lock:" + rf.String(), Priority: engine.PriorityHigh, Target: rf, RoomID: c.RoomID, Wakeup: c.RoomID != 0, Lines: []cube.Message{m}}).Wait(ctx)
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
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Minute)
	defer cancel()
	unlock, err := rt.lockStructure(ctx)
	if err != nil {
		return err
	}
	defer unlock()
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
		return fail(http.StatusBadRequest, "mode must be auto, manual, vacation or boost")
	}
	if mode == cube.ModeBoost && req.Temp == 0 {
		// The thermostats ignore the temperature of a boost: they open
		// their valves by their boost settings for their boost time and
		// then return to the mode they had (seen on hardware 2026-11-13,
		// docs/cube-protocol.md). The command still carries one; the
		// setpoint the device has keeps the line what it was.
		req.Temp = cube.TempOff
		if st, ok := rt.client.Snapshot().State(t.RFAddress); ok && st.Setpoint != nil {
			req.Temp = *st.Setpoint
		}
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
	res := rt.enqueue(ctx, engine.Command{Key: "setpoint:" + key, Priority: engine.PriorityHigh, Target: target, Lines: []cube.Message{m}}).Wait(ctx)
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
	kind, err := kindOf(r)
	if err != nil {
		return err
	}
	ch := make(chan event, 64)
	p := principalOf(r)
	sub := &subscriber{user: p.userID(), audit: p.has("audit"), kind: kind, gone: make(chan struct{})}
	// The last id the client read: the browser's own retry sends the
	// header, a client that opens a new stream (the web UI, with a fresh
	// ticket) the parameter; the header is the newer of the two.
	last := r.Header.Get("Last-Event-ID")
	if last == "" {
		last = r.URL.Query().Get("last_event_id")
	}
	// The replay and the subscription under one lock, so that no event
	// falls between them; head is the id of the newest event so far.
	rt.subsMu.Lock()
	replay, resumed := rt.since(last, sub)
	head := rt.streamID(rt.seq)
	rt.subs[ch] = sub
	rt.subsMu.Unlock()
	defer func() {
		rt.subsMu.Lock()
		delete(rt.subs, ch)
		rt.subsMu.Unlock()
	}()
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	// nginx would hold the stream back until its buffer is full; this
	// header switches that off, and Caddy streams on its own.
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	if resumed {
		for _, ev := range replay {
			writeEvent(w, ev, rt.streamID(ev.seq))
		}
	} else {
		if last != "" {
			// Too old, from before a restart, or not ours: what came
			// since is lost, so the client starts over.
			writeEvent(w, event{Kind: "resync", Source: "system", Level: "info", Time: time.Now()}, "")
		}
		// Current state first, so a client needs no separate fetch; the
		// last of it carries the newest id, where a reconnection resumes.
		devs := s.devicesOf(rt, kind)
		for i, d := range devs {
			id := ""
			if i == len(devs)-1 {
				id = head
			}
			writeEvent(w, event{Kind: "state", Device: d.RFAddress, State: d, Time: time.Now()}, id)
		}
	}
	fl.Flush()
	keep := time.NewTicker(time.Duration(s.streamKeepAlive.Load()))
	defer keep.Stop()
	// The stream ends with the server's run too, so that a stop does not
	// wait out the HTTP server's grace for a client that stays.
	stopped := s.runContext().Done()
	for {
		select {
		case <-r.Context().Done():
			return nil
		case <-stopped:
			return nil
		case ev := <-ch:
			writeEvent(w, ev, rt.streamID(ev.seq))
			fl.Flush()
		case <-sub.gone:
			// Fell behind: the events still queued go out, then the
			// stream ends, and the client's reconnection resumes.
			for {
				select {
				case ev := <-ch:
					writeEvent(w, ev, rt.streamID(ev.seq))
					continue
				default:
				}
				break
			}
			fl.Flush()
			s.log.Info("event stream ended: its client fell behind", "cube", rt.cubeSerial(), "remote", s.clientIP(r))
			return nil
		case <-keep.C:
			// The caller asked again: a stream outlives neither a key
			// revoked nor the first user of an open server (auth.go).
			if !s.stillAdmitted(r.Context(), p) {
				s.log.Info("event stream ended: its caller is not let in any more", "cube", rt.cubeSerial(), "remote", s.clientIP(r))
				return nil
			}
			fmt.Fprint(w, ": keep-alive\n\n")
			fl.Flush()
		}
	}
}

// writeEvent writes an event to a stream, with its id where it has one.
func writeEvent(w http.ResponseWriter, ev event, id string) {
	data, _ := json.Marshal(ev)
	if id != "" {
		fmt.Fprintf(w, "id: %s\n", id)
	}
	fmt.Fprintf(w, "event: %s\ndata: %s\n\n", ev.Kind, data)
}
