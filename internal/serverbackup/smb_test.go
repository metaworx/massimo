// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package serverbackup

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeSMB is a server the SMB target's tests dial instead of one over
// the network: an in-memory share whose calls may fail or hang, and
// which counts its connections.
type fakeSMB struct {
	mu    sync.Mutex
	files map[string]*smbFakeEntry // by path in the share, its parts joined by \
	dirs  map[string]bool
	open  int // connections open
	dials int
	// fail, where set, answers a call (dial, open, write, sync, close,
	// read, stat, readdir, mkdir, rename, remove) on a path with an
	// error, or nil to let it run.
	fail func(op, name string) error
	// hang, where set, makes a call wait until its connection ends.
	hang func(op, name string) bool
	// readAs, where set, changes what a read answers.
	readAs func(name string, b []byte) []byte
}

// smbFakeEntry is a file of the fake share.
type smbFakeEntry struct {
	data []byte
	mod  time.Time
}

func newFakeSMB() *fakeSMB {
	return &fakeSMB{files: map[string]*smbFakeEntry{}, dirs: map[string]bool{}}
}

// smbOn returns an SMB target of the fields given, beside the defaults,
// that dials f, with the timeout given.
func smbOn(t *testing.T, f *fakeSMB, cfg map[string]string, timeout time.Duration) *SMB {
	t.Helper()
	full := map[string]string{"host": "nas.example", "share": "backup", "user": "massimo"}
	for k, v := range cfg {
		full[k] = v
	}
	s, err := NewSMB(full, []byte("s3cret-pass"))
	if err != nil {
		t.Fatal(err)
	}
	s.dial, s.timeout = f.dial, timeout
	return s
}

// call runs the hooks of a call: its failure, or its hang until ctx ends.
func (f *fakeSMB) call(ctx context.Context, op, name string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	f.mu.Lock()
	fail, hang := f.fail, f.hang
	f.mu.Unlock()
	if hang != nil && hang(op, name) {
		<-ctx.Done()
		return ctx.Err()
	}
	if fail != nil {
		return fail(op, name)
	}
	return nil
}

func (f *fakeSMB) dial(ctx context.Context, t *SMB) (smbShare, error) {
	if err := f.call(ctx, "dial", t.share); err != nil {
		return nil, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.open++
	f.dials++
	return &smbFakeShare{f: f, ctx: ctx}, nil
}

// openConns returns the connections open.
func (f *fakeSMB) openConns() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.open
}

// names returns the share's files, sorted.
func (f *fakeSMB) names() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []string
	for n := range f.files {
		out = append(out, n)
	}
	slices.Sort(out)
	return out
}

// put writes a file directly, made at mod.
func (f *fakeSMB) put(name, data string, mod time.Time) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.files[name] = &smbFakeEntry{data: []byte(data), mod: mod}
}

// smbFakeShare is a connection to the fake share.
type smbFakeShare struct {
	f      *fakeSMB
	ctx    context.Context
	closed bool
}

// smbParent is the folder a path is in, "" for the share's root.
func smbParent(name string) string {
	if i := strings.LastIndexByte(name, '\\'); i >= 0 {
		return name[:i]
	}
	return ""
}

func (s *smbFakeShare) OpenFile(name string, flag int, _ fs.FileMode) (smbFile, error) {
	op := "open"
	if flag&os.O_CREATE != 0 {
		op = "create"
	}
	if err := s.f.call(s.ctx, op, name); err != nil {
		return nil, &fs.PathError{Op: op, Path: name, Err: err}
	}
	s.f.mu.Lock()
	defer s.f.mu.Unlock()
	e, ok := s.f.files[name]
	p := smbParent(name)
	switch {
	case flag&os.O_CREATE == 0 && s.f.dirs[name]:
		return &smbFakeFile{s: s, name: name, dir: true}, nil
	case flag&os.O_CREATE == 0 && !ok:
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
	case flag&os.O_EXCL != 0 && ok:
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrExist}
	case p != "" && !s.f.dirs[p]:
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
	case !ok:
		e = &smbFakeEntry{mod: time.Now()}
		s.f.files[name] = e
	}
	return &smbFakeFile{s: s, name: name, e: e}, nil
}

