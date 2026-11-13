// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

// Sign-in through an OpenID Connect provider (docs/api-v1.md, "Sign-in
// through OpenID Connect"; the providers and identities are the store's,
// providers.go and identities.go):
//
//	GET  /api/v1/login/providers        → the enabled providers, for the buttons
//	POST /api/v1/login/oidc/{key}       {redirect} → {url}, the provider's page
//	GET  /api/v1/login/oidc/callback    ?code&state → 303 to the page, #oidc=<code>
//	POST /api/v1/login/oidc/finish      {code} → a session, as POST /api/v1/login
//	POST /api/v1/users/{name}/identities/link/{key}  {redirect} → {url}, a link
//	                                    of the user's own account; the callback
//	                                    sends the page back with #oidc-linked=<key>
//	                                    or #oidc-link-error=<reason>
//
// The authorization code flow with PKCE, the server a confidential
// client. The id_token comes straight from the token endpoint over TLS,
// which OpenID Connect Core 1.0, 3.1.3.7 lets stand in for checking its
// signature; its iss, aud, exp and nonce are checked. Where the
// discovery names a userinfo endpoint, its answer for the same subject
// fills in what the id_token lacks: Nextcloud's OIDC app, since its 2.0,
// puts the e-mail and the profile there only. The callback hands
// the page a one-time code in the fragment, which the page trades for the
// session, so that the session's token never sits in a URL.

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"

	"go.metaworx.ch/massimo/internal/store"
)

const (
	// oidcStartTTL is how long a sign-in started waits for the provider.
	oidcStartTTL = 10 * time.Minute
	// oidcCodeTTL is how long the page has to trade the callback's code.
	oidcCodeTTL = time.Minute
	// oidcDiscoveryTTL is how long a discovery document is kept.
	oidcDiscoveryTTL = time.Hour
	// oidcCallback is the callback's path under the page's address: the
	// redirect URI to register at a provider.
	oidcCallback = "api/v1/login/oidc/callback"
	// oidcLeeway is the clock skew an id_token's exp is given.
	oidcLeeway = time.Minute
)

// The reasons a callback sends the page in #oidc-error=.
const (
	oidcDenied   = "denied"   // the provider said no, or the user cancelled there
	oidcFailed   = "failed"   // the provider's answer did not do
	oidcRefused  = "refused"  // no user here for that account
	oidcDisabled = "disabled" // the user is disabled or has expired
	// oidcNoEmail and oidcUnverified are a refusal (no user here) where
	// the provider sent no e-mail, or one it did not mark verified, and
	// a waiting e-mail would have bound
	oidcNoEmail    = "noemail"
	oidcUnverified = "unverified"
	// oidcTaken is a link of an account another user has
	oidcTaken = "taken"
)

// oidcClient talks to the providers.
var oidcClient = &http.Client{Timeout: 10 * time.Second}

// adminCapabilities are the capabilities a role that a provider gives
// the users it creates may not hold (AP Oidc v1.0, D4-A: never above
// operator), owner aside.
var adminCapabilities = []string{"guests", "users", "passwords", "roles", "roles.disable", "capabilities", "cubes", "settings",
	"update", "restart", "reboot", "audit", "auth"}

// oidcDiscovery is the part of a provider's discovery document the flow
// uses.
type oidcDiscovery struct {
	Issuer                string   `json:"issuer"`
	AuthorizationEndpoint string   `json:"authorization_endpoint"`
	TokenEndpoint         string   `json:"token_endpoint"`
	UserinfoEndpoint      string   `json:"userinfo_endpoint"`
	TokenAuthMethods      []string `json:"token_endpoint_auth_methods_supported"`
	fetched               time.Time
}

// oidcStarted is a sign-in sent to its provider.
type oidcStarted struct {
	provider string
	nonce    string
	verifier string
	redirect string // the page's address
	linkUser int64  // a link of this user's, not a sign-in
	expires  time.Time
}

// oidcProven is a callback's one-time code: the user it found.
type oidcProven struct {
	userID   int64
	provider string
	ip       string
	expires  time.Time
}

