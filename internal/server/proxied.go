// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

// The proxy in front of the server, for GET /api/v1/system: a request a
// trusted proxy (Options.TrustedProxies) forwarded says, in one of the
// common headers, at which scheme, host and port the proxy serves it. No
// header names the proxy's certificate, so the server reads it through a
// TLS handshake with the proxy itself, best effort: at the first proxied
// request after the start, once a day after, and again when the proxied
// address changes.

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.metaworx.ch/massimo/internal/certs"
)

// proxyCheckTimeout bounds a handshake with the proxy, proxyCheckEvery
// is how often its certificate is read again.
const (
	proxyCheckTimeout = 5 * time.Second
	proxyCheckEvery   = 24 * time.Hour
)

// proxyView is the proxy a request came through, as GET /api/v1/system
// shows it: the scheme, the host and the port it serves the server at,
// Proto empty where no header says; and, for https, its certificate as
// the last check read it, nil before the first.
type proxyView struct {
	Proto       string     `json:"proto,omitempty"`
	Host        string     `json:"host"`
	Port        int        `json:"port"`
	Certificate *proxyCert `json:"certificate,omitempty"`
}

// proxyCert is a proxy's certificate: what certs.Describe reads from it,
// who issued it, whether it is valid for the proxied host against the
// system's roots, and when it was read; Error where the handshake failed.
type proxyCert struct {
	certs.Status
	Issuer    string    `json:"issuer,omitempty"`
	Trusted   bool      `json:"trusted"`
	CheckedAt time.Time `json:"checked_at"`
	Error     string    `json:"error,omitempty"`
}

// proxied is the proxy a request came through, nil where it came from no
// trusted proxy or names none: the address it serves, and for https the
// certificate of the proxy at the request's peer and the forwarded port.
func (s *Server) proxied(r *http.Request) *proxyView {
	peer := peerIP(r.RemoteAddr)
	if !proxyHeaders(r.Header) || !s.trustedProxy(peer) {
		return nil
	}
	proto, host, port := forwardedTarget(r.Header, r.Host)
	v := &proxyView{Proto: proto, Host: host, Port: port}
	if proto == "https" {
		v.Certificate = s.proxyCheck.note(s, net.JoinHostPort(peer, strconv.Itoa(port)), host)
	}
	return v
}

// schemeHeaders say the scheme the proxy serves at, https or http;
// onHeaders say https with "on".
var (
	schemeHeaders = []string{"X-Forwarded-Proto", "X-Forwarded-Scheme", "X-Forwarded-Protocol", "X-Url-Scheme", "X-Scheme"}
	onHeaders     = []string{"X-Forwarded-Ssl", "Front-End-Https"}
)

// proxyHeaders reports whether a request carries a header a proxy sets:
// one naming the client (forwarded), the scheme, the host or the port.
func proxyHeaders(h http.Header) bool {
	for _, k := range append(append([]string{"X-Forwarded-For", "X-Real-IP", "Forwarded", "X-Forwarded-Host", "X-Forwarded-Port", "CF-Visitor"},
		schemeHeaders...), onHeaders...) {
		if len(h.Values(k)) > 0 {
			return true
		}
	}
	return false
}

// forwardedTarget reads where a proxy serves a request: the scheme from
// RFC 7239's Forwarded, the X-Forwarded-Proto kind (schemeHeaders),
// X-Forwarded-Ssl or Front-End-Https "on", or Cloudflare's CF-Visitor,
// "" where none says; the host from Forwarded, X-Forwarded-Host, else the
// request's Host; the port from X-Forwarded-Port, else the host's own,
// else the scheme's. Of a list, the first value counts: the proxy the
// client reached.
func forwardedTarget(h http.Header, requestHost string) (proto, host string, port int) {
	fw := forwardedPairs(h.Get("Forwarded"))
	proto = strings.ToLower(fw["proto"])
	for _, k := range schemeHeaders {
		if proto == "" {
			proto = strings.ToLower(firstValue(h.Get(k)))
		}
	}
	for _, k := range onHeaders {
		if proto == "" && strings.EqualFold(firstValue(h.Get(k)), "on") {
			proto = "https"
		}
	}
	if v := h.Get("CF-Visitor"); proto == "" && v != "" {
		var cf struct {
			Scheme string `json:"scheme"`
		}
		if json.Unmarshal([]byte(v), &cf) == nil {
			proto = strings.ToLower(cf.Scheme)
		}
	}
	if proto != "https" && proto != "http" {
		proto = ""
	}
	host = fw["host"]
	if host == "" {
		host = firstValue(h.Get("X-Forwarded-Host"))
	}
	if host == "" {
		host = requestHost
	}
	if hp, p, err := net.SplitHostPort(host); err == nil {
		host = hp
		port, _ = strconv.Atoi(p)
	} else {
		host = strings.Trim(host, "[]")
	}
	if p, err := strconv.Atoi(firstValue(h.Get("X-Forwarded-Port"))); err == nil && p > 0 {
		port = p
	}
	if port == 0 {
		port = 80
		if proto == "https" {
			port = 443
		}
	}
	return proto, host, port
}

