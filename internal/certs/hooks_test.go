// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package certs

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"
)

// fakeInfomaniak answers the calls of packaging/acme-dns-hooks/infomaniak.sh
// as the v2 API does for one zone, example.net.
type fakeInfomaniak struct {
	mu      sync.Mutex
	seq     int
	records map[int]map[string]any
}

func (f *fakeInfomaniak) serve(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	reply := func(v any) { json.NewEncoder(w).Encode(map[string]any{"result": "success", "data": v}) }
	if r.Header.Get("Authorization") != "Bearer test-token" {
		json.NewEncoder(w).Encode(map[string]any{"result": "error", "error": map[string]string{"code": "not_authorized", "description": "cannot authenticate"}})
		return
	}
	p := strings.TrimPrefix(r.URL.Path, "/2/zones/")
	zone, rest, _ := strings.Cut(p, "/")
	f.mu.Lock()
	defer f.mu.Unlock()
	switch {
	case rest == "exists":
		reply(zone == "example.net")
	case zone != "example.net":
		json.NewEncoder(w).Encode(map[string]any{"result": "error", "error": map[string]string{"code": "not_found", "description": "no such zone"}})
	case rest == "records" && r.Method == "POST":
		var rec map[string]any
		json.NewDecoder(r.Body).Decode(&rec)
		f.seq++
		rec["id"] = f.seq
		f.records[f.seq] = rec
		reply(map[string]any{"id": f.seq})
	case rest == "records" && r.Method == "GET":
		var list []map[string]any
		for _, rec := range f.records {
			list = append(list, rec)
		}
		reply(list)
	case strings.HasPrefix(rest, "records/") && r.Method == "DELETE":
		id, _ := strconv.Atoi(strings.TrimPrefix(rest, "records/"))
		delete(f.records, id)
		reply(true)
	default:
		http.NotFound(w, r)
	}
}

// record is a stored record, read under the lock.
func (f *fakeInfomaniak) record(id int) (map[string]any, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	rec, ok := f.records[id]
	return rec, ok
}

func (f *fakeInfomaniak) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.records)
}

func TestInfomaniakHook(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("a shell script")
	}
	for _, tool := range []string{"sh", "curl", "jq"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("%s is not installed", tool)
		}
	}
	fake := &fakeInfomaniak{records: map[int]map[string]any{}}
	srv := httptest.NewServer(http.HandlerFunc(fake.serve))
	defer srv.Close()
	tokenFile := filepath.Join(t.TempDir(), "token")
	os.WriteFile(tokenFile, []byte("test-token\n"), 0o600)
	script, _ := filepath.Abs("../../packaging/acme-dns-hooks/infomaniak.sh")
	// A curl in front of the real one writes down its arguments, which
	// every local user can read through ps: the token must not be there,
	// and every call has a time limit.
	realCurl, _ := exec.LookPath("curl")
	bin := t.TempDir()
	argv := filepath.Join(t.TempDir(), "argv")
	os.WriteFile(filepath.Join(bin, "curl"), []byte("#!/bin/sh\nprintf '%s\\n' \"$*\" >> \""+argv+"\"\nexec \""+realCurl+"\" \"$@\"\n"), 0o755)
	run := func(args ...string) (string, error) {
		cmd := exec.Command(script, args...)
		cmd.Env = append(hookEnv(os.Environ()), "INFOMANIAK_ENDPOINT="+srv.URL, "INFOMANIAK_TOKEN_FILE="+tokenFile, "PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"))
		out, err := cmd.CombinedOutput()
		return strings.TrimSpace(string(out)), err
	}
	defer func() {
		b, _ := os.ReadFile(argv)
		calls := strings.Split(strings.TrimSpace(string(b)), "\n")
		if len(calls) < 5 {
			t.Errorf("%d curl calls recorded", len(calls))
		}
		for _, c := range calls {
			if strings.Contains(c, "test-token") || strings.Contains(c, "wrong") {
				t.Errorf("the token on curl's command line: %s", c)
			}
			if !strings.Contains(c, "--max-time") {
				t.Errorf("a curl call without --max-time: %s", c)
			}
		}
	}()

	out, err := run("set", "_acme-challenge.massimo.example.net", "value-one")
	if err != nil || out != "1" {
		t.Fatalf("set: %v %q", err, out)
	}
	if rec, _ := fake.record(1); rec["type"] != "TXT" || rec["source"] != "_acme-challenge.massimo" || rec["target"] != "value-one" || rec["ttl"] != float64(300) {
		t.Fatalf("record %v", rec)
	}
	// A record of the zone itself: the source relative to it.
	out, err = run("set", "_acme-challenge.example.net", "value-two")
	if rec, _ := fake.record(2); err != nil || out != "2" || rec["source"] != "_acme-challenge" {
		t.Fatalf("set at the apex: %v %q %v", err, out, rec)
	}
	// clear removes the records of that name and value, no other.
	if out, err = run("clear", "_acme-challenge.massimo.example.net", "value-one"); err != nil {
		t.Fatalf("clear: %v %q", err, out)
	}
	if _, left := fake.record(1); left || fake.count() != 1 {
		t.Fatalf("records after clear: %d left", fake.count())
	}
	// A name in no zone here, and a wrong token, fail.
	if out, err = run("set", "_acme-challenge.other.example.org", "x"); err == nil || !strings.Contains(out, "no zone") {
		t.Fatalf("unknown zone: %v %q", err, out)
	}
	os.WriteFile(tokenFile, []byte("wrong\n"), 0o600)
	if out, err = run("set", "_acme-challenge.example.net", "x"); err == nil {
		t.Fatalf("wrong token accepted: %q", out)
	}
}
