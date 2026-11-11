// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package store

// Seeding and precedence (docs/store.md section 4): at every start the
// server hands the settings file's Cube values to Seed. A Cube the store
// does not know is added from them (source 'env'), which is the first
// start's seeding and the way to add a Cube by hand later; a Cube the
// store knows is served from its row, and a file line that differs is
// reported, the store winning. A Cube discovery found earlier counts as
// unknown: its row takes the file's values and becomes configured. A
// Cube removed through the API stays removed.

import (
	"context"
	"sort"
	"strings"
)

// The Cube fields the settings file can hold, as FileCube.Values keys
// and in Difference.Field; they are the names of the API's Cube object.
const (
	FieldName         = "name"
	FieldAddress      = "address"
	FieldProxyAddress = "proxy_address"
	FieldProxySerial  = "proxy_serial"
	FieldHidden       = "hidden"
)

// Fields lists the fields of the settings file in the order they are
// reported.
var Fields = []string{FieldName, FieldAddress, FieldProxyAddress, FieldProxySerial, FieldHidden}

// FileValue is one line of the settings file: the variable and its
// value. A hidden list is its radio addresses joined by commas.
type FileValue struct {
	Variable string
	Value    string
}

// FileCube is what the settings file says about one Cube: a value per
// field that has a line.
type FileCube struct {
	Serial string
	Values map[string]FileValue
}

// Difference is a file line whose value differs from the store's.
type Difference struct {
	Serial   string
	Field    string
	Variable string
	File     string
	Store    string
}

// SeedResult says what Seed did.
type SeedResult struct {
	Added       []string // serials added (or turned from found into configured) from the file
	Differences []Difference
}

// Seed applies the settings file's Cubes to the store, as the package
// comment says, and logs each Cube added and each difference; file
// names the settings file for the log.
func (s *Store) Seed(ctx context.Context, file string, cubes []FileCube) (SeedResult, error) {
	var res SeedResult
	for _, fc := range cubes {
		serial := NormalSerial(fc.Serial)
		if serial == "" {
			continue
		}
		row, ok, err := s.Cube(ctx, serial)
		if err != nil {
			return res, err
		}
		if !ok || !row.Configured && row.Source == SourceDiscovery {
			if ok {
				_, err = s.UpdateCube(ctx, serial, func(c *Cube) error { fromFile(c, fc); return nil })
			} else {
				row = Cube{Serial: serial}
				fromFile(&row, fc)
				_, err = s.AddCube(ctx, row)
			}
			if err != nil {
				return res, err
			}
			res.Added = append(res.Added, serial)
			s.log.Info("cube added from the settings file", "serial", serial, "file", file)
			continue
		}
		for _, d := range Differences(row, fc) {
			res.Differences = append(res.Differences, d)
			s.log.Warn("settings file differs from the store, the store wins", "serial", serial, "variable", d.Variable, "file", d.File, "store", d.Store)
		}
	}
	return res, nil
}

// PlainSerialKey is the setting that remembers the serial of the Cube
// the plain MASSIMO_CUBE_ADDRESS names, learned when that Cube first
// reported it, so that the plain lines are compared with its row.
const PlainSerialKey = "seed.plain_serial"

// PlainSerial returns the serial PlainSerialKey holds, "" when none.
func (s *Store) PlainSerial(ctx context.Context) (string, error) {
	return s.SettingString(ctx, PlainSerialKey)
}

// fromFile sets a row's configuration from the file: configured, from
// the file, and following discovery when the file gives no address.
func fromFile(c *Cube, fc FileCube) {
	v := func(f string) string { return fc.Values[f].Value }
	c.Configured, c.Source = true, SourceEnv
	c.Name, c.Address, c.ProxyAddress, c.ProxySerial = v(FieldName), v(FieldAddress), v(FieldProxyAddress), v(FieldProxySerial)
	c.FollowDiscovery = c.Address == ""
	c.Hidden = SplitHidden(v(FieldHidden))
}

// SplitHidden reads a hidden list as FileValue holds it.
func SplitHidden(v string) []string {
	return normalHidden(strings.FieldsFunc(v, func(r rune) bool { return r == ',' || r == ' ' || r == ';' || r == '\t' }))
}

// StoreValue returns a row's value of a field as FileValue writes it.
func StoreValue(c Cube, field string) string {
	switch field {
	case FieldName:
		return c.Name
	case FieldAddress:
		return c.Address
	case FieldProxyAddress:
		return c.ProxyAddress
	case FieldProxySerial:
		return c.ProxySerial
	case FieldHidden:
		return strings.Join(c.Hidden, ",")
	}
	return ""
}

// Differences lists the file's lines for a Cube whose values differ from
// its row, in the order of Fields. A field without a line is no
// difference; hidden lists are compared as sets.
func Differences(c Cube, fc FileCube) []Difference {
	var out []Difference
	for _, f := range Fields {
		fv, ok := fc.Values[f]
		if !ok {
			continue
		}
		file, have := fv.Value, StoreValue(c, f)
		same := file == have
		if f == FieldHidden {
			same = sameSet(SplitHidden(file), c.Hidden)
		}
		if !same {
			out = append(out, Difference{Serial: c.Serial, Field: f, Variable: fv.Variable, File: file, Store: have})
		}
	}
	return out
}

func sameSet(a, b []string) bool {
	a, b = normalHidden(a), normalHidden(b)
	if len(a) != len(b) {
		return false
	}
	sort.Strings(a)
	sort.Strings(b)
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
