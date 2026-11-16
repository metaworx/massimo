// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package store

// Migrations: migrations/NNNN_<name>.sql, embedded, applied in order in
// one transaction each, each recorded in schema_version with its
// min_reader (docs/store.md section 5, "Migrations"). Every file
// declares its min_reader in a comment line "-- min_reader: N". A
// migration that keeps the previous min_reader is additive; one that
// raises it changes a meaning, and the store copies the file to
// massimo.db.before-<version> before applying it, so that an older
// binary has a file it can open after a rollback. A file with the line
// "-- foreign_keys: off" runs with foreign keys off, for one that makes a
// referenced table again, and has them checked before its commit. A
// migration that raises min_reader comes with NNNN_<name>.down.sql, the
// SQL that undoes it, which the store also keeps in schema_version
// (migration 0023) and massimo store downgrade runs (downgrade.go); so
// does every migration after the first that raised it, 0024. Only
// the numbered files are embedded: migrations/store.sql beside them is
// the schema they make, generated for reading (migrate_test.go,
// TestStoreSQL).

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"go.metaworx.ch/massimo/internal/safefile"
)

//go:embed migrations/[0-9]*.sql
var embedded embed.FS

// migration is one numbered SQL file.
type migration struct {
	version   int
	minReader int
	name      string
	sql       string
	// foreignKeysOff runs the migration with foreign keys off: DROP
	// TABLE with them on deletes the rows first, with the actions of
	// the keys that reference them.
	foreignKeysOff bool
	// down is the SQL of NNNN_<name>.down.sql, which undoes the
	// migration; "" where there is none.
	down string
}

var (
	migrationName      = regexp.MustCompile(`^(\d{4})_[a-z0-9_]+\.sql$`)
	downName           = regexp.MustCompile(`^\d{4}_[a-z0-9_]+\.down\.sql$`)
	minReaderLine      = regexp.MustCompile(`(?m)^--\s*min_reader:\s*(\d+)\s*$`)
	foreignKeysOffLine = regexp.MustCompile(`(?m)^--\s*foreign_keys:\s*off\s*$`)
)

// embeddedMigrations is the migration directory compiled in.
func embeddedMigrations() fs.FS {
	sub, err := fs.Sub(embedded, "migrations")
	if err != nil {
		panic(err) // the directory is compiled in
	}
	return sub
}

// SchemaVersion is the schema version this binary is built for: the
// number of its last migration.
func SchemaVersion() int {
	ms, err := loadMigrations(embeddedMigrations())
	if err != nil || len(ms) == 0 {
		return 0
	}
	return ms[len(ms)-1].version
}

// loadMigrations reads the migrations of fsys, checked: numbered from 1
// without gaps, each with a min_reader that never falls and never
// exceeds its own version, and each down part beside the migration of
// its name.
func loadMigrations(fsys fs.FS) ([]migration, error) {
	entries, err := fs.ReadDir(fsys, ".")
	if err != nil {
		return nil, fmt.Errorf("store: migrations: %w", err)
	}
	var out []migration
	downs := map[string]string{} // the up file's name: its down part
	for _, e := range entries {
		if downName.MatchString(e.Name()) {
			b, err := fs.ReadFile(fsys, e.Name())
			if err != nil {
				return nil, fmt.Errorf("store: migrations: %w", err)
			}
			downs[strings.TrimSuffix(e.Name(), ".down.sql")+".sql"] = string(b)
			continue
		}
		m := migrationName.FindStringSubmatch(e.Name())
		if m == nil {
			continue
		}
		b, err := fs.ReadFile(fsys, e.Name())
		if err != nil {
			return nil, fmt.Errorf("store: migrations: %w", err)
		}
		v, _ := strconv.Atoi(m[1])
		mr := minReaderLine.FindStringSubmatch(string(b))
		if mr == nil {
			return nil, fmt.Errorf("store: migration %s declares no min_reader", e.Name())
		}
		r, _ := strconv.Atoi(mr[1])
		out = append(out, migration{version: v, minReader: r, name: e.Name(), sql: string(b), foreignKeysOff: foreignKeysOffLine.Match(b)})
	}
	for i := range out {
		if d, ok := downs[out[i].name]; ok {
			out[i].down = d
			delete(downs, out[i].name)
		}
	}
	for up := range downs {
		return nil, fmt.Errorf("store: migration %s.down.sql: there is no %s", strings.TrimSuffix(up, ".sql"), up)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].version < out[j].version })
	prev := 0
	for i, m := range out {
		if m.version != i+1 {
			return nil, fmt.Errorf("store: migration %s: expected version %d", m.name, i+1)
		}
		if m.minReader < prev || m.minReader > m.version {
			return nil, fmt.Errorf("store: migration %s: min_reader %d must lie between %d and %d", m.name, m.minReader, prev, m.version)
		}
		prev = m.minReader
	}
	return out, nil
}

