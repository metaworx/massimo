// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package certs

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"log/slog"
	"math/big"
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
	"time"

	"golang.org/x/crypto/acme"
)

// fakeCA is an ACME directory for the tests: RFC 8555's shape, no
// signature check, the dns-01 challenge validated against what the hook
// set (records), the certificate signed by its own CA.
type fakeCA struct {
	t       *testing.T
	srv     *httptest.Server
	caKey   *ecdsa.PrivateKey
	caCert  *x509.Certificate
	records func() map[string][]string // the "DNS": name → TXT values

	mu         sync.Mutex
	seq        int
	thumbprint string
	orders     map[string]*fakeOrder
	authz      map[string]*fakeAuthz
	failNext   bool
}

type fakeOrder struct {
	ids     []acme.AuthzID
	authz   []string
	status  string
	certPEM []byte
}

type fakeAuthz struct {
	name     string
	wildcard bool
	status   string
	token    string
}

func newFakeCA(t *testing.T) *fakeCA {
	t.Helper()
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	tmpl := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "Fake ACME CA"}, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(24 * time.Hour),
		IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign}
	der, _ := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	caCert, _ := x509.ParseCertificate(der)
	ca := &fakeCA{t: t, caKey: key, caCert: caCert, orders: map[string]*fakeOrder{}, authz: map[string]*fakeAuthz{}}
	ca.srv = httptest.NewServer(http.HandlerFunc(ca.serve))
	t.Cleanup(ca.srv.Close)
	return ca
}

// jws reads a signed request's protected header and payload.
func (ca *fakeCA) jws(r *http.Request) (header, payload map[string]any) {
	var env struct{ Protected, Payload string }
	json.NewDecoder(r.Body).Decode(&env)
	h, _ := base64.RawURLEncoding.DecodeString(env.Protected)
	json.Unmarshal(h, &header)
	if p, _ := base64.RawURLEncoding.DecodeString(env.Payload); len(p) > 0 {
		json.Unmarshal(p, &payload)
	}
	return header, payload
}

func (ca *fakeCA) url(p string) string { return ca.srv.URL + p }

