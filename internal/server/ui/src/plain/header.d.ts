// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/plain/header.d.ts
 *
 * The globals of the plain script ui/header.js that the parts call: a
 * Cube's radio budget, its link's sign and word, and its problems as the
 * entries of a problem list.
 */

import type { Cube } from "../api";
import type { Item } from "../shared/problems";

declare global {
  /**
   * A Cube's entry as cubeEntry builds it: texts and items alone, no DOM
   * node (only the header's found entry has those), so that a part can
   * hand it to Problems as an Entry.
   */
  interface CubeEntry {
    sign: { kind: string; state: string };
    title: string;
    sub: string;
    items: Item[];
    /** Its devices with a problem, "Room › Device", each with its badges. */
    subs: { icon: string; title: string; sub: string; items: Item[] }[];
  }
  /** A Cube's radio budget as the page shows it, "–" while unknown. */
  function budgetFacts(c: Cube): { duty: string; slots: string; pending: string; queued: string };
  /** The state of a Cube's network sign: "ok", "warn" or "bad". */
  function linkSign(c: Cube): string;
  /** A Cube's link in a few words, as its badge says it. */
  function linkWord(c: Cube): string;
  /** A Cube's problems as an entry of a problem list; own adds its settings' warnings. */
  function cubeEntry(c: Cube, own?: boolean): CubeEntry;
  /** Names the kinds of problems among entries, for a screen reader and the tooltip. */
  function problemsLabelOf(entries: CubeEntry[]): string;
}

export {};
