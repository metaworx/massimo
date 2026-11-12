// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

// Commands (docs/server.md, "What version 1 prepares", item 7; NOTE
// StoreUsers v1.0: commands history): every request that changes
// something is a row of the store's commands table, the parent, and
// every engine command it puts on a Cube's queue is a step beneath it,
// written by the queue's Observer (D3-B). The parent takes its final
// state from the answer; a restore's parent from its run. A client's
// Idempotency-Key answers a retry with what the first request answered,
// so that nothing is sent to the Cube twice.

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.metaworx.ch/massimo/internal/cubeio"
	"go.metaworx.ch/massimo/internal/engine"
	"go.metaworx.ch/massimo/internal/store"
)

// Limits of the commands.
const (
	// maxIdempotencyKey is the longest Idempotency-Key accepted.
	maxIdempotencyKey = 128
	// maxAnswerKept is how much of an answer a command row keeps, for a
	// replay; a longer one is replayed as its first part.
	maxAnswerKept = 256 << 10
	// maxParentsCached bounds the observer's map of parent rows.
	maxParentsCached = 1024
)

// commandOf returns the uuid of the command a context's request is,
// "" for none.
func commandOf(ctx context.Context) string {
	if a, _ := ctx.Value(auditKey).(*auditInfo); a != nil {
		return a.commandUUID
	}
	return ""
}

// enqueue puts a command on the runtime's queue as a step of the
// context's command.
func (rt *runtime) enqueue(ctx context.Context, cmd engine.Command) *engine.Ticket {
	cmd.ID = commandOf(ctx)
	return rt.queue.Enqueue(cmd)
}

// writerOf is the runtime's writer whose commands are steps of the
// context's command.
func (rt *runtime) writerOf(ctx context.Context) cubeio.Writer {
	w := rt.writer()
	w.ID = commandOf(ctx)
	return w
}

// stepObserver writes the step rows of a Cube's queue, and a line of
// the Cube's log for each step, with its command (AP OneLog v1.1,
// Block 6): queued and sent at debug, acknowledged at info, a discard
// and retry, a drop and a failure at warn. The queue calls it under its
// lock and from its Run (engine.Observer), so every call only hands its
// write to the server's one ordered worker (s.steps), which writes them
// in the order the queue reported them: a command's row exists for its
// Started and Finished, and no radio command waits on the store's one
// connection.
type stepObserver struct {
	s       *Server
	rt      *runtime
	mu      sync.Mutex
	parents map[string]int64 // parent row id by uuid
	steps   map[uint64]int64 // step row id by the queue's Seq; only the worker uses it
	of      map[uint64]int64 // the step's parent row id by the queue's Seq; only the worker uses it
}

func (s *Server) newObserver(rt *runtime) engine.Observer {
	if s.opts.Store == nil {
		return nil
	}
	return &stepObserver{s: s, rt: rt, parents: map[string]int64{}, steps: map[uint64]int64{}, of: map[uint64]int64{}}
}

// stepEvent is the log's line of a step of the command parent, in a
// state: the device it is for, the step's row and what it says.
func (o *stepObserver) stepEvent(cmd engine.Command, parent, step int64, state, level string, summary map[string]any) {
	if summary == nil {
		summary = map[string]any{}
	}
	summary["state"], summary["step"], summary["key"] = state, step, cmd.Key
	ev := event{Kind: "command", Source: "queue", Level: level, Time: time.Now(), Command: parent, Summary: summary}
	if e, ok := summary["error"].(string); ok {
		ev.Error = e
		delete(summary, "error")
	}
	if cmd.Target != 0 {
		ev.Device = cmd.Target.String()
		ev.Serial = o.rt.client.Snapshot().Serial(cmd.Target)
	}
	o.rt.emit(ev)
}

