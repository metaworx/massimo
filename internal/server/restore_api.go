// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

// Restore endpoints: a backup file is compared with the Cube and the
// differences are written back, the way massimo restore does it from
// the command line, in two requests. POST …/restore/plan answers what
// would be sent, with the cost and a hash of the plan; POST …/restore
// with that hash sends the steps through the queue at low priority and
// reports each one as a restore_step event; GET …/restore shows how the
// last restore went. The comparison and the commands are the planner's
// (internal/backup); nothing here builds protocol bytes.

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"go.metaworx.ch/massimo/internal/backup"
	"go.metaworx.ch/massimo/internal/cubelan"
	"go.metaworx.ch/massimo/internal/engine"
	"go.metaworx.ch/massimo/internal/protocol/cube"
	"go.metaworx.ch/massimo/internal/store"
)

// Limits and costs of a restore.
const (
	// maxBackupBytes is the largest backup file accepted.
	maxBackupBytes = 1 << 20
	// maxRestoreDutyPct is the duty cycle above which a restore with
	// radio commands is refused; the write test on hardware ran up to
	// there (docs/write-test-2026-11-05.md).
	maxRestoreDutyPct = 80
	// radioCostPct is about what one radio configuration write costs
	// in duty cycle, measured on hardware (docs/handover-session.md).
	radioCostPct = 1.25
	// ntpReadTimeout bounds the read of the Cube's NTP servers for the
	// clock group of a plan.
	ntpReadTimeout = 30 * time.Second
)

// restoreStep is one step of a plan as the API shows it.
type restoreStep struct {
	What string `json:"what"`
	// Radio says the step sends radio commands, which cost budget.
	Radio bool `json:"radio"`
	// Device is the device the command addresses, Room the room woken
	// up before it; absent for Cube commands (names, NTP, time zone).
	Device string `json:"device,omitempty"`
	Room   uint8  `json:"room,omitempty"`
	// Result is set once the step ran (GET …/restore).
	Result *stepResult `json:"result,omitempty"`
}

// stepResult is the outcome of one step.
type stepResult struct {
	OK        bool           `json:"ok"`
	Discarded bool           `json:"discarded,omitempty"`
	Error     string         `json:"error,omitempty"`
	Budget    cubelan.Budget `json:"budget"`
	Time      time.Time      `json:"time"`
}

// restorePlan is the answer of POST …/restore/plan.
type restorePlan struct {
	// Plan identifies the step list; POST …/restore takes it and refuses
	// to send when the plan it computes then differs.
	Plan          string        `json:"plan"`
	Steps         []restoreStep `json:"steps"`
	Notes         []string      `json:"notes"`
	RadioCommands int           `json:"radio_commands"`
	// DutyCostPct is about what the radio commands will cost in duty
	// cycle, radioCostPct each.
	DutyCostPct float64        `json:"duty_cost_pct"`
	Budget      cubelan.Budget `json:"budget"`
}

// restoreRun is one restore: its plan and how far it got. It is also the
// answer of GET …/restore; the fields change under runtime.restoreMu.
type restoreRun struct {
	ID            string        `json:"id"`
	State         string        `json:"state"` // running, done or failed
	Plan          string        `json:"plan"`
	Started       time.Time     `json:"started"`
	Finished      time.Time     `json:"finished,omitzero"`
	Steps         []restoreStep `json:"steps"`
	Sent          int           `json:"sent"` // steps the Cube accepted
	RadioCommands int           `json:"radio_commands"`
	Error         string        `json:"error,omitempty"`
}

// restoreProgress is the payload of the restore_step and restore_done
// events.
type restoreProgress struct {
	ID string `json:"id"`
	// Step counts from 1 (restore_step only), Steps is the plan's total.
	Step      int    `json:"step,omitempty"`
	Steps     int    `json:"steps"`
	What      string `json:"what,omitempty"`
	OK        bool   `json:"ok"`
	Discarded bool   `json:"discarded,omitempty"`
	Error     string `json:"error,omitempty"`
	// State and Sent come with restore_done: done or failed, and the
	// steps the Cube accepted.
	State string `json:"state,omitempty"`
	Sent  int    `json:"sent,omitempty"`
}

// planRestore handles POST …/restore/plan: the backup in the body is
// compared with the Cube and the steps a restore would send are
// answered, nothing is written.
func (s *Server) planRestore(w http.ResponseWriter, r *http.Request) error {
	rt, err := s.cube(r)
	if err != nil {
		return err
	}
	p, _, _, err := s.makeRestorePlan(w, r, rt)
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, p)
	return nil
}

