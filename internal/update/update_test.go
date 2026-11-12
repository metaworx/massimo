// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package update

import (
	"bytes"
	"cmp"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestCompare(t *testing.T) {
	order := []string{
		"v0.1.0-alpha.2",
		"v0.1.0-alpha.9",
		"v0.1.0-alpha.9-3-gabc1234",
		"v0.1.0-alpha.10",
		"v0.1.0-beta",
		"v0.1.0-beta.1",
		"v0.1.0-rc.1",
		"v0.1.0",
		"v0.1.1",
		"v0.2.0",
		"v1.0.0",
	}
	for i := range order {
		for j := range order {
			a, ok1 := ParseVersion(order[i])
			b, ok2 := ParseVersion(order[j])
			if !ok1 || !ok2 {
				t.Fatalf("parse %s / %s", order[i], order[j])
			}
			want := cmpInt(i, j)
			if got := Compare(a, b); got != want {
				t.Errorf("Compare(%s, %s) = %d, want %d", order[i], order[j], got, want)
			}
		}
	}
}

func TestParseVersion(t *testing.T) {
	v, ok := ParseVersion("v0.1.0-alpha.9-3-g1a2b3c4-dirty")
	if !ok || v.Ahead != 3 || !v.Dirty || v.Tag() || strings.Join(v.Pre, ".") != "alpha.9" {
		t.Fatalf("%+v %v", v, ok)
	}
	if v, ok := ParseVersion("v1.2.3"); !ok || !v.Tag() || v.Major != 1 || v.Patch != 3 {
		t.Fatalf("%+v %v", v, ok)
	}
	for _, bad := range []string{"", "v0.0.0-dev+1a2b3c4d5e6f", "1.2.3", "v1.2", "8308332"} {
		if _, ok := ParseVersion(bad); ok {
			t.Errorf("ParseVersion(%q) accepted", bad)
		}
	}
}

func TestAssetName(t *testing.T) {
	for _, c := range []struct{ goos, arch, arm, want string }{
		{"linux", "arm", "6", "massimo-linux-armv6"},
		{"linux", "arm", "7", "massimo-linux-armv7"},
		{"linux", "arm64", "", "massimo-linux-arm64"},
		{"linux", "amd64", "", "massimo-linux-amd64"},
		{"darwin", "arm64", "", "massimo-darwin-arm64"},
		{"windows", "amd64", "", "massimo-windows-amd64.exe"},
	} {
		if got := assetName(c.goos, c.arch, c.arm); got != c.want {
			t.Errorf("assetName(%s, %s, %s) = %s, want %s", c.goos, c.arch, c.arm, got, c.want)
		}
	}
}

func TestReleaseKeyParses(t *testing.T) {
	if len(ReleaseKey()) != ed25519.PublicKeySize {
		t.Fatal("bad compiled-in key")
	}
	if len(RollingKey()) != ed25519.PublicKeySize {
		t.Fatal("bad compiled-in rolling key")
	}
	if ReleaseKey().Equal(RollingKey()) {
		t.Fatal("the rolling key is the release key")
	}
}

func TestAccepts(t *testing.T) {
	for _, tc := range []struct {
		channel, flavour string
		want             bool
	}{
		{"alpha", "alpha", true},
		{"alpha", "beta", true},
		{"alpha", "stable", true},
		{"alpha", "nightly", false},
		{"alpha", "devel", false},
		{"devel", "devel", true},
		{"devel", "nightly", true},
		{"devel", "stable", true},
		{"nightly", "devel", false},
		{"nightly", "alpha", true},
		{"stable", "beta", false},
		{"stable", "stable", true},
		{"main", "alpha", false}, // a branch's own name takes only itself
		{"ci", "ci", true},
		{"alpha", "ci", false},
	} {
		if got := Accepts(tc.channel, tc.flavour); got != tc.want {
			t.Errorf("Accepts(%q, %q) = %v", tc.channel, tc.flavour, got)
		}
	}
}

func TestLatestRollingBuilds(t *testing.T) {
	pub, priv := testKey(t)
	rollPub, rollPriv := testKey(t)
	gh := newFakeGitHub(t)
	bin := map[string][]byte{"massimo-linux-armv6": []byte("v9")}
	gh.add(t, priv, "v0.12.0-alpha.1", "alpha", bin, nil)
	gh.addRolling(t, rollPriv, "v0.12.0-alpha.1-37-gabc1234", "devel", bin)
	gh.addRolling(t, rollPriv, "v0.12.0-alpha.1-30-gdef5678", "nightly", bin)
	c := &Client{API: gh.srv.URL, Repo: Repo, HTTP: gh.srv.Client(), Key: pub, RollingKey: rollPub}
	ctx := context.Background()

	for channel, want := range map[string]string{
		"devel":   "v0.12.0-alpha.1-37-gabc1234",
		"nightly": "v0.12.0-alpha.1-30-gdef5678",
		"alpha":   "v0.12.0-alpha.1",
		"stable":  "",
	} {
		r, err := c.Latest(ctx, channel)
		if err != nil || (want == "") != (r == nil) || (r != nil && r.Tag != want) {
			t.Errorf("%s: %+v %v, want %q", channel, r, err, want)
		}
	}
	r, err := c.ByTag(ctx, "release/devel")
	if err != nil || r.Tag != "v0.12.0-alpha.1-37-gabc1234" || r.Release != "release/devel" {
		t.Fatalf("ByTag release/devel: %+v %v", r, err)
	}

	// The next alpha release is newer than the devel build before it.
	gh.add(t, priv, "v0.12.0-alpha.2", "alpha", bin, nil)
	if r, err := c.Latest(ctx, "devel"); err != nil || r == nil || r.Tag != "v0.12.0-alpha.2" {
		t.Fatalf("devel after the next alpha: %+v %v", r, err)
	}
}

func TestRollingKeySignsRollingFlavoursAlone(t *testing.T) {
	pub, priv := testKey(t)
	rollPub, rollPriv := testKey(t)
	gh := newFakeGitHub(t)
	bin := map[string][]byte{"massimo-linux-armv6": []byte("v9")}
	gh.add(t, rollPriv, "v0.13.0-alpha.1", "alpha", bin, nil)            // an alpha signed with the rolling key
	gh.addRolling(t, priv, "v0.12.0-alpha.1-2-gabc1234", "nightly", bin) // the release key signs anything
	gh.addManifest(t, rollPriv, Manifest{Tag: "v0.12.0-alpha.1-3-gabc1234", Release: "release/devel", Flavour: "devel"}, bin, nil)
	c := &Client{API: gh.srv.URL, Repo: Repo, HTTP: gh.srv.Client(), Key: pub, RollingKey: rollPub}
	ctx := context.Background()

	if _, err := c.ByTag(ctx, "v0.13.0-alpha.1"); err == nil || !strings.Contains(err.Error(), "does not sign the flavour") {
		t.Fatalf("an alpha signed with the rolling key: %v", err)
	}
	if r, err := c.Latest(ctx, "alpha"); r != nil {
		t.Fatalf("alpha took %+v (%v)", r, err)
	}
	if r, err := c.Latest(ctx, "nightly"); err != nil || r == nil || r.Release != "release/nightly" {
		t.Fatalf("nightly signed with the release key: %+v %v", r, err)
	}
	if r, err := c.Latest(ctx, "devel"); err != nil || r == nil || r.Release != "release/devel" {
		t.Fatalf("devel: %+v %v", r, err)
	}
	// Without the rolling key, its builds do not verify.
	c.RollingKey = nil
	if _, err := c.ByTag(ctx, "release/devel"); err == nil || !strings.Contains(err.Error(), "does not verify") {
		t.Fatalf("without the rolling key: %v", err)
	}
}

func TestManifestVersions(t *testing.T) {
	pub, priv := testKey(t)
	sign := func(m Manifest) ([]byte, []byte) {
		b, _ := json.Marshal(m)
		return b, []byte(base64.StdEncoding.EncodeToString(ed25519.Sign(priv, b)))
	}
	for _, tc := range []struct {
		m  Manifest
		ok bool
	}{
		{Manifest{Tag: "v0.12.0-alpha.1", Flavour: "alpha"}, true},
		{Manifest{Tag: "v0.12.0-alpha.1-3-gabc1234", Flavour: "devel"}, true},
		{Manifest{Tag: "v0.12.0-alpha.1-3-gabc1234", Flavour: "alpha"}, false},
		{Manifest{Tag: "v0.12.0-alpha.1-3-gabc1234-dirty", Flavour: "devel"}, false},
		{Manifest{Tag: "v0.12.0-alpha.1-dirty", Flavour: "alpha"}, false},
		{Manifest{Tag: "release/devel", Flavour: "devel"}, false},
	} {
		b, sig := sign(tc.m)
		if _, err := VerifyManifest(pub, b, sig); (err == nil) != tc.ok {
			t.Errorf("%+v: %v", tc.m, err)
		}
	}
	// A manifest from before the release field still verifies by its tag.
	var old Manifest
	if err := json.Unmarshal([]byte(`{"tag":"v0.12.0-alpha.1","commit":"c","channel":"alpha","prerelease":true,"assets":{}}`), &old); err != nil || old.Flavour != "alpha" || old.Release != "" {
		t.Fatalf("%+v %v", old, err)
	}
}

// fakeGitHub serves a releases list and the release files.
type fakeGitHub struct {
	srv      *httptest.Server
	releases []map[string]any
	files    map[string][]byte
	auth     map[string]string // path → Authorization header seen
	// redirect, where set, is the file host the API's asset URLs send
	// their bytes from, at /dl/ as here.
	redirect string

	mu   sync.Mutex
	fail map[string][]int // path → statuses answered first, in turn
	hang map[string]int   // path → requests left without an answer first
	hits map[string]int   // path → requests
}

// failFirst has path answer each of codes in turn before it serves as usual.
func (f *fakeGitHub) failFirst(path string, codes ...int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.fail[path] = codes
}

// hangFirst has path leave its first n requests without an answer until
// the client gives them up.
func (f *fakeGitHub) hangFirst(path string, n int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.hang[path] = n
}

// count is the number of requests path has had.
func (f *fakeGitHub) count(path string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.hits[path]
}

func newFakeGitHub(t *testing.T) *fakeGitHub {
	f := &fakeGitHub{files: map[string][]byte{}, auth: map[string]string{}, fail: map[string][]int{}, hang: map[string]int{}, hits: map[string]int{}}
	f.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		f.hits[r.URL.Path]++
		var code int
		if codes := f.fail[r.URL.Path]; len(codes) > 0 {
			code, f.fail[r.URL.Path] = codes[0], codes[1:]
		}
		hold := f.hang[r.URL.Path] > 0
		if hold {
			f.hang[r.URL.Path]--
		}
		f.mu.Unlock()
		if hold {
			select {
			case <-r.Context().Done():
			case <-time.After(10 * time.Second): // so that Close never waits longer
			}
			return
		}
		if code != 0 {
			http.Error(w, http.StatusText(code), code)
			return
		}
		f.auth[r.URL.Path] = r.Header.Get("Authorization")
		if r.URL.Path == "/repos/"+Repo+"/releases" {
			json.NewEncoder(w).Encode(f.releases)
			return
		}
		// An asset's API URL: its bytes for fileAccept, its description
		// otherwise, as GitHub's.
		file := r.URL.Path
		if rest, ok := strings.CutPrefix(file, "/repos/"+Repo+"/releases/assets/"); ok {
			switch {
			case r.Header.Get("Accept") != fileAccept:
				json.NewEncoder(w).Encode(map[string]string{"name": rest})
				return
			case f.redirect != "":
				http.Redirect(w, r, f.redirect+"/dl/"+rest, http.StatusFound)
				return
			}
			file = "/dl/" + rest
		}
		b, ok := f.files[file]
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.Write(b)
	}))
	t.Cleanup(f.srv.Close)
	return f
}

