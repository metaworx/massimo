// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

// The server's own lines in the log (AP Logging v1.1, Block 5), source
// system: its start and stop, the settings file read again, a restart, a
// reboot and an update asked for, an update's outcome, the backups and
// their prune, a restore started; each with who asked for it where a
// request did. The log's own prune is written by the store
// (store.pruneEvents). A Cube's new address and its leaving the LAN are
// the Cube's lines (discovery.go).

import (
	"context"
	"encoding/json"
	"sort"
	"time"

	"go.metaworx.ch/massimo/internal/update"
)

// The settings that remember what the log already said across a
// restart.
const (
	logVersionKey = "log.server_version" // the version of the last start
	logUpdateKey  = "log.update_logged"  // the helper's status time last logged
)

// systemEvent is an event of the server's own: a kind, who asked for it
// (0 for nobody) and what it says in short.
func systemEvent(kind string, userID int64, summary map[string]any) event {
	return event{Kind: kind, Source: "system", Time: time.Now(), userID: userID, Summary: summary}
}

// userIDOfContext is the user of a request's context, 0 for none (the
// bootstrap token, an open server).
func userIDOfContext(ctx context.Context) int64 {
	if p, _ := ctx.Value(principalKey).(*principal); p.named() {
		return p.user.ID
	}
	return 0
}

// emitBy sends a Cube's event with who started it named (logServer does
// the same for the server's).
func (s *Server) emitBy(rt *runtime, ev event) {
	s.nameUser(context.Background(), &ev)
	rt.emit(ev)
}

// noteStop says why the server will stop: a restart, a reboot or an
// update asked for; server_stopping says so.
func (s *Server) noteStop(why string) { s.stopWhy.Store(why) }

// logStarted writes server_started with the version, and the one before
// when it changed; and an update's outcome the helper reported since the
// last one the log said.
func (s *Server) logStarted(ctx context.Context) {
	version := s.opts.Update.Version // the release's, without the Go details
	if version == "" {
		version = s.opts.Version
	}
	summary := map[string]any{"version": version}
	st := s.opts.Store
	if st != nil && !st.ReadOnly() {
		var before string
		if raw, ok, err := st.Setting(ctx, logVersionKey); err == nil && ok {
			before = jsonString(raw)
		}
		if before != "" && before != version {
			summary["before"] = before
		}
		if err := st.SetSetting(ctx, logVersionKey, version); err != nil {
			s.log.Debug("log: the version not kept", "error", err)
		}
	}
	s.logServer(systemEvent("server_started", 0, summary))
	s.logUpdateResult(ctx)
}

// logUpdateResult writes the helper's last outcome, once: a final state
// newer than the one the log said last.
func (s *Server) logUpdateResult(ctx context.Context) {
	st := s.opts.Store
	if s.opts.Update.Status == "" || st == nil || st.ReadOnly() {
		return
	}
	us, err := update.ReadStatus(s.opts.Update.Status)
	if err != nil || us.Time.IsZero() || us.Running() {
		return
	}
	switch us.State {
	case "done", "up-to-date", "failed", "rolled-back":
	default:
		return
	}
	if raw, ok, err := st.Setting(ctx, logUpdateKey); err == nil && ok {
		if last, err := time.Parse(time.RFC3339Nano, jsonString(raw)); err == nil && !us.Time.After(last) {
			return
		}
	}
	ev := systemEvent("update_result", 0, map[string]any{"state": us.State, "from": us.From, "to": us.To, "channel": us.Channel})
	if us.State == "failed" || us.State == "rolled-back" {
		ev.Level, ev.Error = "warn", us.Message
	}
	ev.Time = us.Time
	s.logServer(ev)
	if err := st.SetSetting(ctx, logUpdateKey, us.Time.UTC().Format(time.RFC3339Nano)); err != nil {
		s.log.Debug("log: the update's outcome not kept as logged", "error", err)
	}
}

// logStopping writes server_stopping: why, a restart, a reboot or an
// update asked for, else a stop.
func (s *Server) logStopping() {
	why, _ := s.stopWhy.Load().(string)
	if why == "" {
		why = "stop"
	}
	s.logServer(systemEvent("server_stopping", 0, map[string]any{"why": why}))
}

// LogSettingsRead writes settings_read: the settings file read again,
// why (the file changed, SIGHUP, the API), the variables that changed
// and those that wait for the next start, by name, never their values,
// which may be secrets; with who asked for it through the API (ctx).
func (s *Server) LogSettingsRead(ctx context.Context, why string, changed, pending []string) {
	c, p := append([]string{}, changed...), append([]string{}, pending...)
	sort.Strings(c)
	sort.Strings(p)
	s.logServer(systemEvent("settings_read", userIDOfContext(ctx), map[string]any{"why": why, "changed": c, "at_next_start": p}))
}

// LogRotated writes log_rotated: serve's log file rotated (internal/
// logfile), why (size, format, sighup, reload), the name the old file has
// now, the rotated files kept and the names of those deleted; at warn,
// with the error, where a step failed.
func (s *Server) LogRotated(why, previous string, kept int, removed []string, err error) {
	ev := systemEvent("log_rotated", 0, map[string]any{"why": why, "previous": previous, "kept": kept, "removed": append([]string{}, removed...)})
	if err != nil {
		ev.Level, ev.Error = "warn", err.Error()
	}
	s.logServer(ev)
}

// jsonString is a setting's JSON string, "" for anything else.
func jsonString(raw []byte) string {
	if len(raw) < 2 || raw[0] != '"' {
		return ""
	}
	var v string
	if err := json.Unmarshal(raw, &v); err != nil {
		return ""
	}
	return v
}
