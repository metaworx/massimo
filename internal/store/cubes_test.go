// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package store

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"go.metaworx.ch/massimo/internal/testkit"
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
	t.Parallel()
	dir := copied(t)
	log, logged := testkit.LogWatch(nil)
	s, err := Open(dir, Options{Logger: log})
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
	if logged.Count(0, testkit.Msg("cube added from the settings file")) != 2 {
		t.Fatalf("log %v", logged.Since(0, nil))
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
	m := logged.Mark()
	if res, err := s.Seed(ctx, "x.env", file); err != nil || len(res.Added) != 0 || len(res.Differences) != 0 || logged.Count(m, nil) != 0 {
		t.Fatalf("second seed %+v %v %v", res, err, logged.Since(m, nil))
	}

	// The UI renames the Cube and hides another device; the file keeps
	// its lines. The store wins, the lines are reported.
	if _, err := s.UpdateCube(ctx, "KEQ0000001", func(c *Cube) error {
		c.Name, c.Hidden, c.Source = "Basement", []string{"0abc13", "0abc12", "0abc14"}, SourceUI
		return nil
	}, 0); err != nil {
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
	logged.Has(t, m, "the difference, the store's kept", testkit.Msg("settings file differs from the store, the store wins", "variable", "MASSIMO_CUBE_KEQ0000001_NAME"))
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
// names it; one removed through the API stays removed, an unconfigured
// row of before RemoveCube as a removed row (D20-B).
func TestSeedFoundAndRemoved(t *testing.T) {
	t.Parallel()
	s := open(t, t.TempDir())
	ctx := context.Background()
	if _, isNew, err := s.NoteDiscovered(ctx, Discovered{Serial: "KEQ0000001", Address: "10.0.0.1", MAC: "00:1a:22:00:00:01", RFAddress: "0aa001", Firmware: "1.1.3"}); err != nil || !isNew {
		t.Fatalf("%v %v", isNew, err)
	}
	if _, err := s.AddCube(ctx, Cube{Serial: "KEQ0000002", Configured: true, Address: "10.0.0.2"}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.UpdateCube(ctx, "KEQ0000002", func(c *Cube) error { c.Configured = false; return nil }, 0); err != nil {
		t.Fatal(err)
	}
	if _, err := s.AddCube(ctx, Cube{Serial: "KEQ0000003", Configured: true, Address: "10.0.0.3"}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.RemoveCube(ctx, "KEQ0000003", 0); err != nil {
		t.Fatal(err)
	}
	res, err := s.Seed(ctx, "x.env", []FileCube{fileCube("KEQ0000001", FieldName, "Cellar"), fileCube("KEQ0000002", FieldAddress, "10.0.0.2"),
		fileCube("KEQ0000003", FieldAddress, "10.0.0.3")})
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
	if c3, ok, _ := s.Cube(ctx, "KEQ0000003"); ok {
		t.Fatalf("removed row brought back by the file %+v", c3)
	}
}

// A found Cube is a row that survives reopening; a later answer updates
// the discovery columns and keeps the first sighting and a known MAC.
func TestFoundCubeSurvivesRestart(t *testing.T) {
	t.Parallel()
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
	if _, err := s.UpdateCube(ctx, "KEQ0000008", func(*Cube) error { return nil }, 0); !errors.Is(err, ErrNotFound) {
		t.Fatalf("update without a row: %v", err)
	}
}

// proxy_virtual is a column of the Cube's row (migration 0024, which
// merged cube_options into cubes): off by default, kept by UpdateCube.
func TestCubeProxyVirtual(t *testing.T) {
	t.Parallel()
	s := open(t, t.TempDir())
	ctx := context.Background()
	c, err := s.AddCube(ctx, Cube{Serial: "KEQ0000001", Configured: true})
	if err != nil || c.ProxyVirtual {
		t.Fatalf("defaults %+v %v", c, err)
	}
	set := func(on bool) Cube {
		t.Helper()
		c, err := s.UpdateCube(ctx, "KEQ0000001", func(c *Cube) error { c.ProxyVirtual = on; return nil }, 0)
		if err != nil {
			t.Fatal(err)
		}
		return c
	}
	if c := set(true); !c.ProxyVirtual || c.Version != 2 {
		t.Fatalf("proxy_virtual not kept: %+v", c)
	}
	if got, ok, err := s.CubeByID(ctx, c.ID); err != nil || !ok || !got.ProxyVirtual {
		t.Fatalf("by id: %+v %v %v", got, ok, err)
	}
	if c := set(false); c.ProxyVirtual {
		t.Fatal("proxy_virtual not turned off")
	}
}

// A removed row of a serial stays removed (D11-A): RemoveCube frees the
// serial, discovery makes no row for it while it has removed rows
// (D20-B), adding it makes a row of its own, and adding it with a
// removed row's uuid restores that row, one of two, with its id and so
// its log, backups, rooms, commands and audit; RemovedCubes lists what
// can be restored.
func TestRemovedCubeAddedOrRestored(t *testing.T) {
	t.Parallel()
	clock := time.Date(2026, 11, 16, 8, 0, 0, 0, time.UTC)
	s, err := Open(copied(t), Options{Logger: quiet(), Now: func() time.Time { return clock }})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx := context.Background()
	old, err := s.AddCube(ctx, Cube{Serial: "KEQ0000001", Name: "Old", Configured: true})
	if err != nil {
		t.Fatal(err)
	}
	history := func(id int64) (cmd Command, audit int64) {
		t.Helper()
		if err := s.InsertEvent(ctx, Event{CubeID: id, Kind: "state", Data: json.RawMessage(`{}`)}); err != nil {
			t.Fatal(err)
		}
		if _, err := s.AddBackup(ctx, Backup{CubeID: id, Reason: ReasonManual, Format: "json"}, []byte(`{"version":1}`)); err != nil {
			t.Fatal(err)
		}
		if _, err := s.SyncRooms(ctx, id, []uint8{1}); err != nil {
			t.Fatal(err)
		}
		if cmd, err = s.AddCommand(ctx, Command{CubeID: id, Action: "setpoint"}); err != nil {
			t.Fatal(err)
		}
		if audit, err = s.Audit(ctx, AuditEntry{Time: clock, Action: "cube.change", CubeID: id, Result: ResultOK}); err != nil {
			t.Fatal(err)
		}
		return cmd, audit
	}
	kept := func(what string, id int64, cmd Command, audit int64) {
		t.Helper()
		if ev, err := s.Events(ctx, EventQuery{CubeID: id}); err != nil || len(ev) != 1 {
			t.Errorf("%s: log %+v %v", what, ev, err)
		}
		if bk, err := s.Backups(ctx, id); err != nil || len(bk) != 1 {
			t.Errorf("%s: backups %+v %v", what, bk, err)
		}
		if rooms, err := s.Rooms(ctx, id); err != nil || len(rooms) != 1 {
			t.Errorf("%s: rooms %+v %v", what, rooms, err)
		}
		if c, ok, err := s.Command(ctx, cmd.ID); err != nil || !ok || c.CubeID != id {
			t.Errorf("%s: command %+v %v %v", what, c, ok, err)
		}
		var cubeID int64
		if err := s.db.QueryRowContext(ctx, `SELECT cube_id FROM audit WHERE id = ?`, audit).Scan(&cubeID); err != nil || cubeID != id {
			t.Errorf("%s: audit row of cube %d %v", what, cubeID, err)
		}
	}
	oldCmd, oldAudit := history(old.ID)

	clock = clock.Add(time.Hour)
	gone, err := s.RemoveCube(ctx, "keq0000001", 0)
	if err != nil || gone.ID != old.ID || gone.Configured || !gone.RemovedAt.Equal(clock) || gone.Version != old.Version+1 {
		t.Fatalf("removed: %+v %v", gone, err)
	}
	if _, err := s.RemoveCube(ctx, "KEQ0000001", 0); !errors.Is(err, ErrNotFound) {
		t.Fatalf("a removed serial removed again: %v", err)
	}
	if all, _ := s.Cubes(ctx); len(all) != 0 {
		t.Fatalf("a removed row listed: %+v", all)
	}
	found, created, err := s.NoteDiscovered(ctx, Discovered{Serial: "KEQ0000001", Address: "192.0.2.10"})
	if err != nil || created || found.ID != 0 || found.DiscoveredAddress != "192.0.2.10" {
		t.Fatalf("found again: %+v %v %v", found, created, err)
	}
	if all, _ := s.Cubes(ctx); len(all) != 0 {
		t.Fatalf("discovery made a row beside a removed one: %+v", all)
	}

	fresh, err := s.AddCube(ctx, Cube{Serial: "KEQ0000001", Name: "New", Configured: true})
	if err != nil || fresh.ID == old.ID || fresh.Name != "New" {
		t.Fatalf("added as new: %+v %v", fresh, err)
	}
	if ev, _ := s.Events(ctx, EventQuery{CubeID: fresh.ID}); len(ev) != 0 {
		t.Fatalf("the new row has the removed one's log: %+v", ev)
	}
	freshCmd, freshAudit := history(fresh.ID)
	clock = clock.Add(time.Hour)
	if _, err := s.RemoveCube(ctx, "KEQ0000001", 0); err != nil {
		t.Fatal(err)
	}
	removed, err := s.RemovedCubes(ctx, "keq0000001")
	if err != nil || len(removed) != 2 || removed[0].ID != fresh.ID || removed[1].ID != old.ID || removed[1].Name != "Old" || !removed[0].RemovedAt.After(removed[1].RemovedAt) {
		t.Fatalf("removed rows: %+v %v", removed, err)
	}
	if _, err := s.AddCube(ctx, Cube{UUID: fresh.UUID + "x", Serial: "KEQ0000001"}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("restore of no removed row: %v", err)
	}
	back, err := s.AddCube(ctx, Cube{UUID: old.UUID, Serial: "KEQ0000001", Name: "Back", Configured: true})
	if err != nil || back.ID != old.ID || back.Name != "Back" || back.Version != gone.Version+1 {
		t.Fatalf("restored: %+v %v", back, err)
	}
	kept("the restored row", back.ID, oldCmd, oldAudit)
	kept("the other removed row", fresh.ID, freshCmd, freshAudit)
	if removed, _ := s.RemovedCubes(ctx, "KEQ0000001"); len(removed) != 1 || removed[0].ID != fresh.ID {
		t.Fatalf("removed rows after the restore: %+v", removed)
	}
	if _, err := s.AddCube(ctx, Cube{UUID: fresh.UUID, Serial: "KEQ0000001"}); !errors.Is(err, ErrExists) {
		t.Fatalf("a restore beside a live row: %v", err)
	}
}
