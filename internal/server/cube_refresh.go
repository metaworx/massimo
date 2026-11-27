// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"go.metaworx.ch/massimo/internal/cubelan"
	"go.metaworx.ch/massimo/internal/engine"
)

// The refresh of a Cube asked for (POST /api/v1/cubes/{sn}/refresh, AP
// RefreshBatteries, Block 1): the reconnect of the periodic refresh
// (refresh, server.go) at once, through the queue at normal priority so
// that no command is cut off and a user's command still goes first. The
// Cube sends its whole greeting again, which replaces the snapshot: its
// device list with the batteries' flags, its budget, its rooms. It sends
// no radiogram and holds no memory slot.

// refreshTimeout bounds the wait for an asked refresh: the queue bounds
// the reconnect itself to a minute, and a command before it may run.
const refreshTimeout = 2 * time.Minute

// refreshCube handles POST /api/v1/cubes/{sn}/refresh: 409 while the
// Cube's serial or address is not known yet or it reboots, else the
// reconnect, and 200 with the budget of the new greeting once it has
// replaced the snapshot; the reconnect's error is a 502, a newer refresh
// that took its place before its turn a 409.
func (s *Server) refreshCube(w http.ResponseWriter, r *http.Request) error {
	rt, err := s.cube(r)
	if err != nil {
		return err
	}
	serial := rt.cubeSerial()
	if serial == "" {
		return refuse(http.StatusConflict, codeCubeSerialNotKnownYet)
	}
	if rt.client.Address() == "" {
		return refuse(http.StatusConflict, codeCubeAddressNotKnownYet)
	}
	if rt.client.Link().State == cubelan.LinkRebooting {
		return refuse(http.StatusConflict, codeCubeRebooting)
	}
	ctx, cancel := context.WithTimeout(r.Context(), refreshTimeout)
	defer cancel()
	res := rt.enqueue(ctx, engine.Command{Key: "refresh", Priority: engine.PriorityNormal, Reconnect: true}).Wait(ctx)
	asked := []any{"cube", serial, "asked", true}
	if p := principalOf(r); p.named() {
		asked = append(asked, "user", p.user.Name)
	}
	if res.Err != nil {
		s.log.Warn("refresh failed", append(asked, "error", res.Err)...)
		status := http.StatusBadGateway
		if errors.Is(res.Err, engine.ErrSuperseded) {
			status = http.StatusConflict
		}
		reason := res.Err.Error()
		if l := rt.client.Link(); l.State != cubelan.LinkConnected {
			reason = fmt.Sprintf("%v (link %s)", res.Err, l.State)
		}
		msg := fmt.Sprintf("refresh of %s: %s", serial, reason)
		if status == http.StatusConflict {
			return refuseSaying(status, msg, codeCommandSuperseded)
		}
		return refuseSaying(status, msg, codeCubeRefreshFailed, serial, reason)
	}
	s.log.Info("snapshot refreshed", asked...)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "budget": res.Budget})
	return nil
}
