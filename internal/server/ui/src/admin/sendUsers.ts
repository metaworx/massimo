// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/admin/sendUsers.ts
 *
 * What System settings → Users reads and sends, line for line as users.js
 * and providers.js did: the users (GET /api/v1/users) with the enabled
 * sign-in providers and, where they may be read, the roles; a user's
 * change and removal (PUT and DELETE …/users/{name}), a new user (POST)
 * and the password sign-in (the setting auth.password_login); a user's
 * sessions, keys and identities send theirs through this list's runner
 * (run, shared/account.ts). A change reads the users again and says how
 * it went under the list; a user gone from a load that went through
 * forgets its form and its open panel; an answer after a sign-out is
 * dropped (shared/session.ts). What the signed-in user may do with a user is the
 * server's word per user (may), else the capabilities'.
 */

import type { Role, User } from "../api";
import { type Run, userPath } from "../shared/account";
import { actionRunner } from "../shared/action";
import { forgetForms } from "../shared/forms";
import { session, stillSession } from "../shared/session";
import { enabled, load, passwordLogin, reads, roles, said, userFormKey, userUI, users } from "./store";

/** usersAdmin says what the signed-in user may do with users: everything, guests only, or nothing but its own user. */
export function usersAdmin(): "all" | "guests" | "self" {
  return hasCap("users") ? "all" : hasCap("guests") ? "guests" : "self";
}

/** mayManage says whether a user is the signed-in user's to change beyond its own name and password: the server's may, else the capabilities. */
export function mayManage(u: User): boolean {
  if (u.may) return u.may.manage;
  const a = usersAdmin();
  return a === "all" || (a === "guests" && u.role === "guest");
}

/** mayPassword says whether a user's password is the signed-in user's to set. */
export function mayPassword(u: User): boolean {
  return u.may ? u.may.password : mayManage(u) || isSelf(u.name);
}

/**
 * roleOptions are the roles a user may be given, for its choice: with
 * users every role (those read, else the default keys), with guests only
 * guest, owner only for an owner (the server's rank rule); current is
 * kept in the list.
 */
export function roleOptions(current?: string): [string, string][] {
  const read = roles.list;
  let keys: string[];
  if (usersAdmin() === "guests") keys = ["guest"];
  else if (read) keys = read.map((r) => r.key);
  else keys = [...DEFAULT_ROLES];
  if (state.me?.role?.key !== "owner") keys = keys.filter((k) => k !== "owner");
  if (current && !keys.includes(current)) keys.push(current);
  return keys.map((k) => [k, roleLabel(k, (read || []).find((x) => x.key === k)?.name)]);
}

/** loadRolesForUsers reads the roles once for the role choice, where they may be read; none where they may not. */
async function loadRolesForUsers(n: number): Promise<void> {
  if (!hasCap("roles") || roles.list) return;
  let list: Role[] | null;
  try {
    list = (await api<{ roles?: Role[] }>("GET", "api/v1/roles"))?.roles || null;
  } catch {
    list = null;
  }
  if (stillSession(n)) roles.list = list;
}

/**
 * loadUsers reads the users, the enabled providers for their identities
 * and the roles for the role choice; a user the list no longer gives
 * forgets its form (removed, here or by another client). A load that
 * fails throws before it and leaves the list as it was.
 */
export async function loadUsers(): Promise<void> {
  if (!canManageUsers()) return;
  const n = session();
  const list = (await api<User[]>("GET", "api/v1/users")) || [];
  if (!stillSession(n)) return; // signed out meanwhile: nothing of it is the next user's
  const now = new Set(list.map((u) => u.name));
  for (const u of users.list) {
    if (now.has(u.name)) continue;
    forgetForms(userFormKey(u.name));
    userUI.forget(u.name); // made again under the same name, it does not open by itself
  }
  users.list = list;
  users.loaded = true;
  const providers = await loadLoginProviders();
  if (!stillSession(n)) return;
  enabled.value = providers;
  await loadRolesForUsers(n);
  load.bump();
}

/** The users' runner: its message under the list; a change read again, and the open panels' sessions, keys and identities with it. */
const runner = actionRunner({
  set: (res) => {
    said.users = res;
  },
  reload: async () => {
    await loadUsers();
    reads.value++;
  },
  bound: true,
});

/** usersMsg shows text under the list, ok green, else red; no text hides it. */
export const usersMsg = runner.say;

/** run sends a change of the user name, says ok, or that it failed and why; a user's sessions, keys and identities send theirs through it (shared/account.ts). */
export const run: Run = (name, fn, ok) => runner.run(fn, ok, (error) => t("users.failed", { name, error }));

/**
 * saveUser sends a user's change (PUT): body the fields that differ; a
 * new password registered by OPAQUE in the browser first, one's own after
 * proving the current one, neither sent to the server.
 */
export function saveUser(u: User, body: Record<string, unknown>, password: string, current: string | null): Promise<boolean> {
  const send = async () => {
    if (password) {
      if (current !== null) Object.assign(body, await currentPassword(u, current));
      Object.assign(body, await registerPassword(u.name, password));
    }
    return api("PUT", userPath(u.name), body);
  };
  return run(u.name, send, t("users.saved", { name: u.name })).then((ok) => {
    if (ok && password && isSelf(u.name)) loadMe();
    return ok;
  });
}

/** removeUser deletes a user (DELETE). */
export function removeUser(u: User): Promise<boolean> {
  return run(u.name, () => api("DELETE", userPath(u.name)), t("users.removed", { name: u.name }));
}

/**
 * createUser adds a user (POST), its password registered in the browser
 * first; a refusal of its name is worded as the page's check words it
 * and handed to said, the line under the name.
 */
export function createUser(body: Record<string, unknown> & { name: string }, password: string, said: (text: string) => void): Promise<boolean> {
  const create = async () => {
    if (password) Object.assign(body, await registerPassword(body.name, password));
    try {
      await api("POST", "api/v1/users", body);
    } catch (e) {
      const worded = wordNameError(e as Error, body.name);
      if (worded) {
        said(worded);
        (e as Error).message = worded;
      }
      throw e;
    }
  };
  return run(body.name, create, t("users.created", { name: body.name }));
}

/** loadPasswordLogin reads whether the server takes the password sign-in, for a holder of settings, who alone sees the switch. */
export async function loadPasswordLogin(): Promise<void> {
  if (!hasCap("settings")) return;
  const n = session();
  try {
    const all = (await api<Record<string, { value?: string }>>("GET", "api/v1/settings")) || {};
    if (!stillSession(n)) return;
    const v = all["auth.password_login"];
    passwordLogin.value = !v || v.value !== "false";
  } catch {
    // the switch stays as it was
  }
}

/** setPasswordLogin turns the password sign-in on or off, saved at once; refused, the switch stays as stored. */
export async function setPasswordLogin(on: boolean): Promise<void> {
  const n = session();
  try {
    await api("PUT", "api/v1/settings", { "auth.password_login": on });
    if (!stillSession(n)) return;
    passwordLogin.value = on;
    usersMsg(t(on ? "users.passwordLoginOn" : "users.passwordLoginOff"), true);
  } catch (e) {
    if (stillSession(n)) usersMsg(t("users.failed", { name: "", error: (e as Error).message }), false);
  }
}
