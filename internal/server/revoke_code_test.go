// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

// POST /api/v1/tokens/revoke (AP MobileApp v1.6, Block 21, D112-A): a
// token ended by its revoke code, which a client that forgot the token
// posts without one.

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"go.metaworx.ch/massimo/internal/store"
	"go.metaworx.ch/massimo/internal/testkit"
)

// postCode posts a body to POST /api/v1/tokens/revoke, from the address
// a trusted proxy names where from is not "", and returns the status,
// the body and the headers.
func postCode(t *testing.T, ts *httptest.Server, body, from string) (int, string, http.Header) {
	t.Helper()
	req, _ := http.NewRequest("POST", ts.URL+"/api/v1/tokens/revoke", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if from != "" {
		req.Header.Set("X-Forwarded-For", from)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(b), resp.Header
}

// codeBody is the body that posts the code of a secret.
func codeBody(secret string) string { return `{"code":"` + store.RevokeCode(secret) + `"}` }

// newKey makes an API key of a user through the API, as the owner, and
// returns its id and secret.
func newKey(t *testing.T, ts *httptest.Server, owner, user string) (int64, string) {
	t.Helper()
	code, text := callAPI(t, ts, "POST", "/api/v1/users/"+user+"/tokens", owner, `{"label":"phone, login by fingerprint"}`)
	var key struct {
		ID    int64  `json:"id"`
		Token string `json:"token"`
	}
	if json.Unmarshal([]byte(text), &key); code != 201 || key.Token == "" {
		t.Fatalf("a key of %s: %d %s", user, code, text)
	}
	return key.ID, key.Token
}

// A session is ended by its code and refused after; a key by its code
// ends its session too; a code that matches nothing, and one taken
// already, are 204 as well, the tokens in the store unchanged then.
func TestRevokeByCode(t *testing.T) {
	t.Parallel()
	rg := startServer(t, withOptions(storeOptions(t, t.TempDir(), "secret")))
	ts, st := rg.ts, rg.s.opts.Store
	ctx := t.Context()
	anna, err := st.AddUser(ctx, store.User{Name: "anna", Role: store.RoleOperator}, "", 0)
	if err != nil {
		t.Fatal(err)
	}
	sess, sessSecret, err := st.AddToken(ctx, anna.ID, store.TokenSession, "Firefox", time.Now().Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if code, _ := callAPI(t, ts, "GET", "/api/v1/me", sessSecret, ""); code != 200 {
		t.Fatalf("the session before its code: %d", code)
	}
	if code, text, _ := postCode(t, ts, codeBody(sessSecret), ""); code != 204 || text != "" {
		t.Fatalf("a session's code: %d %q", code, text)
	}
	if code, _ := callAPI(t, ts, "GET", "/api/v1/me", sessSecret, ""); code != 401 {
		t.Fatalf("the session after its code: %d", code)
	}

	keyID, key := newKey(t, ts, "secret", "anna")
	code, text := callAPI(t, ts, "POST", "/api/v1/login/key", key, "")
	var keySess struct {
		Token string `json:"token"`
	}
	if json.Unmarshal([]byte(text), &keySess); code != 200 || keySess.Token == "" {
		t.Fatalf("the key's session: %d %s", code, text)
	}
	if code, _, _ := postCode(t, ts, codeBody(key), ""); code != 204 {
		t.Fatalf("a key's code: %d", code)
	}
	for what, tok := range map[string]string{"the key": key, "its session": keySess.Token} {
		if code, _ := callAPI(t, ts, "GET", "/api/v1/me", tok, ""); code != 401 {
			t.Errorf("%s after the key's code: %d", what, code)
		}
	}

	_, other := newKey(t, ts, "secret", "anna")
	before, _, _ := st.Token(ctx, keyID)
	otherBefore, _, _ := st.TokenByHash(ctx, store.HashToken(other))
	for what, body := range map[string]string{"a code that matches nothing": codeBody("nobody's"), "a code taken already": codeBody(key), "the session's again": codeBody(sessSecret)} {
		if code, text, _ := postCode(t, ts, body, ""); code != 204 || text != "" {
			t.Errorf("%s: %d %q", what, code, text)
		}
	}
	if after, _, _ := st.Token(ctx, keyID); after.Version != before.Version || !after.DeletedAt.Equal(before.DeletedAt) {
		t.Errorf("the key taken already changed: %+v, before %+v", after, before)
	}
	if after, ok, _ := st.TokenByHash(ctx, store.HashToken(other)); !ok || after.Version != otherBefore.Version {
		t.Errorf("another key changed: %+v %v", after, ok)
	}
	if got, _, _ := st.Token(ctx, sess.ID); got.DeletedAt.IsZero() {
		t.Error("the session lives again")
	}
}

// The settings file's key is left live by its code, 204, even where it
// holds one: a key made by POST …/tokens and named in the file later.
func TestRevokeByCodeLeavesTheSettingsKey(t *testing.T) {
	t.Parallel()
	rg := startServer(t, withOptions(storeOptions(t, t.TempDir(), "secret")))
	ts, s := rg.ts, rg.s
	_, key := newKey(t, ts, "secret", store.BootstrapUser)
	if err := s.SetToken(t.Context(), key); err != nil {
		t.Fatal(err)
	}
	if tok, ok, _ := s.opts.Store.TokenByHash(t.Context(), store.HashToken(key)); !ok || !tok.RevokeKept {
		t.Fatalf("the key named in the file: %+v %v", tok, ok)
	}
	if code, _, _ := postCode(t, ts, codeBody(key), ""); code != 204 {
		t.Fatalf("the file's key's code: %d", code)
	}
	if code, _ := callAPI(t, ts, "GET", "/api/v1/me", key, ""); code != 200 {
		t.Fatalf("the file's key after its code: %d", code)
	}
}

// A body that is not {"code": <64 lower-case hex digits>} is 400 with
// revoke_code_malformed, and counts toward no lockout.
func TestRevokeByCodeMalformed(t *testing.T) {
	t.Parallel()
	rg := startServer(t, withOptions(storeOptions(t, t.TempDir(), "secret")))
	good := store.RevokeCode("x")
	for what, body := range map[string]string{
		"no body":          "",
		"not JSON":         "code",
		"no code":          "{}",
		"a code too short": `{"code":"` + good[:63] + `"}`,
		"a code too long":  `{"code":"` + good + `0"}`,
		"upper case":       `{"code":"` + strings.ToUpper(good) + `"}`,
		"not hex":          `{"code":"` + strings.Repeat("g", 64) + `"}`,
		"a number":         `{"code":64}`,
		"an unknown field": `{"code":"` + good + `","token":"x"}`,
		"an array":         `["` + good + `"]`,
	} {
		code, text, _ := postCode(t, rg.ts, body, "")
		var e struct {
			Code string `json:"code"`
		}
		json.Unmarshal([]byte(text), &e)
		if code != 400 || e.Code != "revoke_code_malformed" {
			t.Errorf("%s: %d %s", what, code, text)
		}
	}
	if d := rg.s.limiter.lockedFor(t.Context(), store.LockoutAddr("127.0.0.1")); d != 0 {
		t.Fatalf("malformed bodies locked the address out for %v", d)
	}
}

// While the address is locked out the route answers 429 with
// Retry-After and revokes nothing; codes, matched or not, count toward
// no lockout.
func TestRevokeByCodeLockout(t *testing.T) {
	t.Parallel()
	rg := startServer(t, withOptions(storeOptions(t, t.TempDir(), "secret")))
	ts, s := rg.ts, rg.s
	ctx := t.Context()
	addr := store.LockoutAddr("127.0.0.1")
	for i := range 3 * DefaultAuthFailures {
		if code, _, _ := postCode(t, ts, codeBody("guess "+strconv.Itoa(i)), ""); code != 204 {
			t.Fatalf("guess %d: %d", i, code)
		}
	}
	if d := s.limiter.lockedFor(ctx, addr); d != 0 {
		t.Fatalf("codes that match nothing locked the address out for %v", d)
	}
	_, key := newKey(t, ts, "secret", store.BootstrapUser)
	for range DefaultAuthFailures {
		s.limiter.fail(ctx, addr)
	}
	code, text, h := postCode(t, ts, codeBody(key), "")
	if code != 429 || h.Get("Retry-After") == "" {
		t.Fatalf("a locked-out address: %d %s %v", code, text, h)
	}
	if tok, ok, _ := s.opts.Store.TokenByHash(ctx, store.HashToken(key)); !ok || !tok.DeletedAt.IsZero() {
		t.Fatalf("the key revoked while the address was locked out: %+v %v", tok, ok)
	}
	locked := s.limiter.lockedFor(ctx, addr)
	postCode(t, ts, codeBody(key), "")
	if again := s.limiter.lockedFor(ctx, addr); again > locked {
		t.Fatalf("a post while locked out moved the lockout from %v to %v", locked, again)
	}
}

// Codes that match nothing are one row of the minute, counted per
// address as refused requests are, no row of commands, and no line of
// the log; a match is a row of its own with the token as its target and
// its user as its subject, a change line and an Info line "by code".
// No row nor line carries the code.
func TestRevokeByCodeAudit(t *testing.T) {
	t.Parallel()
	rg := startServer(t, withOptions(storeOptions(t, t.TempDir(), "secret")), withLog(nil))
	ts, st := rg.ts, rg.s.opts.Store
	ctx := t.Context()
	anna, _ := st.AddUser(ctx, store.User{Name: "anna", Role: store.RoleOperator}, "", 0)
	sess, sessSecret, _ := st.AddToken(ctx, anna.ID, store.TokenSession, "Firefox", time.Now().Add(time.Hour))
	cmd, err := st.AddCommand(ctx, store.Command{Action: "setpoint"})
	if err != nil {
		t.Fatal(err)
	}
	logged := rg.log.Mark()
	lines := logMarkAs(t, ts, "secret")
	before := len(auditRows(t, ts, "secret", "?limit=1000"))
	var codes []string
	for i := range 30 {
		code := store.RevokeCode("guess " + strconv.Itoa(i))
		codes = append(codes, code)
		if status, _, _ := postCode(t, ts, `{"code":"`+code+`"}`, ""); status != 204 {
			t.Fatalf("guess %d: %d", i, status)
		}
	}
	if n := len(auditRows(t, ts, "secret", "?limit=1000")); n != before {
		t.Fatalf("codes that match nothing wrote %d rows of their own", n-before)
	}
	if next, err := st.AddCommand(ctx, store.Command{Action: "setpoint"}); err != nil || next.ID != cmd.ID+1 {
		t.Fatalf("codes that match nothing wrote a row of commands: %d after %d (%v)", next.ID, cmd.ID, err)
	}

	sessCode := store.RevokeCode(sessSecret)
	codes = append(codes, sessCode)
	if status, _, _ := postCode(t, ts, `{"code":"`+sessCode+`"}`, ""); status != 204 {
		t.Fatalf("the session's code: %d", status)
	}
	target := "token:" + strconv.FormatInt(sess.ID, 10)
	isRevoke := func(e event) bool { return e.Kind == "change" && e.Summary["action"] == "token.revoke_code" }
	line, _ := logAfter(t, lines, "the match's change line", isRevoke)
	if line.Summary["target"] != target {
		t.Errorf("the change line: %+v", line.Summary)
	}
	if subject, _ := line.Summary["subject"].(map[string]any); subject["user"] != "anna" || subject["id"] != float64(sess.ID) {
		t.Errorf("the change line's subject: %+v", line.Summary)
	}
	if n := len(logLines(t, lines, isRevoke)); n != 1 {
		t.Errorf("%d change lines of revokes by code, want the match's alone", n)
	}
	rows := auditRows(t, ts, "secret", "?limit=1000")
	if len(rows) != before+1 {
		t.Fatalf("%d rows, want the match's alone", len(rows)-before)
	}
	row := rows[0]
	if row.Action != "token.revoke_code" || row.Result != store.ResultOK || row.Target != target || row.User != "" {
		t.Errorf("the match's row: %+v", row)
	}
	if n := rg.log.Count(logged, testkit.Msg("token revoked", "by", "code", "user", "anna", "token", strconv.FormatInt(sess.ID, 10))); n != 1 {
		t.Errorf("%d Info lines of the match: %v", n, rg.log.Since(logged, nil))
	}
	if n := rg.log.Count(logged, testkit.Msg("token revoked")); n != 1 {
		t.Errorf("%d Info lines of revokes, want the match's alone", n)
	}

	for _, e := range logLines(t, lines, nil) {
		b, _ := json.Marshal(e)
		for _, c := range codes {
			if strings.Contains(string(b), c) {
				t.Errorf("a line of the log carries a code: %s", b)
			}
		}
	}

	// The minute's row is written at the server's stop at the latest.
	rg.stop()
	all, err := st.AuditList(context.Background(), time.Time{}, 1000, "")
	if err != nil {
		t.Fatal(err)
	}
	var minute []store.AuditEntry
	for _, e := range all {
		if e.Action == "token.revoke_code" && e.Target != target {
			minute = append(minute, e)
		}
	}
	if len(minute) != 1 || !strings.Contains(string(minute[0].Detail), `"count":30`) {
		t.Fatalf("the minute's rows of codes that matched nothing: %+v", minute)
	}
	for _, e := range all {
		for _, c := range codes {
			if strings.Contains(string(e.Detail), c) {
				t.Errorf("a row carries a code: %s %s", e.Action, e.Detail)
			}
		}
	}
	for _, rec := range rg.log.Since(logged, nil) {
		for _, c := range codes {
			if strings.Contains(rec.String(), c) {
				t.Errorf("the log carries a code: %s", rec)
			}
		}
	}
}

// Revokes by code that match nothing and requests refused for a wrong
// token from the same address within one minute are rows of their own,
// each with its count, so that neither hides in the other's row.
func TestRevokeByCodeRowsApartFromRefusals(t *testing.T) {
	t.Parallel()
	rg := startServer(t, withOptions(storeOptions(t, t.TempDir(), "secret")))
	ts, st := rg.ts, rg.s.opts.Store
	for i := range 3 {
		if code, _ := callAPI(t, ts, "POST", "/api/v1/logout", "not-a-token-"+strconv.Itoa(i), ""); code != 401 {
			t.Fatalf("a wrong token: %d", code)
		}
	}
	for i := range 5 {
		if code, _, _ := postCode(t, ts, codeBody("guess "+strconv.Itoa(i)), ""); code != 204 {
			t.Fatalf("guess %d: %d", i, code)
		}
	}
	rg.stop()
	all, err := st.AuditList(context.Background(), time.Time{}, 1000, "")
	if err != nil {
		t.Fatal(err)
	}
	rows := map[string]string{}
	for _, e := range all {
		rows[e.Action] = string(e.Detail)
	}
	if !strings.Contains(rows["logout"], `"count":3`) || !strings.Contains(rows["token.revoke_code"], `"count":5`) {
		t.Fatalf("the refusals and the codes in rows of their own: %v", rows)
	}
}

// A server without a store keeps no token to revoke: 204 at once.
func TestRevokeByCodeWithoutStore(t *testing.T) {
	t.Parallel()
	ts := startServer(t, withOptions(Options{Token: "secret"})).ts
	if code, text, _ := postCode(t, ts, codeBody("secret"), ""); code != 204 || text != "" {
		t.Fatalf("without a store: %d %q", code, text)
	}
	if code, _ := callAPI(t, ts, "GET", "/api/v1/me", "secret", ""); code != 200 {
		t.Fatalf("the token after: %d", code)
	}
}

// A token made before the store kept revoke codes gets its code's hash
// at its first use, within the hour of its last use too, and is ended
// by its code after it; the settings file's key never gets one and is
// touched at most once an hour, as before.
func TestRevokeCodeAtFirstUse(t *testing.T) {
	t.Parallel()
	proxies, err := ParseTrustedProxies("127.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	o := storeOptions(t, t.TempDir(), "secret")
	o.TrustedProxies = proxies
	rg := startServer(t, withOptions(o))
	ts, st := rg.ts, rg.s.opts.Store
	ctx := t.Context()
	anna, _ := st.AddUser(ctx, store.User{Name: "anna", Role: store.RoleOperator}, "", 0)
	const secret = "00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff"
	old, err := st.AddTokenHash(ctx, anna.ID, store.TokenAPI, "made before the update", store.HashToken(secret), time.Time{})
	if err != nil || old.RevokeKept {
		t.Fatalf("a key without a revoke code: %+v %v", old, err)
	}
	if err := st.TouchToken(ctx, old.ID, "192.0.2.9", time.Time{}, ""); err != nil {
		t.Fatal(err)
	}
	if code, _, _ := postCode(t, ts, codeBody(secret), ""); code != 204 {
		t.Fatalf("its code before its use: %d", code)
	}
	if tok, ok, _ := st.TokenByHash(ctx, store.HashToken(secret)); !ok || tok.RevokeKept {
		t.Fatalf("the key after its code before its use: %+v %v", tok, ok)
	}
	if code, _ := callAPI(t, ts, "GET", "/api/v1/me", secret, ""); code != 200 {
		t.Fatalf("its use: %d", code)
	}
	if tok, ok, _ := st.TokenByHash(ctx, store.HashToken(secret)); !ok || !tok.RevokeKept {
		t.Fatalf("the key after its use, within the hour of the last: %+v %v", tok, ok)
	}
	if code, _, _ := postCode(t, ts, codeBody(secret), ""); code != 204 {
		t.Fatalf("its code after its use: %d", code)
	}
	if code, _ := callAPI(t, ts, "GET", "/api/v1/me", secret, ""); code != 401 {
		t.Fatalf("the key after its code: %d", code)
	}

	use := func(from string) store.Token {
		t.Helper()
		req, _ := http.NewRequest("GET", ts.URL+"/api/v1/me", nil)
		req.Header.Set("Authorization", "Bearer secret")
		req.Header.Set("X-Forwarded-For", from)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Fatalf("the file's key from %s: %d", from, resp.StatusCode)
		}
		tok, ok, _ := st.TokenByHash(ctx, store.HashToken("secret"))
		if !ok {
			t.Fatal("the file's key is gone")
		}
		return tok
	}
	first := use("192.0.2.10")
	second := use("192.0.2.11")
	if first.RevokeKept || second.RevokeKept {
		t.Fatalf("the file's key got a revoke code: %+v", second)
	}
	if second.LastUsedFrom != first.LastUsedFrom || !second.LastUsedAt.Equal(first.LastUsedAt) {
		t.Fatalf("the file's key touched twice within the hour: %+v, then %+v", first, second)
	}
}
