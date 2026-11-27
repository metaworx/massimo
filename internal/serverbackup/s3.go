// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package serverbackup

// The S3 target (AP ServerBackup v1.0, Block 9, D10-A): a bucket of an
// S3-compatible store, AWS, MinIO, Backblaze B2, Wasabi, Cloudflare R2 or
// Hetzner among them, reached by five calls of the S3 REST API, signed
// with AWS Signature Version 4, all with the standard library: PUT, GET
// with a range, HEAD, DELETE and ListObjectsV2. A bucket is reached by
// path (https://host/bucket/key) or by virtual host
// (https://bucket.host/key), and the copies go under a prefix in it.

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/md5"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"encoding/base64"
	"encoding/hex"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"
)

// KindS3 is the S3 target's kind.
const KindS3 = "s3"

// The S3 target's limits and constants.
const (
	// s3MaxPut is the largest object one PUT takes (AWS's 5 GiB): the
	// target puts a backup in one piece, so a larger one is refused.
	s3MaxPut = 5 << 30
	// s3DefaultRegion is the region where none is given, AWS's first
	// and MinIO's own.
	s3DefaultRegion = "us-east-1"
	// s3Stall breaks off a request that moves no byte for this long;
	// s3HeaderWait is how long an answer may take once a request is sent.
	s3Stall      = 2 * time.Minute
	s3HeaderWait = 2 * time.Minute
	// s3MinAhead and s3MaxAhead bound what a ranged read fetches: a read
	// that follows the one before fetches twice as much as that did, so
	// that a copy read in small pieces costs few requests.
	s3MinAhead = 64 << 10
	s3MaxAhead = 8 << 20
	// s3MaxListPage and s3MaxErrorBody bound the answers read whole;
	// s3MaxPages the pages of one list.
	s3MaxListPage  = 16 << 20
	s3MaxErrorBody = 64 << 10
	s3MaxPages     = 10000
	// s3MaxPrefix bounds the prefix (a key is at most 1024 bytes).
	s3MaxPrefix = 512

	s3Algorithm       = "AWS4-HMAC-SHA256"
	s3AmzDate         = "20060102T150405Z"
	s3UnsignedPayload = "UNSIGNED-PAYLOAD"
	// s3EmptyHash is the SHA-256 of an empty body, in hex.
	s3EmptyHash = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
)

// errS3Stalled is a request broken off because nothing moved.
var errS3Stalled = errors.New("the request was broken off")

