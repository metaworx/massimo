// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/shared/me.ts
 *
 * Who the page is signed in as, as a signal: app.js' state.me is it
 * underneath (shared/data.ts), written by login.js applyCaps at every
 * read of who is in, and null from a sign-out until the next read; the
 * token the page is signed in with beside it. What who is in may do, the
 * rules of login.js line for line (can, canManageUsersOf, systemOf,
 * settingsPageOf, isMe): a part's drawing that asks one follows who is
 * in, a handler asks at that moment. A user's sessions say by
 * it which is "this device" and which key it was made from (System
 * settings → Users and My profile alike), and My profile and the
 * notifications' card are drawn for whoever it names; whether it has My
 * profile (profileOf); and who is in in
 * the page's words (whoOf): the personal page's row, ☰'s Sign out and
 * Personal settings' tooltip, and the sign-out's question say it alike;
 * and whether the first owner can be created from here (firstOwnerOf).
 */

import { computed, signal } from "@preact/signals";
import type { Me, Token } from "../api";

/** Who the page is signed in as (GET /api/v1/me), null before it is read and after a sign-out. */
export const me = signal<Me | null>(null);

/** The token the page is signed in with, null for none. */
export const own = computed<Token | null>(() => me.value?.token || null);

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

/**
 * can reports whether who is in may do something (hasCap of login.js,
 * line for line): outside users mode, and with nobody known, everyone
 * may do everything. A part's drawing that asks it follows who is in; a
 * handler asks it at that moment.
 */
export function can(c: string, m: Me | null = me.value): boolean {
  if (!m || m.mode !== "users") return true;
  return (m.capabilities || []).includes(c);
}

/** canManageUsersOf reports whether who is in may use the users' routes: with users, guests or passwords (canManageUsers of login.js). */
export function canManageUsersOf(m: Me | null = me.value): boolean {
  return can("users", m) || can("guests", m) || can("passwords", m);
}

/** systemOf reports whether who is in sees System settings or Maintenance: a capability of one of their cards (hasSystem of login.js). */
export function systemOf(m: Me | null = me.value): boolean {
  return ["cubes", "pair", "backup", "roles", "capabilities", "auth", "integrations", "update", "restart", "reboot"].some((c) => can(c, m)) || canManageUsersOf(m);
}

/** settingsPageOf reports whether who is in may open System settings: a capability of one of its cards, not of Maintenance's alone (hasSettingsPage of login.js). */
export function settingsPageOf(m: Me | null = me.value): boolean {
  return ["cubes", "pair", "backup", "roles", "capabilities", "auth", "integrations"].some((c) => can(c, m)) || canManageUsersOf(m);
}

/** isMe reports whether a user is the one signed in (isSelf of login.js). */
export function isMe(name: string, m: Me | null = me.value): boolean {
  return !!(m && m.user && m.user.name === name);
}

/**
 * firstOwnerOf says whether the first owner can be created from here: an
 * open server, a store without users (a server without a store has no
 * owner to create, AP FirstOwner D6), where who is in says the caller may
 * (setup, a caller on the local network). The sign-in card asks for the
 * owner by it, where the server is not kept open (auth.keep_open).
 */
export function firstOwnerOf(m: Me | null): boolean {
  return !!(m && m.mode === "open" && m.setup);
}
