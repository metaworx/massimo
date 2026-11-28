// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package idioms

import (
	"bufio"
	"bytes"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// suite is the UI suite's directory, the parent of this one.
const suite = ".."

// section is where the failures send a test's writer.
const section = "docs/ui-testing.md, section 1"

var (
	// fixedWaitRE is a fixed pause: Playwright's wait_for_timeout called.
	fixedWaitRE = regexp.MustCompile(`\.wait_for_timeout\(`)
	// plainRouteRE is a route of Playwright's own, a page's or a
	// context's; helpers.py's safe_route( and an unroute( are not.
	plainRouteRE = regexp.MustCompile(`\.route\(`)
	// kitDefRE is a name helpers.py defines at its top level.
	kitDefRE = regexp.MustCompile(`(?m)^(?:def|class)\s+([A-Za-z]\w*)`)
	// localDefRE is a name a test file defines, at any depth.
	localDefRE = regexp.MustCompile(`(?m)^[ \t]*(?:def|class)\s+(\w+)`)
	// stateRefRE is a field of the page's state, app.js's state, as a page
	// script names it: state.x, state.x.y, state.x[k]; whether it is
	// written is what comes before and after it (stateWrites).
	stateRefRE = regexp.MustCompile(`state\.\w+(?:\.\w+|\[[^\]\n]+\])*`)
	// assignRE is an assignment, compound or not, an increment or a
	// decrement, at the start of what follows a field: = but not == or =>.
	assignRE = regexp.MustCompile(`^(?:=(?:[^=>]|$)|\+\+|--|\*\*=|<<=|>>>=|>>=|&&=|\|\|=|\?\?=|[-+*/%&|^]=)`)
	// assignStateRE is Object.assign into the state or a field of it.
	assignStateRE = regexp.MustCompile(`\bObject\.assign\(\s*state\b`)
	listLineRE    = regexp.MustCompile(`^(test_\w+\.py)\s+([0-9]+)$`)
)

// mutators are the methods that change an array in place.
var mutators = map[string]bool{"push": true, "pop": true, "shift": true, "unshift": true, "splice": true, "sort": true,
	"reverse": true, "fill": true, "copyWithin": true}

// idiom is one old way of doing a thing that the guard counts per test
// file, against its list.
type idiom struct {
	what    string                     // the idiom, as a failure names it
	list    string                     // its list, in this directory
	instead string                     // what a test does instead
	find    func(text string) []string // its uses in a file's text
	named   bool                       // whether a failure names the uses found
}

// matches is find for a regular expression: each match, as matched.
func matches(re *regexp.Regexp) func(string) []string {
	return func(text string) []string { return re.FindAllString(text, -1) }
}

// localKitDefs is find for the local definitions of a name of kit:
// each name defined, as often as it is.
func localKitDefs(kit map[string]bool) func(string) []string {
	return func(text string) []string {
		var out []string
		for _, m := range localDefRE.FindAllStringSubmatch(text, -1) {
			if kit[m[1]] {
				out = append(out, m[1])
			}
		}
		return out
	}
}

// identOrDot is a byte that goes on a name or a member access before
// state: themeState.x and MassimoParts.log.state.x are not app.js's.
func identOrDot(c byte) bool {
	return c == '_' || c == '$' || c == '.' || '0' <= c && c <= '9' || 'a' <= c && c <= 'z' || 'A' <= c && c <= 'Z'
}

// stateWrites is find for the writes of the page's state in a page
// script: a field assigned (state.x = …, state.x.y += …, state.x[k] = …),
// incremented or decremented, deleted, changed in place by an array's
// method (state.x.push(…)), or the state or a field of it as
// Object.assign's target; each as written. A read (state.x === …) is
// none, and so is a Python dict named state (state["x"] = …).
func stateWrites(text string) []string {
	var out []string
	for _, m := range stateRefRE.FindAllStringIndex(text, -1) {
		s, e := m[0], m[1]
		if s > 0 && identOrDot(text[s-1]) {
			continue
		}
		ref := text[s:e]
		before := strings.TrimRight(text[:s], " \t")
		after := strings.TrimLeft(text[e:], " \t")
		deleted := strings.HasSuffix(before, "delete") &&
			(len(before) == len("delete") || !identOrDot(before[len(before)-len("delete")-1]))
		prefixed := strings.HasSuffix(before, "++") || strings.HasSuffix(before, "--") || deleted
		last := ref[strings.LastIndexByte(ref, '.')+1:]
		if prefixed || assignRE.MatchString(after) || mutators[last] && strings.HasPrefix(after, "(") {
			out = append(out, ref)
		}
	}
	return append(out, assignStateRE.FindAllString(text, -1)...)
}

// kitNames are the public names helpers.py defines at its top level,
// its functions and classes.
func kitNames(t *testing.T) map[string]bool {
	b, err := os.ReadFile(filepath.Join(suite, "helpers.py"))
	if err != nil {
		t.Fatal(err)
	}
	names := map[string]bool{}
	for _, m := range kitDefRE.FindAllStringSubmatch(string(b), -1) {
		names[m[1]] = true
	}
	if len(names) == 0 {
		t.Fatal("helpers.py defines no name at its top level")
	}
	return names
}

// idioms are the old idioms, each with its list.
func idioms(t *testing.T) []idiom {
	return []idiom{
		{"wait_for_timeout( calls", "wait_for_timeout.txt",
			"a fixed pause outlasts nothing on a slow runner and waits for nothing in particular: " +
				"wait for what the test checks, with wait_until, settled, Stream.wait, wait_held or answered of helpers.py",
			matches(fixedWaitRE), false},
		{"plain .route( calls", "page_route.txt",
			"a plain route handler races the page that closes with a request on its way: " +
				"route with safe_route of helpers.py, or tell the page's own reads through Reads",
			matches(plainRouteRE), false},
		{"local definitions of a name helpers.py defines", "kit_names.txt",
			"a local copy of a helper of the kit drifts from it: " +
				"import the name from helpers.py, and a helper several files need goes there under its own name",
			localKitDefs(kitNames(t)), true},
		{"writes of the page's state", "state_writes.txt",
			"data put into the page alone lasts until its next read or event: tell the page's reads what it shows " +
				"through Reads of helpers.py (reads.cube, reads.device, reads.room, reads.change), or the fake Cube",
			stateWrites, true},
	}
}

// readList reads a list: a test file and its count a line, # a comment.
func readList(t *testing.T, name string) map[string]int {
	b, err := os.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]int{}
	sc := bufio.NewScanner(bytes.NewReader(b))
	for n := 1; sc.Scan(); n++ {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		m := listLineRE.FindStringSubmatch(line)
		if m == nil {
			t.Fatalf("%s:%d: want a test file and its count", name, n)
		}
		if _, dup := out[m[1]]; dup {
			t.Fatalf("%s:%d: %s listed twice", name, n, m[1])
		}
		c, _ := strconv.Atoi(m[2])
		out[m[1]] = c
	}
	return out
}

