// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/shared/path.ts
 *
 * A Cube's path in the API, relative to the page, which every part that
 * sends a request about a Cube builds its request's path from, Backup and
 * restore's too. The plain twin, base() of app.js (the chosen Cube's), goes
 * with its block.
 */

/** cubePath is the API path of the Cube serial. */
export function cubePath(serial: string): string {
  return "api/v1/cubes/" + encodeURIComponent(serial);
}
