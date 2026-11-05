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
	"net/http"
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
	API  string // https://api.github.com
	Repo string
	HTTP *http.Client
	Key  ed25519.PublicKey
}

// NewClient returns a client for Repo with the compiled-in key.
func NewClient() *Client {
	return &Client{API: "https://api.github.com", Repo: Repo, HTTP: &http.Client{Timeout: 2 * time.Minute}, Key: ReleaseKey()}
}

type ghRelease struct {
	TagName    string `json:"tag_name"`
	HTMLURL    string `json:"html_url"`
	Draft      bool   `json:"draft"`
	Prerelease bool   `json:"prerelease"`
	Assets     []struct {
		Name string `json:"name"`
		URL  string `json:"browser_download_url"`
	} `json:"assets"`
}

func (c *Client) get(ctx context.Context, url string, limit int64, accept string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	if accept != "" {
		req.Header.Set("Accept", accept)
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s: %s", url, resp.Status)
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

// verify fetches a release's manifest and signature and checks both.
func (c *Client) verify(ctx context.Context, gr ghRelease) (*Release, error) {
	r := &Release{URL: gr.HTMLURL, assets: map[string]string{}}
	for _, a := range gr.Assets {
		r.assets[a.Name] = a.URL
	}
	if r.assets[ManifestName] == "" || r.assets[SignatureName] == "" {
		return nil, fmt.Errorf("release %s has no signed manifest", gr.TagName)
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
// the channel has none. Releases are tried newest first by version, and
// one that fails verification is skipped, its error kept for when no
// release at all verifies.
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
			if firstErr == nil {
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
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GET %s: %s", url, resp.Status)
	}
	h := sha256.New()
	n, err := io.Copy(io.MultiWriter(w, h), io.LimitReader(resp.Body, MaxBinarySize+1))
	if err != nil {
		return err
	}
	if n > MaxBinarySize {
		return fmt.Errorf("%s is larger than %d bytes", name, MaxBinarySize)
	}
	if got := hex.EncodeToString(h.Sum(nil)); !strings.EqualFold(got, want) {
		return fmt.Errorf("%s: SHA-256 %s, the signed manifest says %s", name, got, want)
	}
	return nil
}
