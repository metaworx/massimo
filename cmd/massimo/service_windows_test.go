// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

//go:build windows

package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/windows/svc"
)

// The service manager's stop ends serve's context, and the handler
// reports the service stopped only once serve returned, which writes the
// log's pending events and closes the store; the process used to end at
// once.
func TestServiceStopWaitsForServe(t *testing.T) {
	logPath := tempServiceLog(t)
	defer func(old *serviceRun) { theService = old }(theService)
	theService = newServiceRun()
	ctx, cancel := serviceContext(context.Background())
	defer cancel()

	r := make(chan svc.ChangeRequest)
	s := make(chan svc.Status, 16)
	returned := make(chan struct{})
	go func() {
		serviceHandler{}.Execute(nil, r, s)
		close(returned)
	}()
	request := func(c svc.ChangeRequest) {
		t.Helper()
		select {
		case r <- c:
		case <-returned:
			t.Fatalf("the handler returned before %v reached it", c.Cmd)
		case <-time.After(2 * time.Second):
			t.Fatalf("the handler did not take %v", c.Cmd)
		}
	}
	request(svc.ChangeRequest{Cmd: svc.Stop})
	select {
	case <-ctx.Done():
	case <-returned:
		t.Fatal("the handler returned on the stop without ending serve's context")
	case <-time.After(2 * time.Second):
		t.Fatal("the stop did not end serve's context")
	}
	select {
	case <-returned:
		t.Fatal("the handler returned before serve did")
	case <-time.After(300 * time.Millisecond):
	}
	request(svc.ChangeRequest{Cmd: svc.Interrogate})
	go serviceEnd()
	select {
	case <-returned:
	case <-time.After(2 * time.Second):
		t.Fatal("the handler did not return once serve did")
	}
	close(s)
	var pending []svc.Status
	for st := range s {
		if st.State == svc.StopPending {
			pending = append(pending, st)
		}
	}
	if len(pending) != 2 {
		t.Fatalf("stop pending reported %d times, want for the stop and the interrogation: %+v", len(pending), pending)
	}
	if want := uint32(15 * time.Second / time.Millisecond); pending[0].WaitHint < want {
		t.Errorf("wait hint %d ms, want at least serve's %d", pending[0].WaitHint, want)
	}
	log, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(log), "stop requested\n") || !strings.Contains(string(log), "serve stopped\n") {
		t.Errorf("service.log lacks the stop's lines:\n%s", log)
	}
}

// tempServiceLog points service.log into a temporary %ProgramData%, away
// from the one of a service installed on this machine.
func tempServiceLog(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("ProgramData", dir)
	if err := os.MkdirAll(filepath.Join(dir, "Massimo"), 0o700); err != nil {
		t.Fatal(err)
	}
	if p := serviceLogPath(); !strings.HasPrefix(p, dir) {
		t.Fatalf("service.log at %s, outside %s", p, dir)
	}
	return serviceLogPath()
}

// A serve that ends without a stop ends the process at once, so the
// manager's recovery actions restart the service.
func TestServiceEndWithoutStop(t *testing.T) {
	tempServiceLog(t)
	defer func(old *serviceRun) { theService = old }(theService)
	theService = newServiceRun()
	ended := make(chan struct{})
	go func() {
		serviceEnd()
		close(ended)
	}()
	select {
	case <-ended:
	case <-time.After(2 * time.Second):
		t.Fatal("serviceEnd waited without a stop")
	}
	select {
	case <-theService.done:
		t.Fatal("serviceEnd reported serve's end without a stop")
	default:
	}
}