// reply answers with a fresh nonce; called with the lock held.
func (ca *fakeCA) reply(w http.ResponseWriter, status int, v any, headers ...string) {
	ca.seq++
	w.Header().Set("Replay-Nonce", "nonce-"+strconv.Itoa(ca.seq))
	for i := 0; i+1 < len(headers); i += 2 {
		w.Header().Set(headers[i], headers[i+1])
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func (ca *fakeCA) orderJSON(id string, o *fakeOrder) map[string]any {
	var authz []string
	for _, a := range o.authz {
		authz = append(authz, ca.url("/authz/"+a))
	}
	m := map[string]any{"status": o.status, "identifiers": o.ids, "authorizations": authz, "finalize": ca.url("/order/" + id + "/finalize")}
	if o.status == "valid" {
		m["certificate"] = ca.url("/cert/" + id)
	}
	return m
}

// serve handles one request under the lock: the requests come one after
// the other anyway, and the detector cannot see the order through the
// socket.
func (ca *fakeCA) serve(w http.ResponseWriter, r *http.Request) {
	ca.mu.Lock()
	defer ca.mu.Unlock()
	p := r.URL.Path
	switch {
	case p == "/directory":
		ca.reply(w, 200, map[string]any{"newNonce": ca.url("/nonce"), "newAccount": ca.url("/new-acct"), "newOrder": ca.url("/new-order"), "meta": map[string]any{}})
	case p == "/nonce":
		ca.reply(w, 200, nil)
	case p == "/new-acct":
		header, _ := ca.jws(r)
		if jwk, ok := header["jwk"].(map[string]any); ok {
			x, _ := base64.RawURLEncoding.DecodeString(jwk["x"].(string))
			y, _ := base64.RawURLEncoding.DecodeString(jwk["y"].(string))
			pub := &ecdsa.PublicKey{Curve: elliptic.P256(), X: new(big.Int).SetBytes(x), Y: new(big.Int).SetBytes(y)}
			tp, err := acme.JWKThumbprint(pub)
			if err != nil {
				ca.t.Error(err)
			}
			ca.thumbprint = tp
		}
		ca.reply(w, 201, map[string]any{"status": "valid"}, "Location", ca.url("/acct/1"))
	case p == "/new-order":
		_, payload := ca.jws(r)
		var o fakeOrder
		b, _ := json.Marshal(payload["identifiers"])
		json.Unmarshal(b, &o.ids)
		ca.seq++
		id := strconv.Itoa(ca.seq)
		for i, ident := range o.ids {
			aid := id + "-" + strconv.Itoa(i)
			name, wildcard := strings.CutPrefix(ident.Value, "*.")
			ca.authz[aid] = &fakeAuthz{name: name, wildcard: wildcard, status: "pending", token: "token-" + aid}
			o.authz = append(o.authz, aid)
		}
		o.status = "pending"
		ca.orders[id] = &o
		ca.reply(w, 201, ca.orderJSON(id, &o), "Location", ca.url("/order/"+id))
	case strings.HasPrefix(p, "/authz/"):
		a := ca.authz[strings.TrimPrefix(p, "/authz/")]
		ca.reply(w, 200, map[string]any{"status": a.status, "identifier": map[string]string{"type": "dns", "value": a.name}, "wildcard": a.wildcard,
			"challenges": []map[string]any{{"type": "dns-01", "url": ca.url("/chal/" + strings.TrimPrefix(p, "/authz/")), "token": a.token, "status": a.status}}})
	case strings.HasPrefix(p, "/chal/"):
		// The validation: the record the hook set must be the key
		// authorization's digest.
		aid := strings.TrimPrefix(p, "/chal/")
		a := ca.authz[aid]
		sum := sha256.Sum256([]byte(a.token + "." + ca.thumbprint))
		want := base64.RawURLEncoding.EncodeToString(sum[:])
		fail := ca.failNext
		ca.failNext = false
		a.status = "invalid"
		for _, v := range ca.records()["_acme-challenge."+a.name] {
			if v == want && !fail {
				a.status = "valid"
			}
		}
		ca.reply(w, 200, map[string]any{"type": "dns-01", "url": ca.url(p), "token": a.token, "status": a.status})
	case strings.HasSuffix(p, "/finalize"):
		id := strings.TrimSuffix(strings.TrimPrefix(p, "/order/"), "/finalize")
		_, payload := ca.jws(r)
		csrDER, _ := base64.RawURLEncoding.DecodeString(payload["csr"].(string))
		csr, err := x509.ParseCertificateRequest(csrDER)
		if err != nil {
			ca.t.Error(err)
		}
		o := ca.orders[id]
		for _, aid := range o.authz {
			if ca.authz[aid].status != "valid" {
				ca.reply(w, 403, map[string]any{"type": "urn:ietf:params:acme:error:orderNotReady", "detail": "not ready"})
				return
			}
		}
		tmpl := &x509.Certificate{SerialNumber: big.NewInt(int64(ca.seq)), Subject: csr.Subject, DNSNames: csr.DNSNames, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(90 * 24 * time.Hour),
			KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}
		der, err := x509.CreateCertificate(rand.Reader, tmpl, ca.caCert, csr.PublicKey, ca.caKey)
		if err != nil {
			ca.t.Error(err)
		}
		o.certPEM = append(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: ca.caCert.Raw})...)
		o.status = "valid"
		ca.reply(w, 200, ca.orderJSON(id, o), "Location", ca.url("/order/"+id))
	case strings.HasPrefix(p, "/order/"):
		id := strings.TrimPrefix(p, "/order/")
		o := ca.orders[id]
		if o.status == "pending" {
			ready := true
			for _, aid := range o.authz {
				ready = ready && ca.authz[aid].status == "valid"
			}
			if ready {
				o.status = "ready"
			}
		}
		ca.reply(w, 200, ca.orderJSON(id, o), "Location", ca.url(p))
	case strings.HasPrefix(p, "/cert/"):
		o := ca.orders[strings.TrimPrefix(p, "/cert/")]
		ca.seq++
		w.Header().Set("Replay-Nonce", "nonce-"+strconv.Itoa(ca.seq))
		w.Header().Set("Content-Type", "application/pem-certificate-chain")
		w.Write(o.certPEM)
	default:
		ca.reply(w, 404, map[string]any{"type": "urn:ietf:params:acme:error:malformed", "detail": "no " + p})
	}
}

func (ca *fakeCA) orderCount() int {
	ca.mu.Lock()
	defer ca.mu.Unlock()
	return len(ca.orders)
}

