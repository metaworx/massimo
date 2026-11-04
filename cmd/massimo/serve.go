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
	snFlag(fs)
	fs.Usage = func() {
		fmt.Fprint(os.Stderr, "usage: massimo serve [-listen 127.0.0.1:8099] [-addr <ip>]\n\nKeep every Cube from the environment (MASSIMO_CUBE_<SN>_ADDRESS) connected\nand serve the JSON API; see internal/server for the routes.\n\n")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return err
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
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	srv := server.New(server.Options{Cubes: cubes, Token: *token, PollInterval: *poll, Logger: logger, Version: versionString()})
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
