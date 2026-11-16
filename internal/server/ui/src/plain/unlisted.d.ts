// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/plain/unlisted.d.ts
 *
 * The globals of the plain script ui/unlisted.js that the parts call: a
 * device removed from the Cube, which a room's configuration removes too.
 */

import type { Device } from "../api";

declare global {
  /** Deletes a device from the Cube and loads the rooms again; how it went, as a text. */
  function removeFromCube(d: Device): Promise<{ text: string; ok: boolean }>;
}

export {};
