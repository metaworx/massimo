// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/admin/store.ts
 *
 * What System settings' lists of users, roles, capabilities and sign-in
 * providers are drawn from: the load counter, the lists as the last load
 * that went through gave them, what is kept per panel (open, by the
 * item's key) and per list (the new item's form shown), the message
 * lines' outcomes, and the reads a users' action asks of the open panels
 * again. One store for the four, since they read each other's data: the
 * role choice the roles, a user's identities the enabled providers, a
 * provider's role the roles. What is shown once (a key, a password, a
 * client secret) is never here: it is a part's own state alone.
 */

import { signal } from "@preact/signals";
import type { Capability, LoginProvider, Provider, Role, User } from "../api";
import type { Outcome } from "../shared/action";
import { live, liveMap } from "../shared/live";

/** The users (GET /api/v1/users), as the last load that went through gave them; loaded says that one did, failed that the newest failed. */
export const users = live({ list: [] as User[], loaded: false, failed: false });

/** The roles and the capabilities (GET /api/v1/roles), for the roles' list, the role choice and a provider's role; null where they may not be read. */
export const roles = live({ list: null as Role[] | null, caps: [] as Capability[], loaded: false });

/** The sign-in providers (GET /api/v1/providers), for the providers' list; loaded says that a load went through. */
export const providers = live({ list: [] as Provider[], loaded: false });

/** The enabled sign-in providers (GET /api/v1/login/providers), for a user's identities. */
export const enabled = signal<LoginProvider[]>([]);

/** Whether the server takes the password sign-in (the setting auth.password_login), null before it is read. */
export const passwordLogin = signal<boolean | null>(null);

/** Whether the password fields offer their eye (the setting auth.password_reveal), null before it is read. */
export const passwordReveal = signal<boolean | null>(null);

/** Keeping the server open turned off in this showing of System settings, whose row stays in sight, off, until it is shown again (AP FirstOwner, D32). */
export const keptOff = live({ showing: false });

/** Per user's login name, its panel open. */
export const userUI = liveMap(() => ({ open: false }));
/** Per identity's form key (identityFormKey), its form that changes the e-mail or the user id shown. */
export const identityUI = liveMap(() => ({ changing: false }));
/** Per role's key, its panel open. */
export const roleUI = liveMap(() => ({ open: false }));
/** Per provider's key, its panel open. */
export const providerUI = liveMap(() => ({ open: false }));

/** The new item's form of each list, shown or not. */
export const adding = live({ user: false, role: false, provider: false });

/** The outcome each list's message line shows. */
export const said = live({ users: null as Outcome | null, roles: null as Outcome | null, providers: null as Outcome | null });

/** Counts the users' changes: an open panel reads its sessions, keys and identities again at each, as the plain list's every redraw did. */
export const reads = signal(0);

/** userFormKey is the key of a user's settings form (shared/forms.ts), its identities' under it. */
export function userFormKey(name: string): string {
  return "users:" + name;
}

/** identityFormKey is the key of the form that changes a user's identity, under the user's. */
export function identityFormKey(name: string, id: number): string {
  return userFormKey(name) + ":identity:" + id;
}

/** roleFormKey is the key of a role's settings form. */
export function roleFormKey(key: string): string {
  return "roles:" + key;
}

/** providerFormKey is the key of a provider's settings form. */
export function providerFormKey(key: string): string {
  return "providers:" + key;
}

/**
 * forgetAdmin forgets what the lists kept of the user who was signed in
 * (MassimoParts.signedOut): the lists, the panels open, the new items'
 * forms and the messages; the parts are drawn from nothing.
 */
export function forgetAdmin(): void {
  users.list = [];
  users.loaded = false;
  users.failed = false;
  roles.list = null;
  roles.caps = [];
  roles.loaded = false;
  providers.list = [];
  providers.loaded = false;
  enabled.value = [];
  passwordLogin.value = null;
  passwordReveal.value = null;
  keptOff.showing = false;
  userUI.clear();
  identityUI.clear();
  roleUI.clear();
  providerUI.clear();
  adding.user = adding.role = adding.provider = false;
  said.users = said.roles = said.providers = null;
}
