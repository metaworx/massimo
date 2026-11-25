// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package update

// What a box's package source has (AP AutoUpdate D5-A): apt's index of
// a suite, as packaging/apt/publish.sh writes it, and the Homebrew
// formula a tap holds, as packaging/homebrew/render.sh renders it. The
// server offers a release only where the box's own source has it, since
// GitHub's release is out before the apt index and the tap have it. The
// index is read unsigned: the version offered is never newer than
// GitHub's release, which the compiled-in key verifies, and apt verifies
// the index and the package before the helper installs anything.

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"
)

// DebTag is the version the binary of a Debian package reports: the
// Makefile's DEBVER read backwards, 0.13.0~alpha.1 for v0.13.0-alpha.1,
// and for a build between tags the part git describe adds behind the +,
// 0.13.0~alpha.1+3.gabc1234 for v0.13.0-alpha.1-3-gabc1234. A package of
// the older form, ~ throughout, reads as it did.
func DebTag(v string) string {
	base, build, between := strings.Cut(v, "+")
	tag := "v" + strings.ReplaceAll(base, "~", "-")
	if between {
		tag += "-" + strings.ReplaceAll(build, ".", "-")
	}
	return tag
}

// AptArch is the Debian architecture of a Go one, the binary-<arch>
// directory of the index that holds its package: arm is armhf, which
// holds the armv6 binary as well; "" for one the repository has none
// for.
func AptArch(goarch string) string {
	switch goarch {
	case "amd64", "arm64":
		return goarch
	case "arm":
		return "armhf"
	}
	return ""
}

// The bounds of a source read: an index, compressed and not, and a
// formula. A suite's index lists ten releases of each channel it takes,
// some tens of kilobytes.
const (
	maxIndexGz    = 4 << 20
	maxIndex      = 32 << 20
	maxFormula    = 256 << 10
	sourceTimeout = 30 * time.Second
)

// sourceClient is the HTTP client of a read where the caller gives none.
var sourceClient = &http.Client{Timeout: sourceTimeout}

// AptNewest is the newest version of the package massimo that the apt
// index of suite and component in the repository at uri lists for arch,
// as a tag (DebTag); "" where it lists none. uri is https: or file:, as
// a source's URIs line names it; hc is the client for https, nil for
// one of its own.
func AptNewest(ctx context.Context, hc *http.Client, uri, suite, component, arch string) (string, error) {
	if suite == "" || component == "" || arch == "" {
		return "", fmt.Errorf("apt index: no suite, component or architecture (%q, %q, %q)", suite, component, arch)
	}
	for _, part := range []string{suite, component, arch} {
		if strings.Contains(part, "/") || strings.Contains(part, "..") {
			return "", fmt.Errorf("apt index: %q is not a name", part)
		}
	}
	where := strings.TrimSuffix(uri, "/") + "/dists/" + suite + "/" + component + "/binary-" + arch + "/Packages.gz"
	gz, err := fetch(ctx, hc, where, maxIndexGz)
	if err != nil {
		return "", fmt.Errorf("apt index: %w", err)
	}
	zr, err := gzip.NewReader(bytes.NewReader(gz))
	if err != nil {
		return "", fmt.Errorf("apt index %s: %w", where, err)
	}
	b, err := io.ReadAll(io.LimitReader(zr, maxIndex+1))
	if err != nil {
		return "", fmt.Errorf("apt index %s: %w", where, err)
	}
	if len(b) > maxIndex {
		return "", fmt.Errorf("apt index %s: more than %d bytes", where, maxIndex)
	}
	tag, err := newestDeb(b)
	if err != nil {
		return "", fmt.Errorf("apt index %s: %w", where, err)
	}
	return tag, nil
}