// parentOf finds the parent row of a uuid, cached.
func (o *stepObserver) parentOf(ctx context.Context, uuid string) int64 {
	o.mu.Lock()
	id, ok := o.parents[uuid]
	o.mu.Unlock()
	if ok {
		return id
	}
	c, ok, err := o.s.opts.Store.CommandByUUID(ctx, uuid)
	if err != nil || !ok {
		return 0
	}
	o.mu.Lock()
	if len(o.parents) >= maxParentsCached {
		o.parents = map[string]int64{}
	}
	o.parents[uuid] = c.ID
	o.mu.Unlock()
	return c.ID
}

func (o *stepObserver) Enqueued(cmd engine.Command) { o.s.steps.do(func() { o.enqueued(cmd) }) }

func (o *stepObserver) Started(cmd engine.Command) { o.s.steps.do(func() { o.started(cmd) }) }

func (o *stepObserver) Finished(cmd engine.Command, res engine.Result) {
	o.s.steps.do(func() { o.finished(cmd, res) })
}

func (o *stepObserver) enqueued(cmd engine.Command) {
	ctx := context.Background()
	parent := o.parentOf(ctx, cmd.ID)
	if parent == 0 {
		return
	}
	req, _ := json.Marshal(map[string]any{"key": cmd.Key, "priority": int(cmd.Priority), "lines": len(cmd.Lines), "radio": radioLines(cmd),
		"wakeup": cmd.Wakeup, "room": cmd.RoomID, "reconnect": cmd.Reconnect})
	target := ""
	switch {
	case cmd.Target != 0:
		target = "device:" + cmd.Target.String()
	case cmd.RoomID != 0:
		target = "room:" + strconv.Itoa(int(cmd.RoomID))
	}
	step, err := o.s.opts.Store.AddStep(ctx, parent, store.Command{Target: target, Request: req})
	if err != nil {
		o.s.log.Error("commands: step not written", "command", cmd.ID, "error", err)
		return
	}
	o.steps[cmd.Seq], o.of[cmd.Seq] = step.ID, parent
	o.stepEvent(cmd, parent, step.ID, store.StateQueued, "debug", map[string]any{"radio": radioLines(cmd), "lines": len(cmd.Lines)})
}

func (o *stepObserver) started(cmd engine.Command) {
	id, ok := o.steps[cmd.Seq]
	if !ok {
		return
	}
	if err := o.s.opts.Store.SetCommandState(context.Background(), id, store.StateSent, 0, nil); err != nil {
		o.s.log.Error("commands: step not marked sent", "step", id, "error", err)
	}
	o.stepEvent(cmd, o.of[cmd.Seq], id, store.StateSent, "debug", nil)
	if cmd.Target != 0 && radioLines(cmd) > 0 {
		o.rt.sentTo.sent(cmd.Target, o.of[cmd.Seq])
	}
}

func (o *stepObserver) finished(cmd engine.Command, res engine.Result) {
	id, ok := o.steps[cmd.Seq]
	parent := o.of[cmd.Seq]
	delete(o.steps, cmd.Seq)
	delete(o.of, cmd.Seq)
	if !ok {
		return
	}
	state := store.StateAcknowledged
	result := map[string]any{"discarded": res.Discarded, "budget": res.Budget}
	switch {
	case res.Err == nil:
	case errors.Is(res.Err, engine.ErrSuperseded), errors.Is(res.Err, engine.ErrDiscarded), errors.Is(res.Err, context.Canceled), errors.Is(res.Err, context.DeadlineExceeded):
		state, result["error"] = store.StateDropped, res.Err.Error()
	default:
		state, result["error"] = store.StateFailed, res.Err.Error()
	}
	raw, _ := json.Marshal(result)
	if err := o.s.opts.Store.SetCommandState(context.Background(), id, state, 0, raw); err != nil {
		o.s.log.Error("commands: step not finished", "step", id, "error", err)
	}
	budget := func() map[string]any {
		return map[string]any{"duty_pct": res.Budget.DutyCyclePct, "free_slots": res.Budget.FreeMemorySlots}
	}
	if res.Retried {
		o.stepEvent(cmd, parent, id, "retried", "warn", budget())
	}
	level, summary := "info", budget()
	if res.Discarded {
		summary["discarded"] = true
	}
	if state != store.StateAcknowledged {
		level, summary["error"] = "warn", result["error"]
		if cmd.Target != 0 {
			o.rt.sentTo.forget(cmd.Target, parent)
		}
	}
	o.stepEvent(cmd, parent, id, state, level, summary)
}

