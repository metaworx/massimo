// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Sources of a Cube row: where its configuration came from.
const (
	SourceEnv       = "env"       // seeded from the settings file
	SourceUI        = "ui"        // added or changed through the API
	SourceDiscovery = "discovery" // found on the LAN, not configured
	SourceFlag      = "flag"      // from the command line
)

// ErrExists is the error of adding a Cube whose serial has a row.
var ErrExists = errors.New("store: cube exists")

// ErrNotFound is the error of changing a Cube that has no row.
var ErrNotFound = errors.New("store: no such cube")

// Cube is one row of cubes: a configured Cube (Configured), or one that
// discovery found or that was removed, kept with its history.
type Cube struct {
	ID              int64
	UUID            string
	Serial          string // upper case
	Configured      bool
	Name            string
	Address         string // host[:port]; "" with FollowDiscovery: from discovery
	FollowDiscovery bool
	ProxyAddress    string // the server's listener for this Cube; "" is the default
	ProxySerial     string // the serial in discovery replies; "" is the Cube's
	Hidden          []string
	Source          string

	// What discovery reported last.
	MAC               string
	RFAddress         string
	Firmware          string
	DiscoveredAddress string
	FirstSeen         time.Time
	LastSeen          time.Time

	CreatedAt time.Time
	UpdatedAt time.Time
	Version   int64
	Origin    string
}

// cubeColumns are the columns scanCube reads, in its order.
const cubeColumns = `id, uuid, serial, configured, name, address, follow_discovery, proxy_address, proxy_serial, hidden, source,
	mac, rf_address, firmware, discovered_address, first_seen, last_seen, created_at, updated_at, version, origin`

type scanner interface{ Scan(dest ...any) error }

func scanCube(r scanner) (Cube, error) {
	var c Cube
	var hidden string
	var first, last, created, updated sql.NullString
	err := r.Scan(&c.ID, &c.UUID, &c.Serial, &c.Configured, &c.Name, &c.Address, &c.FollowDiscovery, &c.ProxyAddress, &c.ProxySerial, &hidden, &c.Source,
		&c.MAC, &c.RFAddress, &c.Firmware, &c.DiscoveredAddress, &first, &last, &created, &updated, &c.Version, &c.Origin)
	if err != nil {
		return Cube{}, err
	}
	if err := json.Unmarshal([]byte(hidden), &c.Hidden); err != nil {
		return Cube{}, fmt.Errorf("cube %s: hidden: %w", c.Serial, err)
	}
	c.FirstSeen, c.LastSeen, c.CreatedAt, c.UpdatedAt = parseTime(first), parseTime(last), parseTime(created), parseTime(updated)
	return c, nil
}

// NormalSerial is a serial as the store keeps it: trimmed, upper case.
func NormalSerial(sn string) string { return strings.ToUpper(strings.TrimSpace(sn)) }

// normalHidden lower-cases the radio addresses and drops repeats,
// keeping the order.
func normalHidden(list []string) []string {
	out := []string{}
	seen := map[string]bool{}
	for _, a := range list {
		a = strings.ToLower(strings.TrimSpace(a))
		if a != "" && !seen[a] {
			seen[a] = true
			out = append(out, a)
		}
	}
	return out
}

