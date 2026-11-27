// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package serverbackup

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

// sftpRig is pkg/sftp's server behind an in-process SSH server on a
// loopback port: its host key, the client key it admits, and the
// directory it serves, which a relative path is taken from.
type sftpRig struct {
	addr  string
	root  string
	ln    net.Listener
	wg    sync.WaitGroup
	auths atomic.Int64 // the keys offered to it

	mu    sync.Mutex
	host  ssh.Signer
	admit ssh.PublicKey
	conns []net.Conn
}

// newSigner makes an Ed25519 key.
func newSigner(t *testing.T) ssh.Signer {
	t.Helper()
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	s, err := ssh.NewSignerFromKey(priv)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// startSFTPServer starts a rig that admits the key admit.
func startSFTPServer(t *testing.T, admit ssh.PublicKey) *sftpRig {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	r := &sftpRig{addr: ln.Addr().String(), root: t.TempDir(), ln: ln, host: newSigner(t), admit: admit}
	t.Cleanup(func() {
		ln.Close()
		r.mu.Lock()
		for _, c := range r.conns {
			c.Close()
		}
		r.mu.Unlock()
		r.wg.Wait()
	})
	r.wg.Add(1)
	go r.serve()
	return r
}

// serve accepts connections until the listener closes.
func (r *sftpRig) serve() {
	defer r.wg.Done()
	for {
		c, err := r.ln.Accept()
		if err != nil {
			return
		}
		r.mu.Lock()
		r.conns = append(r.conns, c)
		r.mu.Unlock()
		r.wg.Add(1)
		go func() {
			defer r.wg.Done()
			r.handle(c)
		}()
	}
}

// handle serves one connection: SSH, and SFTP in its sessions.
func (r *sftpRig) handle(c net.Conn) {
	defer c.Close()
	cfg := &ssh.ServerConfig{PublicKeyCallback: func(_ ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
		r.auths.Add(1)
		r.mu.Lock()
		defer r.mu.Unlock()
		if r.admit != nil && bytes.Equal(key.Marshal(), r.admit.Marshal()) {
			return &ssh.Permissions{}, nil
		}
		return nil, errors.New("not admitted")
	}}
	r.mu.Lock()
	cfg.AddHostKey(r.host)
	r.mu.Unlock()
	sc, chans, reqs, err := ssh.NewServerConn(c, cfg)
	if err != nil {
		return
	}
	defer sc.Close()
	go ssh.DiscardRequests(reqs)
	for nc := range chans {
		if nc.ChannelType() != "session" {
			nc.Reject(ssh.UnknownChannelType, "sessions alone")
			continue
		}
		ch, creqs, err := nc.Accept()
		if err != nil {
			return
		}
		r.wg.Add(1)
		go func() {
			defer r.wg.Done()
			defer ch.Close()
			for req := range creqs {
				ok := req.Type == "subsystem" && len(req.Payload) > 4 && string(req.Payload[4:]) == "sftp"
				req.Reply(ok, nil)
				if !ok {
					continue
				}
				go ssh.DiscardRequests(creqs)
				if srv, err := sftp.NewServer(ch, sftp.WithServerWorkingDirectory(r.root)); err == nil {
					srv.Serve()
				}
				return
			}
		}()
	}
}

// hostKey is the rig's host key's fingerprint.
func (r *sftpRig) hostKey() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return ssh.FingerprintSHA256(r.host.PublicKey())
}

// config is a target's fields for the rig, the host's key confirmed,
// with the changes given ("" removes a field).
func (r *sftpRig) config(t *testing.T, change map[string]string) map[string]string {
	t.Helper()
	host, port, _ := net.SplitHostPort(r.addr)
	in := map[string]string{"host": host, "port": port, "user": "backup", "path": "backups", "host_key": r.hostKey()}
	for k, v := range change {
		in[k] = v
	}
	k, _ := KindOf(KindSFTP)
	cfg, err := k.Check(in, true)
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}

