// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package main

// The settings file while serve runs: serve looks at it every few
// seconds and, on Unix, on SIGHUP (sighup.go), and reads it again once it
// has changed. MASSIMO_API_TOKEN applies at once (Server.SetToken, as a
// start would apply it), and so does MASSIMO_REBOOT, the owner's switch
// of the reboot (Server.SetRebootAllowed); every other setting that
// changed is listed as applying at the next restart (GET
// /api/v1/restart), by name, never by value. A setting the next start
// would refuse makes the log's settings_read a warning, in the start's
// own words (startchecks.go), and still waits for that start; a reboot
// switch it would refuse is off. A file that does not parse, or is
// there but cannot be read, changes nothing, and its settings_read is a
// warning too (unreadable). The server keeps what each read found the
// next start refuses, which GET /api/v1/restart and POST /api/v1/reload
// answer (Server.SetRefused): a read that finds nothing clears it, and
// one that finds otherwise than the last writes its settings_read,
// though no setting changed, so that the log's sign agrees.

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"slices"
	"sort"
	"strconv"
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
	"listen": "MASSIMO_LISTEN", "token": tokenVar, "log": "MASSIMO_LOG", "log-level": env.LogLevel,
	"log-format": env.LogFormat, "log-file-format": env.LogFileFormat, "log-process-format": env.LogProcessFormat,
	"log-rotate-size": env.LogRotateSize, "log-keep": env.LogKeep, "log-keep-min": env.LogKeepMin, "update-request": "MASSIMO_UPDATE_REQUEST",
	"tls": env.TLS, "tls-cert": env.TLSCert, "tls-key": env.TLSKey, "tls-names": env.TLSNames, "acme-email": env.ACMEEmail,
	"acme-directory": env.ACMEDirectory, "tls-http": env.TLSHTTP, "trusted-proxies": env.TrustedProxies,
	"acme-dns-hook": env.ACMEDNSHook, "hsts": env.HSTS, "restart": restartVar, "sighup": sighupVar, "allow-reboot": rebootVar,
	"announce": env.Announce,
}

