// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package serve

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"go.metaworx.ch/massimo/internal/cubelan/fakecube"
	"go.metaworx.ch/massimo/internal/env"
	"go.metaworx.ch/massimo/internal/protocol/cube"
	"go.metaworx.ch/massimo/internal/server"
	"go.metaworx.ch/massimo/internal/store"
	"go.metaworx.ch/massimo/internal/testkit"
	"go.metaworx.ch/massimo/internal/testkit/storekit"
)

// clearCubeEnv unsets every MASSIMO_CUBE_* variable for the test, and
// points the settings file and the data directory away from the
// checkout's. The plain forms are the test's whether set or not, so
// that what a settings file the test loads sets in the process
// (env.LoadFile) is undone at its end, before the parallel tests read
// the environment (noCubeEnv). A test that calls it is serial.
func clearCubeEnv(t *testing.T) {
	t.Helper()
	names := []string{env.Address, env.Name, env.Proxy, env.ProxySerial, env.SN, env.Ignore}
	for _, kv := range os.Environ() {
		if k, _, _ := strings.Cut(kv, "="); strings.HasPrefix(k, env.Prefix) {
			names = append(names, k)
		}
	}
	for _, k := range names {
		t.Setenv(k, "")
		os.Unsetenv(k)
	}
	t.Setenv(env.FileVar, os.DevNull)
	t.Setenv(env.DataVar, t.TempDir())
}

// unsetCubeEnv unsets every MASSIMO_CUBE_* variable of the process, as
// TestMain does before the tests run.
func unsetCubeEnv() {
	for _, kv := range os.Environ() {
		if k, _, _ := strings.Cut(kv, "="); strings.HasPrefix(k, env.Prefix) {
			os.Unsetenv(k)
		}
	}
}

// noCubeEnv fails the test where a MASSIMO_CUBE_* variable is set: a
// parallel test serves no Cube from the environment, and reads it as
// TestMain left it, since every serial test restored what it set
// (clearCubeEnv).
func noCubeEnv(t *testing.T) {
	t.Helper()
	for _, kv := range os.Environ() {
		if k, _, _ := strings.Cut(kv, "="); strings.HasPrefix(k, env.Prefix) {
			t.Fatalf("%s is set: a serial test left it", k)
		}
	}
}

func quietLog() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

// testStore opens a store in a temporary directory, or in dir: a copy of
// the migrated template where dir has no store yet (storekit.Open).
func testStore(t *testing.T, dir string) *store.Store {
	t.Helper()
	return storekit.Open(t, dir)
}

// greetingFile is the fake Cube's recorded greeting.
const greetingFile = "../protocol/cube/testdata/cube-fw0113-greeting.txt"

func startFakeCube(t *testing.T) string {
	t.Helper()
	greeting, err := fakecube.LoadGreeting(greetingFile)
	if err != nil {
		t.Fatal(err)
	}
	fc := &fakecube.Server{Greeting: greeting}
	cubeAddr, err := fc.Start("127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { fc.Close() })
	return cubeAddr
}

// freeAddr returns a loopback address whose port was free a moment ago;
// another process can take it before the caller listens on it. It stays
// for the proxy address of MASSIMO_CUBE_PROXY: the server's proxy listens
// at an address alone (server.Options.Proxy), and the hook that takes a
// listener in its place is the server package's own (docs/testing.md,
// "Ports").
func freeAddr(t *testing.T) string {
	t.Helper()
	free, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer free.Close()
	return free.Addr().String()
}

// proxyGreeting dials the proxy at addr until deadline and returns the
// connection with the first line it reads. A refused dial is tried
// again while the proxy starts, and so is a connection that ends before
// its first line (D174-A): make race runs other packages' tests at the
// same time, so another process can listen on freeAddr's port before
// the proxy and drop this client, while the proxy tries its own
// listener again. A line that does not parse, or none by the deadline,
// ends the tries.
func proxyGreeting(addr string, deadline time.Time) (net.Conn, cube.Message, error) {
	err := errors.New("no time left to dial")
	for ; time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
		var conn net.Conn
		if conn, err = net.DialTimeout("tcp", addr, time.Second); err != nil {
			continue
		}
		conn.SetReadDeadline(deadline)
		var m cube.Message
		if m, err = cube.NewReader(conn).Read(); err == nil {
			return conn, m, nil
		}
		conn.Close()
		if !connEnded(err) {
			break
		}
	}
	return nil, cube.Message{}, err
}

// connEnded reports whether err is the end of a connection, closed or
// reset by its peer, rather than a deadline or a line that does not
// parse.
func connEnded(err error) bool {
	var op *net.OpError
	return errors.Is(err, io.EOF) || errors.As(err, &op) && !op.Timeout()
}

// FileCubes reads the Cubes from the list it is given, the process
// environment of the start and the reload, a settings file's lines for
// massimo install, and from nothing else (AP StartConfiguration, D47-A):
// a Cube's lines and the plain address of the list, and a bad _IGNORE
// of the list refused.
func TestFileCubesReadTheListGiven(t *testing.T) {
	t.Parallel()
	cubes, plain, err := FileCubes("", []string{env.PerCube(env.Address, "KEQ0000000") + "=192.0.2.5", env.PerCube(env.Name, "KEQ0000000") + "=Test"})
	if err != nil || plain != nil || len(cubes) != 1 || cubes[0].Serial != "KEQ0000000" || cubes[0].Values[store.FieldAddress].Value != "192.0.2.5" || cubes[0].Values[store.FieldName].Value != "Test" {
		t.Errorf("a Cube's lines: %+v, plain %+v, %v", cubes, plain, err)
	}
	if cubes, plain, err := FileCubes("", []string{env.Address + "=192.0.2.6"}); err != nil || len(cubes) != 0 || plain == nil || plain.Values[store.FieldAddress].Value != "192.0.2.6" {
		t.Errorf("the plain address: %+v, plain %+v, %v", cubes, plain, err)
	}
	ignore := env.PerCube(env.Ignore, "KEQ0000000")
	if _, _, err := FileCubes("", []string{env.PerCube(env.Address, "KEQ0000000") + "=192.0.2.5", ignore + "=zz"}); err == nil || !strings.HasPrefix(err.Error(), ignore+": ") {
		t.Errorf("a bad hidden list: %v", err)
	}
}

