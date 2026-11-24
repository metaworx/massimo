// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

import (
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
	"go.metaworx.ch/massimo/internal/testkit"
)

func TestAuthLimiter(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	clock := testkit.NewClock(time.Date(2026, 11, 5, 12, 0, 0, 0, time.UTC))
	l := newAuthLimiter(3, time.Minute, 5*time.Minute, clock.Now)
	if l.fail(ctx, "a") || l.fail(ctx, "a") {
		t.Fatal("locked too early")
	}
	// Failures in an earlier window do not count.
	clock.Add(2 * time.Minute)
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
	clock.Add(5*time.Minute + time.Second)
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
	t.Parallel()
	clock := testkit.NewClock(time.Date(2026, 11, 5, 12, 0, 0, 0, time.UTC))
	s := newTicketStore(clock.Now)
	id, exp, err := s.issue("keq0000100", nil)
	if err != nil || len(id) != 48 || !exp.Equal(clock.Now().Add(TicketLifetime)) {
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
	clock.Add(TicketLifetime)
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
	ctx, cancel := context.WithTimeout(context.Background(), testkit.Scale(5*time.Second))
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
	t.Parallel()
	ts := startServer(t, withOptions(Options{Token: "secret"})).ts
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
	t.Parallel()
	ts := startServer(t, withOptions(Options{Token: "secret", AuthFailures: 3})).ts
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
	open := startServer(t, withOptions(Options{AuthFailures: 1})).ts
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
	t.Parallel()
	dir := t.TempDir()
	opts := storeOptions(t, dir, "secret")
	opts.AuthFailures = 3
	rg := startServer(t, withOptions(opts))
	ts, s := rg.ts, rg.s
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

// A token the server issued and that has ended is refused with 401, as
// a wrong one is, but guesses nothing and does not count toward the
// address's lockout (AP MobileApp v1.4, D64-A): more refusals than the
// limit within its window, of a revoked session, a session logged out,
// one past its end, a revoked key at POST /api/v1/login/key, a session
// whose key was revoked, and one on the stream's ?token=, lock nothing
// out; as many unknown tokens do, as before, a wrong ?token= on the
// stream among them, and so does a wrong ticket beside an ended token.
func TestEndedTokensAreNotCounted(t *testing.T) {
	t.Parallel()
	opts := storeOptions(t, t.TempDir(), "secret")
	rg := startServer(t, withOptions(opts))
	ts, st := rg.ts, rg.s.opts.Store
	ctx := t.Context()
	anna, err := st.AddUser(ctx, store.User{Name: "anna", Role: store.RoleOperator}, "", 0)
	if err != nil {
		t.Fatal(err)
	}
	session := func(end time.Time) (store.Token, string) {
		t.Helper()
		tok, secret, err := st.AddToken(ctx, anna.ID, store.TokenSession, "Firefox", end)
		if err != nil {
			t.Fatal(err)
		}
		return tok, secret
	}
	live := time.Now().Add(SessionLifetime)
	revokedTok, revoked := session(live)
	if err := st.RevokeToken(ctx, revokedTok.ID); err != nil {
		t.Fatal(err)
	}
	_, loggedOut := session(live)
	if code, text := callAPI(t, ts, "POST", "/api/v1/logout", loggedOut, ""); code != 200 {
		t.Fatalf("logout: %d %s", code, text)
	}
	_, expired := session(time.Now().Add(-time.Minute))
	keyTok, key, err := st.AddToken(ctx, anna.ID, store.TokenAPI, "SM-X, login by fingerprint", time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	code, text := callAPI(t, ts, "POST", "/api/v1/login/key", key, "")
	var made struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal([]byte(text), &made); err != nil || code != 200 || made.Token == "" {
		t.Fatalf("login by key: %d %s %v", code, text, err)
	}
	if err := st.RevokeToken(ctx, keyTok.ID); err != nil {
		t.Fatal(err)
	}
	cubes := "/api/v1/cubes"
	stream := cubes + "/KEQ0000100/events"
	for i := 0; i < DefaultAuthFailures+1; i++ {
		for _, c := range []struct{ what, method, path, token string }{
			{"a revoked session", "GET", cubes, revoked},
			{"a session logged out", "GET", cubes, loggedOut},
			{"a session past its end", "GET", cubes, expired},
			{"a revoked key", "POST", "/api/v1/login/key", key},
			{"the session of a revoked key", "GET", cubes, made.Token},
			{"a revoked session on the stream", "GET", stream + "?token=" + revoked, ""},
		} {
			if code, text := callAPI(t, ts, c.method, c.path, c.token, ""); code != 401 || !strings.Contains(text, errBadToken.Error()) {
				t.Fatalf("%s, refusal %d: %d %s", c.what, i+1, code, text)
			}
		}
	}
	keys, err := st.Lockout(DefaultAuthFailures, DefaultAuthWindow, DefaultAuthLockout).Keys(ctx)
	if err != nil || len(keys) != 0 {
		t.Fatalf("ended tokens counted: %v %v", keys, err)
	}
	if code, _ := callAPI(t, ts, "GET", cubes, "secret", ""); code != 200 {
		t.Fatalf("the right token after the ended ones: %d", code)
	}

	// A wrong ticket beside an ended token is a guess, and counts.
	if code, _ := callAPI(t, ts, "GET", stream+"?ticket=nope", revoked, ""); code != 401 {
		t.Fatalf("a wrong ticket with an ended token: %d", code)
	}
	if keys, _ := st.Lockout(DefaultAuthFailures, DefaultAuthWindow, DefaultAuthLockout).Keys(ctx); len(keys) != 1 {
		t.Fatalf("a wrong ticket with an ended token not counted: %v", keys)
	}
	// A wrong ?token= on the stream, without a Bearer header, is a guess.
	if code, _ := callAPI(t, ts, "GET", stream+"?token=unknown-token", "", ""); code != 401 {
		t.Fatalf("a wrong token on the stream: %d", code)
	}
	for i := 2; i < DefaultAuthFailures; i++ {
		if code, _ := callAPI(t, ts, "GET", cubes, "unknown-token", ""); code != 401 {
			t.Fatalf("unknown token %d: %d", i, code)
		}
	}
	if code, _ := callAPI(t, ts, "GET", cubes, "secret", ""); code != 429 {
		t.Fatalf("after as many guesses as the limit: %d", code)
	}
}

// A guess beside the token of a user who may not use the API now (a
// disabled user, a guest past its end, a disabled role) counts toward
// the lockout: a wrong Bearer header beside such a ?token= on the stream
// is refused 401 as any guessed Bearer is, the query not read, and a
// wrong ticket beside such a Bearer header counts though the answer is
// that token's 403. The token alone is no failed check.
func TestGuessesBesideAForbiddenTokenCount(t *testing.T) {
	t.Parallel()
	opts := storeOptions(t, t.TempDir(), "secret")
	rg := startServer(t, withOptions(opts))
	ts, st := rg.ts, rg.s.opts.Store
	ctx := t.Context()
	if _, err := st.AddUser(ctx, store.User{Name: "anna", Role: store.RoleOperator, Disabled: true}, "", 0); err != nil {
		t.Fatal(err)
	}
	anna, ok, err := st.User(ctx, "anna")
	if err != nil || !ok || !anna.Disabled {
		t.Fatalf("anna %+v %v %v", anna, ok, err)
	}
	_, key, err := st.AddToken(ctx, anna.ID, store.TokenAPI, "Loxone", time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	stream := "/api/v1/cubes/KEQ0000100/events"
	lockout := st.Lockout(DefaultAuthFailures, DefaultAuthWindow, DefaultAuthLockout)
	for i := 0; i < DefaultAuthFailures+1; i++ {
		if code, text := callAPI(t, ts, "GET", "/api/v1/cubes", key, ""); code != 403 || !strings.Contains(text, "disabled") {
			t.Fatalf("the disabled user's key alone, %d: %d %s", i+1, code, text)
		}
	}
	if keys, err := lockout.Keys(ctx); err != nil || len(keys) != 0 {
		t.Fatalf("the disabled user's key alone counted: %v %v", keys, err)
	}
	for i := 0; i < DefaultAuthFailures-1; i++ {
		if code, _ := callAPI(t, ts, "GET", stream+"?token="+key, "wrong-token", ""); code != 401 {
			t.Fatalf("a wrong Bearer beside the disabled user's ?token=, %d: %d", i+1, code)
		}
	}
	if keys, err := lockout.Keys(ctx); err != nil || len(keys) != 1 {
		t.Fatalf("wrong Bearer headers beside a forbidden ?token= not counted: %v %v", keys, err)
	}
	if code, _ := callAPI(t, ts, "GET", stream+"?ticket=nope", key, ""); code != 403 {
		t.Fatalf("a wrong ticket beside the disabled user's Bearer: %d", code)
	}
	if code, _ := callAPI(t, ts, "GET", "/api/v1/cubes", "secret", ""); code != 429 {
		t.Fatalf("after as many guesses as the limit beside a forbidden token: %d", code)
	}
}

// A guessed Bearer header beside a live ?token= on the stream is refused
// 401 and counted, the query not read: the live token neither answers
// for the guess, nor tells by its Deprecation header whether the guess
// was right, nor clears the address's count with its success.
func TestGuessedBearerBesideALiveTokenCounts(t *testing.T) {
	t.Parallel()
	ts := startServer(t, withOptions(storeOptions(t, t.TempDir(), "secret"))).ts
	stream := ts.URL + "/api/v1/cubes/KEQ0000100/events?token=secret"
	try := func(bearer string) *http.Response {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), testkit.Scale(5*time.Second))
		defer cancel()
		req, _ := http.NewRequestWithContext(ctx, "GET", stream, nil)
		req.Header.Set("Authorization", "Bearer "+bearer)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		return resp
	}
	for i := 0; i < DefaultAuthFailures; i++ {
		if r := try("wrong-token"); r.StatusCode != 401 || r.Header.Get("Deprecation") != "" {
			t.Fatalf("a wrong Bearer beside a live ?token=, %d: %d %q", i+1, r.StatusCode, r.Header.Get("Deprecation"))
		}
	}
	if r := try("secret"); r.StatusCode != 429 {
		t.Fatalf("after as many guessed Bearer headers as the limit beside a live ?token=: %d", r.StatusCode)
	}
}

func TestClientIP(t *testing.T) {
	t.Parallel()
	proxies, err := ParseTrustedProxies("127.0.0.1, 10.0.0.0/8 ::1")
	if err != nil || len(proxies) != 3 || proxies[1].String() != "10.0.0.0/8" || proxies[0].String() != "127.0.0.1/32" {
		t.Fatalf("parse: %v %v", proxies, err)
	}
	for _, bad := range []string{"host", "10.0.0.0/33", "192.0.2.1:80"} {
		if _, err := ParseTrustedProxies(bad); err == nil || err.Error() != `-trusted-proxies or MASSIMO_TRUSTED_PROXIES: "`+bad+`" is neither an address nor a CIDR` {
			t.Errorf("%q: %v", bad, err)
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
	t.Parallel()
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
	log, logs := testkit.LogWatch(nil)
	rl := newLimiter(Options{Store: ro}, log)
	for i := 0; i < 3; i++ {
		rl.succeed(ctx, store.LockoutAddr("192.0.2.11"))
	}
	logs.HasNot(t, 0, "a store write for a success without failures", testkit.Contains("not forgotten"))
}
