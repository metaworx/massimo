// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

// Authentication helpers: the per-address limit on failed token checks
// and the short-lived tickets that open the event stream.

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Defaults of the failed-token limit and the stream tickets.
const (
	// DefaultAuthFailures failed token checks from one address within
	// DefaultAuthWindow lock that address out for DefaultAuthLockout.
	DefaultAuthFailures = 10
	DefaultAuthWindow   = time.Minute
	DefaultAuthLockout  = 5 * time.Minute
	// TicketLifetime is how long a stream ticket can be redeemed.
	TicketLifetime = time.Minute
	// maxTracked bounds the addresses the limiter and the tickets keep;
	// beyond it, expired entries are dropped first, then the oldest.
	maxTracked = 4096
	// tokenQueryDeprecated is the Deprecation header (RFC 9745) of a
	// stream opened with ?token=: deprecated since 2026-11-05.
	tokenQueryDeprecated = "@1793836800"
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

// ticketStore holds the stream tickets: random, single-use, bound to one
// Cube, valid for TicketLifetime.
type ticketStore struct {
	now func() time.Time

	mu      sync.Mutex
	tickets map[string]ticket
}

type ticket struct {
	cube    string // lower-case key of the Cube
	expires time.Time
}

func newTicketStore(now func() time.Time) *ticketStore {
	if now == nil {
		now = time.Now
	}
	return &ticketStore{now: now, tickets: map[string]ticket{}}
}

// issue creates a ticket for the event stream of the Cube with the
// given key.
func (s *ticketStore) issue(cubeKey string) (string, time.Time, error) {
	var b [24]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", time.Time{}, err
	}
	id := hex.EncodeToString(b[:])
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now()
	for k, t := range s.tickets {
		if !t.expires.After(now) {
			delete(s.tickets, k)
		}
	}
	if len(s.tickets) >= maxTracked {
		return "", time.Time{}, errTooManyTickets
	}
	exp := now.Add(TicketLifetime)
	s.tickets[id] = ticket{cube: cubeKey, expires: exp}
	return id, exp, nil
}

// redeem uses up a ticket and reports whether it was valid for the Cube.
func (s *ticketStore) redeem(id, cubeKey string) bool {
	if id == "" {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok := s.tickets[id]
	if !ok {
		return false
	}
	delete(s.tickets, id)
	return t.expires.After(s.now()) && t.cube == cubeKey
}

type ticketError string

func (e ticketError) Error() string { return string(e) }

const errTooManyTickets = ticketError("too many stream tickets outstanding")

// authorize checks a request to an /api route against the token. It
// writes the error response and returns false when the request may not
// proceed. The event stream also opens with ?ticket=<ticket> from
// issueTicket and, deprecated, ?token=<token>, because browsers open it
// without headers.
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
		q := r.URL.Query()
		if s.tickets.redeem(q.Get("ticket"), strings.ToLower(r.PathValue("sn"))) {
			return true
		}
		if tokenMatches(s.opts.Token, q.Get("token")) {
			s.limiter.succeed(ip)
			w.Header().Set("Deprecation", tokenQueryDeprecated)
			return true
		}
	}
	if s.limiter.fail(ip) {
		s.log.Warn("api: address locked out after failed token checks", "remote", ip, "for", s.limiter.lockout)
	}
	writeError(w, http.StatusUnauthorized, "missing or wrong token")
	return false
}

// issueTicket handles POST /cubes/{sn}/events/ticket: a single-use
// ticket that opens that Cube's event stream within TicketLifetime, so
// that the token never has to go into a URL.
func (s *Server) issueTicket(w http.ResponseWriter, r *http.Request) error {
	if _, err := s.cube(r); err != nil {
		return err
	}
	id, exp, err := s.tickets.issue(strings.ToLower(r.PathValue("sn")))
	if err != nil {
		return fail(http.StatusServiceUnavailable, "%v", err)
	}
	writeJSON(w, http.StatusOK, map[string]any{"ticket": id, "expires_at": exp})
	return nil
}
