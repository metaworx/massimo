// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

import (
	"database/sql"
	"path/filepath"
	"testing"

	"go.metaworx.ch/massimo/internal/store"
)

// A Cube's and a setting's change through the API name the user who
// made it in updated_by (AP StoreRaise v1.2, Block 2).
func TestAPIChangesSayWhoMadeThem(t *testing.T) {
	t.Parallel()
	opts := storeOptions(t, t.TempDir(), "secret")
	ts := startServer(t, withOptions(opts)).ts
	token := userToken(t, opts.Store, "anna", store.RoleOwner)
	anna, ok, err := opts.Store.User(t.Context(), "anna")
	if err != nil || !ok {
		t.Fatalf("anna: %v %v", ok, err)
	}
	if code, body := callAPI(t, ts, "PUT", "/api/v1/cubes/KEQ0000100", token, `{"name":"Cellar"}`); code != 200 {
		t.Fatalf("PUT cube: %d %s", code, body)
	}
	if code, body := callAPI(t, ts, "PUT", "/api/v1/settings", token, `{"window_warn_after":"2m","window_alert_after":"4m"}`); code != 200 {
		t.Fatalf("PUT settings: %d %s", code, body)
	}
	db, err := sql.Open("sqlite", "file:"+filepath.ToSlash(opts.Store.File())+"?mode=ro")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for what, q := range map[string]string{
		"cube":    `SELECT updated_by FROM cubes WHERE serial = 'KEQ0000100'`,
		"setting": `SELECT updated_by FROM settings WHERE key = '` + store.WindowWarnKey + `'`,
	} {
		var by sql.NullInt64
		if err := db.QueryRow(q).Scan(&by); err != nil || by.Int64 != anna.ID {
			t.Errorf("%s: updated_by %v %v, want %d", what, by, err, anna.ID)
		}
	}
}
