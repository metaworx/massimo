// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package serve

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"go.metaworx.ch/massimo/internal/announce"
	"go.metaworx.ch/massimo/internal/store"
)

// getJSON decodes the answer of a GET into v.
func getJSON(t *testing.T, url string, v any) {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET %s: %s", url, resp.Status)
	}
	if err := json.NewDecoder(resp.Body).Decode(v); err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
}

// runInstance runs inst until the test cancels the returned function,
// which waits for Run to return and gives its error.
func runInstance(t *testing.T, inst *Instance) (stop func() error) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- inst.Run(ctx) }()
	t.Cleanup(cancel)
	return func() error {
		cancel()
		select {
		case err := <-done:
			return err
		case <-time.After(20 * time.Second):
			t.Fatal("Run did not return")
			return nil
		}
	}
}

// waitForCube waits until GET /api/v1/cubes lists the fake Cube's serial
// as connected.
func waitForCube(t *testing.T, base string) {
	t.Helper()
	for deadline := time.Now().Add(10 * time.Second); ; time.Sleep(20 * time.Millisecond) {
		var list []struct {
			Serial    string `json:"serial"`
			Connected bool   `json:"connected"`
		}
		getJSON(t, base+"/api/v1/cubes", &list)
		if len(list) == 1 && list[0].Serial == "KEQ0000100" && list[0].Connected {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("the fake Cube never connected: %+v", list)
		}
	}
}

// never is an interval of 0: no refresh, discovery only on request.
var never = time.Duration(0)

// The call a program embedding the server makes: Open over a data
// directory of its own and Run until the context ends. The Cube is
// served, /healthz names the node, and once Run returns the store is
// closed and the data directory holds massimo.db.
func TestOpenRunsAgainstFakeCube(t *testing.T) {
	clearCubeEnv(t)
	t.Setenv("MASSIMO_CUBE_KEQ0000100_ADDRESS", startFakeCube(t))
	data := t.TempDir()
	var log syncLog
	inst, err := Open(context.Background(), Config{Listen: "127.0.0.1:0", DataDir: data, Version: "massimo test", Proxies: []string{"off"},
		RefreshInterval: &never, DiscoverInterval: &never, Logger: slog.New(slog.NewTextHandler(&log, nil))})
	if err != nil {
		t.Fatal(err)
	}
	if _, port, _ := net.SplitHostPort(inst.Addr()); port == "" || port == "0" {
		t.Fatalf("Addr %q: no port taken", inst.Addr())
	}
	stop := runInstance(t, inst)
	base := "http://" + inst.Addr()
	var h struct {
		OK      bool   `json:"ok"`
		Version string `json:"version"`
		NodeID  string `json:"node_id"`
	}
	getJSON(t, base+"/healthz", &h)
	if !h.OK || h.Version != "massimo test" || h.NodeID == "" || h.NodeID != inst.Store().NodeID() {
		t.Fatalf("healthz %+v", h)
	}
	waitForCube(t, base)
	if err := stop(); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`msg="serving cube" serial=KEQ0000100`, `msg="api listening" addr=` + inst.Addr() + ` auth=open restart=none reboot=none tls=off data=` + data, `msg="store closed"`} {
		if !strings.Contains(log.String(), want) {
			t.Errorf("the log lacks %q:\n%s", want, log.String())
		}
	}
	if _, err := os.Stat(filepath.Join(data, store.FileName)); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(data, store.FileName+"-wal")); err == nil {
		t.Fatal("the store was not closed: its -wal file is left")
	}
}

// With Config.Addr the instance serves that one Cube without a store,
// and writes nothing into the data directory.
func TestOpenWithoutStore(t *testing.T) {
	clearCubeEnv(t)
	data := t.TempDir()
	inst, err := Open(context.Background(), Config{Listen: "127.0.0.1:0", Addr: startFakeCube(t), DataDir: data, Proxies: []string{"off"},
		RefreshInterval: &never, DiscoverInterval: &never, Logger: quietLog()})
	if err != nil {
		t.Fatal(err)
	}
	if inst.Store() != nil {
		t.Fatal("a store with Config.Addr")
	}
	stop := runInstance(t, inst)
	base := "http://" + inst.Addr()
	var h map[string]any
	getJSON(t, base+"/healthz", &h)
	if _, ok := h["node_id"]; ok || h["ok"] != true {
		t.Fatalf("healthz without a store %v", h)
	}
	waitForCube(t, base)
	if err := stop(); err != nil {
		t.Fatal(err)
	}
	if entries, _ := os.ReadDir(data); len(entries) != 0 {
		t.Fatalf("written into the data directory: %v", entries)
	}
}

// An instance that is not run is released by Close: the listener and
// the store; a second Close does nothing.
func TestCloseWithoutRun(t *testing.T) {
	clearCubeEnv(t)
	data := t.TempDir()
	inst, err := Open(context.Background(), Config{Listen: "127.0.0.1:0", DataDir: data, Logger: quietLog()})
	if err != nil {
		t.Fatal(err)
	}
	addr := inst.Addr()
	for range 2 {
		if err := inst.Close(); err != nil {
			t.Fatal(err)
		}
	}
	if c, err := net.Dial("tcp", addr); err == nil {
		c.Close()
		t.Fatal("still listening after Close")
	}
	if _, err := os.Stat(filepath.Join(data, store.FileName+"-wal")); err == nil {
		t.Fatal("the store was not closed: its -wal file is left")
	}
}

