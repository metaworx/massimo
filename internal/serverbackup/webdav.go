// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package serverbackup

// The WebDAV target (AP ServerBackup v1.0, Block 6): a folder on a WebDAV
// server, such as Nextcloud with an app password, a Synology box or a
// storage box, reached with the standard library alone. A copy is put to
// a temporary name and moved to its own (PUT, then MOVE with Overwrite:
// F), read with ranged GETs, listed with PROPFIND at depth 1 and deleted
// with DELETE; a folder missing is made with MKCOL. The user signs in by
// basic authentication, the password being the target's sealed secret,
// over https unless the URL says http, which the card warns of (the url
// field's Warn): the password then travels in clear.

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"
)

// KindWebDAV is the WebDAV target's kind.
const KindWebDAV = "webdav"

// The WebDAV client's timeouts: to connect, to finish TLS, to wait for an
// answer's head once a request is sent, and for a request that moves no
// file to end. A transfer is not bounded as a whole: it is given up once
// no byte moved for davStall.
const (
	davDialTimeout    = 30 * time.Second
	davTLSTimeout     = 30 * time.Second
	davHeaderTimeout  = 5 * time.Minute
	davRequestTimeout = 2 * time.Minute
	davStall          = 2 * time.Minute
)

// davChunk is the least a ranged read asks for: reads that follow one
// another come from one GET, and a read alone fetches at most this.
const davChunk = 4 << 20

// davMaxListing bounds the answer to a PROPFIND.
const davMaxListing = 32 << 20

// davTempSuffix ends the hidden temporary name of a put beside the final
// one, ".<name>.<random>.tmp"; not .part, which Nextcloud keeps for its
// own uploads. One a crash left goes at a later put once it is older
// than a day (partStale).
const davTempSuffix = ".tmp"

// davURLWarn matches a URL in plain http, which the card warns of.
const davURLWarn = `^[Hh][Tt][Tt][Pp]:`

func init() {
	Register(Kind{
		Name: KindWebDAV,
		Fields: []Field{
			{Key: "url", Type: FieldText, Required: true, Warn: davURLWarn},
			{Key: "user", Type: FieldText, Required: true},
			{Key: "password", Type: FieldSecret, Required: true},
		},
		New: func(cfg map[string]string, secret []byte) (Target, error) {
			return NewWebDAV(cfg["url"], cfg["user"], secret)
		},
	})
}

// davTransport is the WebDAV targets' transport, shared so that a target
// opened for each job reuses the connections of the last.
var davTransport = sync.OnceValue(func() *http.Transport {
	return &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		DialContext:           (&net.Dialer{Timeout: davDialTimeout, KeepAlive: 30 * time.Second}).DialContext,
		TLSHandshakeTimeout:   davTLSTimeout,
		ResponseHeaderTimeout: davHeaderTimeout,
		ExpectContinueTimeout: time.Second,
		IdleConnTimeout:       90 * time.Second,
		MaxIdleConnsPerHost:   2,
		ForceAttemptHTTP2:     true,
	}
})

// WebDAV is a target in a folder of a WebDAV server.
type WebDAV struct {
	base   *url.URL // the folder, its path ending in /
	user   string
	pass   string
	client *http.Client
	stall  time.Duration
}

// NewWebDAV returns the target of the folder at rawURL, signed in to as
// user with password: https where the URL names no scheme, http only
// where it says so. A URL with a user or a password in it, a query or a
// fragment is refused, and so is a user with a colon, which basic
// authentication cannot carry.
func NewWebDAV(rawURL, user string, password []byte) (*WebDAV, error) {
	u, err := davURL(rawURL)
	if err != nil {
		return nil, err
	}
	if user == "" || strings.Contains(user, ":") {
		return nil, errors.New("serverbackup: webdav: a user, without a colon")
	}
	return &WebDAV{base: u, user: user, pass: string(password), stall: davStall, client: &http.Client{
		Transport: davTransport(),
		// a redirect is told, not followed: the password would go along
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}}, nil
}

