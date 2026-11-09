// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

// Notifications (docs/api-v1.md, "Notifications"): the rules of
// internal/notify run over the Cubes' events for every user who has a
// push subscription, with the user's notify preference; a browser or an
// app subscribes under /api/v1/me/push with the server's VAPID key from
// GET /api/v1/push/vapid. What a rule decides is logged; its delivery
// by Web Push is internal/push's.

import (
	"context"
	"crypto/ecdh"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"go.metaworx.ch/massimo/internal/notify"
	"go.metaworx.ch/massimo/internal/store"
	"go.metaworx.ch/massimo/internal/view"
)

// The limits of a subscription's fields.
const (
	maxEndpoint  = 2048
	maxPushLabel = 100
)

// vapidSetting is the VAPID key pair in the store's settings: the P-256
// private scalar and the uncompressed public point, base64url.
type vapidSetting struct {
	Private   string    `json:"private"`
	Public    string    `json:"public"`
	CreatedAt time.Time `json:"created_at"`
}

// vapidKey returns the server's VAPID key pair, made and kept at its
// first use.
func (s *Server) vapidKey(st *store.Store, r *http.Request) (*ecdsa.PrivateKey, []byte, error) {
	s.vapidMu.Lock()
	defer s.vapidMu.Unlock()
	raw, ok, err := st.Setting(r.Context(), store.VAPIDKey)
	if err != nil {
		return nil, nil, err
	}
	if ok {
		var v vapidSetting
		if err := json.Unmarshal(raw, &v); err != nil {
			return nil, nil, fmt.Errorf("%s: %w", store.VAPIDKey, err)
		}
		d, err := base64.RawURLEncoding.DecodeString(v.Private)
		if err != nil {
			return nil, nil, fmt.Errorf("%s: %w", store.VAPIDKey, err)
		}
		key, err := ecdsa.ParseRawPrivateKey(elliptic.P256(), d)
		if err != nil {
			return nil, nil, fmt.Errorf("%s: %w", store.VAPIDKey, err)
		}
		pub, err := key.PublicKey.Bytes()
		return key, pub, err
	}
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, err
	}
	d, err := key.Bytes()
	if err != nil {
		return nil, nil, err
	}
	pub, err := key.PublicKey.Bytes()
	if err != nil {
		return nil, nil, err
	}
	v := vapidSetting{Private: base64.RawURLEncoding.EncodeToString(d), Public: base64.RawURLEncoding.EncodeToString(pub), CreatedAt: time.Now().UTC()}
	if err := st.SetSetting(r.Context(), store.VAPIDKey, v); err != nil {
		return nil, nil, err
	}
	s.log.Info("VAPID key created", "setting", store.VAPIDKey)
	return key, pub, nil
}

// getVAPID handles GET /api/v1/push/vapid: the public key a browser
// subscribes with (pushManager.subscribe's applicationServerKey).
func (s *Server) getVAPID(w http.ResponseWriter, r *http.Request) error {
	st, err := s.cubeStore()
	if err != nil {
		return err
	}
	_, pub, err := s.vapidKey(st, r)
	if err != nil {
		return fail(http.StatusInternalServerError, "%v", err)
	}
	writeJSON(w, http.StatusOK, map[string]string{"key": base64.RawURLEncoding.EncodeToString(pub)})
	return nil
}

// pushView is a subscription as the API shows it, without its keys.
type pushView struct {
	ID        int64      `json:"id"`
	Endpoint  string     `json:"endpoint"`
	Label     string     `json:"label"`
	CreatedAt time.Time  `json:"created_at"`
	LastOKAt  *time.Time `json:"last_ok_at,omitempty"`
	Failures  int        `json:"failures"`
	// Current says it was made with the caller's session or key.
	Current bool `json:"current"`
}

func viewPush(p store.PushSubscription, token int64) pushView {
	v := pushView{ID: p.ID, Endpoint: p.Endpoint, Label: p.Label, CreatedAt: p.CreatedAt, Failures: p.Failures, Current: p.TokenID == token}
	if !p.LastOKAt.IsZero() {
		v.LastOKAt = &p.LastOKAt
	}
	return v
}

