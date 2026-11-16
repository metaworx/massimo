// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/plain/cubes.d.ts
 *
 * The globals of the plain script ui/cubes.js that the parts call: the
 * Cubes' plain names, which call the part (MassimoParts.cubes) and draw
 * the Cubes card's badges in its head, the page's chrome.
 */

declare global {
  /** Draws the Cubes where they are shown, and the Cubes card's badges. */
  function renderCubes(): void;
}

export {};
