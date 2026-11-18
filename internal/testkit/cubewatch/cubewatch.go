// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

// Package cubewatch is the test kit's watch of the lines a fake Cube
// receives, with the filters the tests compare them by. It is a package
// of its own because it needs the fake Cube, whose tests import the kit.
// It does not import the Cube client, so that the client's tests can
// use it; their client's events are a testkit.ChanWatch.
//
// Only test files import the package (testkit's
// TestOnlyTestsImportTheKit). docs/testing.md says how a test waits.
package cubewatch

import (
	"bytes"

	"go.metaworx.ch/massimo/internal/cubelan/fakecube"
	"go.metaworx.ch/massimo/internal/protocol/cube"
	"go.metaworx.ch/massimo/internal/testkit"
)

// Lines returns the watch of every line fc receives, from its first on:
// the watch holds what fc.Received holds, in the same order, so that a
// cursor is an index into it too. The lines show as the Cube received
// them ("s:…").
func Lines(fc *fakecube.Server) *testkit.Watch[cube.Message] {
	w := testkit.NewWatch(cube.Message.String)
	fc.OnReceive(func(m cube.Message) { w.Add(m) })
	return w
}

// Type returns a match for a line of one of the types given: Type('s')
// for the radio commands.
func Type(types ...byte) func(cube.Message) bool {
	return func(m cube.Message) bool { return bytes.IndexByte(types, m.Type) >= 0 }
}

// Except returns a match for a line of none of the types given:
// Except('l') leaves the polls out, Except('l', 'f') the NTP reads too.
func Except(types ...byte) func(cube.Message) bool {
	return func(m cube.Message) bool { return bytes.IndexByte(types, m.Type) < 0 }
}