// add publishes a release signed with key; edit may change the manifest
// bytes after signing.
func (f *fakeGitHub) add(t *testing.T, key ed25519.PrivateKey, tag, flavour string, binaries map[string][]byte, edit func([]byte) []byte) {
	f.addManifest(t, key, Manifest{Tag: tag, Commit: "c0ffee", Flavour: flavour, Prerelease: true}, binaries, edit)
}

// addRolling publishes a rolling build of the flavour at version, as
// the release release/<flavour>, signed with key.
func (f *fakeGitHub) addRolling(t *testing.T, key ed25519.PrivateKey, version, flavour string, binaries map[string][]byte) {
	f.addManifest(t, key, Manifest{Tag: version, Release: RollingPrefix + flavour, Commit: "c0ffee", Flavour: flavour, Prerelease: true}, binaries, nil)
}

// addManifest publishes the release m names (m.Release, else m.Tag),
// its assets' sums filled in, signed with key; edit may change the
// manifest bytes after signing.
func (f *fakeGitHub) addManifest(t *testing.T, key ed25519.PrivateKey, m Manifest, binaries map[string][]byte, edit func([]byte) []byte) {
	tag := cmp.Or(m.Release, m.Tag)
	m.Assets = map[string]string{}
	var assets []map[string]any
	for name, b := range binaries {
		sum := sha256.Sum256(b)
		m.Assets[name] = hex.EncodeToString(sum[:])
		f.files["/dl/"+tag+"/"+name] = b
		assets = append(assets, f.asset(tag, name))
	}
	mb, _ := json.Marshal(m)
	sig := base64.StdEncoding.EncodeToString(ed25519.Sign(key, mb)) + "\n"
	if edit != nil {
		mb = edit(mb)
	}
	f.files["/dl/"+tag+"/"+ManifestName] = mb
	f.files["/dl/"+tag+"/"+SignatureName] = []byte(sig)
	for _, n := range []string{ManifestName, SignatureName} {
		assets = append(assets, f.asset(tag, n))
	}
	f.releases = append(f.releases, map[string]any{"tag_name": tag, "html_url": "https://example/" + tag, "assets": assets})
}

