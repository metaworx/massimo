// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

// The last updates (AP UpdateHistory v1.0): one settings row,
// update.history, a JSON list of the last ten, newest first. An update
// started in Massimo adds an entry with who started it, the version it
// ran, the one it asked for and the channel; the next start with another
// version completes it with the version reached. A new version that no
// entry waits for came from outside Massimo: apt, brew or a person on
// the box, on the channel the box followed at that start. An update the
// helper ended without restarting the server ends at the next read of
// the update's state (AP UpdateHistory v1.1). The log says the same in
// its lines, but keeps them for a month only.

import (
	"context"
	"encoding/json"
	"time"

	"go.metaworx.ch/massimo/internal/update"
)

// updateHistoryKey is the setting that keeps the last updates.
const updateHistoryKey = "update.history"

// updateHistoryKeep is how many updates the row keeps.
const updateHistoryKeep = 10

// updateEntry is one update: started in Massimo (By, To) or found at a
// start (Outside), and the version it reached at the next start.
type updateEntry struct {
	At time.Time `json:"at"` // when it was started, or found
	// By is the user who started it; nil for the owner's key, and for an
	// update from outside Massimo.
	By *eventUser `json:"by,omitempty"`
	// Outside is set where Massimo did not start it: the version changed
	// between two starts without an entry waiting for it.
	Outside bool   `json:"outside,omitempty"`
	Manager string `json:"manager,omitempty"` // the package manager of a box that has one, for Outside
	From    string `json:"from,omitempty"`    // the version that ran before
	To      string `json:"to,omitempty"`      // the version asked for, where Massimo started it
	// Channel is the channel the update came from: the one asked for,
	// where Massimo started it, else the one the box followed at the
	// start that found it (updateChannel).
	Channel string `json:"channel,omitempty"`
	// Reached is the version the next start ran, and ReachedAt when; both
	// empty while the update runs.
	Reached   string     `json:"reached,omitempty"`
	ReachedAt *time.Time `json:"reached_at,omitempty"`
	// Result is the update helper's outcome where it ended without
	// reaching To: failed, rolled-back or up-to-date.
	Result string `json:"result,omitempty"`
}

// waiting reports whether the entry waits for the start that completes
// it.
func (e updateEntry) waiting() bool { return e.Reached == "" && e.Result == "" }

// updateHistory is the row's entries, newest first; none where there is
// no store or the row cannot be read.
func (s *Server) updateHistory(ctx context.Context) []updateEntry {
	st := s.opts.Store
	if st == nil {
		return nil
	}
	raw, ok, err := st.Setting(ctx, updateHistoryKey)
	if err != nil || !ok {
		return nil
	}
	var list []updateEntry
	if err := json.Unmarshal(raw, &list); err != nil {
		s.log.Debug("update history not read", "error", err)
		return nil
	}
	return list
}

// editHistory reads the row, lets edit change it, and writes it whole
// where edit says it changed, keeping the newest updateHistoryKeep.
func (s *Server) editHistory(ctx context.Context, edit func([]updateEntry) []updateEntry) {
	st := s.opts.Store
	if st == nil || st.ReadOnly() {
		return
	}
	s.historyMu.Lock()
	defer s.historyMu.Unlock()
	list := edit(s.updateHistory(ctx))
	if list == nil {
		return
	}
	if len(list) > updateHistoryKeep {
		list = list[:updateHistoryKeep]
	}
	if err := st.SetSetting(ctx, updateHistoryKey, list); err != nil {
		s.log.Warn("update history not kept", "error", err)
	}
}

// noteUpdateStart adds the entry of an update started in Massimo at at:
// by whom (userID, 0 for the owner's key), from the running version to
// the one asked for on channel.
func (s *Server) noteUpdateStart(ctx context.Context, at time.Time, userID int64, from, to, channel string) {
	e := updateEntry{At: at.UTC(), From: from, To: to, Channel: channel}
	if userID != 0 {
		if u, ok, err := s.opts.Store.UserByID(ctx, userID); err == nil && ok {
			e.By = &eventUser{Name: u.Name, DisplayName: u.DisplayName}
		}
	}
	s.editHistory(ctx, func(list []updateEntry) []updateEntry {
		return append([]updateEntry{e}, list...)
	})
}

