// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package main

import (
	"flag"
	"reflect"
	"testing"
)

func TestReorderFlags(t *testing.T) {
	fs := flag.NewFlagSet("t", flag.ContinueOnError)
	fs.String("addr", "", "")
	fs.String("login", "", "")
	fs.Bool("quiet", false, "")
	got := reorderFlags(fs, []string{"-addr", "1.2.3.4", "u:http://x", "-login", "a:b", "-quiet", "l:", "-unknown"})
	want := []string{"-addr", "1.2.3.4", "-login", "a:b", "-quiet", "u:http://x", "l:", "-unknown"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %q", got)
	}
}