// asset is a release file as the list names it: its download URL, and
// the API's, which the fake keys by tag and name where GitHub has an id.
func (f *fakeGitHub) asset(tag, name string) map[string]any {
	return map[string]any{
		"name":                 name,
		"browser_download_url": f.srv.URL + "/dl/" + tag + "/" + name,
		"url":                  f.srv.URL + "/repos/" + Repo + "/releases/assets/" + tag + "/" + name,
	}
}

func testKey(t *testing.T) (ed25519.PublicKey, ed25519.PrivateKey) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return pub, priv
}

func TestLatestPicksNewestSignedOnChannel(t *testing.T) {
	pub, priv := testKey(t)
	_, other := testKey(t)
	gh := newFakeGitHub(t)
	bin := map[string][]byte{"massimo-linux-armv6": []byte("v9")}
	gh.add(t, priv, "v0.1.0-alpha.9", "alpha", bin, nil)
	gh.add(t, priv, "v0.1.0-alpha.11", "main", bin, nil)
	gh.add(t, priv, "v0.1.0-alpha.10", "alpha", bin, nil)
	gh.add(t, other, "v0.1.0-alpha.12", "alpha", bin, nil) // wrong key
	gh.add(t, priv, "v0.1.0-alpha.13", "alpha", bin, func(b []byte) []byte {
		return bytes.Replace(b, []byte("alpha"), []byte("beta"), 1) // changed after signing
	})
	c := &Client{API: gh.srv.URL, Repo: Repo, HTTP: gh.srv.Client(), Key: pub}

	r, err := c.Latest(context.Background(), "alpha")
	if err != nil || r == nil || r.Tag != "v0.1.0-alpha.10" {
		t.Fatalf("alpha: %+v %v", r, err)
	}
	// alpha.11 is on main under alpha's label, as the two releases on
	// phase-0 are: Latest goes by the label (hintedChannel).
	if r, err := c.Latest(context.Background(), "main"); r != nil || err != nil {
		t.Fatalf("main, whose one release has alpha's label: %+v %v", r, err)
	}
	if r, _ := c.Latest(context.Background(), "nope"); r != nil {
		t.Fatalf("nope: %+v", r)
	}
	if _, err := c.ByTag(context.Background(), "v0.1.0-alpha.12"); err == nil || !strings.Contains(err.Error(), "does not verify") {
		t.Fatalf("alpha.12: %v", err)
	}
}

func TestManifestOfAnotherRelease(t *testing.T) {
	pub, priv := testKey(t)
	gh := newFakeGitHub(t)
	gh.add(t, priv, "v0.1.0-alpha.5", "alpha", map[string][]byte{"massimo-linux-amd64": []byte("old")}, nil)
	// A newer release carrying the old one's (correctly signed) manifest.
	gh.add(t, priv, "v0.1.0-alpha.6", "alpha", map[string][]byte{"massimo-linux-amd64": []byte("new")}, nil)
	gh.files["/dl/v0.1.0-alpha.6/"+ManifestName] = gh.files["/dl/v0.1.0-alpha.5/"+ManifestName]
	gh.files["/dl/v0.1.0-alpha.6/"+SignatureName] = gh.files["/dl/v0.1.0-alpha.5/"+SignatureName]
	c := &Client{API: gh.srv.URL, Repo: Repo, HTTP: gh.srv.Client(), Key: pub}
	if _, err := c.ByTag(context.Background(), "v0.1.0-alpha.6"); err == nil || !strings.Contains(err.Error(), "carries the manifest of") {
		t.Fatalf("got %v", err)
	}
}

