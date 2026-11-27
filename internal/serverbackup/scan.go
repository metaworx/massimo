// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package serverbackup

// The scan of a target (AP ServerBackup, the analysis's "The catalogue,
// pull and push"): its backups found by their manifests, read through
// the zip's central directory with ranged reads and without the key, so
// that a target added to a new box shows its backups at once.

import (
	"context"
	"fmt"
)

// Found is a backup a scan found on a target: its file, its manifest
// and the manifest as signed, which the catalogue keeps.
type Found struct {
	Entry     Entry
	Manifest  Manifest
	Signature Signature
	Raw       Raw
}

// Skipped is a file a scan could not read as a backup, and why.
type Skipped struct {
	Name string
	Err  error
}

// Scan lists a target's backups and reads each one's manifest
// (ReadManifest) over ranged reads: the backups found, and the files
// skipped, with why: one that is no backup (ErrNotABackup, among them
// the empty file a crashed Write leaves), one of a newer format
// (ErrNewerFormat), one whose manifest's signature does not verify
// (ErrBadSignature), one that could not be read. Only a target that
// cannot be listed is an error.
func Scan(ctx context.Context, t Target) ([]Found, []Skipped, error) {
	entries, err := t.List(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("serverbackup: scan: %w", err)
	}
	found := []Found{}
	var skipped []Skipped
	for _, e := range entries {
		if err := ctx.Err(); err != nil {
			return found, skipped, fmt.Errorf("serverbackup: scan: %w", err)
		}
		m, sig, raw, err := readManifestOn(ctx, t, e.Name)
		if err == nil && !sig.Valid {
			err = ErrBadSignature
		}
		if err != nil {
			skipped = append(skipped, Skipped{Name: e.Name, Err: err})
			continue
		}
		found = append(found, Found{Entry: e, Manifest: m, Signature: sig, Raw: raw})
	}
	return found, skipped, nil
}

// readManifestOn reads the manifest of name on a target.
func readManifestOn(ctx context.Context, t Target, name string) (Manifest, Signature, Raw, error) {
	f, size, err := t.Open(ctx, name)
	if err != nil {
		return Manifest{}, Signature{}, Raw{}, err
	}
	defer f.Close()
	return ReadManifest(f, size)
}