// oidcFlows are the sign-ins started, the codes given and the discovery
// documents read, in memory, at most maxTracked of the first two: a
// restart ends them, and the user starts again.
type oidcFlows struct {
	mu        sync.Mutex
	started   map[string]*oidcStarted // by state
	proven    map[string]*oidcProven  // by code
	discovery map[string]oidcDiscovery
}

// start keeps a sign-in under a new state; false when maxTracked wait.
func (o *oidcFlows) start(st *oidcStarted, now time.Time) (string, bool) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.started == nil {
		o.started = map[string]*oidcStarted{}
	}
	for k, v := range o.started {
		if !now.Before(v.expires) {
			delete(o.started, k)
		}
	}
	if len(o.started) >= maxTracked {
		return "", false
	}
	id := randomID()
	o.started[id] = st
	return id, true
}

// takeStarted ends a sign-in started; false for none, or one too old.
func (o *oidcFlows) takeStarted(state string, now time.Time) (*oidcStarted, bool) {
	o.mu.Lock()
	defer o.mu.Unlock()
	st, ok := o.started[state]
	delete(o.started, state)
	return st, ok && now.Before(st.expires)
}

// prove gives a one-time code for a user found; empty when maxTracked
// wait.
func (o *oidcFlows) prove(p *oidcProven, now time.Time) string {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.proven == nil {
		o.proven = map[string]*oidcProven{}
	}
	for k, v := range o.proven {
		if !now.Before(v.expires) {
			delete(o.proven, k)
		}
	}
	if len(o.proven) >= maxTracked {
		return ""
	}
	code := randomID()
	o.proven[code] = p
	return code
}

// takeProven spends a code: false for none, one too old, or one given to
// another address.
func (o *oidcFlows) takeProven(code, ip string, now time.Time) (*oidcProven, bool) {
	o.mu.Lock()
	defer o.mu.Unlock()
	p, ok := o.proven[code]
	delete(o.proven, code)
	return p, ok && now.Before(p.expires) && p.ip == ip
}

// oidcDiscover reads a provider's discovery document, kept for an hour; its
// issuer must be the provider's, its endpoints https (http on loopback,
// as the issuer may be).
func (s *Server) oidcDiscover(ctx context.Context, p store.AuthProvider) (oidcDiscovery, error) {
	now := time.Now()
	s.oidc.mu.Lock()
	d, ok := s.oidc.discovery[p.Issuer]
	s.oidc.mu.Unlock()
	if ok && now.Sub(d.fetched) < oidcDiscoveryTTL {
		return d, nil
	}
	d, err := fetchDiscovery(ctx, p.Issuer)
	if err != nil {
		return oidcDiscovery{}, err
	}
	if d.Issuer != p.Issuer {
		return oidcDiscovery{}, fmt.Errorf("discovery: the document's issuer %q is not the provider's %q", d.Issuer, p.Issuer)
	}
	d.fetched = now
	s.oidc.mu.Lock()
	if s.oidc.discovery == nil {
		s.oidc.discovery = map[string]oidcDiscovery{}
	}
	s.oidc.discovery[p.Issuer] = d
	s.oidc.mu.Unlock()
	return d, nil
}

// fetchDiscovery reads the discovery document of an issuer, uncached,
// and checks its endpoints: https, or http where the issuer is.
func fetchDiscovery(ctx context.Context, issuer string) (oidcDiscovery, error) {
	var d oidcDiscovery
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimSuffix(issuer, "/")+"/.well-known/openid-configuration", nil)
	if err != nil {
		return d, err
	}
	req.Header.Set("Accept", "application/json")
	resp, err := oidcClient.Do(req)
	if err != nil {
		return d, fmt.Errorf("discovery: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return d, fmt.Errorf("discovery: %s", resp.Status)
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&d); err != nil {
		return d, fmt.Errorf("discovery: %w", err)
	}
	if !endpointOK(d.AuthorizationEndpoint, issuer) || !endpointOK(d.TokenEndpoint, issuer) {
		return d, errors.New("discovery: the endpoints are missing, or not https")
	}
	if d.UserinfoEndpoint != "" && !endpointOK(d.UserinfoEndpoint, issuer) {
		d.UserinfoEndpoint = ""
	}
	return d, nil
}