func TestDownloadChecksSHA256(t *testing.T) {
	pub, priv := testKey(t)
	gh := newFakeGitHub(t)
	gh.add(t, priv, "v1.0.0", "main", map[string][]byte{"massimo-linux-amd64": []byte("the binary")}, nil)
	c := &Client{API: gh.srv.URL, Repo: Repo, HTTP: gh.srv.Client(), Key: pub}
	r, err := c.ByTag(context.Background(), "v1.0.0")
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := c.Download(context.Background(), r, "massimo-linux-amd64", &buf); err != nil || buf.String() != "the binary" {
		t.Fatalf("%q %v", buf.String(), err)
	}
	gh.files["/dl/v1.0.0/massimo-linux-amd64"] = []byte("something else")
	if err := c.Download(context.Background(), r, "massimo-linux-amd64", &bytes.Buffer{}); err == nil || !strings.Contains(err.Error(), "SHA-256") {
		t.Fatalf("tampered binary: %v", err)
	}
	if err := c.Download(context.Background(), r, "massimo-linux-armv6", &bytes.Buffer{}); err == nil {
		t.Fatal("asset not in the manifest accepted")
	}
}

func TestRequest(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "update-request")
	if _, ok, err := TakeRequest(p); ok || err != nil {
		t.Fatalf("no file: %v %v", ok, err)
	}
	if err := WriteRequest(p, "alpha"); err != nil {
		t.Fatal(err)
	}
	if c, ok, err := TakeRequest(p); c != "alpha" || !ok || err != nil {
		t.Fatalf("%q %v %v", c, ok, err)
	}
	if _, err := os.Stat(p); !os.IsNotExist(err) {
		t.Fatal("request not removed")
	}
	if err := WriteRequest(p, "../etc"); err == nil {
		t.Fatal("bad channel written")
	}
	os.WriteFile(p, []byte("a b c\n"), 0o644)
	if _, _, err := TakeRequest(p); err == nil {
		t.Fatal("bad channel taken")
	}
	if runtime.GOOS != "windows" {
		secret := filepath.Join(dir, "secret")
		os.WriteFile(secret, []byte("main"), 0o600)
		os.Symlink(secret, p)
		if _, _, err := TakeRequest(p); err == nil {
			t.Fatal("symbolic link followed")
		}
		if _, err := os.Lstat(p); !os.IsNotExist(err) {
			t.Fatal("symbolic link not removed")
		}
	}
}

func TestStatusRunning(t *testing.T) {
	p := filepath.Join(t.TempDir(), "s", "status.json")
	if s, err := ReadStatus(p); err != nil || !s.Time.IsZero() {
		t.Fatalf("%+v %v", s, err)
	}
	if err := WriteStatus(p, Status{State: "installing", To: "v1.0.0"}); err != nil {
		t.Fatal(err)
	}
	s, err := ReadStatus(p)
	if err != nil || s.State != "installing" || !s.Running() {
		t.Fatalf("%+v %v", s, err)
	}
	s.Time = s.Time.Add(-16 * time.Minute)
	if s.Running() {
		t.Fatal("a stale run counts as running")
	}
}

func TestSwapRollbackSmoke(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell scripts as binaries")
	}
	dir := t.TempDir()
	target := filepath.Join(dir, "massimo")
	script := func(v string) []byte {
		return []byte(fmt.Sprintf("#!/bin/sh\necho 'massimo %s (go, test)'\n", v))
	}
	os.WriteFile(target, script("v1.0.0"), 0o755)

	f, err := Stage(target)
	if err != nil {
		t.Fatal(err)
	}
	f.Write(script("v1.1.0"))
	f.Close()
	if err := Smoke(context.Background(), f.Name(), "v1.2.0"); err == nil {
		t.Fatal("wrong version passed the smoke test")
	}
	if err := Smoke(context.Background(), f.Name(), "v1.1.0"); err != nil {
		t.Fatal(err)
	}
	if err := Swap(target); err != nil {
		t.Fatal(err)
	}
	read := func(p string) string { b, _ := os.ReadFile(p); return string(b) }
	if !strings.Contains(read(target), "v1.1.0") || !strings.Contains(read(PrevPath(target)), "v1.0.0") {
		t.Fatal("swap")
	}
	if fi, _ := os.Stat(target); fi.Mode().Perm() != 0o755 {
		t.Fatalf("mode %v", fi.Mode())
	}
	// Manual rollback swaps; a second one goes forward again.
	if err := Rollback(target, PrevPath(target)); err != nil || !strings.Contains(read(target), "v1.0.0") {
		t.Fatal("rollback", err)
	}
	if err := Rollback(target, PrevPath(target)); err != nil || !strings.Contains(read(target), "v1.1.0") {
		t.Fatal("forward", err)
	}
	// After a failed health check the new one is kept aside.
	if err := Rollback(target, FailedPath(target)); err != nil || !strings.Contains(read(target), "v1.0.0") || !strings.Contains(read(FailedPath(target)), "v1.1.0") {
		t.Fatal("rollback after failure", err)
	}
	if err := Rollback(target, PrevPath(target)); err == nil {
		t.Fatal("rollback without .prev")
	}
}

func TestSwapRemovesFailedAndRollbackWithoutTarget(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "massimo.exe")
	read := func(p string) string { b, _ := os.ReadFile(p); return string(b) }
	os.WriteFile(target, []byte("v1"), 0o755)
	os.WriteFile(FailedPath(target), []byte("v0-failed"), 0o755)
	f, err := Stage(target)
	if err != nil {
		t.Fatal(err)
	}
	f.Write([]byte("v2"))
	f.Close()
	if err := Swap(target); err != nil {
		t.Fatal(err)
	}
	if read(target) != "v2" || read(PrevPath(target)) != "v1" {
		t.Fatalf("target %q prev %q", read(target), read(PrevPath(target)))
	}
	if _, err := os.Stat(FailedPath(target)); !os.IsNotExist(err) {
		t.Fatalf(".failed still there: %v", err)
	}
	// The new binary vanished after the swap: the old one goes back.
	os.Remove(target)
	if err := Rollback(target, FailedPath(target)); err != nil || read(target) != "v1" {
		t.Fatalf("rollback without target: %v, target %q", err, read(target))
	}
	if _, err := os.Stat(PrevPath(target)); !os.IsNotExist(err) {
		t.Fatalf(".prev still there: %v", err)
	}
}

