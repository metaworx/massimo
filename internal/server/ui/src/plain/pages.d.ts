// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/plain/pages.d.ts
 *
 * The globals of the plain script ui/pages.js that the parts call: which
 * page is shown, and a call each time one is.
 */

declare global {
  /** Whether the page of view (tiles, system, cube, …) is the one shown. */
  function isShown(view: string): boolean;
  /** Runs fn each time the page of view is shown. */
  function onView(view: string, fn: () => void): void;
}

export {};
