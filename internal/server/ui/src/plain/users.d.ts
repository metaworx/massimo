// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/plain/users.d.ts
 *
 * The globals of the plain script ui/users.js that the parts call: the
 * users read again, for a change made in My profile; what
 * is wrong with a new user's name, as the first owner's form checks it
 * too, how long a name typed waits before it is checked, and the server's
 * refusal of a name worded as the check words it.
 */

declare global {
  /** Reads the users again and draws them (System settings → Users), for a change made in My profile. */
  function loadUsers(): Promise<void>;
  /** How long a name being typed waits before it is checked, in ms. */
  const NAME_CHECK_MS: number;
  /** What is wrong with a new user's name, "" for nothing: the store's rule, or a name in use among taken. */
  function nameProblem(name: string, taken: string[]): string;
  /** The server's refusal of a new user's name worded as the check does, "" for another error. */
  function wordNameError(e: Error & { status?: number }, name: string): string;
}

export {};
