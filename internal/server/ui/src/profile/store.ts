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
 * provider shown before it is stored, Change password open, and the
 * outcomes its two message lines show. A password typed and a key just
 * made are never here: they are a part's own state alone.
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
 * shown to confirm; Change password open.
 */
export const profile = live({
  user: null as User | null,
  providers: [] as LoginProvider[],
  reads: 0,
  confirm: null as Confirm | null,
  passwordOpen: false,
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
