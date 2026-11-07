// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

// What the server keeps in the store (internal/store): the found Cubes,
// which survive a restart, the plain MASSIMO_CUBE_ADDRESS Cube once it
// has reported its serial, and the settings file's lines that differ
// from the store, shown on the Cube.

import (
	"context"
	"strings"

	"go.metaworx.ch/massimo/internal/store"
)

// fileDifference is a settings file line whose value differs from the
// store's, which wins: the field as the Cube object names it, the
// variable, and both values (a hidden list as a list).
type fileDifference struct {
	Field    string `json:"field"`
	Variable string `json:"variable"`
	File     any    `json:"file"`
	Store    any    `json:"store"`
}

// loadFound reads what discovery found before the restart, so that the
// found Cubes are listed again and announced only once.
func (s *Server) loadFound() {
	s.fileCubes = map[string]store.FileCube{}
	s.diffs = map[string][]fileDifference{}
	for k, v := range s.opts.FileCubes {
		s.fileCubes[strings.ToUpper(k)] = v
	}
	if s.opts.Store == nil {
		return
	}
	rows, err := s.opts.Store.Cubes(context.Background())
	if err != nil {
		s.log.Warn("store: found cubes not read", "error", err)
		return
	}
	for _, c := range rows {
		s.noteRow(c)
		if c.LastSeen.IsZero() {
			continue // never answered discovery
		}
		s.found[c.Serial] = &foundCube{Serial: c.Serial, Address: c.DiscoveredAddress, MAC: c.MAC, RFAddress: c.RFAddress, Firmware: c.Firmware,
			Configured: c.Configured, FoundAt: c.FirstSeen, LastSeen: c.LastSeen}
	}
}

// seedLearned adds the plain MASSIMO_CUBE_ADDRESS Cube to the store once
// it has reported its serial, with the file's values (FileCubes[""]),
// and remembers which serial the plain lines belong to.
func (s *Server) seedLearned(ctx context.Context, rt *runtime) {
	if !rt.config().SeedFromFile || s.opts.Store == nil {
		return
	}
	s.fileMu.Lock()
	fc, ok := s.fileCubes[""]
	if ok {
		fc.Serial = strings.ToUpper(rt.cubeSerial())
		delete(s.fileCubes, "")
		s.fileCubes[fc.Serial] = fc
	}
	s.fileMu.Unlock()
	if !ok {
		return
	}
	if _, err := s.opts.Store.Seed(ctx, s.opts.SettingsFile, []store.FileCube{fc}); err != nil {
		s.log.Warn("store: cube not added", "serial", fc.Serial, "error", err)
		return
	}
	if row, ok, err := s.opts.Store.Cube(ctx, fc.Serial); err == nil && ok {
		s.noteRow(row)
	}
	if err := s.opts.Store.SetSetting(ctx, store.PlainSerialKey, fc.Serial); err != nil {
		s.log.Warn("store: cube not added", "serial", fc.Serial, "error", err)
	}
}

// noteRow computes the settings file's differences for a Cube row: at
// the start, and whenever the row changes through the server, so that
// listing the Cubes asks the store nothing.
func (s *Server) noteRow(row store.Cube) {
	s.fileMu.Lock()
	defer s.fileMu.Unlock()
	fc, ok := s.fileCubes[strings.ToUpper(row.Serial)]
	if !ok {
		delete(s.diffs, row.Serial)
		return
	}
	var out []fileDifference
	for _, d := range store.Differences(row, fc) {
		fd := fileDifference{Field: d.Field, Variable: d.Variable, File: d.File, Store: d.Store}
		if d.Field == store.FieldHidden {
			fd.File, fd.Store = store.SplitHidden(d.File), row.Hidden
		}
		out = append(out, fd)
	}
	s.diffs[row.Serial] = out
}

// fileDiffers lists the settings file's lines for a Cube that differ
// from its row in the store, as noteRow last computed them.
func (s *Server) fileDiffers(serial string) []fileDifference {
	s.fileMu.Lock()
	defer s.fileMu.Unlock()
	return s.diffs[strings.ToUpper(serial)]
}
