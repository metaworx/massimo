// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package main

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"go.metaworx.ch/massimo/internal/cubelan"
	"go.metaworx.ch/massimo/internal/cubelan/fakecube"
	"go.metaworx.ch/massimo/internal/testkit"
)

// netconfigWaits are the waits of netconfig's reads and writes against
// a fake on the loopback, which answers at once: the waits for an answer
// are deadlines a passing test does not wait out, and take the scale;
// the settle, the probe's pace and the re-read, which is waited out
// only where nothing answers, do not.
func netconfigWaits() cubelan.NetWaits {
	return cubelan.NetWaits{Reply: testkit.Scale(500 * time.Millisecond), Settle: 10 * time.Millisecond, Reread: 300 * time.Millisecond,
		Restart: testkit.Scale(5 * time.Second), Probe: 100 * time.Millisecond, Ask: testkit.Scale(500 * time.Millisecond)}
}

// silentWaits are the waits of a call that the fake leaves unanswered,
// each waited out in full: short, and not scaled, since no answer is to
// come within them.
func silentWaits() cubelan.NetWaits {
	return cubelan.NetWaits{Reply: 100 * time.Millisecond, Settle: 10 * time.Millisecond, Reread: 300 * time.Millisecond,
		Restart: 300 * time.Millisecond, Probe: 100 * time.Millisecond, Ask: 100 * time.Millisecond}
}

// runNetconfigTest runs the command with the given arguments and stdin,
// and netconfigWaits.
func runNetconfigTest(t *testing.T, stdin string, args ...string) (string, string, error) {
	t.Helper()
	return runNetconfigWith(t, netconfigWaits(), stdin, args...)
}

// runNetconfigWith is runNetconfigTest with the waits given, such as
// silentWaits for a fake that does not answer.
func runNetconfigWith(t *testing.T, waits cubelan.NetWaits, stdin string, args ...string) (string, string, error) {
	t.Helper()
	testkit.Swapping(t)
	selectedSN, snExplicit = "", false
	t.Cleanup(func() { selectedSN, snExplicit = "", false })
	var out, errw bytes.Buffer
	err := netconfig(context.Background(), args, waits, &out, &errw, strings.NewReader(stdin))
	return out.String(), errw.String(), err
}

