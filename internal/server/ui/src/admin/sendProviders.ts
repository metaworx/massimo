// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/admin/sendProviders.ts
 *
 * What System settings → Sign-in providers reads and sends, line for line
 * as providers.js did: every provider (GET /api/v1/providers), with the
 * capability auth; a provider added, changed or removed (POST, PUT and
 * DELETE …/providers/{key}), the enabled ones read again after each, for
 * the sign-in and the users' identities. A provider gone from a load that
 * went through forgets its form and its open panel; an answer after a
 * sign-out is dropped (shared/session.ts). The roles a provider may give the users
 * it creates hold no administrative capability.
 */

import type { Provider } from "../api";
import { actionRunner } from "../shared/action";
import { forgetForms } from "../shared/forms";
import { session, stillSession } from "../shared/session";
import { enabled, providerFormKey, providers, providerUI, reads, roles, said } from "./store";
import { can } from "../shared/me";

/** ADMIN_CAPS are the capabilities a role may not hold to be given to the users a provider creates (adminCapabilities in internal/server/oidc.go). */
const ADMIN_CAPS = ["guests", "users", "passwords", "roles", "roles.disable", "capabilities", "cubes", "cubes.reboot", "settings", "update", "restart",
  "reboot", "audit", "auth", "backup.server", "backup.retry"];

/**
 * autoRoleOptions are the roles a provider may give the users it
 * creates: those read that hold no administrative capability, else the
 * default ones that hold none; current is kept in the list.
 */
export function autoRoleOptions(current?: string): [string, string][] {
  let out: [string, string][] = (roles.list || [])
    .filter((r) => r.key !== "owner" && !(r.capabilities || []).some((c) => ADMIN_CAPS.includes(c)))
    .map((r) => [r.key, roleLabel(r.key, r.name)]);
  if (!out.length) out = ["guest", "viewer", "operator", "integration"].map((k) => [k, roleLabel(k)]);
  if (current && !out.some(([k]) => k === current)) out.push([current, roleLabel(current)]);
  return out;
}

/** loadProviders reads every provider, for a holder of auth; a provider the list no longer gives forgets its form. */
export async function loadProviders(): Promise<void> {
  if (!can("auth")) return;
  const n = session();
  const list = (await api<Provider[]>("GET", "api/v1/providers")) || [];
  if (!stillSession(n)) return; // signed out meanwhile: nothing of it is the next user's
  const now = new Set(list.map((p) => p.key));
  for (const p of providers.list) {
    if (now.has(p.key)) continue;
    forgetForms(providerFormKey(p.key));
    providerUI.forget(p.key); // added again under the same key, it does not open by itself
  }
  providers.list = list;
  providers.loaded = true;
}

/** The providers' runner: its message under the list, the providers and the enabled ones read again after a change. */
const runner = actionRunner({
  set: (res) => {
    said.providers = res;
  },
  reload: async () => {
    const n = session();
    await loadProviders();
    const list = await loadLoginProviders();
    if (!stillSession(n)) return;
    enabled.value = list;
    reads.value++; // a provider removed takes its identities: the users' open panels read theirs again
  },
  bound: true,
});

/** providersMsg shows text under the providers, ok green, else red; no text hides it. */
export const providersMsg = runner.say;

/** run sends a change of the provider name, says ok, or that it failed and why. */
function run(name: string, fn: () => Promise<unknown>, ok: string): Promise<boolean> {
  return runner.run(fn, ok, (error) => t("providers.failed", { name, error }));
}

/** providerPath is a provider's path in the API. */
function providerPath(key: string): string {
  return "api/v1/providers/" + encodeURIComponent(key);
}

/** saveProvider sends a provider's fields (PUT), the secret only where one is typed. */
export function saveProvider(p: Provider, body: Record<string, unknown>): Promise<boolean> {
  const name = (body.name as string) || p.key;
  return run(name, () => api("PUT", providerPath(p.key), body), t("providers.saved", { name }));
}

/** addProvider adds a provider (POST), its key with it; added, its panel is open. */
export function addProvider(body: Record<string, unknown> & { key: string }): Promise<boolean> {
  const name = (body.name as string) || body.key;
  return run(name, () => api("POST", "api/v1/providers", body), t("providers.added", { name })).then((ok) => {
    if (ok) providerUI.of(body.key).open = true;
    return ok;
  });
}

/** removeProvider removes a provider (DELETE). */
export function removeProvider(p: Provider): Promise<boolean> {
  const name = p.name || p.key;
  return run(p.key, () => api("DELETE", providerPath(p.key)), t("providers.removed", { name }));
}
