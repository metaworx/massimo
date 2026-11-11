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
// binary has a file it can open after a rollback. Only the numbered
// files are embedded: migrations/store.sql beside them is the schema
// they make, generated for reading (migrate_test.go, TestStoreSQL).

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"regexp"
	"sort"
	"strconv"

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
}

var (
	migrationName = regexp.MustCompile(`^(\d{4})_[a-z0-9_]+\.sql$`)
	minReaderLine = regexp.MustCompile(`(?m)^--\s*min_reader:\s*(\d+)\s*$`)
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
// exceeds its own version.
func loadMigrations(fsys fs.FS) ([]migration, error) {
	entries, err := fs.ReadDir(fsys, ".")
	if err != nil {
		return nil, fmt.Errorf("store: migrations: %w", err)
	}
	var out []migration
	for _, e := range entries {
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
		out = append(out, migration{version: v, minReader: r, name: e.Name(), sql: string(b)})
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
	return fmt.Errorf("%w: %s is at schema version %d and needs a binary for version %d or later; this one is for version %d. Install the newer release again, or put %s.before-%d in its place to go back to the state before that migration",
		ErrRefused, s.path, version, minReader, reader, s.path, copyAt)
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
		return nil
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
	return nil
}

// apply runs one migration and records it, in one transaction.
func (s *Store) apply(ctx context.Context, m migration) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("store: migration %s: %w", m.name, err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, m.sql); err != nil {
		return fmt.Errorf("store: migration %s: %w", m.name, err)
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

// afterMigration holds what a migration needs beyond SQL, run in its
// transaction: the first seeds the node and tenant identifiers.
var afterMigration = map[int]func(context.Context, *Store, *sql.Tx) error{
	1: seedIdentifiers,
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
