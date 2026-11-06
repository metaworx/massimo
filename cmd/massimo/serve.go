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

	"go.metaworx.ch/massimo/internal/env"
	"go.metaworx.ch/massimo/internal/install"
	"go.metaworx.ch/massimo/internal/server"
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
	addr := fs.String("addr", "", "serve this one Cube (host[:port]) instead of the configured ones")
	poll := fs.Duration("poll", time.Minute, "how often to poll the device list")
	refresh := fs.Duration("refresh", 6*time.Hour, "how often to reconnect to each Cube to re-read everything it holds (0 = never)")
	discoverEvery := fs.Duration("discover-interval", 30*time.Minute, "how often to look for Cubes on the LAN, with a first run at the start (0 = only on request through the API)")
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
	fs.Func("proxy", "answer as a Cube on this address (repeatable; default :62910 on every interface; \"off\" disables; with several Cubes give <serial>=<addr>)", func(v string) error {
		proxies = append(proxies, v)
		return nil
	})
	var showAll bool
	fs.BoolVar(&showAll, "show-hidden", false, "show the hidden devices (those of room 255 and of "+env.Ignore+") in the API and to proxy clients")
	fs.BoolVar(&showAll, "show-ghosts", false, "older name of -show-hidden")
	proxySerial := fs.String("proxy-serial", "", "serial to use in UDP discovery replies instead of the Cube's (default "+env.ProxySerial+" or its per-Cube form; the original software keeps one answer per serial)")
	updateRequest := fs.String("update-request", os.Getenv("MASSIMO_UPDATE_REQUEST"), "with the update helper installed: the request file that asks it for an update, such as "+update.DefaultRequestPath+"; on Windows the server then starts the helper itself (default MASSIMO_UPDATE_REQUEST; empty = updates cannot be started from the API)")
	updateStatus := fs.String("update-status", defaultUpdateStatus(), "the update helper's status file (on Windows install passes it, and the helper's update.log goes next to it)")
	snFlag(fs)
	fs.Usage = func() {
		fmt.Fprint(os.Stderr, "usage: massimo serve [-listen 127.0.0.1:8099] [-addr <ip>] [-proxy <addr>[:<port>]] [-log <file>]\n\nKeep every Cube from the environment (MASSIMO_CUBE_<SN>_ADDRESS) connected,\nserve the JSON API (see internal/server for the routes) and answer as a Cube\non the LAN (port 62910 and UDP discovery), so that other programs and the\noriginal software connect to the server instead of the Cube, which serves\nonly one client at a time.\n\n")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return err
	}
	cubes, listeners, err := serveTargets(*addr, proxies, *proxySerial)
	if err != nil {
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
	upd := server.UpdateOptions{Version: version, Channel: updateChannel(), Request: *updateRequest, Status: *updateStatus}
	if runtime.GOOS == "windows" && *updateRequest != "" {
		if upd.Start, upd.Blocked, err = serveUpdateHelper(logger, *listen, *updateRequest, *updateStatus); err != nil {
			return fmt.Errorf("serve: %w", err)
		}
	}
	srv := server.New(server.Options{Cubes: cubes, Token: *token, PollInterval: *poll, RefreshInterval: *refresh, DiscoverInterval: *discoverEvery, DiscoveryTargets: discoverTargets, Logger: logger, Version: versionString(), Proxy: listeners, ShowHidden: showAll || showHidden,
		Update: upd})
	ln, err := net.Listen("tcp", *listen)
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	for _, c := range cubes {
		logger.Info("serving cube", "serial", c.Serial, "addr", c.Addr)
	}
	logger.Info("api listening", "addr", ln.Addr().String(), "auth", *token != "")
	for _, pl := range listeners {
		logger.Info("cube proxy listening", "addr", pl.Addr, "serial", pl.Serial, "discovery", pl.Discovery, "discovery_serial", pl.DiscoverySerial)
	}
	hs := &http.Server{Handler: srv.Handler()}
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		hs.Shutdown(shutdown)
	}()
	go srv.Run(ctx)
	if err := hs.Serve(ln); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
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
// /healthz of the listen address and update.log next to the status file.
func serveUpdateHelper(log *slog.Logger, listen, request, status string) (func() error, string, error) {
	exe, err := os.Executable()
	if err != nil {
		return nil, "", err
	}
	health, err := install.HealthURL(listen)
	if err != nil {
		return nil, "", err
	}
	start, blocked := updateHelperStarter(log, exe, request, status, health, filepath.Join(filepath.Dir(status), "update.log"))
	if blocked != "" {
		log.Warn("updates cannot be started from the API", "reason", blocked)
	}
	return start, blocked, nil
}

