// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package serve

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/netip"
	"os"
	"slices"
	"sort"
	"strings"
	"time"

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
// file until the Cube reports its serial, which the server then seeds,
// beside the store's Cubes too, its listener on the plain
// MASSIMO_CUBE_PROXY from the start; at the address of a Cube the store
// has, the plain lines are that Cube's, compared with its row
// (plainOfStore), by name or IP (PlainRow).
func setupCubes(ctx context.Context, st *store.Store, cfg Config, logger *slog.Logger) (setup, error) {
	file, plain, err := FileCubes(cfg.Serial, os.Environ())
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
			sn, err := st.PlainSerial(ctx)
			if err == nil && sn == "" {
				sn, err = plainOfStore(ctx, st, *plain, cfg.lookupHost, logger)
			}
			if err != nil {
				return setup{}, fmt.Errorf("serve: %w", err)
			}
			if sn != "" {
				// The plain lines belong to the Cube that reported this
				// serial before, or to the store's Cube at their
				// address: compared with its row from now on.
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
		// use); a single Cube without one listens on :62910. A Cube
		// without a known serial, the plain MASSIMO_CUBE_ADDRESS's, is
		// named by its address, beside other Cubes too: the server takes
		// the serial it reports.
		for _, t := range targets {
			a := t.proxy
			if a == "" && single {
				a = ":" + env.CubePort
			}
			if a == "" {
				if t.cfg.Serial == "" {
					logger.Warn("cube served without a proxy: several cubes are configured and the plain MASSIMO_CUBE_ADDRESS's cube has no MASSIMO_CUBE_PROXY", "addr", t.cfg.Addr)
				} else {
					logger.Warn("cube served without a proxy: several cubes are configured and this one has no proxy address", "serial", t.cfg.Serial, "addr", t.cfg.Addr)
				}
				continue
			}
			a = env.WithPort(a)
			_, port, _ := net.SplitHostPort(a)
			pl := server.ProxyListener{Serial: t.cfg.Serial, Addr: a, Discovery: port == env.CubePort, DiscoverySerial: cfg.ProxySerial}
			if t.cfg.Serial == "" {
				pl.CubeAddr = t.cfg.Addr
			}
			out.listeners = append(out.listeners, pl)
		}
	}
	for i := range out.listeners {
		pl := &out.listeners[i]
		if pl.Serial == "" && single {
			// "" too for a Cube without a known serial: the server takes
			// the one it reports.
			pl.Serial = targets[0].cfg.Serial
		} else if pl.Serial == "" && pl.CubeAddr == "" {
			return setup{}, fmt.Errorf("serve: several cubes configured; name the cube of each proxy listener as -proxy <serial>=%s, or pick one with -sn", pl.Addr)
		}
		if pl.DiscoverySerial == "" {
			for _, t := range targets {
				if pl.CubeAddr != "" && t.cfg.Serial == "" && t.cfg.Addr == pl.CubeAddr || pl.CubeAddr == "" && strings.EqualFold(t.cfg.Serial, pl.Serial) {
					pl.DiscoverySerial = t.proxySerial
				}
			}
		}
	}
	return out, nil
}

// plainOfStore ties the plain lines to the store's configured Cube at
// the plain MASSIMO_CUBE_ADDRESS, where there is one (PlainRow, with
// lookup): two runtimes would dial one Cube, which serves one client,
// and the one that loses never reports its serial. As where the plain
// Cube reports a serial the store serves (server.PlainServedNote), the
// setting names that Cube from now on, and its row wins over the plain
// lines. The warning names the row's address beside the plain one, which
// a tie by name does not show. It returns the serial, "" for none.
func plainOfStore(ctx context.Context, st *store.Store, plain store.FileCube, lookup HostLookup, logger *slog.Logger) (string, error) {
	rows, err := st.Cubes(ctx)
	if err != nil {
		return "", err
	}
	row, ok, several := PlainRow(ctx, rows, plain, lookup)
	if len(several) > 0 {
		var names []string
		for _, r := range several {
			names = append(names, r.Serial+" at "+r.Address)
		}
		logger.Warn(plainSeveralNote, "addr", plain.Values[store.FieldAddress].Value, "cubes", strings.Join(names, ", "))
	}
	if !ok {
		return "", nil
	}
	if err := st.SetSetting(ctx, store.PlainSerialKey, row.Serial); err != nil {
		return "", err
	}
	logger.Warn(server.PlainServedNote, "serial", row.Serial, "addr", plain.Values[store.FieldAddress].Value, "row_addr", row.Address)
	return row.Serial, nil
}