// checkIssuer reads the discovery document of the issuer a provider is
// saved with (AP Oidc v1.1, D27-A) and returns the issuer to keep: the
// one given, or the document's where the two differ only by a trailing
// slash, since the sign-in compares them exactly.
func checkIssuer(ctx context.Context, issuer string) (string, error) {
	d, err := fetchDiscovery(ctx, issuer)
	switch {
	case err != nil:
		return "", fmt.Errorf("the provider was not reached at %s: %v", issuer, err)
	case d.Issuer == issuer:
		return issuer, nil
	case strings.TrimSuffix(d.Issuer, "/") == strings.TrimSuffix(issuer, "/"):
		return d.Issuer, nil
	}
	return "", fmt.Errorf("the provider calls itself %q, not %q", d.Issuer, issuer)
}

// endpointOK reports whether an endpoint is an absolute https URL, or
// http where the issuer itself is (a provider on loopback).
func endpointOK(v, issuer string) bool {
	u, err := url.Parse(v)
	if err != nil || u.Host == "" {
		return false
	}
	return u.Scheme == "https" || u.Scheme == "http" && strings.HasPrefix(issuer, "http://")
}

// pageAddress checks the page's address a start sends: an absolute http
// or https URL without a query or fragment, ending with a slash.
func pageAddress(v string) (string, bool) {
	u, err := url.Parse(strings.TrimSpace(v))
	if err != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Scheme != "https" && u.Scheme != "http") {
		return "", false
	}
	if !strings.HasSuffix(u.Path, "/") {
		u.Path += "/"
	}
	return u.String(), true
}

// loginProviders handles GET /api/v1/login/providers: the providers a
// user may sign in through, by key and name, for the sign-in's buttons;
// none on a server without users.
func (s *Server) loginProviders(w http.ResponseWriter, r *http.Request) error {
	out := []map[string]string{}
	if st := s.opts.Store; st != nil && s.authMode(r.Context()) == modeUsers {
		list, err := st.Providers(r.Context())
		if err != nil {
			return fail(http.StatusInternalServerError, "%v", err)
		}
		for _, p := range list {
			if p.Enabled {
				out = append(out, map[string]string{"key": p.Key, "name": cmpOr(p.Name, p.Key)})
			}
		}
	}
	writeJSON(w, http.StatusOK, out)
	return nil
}

// cmpOr is the first of the strings that is not empty.
func cmpOr(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

// oidcStartRequest is the body of POST /api/v1/login/oidc/{key}.
type oidcStartRequest struct {
	// Redirect is the page's address, under which the callback is the
	// redirect URI, and to which the callback sends the browser back.
	Redirect string `json:"redirect"`
}

// oidcStartLogin handles POST /api/v1/login/oidc/{key}: the provider's
// authorization URL for a new sign-in, with its state, nonce and PKCE
// challenge, which the page opens. An address locked out is refused, as
// for a password.
func (s *Server) oidcStartLogin(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	if err := s.loginMode(ctx); err != nil {
		return err
	}
	key := strings.ToLower(r.PathValue("key"))
	auditTarget(r, "provider:"+key)
	var req oidcStartRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return fail(http.StatusBadRequest, "body must have \"redirect\", the page's address")
	}
	redirect, ok := pageAddress(req.Redirect)
	if !ok {
		return fail(http.StatusBadRequest, "redirect: an http or https address without a query or fragment")
	}
	if d := s.limiter.lockedFor(ctx, store.LockoutAddr(s.clientIP(r))); d > 0 {
		w.Header().Set("Retry-After", fmt.Sprint(int(d.Round(time.Second)/time.Second)))
		return fail(http.StatusTooManyRequests, "too many failed sign-ins from this address; try again later")
	}
	p, ok, err := s.opts.Store.Provider(ctx, key)
	if err != nil {
		return fail(http.StatusInternalServerError, "%v", err)
	}
	if !ok || !p.Enabled {
		return fail(http.StatusNotFound, "no provider %q", key)
	}
	return s.oidcStartFlow(w, r, p, redirect, 0)
}

