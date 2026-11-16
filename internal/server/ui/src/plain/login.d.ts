// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/plain/login.d.ts
 *
 * The globals of the plain script ui/login.js that the parts call: the
 * capabilities of the one signed in.
 */

declare global {
  /** Whether the one signed in has a capability; true without users. */
  function hasCap(c: string): boolean;
}

export {};
