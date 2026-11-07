// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/bytemare/opaque"

	"go.metaworx.ch/massimo/internal/store"
)

// opaqueOptions is the key stretching of the browser's library
// (opaque-ke) for bytemare/opaque's client: a salt of 16 zero bytes, an
// output of 64 bytes, the parameters of a new record.
func opaqueOptions(t *testing.T) *opaque.ClientOptions {
	t.Helper()
	m, p, l, err := store.ParseKSF(store.OpaqueKSF)
	if err != nil {
		t.Fatal(err)
	}
	return &opaque.ClientOptions{KSFSalt: make([]byte, 16), KSFLength: 64, KSFParameters: []uint64{uint64(p), uint64(m), uint64(l)}}
}

func opaqueClient(t *testing.T) *opaque.Client {
	t.Helper()
	c, err := opaque.DefaultConfiguration().Client()
	if err != nil {
		t.Fatal(err)
	}
	return c
}

// opaqueRecord runs a registration for a name through the route, with a
// token, and returns the record to send.
func opaqueRecord(t *testing.T, ts *httptest.Server, token, name, password string) string {
	t.Helper()
	c := opaqueClient(t)
	req, err := c.RegistrationInit([]byte(password))
	if err != nil {
		t.Fatal(err)
	}
	code, body := callAPI(t, ts, "POST", "/api/v1/users/"+name+"/register", token, `{"request":"`+opaqueB64.EncodeToString(req.Serialize())+`"}`)
	if code != 200 {
		t.Fatalf("register %s: %d %s", name, code, body)
	}
	var out struct{ Response, KSF string }
	json.Unmarshal([]byte(body), &out)
	b, _ := opaqueB64.DecodeString(out.Response)
	resp, err := c.Deserialize.RegistrationResponse(b)
	if err != nil {
		t.Fatal(err)
	}
	if out.KSF != store.OpaqueKSF {
		t.Errorf("register: ksf %q", out.KSF)
	}
	rec, _, err := c.RegistrationFinalize(resp, nil, nil, opaqueOptions(t))
	if err != nil {
		t.Fatal(err)
	}
	return opaqueB64.EncodeToString(rec.Serialize())
}

// loginStarted is the answer of POST /api/v1/login/start.
type loginStarted struct {
	Login string `json:"login"`
	KE2   string `json:"ke2"`
	KSF   string `json:"ksf"`
	V1    bool   `json:"v1"`
}

// opaqueSignIn runs a sign-in by OPAQUE: the start's status and answer,
// then the finish's, with a KE3 of zeros where the client could not
// open the answer.
func opaqueSignIn(t *testing.T, ts *httptest.Server, name, password, purpose string) (started loginStarted, code int, body string) {
	t.Helper()
	c := opaqueClient(t)
	ke1, err := c.GenerateKE1([]byte(password))
	if err != nil {
		t.Fatal(err)
	}
	start, _ := json.Marshal(map[string]string{"user": name, "ke1": opaqueB64.EncodeToString(ke1.Serialize()), "purpose": purpose})
	code, body = callAPI(t, ts, "POST", "/api/v1/login/start", "", string(start))
	if code != 200 {
		return started, code, body
	}
	json.Unmarshal([]byte(body), &started)
	b, _ := opaqueB64.DecodeString(started.KE2)
	ke2, err := c.Deserialize.KE2(b)
	if err != nil {
		t.Fatalf("ke2 of %d bytes: %v", len(b), err)
	}
	ke3 := make([]byte, 64)
	if k, _, _, err := c.GenerateKE3(ke2, nil, nil, opaqueOptions(t)); err == nil {
		ke3 = k.Serialize()
	} else {
		ke3[0] = 1 // a KE3 of zeros does not decode; this one is merely wrong
	}
	code, body = callAPI(t, ts, "POST", "/api/v1/login/finish", "", `{"login":"`+started.Login+`","ke3":"`+opaqueB64.EncodeToString(ke3)+`"}`)
	return started, code, body
}

