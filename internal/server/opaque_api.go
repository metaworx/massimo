// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

// The sign-in by OPAQUE (docs/store.md section 6, auth_version 2;
// internal/store/opaque.go): no password, hash or key that could be
// replayed crosses the network.
//
//	POST /api/v1/login/start            {user, ke1}   → {login, ke2, ksf, v1}
//	POST /api/v1/login/finish           {login, ke3}  → a session, as POST /api/v1/login
//	POST /api/v1/users/{name}/register  {request}     → {response, ksf}
//
// The registration's record goes to POST /api/v1/users or PUT
// /api/v1/users/{name} as "record" and "ksf", in place of "password".
// The messages are base64url without padding, as @serenity-kit/opaque
// writes them.

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"slices"
	"sync"
	"time"

	"go.metaworx.ch/massimo/internal/store"
)

const (
	// loginTTL is how long a sign-in started waits for its finish, and
	// holds its address meanwhile.
	loginTTL = time.Minute
	// proofTTL is how long a proof of the password stands, for one's own
	// password change.
	proofTTL = 5 * time.Minute
	// purposeProof asks a sign-in for a proof of the password in place
	// of a session.
	purposeProof = "proof"
)

// opaqueB64 is the encoding of the messages.
var opaqueB64 = base64.RawURLEncoding

// pendingLogin is a sign-in started, waiting for its finish.
type pendingLogin struct {
	name, ip         string
	addrKey, userKey string
	// locked are the keys this start's failure locked out, which a
	// finish that proves the password lifts again.
	locked  []string
	purpose string
	start   store.OpaqueStart
	expires time.Time
}

// cleared clears the failures of a sign-in that proved the password:
// the lockout its own start set lifted, the others' kept as a password
// sign-in keeps them.
func (s *Server) cleared(ctx context.Context, p *pendingLogin) {
	for _, k := range []string{p.userKey, p.addrKey} {
		if slices.Contains(p.locked, k) {
			s.limiter.forgive(ctx, k)
		} else {
			s.limiter.succeed(ctx, k)
		}
	}
}

// passwordProof is a proof of a user's password, good once.
type passwordProof struct {
	user    int64
	expires time.Time
}

// opaqueLogins are the sign-ins started and the proofs given, in memory,
// at most maxTracked of each: a restart ends them, and the client
// starts again.
type opaqueLogins struct {
	mu      sync.Mutex
	pending map[string]*pendingLogin
	proofs  map[string]passwordProof
}

// randomID is a random identifier of a sign-in or a proof.
func randomID() string {
	var b [24]byte
	rand.Read(b[:])
	return opaqueB64.EncodeToString(b[:])
}

// add keeps a sign-in started under a new identifier; false when
// maxTracked are waiting.
func (o *opaqueLogins) add(p *pendingLogin, now time.Time) (string, bool) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if len(o.pending) >= maxTracked {
		for k, q := range o.pending {
			if !now.Before(q.expires) {
				delete(o.pending, k)
			}
		}
		if len(o.pending) >= maxTracked {
			return "", false
		}
	}
	if o.pending == nil {
		o.pending = map[string]*pendingLogin{}
	}
	id := randomID()
	o.pending[id] = p
	return id, true
}

// take ends a sign-in started; false when there is none of the
// identifier, or it has ended.
func (o *opaqueLogins) take(id string, now time.Time) (*pendingLogin, bool) {
	o.mu.Lock()
	defer o.mu.Unlock()
	p, ok := o.pending[id]
	delete(o.pending, id)
	if !ok || !now.Before(p.expires) {
		return nil, false
	}
	return p, true
}

// issueProof gives a proof of a user's password; empty when maxTracked
// stand.
func (o *opaqueLogins) issueProof(user int64, now time.Time) (string, time.Time) {
	o.mu.Lock()
	defer o.mu.Unlock()
	for k, p := range o.proofs {
		if !now.Before(p.expires) {
			delete(o.proofs, k)
		}
	}
	if len(o.proofs) >= maxTracked {
		return "", time.Time{}
	}
	if o.proofs == nil {
		o.proofs = map[string]passwordProof{}
	}
	id, end := randomID(), now.Add(proofTTL)
	o.proofs[id] = passwordProof{user: user, expires: end}
	return id, end
}

