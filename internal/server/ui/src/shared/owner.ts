// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/shared/owner.ts
 *
 * The ways to the first owner on a server without users (AP FirstOwner,
 * Block 4), for the regions that offer them, the banner, ☰ and System
 * settings → Users, and for the sign-in card that answers them:
 * askOwner opens the card with the first owner's form (askLogin "owner"
 * of login.js) and keeps the control that asked, which the card gives
 * the focus back to when it is left; and the one answer to whether the
 * server is kept open (keptOpen), from who is in, which is read again
 * where the switch of System settings → Users turns it off (D44).
 */

import type { Me } from "../api";

/** The control that asked for the first owner's card, until the card takes it. */
let asker: HTMLElement | null = null;

/** keptOpen says whether the server is kept open without an owner: an open server whose who is in says kept_open. */
export function keptOpen(m: Me | null): boolean {
  return !!(m && m.mode === "open" && m.kept_open);
}

/** askOwner opens the sign-in card with the first owner's form, on the user's word, from (the control that asked) given the focus back when the card is left. */
export function askOwner(from: HTMLElement | null): void {
  asker = from;
  askLogin("owner");
}

/** takeAsker hands the card the control that asked for it, null where the card opened by itself; once. */
export function takeAsker(): HTMLElement | null {
  const a = asker;
  asker = null;
  return a;
}
