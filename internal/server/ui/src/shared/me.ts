// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/shared/me.ts
 *
 * Who the page is signed in as, as a signal: state.me as login.js
 * applyCaps hands it over at every read of who is in
 * (MassimoParts.me), and null from a sign-out until the next read; the
 * token the page is signed in with beside it. A user's sessions say by
 * it which is "this device" and which key it was made from (System
 * settings → Users and My profile alike), and My profile and the
 * notifications' card are drawn for whoever it names; whether it has My
 * profile (profileOf).
 */

import { computed, signal } from "@preact/signals";
import type { Me, Token } from "../api";

/** Who the page is signed in as (GET /api/v1/me), null before it is read and after a sign-out. */
export const me = signal<Me | null>(null);

/** The token the page is signed in with, null for none. */
export const own = computed<Token | null>(() => me.value?.token || null);

/** meChanged takes who is in, as applyCaps of login.js read it, or null at a sign-out. */
export function meChanged(m: Me | null): void {
  me.value = m;
}

/**
 * profileOf says whether who is in has My profile: a user signed in; an
 * open server and the server's token have none. The rule's one home: the
 * parts ask it (me.peek() where they must not follow it), and the plain
 * hasProfile of login.js says the same for the plain code that still asks.
 */
export function profileOf(m: Me | null): boolean {
  return !!(m && m.mode === "users" && m.user);
}
