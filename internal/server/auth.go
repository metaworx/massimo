// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

// Authentication (docs/store.md section 6): who a request comes from
// and what it may do. A server runs in one of three modes. Without a
// store (serve -addr) the one token of Options.Token is checked as it
// always was. With a store and no user yet, the API is open and every
// request may do everything; POST /api/v1/users then creates the first
// owner (users_api.go). With users, every request carries a session or
// an API token, resolved through the store to its user, whose role
// grants the capabilities; a route is registered with the capability it
// needs (server.go) and refused with 403 without it. Failed checks are
// counted per address, and for a login per user name, and lock the key
// out for a while; with a store the counters survive a restart.

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/netip"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.metaworx.ch/massimo/internal/store"
)

// Defaults of the failed-token limit, the stream tickets and the
// sessions.
const (
	// DefaultAuthFailures failed token checks from one address within
	// DefaultAuthWindow lock that address out for DefaultAuthLockout.
	DefaultAuthFailures = 10
	DefaultAuthWindow   = time.Minute
	DefaultAuthLockout  = 5 * time.Minute
	// TicketLifetime is how long a stream ticket can be redeemed.
	TicketLifetime = time.Minute
	// SessionLifetime is how long a web session lasts from its last
	// use; a use moves its end.
	SessionLifetime = 30 * 24 * time.Hour
	// touchEvery bounds the writes of a token's last use and a
	// session's end to one an hour, so that a page polling every ten
	// seconds is not a write every ten seconds.
	touchEvery = time.Hour
	// maxTracked bounds the addresses the limiter and the tickets keep;
	// beyond it, expired entries are dropped first, then the oldest.
	maxTracked = 4096
	// tokenQueryDeprecated is the Deprecation header (RFC 9745) of a
	// stream opened with ?token=: deprecated since 2026-11-05.
	tokenQueryDeprecated = "@1793836800"
)

// capabilities lists every capability of docs/store.md section 6 in its
// order. A route is registered with one of them; the roles' sets are
// rows of the store.
var capabilities = []string{
	"read", "preferences", "setpoint", "lock", "configure", "rename", "structure", "pair", "discover", "backup", "restore",
	"guests", "users", "passwords", "roles", "roles.disable", "capabilities", "cubes", "settings", "update", "restart", "reboot", "audit",
}

// Two more values a route may be registered with, which no role holds:
// capNone, no authentication (the login), and capAny, any authenticated
// principal (logout, me).
const (
	capNone = "none"
	capAny  = "any"
)

func validCapability(c string) bool {
	for _, k := range capabilities {
		if k == c {
			return true
		}
	}
	return false
}

// authMode is how a server authenticates.
type authMode int

const (
	modeLegacy authMode = iota // no store: Options.Token, or nothing
	modeOpen                   // a store without users: everything is open
	modeUsers                  // users: a token names one, its role grants capabilities
)

// principal is who a request comes from: a user with a token in users
// mode, anyone in the other two.
type principal struct {
	mode  authMode
	user  store.User
	token store.Token
	caps  map[string]bool
	// legacyToken is the one token a server without a store had when it
	// let the principal in, "" for none (stillAdmitted).
	legacyToken string
}

// has reports whether the principal holds a capability; outside users
// mode everyone holds every one.
func (p *principal) has(c string) bool {
	if p == nil {
		return false
	}
	if p.mode != modeUsers {
		return true
	}
	return p.caps[c]
}

// named reports whether the principal is a user of the store.
func (p *principal) named() bool { return p != nil && p.mode == modeUsers }

// userID is the principal's user's row, 0 for none: an open server, the
// token of a server without a store.
func (p *principal) userID() int64 {
	if !p.named() {
		return 0
	}
	return p.user.ID
}

type ctxKey int

const principalKey ctxKey = iota

// principalOf returns the request's principal, set by the api wrapper.
func principalOf(r *http.Request) *principal {
	p, _ := r.Context().Value(principalKey).(*principal)
	return p
}

func withPrincipal(r *http.Request, p *principal) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), principalKey, p))
}

// require is a handler's check of a capability the route's registration
// does not cover: PUT …/devices/{rf} needs structure for room_id.
func (s *Server) require(r *http.Request, capability string) error {
	if !principalOf(r).has(capability) {
		return fail(http.StatusForbidden, "capability %q required", capability)
	}
	return nil
}

