// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/plain/users.d.ts
 *
 * The globals of the plain script ui/users.js that the parts call: the
 * users read again, for a change made in My profile.
 */

declare global {
  /** Reads the users again and draws them (System settings → Users), for a change made in My profile. */
  function loadUsers(): Promise<void>;
}

export {};
