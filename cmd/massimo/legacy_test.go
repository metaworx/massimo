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
