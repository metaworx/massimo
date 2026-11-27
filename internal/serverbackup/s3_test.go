// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package serverbackup

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/md5"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"errors"
	"io"
	"io/fs"
	"maps"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"go.metaworx.ch/massimo/internal/store"
)

// The fake's credentials, no real ones.
const (
	fakeS3Key    = "MASSIMO0TEST0KEY"
	fakeS3Secret = "test-secret-not-a-real-key"
	fakeS3Region = "eu-central-1"
	fakeS3Bucket = "backups"
)

// AWS's example of deriving a signing key (IAM's documentation,
// "Examples of how to derive a signing key for Signature Version 4").
func TestS3SigningKey(t *testing.T) {
	t.Parallel()
	got := hex.EncodeToString(s3SigningKey([]byte("wJalrXUtnFEMI/K7MDENG+bPxRfiCYEXAMPLEKEY"), "20120215", "us-east-1", "iam"))
	if want := "f4780e2d9f65fa895f9c67b32ce1baf0b0d8a43505a000a1a9e090d414db404d"; got != want {
		t.Fatalf("signing key %s, want %s", got, want)
	}
}

// AWS's four examples of S3's Authorization header (Amazon S3's
// documentation, "Examples: Signature Calculations"): a GET with a range,
// a PUT whose key needs encoding with two more headers, a sub-resource
// without a value, and a list with a query.
func TestS3SignatureExamples(t *testing.T) {
	t.Parallel()
	at := time.Date(2013, 5, 24, 0, 0, 0, 0, time.UTC)
	secret := []byte("wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY")
	for _, c := range []struct {
		name, method, url string
		header            map[string]string
		payload           string
		signed, signature string
	}{
		{"GET Object", "GET", "https://examplebucket.s3.amazonaws.com/test.txt", map[string]string{"Range": "bytes=0-9"}, s3EmptyHash,
			"host;range;x-amz-content-sha256;x-amz-date", "f0e8bdb87c964420e857bd35b5d6ed310bd44f0170aba48dd91039c6036bdb41"},
		{"PUT Object", "PUT", "https://examplebucket.s3.amazonaws.com/test$file.text",
			map[string]string{"Date": "Fri, 24 May 2013 00:00:00 GMT", "X-Amz-Storage-Class": "REDUCED_REDUNDANCY"},
			"44ce7dd67c959e0d3524ffac1771dfbba87d2b6b4b4e99e42034a8b803f8b072",
			"date;host;x-amz-content-sha256;x-amz-date;x-amz-storage-class", "98ad721746da40c64f1a55b78f14c238d841ea1380cd77a1b5971af0ece108bd"},
		{"GET Bucket Lifecycle", "GET", "https://examplebucket.s3.amazonaws.com/?lifecycle", nil, s3EmptyHash,
			"host;x-amz-content-sha256;x-amz-date", "fea454ca298b7da1c68078a5d1bdbfbbe0d65c699e0f91ac7a200a0136783543"},
		{"Get Bucket (List Objects)", "GET", "https://examplebucket.s3.amazonaws.com/?max-keys=2&prefix=J", nil, s3EmptyHash,
			"host;x-amz-content-sha256;x-amz-date", "34b48302e7b5fa45bde8084f4b7868a86f0a534bc59db6670ed5711ef69dc6f7"},
	} {
		req, err := http.NewRequest(c.method, c.url, nil)
		if err != nil {
			t.Fatal(err)
		}
		for k, v := range c.header {
			req.Header.Set(k, v)
		}
		signV4(req, "AKIAIOSFODNN7EXAMPLE", secret, "us-east-1", c.payload, at)
		want := "AWS4-HMAC-SHA256 Credential=AKIAIOSFODNN7EXAMPLE/20130524/us-east-1/s3/aws4_request,SignedHeaders=" + c.signed + ",Signature=" + c.signature
		if got := req.Header.Get("Authorization"); got != want {
			t.Errorf("%s:\n got %s\nwant %s", c.name, got, want)
		}
	}
}

// fakeObject is an object the fake keeps.
type fakeObject struct {
	data []byte
	etag string
	at   time.Time
}

// fakeS3 is an S3 server in memory with one bucket, which checks each
// request's signature by its own reading of SigV4 and of the wire.
type fakeS3 struct {
	t    *testing.T
	page int // keys per page of a list

	mu      sync.Mutex
	objects map[string]fakeObject
	locked  map[string]bool
	calls   []string
	// refuse answers a request before the fake does, where it returns
	// a status.
	refuse func(r *http.Request, key string) (status int, code, message string)
	// handle takes a request over, where it returns true.
	handle func(w http.ResponseWriter, r *http.Request, key string) bool
}

// newFakeS3 starts a fake whose lists answer two keys a page.
func newFakeS3(t *testing.T) (*fakeS3, *httptest.Server) {
	t.Helper()
	f := &fakeS3{t: t, page: 2, objects: map[string]fakeObject{}, locked: map[string]bool{}}
	srv := httptest.NewServer(f)
	t.Cleanup(srv.Close)
	return f, srv
}

