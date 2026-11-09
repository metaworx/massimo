// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package serve

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	"sort"
	"strings"

	"go.metaworx.ch/massimo/internal/env"
	"go.metaworx.ch/massimo/internal/protocol/cube"
	"go.metaworx.ch/massimo/internal/server"
	"go.metaworx.ch/massimo/internal/store"
)

// setup is what the server starts with: the Cubes, the proxy listeners,
// and the settings file's view of each Cube.
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

// setupCubes seeds the store from the settings file (store.Seed) and
// resolves the Cubes to serve and the proxy listeners from the store's
// rows, or, with Config.Addr, from that address and the file for this
// run as before the store. Config.SerialGiven serves the one Cube;
// Config.Proxies and Config.ProxySerial win over the store for this run.
// The plain MASSIMO_CUBE_ADDRESS without a serial is served from the
// file until the Cube reports its serial, which the server then seeds.
func setupCubes(ctx context.Context, st *store.Store, cfg Config, logger *slog.Logger) (setup, error) {
	file, plain, err := FileCubes(cfg.Serial)
	if err != nil {
		return setup{}, err
	}
	out := setup{fileCubes: map[string]store.FileCube{}}
	var targets []target
	if cfg.Addr != "" {
		// -addr: this one Cube for this run, nothing written; its values
		// from the file, as before the store.
		single := true
		t := target{cfg: server.CubeConfig{Serial: cfg.Serial, Addr: cfg.Addr, Source: store.SourceFlag},
			proxy: env.Own(env.Proxy, cfg.Serial, single), proxySerial: env.Own(env.ProxySerial, cfg.Serial, single)}
		list, err := env.HiddenList(cfg.Serial)
		if err != nil {
			return setup{}, fmt.Errorf("serve: %w", err)
		}
		t.cfg.Hidden, t.cfg.Name = list, env.Own(env.Name, cfg.Serial, single)
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
		if _, err := st.Seed(ctx, cfg.SettingsFile, file); err != nil {
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
			if !row.Configured || cfg.SerialGiven && !strings.EqualFold(row.Serial, cfg.Serial) {
				continue // -sn serves that one Cube only
			}
			t, err := rowTarget(row)
			if err != nil {
				return setup{}, err
			}
			targets = append(targets, t)
		}
		switch {
		case plain != nil && !cfg.SerialGiven:
			v := func(f string) string { return plain.Values[f].Value }
			t := target{cfg: server.CubeConfig{Addr: v(store.FieldAddress), Name: v(store.FieldName), Source: store.SourceEnv, SeedFromFile: true},
				proxy: v(store.FieldProxyAddress), proxySerial: v(store.FieldProxySerial)}
			if t.cfg.Hidden, err = parseHidden(store.SplitHidden(v(store.FieldHidden))); err != nil {
				return setup{}, err
			}
			targets = append(targets, t)
			out.fileCubes[""] = *plain
		case len(targets) == 0 && cfg.Serial != "":
			// -sn or MASSIMO_CUBE_SN alone: found by discovery, for this
			// run.
			targets = append(targets, target{cfg: server.CubeConfig{Serial: strings.ToUpper(cfg.Serial), Source: store.SourceFlag}})
		}
	}
	if len(targets) == 0 {
		// Nothing to serve yet: the web UI adds a Cube (System settings →
		// Cubes), as does the settings file at the next start.
		logger.Warn("no cube configured yet: add one under System settings → Cubes in the web UI, set MASSIMO_CUBE_<SN>_ADDRESS, use -addr, or give the serial with -sn")
		return out, nil
	}
	sort.SliceStable(targets, func(i, j int) bool { return targets[i].cfg.Serial < targets[j].cfg.Serial })
	single := len(targets) == 1
	for _, t := range targets {
		out.cubes = append(out.cubes, t.cfg)
	}
	if len(cfg.Proxies) > 0 {
		for _, v := range cfg.Proxies {
			if v == "" || v == "off" || v == "none" {
				out.listeners = nil
				break
			}
			pl := server.ProxyListener{DiscoverySerial: cfg.ProxySerial}
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
			out.listeners = append(out.listeners, server.ProxyListener{Serial: t.cfg.Serial, Addr: a, Discovery: port == env.CubePort, DiscoverySerial: cfg.ProxySerial})
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

// FileCubes reads the settings file's Cubes from the environment by the
// rule serve used before the store: every Cube with an _ADDRESS, and by
// serial every one with a _NAME or a _PROXY but no _ADDRESS; without any
// of those, the plain MASSIMO_CUBE_ADDRESS, with the serial selected
// (-sn or MASSIMO_CUBE_SN) when one is given, else returned as plain.
// The plain forms of _NAME, _PROXY and _PROXY_SERIAL apply only with a
// single Cube; the plain _IGNORE applies to every Cube.
func FileCubes(selected string) (cubes []store.FileCube, plain *store.FileCube, err error) {
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
		fc, err := fileCube(strings.ToUpper(selected), true, true)
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
		// The plain address is the one Cube's when MASSIMO_CUBE_SN (or
		// -sn) names it, as env.Get reads it for the other commands.
		fc, err := fileCube(sn, single, single && strings.EqualFold(sn, selected))
		if err != nil {
			return nil, nil, err
		}
		cubes = append(cubes, fc)
	}
	return cubes, nil, nil
}

// fileCube reads one Cube's lines; sn "" is the plain forms only. The
// plain MASSIMO_CUBE_ADDRESS counts only for plainCube: the Cube served
// when no Cube has lines of its own, or the one Cube that has some when
// MASSIMO_CUBE_SN names it.
func fileCube(sn string, single, plainCube bool) (store.FileCube, error) {
	fc := store.FileCube{Serial: sn, Values: map[string]store.FileValue{}}
	line := func(field, name string, plainToo bool) {
		if sn != "" {
			if variable, v := env.PerCubeValue(name, sn); v != "" {
				fc.Values[field] = store.FileValue{Variable: variable, Value: v}
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
