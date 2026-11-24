// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package update

import (
	"bytes"
	"compress/gzip"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDebTag(t *testing.T) {
	t.Parallel()
	for v, want := range map[string]string{
		"0.13.0~alpha.1": "v0.13.0-alpha.1", "1.0.0": "v1.0.0", "0.13.0~alpha.1~rc~2": "v0.13.0-alpha.1-rc-2",
		"0.13.0~alpha.2+3.gabc1234":       "v0.13.0-alpha.2-3-gabc1234",
		"0.13.0~alpha.2+3.gabc1234.dirty": "v0.13.0-alpha.2-3-gabc1234-dirty",
		"0.13.0~alpha.2+dirty":            "v0.13.0-alpha.2-dirty",
		"1.0.0+3.gabc1234":                "v1.0.0-3-gabc1234",
		// The older form of a build between tags, still installed on a box.
		"0.13.0~alpha.2~3~gabc1234": "v0.13.0-alpha.2-3-gabc1234",
	} {
		if got := DebTag(v); got != want {
			t.Errorf("DebTag(%q) = %q, want %q", v, got, want)
		}
	}
}

// aptIndex is a Packages file as apt-ftparchive writes one: the keyring
// package, massimo in several versions (a build between tags among them,
// not in order), another package named like it.
const aptIndex = `Package: massimo-archive-keyring
Version: 2026.11.1
Architecture: all

Package: massimo
Version: 0.16.0~alpha.1
Architecture: armhf
Description: Server for eQ-3 MAX! Cube heating systems
 a continuation line: Version: 9.9.9

Package: massimo
Version: 0.16.0~alpha.1+597.g43930b56
Architecture: armhf

Package: massimo
Version: 0.15.2
Architecture: armhf

Package: massimo-tools
Version: 9.9.9
Architecture: armhf
`

// writeIndex writes index gzipped as the Packages.gz of suite's main
// component for arch below dir, as publish.sh lays it out.
func writeIndex(t *testing.T, dir, suite, arch, index string) {
	t.Helper()
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	zw.Write([]byte(index))
	zw.Close()
	p := filepath.Join(dir, "dists", suite, "main", "binary-"+arch, "Packages.gz")
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
}

// The newest massimo of a suite's index, from a file: repository and an
// https one, for the box's architecture: a build between tags after its
// tag, other packages and continuation lines left out; none where the
// index has none, and an error where it is missing or not gzip.
func TestAptNewest(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeIndex(t, dir, "devel", "armhf", aptIndex)
	writeIndex(t, dir, "devel", "amd64", "Package: massimo\nVersion: 0.15.0\n")
	writeIndex(t, dir, "beta", "armhf", "Package: massimo-archive-keyring\nVersion: 1\n")
	ctx := t.Context()
	for _, uri := range []string{"file:" + dir, "file://" + dir + "/"} {
		if got, err := AptNewest(ctx, nil, uri, "devel", "main", "armhf"); err != nil || got != "v0.16.0-alpha.1-597-g43930b56" {
			t.Errorf("%s: %q, %v", uri, got, err)
		}
	}
	if got, err := AptNewest(ctx, nil, "file:"+dir, "devel", "main", "amd64"); err != nil || got != "v0.15.0" {
		t.Errorf("amd64: %q, %v", got, err)
	}
	if got, err := AptNewest(ctx, nil, "file:"+dir, "beta", "main", "armhf"); err != nil || got != "" {
		t.Errorf("an index without massimo: %q, %v", got, err)
	}
	if _, err := AptNewest(ctx, nil, "file:"+dir, "stable", "main", "armhf"); err == nil {
		t.Error("a missing index read")
	}
	if _, err := AptNewest(ctx, nil, "file:"+dir, "../devel", "main", "armhf"); err == nil {
		t.Error("a suite with a path read")
	}
	if _, err := AptNewest(ctx, nil, "http://example.invalid/apt", "devel", "main", "armhf"); err == nil {
		t.Error("plain http read")
	}
	bad := filepath.Join(dir, "dists", "rc", "main", "binary-armhf", "Packages.gz")
	os.MkdirAll(filepath.Dir(bad), 0o755)
	os.WriteFile(bad, []byte("Package: massimo\n"), 0o644)
	if _, err := AptNewest(ctx, nil, "file:"+dir, "rc", "main", "armhf"); err == nil {
		t.Error("an index that is not gzip read")
	}

	ts := httptest.NewTLSServer(http.StripPrefix("/apt", http.FileServer(http.Dir(dir))))
	t.Cleanup(ts.Close)
	if got, err := AptNewest(ctx, ts.Client(), ts.URL+"/apt", "devel", "main", "armhf"); err != nil || got != "v0.16.0-alpha.1-597-g43930b56" {
		t.Errorf("https: %q, %v", got, err)
	}
	if _, err := AptNewest(ctx, ts.Client(), ts.URL+"/apt", "stable", "main", "armhf"); err == nil || !strings.Contains(err.Error(), "404") {
		t.Errorf("https, a missing index: %v", err)
	}
}

// An index larger than its bound, compressed or not, is refused.
func TestAptNewestBounded(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeIndex(t, dir, "devel", "amd64", "Package: massimo\nVersion: 0.15.0\n\n"+strings.Repeat("X-Pad: "+strings.Repeat("x", 1000)+"\n", maxIndex/1000+1))
	if _, err := AptNewest(t.Context(), nil, "file:"+dir, "devel", "main", "amd64"); err == nil || !strings.Contains(err.Error(), "more than") {
		t.Fatalf("a large index: %v", err)
	}
}

func TestAptArch(t *testing.T) {
	t.Parallel()
	for goarch, want := range map[string]string{"amd64": "amd64", "arm64": "arm64", "arm": "armhf", "386": "", "riscv64": ""} {
		if got := AptArch(goarch); got != want {
			t.Errorf("AptArch(%q) = %q, want %q", goarch, got, want)
		}
	}
}

// A formula's version line as render.sh writes it, as a tag, from a
// file and over https; a formula without one, or with a version that is
// no release's, is an error.
func TestFormulaNewest(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	write := func(name, body string) string {
		t.Helper()
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		return p
	}
	good := write("massimo-alpha.rb", "class MassimoAlpha < Formula\n  url \"https://example.invalid/v0.16.0-alpha.2.tar.gz\"\n  version \"0.16.0-alpha.2\"\nend\n")
	if got, err := FormulaNewest(t.Context(), nil, "file:"+good); err != nil || got != "v0.16.0-alpha.2" {
		t.Errorf("the formula: %q, %v", got, err)
	}
	rolling := write("massimo-devel.rb", "class MassimoDevel < Formula\n  version \"0.16.0-alpha.1-597-g43930b56\"\nend\n")
	if got, err := FormulaNewest(t.Context(), nil, "file:"+rolling); err != nil || got != "v0.16.0-alpha.1-597-g43930b56" {
		t.Errorf("a rolling formula: %q, %v", got, err)
	}
	for _, body := range []string{"class Massimo < Formula\nend\n", "class Massimo < Formula\n  version \"head\"\nend\n"} {
		if _, err := FormulaNewest(t.Context(), nil, "file:"+write("bad.rb", body)); err == nil {
			t.Errorf("read %q", body)
		}
	}
	ts := httptest.NewTLSServer(http.FileServer(http.Dir(dir)))
	t.Cleanup(ts.Close)
	if got, err := FormulaNewest(t.Context(), ts.Client(), ts.URL+"/massimo-alpha.rb"); err != nil || got != "v0.16.0-alpha.2" {
		t.Errorf("https: %q, %v", got, err)
	}
}

func TestFormulaURL(t *testing.T) {
	t.Parallel()
	if got, err := FormulaURL("metaworx/massimo-rolling", "massimo-devel"); err != nil ||
		got != "https://raw.githubusercontent.com/metaworx/homebrew-massimo-rolling/HEAD/Formula/massimo-devel.rb" {
		t.Errorf("the rolling tap: %q, %v", got, err)
	}
	for _, bad := range [][2]string{{"metaworx", "massimo"}, {"metaworx/massimo", "../x"}, {"../x/massimo", "massimo"}} {
		if _, err := FormulaURL(bad[0], bad[1]); err == nil {
			t.Errorf("FormulaURL(%q, %q) took it", bad[0], bad[1])
		}
	}
}