// s3Target is an S3 target at the fake, by path, under massimo/, the
// fields in cfg replacing those.
func s3Target(t *testing.T, srv *httptest.Server, cfg map[string]string) *S3 {
	t.Helper()
	all := map[string]string{"endpoint": srv.URL, "region": fakeS3Region, "bucket": fakeS3Bucket, "prefix": "massimo", "path_style": "true", "access_key": fakeS3Key}
	secret := fakeS3Secret
	for k, v := range cfg {
		if k == "secret_key" {
			secret = v
			continue
		}
		all[k] = v
	}
	s, err := NewS3(all, []byte(secret))
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// virtualTarget is an S3 target at the fake by virtual host:
// backups.s3.example.test, dialled to the fake.
func virtualTarget(t *testing.T, srv *httptest.Server, prefix string) *S3 {
	t.Helper()
	_, port, _ := net.SplitHostPort(srv.Listener.Addr().String())
	s := s3Target(t, srv, map[string]string{"endpoint": "http://s3.example.test:" + port, "path_style": "false", "prefix": prefix})
	d := &net.Dialer{Timeout: 5 * time.Second}
	s.client = &http.Client{
		Transport: &http.Transport{DialContext: func(ctx context.Context, network, _ string) (net.Conn, error) {
			return d.DialContext(ctx, network, srv.Listener.Addr().String())
		}},
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	return s
}

// put places an object in the fake, as another program would.
func (f *fakeS3) put(key string, data []byte) {
	f.mu.Lock()
	defer f.mu.Unlock()
	sum := md5.Sum(data)
	f.objects[key] = fakeObject{data: data, etag: `"` + hex.EncodeToString(sum[:]) + `"`, at: time.Now().UTC()}
}

// set changes the fake under its lock.
func (f *fakeS3) set(change func(f *fakeS3)) {
	f.mu.Lock()
	defer f.mu.Unlock()
	change(f)
}

// keys lists the fake's keys.
func (f *fakeS3) keys() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []string
	for k := range f.objects {
		out = append(out, k)
	}
	slices.Sort(out)
	return out
}

// called lists the calls the fake answered and forgets them.
func (f *fakeS3) called() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := f.calls
	f.calls = nil
	return out
}

// awsEscape is the test's own SigV4 encoding of a query part or of a
// path's segment.
func awsEscape(v string) string {
	return strings.ReplaceAll(strings.ReplaceAll(url.QueryEscape(v), "+", "%20"), "%7E", "~")
}

// verify checks a request's signature from what came over the wire: the
// raw path, which must be the canonical one, the query, and the headers
// signed. It returns S3's code and message where it fails.
func (f *fakeS3) verify(r *http.Request, rawPath, rawQuery string) (status int, code, message, region string) {
	rest, ok := strings.CutPrefix(r.Header.Get("Authorization"), "AWS4-HMAC-SHA256 ")
	if !ok {
		return 403, "AccessDenied", "not signed", ""
	}
	parts := map[string]string{}
	for p := range strings.SplitSeq(rest, ",") {
		k, v, _ := strings.Cut(strings.TrimSpace(p), "=")
		parts[k] = v
	}
	cred := strings.Split(parts["Credential"], "/")
	switch {
	case len(cred) != 5 || cred[0] != fakeS3Key || cred[3] != "s3" || cred[4] != "aws4_request":
		return 403, "InvalidAccessKeyId", "The AWS Access Key Id you provided does not exist in our records.", ""
	case cred[2] != fakeS3Region:
		return 400, "AuthorizationHeaderMalformed", "the region '" + cred[2] + "' is wrong; expecting '" + fakeS3Region + "'", fakeS3Region
	case !strings.HasPrefix(r.Header.Get("X-Amz-Date"), cred[1]):
		return 403, "SignatureDoesNotMatch", "the date is not the credential's", ""
	}
	var segs []string
	for seg := range strings.SplitSeq(rawPath, "/") {
		d, err := url.PathUnescape(seg)
		if err != nil {
			return 400, "InvalidURI", "bad path", ""
		}
		segs = append(segs, awsEscape(d))
	}
	if strings.Join(segs, "/") != rawPath {
		return 403, "SignatureDoesNotMatch", "the path " + rawPath + " is not in its canonical form", ""
	}
	q, err := url.ParseQuery(rawQuery)
	if err != nil {
		return 400, "InvalidArgument", "bad query", ""
	}
	var pairs [][2]string
	for k, vs := range q {
		for _, v := range vs {
			pairs = append(pairs, [2]string{awsEscape(k), awsEscape(v)})
		}
	}
	sort.Slice(pairs, func(i, j int) bool {
		return pairs[i][0] < pairs[j][0] || (pairs[i][0] == pairs[j][0] && pairs[i][1] < pairs[j][1])
	})
	var query []string
	for _, p := range pairs {
		query = append(query, p[0]+"="+p[1])
	}
	signed := strings.Split(parts["SignedHeaders"], ";")
	for _, must := range []string{"host", "x-amz-content-sha256", "x-amz-date"} {
		if !slices.Contains(signed, must) {
			return 403, "AccessDenied", must + " not signed", ""
		}
	}
	var headers strings.Builder
	for _, h := range signed {
		vs := slices.Clone(r.Header.Values(h))
		if h == "host" {
			vs = []string{r.Host}
		}
		for i, v := range vs {
			vs[i] = strings.Join(strings.Fields(v), " ")
		}
		headers.WriteString(h + ":" + strings.Join(vs, ",") + "\n")
	}
	payload := r.Header.Get("X-Amz-Content-Sha256")
	canonical := strings.Join([]string{r.Method, rawPath, strings.Join(query, "&"), headers.String(), parts["SignedHeaders"], payload}, "\n")
	sum := sha256.Sum256([]byte(canonical))
	toSign := "AWS4-HMAC-SHA256\n" + r.Header.Get("X-Amz-Date") + "\n" + strings.Join(cred[1:], "/") + "\n" + hex.EncodeToString(sum[:])
	m := hmac.New(sha256.New, s3SigningKey([]byte(fakeS3Secret), cred[1], cred[2], "s3"))
	m.Write([]byte(toSign))
	if !hmac.Equal([]byte(hex.EncodeToString(m.Sum(nil))), []byte(parts["Signature"])) {
		return 403, "SignatureDoesNotMatch", "The request signature we calculated does not match the signature you provided. Check your key and signing method.", ""
	}
	if payload != "UNSIGNED-PAYLOAD" && r.Method != http.MethodPut && payload != s3EmptyHash {
		return 400, "XAmzContentSHA256Mismatch", "a body's hash on a request without one", ""
	}
	return 0, "", "", ""
}

