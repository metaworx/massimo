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
//go:embed ui/index.html ui/app.js ui/tiles.js ui/style.css
var uiFiles embed.FS

// uiRoutes registers the page and its files. The token check applies to
// /api only; the files are public, the API behind them is not.
func (s *Server) uiRoutes(mux *http.ServeMux) {
	sub, err := fs.Sub(uiFiles, "ui")
	if err != nil {
		panic(err)
	}
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-cache")
		http.ServeFileFS(w, r, sub, "index.html")
	})
	mux.HandleFunc("GET /ui/{file}", func(w http.ResponseWriter, r *http.Request) {
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
