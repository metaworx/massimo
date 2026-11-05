// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"
)

// The web UI: plain HTML, CSS and JavaScript, no build step. It is
// served at / (the page) and /ui/ (the files) and talks to the API.
//
//go:embed ui/index.html ui/app.js ui/tiles.js ui/config.js ui/log.js ui/style.css
var uiFiles embed.FS

// uiRoutes registers the page and its files. The token check applies to
// /api only; the files are public, the API behind them is not.
func (s *Server) uiRoutes(mux *http.ServeMux) {
	sub, err := fs.Sub(uiFiles, "ui")
	if err != nil {
		panic(err)
	}
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		securityHeaders(w)
		w.Header().Set("Cache-Control", "no-cache")
		http.ServeFileFS(w, r, sub, "index.html")
	})
	mux.HandleFunc("GET /ui/{file}", func(w http.ResponseWriter, r *http.Request) {
		securityHeaders(w)
		name := r.PathValue("file")
		if name == "index.html" || strings.Contains(name, "/") {
			http.NotFound(w, r)
			return
		}
		if _, err := fs.Stat(sub, name); err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Cache-Control", "no-cache")
		http.ServeFileFS(w, r, sub, name)
	})
}

// securityHeaders restricts the page to its own origin: scripts, styles
// and requests only from the server itself, no framing, no MIME sniffing.
func securityHeaders(w http.ResponseWriter) {
	w.Header().Set("Content-Security-Policy", "default-src 'self'; img-src 'self' data:; frame-ancestors 'none'; base-uri 'self'; form-action 'self'")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Referrer-Policy", "no-referrer")
}
