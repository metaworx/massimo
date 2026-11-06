// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"go.metaworx.ch/massimo/internal/certs"
	"go.metaworx.ch/massimo/internal/env"
	"go.metaworx.ch/massimo/internal/install"
	"go.metaworx.ch/massimo/internal/protocol/cube"
	"go.metaworx.ch/massimo/internal/server"
	"go.metaworx.ch/massimo/internal/store"
	"go.metaworx.ch/massimo/internal/update"
)

// runServe starts the HTTP API over every configured Cube.
func runServe(args []string) error {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	listenDefault := os.Getenv("MASSIMO_LISTEN")
	if listenDefault == "" {
		listenDefault = "127.0.0.1:8099"
	}
	listen := fs.String("listen", listenDefault, "address to listen on (default MASSIMO_LISTEN, else 127.0.0.1:8099)")
	addr := fs.String("addr", "", "serve this one Cube (host[:port]) instead of the configured ones, for this run and without the store: massimo.db is neither opened nor created, and nothing is written")
	poll := fs.Duration("poll", time.Minute, "how often to poll the device list (default the store's poll_interval, else 1m)")
	refresh := fs.Duration("refresh", 6*time.Hour, "how often to reconnect to each Cube to re-read everything it holds (0 = never; default the store's refresh_interval, else 6h)")
	discoverEvery := fs.Duration("discover-interval", 30*time.Minute, "how often to look for Cubes on the LAN, with a first run at the start (0 = only on request through the API; default the store's discover_interval, else 30m)")
	var discoverTargets []string
	fs.Func("discover-target", "send discovery requests to this host[:port] instead of broadcasting (repeatable; for another subnet, or a fake Cube's -discovery)", func(v string) error {
		if _, _, err := net.SplitHostPort(v); err != nil {
			v = net.JoinHostPort(v, "23272")
		}
		discoverTargets = append(discoverTargets, v)
		return nil
	})
	token := fs.String("token", os.Getenv("MASSIMO_API_TOKEN"), "API token required as Bearer token (default MASSIMO_API_TOKEN; empty = no authentication)")
	logPath := fs.String("log", os.Getenv("MASSIMO_LOG"), "append the log (connections, commands, device state changes) to this file as well as stderr (default MASSIMO_LOG)")
	var proxies []string
	fs.Func("proxy", "answer as a Cube on this address, for this run (repeatable; default the store's proxy address per Cube, else :62910 with a single Cube; \"off\" disables; with several Cubes give <serial>=<addr>)", func(v string) error {
		proxies = append(proxies, v)
		return nil
	})
	var showAll bool
	fs.BoolVar(&showAll, "show-hidden", false, "show the hidden devices (those of room 255 and of the Cube's hidden list) in the API and to proxy clients")
	fs.BoolVar(&showAll, "show-ghosts", false, "older name of -show-hidden")
	proxySerial := fs.String("proxy-serial", "", "serial to use in UDP discovery replies instead of the Cube's, for this run (default the store's proxy serial; the original software keeps one answer per serial)")
	updateRequest := fs.String("update-request", os.Getenv("MASSIMO_UPDATE_REQUEST"), "with the update helper installed: the request file that asks it for an update, such as "+update.DefaultRequestPath+"; on Windows the server then starts the helper itself (default MASSIMO_UPDATE_REQUEST; empty = updates cannot be started from the API)")
	updateStatus := fs.String("update-status", defaultUpdateStatus(), "the update helper's status file (on Windows install passes it, and the helper's update.log goes next to it)")
	tlsFlag := fs.String("tls", os.Getenv(tlsVar), "serve over TLS: off; files for the certificate of -tls-cert and -tls-key; self-signed for one the server makes for the box's names in the data directory's tls/ (default "+tlsVar+", else off; files when -tls-cert is given)")
	tlsCert := fs.String("tls-cert", os.Getenv(tlsCertVar), "the certificate for -tls files, PEM, the chain after the server's certificate; re-read when it changes (default "+tlsCertVar+")")
	tlsKey := fs.String("tls-key", os.Getenv(tlsKeyVar), "the certificate's private key, PEM (default "+tlsKeyVar+")")
	tlsNames := fs.String("tls-names", os.Getenv(tlsNamesVar), "with -tls self-signed, names and addresses the certificate holds besides the box's own, comma-separated (default "+tlsNamesVar+")")
	hstsFlag := fs.String("hsts", os.Getenv(hstsVar), "with TLS, tell browsers to use https:// for this host for this many days (Strict-Transport-Security; default "+hstsVar+", else 365); 0 sends max-age=0, which clears it, and is the way before switching TLS off")
	snFlag(fs)
	fs.Usage = func() {
		fmt.Fprint(os.Stderr, "usage: massimo [-data <dir>] serve [-listen 127.0.0.1:8099] [-addr <ip>] [-proxy <addr>[:<port>]] [-log <file>] [-tls files -tls-cert <pem> -tls-key <pem> | -tls self-signed]\n\nKeep every configured Cube connected, serve the JSON API (see internal/server\nfor the routes) and answer as a Cube on the LAN (port 62910 and UDP\ndiscovery), so that other programs and the original software connect to the\nserver instead of the Cube, which serves only one client at a time.\n\nThe Cubes and the server's settings are kept in massimo.db in the data\ndirectory (-data, MASSIMO_DATA, else the current directory). At every start\na Cube of the settings file (MASSIMO_CUBE_<SN>_ADDRESS, _NAME, _PROXY) that\nthe store does not know is added to it; afterwards the store wins, and a\nline of the file that differs is logged.\n\nWith -tls files the API and the web UI are served over TLS from the\ncertificate and key files, re-read when they change, so a renewal needs no\nrestart; with -tls self-signed from a certificate the server makes at its\nfirst start for the box's names (and -tls-names), which browsers warn about\nonce per device: compare the fingerprint the log shows. Browsers are then\ntold to use https:// for a year (-hsts). Without -tls the listener is plain\nHTTP, for localhost or behind a reverse proxy.\n\n")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return err
	}
	var logOut io.Writer = os.Stderr
	if *logPath != "" {
		f, err := os.OpenFile(*logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if err != nil {
			return fmt.Errorf("serve: %w", err)
		}
		defer f.Close()
		logOut = io.MultiWriter(os.Stderr, f)
	}
	logger := slog.New(slog.NewTextHandler(logOut, nil))
	hsts, err := hstsDays(*hstsFlag)
	if err != nil {
		return err
	}
	src, err := tlsSource(tlsOptions{mode: *tlsFlag, certPath: *tlsCert, keyPath: *tlsKey, names: *tlsNames, listen: *listen, dataDir: dataDir()}, logger)
	if err != nil {
		return err
	}

	st, err := openServeStore(*addr, logger)
	if err != nil {
		return err
	}
	if st != nil {
		defer st.Close()
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	// The flags given win for this run; otherwise the store's settings.
	var staleAfter time.Duration
	if st != nil {
		given := map[string]bool{}
		fs.Visit(func(f *flag.Flag) { given[f.Name] = true })
		for _, d := range []struct {
			flag, key string
			v         *time.Duration
		}{{"poll", store.PollIntervalKey, poll}, {"refresh", store.RefreshIntervalKey, refresh}, {"discover-interval", store.DiscoverIntervalKey, discoverEvery}, {"", store.StaleAfterKey, &staleAfter}} {
			if d.flag != "" && given[d.flag] {
				continue
			}
			if v, ok, err := st.SettingDuration(ctx, d.key); err != nil {
				logger.Warn("setting ignored", "key", d.key, "error", err)
			} else if ok {
				*d.v = v
			}
		}
	}

	setup, err := serveSetup(ctx, st, *addr, proxies, *proxySerial, logger)
	if err != nil {
		return err
	}
	upd := server.UpdateOptions{Version: version, Channel: updateChannel(), Request: *updateRequest, Status: *updateStatus}
	if runtime.GOOS == "windows" && *updateRequest != "" {
		if upd.Start, upd.Blocked, err = serveUpdateHelper(logger, *listen, src != nil, *updateRequest, *updateStatus); err != nil {
			return fmt.Errorf("serve: %w", err)
		}
	}
	srv := server.New(server.Options{Cubes: setup.cubes, Token: *token, PollInterval: *poll, RefreshInterval: *refresh, DiscoverInterval: *discoverEvery, DiscoveryTargets: discoverTargets, Logger: logger, Version: versionString(), Proxy: setup.listeners, ShowHidden: showAll || showHidden,
		Store: st, FileCubes: setup.fileCubes, SettingsFile: env.File(), StaleAfter: staleAfter, ProxyFixed: len(proxies) > 0, Update: upd, TLS: src, HSTSDays: hsts})
	ln, err := net.Listen("tcp", *listen)
	if err != nil {
		return err
	}
	for _, c := range setup.cubes {
		logger.Info("serving cube", "serial", c.Serial, "addr", c.Addr, "source", c.Source)
	}
	attrs := []any{"addr", ln.Addr().String(), "auth", *token != "", "tls", "off"}
	hs := &http.Server{Handler: srv.Handler()}
	if src != nil {
		hs.TLSConfig = certs.Config(src)
		cs := src.Status()
		attrs = append(attrs[:len(attrs)-1], cs.Source, "names", strings.Join(cs.Names, ","), "expires", cs.NotAfter.Format(time.RFC3339), "fingerprint", cs.Fingerprint, "hsts_days", hsts)
	}
	if st != nil {
		attrs = append(attrs, "data", st.Dir(), "node", st.NodeID())
	} else {
		attrs = append(attrs, "store", "none (-addr)")
	}
	logger.Info("api listening", attrs...)
	go srv.Run(ctx)
	if src != nil {
		go certs.Watch(ctx, src, certWatch)
	}
	return serveHTTP(ctx, hs, ln)
}

// defaultUpdateStatus is the helper's status file where install puts it:
// the Unix path, on Windows the one in %ProgramData%\Massimo.
func defaultUpdateStatus() string {
	if runtime.GOOS == "windows" {
		return install.WindowsPaths(os.Getenv("ProgramFiles"), os.Getenv("ProgramData")).Status
	}
	return update.DefaultStatusPath
}

// serveUpdateHelper prepares the start of the update helper by the
// server itself (Windows): this binary in its helper mode, with the
// /healthz of the listen address, https:// when this server has TLS,
// and update.log next to the status file.
func serveUpdateHelper(log *slog.Logger, listen string, tls bool, request, status string) (func() error, string, error) {
	exe, err := os.Executable()
	if err != nil {
		return nil, "", err
	}
	health, err := install.HealthURL(listen, tls)
	if err != nil {
		return nil, "", err
	}
	start, blocked := updateHelperStarter(log, exe, request, status, health, filepath.Join(filepath.Dir(status), "update.log"))
	if blocked != "" {
		log.Warn("updates cannot be started from the API", "reason", blocked)
	}
	return start, blocked, nil
}

// openServeStore opens the store of the data directory, or, with -addr,
// none: such a run serves one Cube for this run only and writes nothing,
// not even the found Cubes.
func openServeStore(addr string, logger *slog.Logger) (*store.Store, error) {
	if addr != "" {
		return nil, nil
	}
	st, err := store.Open(dataDir(), store.Options{Version: versionString(), Logger: logger})
	if err != nil {
		return nil, fmt.Errorf("serve: %w", err)
	}
	return st, nil
}

// setup is what serve starts with: the Cubes, the proxy listeners, and
// the settings file's view of each Cube.
type setup struct {
	cubes     []server.CubeConfig
	listeners []server.ProxyListener
	fileCubes map[string]store.FileCube
}

// target is one Cube to serve with its proxy values.
type target struct {
	cfg         server.CubeConfig
	proxy       string // the listener address, "" for the default
	proxySerial string
}

// serveSetup seeds the store from the settings file (store.Seed) and
// resolves the Cubes to serve and the proxy listeners from the store's
// rows, or, with -addr, from -addr and the file for this run as before
// the store. -sn serves the one Cube; -proxy and -proxy-serial win over
// the store for this run. The plain MASSIMO_CUBE_ADDRESS without a
// serial is served from the file until the Cube reports its serial,
// which the server then seeds.
func serveSetup(ctx context.Context, st *store.Store, addr string, proxies []string, proxySerial string, logger *slog.Logger) (setup, error) {
	file, plain, err := fileCubes()
	if err != nil {
		return setup{}, err
	}
	out := setup{fileCubes: map[string]store.FileCube{}}
	var targets []target
	if addr != "" {
		// -addr: this one Cube for this run, nothing written; its values
		// from the file, as before the store.
		single := true
		t := target{cfg: server.CubeConfig{Serial: selectedSN, Addr: addr, Source: store.SourceFlag},
			proxy: env.Own(env.Proxy, selectedSN, single), proxySerial: env.Own(env.ProxySerial, selectedSN, single)}
		list, err := env.HiddenList(selectedSN)
		if err != nil {
			return setup{}, fmt.Errorf("serve: %w", err)
		}
		t.cfg.Hidden, t.cfg.Name = list, env.Own(env.Name, selectedSN, single)
		targets = append(targets, t)
	} else {
		if plain != nil {
			if sn, err := st.PlainSerial(ctx); err != nil {
				return setup{}, fmt.Errorf("serve: %w", err)
			} else if sn != "" {
				// The plain lines belong to the Cube that reported this
				// serial before: compared with its row from now on.
				plain.Serial = sn
				file = append(file, *plain)
				plain = nil
			}
		}
		if _, err := st.Seed(ctx, env.File(), file); err != nil {
			return setup{}, fmt.Errorf("serve: %w", err)
		}
		for _, fc := range file {
			out.fileCubes[store.NormalSerial(fc.Serial)] = fc
		}
		rows, err := st.Cubes(ctx)
		if err != nil {
			return setup{}, fmt.Errorf("serve: %w", err)
		}
		for _, row := range rows {
			if !row.Configured || snExplicit && !strings.EqualFold(row.Serial, selectedSN) {
				continue // -sn serves that one Cube only
			}
			t, err := rowTarget(row)
			if err != nil {
				return setup{}, err
			}
			targets = append(targets, t)
		}
		switch {
		case plain != nil && !snExplicit:
			v := func(f string) string { return plain.Values[f].Value }
			t := target{cfg: server.CubeConfig{Addr: v(store.FieldAddress), Name: v(store.FieldName), Source: store.SourceEnv, SeedFromFile: true},
				proxy: v(store.FieldProxyAddress), proxySerial: v(store.FieldProxySerial)}
			if t.cfg.Hidden, err = parseHidden(store.SplitHidden(v(store.FieldHidden))); err != nil {
				return setup{}, err
			}
			targets = append(targets, t)
			out.fileCubes[""] = *plain
		case len(targets) == 0 && selectedSN != "":
			// -sn or MASSIMO_CUBE_SN alone: found by discovery, for this
			// run.
			targets = append(targets, target{cfg: server.CubeConfig{Serial: strings.ToUpper(selectedSN), Source: store.SourceFlag}})
		}
	}
	if len(targets) == 0 {
		// Nothing to serve yet: the web UI adds a Cube (Settings →
		// Cubes), as does the settings file at the next start.
		logger.Warn("no cube configured yet: add one under Settings → Cubes in the web UI, set MASSIMO_CUBE_<SN>_ADDRESS, use -addr, or give the serial with -sn")
		return out, nil
	}
	sort.SliceStable(targets, func(i, j int) bool { return targets[i].cfg.Serial < targets[j].cfg.Serial })
	single := len(targets) == 1
	for _, t := range targets {
		out.cubes = append(out.cubes, t.cfg)
	}
	if len(proxies) > 0 {
		for _, v := range proxies {
			if v == "" || v == "off" || v == "none" {
				out.listeners = nil
				break
			}
			pl := server.ProxyListener{DiscoverySerial: proxySerial}
			if sn, addr, ok := strings.Cut(v, "="); ok {
				pl.Serial, v = sn, addr
			}
			pl.Addr = env.WithPort(v)
			_, port, _ := net.SplitHostPort(pl.Addr)
			pl.Discovery = port == env.CubePort
			out.listeners = append(out.listeners, pl)
		}
	} else {
		// One listener per Cube on its proxy address (the address clients
		// use); a single Cube without one listens on :62910.
		for _, t := range targets {
			a := t.proxy
			if a == "" && single {
				a = ":" + env.CubePort
			}
			if a == "" {
				logger.Warn("cube served without a proxy: several cubes are configured and this one has no proxy address", "serial", t.cfg.Serial)
				continue
			}
			a = env.WithPort(a)
			_, port, _ := net.SplitHostPort(a)
			out.listeners = append(out.listeners, server.ProxyListener{Serial: t.cfg.Serial, Addr: a, Discovery: port == env.CubePort, DiscoverySerial: proxySerial})
		}
	}
	for i := range out.listeners {
		pl := &out.listeners[i]
		if pl.Serial == "" && single {
			// "" too for a Cube without a known serial: the server takes
			// the one it reports.
			pl.Serial = targets[0].cfg.Serial
		} else if pl.Serial == "" {
			return setup{}, fmt.Errorf("serve: several cubes configured; name the cube of each proxy listener as -proxy <serial>=%s, or pick one with -sn", pl.Addr)
		}
		if pl.DiscoverySerial == "" {
			for _, t := range targets {
				if strings.EqualFold(t.cfg.Serial, pl.Serial) {
					pl.DiscoverySerial = t.proxySerial
				}
			}
		}
	}
	return out, nil
}

// rowTarget is the target of a configured row of the store.
func rowTarget(row store.Cube) (target, error) {
	cfg, err := server.CubeFromRow(row)
	if err != nil {
		return target{}, fmt.Errorf("serve: cube %s: hidden: %w", row.Serial, err)
	}
	return target{cfg: cfg, proxy: row.ProxyAddress, proxySerial: row.ProxySerial}, nil
}

// parseHidden reads a hidden list of radio addresses.
func parseHidden(list []string) ([]cube.RFAddress, error) {
	var out []cube.RFAddress
	for _, a := range list {
		rf, err := cube.ParseRFAddress(a)
		if err != nil {
			return nil, fmt.Errorf("hidden: %w", err)
		}
		out = append(out, rf)
	}
	return out, nil
}

// fileCubes reads the settings file's Cubes by the rule serve used
// before the store: every Cube with an _ADDRESS, and by serial every one
// with a _NAME or a _PROXY but no _ADDRESS; without any of those, the
// plain MASSIMO_CUBE_ADDRESS, with the serial of -sn or MASSIMO_CUBE_SN
// when one is given, else returned as plain. The plain forms of _NAME,
// _PROXY and _PROXY_SERIAL apply only with a single Cube; the plain
// _IGNORE applies to every Cube.
func fileCubes() (cubes []store.FileCube, plain *store.FileCube, err error) {
	have := map[string]bool{}
	var serials []string
	for _, name := range []string{env.Address, env.Name, env.Proxy} {
		sns, _ := env.Serials(name)
		for _, sn := range sns {
			if !have[sn] {
				have[sn] = true
				serials = append(serials, sn)
			}
		}
	}
	sort.Strings(serials)
	if len(serials) == 0 {
		if os.Getenv(env.Address) == "" {
			return nil, nil, nil
		}
		fc, err := fileCube(strings.ToUpper(selectedSN), true, true)
		if err != nil {
			return nil, nil, err
		}
		if fc.Serial == "" {
			return nil, &fc, nil
		}
		return []store.FileCube{fc}, nil, nil
	}
	single := len(serials) == 1
	for _, sn := range serials {
		fc, err := fileCube(sn, single, false)
		if err != nil {
			return nil, nil, err
		}
		cubes = append(cubes, fc)
	}
	return cubes, nil, nil
}

// fileCube reads one Cube's lines; sn "" is the plain forms only. The
// plain MASSIMO_CUBE_ADDRESS counts only for plainCube, the Cube served
// when no Cube has lines of its own.
func fileCube(sn string, single, plainCube bool) (store.FileCube, error) {
	fc := store.FileCube{Serial: sn, Values: map[string]store.FileValue{}}
	line := func(field, name string, plainToo bool) {
		if sn != "" {
			if v := os.Getenv(env.PerCube(name, sn)); v != "" {
				fc.Values[field] = store.FileValue{Variable: env.PerCube(name, sn), Value: v}
				return
			}
		}
		if plainToo || sn == "" {
			if v := os.Getenv(name); v != "" {
				fc.Values[field] = store.FileValue{Variable: name, Value: v}
			}
		}
	}
	line(store.FieldAddress, env.Address, plainCube)
	line(store.FieldName, env.Name, single)
	line(store.FieldProxyAddress, env.Proxy, single)
	line(store.FieldProxySerial, env.ProxySerial, single)
	line(store.FieldHidden, env.Ignore, true)
	if h, ok := fc.Values[store.FieldHidden]; ok {
		list, err := env.ParseList(h.Variable, h.Value)
		if err != nil {
			return fc, err
		}
		var norm []string
		for _, rf := range list {
			norm = append(norm, rf.String())
		}
		h.Value = strings.Join(norm, ",")
		fc.Values[store.FieldHidden] = h
	}
	return fc, nil
}
