// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"maps"
	"math"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"go.metaworx.ch/massimo/internal/cubelan"
	"go.metaworx.ch/massimo/internal/testkit"
)

func TestUIServed(t *testing.T) {
	t.Parallel()
	ts := startServer(t, withOptions(Options{Token: "secret"})).ts
	get := func(path string) (int, string, string) {
		resp, err := http.Get(ts.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		return resp.StatusCode, resp.Header.Get("Content-Type"), string(body)
	}
	// The page and its files are public even with a token configured.
	if code, ct, body := get("/"); code != 200 || !strings.HasPrefix(ct, "text/html") || !strings.Contains(body, `<script src="ui/app.js">`) {
		t.Fatalf("GET /: %d %s %q", code, ct, body[:min(80, len(body))])
	}
	// The apps' connect page is a page of its own, with the server's
	// policy and its own for the app's copy (docs/app.md).
	if code, ct, body := get("/ui/connect.html"); code != 200 || !strings.HasPrefix(ct, "text/html") || !strings.Contains(body, `<script src="ui/connect.js">`) ||
		!strings.Contains(body, `<base href="../">`) || !strings.Contains(body, `http-equiv="Content-Security-Policy"`) {
		t.Fatalf("GET /ui/connect.html: %d %s %q", code, ct, body[:min(80, len(body))])
	}
	// Every script embedded is on the page, or one of those that are not,
	// and each is served as JavaScript.
	_, _, page := get("/")
	onPage := map[string]bool{}
	for _, m := range regexp.MustCompile(`<script src="(ui/[^"]+\.js)"`).FindAllStringSubmatch(page, -1) {
		onPage[m[1]] = true
	}
	scripts, _ := fs.Glob(uiFiles, "ui/*.js")
	if len(scripts) < 10 {
		t.Fatalf("only %d scripts embedded", len(scripts))
	}
	for _, f := range scripts {
		if why, off := uiScriptsOffPage[f]; off == onPage[f] {
			t.Errorf("%s: on the page %v, in uiScriptsOffPage %v (%s)", f, onPage[f], off, why)
		}
		if code, ct, _ := get("/" + f); code != 200 || !strings.Contains(ct, "javascript") {
			t.Fatalf("GET /%s: %d %s", f, code, ct)
		}
	}
	for f := range uiScriptsOffPage {
		if !slices.Contains(scripts, f) {
			t.Errorf("uiScriptsOffPage names %s, which is not embedded", f)
		}
	}
	// The parts' bundle (make ui) comes after the plain scripts, whose
	// globals it calls, and registers the parts the plain code calls.
	if tags := regexp.MustCompile(`<script src="(ui/[^"]+\.js)"`).FindAllStringSubmatch(page, -1); len(tags) == 0 || tags[len(tags)-1][1] != "ui/parts.js" {
		t.Errorf("the page's last script is not ui/parts.js")
	}
	if _, _, body := get("/ui/parts.js"); !strings.HasPrefix(body, "// SPDX-License-Identifier: AGPL-3.0-or-later\n") || !strings.Contains(body, "MassimoParts") {
		t.Errorf("ui/parts.js is not the bundle of make ui: %q", body[:min(120, len(body))])
	}
	if code, ct, _ := get("/ui/style.css"); code != 200 || !strings.HasPrefix(ct, "text/css") {
		t.Fatalf("GET /ui/style.css: %d %s", code, ct)
	}
	if code, ct, body := get("/ui/vendor/opaque.js"); code != 200 || !strings.Contains(ct, "javascript") || !strings.Contains(body, "export { client, ready, server }") {
		t.Fatalf("GET /ui/vendor/opaque.js: %d %s", code, ct)
	}
	if code, _, body := get("/ui/vendor/opaque.LICENSE"); code != 200 || !strings.HasPrefix(body, "MIT License") {
		t.Fatalf("GET /ui/vendor/opaque.LICENSE: %d", code)
	}
	// The service worker and the manifest beside the page, so that the
	// worker's scope and the manifest's addresses are the page's; the
	// icons the manifest names.
	if code, ct, body := get("/sw.js"); code != 200 || !strings.Contains(ct, "javascript") || !strings.Contains(body, `importScripts("ui/notifytext.js")`) {
		t.Fatalf("GET /sw.js: %d %s", code, ct)
	}
	code, ct, body := get("/manifest.webmanifest")
	var manifest struct {
		StartURL string `json:"start_url"`
		Display  string
		Icons    []struct{ Src, Sizes, Purpose string }
	}
	if code != 200 || ct != "application/manifest+json" || json.Unmarshal([]byte(body), &manifest) != nil || manifest.StartURL != "." || manifest.Display != "standalone" || len(manifest.Icons) != 4 {
		t.Fatalf("GET /manifest.webmanifest: %d %s %s", code, ct, body)
	}
	purposes := map[string]int{}
	for _, icon := range manifest.Icons {
		purposes[icon.Purpose]++
		if code, ct, body := get("/" + icon.Src); code != 200 || ct != "image/png" || !strings.HasPrefix(body, "\x89PNG") {
			t.Fatalf("GET /%s: %d %s", icon.Src, code, ct)
		}
	}
	if purposes["any"] != 2 || purposes["maskable"] != 2 {
		t.Fatalf("the manifest's icons by purpose: %v", purposes)
	}
	resp, err := http.Get(ts.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if csp := resp.Header.Get("Content-Security-Policy"); !strings.Contains(csp, "script-src 'self' 'wasm-unsafe-eval';") || strings.Contains(csp, "'unsafe-eval'") && !strings.Contains(csp, "'wasm-unsafe-eval'") ||
		!strings.Contains(csp, "worker-src 'self';") || !strings.Contains(csp, "manifest-src 'self';") {
		t.Fatalf("the page's CSP: %s", csp)
	}
	for _, lang := range uiLanguages {
		p := "/ui/i18n/" + lang + ".json"
		if code, ct, body := get(p); code != 200 || !strings.HasPrefix(ct, "application/json") || !strings.HasPrefix(body, "{") {
			t.Fatalf("GET %s: %d %s", p, code, ct)
		}
	}
	for _, p := range []string{"/ui/", "/ui/index.html", "/ui/nope.js", "/ui/i18n", "/ui/i18n/", "/ui/i18n/xx.json", "/ui/i18n/en.js", "/ui/i18n/EN.json", "/ui/partials/cubes.html", "/ui/partials/", "/ui/vendor/", "/ui/vendor/README.md", "/ui/vendor/nope.js", "/ui/icons/icon-192.png", "/ui/brand/", "/ui/brand/nope.png", "/ui/brand/.make_brand", "/other"} {
		if code, _, _ := get(p); code != 404 {
			t.Fatalf("GET %s: %d, want 404", p, code)
		}
	}
	// Every file carries an ETag and answers 304 to it; no-cache stays.
	for _, p := range []string{"/", "/ui/app.js", "/ui/i18n/de.json"} {
		resp, err := http.Get(ts.URL + p)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		etag := resp.Header.Get("ETag")
		if !strings.HasPrefix(etag, `"`) || !strings.HasSuffix(etag, `"`) || resp.Header.Get("Cache-Control") != "no-cache" {
			t.Fatalf("GET %s: ETag %q, Cache-Control %q", p, etag, resp.Header.Get("Cache-Control"))
		}
		req, _ := http.NewRequest("GET", ts.URL+p, nil)
		req.Header.Set("If-None-Match", etag)
		resp, err = http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusNotModified {
			t.Fatalf("GET %s with If-None-Match %s: %d, want 304", p, etag, resp.StatusCode)
		}
		req.Header.Set("If-None-Match", `"other"`)
		if resp, err = http.DefaultClient.Do(req); err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Fatalf("GET %s with another ETag: %d, want 200", p, resp.StatusCode)
		}
	}
	// The API behind it still needs the token in the header; only the
	// event stream takes it as a query parameter.
	if code, _, _ := get("/api/v1/cubes"); code != 401 {
		t.Fatalf("GET /api/v1/cubes without token: %d", code)
	}
	if code, _, _ := get("/api/v1/cubes?token=secret"); code != 401 {
		t.Fatalf("GET /api/v1/cubes?token=secret: %d, want 401", code)
	}
	req, _ := http.NewRequest("GET", ts.URL+"/api/v1/cubes/KEQ0000100/events?token=secret", nil)
	ctx, cancel := context.WithTimeout(context.Background(), testkit.Scale(2*time.Second))
	defer cancel()
	resp, err = http.DefaultClient.Do(req.WithContext(ctx))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 200 || !strings.HasPrefix(resp.Header.Get("Content-Type"), "text/event-stream") {
		t.Fatalf("events?token=secret: %d %s", resp.StatusCode, resp.Header.Get("Content-Type"))
	}
	req, _ = http.NewRequest("GET", ts.URL+"/api/v1/cubes/KEQ0000100/events?token=wrong", nil)
	resp, err = http.DefaultClient.Do(req.WithContext(ctx))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 401 {
		t.Fatalf("events?token=wrong: %d, want 401", resp.StatusCode)
	}
}

// uiScriptsOffPage are the scripts embedded that the page at / does not
// load, each with where it is loaded instead.
var uiScriptsOffPage = map[string]string{
	"ui/connect.js": "the apps' connect page, ui/connect.html",
	"ui/sw.js":      "the service worker, served at /sw.js",
}

// assembledPage is the page as the server serves it at /.
func assembledPage(t *testing.T) []byte {
	t.Helper()
	sub, err := fs.Sub(uiFiles, "ui")
	if err != nil {
		t.Fatal(err)
	}
	page, err := uiPage(sub)
	if err != nil {
		t.Fatal(err)
	}
	return page
}

// uiPageScripts are the pages of the UI other than the one at /, each
// with the script that alone looks up its ids; every other script looks
// up the ids of the page at /.
var uiPageScripts = map[string]string{"ui/connect.html": "ui/connect.js"}

// pageIDs are the ids of a page, each there once.
func pageIDs(t *testing.T, name, page string) map[string]bool {
	t.Helper()
	ids := map[string]bool{}
	for _, m := range regexp.MustCompile(`\sid="([^"]+)"`).FindAllStringSubmatch(page, -1) {
		if ids[m[1]] {
			t.Errorf("%s has id %q twice", name, m[1])
		}
		ids[m[1]] = true
	}
	return ids
}

// Massimo's mark (make ui-brand): every file the page's head, the connect
// page's, the manifest and the service worker name is served with its
// type; /favicon.ico at the root, where browsers look; an SVG with a
// policy that lets its own style be, the dark rule of the favicon.
func TestUIBrand(t *testing.T) {
	t.Parallel()
	ts := startServer(t).ts
	get := func(p string) (*http.Response, string) {
		t.Helper()
		resp, err := http.Get(ts.URL + p)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		return resp, string(b)
	}
	_, page := get("/")
	_, connect := get("/ui/connect.html")
	sw, err := uiFiles.ReadFile("ui/sw.js")
	if err != nil {
		t.Fatal(err)
	}
	named := regexp.MustCompile(`"(ui/brand/[^"]+|favicon\.ico)"`)
	types := map[string]string{".svg": "image/svg+xml", ".css": "text/css", ".png": "image/png", ".ico": "image/x-icon"}
	seen := map[string]bool{}
	for _, text := range []string{page, connect, string(sw)} {
		for _, m := range named.FindAllStringSubmatch(text, -1) {
			seen[m[1]] = true
		}
	}
	for _, want := range []string{"ui/brand/brand.css", "ui/brand/favicon.svg", "favicon.ico", "ui/brand/apple-touch-180.png", "ui/brand/tile-192.png"} {
		if !seen[want] {
			t.Errorf("%s is named nowhere", want)
		}
	}
	for name := range seen {
		resp, body := get("/" + name)
		ct := resp.Header.Get("Content-Type")
		if resp.StatusCode != 200 || !strings.HasPrefix(ct, types[path.Ext(name)]) || body == "" {
			t.Errorf("GET /%s: %d %s", name, resp.StatusCode, ct)
		}
		csp := resp.Header.Get("Content-Security-Policy")
		if path.Ext(name) == ".svg" && csp != "default-src 'none'; style-src 'unsafe-inline'" {
			t.Errorf("GET /%s: the policy %q", name, csp)
		}
	}
	if resp, body := get("/ui/brand/brand.svg"); resp.StatusCode != 200 || !strings.Contains(body, `<symbol id="logo-m"`) || !strings.Contains(body, `<symbol id="mark-cover"`) {
		t.Fatalf("the sprite: %d", resp.StatusCode)
	}
	if resp, body := get("/favicon.ico"); resp.StatusCode != 200 || !strings.HasPrefix(body, "\x00\x00\x01\x00") || resp.Header.Get("ETag") == "" {
		t.Fatalf("GET /favicon.ico: %d %q", resp.StatusCode, body[:min(4, len(body))])
	}
}

// The page is assembled from index.html and the partials: every file of
// ui/partials/ is listed in uiPartials exactly once and every listed one
// exists, no marker is left, the page is served at / as assembled, and
// every id the scripts look up ($("…")) and every template they clone
// (fromTemplate("…"), fragmentFromTemplate("…")) is in it, or, for the
// script of another page (uiPageScripts), in that page.
func TestUIPage(t *testing.T) {
	t.Parallel()
	files, err := fs.Glob(uiFiles, "ui/partials/*.html")
	if err != nil {
		t.Fatal(err)
	}
	listed := map[string]int{}
	for _, p := range uiPartials {
		for _, f := range p.files {
			listed[f]++
		}
	}
	for _, f := range files {
		name := strings.TrimPrefix(f, "ui/partials/")
		if listed[name] != 1 {
			t.Errorf("ui/partials/%s is listed %d times in uiPartials, want 1", name, listed[name])
		}
		delete(listed, name)
	}
	for name := range listed {
		t.Errorf("uiPartials lists %s, which is not in ui/partials/", name)
	}
	page := string(assembledPage(t))
	if strings.Contains(page, "<!-- partials:") {
		t.Errorf("the assembled page still has a marker")
	}
	if n := strings.Count(page, "<html"); n != 1 || !strings.HasPrefix(page, "<!doctype html>") || !strings.HasSuffix(page, "</html>\n") {
		t.Errorf("the assembled page is not one document (%d <html>)", n)
	}
	ids := pageIDs(t, "the page", page)
	idsOf := map[string]map[string]bool{} // by script, the page whose ids it looks up
	for name, script := range uiPageScripts {
		b, err := uiFiles.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		idsOf[script] = pageIDs(t, name, string(b))
		if !strings.Contains(string(b), `<script src="`+script+`">`) {
			t.Errorf("%s does not load %s", name, script)
		}
	}
	lookup := regexp.MustCompile(`(?:\$|fromTemplate|fragmentFromTemplate)\("([^"]+)"\)`)
	scripts, _ := fs.Glob(uiFiles, "ui/*.js")
	n := 0
	for _, f := range scripts {
		b, err := uiFiles.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		want, where := ids, "the page"
		if other, ok := idsOf[f]; ok {
			want, where = other, "its page"
		}
		for _, m := range lookup.FindAllStringSubmatch(string(b), -1) {
			n++
			if !want[m[1]] {
				t.Errorf("%s looks up %q, which is not an id of %s", f, m[1], where)
			}
		}
	}
	if n < 50 {
		t.Fatalf("only %d lookups found in the scripts", n)
	}

	ts := startServer(t).ts
	resp, err := http.Get(ts.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if string(body) != page {
		t.Errorf("GET / is not the assembled page (%d bytes, want %d)", len(body), len(page))
	}
}

// uiPage replaces each marker with its partials, indented as the marker
// is, and refuses a skeleton without a marker or with one twice.
func TestUIPageMarkers(t *testing.T) {
	t.Parallel()
	part := "<p>\n  x\n</p>\n"
	files := fstest.MapFS{"partials/" + uiPartials[0].files[0]: {Data: []byte(part)}}
	for _, p := range uiPartials {
		for _, f := range p.files {
			if _, ok := files["partials/"+f]; !ok {
				files["partials/"+f] = &fstest.MapFile{}
			}
		}
	}
	// The first marker indented, the others not; each partial but the
	// first is empty.
	skeleton := "<body>\n"
	for i, p := range uiPartials {
		if i == 0 {
			skeleton += "  "
		}
		skeleton += "<!-- partials: " + p.marker + " -->\n"
	}
	skeleton += "</body>\n"
	first := "<!-- partials: " + uiPartials[0].marker + " -->"
	last := "<!-- partials: " + uiPartials[len(uiPartials)-1].marker + " -->"
	files["index.html"] = &fstest.MapFile{Data: []byte(skeleton)}
	got, err := uiPage(files)
	if want := "<body>\n  <p>\n    x\n  </p>\n</body>\n"; err != nil || string(got) != want {
		t.Fatalf("uiPage = %q, %v, want %q", got, err, want)
	}
	for _, bad := range []string{strings.Replace(skeleton, first+"\n", "", 1), skeleton + first + "\n", strings.Replace(skeleton, first+"\n", "<p>"+first+last+"\n", 1)} {
		files["index.html"] = &fstest.MapFile{Data: []byte(bad)}
		if _, err := uiPage(files); err == nil {
			t.Errorf("uiPage(%q): no error", bad)
		}
	}
}

// TestUIRelativeURLs keeps every URL of the UI relative to the page, so
// that a reverse proxy may serve it under a path prefix it strips
// (docs/reverse-proxy.md): no file names /ui/, /api/ or /healthz with a
// leading slash, and no href or src starts at the root.
func TestUIRelativeURLs(t *testing.T) {
	t.Parallel()
	absolute := regexp.MustCompile(`"/(ui/|api/|healthz)|(href|src)="/`)
	err := fs.WalkDir(uiFiles, "ui", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, err := uiFiles.ReadFile(path)
		if err != nil {
			return err
		}
		for i, line := range strings.Split(string(b), "\n") {
			if absolute.MatchString(line) {
				t.Errorf("%s:%d: an absolute URL: %s", path, i+1, strings.TrimSpace(line))
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// The vendored OPAQUE client is @serenity-kit/opaque 1.1.0's
// esm/index.js as published (ui/vendor/README.md): a change to it is
// a change on purpose, with the README.
func TestVendoredOpaque(t *testing.T) {
	t.Parallel()
	b, err := uiFiles.ReadFile("ui/vendor/opaque.js")
	if err != nil {
		t.Fatal(err)
	}
	const want = "1e8d56383a77948e453085b6d4f52dcb9645f8bfff32395738b63d3a2e2fcb52"
	if got := fmt.Sprintf("%x", sha256.Sum256(b)); got != want {
		t.Errorf("ui/vendor/opaque.js: SHA-256 %s, want %s", got, want)
	}
	readme, _ := os.ReadFile("ui/vendor/README.md")
	if !strings.Contains(string(readme), want) {
		t.Error("ui/vendor/README.md does not name the file's SHA-256")
	}
}

// vendoredPreact are the files of third_party/preact/ by path, each
// with its SHA-256: Preact 11.2.0, @preact/signals 2.11.4 and
// @preact/signals-core 1.14.5 as published (third_party/preact/README.md).
var vendoredPreact = map[string]string{
	"preact/LICENSE":                         "1fe6958409c8c257a70c587a18b6f7f412b179b456630790d30b2ec9a8e4b7d4",
	"preact/dist/preact.mjs":                 "bdcbd9abdf2c77c5c45326eab42191e949222d479d0d5f69467387a77040cb55",
	"preact/hooks/dist/hooks.mjs":            "e862ab2973aa6455eaa0f77c5d832d2157d6be59a17c8ba546550a575b58c6b4",
	"preact/hooks/src/index.d.ts":            "21898d939444825cee1232af435330610b09171bea22a2f892cf2058d764b63d",
	"preact/jsx-runtime/dist/jsxRuntime.mjs": "4761ad0e33728d1488ad98a194fe8c6a93cf940fec80e49995c82dc013d081cd",
	"preact/jsx-runtime/src/index.d.ts":      "a563e6d3e9a0f337066d8407d48304943b961ef6b3f7fbb38651455a6af46afc",
	"preact/src/dom.d.ts":                    "8199eb2fc00bb9ce68266df5e97ba914e03a56d4dd823629480c7e68d20b5129",
	"preact/src/index.d.ts":                  "a5437961b23f82d3959e1cc4630c29dfe405068caee25321088b45d3868fef79",
	"preact/src/jsx.d.ts":                    "5d082f468e25c12528dc4486ab6f3f48ba03c938d1c472ab6b76c8eb0aa42818",
	"signals/LICENSE":                        "a11fc89e4c6b118854c7a667734a0b2e6bf2af5e45c6686de31adbccc8f3ae8d",
	"signals/dist/signals.d.ts":              "6761e07a03f7704624ff7af23678a1bcdbd32edd593de8b3f4d1e24e983ab1ef",
	"signals/dist/signals.mjs":               "ba8177597f45acaca4d05dcf3966b527d0a8264cfa2b0a87ace92dff324217f8",
	"signals-core/LICENSE":                   "a11fc89e4c6b118854c7a667734a0b2e6bf2af5e45c6686de31adbccc8f3ae8d",
	"signals-core/dist/signals-core.d.ts":    "7fd86ad5dfe671ca797ac75d99a657f2ae66022140428717f6c17373282e3050",
	"signals-core/dist/signals-core.mjs":     "0c80bc127d72ecb1496ef116ffddae75079829a3af01e7f417c8e5a6474c68b6",
}

// The vendored Preact is the packages' files as published: every file
// of third_party/preact/ but its README is in vendoredPreact with its
// SHA-256, and the README names each, so that a change to one is a
// change on purpose, with the README.
func TestVendoredPreact(t *testing.T) {
	t.Parallel()
	const dir = "../../third_party/preact"
	readme, err := os.ReadFile(dir + "/README.md")
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	err = fs.WalkDir(os.DirFS(dir), ".", func(name string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || name == "README.md" {
			return err
		}
		seen[name] = true
		want, ok := vendoredPreact[name]
		if !ok {
			t.Errorf("third_party/preact/%s is not in vendoredPreact", name)
			return nil
		}
		b, err := os.ReadFile(dir + "/" + name)
		if err != nil {
			return err
		}
		if got := fmt.Sprintf("%x", sha256.Sum256(b)); got != want {
			t.Errorf("third_party/preact/%s: SHA-256 %s, want %s", name, got, want)
		}
		if !strings.Contains(string(readme), want+"  "+name+"\n") {
			t.Errorf("third_party/preact/README.md does not name the SHA-256 of %s", name)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range slices.Sorted(maps.Keys(vendoredPreact)) {
		if !seen[name] {
			t.Errorf("vendoredPreact names %s, which is not in third_party/preact/", name)
		}
	}
}

func TestUIETagVersion(t *testing.T) {
	t.Parallel()
	started := time.Unix(0, 36*36)
	for v, want := range map[string]string{
		"massimo v0.2.0-alpha.1 (go1.26.8, linux/amd64)":        "v0.2.0-alpha.1",
		"massimo v0.2.0-3-g1a2b3c4 (go1.26.8, linux/amd64)":     "v0.2.0-3-g1a2b3c4",
		"massimo v0.2.0-3-g1a2b3c4-dirty (go1.26.8, linux/arm)": "v0.2.0-3-g1a2b3c4-dirty+100",
		"test":  "test",
		"":      "dev+100",
		"v1\"x": "v1_x",
	} {
		if got := uiETagVersion(v, started); got != want {
			t.Errorf("uiETagVersion(%q) = %q, want %q", v, got, want)
		}
	}
}

// uiLanguages are the languages of LANGUAGE_NAMES in i18n.js, each with
// its file ui/i18n/<code>.json.
var uiLanguages = []string{"en", "de", "fr", "it", "nl", "pl"}

// The scripts a page loads share one global scope, so a name declared at
// the top of two of them clashes: a second function declaration silently
// replaces the first (integrations.js's renderFound hid app.js's found
// Cubes), and a second const, let or class stops the later script with a
// SyntaxError. Each page, the one at / and those of uiPageScripts, is
// checked on its own, its scripts read from its <script src> tags; a name
// two pages' scripts share is fine. A top-level declaration is one at the
// start of a line, the scripts' indentation putting everything nested
// further in.
func TestUIScriptGlobals(t *testing.T) {
	t.Parallel()
	pages := map[string][]byte{"the page": assembledPage(t)}
	for name := range uiPageScripts {
		b, err := uiFiles.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		pages[name] = b
	}
	src := regexp.MustCompile(`<script src="(ui/[^"]+\.js)"`)
	decl := regexp.MustCompile(`(?m)^(?:(?:async\s+)?function\s*\*?\s*([\w$]+)\s*\(|(?:const|let)\s+([\w$]+)\s*=|class\s+([\w$]+)\b)`)
	for _, page := range slices.Sorted(maps.Keys(pages)) {
		scripts := src.FindAllSubmatch(pages[page], -1)
		if len(scripts) == 0 {
			t.Errorf("%s loads no script", page)
		}
		where := map[string][]string{} // identifier: the scripts declaring it
		for _, s := range scripts {
			name := string(s[1])
			b, err := uiFiles.ReadFile(name)
			if err != nil {
				t.Errorf("%s loads %s: %v", page, name, err)
				continue
			}
			for _, m := range decl.FindAllSubmatch(b, -1) {
				id := string(slices.Concat(m[1], m[2], m[3]))
				where[id] = append(where[id], name)
			}
		}
		for _, id := range slices.Sorted(maps.Keys(where)) {
			if files := where[id]; len(slices.Compact(slices.Clone(files))) > 1 {
				t.Errorf("%s: %s is declared in %s", page, id, strings.Join(files, ", "))
			}
		}
	}
}

// The parts' shared files know no region (docs/ui.md, "How the page is
// built"): each imports from the shared directory itself, from Preact
// and its signals, and from the API's types alone, so that every region
// can use it. They are read from the disk: the bundle is made from them,
// and they are not embedded.
func TestSharedPartsKnowNoRegion(t *testing.T) {
	t.Parallel()
	partsImportOnly(t, "ui/src/shared")
}

// What the parts read of the API's data beside its fields (ui/src/model/)
// knows no region either: it imports from itself, from the shared parts,
// from Preact and from the API's types alone, so that two regions that
// both need it never import from each other.
func TestModelKnowsNoRegion(t *testing.T) {
	t.Parallel()
	partsImportOnly(t, "ui/src/model", "shared")
}

// No region of the parts imports from another (docs/ui.md, "How the
// page is built"): every directory of ui/src/ but shared, model and plain
// is a region, and imports from itself, from the shared parts, from the
// model, from Preact and from the API's types alone.
func TestRegionsKnowNoOtherRegion(t *testing.T) {
	t.Parallel()
	dirs, err := os.ReadDir("ui/src")
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, d := range dirs {
		if !d.IsDir() || d.Name() == "shared" || d.Name() == "model" || d.Name() == "plain" {
			continue
		}
		n++
		partsImportOnly(t, "ui/src/"+d.Name(), "shared", "model")
	}
	if n == 0 {
		t.Fatal("ui/src holds no region")
	}
}

// partsImportOnly holds the TypeScript files under dir, a directory of
// ui/src/, to their imports: from dir itself, from the directories of
// ui/src/ named in also, from Preact and its signals, and from the API's
// types.
func partsImportOnly(t *testing.T, dir string, also ...string) {
	t.Helper()
	spec := partsImportSpec
	files := 0
	err := fs.WalkDir(os.DirFS(dir), ".", func(name string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !(strings.HasSuffix(name, ".ts") || strings.HasSuffix(name, ".tsx")) {
			return err
		}
		files++
		b, err := os.ReadFile(path.Join(dir, name))
		if err != nil {
			return err
		}
		for n, line := range strings.Split(string(b), "\n") {
			for _, m := range spec.FindAllStringSubmatch(line, -1) {
				from := m[1]
				allowed := from == "preact" || strings.HasPrefix(from, "preact/") || from == "@preact/signals"
				if strings.HasPrefix(from, ".") {
					// A relative import stays inside the directory, but for
					// the API's types and the directories also names.
					target := path.Join(path.Dir(name), from)
					allowed = target == "../api" || !strings.HasPrefix(target, "..")
					for _, a := range also {
						allowed = allowed || strings.HasPrefix(target, "../"+a+"/")
					}
				}
				if !allowed {
					t.Errorf("%s/%s:%d imports %q: a file of %s imports from it, %sfrom Preact and from ../api alone", dir, name, n+1, from, dir, alsoText(also))
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if files == 0 {
		t.Fatalf("%s holds no part", dir)
	}
}

// partsImportSpec finds what a line of a part imports from: from and
// import are words of their own, not the end of a text's key
// ("vwt.from", "appearance.import"), of a text ("import") or of an id
// ("scheme-import"); a dynamic import after an operator counts
// (TestPartsImportSpec).
var partsImportSpec = regexp.MustCompile(`(?:(?:^|[^.\w])from\s*|(?:^|[^.\w"'\x60-])import\s*\(?\s*)["']([^"']+)["']`)

// partsImportSpec takes every way a part imports and leaves the words
// import and from where they are a text's key, a text or an id.
func TestPartsImportSpec(t *testing.T) {
	t.Parallel()
	for _, c := range []struct{ line, want string }{
		{`import { h } from "preact";`, "preact"},
		{`import "./chrome";`, "./chrome"},
		{`const m = await import("./x");`, "./x"},
		{`const m = import("./x");`, "./x"},
		{`const m=import("./x")`, "./x"},
		{`a?import("./x"):0`, "./x"},
		{`f(a,import("./x"))`, "./x"},
		{`{ m:import("./x") }`, "./x"},
		{`a ? import("./x") : null`, "./x"},
		{`!import("./x")`, "./x"},
		{`export { a } from "../shared/b";`, "../shared/b"},
		{`import type { User } from "../api";`, "../api"},
	} {
		if m := partsImportSpec.FindStringSubmatch(c.line); m == nil || m[1] != c.want {
			t.Errorf("%s: want %q, got %v", c.line, c.want, m)
		}
	}
	for _, line := range []string{
		`t("appearance.import")`, `id="scheme-import"`, `t("import")`, `t('import')`, "t(`import`)",
		`t("vwt.from")`, `label={t("appearance.import")} accept="application/json"`,
	} {
		if m := partsImportSpec.FindStringSubmatch(line); m != nil {
			t.Errorf("%s: no import, got %q", line, m[1])
		}
	}
}

// alsoText names the directories partsImportOnly allows beside its own.
func alsoText(also []string) string {
	s := ""
	for _, a := range also {
		s += "from ui/src/" + a + ", "
	}
	return s
}

// wholeFields are the fields of the page's data that are signals
// underneath (ui/src/shared/data.ts, AP UiParts D123-A): a write tells
// the parts only as an assignment of the field, so none of them is
// written in place.
var wholeFields = []string{"state.cubes", "state.found", "state.tz", "state.rooms", "state.unlisted", "state.roomsOf", "state.device", "state.me",
	"themeState.custom", "themeState.draft", "themeState.base"}

// roomDevicesInPlace is a write in place to a room's devices, by any
// name of the room: a room's fields are signals, its devices written whole.
var roomDevicesInPlace = regexp.MustCompile(`\.devices\[[^\]]*\](?:\.[\w$]+)*\s*=[^=]|\.devices\.length\s*=[^=]|\.devices\.(?:push|splice|sort|reverse|unshift|shift|pop|fill|copyWithin)\(`)

// writtenInPlace finds a write in place to a field of wholeFields on a
// line: an item assigned by its index, an array's method that changes
// it, delete, Object.assign into it, or a property of it assigned; and a
// write in place to a room's devices.
func writtenInPlace(line string) bool {
	if roomDevicesInPlace.MatchString(line) {
		return true
	}
	for _, f := range wholeFields {
		q := regexp.QuoteMeta(f)
		re := regexp.MustCompile(`(?:^|[^\w.$])` + q + `(?:\[[^\]]*\]|\.[\w$]+)+\s*=[^=]|(?:^|[^\w.$])` + q +
			`\.(?:push|splice|sort|reverse|unshift|shift|pop|fill|copyWithin)\(|delete\s+` + q + `\b|Object\.assign\(\s*` + q + `\b`)
		if re.MatchString(line) {
			return true
		}
	}
	return false
}

// The page's data are written whole (UiParts D234): no plain script and
// no part writes a field of wholeFields in place, which would change the
// data without telling the parts that draw from them. It reads the
// written name alone: a write through another name of the same array
// (const list = themeState.custom; list.push(…)), nor an item's
// property written through find or for … of (for (const r of
// state.rooms) r.devices[0].x = …), it cannot see; the drawings counted
// per poll at each step of Block 15 and the review cover that.
func TestStateWrittenWhole(t *testing.T) {
	t.Parallel()
	files, err := filepath.Glob("ui/*.js")
	if err != nil {
		t.Fatal(err)
	}
	err = fs.WalkDir(os.DirFS("ui/src"), ".", func(name string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && (strings.HasSuffix(name, ".ts") || strings.HasSuffix(name, ".tsx")) {
			files = append(files, path.Join("ui/src", name))
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, f := range files {
		if f == "ui/parts.js" {
			continue // the bundle
		}
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		n++
		for i, line := range strings.Split(string(b), "\n") {
			if code, _, _ := strings.Cut(line, "//"); writtenInPlace(code) {
				t.Errorf("%s:%d writes the page's data in place, which tells no part: %s", f, i+1, strings.TrimSpace(line))
			}
		}
	}
	if n < 40 {
		t.Fatalf("only %d files read", n)
	}
}

// writtenInPlace tells a write in place from a write of the whole field
// and from a read.
func TestWrittenInPlace(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		line string
		want bool
	}{
		{"themeState.custom.push(entry);", true},
		{"themeState.custom[i] = entry;", true},
		{"themeState.draft.name = name;", true},
		{"themeState.draft.light[k] = c;", true},
		{"delete themeState.draft;", true},
		{"Object.assign(themeState.draft, x);", true},
		{"themeState.custom = [...themeState.custom, entry];", false},
		{"themeState.draft = { ...d, name };", false},
		{"if (themeState.custom[i] === entry) return;", false},
		{"const d = themeState.draft;", false},
		{"themeState.customs.push(x);", false},
		{"r.devices[i] = d;", true},
		{"room.devices.push(twin);", true},
		{"r.devices[i].name = n;", true},
		{"r.devices.length = 0;", true},
		{"state.me.role = r;", true},
		{"r.devices = r.devices.map((x, k) => (k === i ? d : x));", false},
	} {
		if got := writtenInPlace(c.line); got != c.want {
			t.Errorf("writtenInPlace(%q) = %v, want %v", c.line, got, c.want)
		}
	}
}

// Every file of the web UI opens with its licence lines (the contract,
// "Go code"; docs/ui.md, "How the page is built"): a plain script or
// style sheet with the SPDX line and the copyright line, in either
// comment form; a file of the parts, under ui/src/, with both as line
// comments and then the @file header of GUIDELINES/shared/lang/ts/
// LINTING.md, which names the file's own path from the repository root
// and says in a sentence what the file is for.
func TestUIFileHeaders(t *testing.T) {
	t.Parallel()
	spdx := regexp.MustCompile(`^(?://|/\*) SPDX-License-Identifier: AGPL-3\.0-or-later$`)
	copyright := regexp.MustCompile(`^(?://|\s*) ?Copyright \(C\) \d{4}(?:-\d{4})? The Massimo Authors(?: \*/)?$`)
	plain := 0
	for _, glob := range []string{"ui/*.js", "ui/*.css"} {
		names, err := filepath.Glob(glob)
		if err != nil {
			t.Fatal(err)
		}
		for _, name := range names {
			if name == "ui/parts.js" {
				continue // the bundle make ui makes, not a file a person writes
			}
			plain++
			lines := fileLines(t, name)
			if len(lines) < 2 || !spdx.MatchString(lines[0]) || !copyright.MatchString(lines[1]) {
				t.Errorf("%s: the first two lines are not the SPDX line and the copyright line", name)
			}
		}
	}
	if plain == 0 {
		t.Fatal("ui holds no script and no style sheet")
	}
	parts := 0
	err := fs.WalkDir(os.DirFS("ui/src"), ".", func(name string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !(strings.HasSuffix(name, ".ts") || strings.HasSuffix(name, ".tsx")) {
			return err
		}
		parts++
		file := path.Join("ui/src", name)
		lines := fileLines(t, file)
		if msg := partHeader(lines, "internal/server/"+file); msg != "" {
			t.Errorf("%s: %s", file, msg)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if parts == 0 {
		t.Fatal("ui/src holds no part")
	}
}

// partHeader says what is wrong with the opening of a file of the parts
// whose path from the repository root is want, or "" when nothing is.
func partHeader(lines []string, want string) string {
	if len(lines) < 2 || lines[0] != "// SPDX-License-Identifier: AGPL-3.0-or-later" {
		return "line 1 is not the SPDX line, // SPDX-License-Identifier: AGPL-3.0-or-later"
	}
	if !regexp.MustCompile(`^// Copyright \(C\) \d{4}(?:-\d{4})? The Massimo Authors$`).MatchString(lines[1]) {
		return "line 2 is not the copyright line, // Copyright (C) <year> The Massimo Authors"
	}
	i := 2
	for i < len(lines) && strings.TrimSpace(lines[i]) == "" {
		i++
	}
	if i+1 >= len(lines) || lines[i] != "/**" || lines[i+1] != " * @file "+want {
		return fmt.Sprintf("line %d does not open the @file header, /** and then * @file %s", i+1, want)
	}
	for j := i + 2; j < len(lines); j++ {
		text := strings.TrimSpace(lines[j])
		if strings.HasSuffix(text, "*/") {
			return fmt.Sprintf("the @file header, lines %d to %d, says nothing of what the file is for", i+1, j+1)
		}
		if strings.TrimSpace(strings.TrimPrefix(text, "*")) != "" {
			return ""
		}
	}
	return fmt.Sprintf("the @file header from line %d is never closed", i+1)
}

// No file of the web UI puts markup into the page from a string, and
// none sets an inline handler (the contract, "The web UI", both kinds of
// region): API text goes in only as text, and the security policy allows
// no inline script. A part carries no data-i18n either, since applyI18n
// would write over the text the part owns (rule 3 of AP UiParts). Comments
// are left out, so that prose may name what the code must not do.
func TestUINoHTMLInjection(t *testing.T) {
	t.Parallel()
	banned := []struct {
		re   *regexp.Regexp
		what string
	}{
		{regexp.MustCompile(`\.(?:inner|outer)HTML\b`), "innerHTML or outerHTML"},
		{regexp.MustCompile(`\.insertAdjacentHTML\s*\(`), "insertAdjacentHTML"},
		{regexp.MustCompile(`\bdocument\.write(?:ln)?\s*\(`), "document.write"},
		{regexp.MustCompile(`\bdangerouslySetInnerHTML\b`), "dangerouslySetInnerHTML"},
		{regexp.MustCompile("\\.setAttribute\\s*\\(\\s*[\"'`]on"), "an inline handler set by setAttribute"},
	}
	i18n := regexp.MustCompile("\\bdata-i18n(?:-attr)?\\s*=|\\.dataset\\.i18n|setAttribute\\s*\\(\\s*[\"'`]data-i18n")
	handler := regexp.MustCompile(`<[a-zA-Z][^>]*\son[a-z]+\s*=`)
	check := func(file string, code []string, part bool) {
		for n, line := range code {
			for _, b := range banned {
				if b.re.MatchString(line) {
					t.Errorf("%s:%d: %s puts markup or script into the page from a string", file, n+1, b.what)
				}
			}
			if part && i18n.MatchString(line) {
				t.Errorf("%s:%d: a part carries no data-i18n; it calls t()", file, n+1)
			}
		}
	}
	scripts, err := filepath.Glob("ui/*.js")
	if err != nil {
		t.Fatal(err)
	}
	if len(scripts) == 0 {
		t.Fatal("ui holds no script")
	}
	for _, name := range scripts {
		if name == "ui/parts.js" {
			continue // the bundle make ui makes, Preact's own code in it
		}
		check(name, jsCode(fileLines(t, name)), false)
	}
	parts := 0
	err = fs.WalkDir(os.DirFS("ui/src"), ".", func(name string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !(strings.HasSuffix(name, ".ts") || strings.HasSuffix(name, ".tsx")) {
			return err
		}
		parts++
		file := path.Join("ui/src", name)
		check(file, jsCode(fileLines(t, file)), true)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if parts == 0 {
		t.Fatal("ui/src holds no part")
	}
	pages, err := filepath.Glob("ui/*.html")
	if err != nil {
		t.Fatal(err)
	}
	partials, err := filepath.Glob("ui/partials/*.html")
	if err != nil {
		t.Fatal(err)
	}
	if len(pages) == 0 || len(partials) == 0 {
		t.Fatal("ui holds no page or no partial")
	}
	for _, name := range append(pages, partials...) {
		for n, line := range fileLines(t, name) {
			if handler.MatchString(line) {
				t.Errorf("%s:%d: an inline handler; a region's script adds its listener", name, n+1)
			}
		}
	}
}

// fileLines reads a file from the disk as its lines.
func fileLines(t *testing.T, name string) []string {
	t.Helper()
	b, err := os.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	return strings.Split(string(b), "\n")
}

// jsCode is a script's lines with its comments blanked and its strings
// kept, line for line, so that a match names the right line. It knows
// strings in its three quotes and both kinds of comment; a regular
// expression holding a quote may make it take code for a string, which
// keeps that code.
func jsCode(lines []string) []string {
	out := make([]string, len(lines))
	var quote byte // the quote of the string the scan is in, or 0
	block := false // in a block comment
	for n, line := range lines {
		var b strings.Builder
		for i := 0; i < len(line); i++ {
			c := line[i]
			switch {
			case block:
				if c == '*' && i+1 < len(line) && line[i+1] == '/' {
					block = false
					i++
				}
			case quote != 0:
				b.WriteByte(c)
				if c == '\\' && i+1 < len(line) {
					b.WriteByte(line[i+1])
					i++
				} else if c == quote {
					quote = 0
				}
			case c == '/' && i+1 < len(line) && line[i+1] == '/':
				i = len(line)
			case c == '/' && i+1 < len(line) && line[i+1] == '*':
				block = true
				i++
			default:
				if c == '"' || c == '\'' || c == '`' {
					quote = c
				}
				b.WriteByte(c)
			}
		}
		if quote != '`' {
			quote = 0 // only a template literal spans lines
		}
		out[n] = b.String()
	}
	return out
}

// The translation files hold the texts of the page: each parses, the
// other languages have no key English lacks (but their own plural forms
// and the machine-translation note), and English has every key the
// markup, the scripts and the parts ask for.
func TestUITranslations(t *testing.T) {
	t.Parallel()
	js, err := uiFiles.ReadFile("ui/i18n.js")
	if err != nil {
		t.Fatal(err)
	}
	m := regexp.MustCompile(`const LANGUAGE_NAMES = \{([^}]*)\}`).FindSubmatch(js)
	if m == nil {
		t.Fatal("i18n.js: no LANGUAGE_NAMES")
	}
	var named []string
	for _, c := range regexp.MustCompile(`(\w+):`).FindAllSubmatch(m[1], -1) {
		named = append(named, string(c[1]))
	}
	if strings.Join(named, " ") != strings.Join(uiLanguages, " ") {
		t.Fatalf("i18n.js LANGUAGE_NAMES %v, want %v", named, uiLanguages)
	}
	files, _ := fs.Glob(uiFiles, "ui/i18n/*.json")
	if len(files) != len(uiLanguages) {
		t.Errorf("ui/i18n: %d files %v, want one per language %v", len(files), files, uiLanguages)
	}
	tables := map[string]map[string]string{}
	for _, lang := range uiLanguages {
		b, err := uiFiles.ReadFile("ui/i18n/" + lang + ".json")
		if err != nil {
			t.Fatal(err)
		}
		var table map[string]string
		if err := json.Unmarshal(b, &table); err != nil {
			t.Fatalf("ui/i18n/%s.json: %v", lang, err)
		}
		tables[lang] = table
	}
	en := tables["en"]
	plural := regexp.MustCompile(`^(.+)\.(zero|one|two|few|many|other)$`)
	for _, lang := range uiLanguages[1:] {
		for _, k := range slices.Sorted(maps.Keys(tables[lang])) {
			if _, ok := en[k]; ok || k == "i18n.machineNote" {
				continue
			}
			if p := plural.FindStringSubmatch(k); p != nil {
				if _, ok := en[p[1]+".other"]; ok {
					continue
				}
			}
			t.Errorf("ui/i18n/%s.json: %q is not in en.json", lang, k)
		}
	}
	if _, ok := en["i18n.machineNote"]; ok {
		t.Errorf("ui/i18n/en.json: has i18n.machineNote, which marks a machine-translated language")
	}

	// The keys the markup asks for: data-i18n, data-i18n-attr pairs, and
	// the help slots (data-help is the key of help.<key>).
	used := map[string]string{} // key: where
	pages := map[string][]byte{"the page": assembledPage(t)}
	for name := range uiPageScripts {
		b, err := uiFiles.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		pages[name] = b
	}
	for where, html := range pages {
		for _, m := range regexp.MustCompile(`data-(i18n|i18n-attr|help|help-label)="([^"]*)"`).FindAllStringSubmatch(string(html), -1) {
			switch m[1] {
			case "i18n", "help-label":
				used[m[2]] = where
			case "help":
				used["help."+m[2]] = where
			case "i18n-attr":
				for _, pair := range strings.Fields(m[2]) {
					if i := strings.Index(pair, ":"); i > 0 {
						used[pair[i+1:]] = where
					}
				}
			}
		}
	}
	// The string literals of t("…") and tn("…") in the scripts, a key
	// that is not put together ("help." + key); tn's have their plural
	// forms, .one and .other at least.
	call := regexp.MustCompile(`\b(tn?)\("([^"]+)"\s*[,)]`)
	scripts, _ := fs.Glob(uiFiles, "ui/*.js")
	for _, f := range scripts {
		b, err := uiFiles.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		name := strings.TrimPrefix(f, "ui/")
		for _, m := range call.FindAllStringSubmatch(string(b), -1) {
			if m[1] == "tn" {
				used[m[2]+".one"] = name
				used[m[2]+".other"] = name
			} else {
				used[m[2]] = name
			}
		}
	}
	// The parts ask as the scripts do, with t("…"), tn("…") and
	// tParts("…"). Their files are read from the disk: the bundle is made
	// from them, and they are not embedded.
	partCall := regexp.MustCompile(`\b(tn?|tParts)\(\s*"([^"]+)"\s*[,)]`)
	asked := 0
	err = fs.WalkDir(os.DirFS("ui/src"), ".", func(name string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || strings.HasSuffix(name, ".d.ts") || !(strings.HasSuffix(name, ".ts") || strings.HasSuffix(name, ".tsx")) {
			return err
		}
		b, err := os.ReadFile(path.Join("ui/src", name))
		if err != nil {
			return err
		}
		for _, m := range partCall.FindAllStringSubmatch(string(b), -1) {
			asked++
			if m[1] == "tn" {
				used[m[2]+".one"] = "src/" + name
				used[m[2]+".other"] = "src/" + name
			} else {
				used[m[2]] = "src/" + name
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if asked < 20 {
		t.Fatalf("only %d texts found in the parts under ui/src", asked)
	}
	if len(used) < 100 {
		t.Fatalf("only %d keys found in the markup, the scripts and the parts", len(used))
	}
	for _, k := range slices.Sorted(maps.Keys(used)) {
		if _, ok := en[k]; !ok {
			t.Errorf("%s asks for %q, which is not in ui/i18n/en.json", used[k], k)
		}
	}
}

// cssTokens reads the colour tokens of one block of style.css, the one
// that starts with the given selector, by name without "--".
func cssTokens(t *testing.T, css, selector string) map[string]string {
	t.Helper()
	i := strings.Index(css, selector+" {")
	if i < 0 {
		t.Fatalf("style.css: no block %q", selector)
	}
	block := css[i:]
	block = block[:strings.Index(block, "}")]
	tokens := map[string]string{}
	for _, m := range regexp.MustCompile(`--([a-z-]+):\s*(#[0-9a-f]{6})`).FindAllStringSubmatch(block, -1) {
		tokens[m[1]] = m[2]
	}
	return tokens
}

// builtinSchemes reads the built-in schemes of theme.js: by id, then
// "light" and "dark", the tokens by name.
func builtinSchemes(js string) map[string]map[string]map[string]string {
	schemes := map[string]map[string]map[string]string{}
	idLine := regexp.MustCompile(`^  ([a-z]+): \{$`)
	modeLine := regexp.MustCompile(`^    (light|dark): \{$`)
	pair := regexp.MustCompile(`"([a-z-]+)": "(#[0-9a-f]{6})"`)
	var id, mode string
	for _, line := range strings.Split(js, "\n") {
		if m := idLine.FindStringSubmatch(line); m != nil {
			id = m[1]
			schemes[id] = map[string]map[string]string{}
			continue
		}
		if m := modeLine.FindStringSubmatch(line); m != nil && id != "" {
			mode = m[1]
			schemes[id][mode] = map[string]string{}
			continue
		}
		if line == "    }," {
			mode = ""
		}
		if line == "  }," {
			id = ""
		}
		if id == "" || mode == "" {
			continue
		}
		for _, m := range pair.FindAllStringSubmatch(line, -1) {
			schemes[id][mode][m[1]] = m[2]
		}
	}
	return schemes
}

// contrastRatio is the WCAG 2 contrast ratio of two #rrggbb colours.
func contrastRatio(a, b string) float64 {
	lum := func(hex string) float64 {
		var l float64
		for i, w := range []float64{0.2126, 0.7152, 0.0722} {
			v, _ := strconv.ParseUint(hex[1+2*i:3+2*i], 16, 8)
			c := float64(v) / 255
			if c <= 0.03928 {
				c /= 12.92
			} else {
				c = math.Pow((c+0.055)/1.055, 2.4)
			}
			l += w * c
		}
		return l
	}
	la, lb := lum(a), lum(b)
	return (math.Max(la, lb) + 0.05) / (math.Min(la, lb) + 0.05)
}

// The default scheme in theme.js repeats the token blocks of style.css,
// which stay the truth for a page without a stored scheme; the two must
// not drift apart. Every built-in scheme has every token in both sets,
// and the pairs the editor grades, the muted text on the page and the
// tile, and the ok, warn and bad texts on a tile read at WCAG AA (4.5);
// the icons' amber (warn-mark), green and red stand out from a tile at
// 3, WCAG's contrast for a graphic.
func TestUIDefaultSchemeMatchesCSS(t *testing.T) {
	t.Parallel()
	css, err := uiFiles.ReadFile("ui/style.css")
	if err != nil {
		t.Fatal(err)
	}
	js, err := uiFiles.ReadFile("ui/theme.js")
	if err != nil {
		t.Fatal(err)
	}
	light := cssTokens(t, string(css), ":root")
	dark := cssTokens(t, string(css), `:root[data-theme="dark"]`)
	system := cssTokens(t, string(css), `:root:not([data-theme="light"])`)
	if len(light) != 20 || len(dark) != 20 || len(system) != 20 {
		t.Fatalf("style.css: %d light, %d dark, %d system tokens, want 20 each", len(light), len(dark), len(system))
	}
	schemes := builtinSchemes(string(js))
	if len(schemes) < 3 {
		t.Fatalf("theme.js: %d built-in schemes read, want at least 3", len(schemes))
	}
	for name, want := range map[string]map[string]string{"light": light, "dark": dark} {
		got := schemes["massimo"][name]
		for k, v := range want {
			if got[k] != v {
				t.Errorf("theme.js massimo.%s.%s = %q, style.css has %q", name, k, got[k], v)
			}
		}
	}
	for k, v := range dark {
		if system[k] != v {
			t.Errorf("style.css: dark %s = %q but the prefers-color-scheme block has %q", k, v, system[k])
		}
	}
	pairs := [][2]string{
		{"fg", "bg"}, {"accent-fg", "accent"}, {"fill-fg", "fill-ok"}, {"fill-fg", "fill-warn"}, {"fill-fg", "fill-bad"},
		{"fill-fg", "fill-info"}, {"notice-fg", "notice-bg"}, {"muted", "bg"}, {"muted", "card"},
		{"ok", "card"}, {"warn", "card"}, {"bad", "card"},
	}
	for id, s := range schemes {
		for mode, set := range s {
			for k := range light {
				if _, ok := set[k]; !ok {
					t.Errorf("theme.js %s.%s: no %s", id, mode, k)
				}
			}
			for _, p := range pairs {
				if r := contrastRatio(set[p[0]], set[p[1]]); r < 4.5 {
					t.Errorf("theme.js %s.%s: %s on %s has contrast %.1f, want 4.5", id, mode, p[0], p[1], r)
				}
			}
			for _, icon := range []string{"warn-mark", "ok", "bad"} {
				if r := contrastRatio(set[icon], set["card"]); r < 3 {
					t.Errorf("theme.js %s.%s: the icons' %s on card has contrast %.1f, want 3", id, mode, icon, r)
				}
			}
		}
	}
}

// TestLogKinds keeps the page's kinds of event (LOG_KINDS of the log's
// part, ui/src/log/kinds.ts), the server's (eventKinds) and the Cube
// driver's (cubelan.EventKind) one list, each kind with its name in the
// texts (log.kind.<kind> of en.json). The part's file is read from the
// disk: the bundle is made from it, and it is not embedded.
func TestLogKinds(t *testing.T) {
	t.Parallel()
	ts, err := os.ReadFile("ui/src/log/kinds.ts")
	if err != nil {
		t.Fatal(err)
	}
	m := regexp.MustCompile(`(?s)export const LOG_KINDS = \[(.*?)\];`).FindSubmatch(ts)
	if m == nil {
		t.Fatal("ui/src/log/kinds.ts: no LOG_KINDS")
	}
	var page []string
	for _, k := range regexp.MustCompile(`"([a-z_]+)"`).FindAllSubmatch(m[1], -1) {
		page = append(page, string(k[1]))
	}
	if strings.Join(page, " ") != strings.Join(eventKinds, " ") {
		t.Fatalf("ui/src/log/kinds.ts LOG_KINDS %v, the server's eventKinds %v", page, eventKinds)
	}
	b, err := uiFiles.ReadFile("ui/i18n/en.json")
	if err != nil {
		t.Fatal(err)
	}
	var en map[string]string
	if err := json.Unmarshal(b, &en); err != nil {
		t.Fatal(err)
	}
	for _, k := range page {
		if en["log.kind."+k] == "" {
			t.Errorf("the kind %q has no text: no log.kind.%s in ui/i18n/en.json", k, k)
		}
	}
	for k := cubelan.EventKind(0); k.String() != "unknown"; k++ {
		if !slices.Contains(eventKinds, k.String()) {
			t.Errorf("the Cube driver's kind %q is not in eventKinds", k)
		}
	}
}