// A user created with a record signs in by OPAQUE and gets a session; a
// wrong password and an unknown name are answered alike at the start and
// refused at the finish; the audit keeps none of the messages.
func TestOpaqueSignIn(t *testing.T) {
	_, ts, _ := startWith(t, storeOptions(t, t.TempDir(), "secret"))
	rec := opaqueRecord(t, ts, "secret", "anna", "correct horse")
	code, body := callAPI(t, ts, "POST", "/api/v1/users", "secret", `{"name":"anna","role":"operator","record":"`+rec+`","ksf":"`+store.OpaqueKSF+`"}`)
	if code != 201 {
		t.Fatalf("add with a record: %d %s", code, body)
	}
	var u userView
	json.Unmarshal([]byte(body), &u)
	if u.AuthVersion != 2 || !u.HasPassword {
		t.Fatalf("user %+v", u)
	}
	started, code, body := opaqueSignIn(t, ts, "Anna", "correct horse", "")
	if code != 200 {
		t.Fatalf("sign-in: %d %s", code, body)
	}
	if !started.V1 || started.KSF != store.OpaqueKSF || started.Login == "" {
		t.Errorf("start %+v", started)
	}
	var sess struct {
		Token string   `json:"token"`
		User  userView `json:"user"`
	}
	json.Unmarshal([]byte(body), &sess)
	if code, _ := callAPI(t, ts, "GET", "/api/v1/me", sess.Token, ""); code != 200 || sess.User.Name != "anna" || sess.User.LastLoginAt == nil {
		t.Errorf("the session: %d, %+v", code, sess.User)
	}
	for _, c := range []struct{ name, password string }{{"anna", "wrong horse"}, {"nobody", "correct horse"}} {
		started, code, body := opaqueSignIn(t, ts, c.name, c.password, "")
		if code != 401 || started.Login == "" || len(started.KE2) != len(opaqueB64.EncodeToString(make([]byte, 320))) {
			t.Errorf("%s with %q: %d %s, start %+v", c.name, c.password, code, body, started)
		}
	}
	if code, _ := callAPI(t, ts, "POST", "/api/v1/login", "", `{"user":"anna","password":"correct horse"}`); code != 401 {
		t.Errorf("a password sign-in of a user of version 2: %d", code)
	}
	for _, row := range auditRows(t, ts, "secret", "?limit=1000") {
		var d struct{ Body map[string]any }
		json.Unmarshal(row.Detail, &d)
		for _, k := range []string{"record", "ke1", "ke3", "login", "proof"} {
			if v, ok := d.Body[k]; ok && v != "…" {
				t.Errorf("audit row %s %s keeps %s: %v", row.Action, row.Target, k, v)
			}
		}
	}
}

// A start is a guess: it counts against the name and the address at
// once, so that starts never finished lock out as wrong passwords do;
// a finished sign-in clears the count.
func TestOpaqueStartCounts(t *testing.T) {
	opts := storeOptions(t, t.TempDir(), "secret")
	opts.AuthFailures = 3
	_, ts, s := startWith(t, opts)
	rec := opaqueRecord(t, ts, "secret", "anna", "pw")
	if code, body := callAPI(t, ts, "POST", "/api/v1/users", "secret", `{"name":"anna","record":"`+rec+`","ksf":"`+store.OpaqueKSF+`"}`); code != 201 {
		t.Fatalf("add: %d %s", code, body)
	}
	for range 2 {
		if _, code, _ := opaqueSignIn(t, ts, "anna", "wrong", ""); code != 401 {
			t.Fatalf("a wrong password: %d", code)
		}
	}
	// The third start locks the address and the name out; the finish
	// that proves the password lifts what its own start set.
	_, code, body := opaqueSignIn(t, ts, "anna", "pw", "")
	var sess struct{ Token string }
	json.Unmarshal([]byte(body), &sess)
	if code != 200 {
		t.Fatalf("the right one after two wrong: %d %s", code, body)
	}
	if code, body := callAPI(t, ts, "GET", "/api/v1/me", sess.Token, ""); code != 200 {
		t.Fatalf("the session from the address its start locked: %d %s", code, body)
	}
	// Starts alone, their finishes never sent: each one's claim ended by
	// hand, as its minute would.
	for range 3 {
		c := opaqueClient(t)
		ke1, _ := c.GenerateKE1([]byte("guess"))
		if code, body := callAPI(t, ts, "POST", "/api/v1/login/start", "", `{"user":"anna","ke1":"`+opaqueB64.EncodeToString(ke1.Serialize())+`"}`); code != 200 {
			t.Fatalf("a start: %d %s", code, body)
		}
		s.signIns.release(store.LockoutAddr("127.0.0.1"))
	}
	if _, code, body := opaqueSignIn(t, ts, "anna", "pw", ""); code != 429 {
		t.Fatalf("after three starts never finished: %d %s", code, body)
	}
}