// Cubes returns every Cube row, configured or not, by serial.
func (s *Store) Cubes(ctx context.Context) ([]Cube, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+cubeColumns+` FROM cubes WHERE deleted_at IS NULL ORDER BY serial`)
	if err != nil {
		return nil, fmt.Errorf("store: cubes: %w", err)
	}
	defer rows.Close()
	var out []Cube
	for rows.Next() {
		c, err := scanCube(rows)
		if err != nil {
			return nil, fmt.Errorf("store: cubes: %w", err)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// Cube returns the row of a serial, and whether there is one.
func (s *Store) Cube(ctx context.Context, serial string) (Cube, bool, error) {
	return s.cube(ctx, s.db, serial)
}

type queryer interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

func (s *Store) cube(ctx context.Context, q queryer, serial string) (Cube, bool, error) {
	c, err := scanCube(q.QueryRowContext(ctx, `SELECT `+cubeColumns+` FROM cubes WHERE serial = ? AND deleted_at IS NULL`, NormalSerial(serial)))
	if errors.Is(err, sql.ErrNoRows) {
		return Cube{}, false, nil
	}
	if err != nil {
		return Cube{}, false, fmt.Errorf("store: cube %s: %w", serial, err)
	}
	return c, true, nil
}

// rowOf finds the row of a serial, a tombstoned one (deleted_at set)
// included: its id, and whether it is tombstoned.
func (s *Store) rowOf(ctx context.Context, q queryer, serial string) (id int64, tombstoned, ok bool, err error) {
	var deleted sql.NullString
	err = q.QueryRowContext(ctx, `SELECT id, deleted_at FROM cubes WHERE serial = ?`, serial).Scan(&id, &deleted)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, false, false, nil
	}
	if err != nil {
		return 0, false, false, fmt.Errorf("store: cube %s: %w", serial, err)
	}
	return id, deleted.Valid, true, nil
}

// AddCube inserts a row for c.Serial; a serial that has a live row is
// ErrExists. A tombstoned row of the serial is taken over: it gets c's
// values, its tombstone cleared and its version bumped, and keeps its
// identifier, so that what referred to it still does. The identifier,
// the times, the version and the origin are set by the store.
func (s *Store) AddCube(ctx context.Context, c Cube) (Cube, error) {
	if err := s.writable(); err != nil {
		return Cube{}, err
	}
	c.Serial = NormalSerial(c.Serial)
	if c.Serial == "" {
		return Cube{}, errors.New("store: a cube needs a serial")
	}
	if c.Source == "" {
		c.Source = SourceUI
	}
	hidden, _ := json.Marshal(normalHidden(c.Hidden))
	now := s.nowText()
	var first, last any
	if !c.FirstSeen.IsZero() {
		first = formatTime(c.FirstSeen)
	}
	if !c.LastSeen.IsZero() {
		last = formatTime(c.LastSeen)
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Cube{}, fmt.Errorf("store: add cube %s: %w", c.Serial, err)
	}
	defer tx.Rollback()
	id, tombstoned, ok, err := s.rowOf(ctx, tx, c.Serial)
	switch {
	case err != nil:
		return Cube{}, err
	case ok && !tombstoned:
		return Cube{}, fmt.Errorf("%w: %s", ErrExists, c.Serial)
	case ok:
		_, err = tx.ExecContext(ctx, `UPDATE cubes SET configured = ?, name = ?, address = ?, follow_discovery = ?, proxy_address = ?, proxy_serial = ?, hidden = ?, source = ?,
			mac = ?, rf_address = ?, firmware = ?, discovered_address = ?, first_seen = coalesce(?, first_seen), last_seen = coalesce(?, last_seen),
			deleted_at = NULL, updated_at = ?, origin = ?, version = version + 1 WHERE id = ?`,
			c.Configured, c.Name, c.Address, c.FollowDiscovery, c.ProxyAddress, c.ProxySerial, string(hidden), c.Source,
			c.MAC, c.RFAddress, c.Firmware, c.DiscoveredAddress, first, last, now, s.nodeID, id)
	default:
		_, err = tx.ExecContext(ctx, `INSERT INTO cubes (uuid, serial, configured, name, address, follow_discovery, proxy_address, proxy_serial, hidden, source,
			mac, rf_address, firmware, discovered_address, first_seen, last_seen, created_at, updated_at, origin)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			newUUID(), c.Serial, c.Configured, c.Name, c.Address, c.FollowDiscovery, c.ProxyAddress, c.ProxySerial, string(hidden), c.Source,
			c.MAC, c.RFAddress, c.Firmware, c.DiscoveredAddress, first, last, now, now, s.nodeID)
	}
	if err != nil {
		return Cube{}, fmt.Errorf("store: add cube %s: %w", c.Serial, err)
	}
	if err := tx.Commit(); err != nil {
		return Cube{}, fmt.Errorf("store: add cube %s: %w", c.Serial, err)
	}
	out, _, err := s.Cube(ctx, c.Serial)
	return out, err
}

// UpdateCube changes the row of a serial with change, in one
// transaction, and writes back the configuration (Configured, Name,
// Address, FollowDiscovery, ProxyAddress, ProxySerial, Hidden, Source);
// the discovery columns are NoteDiscovered's. A serial without a row is
// ErrNotFound; an error from change leaves the row as it was.
func (s *Store) UpdateCube(ctx context.Context, serial string, change func(*Cube) error) (Cube, error) {
	if err := s.writable(); err != nil {
		return Cube{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Cube{}, fmt.Errorf("store: cube %s: %w", serial, err)
	}
	defer tx.Rollback()
	c, ok, err := s.cube(ctx, tx, serial)
	if err != nil {
		return Cube{}, err
	}
	if !ok {
		return Cube{}, fmt.Errorf("%w: %s", ErrNotFound, NormalSerial(serial))
	}
	if err := change(&c); err != nil {
		return Cube{}, err
	}
	hidden, _ := json.Marshal(normalHidden(c.Hidden))
	if _, err := tx.ExecContext(ctx, `UPDATE cubes SET configured = ?, name = ?, address = ?, follow_discovery = ?, proxy_address = ?, proxy_serial = ?, hidden = ?, source = ?,
		updated_at = ?, origin = ?, version = version + 1 WHERE id = ?`,
		c.Configured, c.Name, c.Address, c.FollowDiscovery, c.ProxyAddress, c.ProxySerial, string(hidden), c.Source, s.nowText(), s.nodeID, c.ID); err != nil {
		return Cube{}, fmt.Errorf("store: cube %s: %w", c.Serial, err)
	}
	if err := tx.Commit(); err != nil {
		return Cube{}, fmt.Errorf("store: cube %s: %w", c.Serial, err)
	}
	out, _, err := s.Cube(ctx, c.Serial)
	return out, err
}

// Discovered is what one discovery answer says about a Cube.
type Discovered struct {
	Serial    string
	Address   string // the IP it answered from
	MAC       string // "" when unknown; a known MAC is kept then
	RFAddress string
	Firmware  string
	At        time.Time
}

// NoteDiscovered records a discovery answer. A Cube without a row gets
// one with configured = 0 and source 'discovery', which survives a
// restart as the found Cube; a tombstoned row comes back that way. Both
// report the row as new.
//
// A known row is not a configuration change: its version, updated_at
// and origin stay, which keep meaning "last changed by a user or a
// node". Its discovery columns are written only when one of them
// differs from the row (a known MAC stays when the answer has none),
// compared here rather than in SQL since some of them are content
// columns; last_seen alone is written once per run of the store, so
// that an answer every 30 minutes is not an SD card write every 30
// minutes. The Cube returned has LastSeen of this answer either way.
func (s *Store) NoteDiscovered(ctx context.Context, d Discovered) (Cube, bool, error) {
	if err := s.writable(); err != nil {
		return Cube{}, false, err
	}
	serial := NormalSerial(d.Serial)
	if serial == "" {
		return Cube{}, false, errors.New("store: a cube needs a serial")
	}
	at := d.At
	if at.IsZero() {
		at = s.now()
	}
	seen := formatTime(at)
	id, tombstoned, ok, err := s.rowOf(ctx, s.db, serial)
	if err != nil {
		return Cube{}, false, err
	}
	switch {
	case !ok:
		now := s.nowText()
		_, err = s.db.ExecContext(ctx, `INSERT INTO cubes (uuid, serial, configured, source, mac, rf_address, firmware, discovered_address, first_seen, last_seen, created_at, updated_at, origin)
			VALUES (?, ?, 0, 'discovery', ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			newUUID(), serial, d.MAC, d.RFAddress, d.Firmware, d.Address, seen, seen, now, now, s.nodeID)
	case tombstoned:
		_, err = s.db.ExecContext(ctx, `UPDATE cubes SET configured = 0, source = 'discovery', deleted_at = NULL,
			mac = CASE ? WHEN '' THEN mac ELSE ? END, rf_address = ?, firmware = ?, discovered_address = ?,
			first_seen = coalesce(first_seen, ?), last_seen = ?, updated_at = ?, origin = ?, version = version + 1 WHERE id = ?`,
			d.MAC, d.MAC, d.RFAddress, d.Firmware, d.Address, seen, seen, s.nowText(), s.nodeID, id)
	default:
		row, _, err := s.Cube(ctx, serial)
		if err != nil {
			return Cube{}, false, err
		}
		mac := row.MAC
		if d.MAC != "" {
			mac = d.MAC
		}
		changed := mac != row.MAC || d.RFAddress != row.RFAddress || d.Firmware != row.Firmware || d.Address != row.DiscoveredAddress || row.FirstSeen.IsZero()
		s.seenMu.Lock()
		first := !s.seenWritten[serial]
		s.seenWritten[serial] = true
		s.seenMu.Unlock()
		switch {
		case changed:
			_, err = s.db.ExecContext(ctx, `UPDATE cubes SET mac = ?, rf_address = ?, firmware = ?, discovered_address = ?,
				first_seen = coalesce(first_seen, ?), last_seen = ? WHERE id = ?`,
				mac, d.RFAddress, d.Firmware, d.Address, seen, seen, id)
		case first:
			_, err = s.db.ExecContext(ctx, `UPDATE cubes SET last_seen = ? WHERE id = ?`, seen, id)
		}
		if err != nil {
			return Cube{}, false, fmt.Errorf("store: cube %s: %w", serial, err)
		}
		if !changed && !first {
			row.LastSeen = at
			return row, false, nil
		}
		out, _, err := s.Cube(ctx, serial)
		out.LastSeen = at
		return out, false, err
	}
	if err != nil {
		return Cube{}, false, fmt.Errorf("store: cube %s: %w", serial, err)
	}
	c, _, err := s.Cube(ctx, serial)
	return c, true, err
}
