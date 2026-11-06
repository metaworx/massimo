// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

// The audit (docs/store.md section 5): every request that changes
// something is a row, the refused ones included. The api wrapper
// (server.go) records it for every non-GET route that names an action
// below: who (the principal), from where, what (the action, the Cube,
// the target), the request with its secrets removed, and how it went.
// A handler names its target where the path does not (auditTarget) and
// may rename the action (auditAction), as the login does when it fails.

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"go.metaworx.ch/massimo/internal/store"
)

// Limits of what an audit row keeps of a request and its answer.
const (
	// maxAuditBody is the longest request body kept in a row; a longer
	// one is kept as its size.
	maxAuditBody = 4096
	// maxAuditAnswer is how much of an error answer a row keeps.
	maxAuditAnswer = 1024
	// maxRequestBody is how much of a body the wrapper reads ahead for
	// the audit: the restore's limit, plus one byte so that the handler
	// still sees a body over it.
	maxRequestBody = maxBackupBytes + 1
)

// actions names the audit action of every route that changes something.
// A non-GET route without an entry is a test failure (audit_test.go);
// an empty action means the route is not audited.
var actions = map[string]string{
	"POST /api/v1/login":                           "login",
	"POST /api/v1/logout":                          "logout",
	"POST /api/v1/cubes":                           "cube.add",
	"PUT /api/v1/cubes/{sn}":                       "cube.update",
	"DELETE /api/v1/cubes/{sn}":                    "cube.remove",
	"POST /api/v1/cubes/discover":                  "discover",
	"PUT /api/v1/settings":                         "settings.update",
	"PUT /api/v1/cubes/{sn}/devices/{rf}/setpoint": "setpoint",
	"PUT /api/v1/cubes/{sn}/rooms/{id}/setpoint":   "setpoint",
	"DELETE /api/v1/cubes/{sn}/rooms/{id}":         "room.delete",
	"PUT /api/v1/cubes/{sn}/rooms/{id}":            "room.rename",
	"PUT /api/v1/cubes/{sn}/devices/{rf}":          "device.update",
	"DELETE /api/v1/cubes/{sn}/devices/{rf}":       "device.remove",
	"PUT /api/v1/cubes/{sn}/devices/{rf}/lock":     "device.lock",
	"PUT /api/v1/cubes/{sn}/rooms/{id}/config":     "room.config",
	"PUT /api/v1/cubes/{sn}/devices/{rf}/config":   "device.config",
	"PUT /api/v1/cubes/{sn}/rooms/{id}/program":    "room.program",
	"PUT /api/v1/cubes/{sn}/devices/{rf}/program":  "device.program",
	"POST /api/v1/cubes/{sn}/restore/plan":         "restore.plan",
	"POST /api/v1/cubes/{sn}/restore":              "restore.start",
	"POST /api/v1/cubes/{sn}/events/ticket":        "", // a read in disguise
	"POST /api/v1/cubes/{sn}/pairing":              "pairing.start",
	"DELETE /api/v1/cubes/{sn}/pairing":            "pairing.stop",
	"POST /api/v1/update":                          "update.start",
	"POST /api/v1/users":                           "user.add",
	"PUT /api/v1/users/{name}":                     "user.update",
	"DELETE /api/v1/users/{name}":                  "user.remove",
	"POST /api/v1/users/{name}/tokens":             "token.add",
	"DELETE /api/v1/users/{name}/tokens/{id}":      "token.revoke",
	"POST /api/v1/roles":                           "role.add",
	"PUT /api/v1/roles/{key}":                      "role.update",
	"DELETE /api/v1/roles/{key}":                   "role.remove",
	"PUT /api/v1/roles/{key}/disabled":             "role.disable",
	"PUT /api/v1/capabilities/{key}":               "capability.disable",
	"PUT /api/v1/me/preferences/{key}":             "preference.set",
	"DELETE /api/v1/me/preferences/{key}":          "preference.delete",
}

// noCommand are the actions that are audited but are no command: they
// send nothing to a Cube and a retry of them is harmless (commands.go).
var noCommand = map[string]bool{"login": true, "logout": true, "preference.set": true, "preference.delete": true}

// secretFields are the body fields an audit row never keeps.
var secretFields = []string{"password", "current_password", "token"}

