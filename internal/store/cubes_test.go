// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package store

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"
)

func fileCube(serial string, kv ...string) FileCube {
	fc := FileCube{Serial: serial, Values: map[string]FileValue{}}
	for i := 0; i+1 < len(kv); i += 2 {
		fc.Values[kv[i]] = FileValue{Variable: "MASSIMO_CUBE_" + serial + "_" + strings.ToUpper(kv[i]), Value: kv[i+1]}
	}
	return fc
}

// The first start seeds the store from the file; on the second the
// store wins and a differing line is reported and logged; a Cube added
// to the file later is seeded then.
func TestSeedAndPrecedence(t *testing.T) {
	dir := t.TempDir()
	var logged bytes.Buffer
	s, err := Open(dir, Options{Logger: slog.New(slog.NewTextHandler(&logged, nil))})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx := context.Background()
	file := []FileCube{
		fileCube("KEQ0000001", FieldAddress, "10.0.0.1", FieldName, "Cellar", FieldProxyAddress, "10.0.0.11", FieldHidden, "0abc12,0ABC13"),
		fileCube("KEQ0000002", FieldName, "Attic"),
	}
	res, err := s.Seed(ctx, "x.env", file)
	if err != nil || len(res.Added) != 2 || len(res.Differences) != 0 {
		t.Fatalf("first seed %+v %v", res, err)
	}
	if strings.Count(logged.String(), "cube added from the settings file") != 2 {
		t.Fatalf("log %s", logged.String())
	}
	c1, ok, _ := s.Cube(ctx, "keq0000001")
	if !ok || !c1.Configured || c1.Source != SourceEnv || c1.Address != "10.0.0.1" || c1.FollowDiscovery || c1.Name != "Cellar" || c1.ProxyAddress != "10.0.0.11" ||
		strings.Join(c1.Hidden, ",") != "0abc12,0abc13" || c1.UUID == "" || c1.Version != 1 || c1.Origin != s.NodeID() {
		t.Fatalf("cube 1 %+v", c1)
	}
	c2, _, _ := s.Cube(ctx, "KEQ0000002")
	if c2.Address != "" || !c2.FollowDiscovery {
		t.Fatalf("cube 2 %+v", c2)
	}

	// Seeding the same file again changes nothing.
	logged.Reset()
	if res, err := s.Seed(ctx, "x.env", file); err != nil || len(res.Added) != 0 || len(res.Differences) != 0 || logged.Len() != 0 {
		t.Fatalf("second seed %+v %v %s", res, err, logged.String())
	}

	// The UI renames the Cube and hides another device; the file keeps
	// its lines. The store wins, the lines are reported.
	if _, err := s.UpdateCube(ctx, "KEQ0000001", func(c *Cube) error {
		c.Name, c.Hidden, c.Source = "Basement", []string{"0abc13", "0abc12", "0abc14"}, SourceUI
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	file = append(file, fileCube("KEQ0000003", FieldAddress, "10.0.0.3"))
	res, err = s.Seed(ctx, "x.env", file)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Added) != 1 || res.Added[0] != "KEQ0000003" {
		t.Fatalf("added %v", res.Added)
	}
	if len(res.Differences) != 2 || res.Differences[0].Field != FieldName || res.Differences[0].File != "Cellar" || res.Differences[0].Store != "Basement" ||
		res.Differences[0].Variable != "MASSIMO_CUBE_KEQ0000001_NAME" || res.Differences[1].Field != FieldHidden {
		t.Fatalf("differences %+v", res.Differences)
	}
	if !strings.Contains(logged.String(), "settings file differs from the store, the store wins") || !strings.Contains(logged.String(), "MASSIMO_CUBE_KEQ0000001_NAME") {
		t.Fatalf("log %s", logged.String())
	}
	c1, _, _ = s.Cube(ctx, "KEQ0000001")
	if c1.Name != "Basement" || c1.Version != 2 {
		t.Fatalf("store did not win: %+v", c1)
	}
	// Hidden lists compare as sets.
	if d := Differences(c1, fileCube("KEQ0000001", FieldHidden, "0abc14, 0abc12 0abc13")); len(d) != 0 {
		t.Fatalf("same set reported: %+v", d)
	}
}

// A Cube discovery found before is seeded from the file when the file
// names it; one removed through the API stays removed.
func TestSeedFoundAndRemoved(t *testing.T) {
	s := open(t, t.TempDir())
	ctx := context.Background()
	if _, isNew, err := s.NoteDiscovered(ctx, Discovered{Serial: "KEQ0000001", Address: "10.0.0.1", MAC: "00:1a:22:00:00:01", RFAddress: "0aa001", Firmware: "1.1.3"}); err != nil || !isNew {
		t.Fatalf("%v %v", isNew, err)
	}
	if _, err := s.AddCube(ctx, Cube{Serial: "KEQ0000002", Configured: true, Address: "10.0.0.2"}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.UpdateCube(ctx, "KEQ0000002", func(c *Cube) error { c.Configured = false; return nil }); err != nil {
		t.Fatal(err)
	}
	res, err := s.Seed(ctx, "x.env", []FileCube{fileCube("KEQ0000001", FieldName, "Cellar"), fileCube("KEQ0000002", FieldAddress, "10.0.0.2")})
	if err != nil || len(res.Added) != 1 || res.Added[0] != "KEQ0000001" {
		t.Fatalf("%+v %v", res, err)
	}
	c1, _, _ := s.Cube(ctx, "KEQ0000001")
	if !c1.Configured || c1.Source != SourceEnv || c1.Name != "Cellar" || !c1.FollowDiscovery || c1.MAC == "" || c1.DiscoveredAddress != "10.0.0.1" {
		t.Fatalf("found cube seeded %+v", c1)
	}
	c2, _, _ := s.Cube(ctx, "KEQ0000002")
	if c2.Configured {
		t.Fatal("removed cube configured again by the file")
	}
}

// A found Cube is a row that survives reopening; a later answer updates
// the discovery columns and keeps the first sighting and a known MAC.
func TestFoundCubeSurvivesRestart(t *testing.T) {
	dir := t.TempDir()
	t0 := time.Date(2026, 11, 6, 8, 0, 0, 0, time.UTC)
	s := open(t, dir)
	ctx := context.Background()
	if _, _, err := s.NoteDiscovered(ctx, Discovered{Serial: "keq0000009", Address: "10.0.0.9", MAC: "00:1a:22:00:00:09", RFAddress: "0aa009", Firmware: "1.1.3", At: t0}); err != nil {
		t.Fatal(err)
	}
	s.Close()
	s = open(t, dir)
	c, isNew, err := s.NoteDiscovered(ctx, Discovered{Serial: "KEQ0000009", Address: "10.0.0.19", RFAddress: "0aa009", Firmware: "1.1.3", At: t0.Add(time.Hour)})
	if err != nil || isNew {
		t.Fatalf("%v %v", isNew, err)
	}
	if c.Configured || c.Source != SourceDiscovery || c.DiscoveredAddress != "10.0.0.19" || c.MAC != "00:1a:22:00:00:09" || !c.FirstSeen.Equal(t0) || !c.LastSeen.Equal(t0.Add(time.Hour)) {
		t.Fatalf("%+v", c)
	}
	all, _ := s.Cubes(ctx)
	if len(all) != 1 {
		t.Fatalf("%d rows", len(all))
	}
	if _, err := s.AddCube(ctx, Cube{Serial: "KEQ0000009"}); !errors.Is(err, ErrExists) {
		t.Fatalf("add over a row: %v", err)
	}
	if _, err := s.UpdateCube(ctx, "KEQ0000008", func(*Cube) error { return nil }); !errors.Is(err, ErrNotFound) {
		t.Fatalf("update without a row: %v", err)
	}
}