// oidcStartFlow keeps a sign-in, or the link of the user linkUser,
// started at the provider p and answers its authorization URL, with its
// state, nonce and PKCE challenge.
func (s *Server) oidcStartFlow(w http.ResponseWriter, r *http.Request, p store.AuthProvider, redirect string, linkUser int64) error {
	ctx := r.Context()
	d, err := s.oidcDiscover(ctx, p)
	if err != nil {
		s.log.Warn("api: OpenID Connect provider not reached", "provider", p.Key, "error", err)
		return fail(http.StatusBadGateway, "provider %s: %v", p.Key, err)
	}
	now := time.Now()
	started := &oidcStarted{provider: p.Key, nonce: randomID(), verifier: randomID() + randomID(), redirect: redirect, linkUser: linkUser,
		expires: now.Add(oidcStartTTL)}
	state, ok := s.oidc.start(started, now)
	if !ok {
		return passwordBusy(store.ErrBusy)
	}
	auth, err := url.Parse(d.AuthorizationEndpoint)
	if err != nil {
		return fail(http.StatusBadGateway, "provider %s: %v", p.Key, err)
	}
	sum := sha256.Sum256([]byte(started.verifier))
	q := auth.Query()
	for k, v := range map[string]string{
		"response_type": "code", "client_id": p.ClientID, "redirect_uri": redirect + oidcCallback, "scope": p.Scopes,
		"state": state, "nonce": started.nonce,
		"code_challenge": base64.RawURLEncoding.EncodeToString(sum[:]), "code_challenge_method": "S256",
	} {
		q.Set(k, v)
	}
	auth.RawQuery = q.Encode()
	writeJSON(w, http.StatusOK, map[string]any{"url": auth.String(), "expires_at": started.expires})
	return nil
}

// oidcClaims are the id_token's claims the sign-in reads.
type oidcClaims struct {
	Issuer            string          `json:"iss"`
	Subject           string          `json:"sub"`
	Audience          json.RawMessage `json:"aud"`
	AuthorizedParty   string          `json:"azp"`
	Expiry            float64         `json:"exp"`
	Nonce             string          `json:"nonce"`
	Email             string          `json:"email"`
	EmailVerified     json.RawMessage `json:"email_verified"`
	Name              string          `json:"name"`
	PreferredUsername string          `json:"preferred_username"`
	// raw are all the claims, for the groups (groupsOf)
	raw map[string]json.RawMessage
}

// decodeClaims reads the claims of a JSON object: those the sign-in
// reads, and all of them raw.
func decodeClaims(data []byte, c *oidcClaims) error {
	if err := json.Unmarshal(data, c); err != nil {
		return err
	}
	return json.Unmarshal(data, &c.raw)
}

// groupsOf reads a claim of groups: a list of strings, or one string;
// none where the claim is absent or of another kind.
func (c oidcClaims) groupsOf(claim string) []string {
	v, ok := c.raw[claim]
	if !ok {
		return nil
	}
	var many []string
	if json.Unmarshal(v, &many) == nil {
		return many
	}
	var one string
	if json.Unmarshal(v, &one) == nil && one != "" {
		return []string{one}
	}
	return nil
}

// details are what the claims say of the account, kept on its identity.
func (c oidcClaims) details() store.IdentityDetails {
	return store.IdentityDetails{Email: c.Email, Name: c.Name, Username: c.PreferredUsername, Groups: c.groupsOf("groups")}
}

// verified reports whether the provider vouches for the e-mail: true,
// or "true" as some providers write it.
func (c oidcClaims) verified() bool {
	v := strings.Trim(string(c.EmailVerified), `"`)
	return v == "true"
}

// audiences is aud, one or a list.
func (c oidcClaims) audiences() []string {
	var one string
	if json.Unmarshal(c.Audience, &one) == nil {
		return []string{one}
	}
	var many []string
	json.Unmarshal(c.Audience, &many)
	return many
}

