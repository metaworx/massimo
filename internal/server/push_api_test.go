// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

import (
	"crypto/ecdh"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"testing"
	"time"

	"go.metaworx.ch/massimo/internal/cubelan/fakecube"
	"go.metaworx.ch/massimo/internal/protocol/cube"
	"go.metaworx.ch/massimo/internal/store"
)

// subscription is a browser's PushSubscription.toJSON() for endpoint,
// with keys of its own.
func subscription(t *testing.T, endpoint, label string) string {
	t.Helper()
	key, err := ecdh.P256().GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	auth := make([]byte, 16)
	rand.Read(auth)
	return fmt.Sprintf(`{"endpoint": %q, "expirationTime": null, "keys": {"p256dh": %q, "auth": %q}, "label": %q}`,
		endpoint, base64.RawURLEncoding.EncodeToString(key.PublicKey().Bytes()), base64.RawURLEncoding.EncodeToString(auth), label)
}

// The VAPID key is made at its first use and kept in the store: the
// same for the next server over it, and not among the settings the API
// shows.
func TestVAPIDKey(t *testing.T) {
	opts := storeOptions(t, t.TempDir(), "secret")
	_, ts, _ := startWith(t, opts)
	code, text := callAPI(t, ts, "GET", "/api/v1/push/vapid", "secret", "")
	var v struct {
		Key string `json:"key"`
	}
	json.Unmarshal([]byte(text), &v)
	raw, err := base64.RawURLEncoding.DecodeString(v.Key)
	if code != 200 || err != nil {
		t.Fatalf("vapid: %d %s %v", code, text, err)
	}
	if _, err := ecdh.P256().NewPublicKey(raw); err != nil {
		t.Fatalf("not a P-256 key: %v", err)
	}
	if _, ok, _ := opts.Store.Setting(t.Context(), store.VAPIDKey); !ok {
		t.Fatal("not kept in the store")
	}
	_, again, _ := startWith(t, opts)
	if _, text := callAPI(t, again, "GET", "/api/v1/push/vapid", "secret", ""); !strings.Contains(text, v.Key) {
		t.Fatalf("another key for the next server: %s", text)
	}
	if _, text := callAPI(t, ts, "GET", "/api/v1/settings", "secret", ""); strings.Contains(text, "vapid") {
		t.Fatalf("the settings show the key: %s", text)
	}
	_, legacy, _ := start(t, "secret")
	if code, _ := callAPI(t, legacy, "GET", "/api/v1/push/vapid", "secret", ""); code != 503 {
		t.Fatalf("without a store: %d", code)
	}
}