// startRestore handles POST …/restore?plan=<hash>: the plan is computed
// again from the body, and when its hash is the one given, its steps are
// sent in the background.
func (s *Server) startRestore(w http.ResponseWriter, r *http.Request) error {
	rt, err := s.cube(r)
	if err != nil {
		return err
	}
	want := r.URL.Query().Get("plan")
	if want == "" {
		return fail(http.StatusBadRequest, "plan=<hash> from POST …/restore/plan is required")
	}
	if run := rt.restoring(); run != nil {
		return s.refuseRestore(rt, http.StatusConflict, "a restore is already running (%s)", run.ID)
	}
	v, p, src, err := s.makeRestorePlan(w, r, rt)
	if err != nil {
		return err
	}
	if v.Plan != want {
		return s.refuseRestore(rt, http.StatusConflict, "the plan has changed since it was made (%s, now %s); plan again", want, v.Plan)
	}
	if v.RadioCommands > 0 && v.Budget.DutyCyclePct > maxRestoreDutyPct {
		return s.refuseRestore(rt, http.StatusConflict, "radio duty cycle at %d %%, above %d %%; wait for the hourly reset", v.Budget.DutyCyclePct, maxRestoreDutyPct)
	}
	// A Cube with a row in the store keeps its configuration before the
	// restore, and the file the restore was sent (docs/store.md 9 (e)).
	if rt.store != nil && rt.storeID.Load() != 0 {
		user := userIDOf(r)
		if _, _, err := s.takeBackup(r.Context(), rt, store.ReasonBeforeRestore, user, ""); err != nil {
			return s.refuseRestore(rt, http.StatusServiceUnavailable, "the backup before the restore was not stored: %v", err)
		}
		if src.data != nil {
			format := "json"
			if src.yaml {
				format = "yaml"
			}
			if _, err := rt.store.AddBackup(r.Context(), store.Backup{CubeID: rt.storeID.Load(), Reason: store.ReasonUpload, Format: format, Tool: src.file.Tool, UserID: user}, src.data); err != nil {
				s.log.Warn("the file of the restore was not stored", "cube", rt.serial, "error", err)
			}
		}
	}
	run := &restoreRun{ID: restoreID(), State: "running", Plan: v.Plan, Started: time.Now(), Steps: v.Steps, RadioCommands: v.RadioCommands}
	if !rt.startRestore(run) {
		return s.refuseRestore(rt, http.StatusConflict, "a restore is already running")
	}
	s.log.Info("restore started", "cube", rt.cubeSerial(), "id", run.ID, "steps", len(p.Steps), "radio_commands", v.RadioCommands, "duty_pct", v.Budget.DutyCyclePct)
	// Counted in s.wg while Run runs, so that Run waits for the restore's
	// restore_done; before Run (the tests), on the background context.
	ctx := s.track()
	tracked := ctx != nil
	if !tracked {
		ctx = s.runContext()
	}
	go func() {
		if tracked {
			defer s.wg.Done()
		}
		s.runRestore(ctx, rt, run, p, commandOf(r.Context()))
		rt.restoreMu.Lock()
		done := run.State == "done"
		rt.restoreMu.Unlock()
		if done {
			// The file restored is the reference now: the next look says
			// whether the Cube took all of it (backups.go).
			rt.bk.restored(backup.ConfigHash(src.file))
		}
	}()
	writeJSON(w, http.StatusAccepted, restoreAnswer(run.ID, len(p.Steps)))
	return nil
}

// restoreAnswer is the 202 a restore's start answers, which its command
// keeps for a replay.
func restoreAnswer(id string, steps int) map[string]any {
	return map[string]any{"id": id, "steps": steps}
}

// getRestore handles GET …/restore: the last restore of the Cube, or
// {"state": "none"} when there was none since the server started.
func (s *Server) getRestore(w http.ResponseWriter, r *http.Request) error {
	rt, err := s.cube(r)
	if err != nil {
		return err
	}
	run, ok := rt.restoreStatus()
	if !ok {
		writeJSON(w, http.StatusOK, map[string]any{"state": "none"})
		return nil
	}
	writeJSON(w, http.StatusOK, run)
	return nil
}

// refuseRestore logs a refused restore and returns its error.
func (s *Server) refuseRestore(rt *runtime, status int, format string, args ...any) error {
	err := fail(status, format, args...)
	s.log.Warn("restore refused", "cube", rt.cubeSerial(), "reason", err.Error())
	return err
}

