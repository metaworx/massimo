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

// fakeCloudflare answers the calls of packaging/acme-dns-hooks/cloudflare.sh
// as API v4 does for one zone, example.net, with the id zone-1; a record's
// content comes back in the quotes Cloudflare may add to TXT content.
type fakeCloudflare struct {
	mu      sync.Mutex
	seq     int
	lookups int
	records map[string]map[string]any
}

func (f *fakeCloudflare) serve(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	reply := func(v any) {
		json.NewEncoder(w).Encode(map[string]any{"success": true, "errors": []any{}, "result": v})
	}
	refuse := func(status, code int, msg string) {
		w.WriteHeader(status)
		json.NewEncoder(w).Encode(map[string]any{"success": false, "errors": []map[string]any{{"code": code, "message": msg}}, "result": nil})
	}
	if r.Header.Get("Authorization") != "Bearer test-token" {
		refuse(http.StatusForbidden, 10000, "Authentication error")
		return
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if r.URL.Path == "/zones" && r.Method == "GET" {
		f.lookups++
		if r.URL.Query().Get("name") == "example.net" {
			reply([]map[string]any{{"id": "zone-1", "name": "example.net"}})
		} else {
			reply([]map[string]any{})
		}
		return
	}
	rest, ok := strings.CutPrefix(r.URL.Path, "/zones/zone-1/dns_records")
	if !ok {
		refuse(http.StatusNotFound, 7003, "Could not route to "+r.URL.Path)
		return
	}
	switch {
	case rest == "" && r.Method == "POST":
		var rec map[string]any
		json.NewDecoder(r.Body).Decode(&rec)
		f.seq++
		rec["id"] = "rec-" + strconv.Itoa(f.seq)
		f.records[rec["id"].(string)] = rec
		reply(rec)
	case rest == "" && r.Method == "GET":
		q := r.URL.Query()
		list := []map[string]any{}
		for _, rec := range f.records {
			if rec["type"] == q.Get("type") && rec["name"] == q.Get("name") {
				shown := map[string]any{}
				for k, v := range rec {
					shown[k] = v
				}
				shown["content"] = `"` + rec["content"].(string) + `"`
				list = append(list, shown)
			}
		}
		reply(list)
	case strings.HasPrefix(rest, "/") && r.Method == "DELETE":
		id := strings.TrimPrefix(rest, "/")
		delete(f.records, id)
		reply(map[string]any{"id": id})
	default:
		refuse(http.StatusNotFound, 7003, "Could not route to "+r.URL.Path)
	}
}

func (f *fakeCloudflare) record(id string) (map[string]any, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	rec, ok := f.records[id]
	return rec, ok
}

func (f *fakeCloudflare) counts() (records, lookups int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.records), f.lookups
}