// fileVersion reads the file's schema version and min_reader; both are
// 0 for an empty file. A file with tables but no schema_version is
// refused.
func (s *Store) fileVersion(ctx context.Context) (version, minReader int, err error) {
	var tables, sv int
	if err := s.db.QueryRowContext(ctx, `SELECT count(*), coalesce(sum(name = 'schema_version'), 0) FROM sqlite_master WHERE type = 'table'`).Scan(&tables, &sv); err != nil {
		return 0, 0, fmt.Errorf("store: %s: %w", s.path, err)
	}
	if tables == 0 {
		return 0, 0, nil
	}
	if sv == 0 {
		return 0, 0, fmt.Errorf("%w: %s has tables but no schema_version, so massimo did not create it; move it away or give another -data", ErrRefused, s.path)
	}
	err = s.db.QueryRowContext(ctx, `SELECT version, min_reader FROM schema_version ORDER BY version DESC LIMIT 1`).Scan(&version, &minReader)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, 0, fmt.Errorf("%w: %s has an empty schema_version", ErrRefused, s.path)
	}
	if err != nil {
		return 0, 0, fmt.Errorf("store: %s: %w", s.path, err)
	}
	return version, minReader, nil
}

// readable refuses a file whose min_reader is above reader, naming the
// copy that an older binary can open.
func (s *Store) readable(ctx context.Context, version, minReader, reader int) error {
	if minReader <= reader {
		return nil
	}
	// The first migration that raised min_reader above this binary left
	// the copy before it.
	copyAt := version
	s.db.QueryRowContext(ctx, `SELECT min(version) FROM schema_version WHERE min_reader > ?`, reader).Scan(&copyAt)
	return fmt.Errorf("%w: %s is at schema version %d and needs a binary for version %d or later; this one is for version %d. Install the newer release again, or take the file back for this one with the newer binary's massimo store downgrade -to %d, or put %s.before-%d in its place to go back to the state before that migration",
		ErrRefused, s.path, version, minReader, reader, reader, s.path, copyAt)
}

// check opens nothing new: it reads the file's version and refuses what
// this binary cannot read.
func (s *Store) check(ctx context.Context, reader int) error {
	if reader == 0 {
		reader = SchemaVersion()
	}
	v, mr, err := s.fileVersion(ctx)
	if err != nil {
		return err
	}
	if v == 0 {
		return fmt.Errorf("%w: %s is empty", ErrRefused, s.path)
	}
	if err := s.readable(ctx, v, mr, reader); err != nil {
		return err
	}
	s.schema, s.minReader = v, mr
	return nil
}