// A single Cube given only as MASSIMO_CUBE_ADDRESS, without a serial, is
// served with its proxy; the serial comes from the Cube's greeting, and
// the server then adds the Cube to the store, so the next start serves
// it from its row.
func TestServeSingleCubeWithoutSerial(t *testing.T) {
	clearCubeEnv(t)
	cubeAddr := startFakeCube(t)
	proxyAddr := freeAddr(t)
	envFile := filepath.Join(t.TempDir(), "x.env")
	os.WriteFile(envFile, []byte("MASSIMO_CUBE_ADDRESS="+cubeAddr+"\nMASSIMO_CUBE_PROXY="+proxyAddr+"\n"), 0o600)
	if _, err := env.LoadFile(envFile); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	st := testStore(t, dir)
	ctx := context.Background()

	su, err := setupCubes(ctx, st, Config{}, quietLog())
	if err != nil {
		t.Fatal(err)
	}
	if len(su.cubes) != 1 || su.cubes[0].Serial != "" || su.cubes[0].Addr != cubeAddr || !su.cubes[0].SeedFromFile || len(su.listeners) != 1 || su.listeners[0].Addr != proxyAddr || su.listeners[0].Serial != "" {
		t.Fatalf("cubes %+v listeners %+v", su.cubes, su.listeners)
	}
	// Without a proxy address the single Cube listens on the Cube port.
	os.Unsetenv(env.Proxy)
	if su, err := setupCubes(ctx, st, Config{}, quietLog()); err != nil || len(su.listeners) != 1 || su.listeners[0].Addr != ":62910" || !su.listeners[0].Discovery {
		t.Fatalf("default listener %+v %v", su.listeners, err)
	}
	os.Setenv(env.Proxy, proxyAddr)

	srv := server.New(server.Options{Cubes: su.cubes, Proxy: su.listeners, Store: st, FileCubes: su.fileCubes, Logger: quietLog()})
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	go srv.Run(runCtx)
	conn, m, err := proxyGreeting(proxyAddr, time.Now().Add(testkit.Scale(20*time.Second)))
	if conn != nil {
		defer conn.Close()
	}
	if err != nil || m.Type != 'H' || !strings.HasPrefix(m.Data, "KEQ0000100,") {
		t.Fatalf("greeting %v %v", m, err)
	}
	var row store.Cube
	testkit.Within(5*time.Second, func() bool {
		var ok bool
		row, ok, _ = st.Cube(ctx, "KEQ0000100")
		return ok
	})
	if !row.Configured || row.Source != store.SourceEnv || row.Address != cubeAddr || row.ProxyAddress != proxyAddr {
		t.Fatalf("row after the serial was learned: %+v", row)
	}
	cancel()

	// The next start: the plain lines belong to KEQ0000100's row.
	su, err = setupCubes(ctx, st, Config{}, quietLog())
	if err != nil || len(su.cubes) != 1 || su.cubes[0].Serial != "KEQ0000100" || su.cubes[0].SeedFromFile || su.cubes[0].Addr != cubeAddr {
		t.Fatalf("second start %+v %v", su.cubes, err)
	}
	if _, ok := su.fileCubes["KEQ0000100"]; !ok {
		t.Fatalf("file values not kept by serial: %+v", su.fileCubes)
	}
}

// With several Cubes, a proxy listener without a serial is refused; a
// Cube without a proxy address is served without a proxy.
func TestServeSeveralCubesNeedNamedListeners(t *testing.T) {
	clearCubeEnv(t)
	t.Setenv("MASSIMO_CUBE_KEQ0000001_ADDRESS", "10.0.0.1")
	t.Setenv("MASSIMO_CUBE_KEQ0000002_ADDRESS", "10.0.0.2")
	ctx := context.Background()
	if _, err := setupCubes(ctx, testStore(t, ""), Config{Proxies: []string{":62910"}}, quietLog()); err == nil || !strings.Contains(err.Error(), "several cubes") {
		t.Fatalf("unnamed listener with two cubes: %v", err)
	}
	logger, logs := testkit.LogWatch(nil)
	su, err := setupCubes(ctx, testStore(t, ""), Config{}, logger)
	if err != nil || len(su.cubes) != 2 || len(su.listeners) != 0 || logs.Count(0, testkit.Contains("without a proxy")) != 2 {
		t.Fatalf("two cubes without proxy addresses: %+v %v %q", su, err, logs.Since(0, nil))
	}
	t.Setenv("MASSIMO_CUBE_KEQ0000001_PROXY", "10.0.0.11")
	t.Setenv("MASSIMO_CUBE_KEQ0000002_PROXY", "10.0.0.12")
	t.Setenv("MASSIMO_CUBE_KEQ0000002_PROXY_SERIAL", "LEQ0000002")
	su, err = setupCubes(ctx, testStore(t, ""), Config{}, quietLog())
	if err != nil || len(su.cubes) != 2 || len(su.listeners) != 2 || su.listeners[0].Serial != "KEQ0000001" || su.listeners[1].Addr != "10.0.0.12:62910" || su.listeners[1].DiscoverySerial != "LEQ0000002" {
		t.Fatalf("cubes %+v listeners %+v %v", su.cubes, su.listeners, err)
	}
}

