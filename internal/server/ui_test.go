// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

import (
	"context"
	"io"
	"net/http"
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
	for _, p := range []string{"/ui/app.js", "/ui/tiles.js", "/ui/config.js", "/ui/log.js", "/ui/update.js"} {
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