func (s *smbFakeShare) Stat(name string) (fs.FileInfo, error) {
	if err := s.f.call(s.ctx, "stat", name); err != nil {
		return nil, &fs.PathError{Op: "stat", Path: name, Err: err}
	}
	s.f.mu.Lock()
	defer s.f.mu.Unlock()
	if s.f.dirs[name] || name == "" {
		return smbFakeInfo{name: name, dir: true}, nil
	}
	if e, ok := s.f.files[name]; ok {
		return smbFakeInfo{name: name, size: int64(len(e.data)), mod: e.mod}, nil
	}
	return nil, &fs.PathError{Op: "stat", Path: name, Err: fs.ErrNotExist}
}

func (s *smbFakeShare) ReadDir(name string) ([]fs.FileInfo, error) {
	if err := s.f.call(s.ctx, "readdir", name); err != nil {
		return nil, &fs.PathError{Op: "open", Path: name, Err: err}
	}
	s.f.mu.Lock()
	defer s.f.mu.Unlock()
	if name != "" && !s.f.dirs[name] {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
	}
	var out []fs.FileInfo
	for n, e := range s.f.files {
		if smbParent(n) == name {
			out = append(out, smbFakeInfo{name: n[strings.LastIndexByte(n, '\\')+1:], size: int64(len(e.data)), mod: e.mod})
		}
	}
	for d := range s.f.dirs {
		if smbParent(d) == name {
			out = append(out, smbFakeInfo{name: d[strings.LastIndexByte(d, '\\')+1:], dir: true})
		}
	}
	slices.SortFunc(out, func(a, b fs.FileInfo) int { return strings.Compare(a.Name(), b.Name()) })
	return out, nil
}

func (s *smbFakeShare) Mkdir(name string, _ fs.FileMode) error {
	if err := s.f.call(s.ctx, "mkdir", name); err != nil {
		return &fs.PathError{Op: "mkdir", Path: name, Err: err}
	}
	s.f.mu.Lock()
	defer s.f.mu.Unlock()
	p := smbParent(name)
	switch {
	case s.f.dirs[name]:
		return &fs.PathError{Op: "mkdir", Path: name, Err: fs.ErrExist}
	case p != "" && !s.f.dirs[p]:
		return &fs.PathError{Op: "mkdir", Path: name, Err: fs.ErrNotExist}
	}
	s.f.dirs[name] = true
	return nil
}

func (s *smbFakeShare) Rename(oldpath, newpath string) error {
	if err := s.f.call(s.ctx, "rename", oldpath); err != nil {
		return &os.LinkError{Op: "rename", Old: oldpath, New: newpath, Err: err}
	}
	s.f.mu.Lock()
	defer s.f.mu.Unlock()
	e, ok := s.f.files[oldpath]
	if !ok {
		return &os.LinkError{Op: "rename", Old: oldpath, New: newpath, Err: fs.ErrNotExist}
	}
	delete(s.f.files, oldpath)
	s.f.files[newpath] = e
	return nil
}

func (s *smbFakeShare) Remove(name string) error {
	if err := s.f.call(s.ctx, "remove", name); err != nil {
		return &fs.PathError{Op: "remove", Path: name, Err: err}
	}
	s.f.mu.Lock()
	defer s.f.mu.Unlock()
	if _, ok := s.f.files[name]; !ok {
		return &fs.PathError{Op: "remove", Path: name, Err: fs.ErrNotExist}
	}
	delete(s.f.files, name)
	return nil
}

func (s *smbFakeShare) Close() error {
	s.f.mu.Lock()
	defer s.f.mu.Unlock()
	if !s.closed {
		s.closed = true
		s.f.open--
	}
	return nil
}

// smbFakeFile is a file of the fake share, open: a write lands at once, as
// a server's does.
type smbFakeFile struct {
	s    *smbFakeShare
	name string
	e    *smbFakeEntry
	dir  bool
}

func (f *smbFakeFile) Write(p []byte) (int, error) {
	if err := f.s.f.call(f.s.ctx, "write", f.name); err != nil {
		return 0, &fs.PathError{Op: "write", Path: f.name, Err: err}
	}
	f.s.f.mu.Lock()
	defer f.s.f.mu.Unlock()
	f.e.data = append(f.e.data, p...)
	return len(p), nil
}

