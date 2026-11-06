// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package cubelan

import (
	"context"
	"net"
	"reflect"
	"testing"
	"time"

	"go.metaworx.ch/massimo/internal/cubelan/fakecube"
)

func TestNetConfigFrame(t *testing.T) {
	cfg := NetConfig{
		IP:      net.ParseIP("192.168.1.50"),
		Gateway: net.ParseIP("192.168.1.1"),
		Netmask: net.ParseIP("255.255.255.0"),
		DNS:     []net.IP{net.ParseIP("192.168.1.1")},
	}
	got, err := NetConfigFrame("KEQ0000000", cfg)
	if err != nil {
		t.Fatal(err)
	}
	want := "eQ3Max*\x00KEQ0000000C" +
		"\xc0\xa8\x01\x32" + // ip 192.168.1.50
		"\xc0\xa8\x01\x01" + // gateway 192.168.1.1
		"\xff\xff\xff\x00" + // netmask 255.255.255.0
		"\xc0\xa8\x01\x01" + // dns1 192.168.1.1
		"\x00\x00\x00\x00" + // dns2 none
		"\x00" + // dhcp off
		"\x00" // encryption off
	if string(got) != want {
		t.Fatalf("frame\n got % x\nwant % x", got, []byte(want))
	}
	if len(got) != 41 {
		t.Fatalf("length %d", len(got))
	}
	// The payload of the static write the original software sent to a
	// Cube with firmware 1.1.3 (capture of 2026-11-06, serial replaced).
	cfg = NetConfig{
		IP:         net.ParseIP("192.168.1.222"),
		Gateway:    net.ParseIP("192.168.1.1"),
		Netmask:    net.ParseIP("255.255.255.0"),
		DNS:        []net.IP{net.ParseIP("192.168.1.1")},
		Encryption: true,
	}
	if got, err = NetConfigFrame("KEQ0000000", cfg); err != nil {
		t.Fatal(err)
	}
	want = "eQ3Max*\x00KEQ0000000C" +
		"\xc0\xa8\x01\xde\xc0\xa8\x01\x01\xff\xff\xff\x00\xc0\xa8\x01\x01\x00\x00\x00\x00\x00\x01"
	if string(got) != want {
		t.Fatalf("captured frame\n got % x\nwant % x", got, []byte(want))
	}
	// DHCP with zero addresses, the encryption flag kept, a short serial
	// padded with '*'.
	got, err = NetConfigFrame("KEQ00", NetConfig{DHCP: true, Encryption: true})
	if err != nil {
		t.Fatal(err)
	}
	want = "eQ3Max*\x00KEQ00*****C" + string(make([]byte, 20)) + "\x01\x01"
	if string(got) != want {
		t.Fatalf("dhcp frame\n got % x\nwant % x", got, []byte(want))
	}
	// The payload decodes back to the configuration it was built from.
	back, err := parseNetConfig(got[19:])
	if err != nil || !back.DHCP || !back.Encryption || len(back.DNS) != 0 {
		t.Fatalf("parse back %+v %v", back, err)
	}
	for _, bad := range []NetConfig{
		{IP: net.ParseIP("2001:db8::1")},
		{DNS: []net.IP{net.IPv4zero, net.IPv4zero, net.IPv4zero}},
	} {
		if _, err := NetConfigFrame("KEQ0000000", bad); err == nil {
			t.Fatalf("%+v accepted", bad)
		}
	}
	if _, err := NetConfigFrame("", cfg); err == nil {
		t.Fatal("empty serial accepted")
	}
	if r := RebootFrame("KEQ0000000"); string(r) != "eQ3Max*\x00KEQ0000000R" {
		t.Fatalf("reboot frame %q", r)
	}
}

func TestNetAddrsFromN(t *testing.T) {
	a, ok := parseNetAddrs([]byte{192, 168, 1, 118, 192, 168, 1, 1, 255, 255, 255, 0, 192, 168, 1, 1, 0, 0, 0, 0})
	if !ok || a.IP.String() != "192.168.1.118" || a.Gateway.String() != "192.168.1.1" || a.Netmask.String() != "255.255.255.0" || len(a.DNS) != 1 {
		t.Fatalf("%+v", a)
	}
	// The original software reads the ip only.
	if a, ok = parseNetAddrs([]byte{10, 0, 0, 5}); !ok || a.IP.String() != "10.0.0.5" || a.Gateway != nil {
		t.Fatalf("ip only: %+v", a)
	}
	if _, ok = parseNetAddrs(nil); ok {
		t.Fatal("empty N reply parsed")
	}
}