var (
	// s3Bucket is a bucket's name as a host name takes it; s3BucketPath
	// one that only a path takes, as old buckets have.
	s3Bucket     = regexp.MustCompile(`^[a-z0-9][a-z0-9.-]{1,61}[a-z0-9]$`)
	s3BucketPath = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{2,254}$`)
	s3Region     = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)
)

func init() {
	Register(Kind{
		Name: KindS3,
		Fields: []Field{
			{Key: "endpoint", Type: FieldText, Required: true},
			{Key: "region", Type: FieldText, Required: true, Default: s3DefaultRegion},
			{Key: "bucket", Type: FieldText, Required: true},
			{Key: "prefix", Type: FieldText},
			{Key: "path_style", Type: FieldBool, Default: "false"},
			{Key: "access_key", Type: FieldText, Required: true},
			{Key: "secret_key", Type: FieldSecret, Required: true},
		},
		New: func(cfg map[string]string, secret []byte) (Target, error) {
			return NewS3(cfg, secret)
		},
	})
}

// s3Transport is the S3 targets' one connection pool, with timeouts
// for the connection, the TLS handshake and the answer's header; a
// request's body is guarded by s3Stall, since an upload may take hours.
var s3Transport = sync.OnceValue(func() *http.Transport {
	return &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		DialContext:           (&net.Dialer{Timeout: 30 * time.Second, KeepAlive: 30 * time.Second}).DialContext,
		TLSClientConfig:       &tls.Config{MinVersion: tls.VersionTLS12},
		TLSHandshakeTimeout:   15 * time.Second,
		ResponseHeaderTimeout: s3HeaderWait,
		ExpectContinueTimeout: 2 * time.Second,
		IdleConnTimeout:       90 * time.Second,
		MaxIdleConns:          16,
		MaxIdleConnsPerHost:   4,
		ForceAttemptHTTP2:     true,
	}
})

// S3 is a target in a bucket of an S3-compatible store.
type S3 struct {
	scheme string // https, or http where the endpoint says so
	host   string // the endpoint's host, with its port where it is not the scheme's
	region string
	bucket string
	prefix string // "" or ending in /
	// pathStyle reaches the bucket by path, not by virtual host.
	pathStyle bool
	accessKey string
	secretKey []byte

	client *http.Client
	now    func() time.Time
	stall  time.Duration
}

// NewS3 returns the S3 target of the kind's fields and its secret key:
// endpoint, a URL without a path, https where it names no scheme;
// region; bucket; prefix, a folder in the bucket; path_style; access_key.
// A virtual host needs a bucket a host name takes, an endpoint by name,
// and over https a bucket without dots, which the endpoint's wildcard
// certificate does not cover.
func NewS3(cfg map[string]string, secret []byte) (*S3, error) {
	scheme, host, err := s3Endpoint(cfg["endpoint"])
	if err != nil {
		return nil, err
	}
	s := &S3{scheme: scheme, host: host, region: cfg["region"], bucket: cfg["bucket"], accessKey: cfg["access_key"],
		secretKey: bytes.Clone(secret), now: time.Now, stall: s3Stall,
		client: &http.Client{Transport: s3Transport(), CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}
	if s.region == "" {
		s.region = s3DefaultRegion
	}
	if v := cfg["path_style"]; v != "" {
		if s.pathStyle, err = strconv.ParseBool(v); err != nil {
			return nil, fmt.Errorf("serverbackup: S3 target: path_style %q: want true or false", v)
		}
	}
	if s.prefix, err = s3Prefix(cfg["prefix"]); err != nil {
		return nil, err
	}
	hostname := (&url.URL{Host: host}).Hostname()
	switch {
	case !s3Region.MatchString(s.region):
		return nil, fmt.Errorf("serverbackup: S3 target: region %q: letters, digits, - and _", s.region)
	case !s3BucketPath.MatchString(s.bucket) || strings.Contains(s.bucket, ".."):
		return nil, fmt.Errorf("serverbackup: S3 target: %q is no bucket's name", s.bucket)
	case !s.pathStyle && !s3Bucket.MatchString(s.bucket):
		return nil, fmt.Errorf("serverbackup: S3 target: the bucket %q is no host name; reach it by path (path_style)", s.bucket)
	case !s.pathStyle && net.ParseIP(hostname) != nil:
		return nil, errors.New("serverbackup: S3 target: an endpoint given by its address reaches a bucket by path (path_style)")
	case !s.pathStyle && s.scheme == "https" && strings.Contains(s.bucket, "."):
		return nil, fmt.Errorf("serverbackup: S3 target: the bucket %q has dots, which the endpoint's certificate does not cover; reach it by path (path_style)", s.bucket)
	case s.accessKey == "" || strings.ContainsFunc(s.accessKey, func(r rune) bool { return r <= ' ' || r > '~' || r == ',' || r == '=' }):
		return nil, errors.New("serverbackup: S3 target: the access key is printable ASCII, without spaces, commas or equals signs")
	case len(s.secretKey) == 0:
		return nil, errors.New("serverbackup: S3 target: the secret key is required")
	}
	return s, nil
}

// s3Endpoint reads an endpoint: a URL of scheme https or http, without
// a path, a query or a user; https where it names no scheme. The host is
// lower case, without the scheme's own port.
func s3Endpoint(v string) (scheme, host string, err error) {
	if v == "" {
		return "", "", errors.New("serverbackup: S3 target: the endpoint is required")
	}
	full := v
	if !strings.Contains(v, "://") {
		full = "https://" + v
	}
	u, err := url.Parse(full)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" || u.Hostname() == "" || u.User != nil ||
		(u.Path != "" && u.Path != "/") || u.RawQuery != "" || u.Fragment != "" || u.Opaque != "" {
		return "", "", fmt.Errorf("serverbackup: S3 target: endpoint %q: a URL such as https://s3.eu-central-1.amazonaws.com, without a path; the bucket is a field of its own", v)
	}
	host = strings.ToLower(u.Host)
	if p := u.Port(); p == "" || (u.Scheme == "https" && p == "443") || (u.Scheme == "http" && p == "80") {
		host = strings.ToLower(u.Hostname())
		if strings.Contains(host, ":") {
			host = "[" + host + "]"
		}
	}
	return u.Scheme, host, nil
}

// s3Prefix reads a prefix: "" for the bucket's top, or folders ending
// in /, without empty, . or .. parts.
func s3Prefix(v string) (string, error) {
	p := strings.Trim(v, "/")
	if p == "" {
		return "", nil
	}
	if len(p) > s3MaxPrefix {
		return "", fmt.Errorf("serverbackup: S3 target: the prefix is longer than %d bytes", s3MaxPrefix)
	}
	for part := range strings.SplitSeq(p, "/") {
		if part == "" || part == "." || part == ".." {
			return "", fmt.Errorf("serverbackup: S3 target: prefix %q: folders without empty, . or .. parts", v)
		}
	}
	return p + "/", nil
}

// String names the target as its errors do, without its keys.
func (s *S3) String() string {
	return "serverbackup: S3 bucket " + s.bucket + " on " + s.host
}

// Put stores r, size bytes, under name in one PUT. S3 has no rename and
// needs none: a PUT is atomic, so that one broken off or refused leaves
// nothing under name, and Put writes name directly rather than by a
// temporary name as the other targets do. A backup larger than the
// 5 GiB one PUT takes is refused; fewer or more bytes than size are
// refused before the last byte goes. The body is signed unhashed
// (UNSIGNED-PAYLOAD), and carries its Content-MD5 (contentMD5), which a
// bucket with object lock asks of every PUT.
func (s *S3) Put(ctx context.Context, name string, r io.Reader, size int64) error {
	if err := CheckName(name); err != nil {
		return err
	}
	switch {
	case size < 0:
		return fmt.Errorf("%s: PUT %s: a size of %d bytes", s, name, size)
	case size > s3MaxPut:
		return fmt.Errorf("%s: PUT %s: the backup is %d bytes, more than the 5 GiB (%d bytes) that one PUT takes, and this target puts a backup in one piece",
			s, name, size, int64(s3MaxPut))
	}
	return s.put(ctx, s.prefix+name, "application/zip", r, size)
}

// put stores r, size bytes, under key, with its Content-MD5 where r
// can seek (contentMD5).
func (s *S3) put(ctx context.Context, key, contentType string, r io.Reader, size int64) error {
	h := http.Header{}
	h.Set("Content-Type", contentType)
	sum, err := contentMD5(ctx, r, size)
	if err != nil {
		return fmt.Errorf("%s: PUT %s: %w", s, key, err)
	}
	if sum != "" {
		h.Set("Content-MD5", sum)
	}
	resp, err := s.do(ctx, s3Call{method: http.MethodPut, key: key, header: h, body: r, size: size})
	if err != nil {
		return fmt.Errorf("%s: PUT %s: %w", s, key, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return s.answer(http.MethodPut, key, resp)
	}
	return nil
}

// contentMD5 is the Content-MD5 of the size bytes r holds, read once
// ahead and r sought back to where it was: a bucket with object lock
// refuses a PUT without it (or a checksum of AWS's newer kind), and the
// store checks the body by it. A reader that cannot seek goes without
// one ("") and so cannot be put into such a bucket; the catalogue's
// always can, a file on the box or a copy on another target read by
// ranges, which is read twice then. Fewer or more bytes than size are
// refused here already.
func contentMD5(ctx context.Context, r io.Reader, size int64) (string, error) {
	sk, ok := r.(io.Seeker)
	if !ok {
		return "", nil
	}
	start, err := sk.Seek(0, io.SeekCurrent)
	if err != nil {
		return "", nil
	}
	h := md5.New()
	n, err := io.Copy(h, io.LimitReader(ctxReader{ctx: ctx, r: r}, size+1))
	if _, serr := sk.Seek(start, io.SeekStart); err == nil && serr != nil {
		err = serr
	}
	switch {
	case err != nil:
		return "", err
	case n > size:
		return "", errors.New("more bytes than the size given")
	case n < size:
		return "", fmt.Errorf("%d bytes short of the size given: %w", size-n, io.ErrUnexpectedEOF)
	}
	return base64.StdEncoding.EncodeToString(h.Sum(nil)), nil
}

// Open returns name for ranged reads, with its size: each read a GET
// with a range, on the version HEAD found (If-Match), and the reads end
// with ctx. A name not there is fs.ErrNotExist.
func (s *S3) Open(ctx context.Context, name string) (File, int64, error) {
	if err := CheckName(name); err != nil {
		return nil, 0, err
	}
	key := s.prefix + name
	resp, err := s.do(ctx, s3Call{method: http.MethodHead, key: key})
	if err != nil {
		return nil, 0, fmt.Errorf("%s: HEAD %s: %w", s, key, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, 0, s.answer(http.MethodHead, key, resp)
	}
	if resp.ContentLength < 0 {
		return nil, 0, fmt.Errorf("%s: HEAD %s: no Content-Length", s, key)
	}
	return &s3File{s: s, ctx: ctx, key: key, etag: resp.Header.Get("ETag"), size: resp.ContentLength}, resp.ContentLength, nil
}

// s3File is an object opened for ranged reads. A read fetches its
// range and, where it follows the read before, more ahead (s3MinAhead
// to s3MaxAhead), which the next reads take from memory.
type s3File struct {
	s    *S3
	ctx  context.Context
	key  string
	etag string
	size int64

	mu     sync.Mutex
	closed bool
	buf    []byte // fetched ahead, from at
	at     int64
	next   int64 // where the last read ended
	ahead  int64
}

// ReadAt reads len(p) bytes at off, fewer with io.EOF at the end.
func (f *s3File) ReadAt(p []byte, off int64) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	switch {
	case f.closed:
		return 0, fmt.Errorf("%s: %s: %w", f.s, f.key, fs.ErrClosed)
	case off < 0:
		return 0, fmt.Errorf("%s: %s: a read at %d", f.s, f.key, off)
	case off >= f.size:
		return 0, io.EOF
	case len(p) == 0:
		return 0, nil
	}
	want := min(int64(len(p)), f.size-off)
	if off < f.at || off+want > f.at+int64(len(f.buf)) {
		if off == f.next {
			f.ahead = min(max(2*f.ahead, s3MinAhead), s3MaxAhead)
		} else {
			f.ahead = s3MinAhead
		}
		f.buf = f.buf[:0]
		if want >= f.ahead {
			if err := f.s.get(f.ctx, f.key, f.etag, off, p[:want]); err != nil {
				return 0, err
			}
			f.next = off + want
			return f.result(p, want)
		}
		n := min(f.ahead, f.size-off)
		if int64(cap(f.buf)) < n {
			f.buf = make([]byte, 0, n)
		}
		if err := f.s.get(f.ctx, f.key, f.etag, off, f.buf[:n]); err != nil {
			return 0, err
		}
		f.buf, f.at = f.buf[:n], off
	}
	copy(p, f.buf[off-f.at:off-f.at+want])
	f.next = off + want
	return f.result(p, want)
}

// result is a read's answer: want bytes, and io.EOF where p wanted more.
func (f *s3File) result(p []byte, want int64) (int, error) {
	if want < int64(len(p)) {
		return int(want), io.EOF
	}
	return int(want), nil
}

// Close drops what was fetched ahead.
func (f *s3File) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.closed, f.buf = true, nil
	return nil
}

// get reads len(dst) bytes of key at off into dst, of the version etag
// where it is given.
func (s *S3) get(ctx context.Context, key, etag string, off int64, dst []byte) error {
	h := http.Header{}
	h.Set("Range", "bytes="+strconv.FormatInt(off, 10)+"-"+strconv.FormatInt(off+int64(len(dst))-1, 10))
	if etag != "" {
		h.Set("If-Match", etag)
	}
	resp, err := s.do(ctx, s3Call{method: http.MethodGet, key: key, header: h})
	if err != nil {
		return fmt.Errorf("%s: GET %s: %w", s, key, err)
	}
	defer resp.Body.Close()
	switch {
	case resp.StatusCode == http.StatusPartialContent:
		if first, ok := contentRangeStart(resp.Header.Get("Content-Range")); !ok || first != off {
			return fmt.Errorf("%s: GET %s: the answer's range %q is not the one asked for, from %d", s, key, resp.Header.Get("Content-Range"), off)
		}
	case resp.StatusCode == http.StatusOK && off == 0:
		// The whole object, or a server that ignores the range: its start.
	case resp.StatusCode == http.StatusOK:
		return fmt.Errorf("%s: GET %s: the server ignores the range", s, key)
	default:
		return s.answer(http.MethodGet, key, resp)
	}
	if _, err := io.ReadFull(resp.Body, dst); err != nil {
		return fmt.Errorf("%s: GET %s: %w", s, key, err)
	}
	return nil
}

// contentRangeStart is the first byte of a Content-Range, bytes a-b/n.
func contentRangeStart(v string) (int64, bool) {
	rest, ok := strings.CutPrefix(v, "bytes ")
	if !ok {
		return 0, false
	}
	first, _, ok := strings.Cut(rest, "-")
	if !ok {
		return 0, false
	}
	n, err := strconv.ParseInt(first, 10, 64)
	return n, err == nil
}

// List lists the backups under the prefix (ListObjectsV2, page by page
// with its continuation): the keys of names a backup may have, the
// folders below the prefix left out.
func (s *S3) List(ctx context.Context) ([]Entry, error) {
	out := []Entry{}
	token := ""
	seen := map[string]bool{}
	for range s3MaxPages {
		page, err := s.listPage(ctx, token, 0)
		if err != nil {
			return nil, err
		}
		for _, c := range page.Contents {
			name, ok := strings.CutPrefix(c.Key, s.prefix)
			if !ok || CheckName(name) != nil {
				continue
			}
			at, _ := time.Parse(time.RFC3339, c.LastModified)
			out = append(out, Entry{Name: name, Size: c.Size, Time: at})
		}
		if !page.IsTruncated {
			return out, nil
		}
		if page.NextContinuationToken == "" || seen[page.NextContinuationToken] {
			return nil, fmt.Errorf("%s: list: a page cut short without a new continuation token", s)
		}
		seen[page.NextContinuationToken] = true
		token = page.NextContinuationToken
	}
	return nil, fmt.Errorf("%s: list: more than %d pages", s, s3MaxPages)
}

// s3ListPage is a page of ListObjectsV2's answer.
type s3ListPage struct {
	IsTruncated           bool
	NextContinuationToken string
	Contents              []struct {
		Key          string
		Size         int64
		LastModified string
	}
}

// listPage asks for one page of the prefix's keys after token, at most
// max of them where max is not 0.
func (s *S3) listPage(ctx context.Context, token string, max int) (s3ListPage, error) {
	q := url.Values{"list-type": {"2"}, "prefix": {s.prefix}, "delimiter": {"/"}}
	if token != "" {
		q.Set("continuation-token", token)
	}
	if max > 0 {
		q.Set("max-keys", strconv.Itoa(max))
	}
	resp, err := s.do(ctx, s3Call{method: http.MethodGet, query: q})
	if err != nil {
		return s3ListPage{}, fmt.Errorf("%s: list: %w", s, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return s3ListPage{}, s.answer("list", "", resp)
	}
	var page s3ListPage
	if err := xml.NewDecoder(io.LimitReader(resp.Body, s3MaxListPage)).Decode(&page); err != nil {
		return s3ListPage{}, fmt.Errorf("%s: list: the answer: %w", s, err)
	}
	return page, nil
}

// Delete removes name; a name not there is no error. A DELETE the
// bucket refuses, its object lock or a key that may not delete, is
// ErrRetained: the copy stays. A bucket with object lock is versioned,
// and there a DELETE succeeds by hiding the object behind a delete
// marker, its locked version kept by the bucket until its lock ends.
func (s *S3) Delete(ctx context.Context, name string) error {
	if err := CheckName(name); err != nil {
		return err
	}
	return s.delete(ctx, s.prefix+name)
}

// delete removes key; a key not there is no error.
func (s *S3) delete(ctx context.Context, key string) error {
	resp, err := s.do(ctx, s3Call{method: http.MethodDelete, key: key})
	if err != nil {
		return fmt.Errorf("%s: DELETE %s: %w", s, key, err)
	}
	defer resp.Body.Close()
	switch resp.StatusCode {
	case http.StatusOK, http.StatusNoContent:
		return nil
	}
	err = s.answer(http.MethodDelete, key, resp)
	var e *s3Error
	if errors.As(err, &e) && e.Status == http.StatusNotFound && e.Code != "NoSuchBucket" {
		return nil
	}
	return err
}

// Check lists the bucket, then writes a small probe under the prefix,
// reads it back and deletes it. A probe the bucket keeps (ErrRetained)
// stays, as every copy would, and is no failure; its name is hidden and
// no backup's.
func (s *S3) Check(ctx context.Context) error {
	if _, err := s.listPage(ctx, "", 1); err != nil {
		return err
	}
	var rnd [8]byte
	rand.Read(rnd[:])
	key := s.prefix + ".massimo-check-" + hex.EncodeToString(rnd[:])
	probe := []byte("massimo\n")
	if err := s.put(ctx, key, "text/plain", bytes.NewReader(probe), int64(len(probe))); err != nil {
		return err
	}
	back := make([]byte, len(probe))
	if err := s.get(ctx, key, "", 0, back); err != nil {
		return err
	}
	if !bytes.Equal(back, probe) {
		return fmt.Errorf("%s: %s: read back other bytes than were written", s, key)
	}
	if err := s.delete(ctx, key); err != nil && !errors.Is(err, ErrRetained) {
		return err
	}
	return nil
}

// s3Call is one request: its method, its object's key ("" the bucket's
// own), query and headers, and a body of size bytes.
type s3Call struct {
	method string
	key    string
	query  url.Values
	header http.Header
	body   io.Reader
	size   int64
}

// do sends a call, signed, and returns the answer whatever its status.
// The call is broken off where nothing moves for s.stall, the answer's
// body being read included; it is the caller's to close.
func (s *S3) do(ctx context.Context, c s3Call) (*http.Response, error) {
	u := &url.URL{Scheme: s.scheme, Host: s.host, Path: "/" + s.bucket, RawQuery: c.query.Encode()}
	if c.key != "" {
		u.Path += "/" + c.key
	}
	if !s.pathStyle {
		u.Host, u.Path = s.bucket+"."+s.host, "/"+c.key
	}
	ctx, cancel := context.WithCancelCause(ctx)
	timer := time.AfterFunc(s.stall, func() { cancel(fmt.Errorf("%w: nothing moved for %v", errS3Stalled, s.stall)) })
	poke := func() { timer.Reset(s.stall) }
	stop := func() { timer.Stop(); cancel(nil) }
	var body io.Reader = http.NoBody
	payload := s3EmptyHash
	if c.body != nil && c.size > 0 {
		body, payload = &exactReader{r: c.body, left: c.size, poke: poke}, s3UnsignedPayload
	}
	req, err := http.NewRequestWithContext(ctx, c.method, u.String(), body)
	if err != nil {
		stop()
		return nil, err
	}
	if c.body != nil {
		req.ContentLength = c.size
		if c.size > 0 {
			req.Header.Set("Expect", "100-continue")
		}
	}
	for k, vs := range c.header {
		req.Header[k] = vs
	}
	signV4(req, s.accessKey, s.secretKey, s.region, payload, s.now())
	resp, err := s.client.Do(req)
	if err != nil {
		cause := context.Cause(ctx)
		stop()
		var ue *url.Error
		if errors.As(err, &ue) {
			err = ue.Err
		}
		if errors.Is(cause, errS3Stalled) {
			err = cause
		}
		return nil, err
	}
	poke()
	resp.Body = &s3Body{ReadCloser: resp.Body, ctx: ctx, poke: poke, stop: stop}
	return resp, nil
}

// s3Body is an answer's body, whose reads keep its call alive.
type s3Body struct {
	io.ReadCloser
	ctx  context.Context
	poke func()
	stop func()
}

// Read reads the body, a read that moves bytes putting off the break.
func (b *s3Body) Read(p []byte) (int, error) {
	n, err := b.ReadCloser.Read(p)
	if n > 0 {
		b.poke()
	}
	if err != nil && err != io.EOF {
		if cause := context.Cause(b.ctx); errors.Is(cause, errS3Stalled) {
			err = cause
		}
	}
	return n, err
}

// Close ends the call.
func (b *s3Body) Close() error {
	b.stop()
	return b.ReadCloser.Close()
}

// exactReader gives r's size bytes and fails where r has fewer or
// more; more fails before the last bytes go, so that the server never
// has the whole body.
type exactReader struct {
	r    io.Reader
	left int64
	poke func()
}

// Read reads at most what is left.
func (e *exactReader) Read(p []byte) (int, error) {
	if e.left <= 0 {
		return 0, io.EOF
	}
	if int64(len(p)) > e.left {
		p = p[:e.left]
	}
	n, err := e.r.Read(p)
	if n > 0 && int64(n) == e.left {
		var one [1]byte
		if m, _ := io.ReadFull(e.r, one[:]); m > 0 {
			return 0, errors.New("more bytes than the size given")
		}
	}
	e.left -= int64(n)
	if n > 0 && e.poke != nil {
		e.poke()
	}
	switch {
	case e.left == 0:
		return n, nil
	case err == io.EOF:
		return n, fmt.Errorf("%d bytes short of the size given: %w", e.left, io.ErrUnexpectedEOF)
	}
	return n, err
}

// s3Error is an answer of the S3 server that is not a success.
type s3Error struct {
	Status  int
	Code    string
	Message string
	// Region is the bucket's, where the answer names another than the
	// target's.
	Region string
	// Retained is a DELETE refused for the bucket's lock or the key's
	// rights: errors.Is(e, ErrRetained).
	Retained bool
}

// Error says what the server answered.
func (e *s3Error) Error() string {
	var b strings.Builder
	fmt.Fprintf(&b, "%d %s", e.Status, http.StatusText(e.Status))
	for _, v := range []string{e.Code, e.Message} {
		if v != "" {
			b.WriteString(": " + v)
		}
	}
	if e.Region != "" {
		fmt.Fprintf(&b, "; the bucket is in region %s", e.Region)
	}
	if e.Retained {
		b.WriteString("; the bucket keeps the file: its object lock, or a key that may not delete")
	}
	return b.String()
}

// Is matches ErrRetained for a retained DELETE, fs.ErrNotExist for a 404.
func (e *s3Error) Is(target error) bool {
	return (target == ErrRetained && e.Retained) || (target == fs.ErrNotExist && e.Status == http.StatusNotFound)
}

// answer reads an answer that is not a success as an error of op on key
// (the bucket where key is ""): its status, S3's code and message, the
// bucket's region where it is another, and for a DELETE whether the
// bucket refuses it for its lock or the key's rights.
func (s *S3) answer(op, key string, resp *http.Response) error {
	var body struct{ Code, Message, Region string }
	data, _ := io.ReadAll(io.LimitReader(resp.Body, s3MaxErrorBody))
	xml.Unmarshal(data, &body)
	e := &s3Error{Status: resp.StatusCode, Code: printable(body.Code, 64), Message: strings.TrimSuffix(printable(body.Message, 300), ".")}
	for _, r := range []string{resp.Header.Get("X-Amz-Bucket-Region"), body.Region} {
		if r = printable(r, 64); r != "" && r != s.region {
			e.Region = r
			break
		}
	}
	e.Retained = op == http.MethodDelete && s3Retained(e)
	what := op + " " + key
	if key == "" {
		what = op
	}
	return fmt.Errorf("%s: %s: %w", s, what, e)
}

// s3Retained reports whether a refused DELETE is the bucket keeping the
// object: AccessDenied (AWS's answer for a locked version, and a key's
// that may not delete), or a code or message that speaks of a lock or a
// retention (MinIO's WORM, a provider's bucket lock).
func s3Retained(e *s3Error) bool {
	if e.Status != http.StatusBadRequest && e.Status != http.StatusForbidden && e.Status != http.StatusConflict {
		return false
	}
	if e.Status == http.StatusForbidden && e.Code == "AccessDenied" {
		return true
	}
	text := strings.ToLower(e.Code + " " + e.Message)
	return slices.ContainsFunc([]string{"lock", "worm", "retention"}, func(w string) bool { return strings.Contains(text, w) })
}

// printable is v without control characters, at most n runes.
func printable(v string, n int) string {
	v = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return ' '
		}
		return r
	}, strings.TrimSpace(v))
	if r := []rune(v); len(r) > n {
		v = string(r[:n]) + "…"
	}
	return v
}

// signV4 signs req for S3 by AWS Signature Version 4 at now, its body's
// hash being payload (UNSIGNED-PAYLOAD for a body not hashed): it sets
// X-Amz-Date and X-Amz-Content-Sha256, writes the path and the query in
// their canonical form, which goes on the wire as signed, and signs the
// host and every header but Authorization, User-Agent and Expect, which
// a proxy may change.
func signV4(req *http.Request, accessKey string, secret []byte, region, payload string, now time.Time) {
	now = now.UTC()
	date, stamp := now.Format("20060102"), now.Format(s3AmzDate)
	req.Header.Del("Authorization")
	req.Header.Set("X-Amz-Date", stamp)
	req.Header.Set("X-Amz-Content-Sha256", payload)
	if req.URL.Path == "" {
		req.URL.Path = "/"
	}
	req.URL.RawPath = s3Escape(req.URL.Path, false)
	req.URL.RawQuery = s3Query(req.URL.Query())

	host := req.Host
	if host == "" {
		host = req.URL.Host
	}
	headers := map[string]string{"host": host}
	for k, vs := range req.Header {
		k = strings.ToLower(k)
		if k == "authorization" || k == "user-agent" || k == "expect" {
			continue
		}
		vals := make([]string, len(vs))
		for i, v := range vs {
			vals[i] = strings.Join(strings.Fields(v), " ")
		}
		headers[k] = strings.Join(vals, ",")
	}
	names := make([]string, 0, len(headers))
	for k := range headers {
		names = append(names, k)
	}
	slices.Sort(names)
	var canon strings.Builder
	for _, k := range names {
		canon.WriteString(k + ":" + headers[k] + "\n")
	}
	signed := strings.Join(names, ";")
	request := strings.Join([]string{req.Method, req.URL.RawPath, req.URL.RawQuery, canon.String(), signed, payload}, "\n")
	scope := date + "/" + region + "/s3/aws4_request"
	sum := sha256.Sum256([]byte(request))
	toSign := s3Algorithm + "\n" + stamp + "\n" + scope + "\n" + hex.EncodeToString(sum[:])
	sig := hex.EncodeToString(hmacSHA256(s3SigningKey(secret, date, region, "s3"), toSign))
	req.Header.Set("Authorization", s3Algorithm+" Credential="+accessKey+"/"+scope+",SignedHeaders="+signed+",Signature="+sig)
}

// s3SigningKey derives SigV4's signing key of a day, a region and a
// service from the secret key.
func s3SigningKey(secret []byte, date, region, service string) []byte {
	k := append([]byte("AWS4"), secret...)
	defer clear(k)
	for _, part := range []string{date, region, service, "aws4_request"} {
		k = hmacSHA256(k, part)
	}
	return k
}

// hmacSHA256 is HMAC-SHA256 of data under key.
func hmacSHA256(key []byte, data string) []byte {
	m := hmac.New(sha256.New, key)
	m.Write([]byte(data))
	return m.Sum(nil)
}

// s3Escape is SigV4's URI encoding: every byte but the unreserved
// A-Z a-z 0-9 - _ . ~ as %XX, the slash kept unless slash is true.
func s3Escape(v string, slash bool) string {
	var b strings.Builder
	for i := 0; i < len(v); i++ {
		c := v[i]
		switch {
		case 'A' <= c && c <= 'Z', 'a' <= c && c <= 'z', '0' <= c && c <= '9', c == '-', c == '_', c == '.', c == '~':
			b.WriteByte(c)
		case c == '/' && !slash:
			b.WriteByte(c)
		default:
			fmt.Fprintf(&b, "%%%02X", c)
		}
	}
	return b.String()
}

// s3Query is SigV4's canonical query: each name and value encoded,
// sorted by name and then by value.
func s3Query(q url.Values) string {
	var pairs [][2]string
	for k, vs := range q {
		for _, v := range vs {
			pairs = append(pairs, [2]string{s3Escape(k, true), s3Escape(v, true)})
		}
	}
	slices.SortFunc(pairs, func(a, b [2]string) int {
		if c := strings.Compare(a[0], b[0]); c != 0 {
			return c
		}
		return strings.Compare(a[1], b[1])
	})
	out := make([]string, len(pairs))
	for i, p := range pairs {
		out[i] = p[0] + "=" + p[1]
	}
	return strings.Join(out, "&")
}
