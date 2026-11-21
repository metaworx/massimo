// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/shared/session.ts
 *
 * The session's number: raised at a sign-out, and for another user's
 * sign-in after a 401 (MassimoParts.signedOut), before anything is
 * forgotten. A store that writes after an await takes the number before
 * it and drops the answer, its message and its next timer where the
 * number has changed meanwhile, so that an answer for the user who was
 * signed in reaches nothing of the next one's.
 */

let number = 0;

/** session is the session's number now, taken before an await. */
export function session(): number {
  return number;
}

/** stillSession says whether n, taken before an await, is still the session's number. */
export function stillSession(n: number): boolean {
  return n === number;
}

/** newSession raises the number: whatever a request of the session before answers is dropped. */
export function newSession(): void {
  number++;
}