func TestAPIOverride(t *testing.T) {
	t.Setenv(TokenVar, "t0k")
	t.Setenv(APIVar, "")
	if c := NewClient(); c.API != "https://api.github.com" || c.Token != "t0k" {
		t.Fatalf("%s %q", c.API, c.Token)
	}
	t.Setenv(APIVar, "http://127.0.0.1:8123/")
	if c := NewClient(); c.API != "http://127.0.0.1:8123" || c.Token != "" {
		t.Fatalf("%s %q", c.API, c.Token)
	}
}

func TestWindowsNamesAndRenamingSwap(t *testing.T) {
	if got := stagedPath(`dir/massimo.exe`); got != filepath.Join("dir", ".massimo.new.exe") {
		t.Fatalf("staged %s", got)
	}
	if got := stagedPath(`dir/massimo`); got != filepath.Join("dir", ".massimo.new") {
		t.Fatalf("staged %s", got)
	}
	dir := t.TempDir()
	target, src, aside := filepath.Join(dir, "massimo.exe"), filepath.Join(dir, "new"), filepath.Join(dir, "massimo.exe.prev")
	os.WriteFile(target, []byte("old"), 0o755)
	os.WriteFile(src, []byte("new"), 0o755)
	os.WriteFile(aside, []byte("older"), 0o755)
	if err := swapRenaming(src, target, aside); err != nil {
		t.Fatal(err)
	}
	read := func(p string) string { b, _ := os.ReadFile(p); return string(b) }
	if read(target) != "new" || read(aside) != "old" {
		t.Fatalf("target %q aside %q", read(target), read(aside))
	}
	// A failing second rename puts the target back.
	if err := swapRenaming(filepath.Join(dir, "missing"), target, aside); err == nil || read(target) != "new" {
		t.Fatalf("%v, target %q", err, read(target))
	}
}

func TestTokenGoesToTheAPIOnly(t *testing.T) {
	pub, priv := testKey(t)
	gh := newFakeGitHub(t)
	gh.add(t, priv, "v0.1.0-alpha.9", "alpha", map[string][]byte{"massimo-linux-armv6": []byte("v9")}, nil)
	list := "/repos/" + Repo + "/releases"

	c := &Client{API: gh.srv.URL, Repo: Repo, HTTP: gh.srv.Client(), Key: pub}
	if _, err := c.Latest(context.Background(), "alpha"); err != nil || gh.auth[list] != "" {
		t.Fatalf("without a token: %v %q", err, gh.auth[list])
	}
	c.Token = "t0k"
	if _, err := c.Latest(context.Background(), "alpha"); err != nil || gh.auth[list] != "Bearer t0k" {
		t.Fatalf("with a token: %v %q", err, gh.auth[list])
	}

	// A release file on another host gets no token.
	var got string
	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { got = r.Header.Get("Authorization") }))
	defer other.Close()
	if _, err := c.get(context.Background(), other.URL+"/file", 10, ""); err != nil || got != "" {
		t.Fatalf("other host: %v %q", err, got)
	}
}

// With a token a release's manifest, signature and binary come through
// the API's asset URLs with it, and the file host the API redirects to
// gets none; without a token, or for an asset URL off the API, they come
// from the download URLs.
func TestTokenFetchesFilesThroughTheAPI(t *testing.T) {
	pub, priv := testKey(t)
	const tag, bin = "v1.0.0", "massimo-linux-amd64"
	files := []string{ManifestName, SignatureName, bin}
	fake := func() *fakeGitHub {
		gh := newFakeGitHub(t)
		gh.add(t, priv, tag, "main", map[string][]byte{bin: []byte("the binary")}, nil)
		return gh
	}
	fetch := func(c *Client) {
		t.Helper()
		r, err := c.ByTag(context.Background(), tag)
		if err != nil {
			t.Fatal(err)
		}
		var buf bytes.Buffer
		if err := c.Download(context.Background(), r, bin, &buf); err != nil || buf.String() != "the binary" {
			t.Fatalf("%q %v", buf.String(), err)
		}
	}
	// requests checks each file's requests on its API and download paths,
	// and the token each was sent.
	requests := func(gh *fakeGitHub, onAPI, onDL int, apiAuth, dlAuth string) {
		t.Helper()
		for _, name := range files {
			api, dl := "/repos/"+Repo+"/releases/assets/"+tag+"/"+name, "/dl/"+tag+"/"+name
			if gh.count(api) != onAPI || gh.count(dl) != onDL || gh.auth[api] != apiAuth || gh.auth[dl] != dlAuth {
				t.Errorf("%s: %d on the API with %q, %d downloads with %q", name, gh.count(api), gh.auth[api], gh.count(dl), gh.auth[dl])
			}
		}
	}

	gh := fake()
	fetch(&Client{API: gh.srv.URL, Repo: Repo, HTTP: gh.srv.Client(), Key: pub})
	requests(gh, 0, 1, "", "")

	gh = fake()
	fetch(&Client{API: gh.srv.URL, Repo: Repo, HTTP: gh.srv.Client(), Key: pub, Token: "t0k"})
	requests(gh, 1, 0, "Bearer t0k", "")

	// The API sends the bytes from another host, as GitHub's file host;
	// the client reaches it, whatever its name, at the fake.
	gh = fake()
	gh.redirect = "http://release-assets.example"
	addr := gh.srv.Listener.Addr().String()
	hc := &http.Client{Transport: &http.Transport{DialContext: func(ctx context.Context, network, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, network, addr)
	}}}
	fetch(&Client{API: gh.srv.URL, Repo: Repo, HTTP: hc, Key: pub, Token: "t0k"})
	requests(gh, 1, 1, "Bearer t0k", "")

	// An asset URL that is not the API's is not taken. (The fake's
	// download URLs are on the API's host, so they get the token here.)
	gh = fake()
	for _, a := range gh.releases[0]["assets"].([]map[string]any) {
		a["url"] = "http://elsewhere.example/assets/" + a["name"].(string)
	}
	fetch(&Client{API: gh.srv.URL, Repo: Repo, HTTP: gh.srv.Client(), Key: pub, Token: "t0k"})
	requests(gh, 0, 1, "", "Bearer t0k")
}

