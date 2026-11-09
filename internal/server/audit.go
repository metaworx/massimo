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
// A request refused before anyone is known (no token, a wrong one, a
// locked-out address) is no row of its own: such requests are one row
// per address and minute with their count, and their bodies are never
// read, so that a loop of them neither fills the store nor reads
// megabytes (refusalAudit).

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.metaworx.ch/massimo/internal/store"
	"go.metaworx.ch/massimo/internal/view"
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
	"POST /api/v1/login/start":                     "login.start",
	"POST /api/v1/login/finish":                    "login",
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
	"POST /api/v1/cubes/{sn}/backups":              "backup.take",
	"DELETE /api/v1/cubes/{sn}/backups/{id}":       "backup.delete",
	"POST /api/v1/cubes/{sn}/events/ticket":        "", // a read in disguise
	"POST /api/v1/cubes/{sn}/pairing":              "pairing.start",
	"DELETE /api/v1/cubes/{sn}/pairing":            "pairing.stop",
	"POST /api/v1/update":                          "update.start",
	"POST /api/v1/restart":                         "server.restart",
	"POST /api/v1/reload":                          "server.reload",
	"POST /api/v1/reboot":                          "server.reboot",
	"DELETE /api/v1/reboot":                        "server.reboot.cancel",
	"POST /api/v1/users":                           "user.add",
	"PUT /api/v1/users/{name}":                     "user.update",
	"DELETE /api/v1/users/{name}":                  "user.remove",
	"POST /api/v1/users/{name}/tokens":             "token.add",
	"POST /api/v1/users/{name}/register":           "", // nothing is kept
	"DELETE /api/v1/users/{name}/tokens/{id}":      "token.revoke",
	"POST /api/v1/roles":                           "role.add",
	"PUT /api/v1/roles/{key}":                      "role.update",
	"DELETE /api/v1/roles/{key}":                   "role.remove",
	"PUT /api/v1/roles/{key}/disabled":             "role.disable",
	"PUT /api/v1/capabilities/{key}":               "capability.disable",
	"PUT /api/v1/me/preferences/{key}":             "preference.set",
	"DELETE /api/v1/me/preferences/{key}":          "preference.delete",
	"POST /api/v1/me/push":                         "push.subscribe",
	"DELETE /api/v1/me/push/{id}":                  "push.unsubscribe",
}

// changeLines says which line of the log each audited action writes once
// it succeeded (AP Logging v1.1, Block 6): "change", a line under
// Configuration with who made it, the setpoints included; "system", none
// here, its handler writing its own line (systemlog.go); "" none: a
// sign-in, a preference or a push subscription, a search, a restore's
// plan. TestEveryActionHasALine holds it to actions.
var changeLines = map[string]string{
	"login": "", "login.start": "", "logout": "", "preference.set": "", "preference.delete": "", "discover": "", "restore.plan": "",
	"push.subscribe": "", "push.unsubscribe": "",
	"update.start": "system", "server.restart": "system", "server.reload": "system", "server.reboot": "system",
	"server.reboot.cancel": "system", "backup.take": "system", "backup.delete": "system", "restore.start": "system",
	"cube.add": "change", "cube.update": "change", "cube.remove": "change", "settings.update": "change",
	"setpoint": "change", "room.delete": "change", "room.rename": "change", "device.update": "change", "device.remove": "change",
	"device.lock": "change", "room.config": "change", "device.config": "change", "room.program": "change", "device.program": "change",
	"pairing.start": "change", "pairing.stop": "change",
	"user.add": "change", "user.update": "change", "user.remove": "change", "token.add": "change", "token.revoke": "change",
	"role.add": "change", "role.update": "change", "role.remove": "change", "role.disable": "change", "capability.disable": "change",
}

// noCommand are the actions that are audited but are no command: they
// send nothing to a Cube and a retry of them is harmless (commands.go).
var noCommand = map[string]bool{"login": true, "login.start": true, "logout": true, "preference.set": true, "preference.delete": true,
	"push.subscribe": true, "push.unsubscribe": true, "server.reload": true}

// secretFields are the body fields an audit row never keeps.
var secretFields = []string{"password", "current_password", "token", "record", "proof", "login", "ke1", "ke3"}

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
	// sn is the Cube the path names; changes and subject what a handler
	// tells the log's change line (auditChange, auditSubject).
	sn      string
	changes map[string]view.Change
	subject map[string]any
}

