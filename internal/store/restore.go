// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package store

// The restore of a server backup by scope (AP ServerBackup v1.0, Block 5,
// D6-A and D7-A; docs/store.md, "Restore by scope"): each table of the
// store belongs to scopes by its rows (scopeTables, a test fails on a
// table without one), and a scope is replaced whole, in one transaction,
// from the backup's massimo.db, opened and brought to this store's schema
// beside (internal/serverbackup):
//
//   - a row of the backup takes the id of the live row of the same uuid,
//     or of the same natural key (a role's key, a Cube's serial, a
//     target's name, a key's fingerprint), or a new one, and its
//     references follow through a map of the backup's ids to this
//     store's; a reference to a row of another scope maps by uuid;
//   - a reference to a row of another scope that this store lacks
//     refuses the restore with the list of what is missing, but one the
//     schema lets go (ON DELETE SET NULL) and one to a session, which
//     never comes back, are cleared;
//   - a live row the backup lacks is removed; where a row outside the
//     scopes still names it and its table keeps removed rows
//     (deleted_at), it stays, marked removed; what the schema removes
//     with it (ON DELETE CASCADE) goes, and what it clears is cleared;
//   - the sealed secrets of the rows taken are opened with the backup's
//     opaque.key and sealed with this store's, or, where this store takes
//     the backup's opaque.key (D7-A), every sealed secret is sealed with
//     that one.
//
// The references are read from the schema (PRAGMA foreign_key_list), so
// that a later table's are followed without new code; a dry run answers
// what a restore would do, and rolls it back.

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"slices"
	"strings"
)

// The scopes a restore replaces, each whole (D6-A).
const (
	// ScopeSettings is the system settings: the settings but the box's
	// identifiers, and the server backups' targets and keys.
	ScopeSettings = "settings"
	// ScopeUsers is users and sign-in: the users that are not an
	// integration's, the roles and their capabilities, the sign-in
	// providers and their identities, the users' API keys, preferences
	// and push subscriptions; opaque.key travels with it (D7-A).
	ScopeUsers = "users"
	// ScopeIntegrations is the integrations with their users, their
	// identities and their keys.
	ScopeIntegrations = "integrations"
	// ScopeCubes is Cubes by their serials, each with its rooms, its Cube
	// backups and its commands.
	ScopeCubes = "cubes"
	// ScopeHistory is the audit, the log and the commands of no Cube.
	ScopeHistory = "history"
	// ScopeIdentity is the box's identifiers, node.id and tenant.id;
	// node.key travels with it.
	ScopeIdentity = "identity"
)

// RestoreScopes are the scopes, in the order a restore names them.
var RestoreScopes = []string{ScopeSettings, ScopeUsers, ScopeIntegrations, ScopeCubes, ScopeHistory, ScopeIdentity}

// The errors of a restore refused.
var (
	// ErrRestoreMissing is a backup whose rows name rows of a scope not
	// restored that this store lacks: RestoreReport.Missing lists them.
	ErrRestoreMissing = errors.New("store: the backup's rows name rows of other scopes that this store lacks; restore those scopes too")
	// ErrRestoreNoOwner is a restore of users and sign-in that would
	// leave a store with an owner without one.
	ErrRestoreNoOwner = errors.New("store: the backup holds no owner; restoring users and sign-in would leave this server without one")
)

// scopeTable is the rows of a table that belong to a scope.
type scopeTable struct {
	table string
	// scope is one of RestoreScopes; "" is never restored.
	scope string
	// where selects the rows, over the table's columns, {db} naming the
	// schema of the tables it reads besides; "" is every row.
	where string
	// match are the columns a row is matched by after its uuid, among
	// the rows not removed.
	match []string
	// null are the columns a row taken from the backup is written
	// without.
	null []string
	// keepLive leaves out a row of the backup that a live row outside
	// the scopes holds a unique value of: a push subscription's endpoint
	// that a session's subscription holds now, which registers itself
	// again anyway.
	keepLive bool
}

// The rows of the scopes' selections that the tables share.
const (
	regularIDs     = `SELECT id FROM {db}.users WHERE integration_id IS NULL`
	integrationIDs = `SELECT id FROM {db}.users WHERE integration_id IS NOT NULL`
	apiTokenIDs    = `SELECT id FROM {db}.tokens WHERE kind = 'api'`
	cubeSerials    = `serial IN (SELECT serial FROM temp.restore_serials)`
	cubeIDs        = `SELECT id FROM {db}.cubes WHERE ` + cubeSerials
)

// scopeTables places every row of every table of the store in one scope,
// or among the rows never restored. A table added later is placed here
// (TestEveryTableInAScope), or a restore would leave it out.
var scopeTables = []scopeTable{
	{table: "settings", scope: ScopeSettings, where: `key NOT IN ('` + NodeIDKey + `', '` + TenantIDKey + `')`},
	{table: "server_backup_targets", scope: ScopeSettings, match: []string{"name"}},
	{table: "server_backup_keys", scope: ScopeSettings, match: []string{"fingerprint"}},

	{table: "roles", scope: ScopeUsers, match: []string{"key"}},
	{table: "role_capabilities", scope: ScopeUsers},
	{table: "capabilities_disabled", scope: ScopeUsers},
	{table: "users", scope: ScopeUsers, where: `integration_id IS NULL`},
	{table: "auth_providers", scope: ScopeUsers, match: []string{"key"}},
	{table: "user_identities", scope: ScopeUsers, where: `provider_id IS NOT NULL`},
	{table: "tokens", scope: ScopeUsers, where: `kind = 'api' AND user_id IN (` + regularIDs + `)`, null: []string{"parent_id"}},
	{table: "preferences", scope: ScopeUsers, where: `user_id IN (` + regularIDs + `)`},
	{table: "push_subscriptions", scope: ScopeUsers, where: `user_id IN (` + regularIDs + `) AND token_id IN (` + apiTokenIDs + `)`, keepLive: true},

	{table: "integrations", scope: ScopeIntegrations},
	{table: "users", scope: ScopeIntegrations, where: `integration_id IS NOT NULL`},
	{table: "user_identities", scope: ScopeIntegrations, where: `integration_id IS NOT NULL`},
	{table: "tokens", scope: ScopeIntegrations, where: `kind = 'api' AND user_id IN (` + integrationIDs + `)`, null: []string{"parent_id"}},
	{table: "preferences", scope: ScopeIntegrations, where: `user_id IN (` + integrationIDs + `)`},
	{table: "push_subscriptions", scope: ScopeIntegrations, where: `user_id IN (` + integrationIDs + `) AND token_id IN (` + apiTokenIDs + `)`, keepLive: true},

	{table: "cubes", scope: ScopeCubes, where: cubeSerials, match: []string{"serial"}},
	{table: "rooms", scope: ScopeCubes, where: `cube_id IN (` + cubeIDs + `)`},
	{table: "backups", scope: ScopeCubes, where: `cube_id IN (` + cubeIDs + `)`},
	{table: "backup_data", scope: ScopeCubes, where: `id IN (SELECT id FROM {db}.backups WHERE cube_id IN (` + cubeIDs + `))`},
	{table: "commands", scope: ScopeCubes, where: `cube_id IN (` + cubeIDs + `) OR parent_id IN (SELECT id FROM {db}.commands WHERE cube_id IN (` + cubeIDs + `))`},

	{table: "commands", scope: ScopeHistory, where: `cube_id IS NULL AND (parent_id IS NULL OR parent_id IN (SELECT id FROM {db}.commands WHERE cube_id IS NULL))`},
	{table: "audit", scope: ScopeHistory},
	{table: "log", scope: ScopeHistory},

	{table: "settings", scope: ScopeIdentity, where: `key IN ('` + NodeIDKey + `', '` + TenantIDKey + `')`},

	// Never restored: the sessions and what hangs on one, the schema's
	// own rows, the sign-in's lockouts, the Cube backups' pending
	// changes, and the live catalogue of the server's backups.
	{table: "tokens", where: `kind = 'session'`},
	{table: "push_subscriptions", where: `token_id NOT IN (` + apiTokenIDs + `)`},
	{table: "schema_version"},
	{table: "auth_failures"},
	{table: "backup_pending"},
	{table: "server_backups"},
	{table: "server_backup_copies"},
}

