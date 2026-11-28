// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package serverbackup

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"math/rand/v2"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"golang.org/x/net/webdav"

	"go.metaworx.ch/massimo/internal/store"
)

// The test server's user and password; the password must never show in
// an error.
const (
	davUser = "massimo"
	davPass = "lemon-tree-17"
)

// davRig is a WebDAV server in memory or on a directory (x/net/webdav),
// its folders under /dav, behind basic authentication as davUser.
type davRig struct {
	srv  *httptest.Server
	fsys webdav.FileSystem
	gets atomic.Int64
}

// newDAV serves fsys, a new one in memory where nil; wrap, where given,
// sees each request first.
func newDAV(t *testing.T, fsys webdav.FileSystem, wrap func(http.Handler) http.Handler) *davRig {
	t.Helper()
	if fsys == nil {
		fsys = webdav.NewMemFS()
	}
	rig := &davRig{fsys: fsys}
	dav := &webdav.Handler{Prefix: "/dav", FileSystem: fsys, LockSystem: webdav.NewMemLS()}
	var h http.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if u, p, ok := r.BasicAuth(); !ok || u != davUser || p != davPass {
			w.Header().Set("WWW-Authenticate", `Basic realm="dav"`)
			http.Error(w, "who are you", http.StatusUnauthorized)
			return
		}
		if r.Method == http.MethodGet {
			rig.gets.Add(1)
		}
		dav.ServeHTTP(w, r)
	})
	if wrap != nil {
		h = wrap(h)
	}
	rig.srv = httptest.NewServer(h)
	t.Cleanup(rig.srv.Close)
	return rig
}

// target is the rig's folder as a target, signed in to with pass.
func (r *davRig) target(t *testing.T, folder, pass string) *WebDAV {
	t.Helper()
	w, err := NewWebDAV(r.srv.URL+folder, davUser, []byte(pass))
	if err != nil {
		t.Fatal(err)
	}
	return w
}

// names lists what the server holds in dir, hidden files too; nil where
// dir is not there.
func (r *davRig) names(t *testing.T, dir string) []string {
	t.Helper()
	f, err := r.fsys.OpenFile(context.Background(), dir, os.O_RDONLY, 0)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	fis, err := f.Readdir(-1)
	if err != nil {
		t.Fatal(err)
	}
	out := []string{}
	for _, fi := range fis {
		out = append(out, fi.Name())
	}
	slices.Sort(out)
	return out
}

// noSecret fails where err shows the password.
func noSecret(t *testing.T, err error) {
	t.Helper()
	if err != nil && strings.Contains(err.Error(), davPass) {
		t.Fatalf("an error shows the password: %v", err)
	}
}