// hookCalls reads the hook's calls file: the records set and not
// cleared, and every call in order.
func hookCalls(path string) (records map[string][]string, calls []string) {
	records = map[string][]string{}
	b, _ := os.ReadFile(path)
	for _, line := range strings.Split(strings.TrimSpace(string(b)), "\n") {
		if line == "" {
			continue
		}
		calls = append(calls, line)
		f := strings.Fields(line)
		if len(f) < 3 {
			continue
		}
		switch f[0] {
		case "set":
			records[f[1]] = append(records[f[1]], f[2])
		case "clear":
			var kept []string
			for _, v := range records[f[1]] {
				if v != f[2] {
					kept = append(kept, v)
				}
			}
			records[f[1]] = kept
		}
	}
	return records, calls
}

func TestDNS01(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the hook is a shell script")
	}
	ca := newFakeCA(t)
	hookDir := t.TempDir()
	hook := filepath.Join(hookDir, "hook.sh")
	calls := filepath.Join(hookDir, "calls.txt")
	// The hook records its calls, and what it sees of the server's
	// settings (nothing: the fourth field stays empty).
	os.WriteFile(hook, []byte("#!/bin/sh\necho \"$1 $2 $3 $MASSIMO_API_TOKEN\" >> \"$(dirname \"$0\")/calls.txt\"\n"), 0o755)
	t.Setenv("MASSIMO_API_TOKEN", "secret")
	ca.records = func() map[string][]string { r, _ := hookCalls(calls); return r }
	data := t.TempDir()
	var buf bytes.Buffer
	log := slog.New(slog.NewTextHandler(&buf, nil))
	names := []string{"*.example.net", "example.net"}

	for _, bad := range []struct {
		names []string
		hook  string
	}{{nil, hook}, {[]string{"192.0.2.10"}, hook}, {[]string{"pi"}, hook}, {names, ""}, {names, filepath.Join(hookDir, "none.sh")}} {
		if _, err := OpenDNS01(data, bad.names, "", ca.url("/directory"), bad.hook, log); err == nil {
			t.Errorf("%+v accepted", bad)
		}
	}
	noExec := filepath.Join(hookDir, "noexec.sh")
	os.WriteFile(noExec, []byte("#!/bin/sh\n"), 0o644)
	if _, err := OpenDNS01(data, names, "", ca.url("/directory"), noExec, log); err == nil {
		t.Error("a hook that is not executable accepted")
	}

	d, err := OpenDNS01(data, names, "owner@example.net", ca.url("/directory"), hook, log)
	if err != nil {
		t.Fatal(err)
	}
	d.lookupTXT = func(ctx context.Context, name string) ([]string, error) { return ca.records()[name], nil }
	d.dnsEvery = 10 * time.Millisecond
	if _, err := d.GetCertificate(&tls.ClientHelloInfo{ServerName: "a.example.net"}); err == nil {
		t.Fatal("served before any certificate")
	}
	if st := d.Status(); st.Source != "acme" || !strings.Contains(st.Warning, "no certificate yet") {
		t.Fatalf("status before: %+v", st)
	}

	// The first order, at the start.
	now := time.Now()
	d.Check(now)
	st := d.Status()
	if st.Fingerprint == "" || st.Warning != "" || strings.Join(st.Names, ",") != "*.example.net,example.net" || st.NotAfter.Before(now.Add(89*24*time.Hour)) {
		t.Fatalf("status after the order: %+v\n%s", st, buf.String())
	}
	cert, err := d.GetCertificate(&tls.ClientHelloInfo{ServerName: "a.example.net"})
	if err != nil {
		t.Fatal(err)
	}
	leaf, _ := x509.ParseCertificate(cert.Certificate[0])
	if strings.Join(leaf.DNSNames, ",") != "*.example.net,example.net" || len(cert.Certificate) != 2 {
		t.Fatalf("leaf %v, chain of %d", leaf.DNSNames, len(cert.Certificate))
	}
	if !strings.Contains(buf.String(), "certificate issued by DNS-01") {
		t.Fatalf("not logged:\n%s", buf.String())
	}
	_, callLines := hookCalls(calls)
	if len(callLines) != 4 || !strings.HasPrefix(callLines[0], "set _acme-challenge.example.net ") || !strings.HasPrefix(callLines[1], "clear _acme-challenge.example.net ") ||
		!strings.HasPrefix(callLines[2], "set _acme-challenge.example.net ") || strings.Fields(callLines[0])[2] == strings.Fields(callLines[2])[2] {
		t.Fatalf("hook calls %q", callLines)
	}
	for _, c := range callLines {
		if len(strings.Fields(c)) != 3 {
			t.Fatalf("the hook saw the server's token: %q", c)
		}
	}
	for _, f := range []string{"_wildcard.example.net.pem", "_wildcard.example.net.key", AccountKeyFile} {
		if fi, err := os.Stat(filepath.Join(data, "tls", "acme", f)); err != nil || fi.Mode().Perm() != 0o600 {
			t.Errorf("%s: %v", f, err)
		}
	}
	if ca.orderCount() != 1 {
		t.Fatalf("%d orders", ca.orderCount())
	}

	// A second start serves the certificate without an order.
	d2, err := OpenDNS01(data, names, "", ca.url("/directory"), hook, log)
	if err != nil {
		t.Fatal(err)
	}
	d2.lookupTXT, d2.dnsEvery = d.lookupTXT, d.dnsEvery
	d2.Check(now)
	if d2.Status().Fingerprint != st.Fingerprint || ca.orderCount() != 1 {
		t.Fatal("ordered again at the second start")
	}

	// Within 30 days of the end: ordered anew and served; the files
	// source sees the rewritten pair.
	d2.Check(st.NotAfter.Add(-20 * 24 * time.Hour))
	renewed := d2.Status()
	if renewed.Fingerprint == st.Fingerprint || ca.orderCount() != 2 {
		t.Fatalf("not renewed: %+v, %d orders", renewed, ca.orderCount())
	}
	if cert, _ := d2.GetCertificate(&tls.ClientHelloInfo{ServerName: "example.net"}); Fingerprint(cert.Certificate[0]) != renewed.Fingerprint {
		t.Fatal("the renewed certificate is not served")
	}

	// A validation the CA refuses: logged, the certificate kept, the
	// record cleared, the warning set.
	buf.Reset()
	ca.mu.Lock()
	ca.failNext = true
	ca.mu.Unlock()
	d2.Check(renewed.NotAfter.Add(-20 * 24 * time.Hour))
	if d2.Status().Fingerprint != renewed.Fingerprint || !strings.Contains(buf.String(), "order failed") || !strings.Contains(d2.Status().Warning, "failed") {
		t.Fatalf("after a refused validation: %+v\n%s", d2.Status(), buf.String())
	}
	if records, _ := hookCalls(calls); len(records["_acme-challenge.example.net"]) != 0 {
		t.Fatalf("records left behind: %v", records)
	}

	// The hook's environment.
	if env := hookEnv([]string{"PATH=/bin", "MASSIMO_CUBE_KEY=k", "massimo_x=1", "HOME=/h"}); strings.Join(env, " ") != "PATH=/bin HOME=/h" {
		t.Fatalf("hook env %v", env)
	}
	fmt.Fprint(os.Stderr, "")
}