// sealedColumn is a column that holds sealed secrets (secret.go), with
// the column of its additional data.
type sealedColumn struct {
	table, column, aad string
	// where selects the rows that hold one; "" every row.
	where string
	// json is a value kept as a JSON string, a setting's.
	json bool
}

// sealedColumns are the sealed secrets of the store: a provider's client
// secret, a target's secret, and the settings of sealedSettings.
var sealedColumns = []sealedColumn{
	{table: "auth_providers", column: "client_secret", aad: "uuid"},
	{table: "server_backup_targets", column: "secret", aad: "uuid"},
	{table: "settings", column: "value", aad: "key", where: `key IN (` + quotedList(sealedSettings) + `)`, json: true},
}

// quotedList is the SQL list of the texts, quoted.
func quotedList(vs []string) string {
	var parts []string
	for _, v := range vs {
		parts = append(parts, "'"+strings.ReplaceAll(v, "'", "''")+"'")
	}
	return strings.Join(parts, ", ")
}

// RestoreSelection is what Restore replaces, and from where.
type RestoreSelection struct {
	// Scopes are the scopes replaced, of RestoreScopes.
	Scopes []string
	// Serials are the Cubes of ScopeCubes; none is every Cube the backup
	// holds.
	Serials []string
	// Source is the backup's data directory: its massimo.db, at this
	// store's schema version, and its opaque.key, which opens the
	// backup's sealed secrets.
	Source string
	// OpaqueKey says that this store takes the backup's opaque.key
	// (D7-A, with ScopeUsers): every sealed secret, the live ones too, is
	// then sealed with it. The caller puts the file in place once
	// Restore has returned.
	OpaqueKey bool
	// DryRun answers what the restore would do and changes nothing.
	DryRun bool
}

// RestoreReport is what a restore did, or would do.
type RestoreReport struct {
	// Tables are the rows per scope and table.
	Tables []RestoreTable `json:"tables"`
	// Missing are the rows of other scopes the backup's rows name and
	// this store lacks; any refuses the restore (ErrRestoreMissing).
	Missing []MissingRow `json:"missing"`
	// Cleared are the references let go: to a row this store lacks
	// that the schema lets go, to a session, or to a live row the
	// restore removed.
	Cleared []RestoreCount `json:"cleared"`
	// Removed are the live rows outside the scopes that went with a row
	// the restore removed (ON DELETE CASCADE).
	Removed []RestoreCount `json:"removed"`
	// Lost counts the sealed secrets that did not open: each is to be
	// entered again.
	Lost int `json:"secrets_lost"`
	// Cubes are the Cubes of ScopeCubes, as the store holds them after.
	Cubes []RestoredCube `json:"cubes"`
}

// RestoreTable is a table's rows in a scope: Restored taken from the
// backup, of which Replaced took the place of a live row; Removed the
// live rows the backup lacks, of which KeptRemoved stay, marked removed,
// since a row outside the scopes names them.
type RestoreTable struct {
	Scope       string `json:"scope"`
	Table       string `json:"table"`
	Restored    int    `json:"restored"`
	Replaced    int    `json:"replaced"`
	Removed     int    `json:"removed"`
	KeptRemoved int    `json:"kept_removed"`
}

// MissingRow is a row of another scope that a row of the backup names
// and this store lacks: its table, uuid and name (a user's, a Cube's
// serial, a role's key, …), and the table that names it.
type MissingRow struct {
	Table string `json:"table"`
	UUID  string `json:"uuid"`
	Name  string `json:"name,omitempty"`
	By    string `json:"by"`
}

// RestoreCount is a number of rows of a table, or of a table's column.
type RestoreCount struct {
	Table  string `json:"table"`
	Column string `json:"column,omitempty"`
	Rows   int    `json:"rows"`
}

// RestoredCube is a Cube of a restore: Live where this store held it
// before, and Backup its newest Cube backup, the configuration the Cube
// restore could write onto it ("" none).
type RestoredCube struct {
	Serial string `json:"serial"`
	Name   string `json:"name"`
	Live   bool   `json:"live"`
	Backup string `json:"backup,omitempty"`
}

