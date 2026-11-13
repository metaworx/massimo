// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package store

import (
	"context"
	"testing"
)

// A Cube without options has the defaults; a set option stays.
func TestCubeOptions(t *testing.T) {
	s := open(t, t.TempDir())
	ctx := context.Background()
	c, err := s.AddCube(ctx, Cube{Serial: "KEQ0000001", Configured: true})
	if err != nil {
		t.Fatal(err)
	}
	if o, err := s.CubeOptions(ctx, c.ID); err != nil || o.ProxyVirtual {
		t.Fatalf("defaults %+v %v", o, err)
	}
	if err := s.SetCubeOptions(ctx, c.ID, CubeOptions{ProxyVirtual: true}, 0); err != nil {
		t.Fatal(err)
	}
	if o, _ := s.CubeOptions(ctx, c.ID); !o.ProxyVirtual {
		t.Fatal("proxy_virtual not kept")
	}
	if err := s.SetCubeOptions(ctx, c.ID, CubeOptions{}, 0); err != nil {
		t.Fatal(err)
	}
	if o, _ := s.CubeOptions(ctx, c.ID); o.ProxyVirtual {
		t.Fatal("proxy_virtual not turned off")
	}
}
