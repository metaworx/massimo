// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

import (
	"context"
	"io"
	"math"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"
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
	if code, ct, body := get("/"); code != 200 || !strings.HasPrefix(ct, "text/html") || !strings.Contains(body, `<script src="/ui/app.js">`) {
		t.Fatalf("GET /: %d %s %q", code, ct, body[:min(80, len(body))])
	}
	for _, p := range []string{"/ui/i18n.js", "/ui/app.js", "/ui/tiles.js", "/ui/config.js", "/ui/log.js", "/ui/update.js", "/ui/theme.js"} {
		if code, ct, _ := get(p); code != 200 || !strings.Contains(ct, "javascript") {
			t.Fatalf("GET %s: %d %s", p, code, ct)
		}
	}
	if code, ct, _ := get("/ui/style.css"); code != 200 || !strings.HasPrefix(ct, "text/css") {
		t.Fatalf("GET /ui/style.css: %d %s", code, ct)
	}
	for _, p := range []string{"/ui/", "/ui/index.html", "/ui/nope.js", "/other"} {
		if code, _, _ := get(p); code != 404 {
			t.Fatalf("GET %s: %d, want 404", p, code)
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
	req, _ := http.NewRequest("GET", ts.URL+"/api/v1/cubes/KEQ0537249/events?token=secret", nil)
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
	req, _ = http.NewRequest("GET", ts.URL+"/api/v1/cubes/KEQ0537249/events?token=wrong", nil)
	resp, err = http.DefaultClient.Do(req.WithContext(ctx))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 401 {
		t.Fatalf("events?token=wrong: %d, want 401", resp.StatusCode)
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
