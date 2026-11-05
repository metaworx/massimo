// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package update

import (
	"regexp"
	"strconv"
	"strings"
)

// Version is a parsed version string as `git describe --tags --dirty`
// prints it: a semantic version tag, optionally followed by the number of
// commits since that tag and the commit hash (-3-g1a2b3c4) and by -dirty.
type Version struct {
	Major, Minor, Patch int
	Pre                 []string // pre-release identifiers: "alpha", "9"
	Ahead               int      // commits after the tag
	Dirty               bool
}

var (
	semverRe   = regexp.MustCompile(`^v(\d+)\.(\d+)\.(\d+)(?:-([0-9A-Za-z.-]+))?$`)
	describeRe = regexp.MustCompile(`-(\d+)-g[0-9a-f]{4,40}$`)
)

// ParseVersion parses "v1.2.3", "v0.1.0-alpha.9", "v0.1.0-alpha.9-3-gabc1234"
// and the same with "-dirty". It reports false for anything else, such as
// the "v0.0.0-dev+hash" of a build without the Makefile.
func ParseVersion(s string) (Version, bool) {
	var v Version
	s = strings.TrimSpace(s)
	if t, ok := strings.CutSuffix(s, "-dirty"); ok {
		s, v.Dirty = t, true
	}
	if m := describeRe.FindStringSubmatch(s); m != nil {
		v.Ahead, _ = strconv.Atoi(m[1])
		s = s[:len(s)-len(m[0])]
	}
	m := semverRe.FindStringSubmatch(s)
	if m == nil {
		return Version{}, false
	}
	v.Major, _ = strconv.Atoi(m[1])
	v.Minor, _ = strconv.Atoi(m[2])
	v.Patch, _ = strconv.Atoi(m[3])
	if m[4] != "" {
		v.Pre = strings.Split(m[4], ".")
	}
	return v, true
}

// Tag reports whether the version is exactly a release tag: no commits
// after it and no local changes.
func (v Version) Tag() bool { return v.Ahead == 0 && !v.Dirty }

// Compare orders versions by semantic versioning; a build some commits
// after a tag sorts after the tag and before the next one.
func Compare(a, b Version) int {
	for _, d := range [][2]int{{a.Major, b.Major}, {a.Minor, b.Minor}, {a.Patch, b.Patch}} {
		if d[0] != d[1] {
			return cmpInt(d[0], d[1])
		}
	}
	if c := comparePre(a.Pre, b.Pre); c != 0 {
		return c
	}
	return cmpInt(a.Ahead, b.Ahead)
}

// comparePre follows semver §11: a version without pre-release sorts after
// one with; identifiers compare numerically when both are numbers, else as
// strings, and numbers sort before strings.
func comparePre(a, b []string) int {
	switch {
	case len(a) == 0 && len(b) == 0:
		return 0
	case len(a) == 0:
		return 1
	case len(b) == 0:
		return -1
	}
	for i := 0; i < len(a) && i < len(b); i++ {
		na, ea := strconv.Atoi(a[i])
		nb, eb := strconv.Atoi(b[i])
		switch {
		case ea == nil && eb == nil:
			if na != nb {
				return cmpInt(na, nb)
			}
		case ea == nil:
			return -1
		case eb == nil:
			return 1
		default:
			if c := strings.Compare(a[i], b[i]); c != 0 {
				return c
			}
		}
	}
	return cmpInt(len(a), len(b))
}

func cmpInt(a, b int) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return 0
}