// The kind is registered with its fields, the url's warning matching
// plain http alone; a warning on a secret, or one that does not compile,
// is a mistake of the code.
func TestWebDAVKind(t *testing.T) {
	t.Parallel()
	k, ok := KindOf(KindWebDAV)
	if !ok || len(k.Fields) != 3 {
		t.Fatalf("webdav %+v %v", k, ok)
	}
	url, user, pass := k.Fields[0], k.Fields[1], k.Fields[2]
	if url.Key != "url" || url.Type != FieldText || !url.Required || user.Key != "user" || !user.Required || pass.Key != "password" || pass.Type != FieldSecret || !pass.Required {
		t.Fatalf("fields %+v", k.Fields)
	}
	if s, ok := k.Secret(); !ok || s.Key != "password" {
		t.Fatalf("secret %+v %v", s, ok)
	}
	warn := regexp.MustCompile(url.Warn)
	for v, want := range map[string]bool{
		"http://nas.local:5005/backups": true, "HTTP://nas.local/": true, "Http://x": true,
		"https://dav.example.org/": false, "dav.example.org/http://": false, "httpd.example.org": false,
	} {
		if warn.MatchString(v) != want {
			t.Errorf("warn %q: %v", v, !want)
		}
	}
	cfg, err := k.Check(map[string]string{"url": " dav.example.org/remote.php/dav/files/anna ", "user": "anna"}, true)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := k.Check(cfg, false); err == nil {
		t.Fatal("no password taken")
	}
	tgt, err := k.New(cfg, []byte(davPass))
	if err != nil {
		t.Fatal(err)
	}
	if w := tgt.(*WebDAV); w.base.String() != "https://dav.example.org/remote.php/dav/files/anna/" || w.pass != davPass || w.user != "anna" {
		t.Fatalf("made %v %s", w.base, w.user)
	}
	newT := func(map[string]string, []byte) (Target, error) { return nil, nil }
	for name, bad := range map[string]Kind{
		"a warning on a secret":  {Name: "secretwarn", New: newT, Fields: []Field{{Key: "a", Type: FieldSecret, Warn: "x"}}},
		"a warning not compiled": {Name: "badwarn", New: newT, Fields: []Field{{Key: "a", Type: FieldText, Warn: "("}}},
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

// The URL is https where it names no scheme, http only where it says so;
// one that holds a user, a query or no server is refused, and so is a
// user basic authentication cannot carry.
func TestWebDAVURL(t *testing.T) {
	t.Parallel()
	for raw, want := range map[string]string{
		"dav.example.org/remote.php/dav/files/anna": "https://dav.example.org/remote.php/dav/files/anna/",
		"http://nas.local:5005/backups/":            "http://nas.local:5005/backups/",
		"HTTPS://dav.example.org":                   "https://dav.example.org/",
		"https://dav.example.org/My Backups":        "https://dav.example.org/My%20Backups/",
	} {
		w, err := NewWebDAV(raw, "anna", nil)
		if err != nil || w.base.String() != want {
			t.Errorf("%q: %v %v, want %s", raw, w, err, want)
		}
	}
	w, _ := NewWebDAV("https://dav.example.org/My Backups", "anna", nil)
	for name, want := range map[string]string{
		"a.zip":       "https://dav.example.org/My%20Backups/a.zip",
		".50%.tmp":    "https://dav.example.org/My%20Backups/.50%25.tmp",
		"a b?c#d.zip": "https://dav.example.org/My%20Backups/a%20b%3Fc%23d.zip",
		"%2e%2e":      "https://dav.example.org/My%20Backups/%252e%252e",
	} {
		if u, err := w.at(name); err != nil || u.String() != want {
			t.Errorf("the URL of %q: %v %v, want %s", name, u, err, want)
		}
	}
	for _, name := range []string{"", ".", "..", "a/b", "../a.zip", `a\b`} {
		if u, err := w.at(name); err == nil {
			t.Errorf("the URL of %q: %s", name, u)
		}
	}
	for _, raw := range []string{"", "ftp://dav.example.org/", "https://anna:pw@dav.example.org/", "https://dav.example.org/?a=b",
		"https://dav.example.org/#x", "https:///backups", "https://dav example.org/"} {
		if _, err := NewWebDAV(raw, "anna", nil); err == nil {
			t.Errorf("%q taken", raw)
		}
	}
	for _, user := range []string{"", "an:na"} {
		if _, err := NewWebDAV("dav.example.org", user, nil); err == nil {
			t.Errorf("user %q taken", user)
		}
	}
}

// A copy goes there by a temporary name, the folders missing made; it is
// listed, read by ranges, a read through coming from a GET per chunk, and
// deleted; a name not there is fs.ErrNotExist to Open and no error to
// Delete.
func TestWebDAVRoundTrip(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	rig := newDAV(t, nil, nil)
	w := rig.target(t, "/dav/backups/massimo", davPass)
	if list, err := w.List(ctx); err != nil || len(list) != 0 {
		t.Fatalf("a folder not made: %v %v", list, err)
	}
	if err := w.Check(ctx); err != nil {
		t.Fatal(err)
	}
	if got := rig.names(t, "/backups/massimo"); got == nil || len(got) != 0 {
		t.Fatalf("the check left %v", got)
	}
	data := make([]byte, 2*davChunk+12345)
	rand.NewChaCha8([32]byte{1}).Read(data)
	size := int64(len(data))
	if err := w.Put(ctx, "a.zip", bytes.NewReader(data), size); err != nil {
		t.Fatal(err)
	}
	if got := rig.names(t, "/backups/massimo"); !slices.Equal(got, []string{"a.zip"}) {
		t.Fatalf("the folder holds %v", got)
	}
	list, err := w.List(ctx)
	if err != nil || len(list) != 1 || list[0].Name != "a.zip" || list[0].Size != size || time.Since(list[0].Time) > time.Hour {
		t.Fatalf("list %+v %v", list, err)
	}
	f, n, err := w.Open(ctx, "a.zip")
	if err != nil || n != size {
		t.Fatalf("open %d %v", n, err)
	}
	p := make([]byte, 100)
	for _, off := range []int64{size - 100, 0, davChunk - 50, davChunk + 7} {
		if k, err := f.ReadAt(p, off); k != 100 || err != nil || !bytes.Equal(p, data[off:off+100]) {
			t.Fatalf("at %d: %d %v", off, k, err)
		}
	}
	if k, err := f.ReadAt(make([]byte, 200), size-100); k != 100 || err != io.EOF {
		t.Fatalf("over the end: %d %v", k, err)
	}
	if k, err := f.ReadAt(p, size); k != 0 || err != io.EOF {
		t.Fatalf("at the end: %d %v", k, err)
	}
	before := rig.gets.Load()
	all, err := io.ReadAll(io.NewSectionReader(f, 0, size))
	if err != nil || !bytes.Equal(all, data) {
		t.Fatalf("read through: %d bytes %v", len(all), err)
	}
	if gets := rig.gets.Load() - before; gets != 3 {
		t.Fatalf("%d GETs for a read through of three chunks", gets)
	}
	f.Close()
	if _, err := f.ReadAt(p, 0); err == nil {
		t.Fatal("a read after Close")
	}
	if _, _, err := w.Open(ctx, "b.zip"); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("open of a name not there: %v", err)
	}
	if err := w.Put(ctx, "../a.zip", strings.NewReader("data"), 4); err == nil {
		t.Fatal("a name outside taken")
	}
	if err := w.Delete(ctx, "a.zip"); err != nil {
		t.Fatal(err)
	}
	if err := w.Delete(ctx, "a.zip"); err != nil {
		t.Fatalf("a name not there: %v", err)
	}
	if list, err := w.List(ctx); err != nil || len(list) != 0 {
		t.Fatalf("after the delete: %v %v", list, err)
	}
}

// cancelAfter ends its context once n bytes are read.
type cancelAfter struct {
	r      io.Reader
	n      int
	cancel context.CancelFunc
}

func (c *cancelAfter) Read(p []byte) (int, error) {
	if c.n <= 0 {
		c.cancel()
		time.Sleep(10 * time.Millisecond)
	}
	k, err := c.r.Read(p)
	c.n -= k
	return k, err
}

// A put broken off, short, long or of a context ended leaves nothing,
// under the name or beside it.
func TestWebDAVPutBrokenOff(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	rig := newDAV(t, nil, nil)
	w := rig.target(t, "/dav/b", davPass)
	if err := w.Put(ctx, "a.zip", &failingReader{n: 300000}, 1000000); err == nil {
		t.Fatal("a put broken off succeeded")
	}
	if err := w.Put(ctx, "a.zip", strings.NewReader("short"), 1000); err == nil || !strings.Contains(err.Error(), "5 bytes, not 1000") {
		t.Fatalf("a short put: %v", err)
	}
	if err := w.Put(ctx, "a.zip", strings.NewReader("too long"), 3); err == nil || !strings.Contains(err.Error(), "more than 3 bytes") {
		t.Fatalf("a long put: %v", err)
	}
	mid, cancel := context.WithCancel(ctx)
	if err := w.Put(mid, "a.zip", &cancelAfter{r: io.LimitReader(rand.NewChaCha8([32]byte{2}), 8<<20), n: 1 << 20, cancel: cancel}, 8<<20); err == nil {
		t.Fatal("a put of a context ended midway succeeded")
	}
	if err := w.Put(mid, "a.zip", strings.NewReader("data"), 4); err == nil {
		t.Fatal("a put of a context ended succeeded")
	}
	if got := rig.names(t, "/b"); got == nil || len(got) != 0 {
		t.Fatalf("left behind: %v", got)
	}
}

// A stale file whose name is no temporary name of the target's stays,
// whatever its name: one the URL would read as an escape, ".50%.tmp",
// once made a put DELETE the folder itself, with everything in it.
func TestWebDAVStaleOthersStay(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "b"), 0o700); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-2 * partStale)
	for _, name := range []string{".50%.tmp", ".notes.tmp", ".x.zip.tmp", ".x.zip.XY.tmp", "keep.zip", ".check-0a1b.tmp", ".check-0a1b.tmp.2c3d.tmp"} {
		p := filepath.Join(dir, "b", name)
		if err := os.WriteFile(p, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
		os.Chtimes(p, old, old)
	}
	rig := newDAV(t, webdav.Dir(dir), nil)
	w := rig.target(t, "/dav/b", davPass)
	if err := w.Put(ctx, "a.zip", strings.NewReader("abc"), 3); err != nil {
		t.Fatal(err)
	}
	if err := w.Check(ctx); err != nil {
		t.Fatal(err)
	}
	want := []string{".50%.tmp", ".notes.tmp", ".x.zip.XY.tmp", ".x.zip.tmp", "a.zip", "keep.zip"}
	if got := rig.names(t, "/b"); !slices.Equal(got, want) {
		t.Fatalf("the folder holds %v, want %v", got, want)
	}
	for name, ours := range map[string]bool{
		".a.zip.0123abcd.tmp": true, ".check-0a1b.tmp": true, ".check-0a1b.tmp.2c3d.tmp": true,
		".a.zip.tmp": false, ".a.zip.0123.part": false, "a.zip.0123.tmp": false, ".50%.tmp": false,
		".a.txt.0123.tmp": false, ".check-.tmp": false, ".a.zip.0X23.tmp": false, ".tmp": false,
	} {
		if davTemp(name) != ours {
			t.Errorf("davTemp(%q) = %v", name, !ours)
		}
	}
}