// serveTargets resolves the Cubes to serve and the proxy listeners from
// -addr, -proxy, -proxy-serial and the environment. A single Cube may
// have no serial (MASSIMO_CUBE_ADDRESS without -sn): the server then
// takes the serial the Cube reports.
func serveTargets(addr string, proxies []string, proxySerial string) ([]server.CubeConfig, []server.ProxyListener, error) {
	fromEnv := len(proxies) == 0 // listeners come from MASSIMO_CUBE_<SN>_PROXY, resolved below
	var listeners []server.ProxyListener
	for _, v := range proxies {
		if v == "" || v == "off" || v == "none" {
			listeners = nil
			break
		}
		pl := server.ProxyListener{DiscoverySerial: proxySerial}
		if sn, addr, ok := strings.Cut(v, "="); ok {
			pl.Serial, v = sn, addr
		}
		pl.Addr = env.WithPort(v)
		_, port, _ := net.SplitHostPort(pl.Addr)
		pl.Discovery = port == env.CubePort
		listeners = append(listeners, pl)
	}
	var cubes []server.CubeConfig
	switch {
	case addr != "":
		cubes = append(cubes, server.CubeConfig{Serial: selectedSN, Addr: addr})
	default:
		sns, addrs := env.Serials(env.Address)
		for _, sn := range sns {
			if snExplicit && !strings.EqualFold(sn, selectedSN) {
				continue // -sn serves that one Cube only
			}
			cubes = append(cubes, server.CubeConfig{Serial: sn, Addr: addrs[sn]})
		}
		// Cubes configured by serial only, with serve's own variables
		// (_NAME, _PROXY) but no _ADDRESS: their address comes from
		// discovery and follows the Cube when it changes.
		have := map[string]bool{}
		for _, c := range cubes {
			have[strings.ToUpper(c.Serial)] = true
		}
		for _, name := range []string{env.Name, env.Proxy} {
			more, _ := env.Serials(name)
			for _, sn := range more {
				if have[sn] || snExplicit && !strings.EqualFold(sn, selectedSN) {
					continue
				}
				have[sn] = true
				cubes = append(cubes, server.CubeConfig{Serial: sn})
			}
		}
		sort.Slice(cubes, func(i, j int) bool { return cubes[i].Serial < cubes[j].Serial })
		if len(cubes) == 0 {
			if a := os.Getenv(env.Address); a != "" {
				cubes = append(cubes, server.CubeConfig{Serial: selectedSN, Addr: a})
			} else if selectedSN != "" {
				// -sn or MASSIMO_CUBE_SN without an address: found by
				// discovery.
				cubes = append(cubes, server.CubeConfig{Serial: strings.ToUpper(selectedSN)})
			}
		}
	}
	if len(cubes) == 0 {
		return nil, nil, fmt.Errorf("serve: no cube configured (set MASSIMO_CUBE_<SN>_ADDRESS, use -addr, or give the serial with -sn to find the Cube on the LAN)")
	}
	// A plain variable applies to a single Cube only where two Cubes must
	// not share the value: the proxy address, its serial and the name.
	single := len(cubes) == 1
	if fromEnv {
		// One listener per Cube on its MASSIMO_CUBE_<SN>_PROXY address (the
		// address clients use); a single Cube without it listens on :62910.
		for _, c := range cubes {
			a := env.Own(env.Proxy, c.Serial, single)
			if a == "" && single {
				a = ":" + env.CubePort
			}
			if a == "" {
				return nil, nil, fmt.Errorf("serve: several cubes configured; set %s for each, give -proxy <serial>=<addr>, or pick one with -sn", env.PerCube(env.Proxy, c.Serial))
			}
			a = env.WithPort(a)
			_, port, _ := net.SplitHostPort(a)
			listeners = append(listeners, server.ProxyListener{Serial: c.Serial, Addr: a, Discovery: port == env.CubePort, DiscoverySerial: proxySerial})
		}
	}
	for i := range listeners {
		pl := &listeners[i]
		if pl.Serial == "" && single {
			// "" too for a Cube without a configured serial: the server
			// takes the one it reports.
			pl.Serial = cubes[0].Serial
		} else if pl.Serial == "" {
			return nil, nil, fmt.Errorf("serve: several cubes configured; name the cube of each proxy listener as -proxy <serial>=%s, or pick one with -sn", pl.Addr)
		}
		if pl.DiscoverySerial == "" {
			pl.DiscoverySerial = env.Own(env.ProxySerial, pl.Serial, single)
		}
	}
	for i := range cubes {
		list, err := env.HiddenList(cubes[i].Serial)
		if err != nil {
			return nil, nil, fmt.Errorf("serve: %w", err)
		}
		cubes[i].Hidden = list
		cubes[i].Name = env.Own(env.Name, cubes[i].Serial, single)
	}
	return cubes, listeners, nil
}
