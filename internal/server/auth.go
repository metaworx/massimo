// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

// Authentication (docs/store.md section 6): who a request comes from
// and what it may do. A server runs in one of three modes. Without a
// store (serve -addr) the one token of Options.Token is checked as it
// always was. With a store and no regular user yet, an integration's
// users not counting (store.CountUsers), the API is open and every
// request may do everything; POST /api/v1/users then creates the first
// owner (users_api.go). With users, every request carries a session or
// an API token, resolved through the store to its user, whose role
// grants the capabilities; a route is registered with the capability it
// needs (server.go) and refused with 403 without it. Failed checks are
// counted per address, and for a login per user name, and lock the key
// out for a while; with a store the counters survive a restart. A token
// the server issued and that ended is refused as a wrong one is, but
// guesses nothing, and is not counted.

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
	"slices"
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

// capabilities lists every capability of the table in docs/api-v1.md
// ("Authentication") in its order; TestCapabilityTableCoversRoutes
// holds the table to it. A route is registered with one of them; the
// roles' sets are rows of the store, and what a capability includes is
// includes'.
var capabilities = []string{
	"read", "preferences", "setpoint", "lock", "configure", "rename", "structure", "pair", "discover", "backup", "restore", "repair",
	"guests", "users", "passwords", "roles", "roles.disable", "capabilities", "cubes", "cubes.reboot", "settings", "update", "restart", "reboot", "audit",
	"auth", "integrations",
}

// integrationCeiling is the fixed role integration's set (migration 0021,
// repair by 0022): every capability but those that manage people,
// sign-in and the integrations themselves, so that an integration can
// never widen its own users. It bounds every call through an
// integration (docs/api-v1.md, "Integrations"); guests is an
// integration's for its own users stored as guest (users_api.go,
// guestOf).
var integrationCeiling = []string{
	"read", "preferences", "setpoint", "lock", "configure", "rename", "structure", "pair", "discover", "backup", "restore", "repair",
	"guests", "cubes", "cubes.reboot", "settings", "update", "restart", "reboot", "audit",
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

// includes is what a capability includes (AP StartConfiguration, D37):
// its holder passes every check of the capabilities it names, as their
// holder would. It is taken at the check (D39-A), on a principal's set as
// every bound left it (has), and nowhere else: no role's stored set
// changes, and a capability disabled for every role is not included. One
// step: what an included capability includes is not taken. An update and
// a reboot each restart the server; a reboot does not include an update,
// which may bring new or incompatible features (D40-A).
var includes = map[string][]string{
	"update": {"restart"},
	"reboot": {"restart"},
}

// included reports whether set holds a capability that includes c
// (includes), c not being one of off, the capabilities disabled for
// every role.
func included(set, off map[string]bool, c string) bool {
	if off[c] {
		return false
	}
	for holder, more := range includes {
		if set[holder] && slices.Contains(more, c) {
			return true
		}
	}
	return false
}

// heldCaps lists the capabilities set passes the checks of, sorted: its
// own and what they include (included), none of off; GET /api/v1/me
// lists them.
func heldCaps(set, off map[string]bool) []string {
	out := []string{}
	for c, ok := range set {
		if ok {
			out = append(out, c)
		}
	}
	for _, more := range includes {
		for _, c := range more {
			if !set[c] && !slices.Contains(out, c) && included(set, off, c) {
				out = append(out, c)
			}
		}
	}
	slices.Sort(out)
	return out
}

// setOf is a list of capabilities as a set.
func setOf(list []string) map[string]bool {
	set := map[string]bool{}
	for _, c := range list {
		set[c] = true
	}
	return set
}

// capabilityRequired is the error of a request refused for want of the
// route's capability: `capability "restart" required`.
func capabilityRequired(capability string) string {
	return fmt.Sprintf("capability %q required", capability)
}

// authMode is how a server authenticates.
type authMode int

const (
	modeLegacy authMode = iota // no store: Options.Token, or nothing
	modeOpen                   // a store without regular users: everything is open
	modeUsers                  // users: a token names one, its role grants capabilities
)

// principal is who a request comes from: a user with a token in users
// mode, anyone in the other two.
type principal struct {
	mode  authMode
	user  store.User
	token store.Token
	// caps is the set every bound left (principalFor, onbehalf.go), what
	// it includes not added: has takes that at the check. off are the
	// capabilities disabled for every role that the roles' effective sets
	// left out (store.Effective), none for owner's; has never includes
	// one of them.
	caps map[string]bool
	off  map[string]bool
	// legacyToken is the one token a server without a store had when it
	// let the principal in, "" for none (stillAdmitted).
	legacyToken string
	// integration is the registered integration whose key it is, and
	// actsFor the integration's user the call acts for (onbehalf.go),
	// with instance, the installation id the call came from, which its
	// event stream asks again at each keep-alive (stillActsFor); "" for
	// every other principal.
	integration *store.Integration
	actsFor     *store.IntegrationIdentity
	instance    string
}

// realUserID is who makes the principal's calls: the integration's user
// a call through an integration acts for, which p.user is linked to
// where that is another; else p.user.
func (p *principal) realUserID() int64 {
	if p.actsFor != nil {
		return p.actsFor.RealUserID
	}
	return p.user.ID
}

// has reports whether the principal holds a capability, or one that
// includes it (included): every check of a capability asks it, so that
// the include is taken on the set as every bound left it. Outside users
// mode everyone holds every one.
func (p *principal) has(c string) bool {
	if p == nil {
		return false
	}
	if p.mode != modeUsers {
		return true
	}
	return p.caps[c] || included(p.caps, p.off, c)
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
		return refuse(http.StatusForbidden, codeCapabilityRequired, capability)
	}
	return nil
}