// A start holds its address until the finish: a second start and a
// password sign-in from it are 429 meanwhile, a finished or an ended
// start lets the next one in, and a login is finished once.
func TestOpaqueOneAtATime(t *testing.T) {
	_, ts, s := startWith(t, storeOptions(t, t.TempDir(), "secret"))
	rec := opaqueRecord(t, ts, "secret", "anna", "pw")
	callAPI(t, ts, "POST", "/api/v1/users", "secret", `{"name":"anna","record":"`+rec+`","ksf":"`+store.OpaqueKSF+`"}`)
	c := opaqueClient(t)
	ke1, _ := c.GenerateKE1([]byte("pw"))
	start := `{"user":"anna","ke1":"` + opaqueB64.EncodeToString(ke1.Serialize()) + `"}`
	code, body := callAPI(t, ts, "POST", "/api/v1/login/start", "", start)
	if code != 200 {
		t.Fatalf("start: %d %s", code, body)
	}
	var started loginStarted
	json.Unmarshal([]byte(body), &started)
	if code, _ := callAPI(t, ts, "POST", "/api/v1/login/start", "", start); code != 429 {
		t.Errorf("a second start meanwhile: %d", code)
	}
	if code, _ := callAPI(t, ts, "POST", "/api/v1/login", "", `{"user":"anna","password":"pw"}`); code != 429 {
		t.Errorf("a password sign-in meanwhile: %d", code)
	}
	b, _ := opaqueB64.DecodeString(started.KE2)
	ke2, _ := c.Deserialize.KE2(b)
	ke3, _, _, err := c.GenerateKE3(ke2, nil, nil, opaqueOptions(t))
	if err != nil {
		t.Fatal(err)
	}
	finish := `{"login":"` + started.Login + `","ke3":"` + opaqueB64.EncodeToString(ke3.Serialize()) + `"}`
	if code, body := callAPI(t, ts, "POST", "/api/v1/login/finish", "", finish); code != 200 {
		t.Fatalf("finish: %d %s", code, body)
	}
	if code, _ := callAPI(t, ts, "POST", "/api/v1/login/finish", "", finish); code != 401 {
		t.Errorf("a login finished twice: %d", code)
	}
	if len(s.signIns.keys) != 0 {
		t.Errorf("claims after the finish: %v", s.signIns.keys)
	}
	// A start whose minute ended: its claim gives way, its finish is 401.
	code, body = callAPI(t, ts, "POST", "/api/v1/login/start", "", start)
	json.Unmarshal([]byte(body), &started)
	s.signIns.hold(store.LockoutAddr("127.0.0.1"), time.Now().Add(-time.Second))
	s.logins.pending[started.Login].expires = time.Now().Add(-time.Second)
	if code, _ := callAPI(t, ts, "POST", "/api/v1/login/finish", "", `{"login":"`+started.Login+`","ke3":"`+opaqueB64.EncodeToString(ke3.Serialize())+`"}`); code != 401 {
		t.Errorf("a finish after the minute: %d", code)
	}
	if _, code, _ := opaqueSignIn(t, ts, "anna", "pw", ""); code != 200 {
		t.Errorf("the next sign-in after one that ended: %d", code)
	}
}

// One's own password of version 2 is set with a proof of the current
// one, good once; someone with users needs none, and ends the user's
// sessions.
func TestOpaqueOwnPassword(t *testing.T) {
	_, ts, _ := startWith(t, storeOptions(t, t.TempDir(), "secret"))
	callAPI(t, ts, "POST", "/api/v1/users", "secret", `{"name":"anna","role":"operator","password":"old"}`)
	_, body := callAPI(t, ts, "POST", "/api/v1/login", "", `{"user":"anna","password":"old"}`)
	var sess struct{ Token string }
	json.Unmarshal([]byte(body), &sess)
	rec := opaqueRecord(t, ts, sess.Token, "anna", "new")
	put := func(extra string) (int, string) {
		return callAPI(t, ts, "PUT", "/api/v1/users/anna", sess.Token, `{"record":"`+rec+`","ksf":"`+store.OpaqueKSF+`"`+extra+`}`)
	}
	if code, _ := put(""); code != 403 {
		t.Errorf("one's own record without a proof: %d", code)
	}
	if code, _ := put(`,"proof":"made-up"`); code != 403 {
		t.Errorf("with a made-up proof: %d", code)
	}
	// Version 1 to 2 with the current password, version 2 to 2 with a
	// proof from a sign-in by OPAQUE.
	if code, body := put(`,"current_password":"old"`); code != 200 {
		t.Fatalf("with the current password: %d %s", code, body)
	}
	_, code, body := opaqueSignIn(t, ts, "anna", "new", purposeProof)
	var proof struct{ Proof string }
	json.Unmarshal([]byte(body), &proof)
	if code != 200 || proof.Proof == "" || strings.Contains(body, "token") {
		t.Fatalf("a proof: %d %s", code, body)
	}
	rec = opaqueRecord(t, ts, sess.Token, "anna", "newer")
	if code, body := put(`,"proof":"` + proof.Proof + `"`); code != 200 {
		t.Fatalf("with the proof: %d %s", code, body)
	}
	if code, _ := put(`,"proof":"` + proof.Proof + `"`); code != 403 {
		t.Errorf("a proof used twice: %d", code)
	}
	if _, code, _ := opaqueSignIn(t, ts, "anna", "newer", ""); code != 200 {
		t.Errorf("the new password: %d", code)
	}
	if code, _ := callAPI(t, ts, "GET", "/api/v1/me", sess.Token, ""); code != 200 {
		t.Errorf("one's own change ended one's session: %d", code)
	}
	rec = opaqueRecord(t, ts, "secret", "anna", "set")
	if code, body := callAPI(t, ts, "PUT", "/api/v1/users/anna", "secret", `{"record":"`+rec+`","ksf":"`+store.OpaqueKSF+`"}`); code != 200 {
		t.Fatalf("set by the owner: %d %s", code, body)
	}
	if code, _ := callAPI(t, ts, "GET", "/api/v1/me", sess.Token, ""); code != 401 {
		t.Errorf("a record set by someone else left the session: %d", code)
	}
}

