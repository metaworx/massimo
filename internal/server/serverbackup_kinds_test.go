// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"golang.org/x/net/webdav"

	"go.metaworx.ch/massimo/internal/serverbackup"
	"go.metaworx.ch/massimo/internal/store"
)

// The kinds' answer carries a field's warning, WebDAV's url warning of
// plain http; a WebDAV target is added with its password apart, tested,
// and shown without it.
func TestServerBackupWebDAVKind(t *testing.T) {
	t.Parallel()
	r, _ := startSB(t, time.Date(2026, 11, 27, 10, 0, 0, 0, time.UTC))
	var kinds []kindView
	if code := getJSON(t, r.ts, "secret", "/api/v1/server-backup-targets/kinds", &kinds); code != http.StatusOK {
		t.Fatalf("kinds: %d", code)
	}
	var dav kindView
	for _, k := range kinds {
		switch k.Name {
		case serverbackup.KindWebDAV:
			dav = k
		case serverbackup.KindLocal:
			if k.Fields[0].Warn != "" {
				t.Fatalf("local warns: %+v", k)
			}
		}
	}
	if f := dav.Fields; dav.Secret != "password" || len(f) != 3 || f[0].Key != "url" || f[0].Warn == "" || f[1].Key != "user" || f[1].Warn != "" || f[2].Type != "secret" {
		t.Fatalf("webdav %+v", dav)
	}

	const pass = "lemon-tree-17"
	h := &webdav.Handler{Prefix: "/dav", FileSystem: webdav.NewMemFS(), LockSystem: webdav.NewMemLS()}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if u, p, ok := req.BasicAuth(); !ok || u != "massimo" || p != pass {
			http.Error(w, "who are you", http.StatusUnauthorized)
			return
		}
		h.ServeHTTP(w, req)
	}))
	t.Cleanup(srv.Close)
	body, _ := json.Marshal(map[string]any{"name": "NAS", "kind": "webdav", "config": map[string]string{"url": srv.URL + "/dav/massimo", "user": "massimo"}, "secret": pass})
	code, text := callAPI(t, r.ts, "POST", "/api/v1/server-backup-targets", "secret", string(body))
	if code != http.StatusCreated || strings.Contains(text, pass) || strings.Contains(text, "scan_error") {
		t.Fatalf("added: %d %s", code, text)
	}
	var added struct {
		Target struct {
			UUID      string `json:"uuid"`
			HasSecret bool   `json:"has_secret"`
		} `json:"target"`
	}
	if err := json.Unmarshal([]byte(text), &added); err != nil || !added.Target.HasSecret {
		t.Fatalf("added %s %v", text, err)
	}
	code, text = callAPI(t, r.ts, "POST", "/api/v1/server-backup-targets/"+added.Target.UUID+"/test", "secret", "")
	var tested struct {
		OK bool `json:"ok"`
	}
	if json.Unmarshal([]byte(text), &tested); code != http.StatusOK || !tested.OK {
		t.Fatalf("test: %d %s", code, text)
	}
	code, text = callAPI(t, r.ts, "GET", "/api/v1/server-backup-targets", "secret", "")
	var list []struct {
		Kind      string `json:"kind"`
		HasSecret bool   `json:"has_secret"`
	}
	if json.Unmarshal([]byte(text), &list); code != http.StatusOK || strings.Contains(text, pass) || len(list) != 1 || list[0].Kind != "webdav" || !list[0].HasSecret {
		t.Fatalf("list: %d %s", code, text)
	}
}

// A change to the settings of a target with a secret field needs the
// secret again in the same request, since they say where it goes: a
// WebDAV server's URL or its user, an SMB host or its domain; else one
// who may change targets but never sees a secret could send it to a
// server of their own with the target's test. The same settings, or a
// change to another field, keep the secret.
func TestServerBackupSecretFollowsConfig(t *testing.T) {
	t.Parallel()
	r, _ := startSB(t, time.Date(2026, 11, 27, 10, 0, 0, 0, time.UTC))
	ctx := t.Context()
	def, _ := json.Marshal(serverbackup.DefaultPolicy)
	add := func(name, kind string, cfg map[string]string) string {
		tg, err := r.st.AddServerBackupTarget(ctx, store.ServerBackupTarget{Name: name, Kind: kind, Config: cfg, Secret: "lemon-tree-17",
			Takes: string(serverbackup.TakesEvery), Retention: def, Enabled: true}, 0)
		if err != nil {
			t.Fatal(err)
		}
		return tg.UUID
	}
	dav := add("NAS", serverbackup.KindWebDAV, map[string]string{"url": "https://nas.example/dav/", "user": "massimo"})
	smb := add("Share", serverbackup.KindSMB, map[string]string{"host": "nas.example", "share": "backups", "user": "massimo"})
	for _, c := range []struct {
		uuid, body string
		want       int
	}{
		{dav, `{"config":{"url":"http://attacker.example/","user":"massimo"}}`, http.StatusBadRequest},
		{dav, `{"config":{"url":"https://nas.example/dav/","user":"mallory"}}`, http.StatusBadRequest},
		{smb, `{"config":{"host":"attacker.example","share":"backups","user":"massimo"}}`, http.StatusBadRequest},
		{smb, `{"config":{"host":"nas.example","share":"backups","user":"massimo","domain":"OTHER"}}`, http.StatusBadRequest},
		{dav, `{"config":{"url":"https://nas.example/dav/","user":"massimo"},"takes":"daily"}`, http.StatusOK},
		{smb, `{"retention":{"last":3}}`, http.StatusOK},
		{smb, `{"config":{"host":"nas2.example","share":"backups","user":"massimo"},"secret":"lime-tree-18"}`, http.StatusOK},
	} {
		code, text := callAPI(t, r.ts, "PUT", "/api/v1/server-backup-targets/"+c.uuid, "secret", c.body)
		if code != c.want || (c.want == http.StatusBadRequest && !strings.Contains(text, "send it again")) {
			t.Errorf("PUT %s: %d %s, want %d", c.body, code, text, c.want)
		}
	}
	if tg, _, _ := r.st.ServerBackupTarget(ctx, dav); tg.Config["url"] != "https://nas.example/dav/" || tg.Config["user"] != "massimo" ||
		tg.Secret != "lemon-tree-17" || tg.Takes != string(serverbackup.TakesDaily) {
		t.Errorf("the WebDAV target %+v", tg.Config)
	}
	if tg, _, _ := r.st.ServerBackupTarget(ctx, smb); tg.Config["host"] != "nas2.example" || tg.Secret != "lime-tree-18" {
		t.Errorf("the SMB target %+v", tg.Config)
	}
}