// A subscription is the caller's, made with the caller's session or
// key: listed without its keys, the same endpoint again updating it,
// removed by its owner only; the audit has both, and no command.
func TestPushSubscriptionRoutes(t *testing.T) {
	_, ts, s := startWith(t, storeOptions(t, t.TempDir(), "secret"))
	st := s.opts.Store
	const owner = "secret"
	body := subscription(t, "https://push.example.net/send/a", "Firefox on the phone")
	code, text := callAPI(t, ts, "POST", "/api/v1/me/push", owner, body)
	var v pushView
	json.Unmarshal([]byte(text), &v)
	if code != 201 || v.ID == 0 || v.Endpoint != "https://push.example.net/send/a" || v.Label != "Firefox on the phone" || !v.Current || v.Failures != 0 || v.LastOKAt != nil || strings.Contains(text, "p256dh") {
		t.Fatalf("subscribe: %d %s", code, text)
	}
	code, text = callAPI(t, ts, "POST", "/api/v1/me/push", owner, subscription(t, "https://push.example.net/send/a", ""))
	var again pushView
	json.Unmarshal([]byte(text), &again)
	if code != 200 || again.ID != v.ID || !strings.HasPrefix(again.Label, "Go-http-client") {
		t.Fatalf("the same endpoint again: %d %s", code, text)
	}
	code, text = callAPI(t, ts, "GET", "/api/v1/me/push", owner, "")
	var list []pushView
	json.Unmarshal([]byte(text), &list)
	if code != 200 || len(list) != 1 || list[0].ID != v.ID || !list[0].Current {
		t.Fatalf("list: %d %s", code, text)
	}

	op := userToken(t, st, "op", store.RoleOperator)
	if code, text := callAPI(t, ts, "GET", "/api/v1/me/push", op, ""); code != 200 || strings.TrimSpace(text) != "[]" {
		t.Fatalf("another user's list: %d %s", code, text)
	}
	if code, _ := callAPI(t, ts, "DELETE", "/api/v1/me/push/"+strconv.FormatInt(v.ID, 10), op, ""); code != 404 {
		t.Fatalf("another user removed it: %d", code)
	}
	for _, role := range []string{store.RoleGuest, store.RoleIntegration} {
		tok := userToken(t, st, "u-"+role, role)
		if code, text := callAPI(t, ts, "POST", "/api/v1/me/push", tok, subscription(t, "https://push.example.net/send/"+role, "")); code != 403 || errorOf(text) != `capability "preferences" required` {
			t.Errorf("%s subscribed: %d %s", role, code, text)
		}
		if code, _ := callAPI(t, ts, "DELETE", "/api/v1/me/push/"+strconv.FormatInt(v.ID, 10), tok, ""); code != 403 {
			t.Errorf("%s unsubscribed: %d", role, code)
		}
	}
	for _, bad := range []string{
		`{`,
		subscription(t, "ftp://push.example.net/a", ""),
		subscription(t, "https:///a", ""),
		`{"endpoint": "https://push.example.net/b", "keys": {"p256dh": "AQID", "auth": "AAAAAAAAAAAAAAAAAAAAAA"}}`,
		strings.Replace(subscription(t, "https://push.example.net/c", ""), `"auth": "`, `"auth": "AA`, 1),
		`{"endpoint": "https://push.example.net/d", "keys": {"p256dh": "` + base64.RawURLEncoding.EncodeToString(append([]byte{4}, make([]byte, 64)...)) + `", "auth": "AAAAAAAAAAAAAAAAAAAAAA"}}`,
	} {
		if code, text := callAPI(t, ts, "POST", "/api/v1/me/push", owner, bad); code != 400 {
			t.Errorf("%s: %d %s", bad, code, text)
		}
	}
	if code, text := callAPI(t, ts, "DELETE", "/api/v1/me/push/"+strconv.FormatInt(v.ID, 10), owner, ""); code != 200 || !strings.Contains(text, `"ok": true`) {
		t.Fatalf("unsubscribe: %d %s", code, text)
	}
	if code, _ := callAPI(t, ts, "DELETE", "/api/v1/me/push/x", owner, ""); code != 400 {
		t.Fatalf("a bad id: %d", code)
	}
	if _, text := callAPI(t, ts, "GET", "/api/v1/me/push", owner, ""); strings.TrimSpace(text) != "[]" {
		t.Fatalf("after the removal: %s", text)
	}
	// Every row names the subscription its path or its answer names, a
	// refused or failed one too; a subscription that was not made, none.
	seen := map[string]int{}
	for _, r := range auditRows(t, ts, owner, "?limit=100") {
		if !strings.HasPrefix(r.Action, "push.") {
			continue
		}
		seen[r.Action+" "+r.Result]++
		if r.CommandID != 0 {
			t.Errorf("a command of %s: %d", r.Action, r.CommandID)
		}
		want := "push:" + strconv.FormatInt(v.ID, 10)
		switch {
		case r.Action == "push.unsubscribe" && strings.Contains(string(r.Detail), `/api/v1/me/push/x"`):
			want = "push:x"
		case r.Action == "push.subscribe" && r.Result != store.ResultOK:
			want = ""
		}
		if r.Target != want {
			t.Errorf("%s %s: target %q, want %q", r.Action, r.Result, r.Target, want)
		}
	}
	if seen["push.subscribe ok"] != 2 || seen["push.unsubscribe ok"] != 1 || seen["push.unsubscribe refused"] != 2 || seen["push.unsubscribe failed"] != 2 {
		t.Errorf("audit rows: %v", seen)
	}
}

