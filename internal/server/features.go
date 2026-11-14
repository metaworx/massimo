// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

// The features of GET /healthz (docs/api-v1.md, "GET /healthz"): what the
// running server can do, by name and version "MAJOR.MINOR", so that a
// client of a build without a release version knows what it may use. A
// minor rises with a compatible addition; under /api/v1 the major stays
// 1. A name never changes meaning, and a feature that goes is taken out
// of the map, never redefined. The map is built from this server's
// options, not its build: without a store the routes and headers that
// need one are not there, and their features are not listed. What a
// key may do stays in /api/v1/me.

// featuresOf is the map GET /healthz answers with.
func (s *Server) featuresOf() map[string]string {
	f := map[string]string{
		"event_ids":    "1.0", // the stream's ids, resume, resync
		"vwt":          "1.0", // virtual wall thermostats, ?devices=, room_uid, temp_method
		"program_etag": "1.0", // the programme's tag, If-Match, 412, pending, changed and etag
		"boost":        "1.0", // {"mode": "boost"} on the setpoint routes
	}
	if s.opts.Store != nil {
		f["idempotency"] = "1.0"   // Idempotency-Key, kept with the command
		f["commands"] = "1.0"      // GET /api/v1/commands/{id}, command ids in answers
		f["proxy_virtual"] = "1.0" // the Cube option proxy_virtual, kept in the store
		f["integration"] = "1.0"   // registered integrations acting for their users, X-MassimoHA-On-Behalf-Of
	}
	return f
}
