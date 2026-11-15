// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package store

// The downgrade (docs/store.md section 5, "Migrations"; AP StoreRaise
// v1.2, Block 1, D1-D): a binary refuses a file whose min_reader is above
// its own schema version, so after a rollback of the binary the file goes
// back first. Downgrade runs the down parts of the migrations that raised
// min_reader above the older binary and of those after them, newest
// first, in one transaction with the foreign keys off and checked before
// the commit, and deletes their schema_version rows. The SQL of each
// comes from this binary where it knows the version, else from the
// file's own schema_version.down_sql (migration 0023), so that a binary
// that knows a version only by its row still undoes it. A copy of the
// file as it was comes first.

import (
	"context"
	"database/sql/driver"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strconv"
)

// ErrInUse is the error of a store that another program has open, such
// as a running serve, where a command needs it alone.
var ErrInUse = errors.New("store: file in use")

// Downgraded is what Downgrade did.
type Downgraded struct {
	// Path is the database file.
	Path string
	// From and To are the file's schema version before and after; equal
	// where there was nothing to do.
	From, To int
	// MinReader is the file's min_reader after.
	MinReader int
	// Undone are the versions whose down parts ran, newest first.
	Undone []int
	// Copy is the copy of the file as it was before, "" where nothing
	// was done.
	Copy string
}

// Downgrade takes the store in dir back to the newest schema version
// that a binary built for version to opens: the migrations from the
// first one whose min_reader is above to up to the file's version are
// undone, newest first; the additive ones before it stay, since that
// binary reads them. It needs the file alone: a running serve, or any
// other program that has it open, makes it fail with ErrInUse, and
// while it runs nobody else opens the file. A missing file is
// fs.ErrNotExist; a version without a down part, in this binary or in
// the file, is an error before anything is changed.
func Downgrade(dir string, to int, o Options) (Downgraded, error) {
	ctx := context.Background()
	s := newStore(dir, o)
	s.exclusive = true
	d := Downgraded{Path: s.path}
	if to < 1 {
		return d, fmt.Errorf("store: downgrade to version %d: a schema version is 1 or above", to)
	}
	if _, err := os.Stat(s.path); err != nil {
		return d, err
	}
	if err := s.open(false); err != nil {
		return d, s.inUse(err)
	}
	defer s.db.Close()
	if err := s.lockAlone(ctx); err != nil {
		return d, err
	}
	cur, minReader, err := s.fileVersion(ctx)
	if err != nil {
		return d, err
	}
	if cur == 0 {
		return d, fmt.Errorf("%w: %s is empty", ErrRefused, s.path)
	}
	d.From, d.To, d.MinReader = cur, cur, minReader
	if minReader <= to {
		s.log.Info("store: nothing to downgrade", "path", s.path, "schema", cur, "min_reader", minReader, "to", to)
		return d, nil
	}
	var first int
	if err := s.db.QueryRowContext(ctx, `SELECT min(version) FROM schema_version WHERE min_reader > ?`, to).Scan(&first); err != nil {
		return d, fmt.Errorf("store: downgrade: %w", err)
	}
	downs, err := s.downParts(ctx, o.migrations, first, cur)
	if err != nil {
		return d, err
	}
	if d.Copy, err = s.copyBeforeDowngrade(ctx, cur); err != nil {
		return d, err
	}
	if err := s.undo(ctx, downs, first); err != nil {
		// The file is unchanged; the copy would only be in the way.
		os.Remove(d.Copy)
		d.Copy = ""
		return d, err
	}
	if d.To, d.MinReader, err = s.fileVersion(ctx); err != nil {
		return d, err
	}
	for _, p := range downs {
		d.Undone = append(d.Undone, p.version)
	}
	s.log.Info("store downgraded", "path", s.path, "from", d.From, "to", d.To, "min_reader", d.MinReader, "copy", d.Copy)
	return d, nil
}

// FileVersion reads the schema version and the min_reader of the store
// in dir, without refusing a file this binary cannot read and without
// writing anything. A missing file is fs.ErrNotExist.
func FileVersion(dir string) (version, minReader int, err error) {
	s := newStore(dir, Options{})
	s.readOnly = true
	if _, err := os.Stat(s.path); err != nil {
		return 0, 0, err
	}
	if err := s.open(true); err != nil {
		return 0, 0, err
	}
	defer s.db.Close()
	return s.fileVersion(context.Background())
}

// lockAlone takes the file's exclusive lock, which the connection keeps
// until it closes (the store was opened with locking_mode EXCLUSIVE, in
// which the first write transaction takes it): a serve that has the
// file open holds a shared lock on it, and the busy timeout ends in
// ErrInUse.
func (s *Store) lockAlone(ctx context.Context) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return s.inUse(err)
	}
	if err := tx.Commit(); err != nil {
		return s.inUse(err)
	}
	return nil
}