// davURL reads a WebDAV folder's URL.
func davURL(raw string) (*url.URL, error) {
	s := strings.TrimSpace(raw)
	if !strings.Contains(s, "://") {
		s = "https://" + s
	}
	u, err := url.Parse(s)
	switch {
	case err != nil:
		return nil, fmt.Errorf("serverbackup: webdav: the URL does not parse: %w", err)
	case u.Scheme != "https" && u.Scheme != "http":
		return nil, fmt.Errorf("serverbackup: webdav: %s:// is no URL of WebDAV, want https:// or http://", u.Scheme)
	case u.Host == "":
		return nil, errors.New("serverbackup: webdav: the URL names no server")
	case u.User != nil:
		return nil, errors.New("serverbackup: webdav: the user and the password go in their own fields, not in the URL")
	case u.RawQuery != "" || u.ForceQuery || u.Fragment != "":
		return nil, errors.New("serverbackup: webdav: the URL has a query or a fragment")
	}
	if !strings.HasSuffix(u.Path, "/") {
		u.Path += "/"
		if u.RawPath != "" {
			u.RawPath += "/"
		}
	}
	return u, nil
}

// at is the URL of the file name in the folder, name escaped as one
// segment of the path. A name that would reach the folder itself or
// another one, such as "", "..", one with a slash, or one the URL would
// read otherwise, is refused: a DELETE of the folder would remove it with
// everything in it.
func (w *WebDAV) at(name string) (*url.URL, error) {
	if name == "" || name == "." || name == ".." || strings.ContainsAny(name, "/\\") {
		return nil, fmt.Errorf("serverbackup: webdav %s: %q is no name of a file in the folder", w.base.Host, name)
	}
	u := w.base.JoinPath(url.PathEscape(name))
	if rest, ok := strings.CutPrefix(u.Path, w.base.Path); !ok || rest != name {
		return nil, fmt.Errorf("serverbackup: webdav %s: %q is no name of a file in the folder", w.base.Host, name)
	}
	return u, nil
}

// davTemp reports whether a name is one of this target's temporary
// names: a put's, ".<name>.<hex>.tmp" of a backup or of Check's probe,
// or the probe's own, ".check-<hex>.tmp"; so that a stale one is ours
// to remove, and nobody else's file is.
func davTemp(n string) bool {
	stem, ok := strings.CutPrefix(n, ".")
	if !ok {
		return false
	}
	if stem, ok = strings.CutSuffix(stem, davTempSuffix); !ok {
		return false
	}
	if davProbe(stem) {
		return true
	}
	i := strings.LastIndexByte(stem, '.')
	if i < 0 || !isHex(stem[i+1:]) {
		return false
	}
	base := stem[:i]
	if probe, ok := strings.CutSuffix(base, davTempSuffix); ok && davProbe(probe) {
		return true
	}
	return CheckName(base) == nil
}

// davProbe reports whether s is "check-<hex>", the stem of Check's probe.
func davProbe(s string) bool {
	h, ok := strings.CutPrefix(s, "check-")
	return ok && isHex(h)
}

// isHex reports whether s is lower-case hexadecimal digits, one at least.
func isHex(s string) bool {
	return s != "" && strings.Trim(s, "0123456789abcdef") == ""
}

// Put sends r, size bytes, to a hidden temporary name beside name and
// moves it there with Overwrite: F, so that a put broken off leaves
// nothing under name; its temporary file is deleted. The folder is made
// where it is missing. A file under name that is there already is not
// replaced: of size bytes it is taken for this one (the name holds the
// backup's uuid, and a move whose answer was lost leaves it so), of
// another size the put fails and says so. Fewer or more bytes than size
// are refused.
func (w *WebDAV) Put(ctx context.Context, name string, r io.Reader, size int64) error {
	if err := CheckName(name); err != nil {
		return err
	}
	if size < 0 {
		return fmt.Errorf("serverbackup: webdav %s: PUT %s: a size of %d", w.base.Host, name, size)
	}
	if err := w.prepare(ctx); err != nil {
		return err
	}
	return w.put(ctx, name, r, size)
}

