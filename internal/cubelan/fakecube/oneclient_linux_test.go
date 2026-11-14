// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package fakecube

import (
	"errors"
	"net"
	"testing"
	"time"
)

// On Linux a refused client's dial times out, as against a Cube that
// holds another session (docs/cube-protocol.md, 2026-11-13), and a dial
// that waits while the slot frees connects within its timeout.
func TestSingleClientDialTimesOutOnLinux(t *testing.T) {
	_, addr := startSingle(t, 0)
	first := dial(t, addr)
	start := time.Now()
	_, err := net.DialTimeout("tcp", addr, 400*time.Millisecond)
	var ne net.Error
	if !errors.As(err, &ne) || !ne.Timeout() {
		t.Fatalf("second dial: %v after %v, want a timeout", err, time.Since(start))
	}

	go func() {
		time.Sleep(300 * time.Millisecond)
		first.conn.Close()
	}()
	// The kernel sends the SYN again after a second; the slot is free by
	// then.
	conn, err := net.DialTimeout("tcp", addr, 3*time.Second)
	if err != nil {
		t.Fatalf("dial while the slot frees: %v", err)
	}
	conn.Close()
}