// authMode says how the server authenticates now: the store's user
// count decides between open and users, and a store that cannot be read
// fails closed.
func (s *Server) authMode(ctx context.Context) authMode {
	if s.opts.Store == nil {
		return modeLegacy
	}
	n, err := s.opts.Store.CountUsers(ctx)
	if err != nil {
		s.log.Error("api: users not counted", "error", err)
		return modeUsers
	}
	if n == 0 {
		return modeOpen
	}
	return modeUsers
}

// AuthMode names the mode for the log: "users", "open" (a store without
// users), "token" or "none" (no store).
func (s *Server) AuthMode() string {
	switch s.authMode(context.Background()) {
	case modeOpen:
		return "open"
	case modeUsers:
		return "users"
	}
	if s.legacyToken() != "" {
		return "token"
	}
	return "none"
}

// authenticated reports whether the API asks for a token at all, which
// is the condition for replacing the binary from the API.
func (s *Server) authenticated(ctx context.Context) bool {
	switch s.authMode(ctx) {
	case modeUsers:
		return true
	case modeOpen:
		return false
	}
	return s.legacyToken() != ""
}

// legacyToken is the one token of a server without a store: Options.Token
// at the start, or what SetToken gave it since.
func (s *Server) legacyToken() string {
	s.tokenMu.RLock()
	defer s.tokenMu.RUnlock()
	return s.token
}

// SetToken applies a MASSIMO_API_TOKEN that the settings file gained,
// changed or lost while the server runs, as a start would (docs/store.md
// section 6): with a store it becomes an owner's key, or the first owner
// with it, and a key the file's token was before is revoked; without a
// store it is the one token. "" is none.
func (s *Server) SetToken(ctx context.Context, token string) error {
	s.tokenMu.Lock()
	s.token = token
	s.tokenMu.Unlock()
	st := s.opts.Store
	if st == nil {
		s.log.Info("api token changed", "auth", s.AuthMode())
		return nil
	}
	res, err := st.Bootstrap(ctx, token)
	if err != nil {
		return err
	}
	s.log.Info("api token applied", "owner_created", res.OwnerCreated, "key_added", res.TokenAdded, "revoked", res.Revoked, "auth", s.AuthMode())
	return nil
}

// tokenMatches compares a presented token with the configured one in
// constant time, so that response timing reveals nothing about it.
func tokenMatches(want, got string) bool {
	return got != "" && subtle.ConstantTimeCompare([]byte(want), []byte(got)) == 1
}

// clientIP is the address a request came from, without the port: the
// TCP peer's, or, when the peer is one of Options.TrustedProxies, the
// client that proxy names in X-Forwarded-For (the rightmost entry that
// is not a trusted proxy itself, the leftmost when every one is), its
// header lines read as one list, or, only when no X-Forwarded-For came
// at all, X-Real-IP. An entry that does not parse is the peer: what a
// client can set never wins over what the proxy appended. Without
// trusted proxies the headers are ignored: anyone can set them. One
// address serves the lockout, the local-network check of the first
// owner, the audit and the log.
func (s *Server) clientIP(r *http.Request) string {
	peer := peerIP(r.RemoteAddr)
	if len(s.opts.TrustedProxies) == 0 || !s.trustedProxy(peer) {
		return peer
	}
	if lines := r.Header.Values("X-Forwarded-For"); len(lines) > 0 {
		parts := strings.Split(strings.Join(lines, ","), ",")
		client := peer
		for i := len(parts) - 1; i >= 0; i-- {
			a, err := netip.ParseAddr(strings.TrimSpace(parts[i]))
			if err != nil {
				return peer // a proxy would not write this
			}
			client = a.Unmap().String()
			if !s.trustedProxy(client) {
				return client
			}
		}
		return client
	}
	if a, err := netip.ParseAddr(strings.TrimSpace(r.Header.Get("X-Real-IP"))); err == nil {
		return a.Unmap().String()
	}
	return peer
}

// peerIP is the host of a remote address.
func peerIP(remoteAddr string) string {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		return remoteAddr
	}
	if a, err := netip.ParseAddr(host); err == nil {
		return a.Unmap().String()
	}
	return host
}

// trustedProxy reports whether an address is one of the trusted proxies.
func (s *Server) trustedProxy(ip string) bool {
	a, err := netip.ParseAddr(ip)
	if err != nil {
		return false
	}
	a = a.Unmap()
	for _, p := range s.opts.TrustedProxies {
		if p.Contains(a) {
			return true
		}
	}
	return false
}

