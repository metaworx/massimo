// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package main

// The settings file while serve runs: serve looks at it every few
// seconds and, on Unix, on SIGHUP (sighup.go), and reads it again once it
// has changed. MASSIMO_API_TOKEN applies at once (Server.SetToken, as a
// start would apply it); every other setting that changed is listed as
// applying at the next restart (GET /api/v1/restart), by name, never by
// value.

import (
	"context"
	"log/slog"
	"os"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"go.metaworx.ch/massimo/internal/env"
	"go.metaworx.ch/massimo/internal/server"
	"go.metaworx.ch/massimo/internal/store"
)

// tokenVar is the API token's variable.
const tokenVar = "MASSIMO_API_TOKEN"

// settingsWatchEvery is how often serve looks at its settings file.
const settingsWatchEvery = 5 * time.Second

// flagVars are serve's flags that default to a variable: a flag given on
// the command line wins over the file, now and at the next start.
var flagVars = map[string]string{
	"listen": "MASSIMO_LISTEN", "token": tokenVar, "log": "MASSIMO_LOG", "log-level": env.LogLevel, "update-request": "MASSIMO_UPDATE_REQUEST",
	"tls": env.TLS, "tls-cert": env.TLSCert, "tls-key": env.TLSKey, "tls-names": env.TLSNames, "acme-email": env.ACMEEmail,
	"acme-directory": env.ACMEDirectory, "tls-http": env.TLSHTTP, "trusted-proxies": env.TrustedProxies,
	"acme-dns-hook": env.ACMEDNSHook, "hsts": env.HSTS, "restart": restartVar, "sighup": sighupVar,
}

// fileStamp is what watchSettings compares: a file's size and time of
// change, ok when it exists.
type fileStamp struct {
	size, mod int64
	ok        bool
}

func stampOf(path string) fileStamp {
	fi, err := os.Stat(path)
	if err != nil {
		return fileStamp{}
	}
	return fileStamp{size: fi.Size(), mod: fi.ModTime().UnixNano(), ok: true}
}

// watchSettings calls reload once the file at path has changed and held
// still for one more look, so that a file an editor is still writing is
// not read; a missing file changes nothing, since an editor may be
// replacing it. It returns when ctx ends.
func watchSettings(ctx context.Context, path string, every time.Duration, reload func()) {
	applied, seen := stampOf(path), fileStamp{}
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
		now := stampOf(path)
		switch {
		case !now.ok || now == applied:
			seen = fileStamp{}
		case now == seen:
			reload()
			applied, seen = now, fileStamp{}
		default:
			seen = now
		}
	}
}

// settings are the variables serve reads, as the process has them.
func settings() map[string]string {
	out := map[string]string{}
	for _, kv := range os.Environ() {
		if k, v, _ := strings.Cut(kv, "="); strings.HasPrefix(k, "MASSIMO_") && env.Known(k) {
			out[k] = v
		}
	}
	return out
}

// unusedByServe reports a variable serve does not read for itself: the
// Cube keys and IVs, which only the portal commands need, and the Cube a
// command picks.
func unusedByServe(k string) bool {
	if k == env.Key || k == env.IV || k == env.SN || k == env.FileVar {
		return true
	}
	mid, ok := strings.CutPrefix(k, env.Prefix)
	return ok && (strings.HasSuffix(mid, "_KEY") || strings.HasSuffix(mid, "_IV"))
}

// settingsReloader reads the settings file again, one reload at a time.
type settingsReloader struct {
	path  string
	log   *slog.Logger
	srv   *server.Server
	start map[string]string // settings() at the start
	fixed map[string]bool   // the variables of the flags given
	mu    sync.Mutex
}

func newSettingsReloader(path string, log *slog.Logger, srv *server.Server, start map[string]string, given map[string]bool) *settingsReloader {
	fixed := map[string]bool{}
	for f, v := range flagVars {
		if given[f] {
			fixed[v] = true
		}
	}
	return &settingsReloader{path: path, log: log, srv: srv, start: start, fixed: fixed}
}

// reload reads the file again and applies what changed; why says what
// asked for it, for the log. It returns the variables that changed and
// those that apply only at the next start (POST /api/v1/reload).
func (r *settingsReloader) reload(ctx context.Context, why string) (changed, pending []string, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	changed, err = env.ReloadFile(r.path)
	if err != nil {
		r.log.Warn("settings file not read again", "file", r.path, "why", why, "error", err)
		return nil, nil, err
	}
	r.srv.SetSettingsRead(time.Now())
	if slices.Contains(changed, tokenVar) && !r.fixed[tokenVar] {
		if err := r.srv.SetToken(ctx, os.Getenv(tokenVar)); err != nil {
			r.log.Error("api token from the settings file not applied", "error", err)
		}
	}
	pending = r.pending()
	r.srv.SetPendingRestart(pending)
	if len(changed) > 0 || why != "the settings file changed" {
		r.log.Info("settings file read again", "why", why, "changed", strings.Join(changed, ","), "at_next_restart", strings.Join(pending, ","))
	}
	return changed, pending, nil
}

// nonNil is a list as JSON shows an empty one: [], not null.
func nonNil(names []string) []string {
	if names == nil {
		return []string{}
	}
	return names
}

// pending are the variables whose value differs from the start and that
// apply only at the next one: not the token, which applies at once, not
// those a flag fixed, and not those serve does not read.
func (r *settingsReloader) pending() []string {
	now := settings()
	var out []string
	seen := map[string]bool{}
	for _, m := range []map[string]string{r.start, now} {
		for k := range m {
			if seen[k] {
				continue
			}
			seen[k] = true
			if r.start[k] != now[k] && k != tokenVar && !r.fixed[k] && !unusedByServe(k) {
				out = append(out, k)
			}
		}
	}
	sort.Strings(out)
	return out
}

// checkpointStore brings the store's WAL into massimo.db, for a SIGHUP
// (Store.Checkpoint); serve's stop does the same when it closes the store.
func checkpointStore(ctx context.Context, st *store.Store, log *slog.Logger) {
	if st == nil {
		return
	}
	ck, err := st.Checkpoint(ctx)
	switch {
	case err != nil:
		log.Warn("store not checkpointed", "error", err)
	case ck.Busy:
		log.Warn("store checkpoint incomplete: a reader holds the WAL", "wal_bytes", ck.WALBytes)
	default:
		log.Info("store checkpointed", "wal_bytes", ck.WALBytes)
	}
}