// held is the queue's hold of its radio commands as a warning of the
// Cube's log: the duty cycle exhausted until its reset, or too few free
// memory slots for a while.
func (rt *runtime) held(h engine.Hold) {
	summary := map[string]any{"reason": h.Reason}
	switch h.Reason {
	case engine.HoldDutyCycle:
		summary["duty_pct"], summary["until"] = h.DutyPct, h.Until.UTC().Format(time.RFC3339)
	case engine.HoldMemorySlots:
		summary["free_slots"], summary["wait_s"] = h.FreeSlots, int(h.Wait.Seconds())
	}
	rt.emit(event{Kind: "hold", Source: "queue", Level: "warn", Time: time.Now(), Summary: summary})
}

// serialWorker runs functions one at a time in the order they were
// handed to it, on a goroutine of its own that lives while there is
// work; do never blocks.
type serialWorker struct {
	mu      sync.Mutex
	jobs    []func()
	running bool
}

// do hands f to the worker.
func (w *serialWorker) do(f func()) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.jobs = append(w.jobs, f)
	if !w.running {
		w.running = true
		go w.run()
	}
}

func (w *serialWorker) run() {
	for {
		w.mu.Lock()
		if len(w.jobs) == 0 {
			w.running = false
			w.mu.Unlock()
			return
		}
		f := w.jobs[0]
		w.jobs[0] = nil
		w.jobs = w.jobs[1:]
		w.mu.Unlock()
		f()
	}
}

// flush waits until everything handed to the worker before it ran, or
// until timeout.
func (w *serialWorker) flush(timeout time.Duration) {
	done := make(chan struct{})
	w.do(func() { close(done) })
	select {
	case <-done:
	case <-time.After(timeout):
	}
}

// fingerprint is what a retried request must repeat for its
// Idempotency-Key: the method, the path with its query and the body.
func fingerprint(a *auditInfo, query string) string {
	h := sha256.New()
	h.Write([]byte(a.method + " " + a.path + "?" + query + "\n"))
	h.Write(a.body)
	return hex.EncodeToString(h.Sum(nil))
}

// beginCommand writes the parent row of a request, once the principal
// is known, or answers it from the row its Idempotency-Key names. It
// reports whether the request was answered here.
func (s *Server) beginCommand(w http.ResponseWriter, r *http.Request, a *auditInfo) (answered bool) {
	st := s.opts.Store
	key := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if len(key) > maxIdempotencyKey {
		writeError(w, http.StatusBadRequest, "Idempotency-Key: at most 128 characters")
		a.skip = true
		return true
	}
	fp := fingerprint(a, r.URL.RawQuery)
	req, _ := json.Marshal(map[string]any{"method": a.method, "path": a.path, "query": r.URL.RawQuery, "hash": fp})
	c := store.Command{CubeID: a.cubeID, Key: key, Action: a.action, Target: a.target, Request: req}
	if a.p.named() {
		c.UserID, c.TokenID = a.p.user.ID, a.p.token.ID
	}
	a.w.keep = true
	row, err := st.AddCommand(r.Context(), c)
	if errors.Is(err, store.ErrKeyExists) {
		a.skip = true
		var had struct {
			Hash string `json:"hash"`
		}
		json.Unmarshal(row.Request, &had)
		switch {
		case !store.Final(row.State):
			writeError(w, http.StatusConflict, "a request with this Idempotency-Key is still running")
		case had.Hash != fp:
			writeError(w, http.StatusUnprocessableEntity, "Idempotency-Key reused for another request")
		default:
			// A row that got no answer, dropped at a start of the server
			// while its request ran, replays as 503 with why.
			status := row.Status
			if status == 0 {
				status = http.StatusServiceUnavailable
			}
			w.Header().Set("Idempotent-Replayed", "true")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(status)
			w.Write(firstAnswer(row))
		}
		return true
	}
	if err != nil {
		s.log.Error("commands: row not written", "action", a.action, "error", err)
		return false
	}
	a.commandID, a.commandUUID = row.ID, row.UUID
	return false
}