// noteVersionStart completes the history at a start of version, where
// the last start ran before ("" for none known). A new version completes
// the newest entry where it waits for it, or for an unknown version;
// one that no entry waits for is an update from outside Massimo, through
// the box's package manager where it has one, on the channel the box
// follows now. An entry the update helper ended without reaching its
// version gets the helper's outcome (helper, nil where the box has
// none); an entry it completes keeps its own channel.
func (s *Server) noteVersionStart(ctx context.Context, before, version string, helper *update.Status) {
	now := time.Now().UTC()
	s.editHistory(ctx, func(list []updateEntry) []updateEntry {
		var newest *updateEntry
		if len(list) > 0 {
			newest = &list[0]
		}
		switch {
		case before == "" || before == version:
			// No new version: an entry still waiting ended where the
			// helper says so.
			if newest != nil && newest.waiting() {
				if r := helperEnded(helper, newest, version); r != "" {
					newest.Result = r
					return list
				}
			}
			return nil
		case newest != nil && newest.waiting() && (newest.To == "" || newest.To == version):
			newest.Reached, newest.ReachedAt = version, &now
			return list
		case newest != nil && newest.Reached == before && newest.From == version && helperEnded(helper, newest, version) == "rolled-back":
			// The new version ran, and the helper put the old one back.
			newest.Result = "rolled-back"
			return list
		}
		e := updateEntry{At: now, Outside: true, Manager: s.opts.Update.Manager, Channel: s.updateChannel(ctx), From: before, Reached: version, ReachedAt: &now}
		return append([]updateEntry{e}, list...)
	})
}

// noteHelperOutcome ends the newest entry where it still waits and the
// update helper has reported a final state since it was started
// (helperFinal): an update the helper ended without restarting the
// server, "apt has nothing newer yet", is not under way until the next
// start, but ends at the next read of the update's state (updateRelease).
// A helper that restarts the server is left to that start: while it
// says restarting, the old version still answers, and the version it
// came from being the one running is no rollback there. The row is
// written only where the entry ends.
func (s *Server) noteHelperOutcome(ctx context.Context, helper *update.Status) {
	if helper == nil || !unreached(helper.State) {
		return
	}
	s.editHistory(ctx, func(list []updateEntry) []updateEntry {
		if len(list) == 0 || !list[0].waiting() {
			return nil
		}
		r := helperFinal(helper, &list[0])
		if r == "" {
			return nil
		}
		list[0].Result = r
		return list
	})
}

// helperEnded is how the update helper's run of e ended without reaching
// its version, "" while it runs or where its status is older than e: a
// final state (helperFinal), or, at a start, rolled-back where it
// restarts the version it came from (version, the one starting) before
// it says so.
func helperEnded(helper *update.Status, e *updateEntry, version string) string {
	if r := helperFinal(helper, e); r != "" {
		return r
	}
	if helper != nil && helper.Time.After(e.At) && helper.State == "restarting" && helper.From == version && helper.Running() {
		return "rolled-back"
	}
	return ""
}

// helperFinal is the final state the update helper reported for e
// without reaching its version, failed, rolled-back or up-to-date; ""
// while it runs, where it says done, and where its status is older than
// e.
func helperFinal(helper *update.Status, e *updateEntry) string {
	if helper == nil || !unreached(helper.State) || !helper.Time.After(e.At) {
		return ""
	}
	return helper.State
}

// unreached reports whether state is one of the update helper's final
// states that leave the version asked for unreached.
func unreached(state string) bool {
	switch state {
	case "failed", "rolled-back", "up-to-date":
		return true
	}
	return false
}

// helperStatus is the update helper's last status, nil where the box
// has none or it cannot be read.
func (s *Server) helperStatus() *update.Status {
	if s.opts.Update.Status == "" {
		return nil
	}
	us, err := update.ReadStatus(s.opts.Update.Status)
	if err != nil || us.Time.IsZero() {
		return nil
	}
	return &us
}
