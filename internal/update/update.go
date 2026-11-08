// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

// Package update finds Massimo releases on GitHub, verifies them and
// replaces the running binary with a newer one.
//
// Every release carries massimo-release.json, a manifest naming the tag,
// the commit, the update channel (the branch the tag was made on) and the
// SHA-256 of every binary, and massimo-release.json.sig, an Ed25519
// signature of the manifest's exact bytes made by the release workflow
// with a key that only tag builds can use. The public key is compiled in
// (release-key.pem), so a binary accepts only what that key signed: a
// release, an asset or a manifest changed on GitHub or on the way fails
// verification. SHA256SUMS is published as well, for people checking a
// download by hand.
package update

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"crypto/x509"
	_ "embed"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"regexp"
	"runtime"
	"runtime/debug"
	"strings"
	"time"
)

// Repo is the GitHub repository releases come from.
const Repo = "metaworx/massimo"

// Names of the release assets beside the binaries.
const (
	ManifestName  = "massimo-release.json"
	SignatureName = ManifestName + ".sig"
)

// MaxBinarySize bounds a download; today's binaries are about 10 MB.
const MaxBinarySize = 64 << 20

const maxManifestSize = 64 << 10

// maxVerify bounds how many releases Latest fetches manifests for.
const maxVerify = 30

//go:embed release-key.pem
var releaseKeyPEM []byte

// ReleaseKey returns the compiled-in public key releases are signed with.
func ReleaseKey() ed25519.PublicKey {
	k, err := ParsePublicKey(releaseKeyPEM)
	if err != nil {
		panic("update: bad compiled-in release key: " + err.Error())
	}
	return k
}

// ParsePublicKey reads an Ed25519 public key in PEM ("BEGIN PUBLIC KEY",
// as `openssl pkey -pubout` writes it).
func ParsePublicKey(b []byte) (ed25519.PublicKey, error) {
	blk, _ := pem.Decode(b)
	if blk == nil {
		return nil, errors.New("no PEM block")
	}
	k, err := x509.ParsePKIXPublicKey(blk.Bytes)
	if err != nil {
		return nil, err
	}
	ek, ok := k.(ed25519.PublicKey)
	if !ok {
		return nil, fmt.Errorf("%T is not an Ed25519 key", k)
	}
	return ek, nil
}

// Manifest is the signed description of one release.
type Manifest struct {
	Tag        string            `json:"tag"`
	Commit     string            `json:"commit"`
	Channel    string            `json:"channel"`
	Prerelease bool              `json:"prerelease"`
	Assets     map[string]string `json:"assets"` // file name → SHA-256, hex
}

// Release is a GitHub release whose manifest verified.
type Release struct {
	Manifest
	Version Version
	URL     string            // the release page
	assets  map[string]string // file name → download URL
}

var channelRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._/-]{0,99}$`)

// ValidChannel reports whether s can name a channel (a git branch name of
// the plain kind).
func ValidChannel(s string) bool { return channelRe.MatchString(s) && !strings.Contains(s, "..") }

// AssetName is the release file holding a binary for this platform, as
// the Makefile names them: massimo-linux-armv6, massimo-windows-amd64.exe.
func AssetName() string {
	return assetName(runtime.GOOS, runtime.GOARCH, goarm())
}

func assetName(goos, goarch, arm string) string {
	arch := goarch
	if goarch == "arm" {
		arch = "armv" + arm
	}
	name := "massimo-" + goos + "-" + arch
	if goos == "windows" {
		name += ".exe"
	}
	return name
}

// goarm returns the ARM version this binary was built for ("6", "7"),
// from the build settings Go records; armv6 binaries run everywhere, so
// an unknown value gives 6.
func goarm() string {
	if bi, ok := debug.ReadBuildInfo(); ok {
		for _, s := range bi.Settings {
			if s.Key == "GOARM" && s.Value != "" {
				return s.Value[:1] // "7" or "7,softfloat"
			}
		}
	}
	return "6"
}

// Client talks to the GitHub releases API.
type Client struct {
	API   string // https://api.github.com
	Repo  string
	HTTP  *http.Client
	Key   ed25519.PublicKey
	Token string // optional, for the API only: see TokenVar
	// Idle is the longest pause in a download's data, DownloadIdle
	// when 0; the download itself has no time limit of the client's.
	Idle time.Duration
}

// DownloadIdle is how long a download may go without data: a Pi 1 on a
// slow line takes minutes for a binary, which the API's two minutes
// would cut off, while a stalled one should not wait for ever.
const DownloadIdle = time.Minute

// TokenVar names an optional GitHub token for the release list. Without
// one GitHub allows 60 API requests an hour per address, which a shared
// address (a CI runner, a large NAT) can use up; a token of any kind,
// with no rights at all, raises that. It only lifts the limit: what is
// installed is still decided by the signature against the compiled-in
// key.
const TokenVar = "MASSIMO_GITHUB_TOKEN"

// APIVar names the API base to use instead of https://api.github.com,
// for tests only: the CI job serves its own releases there, signed with
// a test key compiled into the binaries it builds. A release still has
// to verify against the compiled-in key, so pointing it elsewhere
// installs nothing a release binary would not; the token is not sent
// there.
const APIVar = "MASSIMO_UPDATE_API"

// NewClient returns a client for Repo with the compiled-in key and the
// token from TokenVar, if set.
func NewClient() *Client {
	c := &Client{API: "https://api.github.com", Repo: Repo, HTTP: &http.Client{Timeout: 2 * time.Minute}, Key: ReleaseKey(), Token: os.Getenv(TokenVar)}
	if api := strings.TrimRight(os.Getenv(APIVar), "/"); api != "" {
		c.API, c.Token = api, ""
	}
	return c
}

type ghRelease struct {
	TagName         string `json:"tag_name"`
	TargetCommitish string `json:"target_commitish"`
	HTMLURL         string `json:"html_url"`
	Draft           bool   `json:"draft"`
	Prerelease      bool   `json:"prerelease"`
	Assets          []struct {
		Name string `json:"name"`
		URL  string `json:"browser_download_url"`
	} `json:"assets"`
}

// retryWaits are the pauses before a request is sent again, when GitHub
// answered 502, 503 or 504 or the connection failed before an answer
// (retryable): its hosts have outages of seconds, which the server's
// check would otherwise keep for checkEvery. A variable for the tests.
var retryWaits = []time.Duration{2 * time.Second, 5 * time.Second}

// answerWait is how long an attempt waits for GitHub's answer, its status
// and headers after any redirects, before it is sent again: on 2026-11-08
// github.com held requests without a token until the server's check ran
// out. The body has its own bounds, the client's Timeout or a download's
// idle time: a release list of some MiB takes a Pi on a slow line longer.
// A variable for the tests.
var answerWait = 10 * time.Second

// errNoAnswer ends an attempt whose answer did not come within answerWait.
var errNoAnswer = errors.New("no answer")

// retryable says whether a request that got resp or err is worth sending
// again: a gateway's or server's passing state, a connection that failed
// before an answer, or an answer that did not come within answerWait.
// Not a timeout of the client's, which gave the request its time, nor
// the end of ctx; and not GitHub's rate limits (403, 429), where seconds
// do not help and a token does (TokenVar).
func retryable(ctx context.Context, resp *http.Response, err error) bool {
	if err != nil {
		if ctx.Err() != nil {
			return false
		}
		var ne net.Error
		return errors.Is(err, errNoAnswer) || !(errors.As(err, &ne) && ne.Timeout())
	}
	switch resp.StatusCode {
	case http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return true
	}
	return false
}

// do sends the request newReq makes with an attempt's context (send),
// and again after each of retryWaits while retryable says so. It returns
// the last response or error as it came, and the number of attempts; a
// wait ends with ctx.
func do(ctx context.Context, hc *http.Client, newReq func(context.Context) (*http.Request, error)) (*http.Response, int, error) {
	for attempt := 1; ; attempt++ {
		actx, end := context.WithCancelCause(ctx)
		req, err := newReq(actx)
		if err != nil {
			end(nil)
			return nil, attempt, err
		}
		resp, err := send(hc, req, end)
		if attempt > len(retryWaits) || !retryable(ctx, resp, err) {
			return resp, attempt, err
		}
		wait := time.NewTimer(retryWaits[attempt-1])
		select {
		case <-ctx.Done():
			wait.Stop()
			return resp, attempt, err
		case <-wait.C:
		}
		if resp != nil {
			io.Copy(io.Discard, io.LimitReader(resp.Body, 4<<10)) // so that the connection is reused
			resp.Body.Close()
		}
	}
}

// send sends one attempt, req made with a context that end ends: with
// errNoAnswer when no answer has come within answerWait, and otherwise
// once the response's body is closed.
func send(hc *http.Client, req *http.Request, end context.CancelCauseFunc) (*http.Response, error) {
	late := time.AfterFunc(answerWait, func() { end(errNoAnswer) })
	resp, err := hc.Do(req)
	if !late.Stop() {
		// The wait ran out, perhaps as the answer came: its body would
		// be cut off with the context.
		end(errNoAnswer)
		if err == nil {
			resp.Body.Close()
			err = req.Context().Err()
		}
		if errors.Is(context.Cause(req.Context()), errNoAnswer) {
			err = fmt.Errorf("GET %s: %w within %s", req.URL.Redacted(), errNoAnswer, answerWait)
		}
		return nil, err
	}
	if err != nil {
		end(nil)
		return nil, err
	}
	resp.Body = endingBody{resp.Body, end}
	return resp, nil
}

// endingBody ends its attempt's context when it is closed.
type endingBody struct {
	io.ReadCloser
	end context.CancelCauseFunc
}

func (b endingBody) Close() error {
	err := b.ReadCloser.Close()
	b.end(nil)
	return err
}

// attempts is what an error adds after more than one attempt.
func attempts(n int) string {
	if n < 2 {
		return ""
	}
	return fmt.Sprintf(" (%d attempts)", n)
}

func (c *Client) get(ctx context.Context, url string, limit int64, accept string) ([]byte, error) {
	resp, n, err := do(ctx, c.HTTP, func(actx context.Context) (*http.Request, error) {
		req, err := http.NewRequestWithContext(actx, http.MethodGet, url, nil)
		if err != nil {
			return nil, err
		}
		if accept != "" {
			req.Header.Set("Accept", accept)
		}
		// Only to the API: the release files come from other hosts.
		if c.Token != "" && strings.HasPrefix(url, c.API+"/") {
			req.Header.Set("Authorization", "Bearer "+c.Token)
		}
		return req, nil
	})
	if err != nil {
		return nil, fmt.Errorf("%w%s", err, attempts(n))
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s: %s%s", url, resp.Status, attempts(n))
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(b)) > limit {
		return nil, fmt.Errorf("GET %s: more than %d bytes", url, limit)
	}
	return b, nil
}

func (c *Client) list(ctx context.Context) ([]ghRelease, error) {
	b, err := c.get(ctx, c.API+"/repos/"+c.Repo+"/releases?per_page=100", 8<<20, "application/vnd.github+json")
	if err != nil {
		return nil, err
	}
	var rels []ghRelease
	if err := json.Unmarshal(b, &rels); err != nil {
		return nil, fmt.Errorf("releases: %w", err)
	}
	return rels, nil
}

// errNoManifest is a release from before the signed manifests: no
// channel's, and no reason why a channel has no release.
var errNoManifest = errors.New("no signed manifest")

// verify fetches a release's manifest and signature and checks both.
func (c *Client) verify(ctx context.Context, gr ghRelease) (*Release, error) {
	r := &Release{URL: gr.HTMLURL, assets: map[string]string{}}
	for _, a := range gr.Assets {
		r.assets[a.Name] = a.URL
	}
	if r.assets[ManifestName] == "" || r.assets[SignatureName] == "" {
		return nil, fmt.Errorf("release %s has %w", gr.TagName, errNoManifest)
	}
	manifest, err := c.get(ctx, r.assets[ManifestName], maxManifestSize, "")
	if err != nil {
		return nil, err
	}
	sig, err := c.get(ctx, r.assets[SignatureName], 4<<10, "")
	if err != nil {
		return nil, err
	}
	m, err := VerifyManifest(c.Key, manifest, sig)
	if err != nil {
		return nil, fmt.Errorf("release %s: %w", gr.TagName, err)
	}
	if m.Tag != gr.TagName {
		return nil, fmt.Errorf("release %s carries the manifest of %s", gr.TagName, m.Tag)
	}
	r.Manifest = *m
	r.Version, _ = ParseVersion(m.Tag)
	return r, nil
}

// VerifyManifest checks the signature (base64, as openssl and base64
// write it) over the manifest's bytes and parses the manifest.
func VerifyManifest(key ed25519.PublicKey, manifest, sig []byte) (*Manifest, error) {
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(sig)))
	if err != nil {
		return nil, fmt.Errorf("signature: %w", err)
	}
	if !ed25519.Verify(key, manifest, raw) {
		return nil, errors.New("signature does not verify against the release key")
	}
	var m Manifest
	if err := json.Unmarshal(manifest, &m); err != nil {
		return nil, fmt.Errorf("manifest: %w", err)
	}
	if _, ok := ParseVersion(m.Tag); !ok {
		return nil, fmt.Errorf("manifest names no version tag (%q)", m.Tag)
	}
	return &m, nil
}

// Latest returns the newest signed release on the channel, or nil when
// the channel has none. A release whose tag or target names another
// channel (hintedChannel) is left out unfetched; the others are tried
// newest first by version, and one that fails verification is skipped,
// its error kept for when no release of the channel verifies; one
// without a manifest, from before the manifests, is skipped without an
// error.
func (c *Client) Latest(ctx context.Context, channel string) (*Release, error) {
	rels, err := c.list(ctx)
	if err != nil {
		return nil, err
	}
	type cand struct {
		gr ghRelease
		v  Version
	}
	var cands []cand
	for _, gr := range rels {
		if hint := hintedChannel(gr); hint != "" && hint != channel {
			continue
		}
		if v, ok := ParseVersion(gr.TagName); ok && !gr.Draft && v.Tag() {
			cands = append(cands, cand{gr, v})
		}
	}
	// Newest first; a handful of releases, so a simple insertion sort.
	for i := 1; i < len(cands); i++ {
		for j := i; j > 0 && Compare(cands[j].v, cands[j-1].v) > 0; j-- {
			cands[j], cands[j-1] = cands[j-1], cands[j]
		}
	}
	var firstErr error
	if len(cands) > maxVerify {
		cands = cands[:maxVerify] // a channel without a recent release has none worth installing
	}
	for _, cd := range cands {
		r, err := c.verify(ctx, cd.gr)
		if err != nil {
			if firstErr == nil && !errors.Is(err, errNoManifest) {
				firstErr = err
			}
			continue
		}
		if r.Channel == channel {
			return r, nil
		}
	}
	return nil, firstErr
}

// labelRe is release.yml's rule for the channel a tag names: its
// pre-release label, as alpha of v0.9.0-alpha.1.
var labelRe = regexp.MustCompile(`^v[0-9][0-9.]*-([a-z][a-z0-9]*)\.[0-9]+$`)

// hintedChannel is the channel a release names before its manifest is
// fetched, "" where it names none: release.yml makes the label of a tag
// its channel (labelRe) and creates the release with the channel as its
// target, which a tag without a label goes by. Neither is signed, so the
// hint only spares the requests for another channel's release; the
// manifest decides. The two releases before v0.1.0-alpha.12 are on
// phase-0 under alpha's label and are no longer found there.
func hintedChannel(gr ghRelease) string {
	if m := labelRe.FindStringSubmatch(gr.TagName); m != nil {
		return m[1]
	}
	return gr.TargetCommitish
}

// ByTag returns the signed release with this tag, on whatever channel.
func (c *Client) ByTag(ctx context.Context, tag string) (*Release, error) {
	rels, err := c.list(ctx)
	if err != nil {
		return nil, err
	}
	for _, gr := range rels {
		if gr.TagName == tag && !gr.Draft {
			return c.verify(ctx, gr)
		}
	}
	return nil, fmt.Errorf("no release %s", tag)
}

// Download writes the named binary of the release to w and fails unless
// its SHA-256 is the one the manifest states; w then holds unverified
// bytes and must be discarded.
func (c *Client) Download(ctx context.Context, r *Release, name string, w io.Writer) error {
	want, ok := r.Assets[name]
	if !ok {
		return fmt.Errorf("release %s has no %s", r.Tag, name)
	}
	url := r.assets[name]
	if url == "" {
		return fmt.Errorf("release %s lists %s in its manifest but has no such file", r.Tag, name)
	}
	// Bounded by ctx and by a pause in the data, not by the client's
	// Timeout, which counts the whole body.
	idle := c.Idle
	if idle <= 0 {
		idle = DownloadIdle
	}
	ctx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)
	stall := time.AfterFunc(idle, func() { cancel(fmt.Errorf("GET %s: no data for %s", url, idle)) })
	defer stall.Stop()
	stalled := func(err error) error {
		if cause := context.Cause(ctx); cause != nil && !errors.Is(cause, context.Canceled) && !errors.Is(cause, context.DeadlineExceeded) {
			return cause
		}
		return err
	}
	var hc http.Client
	if c.HTTP != nil {
		hc = *c.HTTP
	}
	hc.Timeout = 0
	// Sent again as do says until the body starts: from then on w holds
	// its bytes, and a failure ends the download. The idle time counts
	// from each attempt; the answer wait, shorter, ends a start that
	// hangs first.
	resp, tries, err := do(ctx, &hc, func(actx context.Context) (*http.Request, error) {
		stall.Reset(idle)
		return http.NewRequestWithContext(actx, http.MethodGet, url, nil)
	})
	if err != nil {
		return stalled(fmt.Errorf("%w%s", err, attempts(tries)))
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GET %s: %s%s", url, resp.Status, attempts(tries))
	}
	h := sha256.New()
	n, err := io.Copy(io.MultiWriter(w, h), io.LimitReader(idleReader{resp.Body, stall, idle}, MaxBinarySize+1))
	if err != nil {
		return stalled(err)
	}
	if n > MaxBinarySize {
		return fmt.Errorf("%s is larger than %d bytes", name, MaxBinarySize)
	}
	if got := hex.EncodeToString(h.Sum(nil)); !strings.EqualFold(got, want) {
		return fmt.Errorf("%s: SHA-256 %s, the signed manifest says %s", name, got, want)
	}
	return nil
}

// idleReader moves a download's stall timer on with every read that
// brings data.
type idleReader struct {
	r     io.Reader
	stall *time.Timer
	idle  time.Duration
}

func (r idleReader) Read(p []byte) (int, error) {
	n, err := r.r.Read(p)
	if n > 0 {
		r.stall.Reset(r.idle)
	}
	return n, err
}