// Restore replaces the scopes of sel from the backup in sel.Source, in
// one transaction (see the file's comment), and reports what it did; with
// sel.DryRun what it would do. A reference this store cannot satisfy is
// ErrRestoreMissing, with the report's Missing; users and sign-in that
// would leave no owner is ErrRestoreNoOwner. Restore takes the store's
// connection for its duration: nothing else is written meanwhile.
func (s *Store) Restore(ctx context.Context, sel RestoreSelection) (RestoreReport, error) {
	if err := s.writable(); err != nil {
		return RestoreReport{}, err
	}
	scopes, err := checkScopes(sel.Scopes)
	if err != nil {
		return RestoreReport{}, err
	}
	src := Path(sel.Source)
	if _, err := os.Stat(src); err != nil {
		return RestoreReport{}, fmt.Errorf("store: restore: %w", err)
	}
	bkKey, err := readSealKey(sel.Source)
	if err != nil {
		return RestoreReport{}, err
	}
	r := &restorer{s: s, sel: sel, scopes: scopes, liveKey: s.sealKey(), bkKey: bkKey}
	r.newKey = r.liveKey
	if sel.OpaqueKey {
		if bkKey == nil {
			return RestoreReport{}, errors.New("store: restore: the backup holds no opaque.key to take")
		}
		r.newKey = bkKey
	}
	conn, err := s.db.Conn(ctx)
	if err != nil {
		return RestoreReport{}, fmt.Errorf("store: restore: %w", err)
	}
	defer conn.Close()
	bg := context.WithoutCancel(ctx)
	// The references are followed by the restore itself, and checked at
	// its end; the pragma is a no-op within a transaction.
	if _, err := conn.ExecContext(ctx, `PRAGMA foreign_keys = OFF`); err != nil {
		return RestoreReport{}, fmt.Errorf("store: restore: %w", err)
	}
	defer conn.ExecContext(bg, `PRAGMA foreign_keys = ON`)
	if _, err := conn.ExecContext(ctx, `ATTACH DATABASE ? AS bk`, src); err != nil {
		return RestoreReport{}, fmt.Errorf("store: restore: the backup's store: %w", err)
	}
	defer conn.ExecContext(bg, `DETACH DATABASE bk`)
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return RestoreReport{}, fmt.Errorf("store: restore: %w", err)
	}
	defer tx.Rollback()
	r.tx = tx
	if err := r.run(ctx); err != nil {
		return r.rep, err
	}
	if sel.DryRun {
		return r.rep, nil
	}
	if err := tx.Commit(); err != nil {
		return r.rep, fmt.Errorf("store: restore: %w", err)
	}
	return r.rep, nil
}

// checkScopes refuses an empty or unknown scope, and returns them as a set.
func checkScopes(list []string) (map[string]bool, error) {
	if len(list) == 0 {
		return nil, errors.New("store: restore: no scope")
	}
	out := map[string]bool{}
	for _, sc := range list {
		if !slices.Contains(RestoreScopes, sc) {
			return nil, fmt.Errorf("store: restore: no scope %q", sc)
		}
		out[sc] = true
	}
	return out, nil
}

// readSealKey derives the sealing key of the opaque.key in dir; nil where
// there is none.
func readSealKey(dir string) ([]byte, error) {
	b, err := os.ReadFile(OpaqueKeyPath(dir))
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return nil, nil
	case err != nil:
		return nil, fmt.Errorf("store: restore: %w", err)
	}
	blk, _ := pem.Decode(b)
	if blk == nil || blk.Type != opaquePEM {
		return nil, fmt.Errorf("store: restore: the backup's %s is not a PEM %s", OpaqueKeyFileName, opaquePEM)
	}
	return deriveSealKey(blk.Bytes)
}

// column is a column of a table, as PRAGMA table_info gives it.
type column struct {
	name    string
	notNull bool
	pk      bool
}

// foreignKey is a reference of a table, as PRAGMA foreign_key_list gives
// it: its id, the parent table, the column and the action on delete.
type foreignKey struct {
	id       int
	parent   string
	from     string
	onDelete string
}

// tableInfo is what the restore knows of a table.
type tableInfo struct {
	cols []column
	fks  []foreignKey
}

func (t tableInfo) has(name string) bool {
	return slices.ContainsFunc(t.cols, func(c column) bool { return c.name == name })
}

func (t tableInfo) col(name string) column {
	for _, c := range t.cols {
		if c.name == name {
			return c
		}
	}
	return column{}
}

// fkOf is the reference of the column name, if any.
func (t tableInfo) fkOf(name string) (foreignKey, bool) {
	for _, fk := range t.fks {
		if fk.from == name {
			return fk, true
		}
	}
	return foreignKey{}, false
}

// identified reports whether the table's rows have an id of their own,
// not a reference, and a uuid: the rows the restore matches and maps.
func (t tableInfo) identified() bool {
	_, ref := t.fkOf("id")
	return t.col("id").pk && t.has("uuid") && !ref
}

// violation is a row that names a missing row, as PRAGMA
// foreign_key_check gives it.
type violation struct {
	table string
	rowid int64
	fkid  int
}

// restorer is one restore's state.
type restorer struct {
	s      *Store
	tx     *sql.Tx
	sel    RestoreSelection
	scopes map[string]bool
	rep    RestoreReport

	liveKey, bkKey, newKey []byte

	info     map[string]tableInfo
	order    []string            // the tables restored, in scopeTables' order
	mapped   map[string]bool     // the tables whose ids the map holds
	own      map[string]bool     // the tables restored whose rows take mapped ids
	temps    []string            // the temporary tables made
	kept     map[string]int      // the rows kept removed per table
	old      map[string]bool     // the tables whose removed rows are kept aside
	children map[string][]string // the tables that name a table
}

// entries are the scopeTables of the scopes restored.
func (r *restorer) entries() []scopeTable {
	var out []scopeTable
	for _, e := range scopeTables {
		if e.scope != "" && r.scopes[e.scope] {
			out = append(out, e)
		}
	}
	return out
}

// where is a selection of e over db.
func where(e scopeTable, db string) string {
	if e.where == "" {
		return "1"
	}
	return "(" + strings.ReplaceAll(e.where, "{db}", db) + ")"
}

// selOf is the rows of a table restored, over db: every entry's selection.
func (r *restorer) selOf(table, db string) string {
	var parts []string
	for _, e := range r.entries() {
		if e.table == table {
			parts = append(parts, where(e, db))
		}
	}
	if len(parts) == 0 {
		return "0"
	}
	return "(" + strings.Join(parts, " OR ") + ")"
}

// neverOf is the rows of table that are never restored, over db; "0"
// none.
func neverOf(table, db string) string {
	var parts []string
	for _, e := range scopeTables {
		if e.table == table && e.scope == "" && e.where != "" {
			parts = append(parts, where(e, db))
		}
	}
	if len(parts) == 0 {
		return "0"
	}
	return "(" + strings.Join(parts, " OR ") + ")"
}

// q quotes an identifier.
func q(name string) string { return `"` + strings.ReplaceAll(name, `"`, `""`) + `"` }

// exec runs a statement of the restore.
func (r *restorer) exec(ctx context.Context, query string, args ...any) (int, error) {
	res, err := r.tx.ExecContext(ctx, query, args...)
	if err != nil {
		return 0, fmt.Errorf("store: restore: %w", err)
	}
	n, _ := res.RowsAffected()
	return int(n), nil
}

