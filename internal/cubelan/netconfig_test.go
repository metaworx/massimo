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
}

func TestNetConfigRoundTrip(t *testing.T) {
	defer func(a, b, c time.Duration) { setReplyWait, setSettle, askWait = a, b, c }(setReplyWait, setSettle, askWait)
	setReplyWait, setSettle, askWait = 50*time.Millisecond, 10*time.Millisecond, 500*time.Millisecond

	fake, err := fakecube.StartDiscovery("127.0.0.1:0", "KEQ0000000", 0x0a0b0c)
	if err != nil {
		t.Fatal(err)
	}
	defer fake.Close()
	ctx := context.Background()

	serial, cfg, err := QueryNetConfig(ctx, fake.Addr())
	if err != nil || serial != "KEQ0000000" || !cfg.DHCP || cfg.Encryption {
		t.Fatalf("first read: %q %+v %v", serial, cfg, err)
	}

	// The fake's own address keeps the re-read on the loopback.
	static := NetConfig{
		IP:      net.ParseIP("127.0.0.1").To4(),
		Gateway: net.ParseIP("127.0.0.254").To4(),
		Netmask: net.ParseIP("255.0.0.0").To4(),
		DNS:     []net.IP{net.ParseIP("127.0.0.53").To4()},
	}
	res, err := SetNetConfig(ctx, fake.Addr(), serial, static)
	if err != nil {
		t.Fatal(err)
	}
	if res.Answered || !reflect.DeepEqual(res.Now, static) {
		t.Fatalf("after static: %+v", res)
	}
	if _, now, err := QueryNetConfig(ctx, fake.Addr()); err != nil || !reflect.DeepEqual(now, static) {
		t.Fatalf("read static: %+v %v", now, err)
	}

	dhcp := static
	dhcp.DHCP = true
	if res, err = SetNetConfig(ctx, fake.Addr(), serial, dhcp); err != nil || !reflect.DeepEqual(res.Now, dhcp) {
		t.Fatalf("after dhcp: %+v %v", res, err)
	}
	if stored, writes := fake.NetConfig(); writes != 2 || stored[20] != 1 {
		t.Fatalf("fake holds % x after %d writes", stored, writes)
	}

	// A write for another serial changes nothing and gets no re-read.
	if _, err := SetNetConfig(ctx, fake.Addr(), "KEQ0000001", static); err == nil {
		t.Fatal("write to another cube answered")
	}
	if _, writes := fake.NetConfig(); writes != 2 {
		t.Fatalf("writes %d", writes)
	}
}
