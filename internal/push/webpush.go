// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package push

// Web Push: the message encrypted for its subscriber (RFC 8291, the
// content coding aes128gcm of RFC 8188), the server identified by its
// VAPID key (RFC 8292), and the POST to the push service (RFC 8030).

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/ecdsa"
	"crypto/hkdf"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// Subscription is where a message goes: the push service's endpoint and
// the subscriber's keys (RFC 8291), its P-256 public key uncompressed and
// its 16-byte authentication secret.
type Subscription struct {
	Endpoint string
	P256dh   []byte
	Auth     []byte
}

// recordSize is the record size of the encrypted body (RFC 8188): the
// message is one record.
const recordSize = 4096

// headerSize is the body's header: the salt, the record size, the key's
// length and the sender's public key.
const headerSize = 16 + 4 + 1 + 65

// MaxPayload is the longest message: a push service need take no body
// over 4096 bytes (RFC 8291 section 4), which holds the header, the
// message, its delimiter and AES-GCM's tag.
const MaxPayload = recordSize - headerSize - 1 - 16

// Encrypt encrypts a message for a subscriber, with a key pair and a
// salt of its own (RFC 8291).
func Encrypt(sub Subscription, plaintext []byte) ([]byte, error) {
	as, err := ecdh.P256().GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return nil, err
	}
	return encrypt(sub, plaintext, as, salt)
}

// encrypt is Encrypt with the sender's key pair and the salt given, as
// RFC 8291's example gives them.
func encrypt(sub Subscription, plaintext []byte, as *ecdh.PrivateKey, salt []byte) ([]byte, error) {
	switch {
	case len(plaintext) > MaxPayload:
		return nil, fmt.Errorf("push: a message of %d bytes, over %d", len(plaintext), MaxPayload)
	case len(sub.Auth) != 16:
		return nil, fmt.Errorf("push: an auth secret of %d bytes, want 16", len(sub.Auth))
	case len(salt) != 16:
		return nil, fmt.Errorf("push: a salt of %d bytes, want 16", len(salt))
	}
	ua, err := ecdh.P256().NewPublicKey(sub.P256dh)
	if err != nil {
		return nil, fmt.Errorf("push: the subscriber's key: %w", err)
	}
	shared, err := as.ECDH(ua)
	if err != nil {
		return nil, fmt.Errorf("push: %w", err)
	}
	asPublic := as.PublicKey().Bytes()
	// The input keying material from the shared secret and the auth
	// secret (RFC 8291 section 3.3), then the content encryption key and
	// the nonce from it and the salt (RFC 8188 section 2.2).
	prkKey, err := hkdf.Extract(sha256.New, shared, sub.Auth)
	if err != nil {
		return nil, err
	}
	keyInfo := "WebPush: info\x00" + string(sub.P256dh) + string(asPublic)
	ikm, err := hkdf.Expand(sha256.New, prkKey, keyInfo, 32)
	if err != nil {
		return nil, err
	}
	prk, err := hkdf.Extract(sha256.New, ikm, salt)
	if err != nil {
		return nil, err
	}
	cek, err := hkdf.Expand(sha256.New, prk, "Content-Encoding: aes128gcm\x00", 16)
	if err != nil {
		return nil, err
	}
	nonce, err := hkdf.Expand(sha256.New, prk, "Content-Encoding: nonce\x00", 12)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(cek)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	out := make([]byte, 0, headerSize+len(plaintext)+1+gcm.Overhead())
	out = append(out, salt...)
	out = binary.BigEndian.AppendUint32(out, recordSize)
	out = append(out, byte(len(asPublic)))
	out = append(out, asPublic...)
	// The last record ends with the delimiter 2, and no padding.
	record := append(append(make([]byte, 0, len(plaintext)+1), plaintext...), 2)
	return gcm.Seal(out, nonce, record, nil), nil
}

// VAPID is how the server identifies itself to a push service (RFC
// 8292): the key the subscription was made with, and how the push
// service reaches whoever runs the server.
type VAPID struct {
	Key *ecdsa.PrivateKey
	// Subject is a mailto: or https: URI, or the box's URN.
	Subject string
}

// tokenLifetime is how long a VAPID token holds; at most 24 hours.
const tokenLifetime = 12 * time.Hour

// token is a JWT for the push service of the origin aud, signed ES256,
// valid until exp (RFC 8292 section 2).
func (v VAPID) token(aud string, exp time.Time) (string, error) {
	if v.Key == nil {
		return "", errors.New("push: no VAPID key")
	}
	claims, err := json.Marshal(struct {
		Aud string `json:"aud"`
		Exp int64  `json:"exp"`
		Sub string `json:"sub,omitempty"`
	}{aud, exp.Unix(), v.Subject})
	if err != nil {
		return "", err
	}
	enc := base64.RawURLEncoding
	input := enc.EncodeToString([]byte(`{"typ":"JWT","alg":"ES256"}`)) + "." + enc.EncodeToString(claims)
	sum := sha256.Sum256([]byte(input))
	r, s, err := ecdsa.Sign(rand.Reader, v.Key, sum[:])
	if err != nil {
		return "", err
	}
	sig := make([]byte, 64)
	r.FillBytes(sig[:32])
	s.FillBytes(sig[32:])
	return input + "." + enc.EncodeToString(sig), nil
}

