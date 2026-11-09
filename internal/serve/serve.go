// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

// Package serve is the server as massimo serve runs it, for a program to
// start: Open builds an Instance from a Config (the certificate source,
// the store of the data directory, the Cubes of the store and the
// settings file, the server and its listener), and Run serves until its
// context ends, then closes the store. A program embedding the server
// calls these two, as the command does.
//
// What only a process of its own has stays in the command: the flags and
// their help, the log file and level, the signals, the service manager,
// the settings file's reloader and SIGHUP, the restart, reboot and
// update helpers (given here as the server's options they are), the
// configuration view and the exit codes.
package serve

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/netip"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.metaworx.ch/massimo/internal/announce"
	"go.metaworx.ch/massimo/internal/certs"
	"go.metaworx.ch/massimo/internal/push"
	"go.metaworx.ch/massimo/internal/server"
	"go.metaworx.ch/massimo/internal/store"
)

// stopGrace is how long Run waits for the server to stop, writing the
// log's pending events, before the store closes under it.
const stopGrace = 10 * time.Second

// Config is what a server is started with.
type Config struct {
	// Listen is the API's address, such as 127.0.0.1:8099; port 0 takes
	// a free one, which Instance.Addr gives.
	Listen string
	// Addr serves this one Cube (host[:port]) instead of the configured
	// ones, without the store: massimo.db is neither opened nor
	// created, and nothing is written.
	Addr string
	// DataDir holds massimo.db and the certificates of TLS.
	DataDir string
	// Version is the build's version, for the store and the API.
	Version string
	// SettingsFile names the settings file the environment was read
	// from, for the log; the Cubes of the file are read from the
	// environment (FileCubes).
	SettingsFile string
	// Serial is the Cube chosen with -sn or MASSIMO_CUBE_SN, which the
	// per-Cube variables are read for; SerialGiven says it came from
	// -sn, and then that Cube alone is served.
	Serial      string
	SerialGiven bool
	// Proxies are the proxy listeners for this run, each an address,
	// <serial>=<address>, or off; none takes each Cube's proxy address,
	// and :62910 for a single Cube without one.
	Proxies []string
	// ProxySerial is the serial in discovery replies for this run,
	// instead of the Cube's.
	ProxySerial string
	// ShowHidden shows the hidden devices in the API and to proxy
	// clients.
	ShowHidden bool
	// Token is the API token (MASSIMO_API_TOKEN); with the store it
	// becomes the owner's key, without one it is the only one.
	Token string
	// PollInterval, RefreshInterval and DiscoverInterval win over the
	// store's settings for this run where they are set, 0 included
	// (never, or only on request); nil takes the store's setting, else
	// server.DefaultPollInterval and the others.
	PollInterval, RefreshInterval, DiscoverInterval *time.Duration
	// DiscoveryTargets are sent the discovery requests instead of the
	// broadcast, each host:port.
	DiscoveryTargets []string
	// DiscoveryListen is the local address of the discovery socket, for
	// the searches and the proxy's answers, instead of UDP 23272, the
	// only port the Cubes answer to and the original software asks; for
	// tests, with DiscoveryTargets.
	DiscoveryListen string
	// TLS is the certificate source of the listener; the zero value
	// serves plain HTTP.
	TLS TLS
	// HSTSDays is how long browsers are told to use https:// for the
	// host, with TLS; 0 sends max-age=0, which clears it.
	HSTSDays int
	// TrustedProxies are the reverse proxies whose headers name the
	// client (server.ParseTrustedProxies).
	TrustedProxies []netip.Prefix
	// Update, Restart and Reboot are what the API's update, restart and
	// reboot do; a program embedding the server leaves them empty, and
	// the API then offers none of them.
	Update  server.UpdateOptions
	Restart server.RestartOptions
	Reboot  server.RebootOptions
	// LogAttrs are added to the first log line, "api listening", after
	// the restart and the reboot: the command's sighup.
	LogAttrs []any
	// Deliverers take the users' notifications beside Web Push: an app
	// embedding the server shows them itself, with push.Local.
	Deliverers []push.Deliverer
	// Announce announces the server on the LAN by mDNS once it listens,
	// until it stops; off in the zero value.
	Announce Announce
	// Logger takes the log; nil is slog.Default().
	Logger *slog.Logger
}

// Announce says whether and how the server announces itself on the LAN
// (internal/announce). A server without a store has no node, and
// announces nothing.
type Announce struct {
	On bool
	// Name is the instance name; "" takes the box's host name.
	Name string
	// Version is the TXT record's v, the build's version alone, such as
	// v0.12.0.
	Version string
}

// startAnnounce starts the responder; a test replaces it, so that no
// test reaches the network.
var startAnnounce = func(ctx context.Context, svc announce.Service, log *slog.Logger) (stop func(), err error) {
	r, err := announce.Start(ctx, svc, log)
	if err != nil {
		return nil, err
	}
	return r.Stop, nil
}