// fail answers an S3 error.
func fakeFail(w http.ResponseWriter, r *http.Request, status int, code, message, region string) {
	if region != "" {
		w.Header().Set("X-Amz-Bucket-Region", region)
	}
	if status >= 300 && status < 400 {
		w.Header().Set("Location", "http://elsewhere.example.test/")
	}
	w.Header().Set("Content-Type", "application/xml")
	w.WriteHeader(status)
	if r.Method == http.MethodHead {
		return
	}
	xml.NewEncoder(w).Encode(struct {
		XMLName xml.Name `xml:"Error"`
		Code    string
		Message string
		Region  string `xml:",omitempty"`
	}{Code: code, Message: message, Region: region})
}

// ServeHTTP answers PUT, GET, HEAD and DELETE of an object and
// ListObjectsV2 of the bucket, by path or by virtual host.
func (f *fakeS3) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	rawPath, rawQuery, _ := strings.Cut(r.RequestURI, "?")
	var rawKey string
	if strings.HasPrefix(r.Host, fakeS3Bucket+".") {
		rawKey = strings.TrimPrefix(rawPath, "/")
	} else {
		rest, ok := strings.CutPrefix(rawPath, "/"+fakeS3Bucket)
		if !ok || (rest != "" && !strings.HasPrefix(rest, "/")) {
			fakeFail(w, r, 404, "NoSuchBucket", "The specified bucket does not exist", "")
			return
		}
		rawKey = strings.TrimPrefix(rest, "/")
	}
	key, _ := url.PathUnescape(rawKey)
	if status, code, message, region := f.verify(r, rawPath, rawQuery); status != 0 {
		fakeFail(w, r, status, code, message, region)
		return
	}
	f.mu.Lock()
	call := r.Method + " " + key
	if key == "" && r.URL.Query().Get("list-type") == "2" {
		call = "LIST " + r.URL.Query().Get("prefix")
	}
	f.calls = append(f.calls, call)
	refuse, handle := f.refuse, f.handle
	f.mu.Unlock()
	if refuse != nil {
		if status, code, message := refuse(r, key); status != 0 {
			fakeFail(w, r, status, code, message, "")
			return
		}
	}
	if handle != nil && handle(w, r, key) {
		return
	}
	switch {
	case key == "" && r.Method == http.MethodGet:
		f.list(w, r)
	case key == "":
		fakeFail(w, r, 405, "MethodNotAllowed", "not on the bucket", "")
	case r.Method == http.MethodPut:
		data, err := io.ReadAll(r.Body)
		if err != nil {
			return // broken off: nothing stored
		}
		if int64(len(data)) != r.ContentLength {
			fakeFail(w, r, 400, "IncompleteBody", "fewer bytes than the Content-Length", "")
			return
		}
		f.put(key, data)
		w.WriteHeader(http.StatusOK)
	case r.Method == http.MethodGet || r.Method == http.MethodHead:
		f.get(w, r, key)
	case r.Method == http.MethodDelete:
		f.mu.Lock()
		locked := f.locked[key]
		if !locked {
			delete(f.objects, key)
		}
		f.mu.Unlock()
		if locked {
			fakeFail(w, r, 403, "AccessDenied", "Access Denied because object protected by object lock.", "")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		fakeFail(w, r, 405, "MethodNotAllowed", r.Method, "")
	}
}

// get answers a GET or a HEAD of an object, with its range and If-Match.
func (f *fakeS3) get(w http.ResponseWriter, r *http.Request, key string) {
	f.mu.Lock()
	o, ok := f.objects[key]
	f.mu.Unlock()
	switch {
	case !ok:
		fakeFail(w, r, 404, "NoSuchKey", "The specified key does not exist.", "")
		return
	case r.Header.Get("If-Match") != "" && r.Header.Get("If-Match") != o.etag:
		fakeFail(w, r, 412, "PreconditionFailed", "At least one of the pre-conditions you specified did not hold", "")
		return
	}
	w.Header().Set("ETag", o.etag)
	w.Header().Set("Last-Modified", o.at.Format(http.TimeFormat))
	data, status := o.data, http.StatusOK
	if rg, ok := strings.CutPrefix(r.Header.Get("Range"), "bytes="); ok {
		a, b, _ := strings.Cut(rg, "-")
		first, err1 := strconv.ParseInt(a, 10, 64)
		last, err2 := strconv.ParseInt(b, 10, 64)
		if err1 != nil || err2 != nil || first >= int64(len(data)) || last < first {
			fakeFail(w, r, 416, "InvalidRange", "The requested range is not satisfiable", "")
			return
		}
		last = min(last, int64(len(data))-1)
		w.Header().Set("Content-Range", "bytes "+a+"-"+strconv.FormatInt(last, 10)+"/"+strconv.Itoa(len(data)))
		data, status = data[first:last+1], http.StatusPartialContent
	}
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))
	w.WriteHeader(status)
	if r.Method == http.MethodGet {
		w.Write(data)
	}
}