// laterRuns are the actions whose 202 is followed by a run that ends
// the command (finishCommand); another 202, such as the update's, which
// hands the work to the helper, is acknowledged with its answer.
var laterRuns = map[string]bool{"restore.start": true}

// endCommand sets the parent's final state from the answer: 2xx is
// acknowledged, 202 of a later run sent, 4xx and 503 dropped, the rest
// failed; the answer, without its secrets, is its result.
func (s *Server) endCommand(ctx context.Context, a *auditInfo, status int) {
	state := store.StateFailed
	switch {
	case status == http.StatusAccepted && laterRuns[a.action]:
		state = store.StateSent
	case status < 300:
		state = store.StateAcknowledged
	case status < 500 || status == http.StatusServiceUnavailable:
		state = store.StateDropped
	}
	result := withoutSecrets(a.w.answer.Bytes())
	if !json.Valid(result) {
		result, _ = json.Marshal(map[string]any{"answer": strings.TrimSpace(string(result))})
	}
	if err := s.opts.Store.SetCommandState(context.WithoutCancel(ctx), a.commandID, state, status, result); err != nil {
		s.log.Error("commands: row not finished", "command", a.commandUUID, "error", err)
	}
}

// withoutSecrets is an answer as a command row keeps it: an object
// without the secretFields at its top level, so that a new API key is
// shown once and its replay answers without it.
func withoutSecrets(answer []byte) []byte {
	var m map[string]json.RawMessage
	if json.Unmarshal(answer, &m) != nil {
		return answer
	}
	found := false
	for _, k := range secretFields {
		if _, ok := m[k]; ok {
			delete(m, k)
			found = true
		}
	}
	if !found {
		return answer
	}
	out, err := json.Marshal(m)
	if err != nil {
		return nil
	}
	return out
}

// firstAnswer is the body a replay answers: what the first request
// answered, which a run keeps under "answer" beside its outcome
// (finishCommand), and which endCommand keeps as the text of "answer"
// when it was not JSON.
func firstAnswer(row store.Command) []byte {
	var m map[string]json.RawMessage
	if json.Unmarshal(row.Result, &m) != nil || m["answer"] == nil {
		return row.Result
	}
	var text string
	if len(m) == 1 && json.Unmarshal(m["answer"], &text) == nil {
		return []byte(text)
	}
	if row.Status == http.StatusAccepted {
		return m["answer"]
	}
	return row.Result
}

// finishCommand ends a parent that was answered 202, from its run: the
// restore's end. The result is the run's outcome, with the 202's answer
// under "answer" for a replay; the run may end before endCommand has
// stored that answer, which a final row then no longer takes.
func (s *Server) finishCommand(uuid string, ok bool, result map[string]any, answer any) {
	st := s.opts.Store
	if st == nil || uuid == "" {
		return
	}
	ctx := context.Background()
	c, found, err := st.CommandByUUID(ctx, uuid)
	if err != nil || !found {
		return
	}
	s.steps.flush(2 * time.Second) // the run's steps before its end
	state := store.StateAcknowledged
	if !ok {
		state = store.StateFailed
	}
	outcome := map[string]any{"answer": answer}
	for k, v := range result {
		outcome[k] = v
	}
	raw, _ := json.Marshal(outcome)
	if err := st.SetCommandState(ctx, c.ID, state, -1, raw); err != nil {
		s.log.Error("commands: run not recorded", "command", uuid, "error", err)
	}
}