// exchange trades the code for the provider's tokens and returns the
// id_token's claims, checked against the provider and the sign-in.
func (s *Server) exchange(ctx context.Context, p store.AuthProvider, started *oidcStarted, code string) (oidcClaims, error) {
	d, err := s.oidcDiscover(ctx, p)
	if err != nil {
		return oidcClaims{}, err
	}
	form := url.Values{"grant_type": {"authorization_code"}, "code": {code}, "redirect_uri": {started.redirect + oidcCallback}, "code_verifier": {started.verifier}}
	post := len(d.TokenAuthMethods) > 0 && !slices.Contains(d.TokenAuthMethods, "client_secret_basic") && slices.Contains(d.TokenAuthMethods, "client_secret_post")
	if post {
		form.Set("client_id", p.ClientID)
		form.Set("client_secret", p.ClientSecret)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, d.TokenEndpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return oidcClaims{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	if !post {
		// RFC 6749, 2.3.1: each form-encoded before the basic scheme
		req.SetBasicAuth(url.QueryEscape(p.ClientID), url.QueryEscape(p.ClientSecret))
	}
	resp, err := oidcClient.Do(req)
	if err != nil {
		return oidcClaims{}, fmt.Errorf("token: %w", err)
	}
	defer resp.Body.Close()
	var answer struct {
		IDToken     string `json:"id_token"`
		AccessToken string `json:"access_token"`
		Error       string `json:"error"`
	}
	json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&answer)
	if resp.StatusCode != http.StatusOK || answer.IDToken == "" {
		return oidcClaims{}, fmt.Errorf("token: %s %s", resp.Status, answer.Error)
	}
	var c oidcClaims
	if err := jwtClaims(answer.IDToken, &c); err != nil {
		return oidcClaims{}, fmt.Errorf("id_token: %w", err)
	}
	exp := time.Unix(int64(c.Expiry), 0)
	switch {
	case c.Issuer != p.Issuer:
		return oidcClaims{}, fmt.Errorf("id_token: issuer %q, not %q", c.Issuer, p.Issuer)
	case !slices.Contains(c.audiences(), p.ClientID):
		return oidcClaims{}, errors.New("id_token: not for this client")
	case c.AuthorizedParty != "" && c.AuthorizedParty != p.ClientID:
		return oidcClaims{}, errors.New("id_token: authorized for another client")
	case c.Expiry == 0 || time.Now().After(exp.Add(oidcLeeway)):
		return oidcClaims{}, errors.New("id_token: expired")
	case c.Nonce != started.nonce:
		return oidcClaims{}, errors.New("id_token: the nonce is not this sign-in's")
	case c.Subject == "":
		return oidcClaims{}, errors.New("id_token: no subject")
	}
	if d.UserinfoEndpoint != "" && answer.AccessToken != "" {
		if err := s.userinfo(ctx, d.UserinfoEndpoint, answer.AccessToken, &c); err != nil {
			if errors.Is(err, errOtherSubject) {
				return oidcClaims{}, err
			}
			s.log.Warn("api: OpenID Connect userinfo not read; the id_token's claims alone", "provider", p.Key, "error", err)
		}
	}
	return c, nil
}

// jwtClaims decodes the payload of a JWT into c, unchecked: the token
// came straight from the provider over TLS.
func jwtClaims(token string, c *oidcClaims) error {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return errors.New("not a JWT")
	}
	raw, err := base64.RawURLEncoding.DecodeString(strings.TrimRight(parts[1], "="))
	if err != nil {
		return err
	}
	return decodeClaims(raw, c)
}

// errOtherSubject is a userinfo answer for another subject than the
// id_token's, which OpenID Connect Core 1.0, 5.3.4 says must not be
// used: the sign-in fails.
var errOtherSubject = errors.New("userinfo: the answer is for another subject")

// userinfo reads the provider's userinfo endpoint with the access token
// and fills in the claims the id_token lacks: the e-mail with its
// verification, the name and the user name. An answer may be JSON or,
// where the client asked for it signed, a JWT.
func (s *Server) userinfo(ctx context.Context, endpoint, accessToken string, c *oidcClaims) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/json")
	resp, err := oidcClient.Do(req)
	if err != nil {
		return fmt.Errorf("userinfo: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("userinfo: %s", resp.Status)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return fmt.Errorf("userinfo: %w", err)
	}
	var u oidcClaims
	if strings.HasPrefix(resp.Header.Get("Content-Type"), "application/jwt") {
		err = jwtClaims(strings.TrimSpace(string(body)), &u)
	} else {
		err = decodeClaims(body, &u)
	}
	if err != nil {
		return fmt.Errorf("userinfo: %w", err)
	}
	if u.Subject != c.Subject {
		return errOtherSubject
	}
	c.fill(u)
	return nil
}

