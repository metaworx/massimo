// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

// Package pushtest is a push service for the tests: it checks each
// message's VAPID token as a push service does, reads the message as its
// subscriber (RFC 8291), and answers with the statuses it is given.
package pushtest

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/hkdf"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// Service is a push service with one subscriber's keys.
type Service struct {
	// URL is the service's origin; an endpoint is a path under it.
	URL string

	ua   *ecdh.PrivateKey
	auth []byte
	srv  *httptest.Server

	mu       sync.Mutex
	key      *ecdsa.PublicKey
	statuses []int
	got      []Received
}

// Received is a message the service took.
type Received struct {
	Path   string
	Header http.Header
	// Message is the decrypted message, nil where Err says why not.
	Message []byte
	Err     error
}

// New starts a service that takes the messages signed with key, answers
// with the statuses given in turn and then 201, and stops with the test.
// A nil key takes the key of the first message's header.
func New(t testing.TB, key *ecdsa.PublicKey, statuses ...int) *Service {
	ua, err := ecdh.P256().GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	auth := make([]byte, 16)
	rand.Read(auth)
	s := &Service{ua: ua, auth: auth, key: key, statuses: statuses}
	s.srv = httptest.NewServer(http.HandlerFunc(s.serve))
	s.URL = s.srv.URL
	t.Cleanup(s.srv.Close)
	return s
}

func (s *Service) serve(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	rec := Received{Path: r.URL.Path, Header: r.Header.Clone()}
	s.mu.Lock()
	key := s.key
	s.mu.Unlock()
	if _, err := CheckVAPID(r.Header.Get("Authorization"), s.URL, key, time.Now()); err != nil {
		rec.Err = err
	} else if ce := r.Header.Get("Content-Encoding"); ce != "aes128gcm" {
		rec.Err = fmt.Errorf("Content-Encoding %q", ce)
	} else {
		rec.Message, rec.Err = Decrypt(s.ua, s.auth, body)
	}
	s.mu.Lock()
	s.got = append(s.got, rec)
	status := http.StatusCreated
	if len(s.statuses) > 0 {
		status, s.statuses = s.statuses[0], s.statuses[1:]
	}
	s.mu.Unlock()
	w.WriteHeader(status)
}

// Keys are the subscriber's keys, base64url, as a browser's
// PushSubscription.toJSON() gives them.
func (s *Service) Keys() (p256dh, auth string) {
	return base64.RawURLEncoding.EncodeToString(s.ua.PublicKey().Bytes()), base64.RawURLEncoding.EncodeToString(s.auth)
}

// Subscription is the body of POST /api/v1/me/push for an endpoint at
// path.
func (s *Service) Subscription(path, label string) string {
	p256dh, auth := s.Keys()
	b, _ := json.Marshal(map[string]any{"endpoint": s.URL + path, "keys": map[string]string{"p256dh": p256dh, "auth": auth}, "label": label})
	return string(b)
}

// Received are the messages taken so far.
func (s *Service) Received() []Received {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]Received(nil), s.got...)
}

// Decrypt reads a message as its subscriber does (RFC 8291), with the
// subscriber's private key and auth secret.
func Decrypt(ua *ecdh.PrivateKey, auth, body []byte) ([]byte, error) {
	if len(body) < 21 {
		return nil, errors.New("pushtest: a short body")
	}
	salt, rs, idlen := body[:16], binary.BigEndian.Uint32(body[16:20]), int(body[20])
	if idlen != 65 || len(body) < 21+idlen {
		return nil, fmt.Errorf("pushtest: a key id of %d bytes", idlen)
	}
	asPublic, record := body[21:21+idlen], body[21+idlen:]
	if uint32(len(record)) > rs {
		return nil, fmt.Errorf("pushtest: a record of %d bytes, over rs %d", len(record), rs)
	}
	as, err := ecdh.P256().NewPublicKey(asPublic)
	if err != nil {
		return nil, err
	}
	shared, err := ua.ECDH(as)
	if err != nil {
		return nil, err
	}
	prkKey, _ := hkdf.Extract(sha256.New, shared, auth)
	ikm, _ := hkdf.Expand(sha256.New, prkKey, "WebPush: info\x00"+string(ua.PublicKey().Bytes())+string(asPublic), 32)
	prk, _ := hkdf.Extract(sha256.New, ikm, salt)
	cek, _ := hkdf.Expand(sha256.New, prk, "Content-Encoding: aes128gcm\x00", 16)
	nonce, _ := hkdf.Expand(sha256.New, prk, "Content-Encoding: nonce\x00", 12)
	block, err := aes.NewCipher(cek)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	plain, err := gcm.Open(nil, nonce, record, nil)
	if err != nil {
		return nil, err
	}
	end := bytes.LastIndexByte(plain, 2)
	if end < 0 || len(bytes.Trim(plain[end+1:], "\x00")) != 0 {
		return nil, errors.New("pushtest: no last-record delimiter")
	}
	return plain[:end], nil
}

// CheckVAPID reads an Authorization header as a push service does (RFC
// 8292): the key it names is key, where key is not nil; the token is
// signed with it, for the origin aud, and expires after now and within a
// day. It returns the token's claims.
func CheckVAPID(header, aud string, key *ecdsa.PublicKey, now time.Time) (map[string]any, error) {
	rest, ok := strings.CutPrefix(header, "vapid ")
	if !ok {
		return nil, fmt.Errorf("pushtest: not vapid: %q", header)
	}
	var tok, k string
	for _, part := range strings.Split(rest, ",") {
		part = strings.TrimSpace(part)
		if v, ok := strings.CutPrefix(part, "t="); ok {
			tok = v
		} else if v, ok := strings.CutPrefix(part, "k="); ok {
			k = v
		}
	}
	kb, err := base64.RawURLEncoding.DecodeString(k)
	if err != nil {
		return nil, fmt.Errorf("pushtest: k: %w", err)
	}
	if key == nil {
		if key, err = ecdsa.ParseUncompressedPublicKey(elliptic.P256(), kb); err != nil {
			return nil, fmt.Errorf("pushtest: k: %w", err)
		}
	} else if pub, _ := key.Bytes(); !bytes.Equal(pub, kb) {
		return nil, errors.New("pushtest: k is not the server's key")
	}
	parts := strings.Split(tok, ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("pushtest: a token of %d parts", len(parts))
	}
	head, _ := base64.RawURLEncoding.DecodeString(parts[0])
	if string(head) != `{"typ":"JWT","alg":"ES256"}` {
		return nil, fmt.Errorf("pushtest: header %s", head)
	}
	sig, _ := base64.RawURLEncoding.DecodeString(parts[2])
	sum := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
	if len(sig) != 64 || !ecdsa.Verify(key, sum[:], new(big.Int).SetBytes(sig[:32]), new(big.Int).SetBytes(sig[32:])) {
		return nil, errors.New("pushtest: the signature does not verify")
	}
	raw, _ := base64.RawURLEncoding.DecodeString(parts[1])
	var claims map[string]any
	if err := json.Unmarshal(raw, &claims); err != nil {
		return nil, err
	}
	if claims["aud"] != aud {
		return nil, fmt.Errorf("pushtest: aud %v, want %s", claims["aud"], aud)
	}
	expf, _ := claims["exp"].(float64)
	exp := time.Unix(int64(expf), 0)
	if !exp.After(now) || exp.Sub(now) > 24*time.Hour {
		return nil, fmt.Errorf("pushtest: exp %v at %v", exp, now)
	}
	return claims, nil
}
