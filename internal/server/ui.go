// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

import (
	"embed"
	"io/fs"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// The web UI: plain HTML, CSS and JavaScript, no build step. It is
// served at / (the page) and /ui/ (the files, the translations under
// /ui/i18n/) and talks to the API.
//
//go:embed ui/index.html ui/i18n.js ui/app.js ui/tiles.js ui/config.js ui/log.js ui/update.js ui/cubes.js ui/theme.js ui/style.css ui/i18n/*.json
var uiFiles embed.FS

// uiLanguageFile is the name of a translation file: a language code.
var uiLanguageFile = regexp.MustCompile(`^[a-z][a-z]\.json$`)

// uiRoutes registers the page and its files. The token check applies to
// /api only; the files are public, the API behind them is not. Every
// file carries an ETag of the server version and its name, so that the
// browser, which revalidates each (no-cache), gets a 304 instead of the
// file again while the server version stays.
func (s *Server) uiRoutes(mux *http.ServeMux) {
	sub, err := fs.Sub(uiFiles, "ui")
	if err != nil {
		panic(err)
	}
	version := uiETagVersion(s.opts.Version, s.started)
	serve := func(w http.ResponseWriter, r *http.Request, name string) {
		if st, err := fs.Stat(sub, name); err != nil || st.IsDir() {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("ETag", `"`+version+"/"+name+`"`)
		http.ServeFileFS(w, r, sub, name)
	}
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		securityHeaders(w)
		serve(w, r, "index.html")
	})
	mux.HandleFunc("GET /ui/{file}", func(w http.ResponseWriter, r *http.Request) {
		securityHeaders(w)
		name := r.PathValue("file")
		if name == "index.html" || strings.Contains(name, "/") {
			http.NotFound(w, r)
			return
		}
		serve(w, r, name)
	})
	mux.HandleFunc("GET /ui/i18n/{lang}", func(w http.ResponseWriter, r *http.Request) {
		securityHeaders(w)
		name := r.PathValue("lang")
		if !uiLanguageFile.MatchString(name) {
			http.NotFound(w, r)
			return
		}
		serve(w, r, "i18n/"+name)
	})
}

// uiETagVersion is the version part of the UI files' ETags: the server
// version without the "massimo" in front and the build details after it,
// in the characters an ETag may hold. A build from a modified tree
// ("-dirty") can differ from another of the same version, so its files
// are told apart by the time the server started as well.
func uiETagVersion(v string, started time.Time) string {
	f := strings.Fields(strings.TrimPrefix(strings.TrimSpace(v), "massimo "))
	short := "dev"
	if len(f) > 0 {
		short = f[0]
	}
	if short == "dev" || strings.Contains(short, "dirty") {
		short += "+" + strconv.FormatInt(started.UnixNano(), 36)
	}
	return strings.Map(func(c rune) rune {
		if c <= ' ' || c == '"' || c == 0x7f || c > 0x7e {
			return '_'
		}
		return c
	}, short)
}

// securityHeaders restricts the page to its own origin: scripts, styles
// and requests only from the server itself, no framing, no MIME sniffing.
func securityHeaders(w http.ResponseWriter) {
	w.Header().Set("Content-Security-Policy", "default-src 'self'; img-src 'self' data:; frame-ancestors 'none'; base-uri 'self'; form-action 'self'")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Referrer-Policy", "no-referrer")
}