// commandView is a command as GET /api/v1/commands/{id} shows it.
type commandView struct {
	ID         string          `json:"id"`
	Action     string          `json:"action"`
	Target     string          `json:"target,omitempty"`
	Cube       string          `json:"cube,omitempty"`
	User       string          `json:"user,omitempty"`
	State      string          `json:"state"`
	Status     int             `json:"status,omitempty"`
	Request    json.RawMessage `json:"request"`
	Result     json.RawMessage `json:"result"`
	CreatedAt  time.Time       `json:"created_at"`
	UpdatedAt  time.Time       `json:"updated_at"`
	FinishedAt *time.Time      `json:"finished_at,omitempty"`
	Steps      []stepView      `json:"steps"`
	// Audit is the request's audit row, for a caller with audit; absent
	// for a proxy client's command, which has none.
	Audit *auditView `json:"audit,omitempty"`
}

// stepView is one engine command of a command.
type stepView struct {
	ID         string          `json:"id"`
	Target     string          `json:"target,omitempty"`
	State      string          `json:"state"`
	Request    json.RawMessage `json:"request"`
	Result     json.RawMessage `json:"result"`
	CreatedAt  time.Time       `json:"created_at"`
	FinishedAt *time.Time      `json:"finished_at,omitempty"`
}

// getCommand handles GET /api/v1/commands/{id}: a command by its id,
// or by its number as the log names it, with its steps, and its audit
// row for a caller with audit; a user without audit sees its own
// commands only.
func (s *Server) getCommand(w http.ResponseWriter, r *http.Request) error {
	st, err := s.userStore()
	if err != nil {
		return err
	}
	ctx := r.Context()
	id := r.PathValue("id")
	var c store.Command
	var ok bool
	if n, perr := strconv.ParseInt(id, 10, 64); perr == nil && n > 0 {
		c, ok, err = st.Command(ctx, n)
	} else {
		c, ok, err = st.CommandByUUID(ctx, id)
	}
	if err != nil {
		return userFail(err)
	}
	p := principalOf(r)
	if !ok || c.ParentID != 0 || p.named() && !p.has("audit") && c.UserID != p.user.ID {
		return fail(http.StatusNotFound, "no command %q", id)
	}
	v := commandView{ID: c.UUID, Action: c.Action, Target: c.Target, State: c.State, Status: c.Status, Request: c.Request, Result: c.Result,
		CreatedAt: c.CreatedAt, UpdatedAt: c.UpdatedAt, FinishedAt: optTime(c.FinishedAt), Steps: []stepView{}}
	if c.CubeID != 0 {
		if rows, err := st.Cubes(ctx); err == nil {
			for _, row := range rows {
				if row.ID == c.CubeID {
					v.Cube = row.Serial
				}
			}
		}
	}
	if c.UserID != 0 {
		if u, ok, _ := st.UserByID(ctx, c.UserID); ok {
			v.User = u.Name
		}
	}
	for _, st := range c.Steps {
		v.Steps = append(v.Steps, stepView{ID: st.UUID, Target: st.Target, State: st.State, Request: st.Request, Result: st.Result, CreatedAt: st.CreatedAt, FinishedAt: optTime(st.FinishedAt)})
	}
	if p.has("audit") {
		if e, found, err := st.AuditOfCommand(ctx, c.ID); err == nil && found {
			v.Audit = &auditView{ID: e.ID, Time: e.Time, User: e.User, TokenID: e.TokenID, Remote: e.Remote, Action: e.Action, Cube: e.Cube, Target: e.Target,
				Detail: e.Detail, Result: e.Result, CommandID: e.CommandID}
		}
	}
	writeJSON(w, http.StatusOK, v)
	return nil
}
