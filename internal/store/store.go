// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

// Package store is the persistent store of massimo serve: one SQLite
// file, massimo.db, in the data directory, with the Cubes, the server's
// settings and, in later stages, the users, the log, the audit and the
// backup history (docs/store.md). The engine is modernc.org/sqlite, pure
// Go, so every release target builds without cgo; nothing outside this
// package sees database/sql.
//
// Open creates the file (mode 0600) on the first start, applies the
// embedded migrations (migrate.go) and creates the node identity
// (identity.go). The typed accessors are per concern: Cubes (cubes.go),
// Settings (settings.go), and Seed (seed.go), which takes the values the
// settings file holds.
package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
	_ "modernc.org/sqlite" // registers the "sqlite" driver
)

// FileName is the database's name in the data directory.
const FileName = "massimo.db"

// timeLayout is how the store writes times: RFC 3339 in UTC with the Z
// suffix and milliseconds, fixed width, so that text order is time order.
const timeLayout = "2006-01-02T15:04:05.000Z"

// ErrRefused is the error of a file the store will not open: one it did
// not create, or one written by a newer binary that an older one cannot
// read.
var ErrRefused = errors.New("store: file refused")

// Options configure Open.
type Options struct {
	// Version is the massimo version string, written to schema_version
	// with every migration.
	Version string
	// Logger receives the store's notices (created, migrated, seeded);
	// nil is slog.Default().
	Logger *slog.Logger
	// Now is the clock; nil is time.Now. For tests.
	Now func() time.Time

	// migrations replaces the embedded migrations, for tests.
	migrations fs.FS
	// readerVersion replaces the schema version this binary is built for,
	// for tests.
	readerVersion int
}

// Store is an open massimo.db.
type Store struct {
	db       *sql.DB
	dir      string
	path     string
	log      *slog.Logger
	now      func() time.Time
	version  string
	readOnly bool

	schema    int // the file's schema version after Open
	minReader int
	created   bool // the file was created by this Open

	nodeID string
	key    nodeKey

	// seenWritten holds the serials whose last_seen this run has
	// written, see NoteDiscovered.
	seenMu      sync.Mutex
	seenWritten map[string]bool

	// The events' writer, events.go.
	ev eventWriter
}

// Path returns the database file of the data directory dir.
func Path(dir string) string { return filepath.Join(dir, FileName) }

// Open opens massimo.db in dir, creating dir and the file when they do
// not exist, applies the pending migrations and makes sure the node
// identity exists.
func Open(dir string, o Options) (*Store, error) {
	s := newStore(dir, o)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, fmt.Errorf("store: %w", err)
	}
	// SQLite would create the file with 0644 less the umask; the store
	// creates it empty with 0600 first, and the WAL and SHM files take
	// the database's mode.
	f, err := os.OpenFile(s.path, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0o600)
	switch {
	case err == nil:
		f.Close()
		s.created = true
	case !errors.Is(err, fs.ErrExist):
		return nil, fmt.Errorf("store: %w", err)
	}
	if err := s.open(false); err != nil {
		return nil, err
	}
	if s.created {
		s.log.Info("store created", "path", s.path)
	}
	if err := s.migrate(context.Background(), o.migrations, o.readerVersion); err != nil {
		s.db.Close()
		return nil, err
	}
	if err := s.loadIdentity(); err != nil {
		s.db.Close()
		return nil, err
	}
	return s, nil
}

// OpenReadOnly opens an existing massimo.db in dir without creating,
// migrating or writing anything, for massimo config and massimo store
// info. A file that does not exist is fs.ErrNotExist.
func OpenReadOnly(dir string, o Options) (*Store, error) {
	s := newStore(dir, o)
	s.readOnly = true
	if _, err := os.Stat(s.path); err != nil {
		return nil, err
	}
	if err := s.open(true); err != nil {
		return nil, err
	}
	if err := s.check(context.Background(), o.readerVersion); err != nil {
		s.db.Close()
		return nil, err
	}
	s.nodeID, _ = s.settingString(context.Background(), NodeIDKey)
	return s, nil
}

func newStore(dir string, o Options) *Store {
	s := &Store{dir: dir, path: Path(dir), log: o.Logger, now: o.Now, version: o.Version, seenWritten: map[string]bool{}}
	s.ev.init()
	if s.log == nil {
		s.log = slog.Default()
	}
	if s.now == nil {
		s.now = time.Now
	}
	if s.version == "" {
		s.version = "unknown"
	}
	return s
}

// open opens the database with the engine settings of docs/store.md
// section 3: WAL, synchronous=NORMAL, a busy timeout, foreign keys, and
// one connection, which is the one writer.
func (s *Store) open(readOnly bool) error {
	q := url.Values{}
	for _, p := range []string{"busy_timeout(5000)", "foreign_keys(1)"} {
		q.Add("_pragma", p)
	}
	if readOnly {
		q.Set("mode", "ro")
	} else {
		q.Add("_pragma", "journal_mode(WAL)")
		q.Add("_pragma", "synchronous(NORMAL)")
	}
	q.Set("_txlock", "immediate")
	db, err := sql.Open("sqlite", "file:"+filepath.ToSlash(s.path)+"?"+q.Encode())
	if err != nil {
		return fmt.Errorf("store: %w", err)
	}
	db.SetMaxOpenConns(1)
	if err := db.Ping(); err != nil {
		db.Close()
		return fmt.Errorf("store: %s: %w", s.path, err)
	}
	s.db = db
	return nil
}

// Close closes the database.
func (s *Store) Close() error { return s.db.Close() }

// Dir returns the data directory.
func (s *Store) Dir() string { return s.dir }

// File returns the database file's path.
func (s *Store) File() string { return s.path }

// Created reports whether this Open created the file.
func (s *Store) Created() bool { return s.created }

// SchemaVersion returns the file's schema version and its min_reader.
func (s *Store) SchemaVersion() (version, minReader int) { return s.schema, s.minReader }

// formatTime writes t as the store keeps times.
func formatTime(t time.Time) string { return t.UTC().Format(timeLayout) }

// parseTime reads a time the store wrote; "" or NULL is the zero time.
func parseTime(v sql.NullString) time.Time {
	if !v.Valid || v.String == "" {
		return time.Time{}
	}
	t, err := time.Parse(time.RFC3339Nano, v.String)
	if err != nil {
		return time.Time{}
	}
	return t
}

// nowText is the store's clock as text.
func (s *Store) nowText() string { return formatTime(s.now()) }

// newUUID returns a UUIDv7, the identifier of every replicated row.
func newUUID() string {
	u, err := uuid.NewV7()
	if err != nil {
		// NewV7 fails only when the system's random source does.
		panic(fmt.Sprintf("store: uuid: %v", err))
	}
	return u.String()
}

// writable refuses writes on a store opened read-only.
func (s *Store) writable() error {
	if s.readOnly {
		return errors.New("store: opened read-only")
	}
	return nil
}
