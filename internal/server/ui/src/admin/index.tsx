// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/admin/index.tsx
 *
 * System settings' lists as parts (AP UiParts, Block 10): Users, Roles,
 * Capabilities for every role and Sign-in providers, each drawn once
 * into its card's body and updated in place, read each time System
 * settings is shown and after each of its changes. The plain code calls
 * MassimoParts.users, for a change made in My profile
 * (profile/send.ts). Every part of the lists reads one store (store.ts),
 * since they read each other's data.
 */

import { mounter, register } from "../shared/region";
import { ProvidersCard } from "./Providers";
import { CapsCard, RolesCard } from "./Roles";
import { loadProviders, providersMsg } from "./sendProviders";
import { loadRoles, rolesMsg } from "./sendRoles";
import { forgetAdmin, reads, said } from "./store";
import { UsersCard } from "./Users";
import { loadPasswordLogin, loadUsers, usersMsg } from "./sendUsers";
import { onView } from "../shared/pages";
import { can, canManageUsersOf } from "../shared/me";

const mountUsers = mounter("sys-users-body", () => <UsersCard />);

/** The functions the plain code calls, MassimoParts.users. */
const users = register("users", {
  /** load reads the users again and draws them, for a change made in My profile (profile/send.ts). */
  async load(): Promise<void> {
    mountUsers();
    await loadUsers();
    reads.value++;
  },
});

/** MassimoParts.users, as index.ts declares it. */
export type UsersPart = typeof users;

const mountRoles = mounter("sys-roles-body", () => <RolesCard />);
const mountCaps = mounter("sys-caps-body", () => <CapsCard />);

const mountProviders = mounter("sys-providers-body", () => <ProvidersCard />);

/** signedOut forgets what the lists kept of the user who was signed in (MassimoParts.signedOut, index.ts). */
export const signedOut = forgetAdmin;

// System settings shown: the users read and drawn, the open panels' sessions, keys and identities with them
onView("system", () => {
  said.users = null;
  if (!canManageUsersOf()) return;
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
  if (!can("roles")) return;
  mountRoles();
  mountCaps();
  loadRoles().catch((e: Error) => rolesMsg(t("roles.failed", { role: "", error: e.message }), false));
});

// System settings shown: the providers read and drawn
onView("system", () => {
  said.providers = null;
  if (!can("auth")) return;
  mountProviders();
  loadProviders().catch((e: Error) => providersMsg(t("providers.failed", { name: "", error: e.message }), false));
});