func TestNetconfigBroadcastRecovery(t *testing.T) {
	// The fake on the loopback stands in for the broadcast target, so
	// nothing goes to the LAN.
	fake, err := fakecube.StartDiscovery("127.0.0.1:0", "KEQ0000000", 0x0a0b0c)
	if err != nil {
		t.Fatal(err)
	}
	defer fake.Close()
	fake.SetStatic(true) // stranded on its stored static configuration

	// The Cube answers the broadcast c: only the DHCP flag changes, the
	// stored addresses and the encryption flag are kept.
	out, _, err := runNetconfigTest(t, "", "-sn", "KEQ0000000", "-broadcast", "-addr", fake.Addr(), "-dhcp", "-dry-run")
	if err != nil {
		t.Fatal(err)
	}
	frame := "frame    65 51 33 4d 61 78 2a 00 4b 45 51 30 30 30 30 30 30 30 43 " +
		"c0 a8 00 de c0 a8 00 01 ff ff 00 00 c0 a8 00 01 00 00 00 00 01 01\n"
	if !strings.Contains(out, "answered the broadcast read") || !strings.Contains(out, frame) || !strings.Contains(out, "dry run: nothing sent") {
		t.Fatalf("answered:\n%s", out)
	}
	// -encryption does not override a flag that was read.
	if out, _, err = runNetconfigTest(t, "", "-sn", "KEQ0000000", "-broadcast", "-addr", fake.Addr(), "-dhcp", "-encryption", "-dry-run"); err != nil || !strings.Contains(out, frame) {
		t.Fatalf("answered with -encryption: %v\n%s", err, out)
	}
	// And the write itself, confirmed.
	out, errOut, err := runNetconfigTest(t, "y\n", "-sn", "KEQ0000000", "-broadcast", "-addr", fake.Addr(), "-dhcp")
	if err != nil || strings.Contains(errOut, "are lost") || !strings.Contains(out, "mode        dhcp\n") {
		t.Fatalf("answered write: %v\n%s\n%s", err, out, errOut)
	}
	if stored, _ := fake.NetConfig(); string(stored[:12]) != "\xc0\xa8\x00\xde\xc0\xa8\x00\x01\xff\xff\x00\x00" || stored[20] != 1 || stored[21] != 1 {
		t.Fatalf("fake holds % x", stored)
	}

	// No answer: zeros, encryption off unless -encryption, and it says
	// that the stored fallback is lost.
	fake.SetSilent(true)
	out, _, err = runNetconfigWith(t, silentWaits(), "", "-sn", "KEQ0000000", "-broadcast", "-addr", fake.Addr(), "-dhcp", "-dry-run")
	frame = "frame    65 51 33 4d 61 78 2a 00 4b 45 51 30 30 30 30 30 30 30 43 " +
		"00 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00 01 00\n"
	if err != nil || !strings.Contains(out, frame) || !strings.Contains(out, "the stored fallback addresses are lost and the factory values are not restored") {
		t.Fatalf("silent:\n%v\n%s", err, out)
	}
	out, _, err = runNetconfigWith(t, silentWaits(), "", "-sn", "KEQ0000000", "-broadcast", "-addr", fake.Addr(), "-dhcp", "-encryption", "-dry-run")
	if err != nil || !strings.Contains(out, " 01 01\n") {
		t.Fatalf("silent with -encryption: %v\n%s", err, out)
	}
	_, errOut, _ = runNetconfigWith(t, silentWaits(), "n\n", "-sn", "KEQ0000000", "-broadcast", "-addr", fake.Addr(), "-dhcp")
	if !strings.Contains(errOut, "fallback addresses are lost and the factory values are not restored") {
		t.Fatalf("silent warning:\n%s", errOut)
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
	fake.SetRestartDelay(100 * time.Millisecond)

	// A factory Cube on DHCP: the lease in use, the stored fallback
	// labelled as such.
	out, _, err := runNetconfigTest(t, "", "-addr", fake.Addr())
	if err != nil || !strings.Contains(out, "serial      KEQ0000000\nmode        dhcp\nencryption  on\n"+
		"in use      ip 127.0.0.1 gw 127.0.0.1 mask 255.255.255.0 dns 127.0.0.1\n"+
		"stored      ip 192.168.0.222 gw 192.168.0.1 mask 255.255.0.0 dns 192.168.0.1 (static configuration, the fallback, not in use while DHCP is on)\n") {
		t.Fatalf("read: %v\n%s", err, out)
	}

	static := []string{"-addr", fake.Addr(), "-static", "127.0.0.1", "-gateway", "127.0.0.254", "-netmask", "255.0.0.0", "-dns", "127.0.0.53"}
	// A dry run reads but writes nothing; a "no" sends nothing. The
	// encryption flag is kept as read.
	if out, _, err = runNetconfigTest(t, "", append(static, "-dry-run")...); err != nil ||
		!strings.Contains(out, "new      static ip 127.0.0.1 gw 127.0.0.254 mask 255.0.0.0 dns 127.0.0.53 encryption on") ||
		!strings.Contains(out, " 00 01\ndry run: nothing sent") {
		t.Fatalf("dry run: %v\n%s", err, out)
	}
	if _, _, err = runNetconfigTest(t, "n\n", static...); err == nil {
		t.Fatal("a no went ahead")
	}
	if _, writes := fake.NetConfig(); writes != 0 {
		t.Fatalf("%d writes before a yes", writes)
	}

	out, errOut, err := runNetconfigTest(t, "y\n", static...)
	if err != nil || !strings.Contains(out, "write    status 1, applied\nrestart  sent R: status 2, the cube restarts\n") ||
		!strings.Contains(out, "mode        static\n") || !strings.Contains(out, "in use      ip 127.0.0.1 gw 127.0.0.254 mask 255.0.0.0 dns 127.0.0.53\n") {
		t.Fatalf("static: %v\n%s\n%s", err, out, errOut)
	}
	if !strings.Contains(errOut, "netconfig -sn KEQ0000000 -broadcast -dhcp") || !strings.Contains(errOut, "cube KEQ0000000?") {
		t.Fatalf("warning and question:\n%s", errOut)
	}
	if fake.Reboots() != 1 {
		t.Fatalf("reboots %d", fake.Reboots())
	}

	// A DHCP write: status 2, the Cube restarts by itself, no R.
	out, errOut, err = runNetconfigTest(t, "", "-addr", fake.Addr(), "-dhcp", "-yes", "-json")
	if err != nil || !strings.Contains(out, `"dhcp": true`) || !strings.Contains(out, `"stored"`) || !strings.Contains(out, `"current"`) {
		t.Fatalf("dhcp: %v\n%s", err, out)
	}
	if !strings.Contains(errOut, "write    status 2, the cube restarts\n") || strings.Contains(errOut, "sent R") || fake.Reboots() != 1 {
		t.Fatalf("dhcp progress:\n%s", errOut)
	}
	if _, _, err = runNetconfigTest(t, "", "-sn", "KEQ0000001", "-addr", fake.Addr()); err == nil {
		t.Fatal("a cube with another serial than -sn accepted")
	}
}

// netconfig -reboot reads the serial, warns, asks, and sends R; a dry
// run prints the frame and sends nothing.
func TestNetconfigReboot(t *testing.T) {
	fake, err := fakecube.StartDiscovery("127.0.0.1:0", "KEQ0000000", 0x0a0b0c)
	if err != nil {
		t.Fatal(err)
	}
	defer fake.Close()
	fake.SetRestartDelay(10 * time.Millisecond)

	out, _, err := runNetconfigTest(t, "", "-addr", fake.Addr(), "-reboot", "-dry-run")
	if err != nil || !strings.Contains(out, "cube     KEQ0000000 at "+fake.Addr()) || !strings.Contains(out, "frame    65 51 33 4d 61 78 2a 00 4b 45 51") ||
		!strings.Contains(out, "dry run: nothing sent") {
		t.Fatalf("dry run: %v\n%s", err, out)
	}
	if _, _, err = runNetconfigTest(t, "n\n", "-addr", fake.Addr(), "-reboot"); err == nil {
		t.Fatal("a no went ahead")
	}
	if fake.Reboots() != 0 {
		t.Fatalf("%d reboots before a yes", fake.Reboots())
	}
	out, errOut, err := runNetconfigTest(t, "y\n", "-addr", fake.Addr(), "-reboot")
	if err != nil || !strings.Contains(out, "reboot   sent, answered with status 2 (restarting)") {
		t.Fatalf("reboot: %v\n%s\n%s", err, out, errOut)
	}
	if !strings.Contains(errOut, "drops the radiograms") || !strings.Contains(errOut, "pending_radiograms") || !strings.Contains(errOut, "reboot cube KEQ0000000?") {
		t.Fatalf("warning and question:\n%s", errOut)
	}
	if fake.Reboots() != 1 {
		t.Fatalf("%d reboots", fake.Reboots())
	}
	if _, _, err = runNetconfigTest(t, "", "-addr", fake.Addr(), "-reboot", "-dhcp"); err == nil {
		t.Fatal("-reboot with -dhcp went ahead")
	}
}
