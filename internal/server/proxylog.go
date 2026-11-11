// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

// The LAN proxy's clients in the log (AP OneLog v1.1, Block 7): a line
// a client passes on to the Cube is a command of the store, as a request
// of the API is, with no user and the client's address in its request;
// the queue's steps beneath it and their lines name it. A client's
// connection, its going, its refusal, its deletes and the proxy's
// warnings are lines of the Cube's log, source proxy. A device's state
// line after a command was sent to it names that command too.

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"time"

	"go.metaworx.ch/massimo/internal/engine"
	"go.metaworx.ch/massimo/internal/protocol/cube"
	"go.metaworx.ch/massimo/internal/proxy"
	"go.metaworx.ch/massimo/internal/store"
)

// stateCommandFor is how long after a command was sent to a device the
// device's next state line names that command.
const stateCommandFor = 2 * time.Minute

// proxyActions are the actions of the commands a proxy's client passes
// on, by the line's letter; another letter is "proxy." and the letter.
var proxyActions = map[byte]string{'s': "proxy.radio", 't': "proxy.delete", 'm': "proxy.metadata", 'c': "proxy.config", 'g': "proxy.walls",
	'z': "proxy.wakeup", 'a': "proxy.reset", 'u': "proxy.url"}

// proxyEvent keeps a proxy's event as a line of the Cube's log.
func (rt *runtime) proxyEvent(pe proxy.Event) {
	summary := map[string]any{}
	for k, v := range pe.Summary {
		summary[k] = v
	}
	if pe.Remote != "" {
		summary["remote"] = pe.Remote
	}
	ev := event{Kind: pe.Kind, Source: "proxy", Level: store.LevelName(int(pe.Level)), Time: time.Now(), Summary: summary}
	if pe.Err != nil {
		ev.Error = pe.Err.Error()
	}
	rt.emit(ev)
}

// proxyCommand is the proxy's Command for the Cube: a commands row for
// the client's line, before it is queued, and its outcome after; none
// for a Cube without a row in the store.
func (s *Server) proxyCommand(rt *runtime) func(remote string, m cube.Message, target cube.RFAddress) (string, func(engine.Result)) {
	return func(remote string, m cube.Message, target cube.RFAddress) (string, func(engine.Result)) {
		st, cubeID := rt.store, rt.storeID.Load()
		if st == nil || cubeID == 0 || st.ReadOnly() {
			return "", nil
		}
		action, ok := proxyActions[m.Type]
		if !ok {
			action = "proxy." + string(m.Type)
		}
		c := store.Command{CubeID: cubeID, Action: action}
		if target != 0 {
			c.Target = "device:" + target.String()
		}
		c.Request, _ = json.Marshal(map[string]any{"remote": remote, "line": string(m.Type)})
		ctx := context.Background()
		row, err := st.AddCommand(ctx, c)
		if err != nil {
			s.log.Error("commands: a proxy client's row not written", "action", action, "error", err)
			return "", nil
		}
		return row.UUID, func(res engine.Result) {
			state, result := store.StateAcknowledged, map[string]any{"discarded": res.Discarded}
			switch {
			case res.Discarded:
				state = store.StateDropped
			case res.Err != nil && (errors.Is(res.Err, context.Canceled) || errors.Is(res.Err, context.DeadlineExceeded)):
				state, result["error"] = store.StateDropped, res.Err.Error()
			case res.Err != nil:
				state, result["error"] = store.StateFailed, res.Err.Error()
			}
			raw, _ := json.Marshal(result)
			// The steps' rows first, so that the command is finished
			// after them, as a request's is.
			s.steps.flush(2 * time.Second)
			if err := st.SetCommandState(context.WithoutCancel(ctx), row.ID, state, 0, raw); err != nil {
				s.log.Error("commands: a proxy client's row not finished", "command", row.UUID, "error", err)
			}
		}
	}
}

// stateCommands remember, per device, the command last sent to it, so
// that the device's next state line names it.
type stateCommands struct {
	mu sync.Mutex
	by map[cube.RFAddress]sentCommand
}

// sentCommand is a command sent to a device, and when.
type sentCommand struct {
	id int64
	at time.Time
}

// sent notes that the command id was sent to rf.
func (sc *stateCommands) sent(rf cube.RFAddress, id int64) {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	if sc.by == nil {
		sc.by = map[cube.RFAddress]sentCommand{}
	}
	sc.by[rf] = sentCommand{id: id, at: time.Now()}
}

// forget drops what was noted for rf: a command that failed or was
// dropped changes no state.
func (sc *stateCommands) forget(rf cube.RFAddress, id int64) {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	if c, ok := sc.by[rf]; ok && c.id == id {
		delete(sc.by, rf)
	}
}

// take returns the command last sent to rf within stateCommandFor, once.
func (sc *stateCommands) take(rf cube.RFAddress) int64 {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	c, ok := sc.by[rf]
	if !ok {
		return 0
	}
	delete(sc.by, rf)
	if time.Since(c.at) > stateCommandFor {
		return 0
	}
	return c.id
}