// A file there already under the name is not replaced: of the same size
// it is taken for the copy, of another the put says so. A temporary file
// a crash left goes at the next put once it is a day old.
func TestWebDAVOverwriteAndStale(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "b"), 0o700); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-2 * partStale)
	for name, at := range map[string]time.Time{".x.zip.0123" + davTempSuffix: old, ".y.zip.4567" + davTempSuffix: time.Now(), ".notes": old} {
		p := filepath.Join(dir, "b", name)
		if err := os.WriteFile(p, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
		os.Chtimes(p, at, at)
	}
	rig := newDAV(t, webdav.Dir(dir), nil)
	w := rig.target(t, "/dav/b", davPass)
	if err := w.Put(ctx, "a.zip", strings.NewReader("abc"), 3); err != nil {
		t.Fatal(err)
	}
	if got := rig.names(t, "/b"); !slices.Equal(got, []string{".notes", ".y.zip.4567" + davTempSuffix, "a.zip"}) {
		t.Fatalf("the folder holds %v", got)
	}
	if err := w.Put(ctx, "a.zip", strings.NewReader("xyz"), 3); err != nil {
		t.Fatalf("the same backup again: %v", err)
	}
	err := w.Put(ctx, "a.zip", strings.NewReader("abcd"), 4)
	if err == nil || !strings.Contains(err.Error(), "there already, of 3 bytes, not 4") {
		t.Fatalf("another file of the name: %v", err)
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "b", "a.zip")); string(b) != "abc" {
		t.Fatalf("replaced: %q", b)
	}
	if got := rig.names(t, "/b"); len(got) != 3 {
		t.Fatalf("the folder holds %v", got)
	}
	if list, err := w.List(ctx); err != nil || len(list) != 1 || list[0].Name != "a.zip" {
		t.Fatalf("list %+v %v", list, err)
	}
}