// testFiles are the suite's test files, by name, with their text.
func testFiles(t *testing.T) map[string]string {
	paths, err := filepath.Glob(filepath.Join(suite, "test_*.py"))
	if err != nil || len(paths) == 0 {
		t.Fatalf("no test file in %s: %v", suite, err)
	}
	out := map[string]string{}
	for _, p := range paths {
		b, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		out[filepath.Base(p)] = string(b)
	}
	return out
}

// check compares one idiom's counts with its list: a count above its
// listed one (none listed is 0) is a new use, a count below it a fall the
// list has not taken.
func check(files map[string]string, id idiom, listed map[string]int) []string {
	var out []string
	where := "tools/ui-test/idioms/" + id.list
	for _, f := range slices.Sorted(maps.Keys(files)) {
		uses := id.find(files[f])
		got, want := len(uses), listed[f]
		switch {
		case got > want:
			named := ""
			if id.named {
				named = " (" + strings.Join(uses, ", ") + ")"
			}
			out = append(out, fmt.Sprintf("tools/ui-test/%s: %s: %d%s, %d listed in %s: %s (%s)",
				f, id.what, got, named, want, where, id.instead, section))
		case got < want:
			lower := fmt.Sprintf("lower its line to %d", got)
			if got == 0 {
				lower = "take its line out"
			}
			out = append(out, fmt.Sprintf("tools/ui-test/%s: %s: %d, %d listed in %s: %s in this commit, "+
				"so that the count does not rise again (%s)", f, id.what, got, want, where, lower, section))
		}
	}
	for f, want := range listed {
		if _, ok := files[f]; !ok {
			out = append(out, fmt.Sprintf("%s: %s is listed with %d, but there is no such test file: take its line out", where, f, want))
		}
	}
	slices.Sort(out)
	return out
}

