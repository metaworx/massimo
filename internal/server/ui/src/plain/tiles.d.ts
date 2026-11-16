// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/plain/tiles.d.ts
 *
 * The globals of the plain script ui/tiles.js that the parts call: a
 * device's type icon and its problems as items of a problem list, which
 * a Cube's entry lists under it (model/cube.tsx).
 */

import type { Device } from "../api";
import type { Item } from "../shared/problems";

declare global {
  /** The symbol of a device's type: a radiator, a wall thermostat, a window as the person chooses it, the eco button's leaf. */
  function deviceIcon(d: Device): string;
  /** What is wrong with a device, as items of a problem list; withWindow, an open window too. */
  function deviceProblems(d: Device, withWindow?: boolean): Item[];
}

export {};