// A channel without a release is no error, also beside a release from
// before the signed manifests; a release whose manifest does not verify
// still is, since it may be the channel's.
func TestLatestWithoutReleaseOnChannel(t *testing.T) {
	pub, priv := testKey(t)
	gh := newFakeGitHub(t)
	bin := map[string][]byte{"massimo-linux-armv6": []byte("v9")}
	gh.add(t, priv, "v0.1.0-alpha.9", "alpha", bin, nil)
	gh.releases = append(gh.releases, map[string]any{"tag_name": "v0.1.0-alpha.10", "html_url": "https://example/v0.1.0-alpha.10", "assets": []any{}})
	c := &Client{API: gh.srv.URL, Repo: Repo, HTTP: gh.srv.Client(), Key: pub}
	if r, err := c.Latest(context.Background(), "stable"); r != nil || err != nil {
		t.Fatalf("stable, which has no release: %+v %v", r, err)
	}
	_, other := testKey(t)
	gh.add(t, other, "v0.2.0", "stable", bin, nil) // a tag without a label
	if r, err := c.Latest(context.Background(), "stable"); r != nil || err == nil || !strings.Contains(err.Error(), "does not verify") {
		t.Fatalf("beside a release that does not verify: %+v %v", r, err)
	}
}

// slowBinary serves a release's binary in pieces with a pause between
// them; pause 0 after the first piece stalls for good.
func slowBinary(t *testing.T, gh *fakeGitHub, tag, name string, body []byte, pieces int, pause time.Duration) {
	t.Helper()
	path := "/dl/" + tag + "/" + name
	slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		step := len(body) / pieces
		for i := 0; i < pieces; i++ {
			end := (i + 1) * step
			if i == pieces-1 {
				end = len(body)
			}
			w.Write(body[i*step : end])
			w.(http.Flusher).Flush()
			select {
			case <-r.Context().Done():
				return
			case <-time.After(pause):
			}
		}
	}))
	t.Cleanup(slow.Close)
	for _, rel := range gh.releases {
		if rel["tag_name"] == tag {
			for _, a := range rel["assets"].([]map[string]any) {
				if a["name"] == name {
					a["browser_download_url"] = slow.URL + path
				}
			}
		}
	}
}

// The download of a binary is bounded by the context and by a pause in
// the data, not by the client's time for the API's answers: a slow line
// that keeps sending gets the binary.
func TestDownloadOutlastsTheClientTimeout(t *testing.T) {
	pub, priv := testKey(t)
	gh := newFakeGitHub(t)
	body := bytes.Repeat([]byte("massimo"), 4096)
	gh.add(t, priv, "v1.0.0", "main", map[string][]byte{"massimo-linux-amd64": body}, nil)
	slowBinary(t, gh, "v1.0.0", "massimo-linux-amd64", body, 6, 100*time.Millisecond)
	c := &Client{API: gh.srv.URL, Repo: Repo, HTTP: &http.Client{Timeout: 250 * time.Millisecond}, Key: pub}
	r, err := c.ByTag(context.Background(), "v1.0.0")
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := c.Download(context.Background(), r, "massimo-linux-amd64", &buf); err != nil || !bytes.Equal(buf.Bytes(), body) {
		t.Fatalf("a slow download: %v, %d of %d bytes", err, buf.Len(), len(body))
	}
}

// A download that stops sending is given up after the idle time.
func TestDownloadGivesUpAStall(t *testing.T) {
	pub, priv := testKey(t)
	gh := newFakeGitHub(t)
	body := bytes.Repeat([]byte("massimo"), 4096)
	gh.add(t, priv, "v1.0.0", "main", map[string][]byte{"massimo-linux-amd64": body}, nil)
	slowBinary(t, gh, "v1.0.0", "massimo-linux-amd64", body, 2, 10*time.Second)
	c := &Client{API: gh.srv.URL, Repo: Repo, HTTP: gh.srv.Client(), Key: pub, Idle: 200 * time.Millisecond}
	r, err := c.ByTag(context.Background(), "v1.0.0")
	if err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	err = c.Download(context.Background(), r, "massimo-linux-amd64", &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "no data for 200ms") || time.Since(start) > 3*time.Second {
		t.Fatalf("a stalled download: %v after %s", err, time.Since(start))
	}
}

// Latest fetches no manifest of a release whose tag, or its target where
// the tag has no label, names another channel (release.yml's rule): a box
// following main, which has no releases, no longer verifies thirty alpha
// releases on every check.
func TestLatestSkipsOtherChannelsUnfetched(t *testing.T) {
	pub, priv := testKey(t)
	gh := newFakeGitHub(t)
	bin := map[string][]byte{"massimo-linux-armv6": []byte("v9")}
	for _, n := range []string{"1", "2", "3", "4", "5"} {
		gh.add(t, priv, "v0.1.0-alpha."+n, "alpha", bin, nil)
	}
	gh.add(t, priv, "v0.2.0", "stable", bin, nil)
	c := &Client{API: gh.srv.URL, Repo: Repo, HTTP: gh.srv.Client(), Key: pub}
	fetched := func() (tags []string) {
		for p := range gh.auth {
			if strings.HasSuffix(p, "/"+ManifestName) || strings.HasSuffix(p, "/"+SignatureName) {
				tags = append(tags, p)
			}
		}
		clear(gh.auth)
		slices.Sort(tags)
		return tags
	}

	if r, err := c.Latest(context.Background(), "main"); r != nil || err != nil {
		t.Fatalf("main: %+v %v", r, err)
	}
	if got := fetched(); len(got) != 0 {
		t.Errorf("main fetched the files of other channels' releases: %v", got)
	}
	// alpha takes the stable release too, which is the newest.
	if r, err := c.Latest(context.Background(), "alpha"); err != nil || r == nil || r.Tag != "v0.2.0" {
		t.Fatalf("alpha: %+v %v", r, err)
	}
	if got, want := fetched(), []string{"/dl/v0.2.0/" + ManifestName, "/dl/v0.2.0/" + SignatureName}; !slices.Equal(got, want) {
		t.Errorf("alpha fetched %v, want the newest release's alone", got)
	}
	if r, err := c.Latest(context.Background(), "stable"); err != nil || r == nil || r.Tag != "v0.2.0" {
		t.Fatalf("stable, by its plain tag: %+v %v", r, err)
	}
	if got := fetched(); len(got) != 2 {
		t.Errorf("stable fetched %v, want v0.2.0's alone", got)
	}
	gh.add(t, priv, "v0.3.0-alpha.1", "alpha", bin, nil)
	if r, err := c.Latest(context.Background(), "stable"); err != nil || r == nil || r.Tag != "v0.2.0" {
		t.Fatalf("stable beside a newer alpha: %+v %v", r, err)
	}
	if got := fetched(); len(got) != 2 {
		t.Errorf("stable fetched %v, want v0.2.0's alone, not the alpha's", got)
	}
}

