// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package store

// A Cube's options beyond its row (docs/store.md section 5, migration
// 0018): whether its LAN proxy shows the rooms' virtual wall thermostats
// (docs/proxy.md). A Cube without a row has the defaults.

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// CubeOptions are a Cube's options.
type CubeOptions struct {
	// ProxyVirtual makes the proxy show clients the rooms' virtual wall
	// thermostats in place of the real wall thermostats.
	ProxyVirtual bool
}

// CubeOptions returns a Cube's options, the defaults where it has none.
func (s *Store) CubeOptions(ctx context.Context, cubeID int64) (CubeOptions, error) {
	var o CubeOptions
	err := s.db.QueryRowContext(ctx, `SELECT proxy_virtual FROM cube_options WHERE cube_id = ?`, cubeID).Scan(&o.ProxyVirtual)
	if errors.Is(err, sql.ErrNoRows) {
		return CubeOptions{}, nil
	}
	if err != nil {
		return CubeOptions{}, fmt.Errorf("store: cube options: %w", err)
	}
	return o, nil
}

// SetCubeOptions keeps a Cube's options, for the user by (0: none).
func (s *Store) SetCubeOptions(ctx context.Context, cubeID int64, o CubeOptions, by int64) error {
	if err := s.writable(); err != nil {
		return err
	}
	var user any
	if by != 0 {
		user = by
	}
	if _, err := s.db.ExecContext(ctx, `INSERT INTO cube_options (cube_id, proxy_virtual, updated_at, updated_by) VALUES (?, ?, ?, ?)
		ON CONFLICT (cube_id) DO UPDATE SET proxy_virtual = excluded.proxy_virtual, updated_at = excluded.updated_at, updated_by = excluded.updated_by`,
		cubeID, o.ProxyVirtual, s.nowText(), user); err != nil {
		return fmt.Errorf("store: cube options: %w", err)
	}
	return nil
}
