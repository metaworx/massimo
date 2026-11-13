// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

import (
	"bufio"
	"context"
	"crypto/ecdh"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"go.metaworx.ch/massimo/internal/cubelan/fakecube"
	"go.metaworx.ch/massimo/internal/notify"
	"go.metaworx.ch/massimo/internal/protocol/cube"
	"go.metaworx.ch/massimo/internal/push"
	"go.metaworx.ch/massimo/internal/push/pushtest"
	"go.metaworx.ch/massimo/internal/store"
)

// loopbackOnly is the tests' Web Push client: it reaches the loopback
// alone, so that a subscription's endpoint in a test, such as
// push.example.net, never reaches the network.
var loopbackOnly = &http.Client{
	Timeout: 5 * time.Second,
	Transport: &http.Transport{DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
		host, _, err := net.SplitHostPort(addr)
		if ip, perr := netip.ParseAddr(host); err != nil || perr != nil || !ip.IsLoopback() {
			return nil, fmt.Errorf("the tests reach no network: %s", addr)
		}
		var d net.Dialer
		return d.DialContext(ctx, network, addr)
	}},
	CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
}

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

// The VAPID key is made at its first use and kept sealed in the store:
// the same for the next server over it, not among the settings the API
// shows, and made anew where it no longer opens.
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
	kept, ok, _ := opts.Store.Setting(t.Context(), store.VAPIDKey)
	if !ok || !strings.HasPrefix(string(kept), `"v2:`) || strings.Contains(string(kept), "private") {
		t.Fatalf("not kept sealed in the store: %s", kept)
	}
	_, again, _ := startWith(t, opts)
	if _, text := callAPI(t, again, "GET", "/api/v1/push/vapid", "secret", ""); !strings.Contains(text, v.Key) {
		t.Fatalf("another key for the next server: %s", text)
	}
	if _, text := callAPI(t, ts, "GET", "/api/v1/settings", "secret", ""); strings.Contains(text, "vapid") {
		t.Fatalf("the settings show the key: %s", text)
	}
	// one that no longer opens is made anew (AP SealedSecrets v1.0, D36-A)
	opts.Store.SetSetting(t.Context(), store.VAPIDKey, "v2:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA")
	_, lost, _ := startWith(t, opts)
	if code, text := callAPI(t, lost, "GET", "/api/v1/push/vapid", "secret", ""); code != 200 || strings.Contains(text, v.Key) {
		t.Fatalf("a lost key: %d %s", code, text)
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

// notificationEvents opens a Cube's event stream with the token and
// returns a channel of the notifications it sends.
func notificationEvents(t *testing.T, ts *httptest.Server, token string) <-chan notify.Notification {
	t.Helper()
	req, _ := http.NewRequest("GET", ts.URL+"/api/v1/cubes/KEQ0000100/events", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil || resp.StatusCode != 200 {
		t.Fatalf("stream: %v %v", resp, err)
	}
	out := make(chan notify.Notification, 8)
	go func() {
		defer resp.Body.Close()
		rd := bufio.NewReader(resp.Body)
		kind := ""
		for {
			l, err := rd.ReadString('\n')
			if err != nil {
				return
			}
			switch {
			case strings.HasPrefix(l, "event: "):
				kind = strings.TrimSpace(strings.TrimPrefix(l, "event: "))
			case strings.HasPrefix(l, "data: ") && kind == "notification":
				var ev struct {
					Notification notify.Notification `json:"notification"`
				}
				json.Unmarshal([]byte(strings.TrimPrefix(l, "data: ")), &ev)
				out <- ev.Notification
			}
		}
	}()
	t.Cleanup(func() { resp.Body.Close() })
	return out
}

// A notification reaches its user by Web Push, encrypted for the
// subscription and signed with the server's VAPID key, on the user's
// own event stream and no one else's, and through the deliverers an app
// embedding the server gives.
func TestNotificationDelivered(t *testing.T) {
	opts := storeOptions(t, t.TempDir(), "secret")
	var log syncBuffer
	opts.Logger = slog.New(slog.NewTextHandler(&log, nil))
	opts.PollInterval = 100 * time.Millisecond
	var mu sync.Mutex
	var local []string
	opts.Deliverers = []push.Deliverer{push.Local(func(user int64, n notify.Notification) {
		mu.Lock()
		defer mu.Unlock()
		local = append(local, fmt.Sprintf("%d %s %s", user, n.Kind, n.Device))
	})}
	fc, ts, s := startWith(t, opts)
	st := s.opts.Store
	op := userToken(t, st, "op", store.RoleOperator)
	_, text := callAPI(t, ts, "GET", "/api/v1/push/vapid", "secret", "")
	var vapid struct{ Key string }
	json.Unmarshal([]byte(text), &vapid)
	kb, _ := base64.RawURLEncoding.DecodeString(vapid.Key)
	pub, err := ecdsa.ParseUncompressedPublicKey(elliptic.P256(), kb)
	if err != nil {
		t.Fatalf("the VAPID key %q: %v", vapid.Key, err)
	}
	svc := pushtest.New(t, pub)
	if code, text := callAPI(t, ts, "POST", "/api/v1/me/push", "secret", svc.Subscription("/send/owner", "phone")); code != 201 {
		t.Fatalf("subscribe: %d %s", code, text)
	}
	eventually(t, "the cube", func() bool {
		var c cubeView
		return getJSON(t, ts, "secret", "/api/v1/cubes/KEQ0000100", &c) == 200 && c.Connected
	})
	mine, theirs := notificationEvents(t, ts, "secret"), notificationEvents(t, ts, op)
	// 0a010d reports manual, 21 °C, valve 30 %; then its battery low.
	greeting, _ := fakecube.LoadGreeting(fixture)
	l := greeting[len(greeting)-1]
	raw, _ := base64.StdEncoding.DecodeString(l.Data)
	copy(raw, []byte{0x0b, 0x0a, 0x01, 0x0d, 0x09, 0x12, 0x01, 30, 42, 0, 0, 0})
	fc.SetDeviceList(cube.Message{Type: 'L', Data: base64.StdEncoding.EncodeToString(raw)})
	eventually(t, "the first state", func() bool { return strings.Contains(log.String(), "device=0a010d") })
	raw[6] |= 0x80
	fc.SetDeviceList(cube.Message{Type: 'L', Data: base64.StdEncoding.EncodeToString(raw)})

	select {
	case n := <-mine:
		if n.Kind != notify.KindBatteryLow || n.Cube != "KEQ0000100" || n.Device != "0a010d" {
			t.Fatalf("the stream's notification: %+v", n)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("no notification on the user's stream")
	}
	eventually(t, "the push", func() bool { return len(svc.Received()) == 1 })
	got := svc.Received()[0]
	var n notify.Notification
	if got.Err != nil || json.Unmarshal(got.Message, &n) != nil || n.Kind != notify.KindBatteryLow || n.Device != "0a010d" || got.Path != "/send/owner" {
		t.Fatalf("the push: %+v %s", got, got.Message)
	}
	if h := got.Header; h.Get("TTL") != "86400" || h.Get("Urgency") != "normal" || h.Get("Topic") != "battery_low-0a010d" {
		t.Fatalf("the push's headers: %v", h)
	}
	eventually(t, "the local deliverer", func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(local) == 1
	})
	if !strings.HasSuffix(local[0], " battery_low 0a010d") || strings.HasPrefix(local[0], "0 ") {
		t.Fatalf("the local deliverer: %v", local)
	}
	eventually(t, "the delivery noted", func() bool {
		_, text := callAPI(t, ts, "GET", "/api/v1/me/push", "secret", "")
		var list []pushView
		json.Unmarshal([]byte(text), &list)
		return len(list) == 1 && list[0].LastOKAt != nil && list[0].Failures == 0
	})
	select {
	case n := <-theirs:
		t.Fatalf("another user's stream got %+v", n)
	case <-time.After(200 * time.Millisecond):
	}
}

// A subscription its push service knows no more is removed at the
// first notification.
func TestNotificationRemovesGoneSubscription(t *testing.T) {
	opts := storeOptions(t, t.TempDir(), "secret")
	var log syncBuffer
	opts.Logger = slog.New(slog.NewTextHandler(&log, nil))
	_, ts, s := startWith(t, opts)
	svc := pushtest.New(t, nil, http.StatusGone)
	if code, text := callAPI(t, ts, "POST", "/api/v1/me/push", "secret", svc.Subscription("/send/gone", "phone")); code != 201 {
		t.Fatalf("subscribe: %d %s", code, text)
	}
	subs, err := s.opts.Store.PushSubscriptions(t.Context(), 0)
	if err != nil || len(subs) != 1 {
		t.Fatalf("subscriptions: %v %v", subs, err)
	}
	s.webPush.Deliver(t.Context(), subs[0].UserID, notify.Notification{Kind: notify.KindUpdate, Values: map[string]any{"version": "v9.9.9"}, Time: time.Now(), URL: "/"})
	if got := svc.Received(); len(got) != 1 || got[0].Err != nil {
		t.Fatalf("the push: %+v", got)
	}
	if _, text := callAPI(t, ts, "GET", "/api/v1/me/push", "secret", ""); strings.TrimSpace(text) != "[]" {
		t.Fatalf("the subscription stayed: %s", text)
	}
	if !strings.Contains(log.String(), "push subscription removed: its push service knows it no more") {
		t.Fatalf("no line:\n%s", log.String())
	}
}