// ParseTrustedProxies reads -trusted-proxies: addresses and CIDRs
// separated by commas or spaces, an address standing for itself.
func ParseTrustedProxies(v string) ([]netip.Prefix, error) {
	var out []netip.Prefix
	for _, f := range strings.FieldsFunc(v, func(r rune) bool { return r == ',' || r == ' ' || r == ';' || r == '\t' }) {
		if p, err := netip.ParsePrefix(f); err == nil {
			out = append(out, p.Masked())
			continue
		}
		a, err := netip.ParseAddr(f)
		if err != nil {
			return nil, fmt.Errorf("-trusted-proxies: %q is neither an address nor a CIDR", f)
		}
		a = a.Unmap()
		out = append(out, netip.PrefixFrom(a, a.BitLen()))
	}
	return out, nil
}

// localPeer reports whether an address is on the local network:
// loopback, a private range (RFC 1918, and the unique local addresses
// of IPv6) or link-local. The first owner may be created from there.
func localPeer(ip string) bool {
	a := net.ParseIP(ip)
	if a == nil {
		return false
	}
	return a.IsLoopback() || a.IsPrivate() || a.IsLinkLocalUnicast()
}

// openRefused is what a server that asks for no key answers a client
// beyond its local network.
const openRefused = "this server is open and answers only its local network; create a user or set MASSIMO_API_TOKEN"

// openAllowed reports whether a server that asks for no key, a store
// without users or no store and no token, answers a request: only a
// client on the local network (localPeer), and a request that names a
// client in a forwarding header only through a trusted proxy, since a
// reverse proxy on the box would otherwise pass the internet through as
// itself.
func (s *Server) openAllowed(r *http.Request) bool {
	if forwarded(r) && !s.trustedProxy(peerIP(r.RemoteAddr)) {
		return false
	}
	return localPeer(s.clientIP(r))
}

// forwarded reports whether a request carries a header that names
// another client than its TCP peer.
func forwarded(r *http.Request) bool {
	for _, h := range []string{"X-Forwarded-For", "X-Real-IP", "Forwarded"} {
		if len(r.Header.Values(h)) > 0 {
			return true
		}
	}
	return false
}

// failureLimiter counts failed checks per key and locks a key out: in
// memory without a store, in auth_failures with one.
type failureLimiter interface {
	lockedFor(ctx context.Context, key string) time.Duration
	fail(ctx context.Context, key string) bool
	succeed(ctx context.Context, key string)
	// forgive forgets the failures of a key and its lockout too: for a
	// sign-in by OPAQUE whose own start locked the key and that then
	// proved the password (opaque_api.go).
	forgive(ctx context.Context, key string)
}

// authLimiter counts failed checks per key in a fixed window and locks
// a key out once it reaches the limit; the in-memory counter of a
// server without a store.
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

// lockedFor reports how long the key is still locked out, 0 if not.
func (l *authLimiter) lockedFor(_ context.Context, key string) time.Duration {
	l.mu.Lock()
	defer l.mu.Unlock()
	if f := l.addrs[key]; f != nil {
		if d := f.locked.Sub(l.now()); d > 0 {
			return d
		}
	}
	return 0
}

