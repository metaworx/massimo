// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/plain/app.d.ts
 *
 * The globals of the plain script ui/app.js that the parts call: the
 * page's state, the API, the reload, the ages of values, a Cube's link
 * and clock.
 */

import type { Cube, Device, FoundCube } from "../api";

declare global {
  /** What the page loaded, as far as a part reads it (app.js state). */
  const state: {
    /** The Cubes the server serves, as the last load of GET /api/v1/cubes gave them. */
    cubes: Cube[];
    /** The Cubes and proxies found on the network and not served. */
    found: FoundCube[];
    /** The Cube chosen in the header. */
    serial: string | null;
  };
  /** The element of the page with the id. */
  function $(id: string): HTMLElement | null;
  /** A request to the API with the page's token; the answer's JSON, or an error with the server's message. */
  function api<T = unknown>(method: string, path: string, body?: unknown): Promise<T>;
  /** Loads the Cubes, the rooms and the log's alerts again. */
  function refresh(): Promise<void>;
  /** A time of day, short, from milliseconds. */
  function fmtShortTime(ms: number): string;
  /** Fills an age (.age[data-at]) with how old it is; refreshAges does so once a minute. */
  function fillAge(el: HTMLElement): void;
  /** When the server first heard from the chosen Cube, in milliseconds; NaN before. */
  function observedSince(): number;
  /** How long after observedSince a value counts as already there. */
  const SINCE_MS: number;
  /** How the server stands with a Cube: connected, held, stale, busy, rebooting or offline. */
  function linkState(c: Cube): string;
  /** Why a Cube is not connected, in a sentence, where the server knows more; "" otherwise. */
  function linkText(c: Cube): string;
  /** A Cube's clock as the page shows it, and its tooltip. */
  function clockText(c: Cube): { text: string; title: string };
  /** A device's name, else its serial, else its radio address. */
  function deviceName(d: Device): string;
}

export {};