// fill takes from u, the userinfo's claims, what c lacks: the e-mail
// with its verification, or the verification of the same e-mail, the
// name and the user name.
func (c *oidcClaims) fill(u oidcClaims) {
	switch {
	case c.Email == "":
		c.Email, c.EmailVerified = u.Email, u.EmailVerified
	case len(c.EmailVerified) == 0 && strings.EqualFold(c.Email, u.Email):
		c.EmailVerified = u.EmailVerified
	}
	c.Name = cmpOr(c.Name, u.Name)
	c.PreferredUsername = cmpOr(c.PreferredUsername, u.PreferredUsername)
	for k, v := range u.raw {
		if _, ok := c.raw[k]; !ok {
			if c.raw == nil {
				c.raw = map[string]json.RawMessage{}
			}
			c.raw[k] = v
		}
	}
}

// autoRoleAllowed checks the role a provider gives the users it creates:
// one that exists, not owner, and holding none of adminCapabilities.
func autoRoleAllowed(ctx context.Context, st *store.Store, key string) error {
	if key == "" {
		key = store.RoleViewer
	}
	if key == store.RoleOwner {
		return fmt.Errorf("the role %s may not be given to users a provider creates", key)
	}
	role, ok, err := st.Role(ctx, key)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("%w: %s", store.ErrNoRole, key)
	}
	for _, c := range role.Capabilities {
		if slices.Contains(adminCapabilities, c) {
			return fmt.Errorf("the role %s holds %s, which users a provider creates may not have", key, c)
		}
	}
	return nil
}

// nameChars are the characters a login name may not have.
var nameChars = regexp.MustCompile(`[^a-z0-9._@-]+`)

// newUserName makes a login name of a provider's claim: lower case, the
// characters a name may not have replaced, at most 60 bytes, starting
// with a letter or a digit; empty where nothing is left.
func newUserName(v string) string {
	v = nameChars.ReplaceAllString(strings.ToLower(strings.TrimSpace(v)), "-")
	v = strings.TrimLeft(v, "._@-")
	if len(v) > 60 {
		v = v[:60]
	}
	return strings.TrimRight(v, "._@-")
}

// oidcUser finds the user of a provider's claims: by its subject, else
// by a pre-link of its verified e-mail, which it binds, else a user
// created where the provider creates users for the e-mail's domain. The
// reason says why there is none.
func (s *Server) oidcUser(ctx context.Context, p store.AuthProvider, c oidcClaims) (store.User, int64, string, error) {
	st := s.opts.Store
	if u, i, ok, err := st.UserBySubject(ctx, p.ID, c.Subject); err != nil || ok {
		return u, i.ID, "", err
	}
	if c.Email != "" && c.verified() {
		if u, i, ok, err := st.BindIdentity(ctx, p.ID, c.Email, c.Subject); err != nil || ok {
			if ok {
				s.log.Info("OpenID Connect identity bound", "provider", p.Key, "user", u.Name)
			}
			return u, i.ID, "", err
		}
	}
	if c.Email == "" {
		return store.User{}, 0, oidcNoEmail, nil
	}
	if !c.verified() {
		// unverified where it would have counted: a waiting e-mail, or a
		// user created
		waits, err := st.EmailWaits(ctx, p.ID, c.Email)
		if err != nil {
			return store.User{}, 0, "", err
		}
		if waits || p.AutoCreate && p.AllowsDomain(c.Email) {
			return store.User{}, 0, oidcUnverified, nil
		}
		return store.User{}, 0, oidcRefused, nil
	}
	if !p.AutoCreate || !p.AllowsDomain(c.Email) {
		return store.User{}, 0, oidcRefused, nil
	}
	if err := autoRoleAllowed(ctx, st, p.AutoRole); err != nil {
		s.log.Warn("api: a provider's users are not created", "provider", p.Key, "error", err)
		return store.User{}, 0, oidcRefused, nil
	}
	role := cmpOr(p.AutoRole, store.RoleViewer)
	name := newUserName(c.PreferredUsername)
	if name == "" {
		name = newUserName(c.Email[:strings.LastIndexByte(c.Email, '@')])
	}
	if name == "" {
		name = "user"
	}
	u, i, err := st.AddIdentityUser(ctx, store.User{Name: name, DisplayName: strings.TrimSpace(c.Name), Role: role}, p.ID, c.Subject, c.Email)
	if errors.Is(err, store.ErrBadName) {
		u, i, err = st.AddIdentityUser(ctx, store.User{Name: "user", DisplayName: strings.TrimSpace(c.Name), Role: role}, p.ID, c.Subject, c.Email)
	}
	if err != nil {
		return store.User{}, 0, "", err
	}
	s.log.Info("user created by an OpenID Connect sign-in", "provider", p.Key, "user", u.Name, "role", u.Role)
	return u, i.ID, "", nil
}