// auditInfo is one audited request on its way through the wrapper.
type auditInfo struct {
	action  string
	target  string
	cubeID  int64
	pattern string
	path    string
	method  string
	body    []byte
	bodyLen int
	started time.Time
	p       *principal
	w       *statusWriter
	// commandID and commandUUID are the command the request is
	// (commands.go), 0 for none; skip says the request was answered
	// from an earlier command and leaves no row.
	commandID   int64
	commandUUID string
	skip        bool
}

const auditKey ctxKey = iota + 1

func auditOf(r *http.Request) *auditInfo {
	a, _ := r.Context().Value(auditKey).(*auditInfo)
	return a
}

// auditTarget names the target of the request's audit row, where the
// path does not: the user a login names, the user or role created.
func auditTarget(r *http.Request, target string) {
	if a := auditOf(r); a != nil {
		a.target = target
	}
}

// auditAction renames the request's action: login.failed for a login
// that failed.
func auditAction(r *http.Request, action string) {
	if a := auditOf(r); a != nil {
		a.action = action
	}
}

// statusWriter records the status answered and the start of an error
// answer.
type statusWriter struct {
	http.ResponseWriter
	status int
	answer bytes.Buffer
	// keep records the whole answer (up to maxAnswerKept) rather than
	// the start of an error, for a command row.
	keep bool
}

func (w *statusWriter) WriteHeader(status int) {
	if w.status == 0 {
		w.status = status
	}
	w.ResponseWriter.WriteHeader(status)
}

func (w *statusWriter) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	limit := maxAuditAnswer
	if w.keep {
		limit = maxAnswerKept
	}
	if (w.keep || w.status >= 400) && w.answer.Len() < limit {
		w.answer.Write(b[:min(len(b), limit-w.answer.Len())])
	}
	return w.ResponseWriter.Write(b)
}