// inUse turns SQLite's "database is locked" into ErrInUse, naming what
// to do; any other error is returned as it is.
func (s *Store) inUse(err error) error {
	var coded interface{ Code() int }
	if errors.As(err, &coded) && coded.Code()&0xff == sqliteBusy {
		return fmt.Errorf("%w: %s is open in another program, such as a running serve; stop the service first", ErrInUse, s.path)
	}
	return err
}

// sqliteBusy is SQLite's primary result code SQLITE_BUSY.
const sqliteBusy = 5

// downPart is the SQL that undoes one version.
type downPart struct {
	version int
	sql     string
}

// downParts collects the down parts of the versions first to cur,
// newest first: this binary's (fsys, nil the embedded migrations) where
// it has one, else the file's own. A version with neither is an error
// that names the copy made before the migration that raised min_reader.
func (s *Store) downParts(ctx context.Context, fsys fs.FS, first, cur int) ([]downPart, error) {
	if fsys == nil {
		fsys = embeddedMigrations()
	}
	ms, err := loadMigrations(fsys)
	if err != nil {
		return nil, err
	}
	known := map[int]string{}
	for _, m := range ms {
		if m.down != "" {
			known[m.version] = m.down
		}
	}
	stored := map[int]string{}
	var has int
	if err := s.db.QueryRowContext(ctx, `SELECT count(*) FROM pragma_table_info('schema_version') WHERE name = 'down_sql'`).Scan(&has); err != nil {
		return nil, fmt.Errorf("store: downgrade: %w", err)
	}
	if has > 0 {
		rows, err := s.db.QueryContext(ctx, `SELECT version, down_sql FROM schema_version WHERE version >= ? AND down_sql IS NOT NULL`, first)
		if err != nil {
			return nil, fmt.Errorf("store: downgrade: %w", err)
		}
		for rows.Next() {
			var v int
			var q string
			if err := rows.Scan(&v, &q); err != nil {
				rows.Close()
				return nil, fmt.Errorf("store: downgrade: %w", err)
			}
			stored[v] = q
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return nil, fmt.Errorf("store: downgrade: %w", err)
		}
	}
	var out []downPart
	for v := cur; v >= first; v-- {
		q, ok := known[v]
		if !ok {
			q, ok = stored[v]
		}
		if !ok {
			return nil, fmt.Errorf("store: downgrade: neither this binary nor %s has the down part of version %d; put %s.before-%d in its place instead, the copy made before version %d",
				s.path, v, s.path, first, first)
		}
		out = append(out, downPart{version: v, sql: q})
	}
	return out, nil
}

// copyBeforeDowngrade writes a consistent copy of the file, at version
// cur, to massimo.db.before-downgrade-<cur>, or with -2, -3, … where an
// earlier downgrade left that name, and returns its path. Made by root
// (the update helper), it gets the file's owner.
func (s *Store) copyBeforeDowngrade(ctx context.Context, cur int) (string, error) {
	base := s.path + ".before-downgrade-" + strconv.Itoa(cur)
	dst := base
	for n := 2; ; n++ {
		if _, err := os.Lstat(dst); errors.Is(err, fs.ErrNotExist) {
			break
		}
		dst = base + "-" + strconv.Itoa(n)
	}
	if err := s.copyTo(ctx, dst); err != nil {
		return "", fmt.Errorf("store: copy before the downgrade: %w", err)
	}
	if err := giveOwner(dst, s.path); err != nil {
		s.log.Warn("store: the copy keeps root as its owner", "copy", dst, "error", err)
	}
	s.log.Info("store copied before a downgrade", "copy", dst, "version", cur)
	return dst, nil
}

// undo runs the down parts, newest first, and deletes the schema_version
// rows from first on, in one transaction with the foreign keys off,
// checked before the commit, as apply runs a migration that makes a
// referenced table again.
func (s *Store) undo(ctx context.Context, downs []downPart, first int) error {
	conn, err := s.db.Conn(ctx)
	if err != nil {
		return fmt.Errorf("store: downgrade: %w", err)
	}
	defer conn.Close()
	if _, err := conn.ExecContext(ctx, `PRAGMA foreign_keys = OFF`); err != nil {
		return fmt.Errorf("store: downgrade: %w", err)
	}
	defer func() {
		if _, err := conn.ExecContext(context.WithoutCancel(ctx), `PRAGMA foreign_keys = ON`); err != nil {
			conn.Raw(func(any) error { return driver.ErrBadConn })
			s.log.Error("store: foreign keys not turned on again after the downgrade", "error", err)
		}
	}()
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("store: downgrade: %w", err)
	}
	defer tx.Rollback()
	for _, p := range downs {
		if _, err := tx.ExecContext(ctx, p.sql); err != nil {
			return fmt.Errorf("store: down part of version %d: %w", p.version, err)
		}
	}
	if err := foreignKeyCheck(ctx, tx); err != nil {
		return fmt.Errorf("store: downgrade: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM schema_version WHERE version >= ?`, first); err != nil {
		return fmt.Errorf("store: downgrade: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("store: downgrade: %w", err)
	}
	return nil
}