// plainSeveralNote is the start's warning where the plain
// MASSIMO_CUBE_ADDRESS's host has the IP of several of the store's
// Cubes at its port (PlainRow): it ties none of them.
const plainSeveralNote = "the plain MASSIMO_CUBE_ADDRESS has the IP of several cubes the store serves: the plain lines are tied to none of them; write the plain address as one cube's IP"

// A HostLookup looks up the IP addresses of a host name, as
// net.Resolver.LookupHost does.
type HostLookup func(ctx context.Context, host string) ([]string, error)

// plainLookupTime bounds the lookups of PlainRow, all of them together
// (D57-A): a resolver that does not answer delays the start by that
// much at most.
const plainLookupTime = 2 * time.Second

// PlainRow returns the configured row whose address is the plain
// MASSIMO_CUBE_ADDRESS's, the plain lines' Cube that serve ties them to
// at its start (and massimo config shows as tied): the same host,
// whatever its case and brackets, and the same port, the Cube's where
// an address gives none. Where no row's address is the plain one as
// written, the plain host and the hosts of the rows at its port are
// looked up, each name once, all within 2 s together, and the one row
// whose host has an IP address of the plain host's is the plain lines'
// Cube: a host name and an IP of one Cube. Where several rows have one,
// none is, and they are returned in several, for the start's warning.
// A lookup that fails or does not answer in time ties nothing. lookup
// nil is the system's resolver. It writes nothing.
func PlainRow(ctx context.Context, rows []store.Cube, plain store.FileCube, lookup HostLookup) (row store.Cube, ok bool, several []store.Cube) {
	want := cubeEndpoint(plain.Values[store.FieldAddress].Value)
	host, port, _ := net.SplitHostPort(want)
	var atPort []store.Cube
	for _, row := range rows {
		if !row.Configured || row.Address == "" {
			continue
		}
		got := cubeEndpoint(row.Address)
		if got == want {
			return row, true, nil
		}
		if _, p, _ := net.SplitHostPort(got); p == port {
			atPort = append(atPort, row)
		}
	}
	if len(atPort) == 0 {
		return store.Cube{}, false, nil
	}
	hosts := []string{host}
	for _, row := range atPort {
		h, _, _ := net.SplitHostPort(cubeEndpoint(row.Address))
		hosts = append(hosts, h)
	}
	addrs := lookupHosts(ctx, lookup, hosts)
	var tied []store.Cube
	for _, row := range atPort {
		h, _, _ := net.SplitHostPort(cubeEndpoint(row.Address))
		if slices.ContainsFunc(addrs[h], func(a netip.Addr) bool { return slices.Contains(addrs[host], a) }) {
			tied = append(tied, row)
		}
	}
	if len(tied) == 1 {
		return tied[0], true, nil
	}
	return store.Cube{}, false, tied
}

