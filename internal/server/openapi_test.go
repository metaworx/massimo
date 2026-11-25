// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// route is one registered /api route: the method, the path pattern and
// the capability it needs.
type route struct {
	method, path, capability string
}

// registeredRoutes reads the api( calls of the package's files.
func registeredRoutes(t *testing.T) []route {
	t.Helper()
	code := ""
	files, _ := filepath.Glob("*.go")
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		code += string(b)
	}
	var out []route
	for _, m := range regexp.MustCompile(`api\("(GET|PUT|POST|DELETE) (/api/v1/[^"]+)", "([a-z.]+)"`).FindAllStringSubmatch(code, -1) {
		out = append(out, route{method: m[1], path: m[2], capability: m[3]})
	}
	if len(out) == 0 {
		t.Fatal("no route found")
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].path != out[j].path {
			return out[i].path < out[j].path
		}
		return out[i].method < out[j].method
	})
	return out
}

// TestOpenAPICoversRoutes checks that every /api route registered in the
// package (the api( calls of its files) has its operation in
// docs/api-v1-openapi.yaml with the same capability as x-capability,
// and the other way round, so the three cannot drift apart.
func TestOpenAPICoversRoutes(t *testing.T) {
	t.Parallel()
	registered := map[string]string{}
	for _, r := range registeredRoutes(t) {
		registered[r.method+" "+r.path] = r.capability
	}
	spec, err := os.ReadFile("../../docs/api-v1-openapi.yaml")
	if err != nil {
		t.Fatal(err)
	}
	// The description parses, and every reference into components
	// resolves: a parameter pasted into the wrong place breaks both.
	var doc map[string]any
	if err := yaml.Unmarshal(spec, &doc); err != nil {
		t.Fatalf("docs/api-v1-openapi.yaml does not parse: %v", err)
	}
	var walk func(path string, v any)
	walk = func(path string, v any) {
		switch v := v.(type) {
		case map[string]any:
			for k, e := range v {
				if k == "$ref" {
					if ref, ok := e.(string); ok && !resolves(doc, ref) {
						t.Errorf("%s: %s does not resolve", path, ref)
					}
					continue
				}
				walk(path+"/"+k, e)
			}
		case []any:
			for i, e := range v {
				walk(path+"/"+strconv.Itoa(i), e)
			}
		}
	}
	walk("", doc)
	documented := map[string]string{} // "METHOD /path" to x-capability
	seen := map[string]bool{}
	path, op := "", ""
	for _, line := range strings.Split(string(spec), "\n") {
		switch {
		case strings.HasPrefix(line, "  /api/v1/") && strings.HasSuffix(line, ":"):
			path = strings.TrimSuffix(strings.TrimSpace(line), ":")
			if seen[path] {
				// YAML parsers keep only the last of duplicate keys, so
				// the methods of the first would be lost.
				t.Errorf("path %s appears twice in docs/api-v1-openapi.yaml; list all its methods under one key", path)
			}
			seen[path] = true
			op = ""
		case strings.HasPrefix(line, "  /") || strings.HasPrefix(line, "components:"):
			path, op = "", ""
		case path != "" && regexp.MustCompile(`^    (get|put|post|delete):$`).MatchString(line):
			op = strings.ToUpper(strings.TrimSuffix(strings.TrimSpace(line), ":")) + " " + path
			documented[op] = ""
		case op != "" && strings.HasPrefix(line, "      x-capability: "):
			documented[op] = strings.TrimSpace(strings.TrimPrefix(line, "      x-capability: "))
		}
	}
	if len(documented) == 0 {
		t.Fatal("no documented operation found")
	}
	for r, c := range registered {
		dc, ok := documented[r]
		switch {
		case !ok:
			t.Errorf("route %s is not in docs/api-v1-openapi.yaml", r)
		case dc != c:
			t.Errorf("route %s needs %q, docs/api-v1-openapi.yaml says x-capability %q", r, c, dc)
		}
	}
	for r := range documented {
		if _, ok := registered[r]; !ok {
			t.Errorf("documented operation %s has no route", r)
		}
	}
}

// capabilityTableHeader is the header of the table of capabilities and
// the routes each guards, which docs/api-v1.md ("Authentication") keeps.
const capabilityTableHeader = "| Capability | Allows | Routes |"

// The table's cells: a row's key, a route mention of its Routes cell
// (methods before it, each a code span joined by "/", share its path),
// a method's code span, and any code span.
var (
	capabilityKey = regexp.MustCompile("^`([a-z.]+)`$")
	routeMention  = regexp.MustCompile("((?:`(?:GET|PUT|POST|DELETE)`/)*)`(GET|PUT|POST|DELETE) ([^`]*)`")
	routeMethod   = regexp.MustCompile("`(GET|PUT|POST|DELETE)`")
	codeSpan      = regexp.MustCompile("`([^`]*)`")
)

// headerLine says whether a line is the capability table's header.
func headerLine(l string) bool {
	return strings.TrimSpace(l) == capabilityTableHeader
}

// mention is one route a row of the capability table names: the text
// it was read from, its methods, and its path, … standing for any
// characters.
type mention struct {
	text    string
	methods []string
	path    *regexp.Regexp
}

// names says whether the mention names route r.
func (m mention) names(r route) bool {
	return slices.Contains(m.methods, r.method) && m.path.MatchString(r.path)
}