// authMode says how the server authenticates now: the store's count of
// its regular users (store.CountUsers) decides between open and users,
// so a server is open while it has no user that is not an
// integration's, a disabled one counting (AP LooseEnds, D1); a store
// that cannot be read fails closed.
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
// regular users), "token" or "none" (no store).
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
// with it, who is told what the open server left (tellOpenSetup), and a
// key the file's token was before is revoked; without a store it is the
// one token. "" is none.
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
	if res.OwnerCreated {
		s.tellOpenSetup(ctx, store.BootstrapUser, store.RoleOwner)
	}
	return nil
}

// warnIntegrationUsersOnly warns at the start of a server whose only
// users are an integration's: it is open, as one without users, until
// its first owner is made (AP LooseEnds, D2). A version before counted
// those users, and kept such a server closed. A server kept open on
// purpose (auth.keep_open) is not told.
func (s *Server) warnIntegrationUsersOnly(ctx context.Context) {
	st := s.opts.Store
	if st == nil || s.authMode(ctx) != modeOpen || st.KeepOpen(ctx) {
		return
	}
	n, err := st.CountIntegrationUsers(ctx)
	if err != nil {
		s.log.Error("store: the integrations' users not counted", "error", err)
		return
	}
	if n > 0 {
		s.log.Warn("the server is open: its only users are an integration's; create the owner", "integration_users", n)
	}
}

