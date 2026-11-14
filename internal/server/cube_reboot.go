// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"strconv"
	"time"

	"go.metaworx.ch/massimo/internal/cubelan"
	"go.metaworx.ch/massimo/internal/view"
)

// The reboot of a Cube (POST /api/v1/cubes/{sn}/reboot): the UDP R that
// restarts it, which frees a Cube that still holds an old connection
// (cubelan.LinkStale) at once instead of after up to about 40 minutes.
// A restart drops the radiograms the Cube has not delivered, so the
// request names the count it accepts, and the server never sends R by
// itself (AP LinkOutage, D5-A): a Cube that refuses the server may hold
// another client's connection (cubelan.LinkBusy), which a reboot would
// knock off.

// rebootCube handles POST /api/v1/cubes/{sn}/reboot, {"pending": n}:
// 409 while more radiograms are pending than n, as far as the Cube last
// said, else R to the Cube and 202 with whether it answered.
func (s *Server) rebootCube(w http.ResponseWriter, r *http.Request) error {
	rt, err := s.cube(r)
	if err != nil {
		return err
	}
	var req struct {
		Pending *int `json:"pending"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
		return fail(http.StatusBadRequest, "bad body: %v", err)
	}
	b := rt.client.Budget()
	if n := view.Pending(rt.client.Snapshot(), b); n > 0 && (req.Pending == nil || *req.Pending < n) {
		at := "never"
		if !b.UpdatedAt.IsZero() {
			at = b.UpdatedAt.Format(time.RFC3339)
		}
		return fail(http.StatusConflict, "%d radiograms pending as the Cube last said (%s): a reboot drops them; send {\"pending\": %d} to reboot all the same", n, at, n)
	}
	serial := rt.cubeSerial()
	if serial == "" {
		return fail(http.StatusConflict, "the Cube's serial is not known yet")
	}
	to := s.rebootTarget(rt)
	if to == "" {
		return fail(http.StatusConflict, "the Cube's address is not known yet")
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	res, err := s.sendReboot(ctx, to, serial)
	if err != nil {
		return fail(http.StatusBadGateway, "reboot of %s: %v", serial, err)
	}
	rt.client.Rebooting(cubelan.RebootWindow)
	out := map[string]any{"sent": true, "answered": res.Answered, "status": nil}
	if res.Answered {
		out["status"] = res.Status
	}
	writeJSON(w, http.StatusAccepted, out)
	return nil
}

// rebootTarget is where a Cube's R goes: the discovery target at the
// Cube's IP, where discovery is pointed at targets (a fake Cube's port in
// the tests), else the Cube's IP and port 23272; "" while the Cube has
// no address.
func (s *Server) rebootTarget(rt *runtime) string {
	host, _, err := net.SplitHostPort(rt.client.Address())
	if err != nil || host == "" {
		return ""
	}
	for _, t := range s.discoveryTargets() {
		if t.IP.String() == host {
			return t.String()
		}
	}
	return net.JoinHostPort(host, strconv.Itoa(cubelan.DiscoveryPort))
}

// sendReboot sends R from the server's discovery socket, where the
// Cube's answer arrives, or from a socket of its own on the discovery
// port; where that port is another program's, from any port, the R
// going out all the same and its answer elsewhere.
func (s *Server) sendReboot(ctx context.Context, to, serial string) (cubelan.RebootResult, error) {
	s.mu.RLock()
	resp := s.responder
	s.mu.RUnlock()
	if resp != nil {
		return resp.Reboot(ctx, to, serial)
	}
	pc, err := net.ListenPacket("udp4", s.discoveryAddr())
	if err != nil {
		if pc, err = net.ListenPacket("udp4", ":0"); err != nil {
			return cubelan.RebootResult{}, err
		}
	}
	defer pc.Close()
	return cubelan.Reboot(ctx, pc, nil, to, serial)
}