// put sends r to a temporary name and moves it to name; the temporary
// file goes where it did not get there.
func (w *WebDAV) put(ctx context.Context, name string, r io.Reader, size int64) (err error) {
	tmp := "." + strings.TrimPrefix(name, ".") + "." + randomHex(6) + davTempSuffix
	defer func() {
		if err != nil {
			w.removeTemp(context.WithoutCancel(ctx), tmp)
		}
	}()
	if err := w.send(ctx, name, tmp, r, size); err != nil {
		return err
	}
	return w.move(ctx, tmp, name, size)
}

// send PUTs r, exactly size bytes, to tmp; name is the file it is for.
func (w *WebDAV) send(ctx context.Context, name, tmp string, r io.Reader, size int64) error {
	ctx, st, done := watch(ctx, w.stall)
	defer done()
	var body io.Reader = http.NoBody
	if size > 0 {
		body = &sendBody{r: r, left: size, size: size, st: st}
	}
	u, err := w.at(tmp)
	if err != nil {
		return err
	}
	resp, err := w.do(ctx, http.MethodPut, u, body, size, map[string]string{
		"Content-Type": "application/octet-stream",
		// a refusal (401, 507) comes before the body is sent
		"Expect": "100-continue",
	})
	if err != nil {
		return w.failed(ctx, "PUT", name, err)
	}
	defer drain(resp)
	switch resp.StatusCode {
	case http.StatusOK, http.StatusCreated, http.StatusNoContent:
		return nil
	}
	return w.refused("PUT", name, resp)
}

// move moves from to to, not over a file there.
func (w *WebDAV) move(ctx context.Context, from, to string, size int64) error {
	ctx, cancel := context.WithTimeout(ctx, davRequestTimeout)
	defer cancel()
	src, err := w.at(from)
	if err != nil {
		return err
	}
	dst, err := w.at(to)
	if err != nil {
		return err
	}
	resp, err := w.do(ctx, "MOVE", src, nil, 0, map[string]string{"Destination": dst.String(), "Overwrite": "F"})
	if err != nil {
		return w.failed(ctx, "MOVE", to, err)
	}
	defer drain(resp)
	switch resp.StatusCode {
	case http.StatusCreated, http.StatusNoContent:
		return nil
	case http.StatusPreconditionFailed:
		// Overwrite: F refused: a file of the name is there
		have, err := w.size(ctx, to)
		if err != nil {
			return fmt.Errorf("serverbackup: webdav %s: MOVE %s: a file of that name is there already and is not replaced (%w)", w.base.Host, to, err)
		}
		if have != size {
			return fmt.Errorf("serverbackup: webdav %s: MOVE %s: a file of that name is there already, of %d bytes, not %d, and is not replaced", w.base.Host, to, have, size)
		}
		w.remove(ctx, from)
		return nil
	}
	return w.refused("MOVE", to, resp)
}

// prepare makes the folder where it is missing, and removes the
// temporary files that puts a crash broke off left there, once they are
// older than a day: names of this target's own (davTemp) alone.
func (w *WebDAV) prepare(ctx context.Context) error {
	entries, found, err := w.propfind(ctx, w.base, "1")
	if err != nil {
		return err
	}
	if !found {
		return w.mkcol(ctx, w.base, 0)
	}
	now := time.Now()
	for _, e := range entries {
		if !e.dir && davTemp(e.name) && !e.time.IsZero() && now.Sub(e.time) > partStale {
			w.remove(ctx, e.name)
		}
	}
	return nil
}

// davMaxDepth bounds how many folders up a missing folder's parents are
// made.
const davMaxDepth = 16