func (f *smbFakeFile) ReadAt(p []byte, off int64) (int, error) {
	if err := f.s.f.call(f.s.ctx, "read", f.name); err != nil {
		return 0, &fs.PathError{Op: "read", Path: f.name, Err: err}
	}
	f.s.f.mu.Lock()
	data := f.e.data
	readAs := f.s.f.readAs
	f.s.f.mu.Unlock()
	if readAs != nil {
		data = readAs(f.name, data)
	}
	if off >= int64(len(data)) {
		return 0, io.EOF
	}
	// As go-smb2 does: a read that ends within p has no error.
	return copy(p, data[off:]), nil
}

func (f *smbFakeFile) Stat() (fs.FileInfo, error) {
	if err := f.s.f.call(f.s.ctx, "stat", f.name); err != nil {
		return nil, err
	}
	if f.dir {
		return smbFakeInfo{name: f.name, dir: true}, nil
	}
	f.s.f.mu.Lock()
	defer f.s.f.mu.Unlock()
	return smbFakeInfo{name: f.name, size: int64(len(f.e.data)), mod: f.e.mod}, nil
}

func (f *smbFakeFile) Sync() error  { return f.s.f.call(f.s.ctx, "sync", f.name) }
func (f *smbFakeFile) Close() error { return f.s.f.call(f.s.ctx, "close", f.name) }

// smbFakeInfo is a file's or a folder's information on the fake share.
type smbFakeInfo struct {
	name string
	size int64
	mod  time.Time
	dir  bool
}

func (i smbFakeInfo) Name() string       { return i.name }
func (i smbFakeInfo) Size() int64        { return i.size }
func (i smbFakeInfo) ModTime() time.Time { return i.mod }
func (i smbFakeInfo) IsDir() bool        { return i.dir }
func (i smbFakeInfo) Sys() any           { return nil }
func (i smbFakeInfo) Mode() fs.FileMode {
	if i.dir {
		return fs.ModeDir | 0o700
	}
	return 0o600
}

// The SMB kind is registered with its fields, the password its one
// secret, and makes a target of them that names itself without it.
func TestSMBKind(t *testing.T) {
	t.Parallel()
	k, ok := KindOf(KindSMB)
	if !ok {
		t.Fatal("no smb kind")
	}
	var keys []string
	for _, f := range k.Fields {
		keys = append(keys, f.Key)
	}
	if strings.Join(keys, " ") != "host port share path user password domain" {
		t.Fatalf("fields %v", keys)
	}
	if f, ok := k.Secret(); !ok || f.Key != "password" || !f.Required {
		t.Fatalf("secret %+v %v", f, ok)
	}
	if _, err := k.Check(map[string]string{"host": "nas", "share": "backup", "user": "massimo"}, false); err == nil {
		t.Fatal("a target without its password passed")
	}
	cfg, err := k.Check(map[string]string{"host": " nas.example ", "share": "backup", "path": "/massimo/box/", "user": "massimo"}, true)
	if err != nil || cfg["port"] != "445" {
		t.Fatalf("check %v %v", cfg, err)
	}
	tgt, err := k.New(cfg, []byte("s3cret-pass"))
	if err != nil {
		t.Fatal(err)
	}
	s := tgt.(*SMB)
	if s.String() != "//nas.example/backup/massimo/box" || s.dir != `massimo\box` || s.addr != "nas.example:445" {
		t.Fatalf("target %s %q %q", s, s.dir, s.addr)
	}
	if shown := fmt.Sprintf("%v %+v %#v %s", s, s, s, s); strings.Contains(shown, "s3cret") {
		t.Fatalf("the password shown: %s", shown)
	}
}