// migrate applies the migrations of fsys (nil: the embedded ones) that
// the file lacks. reader is the schema version this binary reads (0:
// the last migration's).
func (s *Store) migrate(ctx context.Context, fsys fs.FS, reader int) error {
	if fsys == nil {
		fsys = embeddedMigrations()
	}
	ms, err := loadMigrations(fsys)
	if err != nil {
		return err
	}
	if reader == 0 && len(ms) > 0 {
		reader = ms[len(ms)-1].version
	}
	cur, minReader, err := s.fileVersion(ctx)
	if err != nil {
		return err
	}
	if err := s.readable(ctx, cur, minReader, reader); err != nil {
		return err
	}
	if cur > reader {
		// A newer, additive file (a rollback of the binary): readable,
		// and what this binary does not know it ignores.
		s.log.Info("store is at a newer schema version, opened as it is", "path", s.path, "schema", cur, "binary", reader)
		s.schema, s.minReader = cur, minReader
		return s.keepDownParts(ctx, ms)
	}
	for _, m := range ms {
		if m.version <= cur || m.version > reader {
			continue
		}
		copied := ""
		if cur > 0 && m.minReader > minReader {
			if copied, err = s.copyBefore(ctx, m.version, cur); err != nil {
				return err
			}
		}
		if err := s.apply(ctx, m); err != nil {
			if copied != "" {
				// The file is unchanged; a copy left behind would only be
				// in the way of the next start.
				os.Remove(copied)
			}
			return err
		}
		s.log.Info("schema migrated", "path", s.path, "version", m.version, "min_reader", m.minReader)
		cur, minReader = m.version, m.minReader
	}
	s.schema, s.minReader = cur, minReader
	return s.keepDownParts(ctx, ms)
}

// keepDownParts writes the down part of each migration of ms into its
// schema_version row where the row has none, so that a binary that
// knows the version only by its row can still undo it (downgrade.go).
// A file before migration 0023 has no column for them.
func (s *Store) keepDownParts(ctx context.Context, ms []migration) error {
	var has int
	if err := s.db.QueryRowContext(ctx, `SELECT count(*) FROM pragma_table_info('schema_version') WHERE name = 'down_sql'`).Scan(&has); err != nil {
		return fmt.Errorf("store: down parts: %w", err)
	}
	if has == 0 {
		return nil
	}
	missing := map[int]bool{}
	rows, err := s.db.QueryContext(ctx, `SELECT version FROM schema_version WHERE down_sql IS NULL`)
	if err != nil {
		return fmt.Errorf("store: down parts: %w", err)
	}
	for rows.Next() {
		var v int
		if err := rows.Scan(&v); err != nil {
			rows.Close()
			return fmt.Errorf("store: down parts: %w", err)
		}
		missing[v] = true
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return fmt.Errorf("store: down parts: %w", err)
	}
	for _, m := range ms {
		if m.down == "" || !missing[m.version] {
			continue
		}
		if _, err := s.db.ExecContext(ctx, `UPDATE schema_version SET down_sql = ? WHERE version = ? AND down_sql IS NULL`, m.down, m.version); err != nil {
			return fmt.Errorf("store: down part of %s: %w", m.name, err)
		}
		s.log.Info("store: down part kept", "path", s.path, "version", m.version)
	}
	return nil
}