// oidcCallback handles GET /api/v1/login/oidc/callback, where the
// provider sends the browser: the code traded, the user found, and the
// browser sent back to the page with a one-time code in the fragment, or
// with the reason it was refused. A refusal counts as a failed sign-in
// of the address.
func (s *Server) oidcCallback(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	q := r.URL.Query()
	started, ok := s.oidc.takeStarted(q.Get("state"), time.Now())
	if !ok {
		return fail(http.StatusBadRequest, "no sign-in started under this state, or it took too long; sign in again from the page")
	}
	ip := s.clientIP(r)
	back := func(fragment string) error {
		w.Header().Set("Cache-Control", "no-store")
		http.Redirect(w, r, started.redirect+"#"+fragment, http.StatusSeeOther)
		return nil
	}
	refuse := func(reason, who string, err error) error {
		if err != nil {
			s.log.Warn("api: OpenID Connect sign-in failed", "provider", started.provider, "remote", ip, "error", err)
		} else {
			s.log.Warn("api: OpenID Connect sign-in refused", "provider", started.provider, "remote", ip, "account", who, "reason", reason)
		}
		if reason != oidcDenied {
			if s.limiter.fail(ctx, store.LockoutAddr(ip)) {
				s.logLockout(ip, "logins")
			}
			s.logSignIn("login_failed", who, 0, ip)
		}
		return back("oidc-error=" + reason)
	}
	if e := q.Get("error"); e != "" {
		return refuse(oidcDenied, "", fmt.Errorf("the provider answered %s", e))
	}
	p, ok, err := s.opts.Store.Provider(ctx, started.provider)
	if err != nil || !ok || !p.Enabled {
		return refuse(oidcFailed, "", fmt.Errorf("the provider %s is gone or disabled: %v", started.provider, err))
	}
	claims, err := s.exchange(ctx, p, started, q.Get("code"))
	if err != nil {
		if started.linkUser != 0 {
			s.log.Warn("api: OpenID Connect link failed", "provider", started.provider, "remote", ip, "error", err)
			return back("oidc-link-error=" + oidcFailed)
		}
		return refuse(oidcFailed, "", err)
	}
	if started.linkUser != 0 {
		return back(s.oidcLink(ctx, p, started.linkUser, claims, ip))
	}
	who := cmpOr(claims.Email, p.Key+":"+claims.Subject)
	u, identity, reason, err := s.oidcUser(ctx, p, claims)
	switch {
	case err != nil:
		return refuse(oidcFailed, who, err)
	case reason != "":
		return refuse(reason, who, nil)
	case u.Disabled || u.Expired(time.Now()):
		return refuse(oidcDisabled, u.Name, nil)
	}
	if err := s.opts.Store.TouchIdentity(ctx, identity, claims.details()); err != nil {
		s.log.Warn("api: identity not touched", "error", err)
	}
	code := s.oidc.prove(&oidcProven{userID: u.ID, provider: p.Key, ip: ip, expires: time.Now().Add(oidcCodeTTL)}, time.Now())
	if code == "" {
		return refuse(oidcFailed, u.Name, store.ErrBusy)
	}
	return back("oidc=" + code)
}

