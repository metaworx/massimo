// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

// Authentication helpers: the per-address limit on failed token checks.

import (
	"crypto/subtle"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Defaults of the failed-token limit.
const (
	// DefaultAuthFailures failed token checks from one address within
	// DefaultAuthWindow lock that address out for DefaultAuthLockout.
	DefaultAuthFailures = 10
	DefaultAuthWindow   = time.Minute
	DefaultAuthLockout  = 5 * time.Minute
	// maxTracked bounds the addresses the limiter keeps;
	// beyond it, expired entries are dropped first, then the oldest.
	maxTracked = 4096
)

// tokenMatches compares a presented token with the configured one in
// constant time, so that response timing reveals nothing about it.
func tokenMatches(want, got string) bool {
	return got != "" && subtle.ConstantTimeCompare([]byte(want), []byte(got)) == 1
}

// clientIP is the address a request came from, without the port. Headers
// such as X-Forwarded-For are not trusted: anyone can set them.
func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// authLimiter counts failed token checks per address in a fixed window
// and locks an address out once it reaches the limit.
type authLimiter struct {
	max     int
	window  time.Duration
	lockout time.Duration
	now     func() time.Time

	mu    sync.Mutex
	addrs map[string]*authFailures
}

type authFailures struct {
	since  time.Time // start of the current window
	count  int
	locked time.Time // locked out until then
}

func newAuthLimiter(max int, window, lockout time.Duration, now func() time.Time) *authLimiter {
	if max <= 0 {
		max = DefaultAuthFailures
	}
	if window <= 0 {
		window = DefaultAuthWindow
	}
	if lockout <= 0 {
		lockout = DefaultAuthLockout
	}
	if now == nil {
		now = time.Now
	}
	return &authLimiter{max: max, window: window, lockout: lockout, now: now, addrs: map[string]*authFailures{}}
}

// lockedFor reports how long the address is still locked out, 0 if not.
func (l *authLimiter) lockedFor(ip string) time.Duration {
	l.mu.Lock()
	defer l.mu.Unlock()
	if f := l.addrs[ip]; f != nil {
		if d := f.locked.Sub(l.now()); d > 0 {
			return d
		}
	}
	return 0
}

// fail records a failed check and reports whether it locked the address
// out.
func (l *authLimiter) fail(ip string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	f := l.addrs[ip]
	if f == nil {
		l.pruneLocked(now)
		f = &authFailures{since: now}
		l.addrs[ip] = f
	}
	if now.Sub(f.since) > l.window {
		f.since, f.count = now, 0
	}
	f.count++
	if f.count >= l.max {
		f.locked, f.since, f.count = now.Add(l.lockout), now, 0
		return true
	}
	return false
}

// succeed forgets the failures of an address that presented the token.
func (l *authLimiter) succeed(ip string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if f := l.addrs[ip]; f != nil && !f.locked.After(l.now()) {
		delete(l.addrs, ip)
	}
}

// pruneLocked keeps the map bounded: entries whose window and lockout
// are over go first; if that is not enough, the oldest window goes.
func (l *authLimiter) pruneLocked(now time.Time) {
	if len(l.addrs) < maxTracked {
		return
	}
	for ip, f := range l.addrs {
		if now.Sub(f.since) > l.window && !f.locked.After(now) {
			delete(l.addrs, ip)
		}
	}
	for len(l.addrs) >= maxTracked {
		oldest, at := "", now
		for ip, f := range l.addrs {
			if f.since.Before(at) {
				oldest, at = ip, f.since
			}
		}
		if oldest == "" {
			return
		}
		delete(l.addrs, oldest)
	}
}

// authorize checks a request to an /api route against the token. It
// writes the error response and returns false when the request may not
// proceed. The event stream also takes ?token=<token>, because browsers
// open it without headers.
func (s *Server) authorize(w http.ResponseWriter, r *http.Request) bool {
	if s.opts.Token == "" {
		return true
	}
	ip := clientIP(r)
	if d := s.limiter.lockedFor(ip); d > 0 {
		w.Header().Set("Retry-After", strconv.Itoa(int(d.Round(time.Second)/time.Second)))
		writeError(w, http.StatusTooManyRequests, "too many failed token checks from this address; try again later")
		return false
	}
	if tokenMatches(s.opts.Token, strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")) {
		s.limiter.succeed(ip)
		return true
	}
	if r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/events") {
		if tokenMatches(s.opts.Token, r.URL.Query().Get("token")) {
			s.limiter.succeed(ip)
			return true
		}
	}
	if s.limiter.fail(ip) {
		s.log.Warn("api: address locked out after failed token checks", "remote", ip, "for", s.limiter.lockout)
	}
	writeError(w, http.StatusUnauthorized, "missing or wrong token")
	return false
}
