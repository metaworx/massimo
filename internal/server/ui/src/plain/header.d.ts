// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/plain/header.d.ts
 *
 * The globals of the plain script ui/header.js that the parts call: a
 * Cube's radio budget, and its link's sign and word; the server's sign
 * drawn again.
 */

import type { Cube } from "../api";

declare global {
  /** A Cube's radio budget as the page shows it, "–" while unknown. */
  function budgetFacts(c: Cube): { duty: string; slots: string; pending: string; queued: string };
  /** The state of a Cube's network sign: "ok", "warn" or "bad". */
  function linkSign(c: Cube): string;
  /** A Cube's link in a few words, as its badge says it. */
  function linkWord(c: Cube): string;
  /** Draws the server's sign again from the server's state (a part of the header). */
  function renderServerSign(): void;
}

export {};