// capabilityTable reads the table of docs/api-v1.md: the rows' keys in
// their order, and each row's route mentions.
func capabilityTable(t *testing.T, doc string) ([]string, map[string][]mention) {
	t.Helper()
	lines := strings.Split(doc, "\n")
	start := slices.IndexFunc(lines, headerLine)
	if start < 0 {
		t.Fatalf("docs/api-v1.md has no table %q", capabilityTableHeader)
	}
	var keys []string
	rows := map[string][]mention{}
	for _, l := range lines[min(start+2, len(lines)):] {
		l = strings.TrimSpace(l)
		if !strings.HasPrefix(l, "|") {
			break
		}
		cells := strings.Split(strings.TrimSuffix(strings.TrimPrefix(l, "|"), "|"), "|")
		if len(cells) != 3 {
			t.Errorf("docs/api-v1.md: a row of the capability table has %d cells, not 3: %s", len(cells), l)
			continue
		}
		m := capabilityKey.FindStringSubmatch(strings.TrimSpace(cells[0]))
		if m == nil {
			t.Errorf("docs/api-v1.md: row %s: the first cell is no capability's key in a code span", strings.TrimSpace(cells[0]))
			continue
		}
		if _, twice := rows[m[1]]; twice {
			t.Errorf("docs/api-v1.md: row %s appears twice", m[1])
			continue
		}
		keys = append(keys, m[1])
		rows[m[1]] = mentions(t, m[1], cells[2])
	}
	return keys, rows
}

// mentions reads the route mentions of row key's Routes cell; a code
// span left over that is a method or a path is an error.
func mentions(t *testing.T, key, cell string) []mention {
	t.Helper()
	out := []mention{}
	for _, m := range routeMention.FindAllStringSubmatch(cell, -1) {
		if !strings.HasPrefix(m[3], "/api/v1/") && !strings.HasPrefix(m[3], "…") {
			t.Errorf("docs/api-v1.md: row %s: %s: a path starts with /api/v1/ or …", key, m[0])
			continue
		}
		var methods []string
		for _, mm := range routeMethod.FindAllStringSubmatch(m[1], -1) {
			methods = append(methods, mm[1])
		}
		parts := strings.Split(m[3], "…")
		for i, p := range parts {
			parts[i] = regexp.QuoteMeta(p)
		}
		out = append(out, mention{
			text:    m[0],
			methods: append(methods, m[2]),
			path:    regexp.MustCompile("^" + strings.Join(parts, ".*") + "$"),
		})
	}
	for _, s := range codeSpan.FindAllStringSubmatch(routeMention.ReplaceAllString(cell, ""), -1) {
		if routeMethod.MatchString(s[0]) || strings.HasPrefix(s[1], "/") || strings.HasPrefix(s[1], "…") {
			t.Errorf("docs/api-v1.md: row %s: %s is no route of the form `METHOD path`", key, s[0])
		}
	}
	return out
}

// TestCapabilityTableCoversRoutes checks the table of capabilities in
// docs/api-v1.md ("Authentication") against the package: it is the only
// such table under docs/, its rows are the capabilities of auth.go in
// their order, every route a row names is registered, and every route
// registered with a capability is named in that capability's row. A row
// may name a route registered with another capability, where a handler,
// or allowed in auth.go, checks its own as well; of such a route only
// that it exists is checked.
func TestCapabilityTableCoversRoutes(t *testing.T) {
	t.Parallel()
	const apiDoc = "../../docs/api-v1.md"
	doc, err := os.ReadFile(apiDoc)
	if err != nil {
		t.Fatal(err)
	}
	err = filepath.WalkDir("../../docs", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".md") || filepath.ToSlash(path) == apiDoc {
			return err
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if slices.ContainsFunc(strings.Split(string(b), "\n"), headerLine) {
			t.Errorf("%s has a table %q too; it is kept in docs/api-v1.md alone", strings.TrimPrefix(filepath.ToSlash(path), "../../"), capabilityTableHeader)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	keys, rows := capabilityTable(t, string(doc))
	for _, k := range keys {
		if !validCapability(k) {
			t.Errorf("docs/api-v1.md: row %s is no capability of auth.go's capabilities", k)
		}
	}
	var want []string
	for _, c := range capabilities {
		if _, ok := rows[c]; ok {
			want = append(want, c)
		} else {
			t.Errorf("docs/api-v1.md: capability %s has no row", c)
		}
	}
	got := slices.DeleteFunc(slices.Clone(keys), func(k string) bool { return !validCapability(k) })
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("docs/api-v1.md: row %d is %s, where auth.go's capabilities has %s", i+1, got[i], want[i])
			break
		}
	}

	routes := registeredRoutes(t)
	for _, k := range keys {
		for _, m := range rows[k] {
			for _, method := range m.methods {
				one := mention{text: m.text, methods: []string{method}, path: m.path}
				if !slices.ContainsFunc(routes, one.names) {
					t.Errorf("docs/api-v1.md: row %s: %s: no %s route has that path", k, m.text, method)
				}
			}
		}
	}
	for _, r := range routes {
		if r.capability == capNone || r.capability == capAny {
			continue
		}
		ms, ok := rows[r.capability]
		switch {
		case !ok:
			t.Errorf("route %s %s needs %s, which has no row in docs/api-v1.md", r.method, r.path, r.capability)
		case !slices.ContainsFunc(ms, func(m mention) bool { return m.names(r) }):
			t.Errorf("docs/api-v1.md: row %s names no route %s %s, registered with it", r.capability, r.method, r.path)
		}
	}
}

// resolves says whether a "#/components/..." reference names something
// in the document.
func resolves(doc map[string]any, ref string) bool {
	path, ok := strings.CutPrefix(ref, "#/")
	if !ok {
		return false
	}
	var cur any = doc
	for _, part := range strings.Split(path, "/") {
		m, ok := cur.(map[string]any)
		if !ok {
			return false
		}
		if cur, ok = m[part]; !ok {
			return false
		}
	}
	return true
}
