// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package serverbackup

import (
	"bytes"
	"context"
	"crypto/rand"
	"errors"
	"io"
	"io/fs"
	"maps"
	"net"
	"os"
	"slices"
	"strings"
	"testing"
	"time"
)

// smbLive returns a target on the live test's server, of the fields
// given beside its address, share and user, with the password given.
func smbLive(t *testing.T, cfg map[string]string, password string) (*SMB, error) {
	t.Helper()
	host, port, err := net.SplitHostPort(os.Getenv("MASSIMO_TARGET_TEST_SMB_ADDR"))
	if err != nil {
		t.Fatalf("MASSIMO_TARGET_TEST_SMB_ADDR: %v", err)
	}
	full := map[string]string{"host": host, "port": port,
		"share": os.Getenv("MASSIMO_TARGET_TEST_SMB_SHARE"), "user": os.Getenv("MASSIMO_TARGET_TEST_SMB_USER")}
	maps.Copy(full, cfg)
	k, _ := KindOf(KindSMB)
	checked, err := k.Check(full, password != "")
	if err != nil {
		return nil, err
	}
	tgt, err := k.New(checked, []byte(password))
	if err != nil {
		return nil, err
	}
	return tgt.(*SMB), nil
}

// smbRaw lists a folder of the share as the server holds it, hidden
// files and folders too.
func smbRaw(t *testing.T, ctx context.Context, s *SMB, dir string) []string {
	t.Helper()
	c, err := s.connect(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer c.close()
	fis, err := c.share.ReadDir(dir)
	if err != nil {
		t.Fatalf("list %s: %v", dir, err)
	}
	var names []string
	for _, fi := range fis {
		names = append(names, fi.Name())
	}
	return names
}

// smbRemoveAll removes a folder of the share and what it holds.
func smbRemoveAll(ctx context.Context, c *smbConn, dir string) error {
	fis, err := c.share.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, fi := range fis {
		p := dir + `\` + fi.Name()
		if fi.IsDir() {
			err = smbRemoveAll(ctx, c, p)
		} else {
			err = c.share.Remove(p)
		}
		if err != nil {
			return err
		}
	}
	return c.share.Remove(dir)
}

// TestSMBLive tests the SMB target against a real server: Samba with
// signing and encryption required, as make target-test runs it
// (tools/target-test/README.md). It skips unless
// MASSIMO_TARGET_TEST_SMB_ADDR (host:port), _SMB_SHARE, _SMB_USER and
// _SMB_PASS are set. Everything it writes is in a folder of its own,
// removed at its end.
func TestSMBLive(t *testing.T) {
	pass := os.Getenv("MASSIMO_TARGET_TEST_SMB_PASS")
	for _, v := range []string{"MASSIMO_TARGET_TEST_SMB_ADDR", "MASSIMO_TARGET_TEST_SMB_SHARE", "MASSIMO_TARGET_TEST_SMB_USER", "MASSIMO_TARGET_TEST_SMB_PASS"} {
		if os.Getenv(v) == "" {
			t.Skipf("%s is not set: make target-test runs this test against Samba", v)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	base := "massimo-live-" + smbRandomHex()
	s, err := smbLive(t, map[string]string{"path": base + "/box"}, pass)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()
		c, err := s.connect(ctx)
		if err == nil {
			err = smbRemoveAll(ctx, c, base)
			c.close()
		}
		if err != nil {
			t.Errorf("removing %s: %v", base, err)
		}
	})
	dir := base + `\box`

	// The test makes the folder, and leaves nothing in it.
	if err := s.Check(ctx); err != nil {
		t.Fatal(err)
	}
	if left := smbRaw(t, ctx, s, dir); len(left) != 0 {
		t.Fatalf("the test left %v", left)
	}

	// A copy of several writes, listed and read back by ranges.
	data := make([]byte, 3*smbChunk+12345)
	rand.Read(data)
	const name = "massimo-20261127-033000-live.zip"
	if err := s.Put(ctx, name, bytes.NewReader(data), int64(len(data))); err != nil {
		t.Fatal(err)
	}
	list, err := s.List(ctx)
	if err != nil || len(list) != 1 || list[0].Name != name || list[0].Size != int64(len(data)) {
		t.Fatalf("list %+v %v", list, err)
	}
	f, size, err := s.Open(ctx, name)
	if err != nil || size != int64(len(data)) {
		t.Fatalf("open %d %v", size, err)
	}
	defer f.Close() // an open copy cannot be removed at the end
	for _, r := range []struct{ off, n int }{{0, 100}, {smbChunk - 50, 100}, {2*smbChunk + 7, smbChunk + 1}, {len(data) - 10, 10}} {
		got := make([]byte, r.n)
		if n, err := f.ReadAt(got, int64(r.off)); err != nil || n != r.n || !bytes.Equal(got, data[r.off:r.off+r.n]) {
			t.Fatalf("read %d at %d: %d %v", r.n, r.off, n, err)
		}
	}
	if n, err := f.ReadAt(make([]byte, 10), int64(len(data)-5)); n != 5 || err != io.EOF {
		t.Fatalf("a read to the end: %d %v, not 5 and io.EOF", n, err)
	}
	if n, err := f.ReadAt(make([]byte, 10), int64(len(data))); n != 0 || err != io.EOF {
		t.Fatalf("a read at the end: %d %v, not io.EOF", n, err)
	}
	whole, err := io.ReadAll(io.NewSectionReader(f, 0, size))
	if err != nil || !bytes.Equal(whole, data) {
		t.Fatalf("the whole copy read back: %d bytes, %v", len(whole), err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	// A put broken off leaves nothing under its name, nor its temporary
	// file; a put of a name there replaces it.
	if err := s.Put(ctx, "broken.zip", &failingReader{n: smbChunk + 7}, 2*smbChunk); err == nil {
		t.Fatal("a put broken off succeeded")
	}
	if got := smbRaw(t, ctx, s, dir); !slices.Equal(got, []string{name}) {
		t.Fatalf("after a put broken off: %v", got)
	}
	if err := s.Put(ctx, name, strings.NewReader("again"), 5); err != nil {
		t.Fatal(err)
	}
	if list, err := s.List(ctx); err != nil || len(list) != 1 || list[0].Size != 5 {
		t.Fatalf("a copy put again %+v %v", list, err)
	}

	// A backup is found by its manifest, read by ranges over SMB.
	st := testStore(t)
	rcp, _ := keyOf(t, KindX25519, "laptop")
	path, m := write(t, st, Inputs{Trigger: TriggerScheduled}, rcp)
	bf, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	fi, _ := bf.Stat()
	err = s.Put(ctx, FileName(m), bf, fi.Size())
	bf.Close()
	if err != nil {
		t.Fatal(err)
	}
	found, skipped, err := Scan(ctx, s)
	if err != nil || len(found) != 1 || found[0].Manifest.UUID != m.UUID || len(skipped) != 1 || skipped[0].Name != name {
		t.Fatalf("scan %+v %+v %v", found, skipped, err)
	}

	// Deleted, and again; a copy not there.
	if err := s.Delete(ctx, name); err != nil {
		t.Fatal(err)
	}
	if err := s.Delete(ctx, name); err != nil {
		t.Fatalf("a name not there: %v", err)
	}
	if _, _, err := s.Open(ctx, name); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("a copy deleted: %v", err)
	}

	// The share's root, with no folder; the user as DOMAIN\user.
	root, err := smbLive(t, nil, pass)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := root.List(ctx); err != nil {
		t.Fatalf("the share's root: %v", err)
	}
	dom, err := smbLive(t, map[string]string{"path": base + "/domain", "user": `WORKGROUP\` + os.Getenv("MASSIMO_TARGET_TEST_SMB_USER")}, pass)
	if err != nil || dom.domain != "WORKGROUP" {
		t.Fatalf("DOMAIN\\user: %+v %v", dom, err)
	}
	if err := dom.Check(ctx); err != nil {
		t.Fatalf("DOMAIN\\user: %v", err)
	}

	// A wrong password, a share not there and a port nothing answers on
	// are each refused, saying what refused, without the password.
	wrong := pass + "-wrong"
	bad, err := smbLive(t, map[string]string{"path": base}, wrong)
	if err != nil {
		t.Fatal(err)
	}
	if err := bad.Check(ctx); err == nil || !strings.Contains(err.Error(), "sign in as") || strings.Contains(err.Error(), wrong) {
		t.Fatalf("a wrong password: %v", err)
	}
	none, err := smbLive(t, map[string]string{"share": "no-such-share"}, pass)
	if err != nil {
		t.Fatal(err)
	}
	if err := none.Check(ctx); err == nil || !strings.Contains(err.Error(), "mount the share no-such-share") {
		t.Fatalf("a share not there: %v", err)
	}
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	closed := l.Addr().String()
	l.Close()
	host, port, _ := net.SplitHostPort(closed)
	gone, err := smbLive(t, map[string]string{"host": host, "port": port}, pass)
	if err != nil {
		t.Fatal(err)
	}
	if err := gone.Check(ctx); err == nil || !strings.Contains(err.Error(), "connect") {
		t.Fatalf("a port nothing answers on: %v", err)
	}
	t.Logf("SMB at %s: a copy of %d bytes put, read by ranges and deleted; refusals: %v", s, len(data), bad.Check(ctx))
}
