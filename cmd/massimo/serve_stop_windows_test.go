// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

//go:build windows

package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"os"
	"testing"
	"time"

	"go.metaworx.ch/massimo/internal/cubelan/fakecube"
	"go.metaworx.ch/massimo/internal/env"
	"go.metaworx.ch/massimo/internal/store"
)

// The service manager's stop ends serve as SIGTERM does on Unix
// (serve_stop_test.go): a request still being answered gets its answer
// and its audit row, the event of the moment before the stop is in the
// store, and the store is closed, its -wal file gone. serve runs in the
// test's process, stopped through theService as the manager's stop does
// (stopServe), with a fake Cube, a data directory and a %ProgramData% of
// its own.
func TestServiceStopClosesTheStore(t *testing.T) {
	clearCubeEnv(t)
	tempServiceLog(t)
	data := os.Getenv(env.DataVar)
	defer func(old *serviceRun) { theService = old }(theService)
	theService = newServiceRun()

	greeting, err := fakecube.LoadGreeting("../../internal/protocol/cube/testdata/cube-fw0113-greeting.txt")
	if err != nil {
		t.Fatal(err)
	}
	fc := &fakecube.Server{Greeting: greeting}
	cubeAddr, err := fc.Start("127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer fc.Close()
	t.Setenv(env.Address, cubeAddr)
	addr := freeAddr(t)
	served := make(chan error, 1)
	go func() {
		served <- runServe([]string{"-listen", addr, "-discover-interval", "0", "-proxy", "off", "-token", "test-token"})
	}()

	get := func(path string, out any) error {
		req, err := http.NewRequest(http.MethodGet, "http://"+addr+path, nil)
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer test-token")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("%s: %s", path, resp.Status)
		}
		return json.NewDecoder(resp.Body).Decode(out)
	}
	// Up once the Cube has its row and its first event is in the store,
	// which its id shows.
	var sn string
	for deadline := time.Now().Add(15 * time.Second); ; time.Sleep(50 * time.Millisecond) {
		select {
		case err := <-served:
			t.Fatalf("serve ended before the stop: %v", err)
		default:
		}
		if time.Now().After(deadline) {
			t.Fatal("serve did not store the fake Cube's first event")
		}
		var cubes []struct {
			Serial string `json:"serial"`
		}
		if get("/api/v1/cubes", &cubes) != nil || len(cubes) != 1 || cubes[0].Serial == "" {
			continue
		}
		var events []struct {
			ID int64 `json:"id"`
		}
		if get("/api/v1/cubes/"+cubes[0].Serial+"/log", &events) == nil && len(events) > 0 && events[0].ID != 0 {
			sn = cubes[0].Serial
			break
		}
	}

	// The Cube goes away just before the stop: its disconnect is the last
	// event, still waiting for the writer's batch when the stop comes.
	fc.Close()
	time.Sleep(100 * time.Millisecond)
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	body := `{"stale_after": "36h"}`
	fmt.Fprintf(conn, "PUT /api/v1/settings HTTP/1.1\r\nHost: %s\r\nAuthorization: Bearer test-token\r\nContent-Type: application/json\r\nContent-Length: %d\r\n\r\n%s",
		addr, len(body), body[:8])
	time.Sleep(300 * time.Millisecond)
	close(theService.stop) // what the manager's stop does
	time.Sleep(500 * time.Millisecond)
	io.WriteString(conn, body[8:])
	conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	resp, err := http.ReadResponse(bufio.NewReader(conn), nil)
	if err != nil {
		t.Fatalf("no answer to the request in flight at the stop: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("the request in flight was answered %s", resp.Status)
	}
	select {
	case err := <-served:
		if err != nil {
			t.Fatalf("serve ended on the stop with %v", err)
		}
	case <-time.After(stopWait):
		t.Fatalf("serve did not end within the %s the manager is told", stopWait)
	}

	if _, err := os.Stat(store.Path(data) + "-wal"); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("the store was not closed: its -wal file is left (%v)", err)
	}
	st, err := store.OpenReadOnly(data, store.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	ctx := context.Background()
	cubes, err := st.Cubes(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var cubeID int64
	for _, c := range cubes {
		if c.Serial == sn {
			cubeID = c.ID
		}
	}
	if events, err := st.Events(ctx, store.EventQuery{CubeID: cubeID, Kind: "disconnected", Limit: 10}); err != nil || len(events) == 0 {
		t.Errorf("the disconnect before the stop is not in the store: %d events, %v", len(events), err)
	}
	rows, err := st.AuditList(ctx, time.Time{}, 10, "")
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, r := range rows {
		found = found || r.Action == "settings.update" && r.Result == store.ResultOK
	}
	if !found {
		t.Errorf("no audit row of the request in flight: %+v", rows)
	}
}