// DropKeepOpen removes the setting auth.keep_open where the server has
// regular users (store.DropKeepOpen): at the start and at each reload of the
// settings file, for what another process, or an older version, left
// behind (AP FirstOwner, D12). Without a store there is no setting.
func (s *Server) DropKeepOpen(ctx context.Context) {
	if s.opts.Store == nil {
		return
	}
	if _, err := s.opts.Store.DropKeepOpen(ctx); err != nil {
		s.log.Error("store: the keep-open setting not checked", "error", err)
	}
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

// ParseTrustedProxies reads serve's -trusted-proxies, whose default is
// MASSIMO_TRUSTED_PROXIES: addresses and CIDRs separated by commas or
// spaces, an address standing for itself.
func ParseTrustedProxies(v string) ([]netip.Prefix, error) {
	var out []netip.Prefix
	for _, f := range strings.FieldsFunc(v, func(r rune) bool { return r == ',' || r == ' ' || r == ';' || r == '\t' }) {
		if p, err := netip.ParsePrefix(f); err == nil {
			out = append(out, p.Masked())
			continue
		}
		a, err := netip.ParseAddr(f)
		if err != nil {
			return nil, fmt.Errorf("-trusted-proxies or MASSIMO_TRUSTED_PROXIES: %q is neither an address nor a CIDR", f)
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
// beyond its local network, with the code server_open_local_only
// (errcodes_users.go).
const openRefused = "this server is open and answers only its local network; create a user or set MASSIMO_API_TOKEN"

// openAllowed reports whether a server that asks for no key, a store
// without regular users or no store and no token, answers a request:
// only a client on the local network (localPeer), and a request that
// names a client in a forwarding header only through a trusted proxy,
// since a reverse proxy on the box would otherwise pass the internet
// through as itself.
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

// errEndedToken is a token the server issued that has ended: revoked,
// logged out, expired, or a session whose key was revoked
// (store.TokenEnded). It is refused as errBadToken is, with its words,
// but is no guess, and does not count toward the lockout (AP MobileApp
// v1.4, D64-A).
var errEndedToken = fmt.Errorf("%w: ended", errBadToken)

// guessed reports whether a token's refusal is a guess, which counts
// toward the lockout: a token nobody holds, and not one that ended.
func guessed(why error) bool {
	return errors.Is(why, errBadToken) && !errors.Is(why, errEndedToken)
}

// forbiddenError is the refusal (refuse) of a token whose user may not
// use the API now: its words, its code and its params (errcodes.go).
type forbiddenError struct{ he *httpError }

func (e *forbiddenError) Error() string { return e.he.msg }

// forbidden is a refusal, made by refuse or refuseSaying with 403, as a
// forbiddenError.
func forbidden(err error) error {
	return &forbiddenError{he: err.(*httpError)}
}

// resolve turns a presented token into a principal, or says why not: a
// token the store issued that has ended is errEndedToken, one it never
// issued errBadToken.
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
	hash := store.HashToken(secret)
	tok, ok, err := s.opts.Store.TokenByHash(ctx, hash)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	if ok && tok.Expired(now) {
		return nil, errEndedToken
	}
	if !ok {
		return nil, s.unknownToken(ctx, hash, now)
	}
	p, err := s.principalFor(ctx, tok, now)
	if err != nil {
		return nil, err
	}
	s.touch(ctx, tok, secret, ip, now)
	return p, nil
}

// unknownToken says why a hash that is no live token's is refused:
// errEndedToken where the store issued it and it ended, else
// errBadToken. A store that cannot tell is logged, and the token counts
// as a guess, so that an error never lifts the lockout.
func (s *Server) unknownToken(ctx context.Context, hash string, now time.Time) error {
	ended, err := s.opts.Store.TokenEnded(ctx, hash, now)
	if err != nil {
		s.log.Error("api: ended tokens not read", "error", err)
		return errBadToken
	}
	if ended {
		return errEndedToken
	}
	return errBadToken
}

// principalFor is the principal of a live token: its user, who must be
// enabled and not past its end, and the capabilities of the user's
// role, which must be enabled; an integration's user's only while its
// integration is active, and within the role integration and its
// maximum. What the set includes is not added here but at the check
// (has), after these bounds and those of onbehalf.go.
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
		return nil, forbidden(refuse(http.StatusForbidden, codeUserAccountDisabled, u.Name))
	}
	if u.Expired(now) {
		return nil, forbidden(refuse(http.StatusForbidden, codeUserAccountExpired, u.Name))
	}
	caps, off, roleOff, err := st.Effective(ctx, u.Role, capabilities)
	if err != nil {
		return nil, err
	}
	if roleOff {
		return nil, forbidden(refuse(http.StatusForbidden, codeRoleDisabled, u.Role))
	}
	p := &principal{mode: modeUsers, user: u, token: tok, caps: setOf(caps), off: setOf(off)}
	if u.IntegrationID != 0 {
		// An integration's user, by whatever key it came: only while its
		// integration is active, and within the role integration and the
		// integration's maximum, whatever its own role (docs/api-v1.md,
		// "Acting for an integration's users").
		integ, ok, err := st.IntegrationByID(ctx, u.IntegrationID)
		if err != nil {
			return nil, err
		}
		if !ok || integ.State != store.IntegrationActive {
			return nil, forbidden(refuseSaying(http.StatusForbidden, fmt.Sprintf("the integration of user %s is disabled", u.Name), codeIntegrationDisabled))
		}
		p.caps = withinIntegration(p.caps, integ)
	}
	return p, nil
}

// stillAdmitted reports whether the caller an event stream was opened
// for would be let in now: an open server still open, the one token of a
// server without a store unchanged, a user's token neither revoked nor
// expired, its user and role still allowed and holding read, and for a
// call through an integration for one of its users the chain and the
// installation id again (stillActsFor). A store that cannot be read ends
// the stream, as authorize fails closed. The stream asks at each
// keep-alive (server.go, events), and authorize when it redeems a
// stream's ticket.
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
	if err != nil || !ok || !tok.DeletedAt.IsZero() || tok.Expired(now) {
		return false
	}
	if tok.ParentID != 0 {
		// A session made from a key lives while the key does.
		if key, ok, err := s.opts.Store.Token(ctx, tok.ParentID); err != nil || !ok || !key.DeletedAt.IsZero() {
			return false
		}
	}
	np, err := s.principalFor(ctx, tok, now)
	if err != nil || !np.has("read") {
		return false
	}
	return p.actsFor == nil || s.stillActsFor(ctx, p, np, now)
}