// apply runs one migration and records it, in one transaction; on the
// one connection, whose foreign keys a migration may turn off around its
// transaction, since SQLite ignores the pragma inside one.
func (s *Store) apply(ctx context.Context, m migration) error {
	conn, err := s.db.Conn(ctx)
	if err != nil {
		return fmt.Errorf("store: migration %s: %w", m.name, err)
	}
	defer conn.Close()
	if m.foreignKeysOff {
		if _, err := conn.ExecContext(ctx, `PRAGMA foreign_keys = OFF`); err != nil {
			return fmt.Errorf("store: migration %s: %w", m.name, err)
		}
		defer func() {
			if _, err := conn.ExecContext(context.WithoutCancel(ctx), `PRAGMA foreign_keys = ON`); err != nil {
				// The connection would go on without its keys: drop it.
				conn.Raw(func(any) error { return driver.ErrBadConn })
				s.log.Error("store: foreign keys not turned on again", "migration", m.name, "error", err)
			}
		}()
	}
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("store: migration %s: %w", m.name, err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, m.sql); err != nil {
		return fmt.Errorf("store: migration %s: %w", m.name, err)
	}
	if m.foreignKeysOff {
		if err := foreignKeyCheck(ctx, tx); err != nil {
			return fmt.Errorf("store: migration %s: %w", m.name, err)
		}
	}
	if hook := afterMigration[m.version]; hook != nil {
		if err := hook(ctx, s, tx); err != nil {
			return fmt.Errorf("store: migration %s: %w", m.name, err)
		}
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO schema_version (version, min_reader, applied_at, applied_by) VALUES (?, ?, ?, ?)`,
		m.version, m.minReader, s.nowText(), s.version); err != nil {
		return fmt.Errorf("store: migration %s: %w", m.name, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("store: migration %s: %w", m.name, err)
	}
	return nil
}

// foreignKeyCheck fails on the first row whose foreign key names no row,
// as PRAGMA foreign_key_check reports it.
func foreignKeyCheck(ctx context.Context, tx *sql.Tx) error {
	rows, err := tx.QueryContext(ctx, `PRAGMA foreign_key_check`)
	if err != nil {
		return err
	}
	defer rows.Close()
	if rows.Next() {
		var table, parent string
		var rowid sql.NullInt64
		var fk int
		if err := rows.Scan(&table, &rowid, &parent, &fk); err != nil {
			return err
		}
		return fmt.Errorf("a row of %s (rowid %d) names no row of %s", table, rowid.Int64, parent)
	}
	return rows.Err()
}

// afterMigration holds what a migration needs beyond SQL, run in its
// transaction: the first seeds the node and tenant identifiers, 0024
// gives the roles their uuids.
var afterMigration = map[int]func(context.Context, *Store, *sql.Tx) error{
	1:  seedIdentifiers,
	24: roleUUIDs,
}

// roleUUIDs gives every role the UUIDv7 that migration 0024 left as
// raise-<id>, since SQL makes none.
func roleUUIDs(ctx context.Context, _ *Store, tx *sql.Tx) error {
	rows, err := tx.QueryContext(ctx, `SELECT id FROM roles WHERE uuid = 'raise-' || id`)
	if err != nil {
		return err
	}
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		ids = append(ids, id)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	for _, id := range ids {
		if _, err := tx.ExecContext(ctx, `UPDATE roles SET uuid = ? WHERE id = ?`, newUUID(), id); err != nil {
			return err
		}
	}
	return nil
}

// copyBefore writes a consistent copy of the file, as it is before the
// migration to version, to massimo.db.before-<version>, mode 0600, and
// returns its path; cur is the file's version. A copy left from an
// earlier attempt (a crash between the copy and the commit) is of the
// same state while the file is still at version-1, so it is replaced;
// a file that has moved on is refused, since the copy is then the only
// state an older binary can read.
func (s *Store) copyBefore(ctx context.Context, version, cur int) (string, error) {
	dst := s.path + ".before-" + strconv.Itoa(version)
	if cur != version-1 {
		return "", fmt.Errorf("store: %s is at version %d, not %d; the copy %s is not made again", s.path, cur, version-1, dst)
	}
	if _, err := os.Stat(dst); err == nil {
		s.log.Info("store: replacing a copy left by an earlier attempt", "copy", dst, "version", version)
		if err := os.Remove(dst); err != nil {
			return "", fmt.Errorf("store: %w", err)
		}
	}
	if err := s.copyTo(ctx, dst); err != nil {
		return "", fmt.Errorf("store: copy before migration %d: %w", version, err)
	}
	s.log.Info("store copied before a migration that older binaries cannot read", "copy", dst, "version", version)
	return dst, nil
}

// copyTo writes a consistent copy of the database to dst, which must
// not exist, with mode 0600: the WAL is checkpointed and VACUUM INTO
// writes the copy in one read transaction, so a running server's last
// writes are in it. dst is taken first, empty, so that nobody else does;
// the copy goes to a temporary file beside it and is renamed over it once
// complete (internal/safefile), so that a crash or a full disk leaves dst
// empty, never a partial database.
func (s *Store) copyTo(ctx context.Context, dst string) error {
	f, err := os.OpenFile(dst, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	f.Close()
	if !s.readOnly {
		if _, err := s.db.ExecContext(ctx, `PRAGMA wal_checkpoint(TRUNCATE)`); err != nil {
			os.Remove(dst)
			return err
		}
	}
	err = safefile.Replace(dst, safefile.Options{Mode: 0o600, Logger: s.log}, func(tmp string) error {
		_, err := s.db.ExecContext(ctx, `VACUUM INTO ?`, tmp)
		return err
	})
	if err != nil {
		os.Remove(dst)
		return err
	}
	return nil
}