// A target's fields are read as an SMB server takes them, and what it
// would not take is refused.
func TestNewSMB(t *testing.T) {
	t.Parallel()
	base := map[string]string{"host": "nas", "share": "backup", "user": "massimo"}
	with := func(k, v string) map[string]string {
		m := map[string]string{}
		for a, b := range base {
			m[a] = b
		}
		m[k] = v
		return m
	}
	for name, c := range map[string]struct {
		cfg                    map[string]string
		label, addr, user, dom string
	}{
		"plain":        {base, "//nas/backup", "nas:445", "massimo", ""},
		"a port":       {with("port", "4450"), "//nas:4450/backup", "nas:4450", "massimo", ""},
		"IPv4":         {with("host", "192.0.2.10"), "//192.0.2.10/backup", "192.0.2.10:445", "massimo", ""},
		"IPv6":         {with("host", "[2001:db8::1]"), "//[2001:db8::1]/backup", "[2001:db8::1]:445", "massimo", ""},
		"a share of $": {with("share", "backup$"), "//nas/backup$", "nas:445", "massimo", ""},
		"backslashes":  {with("path", `\massimo\\box\`), "//nas/backup/massimo/box", "nas:445", "massimo", ""},
		"DOMAIN\\user": {with("user", `HOME\massimo`), "//nas/backup", "nas:445", "massimo", "HOME"},
	} {
		s, err := NewSMB(c.cfg, nil)
		if err != nil || s.label != c.label || s.addr != c.addr || s.user != c.user || s.domain != c.dom {
			t.Errorf("%s: %+v %v", name, s, err)
		}
	}
	dom := with("user", `HOME\massimo`)
	dom["domain"] = "OFFICE"
	if s, err := NewSMB(dom, nil); err != nil || s.user != `HOME\massimo` || s.domain != "OFFICE" {
		t.Errorf("a domain given keeps the user whole: %+v %v", s, err)
	}
	for name, cfg := range map[string]map[string]string{
		"a URL":        with("host", "smb://nas"),
		"a UNC path":   with("host", `\\nas`),
		"a host path":  with("host", "nas/backup"),
		"port 0":       with("port", "0"),
		"port 70000":   with("port", "70000"),
		"a share path": with("share", "backup/massimo"),
		"no share":     with("share", ""),
		"..":           with("path", "massimo/../etc"),
		"a colon":      with("path", "c:/massimo"),
		"a dot at end": with("path", "massimo."),
		"no user":      with("user", ""),
	} {
		if s, err := NewSMB(cfg, nil); err == nil {
			t.Errorf("%s: taken as %s", name, s)
		}
	}
}

// A put goes by a temporary name into a folder made where missing, and
// a copy is listed, read by ranges and deleted, each call on a
// connection of its own that ends with it.
func TestSMBRoundTrip(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := newFakeSMB()
	s := smbOn(t, f, map[string]string{"path": "massimo/box"}, time.Minute)
	if list, err := s.List(ctx); err != nil || len(list) != 0 {
		t.Fatalf("a folder not made yet: %v %v", list, err)
	}
	data := bytes.Repeat([]byte("massimo "), smbChunk/8*2+100) // three writes
	var renamed []string
	f.fail = func(op, name string) error {
		if op == "rename" {
			renamed = append(renamed, name)
		}
		return nil
	}
	if err := s.Put(ctx, "a.zip", bytes.NewReader(data), int64(len(data))); err != nil {
		t.Fatal(err)
	}
	if len(renamed) != 1 || !strings.HasPrefix(renamed[0], `massimo\box\.a.zip.`) || !strings.HasSuffix(renamed[0], partSuffix) {
		t.Fatalf("not put by a temporary name: %v", renamed)
	}
	if got := f.names(); !slices.Equal(got, []string{`massimo\box\a.zip`}) || !f.dirs["massimo"] || !f.dirs[`massimo\box`] {
		t.Fatalf("on the share %v %v", got, f.dirs)
	}
	f.put(`massimo\box\notes.txt`, "x", time.Now())
	f.put(`massimo\box\.b.zip.1`+partSuffix, "x", time.Now())
	f.mu.Lock()
	f.dirs[`massimo\box\sub.zip`] = true
	f.mu.Unlock()
	list, err := s.List(ctx)
	if err != nil || len(list) != 1 || list[0].Name != "a.zip" || list[0].Size != int64(len(data)) {
		t.Fatalf("list %+v %v", list, err)
	}
	r, size, err := s.Open(ctx, "a.zip")
	if err != nil || size != int64(len(data)) {
		t.Fatalf("open %d %v", size, err)
	}
	part := make([]byte, 8)
	if n, err := r.ReadAt(part, 8*1000); err != nil || n != 8 || string(part) != "massimo " {
		t.Fatalf("a ranged read %q %v", part, err)
	}
	if n, err := r.ReadAt(part, size-4); n != 4 || err != io.EOF {
		t.Fatalf("a read to the end: %d %v, not 4 and io.EOF itself", n, err)
	}
	if n, err := r.ReadAt(part, size); n != 0 || err != io.EOF {
		t.Fatalf("a read at the end: %d %v, not io.EOF itself", n, err)
	}
	if f.openConns() != 1 {
		t.Fatalf("%d connections while a copy is open, not its own", f.openConns())
	}
	if err := r.Close(); err != nil || r.Close() != nil {
		t.Fatalf("close %v", err)
	}
	if _, _, err := s.Open(ctx, "gone.zip"); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("a copy not there: %v", err)
	}
	if _, _, err := s.Open(ctx, "sub.zip"); err == nil || !strings.Contains(err.Error(), "no regular file") {
		t.Fatalf("a folder opened: %v", err)
	}
	if err := s.Delete(ctx, "a.zip"); err != nil {
		t.Fatal(err)
	}
	if err := s.Delete(ctx, "a.zip"); err != nil {
		t.Fatalf("a name not there: %v", err)
	}
	for _, bad := range []string{"../a.zip", `x\a.zip`, ".hidden.zip"} {
		if err := s.Put(ctx, bad, strings.NewReader("x"), 1); err == nil {
			t.Errorf("put %q", bad)
		}
	}
	if f.openConns() != 0 || f.dials < 8 {
		t.Fatalf("%d connections open after %d", f.openConns(), f.dials)
	}
}

// A put broken off, short, too long, refused at its rename or ended by
// its context leaves nothing under the name, and its temporary file is
// removed; a stale one a crash left goes at the next put.
func TestSMBPutFailures(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := newFakeSMB()
	s := smbOn(t, f, nil, time.Minute)
	if err := s.Put(ctx, "a.zip", &failingReader{n: smbChunk + 10}, 3*smbChunk); err == nil || !strings.Contains(err.Error(), "reading the backup") {
		t.Fatalf("a put broken off: %v", err)
	}
	if err := s.Put(ctx, "a.zip", strings.NewReader("short"), 1000); err == nil || !strings.Contains(err.Error(), "5 bytes, not 1000") {
		t.Fatalf("a short put: %v", err)
	}
	if err := s.Put(ctx, "a.zip", strings.NewReader("too long"), 3); err == nil {
		t.Fatal("a long put succeeded")
	}
	ended, cancel := context.WithCancel(ctx)
	cancel()
	if err := s.Put(ended, "a.zip", strings.NewReader("data"), 4); err == nil {
		t.Fatal("a put of a context ended succeeded")
	}
	f.fail = func(op, _ string) error {
		if op == "rename" {
			return fs.ErrPermission
		}
		return nil
	}
	err := s.Put(ctx, "a.zip", strings.NewReader("data"), 4)
	if !errors.Is(err, fs.ErrPermission) || !strings.Contains(err.Error(), "SMB target //nas.example/backup: rename") {
		t.Fatalf("a rename refused: %v", err)
	}
	f.fail = func(op, _ string) error {
		if op == "write" {
			return errors.New("An operation failed because the disk was full")
		}
		return nil
	}
	if err := s.Put(ctx, "a.zip", strings.NewReader("data"), 4); err == nil || !strings.Contains(err.Error(), "disk was full") {
		t.Fatalf("a full disk: %v", err)
	}
	f.fail = nil
	if left := f.names(); len(left) != 0 {
		t.Fatalf("left on the share: %v", left)
	}
	old := time.Now().Add(-2 * partStale)
	f.put(".b.zip.1"+partSuffix, "x", old)
	f.put(".c.zip.1"+partSuffix, "x", time.Now())
	f.put("old.zip", "x", old)
	if err := s.Put(ctx, "a.zip", strings.NewReader("data"), 4); err != nil {
		t.Fatal(err)
	}
	if got := f.names(); !slices.Equal(got, []string{".c.zip.1" + partSuffix, "a.zip", "old.zip"}) {
		t.Fatalf("after a put, a stale part should be gone, a fresh one and the backups stay: %v", got)
	}
	if f.openConns() != 0 {
		t.Fatalf("%d connections left open", f.openConns())
	}
}

// Check writes, reads back and deletes a small file, in a folder it
// makes; a refusal at any step, or other bytes read back, fails it with
// what refused.
func TestSMBCheck(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := newFakeSMB()
	s := smbOn(t, f, map[string]string{"path": "massimo"}, time.Minute)
	if err := s.Check(ctx); err != nil {
		t.Fatal(err)
	}
	if len(f.names()) != 0 || !f.dirs["massimo"] {
		t.Fatalf("after a check %v %v", f.names(), f.dirs)
	}
	for op, want := range map[string]string{
		"dial":   "connect",
		"mkdir":  "mkdir",
		"create": "create",
		"write":  "write",
		"read":   "read",
		"remove": "remove",
	} {
		f.dirs = map[string]bool{}
		f.fail = func(o, _ string) error {
			if o == op {
				return fmt.Errorf("%s refused", want)
			}
			return nil
		}
		err := s.Check(ctx)
		if err == nil || !strings.Contains(err.Error(), want+" refused") || !strings.Contains(err.Error(), "//nas.example/backup/massimo") {
			t.Errorf("%s refused: %v", op, err)
		}
		if op != "remove" && len(f.names()) != 0 {
			t.Errorf("%s refused, left %v", op, f.names())
		}
		f.files = map[string]*smbFakeEntry{}
	}
	f.fail = nil
	f.readAs = func(_ string, b []byte) []byte { return bytes.ToUpper(b) }
	if err := s.Check(ctx); err == nil || !strings.Contains(err.Error(), "other bytes") {
		t.Fatalf("other bytes read back: %v", err)
	}
	if f.openConns() != 0 {
		t.Fatalf("%d connections left open", f.openConns())
	}
}

// A call that does not answer within the timeout ends its connection,
// and says so; the dial is bounded alike.
func TestSMBTimeout(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := newFakeSMB()
	s := smbOn(t, f, nil, 50*time.Millisecond)
	f.hang = func(op, _ string) bool { return op == "write" }
	start := time.Now()
	err := s.Put(ctx, "a.zip", strings.NewReader("data"), 4)
	if err == nil || !strings.Contains(err.Error(), "no answer within 50ms") || !errors.Is(err, context.Canceled) {
		t.Fatalf("a write that hangs: %v", err)
	}
	if d := time.Since(start); d > 5*time.Second {
		t.Fatalf("a write that hangs took %s", d)
	}
	f.hang = func(op, _ string) bool { return op == "dial" }
	if err := s.Check(ctx); err == nil || !strings.Contains(err.Error(), "no answer within") {
		t.Fatalf("a dial that hangs: %v", err)
	}
	f.hang = func(op, _ string) bool { return op == "read" }
	f.put("b.zip", "data", time.Now())
	r, _, err := s.Open(ctx, "b.zip")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.ReadAt(make([]byte, 4), 0); err == nil || !strings.Contains(err.Error(), "no answer within") {
		t.Fatalf("a read that hangs: %v", err)
	}
	r.Close()
	if f.openConns() != 0 {
		t.Fatalf("%d connections left open", f.openConns())
	}
}

// An error says which target refused, and never holds its password.
func TestSMBErrorsNameTheTarget(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := newFakeSMB()
	s := smbOn(t, f, map[string]string{"port": "1445", "path": "massimo"}, time.Minute)
	f.fail = func(op, _ string) error {
		if op == "dial" {
			return errors.New("sign in as massimo: The attempted logon is invalid")
		}
		return nil
	}
	for name, err := range map[string]error{
		"check":  s.Check(ctx),
		"put":    s.Put(ctx, "a.zip", strings.NewReader("x"), 1),
		"list":   func() error { _, err := s.List(ctx); return err }(),
		"open":   func() error { _, _, err := s.Open(ctx, "a.zip"); return err }(),
		"delete": s.Delete(ctx, "a.zip"),
	} {
		if err == nil || !strings.Contains(err.Error(), "SMB target //nas.example:1445/backup/massimo: sign in as massimo") || strings.Contains(err.Error(), "s3cret") {
			t.Errorf("%s: %v", name, err)
		}
	}
}