// oidcLink links the account of the claims to the user who started the
// link (AP Oidc v1.1, Block 7) and returns the fragment for the page:
// oidc-linked=<key>, or oidc-link-error=taken where another user has
// the account (failed where it failed), which a page signed in shows in
// My profile, not on the sign-in card. A link is no sign-in: no session, nothing counted against
// the address.
func (s *Server) oidcLink(ctx context.Context, p store.AuthProvider, userID int64, c oidcClaims, ip string) string {
	st := s.opts.Store
	u, ok, err := st.UserByID(ctx, userID)
	if err != nil || !ok {
		s.log.Warn("api: OpenID Connect link failed", "provider", p.Key, "remote", ip, "error", cmpOr(fmt.Sprint(err), "the user is gone"))
		return "oidc-link-error=" + oidcFailed
	}
	if _, err := st.LinkIdentity(ctx, u.ID, p.ID, c.Subject, c.details()); errors.Is(err, store.ErrIdentityTaken) {
		s.log.Warn("api: OpenID Connect link refused", "provider", p.Key, "user", u.Name, "remote", ip, "reason", oidcTaken)
		return "oidc-link-error=" + oidcTaken
	} else if err != nil {
		s.log.Warn("api: OpenID Connect link failed", "provider", p.Key, "user", u.Name, "remote", ip, "error", err)
		return "oidc-link-error=" + oidcFailed
	}
	s.log.Info("account linked", "provider", p.Key, "user", u.Name, "remote", ip)
	return "oidc-linked=" + p.Key
}

// oidcStartLink handles POST /api/v1/users/{name}/identities/link/{key}:
// a user links its own account at a provider (AP Oidc v1.1, D21-A), a
// sign-in there whose callback binds the account to the user. Only the
// user itself starts it: nobody else signs in at the provider for it.
func (s *Server) oidcStartLink(w http.ResponseWriter, r *http.Request) error {
	st, err := s.userStore()
	if err != nil {
		return err
	}
	u, a, err := s.targetUser(r, st, true)
	if err != nil {
		return err
	}
	auditTarget(r, "user:"+u.Name)
	if !a.self {
		return fail(http.StatusForbidden, "a user links only its own accounts")
	}
	var req oidcStartRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return fail(http.StatusBadRequest, "body must have \"redirect\", the page's address")
	}
	redirect, ok := pageAddress(req.Redirect)
	if !ok {
		return fail(http.StatusBadRequest, "redirect: an http or https address without a query or fragment")
	}
	key := strings.ToLower(r.PathValue("key"))
	p, ok, err := st.Provider(r.Context(), key)
	if err != nil {
		return fail(http.StatusInternalServerError, "%v", err)
	}
	if !ok || !p.Enabled {
		return fail(http.StatusNotFound, "no provider %q", key)
	}
	auditSubject(r, map[string]any{"provider": p.Key, "user": u.Name})
	return s.oidcStartFlow(w, r, p, redirect, u.ID)
}

// oidcFinishRequest is the body of POST /api/v1/login/oidc/finish.
type oidcFinishRequest struct {
	Code  string `json:"code"`
	Label string `json:"label,omitempty"`
}

// oidcFinish handles POST /api/v1/login/oidc/finish: the callback's code
// traded for a session, from the address the callback came from, once.
func (s *Server) oidcFinish(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	if err := s.loginMode(ctx); err != nil {
		return err
	}
	var req oidcFinishRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Code == "" {
		return fail(http.StatusBadRequest, "body must have \"code\"")
	}
	proven, ok := s.oidc.takeProven(req.Code, s.clientIP(r), time.Now())
	if !ok {
		return fail(http.StatusUnauthorized, "no sign-in under this code, or it took too long; sign in again")
	}
	// by its id, and live under its name: not removed since the callback
	u, ok, err := s.opts.Store.UserByID(ctx, proven.userID)
	if err == nil && ok {
		var live store.User
		live, ok, err = s.opts.Store.User(ctx, u.Name)
		ok = ok && live.ID == u.ID
	}
	if err != nil || !ok {
		return fail(http.StatusUnauthorized, "the user is gone; sign in again")
	}
	auditTarget(r, "user:"+u.Name)
	return s.openSession(w, r, u, req.Label, map[string]any{"provider": proven.provider})
}