// A server that refuses says so, with its status and what it means, and
// never with the password: a 401, a 507 before the body was sent, a
// redirect not followed.
func TestWebDAVRefusals(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	rig := newDAV(t, nil, nil)
	wrong := rig.target(t, "/dav/b", "not-"+davPass)
	err := wrong.Check(ctx)
	if err == nil || !strings.Contains(err.Error(), "401 Unauthorized: the user or the password is refused") || !strings.Contains(err.Error(), "127.0.0.1") {
		t.Fatalf("a wrong password: %v", err)
	}
	noSecret(t, err)
	if err := wrong.Put(ctx, "a.zip", strings.NewReader("abc"), 3); err == nil || !strings.Contains(err.Error(), "401") {
		t.Fatalf("a put with a wrong password: %v", err)
	}

	var sent atomic.Int64
	full := newDAV(t, nil, func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodPut && r.ContentLength > 1000 {
				w.WriteHeader(http.StatusInsufficientStorage)
				return
			}
			h.ServeHTTP(w, r)
		})
	})
	w := full.target(t, "/dav/b", davPass)
	body := io.TeeReader(io.LimitReader(rand.NewChaCha8([32]byte{3}), 1<<20), writerFunc(func(p []byte) { sent.Add(int64(len(p))) }))
	err = w.Put(ctx, "a.zip", body, 1<<20)
	if err == nil || !strings.Contains(err.Error(), "PUT a.zip: 507 Insufficient Storage: the server is full") {
		t.Fatalf("a full server: %v", err)
	}
	noSecret(t, err)
	if n := sent.Load(); n > 64<<10 {
		t.Errorf("%d bytes read for a put the server refused first", n)
	}
	if got := full.names(t, "/b"); len(got) != 0 {
		t.Fatalf("left behind: %v", got)
	}

	moved := newDAV(t, nil, func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, "https://dav.example.org/elsewhere/", http.StatusMovedPermanently)
		})
	})
	err = moved.target(t, "/dav/b", davPass).Check(ctx)
	if err == nil || !strings.Contains(err.Error(), "301 Moved Permanently: the server sends us to https://dav.example.org/elsewhere/") {
		t.Fatalf("a redirect: %v", err)
	}
}