// list answers ListObjectsV2: the keys under the prefix, those below the
// delimiter as common prefixes, f.page keys a page, the continuation
// token an opaque one with characters a query encodes.
func (f *fakeS3) list(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	prefix, after := q.Get("prefix"), ""
	if tok := q.Get("continuation-token"); tok != "" {
		b, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(tok, "next+/"))
		if err != nil {
			fakeFail(w, r, 400, "InvalidArgument", "The continuation token provided is incorrect", "")
			return
		}
		after = string(b)
	}
	max := f.page
	if m, err := strconv.Atoi(q.Get("max-keys")); err == nil && m > 0 {
		max = min(m, max)
	}
	type content struct {
		Key          string
		LastModified string
		ETag         string
		Size         int64
	}
	var res struct {
		XMLName               xml.Name `xml:"http://s3.amazonaws.com/doc/2006-03-01/ ListBucketResult"`
		Name                  string
		Prefix                string
		KeyCount              int
		IsTruncated           bool
		NextContinuationToken string `xml:",omitempty"`
		Contents              []content
		CommonPrefixes        []struct{ Prefix string }
	}
	res.Name, res.Prefix = fakeS3Bucket, prefix
	f.mu.Lock()
	for _, k := range f.keysLocked() {
		rest, ok := strings.CutPrefix(k, prefix)
		if !ok || k <= after {
			continue
		}
		if len(res.Contents) == max {
			res.IsTruncated = true
			res.NextContinuationToken = "next+/" + base64.StdEncoding.EncodeToString([]byte(res.Contents[max-1].Key))
			break
		}
		if q.Get("delimiter") == "/" && strings.Contains(rest, "/") {
			res.CommonPrefixes = append(res.CommonPrefixes, struct{ Prefix string }{prefix + rest[:strings.Index(rest, "/")+1]})
			continue
		}
		o := f.objects[k]
		res.Contents = append(res.Contents, content{Key: k, LastModified: o.at.Format("2006-01-02T15:04:05.000Z"), ETag: o.etag, Size: int64(len(o.data))})
	}
	f.mu.Unlock()
	res.KeyCount = len(res.Contents)
	w.Header().Set("Content-Type", "application/xml")
	io.WriteString(w, xml.Header)
	xml.NewEncoder(w).Encode(res)
}

// keysLocked is keys with the lock held.
func (f *fakeS3) keysLocked() []string {
	out := make([]string, 0, len(f.objects))
	for k := range f.objects {
		out = append(out, k)
	}
	slices.Sort(out)
	return out
}

