// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/shared/account.ts
 *
 * What a user's sessions, API keys and identities at the sign-in
 * providers read and send, line for line as users.js, profile.js and
 * providers.js did (…/users/{name}/tokens and …/identities), for the two
 * places that draw them: a user's panel in System settings → Users and
 * My profile. A change runs through the caller's runner (Run), which
 * reloads what it shows and says how it went in its own message line; a
 * read is the caller's to drop where its answer comes too late.
 */

import type { Identity, Token, TokenHeirs } from "../api";
import type { OkText } from "./action";

/**
 * Run sends fn's request for the user of the login name name through the
 * caller's runner: it reloads what the caller shows, says ok or that it
 * failed and why, and resolves to whether the request was taken; ok may
 * be made once the request is done (OkText).
 */
export type Run = (name: string, fn: () => Promise<unknown>, ok: OkText) => Promise<boolean>;

/** userPath is a user's path in the API, rest after it. */
export function userPath(name: string, rest = ""): string {
  return "api/v1/users/" + encodeURIComponent(name) + rest;
}

/** loadTokens reads a user's sessions and keys. */
export async function loadTokens(name: string): Promise<Token[]> {
  return (await api<Token[]>("GET", userPath(name, "/tokens"))) || [];
}

/**
 * revokeToken revokes one of a user's sessions or keys (DELETE), a key's
 * heirs with it, the keys it made and theirs, which its message names
 * by id (AP KeyLife, D5-A).
 */
export function revokeToken(run: Run, name: string, tk: Token): Promise<boolean> {
  let heirs: TokenHeirs = [];
  return run(name, async () => {
    const res = await api<{ heirs?: TokenHeirs }>("DELETE", userPath(name, "/tokens/" + tk.id));
    heirs = (res && res.heirs) || [];
  }, () => (heirs.length ? t("users.revokedWithHeirs", { id: tk.id, ids: heirs.join(", ") }) : t("users.revoked", { id: tk.id })));
}

/**
 * revokeOwn revokes the token the page is signed in with, as Sign out
 * ends it: a session by Sign out itself (logout, login.js); a key is
 * revoked first, then the page forgets it as Sign out does, and a
 * refusal leaves it signed in, saying nothing, as users.js did.
 */
export function revokeOwn(name: string, tk: Token): Promise<unknown> {
  if (tk.kind === "session") return logout();
  return api("DELETE", userPath(name, "/tokens/" + tk.id)).then(logout, () => {});
}

/**
 * createKey makes an API key for a user (POST): the key, which the server
 * shows once, goes to shown alone, before the caller reads again; it is
 * kept nowhere else.
 */
export function createKey(run: Run, name: string, label: string, shown: (key: string) => void): Promise<boolean> {
  return run(name, async () => {
    const res = await api<{ token: string }>("POST", userPath(name, "/tokens"), { label });
    shown(res.token);
  }, t("users.keyCreated", { label }));
}

/** loadIdentities reads a user's identities at the sign-in providers. */
export async function loadIdentities(name: string): Promise<Identity[]> {
  return (await api<Identity[]>("GET", userPath(name, "/identities"))) || [];
}

/** addIdentity assigns an identity at a provider to a user, by an e-mail or by the user id there (POST). */
export function addIdentity(run: Run, name: string, provider: string, bySubject: boolean, value: string): Promise<boolean> {
  const body = bySubject ? { provider, subject: value } : { provider, email: value };
  return run(name, () => api("POST", userPath(name, "/identities"), body),
    t("users.identityAdded", { email: bySubject ? t("users.identityUserID", { id: value }) : value }));
}

/** changeIdentity changes an identity's e-mail or user id while no sign-in used it (PUT). */
export function changeIdentity(run: Run, name: string, i: Identity, bySubject: boolean, value: string): Promise<boolean> {
  return run(name, () => api("PUT", userPath(name, "/identities/" + i.id), bySubject ? { subject: value } : { email: value }), t("users.identityChanged"));
}

/** removeIdentity removes one of a user's identities (DELETE). */
export function removeIdentity(run: Run, name: string, i: Identity): Promise<boolean> {
  return run(name, () => api("DELETE", userPath(name, "/identities/" + i.id)), t("users.identityRemoved"));
}
