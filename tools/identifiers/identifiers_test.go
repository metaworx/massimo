// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package identifiers

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"go.metaworx.ch/massimo/internal/protocol/cube"
)

// The placeholders: a serial of three letters and 0000 and three digits
// (KEQ0000100), a MAC whose last three octets are each below 0x10 or
// all zero (00:1a:22:0a:0b:0c), a radio address starting 0a (0a0101).
var (
	serialRE      = regexp.MustCompile(`\b[A-Z]{3}[0-9]{7}\b`)
	serialOK      = regexp.MustCompile(`^[A-Z]{3}0000[0-9]{3}$`)
	macRE         = regexp.MustCompile(`(?i)\b(?:[0-9a-f]{2}[:-]){5}[0-9a-f]{2}\b`)
	macOK         = regexp.MustCompile(`(?i)^(?:[0-9a-f]{2}[:-]){3}0[0-9a-f][:-]0[0-9a-f][:-]0[0-9a-f]$`)
	rfOK          = regexp.MustCompile(`(?i)^0a[0-9a-f]{4}$`)
	allowedLineRE = regexp.MustCompile(`^(\S+)\s+(\S.*)$`)
)

// rfContexts are where six hex digits are a radio address: alone they are
// everywhere (colours, hashes).
var rfContexts = []*regexp.Regexp{
	regexp.MustCompile(`\bC:([0-9a-fA-F]{6}),`),
	regexp.MustCompile(`\bH:[A-Z]{3}[0-9]{7},([0-9a-fA-F]{6}),`),
	regexp.MustCompile(`rf_address["']?\s*[:=]\s*["']?([0-9a-fA-F]{6})\b`),
	regexp.MustCompile(`"rf"\s*:\s*"([0-9a-fA-F]{6})"`),
	regexp.MustCompile(`RFAddress\s*(?:[:=!]=?|\()\s*0x([0-9a-fA-F]{6})\b`),
	regexp.MustCompile(`\b(?:device|target)=([0-9a-fA-F]{6})\b`),
	regexp.MustCompile(`/devices/([0-9a-fA-F]{6})\b`),
}

// goRF is one more context, in Go code: a hex literal of six digits,
// which in this repository is a radio address wherever it appears.
var goRF = regexp.MustCompile(`\b0x([0-9a-fA-F]{6})\b`)

// fixtures are the recorded protocol lines, checked as decoded.
var fixtures = []string{"internal/protocol/cube/testdata/*.txt", "tools/ui-test/greeting.txt"}

// skipped are paths not checked: code that is not the project's, the
// check's own files (the allowlist, and the samples of TestScanText),
// and the plans and their NOTEs, which may name what a change replaces
// and go when the plan retires.
func skipped(path string) bool {
	for _, p := range []string{"internal/server/ui/vendor/", "third_party/", "GUIDELINES/shared", "GUIDELINES/wip/", "tools/identifiers/"} {
		if strings.HasPrefix(path, p) {
			return true
		}
	}
	return false
}

// finding is one identifier that is not a placeholder.
type finding struct {
	where, kind, value string
}

func (f finding) String() string { return fmt.Sprintf("%s: %s %s", f.where, f.kind, f.value) }

// repoRoot is the repository's top directory, or "" outside a git
// checkout (a source tarball).
func repoRoot(t *testing.T) string {
	out, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// trackedFiles are the files git tracks.
func trackedFiles(t *testing.T, root string) []string {
	out, err := exec.Command("git", "-C", root, "ls-files", "-z").Output()
	if err != nil {
		t.Fatalf("git ls-files: %v", err)
	}
	var files []string
	for _, f := range strings.Split(string(out), "\x00") {
		if f != "" && !skipped(f) {
			files = append(files, f)
		}
	}
	return files
}

// allowed reads allowed.txt: a value and its reason a line, # a comment.
func allowed(t *testing.T) map[string]string {
	b, err := os.ReadFile("allowed.txt")
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]string{}
	sc := bufio.NewScanner(bytes.NewReader(b))
	for n := 1; sc.Scan(); n++ {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		m := allowedLineRE.FindStringSubmatch(line)
		if m == nil {
			t.Fatalf("allowed.txt:%d: want a value and its reason", n)
		}
		out[strings.ToLower(m[1])] = m[2]
	}
	return out
}

// scanText finds the identifiers of one file's text that are not
// placeholders.
func scanText(path string, text string) []finding {
	var out []finding
	for i, line := range strings.Split(text, "\n") {
		where := fmt.Sprintf("%s:%d", path, i+1)
		for _, s := range serialRE.FindAllString(line, -1) {
			if !serialOK.MatchString(s) {
				out = append(out, finding{where, "serial", s})
			}
		}
		for _, m := range macRE.FindAllString(line, -1) {
			if !macOK.MatchString(m) && strings.Trim(m, "0:-") != "" {
				out = append(out, finding{where, "MAC", m})
			}
		}
		contexts := rfContexts
		if strings.HasSuffix(path, ".go") {
			contexts = append(slices.Clip(rfContexts), goRF)
		}
		for _, re := range contexts {
			for _, m := range re.FindAllStringSubmatch(line, -1) {
				if !rfOK.MatchString(m[1]) {
					out = append(out, finding{where, "RF address", strings.ToLower(m[1])})
				}
			}
		}
	}
	return out
}

