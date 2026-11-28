// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"go.metaworx.ch/massimo/internal/serve"
	"go.metaworx.ch/massimo/internal/serverbackup"
	"go.metaworx.ch/massimo/internal/testkit/storekit"
)

// nodeServer answers /healthz with a node, and counts the requests
// that came with a key.
func nodeServer(t *testing.T, node string, keyed *atomic.Int64) *httptest.Server {
	t.Helper()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "" {
			keyed.Add(1)
		}
		json.NewEncoder(w).Encode(map[string]any{"ok": true, "node_id": node})
	}))
	t.Cleanup(ts.Close)
	return ts
}

// The serve of the data directory is the one serve.api names, else the
// one at MASSIMO_LISTEN, and only where its /healthz names the store's
// node: another server gets no key, and the command acts on the store.
func TestFindServe(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	dir := t.TempDir()
	st := storekit.Open(t, dir)
	node := st.NodeID()
	st.Close()
	var keyed atomic.Int64
	own, other := nodeServer(t, node, &keyed), nodeServer(t, "01a21e2c-0000-7000-8000-00000000ffff", &keyed)
	listenOf := func(ts *httptest.Server) string { return strings.TrimPrefix(ts.URL, "http://") }

	if api, at := findServe(ctx, dir, listenOf(other), false, "tok"); api != nil || at != other.URL {
		t.Fatalf("another node's server: %+v %q", api, at)
	}
	if api, _ := findServe(ctx, dir, listenOf(own), false, "tok"); api == nil || api.base != own.URL || api.token != "tok" {
		t.Fatalf("the serve at MASSIMO_LISTEN: %+v", api)
	}
	// serve.api names the serve wherever MASSIMO_LISTEN points.
	b, _ := json.Marshal(serve.APIFile{URL: own.URL, Node: node})
	if err := os.WriteFile(serve.APIFilePath(dir), b, 0o600); err != nil {
		t.Fatal(err)
	}
	if api, _ := findServe(ctx, dir, "127.0.0.1:1", false, "tok"); api == nil || api.base != own.URL {
		t.Fatalf("the serve of serve.api: %+v", api)
	}
	if n := keyed.Load(); n != 0 {
		t.Fatalf("%d requests with a key while finding", n)
	}
	if api, _ := findServe(ctx, t.TempDir(), listenOf(own), false, "tok"); api != nil {
		t.Fatalf("a directory without a store: %+v", api)
	}
}

// A job on the store waits for the jobs' lock another holds, serve's or
// a command's, and is refused with why once its wait is over.
func TestServerBackupCommandsWaitForTheLock(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	dir := t.TempDir()
	storekit.Open(t, dir).Close()
	unlock, err := serverbackup.LockJobs(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	defer unlock()
	var out bytes.Buffer
	c := &backupCLI{dir: dir, out: &out, log: slog.New(slog.DiscardHandler), lockWait: 200 * time.Millisecond}
	if err := c.make(ctx, "", ""); err == nil || !strings.Contains(err.Error(), "another job of the server's backups runs") {
		t.Fatalf("make beside a job: %v", err)
	}
	if err := c.copy(ctx, "01a21e2c-0000-7000-8000-0000000000aa", "nas", false); err == nil {
		t.Fatal("push beside a job")
	}
}
