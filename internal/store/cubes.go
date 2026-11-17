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
	// ProxyVirtual makes the LAN proxy show its clients the rooms'
	// virtual wall thermostats in place of the real wall thermostats
	// (docs/proxy.md); cube_options before migration 0024.
	ProxyVirtual bool
	Hidden       []string
	Source       string

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
const cubeColumns = `id, uuid, serial, configured, name, address, follow_discovery, proxy_address, proxy_serial, proxy_virtual, hidden, source,
	mac, rf_address, firmware, discovered_address, first_seen, last_seen, created_at, updated_at, version, origin`

type scanner interface{ Scan(dest ...any) error }

func scanCube(r scanner) (Cube, error) {
	var c Cube
	var hidden string
	var first, last, created, updated sql.NullString
	err := r.Scan(&c.ID, &c.UUID, &c.Serial, &c.Configured, &c.Name, &c.Address, &c.FollowDiscovery, &c.ProxyAddress, &c.ProxySerial, &c.ProxyVirtual, &hidden, &c.Source,
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

// AddCube inserts a row for c.Serial; a serial that has a live row is
// ErrExists. A removed row of the serial (deleted_at set) stays as it
// is, with its history, and the Cube gets a row of its own; unless c.UUID
// names that removed row, which is restored instead: it gets c's values,
// its tombstone cleared and its version bumped, and keeps its identifier,
// so that its log, backups, rooms, commands and audit rows are the
// Cube's again (D11-A). A c.UUID that names no removed row of the serial
// is ErrNotFound. The identifier, the times, the version and the origin
// are set by the store.
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
	if _, ok, err := s.cube(ctx, tx, c.Serial); err != nil {
		return Cube{}, err
	} else if ok {
		return Cube{}, fmt.Errorf("%w: %s", ErrExists, c.Serial)
	}
	if c.UUID != "" {
		var res sql.Result
		res, err = tx.ExecContext(ctx, `UPDATE cubes SET configured = ?, name = ?, address = ?, follow_discovery = ?, proxy_address = ?, proxy_serial = ?, proxy_virtual = ?,
			hidden = ?, source = ?, mac = ?, rf_address = ?, firmware = ?, discovered_address = ?, first_seen = coalesce(?, first_seen), last_seen = coalesce(?, last_seen),
			deleted_at = NULL, updated_at = ?, origin = ?, version = version + 1 WHERE uuid = ? AND serial = ? AND deleted_at IS NOT NULL`,
			c.Configured, c.Name, c.Address, c.FollowDiscovery, c.ProxyAddress, c.ProxySerial, c.ProxyVirtual, string(hidden), c.Source,
			c.MAC, c.RFAddress, c.Firmware, c.DiscoveredAddress, first, last, now, s.nodeID, c.UUID, c.Serial)
		if err == nil {
			if n, _ := res.RowsAffected(); n == 0 {
				return Cube{}, fmt.Errorf("%w: no removed row %s of %s", ErrNotFound, c.UUID, c.Serial)
			}
		}
	} else {
		_, err = tx.ExecContext(ctx, `INSERT INTO cubes (uuid, serial, configured, name, address, follow_discovery, proxy_address, proxy_serial, proxy_virtual, hidden, source,
			mac, rf_address, firmware, discovered_address, first_seen, last_seen, created_at, updated_at, origin)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			newUUID(), c.Serial, c.Configured, c.Name, c.Address, c.FollowDiscovery, c.ProxyAddress, c.ProxySerial, c.ProxyVirtual, string(hidden), c.Source,
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

// RemovedCubes lists the removed rows of a serial, newest removal first:
// what AddCube can restore.
func (s *Store) RemovedCubes(ctx context.Context, serial string) ([]RemovedCube, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+cubeColumns+`, deleted_at FROM cubes WHERE serial = ? AND deleted_at IS NOT NULL ORDER BY deleted_at DESC, id DESC`,
		NormalSerial(serial))
	if err != nil {
		return nil, fmt.Errorf("store: removed cubes: %w", err)
	}
	defer rows.Close()
	out := []RemovedCube{}
	for rows.Next() {
		var r RemovedCube
		var at sql.NullString
		c, err := scanCube(scannerPlus{rows, &at})
		if err != nil {
			return nil, fmt.Errorf("store: removed cubes: %w", err)
		}
		r.Cube, r.RemovedAt = c, parseTime(at)
		out = append(out, r)
	}
	return out, rows.Err()
}

// hasRemoved reports whether a serial has removed rows.
func (s *Store) hasRemoved(ctx context.Context, serial string) (bool, error) {
	var removed bool
	if err := s.db.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM cubes WHERE serial = ? AND deleted_at IS NOT NULL)`, NormalSerial(serial)).Scan(&removed); err != nil {
		return false, fmt.Errorf("store: removed cubes %s: %w", serial, err)
	}
	return removed, nil
}

// RemovedCube is a removed row of cubes, with the time of its removal.
type RemovedCube struct {
	Cube
	RemovedAt time.Time
}

// scannerPlus scans a row whose first columns are another scan's and
// whose last ones go to extra.
type scannerPlus struct {
	row   scanner
	extra *sql.NullString
}

func (p scannerPlus) Scan(dest ...any) error { return p.row.Scan(append(dest, p.extra)...) }

// CubeByID returns the row of an identifier that is not removed, and
// whether there is one.
func (s *Store) CubeByID(ctx context.Context, id int64) (Cube, bool, error) {
	c, err := scanCube(s.db.QueryRowContext(ctx, `SELECT `+cubeColumns+` FROM cubes WHERE id = ? AND deleted_at IS NULL`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return Cube{}, false, nil
	}
	if err != nil {
		return Cube{}, false, fmt.Errorf("store: cube %d: %w", id, err)
	}
	return c, true, nil
}

// UpdateCube changes the row of a serial with change, in one
// transaction, and writes back the configuration (Configured, Name,
// Address, FollowDiscovery, ProxyAddress, ProxySerial, ProxyVirtual,
// Hidden, Source);
// the discovery columns are NoteDiscovered's. by is the user who made
// the change, 0 for the server itself. A serial without a row is
// ErrNotFound; an error from change leaves the row as it was.
func (s *Store) UpdateCube(ctx context.Context, serial string, change func(*Cube) error, by int64) (Cube, error) {
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
	if _, err := tx.ExecContext(ctx, `UPDATE cubes SET configured = ?, name = ?, address = ?, follow_discovery = ?, proxy_address = ?, proxy_serial = ?, proxy_virtual = ?,
		hidden = ?, source = ?, updated_at = ?, updated_by = ?, origin = ?, version = version + 1 WHERE id = ?`,
		c.Configured, c.Name, c.Address, c.FollowDiscovery, c.ProxyAddress, c.ProxySerial, c.ProxyVirtual, string(hidden), c.Source, s.nowText(), nullID(by), s.nodeID, c.ID); err != nil {
		return Cube{}, fmt.Errorf("store: cube %s: %w", c.Serial, err)
	}
	if err := tx.Commit(); err != nil {
		return Cube{}, fmt.Errorf("store: cube %s: %w", c.Serial, err)
	}
	out, _, err := s.Cube(ctx, c.Serial)
	return out, err
}

// RemoveCube removes the live row of a serial: configured = 0 and
// deleted_at set, by the user by (0 for the server itself), its version
// bumped. The row keeps its identifier, and with it its log, backups,
// rooms, commands and audit rows, until AddCube restores it; the serial
// is free for a row of its own (D11-A, D20-B). A serial without a live
// row is ErrNotFound. It returns the removed row.
func (s *Store) RemoveCube(ctx context.Context, serial string, by int64) (RemovedCube, error) {
	if err := s.writable(); err != nil {
		return RemovedCube{}, err
	}
	sn := NormalSerial(serial)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return RemovedCube{}, fmt.Errorf("store: remove cube %s: %w", sn, err)
	}
	defer tx.Rollback()
	c, ok, err := s.cube(ctx, tx, sn)
	if err != nil {
		return RemovedCube{}, err
	}
	if !ok {
		return RemovedCube{}, fmt.Errorf("%w: %s", ErrNotFound, sn)
	}
	now := s.nowText()
	if _, err := tx.ExecContext(ctx, `UPDATE cubes SET configured = 0, source = ?, deleted_at = ?, updated_at = ?, updated_by = ?, origin = ?, version = version + 1 WHERE id = ?`,
		SourceUI, now, now, nullID(by), s.nodeID, c.ID); err != nil {
		return RemovedCube{}, fmt.Errorf("store: remove cube %s: %w", sn, err)
	}
	var r RemovedCube
	var at sql.NullString
	if r.Cube, err = scanCube(scannerPlus{tx.QueryRowContext(ctx, `SELECT `+cubeColumns+`, deleted_at FROM cubes WHERE id = ?`, c.ID), &at}); err != nil {
		return RemovedCube{}, fmt.Errorf("store: remove cube %s: %w", sn, err)
	}
	r.RemovedAt = parseTime(at)
	if err := tx.Commit(); err != nil {
		return RemovedCube{}, fmt.Errorf("store: remove cube %s: %w", sn, err)
	}
	return r, nil
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

// NoteDiscovered records a discovery answer. A Cube without a live row
// gets one with configured = 0 and source 'discovery', which survives a
// restart as the found Cube, and is reported as new; but for a serial
// with removed rows (RemoveCube), which gets no row and is returned as
// the answer says it, not new: adding it makes a row, or restores one
// (AddCube). A removed row stays removed (RemovedCubes).
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
	row, ok, err := s.Cube(ctx, serial)
	if err != nil {
		return Cube{}, false, err
	}
	if !ok {
		// A serial with removed rows gets no row of its own until it is
		// added, as new or restored (D20-B): its found entry is the
		// server's, in memory, and the next start's first run finds it
		// again.
		removed, err := s.hasRemoved(ctx, serial)
		if err != nil {
			return Cube{}, false, err
		}
		if removed {
			return Cube{Serial: serial, MAC: d.MAC, RFAddress: d.RFAddress, Firmware: d.Firmware, DiscoveredAddress: d.Address, FirstSeen: at, LastSeen: at}, false, nil
		}
	}
	switch {
	case !ok:
		now := s.nowText()
		_, err = s.db.ExecContext(ctx, `INSERT INTO cubes (uuid, serial, configured, source, mac, rf_address, firmware, discovered_address, first_seen, last_seen, created_at, updated_at, origin)
			VALUES (?, ?, 0, 'discovery', ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			newUUID(), serial, d.MAC, d.RFAddress, d.Firmware, d.Address, seen, seen, now, now, s.nodeID)
	default:
		id := row.ID
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