// sftpKey makes a target's key and returns it with its public part.
func sftpKey(t *testing.T) (string, ssh.PublicKey) {
	t.Helper()
	secret, err := MakeSFTPKey()
	if err != nil {
		t.Fatal(err)
	}
	signer, err := ssh.ParsePrivateKey([]byte(secret))
	if err != nil {
		t.Fatal(err)
	}
	return secret, signer.PublicKey()
}

// sftpOf makes the target of a configuration.
func sftpOf(t *testing.T, cfg map[string]string, secret string) Target {
	t.Helper()
	k, _ := KindOf(KindSFTP)
	tgt, err := k.New(cfg, []byte(secret))
	if err != nil {
		t.Fatal(err)
	}
	return tgt
}

// names lists a directory of the rig.
func names(t *testing.T, dir string) []string {
	t.Helper()
	entries, _ := os.ReadDir(dir)
	var out []string
	for _, e := range entries {
		out = append(out, e.Name())
	}
	return out
}

// The kind: its fields, a key it makes and the public line shown of it,
// no secret field; what New refuses.
func TestSFTPKind(t *testing.T) {
	t.Parallel()
	k, ok := KindOf(KindSFTP)
	if !ok || k.MakeSecret == nil || k.Public == nil {
		t.Fatalf("sftp %+v %v", k, ok)
	}
	var keys []string
	for _, f := range k.Fields {
		keys = append(keys, f.Key)
	}
	if strings.Join(keys, " ") != "host port user path host_key" || k.Fields[1].Default != "22" {
		t.Fatalf("fields %+v", k.Fields)
	}
	if _, ok := k.Secret(); ok {
		t.Fatal("a secret field the owner would fill in")
	}
	secret, err := k.MakeSecret()
	if err != nil || !strings.HasPrefix(secret, "-----BEGIN OPENSSH PRIVATE KEY-----") {
		t.Fatalf("made %v", err)
	}
	line, err := k.Public(secret)
	if err != nil || !strings.HasPrefix(line, "ssh-ed25519 AAAA") || !strings.HasSuffix(line, " "+sftpKeyComment) || strings.Contains(line, "PRIVATE") {
		t.Fatalf("public line %q %v", line, err)
	}
	if pub, _, _, _, err := ssh.ParseAuthorizedKey([]byte(line)); err != nil || pub.Type() != ssh.KeyAlgoED25519 {
		t.Fatalf("authorized_keys refuses %q: %v", line, err)
	}
	if other, _ := k.MakeSecret(); other == secret {
		t.Fatal("two targets, one key")
	}
	cfg := map[string]string{"host": "nas.example", "port": "22", "user": "backup"}
	for name, c := range map[string]struct {
		cfg    map[string]string
		secret []byte
		want   string
	}{
		"no key":          {cfg, nil, "no key"},
		"a bad key":       {cfg, []byte("no key"), "its key"},
		"a bad port":      {map[string]string{"host": "nas.example", "port": "70000", "user": "u"}, []byte(secret), "port"},
		"a path for host": {map[string]string{"host": "nas/x", "user": "u"}, []byte(secret), "host"},
		"no user":         {map[string]string{"host": "nas.example"}, []byte(secret), "user"},
		"a bad host key":  {map[string]string{"host": "nas.example", "user": "u", "host_key": "SHA256:abc"}, []byte(secret), "host_key"},
		"a bad key line":  {map[string]string{"host": "nas.example", "user": "u", "host_key": "ssh-ed25519 nope"}, []byte(secret), "host_key"},
	} {
		if _, err := k.New(c.cfg, c.secret); err == nil || !strings.Contains(err.Error(), c.want) || strings.Contains(err.Error(), "PRIVATE") {
			t.Errorf("%s: %v, want %q", name, err, c.want)
		}
	}
	if _, err := k.New(map[string]string{"host": "[::1]", "user": "u"}, []byte(secret)); err != nil {
		t.Errorf("an IPv6 address in brackets: %v", err)
	}
}

