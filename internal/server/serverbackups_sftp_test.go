// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"errors"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
)

// sshHost is an SSH server on a loopback port that shows its host key
// and admits nobody: enough for a client that stops at the host key.
func sshHost(t *testing.T) (addr, fingerprint string) {
	t.Helper()
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := ssh.NewSignerFromKey(priv)
	if err != nil {
		t.Fatal(err)
	}
	cfg := &ssh.ServerConfig{PublicKeyCallback: func(ssh.ConnMetadata, ssh.PublicKey) (*ssh.Permissions, error) {
		return nil, errors.New("nobody is admitted")
	}}
	cfg.AddHostKey(signer)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	t.Cleanup(func() { ln.Close(); wg.Wait() })
	wg.Go(func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			wg.Go(func() {
				c.SetDeadline(time.Now().Add(10 * time.Second))
				if sc, _, _, err := ssh.NewServerConn(c, cfg); err == nil {
					sc.Close()
				}
				c.Close()
			})
		}
	})
	return ln.Addr().String(), ssh.FingerprintSHA256(signer.PublicKey())
}

// An SFTP target: the server makes its key and shows the public line,
// takes none given, makes a new one for "secret": "", and answers the
// host's key to confirm, at the add's scan and at a test, with the one
// confirmed where it changed; the private key in no answer.
func TestServerBackupSFTPTarget(t *testing.T) {
	t.Parallel()
	r, _ := startSB(t, time.Date(2026, 11, 27, 10, 0, 0, 0, time.UTC))
	addr, fp := sshHost(t)
	host, port, _ := net.SplitHostPort(addr)
	call := func(method, path, body string, want int) map[string]any {
		t.Helper()
		code, text := callAPI(t, r.ts, method, path, "secret", body)
		if code != want {
			t.Fatalf("%s %s %s: %d %s, want %d", method, path, body, code, text, want)
		}
		if strings.Contains(text, "PRIVATE KEY") {
			t.Fatalf("%s %s answers the private key", method, path)
		}
		var v map[string]any
		json.Unmarshal([]byte(text), &v)
		return v
	}
	config := func(hostKey string) string {
		return `{"host":"` + host + `","port":"` + port + `","user":"backup","path":"massimo","host_key":"` + hostKey + `"}`
	}
	if v := call("POST", "/api/v1/server-backup-targets", `{"name":"nas","kind":"sftp","config":`+config("")+`,"secret":"mine"}`, 400); !strings.Contains(v["error"].(string), "the server makes its key") {
		t.Fatalf("a secret given: %v", v)
	}
	v := call("POST", "/api/v1/server-backup-targets", `{"name":"nas","kind":"sftp","config":`+config("")+`}`, 201)
	target := v["target"].(map[string]any)
	key, _ := target["public_key"].(string)
	if !strings.HasPrefix(key, "ssh-ed25519 AAAA") || target["has_secret"] != true || v["host_key"] != fp || v["pinned_host_key"] != nil {
		t.Fatalf("added %v", v)
	}
	if !strings.Contains(v["scan_error"].(string), "not confirmed") {
		t.Fatalf("scan %v", v["scan_error"])
	}
	id := target["uuid"].(string)
	if v := call("POST", "/api/v1/server-backup-targets/"+id+"/test", "", 200); v["ok"] != false || v["host_key"] != fp || v["pinned_host_key"] != nil {
		t.Fatalf("test %v", v)
	}
	other := "SHA256:" + strings.Repeat("A", 43)
	call("PUT", "/api/v1/server-backup-targets/"+id, `{"config":`+config(other)+`}`, 200)
	if v := call("POST", "/api/v1/server-backup-targets/"+id+"/test", "", 200); v["ok"] != false || v["host_key"] != fp || v["pinned_host_key"] != other || !strings.Contains(v["error"].(string), "refused") {
		t.Fatalf("test of a changed key %v", v)
	}
	if v := call("PUT", "/api/v1/server-backup-targets/"+id, `{"takes":"weekly"}`, 200); v["target"].(map[string]any)["public_key"] != key {
		t.Fatalf("a change of what it takes changed the key: %v", v)
	}
	call("PUT", "/api/v1/server-backup-targets/"+id, `{"secret":"mine"}`, 400)
	v = call("PUT", "/api/v1/server-backup-targets/"+id, `{"secret":""}`, 200)
	if fresh, _ := v["target"].(map[string]any)["public_key"].(string); !strings.HasPrefix(fresh, "ssh-ed25519 AAAA") || fresh == key {
		t.Fatalf("a new key %q, was %q", fresh, key)
	}
	var list []map[string]any
	getJSON(t, r.ts, "secret", "/api/v1/server-backup-targets", &list)
	if len(list) != 1 || list[0]["public_key"] == nil || list[0]["config"].(map[string]any)["host_key"] != other {
		t.Fatalf("list %v", list)
	}
	call("POST", "/api/v1/server-backup-targets", `{"name":"disk","kind":"local","config":{"path":"backups"}}`, 201)
	getJSON(t, r.ts, "secret", "/api/v1/server-backup-targets", &list)
	for _, tv := range list {
		if (tv["public_key"] != nil) != (tv["kind"] == "sftp") {
			t.Fatalf("public_key of %v", tv)
		}
	}
}
