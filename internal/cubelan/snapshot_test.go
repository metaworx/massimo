// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package cubelan

import (
	"testing"
	"time"

	"go.metaworx.ch/massimo/internal/cubelan/fakecube"
)

// A snapshot built from a greeting's lines holds what a connection
// holds, and gives the same lines back.
func TestSnapshotOf(t *testing.T) {
	greeting, err := fakecube.LoadGreeting(fixture)
	if err != nil {
		t.Fatal(err)
	}
	s, err := SnapshotOf(greeting, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if h := s.Hello(); h == nil || h.Serial == "" {
		t.Fatalf("hello %+v", h)
	}
	if md := s.Metadata(); md == nil || len(md.Rooms) == 0 || len(s.Devices()) != len(md.Devices) {
		t.Fatalf("metadata %+v, devices %v", md, s.Devices())
	}
	for _, rf := range s.Devices() {
		if s.Config(rf) == nil {
			t.Fatalf("no configuration of %s", rf)
		}
		if _, ok := s.State(rf); !ok {
			t.Fatalf("no state of %s", rf)
		}
	}
	again, err := SnapshotOf(s.Lines(), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	a, b := s.Lines(), again.Lines()
	if len(a) != len(b) {
		t.Fatalf("%d lines, again %d", len(a), len(b))
	}
	for i := range a {
		if a[i].String() != b[i].String() {
			t.Fatalf("line %d: %s, again %s", i, a[i], b[i])
		}
	}
}