// Instance is a server opened and not yet stopped.
type Instance struct {
	cfg   Config
	log   *slog.Logger
	src   certs.Source // nil without TLS
	st    *store.Store // nil with Config.Addr
	srv   *server.Server
	ln    net.Listener
	cubes []server.CubeConfig

	release sync.Once // the listener and the store, by Run's end or Close
}

// Open makes the certificate source, opens the store, seeds it from the
// settings file, resolves the Cubes and the proxy listeners, creates the
// server and opens its listener. The server does not run yet: Run
// starts it; an instance that is not run is released with Close.
func Open(ctx context.Context, cfg Config) (*Instance, error) {
	log := cfg.Logger
	if log == nil {
		log = slog.Default()
	}
	src, err := tlsSource(cfg.TLS, cfg.Listen, cfg.DataDir, log)
	if err != nil {
		return nil, err
	}
	st, err := openStore(cfg, log)
	if err != nil {
		return nil, err
	}
	inst, err := open(ctx, cfg, log, src, st)
	if err != nil && st != nil {
		closeStore(st, log)
	}
	return inst, err
}

// open is Open once the store is open.
func open(ctx context.Context, cfg Config, log *slog.Logger, src certs.Source, st *store.Store) (*Instance, error) {
	// The values given win for this run; otherwise the store's settings.
	poll, refresh, discover := server.DefaultPollInterval, server.DefaultRefreshInterval, server.DefaultDiscoverInterval
	var staleAfter time.Duration
	for _, d := range []struct {
		key   string
		given *time.Duration
		v     *time.Duration
	}{{store.PollIntervalKey, cfg.PollInterval, &poll}, {store.RefreshIntervalKey, cfg.RefreshInterval, &refresh}, {store.DiscoverIntervalKey, cfg.DiscoverInterval, &discover}, {store.StaleAfterKey, nil, &staleAfter}} {
		if d.given != nil {
			*d.v = *d.given
			continue
		}
		if st == nil {
			continue
		}
		if v, ok, err := st.SettingDuration(ctx, d.key); err != nil {
			log.Warn("setting ignored", "key", d.key, "error", err)
		} else if ok {
			*d.v = v
		}
	}

	su, err := setupCubes(ctx, st, cfg, log)
	if err != nil {
		return nil, err
	}
	srv := server.New(server.Options{Cubes: su.cubes, Token: cfg.Token, PollInterval: poll, RefreshInterval: refresh, DiscoverInterval: discover, DiscoveryTargets: cfg.DiscoveryTargets, DiscoveryAddr: cfg.DiscoveryListen, Logger: log, Version: cfg.Version, Proxy: su.listeners, ShowHidden: cfg.ShowHidden,
		Store: st, FileCubes: su.fileCubes, SettingsFile: cfg.SettingsFile, StaleAfter: staleAfter, ProxyFixed: len(cfg.Proxies) > 0, Update: cfg.Update, TLS: src, HSTSDays: cfg.HSTSDays, TrustedProxies: cfg.TrustedProxies,
		Restart: cfg.Restart, Reboot: cfg.Reboot, Deliverers: cfg.Deliverers})
	ln, err := net.Listen("tcp", cfg.Listen)
	if err != nil {
		return nil, err
	}
	return &Instance{cfg: cfg, log: log, src: src, st: st, srv: srv, ln: ln, cubes: su.cubes}, nil
}

// Addr is the address the API listens on, its port the one taken where
// Config.Listen asked for port 0.
func (i *Instance) Addr() string { return i.ln.Addr().String() }

// Server is the server, for the hooks a program adds before Run: the
// reloader, the configuration view.
func (i *Instance) Server() *server.Server { return i.srv }

// Store is the store, nil with Config.Addr.
func (i *Instance) Store() *store.Store { return i.st }