// mkcol makes the folder u, and its parents where they are missing too
// (409 Conflict), depth of them so far. A folder there already (405
// Method Not Allowed) is no error.
func (w *WebDAV) mkcol(ctx context.Context, u *url.URL, depth int) error {
	code, err := w.mkcolOnce(ctx, u)
	if err != nil {
		return err
	}
	if code == http.StatusConflict {
		parent := u.ResolveReference(&url.URL{Path: ".."})
		if parent.Path == u.Path || depth >= davMaxDepth {
			return fmt.Errorf("serverbackup: webdav %s: MKCOL %s: %s", w.base.Host, u.Path, statusText(code))
		}
		if err := w.mkcol(ctx, parent, depth+1); err != nil {
			return err
		}
		if code, err = w.mkcolOnce(ctx, u); err != nil {
			return err
		}
	}
	switch code {
	case http.StatusCreated, http.StatusMethodNotAllowed:
		return nil
	}
	return fmt.Errorf("serverbackup: webdav %s: MKCOL %s: %s%s", w.base.Host, u.Path, statusText(code), hint(code))
}

// mkcolOnce sends one MKCOL and returns the status it was answered.
func (w *WebDAV) mkcolOnce(ctx context.Context, u *url.URL) (int, error) {
	ctx, cancel := context.WithTimeout(ctx, davRequestTimeout)
	defer cancel()
	resp, err := w.do(ctx, "MKCOL", u, nil, 0, nil)
	if err != nil {
		return 0, w.failed(ctx, "MKCOL", u.Path, err)
	}
	drain(resp)
	return resp.StatusCode, nil
}

// Open opens name for ranged reads, its size from PROPFIND at depth 0;
// a name not there is fs.ErrNotExist. The reads use ctx.
func (w *WebDAV) Open(ctx context.Context, name string) (File, int64, error) {
	if err := CheckName(name); err != nil {
		return nil, 0, err
	}
	size, err := w.size(ctx, name)
	if err != nil {
		return nil, 0, err
	}
	return &davFile{w: w, ctx: ctx, name: name, size: size}, size, nil
}

// size is the size of name, from PROPFIND at depth 0.
func (w *WebDAV) size(ctx context.Context, name string) (int64, error) {
	u, err := w.at(name)
	if err != nil {
		return 0, err
	}
	es, found, err := w.propfind(ctx, u, "0")
	switch {
	case err != nil:
		return 0, err
	case !found:
		return 0, fmt.Errorf("serverbackup: webdav %s: %s: %w", w.base.Host, name, fs.ErrNotExist)
	case len(es) == 0 || es[0].dir || es[0].size < 0:
		return 0, fmt.Errorf("serverbackup: webdav %s: %s is no file of a known size", w.base.Host, name)
	}
	return es[0].size, nil
}

// List lists the backups of the folder: its files whose names a backup
// may have; a folder not made yet holds none.
func (w *WebDAV) List(ctx context.Context) ([]Entry, error) {
	entries, found, err := w.propfind(ctx, w.base, "1")
	if err != nil {
		return nil, err
	}
	out := []Entry{}
	if !found {
		return out, nil
	}
	for _, e := range entries {
		if e.dir || e.size < 0 || CheckName(e.name) != nil {
			continue
		}
		out = append(out, Entry{Name: e.name, Size: e.size, Time: e.time})
	}
	return out, nil
}

// Delete removes name; one not there is no error.
func (w *WebDAV) Delete(ctx context.Context, name string) error {
	if err := CheckName(name); err != nil {
		return err
	}
	return w.remove(ctx, name)
}

// remove DELETEs name in the folder; one not there is no error.
func (w *WebDAV) remove(ctx context.Context, name string) error {
	_, err := w.removeOnce(ctx, name)
	return err
}

// davTempTries is how often the temporary file of a put that failed is
// deleted while the server holds it locked, davTempPause apart and
// longer each time.
const (
	davTempTries = 5
	davTempPause = 50 * time.Millisecond
)

// removeTemp deletes the temporary file of a put that failed. The server
// may still hold the upload's lock (423 Locked) until it sees the
// connection gone, so a locked file is tried again a few times; one left
// goes at a later put once it is a day old.
func (w *WebDAV) removeTemp(ctx context.Context, tmp string) {
	pause := davTempPause
	for range davTempTries {
		if code, _ := w.removeOnce(ctx, tmp); code != http.StatusLocked {
			return
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(pause):
		}
		pause *= 2
	}
}

