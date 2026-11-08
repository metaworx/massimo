// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

// What massimo store info and massimo config show, for a holder of the
// capability settings (System settings → Maintenance): GET /api/v1/store
// the store's file, schema, identities and rows; GET /api/v1/config the
// settings in effect, which cmd/massimo gathers (SetConfigView) since it
// knows the flags and the settings file. A secret's value never leaves
// the server: a key, an IV, a token or a password says only that it is
// set.

import (
	"net/http"
	"strings"
)

// storeView is GET /api/v1/store's answer (store.Info).
type storeView struct {
	File      string         `json:"file"`
	Schema    int            `json:"schema"`
	MinReader int            `json:"min_reader"`
	Binary    int            `json:"binary"`
	Node      string         `json:"node"`
	Tenant    string         `json:"tenant"`
	Rows      map[string]int `json:"rows"`
}

// ConfigView is the settings in effect, as massimo config shows them.
type ConfigView struct {
	Version   string           `json:"version"`
	File      string           `json:"file"`
	FileFound bool             `json:"file_found"`
	DataDir   string           `json:"data_dir"`
	Listen    string           `json:"listen"`
	TLS       string           `json:"tls"`
	LogLevel  string           `json:"log_level"`
	Variables []ConfigVariable `json:"variables"`
}

// ConfigVariable is a MASSIMO_* variable in effect: its value, empty
// for a secret, and where it came from (the environment, the settings
// file).
type ConfigVariable struct {
	Name    string `json:"name"`
	Value   string `json:"value,omitempty"`
	Secret  bool   `json:"secret,omitempty"`
	Source  string `json:"source"`
	Unknown bool   `json:"unknown,omitempty"`
}

// SecretVariable reports whether a variable's value is a secret, which
// GET /api/v1/config does not show: a key, an IV, a token, a password.
func SecretVariable(name string) bool {
	return strings.HasSuffix(name, "_KEY") || strings.HasSuffix(name, "_IV") || strings.HasSuffix(name, "_TOKEN") ||
		strings.Contains(name, "PASSWORD") || strings.Contains(name, "SECRET")
}

// SetConfigView hands the server what gathers the settings in effect
// (cmd/massimo); without one GET /api/v1/config answers 503.
func (s *Server) SetConfigView(view func() ConfigView) {
	s.pendingMu.Lock()
	s.configView = view
	s.pendingMu.Unlock()
}

func (s *Server) getStore(w http.ResponseWriter, r *http.Request) error {
	if s.opts.Store == nil {
		return fail(http.StatusServiceUnavailable, "this server has no store")
	}
	in, err := s.opts.Store.Info(r.Context())
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, storeView{File: in.Path, Schema: in.Schema, MinReader: in.MinReader, Binary: in.Binary,
		Node: in.NodeID, Tenant: in.TenantID, Rows: in.Counts})
	return nil
}

func (s *Server) getConfig(w http.ResponseWriter, r *http.Request) error {
	s.pendingMu.Lock()
	view := s.configView
	s.pendingMu.Unlock()
	if view == nil {
		return fail(http.StatusServiceUnavailable, "this server does not show its settings")
	}
	v := view()
	v.Version = s.opts.Version
	// whatever gathered it, no secret leaves the server
	for i := range v.Variables {
		if SecretVariable(v.Variables[i].Name) {
			v.Variables[i].Value, v.Variables[i].Secret = "", true
		}
	}
	if v.Variables == nil {
		v.Variables = []ConfigVariable{}
	}
	writeJSON(w, http.StatusOK, v)
	return nil
}
