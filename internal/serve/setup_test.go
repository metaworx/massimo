// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package serve

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"go.metaworx.ch/massimo/internal/cubelan/fakecube"
	"go.metaworx.ch/massimo/internal/env"
	"go.metaworx.ch/massimo/internal/protocol/cube"
	"go.metaworx.ch/massimo/internal/server"
	"go.metaworx.ch/massimo/internal/store"
)

// clearCubeEnv unsets every MASSIMO_CUBE_* variable for the test, and
// points the settings file and the data directory away from the
// checkout's.
func clearCubeEnv(t *testing.T) {
	t.Helper()
	for _, kv := range os.Environ() {
		if k, _, _ := strings.Cut(kv, "="); strings.HasPrefix(k, env.Prefix) {
			t.Setenv(k, "")
			os.Unsetenv(k)
		}
	}
	t.Setenv(env.FileVar, os.DevNull)
	t.Setenv(env.DataVar, t.TempDir())
}

func quietLog() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

// testStore opens a store in a temporary directory.
func testStore(t *testing.T, dir string) *store.Store {
	t.Helper()
	if dir == "" {
		dir = t.TempDir()
	}
	st, err := store.Open(dir, store.Options{Version: "test", Logger: quietLog()})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	return st
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

func freeAddr(t *testing.T) string {
	t.Helper()
	free, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer free.Close()
	return free.Addr().String()
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
	if err := env.LoadFile(envFile); err != nil {
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
	var conn net.Conn
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
		if conn, err = net.Dial("tcp", proxyAddr); err == nil {
			break
		}
	}
	if conn == nil {
		t.Fatal("proxy never listened")
	}
	defer conn.Close()
	conn.SetReadDeadline(time.Now().Add(20 * time.Second))
	m, err := cube.NewReader(conn).Read()
	if err != nil || m.Type != 'H' || !strings.HasPrefix(m.Data, "KEQ0000100,") {
		t.Fatalf("greeting %v %v", m, err)
	}
	var row store.Cube
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
		var ok bool
		if row, ok, _ = st.Cube(ctx, "KEQ0000100"); ok {
			break
		}
	}
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
	var logged bytes.Buffer
	su, err := setupCubes(ctx, testStore(t, ""), Config{}, slog.New(slog.NewTextHandler(&logged, nil)))
	if err != nil || len(su.cubes) != 2 || len(su.listeners) != 0 || strings.Count(logged.String(), "without a proxy") != 2 {
		t.Fatalf("two cubes without proxy addresses: %+v %v %s", su, err, logged.String())
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
	}); err != nil {
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
	clearCubeEnv(t)
	var logged bytes.Buffer
	su, err := setupCubes(context.Background(), testStore(t, ""), Config{}, slog.New(slog.NewTextHandler(&logged, nil)))
	if err != nil || len(su.cubes) != 0 || len(su.listeners) != 0 || !strings.Contains(logged.String(), "no cube configured yet") {
		t.Fatalf("%+v %v %s", su, err, logged.String())
	}
}

// serve -addr runs without the store: no massimo.db, no node.key.
func TestServeAddrOpensNoStore(t *testing.T) {
	clearCubeEnv(t)
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
			if err := env.LoadFile(envFile); err != nil {
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
			var log syncLog
			srv := server.New(server.Options{Cubes: su.cubes, Proxy: su.listeners, Store: st, FileCubes: su.fileCubes, Logger: slog.New(slog.NewTextHandler(&log, nil))})
			runCtx, cancel := context.WithCancel(ctx)
			done := make(chan struct{})
			go func() {
				srv.Run(runCtx)
				close(done)
			}()
			for deadline := time.Now().Add(10 * time.Second); !strings.Contains(log.String(), "cut short"); time.Sleep(20 * time.Millisecond) {
				if time.Now().After(deadline) {
					cancel()
					<-done
					t.Fatalf("no write refused:\n%s", log.String())
				}
			}
			cancel()
			<-done
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

// syncLog collects a log written from several goroutines.
type syncLog struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (l *syncLog) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.Write(p)
}

func (l *syncLog) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.String()
}