// removeOnce sends one DELETE of name and returns the status it was
// answered, 0 where none came.
func (w *WebDAV) removeOnce(ctx context.Context, name string) (int, error) {
	ctx, cancel := context.WithTimeout(ctx, davRequestTimeout)
	defer cancel()
	u, err := w.at(name)
	if err != nil {
		return 0, err
	}
	resp, err := w.do(ctx, http.MethodDelete, u, nil, 0, nil)
	if err != nil {
		return 0, w.failed(ctx, "DELETE", name, err)
	}
	defer drain(resp)
	switch resp.StatusCode {
	case http.StatusOK, http.StatusAccepted, http.StatusNoContent, http.StatusNotFound:
		return resp.StatusCode, nil
	}
	return resp.StatusCode, w.refused("DELETE", name, resp)
}

// Check makes the folder where it is missing, then puts a small file as
// a backup is put, by a temporary name and a move, reads its size and
// its bytes back, and deletes it.
func (w *WebDAV) Check(ctx context.Context) error {
	if err := w.prepare(ctx); err != nil {
		return err
	}
	probe := ".check-" + randomHex(6) + davTempSuffix
	want := []byte("massimo\n")
	if err := w.put(ctx, probe, bytes.NewReader(want), int64(len(want))); err != nil {
		return err
	}
	removed := false
	defer func() {
		if !removed {
			w.remove(context.WithoutCancel(ctx), probe)
		}
	}()
	size, err := w.size(ctx, probe)
	if err != nil {
		return err
	}
	f := &davFile{w: w, ctx: ctx, name: probe, size: size}
	got := make([]byte, len(want))
	n, err := f.ReadAt(got, 0)
	f.Close()
	switch {
	case size != int64(len(want)):
		return fmt.Errorf("serverbackup: webdav %s: %s has %d bytes, not the %d written", w.base.Host, probe, size, len(want))
	case err != nil:
		return err
	case !bytes.Equal(got[:n], want):
		return fmt.Errorf("serverbackup: webdav %s: %s reads back other bytes than were written", w.base.Host, probe)
	}
	removed = true
	return w.remove(ctx, probe)
}

// do sends a request to u, signed in, with a body of size bytes where
// body is not nil.
func (w *WebDAV) do(ctx context.Context, method string, u *url.URL, body io.Reader, size int64, header map[string]string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, u.String(), body)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.ContentLength = size
	}
	req.SetBasicAuth(w.user, w.pass)
	for k, v := range header {
		req.Header.Set(k, v)
	}
	return w.client.Do(req)
}

// failed is the error of a request that got no answer, ctx being its
// own: why, the stall's where that gave it up.
func (w *WebDAV) failed(ctx context.Context, method, name string, err error) error {
	if cause := context.Cause(ctx); errors.Is(cause, errStalled) {
		err = cause
	}
	var ue *url.Error
	if errors.As(err, &ue) {
		err = ue.Err
	}
	return fmt.Errorf("serverbackup: webdav %s: %s %s: %w", w.base.Host, method, name, err)
}

// refused is the error of an answer that refused a request: its status,
// what that means where it helps, and where a redirect points.
func (w *WebDAV) refused(method, name string, resp *http.Response) error {
	why := hint(resp.StatusCode)
	if resp.StatusCode >= 300 && resp.StatusCode < 400 {
		why = ": the server sends us elsewhere; put that address in the target"
		if loc, err := resp.Location(); err == nil {
			loc.User = nil
			why = fmt.Sprintf(": the server sends us to %s; put that address in the target", loc)
		}
	}
	return fmt.Errorf("serverbackup: webdav %s: %s %s: %s%s", w.base.Host, method, name, statusText(resp.StatusCode), why)
}

// statusText is a status as its code and Go's text for it, not the
// server's.
func statusText(code int) string {
	if t := http.StatusText(code); t != "" {
		return strconv.Itoa(code) + " " + t
	}
	return strconv.Itoa(code)
}

