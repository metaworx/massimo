// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

import (
	"bytes"
	"embed"
	"fmt"
	"io/fs"
	"net/http"
	"path"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
)

// The web UI: HTML, CSS and plain JavaScript, and the page's parts,
// TypeScript that make ui bundles from ui/src/ into ui/parts.js
// (tools/uibundle, docs/ui.md). git does not keep the bundle: it is named
// here, so that a build without it stops with its name. The UI is
// served at / (the page, assembled from index.html and the partials at
// start) and /ui/ (the files, the translations under /ui/i18n/, Massimo's
// mark under /ui/brand/, which make ui-brand writes from tools/brand,
// the vendored OPAQUE client under /ui/vendor/, ui/vendor/README.md) and
// talks to the API; beside the page, /sw.js is its service worker, whose
// scope is the page's, /manifest.webmanifest makes it installable, and
// /favicon.ico is where browsers look for an icon. /ui/connect.html is the
// apps' connect page, a page of its own (docs/app.md).
//
//go:embed ui/index.html ui/connect.html ui/connect.js ui/i18n.js ui/components.js ui/app.js ui/header.js ui/home.js ui/prefs.js ui/rooms.js ui/tiles.js ui/unlisted.js ui/house.js ui/week-program.js ui/config.js ui/log.js ui/pages.js ui/update.js ui/maintenance.js ui/system-info.js ui/cubes.js ui/repair.js ui/pairing.js ui/login.js ui/users.js ui/providers.js ui/integrations.js ui/backup.js ui/roles.js ui/theme.js ui/appearance.js ui/notify.js ui/notifytext.js ui/sw.js ui/manifest.webmanifest ui/style.css ui/i18n/*.json ui/brand/*.svg ui/brand/*.css ui/brand/*.png ui/brand/*.ico ui/partials/*.html ui/vendor/opaque.js ui/vendor/opaque.LICENSE ui/parts.js
var uiFiles embed.FS

// uiPartials are the parts of the page in ui/partials/, a file per
// feature, by the marker of index.html they replace
// (<!-- partials: system -->), in their order there: the header, the
// home notice, the first owner's form, the panels of My profile, of My
// settings (its rows first), of Notifications, the panels of System
// settings (its section cards in three markers: before, in and after the
// cards that flow in two columns), the cards of Maintenance, System
// info's card, the Cube page, the House tile, the devices without a
// room, the log, the sign-in dialog, the toast, then the templates the
// scripts clone, each named for what it holds and saying which script
// uses it.
var uiPartials = []struct {
	marker string
	files  []string
}{
	{"header", []string{"header.html"}},
	{"home", []string{"home.html"}},
	{"setup", []string{"setup.html"}},
	{"profile", []string{"profile.html"}},
	{"prefs", []string{"prefs.html", "appearance.html", "page.html"}},
	{"notify", []string{"notify.html"}},
	{"system", []string{"cubes.html"}},
	{"system-flow", []string{"users.html", "providers.html", "integrations.html", "roles.html", "capabilities.html"}},
	{"system-end", []string{"backup.html"}},
	{"maint", []string{"software.html", "maintenance.html"}},
	{"info", []string{"system-info.html"}},
	{"cube-page", []string{"cube-page.html"}},
	{"house", []string{"house-tile.html"}},
	{"unlisted", []string{"unlisted.html"}},
	{"logview", []string{"log-view.html"}},
	{"login", []string{"login.html"}},
	{"toast", []string{"toast.html"}},
	{"templates", []string{"house.html", "config.html", "pairing.html", "scheme.html", "log.html", "users-panel.html", "providers-panel.html", "roles-panel.html", "backups-panel.html"}},
}