// Run serves the API and every Cube until ctx ends, then closes the
// store: once the server has stopped and written the log's pending
// events, or after a grace of ten seconds. An instance runs once.
func (i *Instance) Run(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	defer i.Close()
	for _, c := range i.cubes {
		i.log.Info("serving cube", "serial", c.Serial, "addr", c.Addr, "source", c.Source)
	}
	auth := any(i.cfg.Token != "")
	if i.st != nil {
		auth = i.srv.AuthMode()
	}
	restart := i.cfg.Restart.By
	if restart == "" {
		restart = "none"
	}
	attrs := append([]any{"addr", i.Addr(), "auth", auth, "restart", restart, "reboot", rebootAttr(i.cfg.Reboot)}, i.cfg.LogAttrs...)
	hs := &http.Server{Handler: i.srv.Handler()}
	if i.src != nil {
		hs.TLSConfig = certs.Config(i.src)
		cs := i.src.Status()
		attrs = append(attrs, "tls", cs.Source, "names", strings.Join(cs.Names, ","), "expires", cs.NotAfter.Format(time.RFC3339), "fingerprint", cs.Fingerprint, "hsts_days", i.cfg.HSTSDays)
	} else {
		attrs = append(attrs, "tls", "off")
	}
	if i.st != nil {
		attrs = append(attrs, "data", i.st.Dir(), "node", i.st.NodeID())
	} else {
		attrs = append(attrs, "store", "none (-addr)")
	}
	i.log.Info("api listening", attrs...)
	if i.cfg.Announce.On {
		defer i.announce(ctx)()
	}
	if a, ok := i.src.(*certs.ACME); ok && i.cfg.TLS.HTTP != "" {
		// The plain helper: the HTTP-01 challenge and a redirect to
		// https://; serving it is what lets the manager use HTTP-01.
		hln, err := net.Listen("tcp", i.cfg.TLS.HTTP)
		if err != nil {
			return fmt.Errorf("serve: -tls-http: %w", err)
		}
		i.log.Info("http helper listening", "addr", hln.Addr().String(), "for", "the ACME HTTP-01 challenge and the redirect to https://")
		go func() {
			if err := serveHTTP(ctx, &http.Server{Handler: a.HTTPHandler()}, hln); err != nil {
				i.log.Error("http helper", "error", err)
			}
		}()
	}
	runDone := make(chan struct{})
	go func() { i.srv.Run(ctx); close(runDone) }()
	if i.src != nil {
		go certs.Watch(ctx, i.src, certWatch)
	}
	err := serveHTTP(ctx, hs, i.ln)
	// The store is closed when this returns; the server's run writes the
	// log's pending events first.
	cancel()
	select {
	case <-runDone:
	case <-time.After(stopGrace):
		i.log.Warn("the server did not stop within 10s: the store closes under it, and what it writes from now on is lost")
	}
	return err
}

// announce starts the announcement on the LAN, which ends with ctx, and
// returns what waits for its goodbye. A responder that cannot start
// (no multicast, the port refused) is a warning, and the server runs
// unannounced.
func (i *Instance) announce(ctx context.Context) (wait func()) {
	if i.st == nil {
		i.log.Info("not announced on the LAN", "reason", "no store (-addr), so no node")
		return func() {}
	}
	_, p, _ := net.SplitHostPort(i.Addr())
	port, err := strconv.ParseUint(p, 10, 16)
	if err != nil {
		i.log.Warn("not announced on the LAN", "error", fmt.Errorf("the listener's port %q: %w", p, err))
		return func() {}
	}
	name := i.cfg.Announce.Name
	if name == "" {
		name, _ = os.Hostname()
	}
	svc := announce.Service{Instance: name, Port: uint16(port), Node: i.st.NodeID(), Version: i.cfg.Announce.Version}
	if src := i.src; src != nil {
		svc.Fingerprint = func() string { return src.Status().Fingerprint }
	}
	stop, err := startAnnounce(ctx, svc, i.log)
	if err != nil {
		i.log.Warn("not announced on the LAN", "error", err)
		return func() {}
	}
	i.log.Info("announced on the LAN", "service", announce.ServiceType, "name", announce.InstanceName(name), "port", port)
	return stop
}

// Close releases an instance that was not run: its listener and its
// store. Once Run has returned, it does nothing.
func (i *Instance) Close() error {
	var err error
	i.release.Do(func() {
		if err = i.ln.Close(); errors.Is(err, net.ErrClosed) {
			err = nil // closed by the shutdown already
		}
		if i.st != nil {
			closeStore(i.st, i.log)
		}
	})
	return err
}

// openStore opens the store of the data directory, or, with Config.Addr,
// none: such a run serves one Cube for this run only and writes nothing,
// not even the found Cubes.
func openStore(cfg Config, log *slog.Logger) (*store.Store, error) {
	if cfg.Addr != "" {
		return nil, nil
	}
	st, err := store.Open(cfg.DataDir, store.Options{Version: cfg.Version, Logger: log})
	if err != nil {
		return nil, fmt.Errorf("serve: %w", err)
	}
	return st, nil
}

// closeStore closes the store and says so, or says why massimo.db alone
// is behind: Close brings the WAL into the file first.
func closeStore(st *store.Store, log *slog.Logger) {
	if err := st.Close(); err != nil {
		log.Error("store not closed cleanly", "path", st.File(), "error", err)
		return
	}
	log.Info("store closed", "path", st.File())
}

// rebootAttr is the reboot in the first log line: who reboots, off
// while the owner's switch is, no-helper while one is missing, none
// where nobody can.
func rebootAttr(o server.RebootOptions) string {
	switch {
	case o.By == "":
		return "none"
	case !o.Allowed:
		return "off"
	case o.Helper != nil && o.Helper() != "":
		return "no-helper"
	}
	return o.By
}