// fail records a failed check and reports whether it locked the key
// out.
func (l *authLimiter) fail(_ context.Context, key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	f := l.addrs[key]
	if f == nil {
		l.pruneLocked(now)
		f = &authFailures{since: now}
		l.addrs[key] = f
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

// succeed forgets the failures of a key that passed a check.
func (l *authLimiter) succeed(_ context.Context, key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if f := l.addrs[key]; f != nil && !f.locked.After(l.now()) {
		delete(l.addrs, key)
	}
}

func (l *authLimiter) forgive(_ context.Context, key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.addrs, key)
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

// storeLimiter is the limiter on auth_failures; an error of the store
// is logged, and counts as not locked. It keeps the keys that have a
// row in memory, so that the success of every authenticated request
// writes the store only for an address that failed.
type storeLimiter struct {
	l   *store.Lockout
	log *slog.Logger

	mu     sync.Mutex
	failed map[string]bool // keys with failures in the store, up to maxTracked
}

func (l *storeLimiter) lockedFor(ctx context.Context, key string) time.Duration {
	d, err := l.l.LockedFor(ctx, key)
	if err != nil {
		l.log.Error("api: lockout not read", "error", err)
	}
	return d
}

func (l *storeLimiter) fail(ctx context.Context, key string) bool {
	locked, err := l.l.Fail(ctx, key)
	if err != nil {
		l.log.Error("api: failure not counted", "error", err)
	}
	l.mu.Lock()
	if l.failed == nil {
		l.failed = map[string]bool{}
	}
	if len(l.failed) < maxTracked {
		l.failed[key] = true
	}
	l.mu.Unlock()
	return locked
}

func (l *storeLimiter) succeed(ctx context.Context, key string) {
	l.mu.Lock()
	had := l.failed[key]
	l.mu.Unlock()
	if !had {
		return
	}
	if err := l.l.Succeed(ctx, key); err != nil {
		l.log.Error("api: failures not forgotten", "error", err)
		return
	}
	l.mu.Lock()
	delete(l.failed, key)
	l.mu.Unlock()
}

func (l *storeLimiter) forgive(ctx context.Context, key string) {
	if err := l.l.Forgive(ctx, key); err != nil {
		l.log.Error("api: lockout not lifted", "error", err)
		return
	}
	l.mu.Lock()
	delete(l.failed, key)
	l.mu.Unlock()
}

// newLimiter picks the limiter of the options: on the store when there
// is one.
func newLimiter(opts Options, log *slog.Logger) failureLimiter {
	if opts.Store == nil {
		return newAuthLimiter(opts.AuthFailures, opts.AuthWindow, opts.AuthLockout, nil)
	}
	max, window, lockout := opts.AuthFailures, opts.AuthWindow, opts.AuthLockout
	if max <= 0 {
		max = DefaultAuthFailures
	}
	if window <= 0 {
		window = DefaultAuthWindow
	}
	if lockout <= 0 {
		lockout = DefaultAuthLockout
	}
	l := &storeLimiter{l: opts.Store.Lockout(max, window, lockout), log: log, failed: map[string]bool{}}
	// The failures from before a restart go with the next success too.
	keys, err := l.l.Keys(context.Background())
	if err != nil {
		log.Error("api: lockout not read", "error", err)
	}
	for _, k := range keys {
		if len(l.failed) < maxTracked {
			l.failed[k] = true
		}
	}
	return l
}

// ticketStore holds the stream tickets: random, single-use, bound to one
// Cube and to the principal that asked, valid for TicketLifetime.
type ticketStore struct {
	now func() time.Time

	mu      sync.Mutex
	tickets map[string]ticket
}

type ticket struct {
	cube    string // lower-case key of the Cube
	expires time.Time
	p       *principal
}

func newTicketStore(now func() time.Time) *ticketStore {
	if now == nil {
		now = time.Now
	}
	return &ticketStore{now: now, tickets: map[string]ticket{}}
}

// issue creates a ticket for the event stream of the Cube with the
// given key, on behalf of p.
func (s *ticketStore) issue(cubeKey string, p *principal) (string, time.Time, error) {
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
	s.tickets[id] = ticket{cube: cubeKey, expires: exp, p: p}
	return id, exp, nil
}

// redeem uses up a ticket and returns its principal when it was valid
// for the Cube.
func (s *ticketStore) redeem(id, cubeKey string) (*principal, bool) {
	if id == "" {
		return nil, false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok := s.tickets[id]
	if !ok {
		return nil, false
	}
	delete(s.tickets, id)
	if !t.expires.After(s.now()) || t.cube != cubeKey {
		return nil, false
	}
	return t.p, true
}

type ticketError string

func (e ticketError) Error() string { return string(e) }

const errTooManyTickets = ticketError("too many stream tickets outstanding")

// errBadToken is a token nobody holds; forbiddenError a token whose
// user may not use the API now, which is not a failed check.
var errBadToken = errors.New("missing or wrong token")

type forbiddenError struct{ msg string }

func (e *forbiddenError) Error() string { return e.msg }

func forbidden(format string, args ...any) error {
	return &forbiddenError{msg: fmt.Sprintf(format, args...)}
}

// resolve turns a presented token into a principal, or says why not.
func (s *Server) resolve(ctx context.Context, mode authMode, secret, ip string) (*principal, error) {
	if mode == modeLegacy {
		if want := s.legacyToken(); tokenMatches(want, secret) {
			return &principal{mode: modeLegacy, legacyToken: want}, nil
		}
		return nil, errBadToken
	}
	if secret == "" {
		return nil, errBadToken
	}
	tok, ok, err := s.opts.Store.TokenByHash(ctx, store.HashToken(secret))
	if err != nil {
		return nil, err
	}
	now := time.Now()
	if !ok || tok.Expired(now) {
		return nil, errBadToken
	}
	p, err := s.principalFor(ctx, tok, now)
	if err != nil {
		return nil, err
	}
	s.touch(ctx, tok, ip, now)
	return p, nil
}

// principalFor is the principal of a live token: its user, who must be
// enabled and not past its end, and the capabilities of the user's
// role, which must be enabled.
func (s *Server) principalFor(ctx context.Context, tok store.Token, now time.Time) (*principal, error) {
	st := s.opts.Store
	u, ok, err := st.UserByID(ctx, tok.UserID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, errBadToken
	}
	if u.Disabled {
		return nil, forbidden("user %s is disabled", u.Name)
	}
	if u.Expired(now) {
		return nil, forbidden("user %s has expired", u.Name)
	}
	caps, roleOff, err := st.Effective(ctx, u.Role, capabilities)
	if err != nil {
		return nil, err
	}
	if roleOff {
		return nil, forbidden("role %s is disabled", u.Role)
	}
	p := &principal{mode: modeUsers, user: u, token: tok, caps: map[string]bool{}}
	for _, c := range caps {
		p.caps[c] = true
	}
	return p, nil
}

// stillAdmitted reports whether the caller an event stream was opened
// for would be let in now: an open server still open, the one token of a
// server without a store unchanged, a user's token neither revoked nor
// expired, its user and role still allowed and holding read. A store
// that cannot be read ends the stream, as authorize fails closed. The
// stream asks at each keep-alive (server.go, events).
func (s *Server) stillAdmitted(ctx context.Context, p *principal) bool {
	if p == nil {
		return false
	}
	mode := s.authMode(ctx)
	switch p.mode {
	case modeOpen:
		return mode == modeOpen
	case modeLegacy:
		return mode == modeLegacy && p.legacyToken == s.legacyToken()
	}
	if mode != modeUsers {
		return false
	}
	now := time.Now()
	tok, ok, err := s.opts.Store.Token(ctx, p.token.ID)
	if err != nil || !ok || !tok.RevokedAt.IsZero() || tok.Expired(now) {
		return false
	}
	np, err := s.principalFor(ctx, tok, now)
	return err == nil && np.has("read")
}

// touch records a token's use at most once an hour, and moves a
// session's end.
func (s *Server) touch(ctx context.Context, tok store.Token, ip string, now time.Time) {
	if !tok.LastUsedAt.IsZero() && now.Sub(tok.LastUsedAt) < touchEvery {
		return
	}
	var end time.Time
	if tok.Kind == store.TokenSession {
		end = now.Add(SessionLifetime)
	}
	if err := s.opts.Store.TouchToken(ctx, tok.ID, ip, end); err != nil {
		s.log.Error("api: token use not recorded", "token", tok.ID, "error", err)
	}
}

// allowed reports whether a principal may call a route registered with
// a capability. The users routes also admit a guests or passwords
// holder and a user's own name, and the device route a structure
// holder; the handlers narrow what those may do.
func (s *Server) allowed(p *principal, capability string, r *http.Request) bool {
	if capability == capAny || p.has(capability) {
		return true
	}
	if capability == "users" && (p.has("guests") || p.has("passwords") || p.named() && strings.EqualFold(r.PathValue("name"), p.user.Name)) {
		return true
	}
	if capability == "rename" && r.Pattern == "PUT /api/v1/cubes/{sn}/devices/{rf}" && p.has("structure") {
		// A move: the handler asks for structure with room_id and rename with name.
		return true
	}
	return false
}

// authorize checks a request to an /api route registered with a
// capability. It writes the error response and returns false when the
// request may not proceed, else the principal. The event stream also
// opens with ?ticket=<ticket> from issueTicket and, deprecated,
// ?token=<token>, because browsers open it without headers.
func (s *Server) authorize(w http.ResponseWriter, r *http.Request, capability string) (*principal, bool) {
	ctx := r.Context()
	mode := s.authMode(ctx)
	if mode == modeOpen || mode == modeLegacy && s.legacyToken() == "" {
		// A server that asks for no key answers its local network only.
		if !s.openAllowed(r) {
			writeError(w, http.StatusForbidden, openRefused)
			return nil, false
		}
		return &principal{mode: mode}, true
	}
	if capability == capNone {
		// The sign-ins: nobody yet. Each checks the address's lockout
		// itself (lockedOut), since one that goes on with a sign-in by
		// OPAQUE started was checked and counted at the start, and a
		// lockout that start set is its own to lift (opaque_api.go).
		return &principal{mode: mode}, true
	}
	ip := s.clientIP(r)
	key := store.LockoutAddr(ip)
	if d := s.limiter.lockedFor(ctx, key); d > 0 {
		w.Header().Set("Retry-After", strconv.Itoa(int(d.Round(time.Second)/time.Second)))
		writeError(w, http.StatusTooManyRequests, "too many failed token checks from this address; try again later")
		return nil, false
	}
	var p *principal
	var why error
	// offered: the request presented something to check, a Bearer
	// header (an empty one included, which arrives as "Bearer", its
	// space trimmed) or a stream ticket or token. Only such a request
	// guesses, and only its refusal counts toward the lockout; one
	// without any is refused all the same, uncounted, so that a page
	// before its sign-in, or a scanner behind a proxy that is not
	// trusted, does not lock the address out.
	h := r.Header.Get("Authorization")
	offered := h == "Bearer" || strings.HasPrefix(h, "Bearer ")
	if strings.HasPrefix(h, "Bearer ") {
		p, why = s.resolve(ctx, mode, strings.TrimPrefix(h, "Bearer "), ip)
	}
	if p == nil && r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/events") {
		q := r.URL.Query()
		ticket, tok := q.Get("ticket"), q.Get("token")
		offered = offered || ticket != "" || tok != ""
		if tp, ok := s.tickets.redeem(ticket, strings.ToLower(r.PathValue("sn"))); ok {
			p = tp
		} else if tok != "" {
			if p, why = s.resolve(ctx, mode, tok, ip); p != nil {
				w.Header().Set("Deprecation", tokenQueryDeprecated)
			}
		}
	}
	if p == nil {
		var fe *forbiddenError
		switch {
		case errors.As(why, &fe):
			// A real token whose user may not use the API now: not a
			// failed check, and the message says why.
			writeError(w, http.StatusForbidden, fe.msg)
		case why != nil && !errors.Is(why, errBadToken):
			s.log.Error("api: token not resolved", "error", why)
			writeError(w, http.StatusInternalServerError, "authentication failed; see the log")
		default:
			if offered && s.limiter.fail(ctx, key) {
				s.log.Warn("api: address locked out after failed token checks", "remote", ip, "for", s.authLockout)
			}
			writeError(w, http.StatusUnauthorized, errBadToken.Error())
		}
		return nil, false
	}
	s.limiter.succeed(ctx, key)
	if !s.allowed(p, capability, r) {
		writeError(w, http.StatusForbidden, fmt.Sprintf("capability %q required", capability))
		return p, false // the audit names who was refused
	}
	return p, true
}

// lockedOut answers a sign-in from an address or for a name that is
// locked out: 429 with Retry-After, nil when neither is.
func (s *Server) lockedOut(w http.ResponseWriter, ctx context.Context, addrKey, userKey string) error {
	for _, k := range []struct{ key, msg string }{
		{addrKey, "too many failed token checks from this address; try again later"},
		{userKey, "too many failed logins for this user; try again later"},
	} {
		if d := s.limiter.lockedFor(ctx, k.key); d > 0 {
			w.Header().Set("Retry-After", strconv.Itoa(int(d.Round(time.Second)/time.Second)))
			return fail(http.StatusTooManyRequests, "%s", k.msg)
		}
	}
	return nil
}

// issueTicket handles POST /cubes/{sn}/events/ticket: a single-use
// ticket that opens that Cube's event stream within TicketLifetime, so
// that the token never has to go into a URL.
func (s *Server) issueTicket(w http.ResponseWriter, r *http.Request) error {
	if _, err := s.cube(r); err != nil {
		return err
	}
	id, exp, err := s.tickets.issue(strings.ToLower(r.PathValue("sn")), principalOf(r))
	if err != nil {
		return fail(http.StatusServiceUnavailable, "%v", err)
	}
	writeJSON(w, http.StatusOK, map[string]any{"ticket": id, "expires_at": exp})
	return nil
}