// restoreSource is the backup a plan or a restore is made from: a
// stored one (backup=<id>), or the file in the body as it came.
type restoreSource struct {
	file   *backup.File
	data   []byte // the body as sent; nil for a stored backup
	yaml   bool
	stored int64 // the stored backup's id; 0 for a body
}

// readRestoreSource reads the backup of a plan or restore request: the
// stored backup backup=<id> names, else the file in the body.
func (s *Server) readRestoreSource(w http.ResponseWriter, r *http.Request, rt *runtime) (*restoreSource, error) {
	q := r.URL.Query().Get("backup")
	if q == "" {
		f, data, yaml, err := readBackup(w, r)
		if err != nil {
			return nil, err
		}
		return &restoreSource{file: f, data: data, yaml: yaml}, nil
	}
	if rt.store == nil || rt.storeID.Load() == 0 {
		return nil, errNoHistory
	}
	id, err := strconv.ParseInt(q, 10, 64)
	if err != nil || id < 1 {
		return nil, fail(http.StatusBadRequest, "backup=%q is not a backup's id", q)
	}
	cubeID := rt.storeID.Load()
	b, ok, err := rt.store.StoredBackup(r.Context(), cubeID, id)
	if err != nil {
		return nil, fail(http.StatusInternalServerError, "%v", err)
	}
	if !ok {
		return nil, fail(http.StatusNotFound, "no backup %d of cube %s", id, rt.serial)
	}
	body, ok, err := rt.store.BackupBody(r.Context(), cubeID, id)
	if err != nil || !ok {
		return nil, fail(http.StatusInternalServerError, "backup %d: %v", id, err)
	}
	f, err := backup.Unmarshal(body, b.Format == "yaml")
	if err != nil {
		return nil, fail(http.StatusInternalServerError, "backup %d: %v", id, err)
	}
	return &restoreSource{file: f, yaml: b.Format == "yaml", stored: id}, nil
}

// makeRestorePlan reads the backup and the options of a plan or restore
// request and computes the plan, with the checks both requests share:
// the Cube is connected, the backup is this Cube's, the file and the
// options are good.
func (s *Server) makeRestorePlan(w http.ResponseWriter, r *http.Request, rt *runtime) (restorePlan, *backup.Plan, *restoreSource, error) {
	opts, err := restoreOptions(r.URL.Query())
	if err != nil {
		return restorePlan{}, nil, nil, err
	}
	src, err := s.readRestoreSource(w, r, rt)
	if err != nil {
		return restorePlan{}, nil, nil, err
	}
	f := src.file
	snap := rt.client.Snapshot()
	h := snap.Hello()
	if h == nil || !rt.client.Connected() {
		return restorePlan{}, nil, nil, fail(http.StatusServiceUnavailable, "cube not connected")
	}
	if f.Cube.Serial == "" {
		return restorePlan{}, nil, nil, s.refuseRestore(rt, http.StatusBadRequest, "the backup names no cube; this is cube %s", h.Serial)
	}
	if !strings.EqualFold(f.Cube.Serial, h.Serial) {
		return restorePlan{}, nil, nil, s.refuseRestore(rt, http.StatusBadRequest, "the backup is from cube %s, this is cube %s", f.Cube.Serial, h.Serial)
	}
	opts.Hidden, opts.ShowHidden = rt.config().Hidden, rt.showHidden
	cur := backup.Current{Snapshot: snap}
	var ntpErr error
	if !opts.SkipClock {
		cur.NTP, ntpErr = s.readNTP(r.Context(), rt)
	}
	p, err := backup.NewPlan(f, cur, opts)
	if err != nil {
		return restorePlan{}, nil, nil, fail(http.StatusBadRequest, "bad backup file: %v", err)
	}
	if ntpErr != nil {
		p.Notes = append(p.Notes, "the cube's NTP servers were not read ("+ntpErr.Error()+"); they are not compared")
	}
	return planView(p, rt.client.Budget()), p, src, nil
}