// firstValue is the first of a header's comma-separated values, without
// spaces or quotes.
func firstValue(v string) string {
	first, _, _ := strings.Cut(v, ",")
	return strings.Trim(strings.TrimSpace(first), `"`)
}

// forwardedPairs reads the first element of an RFC 7239 Forwarded header,
// for=…;proto=https;host=…, into its pairs, the keys in lower case.
func forwardedPairs(v string) map[string]string {
	out := map[string]string{}
	first, _, _ := strings.Cut(v, ",")
	for _, pair := range strings.Split(first, ";") {
		k, val, ok := strings.Cut(strings.TrimSpace(pair), "=")
		if ok {
			out[strings.ToLower(k)] = strings.Trim(strings.TrimSpace(val), `"`)
		}
	}
	return out
}

// proxyCheck keeps the proxy's certificate: which proxy, at its address
// and for its host, the last check's result, and when the next is due.
type proxyCheck struct {
	mu      sync.Mutex
	addr    string
	host    string
	result  *proxyCert
	due     time.Time
	running bool
}

// note takes the proxy a request came through and gives the certificate
// the last check of it read, nil before the first; a check starts in the
// background where none of this proxy ran yet or the last is a day old.
func (c *proxyCheck) note(s *Server, addr, host string) *proxyCert {
	c.mu.Lock()
	defer c.mu.Unlock()
	if addr != c.addr || host != c.host {
		c.addr, c.host, c.result, c.due = addr, host, nil, time.Time{}
	}
	c.start(s)
	if c.result == nil {
		return nil
	}
	r := *c.result
	return &r
}

// start starts a check where one is due and none is running; c.mu held.
func (c *proxyCheck) start(s *Server) {
	if c.running || time.Now().Before(c.due) {
		return
	}
	c.running = true
	go s.checkProxy(c.addr, c.host)
}

// checkProxy reads the proxy's certificate, keeps it where the proxy is
// still the one noted, and reads it again in a day.
func (s *Server) checkProxy(addr, host string) {
	s.mu.RLock()
	ctx := s.ctx
	s.mu.RUnlock()
	if ctx == nil {
		ctx = context.Background()
	}
	cctx, cancel := context.WithTimeout(ctx, proxyCheckTimeout)
	res := readProxyCert(cctx, addr, host, nil)
	cancel()
	if res.Error != "" {
		s.log.Info("proxy certificate not read", "proxy", addr, "host", host, "error", res.Error)
	} else {
		s.log.Info("proxy certificate read", "proxy", addr, "host", host, "names", strings.Join(res.Names, ","),
			"expires", res.NotAfter.Format(time.RFC3339), "trusted", res.Trusted)
	}
	c := &s.proxyCheck
	c.mu.Lock()
	c.running = false
	current := c.addr == addr && c.host == host
	if current {
		c.result = &res
		c.due = res.CheckedAt.Add(proxyCheckEvery)
	}
	c.mu.Unlock()
	if !current || ctx.Err() != nil {
		return
	}
	time.AfterFunc(proxyCheckEvery, func() {
		if ctx.Err() != nil {
			return
		}
		c.mu.Lock()
		defer c.mu.Unlock()
		if c.addr == addr && c.host == host {
			c.start(s)
		}
	})
}

// readProxyCert reads the certificate the TLS server at addr presents for
// host, whatever it is, and whether it is valid for host against roots
// (nil: the system's). The handshake verifies it as any client does; one
// it refuses, expired, self-signed or for another name, still hands over
// the certificates (tls.CertificateVerificationError), which are read as
// not trusted. It only reads: the connection is closed after the
// handshake, nothing is sent.
func readProxyCert(ctx context.Context, addr, host string, roots *x509.CertPool) proxyCert {
	out := proxyCert{CheckedAt: time.Now()}
	// An address as ServerName is verified against the certificate's
	// addresses and sent as no SNI.
	d := tls.Dialer{Config: &tls.Config{ServerName: host, RootCAs: roots}}
	var chain []*x509.Certificate
	conn, err := d.DialContext(ctx, "tcp", addr)
	if err == nil {
		defer conn.Close()
		chain = conn.(*tls.Conn).ConnectionState().PeerCertificates
		out.Trusted = true
	} else {
		var refused *tls.CertificateVerificationError
		if !errors.As(err, &refused) {
			out.Error = err.Error()
			return out
		}
		chain = refused.UnverifiedCertificates
	}
	if len(chain) == 0 {
		out.Error = "the proxy presented no certificate"
		out.Trusted = false
		return out
	}
	raw := make([][]byte, len(chain))
	for i, c := range chain {
		raw[i] = c.Raw
	}
	st, err := certs.Describe("proxy", &tls.Certificate{Certificate: raw, Leaf: chain[0]})
	if err != nil {
		out.Error = err.Error()
		out.Trusted = false
		return out
	}
	out.Status = st
	out.Issuer = chain[0].Issuer.CommonName
	if out.Issuer == "" && len(chain[0].Issuer.Organization) > 0 {
		out.Issuer = chain[0].Issuer.Organization[0]
	}
	return out
}
