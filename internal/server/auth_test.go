// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"go.metaworx.ch/massimo/internal/store"
)

type fakeClock struct{ t time.Time }

func (c *fakeClock) now() time.Time { return c.t }

func TestAuthLimiter(t *testing.T) {
	ctx := context.Background()
	clock := &fakeClock{t: time.Date(2026, 11, 5, 12, 0, 0, 0, time.UTC)}
	l := newAuthLimiter(3, time.Minute, 5*time.Minute, clock.now)
	if l.fail(ctx, "a") || l.fail(ctx, "a") {
		t.Fatal("locked too early")
	}
	// Failures in an earlier window do not count.
	clock.t = clock.t.Add(2 * time.Minute)
	if l.fail(ctx, "a") || l.fail(ctx, "a") {
		t.Fatal("old failures counted")
	}
	if !l.fail(ctx, "a") {
		t.Fatal("third failure in the window did not lock")
	}
	if d := l.lockedFor(ctx, "a"); d != 5*time.Minute {
		t.Fatalf("locked for %v", d)
	}
	if l.lockedFor(ctx, "b") != 0 {
		t.Fatal("other address locked")
	}
	l.succeed(ctx, "a") // a success during the lockout does not lift it
	clock.t = clock.t.Add(5*time.Minute + time.Second)
	if l.lockedFor(ctx, "a") != 0 {
		t.Fatal("still locked after the lockout")
	}
	l.fail(ctx, "b")
	l.succeed(ctx, "b")
	if _, ok := l.addrs["b"]; ok {
		t.Fatal("success did not forget the failures")
	}
}

func TestTicketStore(t *testing.T) {
	clock := &fakeClock{t: time.Date(2026, 11, 5, 12, 0, 0, 0, time.UTC)}
	s := newTicketStore(clock.now)
	id, exp, err := s.issue("keq0000100", nil)
	if err != nil || len(id) != 48 || !exp.Equal(clock.t.Add(TicketLifetime)) {
		t.Fatalf("issue %q %v %v", id, exp, err)
	}
	if _, ok := s.redeem(id, "keq0000200"); ok {
		t.Fatal("ticket valid for another cube")
	}
	id, _, _ = s.issue("keq0000100", nil)
	if _, ok := s.redeem(id, "keq0000100"); !ok {
		t.Fatal("fresh ticket refused")
	}
	if _, ok := s.redeem(id, "keq0000100"); ok {
		t.Fatal("ticket used twice")
	}
	id, _, _ = s.issue("keq0000100", nil)
	clock.t = clock.t.Add(TicketLifetime)
	if _, ok := s.redeem(id, "keq0000100"); ok {
		t.Fatal("expired ticket accepted")
	}
	if _, ok := s.redeem("", "keq0000100"); ok {
		t.Fatal("empty ticket accepted")
	}
}

// send sends a request and returns the response with its body closed; for
// the event stream, the context is cancelled once the headers are in.
func send(t *testing.T, method, url, token string) *http.Response {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, method, url, nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	return resp
}

