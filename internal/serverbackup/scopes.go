// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package serverbackup

// The scopes of a restore (AP ServerBackup v1.0, Block 5, D6-A and D7-A;
// docs/store.md, "Restore by scope"): what each holds of the store and of
// the files beside it. Each table of the store belongs to scopes by its
// rows, in one list beside the store's own queries (internal/store,
// restore.go, scopeTables), which a test checks against every table of
// the schema; this file names the scopes for the restore's callers and
// says which file goes with which.
//
//	scope         rows                                              file
//	settings      the settings, the backups' targets and keys       -
//	users         users, roles, providers, their identities, keys   opaque.key
//	integrations  the integrations, their users, identities, keys   -
//	cubes         each Cube by its serial: rooms, backups, commands -
//	history       the audit, the log, the commands of no Cube       -
//	identity      node.id and tenant.id, with Identity              node.key
//
// Never restored: the sessions, the sign-in's lockouts, the Cube backups'
// pending changes, the schema's rows and the live catalogue of the
// server's backups. The env file and the TLS files are restored where
// asked for, each on its own (RestoreOptions.Env, TLS).

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"go.metaworx.ch/massimo/internal/store"
)

// The scopes a restore takes by name; ScopeIdentity is taken through
// RestoreOptions.Identity.
const (
	ScopeSettings     = store.ScopeSettings
	ScopeUsers        = store.ScopeUsers
	ScopeIntegrations = store.ScopeIntegrations
	ScopeCubes        = store.ScopeCubes
	ScopeHistory      = store.ScopeHistory
	ScopeIdentity     = store.ScopeIdentity
	// ScopeAll is every scope, every Cube of the backup among them.
	ScopeAll = "all"
	// cubePrefix names a Cube by its serial: cube:KEQ0000000.
	cubePrefix = "cube:"
)

// Scopes are the scopes a restore takes by name, in their order.
var Scopes = []string{ScopeSettings, ScopeUsers, ScopeIntegrations, ScopeCubes, ScopeHistory}

// Selection is a list of scopes read: the scopes, and the Cubes of
// ScopeCubes (none, every Cube of the backup).
type Selection struct {
	Scopes  []string
	Serials []string
}

// Has reports whether the selection holds a scope.
func (s Selection) Has(scope string) bool { return slices.Contains(s.Scopes, scope) }

// ParseScopes reads a list of scopes, each an item or several separated
// by commas: settings, users, integrations, cubes (every Cube of the
// backup), cube:<serial> (that Cube), history, or all (every one). A
// Cube's serial is ten letters and digits, taken in upper case.
func ParseScopes(list []string) (Selection, error) {
	var sel Selection
	add := func(s string) {
		if !slices.Contains(sel.Scopes, s) {
			sel.Scopes = append(sel.Scopes, s)
		}
	}
	every := false
	for _, item := range list {
		for _, s := range strings.Split(item, ",") {
			s = strings.ToLower(strings.TrimSpace(s))
			switch {
			case s == "":
			case s == ScopeAll:
				for _, sc := range Scopes {
					add(sc)
				}
				every = true
			case slices.Contains(Scopes, s):
				add(s)
				every = every || s == ScopeCubes
			case strings.HasPrefix(s, cubePrefix):
				sn := strings.ToUpper(strings.TrimPrefix(s, cubePrefix))
				if !validSerial(sn) {
					return Selection{}, fmt.Errorf("serverbackup: %q is not a Cube's serial, ten letters and digits", sn)
				}
				add(ScopeCubes)
				if !slices.Contains(sel.Serials, sn) {
					sel.Serials = append(sel.Serials, sn)
				}
			default:
				return Selection{}, fmt.Errorf("serverbackup: no scope %q; the scopes are %s, cube:<serial> and all", s, strings.Join(Scopes, ", "))
			}
		}
	}
	if every {
		sel.Serials = nil
	}
	slices.SortFunc(sel.Scopes, func(a, b string) int { return slices.Index(Scopes, a) - slices.Index(Scopes, b) })
	return sel, nil
}

// validSerial reports whether s is ten upper-case letters and digits, as
// a Cube's serial is.
func validSerial(s string) bool {
	return len(s) == 10 && strings.Trim(s, "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789") == ""
}

// errNothing is a restore that names nothing to restore.
var errNothing = errors.New("serverbackup: nothing to restore; name a scope, the identity, the env file or the TLS files")