// count runs a count of the restore.
func (r *restorer) count(ctx context.Context, query string, args ...any) (int, error) {
	var n int
	if err := r.tx.QueryRowContext(ctx, query, args...).Scan(&n); err != nil {
		return 0, fmt.Errorf("store: restore: %w", err)
	}
	return n, nil
}

// temp makes a temporary table of the restore, dropped at its end.
func (r *restorer) temp(ctx context.Context, name, def string) error {
	if _, err := r.exec(ctx, `CREATE TEMP TABLE `+q(name)+` `+def); err != nil {
		return err
	}
	r.temps = append(r.temps, name)
	return nil
}

// run is the restore inside its transaction.
func (r *restorer) run(ctx context.Context) error {
	var bkVersion int
	if err := r.tx.QueryRowContext(ctx, `SELECT coalesce(max(version), 0) FROM bk.schema_version`).Scan(&bkVersion); err != nil {
		return fmt.Errorf("store: restore: the backup's store: %w", err)
	}
	if bkVersion != r.s.schema {
		return fmt.Errorf("store: restore: the backup's store is of schema version %d, this one of %d", bkVersion, r.s.schema)
	}
	if err := r.loadInfo(ctx); err != nil {
		return err
	}
	if err := r.temp(ctx, "restore_serials", `(serial TEXT PRIMARY KEY)`); err != nil {
		return err
	}
	if err := r.temp(ctx, "restore_map", `(tbl TEXT NOT NULL, old INTEGER NOT NULL, new INTEGER NOT NULL, mine INTEGER NOT NULL, matched INTEGER NOT NULL, PRIMARY KEY (tbl, old))`); err != nil {
		return err
	}
	if _, err := r.exec(ctx, `CREATE INDEX temp.restore_map_new ON restore_map (tbl, new)`); err != nil {
		return err
	}
	if err := r.serials(ctx); err != nil {
		return err
	}
	if err := r.buildMaps(ctx); err != nil {
		return err
	}
	if err := r.counts(ctx); err != nil {
		return err
	}
	if err := r.missing(ctx); err != nil {
		return err
	}
	if len(r.rep.Missing) > 0 {
		return fmt.Errorf("%w: %d rows", ErrRestoreMissing, len(r.rep.Missing))
	}
	ownersBefore, err := r.owners(ctx)
	if err != nil {
		return err
	}
	baseline, err := r.violations(ctx)
	if err != nil {
		return err
	}
	if err := r.replace(ctx); err != nil {
		return err
	}
	if err := r.reseal(ctx); err != nil {
		return err
	}
	if err := r.repair(ctx, baseline); err != nil {
		return err
	}
	if r.scopes[ScopeUsers] {
		after, err := r.owners(ctx)
		if err != nil {
			return err
		}
		if after == 0 && ownersBefore > 0 {
			return ErrRestoreNoOwner
		}
	}
	if r.scopes[ScopeUsers] || r.scopes[ScopeIntegrations] {
		// The sessions of a user the restore removed end with it, as
		// DeleteUser ends them.
		if _, err := r.exec(ctx, `UPDATE main.tokens SET deleted_at = ? WHERE deleted_at IS NULL AND user_id IN (SELECT id FROM main.users WHERE deleted_at IS NOT NULL)`, r.s.nowText()); err != nil {
			return err
		}
	}
	if err := r.cubes(ctx); err != nil {
		return err
	}
	for i := len(r.temps) - 1; i >= 0; i-- {
		if _, err := r.exec(ctx, `DROP TABLE temp.`+q(r.temps[i])); err != nil {
			return err
		}
	}
	r.temps = nil
	return nil
}

// loadInfo reads the columns and references of every table of the store.
func (r *restorer) loadInfo(ctx context.Context) error {
	tables, err := tableNames(ctx, r.tx, "main")
	if err != nil {
		return err
	}
	r.info = map[string]tableInfo{}
	r.children = map[string][]string{}
	for _, t := range tables {
		var ti tableInfo
		rows, err := r.tx.QueryContext(ctx, `PRAGMA main.table_info(`+q(t)+`)`)
		if err != nil {
			return fmt.Errorf("store: restore: %w", err)
		}
		for rows.Next() {
			var cid, notNull, pk int
			var name, typ string
			var dflt sql.NullString
			if err := rows.Scan(&cid, &name, &typ, &notNull, &dflt, &pk); err != nil {
				rows.Close()
				return fmt.Errorf("store: restore: %w", err)
			}
			ti.cols = append(ti.cols, column{name: name, notNull: notNull != 0, pk: pk == 1})
		}
		rows.Close()
		rows, err = r.tx.QueryContext(ctx, `PRAGMA main.foreign_key_list(`+q(t)+`)`)
		if err != nil {
			return fmt.Errorf("store: restore: %w", err)
		}
		for rows.Next() {
			var fk foreignKey
			var seq int
			var to sql.NullString
			var onUpdate, match string
			if err := rows.Scan(&fk.id, &seq, &fk.parent, &fk.from, &to, &onUpdate, &fk.onDelete, &match); err != nil {
				rows.Close()
				return fmt.Errorf("store: restore: %w", err)
			}
			ti.fks = append(ti.fks, fk)
		}
		rows.Close()
		r.info[t] = ti
		for _, fk := range ti.fks {
			if !slices.Contains(r.children[fk.parent], t) {
				r.children[fk.parent] = append(r.children[fk.parent], t)
			}
		}
	}
	for _, e := range r.entries() {
		if _, ok := r.info[e.table]; !ok {
			return fmt.Errorf("store: restore: no table %s", e.table)
		}
		if !slices.Contains(r.order, e.table) {
			r.order = append(r.order, e.table)
		}
	}
	return nil
}

