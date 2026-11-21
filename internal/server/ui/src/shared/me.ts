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
 * profile (profileOf); and who is in in
 * the page's words (whoOf): the personal page's row, ☰'s Sign out and
 * Personal settings' tooltip, and the sign-out's question say it alike.
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

/** What the page says of who is in (whoOf): the row's line, Sign out's label (null for none) and icon, the name for ☰'s tooltip, and whether My profile is there. */
export interface Who {
  line: string;
  signOut: string | null;
  icon: string;
  name: string;
  profile: boolean;
}

/**
 * whoOf words who is in, in the page's language, for the personal page's
 * row, ☰ and the dialog that asks before a sign-out (renderMe of login.js
 * before): a session ends; an API key or the token is only forgotten
 * (Forget key, with a key's icon); an open server or one without
 * authentication has nothing to end and no profile. null is before it is
 * known and after a sign-out.
 */
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

/**
 * profileOf says whether who is in has My profile: a user signed in; an
 * open server and the server's token have none. The rule's one home: the
 * parts ask it, me.peek() where they must not follow it.
 */
export function profileOf(m: Me | null): boolean {
  return !!(m && m.mode === "users" && m.user);
}
