// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

import (
	"context"
	"encoding/json"
	"io"
	"io/fs"
	"maps"
	"math"
	"net/http"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"go.metaworx.ch/massimo/internal/cubelan"
)

func TestUIServed(t *testing.T) {
	_, ts, _ := start(t, "secret")
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
	for _, p := range []string{"/ui/i18n.js", "/ui/components.js", "/ui/app.js", "/ui/tiles.js", "/ui/config.js", "/ui/log.js", "/ui/update.js", "/ui/cubes.js", "/ui/login.js", "/ui/users.js", "/ui/roles.js", "/ui/theme.js"} {
		if code, ct, _ := get(p); code != 200 || !strings.Contains(ct, "javascript") {
			t.Fatalf("GET %s: %d %s", p, code, ct)
		}
	}
	if code, ct, _ := get("/ui/style.css"); code != 200 || !strings.HasPrefix(ct, "text/css") {
		t.Fatalf("GET /ui/style.css: %d %s", code, ct)
	}
	for _, lang := range uiLanguages {
		p := "/ui/i18n/" + lang + ".json"
		if code, ct, body := get(p); code != 200 || !strings.HasPrefix(ct, "application/json") || !strings.HasPrefix(body, "{") {
			t.Fatalf("GET %s: %d %s", p, code, ct)
		}
	}
	for _, p := range []string{"/ui/", "/ui/index.html", "/ui/nope.js", "/ui/i18n", "/ui/i18n/", "/ui/i18n/xx.json", "/ui/i18n/en.js", "/ui/i18n/EN.json", "/ui/partials/cubes.html", "/ui/partials/", "/other"} {
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
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	resp, err := http.DefaultClient.Do(req.WithContext(ctx))
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

// The page is assembled from index.html and the partials: every file of
// ui/partials/ is listed in uiPartials exactly once and every listed one
// exists, no marker is left, the page is served at / as assembled, and
// every id the scripts look up ($("…")) and every template they clone
// (fromTemplate("…"), fragmentFromTemplate("…")) is in it.
func TestUIPage(t *testing.T) {
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
	ids := map[string]bool{}
	for _, m := range regexp.MustCompile(`\sid="([^"]+)"`).FindAllStringSubmatch(page, -1) {
		if ids[m[1]] {
			t.Errorf("the page has id %q twice", m[1])
		}
		ids[m[1]] = true
	}
	lookup := regexp.MustCompile(`(?:\$|fromTemplate|fragmentFromTemplate)\("([^"]+)"\)`)
	scripts, _ := fs.Glob(uiFiles, "ui/*.js")
	n := 0
	for _, f := range scripts {
		b, err := uiFiles.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range lookup.FindAllStringSubmatch(string(b), -1) {
			n++
			if !ids[m[1]] {
				t.Errorf("%s looks up %q, which is not an id of the page", f, m[1])
			}
		}
	}
	if n < 50 {
		t.Fatalf("only %d lookups found in the scripts", n)
	}

	_, ts, _ := start(t, "")
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

func TestUIETagVersion(t *testing.T) {
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

// The translation files hold the texts of the page: each parses, the
// other languages have no key English lacks (but their own plural forms
// and the machine-translation note), and English has every key the
// markup and the scripts ask for.
func TestUITranslations(t *testing.T) {
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
	html := assembledPage(t)
	for _, m := range regexp.MustCompile(`data-(i18n|i18n-attr|help|help-label)="([^"]*)"`).FindAllStringSubmatch(string(html), -1) {
		switch m[1] {
		case "i18n", "help-label":
			used[m[2]] = "the page"
		case "help":
			used["help."+m[2]] = "the page"
		case "i18n-attr":
			for _, pair := range strings.Fields(m[2]) {
				if i := strings.Index(pair, ":"); i > 0 {
					used[pair[i+1:]] = "the page"
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
	if len(used) < 100 {
		t.Fatalf("only %d keys found in the markup and the scripts", len(used))
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
// tile, and the ok, warn and bad texts on a tile read at WCAG AA (4.5).
func TestUIDefaultSchemeMatchesCSS(t *testing.T) {
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
	if len(light) != 19 || len(dark) != 19 || len(system) != 19 {
		t.Fatalf("style.css: %d light, %d dark, %d system tokens, want 19 each", len(light), len(dark), len(system))
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
		}
	}
}

// TestLogKinds keeps the page's kinds of event (LOG_KINDS in log.js), the
// server's (eventKinds) and the Cube driver's (cubelan.EventKind) one
// list.
func TestLogKinds(t *testing.T) {
	js, err := uiFiles.ReadFile("ui/log.js")
	if err != nil {
		t.Fatal(err)
	}
	m := regexp.MustCompile(`(?s)const LOG_KINDS = \[(.*?)\];`).FindSubmatch(js)
	if m == nil {
		t.Fatal("log.js: no LOG_KINDS")
	}
	var page []string
	for _, k := range regexp.MustCompile(`"([a-z_]+)"`).FindAllSubmatch(m[1], -1) {
		page = append(page, string(k[1]))
	}
	if strings.Join(page, " ") != strings.Join(eventKinds, " ") {
		t.Fatalf("log.js LOG_KINDS %v, the server's eventKinds %v", page, eventKinds)
	}
	for k := cubelan.EventKind(0); k.String() != "unknown"; k++ {
		if !slices.Contains(eventKinds, k.String()) {
			t.Errorf("the Cube driver's kind %q is not in eventKinds", k)
		}
	}
}
