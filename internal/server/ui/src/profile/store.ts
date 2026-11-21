// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/profile/store.ts
 *
 * What My profile is drawn from: the signed-in user as the last read
 * that went through gave it (GET /api/v1/users/{name}, each time the
 * personal page is shown and after each change made here), the enabled
 * sign-in providers for its Link buttons, the counter at which its
 * sessions, keys and identities are read again, a link come back from a
 * provider shown before it is stored, Change password open, the key
 * the page's address marks, and the outcomes its two message lines show.
 * A password typed and a key just made are never here: they are a part's
 * own state alone.
 */

import type { LoginProvider, PendingLink, User } from "../api";
import type { Outcome } from "../shared/action";
import { live } from "../shared/live";

/** A link of the own account come back from a provider, before it is stored: its code and what the provider said (GET …/identities/link/pending/{code}). */
export interface Confirm {
  code: string;
  p: PendingLink;
}

/**
 * The signed-in user as My profile last read it, null before the page
 * was shown; the enabled providers; reads, counted at every read of it,
 * at which the sessions, keys and identities are read again; the link
 * shown to confirm; Change password open; the id of the API key whose
 * row is marked, null for none (markKey).
 */
export const profile = live({
  user: null as User | null,
  providers: [] as LoginProvider[],
  reads: 0,
  confirm: null as Confirm | null,
  passwordOpen: false,
  marked: null as number | null,
});

/** The outcomes of My profile's message line (#profile-msg) and of the password's (#me-password-msg). */
export const said = live({ profile: null as Outcome | null, password: null as Outcome | null });

/** The key of the display name's form (shared/forms.ts), and of the own password's; the card's pill shows those under "profile". */
export const NAME_FORM = "profile:name";
export const PASSWORD_FORM = "profile:password";

/**
 * forgetProfile forgets what My profile kept of the user who was signed
 * in (MassimoParts.signedOut): the user read, the providers, the link
 * shown, Change password open and the messages.
 */
export function forgetProfile(): void {
  profile.user = null;
  profile.providers = [];
  profile.confirm = null;
  profile.passwordOpen = false;
  said.profile = said.password = null;
}

/** Whether My profile's keys were drawn since the mark was set (markKey): its row was brought into view then, and the next page shown takes the mark away. */
let markDrawn = false;

/**
 * markKey sets the API key My profile marks, by its id, as the page's
 * address names it (#profile?key=<id>, MassimoParts.profile.mark from
 * pages.js readProfileHash), or none with null: its row is tinted
 * (shared/tokens.tsx) and brought into view the first time the keys are
 * drawn after it (firstDrawn), and stays tinted as they are drawn again
 * after a change there, until another page is shown, or this one anew
 * (pageShown), the address no longer naming the key. An id of no API key
 * of the user's marks nothing.
 */
export function markKey(id: number | null): void {
  profile.marked = id;
  markDrawn = false;
}

/** firstDrawn reports whether My profile's keys are drawn for the first time since the mark was set, and counts this drawing. */
export function firstDrawn(): boolean {
  const first = !markDrawn;
  markDrawn = true;
  return first;
}

/** pageShown takes the mark away once the keys were drawn with it: a page shown after that, My profile again among them, marks none (viewshown, index.tsx). */
export function pageShown(): void {
  if (markDrawn) markKey(null);
}