// writerFunc counts what passes.
type writerFunc func([]byte)

func (f writerFunc) Write(p []byte) (int, error) { f(p); return len(p), nil }

// A transfer in which no byte moves is given up after the stall time:
// a GET whose answer stops, a PUT whose server stops reading.
func TestWebDAVStall(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	var hang atomic.Bool
	release := make(chan struct{})
	rig := newDAV(t, nil, func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch {
			case !hang.Load():
				h.ServeHTTP(w, r)
			case r.Method == http.MethodGet:
				w.Header().Set("Content-Range", "bytes 0-999/1000")
				w.Header().Set("Content-Length", "1000")
				w.WriteHeader(http.StatusPartialContent)
				w.Write(make([]byte, 10))
				w.(http.Flusher).Flush()
				<-release
			case r.Method == http.MethodPut:
				r.Body.Read(make([]byte, 1)) // the 100 Continue goes, then nothing is read
				<-release
			default:
				h.ServeHTTP(w, r)
			}
		})
	})
	t.Cleanup(func() { close(release) }) // before the server closes
	w := rig.target(t, "/dav/b", davPass)
	w.stall = 300 * time.Millisecond
	if err := w.Put(ctx, "a.zip", bytes.NewReader(make([]byte, 1000)), 1000); err != nil {
		t.Fatal(err)
	}
	f, _, err := w.Open(ctx, "a.zip")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	hang.Store(true)
	start := time.Now()
	if _, err := f.ReadAt(make([]byte, 100), 0); err == nil || !strings.Contains(err.Error(), "no byte moved for 300ms") {
		t.Fatalf("a GET that stops: %v", err)
	}
	start2 := time.Now()
	if err := w.Put(ctx, "b.zip", io.LimitReader(rand.NewChaCha8([32]byte{4}), 256<<20), 256<<20); err == nil || !strings.Contains(err.Error(), "no byte moved for 300ms") {
		t.Fatalf("a PUT that stops: %v", err)
	}
	if d1, d2 := start2.Sub(start), time.Since(start2); d1 > 10*time.Second || d2 > 20*time.Second {
		t.Fatalf("given up after %v and %v", d1, d2)
	}
}

