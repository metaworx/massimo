// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/chrome/me.ts
 *
 * Who is in, in the page's chrome's words (renderMe of login.js before,
 * AP UiParts, Block 14): what the personal page's row says of the one
 * signed in, what Sign out does there and in ☰, and whom ☰'s Personal
 * settings names, all from who is in as login.js hands it over
 * (shared/me.ts). A session ends; an API key or the token is only
 * forgotten (Forget key, with a key's icon); an open server or one
 * without authentication has nothing to end and no profile.
 */

import type { Me } from "../api";
import { profileOf } from "../shared/me";

/** What the chrome says of who is in: the row's line, Sign out's label (null for none) and icon, the name for ☰'s tooltip, and whether My profile is there. */
export interface Who {
  line: string;
  signOut: string | null;
  icon: string;
  name: string;
  profile: boolean;
}

/** whoOf words who is in, in the page's language; null before it is known and after a sign-out. */
export function whoOf(me: Me | null): Who {
  let line = "";
  let signOut: string | null = null;
  if (me && me.mode === "users" && me.user) {
    line = t("me.signedIn", { name: me.user.display_name || me.user.name, role: roleLabel(me.role?.key || "", me.role?.name) });
    if (me.token && me.token.kind === "api") {
      line = t("me.withKey", { name: me.user.name, label: me.token.label || "" });
      signOut = t("me.forgetKey");
    } else {
      signOut = t("me.signOut");
    }
  } else if (me && me.mode === "open") {
    line = t("me.open");
  } else if (me && me.mode === "token") {
    line = t("me.token");
    signOut = t("me.forgetKey");
  } else if (me) {
    line = t("me.none");
  }
  const keyOnly = !!me && ((!!me.token && me.token.kind === "api") || me.mode === "token");
  const u = me && me.user;
  const name = u ? (u.display_name && u.display_name !== u.name ? u.display_name + " (" + u.name + ")" : u.name) : "";
  return { line, signOut, icon: keyOnly ? "icon-key" : "icon-signout", name, profile: profileOf(me) };
}
