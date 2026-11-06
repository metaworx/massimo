// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package main

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"go.metaworx.ch/massimo/internal/cubelan/fakecube"
)

// runNetconfigTest runs the command with the given arguments and stdin.
func runNetconfigTest(t *testing.T, stdin string, args ...string) (string, string, error) {
	t.Helper()
	selectedSN, snExplicit = "", false
	t.Cleanup(func() { selectedSN, snExplicit = "", false })
	var out, errw bytes.Buffer
	err := netconfig(context.Background(), args, &out, &errw, strings.NewReader(stdin))
	return out.String(), errw.String(), err
}

func TestNetconfigDryRunBroadcast(t *testing.T) {
	out, _, err := runNetconfigTest(t, "", "-sn", "KEQ0000000", "-broadcast", "-dhcp", "-dry-run")
	if err != nil {
		t.Fatal(err)
	}
	// Zeros for the addresses, DHCP on, encryption off.
	frame := "frame    65 51 33 4d 61 78 2a 00 4b 45 51 30 30 30 30 30 30 30 43 " +
		"00 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00 01 00\n"
	if !strings.Contains(out, frame) || !strings.Contains(out, "dry run: nothing sent") ||
		!strings.Contains(out, "cube     KEQ0000000 at 255.255.255.255\n") {
		t.Fatalf("output:\n%s", out)
	}
	out, _, err = runNetconfigTest(t, "", "-sn", "KEQ0000000", "-broadcast", "-dhcp", "-encryption", "-dry-run")
	if err != nil || !strings.Contains(out, " 01 01\n") {
		t.Fatalf("with -encryption: %v\n%s", err, out)
	}
}

func TestNetconfigRefusesAGatewayOutsideTheSubnet(t *testing.T) {
	_, _, err := runNetconfigTest(t, "", "-sn", "KEQ0000000", "-broadcast", "-dry-run",
		"-static", "192.168.1.50", "-gateway", "192.168.2.1", "-netmask", "255.255.255.0")
	if err == nil || !strings.Contains(err.Error(), "outside the subnet") {
		t.Fatalf("error %v", err)
	}
	for _, bad := range [][]string{
		{"-netmask", "255.0.255.0", "-gateway", "192.168.1.1"},
		{"-netmask", "255.255.255.0", "-gateway", "192.168.1.50"},
	} {
		args := append([]string{"-sn", "KEQ0000000", "-broadcast", "-dry-run", "-static", "192.168.1.50"}, bad...)
		if _, _, err := runNetconfigTest(t, "", args...); err == nil {
			t.Fatalf("%v accepted", bad)
		}
	}
	if _, _, err := runNetconfigTest(t, "", "-addr", "127.0.0.1", "-dhcp", "-encryption"); err == nil {
		t.Fatal("-encryption without -broadcast accepted")
	}
}

func TestNetconfigAgainstTheFakeCube(t *testing.T) {
	fake, err := fakecube.StartDiscovery("127.0.0.1:0", "KEQ0000000", 0x0a0b0c)
	if err != nil {
		t.Fatal(err)
	}
	defer fake.Close()

	out, _, err := runNetconfigTest(t, "", "-addr", fake.Addr())
	if err != nil || !strings.Contains(out, "serial      KEQ0000000\nmode        dhcp\n") {
		t.Fatalf("read: %v\n%s", err, out)
	}

	static := []string{"-addr", fake.Addr(), "-static", "127.0.0.1", "-gateway", "127.0.0.254", "-netmask", "255.0.0.0", "-dns", "127.0.0.53"}
	// A dry run reads but writes nothing; a "no" sends nothing.
	if out, _, err = runNetconfigTest(t, "", append(static, "-dry-run")...); err != nil ||
		!strings.Contains(out, "new      static ip 127.0.0.1 gw 127.0.0.254 mask 255.0.0.0 dns 127.0.0.53 encryption off") {
		t.Fatalf("dry run: %v\n%s", err, out)
	}
	if _, _, err = runNetconfigTest(t, "n\n", static...); err == nil {
		t.Fatal("a no went ahead")
	}
	if _, writes := fake.NetConfig(); writes != 0 {
		t.Fatalf("%d writes before a yes", writes)
	}

	out, errOut, err := runNetconfigTest(t, "y\n", static...)
	if err != nil || !strings.Contains(out, "the cube now reports:\nserial      KEQ0000000\nmode        static\nip          127.0.0.1\n") {
		t.Fatalf("static: %v\n%s\n%s", err, out, errOut)
	}
	if !strings.Contains(errOut, "netconfig -sn KEQ0000000 -broadcast -dhcp") || !strings.Contains(errOut, "cube KEQ0000000?") {
		t.Fatalf("warning and question:\n%s", errOut)
	}

	if out, _, err = runNetconfigTest(t, "", "-addr", fake.Addr(), "-dhcp", "-yes", "-json"); err != nil ||
		!strings.Contains(out, `"dhcp": true`) || !strings.Contains(out, `"ip": "127.0.0.1"`) {
		t.Fatalf("dhcp: %v\n%s", err, out)
	}
	if _, _, err = runNetconfigTest(t, "", "-sn", "KEQ0000001", "-addr", fake.Addr()); err == nil {
		t.Fatal("a cube with another serial than -sn accepted")
	}
}
