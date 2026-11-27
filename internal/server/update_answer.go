// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

// The question of an automatic update (AP AutoUpdate D3-A, D10-A): where
// automatic updates are on and an update is due, a holder of update is
// asked Install now, Tonight or Skip this version, in a banner under the
// header and in Software update. The answer is the box's, one row,
// update.answer, PUT /api/v1/update/answer: Tonight ends the question for
// its version, which the night installs as it would unanswered; Skip
// holds its version back from the night, and the question with it, until
// a newer release is out; Install now starts the update at once, as POST
// /api/v1/update does, its backup of the server first, and keeps nothing. A person may still install a
// skipped version with Update.

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"go.metaworx.ch/massimo/internal/store"
)

// updateAnswerKey is the setting that keeps the answer to the question.
const updateAnswerKey = "update.answer"

// The answers to the question; answerNow is never kept.
const (
	answerNow     = "now"
	answerTonight = "tonight"
	answerSkip    = "skip"
)

// updateAnswer is the answer to the question about a version: tonight or
// skip, by whom and when.
type updateAnswer struct {
	Version string `json:"version"`
	Answer  string `json:"answer"`
	// By is the user who answered; nil for the owner's key.
	By *eventUser `json:"by,omitempty"`
	At time.Time  `json:"at"`
}

// answerFor is the kept answer where it names version or a newer one,
// nil where there is none: an answer about an older version is no answer
// to a newer one's question.
func (s *Server) answerFor(ctx context.Context, version string) *updateAnswer {
	st := s.opts.Store
	if st == nil || version == "" {
		return nil
	}
	raw, ok, err := st.Setting(ctx, updateAnswerKey)
	if err != nil || !ok {
		return nil
	}
	var a updateAnswer
	if err := json.Unmarshal(raw, &a); err != nil || olderTag(a.Version, version) {
		return nil
	}
	return &a
}

// skippedVersion reports whether Skip this version holds version back
// from the night: the kept answer is skip, for it or a newer one.
func (s *Server) skippedVersion(ctx context.Context, version string) bool {
	a := s.answerFor(ctx, version)
	return a != nil && a.Answer == answerSkip
}

// putUpdateAnswer handles PUT /api/v1/update/answer, {"version": "v…",
// "answer": "now" | "tonight" | "skip"}, for a holder of update: the
// answer to the question about the version offered, 409 where the
// version is not the one an update would install. Install now answers as
// POST /api/v1/update does (202); Tonight and Skip, which need automatic
// updates on, keep the answer and answer what GET /api/v1/update says.
func (s *Server) putUpdateAnswer(w http.ResponseWriter, r *http.Request) error {
	st, err := s.cubeStore()
	if err != nil {
		return err
	}
	var req struct {
		Version string `json:"version"`
		Answer  string `json:"answer"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return fail(http.StatusBadRequest, "bad body: %v (want {\"version\": \"v…\", \"answer\": \"now\" | \"tonight\" | \"skip\"})", err)
	}
	switch req.Answer {
	case answerNow, answerTonight, answerSkip:
	default:
		return fail(http.StatusBadRequest, "answer: want %s, %s or %s", answerNow, answerTonight, answerSkip)
	}
	ctx := r.Context()
	v, _ := s.updateRelease(ctx, false)
	if !v.Available || req.Version != v.Latest {
		offered := v.Latest
		if !v.Available {
			offered = "none"
		}
		return fail(http.StatusConflict, "%q is not the version offered (%s)", req.Version, offered)
	}
	if req.Answer == answerNow {
		v, err := s.beginUpdate(ctx, userIDOf(r), false, false)
		if err != nil {
			return err
		}
		writeJSON(w, http.StatusAccepted, map[string]any{"ok": true, "requested": v.Latest, "channel": v.Channel})
		return nil
	}
	if !s.nightAt(ctx, time.Now()).on {
		return fail(http.StatusConflict, "automatic updates are off (the setting %s): there is no night to install it in or to skip", store.UpdateAutoKey)
	}
	a := updateAnswer{Version: req.Version, Answer: req.Answer, At: time.Now().UTC()}
	if id := userIDOf(r); id != 0 {
		if u, ok, err := st.UserByID(ctx, id); err == nil && ok {
			a.By = &eventUser{Name: u.Name, DisplayName: u.DisplayName}
		}
	}
	if err := st.SetSetting(ctx, updateAnswerKey, a); err != nil {
		return fail(http.StatusInternalServerError, "%v", err)
	}
	s.log.Info("update question answered", "version", a.Version, "answer", a.Answer)
	writeJSON(w, http.StatusOK, s.updateState(ctx, false))
	return nil
}