// newestDeb is the newest Version of the stanzas of the package massimo
// in an index, as a tag; "" where there is none, a version that is no
// release's left out.
func newestDeb(index []byte) (string, error) {
	var best string
	var bestV Version
	pkg, ver := "", ""
	end := func() {
		if pkg == "massimo" && ver != "" {
			tag := DebTag(ver)
			if v, ok := ParseVersion(tag); ok && (best == "" || Compare(v, bestV) > 0) {
				best, bestV = tag, v
			}
		}
		pkg, ver = "", ""
	}
	s := bufio.NewScanner(bytes.NewReader(index))
	s.Buffer(make([]byte, 64<<10), 1<<20)
	for s.Scan() {
		line := s.Text()
		if strings.TrimSpace(line) == "" {
			end()
			continue
		}
		if line[0] == ' ' || line[0] == '\t' {
			continue // a continuation line
		}
		k, v, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		switch strings.ToLower(k) {
		case "package":
			pkg = strings.TrimSpace(v)
		case "version":
			ver = strings.TrimSpace(v)
		}
	}
	if err := s.Err(); err != nil {
		return "", err
	}
	end()
	return best, nil
}

// formulaVersionRe finds a formula's version line, which render.sh
// writes as version "0.16.0-alpha.1".
var formulaVersionRe = regexp.MustCompile(`(?m)^\s*version\s+"([^"]+)"`)

// FormulaNewest is the version of the Homebrew formula at uri (https:
// or file:) as a tag: the formula builds with -X main.version=v<version>.
// hc is the client for https, nil for one of its own.
func FormulaNewest(ctx context.Context, hc *http.Client, uri string) (string, error) {
	b, err := fetch(ctx, hc, uri, maxFormula)
	if err != nil {
		return "", fmt.Errorf("formula: %w", err)
	}
	m := formulaVersionRe.FindSubmatch(b)
	if m == nil {
		return "", fmt.Errorf("formula %s: no version line", uri)
	}
	tag := "v" + string(m[1])
	if _, ok := ParseVersion(tag); !ok {
		return "", fmt.Errorf("formula %s: the version %q is no release's", uri, m[1])
	}
	return tag, nil
}

// tapRe is a tap's name, owner/name, as brew's install receipt gives
// it, and formulaRe a formula's.
var (
	tapRe     = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9-]*/[A-Za-z0-9][A-Za-z0-9._-]*$`)
	formulaRe = regexp.MustCompile(`^[a-z0-9][a-z0-9._@+-]*$`)
)

// FormulaURL is where the tap's formula is on GitHub, the file the
// helper's brew update fetches (AP AutoUpdate D11): the tap owner/name
// is the repository owner/homebrew-name, its default branch.
func FormulaURL(tap, formula string) (string, error) {
	if !tapRe.MatchString(tap) || !formulaRe.MatchString(formula) {
		return "", fmt.Errorf("the tap %q or the formula %q is not a name", tap, formula)
	}
	owner, name, _ := strings.Cut(tap, "/")
	return "https://raw.githubusercontent.com/" + owner + "/homebrew-" + name + "/HEAD/Formula/" + formula + ".rb", nil
}

// fetch reads at most limit bytes from where: https through hc, or a
// local file (file:).
func fetch(ctx context.Context, hc *http.Client, where string, limit int64) ([]byte, error) {
	u, err := url.Parse(where)
	if err != nil {
		return nil, err
	}
	var body io.Reader
	switch u.Scheme {
	case "file":
		p, err := filePath(u)
		if err != nil {
			return nil, err
		}
		f, err := os.Open(p)
		if err != nil {
			return nil, err
		}
		defer f.Close()
		body = f
	case "https":
		if hc == nil {
			hc = sourceClient
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, where, nil)
		if err != nil {
			return nil, err
		}
		resp, err := hc.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("GET %s: %s", where, resp.Status)
		}
		body = resp.Body
	default:
		return nil, errors.New(where + " is neither https: nor file:")
	}
	b, err := io.ReadAll(io.LimitReader(body, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(b)) > limit {
		return nil, fmt.Errorf("%s: more than %d bytes", where, limit)
	}
	return b, nil
}

// filePath is the local file a file: URL names, file:/srv/apt or
// file:///srv/apt as an apt source writes it: the URL's path, which is
// absolute. On Windows a drive's path is written file:///C:/apt: the
// slash before the drive letter is dropped, the others become
// backslashes.
func filePath(u *url.URL) (string, error) {
	p := u.Path
	if !strings.HasPrefix(p, "/") {
		return "", fmt.Errorf("%s names no absolute path", u.Redacted())
	}
	if runtime.GOOS == "windows" && filepath.VolumeName(p[1:]) != "" {
		p = p[1:]
	}
	return filepath.FromSlash(p), nil
}