// fastRetries makes the waits between attempts short for one test.
func fastRetries(t *testing.T) {
	old := retryWaits
	retryWaits = []time.Duration{time.Millisecond, time.Millisecond}
	t.Cleanup(func() { retryWaits = old })
}

// A manifest answered 504 and 502 first is fetched on the third attempt.
func TestRetriesGatewayErrors(t *testing.T) {
	fastRetries(t)
	pub, priv := testKey(t)
	gh := newFakeGitHub(t)
	gh.add(t, priv, "v1.0.0", "main", map[string][]byte{"massimo-linux-amd64": []byte("the binary")}, nil)
	manifest := "/dl/v1.0.0/" + ManifestName
	gh.failFirst(manifest, http.StatusGatewayTimeout, http.StatusBadGateway)
	c := &Client{API: gh.srv.URL, Repo: Repo, HTTP: gh.srv.Client(), Key: pub}
	if _, err := c.ByTag(context.Background(), "v1.0.0"); err != nil || gh.count(manifest) != 3 {
		t.Fatalf("after a 504 and a 502: %v, %d requests", err, gh.count(manifest))
	}
}

// A list that keeps answering 503 is given up after three attempts,
// and the error says so.
func TestRetryGivesUp(t *testing.T) {
	fastRetries(t)
	pub, _ := testKey(t)
	gh := newFakeGitHub(t)
	list := "/repos/" + Repo + "/releases"
	gh.failFirst(list, 503, 503, 503, 503)
	c := &Client{API: gh.srv.URL, Repo: Repo, HTTP: gh.srv.Client(), Key: pub}
	_, err := c.Latest(context.Background(), "alpha")
	if err == nil || !strings.Contains(err.Error(), "503 Service Unavailable (3 attempts)") || gh.count(list) != 3 {
		t.Fatalf("got %v after %d requests", err, gh.count(list))
	}
}

// An answer is not asked again: a rate limit, a missing file.
func TestNoRetryOfAnAnswer(t *testing.T) {
	fastRetries(t)
	pub, _ := testKey(t)
	list := "/repos/" + Repo + "/releases"
	for _, code := range []int{http.StatusForbidden, http.StatusNotFound, http.StatusTooManyRequests, http.StatusInternalServerError} {
		gh := newFakeGitHub(t)
		gh.failFirst(list, code)
		c := &Client{API: gh.srv.URL, Repo: Repo, HTTP: gh.srv.Client(), Key: pub}
		_, err := c.Latest(context.Background(), "alpha")
		if err == nil || strings.Contains(err.Error(), "attempts") || gh.count(list) != 1 {
			t.Errorf("%d: %v after %d requests", code, err, gh.count(list))
		}
	}
}

// A wait between attempts ends with the context, and the answer that
// came is the error.
func TestRetryWaitEndsWithTheContext(t *testing.T) {
	old := retryWaits
	retryWaits = []time.Duration{time.Minute, time.Minute}
	t.Cleanup(func() { retryWaits = old })
	pub, _ := testKey(t)
	gh := newFakeGitHub(t)
	list := "/repos/" + Repo + "/releases"
	gh.failFirst(list, 503, 503, 503)
	c := &Client{API: gh.srv.URL, Repo: Repo, HTTP: gh.srv.Client(), Key: pub}
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	start := time.Now()
	_, err := c.Latest(ctx, "alpha")
	if err == nil || !strings.Contains(err.Error(), "503") || time.Since(start) > 5*time.Second || gh.count(list) != 1 {
		t.Fatalf("got %v after %s and %d requests", err, time.Since(start), gh.count(list))
	}
}

// A binary answered 503 first is downloaded on the second attempt and
// verifies.
func TestDownloadRetriesItsStart(t *testing.T) {
	fastRetries(t)
	pub, priv := testKey(t)
	gh := newFakeGitHub(t)
	gh.add(t, priv, "v1.0.0", "main", map[string][]byte{"massimo-linux-amd64": []byte("the binary")}, nil)
	c := &Client{API: gh.srv.URL, Repo: Repo, HTTP: gh.srv.Client(), Key: pub}
	r, err := c.ByTag(context.Background(), "v1.0.0")
	if err != nil {
		t.Fatal(err)
	}
	bin := "/dl/v1.0.0/massimo-linux-amd64"
	gh.failFirst(bin, http.StatusServiceUnavailable)
	var buf bytes.Buffer
	if err := c.Download(context.Background(), r, "massimo-linux-amd64", &buf); err != nil || buf.String() != "the binary" || gh.count(bin) != 2 {
		t.Fatalf("%q %v after %d requests", buf.String(), err, gh.count(bin))
	}
}

// shortAnswerWait makes the wait for an answer short for one test, long
// enough for the fake's answers under -race on a busy runner.
func shortAnswerWait(t *testing.T) {
	old := answerWait
	answerWait = 250 * time.Millisecond
	t.Cleanup(func() { answerWait = old })
}