// hint says what a status means for a target, where that helps.
func hint(code int) string {
	switch code {
	case http.StatusUnauthorized:
		return ": the user or the password is refused"
	case http.StatusForbidden:
		return ": the user may not write there"
	case http.StatusInsufficientStorage:
		return ": the server is full, or the user's quota spent"
	case http.StatusRequestEntityTooLarge:
		return ": the server takes no file this large"
	case http.StatusLocked:
		return ": the server holds the file locked"
	}
	return ""
}

// drain reads a little of what is left of an answer, so that its
// connection is used again, and closes it.
func drain(resp *http.Response) {
	io.Copy(io.Discard, io.LimitReader(resp.Body, 64<<10))
	resp.Body.Close()
}

// randomHex is n random bytes in hex.
func randomHex(n int) string {
	b := make([]byte, n)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// davEntry is a resource a PROPFIND answers: its name, its size (-1
// where not given), when it changed, and whether it is a folder.
type davEntry struct {
	name string
	size int64
	time time.Time
	dir  bool
}

// davPropfind asks for what List and size read.
const davPropfind = `<?xml version="1.0" encoding="utf-8"?>` +
	`<d:propfind xmlns:d="DAV:"><d:prop><d:resourcetype/><d:getcontentlength/><d:getlastmodified/></d:prop></d:propfind>`

// davMultistatus is the answer to a PROPFIND, as far as it is read.
type davMultistatus struct {
	Responses []struct {
		Href     string `xml:"DAV: href"`
		Propstat []struct {
			Status string `xml:"DAV: status"`
			Prop   struct {
				Length       string `xml:"DAV: getcontentlength"`
				Modified     string `xml:"DAV: getlastmodified"`
				ResourceType struct {
					Collection *struct{} `xml:"DAV: collection"`
				} `xml:"DAV: resourcetype"`
			} `xml:"DAV: prop"`
		} `xml:"DAV: propstat"`
	} `xml:"DAV: response"`
}

// propfind sends PROPFIND at depth to u and returns what it lists,
// u itself among them; found is false where u is not there (404).
func (w *WebDAV) propfind(ctx context.Context, u *url.URL, depth string) (entries []davEntry, found bool, err error) {
	ctx, cancel := context.WithTimeout(ctx, davRequestTimeout)
	defer cancel()
	resp, err := w.do(ctx, "PROPFIND", u, strings.NewReader(davPropfind), int64(len(davPropfind)),
		map[string]string{"Depth": depth, "Content-Type": `application/xml; charset="utf-8"`})
	if err != nil {
		return nil, false, w.failed(ctx, "PROPFIND", u.Path, err)
	}
	defer drain(resp)
	switch resp.StatusCode {
	case http.StatusMultiStatus:
	case http.StatusNotFound:
		return nil, false, nil
	default:
		return nil, false, w.refused("PROPFIND", u.Path, resp)
	}
	var ms davMultistatus
	if err := xml.NewDecoder(io.LimitReader(resp.Body, davMaxListing)).Decode(&ms); err != nil {
		return nil, false, fmt.Errorf("serverbackup: webdav %s: PROPFIND %s: the answer does not parse: %w", w.base.Host, u.Path, err)
	}
	for _, r := range ms.Responses {
		ref, err := url.Parse(strings.TrimSpace(r.Href))
		if err != nil {
			continue
		}
		p := u.ResolveReference(ref).Path
		e := davEntry{name: path.Base(strings.TrimSuffix(p, "/")), size: -1, dir: strings.HasSuffix(p, "/")}
		for _, ps := range r.Propstat {
			if f := strings.Fields(ps.Status); len(f) < 2 || !strings.HasPrefix(f[1], "2") {
				continue
			}
			if ps.Prop.ResourceType.Collection != nil {
				e.dir = true
			}
			if n, err := strconv.ParseInt(strings.TrimSpace(ps.Prop.Length), 10, 64); err == nil && n >= 0 {
				e.size = n
			}
			if t, err := http.ParseTime(strings.TrimSpace(ps.Prop.Modified)); err == nil {
				e.time = t
			}
		}
		entries = append(entries, e)
	}
	return entries, true, nil
}

// davFile is a file on a WebDAV server open for ranged reads. A read
// asks for davChunk bytes at least, and the next one that goes on where
// it ended reads on from the same answer.
type davFile struct {
	w    *WebDAV
	ctx  context.Context
	name string
	size int64

	mu     sync.Mutex
	body   io.ReadCloser // the answer read from, nil where none is open
	pos    int64         // the offset of the next byte body gives
	end    int64         // the offset after the last byte body gives
	closed bool
}

// ReadAt reads len(p) bytes at off, fewer at the file's end with io.EOF.
func (f *davFile) ReadAt(p []byte, off int64) (int, error) {
	if off < 0 {
		return 0, fmt.Errorf("serverbackup: webdav: a read at %d", off)
	}
	if off >= f.size {
		return 0, io.EOF
	}
	want := int(min(int64(len(p)), f.size-off))
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.closed {
		return 0, fs.ErrClosed
	}
	n := 0
	for n < want {
		at := off + int64(n)
		if f.body == nil || f.pos != at || f.pos >= f.end {
			f.closeBody()
			if err := f.fetch(at, want-n); err != nil {
				return n, err
			}
		}
		k, err := io.ReadFull(f.body, p[n:n+int(min(int64(want-n), f.end-f.pos))])
		n += k
		f.pos += int64(k)
		if err != nil {
			f.closeBody()
			return n, fmt.Errorf("serverbackup: webdav %s: GET %s: %w", f.w.base.Host, f.name, err)
		}
	}
	if want < len(p) {
		return n, io.EOF
	}
	return n, nil
}

// fetch GETs the range from off, want bytes and davChunk at least.
func (f *davFile) fetch(off int64, want int) error {
	end := min(off+max(int64(want), davChunk), f.size)
	u, err := f.w.at(f.name)
	if err != nil {
		return err
	}
	ctx, st, done := watch(f.ctx, f.w.stall)
	resp, err := f.w.do(ctx, http.MethodGet, u, nil, 0, map[string]string{"Range": fmt.Sprintf("bytes=%d-%d", off, end-1)})
	if err != nil {
		done()
		return f.w.failed(ctx, "GET", f.name, err)
	}
	st.stop()
	body := &recvBody{rc: resp.Body, ctx: ctx, st: st, done: done}
	switch resp.StatusCode {
	case http.StatusPartialContent:
		first, last, ok := contentRange(resp.Header.Get("Content-Range"))
		if !ok || first != off || last < off || last >= f.size {
			body.Close()
			return fmt.Errorf("serverbackup: webdav %s: GET %s: the range from %d answered with %q", f.w.base.Host, f.name, off, resp.Header.Get("Content-Range"))
		}
		f.body, f.pos, f.end = body, off, last+1
		return nil
	case http.StatusOK:
		// the server ignores the range and sends the whole file: what
		// comes before off is read past
		if resp.ContentLength >= 0 && resp.ContentLength != f.size {
			body.Close()
			return fmt.Errorf("serverbackup: webdav %s: %s has %d bytes now, not %d", f.w.base.Host, f.name, resp.ContentLength, f.size)
		}
		if _, err := io.CopyN(io.Discard, body, off); err != nil {
			body.Close()
			return fmt.Errorf("serverbackup: webdav %s: GET %s: %w", f.w.base.Host, f.name, err)
		}
		f.body, f.pos, f.end = body, off, f.size
		return nil
	case http.StatusNotFound:
		body.Close()
		return fmt.Errorf("serverbackup: webdav %s: %s: %w", f.w.base.Host, f.name, fs.ErrNotExist)
	}
	err = f.w.refused("GET", f.name, resp)
	body.Close()
	return err
}

// closeBody closes the answer read from, read to its end first where
// little of it is left, so that its connection is used again.
func (f *davFile) closeBody() {
	if f.body == nil {
		return
	}
	if f.end-f.pos <= 64<<10 {
		io.Copy(io.Discard, io.LimitReader(f.body, 64<<10+1))
	}
	f.body.Close()
	f.body = nil
}

// Close ends the reads.
func (f *davFile) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.closeBody()
	f.closed = true
	return nil
}

