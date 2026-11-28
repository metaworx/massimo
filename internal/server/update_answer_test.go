// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

// The question of an automatic update (AP AutoUpdate v1.0, Block 5):
// PUT /api/v1/update/answer, what each answer does to the question and to
// the night.

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"go.metaworx.ch/massimo/internal/store"
)

// answerBox is a box with automatic updates at night that runs
// v0.1.0-alpha.9 on alpha and finds f's release, with the helper's
// request file; anna is an owner, val a viewer, who lacks update.
type answerBox struct {
	s            *Server
	ts           *httptest.Server
	request      string
	anna, viewer string
	st           *store.Store
}

func newAnswerBox(t *testing.T, f *fakeFinder) answerBox {
	t.Helper()
	dir := t.TempDir()
	st := testStore(t, "")
	b := answerBox{st: st, request: filepath.Join(dir, "update-request")}
	b.s = New(Options{Store: st, Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), Update: UpdateOptions{
		Version: "v0.1.0-alpha.9", Channel: "alpha", Request: b.request, Status: filepath.Join(dir, "status.json"), Finder: f,
	}})
	b.ts = httptest.NewServer(b.s.Handler())
	t.Cleanup(b.ts.Close)
	b.anna, b.viewer = userToken(t, st, "anna", store.RoleOwner), userToken(t, st, "val", store.RoleViewer)
	if err := st.SetSetting(t.Context(), store.UpdateAutoKey, store.UpdateAutoNight); err != nil {
		t.Fatal(err)
	}
	return b
}

// auto is GET /api/v1/update's auto.
func (b answerBox) auto(t *testing.T, refresh bool) autoView {
	t.Helper()
	path := "/api/v1/update"
	if refresh {
		path += "?refresh=1"
	}
	var out struct {
		Auto *autoView `json:"auto"`
	}
	if code := getJSON(t, b.ts, b.anna, path, &out); code != 200 || out.Auto == nil {
		t.Fatalf("GET: %d %+v", code, out.Auto)
	}
	return *out.Auto
}

// answer puts an answer as token's user.
func (b answerBox) answer(t *testing.T, token, version, answer string) (int, string) {
	t.Helper()
	return callAPI(t, b.ts, "PUT", "/api/v1/update/answer", token, `{"version": "`+version+`", "answer": "`+answer+`"}`)
}

// The route: update needed, an answer of the three, the version offered
// and no other; Tonight ends the question, kept with who answered,
// audited as update.answer; the night installs the version as it would
// unanswered.
func TestUpdateAnswerTonight(t *testing.T) {
	t.Parallel()
	b := newAnswerBox(t, &fakeFinder{tag: "v0.1.0-alpha.10"})
	if a := b.auto(t, true); !a.Ask || a.Answer != nil || a.Next == nil {
		t.Fatalf("before an answer: %+v", a)
	}
	if code, text := b.answer(t, b.viewer, "v0.1.0-alpha.10", "tonight"); code != 403 {
		t.Fatalf("a viewer: %d %s", code, text)
	}
	if code, text := b.answer(t, b.anna, "v0.1.0-alpha.9", "tonight"); code != 409 ||
		!refusalHas(t, text, "update_not_offered", `{"offered":"v0.1.0-alpha.10","version":"v0.1.0-alpha.9"}`) {
		t.Fatalf("another version: %d %s", code, text)
	}
	if code, text := b.answer(t, b.anna, "v0.1.0-alpha.10", "later"); code != 400 {
		t.Fatalf("another answer: %d %s", code, text)
	}
	code, text := b.answer(t, b.anna, "v0.1.0-alpha.10", "tonight")
	var out struct {
		Auto autoView `json:"auto"`
	}
	if code != 200 || json.Unmarshal([]byte(text), &out) != nil {
		t.Fatalf("tonight: %d %s", code, text)
	}
	if a := out.Auto; a.Ask || a.Answer == nil || a.Answer.Answer != "tonight" || a.Answer.Version != "v0.1.0-alpha.10" ||
		a.Answer.By == nil || a.Answer.By.Name != "anna" || a.Next == nil {
		t.Fatalf("the answer: %+v %+v", a, a.Answer)
	}
	rows := auditRows(t, b.ts, b.anna, "?limit=1")
	if len(rows) != 1 || rows[0].Action != "update.answer" {
		t.Fatalf("the audit: %+v", rows)
	}
	b.s.updateTick(t.Context(), nightAt(3, 0))
	if c, err := os.ReadFile(b.request); err != nil || string(c) != "alpha\n" {
		t.Fatalf("the night's request: %q %v", c, err)
	}
}