// listPush handles GET /api/v1/me/push: the caller's subscriptions.
func (s *Server) listPush(w http.ResponseWriter, r *http.Request) error {
	st, p, err := s.prefUser(r)
	if err != nil {
		return err
	}
	list, err := st.PushSubscriptions(r.Context(), p.user.ID)
	if err != nil {
		return fail(http.StatusInternalServerError, "%v", err)
	}
	out := []pushView{}
	for _, sub := range list {
		out = append(out, viewPush(sub, p.token.ID))
	}
	writeJSON(w, http.StatusOK, out)
	return nil
}

// pushRequest is a browser's PushSubscription.toJSON() with a label.
type pushRequest struct {
	Endpoint string `json:"endpoint"`
	Keys     struct {
		P256dh string `json:"p256dh"`
		Auth   string `json:"auth"`
	} `json:"keys"`
	Label string `json:"label"`
}

// decodeKey reads a subscriber's key, base64url with or without its
// padding, of n bytes.
func decodeKey(name, v string, n int) (string, error) {
	b, err := base64.RawURLEncoding.DecodeString(strings.TrimRight(v, "="))
	if err != nil || len(b) != n {
		return "", fail(http.StatusBadRequest, "keys.%s: want %d bytes, base64url", name, n)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// onP256 says whether b is an uncompressed point of P-256, as a
// subscriber's key for the message's encryption is (RFC 8291).
func onP256(b []byte) bool {
	_, err := ecdh.P256().NewPublicKey(b)
	return err == nil
}

// addPush handles POST /api/v1/me/push: a subscription for the caller,
// made with the caller's session or key; the same endpoint again
// updates its row.
func (s *Server) addPush(w http.ResponseWriter, r *http.Request) error {
	st, p, err := s.prefUser(r)
	if err != nil {
		return err
	}
	var req pushRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10)).Decode(&req); err != nil {
		return fail(http.StatusBadRequest, "bad body: %v", err)
	}
	u, err := url.Parse(req.Endpoint)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" || len(req.Endpoint) > maxEndpoint {
		return fail(http.StatusBadRequest, "endpoint: want the push service's http or https URL, at most %d characters", maxEndpoint)
	}
	p256dh, err := decodeKey("p256dh", req.Keys.P256dh, 65)
	if err != nil {
		return err
	}
	if b, _ := base64.RawURLEncoding.DecodeString(p256dh); !onP256(b) {
		return fail(http.StatusBadRequest, "keys.p256dh: want an uncompressed point of P-256")
	}
	auth, err := decodeKey("auth", req.Keys.Auth, 16)
	if err != nil {
		return err
	}
	label := strings.TrimSpace(req.Label)
	if label == "" {
		label = strings.TrimSpace(r.UserAgent())
	}
	for utf8.RuneCountInString(label) > maxPushLabel {
		_, size := utf8.DecodeLastRuneInString(label)
		label = label[:len(label)-size]
	}
	sub, created, err := st.AddPushSubscription(r.Context(), store.PushSubscription{UserID: p.user.ID, TokenID: p.token.ID, Endpoint: req.Endpoint, P256dh: p256dh, Auth: auth, Label: label})
	if err != nil {
		return fail(http.StatusInternalServerError, "%v", err)
	}
	auditTarget(r, "push:"+strconv.FormatInt(sub.ID, 10))
	s.refreshNotify()
	status := http.StatusOK
	if created {
		status = http.StatusCreated
	}
	writeJSON(w, status, viewPush(sub, p.token.ID))
	return nil
}

// deletePush handles DELETE /api/v1/me/push/{id}: one of the caller's
// subscriptions removed.
func (s *Server) deletePush(w http.ResponseWriter, r *http.Request) error {
	st, p, err := s.prefUser(r)
	if err != nil {
		return err
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		return fail(http.StatusBadRequest, "bad subscription id %q", r.PathValue("id"))
	}
	auditTarget(r, "push:"+strconv.FormatInt(id, 10))
	switch err := st.DeletePushSubscription(r.Context(), p.user.ID, id); {
	case errors.Is(err, store.ErrNoPushSubscription):
		return fail(http.StatusNotFound, "no subscription %d of yours", id)
	case err != nil:
		return fail(http.StatusInternalServerError, "%v", err)
	}
	s.refreshNotify()
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	return nil
}

// newNotify makes the rules of a server with a store.
func (s *Server) newNotify() *notify.Sink {
	return notify.New(notify.Options{Users: s.notifyUsers, Deliver: s.deliverNotification, Devices: s.notifyDevices, Clock: s.opts.notifyClock, Logger: s.log})
}

