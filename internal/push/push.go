// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

// Package push takes what the rules of internal/notify decided to where
// a user is told: by Web Push to the browsers and apps the user
// subscribed (WebPush), which UnifiedPush takes as well, or to a Go
// function, for a server embedded in an app (Local).
//
// A message is the notification as JSON, encrypted for each subscriber
// (RFC 8291) and signed with the server's VAPID key (RFC 8292): the
// push service reads none of it.
package push

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/url"
	"time"

	"go.metaworx.ch/massimo/internal/notify"
	"go.metaworx.ch/massimo/internal/store"
)

// Deliverer takes a user's notification to where it goes; it may wait
// for the network, and ends when ctx does.
type Deliverer interface {
	Deliver(ctx context.Context, userID int64, n notify.Notification)
}

// Local hands every notification to a Go function: the app that embeds
// the server shows it itself.
type Local func(userID int64, n notify.Notification)

// Deliver calls the function.
func (l Local) Deliver(_ context.Context, userID int64, n notify.Notification) { l(userID, n) }

// Subscriptions is what WebPush reads and writes of the store.
type Subscriptions interface {
	PushSubscriptions(ctx context.Context, userID int64) ([]store.PushSubscription, error)
	NotePushResult(ctx context.Context, id int64, ok bool) error
	RemovePushSubscription(ctx context.Context, id int64) error
}

// TTL is how long a push service keeps a notification it could not
// deliver yet: a day.
const TTL = 24 * time.Hour

// FailingFor is how long a subscription may fail before it is removed.
const FailingFor = 7 * 24 * time.Hour

// WebPush delivers a notification to every subscription of its user
// that may be sent to, and notes how each went: one its push service
// knows no more (404, 410) is removed, as is one failing for seven days.
type WebPush struct {
	Store Subscriptions
	// VAPID is the server's key and subject, read at each delivery.
	VAPID func(ctx context.Context) (VAPID, error)
	// Client and Retry are the Sender's; nil for its defaults.
	Client *http.Client
	Retry  []time.Duration
	// Removed is told that a subscription was removed, so that the
	// rules read their users again.
	Removed func()
	Logger  *slog.Logger
	// now is the time; nil is the clock's.
	now func() time.Time
}

// Deliver sends the notification to the user's subscriptions, one after
// the other.
func (w *WebPush) Deliver(ctx context.Context, userID int64, n notify.Notification) {
	log := w.Logger
	if log == nil {
		log = slog.Default()
	}
	all, err := w.Store.PushSubscriptions(ctx, 0)
	if err != nil {
		log.Warn("push: the subscriptions not read; the notification is not sent", "user", userID, "kind", n.Kind, "error", err)
		return
	}
	var subs []store.PushSubscription
	for _, s := range all {
		if s.UserID == userID {
			subs = append(subs, s)
		}
	}
	if len(subs) == 0 {
		return
	}
	v, err := w.VAPID(ctx)
	if err != nil {
		log.Warn("push: no VAPID key; the notification is not sent", "user", userID, "kind", n.Kind, "error", err)
		return
	}
	payload, err := json.Marshal(n)
	if err != nil {
		log.Warn("push: the notification not encoded", "user", userID, "kind", n.Kind, "error", err)
		return
	}
	now := time.Now
	if w.now != nil {
		now = w.now
	}
	sender := Sender{VAPID: v, Client: w.Client, Retry: w.Retry, now: w.now}
	msg := MessageOf(n)
	removed := false
	for _, s := range subs {
		p256dh, err1 := base64.RawURLEncoding.DecodeString(s.P256dh)
		auth, err2 := base64.RawURLEncoding.DecodeString(s.Auth)
		if err1 != nil || err2 != nil {
			log.Warn("push: a subscription's keys not read", "user", userID, "subscription", s.ID)
			continue
		}
		r := sender.Send(ctx, Subscription{Endpoint: s.Endpoint, P256dh: p256dh, Auth: auth}, payload, msg)
		if ctx.Err() != nil {
			return // the server stops: nothing is counted
		}
		// The endpoint's path is the subscriber's capability: the log
		// names its host alone.
		attrs := []any{"user", userID, "subscription", s.ID, "kind", n.Kind, "host", host(s.Endpoint)}
		if r.Status != 0 {
			attrs = append(attrs, "status", r.Status)
		}
		switch {
		case r.Err == nil:
			log.Debug("push delivered", attrs...)
			if err := w.Store.NotePushResult(ctx, s.ID, true); err != nil {
				log.Warn("push: the delivery not noted", "subscription", s.ID, "error", err)
			}
		case r.Gone:
			log.Info("push subscription removed: its push service knows it no more", attrs...)
			if err := w.Store.RemovePushSubscription(ctx, s.ID); err != nil {
				log.Warn("push: the subscription not removed", "subscription", s.ID, "error", err)
			}
			removed = true
		default:
			log.Warn("push not delivered", append(attrs, "attempts", r.Attempts, "error", r.Err)...)
			if err := w.Store.NotePushResult(ctx, s.ID, false); err != nil {
				log.Warn("push: the failure not noted", "subscription", s.ID, "error", err)
			}
			since := s.LastOKAt
			if since.IsZero() {
				since = s.CreatedAt
			}
			if !since.IsZero() && now().Sub(since) >= FailingFor {
				log.Info("push subscription removed: failing for seven days", attrs...)
				if err := w.Store.RemovePushSubscription(ctx, s.ID); err != nil {
					log.Warn("push: the subscription not removed", "subscription", s.ID, "error", err)
				}
				removed = true
			}
		}
	}
	if removed && w.Removed != nil {
		w.Removed()
	}
}

// MessageOf is how a push service is to treat a notification: kept a
// day, urgent for frost and a Cube offline, and replacing an undelivered
// one of the same kind and device or Cube.
func MessageOf(n notify.Notification) Message {
	m := Message{TTL: TTL, Urgency: "normal", Topic: topic(n)}
	switch n.Kind {
	case notify.KindFrost, notify.KindCubeOffline:
		m.Urgency = "high"
	}
	return m
}

// topic is a notification's Topic: its kind and its device's radio
// address or its Cube's serial, in the base64url alphabet and at most
// 32 characters (RFC 8030 section 5.4).
func topic(n notify.Notification) string {
	t := n.Kind
	if n.Device != "" {
		t += "-" + n.Device
	} else if n.Cube != "" {
		t += "-" + n.Cube
	}
	out := make([]byte, 0, 32)
	for i := 0; i < len(t) && len(out) < 32; i++ {
		c := t[i]
		if c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-' || c == '_' {
			out = append(out, c)
		}
	}
	return string(out)
}

// host is an endpoint's host, for the log.
func host(endpoint string) string {
	if u, err := url.Parse(endpoint); err == nil {
		return u.Host
	}
	return ""
}