// The registration is open to whoever may set the name's password or
// add its user, the first owner from the local network among them; bad
// messages are 400.
func TestOpaqueRegister(t *testing.T) {
	_, open, _ := startWith(t, storeOptions(t, t.TempDir(), ""))
	rec := opaqueRecord(t, open, "", "root", "pw")
	if code, body := callAPI(t, open, "POST", "/api/v1/users", "", `{"name":"root","record":"`+rec+`","ksf":"`+store.OpaqueKSF+`"}`); code != 201 {
		t.Fatalf("the first owner with a record: %d %s", code, body)
	}
	if _, code, body := opaqueSignIn(t, open, "root", "pw", ""); code != 200 {
		t.Fatalf("the first owner signs in: %d %s", code, body)
	}

	_, ts, s := startWith(t, storeOptions(t, t.TempDir(), "secret"))
	viewer := userToken(t, s.opts.Store, "vera", store.RoleViewer)
	userToken(t, s.opts.Store, "otto", store.RoleOperator)
	req := `{"request":"` + opaqueB64.EncodeToString(must(must(opaque.DefaultConfiguration().Client()).RegistrationInit([]byte("pw"))).Serialize()) + `"}`
	for _, c := range []struct {
		token, name, body string
		want              int
	}{
		{viewer, "vera", req, 200},
		{viewer, "otto", req, 403},
		{viewer, "nobody", req, 403},
		{"secret", "nobody", req, 200},
		{"secret", "otto", `{"request":"!"}`, 400},
		{"secret", "otto", `{"request":"AAAA"}`, 400},
		{"secret", "otto", `{}`, 400},
	} {
		if code, body := callAPI(t, ts, "POST", "/api/v1/users/"+c.name+"/register", c.token, c.body); code != c.want {
			t.Errorf("%s for %s, %s: %d %s", c.token, c.name, c.body, code, body)
		}
	}
	rec = opaqueRecord(t, ts, "secret", "anna", "pw")
	for _, body := range []string{
		`{"name":"anna","record":"` + rec + `","ksf":"m=1024,t=1,p=1"}`,
		`{"name":"anna","record":"` + rec[:40] + `","ksf":"` + store.OpaqueKSF + `"}`,
		`{"name":"anna","record":"` + rec + `","ksf":"` + store.OpaqueKSF + `","password":"pw"}`,
	} {
		if code, _ := callAPI(t, ts, "POST", "/api/v1/users", "secret", body); code != 400 {
			t.Errorf("%s: %d", body, code)
		}
	}
	for _, body := range []string{`{"user":"anna","ke1":"!"}`, `{"user":"anna","ke1":"AAAA"}`, `{"user":"anna"}`, `{"user":"anna","ke1":"AAAA","purpose":"session"}`} {
		if code, _ := callAPI(t, ts, "POST", "/api/v1/login/start", "", body); code != 400 {
			t.Errorf("start %s: %d", body, code)
		}
	}
	if code, _ := callAPI(t, ts, "POST", "/api/v1/login/finish", "", `{"login":"made-up","ke3":"AAAA"}`); code != 401 {
		t.Errorf("a finish of nothing started: %d", code)
	}
}

func must[T any](v T, err error) T {
	if err != nil {
		panic(err)
	}
	return v
}
