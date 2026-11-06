// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
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