// auditChange tells the request's change line a value that changed, from
// and to: a Cube's name, a setting.
func auditChange(r *http.Request, name string, from, to any) {
	if a := auditOf(r); a != nil {
		if a.changes == nil {
			a.changes = map[string]view.Change{}
		}
		a.changes[name] = view.Change{From: from, To: to}
	}
}

// auditSubject tells the request's change line the user, role or key it
// concerns, as they are then: their ids and names, never a secret.
func auditSubject(r *http.Request, subject map[string]any) {
	if a := auditOf(r); a != nil {
		a.subject = subject
	}
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

// beginAudit prepares the row of an authenticated request, whose
// answer w records: the body of an authorized one is read ahead and
// handed on; a refused one's is not read.
func (s *Server) beginAudit(w *statusWriter, r *http.Request, pattern, action string, authorized bool) (*auditInfo, *http.Request) {
	a := &auditInfo{action: action, pattern: pattern, path: r.URL.Path, method: r.Method, started: time.Now(), target: pathTarget(r, pattern), w: w}
	if rf := r.PathValue("rf"); rf != "" {
		// A device by its serial number, whichever form the path held.
		a.target = s.deviceTarget(r.PathValue("sn"), rf)
	}
	if r.Body != nil && authorized {
		b, err := io.ReadAll(io.LimitReader(r.Body, maxRequestBody))
		r.Body.Close()
		if err != nil {
			s.log.Warn("api: body not read", "error", err)
		}
		a.body, a.bodyLen = b, len(b)
		r.Body = io.NopCloser(bytes.NewReader(b))
	}
	a.cubeID, a.sn = s.cubeIDOf(r.Context(), r.PathValue("sn")), r.PathValue("sn")
	return a, r.WithContext(context.WithValue(r.Context(), auditKey, a))
}

// cubeIDOf is the store's id of the Cube a path names, 0 for none.
func (s *Server) cubeIDOf(ctx context.Context, sn string) int64 {
	if sn == "" || s.opts.Store == nil {
		return 0
	}
	if row, ok, err := s.opts.Store.Cube(ctx, sn); err == nil && ok {
		return row.ID
	}
	return 0
}

// resultOf is how a status went, as an audit row says it.
func resultOf(status int) string {
	switch {
	case status < 300:
		return store.ResultOK
	case status == http.StatusUnauthorized, status == http.StatusForbidden, status == http.StatusTooManyRequests:
		return store.ResultRefused
	}
	return store.ResultFailed
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
	result := resultOf(status)
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
	if a.commandID != 0 {
		// The command's steps are written first, so that whoever reads
		// the row reads its steps.
		s.steps.flush(2 * time.Second)
	}
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
	if result == store.ResultOK && changeLines[a.action] == "change" {
		s.logChange(a)
	}
}

// logChange writes the change line of a request that changed something:
// its action, its target (a device by its serial), what it set (the
// request's body, its secrets removed), what changed from and to and the
// user, role or key it concerns, where the handler said; with who made
// it. A Cube's change is the Cube's line, every other the server's.
func (s *Server) logChange(a *auditInfo) {
	summary := map[string]any{"action": a.action}
	if a.target != "" {
		summary["target"] = a.target
	}
	if body := redacted(a.body); body != nil {
		var set map[string]any
		if json.Unmarshal(body, &set) == nil && len(set) > 0 {
			summary["set"] = set
		}
	}
	if len(a.changes) > 0 {
		summary["changes"] = a.changes
	}
	if a.subject != nil {
		summary["subject"] = a.subject
	}
	ev := event{Kind: "change", Source: "config", Time: a.started, Summary: summary}
	if a.p.named() {
		ev.userID = a.p.user.ID
	}
	if dev, ok := strings.CutPrefix(a.target, "device:"); ok && deviceSerial.MatchString(dev) {
		ev.Serial = strings.ToUpper(dev)
	}
	sn := a.sn
	if c, ok := strings.CutPrefix(a.target, "cube:"); ok && sn == "" {
		sn = c
	}
	s.mu.RLock()
	rt := s.cubes[strings.ToLower(sn)]
	s.mu.RUnlock()
	if rt != nil && rt.storeID.Load() != 0 {
		s.emitBy(rt, ev)
		return
	}
	if sn != "" {
		summary["cube"] = store.NormalSerial(sn)
	}
	s.logServer(ev)
}

// refusalWindow is the span one row of refused requests covers.
const refusalWindow = time.Minute

// refusalAudit keeps the refused requests of each address for a minute
// and writes them as one row: the first request's action, target and
// path, with the count of requests within the minute and of each
// status. A row is written when its minute ends, at the address's next
// refused request after it, when more addresses are open than the
// limiter tracks (the oldest first), and when the server stops.
type refusalAudit struct {
	window time.Duration
	max    int
	now    func() time.Time
	write  func(refusal)

	mu     sync.Mutex
	open   map[string]*refusal
	closed bool
}

// refusal is one address's refused requests within a minute.
type refusal struct {
	remote, action, target, sn, method, path string
	first, last                              time.Time
	status, count                            int // the first request's status, and how many
	statuses                                 map[int]int
	timer                                    *time.Timer
}

func newRefusalAudit(write func(refusal)) *refusalAudit {
	return &refusalAudit{window: refusalWindow, max: maxTracked, now: time.Now, write: write, open: map[string]*refusal{}}
}

// note counts a refused request of the address.
func (ra *refusalAudit) note(r *http.Request, pattern, action, remote string, status int) {
	if ra == nil {
		return
	}
	if status == 0 {
		status = http.StatusOK
	}
	now := ra.now()
	var due []refusal
	ra.mu.Lock()
	f := ra.open[remote]
	if f != nil && now.Sub(f.first) >= ra.window {
		due = append(due, ra.takeLocked(remote))
		f = nil
	}
	if f == nil {
		for len(ra.open) >= ra.max {
			due = append(due, ra.takeLocked(ra.oldestLocked()))
		}
		nf := &refusal{remote: remote, action: action, target: pathTarget(r, pattern), sn: r.PathValue("sn"), method: r.Method, path: r.URL.Path,
			first: now, status: status, statuses: map[int]int{}}
		if !ra.closed {
			nf.timer = time.AfterFunc(ra.window, func() { ra.flush(remote, nf) })
		}
		ra.open[remote], f = nf, nf
	}
	f.last = now
	f.count++
	f.statuses[status]++
	if ra.closed {
		due = append(due, ra.takeLocked(remote))
	}
	ra.mu.Unlock()
	for _, d := range due {
		ra.write(d)
	}
}

// flush writes the address's row once its minute is over, if it is
// still the open one.
func (ra *refusalAudit) flush(remote string, f *refusal) {
	ra.mu.Lock()
	if ra.open[remote] != f {
		ra.mu.Unlock()
		return
	}
	d := ra.takeLocked(remote)
	ra.mu.Unlock()
	ra.write(d)
}

// close writes every open row; later refusals are written at once.
func (ra *refusalAudit) close() {
	if ra == nil {
		return
	}
	ra.mu.Lock()
	ra.closed = true
	var due []refusal
	for remote := range ra.open {
		due = append(due, ra.takeLocked(remote))
	}
	ra.mu.Unlock()
	for _, d := range due {
		ra.write(d)
	}
}

// takeLocked removes an open row and stops its timer.
func (ra *refusalAudit) takeLocked(remote string) refusal {
	f := ra.open[remote]
	delete(ra.open, remote)
	if f.timer != nil {
		f.timer.Stop()
	}
	return *f
}

// oldestLocked is the address whose open row began first.
func (ra *refusalAudit) oldestLocked() string {
	oldest, at := "", time.Time{}
	for remote, f := range ra.open {
		if oldest == "" || f.first.Before(at) {
			oldest, at = remote, f.first
		}
	}
	return oldest
}

// writeRefusal writes the row of an address's refused requests.
func (s *Server) writeRefusal(f refusal) {
	ctx := context.Background()
	statuses := map[string]int{}
	for status, n := range f.statuses {
		statuses[strconv.Itoa(status)] = n
	}
	detail := map[string]any{"method": f.method, "path": f.path, "status": f.status, "count": f.count, "statuses": statuses, "last": f.last}
	raw, _ := json.Marshal(detail)
	e := store.AuditEntry{Time: f.first, Remote: f.remote, Action: f.action, CubeID: s.cubeIDOf(ctx, f.sn), Target: f.target, Detail: raw, Result: resultOf(f.status)}
	if _, err := s.opts.Store.Audit(ctx, e); err != nil {
		s.log.Error("api: audit row of refused requests not written", "remote", f.remote, "error", err)
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
