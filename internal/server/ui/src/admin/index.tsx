// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/admin/index.tsx
 *
 * System settings' lists as parts (AP UiParts, Block 10): Users, Roles,
 * Capabilities for every role and Sign-in providers, each drawn once
 * into its card's body and updated in place, read each time System
 * settings is shown and after each of its changes. The plain code calls
 * MassimoParts.users (users.js loadUsers and renderUsers, for a change
 * made in My profile and a change of language), MassimoParts.roles
 * (roles.js renderRoles) and MassimoParts.providers (providers.js
 * renderProviders). Every part of the lists reads one store (store.ts),
 * since they read each other's data.
 */

import { mounter, register } from "../shared/region";
import { ProvidersCard } from "./Providers";
import { CapsCard, RolesCard } from "./Roles";
import { loadProviders, providersMsg } from "./sendProviders";
import { loadRoles, rolesMsg } from "./sendRoles";
import { forgetAdmin, load, reads, said } from "./store";
import { UsersCard } from "./Users";
import { loadPasswordLogin, loadUsers, usersMsg } from "./sendUsers";

const mountUsers = mounter("sys-users-body", () => <UsersCard />);

/** The functions the plain code calls, MassimoParts.users. */
const users = register("users", {
  /** load reads the users again and draws them (users.js loadUsers), for a change made in My profile. */
  async load(): Promise<void> {
    mountUsers();
    await loadUsers();
    reads.value++;
  },
  /** render draws the users again (users.js renderUsers). */
  render(): void {
    mountUsers();
    load.bump();
  },
});

/** MassimoParts.users, as index.ts declares it. */
export type UsersPart = typeof users;

const mountRoles = mounter("sys-roles-body", () => <RolesCard />);
const mountCaps = mounter("sys-caps-body", () => <CapsCard />);

/** The functions the plain code calls, MassimoParts.roles. */
const roles = register("roles", {
  /** render draws the roles and the capabilities again (roles.js renderRoles). */
  render(): void {
    mountRoles();
    mountCaps();
    load.bump();
  },
});

/** MassimoParts.roles, as index.ts declares it. */
export type RolesPart = typeof roles;

const mountProviders = mounter("sys-providers-body", () => <ProvidersCard />);

/** The functions the plain code calls, MassimoParts.providers. */
const providers = register("providers", {
  /** render draws the providers again (providers.js renderProviders). */
  render(): void {
    mountProviders();
    load.bump();
  },
});

/** MassimoParts.providers, as index.ts declares it. */
export type ProvidersPart = typeof providers;

/** signedOut forgets what the lists kept of the user who was signed in (MassimoParts.signedOut, index.ts). */
export const signedOut = forgetAdmin;

// System settings shown: the users read and drawn, the open panels' sessions, keys and identities with them
onView("system", () => {
  said.users = null;
  if (!canManageUsers()) return;
  mountUsers();
  loadUsers()
    .then(() => {
      reads.value++;
      return loadPasswordLogin();
    })
    .catch((e: Error) => usersMsg(t("users.failed", { name: "", error: e.message }), false));
});

// System settings shown: the roles and the capabilities read and drawn
onView("system", () => {
  said.roles = null;
  if (!hasCap("roles")) return;
  mountRoles();
  mountCaps();
  loadRoles().catch((e: Error) => rolesMsg(t("roles.failed", { role: "", error: e.message }), false));
});

// System settings shown: the providers read and drawn
onView("system", () => {
  said.providers = null;
  if (!hasCap("auth")) return;
  mountProviders();
  loadProviders().catch((e: Error) => providersMsg(t("providers.failed", { name: "", error: e.message }), false));
});
