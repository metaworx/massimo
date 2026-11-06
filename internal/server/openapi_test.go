// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

import (
	"os"
	"path/filepath"
	"regexp"
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