// A round trip by path and by virtual host, the second under a prefix
// that needs encoding: the check leaves nothing; puts, a list over pages
// that leaves out what is no backup under the prefix; a scan that reads a
// real backup's manifest by ranged reads; reads at the end; deletes.
func TestS3RoundTrip(t *testing.T) {
	t.Parallel()
	st := testStore(t)
	r, _ := keyOf(t, KindX25519, "laptop")
	path, m := write(t, st, Inputs{Trigger: TriggerScheduled}, r)
	backup, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		name, prefix string
		target       func(*httptest.Server) *S3
	}{
		{"path", "massimo/", func(srv *httptest.Server) *S3 { return s3Target(t, srv, nil) }},
		{"virtual host", "Massimo Backups/box $1/", func(srv *httptest.Server) *S3 { return virtualTarget(t, srv, "/Massimo Backups/box $1/") }},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			f, srv := newFakeS3(t)
			s := c.target(srv)
			if err := s.Check(ctx); err != nil {
				t.Fatal(err)
			}
			if keys, calls := f.keys(), f.called(); len(keys) != 0 || len(calls) != 4 || calls[0] != "LIST "+c.prefix ||
				!strings.HasPrefix(calls[1], "PUT "+c.prefix+".massimo-check-") || calls[2] != "GET"+calls[1][3:] || calls[3] != "DELETE"+calls[1][3:] {
				t.Fatalf("the check left %v, called %q", keys, calls)
			}
			if err := s.Put(ctx, FileName(m), bytes.NewReader(backup), int64(len(backup))); err != nil {
				t.Fatal(err)
			}
			names := []string{FileName(m)}
			for _, n := range []string{"massimo-a.zip", "massimo-b.zip", "massimo-c.zip", "massimo-d.zip"} {
				if err := s.Put(ctx, n, strings.NewReader("no backup "+n), int64(len("no backup "+n))); err != nil {
					t.Fatal(err)
				}
				names = append(names, n)
			}
			if err := s.Put(ctx, "empty.zip", strings.NewReader(""), 0); err != nil {
				t.Fatal(err)
			}
			names = append(names, "empty.zip")
			for _, other := range []string{"other/x.zip", c.prefix + "old/y.zip", c.prefix + "notes.txt", c.prefix + ".hidden.zip", "z.zip"} {
				f.put(other, []byte("x"))
			}
			list, err := s.List(ctx)
			if err != nil {
				t.Fatal(err)
			}
			var got []string
			for _, e := range list {
				got = append(got, e.Name)
				if e.Time.IsZero() {
					t.Errorf("%s without a time", e.Name)
				}
				if e.Name == FileName(m) && e.Size != int64(len(backup)) {
					t.Errorf("%s of %d bytes, not %d", e.Name, e.Size, len(backup))
				}
			}
			slices.Sort(got)
			slices.Sort(names)
			if !slices.Equal(got, names) {
				t.Fatalf("listed %q, want %q", got, names)
			}
			if n := strings.Count(strings.Join(f.called(), "\n"), "LIST "); n < 4 {
				t.Fatalf("listed in %d pages", n)
			}
			found, skipped, err := Scan(ctx, s)
			if err != nil || len(found) != 1 || len(skipped) != 5 || found[0].Manifest.UUID != m.UUID || !found[0].Signature.Valid {
				t.Fatalf("scan %+v %+v %v", found, skipped, err)
			}
			file, size, err := s.Open(ctx, FileName(m))
			if err != nil || size != int64(len(backup)) {
				t.Fatalf("open %d %v", size, err)
			}
			p := make([]byte, 10)
			if n, err := file.ReadAt(p, size-4); n != 4 || err != io.EOF || !bytes.Equal(p[:4], backup[len(backup)-4:]) {
				t.Fatalf("a read past the end %d %v", n, err)
			}
			if n, err := file.ReadAt(p, size); n != 0 || err != io.EOF {
				t.Fatalf("a read at the end %d %v", n, err)
			}
			file.Close()
			if _, err := file.ReadAt(p, 0); !errors.Is(err, fs.ErrClosed) {
				t.Fatalf("a read after the close: %v", err)
			}
			for range 2 {
				if err := s.Delete(ctx, FileName(m)); err != nil {
					t.Fatal(err)
				}
			}
			if _, _, err := s.Open(ctx, FileName(m)); !errors.Is(err, fs.ErrNotExist) {
				t.Fatalf("open of a name deleted: %v", err)
			}
			if list, _ := s.List(ctx); len(list) != len(names)-1 {
				t.Fatalf("after the delete %+v", list)
			}
		})
	}
}

