// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/plain/repair.d.ts
 *
 * The globals of the plain script ui/repair.js that the parts call: a
 * Cube's Repair… and the box of its repairs, which stay plain inside its
 * panel, filled into the panel's .cube-repair-area.
 */

declare global {
  /** The scope of a box of repairs: the Cube, a room or a device of it. */
  function repairScope(serial: string, kind: string, id?: number | string): string;
  /** The button that opens and closes the box of a scope; label names what it repairs. */
  function repairButton(scope: string, kind: string, path: string, opts?: { label?: string; skip?: string[] }): HTMLButtonElement;
  /** Where a scope's box goes: the box while it is open, an empty place otherwise. */
  function repairSlot(scope: string): HTMLElement;
}

export {};