// Cubes with serve's own variables but no address are served by serial,
// their address left to discovery; so is a Cube selected with -sn alone.
func TestServeCubesConfiguredBySerial(t *testing.T) {
	clearCubeEnv(t)
	t.Setenv("MASSIMO_CUBE_KEQ0000001_ADDRESS", "10.0.0.1")
	t.Setenv("MASSIMO_CUBE_KEQ0000002_NAME", "Attic")
	t.Setenv("MASSIMO_CUBE_KEQ0000003_PROXY", "10.0.0.13")
	t.Setenv("MASSIMO_CUBE_KEQ0000004_KEY", "k0123") // a key alone does not make serve take the Cube
	t.Setenv("MASSIMO_CUBE_KEQ0000001_PROXY", "10.0.0.11")
	t.Setenv("MASSIMO_CUBE_KEQ0000002_PROXY", "10.0.0.12")
	ctx := context.Background()
	su, err := setupCubes(ctx, testStore(t, ""), Config{}, quietLog())
	if err != nil {
		t.Fatal(err)
	}
	cubes := su.cubes
	if len(cubes) != 3 || cubes[0].Addr != "10.0.0.1" || cubes[1].Serial != "KEQ0000002" || cubes[1].Addr != "" || !cubes[1].FollowDiscovery || cubes[1].Name != "Attic" || cubes[2].Serial != "KEQ0000003" || cubes[2].Addr != "" {
		t.Fatalf("cubes %+v", cubes)
	}

	clearCubeEnv(t)
	su, err = setupCubes(ctx, testStore(t, ""), Config{Serial: "keq0000009", SerialGiven: true}, quietLog())
	if err != nil || len(su.cubes) != 1 || su.cubes[0].Serial != "KEQ0000009" || su.cubes[0].Addr != "" || su.cubes[0].Source != store.SourceFlag {
		t.Fatalf("-sn alone: %+v %v", su.cubes, err)
	}
}

// The first start seeds the store from the file; afterwards the store
// wins, the differing line is reported on the Cube in the API, and -addr
// writes nothing.
func TestServeStoreWins(t *testing.T) {
	clearCubeEnv(t)
	t.Setenv("MASSIMO_CUBE_KEQ0000001_ADDRESS", "10.0.0.1")
	t.Setenv("MASSIMO_CUBE_KEQ0000001_NAME", "Cellar")
	t.Setenv("MASSIMO_CUBE_KEQ0000001_IGNORE", "0ABC12")
	ctx := context.Background()
	dir := t.TempDir()
	st := testStore(t, dir)
	su, err := setupCubes(ctx, st, Config{}, quietLog())
	if err != nil || len(su.cubes) != 1 || su.cubes[0].Name != "Cellar" || len(su.cubes[0].Hidden) != 1 || su.cubes[0].Source != store.SourceEnv {
		t.Fatalf("first start %+v %v", su.cubes, err)
	}
	if _, err := st.UpdateCube(ctx, "KEQ0000001", func(c *store.Cube) error {
		c.Name, c.Address, c.Source = "Basement", "10.0.0.2", store.SourceUI
		return nil
	}, 0); err != nil {
		t.Fatal(err)
	}
	su, err = setupCubes(ctx, st, Config{}, quietLog())
	if err != nil || su.cubes[0].Name != "Basement" || su.cubes[0].Addr != "10.0.0.2" || su.cubes[0].Source != store.SourceUI {
		t.Fatalf("second start %+v %v", su.cubes, err)
	}
	srv := server.New(server.Options{Cubes: su.cubes, Store: st, FileCubes: su.fileCubes, Logger: quietLog()})
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()
	resp, err := http.Get(ts.URL + "/api/v1/cubes")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var list []struct {
		Serial      string `json:"serial"`
		Source      string `json:"source"`
		FileDiffers []struct {
			Field, Variable string
			File, Store     any
		} `json:"file_differs"`
	}
	json.NewDecoder(resp.Body).Decode(&list)
	if len(list) != 1 || list[0].Source != "ui" || len(list[0].FileDiffers) != 2 || list[0].FileDiffers[0].Field != "name" || list[0].FileDiffers[0].File != "Cellar" ||
		list[0].FileDiffers[1].Field != "address" || list[0].FileDiffers[1].Variable != "MASSIMO_CUBE_KEQ0000001_ADDRESS" {
		t.Fatalf("cube list %+v", list)
	}
	resp2, _ := http.Get(ts.URL + "/healthz")
	var h map[string]any
	json.NewDecoder(resp2.Body).Decode(&h)
	resp2.Body.Close()
	if h["node_id"] != st.NodeID() {
		t.Fatalf("healthz %v", h)
	}

	// -addr serves one Cube for this run and writes nothing.
	before, _ := st.Cubes(ctx)
	su, err = setupCubes(ctx, st, Config{Addr: "10.0.0.9"}, quietLog())
	after, _ := st.Cubes(ctx)
	if err != nil || len(su.cubes) != 1 || su.cubes[0].Addr != "10.0.0.9" || su.cubes[0].Source != store.SourceFlag || len(after) != len(before) || after[0].Version != before[0].Version {
		t.Fatalf("-addr %+v %v", su.cubes, err)
	}
}

