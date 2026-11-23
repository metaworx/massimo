// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package store

import (
	"bytes"
	"context"
	"log/slog"
	"testing"
)

// The password fields' eye (AP MobileApp, D84-A) is on by default, and off
// as the setting says; GET /healthz reads it at every call, so a request
// that went away before the read leaves the default and nothing in the
// log, and so does the password sign-in's setting, read the same way.
func TestPasswordReveal(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := open(t, t.TempDir())
	var log bytes.Buffer
	s.log = slog.New(slog.NewTextHandler(&log, nil))
	if !s.PasswordReveal(ctx) {
		t.Fatal("off by default")
	}
	if err := s.SetSetting(ctx, PasswordRevealKey, false); err != nil {
		t.Fatal(err)
	}
	if s.PasswordReveal(ctx) {
		t.Fatal("on, the setting off")
	}
	gone, cancel := context.WithCancel(ctx)
	cancel()
	if !s.PasswordReveal(gone) || !s.PasswordLogin(gone) {
		t.Error("a request gone: the defaults, on")
	}
	if log.Len() != 0 {
		t.Errorf("a request gone is no failure of the store's: %s", log.String())
	}
}