// A hook whose child keeps the output open does not hold the order past
// the hook's time: runHook returns once the time is up, and the child
// goes with the hook.
func TestHookTimeoutEndsTheChildren(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the hook is a shell script")
	}
	dir := t.TempDir()
	hook := filepath.Join(dir, "hook.sh")
	pidFile := filepath.Join(dir, "child.pid")
	os.WriteFile(hook, []byte("#!/bin/sh\nsleep 20 &\necho $! > \""+pidFile+"\"\nwait\n"), 0o755)
	d := &DNS01{hook: hook, log: slog.New(slog.NewTextHandler(io.Discard, nil))}
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	start := time.Now()
	err := d.runHook(ctx, "set", "_acme-challenge.example.net", "value")
	if took := time.Since(start); err == nil || took > 3*time.Second {
		t.Fatalf("runHook returned %v after %s, want an error within the time", err, took)
	}
	b, _ := os.ReadFile(pidFile)
	pid, _ := strconv.Atoi(strings.TrimSpace(string(b)))
	if pid == 0 {
		t.Fatal("the hook wrote no pid")
	}
	deadline := time.Now().Add(2 * time.Second)
	for exec.Command("kill", "-0", strconv.Itoa(pid)).Run() == nil {
		if time.Now().After(deadline) {
			exec.Command("kill", strconv.Itoa(pid)).Run()
			t.Fatal("the hook's child outlived the hook's time")
		}
		time.Sleep(20 * time.Millisecond)
	}
}
