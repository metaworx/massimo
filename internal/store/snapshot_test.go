// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package store

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

// A snapshot taken while a writer writes is whole: it opens as a store,
// and its counts are its own rows, some of the writer's and none half.
func TestSnapshotWhileAWriterWrites(t *testing.T) {
	t.Parallel()
	s := open(t, t.TempDir())
	ctx := context.Background()
	c, err := s.AddCube(ctx, Cube{Serial: "KEQ0000000", Configured: true})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.SyncRooms(ctx, c.ID, []uint8{1, 2}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.AddUser(ctx, User{Name: "anna", Role: RoleOperator}, "", 0); err != nil {
		t.Fatal(err)
	}
	req, err := s.AddIntegrationRequest(ctx, haRequest(instanceA), "AAAAAA")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.ApproveIntegration(ctx, req.UUID, nil, "", 0); err != nil {
		t.Fatal(err)
	}
	var audit0 int
	s.db.QueryRow(`SELECT count(*) FROM audit`).Scan(&audit0)

	const writes = 200
	var wg sync.WaitGroup
	wg.Add(1)
	started := make(chan struct{})
	go func() {
		defer wg.Done()
		for i := range writes {
			if _, err := s.Audit(ctx, AuditEntry{Action: "test.write"}); err != nil {
				t.Error(err)
				return
			}
			if i == 10 {
				close(started)
			}
		}
	}()
	<-started
	dst := filepath.Join(t.TempDir(), "snapshot.db")
	sn, err := s.Snapshot(ctx, dst)
	wg.Wait()
	if err != nil {
		t.Fatal(err)
	}
	if sn.Schema != SchemaVersion() || sn.MinReader < 1 {
		t.Fatalf("schema %d, min_reader %d", sn.Schema, sn.MinReader)
	}
	// anna; the integration's own users are not counted.
	got := sn.Counts
	if got.Users != 1 || got.Integrations != 1 || got.Cubes != 1 || got.Rooms != 2 || got.Log != 0 {
		t.Fatalf("counts %+v", got)
	}
	if got.Audit < audit0+11 || got.Audit > audit0+writes {
		t.Fatalf("audit %d, from %d with %d written", got.Audit, audit0, writes)
	}
	if fi, err := os.Stat(dst); err != nil || fi.Mode().Perm() != 0o600 {
		t.Fatalf("the snapshot: %v %v", fi, err)
	}
	if left, _ := filepath.Glob(dst + "-*"); len(left) != 0 {
		t.Fatalf("beside the snapshot: %v", left)
	}

	// The copy is a store of its own, with the rows it counted.
	if err := os.Rename(dst, Path(filepath.Dir(dst))); err != nil {
		t.Fatal(err)
	}
	cp, err := OpenReadOnly(filepath.Dir(dst), Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer cp.Close()
	in, err := cp.Info(ctx)
	if err != nil || in.Counts["audit"] != got.Audit || in.Counts["cubes"] != 1 || in.NodeID != s.NodeID() {
		t.Fatalf("the copy: %+v %v", in, err)
	}
	if _, err := s.Snapshot(ctx, filepath.Join(filepath.Dir(dst), FileName)); err == nil {
		t.Fatal("a snapshot over an existing file")
	}
}

// A node's signature verifies with its public key, this box's as read
// from node.key, and with no other key or message.
func TestVerifyNode(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	s := open(t, dir)
	msg := []byte(`{"format":1}`)
	sig, err := s.Sign(msg)
	if err != nil {
		t.Fatal(err)
	}
	pub, err := ReadNodePublicKey(dir)
	if err != nil || !pub.Equal(s.NodePublicKey()) {
		t.Fatalf("read %x, the store's %x: %v", pub, s.NodePublicKey(), err)
	}
	if !VerifyNode(msg, sig, pub) {
		t.Fatal("the node's own signature")
	}
	other, _, _ := ed25519.GenerateKey(rand.Reader)
	changed := append([]byte(nil), msg...)
	changed[2] ^= 1
	for name, ok := range map[string]bool{
		"another key":     VerifyNode(msg, sig, other),
		"a changed byte":  VerifyNode(changed, sig, pub),
		"a short key":     VerifyNode(msg, sig, pub[:16]),
		"a short sig":     VerifyNode(msg, sig[:10], pub),
		"no key, no sig":  VerifyNode(msg, nil, nil),
		"another message": VerifyNode([]byte("x"), sig, pub),
	} {
		if ok {
			t.Errorf("%s verified", name)
		}
	}
	if _, err := ReadNodePublicKey(t.TempDir()); err == nil {
		t.Fatal("a directory without node.key")
	}
}