// A manifest that gets no answer twice is fetched on the third attempt.
func TestRetriesWithoutAnAnswer(t *testing.T) {
	fastRetries(t)
	shortAnswerWait(t)
	pub, priv := testKey(t)
	gh := newFakeGitHub(t)
	gh.add(t, priv, "v1.0.0", "main", map[string][]byte{"massimo-linux-amd64": []byte("the binary")}, nil)
	manifest := "/dl/v1.0.0/" + ManifestName
	gh.hangFirst(manifest, 2)
	c := &Client{API: gh.srv.URL, Repo: Repo, HTTP: gh.srv.Client(), Key: pub}
	if _, err := c.ByTag(context.Background(), "v1.0.0"); err != nil || gh.count(manifest) != 3 {
		t.Fatalf("after two requests without an answer: %v, %d requests", err, gh.count(manifest))
	}
}

// A list that never answers is given up after three attempts, and the
// error says why.
func TestNoAnswerGivesUp(t *testing.T) {
	fastRetries(t)
	shortAnswerWait(t)
	pub, _ := testKey(t)
	gh := newFakeGitHub(t)
	list := "/repos/" + Repo + "/releases"
	gh.hangFirst(list, 4)
	c := &Client{API: gh.srv.URL, Repo: Repo, HTTP: gh.srv.Client(), Key: pub}
	_, err := c.Latest(context.Background(), "alpha")
	if err == nil || !strings.Contains(err.Error(), "no answer within 250ms (3 attempts)") || gh.count(list) != 3 {
		t.Fatalf("got %v after %d requests", err, gh.count(list))
	}
}

// A binary whose first request gets no answer is downloaded on the
// second and verifies.
func TestDownloadRetriesWithoutAnAnswer(t *testing.T) {
	fastRetries(t)
	shortAnswerWait(t)
	pub, priv := testKey(t)
	gh := newFakeGitHub(t)
	gh.add(t, priv, "v1.0.0", "main", map[string][]byte{"massimo-linux-amd64": []byte("the binary")}, nil)
	c := &Client{API: gh.srv.URL, Repo: Repo, HTTP: gh.srv.Client(), Key: pub}
	r, err := c.ByTag(context.Background(), "v1.0.0")
	if err != nil {
		t.Fatal(err)
	}
	bin := "/dl/v1.0.0/massimo-linux-amd64"
	gh.hangFirst(bin, 1)
	var buf bytes.Buffer
	if err := c.Download(context.Background(), r, "massimo-linux-amd64", &buf); err != nil || buf.String() != "the binary" || gh.count(bin) != 2 {
		t.Fatalf("%q %v after %d requests", buf.String(), err, gh.count(bin))
	}
}

// The answer wait ends with the answer: a body slower than it, which
// keeps coming, is not cut off.
func TestAnswerWaitLeavesTheBody(t *testing.T) {
	shortAnswerWait(t)
	pub, priv := testKey(t)
	gh := newFakeGitHub(t)
	body := bytes.Repeat([]byte("massimo"), 4096)
	gh.add(t, priv, "v1.0.0", "main", map[string][]byte{"massimo-linux-amd64": body}, nil)
	slowBinary(t, gh, "v1.0.0", "massimo-linux-amd64", body, 4, 200*time.Millisecond)
	c := &Client{API: gh.srv.URL, Repo: Repo, HTTP: gh.srv.Client(), Key: pub}
	r, err := c.ByTag(context.Background(), "v1.0.0")
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := c.Download(context.Background(), r, "massimo-linux-amd64", &buf); err != nil || !bytes.Equal(buf.Bytes(), body) {
		t.Fatalf("a body slower than the answer wait: %v, %d of %d bytes", err, buf.Len(), len(body))
	}
}

type timeoutError struct{}

func (timeoutError) Error() string   { return "timed out" }
func (timeoutError) Timeout() bool   { return true }
func (timeoutError) Temporary() bool { return true }

// What is sent again: a gateway's state, a connection that failed
// before an answer, an answer that did not come; not a timeout of the
// client's, nor the end of the context.
func TestRetryable(t *testing.T) {
	ctx := context.Background()
	for code, want := range map[int]bool{200: false, 403: false, 404: false, 429: false, 500: false, 502: true, 503: true, 504: true} {
		if got := retryable(ctx, &http.Response{StatusCode: code}, nil); got != want {
			t.Errorf("%d: %v", code, got)
		}
	}
	dropped := &url.Error{Op: "Get", URL: "https://example/", Err: io.ErrUnexpectedEOF}
	if !retryable(ctx, nil, dropped) {
		t.Error("a dropped connection is not sent again")
	}
	if retryable(ctx, nil, &url.Error{Op: "Get", URL: "https://example/", Err: timeoutError{}}) {
		t.Error("a timeout is sent again")
	}
	noAnswer := fmt.Errorf("GET https://example/: %w within 10s", errNoAnswer)
	if !retryable(ctx, nil, noAnswer) {
		t.Error("a request without an answer is not sent again")
	}
	ended, cancel := context.WithCancel(ctx)
	cancel()
	if retryable(ended, nil, dropped) || retryable(ended, nil, noAnswer) {
		t.Error("a request is sent again after the context's end")
	}
}

// TestHintedFlavour: only a tag of the full shape names a flavour, so that
// a tag that merely starts like a version is fetched and decided by its
// manifest, as any release without a hint.
func TestHintedFlavour(t *testing.T) {
	for tag, want := range map[string]string{
		"v0.14.0-alpha.1":   "alpha",
		"v1.2.3-rc.2":       "rc",
		"v1.2.3":            "stable",
		"release/nightly":   "nightly",
		"v1..2-alpha.1":     "",
		"v1.2-alpha.1":      "",
		"v1.2":              "",
		"v1.2.3-rc":         "",
		"v1.2.3-Alpha.1":    "",
		"verification-test": "",
	} {
		if got := hintedFlavour(ghRelease{TagName: tag}); got != want {
			t.Errorf("hintedFlavour(%q) = %q, want %q", tag, got, want)
		}
	}
}