// lookupHosts returns the IP addresses of each host: an IP is its own,
// and a name is looked up once, every name at the same time, all within
// plainLookupTime; a name whose lookup failed or did not answer in time
// has none. lookup nil is the system's resolver.
func lookupHosts(ctx context.Context, lookup HostLookup, hosts []string) map[string][]netip.Addr {
	if lookup == nil {
		lookup = net.DefaultResolver.LookupHost
	}
	ctx, cancel := context.WithTimeout(ctx, plainLookupTime)
	defer cancel()
	type answer struct {
		host  string
		addrs []netip.Addr
	}
	answers := make(chan answer, len(hosts)) // a lookup that answers late does not block
	out := map[string][]netip.Addr{}
	asked := 0
	for _, h := range hosts {
		if _, seen := out[h]; seen {
			continue
		}
		if a, err := netip.ParseAddr(h); err == nil {
			out[h] = []netip.Addr{a.Unmap()}
			continue
		}
		out[h], asked = nil, asked+1
		go func() {
			var addrs []netip.Addr
			if list, err := lookup(ctx, h); err == nil {
				for _, s := range list {
					if a, err := netip.ParseAddr(s); err == nil {
						addrs = append(addrs, a.Unmap())
					}
				}
			}
			answers <- answer{h, addrs}
		}()
	}
	for ; asked > 0; asked-- {
		select {
		case a := <-answers:
			out[a.host] = a.addrs
		case <-ctx.Done():
			return out
		}
	}
	return out
}

// cubeEndpoint is a Cube's address as host:port to compare: the host in
// lower case, an IPv6 one bracketed once, and the Cube's port where the
// address gives none.
func cubeEndpoint(addr string) string {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		host, port = strings.TrimSuffix(strings.TrimPrefix(addr, "["), "]"), env.CubePort
	}
	return net.JoinHostPort(strings.ToLower(host), port)
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

// FileCubes reads the settings file's Cubes from environ, KEY=VALUE
// lines as os.Environ gives them, by the rule serve used before the
// store: every Cube with an _ADDRESS, and by serial every one with a
// _NAME or a _PROXY but no _ADDRESS; without any of those, the plain
// MASSIMO_CUBE_ADDRESS, with the serial selected (-sn or
// MASSIMO_CUBE_SN) when one is given, else returned as plain. The plain
// forms of _NAME, _PROXY and _PROXY_SERIAL apply only with a single
// Cube; the plain _IGNORE applies to every Cube. The start and a reload
// pass the process's environment, which the settings file was loaded
// into; massimo install a settings file's lines alone, so that its own
// environment does not count (AP StartConfiguration, D47-A).
func FileCubes(selected string, environ []string) (cubes []store.FileCube, plain *store.FileCube, err error) {
	have := map[string]bool{}
	var serials []string
	for _, name := range []string{env.Address, env.Name, env.Proxy} {
		sns, _ := env.Serials(name, environ)
		for _, sn := range sns {
			if !have[sn] {
				have[sn] = true
				serials = append(serials, sn)
			}
		}
	}
	sort.Strings(serials)
	if len(serials) == 0 {
		if env.Lookup(env.Address, environ) == "" {
			return nil, nil, nil
		}
		fc, err := fileCube(strings.ToUpper(selected), true, true, environ)
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
		fc, err := fileCube(sn, single, single && strings.EqualFold(sn, selected), environ)
		if err != nil {
			return nil, nil, err
		}
		cubes = append(cubes, fc)
	}
	return cubes, nil, nil
}

// fileCube reads one Cube's lines from environ (FileCubes); sn "" is the
// plain forms only. The plain MASSIMO_CUBE_ADDRESS counts only for
// plainCube: the Cube served when no Cube has lines of its own, or the
// one Cube that has some when MASSIMO_CUBE_SN names it.
func fileCube(sn string, single, plainCube bool, environ []string) (store.FileCube, error) {
	fc := store.FileCube{Serial: sn, Values: map[string]store.FileValue{}}
	line := func(field, name string, plainToo bool) {
		if sn != "" {
			if variable, v := env.PerCubeValue(name, sn, environ); v != "" {
				fc.Values[field] = store.FileValue{Variable: variable, Value: v}
				return
			}
		}
		if plainToo || sn == "" {
			if v := env.Lookup(name, environ); v != "" {
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
