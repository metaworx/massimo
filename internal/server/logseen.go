// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

// A person's seen marks of the log (AP LogAlerts v1.0, Block 2): the
// preference log.seen, {"all": <id>, "lines": [<id>, …]}, which the web
// UI writes: all marks every line up to that id seen ("Mark all as
// seen"), lines the lines marked one by one ("Mark seen"). GET
// /api/v1/log/alerts leaves out what the signed-in person marked.
// Opening the log marks nothing. A browser without a user to keep them
// for keeps its marks in its own storage and asks with after.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
)

// seenPrefKey is the preference that keeps a person's marks.
const seenPrefKey = "log.seen"

// maxSeenLines is how many lines a person may mark one by one; the page
// drops those that all covers.
const maxSeenLines = 500

// seenMarks are a person's marks: every line up to All, and Lines.
type seenMarks struct {
	All   int64   `json:"all"`
	Lines []int64 `json:"lines,omitempty"`
}

// parseSeen reads a log.seen document, refusing anything else.
func parseSeen(raw []byte) (seenMarks, error) {
	var m seenMarks
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&m); err != nil {
		return seenMarks{}, fmt.Errorf("%s: %v (want {\"all\": <id>, \"lines\": [<id>, …]})", seenPrefKey, err)
	}
	if dec.More() {
		return seenMarks{}, fmt.Errorf("%s: one object only", seenPrefKey)
	}
	if m.All < 0 {
		return seenMarks{}, fmt.Errorf("%s: all %d is not an event's id", seenPrefKey, m.All)
	}
	if len(m.Lines) > maxSeenLines {
		return seenMarks{}, fmt.Errorf("%s: %d lines, at most %d", seenPrefKey, len(m.Lines), maxSeenLines)
	}
	for _, id := range m.Lines {
		if id < 1 {
			return seenMarks{}, fmt.Errorf("%s: line %d is not an event's id", seenPrefKey, id)
		}
	}
	return m, nil
}

// seen reports whether the marks cover a line.
func (m seenMarks) seen(id int64) bool { return id <= m.All || slices.Contains(m.Lines, id) }

// seenOf is the signed-in person's marks; none without one, without a
// store, or where the preference cannot be read.
func (s *Server) seenOf(r *http.Request) seenMarks {
	p, st := principalOf(r), s.opts.Store
	if !p.named() || st == nil {
		return seenMarks{}
	}
	pr, ok, err := st.Preference(r.Context(), p.user.ID, seenPrefKey)
	if err != nil || !ok {
		return seenMarks{}
	}
	m, err := parseSeen(pr.Value)
	if err != nil {
		s.log.Debug("log: the seen marks not read", "user", p.user.Name, "error", err)
		return seenMarks{}
	}
	return m
}
