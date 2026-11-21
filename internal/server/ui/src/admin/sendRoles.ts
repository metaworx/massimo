// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/admin/sendRoles.ts
 *
 * What System settings → Roles and Capabilities for every role read and
 * send, line for line as roles.js did: the roles with every capability
 * and whether it is disabled for every role (GET /api/v1/roles), a
 * role's change, its disabling and its removal (PUT, PUT …/disabled,
 * DELETE …/roles/{key}), a new role (POST), and a capability switched off
 * or on for every role but the owner (PUT /api/v1/capabilities/{key}). A
 * change reads the roles again and says how it went under the roles; a
 * role gone from a load that went through forgets its form and its open
 * panel; an answer after a sign-out is dropped (shared/session.ts).
 */

import type { Capability, Role } from "../api";
import { actionRunner } from "../shared/action";
import { capLabel } from "../model/cap";
import { forgetForms } from "../shared/forms";
import { session, stillSession } from "../shared/session";
import { load, roleFormKey, roleUI, roles, said } from "./store";

/** rolePath is a role's path in the API, rest after it. */
function rolePath(key: string, rest = ""): string {
  return "api/v1/roles/" + encodeURIComponent(key) + rest;
}

/**
 * loadRoles reads the roles and the capabilities, for a holder of roles;
 * a role the list no longer gives forgets its form. A load that fails
 * throws before it and leaves the lists as they were.
 */
export async function loadRoles(): Promise<void> {
  if (!hasCap("roles")) return;
  const n = session();
  const res = (await api<{ roles?: Role[]; capabilities?: Capability[] }>("GET", "api/v1/roles")) || {};
  if (!stillSession(n)) return; // signed out meanwhile: nothing of it is the next user's
  const list = res.roles || [];
  const now = new Set(list.map((r) => r.key));
  for (const r of roles.list || []) {
    if (now.has(r.key)) continue;
    forgetForms(roleFormKey(r.key));
    roleUI.forget(r.key); // made again under the same key, it does not open by itself
  }
  roles.list = list;
  roles.caps = res.capabilities || [];
  roles.loaded = true;
  load.bump();
}

/** The roles' runner: its message under the roles, the roles read again after a change. */
const runner = actionRunner({
  set: (res) => {
    said.roles = res;
  },
  reload: loadRoles,
  bound: true,
});

/** rolesMsg shows text under the roles, ok green, else red; no text hides it. */
export const rolesMsg = runner.say;

/** run sends a change of the role or capability key, says ok, or that it failed and why. */
function run(key: string, fn: () => Promise<unknown>, ok: string): Promise<boolean> {
  return runner.run(fn, ok, (error) => t("roles.failed", { role: key, error }));
}

/** saveRole sends a role's change, its name and its set where they differ (PUT). */
export function saveRole(r: Role, body: Record<string, unknown>): Promise<boolean> {
  return run(r.key, () => api("PUT", rolePath(r.key), body), t("roles.saved", { role: r.key }));
}

/** disableRole disables a role, or enables it again (PUT …/disabled). */
export function disableRole(r: Role): Promise<boolean> {
  return run(r.key, () => api("PUT", rolePath(r.key, "/disabled"), { disabled: !r.disabled }),
    t(r.disabled ? "roles.enabledNow" : "roles.disabledNow", { role: r.key }));
}

/** removeRole removes a created role nobody has (DELETE). */
export function removeRole(r: Role): Promise<boolean> {
  return run(r.key, () => api("DELETE", rolePath(r.key)), t("roles.removed", { role: r.key }));
}

/** createRole adds a role (POST): its key, its set and, where given, its name. */
export function createRole(body: { key: string; capabilities: string[]; name?: string }): Promise<boolean> {
  return run(body.key, () => api("POST", "api/v1/roles", body), t("roles.created", { role: body.key }));
}

/** setCapability switches a capability on or off for every role but the owner (PUT /api/v1/capabilities/{key}). */
export function setCapability(key: string, on: boolean): Promise<boolean> {
  return run(key, () => api("PUT", "api/v1/capabilities/" + encodeURIComponent(key), { disabled: !on }),
    t(on ? "roles.capEnabled" : "roles.capDisabled", { cap: capLabel(key) }));
}
