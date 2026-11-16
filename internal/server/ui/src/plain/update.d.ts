// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/plain/update.d.ts
 *
 * The globals of the plain script ui/update.js that the parts call:
 * whether a software update is out, and its version, for the server's
 * sign.
 */

declare global {
  /** Whether an update is out, for one who may install it. */
  function updateOut(): boolean;
  /** The version an update would install, "" for none. */
  function updateLatest(): string;
}

export {};