// contentRange reads a Content-Range of bytes: its first and last byte.
func contentRange(v string) (first, last int64, ok bool) {
	r, found := strings.CutPrefix(strings.TrimSpace(v), "bytes ")
	if !found {
		return 0, 0, false
	}
	r, _, _ = strings.Cut(r, "/")
	a, b, found := strings.Cut(r, "-")
	if !found {
		return 0, 0, false
	}
	first, err1 := strconv.ParseInt(a, 10, 64)
	last, err2 := strconv.ParseInt(b, 10, 64)
	return first, last, err1 == nil && err2 == nil
}

// errStalled gives up a transfer in which no byte moved for its stall
// time.
var errStalled = errors.New("no byte moved")

// stallTimer gives a transfer up once no byte moved for d, cancelling its
// request's context with errStalled. It runs from the request's start;
// a body's reads stop it while the other side has the turn.
type stallTimer struct {
	t *time.Timer
	d time.Duration
}

// watch returns the context of a transfer, its stall timer, running, and
// the function that ends both.
func watch(ctx context.Context, d time.Duration) (context.Context, *stallTimer, func()) {
	ctx, cancel := context.WithCancelCause(ctx)
	st := &stallTimer{d: d}
	st.t = time.AfterFunc(d, func() { cancel(fmt.Errorf("%w for %v", errStalled, d)) })
	return ctx, st, func() {
		st.t.Stop()
		cancel(context.Canceled)
	}
}

