// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// TestOpenAPICoversRoutes checks that every /api route registered in
// server.go and config_api.go has a path entry in docs/api-v1-openapi.yaml
// and the other way round, so the two cannot drift apart.
func TestOpenAPICoversRoutes(t *testing.T) {
	code := ""
	for _, f := range []string{"server.go", "config_api.go"} {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		code += string(b)
	}
	registered := map[string]bool{}
	for _, m := range regexp.MustCompile(`api\("(GET|PUT|POST|DELETE) (/api/v1/[^"]+)"`).FindAllStringSubmatch(code, -1) {
		registered[m[2]] = true
	}
	spec, err := os.ReadFile("../../docs/api-v1-openapi.yaml")
	if err != nil {
		t.Fatal(err)
	}
	documented := map[string]bool{}
	for _, line := range strings.Split(string(spec), "\n") {
		if strings.HasPrefix(line, "  /api/v1/") && strings.HasSuffix(line, ":") {
			p := strings.TrimSuffix(strings.TrimSpace(line), ":")
			if documented[p] {
				// YAML parsers keep only the last of duplicate keys, so
				// the methods of the first would be lost.
				t.Errorf("path %s appears twice in docs/api-v1-openapi.yaml; list all its methods under one key", p)
			}
			documented[p] = true
		}
	}
	if len(registered) == 0 || len(documented) == 0 {
		t.Fatalf("found %d routes and %d documented paths", len(registered), len(documented))
	}
	for p := range registered {
		if !documented[p] {
			t.Errorf("route %s is not in docs/api-v1-openapi.yaml", p)
		}
	}
	for p := range documented {
		if !registered[p] {
			t.Errorf("documented path %s has no route", p)
		}
	}
}