func ticketFor(t *testing.T, ts *httptest.Server, token, sn string) string {
	t.Helper()
	req, _ := http.NewRequest("POST", ts.URL+"/api/v1/cubes/"+sn+"/events/ticket", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var out struct {
		Ticket    string    `json:"ticket"`
		ExpiresAt time.Time `json:"expires_at"`
	}
	if resp.StatusCode != 200 || json.NewDecoder(resp.Body).Decode(&out) != nil || out.Ticket == "" || out.ExpiresAt.IsZero() {
		t.Fatalf("ticket: %d %+v", resp.StatusCode, out)
	}
	return out.Ticket
}

func TestEventStreamTicket(t *testing.T) {
	_, ts, _ := start(t, "secret")
	events := ts.URL + "/api/v1/cubes/KEQ0000100/events"
	if r := send(t, "POST", ts.URL+"/api/v1/cubes/KEQ0000100/events/ticket", ""); r.StatusCode != 401 {
		t.Fatalf("ticket without the token: %d", r.StatusCode)
	}
	if r := send(t, "POST", ts.URL+"/api/v1/cubes/NOPE/events/ticket", "secret"); r.StatusCode != 404 {
		t.Fatalf("ticket for an unknown cube: %d", r.StatusCode)
	}
	tk := ticketFor(t, ts, "secret", "keq0000100")
	if r := send(t, "GET", events+"?ticket="+tk, ""); r.StatusCode != 200 || r.Header.Get("Deprecation") != "" {
		t.Fatalf("stream with a ticket: %d %v", r.StatusCode, r.Header)
	}
	if r := send(t, "GET", events+"?ticket="+tk, ""); r.StatusCode != 401 {
		t.Fatalf("ticket reused: %d", r.StatusCode)
	}
	// A ticket opens only the event stream.
	tk = ticketFor(t, ts, "secret", "KEQ0000100")
	if r := send(t, "GET", ts.URL+"/api/v1/cubes/KEQ0000100/devices?ticket="+tk, ""); r.StatusCode != 401 {
		t.Fatalf("ticket on another route: %d", r.StatusCode)
	}
	// The old way still works, marked deprecated.
	if r := send(t, "GET", events+"?token=secret", ""); r.StatusCode != 200 || r.Header.Get("Deprecation") != tokenQueryDeprecated {
		t.Fatalf("stream with ?token=: %d %v", r.StatusCode, r.Header)
	}
}

func TestFailedTokensLockTheAddressOut(t *testing.T) {
	_, ts, _ := startWith(t, Options{Token: "secret", AuthFailures: 3})
	cubes := ts.URL + "/api/v1/cubes"
	// A success in between forgets the failures.
	send(t, "GET", cubes, "wrong")
	send(t, "GET", cubes, "wrong")
	if r := send(t, "GET", cubes, "secret"); r.StatusCode != 200 {
		t.Fatalf("right token: %d", r.StatusCode)
	}
	for i := 0; i < 3; i++ {
		if r := send(t, "GET", cubes, "wrong"); r.StatusCode != 401 {
			t.Fatalf("failure %d: %d", i+1, r.StatusCode)
		}
	}
	r := send(t, "GET", cubes, "secret")
	if r.StatusCode != 429 || r.Header.Get("Retry-After") == "" {
		t.Fatalf("after the limit: %d %v", r.StatusCode, r.Header)
	}
	// Wrong stream tickets count as failures too, and the lockout covers
	// the stream.
	if r := send(t, "GET", cubes+"/KEQ0000100/events?ticket=nope", ""); r.StatusCode != 429 {
		t.Fatalf("stream during the lockout: %d", r.StatusCode)
	}
	// Without a token configured nothing is limited.
	_, open, _ := startWith(t, Options{AuthFailures: 1})
	for i := 0; i < 3; i++ {
		if r := send(t, "GET", open.URL+"/api/v1/cubes", "wrong"); r.StatusCode != 200 {
			t.Fatalf("open server: %d", r.StatusCode)
		}
	}
	if r := send(t, "GET", ts.URL+"/healthz", ""); r.StatusCode != 200 {
		t.Fatalf("healthz during the lockout: %d", r.StatusCode)
	}
}

// A request that offers no token, ticket or ?token= guesses nothing: it
// is refused, but does not count toward the lockout. An empty Bearer
// header is offered, and counts.
func TestRequestsWithoutTokenAreNotCounted(t *testing.T) {
	dir := t.TempDir()
	opts := storeOptions(t, dir, "secret")
	opts.AuthFailures = 3
	_, ts, s := startWith(t, opts)
	cubes := ts.URL + "/api/v1/cubes"
	stream := cubes + "/KEQ0000100/events"
	for i := 0; i < 5; i++ {
		for _, url := range []string{cubes, stream, stream + "?ticket=&token="} {
			if r := send(t, "GET", url, ""); r.StatusCode != 401 {
				t.Fatalf("%s without a token: %d", url, r.StatusCode)
			}
		}
	}
	keys, err := s.opts.Store.Lockout(3, time.Minute, time.Minute).Keys(context.Background())
	if err != nil || len(keys) != 0 {
		t.Fatalf("requests without a token counted: %v %v", keys, err)
	}
	if r := send(t, "GET", cubes, "secret"); r.StatusCode != 200 {
		t.Fatalf("right token after requests without one: %d", r.StatusCode)
	}
	emptyBearer := func() int {
		req, _ := http.NewRequest("GET", cubes, nil)
		req.Header.Set("Authorization", "Bearer ")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		return resp.StatusCode
	}
	for i := 0; i < 3; i++ {
		if code := emptyBearer(); code != 401 {
			t.Fatalf("empty Bearer %d: %d", i+1, code)
		}
	}
	if r := send(t, "GET", cubes, "secret"); r.StatusCode != 429 {
		t.Fatalf("after three empty Bearer headers: %d", r.StatusCode)
	}
	// Locked out, a request without a token is 429 too.
	if r := send(t, "GET", cubes, ""); r.StatusCode != 429 {
		t.Fatalf("without a token during the lockout: %d", r.StatusCode)
	}
}

func TestClientIP(t *testing.T) {
	proxies, err := ParseTrustedProxies("127.0.0.1, 10.0.0.0/8 ::1")
	if err != nil || len(proxies) != 3 || proxies[1].String() != "10.0.0.0/8" || proxies[0].String() != "127.0.0.1/32" {
		t.Fatalf("parse: %v %v", proxies, err)
	}
	for _, bad := range []string{"host", "10.0.0.0/33", "192.0.2.1:80"} {
		if _, err := ParseTrustedProxies(bad); err == nil {
			t.Errorf("%q accepted", bad)
		}
	}
	plain := &Server{opts: Options{}}
	trusting := &Server{opts: Options{TrustedProxies: proxies}}
	req := func(remote, xff, real string) *http.Request {
		r := httptest.NewRequest("GET", "/api/v1/cubes", nil)
		r.RemoteAddr = remote
		if xff != "" {
			r.Header.Set("X-Forwarded-For", xff)
		}
		if real != "" {
			r.Header.Set("X-Real-IP", real)
		}
		return r
	}
	for _, c := range []struct {
		s                 *Server
		remote, xff, real string
		want              string
	}{
		{plain, "192.0.2.10:1234", "", "", "192.0.2.10"},
		{plain, "127.0.0.1:1234", "192.0.2.10", "192.0.2.11", "127.0.0.1"}, // headers ignored without trusted proxies
		{trusting, "192.0.2.10:1234", "192.0.2.99", "", "192.0.2.10"},      // not from a proxy
		{trusting, "127.0.0.1:1234", "192.0.2.10", "", "192.0.2.10"},
		{trusting, "127.0.0.1:1234", "192.0.2.10, 10.1.2.3", "", "192.0.2.10"},     // a chain through two trusted proxies
		{trusting, "127.0.0.1:1234", "198.51.100.1, 192.0.2.10", "", "192.0.2.10"}, // the rightmost client
		{trusting, "127.0.0.1:1234", "10.1.2.3", "192.0.2.12", "10.1.2.3"},         // every hop trusted: the leftmost, never X-Real-IP
		{trusting, "127.0.0.1:1234", "", "192.0.2.12", "192.0.2.12"},
		{trusting, "127.0.0.1:1234", "", "", "127.0.0.1"},
		{trusting, "127.0.0.1:1234", "garbage", "", "127.0.0.1"},
		// An entry that does not parse is the peer, whatever X-Real-IP
		// the client set; X-Real-IP counts only without X-Forwarded-For.
		{trusting, "127.0.0.1:1234", "unknown", "192.0.2.13", "127.0.0.1"},
		{trusting, "127.0.0.1:1234", "192.0.2.10:5555", "192.0.2.13", "127.0.0.1"},
		{trusting, "127.0.0.1:1234", "192.0.2.10, unknown", "192.0.2.13", "127.0.0.1"},
		{trusting, "127.0.0.1:1234", "198.51.100.7", "192.0.2.13", "198.51.100.7"},
		{trusting, "[::1]:1234", "2001:db8::10", "", "2001:db8::10"},
		{trusting, "[::ffff:127.0.0.1]:1234", "192.0.2.10", "", "192.0.2.10"},
	} {
		if got := c.s.clientIP(req(c.remote, c.xff, c.real)); got != c.want {
			t.Errorf("%s xff=%q real=%q: %q, want %q", c.remote, c.xff, c.real, got, c.want)
		}
	}
	// A proxy that adds a header line of its own after the client's: the
	// lines are one list, the proxy's entry the rightmost.
	r := req("127.0.0.1:1234", "192.0.2.66", "")
	r.Header.Add("X-Forwarded-For", "192.0.2.10")
	if got := trusting.clientIP(r); got != "192.0.2.10" {
		t.Errorf("two X-Forwarded-For lines: %q, want the proxy's 192.0.2.10", got)
	}
}

// A success writes the store only for an address that has failures: an
// authenticated request from an address without any writes nothing (a
// read-only store makes a write show as an error in the log). Failures
// still go with a success, also those from before a restart.
func TestStoreLimiterWritesOnlyAfterFailures(t *testing.T) {
	dir := t.TempDir()
	quiet := slog.New(slog.NewTextHandler(io.Discard, nil))
	st := testStore(t, dir)
	ctx := context.Background()
	key := store.LockoutAddr("192.0.2.10")
	l := newLimiter(Options{Store: st, AuthFailures: 3}, quiet)
	l.fail(ctx, key)
	l.fail(ctx, key)
	l.succeed(ctx, key)
	if l.fail(ctx, key) || l.fail(ctx, key) {
		t.Fatal("a success did not forget the failures")
	}
	// A restart: the failures in the store go with the next success.
	again := newLimiter(Options{Store: st, AuthFailures: 3}, quiet)
	again.succeed(ctx, key)
	if again.fail(ctx, key) {
		t.Fatal("a success after a restart did not forget the failures")
	}
	ro, err := store.OpenReadOnly(dir, store.Options{Version: "test", Logger: quiet})
	if err != nil {
		t.Fatal(err)
	}
	defer ro.Close()
	var buf bytes.Buffer
	rl := newLimiter(Options{Store: ro}, slog.New(slog.NewTextHandler(&buf, nil)))
	for i := 0; i < 3; i++ {
		rl.succeed(ctx, store.LockoutAddr("192.0.2.11"))
	}
	if strings.Contains(buf.String(), "not forgotten") {
		t.Fatalf("a success without failures wrote the store:\n%s", buf.String())
	}
}