// run starts the timer again from its full time.
func (s *stallTimer) run() { s.t.Reset(s.d) }

// stop stops the timer.
func (s *stallTimer) stop() { s.t.Stop() }

// sendBody is a put's body: exactly size bytes of r. Its timer runs
// while the transport sends what was read, and stops while r is read.
type sendBody struct {
	r          io.Reader
	left, size int64
	st         *stallTimer
}

// Read reads on from r, refusing fewer or more bytes than size.
func (b *sendBody) Read(p []byte) (int, error) {
	b.st.stop()
	n, err := b.read(p)
	if err == nil {
		b.st.run()
	}
	return n, err
}

// read reads on from r within size. The last bytes are handed over only
// once r is known to end with them, so that the server never gets a
// whole body of a reader that was longer, nor of one that was shorter.
func (b *sendBody) read(p []byte) (int, error) {
	if b.left == 0 {
		return 0, io.EOF
	}
	if int64(len(p)) > b.left {
		p = p[:b.left]
	}
	n, err := b.r.Read(p)
	b.left -= int64(n)
	switch {
	case b.left == 0:
		var one [1]byte
		if k, _ := io.ReadFull(b.r, one[:]); k > 0 {
			return 0, fmt.Errorf("more than %d bytes", b.size)
		}
		return n, nil
	case errors.Is(err, io.EOF):
		return 0, fmt.Errorf("%d bytes, not %d", b.size-b.left, b.size)
	}
	return n, err
}

// recvBody is an answer's body read with its stall timer running while a
// read waits; a read the timer gave up says so.
type recvBody struct {
	rc   io.ReadCloser
	ctx  context.Context
	st   *stallTimer
	done func()
}

// Read reads on from the answer.
func (b *recvBody) Read(p []byte) (int, error) {
	b.st.run()
	n, err := b.rc.Read(p)
	b.st.stop()
	if err != nil && !errors.Is(err, io.EOF) {
		if cause := context.Cause(b.ctx); errors.Is(cause, errStalled) {
			err = cause
		}
	}
	return n, err
}

// Close closes the answer and ends its context.
func (b *recvBody) Close() error {
	err := b.rc.Close()
	b.done()
	return err
}
