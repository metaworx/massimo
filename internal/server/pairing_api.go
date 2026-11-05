// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

// Pairing endpoints: inclusion mode on a Cube, its state, and the end of
// it. A device that joins appears in the device list as unlisted; PUT
// /devices/{rf} with a room_id puts it into a room (config_api.go).

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"go.metaworx.ch/massimo/internal/cubeio"
	"go.metaworx.ch/massimo/internal/cubelan"
)

// defaultPairingTime is the inclusion time when the request names none,
// the original software's.
const defaultPairingTime = 60 * time.Second

// startPairing handles POST /pairing with an optional {"seconds": N}.
func (s *Server) startPairing(w http.ResponseWriter, r *http.Request) error {
	rt, err := s.cube(r)
	if err != nil {
		return err
	}
	var req struct {
		Seconds *int `json:"seconds"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
		return fail(http.StatusBadRequest, "bad body: %v", err)
	}
	timeout := defaultPairingTime
	if req.Seconds != nil {
		timeout = time.Duration(*req.Seconds) * time.Second
		if timeout < time.Second || timeout > cubeio.MaxPairingTime {
			return fail(http.StatusBadRequest, "seconds must be 1..%d", int(cubeio.MaxPairingTime.Seconds()))
		}
	}
	if !rt.client.Connected() {
		return fail(http.StatusServiceUnavailable, "cube not connected")
	}
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Minute)
	defer cancel()
	if err := rt.writer().StartPairing(ctx, timeout); err != nil {
		if errors.Is(err, cubelan.ErrInclusionActive) {
			return fail(http.StatusConflict, "pairing is already running")
		}
		return fail(http.StatusBadGateway, "%v", err)
	}
	writeJSON(w, http.StatusOK, rt.client.Inclusion())
	return nil
}

// getPairing handles GET /pairing.
func (s *Server) getPairing(w http.ResponseWriter, r *http.Request) error {
	rt, err := s.cube(r)
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, rt.client.Inclusion())
	return nil
}

// stopPairing handles DELETE /pairing: x: to the Cube, also when the
// server thinks the mode is over, so that a Cube still in it stops.
func (s *Server) stopPairing(w http.ResponseWriter, r *http.Request) error {
	rt, err := s.cube(r)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Minute)
	defer cancel()
	if err := rt.writer().StopPairing(ctx); err != nil {
		return fail(http.StatusBadGateway, "%v", err)
	}
	writeJSON(w, http.StatusOK, rt.client.Inclusion())
	return nil
}