func TestCloudflareHook(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("a shell script")
	}
	for _, tool := range []string{"sh", "curl", "jq"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("%s is not installed", tool)
		}
	}
	fake := &fakeCloudflare{records: map[string]map[string]any{}}
	srv := httptest.NewServer(http.HandlerFunc(fake.serve))
	defer srv.Close()
	tokenFile := filepath.Join(t.TempDir(), "token")
	os.WriteFile(tokenFile, []byte("test-token\n"), 0o600)
	script, _ := filepath.Abs("../../packaging/acme-dns-hooks/cloudflare.sh")
	// As for the Infomaniak hook: a curl in front of the real one writes
	// down its arguments, which must hold neither token, and every call
	// has a time limit.
	realCurl, _ := exec.LookPath("curl")
	bin := t.TempDir()
	argv := filepath.Join(t.TempDir(), "argv")
	os.WriteFile(filepath.Join(bin, "curl"), []byte("#!/bin/sh\nprintf '%s\\n' \"$*\" >> \""+argv+"\"\nexec \""+realCurl+"\" \"$@\"\n"), 0o755)
	run := func(extra []string, args ...string) (string, error) {
		cmd := exec.Command(script, args...)
		cmd.Env = append(hookEnv(os.Environ()), "CLOUDFLARE_ENDPOINT="+srv.URL, "CLOUDFLARE_TOKEN_FILE="+tokenFile, "PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"))
		cmd.Env = append(cmd.Env, extra...)
		out, err := cmd.CombinedOutput()
		return strings.TrimSpace(string(out)), err
	}
	defer func() {
		b, _ := os.ReadFile(argv)
		calls := strings.Split(strings.TrimSpace(string(b)), "\n")
		if len(calls) < 8 {
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

	// Below the zone: massimo.example.net is no zone, example.net is.
	out, err := run(nil, "set", "_acme-challenge.massimo.example.net", "value-one")
	if err != nil || out != "rec-1" {
		t.Fatalf("set: %v %q", err, out)
	}
	if rec, _ := fake.record("rec-1"); rec["type"] != "TXT" || rec["name"] != "_acme-challenge.massimo.example.net" || rec["content"] != "value-one" || rec["ttl"] != float64(120) {
		t.Fatalf("record %v", rec)
	}
	if _, lookups := fake.counts(); lookups != 2 {
		t.Fatalf("%d zone lookups, want 2", lookups)
	}
	// At the zone's apex, the full name as well.
	out, err = run(nil, "set", "_acme-challenge.example.net", "value-two")
	if rec, _ := fake.record("rec-2"); err != nil || out != "rec-2" || rec["name"] != "_acme-challenge.example.net" {
		t.Fatalf("set at the apex: %v %q %v", err, out, rec)
	}
	// clear removes the records of that name and value, quoted by the
	// API, and no other.
	if out, err = run(nil, "clear", "_acme-challenge.massimo.example.net", "value-one"); err != nil {
		t.Fatalf("clear: %v %q", err, out)
	}
	if _, left := fake.record("rec-1"); left {
		t.Fatal("rec-1 left after clear")
	}
	if n, _ := fake.counts(); n != 1 {
		t.Fatalf("records after clear: %d, want 1", n)
	}
	// CLOUDFLARE_ZONE_ID needs no lookup.
	_, before := fake.counts()
	out, err = run([]string{"CLOUDFLARE_ZONE_ID=zone-1"}, "set", "_acme-challenge.massimo.example.net", "value-three")
	if _, after := fake.counts(); err != nil || out != "rec-3" || after != before {
		t.Fatalf("with CLOUDFLARE_ZONE_ID: %v %q, lookups %d then %d", err, out, before, after)
	}
	// A name in no zone here, and a wrong token, fail and say why.
	if out, err = run(nil, "set", "_acme-challenge.other.example.org", "x"); err == nil || !strings.Contains(out, "CLOUDFLARE_ZONE_ID") {
		t.Fatalf("unknown zone: %v %q", err, out)
	}
	os.WriteFile(tokenFile, []byte("wrong\n"), 0o600)
	if out, err = run(nil, "set", "_acme-challenge.example.net", "x"); err == nil || !strings.Contains(out, "Authentication error") {
		t.Fatalf("wrong token: %v %q", err, out)
	}
}

// TestHooksNeedCurlAndJq: a hook on a box without curl or jq says which
// one is missing and fails before any call, where it would otherwise
// blame the zone or the API's answer.
func TestHooksNeedCurlAndJq(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("a shell script")
	}
	if _, err := os.Stat("/bin/sh"); err != nil {
		t.Skip("no /bin/sh")
	}
	// A PATH with nothing on it, and one with only a curl that notes that
	// it ran (with the shell's own redirection, as nothing else is there).
	none := t.TempDir()
	onlyCurl := t.TempDir()
	ran := filepath.Join(t.TempDir(), "ran")
	os.WriteFile(filepath.Join(onlyCurl, "curl"), []byte("#!/bin/sh\n: > \""+ran+"\"\n"), 0o755)
	for _, hook := range []string{"infomaniak", "cloudflare"} {
		script, _ := filepath.Abs("../../packaging/acme-dns-hooks/" + hook + ".sh")
		for _, c := range []struct{ path, missing string }{{none, "curl"}, {onlyCurl, "jq"}} {
			cmd := exec.Command(script, "set", "_acme-challenge.massimo.example.net", "value")
			cmd.Env = append(hookEnv(os.Environ()), "PATH="+c.path)
			out, err := cmd.CombinedOutput()
			want := hook + ": " + c.missing + " is not installed"
			if err == nil || !strings.Contains(string(out), want) {
				t.Errorf("%s without %s: %v %q, want %q", hook, c.missing, err, out, want)
			}
		}
	}
	if _, err := os.Stat(ran); err == nil {
		t.Error("a hook called curl without jq")
	}
}