// A put refused or broken off leaves nothing: more than one PUT takes
// (refused before any request), a reader that fails, fewer or more bytes
// than the size, a context ended, a name no backup has.
func TestS3PutRefusals(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f, srv := newFakeS3(t)
	s := s3Target(t, srv, nil)
	err := s.Put(ctx, "big.zip", strings.NewReader(""), s3MaxPut+1)
	if err == nil || !strings.Contains(err.Error(), "5 GiB") || len(f.called()) != 0 {
		t.Fatalf("a put over 5 GiB: %v", err)
	}
	if err := s.Put(ctx, "../a.zip", strings.NewReader("data"), 4); err == nil || len(f.called()) != 0 {
		t.Fatalf("a name outside: %v", err)
	}
	if err := s.Put(ctx, "a.zip", &failingReader{n: 100 << 10}, 1<<20); err == nil {
		t.Fatal("a put broken off succeeded")
	}
	if err := s.Put(ctx, "a.zip", strings.NewReader("short"), 1000); err == nil {
		t.Fatal("a short put succeeded")
	}
	if err := s.Put(ctx, "a.zip", strings.NewReader("too long"), 3); err == nil {
		t.Fatal("a long put succeeded")
	}
	big := bytes.Repeat([]byte("x"), 300<<10)
	if err := s.Put(ctx, "a.zip", bytes.NewReader(append(big, 'y')), int64(len(big))); err == nil {
		t.Fatal("a long put of many reads succeeded")
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if err := s.Put(cancelled, "a.zip", strings.NewReader("data"), 4); err == nil {
		t.Fatal("a put of a context ended succeeded")
	}
	if keys := f.keys(); len(keys) != 0 {
		t.Fatalf("left behind: %v", keys)
	}
}

// What the server refuses is said, with S3's code, and never with the
// secret: a wrong secret, the wrong region (the bucket's named), an
// error of the server, a redirect (not followed), a bucket not there.
func TestS3Errors(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f, srv := newFakeS3(t)
	wrong := s3Target(t, srv, map[string]string{"secret_key": "another-secret-key"})
	err := wrong.Check(ctx)
	if err == nil || !strings.Contains(err.Error(), "403 Forbidden: SignatureDoesNotMatch") || !strings.Contains(err.Error(), "S3 bucket backups on 127.0.0.1:") {
		t.Fatalf("a wrong secret: %v", err)
	}
	if strings.Contains(err.Error(), "another-secret-key") || strings.Contains(err.Error(), fakeS3Secret) {
		t.Fatalf("the secret in an error: %v", err)
	}
	if s := wrong.String(); strings.Contains(s, "secret") {
		t.Fatalf("the target's name %q", s)
	}
	err = s3Target(t, srv, map[string]string{"region": "us-west-2"}).Check(ctx)
	if err == nil || !strings.Contains(err.Error(), "AuthorizationHeaderMalformed") || !strings.Contains(err.Error(), "the bucket is in region "+fakeS3Region) {
		t.Fatalf("the wrong region: %v", err)
	}
	s := s3Target(t, srv, nil)
	refuse := func(r *http.Request, key string) (int, string, string) {
		switch {
		case strings.HasSuffix(key, "/a.zip"):
			return 500, "InternalError", "We encountered an internal error. Please try again."
		case strings.HasSuffix(key, "/b.zip"):
			return 307, "TemporaryRedirect", "Please re-send this request to the specified temporary endpoint."
		}
		return 0, "", ""
	}
	f.set(func(f *fakeS3) { f.refuse = refuse })
	f.called()
	if err := s.Put(ctx, "a.zip", strings.NewReader("data"), 4); err == nil || !strings.Contains(err.Error(), "PUT massimo/a.zip: 500 Internal Server Error: InternalError") {
		t.Fatalf("an error of the server: %v", err)
	}
	if err := s.Put(ctx, "b.zip", strings.NewReader("data"), 4); err == nil || !strings.Contains(err.Error(), "307") {
		t.Fatalf("a redirect: %v", err)
	}
	if calls := f.called(); len(calls) != 2 {
		t.Fatalf("a redirect followed: %q", calls)
	}
	other := s3Target(t, srv, map[string]string{"bucket": "elsewhere"})
	if err := other.Delete(ctx, "a.zip"); err == nil || !strings.Contains(err.Error(), "NoSuchBucket") {
		t.Fatalf("a delete in a bucket not there: %v", err)
	}
}

// A delete the bucket refuses for its lock is ErrRetained, not a name
// gone: AWS's answer for a locked version, MinIO's WORM; other refusals
// are not. A prune keeps such a copy, warns and goes on, and deletes it
// once the lock is gone.
func TestS3ObjectLock(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f, srv := newFakeS3(t)
	s := s3Target(t, srv, nil)
	f.put("massimo/a.zip", []byte("a"))
	f.set(func(f *fakeS3) { f.locked["massimo/a.zip"] = true })
	err := s.Delete(ctx, "a.zip")
	if !errors.Is(err, ErrRetained) || !strings.Contains(err.Error(), "object lock") {
		t.Fatalf("a locked delete: %v", err)
	}
	if keys := f.keys(); len(keys) != 1 {
		t.Fatalf("the locked copy went: %v", keys)
	}
	refuse := func(r *http.Request, key string) (int, string, string) {
		switch {
		case strings.HasSuffix(key, "/worm.zip"):
			return 400, "InvalidRequest", "Object is WORM protected and cannot be overwritten"
		case strings.HasSuffix(key, "/denied.zip") && r.Method == http.MethodPut:
			return 403, "AccessDenied", "Access Denied."
		case strings.HasSuffix(key, "/broken.zip"):
			return 500, "InternalError", "We encountered an internal error. Please try again."
		}
		return 0, "", ""
	}
	f.set(func(f *fakeS3) { f.refuse = refuse })
	if err := s.Delete(ctx, "worm.zip"); !errors.Is(err, ErrRetained) {
		t.Fatalf("MinIO's WORM: %v", err)
	}
	if err := s.Put(ctx, "denied.zip", strings.NewReader("d"), 1); err == nil || errors.Is(err, ErrRetained) {
		t.Fatalf("a put refused is retained: %v", err)
	}
	if err := s.Delete(ctx, "broken.zip"); err == nil || errors.Is(err, ErrRetained) {
		t.Fatalf("an error of the server is retained: %v", err)
	}
	f.set(func(f *fakeS3) {
		f.refuse = nil
		clear(f.locked)
		clear(f.objects)
	})

	// The catalogue over the fake: the last one kept, the first locked.
	c, at := catalogue(t)
	ret, _ := json.Marshal(Policy{Last: 1})
	tg, err := c.Store.AddServerBackupTarget(ctx, store.ServerBackupTarget{Name: "bucket", Kind: KindS3, Secret: fakeS3Secret, Takes: string(TakesEvery), Retention: ret, Enabled: true,
		Config: map[string]string{"endpoint": srv.URL, "region": fakeS3Region, "bucket": fakeS3Bucket, "prefix": "box", "path_style": "true", "access_key": fakeS3Key}}, 0)
	if err != nil {
		t.Fatal(err)
	}
	first, outs, err := c.Make(ctx, MakeRequest{Trigger: TriggerScheduled})
	if err != nil || len(first.Copies) != 1 || first.Copies[0].State != store.CopyThere {
		t.Fatalf("the first backup %+v %+v %v", first, outs, err)
	}
	f.set(func(f *fakeS3) { f.locked["box/"+first.Copies[0].Path] = true })
	at.Add(int64(24 * time.Hour))
	if _, outs, err = c.Make(ctx, MakeRequest{Trigger: TriggerScheduled}); err != nil {
		t.Fatal(err)
	}
	for _, o := range outs {
		if o.Err != nil || (o.Step == "prune" && o.Removed != 0) {
			t.Fatalf("a locked copy failed or removed the prune: %+v", outs)
		}
	}
	if b, ok, _ := c.Store.ServerBackup(ctx, first.UUID); !ok || len(b.Copies) != 1 || b.Copies[0].State != store.CopyThere {
		t.Fatalf("the locked copy left the catalogue: %+v %v", b, ok)
	}
	if keys := f.keys(); len(keys) != 2 {
		t.Fatalf("on the bucket: %v", keys)
	}
	f.set(func(f *fakeS3) { clear(f.locked) })
	if tg, _, err = c.Store.ServerBackupTarget(ctx, tg.UUID); err != nil {
		t.Fatal(err)
	}
	if n, err := c.Prune(ctx, tg); err != nil || n != 1 {
		t.Fatalf("the prune once unlocked: %d %v", n, err)
	}
	if keys := f.keys(); len(keys) != 1 {
		t.Fatalf("on the bucket after the prune: %v", keys)
	}
}

// The fields: an endpoint as a URL without a path, https by default and
// without its own port; a bucket a host name takes where it is reached by
// virtual host; a prefix of folders; the keys.
func TestS3Config(t *testing.T) {
	t.Parallel()
	ok := map[string]string{"endpoint": "https://s3.example.test", "bucket": "backups", "access_key": "key"}
	with := func(kv ...string) map[string]string {
		m := maps.Clone(ok)
		for i := 0; i < len(kv); i += 2 {
			m[kv[i]] = kv[i+1]
		}
		return m
	}
	for _, c := range []struct {
		cfg                          map[string]string
		scheme, host, prefix, region string
	}{
		{ok, "https", "s3.example.test", "", s3DefaultRegion},
		{with("endpoint", "S3.Example.Test:443/", "region", "auto"), "https", "s3.example.test", "", "auto"},
		{with("endpoint", "http://minio.example.test:9000", "path_style", "true", "prefix", "/a/b/"), "http", "minio.example.test:9000", "a/b/", s3DefaultRegion},
		{with("endpoint", "http://[::1]:80", "path_style", "true", "bucket", "Old_Bucket"), "http", "[::1]", "", s3DefaultRegion},
		{with("endpoint", "https://192.0.2.1:9000", "path_style", "true", "bucket", "a.b.c"), "https", "192.0.2.1:9000", "", s3DefaultRegion},
	} {
		s, err := NewS3(c.cfg, []byte("secret"))
		if err != nil || s.scheme != c.scheme || s.host != c.host || s.prefix != c.prefix || s.region != c.region {
			t.Errorf("%v: %+v %v", c.cfg, s, err)
		}
	}
	for name, cfg := range map[string]map[string]string{
		"no endpoint":           with("endpoint", ""),
		"a path":                with("endpoint", "https://s3.example.test/backups"),
		"a query":               with("endpoint", "https://s3.example.test?x=1"),
		"a user":                with("endpoint", "https://me@s3.example.test"),
		"ftp":                   with("endpoint", "ftp://s3.example.test"),
		"an address by host":    with("endpoint", "http://192.0.2.1:9000"),
		"a bucket no host":      with("bucket", "Old_Bucket"),
		"dots over https":       with("bucket", "a.b.c"),
		"a short bucket":        with("bucket", "ab"),
		"a bucket with a slash": with("bucket", "a/b", "path_style", "true"),
		"a prefix ..":           with("prefix", "a/../b"),
		"a prefix //":           with("prefix", "a//b"),
		"a long prefix":         with("prefix", strings.Repeat("a", s3MaxPrefix+1)),
		"a region with /":       with("region", "eu/central"),
		"a key with a comma":    with("access_key", "a,b"),
		"no key":                with("access_key", ""),
		"a bad bool":            with("path_style", "maybe"),
	} {
		if _, err := NewS3(cfg, []byte("secret")); err == nil {
			t.Errorf("%s taken", name)
		}
	}
	if _, err := NewS3(ok, nil); err == nil {
		t.Error("no secret taken")
	}
	k, found := KindOf(KindS3)
	if f, has := k.Secret(); !found || !has || f.Key != "secret_key" {
		t.Fatalf("the kind %+v", k)
	}
	cfg, err := k.Check(map[string]string{"endpoint": "s3.example.test", "bucket": "backups", "access_key": "key"}, true)
	if err != nil || cfg["region"] != s3DefaultRegion || cfg["path_style"] != "false" {
		t.Fatalf("the kind's defaults %v %v", cfg, err)
	}
	if _, err := k.New(cfg, []byte("secret")); err != nil {
		t.Fatal(err)
	}
}

// A copy read in small pieces costs few requests, each read fetching
// more ahead than the one before; reads out of order are right; an
// object replaced while it is read is refused (If-Match).
func TestS3ReadAhead(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f, srv := newFakeS3(t)
	s := s3Target(t, srv, nil)
	data := make([]byte, 1<<20+123)
	rand.Read(data)
	f.put("massimo/a.zip", data)
	file, size, err := s.Open(ctx, "a.zip")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	f.called()
	var got bytes.Buffer
	// The writer hides got's ReadFrom, so that the copy reads by 32 KiB.
	if _, err := io.CopyBuffer(struct{ io.Writer }{&got}, io.NewSectionReader(file, 0, size), make([]byte, 32<<10)); err != nil || !bytes.Equal(got.Bytes(), data) {
		t.Fatalf("the copy: %v", err)
	}
	if calls := f.called(); len(calls) > 6 {
		t.Fatalf("%d requests for %d bytes in pieces of 32 KiB", len(calls), size)
	}
	p := make([]byte, 1000)
	for _, off := range []int64{size - 1000, 0, 500000, 12345} {
		if n, err := file.ReadAt(p, off); n != 1000 || err != nil || !bytes.Equal(p, data[off:off+1000]) {
			t.Fatalf("a read at %d: %d %v", off, n, err)
		}
	}
	whole := make([]byte, size)
	if n, err := file.ReadAt(whole, 0); n != int(size) || err != nil || !bytes.Equal(whole, data) {
		t.Fatalf("a read of the whole: %d %v", n, err)
	}
	f.put("massimo/a.zip", append([]byte("changed"), data...))
	if _, err := file.ReadAt(p, 700000); err == nil || !strings.Contains(err.Error(), "412") {
		t.Fatalf("a read of an object replaced: %v", err)
	}
}

// zeros is an endless reader of zeros.
type zeros struct{}

func (zeros) Read(p []byte) (int, error) { clear(p); return len(p), nil }

// A request that moves nothing is broken off: an answer that stops
// coming, a put the server does not read.
func TestS3Stall(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f, srv := newFakeS3(t)
	s := s3Target(t, srv, nil)
	s.stall = 300 * time.Millisecond
	f.put("massimo/a.zip", make([]byte, 1000))
	// A server that does not read a body does not see the client go:
	// the handlers are let go before the server closes.
	release := make(chan struct{})
	t.Cleanup(func() { close(release) })
	handle := func(w http.ResponseWriter, r *http.Request, key string) bool {
		switch r.Method {
		case http.MethodHead:
			return false
		case http.MethodGet:
			w.Header().Set("Content-Range", "bytes 0-999/1000")
			w.Header().Set("Content-Length", "1000")
			w.WriteHeader(http.StatusPartialContent)
			w.Write(make([]byte, 10))
			w.(http.Flusher).Flush()
		case http.MethodPut:
			io.ReadFull(r.Body, make([]byte, 1<<10)) // 100 Continue, then nothing more
		}
		select {
		case <-r.Context().Done():
		case <-release:
		}
		return true
	}
	f.set(func(f *fakeS3) { f.handle = handle })
	file, size, err := s.Open(ctx, "a.zip")
	if err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	if _, err := file.ReadAt(make([]byte, size), 0); !errors.Is(err, errS3Stalled) {
		t.Fatalf("a stalled answer: %v", err)
	}
	if err := s.Put(ctx, "b.zip", io.LimitReader(zeros{}, 256<<20), 256<<20); !errors.Is(err, errS3Stalled) {
		t.Fatalf("a stalled put: %v", err)
	}
	if d := time.Since(start); d > 20*time.Second {
		t.Fatalf("broken off after %v", d)
	}
}

// The target against a real S3 server, MinIO in make target-test: the
// bucket made where it is missing, the check, a real backup scanned by
// ranged reads, a larger copy read whole, a put broken off leaving
// nothing, the list and the deletes; all under a prefix of its own.
func TestS3Live(t *testing.T) {
	env := func(name string) string { return os.Getenv("MASSIMO_TARGET_TEST_S3_" + name) }
	if env("ENDPOINT") == "" {
		t.Skip("MASSIMO_TARGET_TEST_S3_ENDPOINT is not set (make target-test)")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	var rnd [6]byte
	rand.Read(rnd[:])
	prefix := "massimo-live-" + hex.EncodeToString(rnd[:])
	s, err := NewS3(map[string]string{"endpoint": env("ENDPOINT"), "region": env("REGION"), "bucket": env("BUCKET"), "prefix": prefix,
		"path_style": "true", "access_key": env("KEY")}, []byte(env("SECRET")))
	if err != nil {
		t.Fatal(err)
	}
	resp, err := s.do(ctx, s3Call{method: http.MethodPut})
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusConflict {
		t.Fatalf("the bucket: %s", resp.Status)
	}
	if err := s.Check(ctx); err != nil {
		t.Fatal(err)
	}
	st := testStore(t)
	r, _ := keyOf(t, KindX25519, "laptop")
	path, m := write(t, st, Inputs{Trigger: TriggerScheduled}, r)
	backup, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Put(ctx, FileName(m), bytes.NewReader(backup), int64(len(backup))); err != nil {
		t.Fatal(err)
	}
	large := make([]byte, 3<<20+7)
	rand.Read(large)
	if err := s.Put(ctx, "large.zip", bytes.NewReader(large), int64(len(large))); err != nil {
		t.Fatal(err)
	}
	if err := s.Put(ctx, "broken.zip", &failingReader{n: 1 << 20}, 2<<20); err == nil {
		t.Fatal("a put broken off succeeded")
	}
	found, skipped, err := Scan(ctx, s)
	if err != nil || len(found) != 1 || found[0].Manifest.UUID != m.UUID || len(skipped) != 1 || skipped[0].Name != "large.zip" {
		t.Fatalf("scan %+v %+v %v", found, skipped, err)
	}
	f, size, err := s.Open(ctx, "large.zip")
	if err != nil || size != int64(len(large)) {
		t.Fatalf("open %d %v", size, err)
	}
	h := sha256.New()
	if _, err := io.Copy(h, io.NewSectionReader(f, 0, size)); err != nil {
		t.Fatal(err)
	}
	f.Close()
	if want := sha256.Sum256(large); !bytes.Equal(h.Sum(nil), want[:]) {
		t.Fatal("the large copy read back other bytes")
	}
	for _, n := range []string{FileName(m), "large.zip", "large.zip", "broken.zip"} {
		if err := s.Delete(ctx, n); err != nil {
			t.Fatal(err)
		}
	}
	if list, err := s.List(ctx); err != nil || len(list) != 0 {
		t.Fatalf("left behind %+v %v", list, err)
	}
}
