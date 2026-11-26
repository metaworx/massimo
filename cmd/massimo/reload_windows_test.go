// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

//go:build windows

package main

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"go.metaworx.ch/massimo/internal/env"
	"go.metaworx.ch/massimo/internal/server"
	"go.metaworx.ch/massimo/internal/testkit/storekit"
)

// On Windows, which takes a name in any case, a reload sees a setting
// named in another case as the start reads it, by its key (env.NameKey):
// a change is pending, a value the next start refuses is refused, and a
// change of value and case after a start in lower case is a change,
// Windows keeping the name's first spelling (AP StartConfiguration,
// D53-A, D54-A).
func TestReloadSeesANameInAnyCase(t *testing.T) {
	unsetForTest(t, env.TrustedProxies, env.HSTS, "MASSIMO_LISTEN")
	discard := slog.New(slog.NewTextHandler(io.Discard, nil))
	for _, c := range []struct {
		what, start, reload string
		pending, refused    []string // refused: the settings each refusal names
		spelled             string   // the name as the environment holds it after the reload
	}{
		{"a change in lower case", "", "massimo_trusted_proxies=127.0.0.1\n", []string{env.TrustedProxies}, nil, "massimo_trusted_proxies"},
		{"a refused value in lower case", "", "massimo_hsts=x\n", []string{env.HSTS}, []string{env.HSTS}, "massimo_hsts"},
		{"massimo_listen=:1, then MASSIMO_LISTEN=:3", "massimo_listen=:1\n", "MASSIMO_LISTEN=:3\n", []string{"MASSIMO_LISTEN"}, nil, "massimo_listen"},
	} {
		p := filepath.Join(t.TempDir(), "env")
		os.WriteFile(p, []byte(c.start), 0o600)
		if _, err := env.LoadFile(p); err != nil {
			t.Fatal(err)
		}
		srv := server.New(server.Options{Store: storekit.Open(t, ""), Logger: discard})
		r := newSettingsReloader(p, discard, srv, settings(), fixedSettings(nil), nil)
		os.WriteFile(p, []byte(c.reload), 0o600)
		_, pending, refused, err := r.reload(t.Context(), "test")
		var by []string
		for _, f := range refused {
			by = append(by, f.Settings...)
		}
		spelled := slices.ContainsFunc(os.Environ(), func(kv string) bool { return strings.HasPrefix(kv, c.spelled+"=") })
		if err != nil || !slices.Equal(pending, c.pending) || !slices.Equal(by, c.refused) || !spelled {
			t.Errorf("%s: pending %v, refused %+v, %v, spelled %s: %v; want pending %v, refusals of %v", c.what, pending, refused, err, c.spelled, spelled, c.pending, c.refused)
		}
		os.WriteFile(p, nil, 0o600)
		env.ReloadFile(p) // the env package forgets the case's names
	}
}
