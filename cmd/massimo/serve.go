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
	"strings"
	"time"

	"go.metaworx.ch/massimo/internal/env"
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
	updateRequest := fs.String("update-request", os.Getenv("MASSIMO_UPDATE_REQUEST"), "with the root update helper installed: the request file that asks it for an update, such as "+update.DefaultRequestPath+" (default MASSIMO_UPDATE_REQUEST; empty = updates cannot be started from the API)")
	updateStatus := fs.String("update-status", update.DefaultStatusPath, "the update helper's status file")
	snFlag(fs)
	fs.Usage = func() {
		fmt.Fprint(os.Stderr, "usage: massimo serve [-listen 127.0.0.1:8099] [-addr <ip>] [-proxy <addr>[:<port>]] [-log <file>]\n\nKeep every Cube from the environment (MASSIMO_CUBE_<SN>_ADDRESS) connected,\nserve the JSON API (see internal/server for the routes) and answer as a Cube\non the LAN (port 62910 and UDP discovery), so that other programs and the\noriginal software connect to the server instead of the Cube, which serves\nonly one client at a time.\n\n")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return err
	}
	fromEnv := len(proxies) == 0 // listeners come from MASSIMO_CUBE_<SN>_PROXY, resolved below
	var listeners []server.ProxyListener
	for _, v := range proxies {
		if v == "" || v == "off" || v == "none" {
			listeners = nil
			break
		}
		pl := server.ProxyListener{DiscoverySerial: *proxySerial}
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
	case *addr != "":
		cubes = append(cubes, server.CubeConfig{Serial: selectedSN, Addr: *addr})
	default:
		sns, addrs := env.Serials(env.Address)
		for _, sn := range sns {
			if snExplicit && !strings.EqualFold(sn, selectedSN) {
				continue // -sn serves that one Cube only
			}
			cubes = append(cubes, server.CubeConfig{Serial: sn, Addr: addrs[sn]})
		}
		if len(cubes) == 0 {
			if a := os.Getenv(env.Address); a != "" {
				cubes = append(cubes, server.CubeConfig{Serial: selectedSN, Addr: a})
			}
		}
	}
	if len(cubes) == 0 {
		return fmt.Errorf("serve: no cube configured (set MASSIMO_CUBE_<SN>_ADDRESS or use -addr)")
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
				return fmt.Errorf("serve: several cubes configured; set %s for each, give -proxy <serial>=<addr>, or pick one with -sn", env.PerCube(env.Proxy, c.Serial))
			}
			a = env.WithPort(a)
			_, port, _ := net.SplitHostPort(a)
			listeners = append(listeners, server.ProxyListener{Serial: c.Serial, Addr: a, Discovery: port == env.CubePort, DiscoverySerial: *proxySerial})
		}
	}
	for i := range listeners {
		pl := &listeners[i]
		if pl.Serial == "" && single {
			pl.Serial = cubes[0].Serial
		}
		if pl.Serial == "" {
			return fmt.Errorf("serve: several cubes configured; name the cube of each proxy listener as -proxy <serial>=%s, or pick one with -sn", pl.Addr)
		}
		if pl.DiscoverySerial == "" {
			pl.DiscoverySerial = env.Own(env.ProxySerial, pl.Serial, single)
		}
	}
	for i := range cubes {
		list, err := env.HiddenList(cubes[i].Serial)
		if err != nil {
			return fmt.Errorf("serve: %w", err)
		}
		cubes[i].Hidden = list
		cubes[i].Name = env.Own(env.Name, cubes[i].Serial, single)
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
	srv := server.New(server.Options{Cubes: cubes, Token: *token, PollInterval: *poll, Logger: logger, Version: versionString(), Proxy: listeners, ShowHidden: showAll || showHidden,
		Update: server.UpdateOptions{Version: version, Channel: updateChannel(), Request: *updateRequest, Status: *updateStatus}})
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