// tableNames are the tables of the schema db, without SQLite's own.
func tableNames(ctx context.Context, q interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}, db string) ([]string, error) {
	rows, err := q.QueryContext(ctx, `SELECT name FROM `+db+`.sqlite_master WHERE type = 'table' AND name NOT LIKE 'sqlite\_%' ESCAPE '\' ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("store: tables: %w", err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			return nil, fmt.Errorf("store: tables: %w", err)
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

// serials fills restore_serials with the Cubes of ScopeCubes: those
// asked for, each of which the backup must hold, or every Cube it holds.
func (r *restorer) serials(ctx context.Context) error {
	if !r.scopes[ScopeCubes] {
		return nil
	}
	if len(r.sel.Serials) == 0 {
		_, err := r.exec(ctx, `INSERT OR IGNORE INTO temp.restore_serials SELECT serial FROM bk.cubes WHERE deleted_at IS NULL`)
		return err
	}
	for _, sn := range r.sel.Serials {
		sn = strings.ToUpper(strings.TrimSpace(sn))
		n, err := r.count(ctx, `SELECT count(*) FROM bk.cubes WHERE serial = ? AND deleted_at IS NULL`, sn)
		if err != nil {
			return err
		}
		if n == 0 {
			return fmt.Errorf("store: restore: the backup holds no Cube %s", sn)
		}
		if _, err := r.exec(ctx, `INSERT OR IGNORE INTO temp.restore_serials VALUES (?)`, sn); err != nil {
			return err
		}
	}
	return nil
}

// matchFor are the natural keys of a table, from its entries.
func (r *restorer) matchFor(table string) []string {
	var out []string
	for _, e := range scopeTables {
		if e.table == table {
			for _, m := range e.match {
				if !slices.Contains(out, m) {
					out = append(out, m)
				}
			}
		}
	}
	return out
}

// buildMaps maps the backup's ids to this store's: of the tables
// restored whose rows have an id and a uuid, and of every table a
// restored row names.
func (r *restorer) buildMaps(ctx context.Context) error {
	r.mapped, r.own = map[string]bool{}, map[string]bool{}
	for _, t := range r.order {
		ti := r.info[t]
		// A table restored whole that nothing names keeps the backup's
		// ids: every live row goes, and no reference is to follow.
		whole := r.selOf(t, "bk") == "(1)" && len(r.children[t]) == 0
		if ti.identified() && !whole {
			r.mapped[t], r.own[t] = true, true
		}
		for _, fk := range ti.fks {
			if r.info[fk.parent].identified() {
				r.mapped[fk.parent] = true
			}
		}
	}
	tables := make([]string, 0, len(r.mapped))
	for t := range r.mapped {
		tables = append(tables, t)
	}
	slices.Sort(tables)
	for _, t := range tables {
		if err := r.buildMap(ctx, t); err != nil {
			return err
		}
	}
	return nil
}

// buildMap maps the ids of a table.
func (r *restorer) buildMap(ctx context.Context, t string) error {
	ti := r.info[t]
	qt := q(t)
	notMapped := `b.id NOT IN (SELECT old FROM temp.restore_map WHERE tbl = '` + t + `')`
	liveFree := `m.id NOT IN (SELECT new FROM temp.restore_map WHERE tbl = '` + t + `')`
	alive := ""
	if ti.has("deleted_at") {
		alive = ` AND b.deleted_at IS NULL AND m.deleted_at IS NULL`
	}
	natural := func(mine int, scoped string) error {
		for _, k := range r.matchFor(t) {
			eq := `m.` + q(k) + ` = b.` + q(k)
			if t == "server_backup_targets" && k == "name" {
				eq += ` COLLATE NOCASE`
			}
			if _, err := r.exec(ctx, `INSERT INTO temp.restore_map SELECT '`+t+`', b.id, m.id, ?, 1 FROM bk.`+qt+` b JOIN main.`+qt+` m ON `+eq+
				` WHERE `+notMapped+` AND `+liveFree+alive+scoped, mine); err != nil {
				return err
			}
		}
		return nil
	}
	if r.own[t] {
		scoped := ` AND b.id IN (SELECT id FROM bk.` + qt + ` WHERE ` + r.selOf(t, "bk") + `) AND m.id IN (SELECT id FROM main.` + qt + ` WHERE ` + r.selOf(t, "main") + `)`
		if _, err := r.exec(ctx, `INSERT INTO temp.restore_map SELECT '`+t+`', b.id, m.id, 1, 1 FROM bk.`+qt+` b JOIN main.`+qt+` m ON m.uuid = b.uuid WHERE 1`+scoped); err != nil {
			return err
		}
		if err := natural(1, scoped); err != nil {
			return err
		}
		base, err := r.idBase(ctx, t)
		if err != nil {
			return err
		}
		if _, err := r.exec(ctx, `INSERT INTO temp.restore_map SELECT '`+t+`', b.id, ? + row_number() OVER (ORDER BY b.id), 1, 0 FROM bk.`+qt+` b
			WHERE b.id IN (SELECT id FROM bk.`+qt+` WHERE `+r.selOf(t, "bk")+`) AND `+notMapped, base); err != nil {
			return err
		}
	}
	// The other rows a restored row may name: the live row of the same
	// uuid, or of the same natural key.
	if _, err := r.exec(ctx, `INSERT INTO temp.restore_map SELECT '`+t+`', b.id, m.id, 0, 1 FROM bk.`+qt+` b JOIN main.`+qt+` m ON m.uuid = b.uuid WHERE `+notMapped+` AND `+liveFree); err != nil {
		return err
	}
	return natural(0, "")
}

// idBase is the highest id a table has given: its largest, or its
// sequence's for an AUTOINCREMENT table, whichever is higher.
func (r *restorer) idBase(ctx context.Context, t string) (int64, error) {
	var base int64
	if err := r.tx.QueryRowContext(ctx, `SELECT coalesce(max(id), 0) FROM main.`+q(t)).Scan(&base); err != nil {
		return 0, fmt.Errorf("store: restore: %w", err)
	}
	var seq sql.NullInt64
	err := r.tx.QueryRowContext(ctx, `SELECT seq FROM main.sqlite_sequence WHERE name = ?`, t).Scan(&seq)
	if err != nil && !errors.Is(err, sql.ErrNoRows) && !strings.Contains(err.Error(), "no such table") {
		return 0, fmt.Errorf("store: restore: %w", err)
	}
	if seq.Valid && seq.Int64 > base {
		base = seq.Int64
	}
	return base, nil
}

// counts counts each entry's rows: in the backup, of them those that take
// a live row's place, and the live rows the backup lacks.
func (r *restorer) counts(ctx context.Context) error {
	for _, e := range r.entries() {
		qt := q(e.table)
		bkSel := `SELECT rowid FROM bk.` + qt + ` WHERE ` + where(e, "bk")
		liveSel := `SELECT rowid FROM main.` + qt + ` WHERE ` + where(e, "main")
		restored, err := r.count(ctx, `SELECT count(*) FROM (`+bkSel+`)`)
		if err != nil {
			return err
		}
		live, err := r.count(ctx, `SELECT count(*) FROM (`+liveSel+`)`)
		if err != nil {
			return err
		}
		replaced := 0
		ti := r.info[e.table]
		switch {
		case r.own[e.table]:
			replaced, err = r.count(ctx, `SELECT count(*) FROM bk.`+qt+` b JOIN temp.restore_map m ON m.tbl = '`+e.table+`' AND m.old = b.id AND m.mine = 1 AND m.matched = 1 WHERE b.rowid IN (`+bkSel+`)`)
		case ti.has("uuid"):
			replaced, err = r.count(ctx, `SELECT count(*) FROM bk.`+qt+` b WHERE b.rowid IN (`+bkSel+`) AND b.uuid IN (SELECT uuid FROM main.`+qt+` WHERE rowid IN (`+liveSel+`))`)
		case e.table == "settings":
			replaced, err = r.count(ctx, `SELECT count(*) FROM bk.settings b WHERE b.rowid IN (`+bkSel+`) AND b.key IN (SELECT key FROM main.settings WHERE rowid IN (`+liveSel+`))`)
		}
		if err != nil {
			return err
		}
		r.rep.Tables = append(r.rep.Tables, RestoreTable{Scope: e.scope, Table: e.table, Restored: restored, Replaced: replaced, Removed: live - replaced})
	}
	return nil
}

// missing finds the rows of other scopes the backup's rows name that this
// store lacks: each in Missing, or, where the schema lets the reference
// go or it is to a session, counted in Cleared.
func (r *restorer) missing(ctx context.Context) error {
	seen := map[string]bool{}
	cleared := map[[2]string]int{}
	for _, e := range r.entries() {
		ti := r.info[e.table]
		qt := q(e.table)
		for _, fk := range ti.fks {
			if slices.Contains(e.null, fk.from) || !r.mapped[fk.parent] {
				continue
			}
			col := `b.` + q(fk.from)
			rows, err := r.tx.QueryContext(ctx, `SELECT `+col+`, count(*) FROM bk.`+qt+` b WHERE b.rowid IN (SELECT rowid FROM bk.`+qt+` WHERE `+where(e, "bk")+`)
				AND `+col+` IS NOT NULL AND NOT EXISTS (SELECT 1 FROM temp.restore_map m WHERE m.tbl = '`+fk.parent+`' AND m.old = `+col+`) GROUP BY `+col)
			if err != nil {
				return fmt.Errorf("store: restore: %w", err)
			}
			type ref struct {
				id int64
				n  int
			}
			var refs []ref
			for rows.Next() {
				var f ref
				if err := rows.Scan(&f.id, &f.n); err != nil {
					rows.Close()
					return fmt.Errorf("store: restore: %w", err)
				}
				refs = append(refs, f)
			}
			rows.Close()
			for _, f := range refs {
				never, err := r.count(ctx, `SELECT count(*) FROM bk.`+q(fk.parent)+` WHERE id = ? AND `+neverOf(fk.parent, "bk"), f.id)
				if err != nil {
					return err
				}
				if fk.onDelete == "SET NULL" || never > 0 && !ti.col(fk.from).notNull {
					cleared[[2]string{e.table, fk.from}] += f.n
					continue
				}
				key := fk.parent + "/" + fmt.Sprint(f.id)
				if seen[key] {
					continue
				}
				seen[key] = true
				m := MissingRow{Table: fk.parent, By: e.table}
				r.describe(ctx, fk.parent, f.id, &m)
				r.rep.Missing = append(r.rep.Missing, m)
			}
		}
	}
	for k, n := range cleared {
		r.rep.Cleared = append(r.rep.Cleared, RestoreCount{Table: k[0], Column: k[1], Rows: n})
	}
	slices.SortFunc(r.rep.Cleared, func(a, b RestoreCount) int { return strings.Compare(a.Table+"."+a.Column, b.Table+"."+b.Column) })
	return nil
}

// describe names a row of the backup for MissingRow: its uuid, and its
// name, serial, key or label.
func (r *restorer) describe(ctx context.Context, table string, id int64, m *MissingRow) {
	ti := r.info[table]
	name := "''"
	for _, c := range []string{"name", "serial", "key", "label"} {
		if ti.has(c) {
			name = q(c)
			break
		}
	}
	r.tx.QueryRowContext(ctx, `SELECT uuid, `+name+` FROM bk.`+q(table)+` WHERE id = ?`, id).Scan(&m.UUID, &m.Name)
}

// owners counts the owners who may sign in.
func (r *restorer) owners(ctx context.Context) (int, error) {
	return r.count(ctx, `SELECT count(*) FROM main.users u JOIN main.roles o ON o.id = u.role_id
		WHERE o.key = ? AND u.deleted_at IS NULL AND u.integration_id IS NULL AND u.disabled = 0`, RoleOwner)
}

// checked are the tables whose references the restore checks: those that
// name a table restored.
func (r *restorer) checked() []string {
	var out []string
	for _, t := range r.order {
		for _, c := range r.children[t] {
			if !slices.Contains(out, c) {
				out = append(out, c)
			}
		}
	}
	slices.Sort(out)
	return out
}

// violations are the rows of the checked tables that name a missing row.
func (r *restorer) violations(ctx context.Context) (map[violation]bool, error) {
	out := map[violation]bool{}
	for _, t := range r.checked() {
		rows, err := r.tx.QueryContext(ctx, `PRAGMA main.foreign_key_check(`+q(t)+`)`)
		if err != nil {
			return nil, fmt.Errorf("store: restore: %w", err)
		}
		for rows.Next() {
			var v violation
			var rowid sql.NullInt64
			var parent string
			if err := rows.Scan(&v.table, &rowid, &parent, &v.fkid); err != nil {
				rows.Close()
				return nil, fmt.Errorf("store: restore: %w", err)
			}
			v.rowid = rowid.Int64
			out[v] = true
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return nil, fmt.Errorf("store: restore: %w", err)
		}
	}
	return out, nil
}

// replace removes the live rows of the scopes, keeping aside those the
// backup lacks in a table that keeps removed rows, and inserts the
// backup's, their ids and references mapped.
func (r *restorer) replace(ctx context.Context) error {
	r.old, r.kept = map[string]bool{}, map[string]int{}
	// The live selections first: a selection reads other tables, which
	// the removal changes.
	for _, t := range r.order {
		if err := r.temp(ctx, "restore_live_"+t, `AS SELECT rowid AS rid FROM main.`+q(t)+` WHERE `+r.selOf(t, "main")); err != nil {
			return err
		}
		if r.own[t] && r.info[t].has("deleted_at") {
			if err := r.temp(ctx, "restore_old_"+t, `AS SELECT * FROM main.`+q(t)+` WHERE rowid IN (SELECT rid FROM temp.`+q("restore_live_"+t)+`)
				AND id NOT IN (SELECT new FROM temp.restore_map WHERE tbl = '`+t+`' AND mine = 1 AND matched = 1)`); err != nil {
				return err
			}
			r.old[t] = true
		}
	}
	for _, t := range r.order {
		if _, err := r.exec(ctx, `DELETE FROM main.`+q(t)+` WHERE rowid IN (SELECT rid FROM temp.`+q("restore_live_"+t)+`)`); err != nil {
			return err
		}
	}
	for _, t := range r.order {
		ti := r.info[t]
		var nulls []string
		insert := "INSERT"
		for _, e := range r.entries() {
			if e.table == t {
				nulls = append(nulls, e.null...)
				if e.keepLive {
					insert = "INSERT OR IGNORE"
				}
			}
		}
		var cols, exprs []string
		for _, c := range ti.cols {
			cols = append(cols, q(c.name))
			fk, ref := ti.fkOf(c.name)
			switch {
			case slices.Contains(nulls, c.name):
				exprs = append(exprs, "NULL")
			case ref && r.mapped[fk.parent]:
				exprs = append(exprs, `(SELECT m.new FROM temp.restore_map m WHERE m.tbl = '`+fk.parent+`' AND m.old = b.`+q(c.name)+`)`)
			case c.name == "id" && r.own[t]:
				exprs = append(exprs, `(SELECT m.new FROM temp.restore_map m WHERE m.tbl = '`+t+`' AND m.old = b.id)`)
			default:
				exprs = append(exprs, `b.`+q(c.name))
			}
		}
		if _, err := r.exec(ctx, insert+` INTO main.`+q(t)+` (`+strings.Join(cols, ", ")+`) SELECT `+strings.Join(exprs, ", ")+
			` FROM bk.`+q(t)+` b WHERE b.rowid IN (SELECT rowid FROM bk.`+q(t)+` WHERE `+r.selOf(t, "bk")+`)`); err != nil {
			return fmt.Errorf("%w (the table %s)", err, t)
		}
	}
	return nil
}

// fromBackup selects the rows of a table that the restore took from the
// backup; "0" none.
func (r *restorer) fromBackup(t string) string {
	switch {
	case !slices.Contains(r.order, t):
		return "0"
	case r.own[t]:
		return `id IN (SELECT new FROM temp.restore_map WHERE tbl = '` + t + `' AND mine = 1)`
	case r.info[t].has("uuid"):
		return `uuid IN (SELECT uuid FROM bk.` + q(t) + ` WHERE ` + r.selOf(t, "bk") + `)`
	case t == "settings":
		return `key IN (SELECT key FROM bk.settings WHERE ` + r.selOf(t, "bk") + `)`
	}
	return "0"
}

// reseal opens the sealed secrets of the rows taken with the backup's
// key, and, where the store takes the backup's opaque.key, the live ones
// with the live key, and seals them with the key the store keeps after.
// One that does not open is left, and counted in Lost.
func (r *restorer) reseal(ctx context.Context) error {
	for _, sc := range sealedColumns {
		if _, ok := r.info[sc.table]; !ok {
			continue
		}
		filter := "1"
		if sc.where != "" {
			filter = sc.where
		}
		rows, err := r.tx.QueryContext(ctx, `SELECT rowid, `+q(sc.aad)+`, `+q(sc.column)+`, `+r.fromBackup(sc.table)+` FROM main.`+q(sc.table)+` WHERE `+filter+` AND `+q(sc.column)+` != ''`)
		if err != nil {
			return fmt.Errorf("store: restore: %w", err)
		}
		type kept struct {
			rowid     int64
			aad, val  string
			fromTaken bool
		}
		var all []kept
		for rows.Next() {
			var k kept
			if err := rows.Scan(&k.rowid, &k.aad, &k.val, &k.fromTaken); err != nil {
				rows.Close()
				return fmt.Errorf("store: restore: %w", err)
			}
			all = append(all, k)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return fmt.Errorf("store: restore: %w", err)
		}
		for _, k := range all {
			key := r.liveKey
			if k.fromTaken {
				key = r.bkKey
			}
			if bytes.Equal(key, r.newKey) {
				continue
			}
			val, ok, err := r.resealOne(sc, key, k.aad, k.val)
			if err != nil {
				return err
			}
			if !ok {
				r.rep.Lost++
				continue
			}
			if _, err := r.exec(ctx, `UPDATE main.`+q(sc.table)+` SET `+q(sc.column)+` = ? WHERE rowid = ?`, val, k.rowid); err != nil {
				return err
			}
		}
	}
	return nil
}

// resealOne opens a kept secret with key and seals it with the new key;
// false where it does not open.
func (r *restorer) resealOne(sc sealedColumn, key []byte, aad, kept string) (string, bool, error) {
	text := kept
	if sc.json {
		// A setting is a JSON string when sealed; one kept plain, from
		// before the sealing, is sealed as its JSON, as
		// sealPlainSettings does.
		var s string
		if json.Unmarshal([]byte(kept), &s) == nil && (strings.HasPrefix(s, sealedPrefix) || strings.HasPrefix(s, legacyPrefix)) {
			text = s
		}
	}
	var plain string
	switch {
	case strings.HasPrefix(text, sealedPrefix):
		if key == nil {
			return "", false, nil
		}
		out, err := openBox(key, aad, strings.TrimPrefix(text, sealedPrefix))
		if err != nil {
			return "", false, nil
		}
		plain = out
	case strings.HasPrefix(text, legacyPrefix):
		// secret.key's, which no backup carries.
		return "", false, nil
	default:
		plain = text
	}
	if r.newKey == nil {
		return "", false, errors.New("store: restore: no opaque.key to seal with")
	}
	box, err := sealBox(r.newKey, aad, plain)
	if err != nil {
		return "", false, err
	}
	sealed := sealedPrefix + box
	if sc.json {
		b, err := json.Marshal(sealed)
		if err != nil {
			return "", false, fmt.Errorf("store: restore: %w", err)
		}
		return string(b), true, nil
	}
	return sealed, true, nil
}

// repair follows the references to the live rows the restore removed, as
// a removal does: a reference the schema removes with its row (CASCADE)
// removes the row naming it, one it lets go (SET NULL) is cleared, and
// any other keeps the removed row, marked removed, where its table keeps
// removed rows; else the restore is refused. Each round may make more.
func (r *restorer) repair(ctx context.Context, baseline map[violation]bool) error {
	cleared, removed := map[[2]string]int{}, map[string]int{}
	now := r.s.nowText()
	for round := 0; ; round++ {
		vs, err := r.violations(ctx)
		if err != nil {
			return err
		}
		var todo []violation
		for v := range vs {
			if !baseline[v] {
				todo = append(todo, v)
			}
		}
		if len(todo) == 0 {
			break
		}
		if round == 64 {
			return fmt.Errorf("store: restore: %d references still name removed rows", len(todo))
		}
		slices.SortFunc(todo, func(a, b violation) int {
			if c := strings.Compare(a.table, b.table); c != 0 {
				return c
			}
			return int(a.rowid - b.rowid)
		})
		for _, v := range todo {
			var fk foreignKey
			for _, f := range r.info[v.table].fks {
				if f.id == v.fkid {
					fk = f
				}
			}
			// A row put back earlier in this round settles the
			// references to it.
			if n, err := r.count(ctx, `SELECT count(*) FROM main.`+q(v.table)+` c JOIN main.`+q(fk.parent)+` p ON p.id = c.`+q(fk.from)+` WHERE c.rowid = ?`, v.rowid); err != nil {
				return err
			} else if n > 0 {
				continue
			}
			switch fk.onDelete {
			case "CASCADE":
				n, err := r.exec(ctx, `DELETE FROM main.`+q(v.table)+` WHERE rowid = ?`, v.rowid)
				if err != nil {
					return err
				}
				removed[v.table] += n
			case "SET NULL":
				n, err := r.exec(ctx, `UPDATE main.`+q(v.table)+` SET `+q(fk.from)+` = NULL WHERE rowid = ?`, v.rowid)
				if err != nil {
					return err
				}
				cleared[[2]string{v.table, fk.from}] += n
			default:
				if err := r.keepRemoved(ctx, v, fk, now); err != nil {
					return err
				}
			}
		}
	}
	for k, n := range cleared {
		r.rep.Cleared = append(r.rep.Cleared, RestoreCount{Table: k[0], Column: k[1], Rows: n})
	}
	for t, n := range removed {
		r.rep.Removed = append(r.rep.Removed, RestoreCount{Table: t, Rows: n})
	}
	slices.SortFunc(r.rep.Removed, func(a, b RestoreCount) int { return strings.Compare(a.Table, b.Table) })
	for i, tb := range r.rep.Tables {
		// The rows kept are counted on the table's first entry.
		if n := r.kept[tb.Table]; n > 0 {
			r.rep.Tables[i].KeptRemoved = n
			r.kept[tb.Table] = 0
		}
	}
	return nil
}

// keepRemoved puts back the removed row a violation names, marked
// removed.
func (r *restorer) keepRemoved(ctx context.Context, v violation, fk foreignKey, now string) error {
	var id sql.NullInt64
	if err := r.tx.QueryRowContext(ctx, `SELECT `+q(fk.from)+` FROM main.`+q(v.table)+` WHERE rowid = ?`, v.rowid).Scan(&id); err != nil {
		return fmt.Errorf("store: restore: %w", err)
	}
	refused := fmt.Errorf("store: restore: a row of %s names a row of %s that the restore removes, and %s keeps no removed rows", v.table, fk.parent, fk.parent)
	if !id.Valid {
		return refused
	}
	if n, err := r.count(ctx, `SELECT count(*) FROM main.`+q(fk.parent)+` WHERE id = ?`, id.Int64); err != nil || n > 0 {
		// Put back already, for another reference of this round.
		return err
	}
	if !r.old[fk.parent] {
		return refused
	}
	var cols, exprs []string
	for _, c := range r.info[fk.parent].cols {
		cols = append(cols, q(c.name))
		if c.name == "deleted_at" {
			exprs = append(exprs, `coalesce(deleted_at, '`+now+`')`)
		} else {
			exprs = append(exprs, q(c.name))
		}
	}
	old := `temp.` + q("restore_old_"+fk.parent)
	n, err := r.exec(ctx, `INSERT INTO main.`+q(fk.parent)+` (`+strings.Join(cols, ", ")+`) SELECT `+strings.Join(exprs, ", ")+` FROM `+old+` WHERE id = ?`, id.Int64)
	if err != nil {
		return err
	}
	if n == 0 {
		return refused
	}
	if _, err := r.exec(ctx, `DELETE FROM `+old+` WHERE id = ?`, id.Int64); err != nil {
		return err
	}
	r.kept[fk.parent] += n
	return nil
}

// cubes reports the Cubes of ScopeCubes as the store holds them after:
// whether it held each before, and its newest Cube backup.
func (r *restorer) cubes(ctx context.Context) error {
	if !r.scopes[ScopeCubes] {
		return nil
	}
	rows, err := r.tx.QueryContext(ctx, `SELECT c.serial, c.name, c.id, EXISTS (SELECT 1 FROM temp.restore_map m WHERE m.tbl = 'cubes' AND m.new = c.id AND m.mine = 1 AND m.matched = 1)
		FROM main.cubes c WHERE c.`+cubeSerials+` AND c.deleted_at IS NULL ORDER BY c.serial`)
	if err != nil {
		return fmt.Errorf("store: restore: %w", err)
	}
	type cube struct {
		RestoredCube
		id int64
	}
	var all []cube
	for rows.Next() {
		var c cube
		if err := rows.Scan(&c.Serial, &c.Name, &c.id, &c.Live); err != nil {
			rows.Close()
			return fmt.Errorf("store: restore: %w", err)
		}
		all = append(all, c)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return fmt.Errorf("store: restore: %w", err)
	}
	for _, c := range all {
		var b sql.NullString
		err := r.tx.QueryRowContext(ctx, `SELECT uuid FROM main.backups WHERE cube_id = ? AND deleted_at IS NULL ORDER BY taken_at DESC, id DESC LIMIT 1`, c.id).Scan(&b)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("store: restore: %w", err)
		}
		c.Backup = b.String
		r.rep.Cubes = append(r.rep.Cubes, c.RestoredCube)
	}
	return nil
}

// IntegrityCheck runs PRAGMA integrity_check on the store file at path,
// opened read-only and immutable: "ok", or what it found (for "Check with
// the key", AP ServerBackup, Block 5).
func IntegrityCheck(ctx context.Context, path string) ([]string, error) {
	db, err := openImmutable(path)
	if err != nil {
		return nil, err
	}
	defer db.Close()
	rows, err := db.QueryContext(ctx, `PRAGMA integrity_check`)
	if err != nil {
		return nil, fmt.Errorf("store: integrity check: %w", err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var line string
		if err := rows.Scan(&line); err != nil {
			return nil, fmt.Errorf("store: integrity check: %w", err)
		}
		out = append(out, line)
	}
	return out, rows.Err()
}
