// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"go.metaworx.ch/massimo/internal/env"
	"go.metaworx.ch/massimo/internal/store"
	"go.metaworx.ch/massimo/internal/update"
)

func TestHealthClient(t *testing.T) {
	if healthClient("http://127.0.0.1:8099/healthz").Transport != nil {
		t.Fatal("plain http given a transport of its own")
	}
	if healthClient("https://192.0.2.10:8099/healthz").Transport != nil {
		t.Fatal("a LAN host not verified")
	}
	for _, u := range []string{"https://127.0.0.1:8099/healthz", "https://[::1]:8099/healthz", "https://localhost:8099/healthz"} {
		tr, ok := healthClient(u).Transport.(*http.Transport)
		if !ok || tr.TLSClientConfig == nil || !tr.TLSClientConfig.InsecureSkipVerify {
			t.Fatalf("%s verified", u)
		}
	}
}

// TestWaitHealthyOverTLS checks the helper's health check against a
// server with a certificate nothing trusts, on the loopback.
func TestWaitHealthyOverTLS(t *testing.T) {
	ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"ok":true,"version":"massimo v1.2.3 (go)"}`)
	}))
	defer ts.Close()
	if err := waitHealthy(context.Background(), ts.URL+"/healthz", "v1.2.3", 5*time.Second); err != nil {
		t.Fatal(err)
	}
	if err := waitHealthy(context.Background(), ts.URL+"/healthz", "v1.2.4", time.Second); err == nil {
		t.Fatal("wrong version passed")
	}
}

// TestUpdateVariablesAreKnown keeps massimo config from flagging the
// variables massimo update reads as typos.
func TestUpdateVariablesAreKnown(t *testing.T) {
	for _, k := range []string{update.TokenVar, "MASSIMO_UPDATE_CHANNEL"} {
		if !env.Known(k) {
			t.Errorf("%s unknown to massimo config", k)
		}
	}
}

// TestSmokeRunsRealBinary builds massimo and runs the update's smoke test
// on it: the stand-in scripts of internal/update cannot show how the real
// binary starts (it loads its .env before every command).
func TestSmokeRunsRealBinary(t *testing.T) {
	if testing.Short() {
		t.Skip("builds the binary")
	}
	bin := filepath.Join(t.TempDir(), "massimo")
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}
	out, err := exec.Command("go", "build", "-ldflags", "-X main.version=v9.8.7", "-o", bin, ".").CombinedOutput()
	if err != nil {
		t.Fatalf("go build: %v\n%s", err, out)
	}
	if err := update.Smoke(context.Background(), bin, "v9.8.7"); err != nil {
		t.Fatal(err)
	}
	if err := update.Smoke(context.Background(), bin, "v9.8.8"); err == nil {
		t.Fatal("wrong version passed")
	}
}

// ownAddress is one of this box's addresses that is not a loopback, ""
// when it has none.
func ownAddress(t *testing.T) string {
	t.Helper()
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return ""
	}
	for _, a := range addrs {
		if n, ok := a.(*net.IPNet); ok && !n.IP.IsLoopback() && n.IP.To4() != nil {
			return n.IP.String()
		}
	}
	return ""
}

// A health URL on one of the box's own addresses, which install writes
// for a specific -listen address, is checked over TLS without
// verification, as the loopback is: the certificate may be self-signed,
// or for a name rather than the address.
func TestWaitHealthyOnOwnAddress(t *testing.T) {
	ip := ownAddress(t)
	if ip == "" {
		t.Skip("no address but the loopback")
	}
	ln, err := net.Listen("tcp", net.JoinHostPort(ip, "0"))
	if err != nil {
		t.Skipf("cannot listen on %s: %v", ip, err)
	}
	ts := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"ok":true,"version":"massimo v1.2.3 (go)"}`)
	}))
	ts.Listener.Close()
	ts.Listener = ln
	ts.StartTLS()
	defer ts.Close()
	if err := waitHealthy(context.Background(), ts.URL+"/healthz", "v1.2.3", 5*time.Second); err != nil {
		t.Fatalf("%s: %v", ts.URL, err)
	}
}

// The box's own addresses are those its interfaces have when the check
// starts; another address, and a name, stay verified.
func TestHealthClientOwnAddresses(t *testing.T) {
	was := interfaceAddrs
	defer func() { interfaceAddrs = was }()
	interfaceAddrs = func() ([]net.Addr, error) {
		return []net.Addr{&net.IPNet{IP: net.ParseIP("192.0.2.10"), Mask: net.CIDRMask(24, 32)}, &net.IPNet{IP: net.ParseIP("2001:db8::10"), Mask: net.CIDRMask(64, 128)}}, nil
	}
	for u, skip := range map[string]bool{
		"https://192.0.2.10:443/healthz":      true,
		"https://[2001:db8::10]:443/healthz":  true,
		"https://192.0.2.11:443/healthz":      false,
		"https://massimo.example.net/healthz": false,
		"http://192.0.2.10:8099/healthz":      false,
	} {
		tr, _ := healthClient(u).Transport.(*http.Transport)
		if got := tr != nil && tr.TLSClientConfig != nil && tr.TLSClientConfig.InsecureSkipVerify; got != skip {
			t.Errorf("%s: unverified %v, want %v", u, got, skip)
		}
	}
}

// The command line follows serve's channel: MASSIMO_UPDATE_CHANNEL, else
// the store's update.channel, else the build's flavour; the helper, which
// reads nothing the server's user can write, skips the store.
func TestUpdateChannelSources(t *testing.T) {
	data := t.TempDir()
	t.Setenv(env.DataVar, data)
	t.Setenv("MASSIMO_UPDATE_CHANNEL", "")
	old := flavour
	flavour = "beta"
	t.Cleanup(func() { flavour = old })

	if c, from := updateChannel(true); c != "beta" || from != fromFlavour {
		t.Fatalf("no store: %s from %s", c, from)
	}
	st, err := store.Open(data, store.Options{Version: "test", Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	if err != nil {
		t.Fatal(err)
	}
	if err := st.SetSetting(context.Background(), store.UpdateChannelKey, "alpha"); err != nil {
		t.Fatal(err)
	}
	st.Close()
	if c, from := updateChannel(true); c != "alpha" || from != fromSetting {
		t.Fatalf("the setting: %s from %s", c, from)
	}
	if c, from := updateChannel(false); c != "beta" || from != fromFlavour {
		t.Fatalf("the helper: %s from %s", c, from)
	}
	t.Setenv("MASSIMO_UPDATE_CHANNEL", "devel")
	if c, from := updateChannel(true); c != "devel" || from != fromVariable {
		t.Fatalf("the variable: %s from %s", c, from)
	}
}