// Without a signed-in user the routes answer 409, without a store 503.
func TestPushWithoutUser(t *testing.T) {
	_, open, _ := startWith(t, storeOptions(t, t.TempDir(), ""))
	for _, m := range []string{"GET", "POST"} {
		if code, text := callAPI(t, open, m, "/api/v1/me/push", "", subscription(t, "https://push.example.net/a", "")); code != 409 || !strings.Contains(text, "no user signed in") {
			t.Errorf("open server %s: %d %s", m, code, text)
		}
	}
	_, legacy, _ := start(t, "secret")
	if code, _ := callAPI(t, legacy, "GET", "/api/v1/me/push", "secret", ""); code != 503 {
		t.Fatalf("no store: %d", code)
	}
}

// The notify preference is read as the rules read it: a document they
// cannot is refused.
func TestNotifyPreferenceChecked(t *testing.T) {
	_, ts, _ := startWith(t, storeOptions(t, t.TempDir(), "secret"))
	for _, bad := range []string{`{"window_open": {"on": true, "after_min": 0}}`, `{"sunshine": {"on": true}}`, `[]`} {
		if code, text := callAPI(t, ts, "PUT", "/api/v1/me/preferences/notify", "secret", bad); code != 400 {
			t.Errorf("%s: %d %s", bad, code, text)
		}
	}
	if code, text := callAPI(t, ts, "PUT", "/api/v1/me/preferences/notify", "secret", `{"window_open": {"on": true, "after_min": 15}}`); code != 200 {
		t.Fatalf("a good one: %d %s", code, text)
	}
	if code, _ := callAPI(t, ts, "PUT", "/api/v1/me/preferences/ui", "secret", `{"after_min": 0}`); code != 200 {
		t.Fatal("another key is not the rules'")
	}
}

// A device of the fake Cube whose battery goes low is told to the user
// with a subscription, through the server's events and the rules; a
// user without one is told nothing.
func TestNotificationFromTheCube(t *testing.T) {
	opts := storeOptions(t, t.TempDir(), "secret")
	var log syncBuffer
	opts.Logger = slog.New(slog.NewTextHandler(&log, nil))
	opts.PollInterval = 100 * time.Millisecond
	fc, ts, s := startWith(t, opts)
	userToken(t, s.opts.Store, "op", store.RoleOperator)
	if code, text := callAPI(t, ts, "POST", "/api/v1/me/push", "secret", subscription(t, "https://push.example.net/send/a", "phone")); code != 201 {
		t.Fatalf("subscribe: %d %s", code, text)
	}
	eventually(t, "the cube", func() bool {
		var c cubeView
		return getJSON(t, ts, "secret", "/api/v1/cubes/KEQ0000100", &c) == 200 && c.Connected
	})
	// 0a010d reports manual, 21 °C, valve 30 %; then its battery low.
	greeting, _ := fakecube.LoadGreeting(fixture)
	l := greeting[len(greeting)-1]
	raw, _ := base64.StdEncoding.DecodeString(l.Data)
	copy(raw, []byte{0x0b, 0x0a, 0x01, 0x0d, 0x09, 0x12, 0x01, 30, 42, 0, 0, 0})
	fc.SetDeviceList(cube.Message{Type: 'L', Data: base64.StdEncoding.EncodeToString(raw)})
	eventually(t, "the first state", func() bool { return strings.Contains(log.String(), "device=0a010d") })
	raw[6] |= 0x80
	fc.SetDeviceList(cube.Message{Type: 'L', Data: base64.StdEncoding.EncodeToString(raw)})
	eventually(t, "the notification", func() bool {
		return strings.Contains(log.String(), "msg=notification") && strings.Contains(log.String(), "kind=battery_low cube=KEQ0000100 device=0a010d")
	})
	if n := strings.Count(log.String(), "msg=notification"); n != 1 {
		t.Fatalf("%d notifications, want the subscriber's one:\n%s", n, log.String())
	}
}