// A kind that makes its secret has Public with it, and no secret field.
func TestKindMakesSecret(t *testing.T) {
	t.Parallel()
	newT := func(map[string]string, []byte) (Target, error) { return nil, nil }
	makeS := func() (string, error) { return "s", nil }
	public := func(string) (string, error) { return "p", nil }
	for name, bad := range map[string]Kind{
		"no Public":      {Name: "nopublic", New: newT, MakeSecret: makeS},
		"no MakeSecret":  {Name: "nomake", New: newT, Public: public},
		"a secret field": {Name: "madefield", New: newT, MakeSecret: makeS, Public: public, Fields: []Field{{Key: "password", Type: FieldSecret}}},
	} {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("%s registered", name)
				}
			}()
			Register(bad)
		}()
	}
}

// The round trip: checked, put by a temporary name and over a copy,
// listed, read by ranges, found by a scan by its manifest, deleted.
func TestSFTPRoundTrip(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	secret, pub := sftpKey(t)
	r := startSFTPServer(t, pub)
	tgt := sftpOf(t, r.config(t, nil), secret)
	dir := filepath.Join(r.root, "backups")
	if err := tgt.Check(ctx); err != nil {
		t.Fatal(err)
	}
	if got := names(t, dir); len(got) != 0 {
		t.Fatalf("the probe stayed: %v", got)
	}
	if list, err := tgt.List(ctx); err != nil || len(list) != 0 {
		t.Fatalf("list %v %v", list, err)
	}
	data := make([]byte, 300<<10)
	rand.Read(data)
	if err := tgt.Put(ctx, "a.zip", bytes.NewReader(data), int64(len(data))); err != nil {
		t.Fatal(err)
	}
	if fi, err := os.Stat(filepath.Join(dir, "a.zip")); err != nil || fi.Size() != int64(len(data)) || (fi.Mode().Perm()&0o077 != 0 && os.PathSeparator == '/') {
		t.Fatalf("the file %v %v", fi, err)
	}
	f, size, err := tgt.Open(ctx, "a.zip")
	if err != nil || size != int64(len(data)) {
		t.Fatalf("open %d %v", size, err)
	}
	part := make([]byte, 100)
	if n, err := f.ReadAt(part, 200<<10); err != nil || n != 100 || !bytes.Equal(part, data[200<<10:200<<10+100]) {
		t.Fatalf("ranged read %d %v", n, err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	// Over a copy of the name: replaced whole.
	if err := tgt.Put(ctx, "a.zip", strings.NewReader("again"), 5); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "a.zip")); string(b) != "again" {
		t.Fatalf("replaced %q", b)
	}
	// A real backup, found by its manifest over ranged reads.
	st := testStore(t)
	rec, _ := keyOf(t, KindX25519, "laptop")
	path, m := write(t, st, Inputs{Trigger: TriggerScheduled}, rec)
	bf, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	fi, _ := bf.Stat()
	if err := tgt.Put(ctx, FileName(m), bf, fi.Size()); err != nil {
		t.Fatal(err)
	}
	bf.Close()
	os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("x"), 0o600)
	found, skipped, err := Scan(ctx, tgt)
	if err != nil || len(found) != 1 || found[0].Manifest.UUID != m.UUID || len(skipped) != 1 || skipped[0].Name != "a.zip" {
		t.Fatalf("scan %+v %+v %v", found, skipped, err)
	}
	if err := tgt.Delete(ctx, "a.zip"); err != nil {
		t.Fatal(err)
	}
	if err := tgt.Delete(ctx, "a.zip"); err != nil {
		t.Fatalf("a name not there: %v", err)
	}
	if got := names(t, dir); strings.Join(got, " ") != FileName(m)+" notes.txt" {
		t.Fatalf("left %v", got)
	}
	for _, bad := range []string{"../a.zip", ".a.zip", "a.txt"} {
		if err := tgt.Put(ctx, bad, strings.NewReader("x"), 1); err == nil {
			t.Errorf("put %q", bad)
		}
		if _, _, err := tgt.Open(ctx, bad); err == nil {
			t.Errorf("open %q", bad)
		}
	}
	// A directory not made yet holds none, and a missing file is no file.
	none := sftpOf(t, r.config(t, map[string]string{"path": "none/yet"}), secret)
	if list, err := none.List(ctx); err != nil || len(list) != 0 {
		t.Fatalf("a directory not made: %v %v", list, err)
	}
	if _, _, err := none.Open(ctx, "a.zip"); err == nil || !strings.Contains(err.Error(), "backup@") {
		t.Fatalf("a missing file: %v", err)
	}
}