// No test file of the UI suite holds more of an old idiom than its list
// says, and none holds fewer: a list falls with the commit that lowered
// the count, and never rises.
func TestOldIdioms(t *testing.T) {
	files := testFiles(t)
	for _, id := range idioms(t) {
		listed := readList(t, id.list)
		for _, problem := range check(files, id, listed) {
			t.Error(problem)
		}
		total, in := 0, 0
		for _, n := range listed {
			total += n
			in++
		}
		t.Logf("%s: %d listed in %d of %d files", id.what, total, in, len(files))
	}
}

// Each idiom is found where it is used, and only there.
func TestFind(t *testing.T) {
	kit := map[string]bool{"wait_until": true, "mark": true, "Reads": true}
	for _, c := range []struct {
		find func(string) []string
		text string
		want int
	}{
		{matches(fixedWaitRE), `page.wait_for_timeout(200)`, 1},
		{matches(fixedWaitRE), `other.wait_for_timeout(5); tile.page.wait_for_timeout(1)`, 2},
		{matches(fixedWaitRE), `"""in place of a fixed wait_for_timeout"""`, 0},
		{matches(plainRouteRE), `page.route("**/api/v1/me", handle)`, 1},
		{matches(plainRouteRE), `context.route(p, h); ctx.route(p, h)`, 2},
		{matches(plainRouteRE), `safe_route(page, p, h); page.unroute(p); route.fulfill(status=200); route_matches(p, u)`, 0},
		{localKitDefs(kit), "def wait_until(page, expr):\n    def mark(page, s):\nclass Reads:\n", 3},
		{localKitDefs(kit), "def wait_until_shown(page):\ndef _mark(page):\nfrom helpers import mark, wait_until\n", 0},
		{stateWrites, `() => { state.settings.expert = true; applySettings(); }`, 1},
		{stateWrites, `state.updateAt = 0; state.n += 1; state.n++; --state.m; state.ui[rid].timer = null`, 5},
		{stateWrites, `state.cubes = state.cubes.map((c) => c); state.a = state.b = 1`, 3},
		{stateWrites, `delete state.details[rf]; state.rooms.push(r); state.found.splice(0, 1)`, 3},
		{stateWrites, `Object.assign(state, { cubes: [] }); Object.assign(state.settings, { poll: 5 })`, 2},
		{stateWrites, `state.x === 1 && state.y == 2 && state.z !== 3 && state.w >= 4 && state.v <= 5`, 0},
		{stateWrites, `state.events.dispatchEvent(ev); clearInterval(state.pollTimer); f(state.rooms.find((r) => r.id))`, 0},
		{stateWrites, `themeState.scheme = "x"; MassimoParts.log.state.lines = []; undelete state.x`, 0},
		{stateWrites, "state = {\"helper\": \"up-to-date\"}\nstate[\"helper\"] = \"installing\"", 0},
	} {
		if got := len(c.find(c.text)); got != c.want {
			t.Errorf("%q: found %d, want %d", c.text, got, c.want)
		}
	}
}

// The failures say what to do: a new use names the list, what to use
// instead and the section; a fall asks for the list to be lowered; a
// file gone asks for its line to go.
func TestCheck(t *testing.T) {
	id := idiom{"wait_for_timeout( calls", "wait_for_timeout.txt", "wait for what the test checks", matches(fixedWaitRE), false}
	files := map[string]string{
		"test_a.py": "page.wait_for_timeout(1)\npage.wait_for_timeout(2)\n",
		"test_b.py": "page.wait_for_timeout(1)\n",
		"test_c.py": "",
	}
	got := check(files, id, map[string]int{"test_a.py": 1, "test_b.py": 2, "test_c.py": 1, "test_gone.py": 3})
	want := []string{
		"tools/ui-test/idioms/wait_for_timeout.txt: test_gone.py is listed with 3, but there is no such test file: take its line out",
		"tools/ui-test/test_a.py: wait_for_timeout( calls: 2, 1 listed in tools/ui-test/idioms/wait_for_timeout.txt: " +
			"wait for what the test checks (docs/ui-testing.md, section 1)",
		"tools/ui-test/test_b.py: wait_for_timeout( calls: 1, 2 listed in tools/ui-test/idioms/wait_for_timeout.txt: " +
			"lower its line to 1 in this commit, so that the count does not rise again (docs/ui-testing.md, section 1)",
		"tools/ui-test/test_c.py: wait_for_timeout( calls: 0, 1 listed in tools/ui-test/idioms/wait_for_timeout.txt: " +
			"take its line out in this commit, so that the count does not rise again (docs/ui-testing.md, section 1)",
	}
	if !slices.Equal(got, want) {
		t.Errorf("check:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}
