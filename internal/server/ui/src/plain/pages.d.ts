// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/plain/pages.d.ts
 *
 * The globals of the plain script ui/pages.js that the parts call: which
 * page is shown, a call each time one is, whether one may be opened, and
 * showing one.
 */

declare global {
  /** Whether the page of view (tiles, system, cube, …) is the one shown. */
  function isShown(view: string): boolean;
  /** Runs fn each time the page of view is shown. */
  function onView(view: string, fn: () => void): void;
  /** Whether the one signed in may open the page of view (MENU_VIEWS). */
  function mayShow(view: string): boolean;
  /** Shows the page of view (tiles, system, maint, …), as its button does. */
  function showView(view: string): void;
}

export {};