// touch records a token's use at most once an hour, and moves a
// session's end. A token made before the store kept revoke codes gets
// the hash of its code, from the secret presented, at its first use,
// whatever touchEvery says (AP MobileApp v1.6, D112-A); the settings
// file's key gets none and is touched at most once an hour, as no
// revoke ends it (settingsKey).
func (s *Server) touch(ctx context.Context, tok store.Token, secret, ip string, now time.Time) {
	var revoke string
	if !tok.RevokeKept && !tokenMatches(s.legacyToken(), secret) {
		revoke = store.HashToken(store.RevokeCode(secret))
	}
	if revoke == "" && !tok.LastUsedAt.IsZero() && now.Sub(tok.LastUsedAt) < touchEvery {
		return
	}
	var end time.Time
	if tok.Kind == store.TokenSession {
		end = now.Add(SessionLifetime)
	}
	if err := s.opts.Store.TouchToken(ctx, tok.ID, ip, end, revoke); err != nil {
		s.log.Error("api: token use not recorded", "token", tok.ID, "error", err)
	}
}

// allowed reports whether a principal may call a route registered with
// a capability. The users routes also admit a guests, passwords or
// integrations holder (AP AccessEnds, D46-A) and a user's own name, but
// not to a call through an integration (users_api.go, selfOf), and the
// device route a structure holder; the handlers narrow what those may
// do (users_api.go, access).
func (s *Server) allowed(p *principal, capability string, r *http.Request) bool {
	if capability == capAny || p.has(capability) {
		return true
	}
	if capability == "users" && (p.has("guests") || p.has("passwords") || p.has("integrations") || selfOf(p, r.PathValue("name"))) {
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
			writeRefusal(w, refuse(http.StatusForbidden, codeServerOpenLocalOnly))
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
		writeRefusal(w, refuse(http.StatusTooManyRequests, codeAddressLockedOut))
		return nil, false
	}
	var p *principal
	var why error
	// guess: the request presented something to check that is a guess,
	// a Bearer header (an empty one included, which arrives as "Bearer",
	// its space trimmed), a stream ticket or a stream token, that no
	// live token or ticket matched and that is no token the server
	// issued and that ended. Only such a request guesses, and only its
	// refusal counts toward the lockout; one that offers nothing, or
	// only a token that ended (revoked, logged out, expired, a session
	// whose key was revoked: D64-A), is refused all the same,
	// uncounted, so that a page before its sign-in, a client sending
	// again a session that ended, or a scanner behind a proxy that is
	// not trusted, does not lock the address out. A guessed secret
	// never hits the hash of a token that was issued. A Bearer header
	// that is a guess is refused as it is, the stream's ticket and token
	// beside it not read, so that a live one neither answers for the
	// guess, nor tells by its Deprecation header whether the guess was
	// right, nor clears the address's count with its success.
	h := r.Header.Get("Authorization")
	guess := h == "Bearer"
	if strings.HasPrefix(h, "Bearer ") {
		p, why = s.resolve(ctx, mode, strings.TrimPrefix(h, "Bearer "), ip)
		guess = guessed(why)
	}
	if p == nil && !guess && r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/events") {
		q := r.URL.Query()
		ticket, tok := q.Get("ticket"), q.Get("token")
		if tp, ok := s.tickets.redeem(ticket, strings.ToLower(r.PathValue("sn"))); ok {
			// The ticket's caller is asked again before the stream sends
			// anything, as at each keep-alive: one no longer let in since
			// the ticket was issued (its key revoked, its user disabled,
			// another installation accepted) is refused as a token that
			// ended is, uncounted, and the page's next ticket asks it to
			// sign in.
			if s.stillAdmitted(ctx, tp) {
				p = tp
			} else {
				why = errEndedToken
			}
		} else {
			guess = guess || ticket != ""
			if tok != "" {
				if p, why = s.resolve(ctx, mode, tok, ip); p != nil {
					w.Header().Set("Deprecation", tokenQueryDeprecated)
				}
				guess = guess || guessed(why)
			}
		}
	}
	if p == nil {
		var fe *forbiddenError
		switch {
		case errors.As(why, &fe):
			// A real token whose user may not use the API now: not a
			// failed check, and the message says why; a guess beside it
			// (a wrong ticket beside such a Bearer header) counts all the
			// same.
			if guess && s.limiter.fail(ctx, key) {
				s.logLockout(ip, "tokens")
			}
			writeRefusal(w, fe.he)
		case why != nil && !errors.Is(why, errBadToken):
			s.log.Error("api: token not resolved", "error", why)
			writeError(w, http.StatusInternalServerError, "authentication failed; see the log")
		default:
			if guess && s.limiter.fail(ctx, key) {
				s.logLockout(ip, "tokens")
			}
			writeRefusal(w, refuse(http.StatusUnauthorized, codeTokenWrong)) // errBadToken's words
		}
		return nil, false
	}
	s.limiter.succeed(ctx, key)
	q, no := s.integrationPrincipal(ctx, r, p)
	if no != nil {
		body := map[string]any{"error": no.msg}
		if no.code != nil {
			body["code"] = no.code.name
		}
		writeJSON(w, no.status, body)
		return p, false // the audit names who was refused
	}
	p = q
	if !s.allowed(p, capability, r) {
		if p.integration != nil {
			writeJSON(w, http.StatusForbidden, map[string]any{"error": capabilityRequired(capability), "code": codeForbidden.name})
			return p, false
		}
		writeRefusal(w, refuse(http.StatusForbidden, codeCapabilityRequired, capability))
		return p, false // the audit names who was refused
	}
	return p, true
}

// lockedOut answers a sign-in from an address or for a name that is
// locked out: 429 with Retry-After, nil when neither is. A key "" is
// not checked: the revoke by code checks the address alone.
func (s *Server) lockedOut(w http.ResponseWriter, ctx context.Context, addrKey, userKey string) error {
	locked := func(key string) bool {
		if key == "" {
			return false
		}
		d := s.limiter.lockedFor(ctx, key)
		if d > 0 {
			w.Header().Set("Retry-After", strconv.Itoa(int(d.Round(time.Second)/time.Second)))
		}
		return d > 0
	}
	switch {
	case locked(addrKey):
		return refuse(http.StatusTooManyRequests, codeAddressLockedOut)
	case locked(userKey):
		return refuse(http.StatusTooManyRequests, codeUserLockedOut)
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
	if errors.Is(err, errTooManyTickets) {
		return refuseSaying(http.StatusServiceUnavailable, err.Error(), codeStreamTicketsTooMany)
	}
	if err != nil {
		return fail(http.StatusInternalServerError, "%v", err)
	}
	writeJSON(w, http.StatusOK, map[string]any{"ticket": id, "expires_at": exp})
	return nil
}
