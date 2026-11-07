// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"go.metaworx.ch/massimo/internal/store"
)

// builtServe is massimo serve run from a binary the test built, with a
// data directory of its own and no settings file, stopped by a signal as
// systemd, launchd and kill stop it.
type builtServe struct {
	cmd  *exec.Cmd
	data string
	addr string // the API's, from the "api listening" line

	mu    sync.Mutex
	lines []string
	added chan struct{} // closed when a line arrives

	exited chan struct{} // closed once the process ended
	err    error         // its Wait's, once exited
}

var logAttr = regexp.MustCompile(`\b(\w+)=("[^"]*"|\S+)`)

// startBuiltServe builds massimo and starts serve with args after its
// own, env added to this process's environment without its MASSIMO_
// variables; it returns once the API listens.
func startBuiltServe(t *testing.T, env []string, args ...string) *builtServe {
	t.Helper()
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
	s := &builtServe{data: t.TempDir(), added: make(chan struct{}), exited: make(chan struct{})}
	s.cmd = exec.Command(bin, append([]string{"-data", s.data, "serve", "-listen", "127.0.0.1:0", "-discover-interval", "0"}, args...)...)
	for _, kv := range os.Environ() {
		if !strings.HasPrefix(kv, "MASSIMO_") {
			s.cmd.Env = append(s.cmd.Env, kv)
		}
	}
	s.cmd.Env = append(append(s.cmd.Env, "MASSIMO_ENV_FILE="+os.DevNull), env...)
	stderr, err := s.cmd.StderrPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := s.cmd.Start(); err != nil {
		t.Fatal(err)
	}
	go func() {
		sc := bufio.NewScanner(stderr)
		for sc.Scan() {
			s.mu.Lock()
			s.lines = append(s.lines, sc.Text())
			close(s.added)
			s.added = make(chan struct{})
			s.mu.Unlock()
		}
		s.err = s.cmd.Wait()
		close(s.exited)
	}()
	t.Cleanup(func() {
		s.cmd.Process.Kill()
		<-s.exited
		if t.Failed() {
			t.Logf("serve's log:\n%s", strings.Join(s.log(), "\n"))
		}
	})
	s.addr = s.waitFor(t, "api listening")["addr"]
	return s
}

func (s *builtServe) log() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.lines...)
}

// waitFor waits for a log line with msg and returns its attributes.
func (s *builtServe) waitFor(t *testing.T, msg string) map[string]string {
	t.Helper()
	deadline := time.After(10 * time.Second)
	for seen := 0; ; {
		s.mu.Lock()
		lines, added := s.lines[seen:], s.added
		seen = len(s.lines)
		s.mu.Unlock()
		for _, l := range lines {
			if strings.Contains(l, `msg="`+msg+`"`) || strings.Contains(l, "msg="+msg+" ") {
				attrs := map[string]string{}
				for _, m := range logAttr.FindAllStringSubmatch(l, -1) {
					attrs[m[1]] = strings.Trim(m[2], `"`)
				}
				return attrs
			}
		}
		select {
		case <-added:
		case <-s.exited:
			t.Fatalf("serve ended before logging %q: %v", msg, s.err)
		case <-deadline:
			t.Fatalf("serve did not log %q", msg)
		}
	}
}

// stop sends SIGTERM and waits for the process, which must end without
// an error; it returns how long that took.
func (s *builtServe) stop(t *testing.T) time.Duration {
	t.Helper()
	start := time.Now()
	s.signal(t)
	return s.wait(t, start)
}

func (s *builtServe) signal(t *testing.T) {
	t.Helper()
	if err := s.cmd.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
}

func (s *builtServe) wait(t *testing.T, since time.Time) time.Duration {
	t.Helper()
	select {
	case <-s.exited:
	case <-time.After(20 * time.Second):
		t.Fatal("serve did not end on SIGTERM")
	}
	if s.err != nil {
		t.Fatalf("serve ended on SIGTERM with %v", s.err)
	}
	return time.Since(since)
}