// fixedSettings are the variables of the flags given (flag name to its
// value), each with its flag's value, and MASSIMO_CUBE_SN with the Cube
// -sn selected: a flag given wins over the settings file, at this start
// and at the next.
func fixedSettings(given map[string]string) map[string]string {
	fixed := map[string]string{}
	for f, v := range flagVars {
		if value, ok := given[f]; ok {
			fixed[v] = value
		}
	}
	if snExplicit {
		fixed[env.SN] = selectedSN
	}
	return fixed
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
// Cube keys and IVs nothing reads any more (env.Removed), and the Cube a
// command picks.
func unusedByServe(k string) bool {
	return env.Removed(k) || k == env.SN || k == env.FileVar
}

// settingsReloader reads the settings file again, one reload at a time.
type settingsReloader struct {
	path  string
	log   *slog.Logger
	srv   *server.Server
	start map[string]string // settings() at the start
	fixed map[string]string // the variables of the flags given, with the flags' values (fixedSettings)
	// unread is set where the last read failed and its settings_read
	// warned of it (unreadable): the next read writes its own, which
	// settles it, though nothing changed.
	unread bool
	// lastRefused is what the last read found the next start refuses
	// (Server.SetRefused): a read whose settings did not change writes
	// its settings_read where that differs, a check of the box's state
	// having answered otherwise (a DNS hook made, a key file put in
	// place), so that the log's sign and GET /api/v1/restart agree.
	lastRefused []server.Refusal
	mu          sync.Mutex
}

func newSettingsReloader(path string, log *slog.Logger, srv *server.Server, start, fixed map[string]string) *settingsReloader {
	return &settingsReloader{path: path, log: log, srv: srv, start: start, fixed: fixed}
}

// isFixed reports whether a flag given fixed the variable k.
func (r *settingsReloader) isFixed(k string) bool {
	_, ok := r.fixed[k]
	return ok
}

// reload reads the file again and applies what changed; why says what
// asked for it, for the log. It returns the variables that changed,
// those that apply only at the next start and what that start refuses
// (POST /api/v1/reload, apiReload).
func (r *settingsReloader) reload(ctx context.Context, why string) (changed, pending []string, refused []server.Refusal, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	changed, err = env.ReloadFile(r.path)
	if err != nil {
		r.unreadable(ctx, why, err)
		return nil, nil, nil, err
	}
	r.srv.SetSettingsRead(time.Now())
	if slices.Contains(changed, tokenVar) && !r.isFixed(tokenVar) {
		if err := r.srv.SetToken(ctx, os.Getenv(tokenVar)); err != nil {
			r.log.Error("api token from the settings file not applied", "error", err)
		}
	}
	// The keep-open setting beside users, as a process of an older
	// version or a store put in place by hand can leave it (AP
	// FirstOwner, D12).
	r.srv.DropKeepOpen(ctx)
	if slices.Contains(changed, rebootVar) && !r.isFixed(rebootVar) {
		// A value the switch does not take is off; settings_read warns
		// of it, as the next start refuses it (refused; AP ReloadChecks,
		// D6-A).
		on, _ := parseSwitch(rebootVar, os.Getenv(rebootVar))
		r.srv.SetRebootAllowed(on)
	}
	pending = r.pending()
	r.srv.SetPendingRestart(pending)
	// What the next start refuses, kept for the API, cleared by a read
	// that finds nothing (AP ReloadChecks, D8-B); a change of it is
	// written to the log, though no setting changed.
	refused = r.refused()
	r.srv.SetRefused(refused)
	refusalsChanged := !slices.EqualFunc(refused, r.lastRefused, sameRefusal)
	r.lastRefused = refused
	if len(changed) > 0 || why != "the settings file changed" || r.unread || refusalsChanged {
		r.unread = false
		r.log.Info("settings file read again", "why", why, "changed", strings.Join(changed, ","), "at_next_restart", strings.Join(pending, ","))
		// The names, never the values; the errors of the refused
		// settings quote theirs, a secret's by its name. The event's
		// own line is the warning, on stderr as in the web UI's log, as
		// log_rotated's is.
		r.srv.LogSettingsRead(ctx, why, changed, pending, refusedError(refused))
	}
	return changed, pending, refused, nil
}

// apiReload is POST /api/v1/reload's read of the file (Server.SetReloader):
// reload, then after, what else the API's read does (serve.go: the log
// file's rotation, the store's checkpoint); its answer is what reload
// found, every list [] rather than null for none.
func (r *settingsReloader) apiReload(after func(context.Context)) func(context.Context) (server.ReloadResult, error) {
	return func(ctx context.Context) (server.ReloadResult, error) {
		changed, pending, refused, err := r.reload(ctx, "the API")
		if err != nil {
			return server.ReloadResult{}, err
		}
		after(ctx)
		return server.ReloadResult{Changed: nonNil(changed), Pending: nonNil(pending), Refused: refused, ReadAt: time.Now()}, nil
	}
}

// unreadable says that the settings file could not be read again, which
// changed nothing: env.ReloadFile parses the file whole first, so the
// process's settings, the token and what waits for the next start stay
// as they were. A missing file is the reloader's warning alone: an
// editor may be replacing it, and the next start takes it as no file, as
// env.LoadFile does. Any other error stops the next start of every
// command (main.go), so settings_read is a warning with it after "the
// next start refuses: ", the start's own words (AP ReloadChecks, D9-A),
// and the read's time is the event's, which the web UI shows as the
// file's; the next read writes its settings_read, which settles it. The
// error names the file and the line by its number, never the line,
// which may hold a secret (env.ReloadFile). The server keeps it as what
// the next start refuses, the file's alone, without a setting: the
// start refuses the file before it looks at one (D8-B).
func (r *settingsReloader) unreadable(ctx context.Context, why string, err error) {
	if os.IsNotExist(err) {
		r.log.Warn("settings file not read again", "file", r.path, "why", why, "error", err)
		return
	}
	refused := []server.Refusal{{Error: err.Error()}}
	r.srv.SetSettingsRead(time.Now())
	r.srv.SetRefused(refused)
	r.lastRefused = refused
	r.srv.LogSettingsRead(ctx, why, nil, r.pending(), refusedError(refused))
	r.unread = true
}

// refused is what the next start refuses of the settings now, an empty
// list for nothing: the start's checks that refuse (startRefusals), in
// its order, each with the variables it reads that differ from the
// start, and its error. A check runs where one of the variables it reads
// differs from the start and no flag fixed it, and reads a fixed flag's
// value where it has one, as the next start will (AP ReloadChecks,
// D2-A): an unchanged MASSIMO_TLS_HTTP is refused once MASSIMO_TLS
// changes from acme, and the refusal names MASSIMO_TLS. env.ReloadFile
// keeps a value the process environment set, which therefore never
// differs: the start took it and takes it again. A secret's value, the
// path of MASSIMO_TLS_KEY, is named by its variable instead (D5-A): the
// log and GET /api/v1/restart are read by more people than GET
// /api/v1/config, which does not show it; the start's own error, on its
// stderr, keeps the value.
func (r *settingsReloader) refused() []server.Refusal {
	now := settings()
	differs := map[string]bool{}
	for _, m := range []map[string]string{r.start, now} {
		for k := range m {
			if r.start[k] != now[k] && !r.isFixed(k) {
				differs[k] = true
			}
		}
	}
	get := settingOf(r.fixed)
	out := []server.Refusal{}
	for _, f := range startRefusals(get, differs) {
		out = append(out, server.Refusal{Settings: f.vars, Error: hideSecrets(f.err.Error(), get)})
	}
	return out
}

// sameRefusal reports whether two refusals name the same settings and
// say the same.
func sameRefusal(a, b server.Refusal) bool {
	return slices.Equal(a.Settings, b.Settings) && a.Error == b.Error
}

// refusedError is settings_read's error of what the next start refuses:
// the start's texts after "the next start refuses: ", in its order, nil
// for none.
func refusedError(refused []server.Refusal) error {
	if len(refused) == 0 {
		return nil
	}
	texts := make([]string, len(refused))
	for i, f := range refused {
		texts[i] = f.Error
	}
	return errors.New("the next start refuses: " + strings.Join(texts, "; "))
}

// hideSecrets names a secret the start's checks read by its variable
// where text quotes its value, escaped by %q or as it is, in one pass,
// so that a name put in is not searched again.
func hideSecrets(text string, get func(string) string) string {
	var pairs []string
	for _, c := range startChecks {
		for _, k := range c.vars {
			v := get(k)
			if v == "" || !server.SecretVariable(k) {
				continue
			}
			q := strconv.Quote(v)
			pairs = append(pairs, q[1:len(q)-1], k, v, k)
		}
	}
	if len(pairs) == 0 {
		return text
	}
	return strings.NewReplacer(pairs...).Replace(text)
}

// nonNil is a list as JSON shows an empty one: [], not null.
func nonNil(names []string) []string {
	if names == nil {
		return []string{}
	}
	return names
}

// pending are the variables whose value differs from the start and that
// apply only at the next one: not the token and the reboot's switch,
// which apply at once, not
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
			if r.start[k] != now[k] && k != tokenVar && k != rebootVar && !r.isFixed(k) && !unusedByServe(k) {
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