// refreshNotify tells the rules that a subscription or a notify
// preference changed.
func (s *Server) refreshNotify() {
	if s.notify != nil {
		s.notify.Refresh()
	}
}

// pushNotify hands an event to the rules, where the server has them.
func (s *Server) pushNotify(ev notify.Event) {
	if s.notify != nil {
		if ev.Time.IsZero() {
			ev.Time = time.Now()
		}
		s.notify.Push(ev)
	}
}

// notifyEvent hands a Cube's event to the rules: its connection, a
// device's state, pairing, a device forgotten.
func (s *Server) notifyEvent(rt *runtime, ev event) {
	switch ev.Kind {
	case "connected", "disconnected", "state", "paired", "device_forgotten":
	default:
		return
	}
	n := notify.Event{Kind: ev.Kind, Cube: rt.cubeSerial(), Device: ev.Device, Serial: ev.Serial, Time: ev.Time}
	if dv, ok := ev.State.(view.Device); ok {
		n.DeviceName, n.Room = dv.Name, int(dv.RoomID)
		n.RoomName = roomName(rt, dv.RoomID)
		if ev.Kind == "state" && dv.State != nil {
			st := dv.State
			n.State = &notify.State{WindowOpen: st.WindowOpen, BatteryLow: st.BatteryLow, Error: st.Error, LinkError: st.LinkError, ActualTemp: st.ActualTemp}
		}
	}
	if ev.Paired != nil {
		n.Serial = ev.Paired.Serial
		n.Values = map[string]any{"type": ev.Paired.Type}
	}
	s.pushNotify(n)
}

// roomName is a room's name in the Cube's metadata, "" for none.
func roomName(rt *runtime, id uint8) string {
	if md := rt.client.Snapshot().Metadata(); md != nil {
		for _, r := range md.Rooms {
			if r.ID == id {
				return r.Name
			}
		}
	}
	return ""
}

// notifyUsers are the users with a push subscription that may be sent
// to, each with the notify preference; one that does not parse is
// logged, and the defaults apply.
func (s *Server) notifyUsers(ctx context.Context) ([]notify.User, error) {
	st := s.opts.Store
	subs, err := st.PushSubscriptions(ctx, 0)
	if err != nil {
		return nil, err
	}
	var out []notify.User
	seen := map[int64]bool{}
	for _, sub := range subs {
		if seen[sub.UserID] {
			continue
		}
		seen[sub.UserID] = true
		prefs := notify.Defaults()
		if pr, ok, err := st.Preference(ctx, sub.UserID, notify.PrefKey); err != nil {
			return nil, err
		} else if ok {
			if p, err := notify.ParsePrefs(pr.Value); err != nil {
				s.log.Warn("notify preference ignored, the defaults apply", "user", sub.UserID, "error", err)
			} else {
				prefs = p
			}
		}
		out = append(out, notify.User{ID: sub.UserID, Prefs: prefs})
	}
	return out, nil
}

// notifyDevices are every Cube's devices for the stale scan.
func (s *Server) notifyDevices() []notify.Device {
	var out []notify.Device
	for _, rt := range s.runtimes() {
		snap := rt.client.Snapshot()
		for _, d := range view.Devices(snap, rt.hidden(), s.StaleAfter()) {
			out = append(out, notify.Device{Cube: rt.cubeSerial(), Device: d.RFAddress, Serial: d.Serial, DeviceName: d.Name, Room: int(d.RoomID), RoomName: roomName(rt, d.RoomID), Stale: d.Stale})
		}
	}
	return out
}

// deliverNotification takes what a rule decided: logged, for now the
// only delivery.
func (s *Server) deliverNotification(userID int64, n notify.Notification) {
	attrs := []any{"user", userID, "kind", n.Kind}
	if s.opts.Store != nil {
		if u, ok, err := s.opts.Store.UserByID(context.Background(), userID); err == nil && ok {
			attrs[1] = u.Name
		}
	}
	for _, kv := range []struct{ k, v string }{{"cube", n.Cube}, {"device", n.Device}, {"serial", n.Serial}, {"room", n.RoomName}} {
		if kv.v != "" {
			attrs = append(attrs, kv.k, kv.v)
		}
	}
	s.log.Info("notification", attrs...)
}