// serve stops on SIGTERM, which systemd, launchd and kill send, as it
// does on SIGINT: the server shuts down and the store is closed, which
// SQLite shows by removing its -wal file at the last close, and the log
// says so.
func TestServeStopsOnSIGTERM(t *testing.T) {
	s := startBuiltServe(t, nil)
	if _, err := os.Stat(filepath.Join(s.data, "massimo.db-wal")); err != nil {
		t.Fatalf("no -wal file while serve runs: %v", err)
	}
	s.stop(t)
	if _, err := os.Stat(filepath.Join(s.data, "massimo.db-wal")); err == nil {
		t.Fatal("the store was not closed: its -wal file is left")
	}
	if p := s.waitFor(t, "store closed")["path"]; p != store.Path(s.data) {
		t.Errorf("store closed names %s, want %s", p, store.Path(s.data))
	}
}

// A request still being answered when serve is stopped gets its answer,
// and its audit row and its command are in the store after the exit:
// the stop waits for the HTTP server's shutdown before the store closes.
// The process used to end as soon as the listener closed.
func TestServeAnswersRequestInFlightAtStop(t *testing.T) {
	s := startBuiltServe(t, nil, "-token", "test-token")
	conn, err := net.Dial("tcp", s.addr)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	body := `{"stale_after": "36h"}`
	fmt.Fprintf(conn, "PUT /api/v1/settings HTTP/1.1\r\nHost: %s\r\nAuthorization: Bearer test-token\r\nContent-Type: application/json\r\nContent-Length: %d\r\n\r\n%s",
		s.addr, len(body), body[:8])
	// The server reads the header and waits for the rest of the body
	// while it is stopped.
	time.Sleep(300 * time.Millisecond)
	start := time.Now()
	s.signal(t)
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
	s.wait(t, start)

	st, err := store.OpenReadOnly(s.data, store.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	ctx := context.Background()
	if _, ok, err := st.Setting(ctx, store.StaleAfterKey); err != nil || !ok {
		t.Errorf("the request's setting is not in the store: %v", err)
	}
	rows, err := st.AuditList(ctx, time.Time{}, 10, "")
	if err != nil {
		t.Fatal(err)
	}
	var row *store.AuditEntry
	for i := range rows {
		if rows[i].Action == "settings.update" {
			row = &rows[i]
		}
	}
	if row == nil {
		t.Fatalf("no audit row of the request in flight: %+v", rows)
	}
	if row.Result != store.ResultOK {
		t.Errorf("its audit row says %s", row.Result)
	}
	c, ok, err := st.Command(ctx, row.CommandID)
	if err != nil || !ok {
		t.Fatalf("its command %d: %v %v", row.CommandID, ok, err)
	}
	if c.State != store.StateAcknowledged {
		t.Errorf("its command is %s, want %s", c.State, store.StateAcknowledged)
	}
}

// An event stream open when serve is stopped ends with the server's run,
// so that the stop does not wait out the HTTP server's grace for it,
// and its client sees the end of the stream rather than a cut.
func TestServeStopEndsEventStreams(t *testing.T) {
	s := startBuiltServe(t, []string{"MASSIMO_CUBE_ADDRESS=" + startFakeCube(t)}, "-token", "test-token", "-proxy", "off")
	sn := s.waitFor(t, "cube added from the settings file")["serial"]
	req, err := http.NewRequest(http.MethodGet, "http://"+s.addr+"/api/v1/cubes/"+sn+"/events", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer test-token")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("the stream answered %s", resp.Status)
	}
	br := bufio.NewReader(resp.Body)
	if _, err := br.ReadString('\n'); err != nil {
		t.Fatalf("no first event: %v", err)
	}
	read := make(chan error, 1)
	go func() {
		_, err := io.Copy(io.Discard, br)
		read <- err
	}()
	if took := s.stop(t); took > 3*time.Second {
		t.Errorf("the stop took %s with an event stream open, want it well within the HTTP server's 5 s grace", took.Round(time.Millisecond))
	}
	select {
	case err := <-read:
		if err != nil {
			t.Errorf("the stream was cut rather than ended: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Error("the stream did not end")
	}
}