// Skip this version holds it back from the night and the question with
// it, until a newer release is out, which is asked about and installed.
func TestUpdateAnswerSkip(t *testing.T) {
	t.Parallel()
	f := &fakeFinder{tag: "v0.1.0-alpha.10"}
	b := newAnswerBox(t, f)
	if code, text := b.answer(t, b.anna, "v0.1.0-alpha.10", "skip"); code != 200 {
		t.Fatalf("skip: %d %s", code, text)
	}
	if a := b.auto(t, false); a.Ask || a.Answer == nil || a.Answer.Answer != "skip" || a.Next != nil {
		t.Fatalf("skipped: %+v", a)
	}
	b.s.updateTick(t.Context(), nightAt(3, 0))
	if _, err := os.Stat(b.request); err == nil {
		t.Fatal("the skipped version was installed")
	}
	f.tag = "v0.1.0-alpha.11"
	if a := b.auto(t, true); !a.Ask || a.Answer != nil || a.Next == nil {
		t.Fatalf("a newer release: %+v", a)
	}
	b.s.updateTick(t.Context(), nightAt(3, 0).AddDate(0, 0, 1))
	if _, err := os.Stat(b.request); err != nil {
		t.Fatalf("the newer release not installed: %v", err)
	}
}

// Install now starts the update at once, by whoever answered, as POST
// /api/v1/update does; it keeps no answer.
func TestUpdateAnswerNow(t *testing.T) {
	t.Parallel()
	b := newAnswerBox(t, &fakeFinder{tag: "v0.1.0-alpha.10"})
	code, text := b.answer(t, b.anna, "v0.1.0-alpha.10", "now")
	if code != 202 {
		t.Fatalf("now: %d %s", code, text)
	}
	if _, err := os.Stat(b.request); err != nil {
		t.Fatalf("no request: %v", err)
	}
	list := b.s.updateHistory(t.Context())
	if len(list) != 1 || list[0].Auto || list[0].By == nil || list[0].By.Name != "anna" || list[0].To != "v0.1.0-alpha.10" {
		t.Fatalf("the history: %+v", list)
	}
	if a := b.s.answerFor(t.Context(), "v0.1.0-alpha.10"); a != nil {
		t.Fatalf("an answer kept: %+v", a)
	}
}

// With automatic updates off nothing is asked, Tonight and Skip are
// refused, and Install now still starts the update.
func TestUpdateAnswerOff(t *testing.T) {
	t.Parallel()
	b := newAnswerBox(t, &fakeFinder{tag: "v0.1.0-alpha.10"})
	if err := b.st.SetSetting(t.Context(), store.UpdateAutoKey, store.UpdateAutoOff); err != nil {
		t.Fatal(err)
	}
	if a := b.auto(t, true); a.Ask || a.Mode != "off" {
		t.Fatalf("off: %+v", a)
	}
	for _, answer := range []string{"tonight", "skip"} {
		if code, text := b.answer(t, b.anna, "v0.1.0-alpha.10", answer); code != 409 || !refusalHas(t, text, "update_auto_off", `{"setting":"update.auto"}`) {
			t.Fatalf("%s while off: %d %s", answer, code, text)
		}
	}
	if code, text := b.answer(t, b.anna, "v0.1.0-alpha.10", "now"); code != 202 {
		t.Fatalf("now while off: %d %s", code, text)
	}
}