// A Cube removed through the API stays removed at the next start,
// though the settings file still names it: by its own variable, or as
// the plain MASSIMO_CUBE_ADDRESS whose serial the store learned.
func TestServeRemovedCubeStaysRemoved(t *testing.T) {
	clearCubeEnv(t)
	t.Setenv("MASSIMO_CUBE_KEQ0000001_ADDRESS", "192.0.2.1")
	t.Setenv(env.Address, "192.0.2.2")
	ctx := context.Background()
	st := testStore(t, "")
	for _, sn := range []string{"KEQ0000001", "KEQ0000002"} {
		if _, err := st.AddCube(ctx, store.Cube{Serial: sn, Configured: false, Source: store.SourceUI}); err != nil {
			t.Fatal(err)
		}
	}
	if err := st.SetSetting(ctx, store.PlainSerialKey, "KEQ0000002"); err != nil {
		t.Fatal(err)
	}
	su, err := setupCubes(ctx, st, Config{}, quietLog())
	if err != nil || len(su.cubes) != 0 {
		t.Fatalf("served %+v %v", su.cubes, err)
	}
	for _, sn := range []string{"KEQ0000001", "KEQ0000002"} {
		if row, _, _ := st.Cube(ctx, sn); row.Configured || row.Source != store.SourceUI {
			t.Fatalf("%s configured again: %+v", sn, row)
		}
	}
}

// Without any Cube configured serve starts anyway, with a warning: the
// web UI can add one.
func TestServeStartsWithoutCubes(t *testing.T) {
	t.Parallel()
	noCubeEnv(t)
	logger, logs := testkit.LogWatch(nil)
	su, err := setupCubes(context.Background(), testStore(t, ""), Config{}, logger)
	if err != nil || len(su.cubes) != 0 || len(su.listeners) != 0 {
		t.Fatalf("%+v %v", su, err)
	}
	logs.Has(t, 0, "the warning that no cube is configured", testkit.Contains("no cube configured yet"))
}