// shortWaits shortens the waits of a write and of the fake's restart.
// The fake listens on the loopback, so the write sends no broadcast.
func shortWaits(t *testing.T, fake *fakecube.DiscoveryResponder) {
	t.Helper()
	a, b, c, d, e := setReplyWait, setSettle, rereadWait, restartWait, askWait
	t.Cleanup(func() { setReplyWait, setSettle, rereadWait, restartWait, askWait = a, b, c, d, e })
	setReplyWait, setSettle, rereadWait, restartWait, askWait = 300*time.Millisecond, 10*time.Millisecond, time.Second, 3*time.Second, 200*time.Millisecond
	fake.SetRestartDelay(100 * time.Millisecond)
}

func TestNetConfigRoundTrip(t *testing.T) {
	fake, err := fakecube.StartDiscovery("127.0.0.1:0", "KEQ0000000", 0x0a0b0c)
	if err != nil {
		t.Fatal(err)
	}
	defer fake.Close()
	shortWaits(t, fake)
	ctx := context.Background()

	// A factory Cube: c gives the stored fallback, N the lease.
	st, err := QueryNetConfig(ctx, fake.Addr())
	if err != nil || st.Serial != "KEQ0000000" || !st.Stored.DHCP || !st.Stored.Encryption ||
		st.Stored.IP.String() != "192.168.0.222" || st.Stored.Netmask.String() != "255.255.0.0" {
		t.Fatalf("first read: %+v %v", st, err)
	}
	if st.Current == nil || st.Current.IP.String() != "127.0.0.1" || st.Current.Netmask.String() != "255.255.255.0" {
		t.Fatalf("in use: %+v", st.Current)
	}

	// A static write: status 1, then R, answered with 2. The fake's own
	// address keeps the re-read on the loopback.
	static := NetConfig{
		IP:         net.ParseIP("127.0.0.1").To4(),
		Gateway:    net.ParseIP("127.0.0.254").To4(),
		Netmask:    net.ParseIP("255.0.0.0").To4(),
		DNS:        []net.IP{net.ParseIP("127.0.0.53").To4()},
		Encryption: true,
	}
	res, err := SetNetConfig(ctx, fake.Addr(), st.Serial, static)
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != SetStatusOK || !res.RebootSent || res.RebootStatus != SetStatusRestarting || !res.Restarting() {
		t.Fatalf("static write: %+v", res)
	}
	if !reflect.DeepEqual(res.Now.Stored, static) || res.Now.Current == nil || !res.Now.Current.Gateway.Equal(static.Gateway) {
		t.Fatalf("after static: %+v %+v", res.Now.Stored, res.Now.Current)
	}
	if fake.Reboots() != 1 {
		t.Fatalf("reboots %d", fake.Reboots())
	}

	// A DHCP write: status 2, the Cube restarts by itself, no R.
	dhcp := static
	dhcp.DHCP = true
	if res, err = SetNetConfig(ctx, fake.Addr(), st.Serial, dhcp); err != nil {
		t.Fatal(err)
	}
	if res.Status != SetStatusRestarting || res.RebootSent || !reflect.DeepEqual(res.Now.Stored, dhcp) {
		t.Fatalf("dhcp write: %+v", res)
	}
	if res.Now.Current == nil || res.Now.Current.Gateway.String() != "127.0.0.1" {
		t.Fatalf("lease after dhcp: %+v", res.Now.Current)
	}
	if stored, writes := fake.NetConfig(); writes != 2 || stored[20] != 1 || fake.Reboots() != 1 {
		t.Fatalf("fake holds % x after %d writes, %d reboots", stored, writes, fake.Reboots())
	}

	// A write for another serial changes nothing and gets no answer.
	res, err = SetNetConfig(ctx, fake.Addr(), "KEQ0000001", static)
	if err == nil || res.Status != 0 || res.RebootSent {
		t.Fatalf("write to another cube: %+v %v", res, err)
	}
	if _, writes := fake.NetConfig(); writes != 2 {
		t.Fatalf("writes %d", writes)
	}
}