// readNTP asks the Cube for its NTP servers through the queue, ahead of
// the queued work, so that a plan is not held up by a running restore.
func (s *Server) readNTP(ctx context.Context, rt *runtime) ([]string, error) {
	ctx, cancel := context.WithTimeout(ctx, ntpReadTimeout)
	defer cancel()
	res := rt.enqueue(ctx, engine.Command{Priority: engine.PriorityHigh, Lines: []cube.Message{cube.CmdGetNTP}}).Wait(ctx)
	if res.Err != nil {
		return nil, res.Err
	}
	if len(res.Replies) != 1 || res.Replies[0].Type != 'F' {
		return nil, errors.New("no F reply")
	}
	return cube.ParseNTPServers(res.Replies[0].Data), nil
}

// readBackup reads the backup file from the body: JSON, or YAML when
// the Content-Type says so or the body does not start with a brace.
func readBackup(w http.ResponseWriter, r *http.Request) (*backup.File, []byte, bool, error) {
	r.Body = http.MaxBytesReader(w, r.Body, maxBackupBytes)
	data, err := io.ReadAll(r.Body)
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			return nil, nil, false, fail(http.StatusRequestEntityTooLarge, "backup larger than %d bytes", maxBackupBytes)
		}
		return nil, nil, false, fail(http.StatusBadRequest, "reading the body: %v", err)
	}
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return nil, nil, false, fail(http.StatusBadRequest, "the body must be a backup file, JSON or YAML")
	}
	yaml := isYAML(r.Header.Get("Content-Type")) || trimmed[0] != '{'
	f, err := backup.Unmarshal(data, yaml)
	if err != nil {
		return nil, nil, false, fail(http.StatusBadRequest, "bad backup file: %v", err)
	}
	return f, data, yaml, nil
}

// isYAML reports whether a Content-Type names YAML.
func isYAML(contentType string) bool {
	mt, _, _ := mime.ParseMediaType(contentType)
	switch mt {
	case "application/yaml", "application/x-yaml", "text/yaml", "text/x-yaml":
		return true
	}
	return false
}

// restoreOptions reads the plan options from the query: portal and
// state as flags, skip as a comma-separated list of groups, the same
// as the flags of massimo restore.
func restoreOptions(q url.Values) (backup.PlanOptions, error) {
	var opts backup.PlanOptions
	var err error
	if opts.Portal, err = queryFlag(q, "portal"); err != nil {
		return opts, err
	}
	if opts.State, err = queryFlag(q, "state"); err != nil {
		return opts, err
	}
	for _, g := range strings.Split(q.Get("skip"), ",") {
		switch strings.TrimSpace(g) {
		case "":
		case "names":
			opts.SkipNames = true
		case "settings":
			opts.SkipSettings = true
		case "programs":
			opts.SkipPrograms = true
		case "clock":
			opts.SkipClock = true
		default:
			return opts, fail(http.StatusBadRequest, "unknown skip group %q; names, settings, programs or clock", g)
		}
	}
	return opts, nil
}

// queryFlag reads a boolean query parameter: absent is false, present
// without a value is true, otherwise true, false, 1 or 0.
func queryFlag(q url.Values, name string) (bool, error) {
	if !q.Has(name) {
		return false, nil
	}
	v := q.Get(name)
	if v == "" {
		return true, nil
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return false, fail(http.StatusBadRequest, "%s must be true or false", name)
	}
	return b, nil
}

// planView turns a plan into its API answer.
func planView(p *backup.Plan, b cubelan.Budget) restorePlan {
	v := restorePlan{Plan: planHash(p), Steps: []restoreStep{}, Notes: p.Notes, Budget: b}
	if v.Notes == nil {
		v.Notes = []string{}
	}
	for _, st := range p.Steps {
		n := radioLines(st.Command)
		v.RadioCommands += n
		s := restoreStep{What: st.What, Radio: n > 0}
		if st.Command.Target != 0 {
			s.Device = st.Command.Target.String()
		}
		if st.Command.Wakeup {
			s.Room = st.Command.RoomID
		}
		v.Steps = append(v.Steps, s)
	}
	v.DutyCostPct = float64(v.RadioCommands) * radioCostPct
	return v
}

// radioLines counts the radio commands (s: lines) of a command.
func radioLines(cmd engine.Command) int {
	n := 0
	for _, l := range cmd.Lines {
		if l.Type == 's' {
			n++
		}
	}
	return n
}

// planHash identifies a plan by its steps: the texts and the command
// lines, so that a plan made from the same backup against an unchanged
// Cube hashes the same and any change to what would be sent shows.
func planHash(p *backup.Plan) string {
	h := sha256.New()
	for _, st := range p.Steps {
		c := st.Command
		fmt.Fprintf(h, "%s\n%s %d %v\n", st.What, c.Target, c.RoomID, c.Wakeup)
		for _, l := range c.Lines {
			fmt.Fprintln(h, l.String())
		}
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))
}