// Without a scheme the URL is https: the target reaches a TLS server.
func TestWebDAVHTTPSByDefault(t *testing.T) {
	t.Parallel()
	fsys := webdav.NewMemFS()
	dav := &webdav.Handler{Prefix: "/dav", FileSystem: fsys, LockSystem: webdav.NewMemLS()}
	srv := httptest.NewTLSServer(dav)
	t.Cleanup(srv.Close)
	w, err := NewWebDAV(strings.TrimPrefix(srv.URL, "https://")+"/dav/b", davUser, []byte(davPass))
	if err != nil {
		t.Fatal(err)
	}
	if w.base.Scheme != "https" {
		t.Fatalf("scheme %s", w.base.Scheme)
	}
	w.client.Transport = srv.Client().Transport
	if err := w.Check(t.Context()); err != nil {
		t.Fatal(err)
	}
}

// In the catalogue, a WebDAV target takes a backup with its sealed
// password, checks it by its hash, finds it by its manifest in a scan,
// and gives its copy up; a wrong password fails the copy, without
// showing it.
func TestWebDAVInTheCatalogue(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	c, at := catalogue(t)
	rig := newDAV(t, nil, nil)
	ret, _ := json.Marshal(Policy{})
	add := func(name, pass string) store.ServerBackupTarget {
		tg, err := c.Store.AddServerBackupTarget(ctx, store.ServerBackupTarget{Name: name, Kind: KindWebDAV,
			Config: map[string]string{"url": rig.srv.URL + "/dav/" + name, "user": davUser}, Secret: pass,
			Takes: string(TakesEvery), Retention: ret, Enabled: true}, 0)
		if err != nil {
			t.Fatal(err)
		}
		return tg
	}
	nas := add("nas", davPass)
	b, outs, err := c.Make(ctx, MakeRequest{Trigger: TriggerScheduled})
	if err != nil {
		t.Fatalf("make %v %+v", err, outs)
	}
	if cp, ok := b.Copy(nas.UUID); !ok || cp.State != store.CopyThere {
		t.Fatalf("copies %+v %+v", b.Copies, outs)
	}
	if got := rig.names(t, "/nas"); !slices.Equal(got, []string{FileName(mustManifest(t, b))}) {
		t.Fatalf("the folder holds %v", got)
	}
	if _, outs, err := c.Check(ctx, b.UUID, "nas"); err != nil || len(outs) != 1 || outs[0].Err != nil {
		t.Fatalf("check %v %+v", err, outs)
	}
	tgt, err := c.Open(nas)
	if err != nil {
		t.Fatal(err)
	}
	found, skipped, err := Scan(ctx, tgt)
	if err != nil || len(found) != 1 || len(skipped) != 0 || found[0].Manifest.UUID != b.UUID {
		t.Fatalf("scan %+v %+v %v", found, skipped, err)
	}

	wrong := add("wrong", "not-"+davPass)
	at.Add(int64(time.Hour))
	b2, _, err := c.Make(ctx, MakeRequest{Trigger: TriggerScheduled})
	if err != nil {
		t.Fatal(err)
	}
	cp, ok := b2.Copy(wrong.UUID)
	if !ok || cp.State != store.CopyFailed || !strings.Contains(cp.Error, "401") {
		t.Fatalf("a wrong password's copy %+v", cp)
	}
	noSecret(t, errors.New(cp.Error))
	if err := c.DeleteCopy(ctx, b.UUID, "nas"); err != nil {
		t.Fatal(err)
	}
	if got := rig.names(t, "/nas"); slices.Contains(got, FileName(mustManifest(t, b))) {
		t.Fatalf("the copy stayed: %v", got)
	}
}