// redeemProof spends a proof: true when it stood and was the user's.
func (o *opaqueLogins) redeemProof(id string, user int64, now time.Time) bool {
	o.mu.Lock()
	defer o.mu.Unlock()
	p, ok := o.proofs[id]
	delete(o.proofs, id)
	return ok && p.user == user && now.Before(p.expires)
}

// loginStartRequest is the body of POST /api/v1/login/start.
type loginStartRequest struct {
	User string `json:"user"`
	KE1  string `json:"ke1"`
	// Purpose "proof" ends in a proof of the password, for one's own
	// password change, in place of a session.
	Purpose string `json:"purpose,omitempty"`
}

// loginStart handles POST /api/v1/login/start: the server's answer to a
// client's KE1 for a name. A name without a record of version 2 is
// answered from a fake one, so that the answer does not say which names
// exist. A start is a guess, since the client can tell from KE2 alone
// whether its password was right: it counts as a failure against the
// address and the name at once, which a finished sign-in clears. The
// address is held until the finish, at most loginTTL.
func (s *Server) loginStart(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	if err := s.loginMode(ctx); err != nil {
		return err
	}
	var req loginStartRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.User == "" || req.KE1 == "" {
		return fail(http.StatusBadRequest, "body must have \"user\" and \"ke1\"")
	}
	if req.Purpose != "" && req.Purpose != purposeProof {
		return fail(http.StatusBadRequest, "purpose: want %q or none", purposeProof)
	}
	ke1, err := opaqueB64.DecodeString(req.KE1)
	if err != nil {
		return fail(http.StatusBadRequest, "ke1: not base64url: %v", err)
	}
	st := s.opts.Store
	name := store.NormalName(req.User)
	ip := s.clientIP(r)
	auditTarget(r, "user:"+name)
	addrKey, userKey := store.LockoutAddr(ip), store.LockoutUser(name)
	if err := s.lockedOut(w, ctx, addrKey, userKey); err != nil {
		return err
	}
	switch ok, full := s.signIns.claim(addrKey); {
	case full:
		return passwordBusy(store.ErrBusy)
	case !ok:
		return &httpError{status: http.StatusTooManyRequests, msg: "a sign-in from this address is already running; try again when it is done", retryAfter: 1}
	}
	held := false
	defer func() {
		if !held {
			s.signIns.release(addrKey)
		}
	}()
	start, err := st.StartOpaque(ctx, name, ke1)
	if errors.Is(err, store.ErrBadMessage) {
		return fail(http.StatusBadRequest, "ke1: %v", err)
	}
	if err != nil {
		return fail(http.StatusInternalServerError, "%v", err)
	}
	var locked []string
	if s.limiter.fail(ctx, userKey) {
		locked = append(locked, userKey)
	}
	if s.limiter.fail(ctx, addrKey) {
		locked = append(locked, addrKey)
		s.log.Warn("api: address locked out after failed logins", "remote", ip, "for", s.authLockout)
	}
	now := time.Now()
	end := now.Add(loginTTL)
	id, ok := s.logins.add(&pendingLogin{name: name, ip: ip, addrKey: addrKey, userKey: userKey, locked: locked, purpose: req.Purpose, start: start, expires: end}, now)
	if !ok {
		return passwordBusy(store.ErrBusy)
	}
	s.signIns.hold(addrKey, end)
	held = true
	writeJSON(w, http.StatusOK, map[string]any{"login": id, "ke2": opaqueB64.EncodeToString(start.KE2), "ksf": start.KSF, "v1": st.PasswordLogin(ctx), "expires_at": end})
	return nil
}

// loginFinishRequest is the body of POST /api/v1/login/finish.
type loginFinishRequest struct {
	Login string `json:"login"`
	KE3   string `json:"ke3"`
	// Label names the session, as in POST /api/v1/login.
	Label string `json:"label,omitempty"`
}