// An interval given wins over the store's setting, 0 included; one not
// given takes the store's, else the default.
func TestOpenIntervals(t *testing.T) {
	clearCubeEnv(t)
	data := t.TempDir()
	st := testStore(t, data)
	ctx := context.Background()
	for k, v := range map[string]string{store.PollIntervalKey: "5m", store.RefreshIntervalKey: "12h"} {
		if err := st.SetSetting(ctx, k, v); err != nil {
			t.Fatal(err)
		}
	}
	st.Close()
	inst, err := Open(ctx, Config{Listen: "127.0.0.1:0", DataDir: data, RefreshInterval: &never, Logger: quietLog()})
	if err != nil {
		t.Fatal(err)
	}
	defer inst.Close()
	ts := httptest.NewServer(inst.Server().Handler())
	defer ts.Close()
	var settings map[string]struct {
		InEffect string `json:"in_effect"`
	}
	getJSON(t, ts.URL+"/api/v1/settings", &settings)
	for k, want := range map[string]string{store.PollIntervalKey: "5m", store.RefreshIntervalKey: "0", store.DiscoverIntervalKey: "30m"} {
		if got := settings[k].InEffect; got != want {
			t.Errorf("%s in effect %q, want %q", k, got, want)
		}
	}
}

// fakeAnnounce replaces the responder for the test: it records the
// service, and fails with err where that is set.
func fakeAnnounce(t *testing.T, err error) (got chan announce.Service, stopped chan struct{}) {
	t.Helper()
	got, stopped = make(chan announce.Service, 1), make(chan struct{})
	old := startAnnounce
	startAnnounce = func(ctx context.Context, svc announce.Service, log *slog.Logger) (func(), error) {
		got <- svc
		if err != nil {
			return nil, err
		}
		return func() { <-ctx.Done(); close(stopped) }, nil
	}
	t.Cleanup(func() { startAnnounce = old })
	return got, stopped
}

// With Config.Announce on, Run announces the server once it listens,
// with its port, node, version and name, and waits for the goodbye when
// it stops.
func TestRunAnnounces(t *testing.T) {
	clearCubeEnv(t)
	got, stopped := fakeAnnounce(t, nil)
	var log syncLog
	inst, err := Open(context.Background(), Config{Listen: "127.0.0.1:0", DataDir: t.TempDir(), Proxies: []string{"off"}, DiscoverInterval: &never,
		Announce: Announce{On: true, Name: "box.example.lan", Version: "v0.12.0"}, Logger: slog.New(slog.NewTextHandler(&log, nil))})
	if err != nil {
		t.Fatal(err)
	}
	stop := runInstance(t, inst)
	svc := <-got
	_, port, _ := net.SplitHostPort(inst.Addr())
	if svc.Instance != "box.example.lan" || fmt.Sprint(svc.Port) != port || svc.Node != inst.Store().NodeID() || svc.Version != "v0.12.0" || svc.Fingerprint != nil {
		t.Fatalf("service %+v", svc)
	}
	if err := stop(); err != nil {
		t.Fatal(err)
	}
	select {
	case <-stopped:
	default:
		t.Fatal("Run returned before the announcement stopped")
	}
	if !strings.Contains(log.String(), `msg="announced on the LAN" service=_massimo._tcp name=box port=`+port) {
		t.Fatalf("log:\n%s", log.String())
	}
}

// A responder that cannot start is a warning, and the server serves
// unannounced; without a store nothing is announced.
func TestRunUnannouncedWarns(t *testing.T) {
	clearCubeEnv(t)
	got, _ := fakeAnnounce(t, errors.New("no interface joined the mDNS group"))
	var log syncLog
	inst, err := Open(context.Background(), Config{Listen: "127.0.0.1:0", DataDir: t.TempDir(), Proxies: []string{"off"}, DiscoverInterval: &never,
		Announce: Announce{On: true}, Logger: slog.New(slog.NewTextHandler(&log, nil))})
	if err != nil {
		t.Fatal(err)
	}
	stop := runInstance(t, inst)
	<-got
	var h map[string]any
	getJSON(t, "http://"+inst.Addr()+"/healthz", &h)
	if err := stop(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(log.String(), `level=WARN msg="not announced on the LAN" error="no interface joined the mDNS group"`) {
		t.Fatalf("log:\n%s", log.String())
	}

	log = syncLog{}
	inst, err = Open(context.Background(), Config{Listen: "127.0.0.1:0", Addr: "127.0.0.1:1", Proxies: []string{"off"}, DiscoverInterval: &never,
		Announce: Announce{On: true}, Logger: slog.New(slog.NewTextHandler(&log, nil))})
	if err != nil {
		t.Fatal(err)
	}
	if err := runInstance(t, inst)(); err != nil {
		t.Fatal(err)
	}
	select {
	case svc := <-got:
		t.Fatalf("announced without a store: %+v", svc)
	default:
	}
	if !strings.Contains(log.String(), `msg="not announced on the LAN" reason="no store (-addr), so no node"`) {
		t.Fatalf("log:\n%s", log.String())
	}
}
