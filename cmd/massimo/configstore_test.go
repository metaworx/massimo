// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package main

import (
	"context"
	"errors"
	"strings"
	"testing"

	"go.metaworx.ch/massimo/internal/env"
	"go.metaworx.ch/massimo/internal/store"
	"go.metaworx.ch/massimo/internal/testkit"
	"go.metaworx.ch/massimo/internal/testkit/storekit"
)

// massimo config shows the plain lines at a stored Cube's address as
// that Cube's, compared with its row, as serve's next start ties them
// (serve.PlainRow), by name or IP with serve's lookup; at another
// address they stay the plain lines whose serial is not known yet.
func TestConfigTiesThePlainLines(t *testing.T) {
	testkit.Swapping(t)
	asked := 0
	plainLookup = func(_ context.Context, host string) ([]string, error) {
		asked++
		if host == "cube.example" {
			return []string{"192.0.2.1"}, nil
		}
		return nil, errors.New("no such host")
	}
	t.Cleanup(func() { plainLookup = nil })
	for _, c := range []struct {
		plain string
		tied  bool
	}{
		{"192.0.2.1", true},
		{"192.0.2.10", false},
		{"cube.example", true},
		{"other.example", false},
	} {
		t.Run(c.plain, func(t *testing.T) {
			clearCubeEnv(t)
			asked = 0
			st := storekit.Open(t, dataDir())
			if _, err := st.AddCube(context.Background(), store.Cube{Serial: "KEQ0000100", Address: "192.0.2.1:62910", Configured: true, Source: store.SourceUI}); err != nil {
				t.Fatal(err)
			}
			st.Close()
			t.Setenv(env.Address, c.plain)
			t.Setenv(env.Proxy, "192.0.2.50")
			var b strings.Builder
			printStore(&b)
			out := b.String()
			tied := strings.Contains(out, "the file's MASSIMO_CUBE_PROXY=192.0.2.50 differs and is ignored")
			plain := strings.Contains(out, "(serial not known yet)")
			if tied != c.tied || plain == c.tied {
				t.Fatalf("tied %v, plain lines %v:\n%s", tied, plain, out)
			}
			if names := strings.HasSuffix(c.plain, ".example"); names != (asked == 1) {
				t.Fatalf("looked up %d times", asked)
			}
		})
	}
}
