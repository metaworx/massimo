// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package push

import (
	"bytes"
	"context"
	"crypto/ecdh"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"go.metaworx.ch/massimo/internal/notify"
	"go.metaworx.ch/massimo/internal/push/pushtest"
	"go.metaworx.ch/massimo/internal/store"
)

func b64(t *testing.T, s string) []byte {
	t.Helper()
	b, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// RFC 8291's Appendix A: its keys, salt and message give its body, byte
// for byte, and the subscriber's side reads the message back.
func TestRFC8291Vector(t *testing.T) {
	asPriv, err := ecdh.P256().NewPrivateKey(b64(t, "yfWPiYE-n46HLnH0KqZOF1fJJU3MYrct3AELtAQ-oRw"))
	if err != nil {
		t.Fatal(err)
	}
	if got := base64.RawURLEncoding.EncodeToString(asPriv.PublicKey().Bytes()); got != "BP4z9KsN6nGRTbVYI_c7VJSPQTBtkgcy27mlmlMoZIIgDll6e3vCYLocInmYWAmS6TlzAC8wEqKK6PBru3jl7A8" {
		t.Fatalf("the application server's public key: %s", got)
	}
	uaPriv, err := ecdh.P256().NewPrivateKey(b64(t, "q1dXpw3UpT5VOmu_cf_v6ih07Aems3njxI-JWgLcM94"))
	if err != nil {
		t.Fatal(err)
	}
	sub := Subscription{
		Endpoint: "https://push.example.net/push/JzLQ3raZJfFBR0aqvOMsLrt54w4rJUsV",
		P256dh:   b64(t, "BCVxsr7N_eNgVRqvHtD0zTZsEc6-VV-JvLexhqUzORcxaOzi6-AYWXvTBHm4bjyPjs7Vd8pZGH6SRpkNtoIAiw4"),
		Auth:     b64(t, "BTBZMqHH6r4Tts7J_aSIgg"),
	}
	if !bytes.Equal(uaPriv.PublicKey().Bytes(), sub.P256dh) {
		t.Fatal("the user agent's keys do not match")
	}
	plaintext := []byte("When I grow up, I want to be a watermelon")
	body, err := encrypt(sub, plaintext, asPriv, b64(t, "DGv6ra1nlYgDCS1FRnbzlw"))
	if err != nil {
		t.Fatal(err)
	}
	const want = "DGv6ra1nlYgDCS1FRnbzlwAAEABBBP4z9KsN6nGRTbVYI_c7VJSPQTBtkgcy27mlmlMoZIIgDll6e3vCYLocInmYWAmS6TlzAC8wEqKK6PBru3jl7A_yl95bQpu6cVPTpK4Mqgkf1CXztLVBSt2Ks3oZwbuwXPXLWyouBWLVWGNWQexSgSxsj_Qulcy4a-fN"
	if got := base64.RawURLEncoding.EncodeToString(body); got != want {
		t.Fatalf("body\n got %s\nwant %s", got, want)
	}
	back, err := pushtest.Decrypt(uaPriv, sub.Auth, body)
	if err != nil || !bytes.Equal(back, plaintext) {
		t.Fatalf("decrypted %q, %v", back, err)
	}
}

func TestEncryptLimits(t *testing.T) {
	ua, _ := ecdh.P256().GenerateKey(rand.Reader)
	sub := Subscription{P256dh: ua.PublicKey().Bytes(), Auth: make([]byte, 16)}
	body, err := Encrypt(sub, bytes.Repeat([]byte("x"), MaxPayload))
	if err != nil || len(body) != 4096 {
		t.Fatalf("the longest message: %d bytes, %v", len(body), err)
	}
	if _, err := Encrypt(sub, make([]byte, MaxPayload+1)); err == nil {
		t.Fatal("a message over the limit encrypted")
	}
	if _, err := Encrypt(Subscription{P256dh: sub.P256dh, Auth: make([]byte, 15)}, nil); err == nil {
		t.Fatal("a short auth secret taken")
	}
	if _, err := Encrypt(Subscription{P256dh: []byte{4, 1, 2}, Auth: sub.Auth}, nil); err == nil {
		t.Fatal("a key off the curve taken")
	}
}

func TestVAPIDToken(t *testing.T) {
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	v := VAPID{Key: key, Subject: "urn:uuid:00000000-0000-7000-8000-000000000000"}
	now := time.Now()
	h, err := v.authorization("https://push.example.net", now.Add(tokenLifetime))
	if err != nil {
		t.Fatal(err)
	}
	claims, err := pushtest.CheckVAPID(h, "https://push.example.net", &key.PublicKey, now)
	if err != nil {
		t.Fatal(err)
	}
	if claims["sub"] != v.Subject {
		t.Fatalf("claims %v", claims)
	}
	if _, err := (VAPID{}).token("https://push.example.net", now); err == nil {
		t.Fatal("a token without a key")
	}
}

// row is a store row for a subscription at the service.
func row(s *pushtest.Service, path string, id, user int64) store.PushSubscription {
	p256dh, auth := s.Keys()
	return store.PushSubscription{ID: id, UserID: user, Endpoint: s.URL + path, P256dh: p256dh, Auth: auth, CreatedAt: time.Now()}
}

// subs is a store of subscriptions for the tests.
type subs struct {
	mu      sync.Mutex
	rows    []store.PushSubscription
	results map[int64][]bool
	removed []int64
}

func (s *subs) PushSubscriptions(_ context.Context, userID int64) ([]store.PushSubscription, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []store.PushSubscription
	for _, r := range s.rows {
		if userID == 0 || r.UserID == userID {
			out = append(out, r)
		}
	}
	return out, nil
}

func (s *subs) NotePushResult(_ context.Context, id int64, ok bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.results == nil {
		s.results = map[int64][]bool{}
	}
	s.results[id] = append(s.results[id], ok)
	return nil
}

func (s *subs) RemovePushSubscription(_ context.Context, id int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.removed = append(s.removed, id)
	return nil
}

func webPush(st *subs, key *ecdsa.PrivateKey) (*WebPush, *int) {
	removed := 0
	return &WebPush{
		Store: st,
		VAPID: func(context.Context) (VAPID, error) {
			return VAPID{Key: key, Subject: "urn:uuid:00000000-0000-7000-8000-000000000000"}, nil
		},
		Retry:   []time.Duration{time.Millisecond, time.Millisecond, time.Millisecond},
		Removed: func() { removed++ },
	}, &removed
}

var windowOpen = notify.Notification{Kind: notify.KindWindowOpen, Cube: "KEQ0000000", Device: "0a0102", Room: 3, RoomName: "Bath", DeviceName: "Window", Values: map[string]any{"minutes": 30}, URL: "/"}

// A notification reaches every subscription of its user, and no other's:
// encrypted for each, signed with the server's key, with its TTL,
// urgency and topic.
func TestDeliverToEverySubscriptionOfTheUser(t *testing.T) {
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	a, b, other := pushtest.New(t, &key.PublicKey), pushtest.New(t, &key.PublicKey), pushtest.New(t, &key.PublicKey)
	st := &subs{rows: []store.PushSubscription{row(a, "/a", 1, 7), row(b, "/b", 2, 7), row(other, "/other", 3, 8)}}
	w, removed := webPush(st, key)
	w.Deliver(t.Context(), 7, windowOpen)
	for name, s := range map[string]*pushtest.Service{"a": a, "b": b} {
		got := s.Received()
		if len(got) != 1 || got[0].Err != nil {
			t.Fatalf("%s: %+v", name, got)
		}
		var n notify.Notification
		if err := json.Unmarshal(got[0].Message, &n); err != nil || n.Kind != notify.KindWindowOpen || n.Device != "0a0102" || n.RoomName != "Bath" {
			t.Fatalf("%s: %s %v", name, got[0].Message, err)
		}
		h := got[0].Header
		if h.Get("TTL") != "86400" || h.Get("Urgency") != "normal" || h.Get("Topic") != "window_open-0a0102" {
			t.Fatalf("%s: TTL %q, Urgency %q, Topic %q", name, h.Get("TTL"), h.Get("Urgency"), h.Get("Topic"))
		}
	}
	if got := other.Received(); len(got) != 0 {
		t.Fatalf("another user's subscription got %d", len(got))
	}
	if fmt.Sprint(st.results) != "map[1:[true] 2:[true]]" || len(st.removed) != 0 || *removed != 0 {
		t.Fatalf("results %v, removed %v", st.results, st.removed)
	}
}

// A subscription its push service knows no more is removed, and the
// rules are told.
func TestDeliverRemovesGoneSubscription(t *testing.T) {
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	for _, status := range []int{http.StatusGone, http.StatusNotFound} {
		s := pushtest.New(t, &key.PublicKey, status)
		st := &subs{rows: []store.PushSubscription{row(s, "/gone", 4, 7)}}
		w, removed := webPush(st, key)
		w.Deliver(t.Context(), 7, windowOpen)
		if len(s.Received()) != 1 || fmt.Sprint(st.removed) != "[4]" || *removed != 1 || len(st.results) != 0 {
			t.Fatalf("%d: sent %d, removed %v (told %d), results %v", status, len(s.Received()), st.removed, *removed, st.results)
		}
	}
}

// A 503 and a 429 are tried again, three more times at most; a 400 is
// not.
func TestDeliverRetries(t *testing.T) {
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	for _, c := range []struct {
		statuses []int
		sent     int
		ok       bool
	}{
		{[]int{503}, 2, true},
		{[]int{429, 500, 502}, 4, true},
		{[]int{503, 503, 503, 503}, 4, false},
		{[]int{400}, 1, false},
	} {
		s := pushtest.New(t, &key.PublicKey, c.statuses...)
		st := &subs{rows: []store.PushSubscription{row(s, "/retry", 5, 7)}}
		w, _ := webPush(st, key)
		w.Deliver(t.Context(), 7, windowOpen)
		if len(s.Received()) != c.sent || fmt.Sprint(st.results[5]) != fmt.Sprint([]bool{c.ok}) || len(st.removed) != 0 {
			t.Fatalf("%v: sent %d, results %v, removed %v", c.statuses, len(s.Received()), st.results, st.removed)
		}
	}
}

// A subscription failing for seven days is removed; one that failed
// once after a delivery is not.
func TestDeliverRemovesFailingSubscription(t *testing.T) {
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	now := time.Now()
	for _, c := range []struct {
		name            string
		created, lastOK time.Time
		removed         bool
	}{
		{"never delivered, made eight days ago", now.Add(-8 * 24 * time.Hour), time.Time{}, true},
		{"delivered eight days ago", now.Add(-30 * 24 * time.Hour), now.Add(-8 * 24 * time.Hour), true},
		{"delivered yesterday", now.Add(-30 * 24 * time.Hour), now.Add(-24 * time.Hour), false},
		{"made today", now, time.Time{}, false},
	} {
		r := row(pushtest.New(t, &key.PublicKey, 400), "/failing", 6, 7)
		r.CreatedAt, r.LastOKAt = c.created, c.lastOK
		st := &subs{rows: []store.PushSubscription{r}}
		w, removed := webPush(st, key)
		w.Deliver(t.Context(), 7, windowOpen)
		if fmt.Sprint(st.results[6]) != "[false]" || (len(st.removed) == 1) != c.removed || (*removed == 1) != c.removed {
			t.Fatalf("%s: results %v, removed %v", c.name, st.results, st.removed)
		}
	}
}

// A delivery the server's stop cuts off counts nothing.
func TestDeliverStopped(t *testing.T) {
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	st := &subs{rows: []store.PushSubscription{row(pushtest.New(t, &key.PublicKey, 503), "/stop", 9, 7)}}
	w, _ := webPush(st, key)
	w.Retry = []time.Duration{time.Hour}
	ctx, cancel := context.WithCancel(t.Context())
	time.AfterFunc(50*time.Millisecond, cancel)
	w.Deliver(ctx, 7, windowOpen)
	if len(st.results) != 0 || len(st.removed) != 0 {
		t.Fatalf("results %v, removed %v", st.results, st.removed)
	}
}

func TestMessageOf(t *testing.T) {
	for _, c := range []struct {
		n       notify.Notification
		urgency string
		topic   string
	}{
		{notify.Notification{Kind: notify.KindFrost, Device: "0a0102"}, "high", "frost-0a0102"},
		{notify.Notification{Kind: notify.KindCubeOffline, Cube: "KEQ0000000"}, "high", "cube_offline-KEQ0000000"},
		{notify.Notification{Kind: notify.KindUpdate, Values: map[string]any{"version": "v1.2.3"}}, "normal", "update"},
		{notify.Notification{Kind: notify.KindDeviceStale, Device: "0a0102.x/y"}, "normal", "device_stale-0a0102xy"},
		{notify.Notification{Kind: notify.KindCubeFound, Cube: strings.Repeat("K", 40)}, "normal", "cube_found-" + strings.Repeat("K", 21)},
	} {
		m := MessageOf(c.n)
		if m.TTL != 24*time.Hour || m.Urgency != c.urgency || m.Topic != c.topic {
			t.Errorf("%s: %+v", c.n.Kind, m)
		}
	}
}

// A redirect is not followed: the endpoint is not a way to send the
// server's requests elsewhere.
func TestSendFollowsNoRedirect(t *testing.T) {
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	target := pushtest.New(t, &key.PublicKey)
	redirect := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL+"/elsewhere", http.StatusTemporaryRedirect)
	}))
	defer redirect.Close()
	k, a := target.Keys()
	p256dh, _ := base64.RawURLEncoding.DecodeString(k)
	auth, _ := base64.RawURLEncoding.DecodeString(a)
	s := Sender{VAPID: VAPID{Key: key}}
	r := s.Send(t.Context(), Subscription{Endpoint: redirect.URL + "/x", P256dh: p256dh, Auth: auth}, []byte("{}"), Message{})
	if r.Err == nil || r.Status != http.StatusTemporaryRedirect || len(target.Received()) != 0 {
		t.Fatalf("%+v, target got %d", r, len(target.Received()))
	}
}

func TestLocal(t *testing.T) {
	var got []string
	var d Deliverer = Local(func(user int64, n notify.Notification) { got = append(got, fmt.Sprintf("%d %s", user, n.Kind)) })
	d.Deliver(t.Context(), 7, windowOpen)
	if fmt.Sprint(got) != "[7 window_open]" {
		t.Fatal(got)
	}
}