// restoreID returns a random identifier for a restore.
func restoreID() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return strconv.FormatInt(time.Now().UnixNano(), 16)
	}
	return hex.EncodeToString(b[:])
}

// runContext is the server's run context, for work that outlives a
// request; Background before Run.
func (s *Server) runContext() context.Context {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.ctx == nil {
		return context.Background()
	}
	return s.ctx
}

// restoring returns the Cube's running restore, or nil.
func (rt *runtime) restoring() *restoreRun {
	rt.restoreMu.Lock()
	defer rt.restoreMu.Unlock()
	if rt.restore != nil && rt.restore.State == "running" {
		return rt.restore
	}
	return nil
}

// startRestore makes run the Cube's restore unless one is running.
func (rt *runtime) startRestore(run *restoreRun) bool {
	rt.restoreMu.Lock()
	defer rt.restoreMu.Unlock()
	if rt.restore != nil && rt.restore.State == "running" {
		return false
	}
	rt.restore = run
	return true
}

// restoreStatus returns a copy of the Cube's last restore; false when
// there was none.
func (rt *runtime) restoreStatus() (restoreRun, bool) {
	rt.restoreMu.Lock()
	defer rt.restoreMu.Unlock()
	if rt.restore == nil {
		return restoreRun{}, false
	}
	cp := *rt.restore
	cp.Steps = append([]restoreStep(nil), rt.restore.Steps...)
	return cp, true
}

// runRestore sends the plan's steps one after the other and reports
// each as a restore_step event, then restore_done; it stops at the
// first failure, as the command does. Devices whose configuration was
// written are followed up like after any configuration write.
func (s *Server) runRestore(ctx context.Context, rt *runtime, run *restoreRun, p *backup.Plan, command string) {
	var configured []cube.RFAddress
	radio := false
	w := rt.writer()
	w.ID = command // every step is a step of the request's command
	_, err := p.Run(ctx, w, func(i int, st backup.Step, r engine.Result) {
		res := &stepResult{OK: r.Err == nil, Discarded: r.Discarded, Budget: r.Budget, Time: time.Now()}
		if r.Err != nil {
			res.Error = r.Err.Error()
		}
		rt.restoreMu.Lock()
		run.Steps[i].Result = res
		if res.OK {
			run.Sent++
		}
		rt.restoreMu.Unlock()
		ev := event{Kind: "restore_step", Error: res.Error, Time: res.Time, Restore: &restoreProgress{ID: run.ID, Step: i + 1, Steps: len(p.Steps),
			What: st.What, OK: res.OK, Discarded: r.Discarded, Error: res.Error}}
		if st.Command.Target != 0 {
			ev.Device = st.Command.Target.String()
		}
		rt.emit(ev)
		if res.OK && radioLines(st.Command) > 0 {
			radio = true
			if st.Command.Target != 0 && st.Command.Wakeup {
				configured = append(configured, st.Command.Target)
			}
		}
		if r.Discarded {
			s.log.Warn("restore step discarded by the cube", "cube", rt.cubeSerial(), "id", run.ID, "step", i+1, "what", st.What,
				"duty_pct", r.Budget.DutyCyclePct, "free_slots", r.Budget.FreeMemorySlots)
		}
	})
	if radio {
		rt.afterWrite(configured...)
	}
	finished := time.Now()
	rt.restoreMu.Lock()
	run.Finished = finished
	run.State = "done"
	if err != nil {
		run.State, run.Error = "failed", err.Error()
	}
	done := restoreProgress{ID: run.ID, Steps: len(run.Steps), OK: err == nil, Error: run.Error, State: run.State, Sent: run.Sent}
	rt.restoreMu.Unlock()
	rt.emit(event{Kind: "restore_done", Error: done.Error, Time: finished, Restore: &done})
	s.finishCommand(command, err == nil, map[string]any{"id": run.ID, "state": done.State, "sent": done.Sent, "steps": done.Steps, "error": done.Error},
		restoreAnswer(run.ID, len(p.Steps)))
	if err != nil {
		s.log.Warn("restore failed", "cube", rt.cubeSerial(), "id", run.ID, "sent", done.Sent, "steps", done.Steps, "error", err)
		return
	}
	s.log.Info("restore done", "cube", rt.cubeSerial(), "id", run.ID, "sent", done.Sent, "steps", done.Steps)
}
