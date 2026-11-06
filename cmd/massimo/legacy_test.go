// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package main

import "testing"

func TestDerivedLANLines(t *testing.T) {
	cases := map[string]string{
		`15:56:31,042 INFO  [de.eq3.max.il.local.CubeGateway:methodCalled] () --> commitDeviceRadiogram(DeviceRadiogram(opCode=s:, payload=[0,4,64,0,0,0,1,44,-87,5,42]))`: "s:AARAAAAAASypBSo=",
		`16:16:30,706 INFO  [de.eq3.max.il.local.CubeGateway:methodCalled] () --> deleteDevices([6247],true)`:                                                              "t:01,1,ABhn",
		`16:16:30,957 INFO  [de.eq3.max.il.local.CubeGateway:methodCalled] () --> sendMetaData(VgIGAQ==)`:                                                                  "m:00,VgIGAQ==",
		`14:42:09,619 INFO  [de.eq3.max.il.local.CubeGateway:methodCalled] () --> wakeUpRoom(4)`:                                                                           "z:1e,G,04",
		`14:42:12,179 INFO  [de.eq3.max.il.local.CubeGateway:methodCalled] () --> getConfiguration(1402493)`:                                                               "c:15667d",
		`14:40:43,955 INFO  [de.eq3.max.il.local.CubeGateway:methodCalled] () --> getDeviceList()`:                                                                         "l:",
		`something else`: "",
	}
	for line, want := range cases {
		if got := derivedLANLine(line); got != want {
			t.Errorf("%s\n got %q\nwant %q", line, got, want)
		}
	}
}

// The Cube search prints without a log prefix; those lines are kept, the
// framework's own chatter is not.
func TestLegacyKeepsTheCubeSearch(t *testing.T) {
	keep := []string{
		`--> bind multicast address to each active network interface available`,
		`eth3 : 169.254.244.146 : subnetMask 255.255.0.0`,
		`subnetMask 255.255.255.255`,
		`eth7 : 192.168.1.45 : subnetMask 255.255.255.0`,
		`trying Npcap Loopback Adapter (eth3)`,
		`trying Intel(R) Ethernet Connection (11) I219-LM (eth7)`,
		`trying Hyper-V Virtual Ethernet Adapter #2 (eth22)`,
		`--> listened to socket for 2s`,
		`--> 1 LAN Gateways total`,
		`01:56:33,161 INFO  [de.eq3.max.al.local.MaxLocalApi:methodCalled] () --> getNetworkConfiguration(192.168.1.46)`,
		`01:59:31,628 WARN  [de.eq3.max.al.local.update.CubeUdpProxy:sendRequest] () java.net.NoRouteToHostException: No route to host: Datagram send failed`,
	}
	for _, line := range keep {
		if !legacyKeep.MatchString(line) {
			t.Errorf("dropped: %s", line)
		}
	}
	drop := []string{
		`01:55:05,772 DEBUG [org.springframework.beans.factory.support.DefaultListableBeanFactory] () Creating shared instance`,
		`        at org.springframework.beans.factory.support.AbstractBeanFactory.getBean(AbstractBeanFactory.java:312)`,
		`something trying hard without an interface`,
		`ethernet : no colon pattern`,
	}
	for _, line := range drop {
		if legacyKeep.MatchString(line) {
			t.Errorf("kept: %s", line)
		}
	}
}
