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
	"runtime"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"go.metaworx.ch/massimo/internal/store"
	"go.metaworx.ch/massimo/internal/testkit"
)

// builtServe is massimo serve run from the binary the tests built
// (builtBinary, runBuilt), stopped by a signal as systemd, launchd and
// kill stop it; launchBuiltServe's has a data directory of its own and
// no settings file.
type builtServe struct {
	cmd  *exec.Cmd
	data string
	addr string // the API's, from the "api listening" line

	// log is the watch of serve's log on stderr, a record a line, ended
	// with the exit status once the process ended. A wait for a line the
	// test causes marks it before the action; one of the start's lines
	// waits from 0, the start being the action.
	log *testkit.Watch[testkit.Record]

	exited chan struct{} // closed once the process ended
	err    error         // its Wait's, once exited
}

// serveWait is how long a test waits for a line of serve's, before the
// scale: a binary that starts beside the other tests' builds.
var serveWait = testkit.For(10 * time.Second)

// built is massimo built once for the tests that run it as a binary of
// their own (builtBinary), in a directory TestMain removes after the run.
var built struct {
	once     sync.Once
	dir, bin string
	out      []byte // go build's output
	err      error
}

// builtBinary returns massimo built from this package, once for every
// test that runs it: the tests run the one binary side by side, each
// with a data directory and an API port of its own.
func builtBinary(t *testing.T) string {
	t.Helper()
	built.once.Do(func() {
		dir, err := os.MkdirTemp("", "massimo-test-bin-")
		if err != nil {
			built.err = fmt.Errorf("a directory for the binary: %w", err)
			return
		}
		built.dir, built.bin = dir, filepath.Join(dir, "massimo")
		if built.out, err = exec.Command("go", "build", "-o", built.bin, ".").CombinedOutput(); err != nil {
			built.err = fmt.Errorf("go build: %w", err)
		}
	})
	if built.err != nil {
		t.Fatalf("%v\n%s", built.err, built.out)
	}
	return built.bin
}

// startBuiltServe starts serve from the built massimo with args after its
// own, env added to this process's environment without its MASSIMO_
// variables; it returns once the API listens.
func startBuiltServe(t *testing.T, env []string, args ...string) *builtServe {
	t.Helper()
	s := launchBuiltServe(t, env, args...)
	r, _ := s.log.Await(t, 0, "api listening", testkit.Msg("api listening"), serveWait)
	s.addr = r.Attrs["addr"]
	return s
}

// launchBuiltServe is startBuiltServe without the wait: it returns once
// serve is started, for a test of its start.
func launchBuiltServe(t *testing.T, env []string, args ...string) *builtServe {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("no SIGTERM to send")
	}
	if testing.Short() {
		t.Skip("builds the binary")
	}
	data := t.TempDir()
	cmd := exec.Command(builtBinary(t), append([]string{"-data", data, "serve", "-listen", "127.0.0.1:0", "-discover-interval", "0"}, args...)...)
	for _, kv := range os.Environ() {
		if !strings.HasPrefix(kv, "MASSIMO_") {
			cmd.Env = append(cmd.Env, kv)
		}
	}
	cmd.Env = append(append(cmd.Env, "MASSIMO_ENV_FILE="+os.DevNull), env...)
	s := runBuilt(t, cmd)
	s.data = data
	return s
}

// runBuilt starts cmd, a massimo the test built, with its stderr read
// into the watch of its log, which ends with the exit status once the
// process ended. The test's end kills the process, and shows its log
// where the test failed.
func runBuilt(t *testing.T, cmd *exec.Cmd) *builtServe {
	t.Helper()
	s := &builtServe{cmd: cmd, log: testkit.NewWatch[testkit.Record](nil), exited: make(chan struct{})}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	go func() {
		// Every line is read before Wait, which closes the pipe.
		testkit.ReadLog(s.log, stderr)
		s.err = cmd.Wait()
		if s.err != nil {
			s.log.End(fmt.Errorf("serve ended: %w", s.err))
		} else {
			s.log.End(nil)
		}
		close(s.exited)
	}()
	t.Cleanup(func() {
		cmd.Process.Kill()
		<-s.exited
		if t.Failed() {
			t.Logf("serve's log:\n%s", s.logText())
		}
	})
	return s
}

// logText is serve's log so far, its lines as serve wrote them, for a
// failure.
func (s *builtServe) logText() string {
	var b strings.Builder
	for _, r := range s.log.Since(0, nil) {
		b.WriteString(r.Line + "\n")
	}
	return b.String()
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
	case <-time.After(testkit.Scale(20 * time.Second)):
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
	t.Parallel()
	s := startBuiltServe(t, nil)
	if _, err := os.Stat(filepath.Join(s.data, "massimo.db-wal")); err != nil {
		t.Fatalf("no -wal file while serve runs: %v", err)
	}
	m := s.log.Mark()
	s.stop(t)
	if _, err := os.Stat(filepath.Join(s.data, "massimo.db-wal")); err == nil {
		t.Fatal("the store was not closed: its -wal file is left")
	}
	if r, _ := s.log.Await(t, m, "store closed", testkit.Msg("store closed"), serveWait); r.Attrs["path"] != store.Path(s.data) {
		t.Errorf("store closed names %s, want %s", r.Attrs["path"], store.Path(s.data))
	}
}

// A request still being answered when serve is stopped gets its answer,
// and its audit row and its command are in the store after the exit:
// the stop waits for the HTTP server's shutdown before the store closes.
// The process used to end as soon as the listener closed.
func TestServeAnswersRequestInFlightAtStop(t *testing.T) {
	t.Parallel()
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
	conn.SetReadDeadline(time.Now().Add(testkit.Scale(10 * time.Second)))
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
	t.Parallel()
	s := startBuiltServe(t, []string{"MASSIMO_CUBE_ADDRESS=" + startFakeCube(t)}, "-token", "test-token", "-proxy", "off")
	r, _ := s.log.Await(t, 0, "cube added from the settings file", testkit.Msg("cube added from the settings file"), serveWait)
	sn := r.Attrs["serial"]
	req, err := http.NewRequest(http.MethodGet, "http://"+s.addr+"/api/v1/cubes/"+sn+"/events", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer test-token")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		t.Fatalf("the stream answered %s", resp.Status)
	}
	events := testkit.StreamWatch(t, resp.Body)
	events.Await(t, 0, "a first event", nil)
	if took := s.stop(t); took > 3*time.Second {
		t.Errorf("the stop took %s with an event stream open, want it well within the HTTP server's 5 s grace", took.Round(time.Millisecond))
	}
	testkit.Eventually(t, "the stream's end", events.Ended)
	if err := events.Err(); err != nil {
		t.Errorf("the stream was cut rather than ended: %v", err)
	}
}
