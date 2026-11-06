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

// stepObserver writes the step rows of a Cube's queue.
type stepObserver struct {
	s       *Server
	mu      sync.Mutex
	parents map[string]int64 // parent row id by uuid
	steps   map[uint64]int64 // step row id by the queue's Seq
}

func (s *Server) newObserver() engine.Observer {
	if s.opts.Store == nil {
		return nil
	}
	return &stepObserver{s: s, parents: map[string]int64{}, steps: map[uint64]int64{}}
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

func (o *stepObserver) Enqueued(cmd engine.Command) {
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
		target = "room:" + itoa(int(cmd.RoomID))
	}
	step, err := o.s.opts.Store.AddStep(ctx, parent, store.Command{Target: target, Request: req})
	if err != nil {
		o.s.log.Error("commands: step not written", "command", cmd.ID, "error", err)
		return
	}
	o.mu.Lock()
	o.steps[cmd.Seq] = step.ID
	o.mu.Unlock()
}

func (o *stepObserver) Started(cmd engine.Command) {
	o.mu.Lock()
	id, ok := o.steps[cmd.Seq]
	o.mu.Unlock()
	if !ok {
		return
	}
	if err := o.s.opts.Store.SetCommandState(context.Background(), id, store.StateSent, 0, nil); err != nil {
		o.s.log.Error("commands: step not marked sent", "step", id, "error", err)
	}
}

func (o *stepObserver) Finished(cmd engine.Command, res engine.Result) {
	o.mu.Lock()
	id, ok := o.steps[cmd.Seq]
	delete(o.steps, cmd.Seq)
	o.mu.Unlock()
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
}

func itoa(n int) string { return strconv.Itoa(n) }

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
			w.Header().Set("Idempotent-Replayed", "true")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(row.Status)
			w.Write(row.Result)
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

// endCommand sets the parent's final state from the answer: 2xx is
// acknowledged, 202 sent (a run that reports later), 4xx and 503
// dropped, the rest failed; the answer is its result.
func (s *Server) endCommand(ctx context.Context, a *auditInfo, status int) {
	state := store.StateFailed
	switch {
	case status == http.StatusAccepted:
		state = store.StateSent
	case status < 300:
		state = store.StateAcknowledged
	case status < 500 || status == http.StatusServiceUnavailable:
		state = store.StateDropped
	}
	result := a.w.answer.Bytes()
	if !json.Valid(result) {
		result, _ = json.Marshal(map[string]any{"answer": strings.TrimSpace(string(result))})
	}
	if err := s.opts.Store.SetCommandState(context.WithoutCancel(ctx), a.commandID, state, status, result); err != nil {
		s.log.Error("commands: row not finished", "command", a.commandUUID, "error", err)
	}
}

// finishCommand ends a parent that was answered 202, from its run: the
// restore's end.
func (s *Server) finishCommand(uuid string, ok bool, result any) {
	st := s.opts.Store
	if st == nil || uuid == "" {
		return
	}
	ctx := context.Background()
	c, found, err := st.CommandByUUID(ctx, uuid)
	if err != nil || !found {
		return
	}
	state := store.StateAcknowledged
	if !ok {
		state = store.StateFailed
	}
	raw, _ := json.Marshal(result)
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
// with its steps; a user without audit sees its own commands only.
func (s *Server) getCommand(w http.ResponseWriter, r *http.Request) error {
	st, err := s.userStore()
	if err != nil {
		return err
	}
	ctx := r.Context()
	id := r.PathValue("id")
	c, ok, err := st.CommandByUUID(ctx, id)
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
	writeJSON(w, http.StatusOK, v)
	return nil
}