// A put broken off, short or long leaves nothing under the name, and its
// temporary file goes; one a broken line left goes after a day, and the
// others' hidden files stay.
func TestSFTPPutBrokenOff(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	secret, pub := sftpKey(t)
	r := startSFTPServer(t, pub)
	tgt := sftpOf(t, r.config(t, nil), secret)
	dir := filepath.Join(r.root, "backups")
	if err := tgt.Put(ctx, "a.zip", &failingReader{n: 100 << 10}, 1<<20); err == nil || !strings.Contains(err.Error(), "the line went down") {
		t.Fatalf("a put broken off: %v", err)
	}
	if err := tgt.Put(ctx, "a.zip", strings.NewReader("short"), 1000); err == nil {
		t.Fatal("a short put succeeded")
	}
	if err := tgt.Put(ctx, "a.zip", strings.NewReader("too long"), 3); err == nil {
		t.Fatal("a long put succeeded")
	}
	if got := names(t, dir); len(got) != 0 {
		t.Fatalf("left behind: %v", got)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if err := tgt.Put(cancelled, "a.zip", strings.NewReader("data"), 4); !errors.Is(err, context.Canceled) {
		t.Fatalf("a put of a context ended: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "a.zip")); err == nil {
		t.Fatal("a put of a context ended left the name")
	}
	stale := filepath.Join(dir, ".b.zip.0011aabbccdd"+partSuffix)
	fresh := filepath.Join(dir, ".c.zip.0011aabbccdd"+partSuffix)
	foreign := filepath.Join(dir, ".notes.txt.1"+partSuffix)
	for _, p := range []string{stale, fresh, foreign} {
		os.WriteFile(p, []byte("x"), 0o600)
	}
	old := time.Now().Add(-2 * partStale)
	os.Chtimes(stale, old, old)
	os.Chtimes(foreign, old, old)
	if err := tgt.Put(ctx, "b.zip", strings.NewReader("bb"), 2); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(stale); err == nil {
		t.Fatal("a stale part stayed")
	}
	for _, p := range []string{fresh, foreign} {
		if _, err := os.Stat(p); err != nil {
			t.Fatalf("%s went", filepath.Base(p))
		}
	}
	for n, want := range map[string]bool{".a.zip.00ff" + partSuffix: true, "." + sftpCheckName + ".1" + partSuffix: true, ".a.txt.1" + partSuffix: false, "a.zip" + partSuffix: false, ".a.zip": false} {
		if isPart(n) != want {
			t.Errorf("isPart(%q) = %v", n, !want)
		}
	}
}

// The host's key: not confirmed yet, it is answered to confirm, and
// nothing is offered to the host or written; a key changed is refused
// with both; a key line pins it as its fingerprint does.
func TestSFTPHostKey(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	secret, pub := sftpKey(t)
	r := startSFTPServer(t, pub)
	unpinned := sftpOf(t, r.config(t, map[string]string{"host_key": ""}), secret)
	var hk *HostKeyError
	err := unpinned.Check(ctx)
	if !errors.As(err, &hk) || hk.Fingerprint != r.hostKey() || hk.Pinned != "" || hk.Type != ssh.KeyAlgoED25519 {
		t.Fatalf("not confirmed: %v", err)
	}
	if msg := err.Error(); !strings.Contains(msg, r.hostKey()) || !strings.Contains(msg, "confirm") || !strings.Contains(msg, "backup@127.0.0.1:") {
		t.Fatalf("message %q", msg)
	}
	if err := unpinned.Put(ctx, "a.zip", strings.NewReader("x"), 1); !errors.As(err, &hk) {
		t.Fatalf("a put to a host not confirmed: %v", err)
	}
	if _, err := unpinned.List(ctx); !errors.As(err, &hk) {
		t.Fatalf("a list of a host not confirmed: %v", err)
	}
	if n := r.auths.Load(); n != 0 {
		t.Fatalf("the target's key offered %d times to a host not confirmed", n)
	}
	if got := names(t, r.root); len(got) != 0 {
		t.Fatalf("written to a host not confirmed: %v", got)
	}

	other := ssh.FingerprintSHA256(newSigner(t).PublicKey())
	changed := sftpOf(t, r.config(t, map[string]string{"host_key": other}), secret)
	err = changed.Check(ctx)
	if !errors.As(err, &hk) || hk.Fingerprint != r.hostKey() || hk.Pinned != other {
		t.Fatalf("a changed key: %v", err)
	}
	if msg := err.Error(); !strings.Contains(msg, r.hostKey()) || !strings.Contains(msg, other) || !strings.Contains(msg, "refused") {
		t.Fatalf("message %q", msg)
	}
	if n := r.auths.Load(); n != 0 {
		t.Fatalf("the target's key offered %d times to a changed host", n)
	}

	r.mu.Lock()
	line := string(ssh.MarshalAuthorizedKey(r.host.PublicKey()))
	r.mu.Unlock()
	byLine := sftpOf(t, r.config(t, map[string]string{"host_key": "nas.example " + strings.TrimSpace(line) + " root@nas"}), secret)
	if err := byLine.Check(ctx); err != nil {
		t.Fatalf("pinned by its line: %v", err)
	}
	if err := sftpOf(t, r.config(t, nil), secret).Check(ctx); err != nil {
		t.Fatalf("pinned by its fingerprint: %v", err)
	}
}

// A host that does not admit the target's key says so, and where the
// key's line goes; the key itself is in no error.
func TestSFTPRefusedKey(t *testing.T) {
	t.Parallel()
	secret, _ := sftpKey(t)
	_, otherPub := sftpKey(t)
	r := startSFTPServer(t, otherPub)
	err := sftpOf(t, r.config(t, nil), secret).Check(context.Background())
	if err == nil || !strings.Contains(err.Error(), "refused the target's key") || !strings.Contains(err.Error(), "backup's authorized_keys") {
		t.Fatalf("a refused key: %v", err)
	}
	if strings.Contains(err.Error(), "PRIVATE") || strings.Contains(err.Error(), secret[40:80]) {
		t.Fatalf("the key in the error: %v", err)
	}
	if r.auths.Load() == 0 {
		t.Fatal("no key offered")
	}
}

// A host without posix-rename@openssh.com, whose rename does not replace
// a name: the copy there is removed first. Not parallel: the extensions
// pkg/sftp's server offers are its package's.
func TestSFTPRenameWithoutPosixRename(t *testing.T) {
	if err := sftp.SetSFTPExtensions("statvfs@openssh.com"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		sftp.SetSFTPExtensions("hardlink@openssh.com", "posix-rename@openssh.com", "statvfs@openssh.com")
	})
	ctx := context.Background()
	secret, pub := sftpKey(t)
	r := startSFTPServer(t, pub)
	tgt := sftpOf(t, r.config(t, nil), secret)
	for _, body := range []string{"first", "second"} {
		if err := tgt.Put(ctx, "a.zip", strings.NewReader(body), int64(len(body))); err != nil {
			t.Fatal(err)
		}
	}
	dir := filepath.Join(r.root, "backups")
	if b, _ := os.ReadFile(filepath.Join(dir, "a.zip")); string(b) != "second" {
		t.Fatalf("replaced %q", b)
	}
	if got := names(t, dir); len(got) != 1 {
		t.Fatalf("left %v", got)
	}
}
