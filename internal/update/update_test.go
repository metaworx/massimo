// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package update

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
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
}

// fakeGitHub serves a releases list and the release files.
type fakeGitHub struct {
	srv      *httptest.Server
	releases []map[string]any
	files    map[string][]byte
}

func newFakeGitHub(t *testing.T) *fakeGitHub {
	f := &fakeGitHub{files: map[string][]byte{}}
	f.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/repos/"+Repo+"/releases" {
			json.NewEncoder(w).Encode(f.releases)
			return
		}
		b, ok := f.files[r.URL.Path]
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
func (f *fakeGitHub) add(t *testing.T, key ed25519.PrivateKey, tag, channel string, binaries map[string][]byte, edit func([]byte) []byte) {
	m := Manifest{Tag: tag, Commit: "c0ffee", Channel: channel, Prerelease: true, Assets: map[string]string{}}
	var assets []map[string]any
	for name, b := range binaries {
		sum := sha256.Sum256(b)
		m.Assets[name] = hex.EncodeToString(sum[:])
		f.files["/dl/"+tag+"/"+name] = b
		assets = append(assets, map[string]any{"name": name, "browser_download_url": f.srv.URL + "/dl/" + tag + "/" + name})
	}
	mb, _ := json.Marshal(m)
	sig := base64.StdEncoding.EncodeToString(ed25519.Sign(key, mb)) + "\n"
	if edit != nil {
		mb = edit(mb)
	}
	f.files["/dl/"+tag+"/"+ManifestName] = mb
	f.files["/dl/"+tag+"/"+SignatureName] = []byte(sig)
	for _, n := range []string{ManifestName, SignatureName} {
		assets = append(assets, map[string]any{"name": n, "browser_download_url": f.srv.URL + "/dl/" + tag + "/" + n})
	}
	f.releases = append(f.releases, map[string]any{"tag_name": tag, "html_url": "https://example/" + tag, "assets": assets})
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
	gh.add(t, priv, "v0.1.0-alpha.9", "phase-0", bin, nil)
	gh.add(t, priv, "v0.1.0-alpha.11", "main", bin, nil)
	gh.add(t, priv, "v0.1.0-alpha.10", "phase-0", bin, nil)
	gh.add(t, other, "v0.1.0-alpha.12", "phase-0", bin, nil) // wrong key
	gh.add(t, priv, "v0.1.0-alpha.13", "phase-0", bin, func(b []byte) []byte {
		return bytes.Replace(b, []byte("phase-0"), []byte("phase-1"), 1) // changed after signing
	})
	c := &Client{API: gh.srv.URL, Repo: Repo, HTTP: gh.srv.Client(), Key: pub}

	r, err := c.Latest(context.Background(), "phase-0")
	if err != nil || r == nil || r.Tag != "v0.1.0-alpha.10" {
		t.Fatalf("phase-0: %+v %v", r, err)
	}
	if r, err := c.Latest(context.Background(), "main"); err != nil || r == nil || r.Tag != "v0.1.0-alpha.11" {
		t.Fatalf("main: %+v %v", r, err)
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
	gh.add(t, priv, "v0.1.0-alpha.5", "phase-0", map[string][]byte{"massimo-linux-amd64": []byte("old")}, nil)
	// A newer release carrying the old one's (correctly signed) manifest.
	gh.add(t, priv, "v0.1.0-alpha.6", "phase-0", map[string][]byte{"massimo-linux-amd64": []byte("new")}, nil)
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
	if err := WriteRequest(p, "phase-0"); err != nil {
		t.Fatal(err)
	}
	if c, ok, err := TakeRequest(p); c != "phase-0" || !ok || err != nil {
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
