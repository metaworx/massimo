// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/profile/send.ts
 *
 * What My profile reads and sends, line for line as profile.js,
 * linked.js and login.js did: the signed-in user (GET …/users/{name})
 * with the enabled sign-in providers, the display name and the own
 * password (PUT …/users/{name}, the password registered by OPAQUE in the
 * browser after the current one is proved), a link of the own account
 * at a provider started, shown and stored or dropped (…/identities/link);
 * the sessions, keys and identities through shared/account.ts with this
 * card's runner. A change reads the users again where the signed-in user
 * may read them (System settings → Users), then My profile, and says how
 * it went under the card; an answer after a sign-out is dropped
 * (shared/session.ts).
 */

import type { PendingLink, User } from "../api";
import { type Run, userPath } from "../shared/account";
import { actionRunner } from "../shared/action";
import { canManageUsersOf, me, profileOf } from "../shared/me";
import { session, stillSession } from "../shared/session";
import { profile, said } from "./store";

/** My profile's runner: its message under the card; a change read again, System settings' users and My profile with its sessions, keys and identities. */
const runner = actionRunner({
  set: (res) => {
    said.profile = res;
  },
  reload: async () => {
    if (canManageUsersOf()) await window.MassimoParts.users.load(); // System settings → Users, another region
    await loadProfile();
  },
  bound: true,
});

/** profileMsg shows text under the card, ok green, else red; no text hides it. */
export const profileMsg = runner.say;

/** run sends a change of the user name, says ok, or that it failed and why: what this card and its sessions, keys and identities send. */
export const run: Run = (name, fn, ok) => runner.run(fn, ok, (error) => t("users.failed", { name, error }));

/** The own password's runner: its message under its form, and nothing read again. */
const passwordRunner = actionRunner({
  set: (res) => {
    said.password = res;
  },
  bound: true,
});

/** passwordMsg shows text under the password's form, ok green, else red. */
export const passwordMsg = passwordRunner.say;

/**
 * loadProfile reads the signed-in user for My profile, with the enabled
 * providers where none are known yet, and counts the read, at which its
 * sessions, keys and identities are read again; then a link come back
 * from a provider (oidcReturn, login.js) is shown to confirm, or why there
 * is none. A read that fails says so and leaves the card as it was;
 * nothing is read where no user name is left to read, as after a
 * sign-out.
 */
export async function loadProfile(): Promise<void> {
  if (!profileOf(me.peek())) return;
  const name = me.peek()?.user?.name || "";
  if (!name) return; // nobody in any more
  const n = session();
  let u: User;
  try {
    u = await api<User>("GET", userPath(name));
  } catch (e) {
    if (stillSession(n)) profileMsg(t("users.failed", { name, error: (e as Error).message }), false);
    return;
  }
  if (!stillSession(n)) return;
  if (!providersUI.login.length) await loadLoginProviders();
  if (!stillSession(n)) return;
  profile.providers = providersUI.login;
  profile.user = u;
  profile.reads++;
  if (oidcLinked) {
    // back from a link: shown to confirm, or why there is none
    const back = oidcLinked;
    oidcLinked = null;
    if (back.confirm) await readConfirm(name, back.confirm, n);
    else profileMsg(t("profile.linkError." + back.error), false);
  }
}

/** pendingPath is the path of a link come back, before it is stored. */
function pendingPath(name: string, code: string): string {
  return userPath(name, "/identities/link/pending/" + encodeURIComponent(code));
}

/** readConfirm reads a link come back (GET …/link/pending/{code}) and shows it; one gone or expired says so. */
async function readConfirm(name: string, code: string, n: number): Promise<void> {
  let p: PendingLink;
  try {
    p = await api<PendingLink>("GET", pendingPath(name, code));
  } catch (e) {
    if (stillSession(n)) profileMsg(t("profile.linkError.expired", { error: (e as Error).message }), false);
    return;
  }
  if (stillSession(n)) profile.confirm = { code, p };
}

/** saveName sends the own display name (PUT); taken, who is in is read again, since the 👤 menu names the user by it. */
export function saveName(name: string, display: string): Promise<boolean> {
  return run(name, () => api("PUT", userPath(name), { display_name: display }), t("users.saved", { name })).then((ok) => {
    if (ok) loadMe();
    return ok;
  });
}

/**
 * changePassword sets the own password (PUT): the current one proved
 * first, where the user has one, by a sign-in by OPAQUE that sends it
 * nowhere, and the new one registered by OPAQUE in the browser; neither
 * goes to the server. Taken, who is in is read again.
 */
export function changePassword(u: User, current: string, password: string): Promise<boolean> {
  const first = !u.has_password; // a user with keys only: no current one
  const send = async () => {
    const body: Record<string, unknown> = first ? {} : await currentPassword(u, current);
    Object.assign(body, await registerPassword(u.name, password));
    return api("PUT", userPath(u.name), body);
  };
  return passwordRunner.run(send, t(first ? "me.passwordSet" : "me.passwordChanged"), (error) => t("users.failed", { name: u.name, error })).then((ok) => {
    if (ok) loadMe();
    return ok;
  });
}

/** startLink starts the link of the own account at a provider: a sign-in there, which comes back to My profile (oidcReturn, login.js). */
export async function startLink(name: string, key: string): Promise<void> {
  try {
    const started = await api<{ url: string }>("POST", userPath(name, "/identities/link/" + encodeURIComponent(key)), { redirect: pageBase() });
    location.assign(started.url);
  } catch (e) {
    profileMsg(t("profile.linkFailed", { error: (e as Error).message }), false);
  }
}

/** confirmLink stores the link shown (POST); taken, it goes, and My profile is read again. */
export function confirmLink(name: string, c: { code: string; p: PendingLink }): Promise<boolean> {
  return run(name, () => api("POST", pendingPath(name, c.code)), t("profile.linkDone", { name: c.p.provider_name || "" })).then((ok) => {
    if (ok && profile.confirm === c) profile.confirm = null;
    return ok;
  });
}

/** cancelLink drops the link shown (DELETE): one gone already stored nothing either way. */
export async function cancelLink(name: string, c: { code: string; p: PendingLink }): Promise<void> {
  const n = session();
  try {
    await api("DELETE", pendingPath(name, c.code));
  } catch {
    // gone already: nothing was stored either way
  }
  if (!stillSession(n)) return;
  if (profile.confirm === c) profile.confirm = null;
  profileMsg(t("profile.linkCancelled"), true);
}