// uiPage assembles the page: index.html, the skeleton, with each marker
// line replaced by its partials, every line of them indented as the
// marker is. A marker that is missing or there twice, or a partial that
// is missing, is an error.
func uiPage(files fs.FS) ([]byte, error) {
	page, err := fs.ReadFile(files, "index.html")
	if err != nil {
		return nil, err
	}
	for _, p := range uiPartials {
		marker := []byte("<!-- partials: " + p.marker + " -->")
		if n := bytes.Count(page, marker); n != 1 {
			return nil, fmt.Errorf("index.html: %d markers %s, want 1", n, marker)
		}
		at := bytes.Index(page, marker)
		start := bytes.LastIndexByte(page[:at], '\n') + 1
		indent := page[start:at]
		if len(bytes.TrimLeft(indent, " \t")) != 0 {
			return nil, fmt.Errorf("index.html: marker %s not on a line of its own", marker)
		}
		end := at + len(marker)
		if i := bytes.IndexByte(page[end:], '\n'); i >= 0 {
			end += i + 1
		} else {
			end = len(page)
		}
		var b bytes.Buffer
		for _, name := range p.files {
			part, err := fs.ReadFile(files, "partials/"+name)
			if err != nil {
				return nil, err
			}
			for _, line := range bytes.SplitAfter(part, []byte("\n")) {
				if len(bytes.TrimSpace(line)) > 0 {
					b.Write(indent)
				}
				b.Write(line)
			}
			if len(part) > 0 && part[len(part)-1] != '\n' {
				b.WriteByte('\n')
			}
		}
		page = slices.Concat(page[:start], b.Bytes(), page[end:])
	}
	return page, nil
}

// uiLanguageFile is the name of a translation file: a language code.
var uiLanguageFile = regexp.MustCompile(`^[a-z][a-z]\.json$`)

// uiRoutes registers the page and its files. The page is assembled once
// (uiPage); index.html and the partials are not served on their own.
// The token check applies to /api only; the files are public, the API
// behind them is not. Every file carries an ETag of the server version and its name, so that the
// browser, which revalidates each (no-cache), gets a 304 instead of the
// file again while the server version stays.
func (s *Server) uiRoutes(mux *http.ServeMux) {
	sub, err := fs.Sub(uiFiles, "ui")
	if err != nil {
		panic(err)
	}
	page, err := uiPage(sub)
	if err != nil {
		panic(err)
	}
	version := uiETagVersion(s.opts.Version, s.started)
	cache := func(w http.ResponseWriter, name string) {
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("ETag", `"`+version+"/"+name+`"`)
	}
	serve := func(w http.ResponseWriter, r *http.Request, name string) {
		if st, err := fs.Stat(sub, name); err != nil || st.IsDir() {
			http.NotFound(w, r)
			return
		}
		cache(w, name)
		http.ServeFileFS(w, r, sub, name)
	}
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		securityHeaders(w)
		cache(w, "index.html")
		http.ServeContent(w, r, "index.html", time.Time{}, bytes.NewReader(page))
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
	// The service worker beside the page, so that its scope is the
	// page's, and the manifest, whose relative addresses resolve there.
	mux.HandleFunc("GET /sw.js", func(w http.ResponseWriter, r *http.Request) {
		securityHeaders(w)
		serve(w, r, "sw.js")
	})
	mux.HandleFunc("GET /manifest.webmanifest", func(w http.ResponseWriter, r *http.Request) {
		securityHeaders(w)
		w.Header().Set("Content-Type", "application/manifest+json")
		serve(w, r, "manifest.webmanifest")
	})
	// Massimo's mark: an SVG is an image with a style of its own (the
	// favicon's dark rule), which the page's policy would refuse it; an
	// image runs no script. Go's table has no .ico.
	brand := func(w http.ResponseWriter, r *http.Request, name string) {
		securityHeaders(w)
		switch path.Ext(name) {
		case ".svg":
			w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'")
		case ".ico":
			w.Header().Set("Content-Type", "image/x-icon")
		}
		serve(w, r, "brand/"+name)
	}
	mux.HandleFunc("GET /ui/brand/{file}", func(w http.ResponseWriter, r *http.Request) {
		brand(w, r, r.PathValue("file"))
	})
	mux.HandleFunc("GET /favicon.ico", func(w http.ResponseWriter, r *http.Request) {
		brand(w, r, "favicon.ico")
	})
	mux.HandleFunc("GET /ui/vendor/{file}", func(w http.ResponseWriter, r *http.Request) {
		securityHeaders(w)
		serve(w, r, "vendor/"+r.PathValue("file"))
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
// WebAssembly is allowed to compile, which the vendored OPAQUE client
// needs ('wasm-unsafe-eval' allows that and no eval of JavaScript); the
// service worker and the manifest are the server's own as well.
func securityHeaders(w http.ResponseWriter) {
	w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self' 'wasm-unsafe-eval'; img-src 'self' data:; worker-src 'self'; manifest-src 'self'; frame-ancestors 'none'; base-uri 'self'; form-action 'self'")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Referrer-Policy", "no-referrer")
}