// serve -addr runs without the store: no massimo.db, no node.key.
func TestServeAddrOpensNoStore(t *testing.T) {
	t.Parallel()
	noCubeEnv(t)
	dir := t.TempDir()
	st, err := openStore(Config{Addr: "10.0.0.9", DataDir: dir}, quietLog())
	if err != nil || st != nil {
		t.Fatalf("-addr opened a store: %v %v", st, err)
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 0 {
		t.Fatalf("-addr wrote into the data directory: %v", entries)
	}
	su, err := setupCubes(context.Background(), nil, Config{Addr: "10.0.0.9"}, quietLog())
	if err != nil || len(su.cubes) != 1 || su.cubes[0].Source != store.SourceFlag {
		t.Fatalf("-addr without a store: %+v %v", su, err)
	}
	st, err = openStore(Config{DataDir: dir}, quietLog())
	if err != nil || st == nil {
		t.Fatalf("without -addr: %v %v", st, err)
	}
	st.Close()
	if _, err := os.Stat(filepath.Join(dir, store.FileName)); err != nil {
		t.Fatal(err)
	}
}

// One Cube with a variable of its own beside the plain address: the
// plain MASSIMO_CUBE_ADDRESS is that Cube's when MASSIMO_CUBE_SN names
// it, as env.Get reads it for the other commands; with another serial
// named, or two Cubes, it is no Cube's.
func TestServePlainAddressOfTheNamedCube(t *testing.T) {
	ctx := context.Background()
	for _, c := range []struct {
		sn, other, want string
	}{
		{"KEQ0000000", "", "192.0.2.5"},
		{"keq0000000", "", "192.0.2.5"},
		{"KEQ0000001", "", ""},
		{"KEQ0000000", "KEQ0000002", ""},
	} {
		clearCubeEnv(t)
		t.Setenv(env.SN, c.sn)
		t.Setenv(env.Address, "192.0.2.5")
		t.Setenv("MASSIMO_CUBE_KEQ0000000_PROXY", "192.0.2.6")
		if c.other != "" {
			t.Setenv("MASSIMO_CUBE_"+c.other+"_NAME", "Attic")
		}
		su, err := setupCubes(ctx, testStore(t, ""), Config{Serial: c.sn}, quietLog()) // as main takes it from MASSIMO_CUBE_SN
		if err != nil {
			t.Fatal(err)
		}
		var got string
		for _, cu := range su.cubes {
			if cu.Serial == "KEQ0000000" {
				got = cu.Addr
			}
		}
		if got != c.want {
			t.Errorf("MASSIMO_CUBE_SN=%s, other %q: the Cube's address %q, want %q (%+v)", c.sn, c.other, got, c.want, su.cubes)
		}
	}
}

// A Cube whose serial a variable's name has in lower case is served with
// that variable's address.
func TestServeLowerCaseSerial(t *testing.T) {
	clearCubeEnv(t)
	t.Setenv("MASSIMO_CUBE_keq0000000_ADDRESS", "192.0.2.5")
	su, err := setupCubes(context.Background(), testStore(t, ""), Config{}, quietLog())
	if err != nil {
		t.Fatal(err)
	}
	if len(su.cubes) != 1 || su.cubes[0].Serial != "KEQ0000000" || su.cubes[0].Addr != "192.0.2.5" {
		t.Fatalf("cubes %+v", su.cubes)
	}
}

// Once the Cube of the plain MASSIMO_CUBE_ADDRESS reported its serial,
// the server writes its row and the setting that ties the plain lines to
// it; a stop between the two must leave a store the next start serves as
// one Cube. An SQLite trigger refuses one of the writes, which cuts the
// run short there. The setting refused used to leave the row alone, and
// the next start refused the row and the plain lines as several Cubes.
func TestServePlainSeedCutShort(t *testing.T) {
	for _, c := range []struct{ name, trigger string }{
		{"the setting refused", `CREATE TRIGGER cut BEFORE INSERT ON settings WHEN NEW.key = 'seed.plain_serial' BEGIN SELECT RAISE(ABORT, 'cut short'); END`},
		{"the row refused", `CREATE TRIGGER cut BEFORE INSERT ON cubes BEGIN SELECT RAISE(ABORT, 'cut short'); END`},
	} {
		t.Run(c.name, func(t *testing.T) {
			clearCubeEnv(t)
			cubeAddr := startFakeCube(t)
			envFile := filepath.Join(t.TempDir(), "x.env")
			os.WriteFile(envFile, []byte("MASSIMO_CUBE_ADDRESS="+cubeAddr+"\nMASSIMO_CUBE_PROXY="+freeAddr(t)+"\n"), 0o600)
			if _, err := env.LoadFile(envFile); err != nil {
				t.Fatal(err)
			}
			st := testStore(t, "")
			ctx := context.Background()
			other, err := sql.Open("sqlite", "file:"+filepath.ToSlash(st.File())+"?_pragma=busy_timeout(5000)")
			if err != nil {
				t.Fatal(err)
			}
			defer other.Close()
			if _, err := other.Exec(c.trigger); err != nil {
				t.Fatal(err)
			}

			su, err := setupCubes(ctx, st, Config{}, quietLog())
			if err != nil {
				t.Fatal(err)
			}
			logger, logs := testkit.LogWatch(nil)
			srv := server.New(server.Options{Cubes: su.cubes, Proxy: su.listeners, Store: st, FileCubes: su.fileCubes, Logger: logger})
			runCtx, cancel := context.WithCancel(ctx)
			done := make(chan struct{})
			go func() {
				srv.Run(runCtx)
				close(done)
			}()
			func() {
				defer func() { cancel(); <-done }() // the server stops before a failure ends the test, too
				logs.Await(t, 0, "a write refused", testkit.Contains("cut short"), testkit.For(10*time.Second))
			}()
			if _, err := other.Exec(`DROP TRIGGER cut`); err != nil {
				t.Fatal(err)
			}

			su, err = setupCubes(ctx, st, Config{}, quietLog())
			if err != nil || len(su.cubes) != 1 {
				t.Fatalf("the next start: %d cubes, %v", len(su.cubes), err)
			}
		})
	}
}

// nowhere is a Cube address at which nothing answers: the dial is
// refused.
const nowhere = "127.0.0.1:9"

// plainEnv loads a settings file with the plain MASSIMO_CUBE_ADDRESS and
// MASSIMO_CUBE_PROXY, as the README's single-Cube setup has them.
func plainEnv(t *testing.T, cubeAddr, proxyAddr string) {
	t.Helper()
	envFile := filepath.Join(t.TempDir(), "x.env")
	if err := os.WriteFile(envFile, []byte("MASSIMO_CUBE_ADDRESS="+cubeAddr+"\nMASSIMO_CUBE_PROXY="+proxyAddr+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := env.LoadFile(envFile); err != nil {
		t.Fatal(err)
	}
}

// addUICube adds a Cube to the store as the web UI does.
func addUICube(t *testing.T, st *store.Store, serial, addr string) {
	t.Helper()
	if _, err := st.AddCube(context.Background(), store.Cube{Serial: serial, Address: addr, Configured: true, Source: store.SourceUI}); err != nil {
		t.Fatal(err)
	}
}

// runSetup serves what setupCubes returned until the returned stop, or
// the test's end, and returns the API's base URL.
func runSetup(t *testing.T, st *store.Store, su setup, logger *slog.Logger) (base string, stop func()) {
	t.Helper()
	srv := server.New(server.Options{Cubes: su.cubes, Proxy: su.listeners, Store: st, FileCubes: su.fileCubes, Logger: logger})
	ts := httptest.NewServer(srv.Handler())
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { srv.Run(ctx); close(done) }()
	var once bool
	stop = func() {
		if once {
			return
		}
		once = true
		ts.Close()
		cancel()
		<-done
	}
	t.Cleanup(stop)
	return ts.URL, stop
}

// listenWait is how long a test waits for a proxy on freeAddr's port to
// listen: another process may hold the port a while, and the server
// tries again after 1, 2 and 4 s (docs/proxy.md).
var listenWait = testkit.For(20 * time.Second)

// listedProxy is a proxy listener as GET /api/v1/system lists it.
type listedProxy struct {
	Serial    string `json:"serial"`
	Addr      string `json:"addr"`
	Listening bool   `json:"listening"`
}

// proxyAt returns the listener at addr as GET /api/v1/system lists it.
func proxyAt(t *testing.T, base, addr string) (listedProxy, bool) {
	t.Helper()
	var sys struct {
		CubeProxies []listedProxy `json:"cube_proxies"`
	}
	getJSON(t, base+"/api/v1/system", &sys)
	for _, p := range sys.CubeProxies {
		if p.Addr == addr {
			return p, true
		}
	}
	return listedProxy{}, false
}

// The plain MASSIMO_CUBE_ADDRESS and MASSIMO_CUBE_PROXY beside a Cube
// added in the web UI (D24-B): the start goes on, and the plain proxy
// serves the plain Cube at once, tied to it by its address. A client
// that comes before the Cube has reported its serial gets its greeting
// once it has; the server then adds the Cube to the store, and the next
// start ties the plain lines to its row. A -proxy flag without a serial
// is refused as before.
func TestServePlainCubeBesideStored(t *testing.T) {
	clearCubeEnv(t)
	// The plain Cube comes up only once a client waits at its proxy: its
	// port is held, and the server's dial waits for the greeting.
	held, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer held.Close()
	greeting, err := fakecube.LoadGreeting(greetingFile)
	if err != nil {
		t.Fatal(err)
	}
	fc := &fakecube.Server{Greeting: greeting}
	cubeAddr, proxyAddr := held.Addr().String(), freeAddr(t)
	plainEnv(t, cubeAddr, proxyAddr)
	t.Setenv(env.ProxySerial, "LEQ0000100")
	st := testStore(t, "")
	ctx := context.Background()
	addUICube(t, st, "KEQ0000200", nowhere)

	if _, err := setupCubes(ctx, st, Config{Proxies: []string{proxyAddr}}, quietLog()); err == nil || !strings.Contains(err.Error(), "several cubes") {
		t.Fatalf("-proxy without a serial beside two cubes: %v", err)
	}
	su, err := setupCubes(ctx, st, Config{}, quietLog())
	if err != nil {
		t.Fatalf("the start: %v", err)
	}
	if len(su.cubes) != 2 || len(su.listeners) != 1 || su.listeners[0].Addr != proxyAddr || su.listeners[0].Serial != "" || su.listeners[0].CubeAddr != cubeAddr || su.listeners[0].DiscoverySerial != "LEQ0000100" {
		t.Fatalf("cubes %+v listeners %+v", su.cubes, su.listeners)
	}
	base, stop := runSetup(t, st, su, quietLog())
	var p listedProxy
	testkit.Eventually(t, "the plain proxy listening", func() bool {
		var ok bool
		p, ok = proxyAt(t, base, proxyAddr)
		return ok && p.Listening
	}, listenWait)
	if p.Serial != "" {
		t.Fatalf("the plain Cube's serial known before it came up: %+v", p)
	}
	conn, err := net.DialTimeout("tcp", proxyAddr, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	fc.Serve(held)
	defer fc.Close()
	conn.SetReadDeadline(time.Now().Add(testkit.Scale(20 * time.Second)))
	if m, err := cube.NewReader(conn).Read(); err != nil || m.Type != 'H' || !strings.HasPrefix(m.Data, "KEQ0000100,") {
		t.Fatalf("greeting %v %v", m, err)
	}
	testkit.Eventually(t, "the plain proxy under the serial reported", func() bool {
		p, _ = proxyAt(t, base, proxyAddr)
		return p.Serial == "KEQ0000100" && p.Listening
	})
	if !testkit.Within(5*time.Second, func() bool { _, ok, _ := st.Cube(ctx, "KEQ0000100"); return ok }) {
		t.Fatal("the plain Cube not added to the store")
	}
	stop()

	// The next start: the plain lines belong to KEQ0000100's row.
	su, err = setupCubes(ctx, st, Config{}, quietLog())
	if err != nil || len(su.cubes) != 2 || len(su.listeners) != 1 || su.listeners[0].Serial != "KEQ0000100" || su.listeners[0].CubeAddr != "" || su.listeners[0].Addr != proxyAddr || su.listeners[0].DiscoverySerial != "LEQ0000100" {
		t.Fatalf("the next start: cubes %+v listeners %+v %v", su.cubes, su.listeners, err)
	}
}

// listedCube is a Cube as GET /api/v1/cubes lists it.
type listedCube struct {
	Serial         string   `json:"serial"`
	Connected      bool     `json:"connected"`
	ProxyAddress   string   `json:"proxy_address"`
	ProxyListening []string `json:"proxy_listening"`
	FileDiffers    []struct {
		Field, Variable string
		File, Store     any
	} `json:"file_differs"`
}

// differs reports whether the Cube shows the settings file's variable as
// a line that differs from its row, with the file's value.
func (c listedCube) differs(variable, file string) bool {
	for _, d := range c.FileDiffers {
		if d.Variable == variable && d.File == file {
			return true
		}
	}
	return false
}

// cubeListed returns the Cube serial as GET /api/v1/cubes lists it, and
// how many entries have that serial.
func cubeListed(t *testing.T, base, serial string) (listedCube, int) {
	t.Helper()
	var list []listedCube
	getJSON(t, base+"/api/v1/cubes", &list)
	var out listedCube
	n := 0
	for _, c := range list {
		if c.Serial == serial {
			out, n = c, n+1
		}
	}
	return out, n
}

// The plain MASSIMO_CUBE_ADDRESS is a stored Cube's where both name the
// same host and port, whatever the host's case and brackets, the Cube's
// port where one gives none; the plain address alone ties too, another
// port does not. The start then serves the one Cube, and the setting
// names it.
func TestServePlainAddressOfAStoredCube(t *testing.T) {
	ctx := context.Background()
	for _, c := range []struct {
		plain, row string
		tied       bool
	}{
		{"192.0.2.1", "192.0.2.1:62910", true},
		{"192.0.2.1:62910", "192.0.2.1", true},
		{"Cube.example", "cube.example:62910", true},
		{"fe80::1", "[fe80::1]:62910", true},
		{"[fe80::1]", "[FE80::1]:62910", true},
		{"192.0.2.1", "192.0.2.1:62911", false},
		{"192.0.2.1", "192.0.2.10", false},
	} {
		t.Run(c.plain+" and "+c.row, func(t *testing.T) {
			clearCubeEnv(t)
			t.Setenv(env.Address, c.plain)
			st := testStore(t, "")
			addUICube(t, st, "KEQ0000100", c.row)
			su, err := setupCubes(ctx, st, Config{}, quietLog())
			sn, serr := st.PlainSerial(ctx)
			if err != nil || serr != nil {
				t.Fatal(err, serr)
			}
			if c.tied && (len(su.cubes) != 1 || su.cubes[0].Serial != "KEQ0000100" || sn != "KEQ0000100") {
				t.Fatalf("not tied: cubes %+v, the setting %q", su.cubes, sn)
			}
			if !c.tied && (len(su.cubes) != 2 || sn != "") {
				t.Fatalf("tied: cubes %+v, the setting %q", su.cubes, sn)
			}
		})
	}
}

// The README's path where the plain Cube is one the store has: the first
// start did not reach it, and the owner then added it in the web UI at
// the plain address. The start ties the plain lines to it, so that the
// Cube is dialed once: the setting names it, and its row wins over the
// plain lines (docs/store.md, "Seeding and precedence"). The plain
// MASSIMO_CUBE_PROXY, which the row lacks, is a line that differs, and
// nothing listens on it. The next start is the same, without the note.
func TestServePlainCubeAtStoredCube(t *testing.T) {
	clearCubeEnv(t)
	cubeAddr, proxyAddr := startFakeCube(t), freeAddr(t)
	plainEnv(t, cubeAddr, proxyAddr)
	st := testStore(t, "")
	ctx := context.Background()
	addUICube(t, st, "KEQ0000100", cubeAddr)
	addUICube(t, st, "KEQ0000200", nowhere) // so that KEQ0000100 has no listener of the single Cube's
	logger, logs := testkit.LogWatch(slog.LevelWarn)
	su, err := setupCubes(ctx, st, Config{}, logger)
	if err != nil || len(su.cubes) != 2 || su.cubes[0].Serial != "KEQ0000100" || su.cubes[0].SeedFromFile || len(su.listeners) != 0 {
		t.Fatalf("the start: cubes %+v listeners %+v %v", su.cubes, su.listeners, err)
	}
	note := testkit.Msg(server.PlainServedNote, "serial", "KEQ0000100", "addr", cubeAddr)
	logs.Has(t, 0, "the note that the plain lines are the stored Cube's", note)
	logs.Has(t, 0, "the note's word on the row and the way out", testkit.Contains("compared with its row, whose values win", "take the plain lines out of the settings file, or take a value from the file in the web UI"))
	if sn, err := st.PlainSerial(ctx); err != nil || sn != "KEQ0000100" {
		t.Fatalf("the setting: %q %v", sn, err)
	}
	if row, _, _ := st.Cube(ctx, "KEQ0000100"); row.ProxyAddress != "" || row.Source != store.SourceUI || row.Address != cubeAddr {
		t.Fatalf("the row changed: %+v", row)
	}
	base, _ := runSetup(t, st, su, quietLog())
	var c listedCube
	if !testkit.Within(10*time.Second, func() bool {
		c, _ = cubeListed(t, base, "KEQ0000100")
		return c.Connected
	}) {
		t.Fatalf("the stored Cube not served: %+v", c)
	}
	if !c.differs(env.Proxy, proxyAddr) || c.ProxyAddress != "" || len(c.ProxyListening) != 0 {
		t.Fatalf("the plain proxy line: %+v", c)
	}
	if p, ok := proxyAt(t, base, proxyAddr); ok {
		t.Fatalf("a listener on the plain proxy: %+v", p)
	}

	mark := logs.Mark()
	su, err = setupCubes(ctx, st, Config{}, logger)
	if err != nil || len(su.cubes) != 2 || len(su.listeners) != 0 || logs.Count(mark, note) != 0 {
		t.Fatalf("the next start: cubes %+v listeners %+v %v %q", su.cubes, su.listeners, err, logs.Since(mark, nil))
	}
}

// The plain Cube reports a serial the store serves, at another address:
// here a second fake Cube with the same serial. The server stops the
// plain lines' runtime with its listener, so that the Cube is served
// once, and ties the plain lines to the stored Cube: the setting names
// it, its row is as it was, the plain MASSIMO_CUBE_ADDRESS and
// MASSIMO_CUBE_PROXY are lines that differ, and nothing listens on the
// plain proxy, now or at the next start.
func TestServePlainCubeServedAlready(t *testing.T) {
	clearCubeEnv(t)
	plainAddr, storedAddr, proxyAddr := startFakeCube(t), startFakeCube(t), freeAddr(t)
	plainEnv(t, plainAddr, proxyAddr)
	st := testStore(t, "")
	ctx := context.Background()
	addUICube(t, st, "KEQ0000100", storedAddr)
	addUICube(t, st, "KEQ0000200", nowhere)
	su, err := setupCubes(ctx, st, Config{}, quietLog())
	if err != nil || len(su.cubes) != 3 || len(su.listeners) != 1 || su.listeners[0].CubeAddr != plainAddr {
		t.Fatalf("the start: cubes %+v listeners %+v %v", su.cubes, su.listeners, err)
	}
	logger, logs := testkit.LogWatch(nil)
	base, _ := runSetup(t, st, su, logger)
	logs.Await(t, 0, "the note that the plain lines are the stored Cube's", testkit.Msg(server.PlainServedNote, "serial", "KEQ0000100", "addr", plainAddr), listenWait)
	var c listedCube
	var n int
	var ok bool
	if !testkit.Within(20*time.Second, func() bool {
		c, n = cubeListed(t, base, "KEQ0000100")
		_, ok = proxyAt(t, base, proxyAddr)
		return n == 1 && !ok && c.differs(env.Proxy, proxyAddr) && c.differs(env.Address, plainAddr)
	}) {
		t.Fatalf("the plain lines not the stored Cube's: %d listed, %+v, a listener on the plain proxy %v", n, c, ok)
	}
	if c.ProxyAddress != "" || len(c.ProxyListening) != 0 {
		t.Fatalf("the stored Cube's proxy: %+v", c)
	}
	if sn, err := st.PlainSerial(ctx); err != nil || sn != "KEQ0000100" {
		t.Fatalf("the setting: %q %v", sn, err)
	}
	if row, _, _ := st.Cube(ctx, "KEQ0000100"); row.ProxyAddress != "" || row.Address != storedAddr {
		t.Fatalf("the row changed: %+v", row)
	}

	su, err = setupCubes(ctx, st, Config{}, quietLog())
	if err != nil || len(su.cubes) != 2 || su.cubes[0].Addr != storedAddr || len(su.listeners) != 0 {
		t.Fatalf("the next start: cubes %+v listeners %+v %v", su.cubes, su.listeners, err)
	}
}

// The README's path where the first start did not reach the plain Cube:
// no serial is learned, the owner then adds the Cube in the web UI at its
// right address, and the next start goes on (D24-B), where it stopped
// with "several cubes configured" and a service restarted into that.
// The Cube added is served, and the plain proxy listens for the plain
// Cube, which is not reached.
func TestServePlainCubeNotReachedThenAdded(t *testing.T) {
	clearCubeEnv(t)
	cubeAddr, proxyAddr := startFakeCube(t), freeAddr(t)
	plainEnv(t, nowhere, proxyAddr)
	st := testStore(t, "")
	ctx := context.Background()

	su, err := setupCubes(ctx, st, Config{}, quietLog())
	if err != nil || len(su.cubes) != 1 || len(su.listeners) != 1 {
		t.Fatalf("the first start: cubes %+v listeners %+v %v", su.cubes, su.listeners, err)
	}
	base, stop := runSetup(t, st, su, quietLog())
	testkit.Eventually(t, "the plain proxy listening", func() bool { p, ok := proxyAt(t, base, proxyAddr); return ok && p.Listening }, listenWait)
	stop()
	if sn, err := st.PlainSerial(ctx); err != nil || sn != "" {
		t.Fatalf("a serial learned from a Cube not reached: %q %v", sn, err)
	}

	addUICube(t, st, "KEQ0000100", cubeAddr)
	su, err = setupCubes(ctx, st, Config{}, quietLog())
	if err != nil {
		t.Fatalf("the start after the Cube was added: %v", err)
	}
	if len(su.cubes) != 2 || len(su.listeners) != 1 || su.listeners[0].CubeAddr != nowhere {
		t.Fatalf("cubes %+v listeners %+v", su.cubes, su.listeners)
	}
	base, _ = runSetup(t, st, su, quietLog())
	var list []struct {
		Serial    string `json:"serial"`
		Connected bool   `json:"connected"`
	}
	if !testkit.Within(10*time.Second, func() bool {
		list = nil
		getJSON(t, base+"/api/v1/cubes", &list)
		for _, c := range list {
			if c.Serial == "KEQ0000100" && c.Connected {
				return true
			}
		}
		return false
	}) {
		t.Fatalf("the Cube added is not served: %+v", list)
	}
	testkit.Eventually(t, "the plain proxy listening", func() bool { p, ok := proxyAt(t, base, proxyAddr); return ok && p.Listening && p.Serial == "" }, listenWait)
}

// The plain MASSIMO_CUBE_ADDRESS without MASSIMO_CUBE_PROXY beside a
// Cube of the store: each is served without a proxy, and the warning
// names each by its address, the plain one as such.
func TestServePlainCubeWithoutProxyBeside(t *testing.T) {
	clearCubeEnv(t)
	t.Setenv(env.Address, "192.0.2.1")
	st := testStore(t, "")
	addUICube(t, st, "KEQ0000200", "192.0.2.2")
	logger, logs := testkit.LogWatch(slog.LevelWarn)
	su, err := setupCubes(context.Background(), st, Config{}, logger)
	if err != nil || len(su.cubes) != 2 || len(su.listeners) != 0 {
		t.Fatalf("cubes %+v listeners %+v %v", su.cubes, su.listeners, err)
	}
	logs.Has(t, 0, "the plain Cube's warning", testkit.Contains("without a proxy", "plain MASSIMO_CUBE_ADDRESS's cube has no MASSIMO_CUBE_PROXY", "addr=192.0.2.1"))
	logs.Has(t, 0, "the stored Cube's warning", testkit.Contains("without a proxy", "serial=KEQ0000200", "addr=192.0.2.2"))
}

// A plain MASSIMO_CUBE_PROXY without the plain address is as before,
// with no warning (D24-B): serve ignores it beside the store's Cube,
// which listens on the Cube port as the single Cube, and applies it to
// the one Cube that has lines of its own in the file.
func TestServePlainProxyAlone(t *testing.T) {
	clearCubeEnv(t)
	t.Setenv(env.Proxy, "192.0.2.7")
	ctx := context.Background()
	st := testStore(t, "")
	addUICube(t, st, "KEQ0000200", "192.0.2.2")
	logger, logs := testkit.LogWatch(slog.LevelWarn)
	su, err := setupCubes(ctx, st, Config{}, logger)
	if err != nil || len(su.cubes) != 1 || len(su.listeners) != 1 || su.listeners[0].Serial != "KEQ0000200" || su.listeners[0].Addr != ":62910" {
		t.Fatalf("the store's Cube: cubes %+v listeners %+v %v", su.cubes, su.listeners, err)
	}
	if n := logs.Count(0, nil); n != 0 {
		t.Fatalf("warnings: %q", logs.Since(0, nil))
	}

	t.Setenv("MASSIMO_CUBE_KEQ0000001_ADDRESS", "192.0.2.1")
	mark := logs.Mark()
	su, err = setupCubes(ctx, st, Config{}, logger)
	if err != nil || len(su.cubes) != 2 || len(su.listeners) != 1 || su.listeners[0].Serial != "KEQ0000001" || su.listeners[0].Addr != "192.0.2.7:62910" || !su.listeners[0].Discovery {
		t.Fatalf("the file's Cube: cubes %+v listeners %+v %v", su.cubes, su.listeners, err)
	}
	if n := logs.Count(mark, nil); n != 1 || logs.Count(mark, testkit.Contains("without a proxy", "KEQ0000200")) != 1 {
		t.Fatalf("warnings: %q", logs.Since(mark, nil))
	}
}