func (w *statusWriter) Flush() {
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// pathTarget derives the target from the path's values: the device,
// the room, the token, the user, the role or capability, or the Cube.
func pathTarget(r *http.Request, pattern string) string {
	switch {
	case r.PathValue("rf") != "":
		return "device:" + strings.ToLower(r.PathValue("rf"))
	case r.PathValue("id") != "" && strings.Contains(pattern, "/tokens/"):
		return "token:" + r.PathValue("id")
	case r.PathValue("id") != "":
		return "room:" + r.PathValue("id")
	case r.PathValue("name") != "":
		return "user:" + store.NormalName(r.PathValue("name"))
	case r.PathValue("key") != "" && strings.Contains(pattern, "/preferences/"):
		return "preference:" + r.PathValue("key")
	case r.PathValue("key") != "" && strings.Contains(pattern, "/capabilities/"):
		return "capability:" + r.PathValue("key")
	case r.PathValue("key") != "":
		return "role:" + r.PathValue("key")
	case r.PathValue("sn") != "":
		return "cube:" + store.NormalSerial(r.PathValue("sn"))
	}
	return ""
}

// beginAudit prepares the row of a request: the body is read ahead and
// handed on, and the answer's status is recorded.
func (s *Server) beginAudit(w http.ResponseWriter, r *http.Request, pattern, action string) (*auditInfo, http.ResponseWriter, *http.Request) {
	a := &auditInfo{action: action, pattern: pattern, path: r.URL.Path, method: r.Method, started: time.Now(), target: pathTarget(r, pattern)}
	if r.Body != nil {
		b, err := io.ReadAll(io.LimitReader(r.Body, maxRequestBody))
		r.Body.Close()
		if err != nil {
			s.log.Warn("api: body not read", "error", err)
		}
		a.body, a.bodyLen = b, len(b)
		r.Body = io.NopCloser(bytes.NewReader(b))
	}
	if sn := r.PathValue("sn"); sn != "" && s.opts.Store != nil {
		if row, ok, err := s.opts.Store.Cube(r.Context(), sn); err == nil && ok {
			a.cubeID = row.ID
		}
	}
	a.w = &statusWriter{ResponseWriter: w}
	return a, a.w, r.WithContext(context.WithValue(r.Context(), auditKey, a))
}

// redacted is the body as the row keeps it: the JSON with the secret
// fields replaced, when it is JSON and short; nothing otherwise.
func redacted(body []byte) json.RawMessage {
	if len(body) == 0 || len(body) > maxAuditBody || !json.Valid(body) {
		return nil
	}
	var m map[string]any
	if json.Unmarshal(body, &m) != nil {
		return json.RawMessage(body) // an array or a scalar
	}
	for _, k := range secretFields {
		if _, ok := m[k]; ok {
			m[k] = "…"
		}
	}
	out, _ := json.Marshal(m)
	return out
}

// endAudit writes the row once the answer is out.
func (s *Server) endAudit(ctx context.Context, a *auditInfo, remote string) {
	st := s.opts.Store
	if st == nil {
		return
	}
	if a.skip {
		return
	}
	status := a.w.status
	if status == 0 {
		status = http.StatusOK
	}
	result := store.ResultFailed
	switch {
	case status < 300:
		result = store.ResultOK
	case status == http.StatusUnauthorized, status == http.StatusForbidden, status == http.StatusTooManyRequests:
		result = store.ResultRefused
	}
	detail := map[string]any{"method": a.method, "path": a.path, "status": status, "ms": time.Since(a.started).Milliseconds()}
	if a.bodyLen > 0 {
		detail["bytes"] = a.bodyLen
		if body := redacted(a.body); body != nil {
			detail["body"] = body
		}
	}
	if status >= 400 && a.w.answer.Len() > 0 {
		var e struct {
			Error string `json:"error"`
		}
		if json.Unmarshal(a.w.answer.Bytes(), &e) == nil && e.Error != "" {
			detail["error"] = e.Error
		} else {
			detail["error"] = strings.TrimSpace(a.w.answer.String())
		}
	}
	raw, _ := json.Marshal(detail)
	e := store.AuditEntry{Time: a.started, Remote: remote, Action: a.action, CubeID: a.cubeID, Target: a.target, Detail: raw, Result: result, CommandID: a.commandID}
	if a.p.named() {
		e.UserID, e.TokenID = a.p.user.ID, a.p.token.ID
	}
	// The request's context may be over; the row is written anyway.
	if _, err := st.Audit(context.WithoutCancel(ctx), e); err != nil {
		s.log.Error("api: audit row not written", "action", a.action, "error", err)
	}
	if a.commandID != 0 {
		s.endCommand(ctx, a, status)
	}
}

// auditView is one row as GET /api/v1/audit shows it.
type auditView struct {
	ID        int64           `json:"id"`
	Time      time.Time       `json:"time"`
	User      string          `json:"user,omitempty"`
	TokenID   int64           `json:"token_id,omitempty"`
	Remote    string          `json:"remote"`
	Action    string          `json:"action"`
	Cube      string          `json:"cube,omitempty"`
	Target    string          `json:"target,omitempty"`
	Detail    json.RawMessage `json:"detail"`
	Result    string          `json:"result"`
	CommandID int64           `json:"command_id,omitempty"`
}

// getAudit handles GET /api/v1/audit?before=&limit=&user=: the newest
// rows first, before a time, at most limit (100; at most 1000), of one
// user.
func (s *Server) getAudit(w http.ResponseWriter, r *http.Request) error {
	st, err := s.userStore()
	if err != nil {
		return err
	}
	q := r.URL.Query()
	var before time.Time
	if v := q.Get("before"); v != "" {
		if before, err = time.Parse(time.RFC3339Nano, v); err != nil {
			return fail(http.StatusBadRequest, "before: %v", err)
		}
	}
	limit := 100
	if v := q.Get("limit"); v != "" {
		if limit, err = strconv.Atoi(v); err != nil || limit < 1 || limit > 1000 {
			return fail(http.StatusBadRequest, "limit: 1 to 1000")
		}
	}
	rows, err := st.AuditList(r.Context(), before, limit, q.Get("user"))
	if err != nil {
		return userFail(err)
	}
	out := []auditView{}
	for _, e := range rows {
		out = append(out, auditView{ID: e.ID, Time: e.Time, User: e.User, TokenID: e.TokenID, Remote: e.Remote, Action: e.Action, Cube: e.Cube, Target: e.Target, Detail: e.Detail, Result: e.Result, CommandID: e.CommandID})
	}
	writeJSON(w, http.StatusOK, out)
	return nil
}
