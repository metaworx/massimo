// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sort"
	"strings"
	"time"

	"go.metaworx.ch/massimo/internal/server"
)

// runServe starts the HTTP API over every configured Cube.
func runServe(args []string) error {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	listen := fs.String("listen", "127.0.0.1:8099", "address to listen on")
	addr := fs.String("addr", "", "serve this one Cube (host[:port]) instead of the configured ones")
	poll := fs.Duration("poll", time.Minute, "how often to poll the device list")
	token := fs.String("token", os.Getenv("MASSIMO_API_TOKEN"), "API token required as Bearer token (default MASSIMO_API_TOKEN; empty = no authentication)")
	var proxies []string
	fs.Func("proxy", "answer as a Cube on this address (repeatable; default :62910 on every interface; \"off\" disables; with several Cubes give <serial>=<addr>)", func(v string) error {
		proxies = append(proxies, v)
		return nil
	})
	showGhosts := fs.Bool("show-ghosts", false, "show the devices of room 255 and of the ignore lists in the API and to proxy clients instead of hiding them")
	proxySerial := fs.String("proxy-serial", "", "serial to use in UDP discovery replies instead of the Cube's (default "+proxySerialEnv+" or its per-Cube form; the original software keeps one answer per serial)")
	snFlag(fs)
	fs.Usage = func() {
		fmt.Fprint(os.Stderr, "usage: massimo serve [-listen 127.0.0.1:8099] [-addr <ip>] [-proxy <addr>[:<port>]]\n\nKeep every Cube from the environment (MASSIMO_CUBE_<SN>_ADDRESS) connected,\nserve the JSON API (see internal/server for the routes) and answer as a Cube\non the LAN (port 62910 and UDP discovery), so that other programs and the\noriginal software connect to the server instead of the Cube, which serves\nonly one client at a time.\n\n")
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
		if _, _, err := net.SplitHostPort(v); err != nil {
			v = net.JoinHostPort(v, "62910")
		}
		pl.Addr = v
		_, port, _ := net.SplitHostPort(v)
		pl.Discovery = port == "62910"
		listeners = append(listeners, pl)
	}
	var cubes []server.CubeConfig
	switch {
	case *addr != "":
		cubes = append(cubes, server.CubeConfig{Serial: selectedSN, Addr: *addr})
	default:
		var sns []string
		for _, kv := range os.Environ() {
			k, v, _ := strings.Cut(kv, "=")
			mid, ok := strings.CutPrefix(k, envPrefix)
			if !ok {
				continue
			}
			if sn, ok := strings.CutSuffix(mid, "_ADDRESS"); ok && sn != "" && v != "" {
				if snExplicit && !strings.EqualFold(sn, selectedSN) {
					continue // -sn serves that one Cube only
				}
				sns = append(sns, sn)
				cubes = append(cubes, server.CubeConfig{Serial: sn, Addr: v})
			}
		}
		if len(cubes) == 0 {
			if a := os.Getenv(addrEnv); a != "" {
				cubes = append(cubes, server.CubeConfig{Serial: selectedSN, Addr: a})
			}
		}
		sort.Strings(sns)
	}
	if len(cubes) == 0 {
		return fmt.Errorf("serve: no cube configured (set MASSIMO_CUBE_<SN>_ADDRESS or use -addr)")
	}
	if fromEnv {
		// One listener per Cube on its MASSIMO_CUBE_<SN>_PROXY address (the
		// address clients use); a single Cube without it listens on :62910.
		for _, c := range cubes {
			a := os.Getenv(perCubeVar(proxyEnv, c.Serial))
			if a == "" && len(cubes) == 1 {
				a = os.Getenv(proxyEnv)
			}
			if a == "" && len(cubes) == 1 {
				a = ":62910"
			}
			if a == "" {
				return fmt.Errorf("serve: several cubes configured; set %s for each, give -proxy <serial>=<addr>, or pick one with -sn", perCubeVar(proxyEnv, c.Serial))
			}
			if _, _, err := net.SplitHostPort(a); err != nil {
				a = net.JoinHostPort(a, "62910")
			}
			_, port, _ := net.SplitHostPort(a)
			listeners = append(listeners, server.ProxyListener{Serial: c.Serial, Addr: a, Discovery: port == "62910", DiscoverySerial: *proxySerial})
		}
	}
	for i := range listeners {
		pl := &listeners[i]
		if pl.Serial == "" && len(cubes) == 1 {
			pl.Serial = cubes[0].Serial
		}
		if pl.Serial == "" {
			return fmt.Errorf("serve: several cubes configured; name the cube of each proxy listener as -proxy <serial>=%s, or pick one with -sn", pl.Addr)
		}
		if pl.DiscoverySerial == "" {
			pl.DiscoverySerial = os.Getenv(perCubeVar(proxySerialEnv, pl.Serial))
		}
		if pl.DiscoverySerial == "" {
			pl.DiscoverySerial = os.Getenv(proxySerialEnv)
		}
	}
	for i := range cubes {
		name := ignoreEnv
		if sn := cubes[i].Serial; sn != "" && os.Getenv(perCubeVar(ignoreEnv, sn)) != "" {
			name = perCubeVar(ignoreEnv, sn)
		}
		ignore, err := parseIgnoreList(name, os.Getenv(name))
		if err != nil {
			return fmt.Errorf("serve: %w", err)
		}
		cubes[i].Ignore = ignore
	}
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	srv := server.New(server.Options{Cubes: cubes, Token: *token, PollInterval: *poll, Logger: logger, Version: versionString(), Proxy: listeners, ShowGhosts: *showGhosts || noIgnore})
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