// loginFinish handles POST /api/v1/login/finish: the client's KE3 for a
// sign-in started. A match is a session, or with the purpose proof a
// proof of the password; anything else is a wrong password, which the
// start has counted already.
func (s *Server) loginFinish(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	if err := s.loginMode(ctx); err != nil {
		return err
	}
	var req loginFinishRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Login == "" || req.KE3 == "" {
		return fail(http.StatusBadRequest, "body must have \"login\" and \"ke3\"")
	}
	now := time.Now()
	p, ok := s.logins.take(req.Login, now)
	if !ok {
		return fail(http.StatusUnauthorized, "no sign-in started under this login, or it took too long; start again")
	}
	defer s.signIns.release(p.addrKey)
	auditTarget(r, "user:"+p.name)
	st := s.opts.Store
	ke3, _ := opaqueB64.DecodeString(req.KE3)
	err := st.FinishOpaque(p.start, ke3)
	var u store.User
	if err == nil {
		var ok bool
		u, ok, err = st.User(ctx, p.name)
		if err == nil && (!ok || u.ID != p.start.UserID()) {
			// Removed, or another under the name, since the start.
			err = store.ErrBadPassword
		}
	}
	if errors.Is(err, store.ErrBadPassword) {
		auditAction(r, "login.failed")
		s.log.Warn("api: login failed", "user", p.name, "remote", p.ip)
		return fail(http.StatusUnauthorized, "wrong user or password")
	}
	if err != nil {
		return fail(http.StatusInternalServerError, "%v", err)
	}
	if p.purpose != purposeProof {
		if err := s.openSession(w, r, u, req.Label, nil); err != nil {
			return err
		}
		s.cleared(ctx, p)
		return nil
	}
	s.cleared(ctx, p)
	proof, end := s.logins.issueProof(u.ID, now)
	if proof == "" {
		return passwordBusy(store.ErrBusy)
	}
	writeJSON(w, http.StatusOK, map[string]any{"proof": proof, "expires_at": end})
	return nil
}

// registerRequest is the body of POST /api/v1/users/{name}/register.
type registerRequest struct {
	Request string `json:"request"`
}

// registerOpaque handles POST /api/v1/users/{name}/register: the first step of
// a password of version 2, the server's answer to the client's
// registration request. Nothing is kept. Allowed to whoever may set the
// user's password, or, for a name that has no user, add one.
func (s *Server) registerOpaque(w http.ResponseWriter, r *http.Request) error {
	st, err := s.userStore()
	if err != nil {
		return err
	}
	name := store.NormalName(r.PathValue("name"))
	if err := s.mayRegister(r, st, name); err != nil {
		return err
	}
	var req registerRequest
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil || req.Request == "" {
		return fail(http.StatusBadRequest, "body must have \"request\"")
	}
	b, err := opaqueB64.DecodeString(req.Request)
	if err != nil {
		return fail(http.StatusBadRequest, "request: not base64url: %v", err)
	}
	resp, err := st.OpaqueRegistration(name, b)
	if err != nil {
		return userFail(err)
	}
	writeJSON(w, http.StatusOK, map[string]any{"response": opaqueB64.EncodeToString(resp), "ksf": store.OpaqueKSF})
	return nil
}

// mayRegister checks that the principal may set a password for a name:
// its own, a user it manages, or a user it may add, the first owner from
// the local network on an open server among them.
func (s *Server) mayRegister(r *http.Request, st *store.Store, name string) error {
	p := principalOf(r)
	if p.mode == modeOpen {
		if !localPeer(s.clientIP(r)) {
			return fail(http.StatusForbidden, "the first owner is created from the local network, or on the box with massimo user add")
		}
		return nil
	}
	u, ok, err := st.User(r.Context(), name)
	if err != nil {
		return userFail(err)
	}
	a := s.accessOf(r.Context(), p, name)
	switch {
	case ok && (a.maySetPassword(u) || a.self):
		return nil
	case ok:
		return a.rankFail(u.Role, "users")
	case a.all || a.guests:
		return nil
	}
	return fail(http.StatusNotFound, "no user %q", name)
}