// scanFixture finds the identifiers of a recorded greeting, decoded, that
// are not placeholders: the Cube's serial and address, and each device's.
func scanFixture(t *testing.T, path, text string) []finding {
	msgs, err := cube.SplitLines(text)
	if err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	var out []finding
	serial := func(where, s string) {
		if !serialOK.MatchString(s) {
			out = append(out, finding{where, "serial", s})
		}
	}
	rf := func(where string, a cube.RFAddress) {
		if !rfOK.MatchString(a.String()) {
			out = append(out, finding{where, "RF address", a.String()})
		}
	}
	var parts []cube.MetadataPart
	for i, m := range msgs {
		where := fmt.Sprintf("%s:%d (%c, decoded)", path, i+1, m.Type)
		switch m.Type {
		case 'H':
			h, err := cube.ParseHello(m.Data)
			if err != nil {
				t.Fatalf("%s: %v", where, err)
			}
			serial(where, h.Serial)
			rf(where, h.RFAddress)
		case 'C':
			c, err := cube.ParseConfig(m.Data)
			if err != nil {
				t.Fatalf("%s: %v", where, err)
			}
			serial(where, c.Serial)
			rf(where, c.RFAddress)
		case 'M':
			p, err := cube.ParseMetadataPart(m.Data)
			if err != nil {
				t.Fatalf("%s: %v", where, err)
			}
			parts = append(parts, p)
			if p.Index+1 < p.Count {
				continue
			}
			md, err := cube.ParseMetadata(parts)
			if err != nil {
				t.Fatalf("%s: %v", where, err)
			}
			for _, r := range md.Rooms {
				rf(where+" room "+r.Name, r.RFAddress)
			}
			for _, d := range md.Devices {
				serial(where+" device "+d.Name, d.Serial)
				rf(where+" device "+d.Name, d.RFAddress)
			}
		case 'L':
			states, err := cube.ParseDeviceList(m.Data, nil)
			if err != nil {
				t.Fatalf("%s: %v", where, err)
			}
			for _, s := range states {
				rf(where, s.RFAddress)
			}
		}
	}
	return out
}

// No tracked file and no recorded greeting holds a serial, a MAC or a
// radio address that is not a placeholder, unless allowed.txt names it;
// and allowed.txt names nothing the tree no longer holds.
func TestNoRealIdentifiers(t *testing.T) {
	root := repoRoot(t)
	if root == "" {
		t.Skip("not a git checkout")
	}
	allow := allowed(t)
	used := map[string]bool{}
	var bad []finding
	keep := func(fs []finding) {
		for _, f := range fs {
			if _, ok := allow[strings.ToLower(f.value)]; ok {
				used[strings.ToLower(f.value)] = true
				continue
			}
			bad = append(bad, f)
		}
	}
	for _, path := range trackedFiles(t, root) {
		b, err := os.ReadFile(filepath.Join(root, path))
		if err != nil || bytes.IndexByte(b[:min(len(b), 8000)], 0) >= 0 {
			continue // a directory (a submodule), or not text
		}
		keep(scanText(path, string(b)))
	}
	for _, glob := range fixtures {
		matches, _ := filepath.Glob(filepath.Join(root, glob))
		if len(matches) == 0 {
			t.Errorf("no fixture matches %s", glob)
		}
		for _, p := range matches {
			b, err := os.ReadFile(p)
			if err != nil {
				t.Fatal(err)
			}
			rel, _ := filepath.Rel(root, p)
			keep(scanFixture(t, rel, string(b)))
		}
	}
	for _, f := range bad {
		t.Errorf("%v: not a placeholder; replace it, or name it in tools/identifiers/allowed.txt with its reason", f)
	}
	var stale []string
	for v := range allow {
		if !used[v] {
			stale = append(stale, v)
		}
	}
	slices.Sort(stale)
	for _, v := range stale {
		t.Errorf("allowed.txt: %s is in no tracked file any more; take it out", v)
	}
}

// The check finds what it is for, in each of its forms, and lets the
// placeholders pass.
func TestScanText(t *testing.T) {
	text := strings.Join([]string{
		`serial KEQ0000100 and KEQ0123456`,
		`mac 00:1a:22:0a:0b:0c and 00:1A:22:12:34:56 and 00:00:00:00:00:00`,
		`C:0a0101,AAAA and C:1234ab,AAAA`,
		`H:KEQ0000100,0a0001,0113 and H:KEQ0000100,12cd34,0113`,
		`{"rf_address": "0abcde", "rf_address":"a1b2c3"}`,
		`d.RFAddress != 0x0aed69 && d.RFAddress != 0x0bed69`,
		`device=0a0102 device=fedcba /devices/0a0103 /devices/abcdef`,
		`color: #1514b6; sha 1514b6aa`,
	}, "\n")
	var got []string
	for _, f := range scanText("x", text) {
		got = append(got, f.kind+" "+f.value)
	}
	want := []string{"serial KEQ0123456", "MAC 00:1A:22:12:34:56", "RF address 1234ab", "RF address 12cd34", "RF address a1b2c3", "RF address 0bed69", "RF address fedcba", "RF address abcdef"}
	if !slices.Equal(got, want) {
		t.Errorf("found %q, want %q", got, want)
	}
	// In Go code a six-digit hex literal is a radio address too.
	got = got[:0]
	for _, f := range scanText("x.go", "CmdDeleteDevices(true, 0x0f1d54, 0x0a0101) // 0x1234567") {
		got = append(got, f.value)
	}
	if !slices.Equal(got, []string{"0f1d54"}) {
		t.Errorf("in Go: found %q, want [0f1d54]", got)
	}
}
