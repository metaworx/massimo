// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package main

import (
	"bufio"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"
)

// serve stops on SIGTERM, which systemd, launchd and kill send, as it
// does on SIGINT: the server shuts down and the store is closed, which
// SQLite shows by removing its -wal file at the last close.
func TestServeStopsOnSIGTERM(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("no SIGTERM to send")
	}
	if testing.Short() {
		t.Skip("builds the binary")
	}
	bin := filepath.Join(t.TempDir(), "massimo")
	if out, err := exec.Command("go", "build", "-o", bin, ".").CombinedOutput(); err != nil {
		t.Fatalf("go build: %v\n%s", err, out)
	}
	data := t.TempDir()
	cmd := exec.Command(bin, "-data", data, "serve", "-listen", "127.0.0.1:0", "-discover-interval", "0")
	for _, kv := range os.Environ() {
		if !strings.HasPrefix(kv, "MASSIMO_") {
			cmd.Env = append(cmd.Env, kv)
		}
	}
	cmd.Env = append(cmd.Env, "MASSIMO_ENV_FILE="+os.DevNull)
	stderr, err := cmd.StderrPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer cmd.Process.Kill()
	listening := make(chan struct{})
	go func() {
		sc := bufio.NewScanner(stderr)
		for sc.Scan() {
			if strings.Contains(sc.Text(), "api listening") {
				close(listening)
			}
		}
	}()
	select {
	case <-listening:
	case <-time.After(10 * time.Second):
		t.Fatal("serve did not start")
	}
	if _, err := os.Stat(filepath.Join(data, "massimo.db-wal")); err != nil {
		t.Fatalf("no -wal file while serve runs: %v", err)
	}
	cmd.Process.Signal(syscall.SIGTERM)
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("serve ended on SIGTERM with %v", err)
		}
	case <-time.After(20 * time.Second):
		t.Fatal("serve did not end on SIGTERM")
	}
	if _, err := os.Stat(filepath.Join(data, "massimo.db-wal")); err == nil {
		t.Fatal("the store was not closed: its -wal file is left")
	}
}