// authorization is the Authorization header for the push service of
// aud: the token and the public key (RFC 8292 section 3).
func (v VAPID) authorization(aud string, exp time.Time) (string, error) {
	t, err := v.token(aud, exp)
	if err != nil {
		return "", err
	}
	pub, err := v.Key.PublicKey.Bytes()
	if err != nil {
		return "", err
	}
	return "vapid t=" + t + ", k=" + base64.RawURLEncoding.EncodeToString(pub), nil
}

// Message is how a push service is to treat a message (RFC 8030
// section 5).
type Message struct {
	// TTL is how long the push service keeps a message it could not
	// deliver yet; 0 drops it then.
	TTL time.Duration
	// Urgency is very-low, low, normal or high; "" says none.
	Urgency string
	// Topic is the message's topic: a newer one of the same topic
	// replaces one not delivered yet; "" for none.
	Topic string
}

// Result is how a message went.
type Result struct {
	// Status is the push service's last answer, 0 for none.
	Status int
	// Attempts is how many times the message was sent.
	Attempts int
	// Gone says that the push service knows the subscription no more
	// (404 or 410): it is to be removed.
	Gone bool
	// Err is nil for a message the push service took.
	Err error
}

// defaultRetry are the waits before the tries after a 429 or a 5xx:
// three more over a minute.
var defaultRetry = []time.Duration{5 * time.Second, 15 * time.Second, 40 * time.Second}

// Sender sends messages by Web Push.
type Sender struct {
	VAPID VAPID
	// Client sends the requests; nil is one of 30 s a request that
	// follows no redirect.
	Client *http.Client
	// Retry are the waits before trying again after a 429 or a 5xx; nil
	// is 5, 15 and 40 seconds.
	Retry []time.Duration
	// now is the time of the token's expiry; nil is the clock's.
	now func() time.Time
}

// defaultClient follows no redirect: a push service answers where it
// was asked, and a subscription's endpoint is no way to send the
// server's requests elsewhere.
var defaultClient = &http.Client{
	Timeout:       30 * time.Second,
	CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
}

// Send encrypts a message for a subscription and posts it to its push
// service, trying again after a 429 or a 5xx, until ctx ends.
func (s *Sender) Send(ctx context.Context, sub Subscription, payload []byte, m Message) Result {
	u, err := url.Parse(sub.Endpoint)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" {
		return Result{Err: fmt.Errorf("push: the endpoint is no http or https URL")}
	}
	body, err := Encrypt(sub, payload)
	if err != nil {
		return Result{Err: err}
	}
	now := time.Now
	if s.now != nil {
		now = s.now
	}
	client := s.Client
	if client == nil {
		client = defaultClient
	}
	retry := s.Retry
	if retry == nil {
		retry = defaultRetry
	}
	aud := u.Scheme + "://" + u.Host
	var r Result
	for {
		auth, err := s.VAPID.authorization(aud, now().Add(tokenLifetime))
		if err != nil {
			return Result{Err: err, Attempts: r.Attempts}
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, sub.Endpoint, bytes.NewReader(body))
		if err != nil {
			return Result{Err: err, Attempts: r.Attempts}
		}
		req.Header.Set("Content-Type", "application/octet-stream")
		req.Header.Set("Content-Encoding", "aes128gcm")
		req.Header.Set("TTL", strconv.Itoa(int(m.TTL/time.Second)))
		if m.Urgency != "" {
			req.Header.Set("Urgency", m.Urgency)
		}
		if m.Topic != "" {
			req.Header.Set("Topic", m.Topic)
		}
		req.Header.Set("Authorization", auth)
		r.Attempts++
		resp, err := client.Do(req)
		if err != nil {
			return Result{Err: fmt.Errorf("push: %w", err), Attempts: r.Attempts}
		}
		io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
		resp.Body.Close()
		r.Status = resp.StatusCode
		switch {
		case r.Status >= 200 && r.Status < 300:
			return r
		case r.Status == http.StatusNotFound || r.Status == http.StatusGone:
			r.Gone = true
			r.Err = fmt.Errorf("push: the push service knows the subscription no more (%d)", r.Status)
			return r
		case (r.Status == http.StatusTooManyRequests || r.Status >= 500) && r.Attempts <= len(retry):
			t := time.NewTimer(retry[r.Attempts-1])
			select {
			case <-ctx.Done():
				t.Stop()
				r.Err = ctx.Err()
				return r
			case <-t.C:
			}
		default:
			r.Err = fmt.Errorf("push: the push service answered %d", r.Status)
			return r
		}
	}
}
