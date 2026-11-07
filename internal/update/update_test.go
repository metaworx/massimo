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
	"slices"
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
	auth     map[string]string // path → Authorization header seen
}

func newFakeGitHub(t *testing.T) *fakeGitHub {
	f := &fakeGitHub{files: map[string][]byte{}, auth: map[string]string{}}
	f.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.auth[r.URL.Path] = r.Header.Get("Authorization")
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
	if r, err := c.Latest(context.Background(), "main"); r != nil || err != nil {
		t.Fatalf("main, which has no release: %+v %v", r, err)
	}
	_, other := testKey(t)
	gh.add(t, other, "v0.2.0", "main", bin, nil) // a tag without a label, nor a target
	if r, err := c.Latest(context.Background(), "main"); r != nil || err == nil || !strings.Contains(err.Error(), "does not verify") {
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
	gh.releases[len(gh.releases)-1]["target_commitish"] = "stable"
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
	if r, err := c.Latest(context.Background(), "alpha"); err != nil || r == nil || r.Tag != "v0.1.0-alpha.5" {
		t.Fatalf("alpha: %+v %v", r, err)
	}
	if got, want := fetched(), []string{"/dl/v0.1.0-alpha.5/" + ManifestName, "/dl/v0.1.0-alpha.5/" + SignatureName}; !slices.Equal(got, want) {
		t.Errorf("alpha fetched %v, want the newest alpha's alone", got)
	}
	if r, err := c.Latest(context.Background(), "stable"); err != nil || r == nil || r.Tag != "v0.2.0" {
		t.Fatalf("stable, by its target: %+v %v", r, err)
	}
	if got := fetched(); len(got) != 2 {
		t.Errorf("stable fetched %v, want v0.2.0's alone", got)
	}
}
