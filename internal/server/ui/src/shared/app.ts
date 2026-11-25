// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/shared/app.ts
 *
 * The app's bridge as the parts reach it (appConnect, appServerSettings,
 * appAppSettings and appLogin of login.js): back to the app's servers,
 * from ☰'s Select server, the sign-in card's ☰ and the Cancels of its
 * forms and of the first owner's form, and to this server's settings and
 * the app's on its own page, from ☰'s two entries after Select server;
 * the app's status of the login by fingerprint as a signal, read
 * again each time it is asked (readApp: each opening of the sign-in, a
 * fingerprint refused, the first owner's form shown, My profile shown),
 * which the sign-in card and My profile read, its key of this device
 * among them (AP MobileApp, D65-A), and the server it opened the page
 * for (serverOf), which the sign-in card names and the rule of a new
 * password takes a name of; and what the app's setToken of the
 * last sign-in answered, the keys and sessions left it revoked (D67-A).
 * Nothing in a browser: the status is null there, and no sign-in revoked
 * anything.
 */

import { signal } from "@preact/signals";

/** toServers goes back to the app's servers, where the page is in the app. */
export function toServers(): void {
  const go = appConnect();
  if (go) go().catch(() => {});
}

/** toServerSettings shows this server's settings on the app's own page, where the app has the call (contract version 5; AP MobileApp, D73-A). */
export function toServerSettings(): void {
  const go = appServerSettings();
  if (go) go().catch(() => {});
}

/** toAppSettings shows the app's settings on its own page, where the app has the call (contract version 5). */
export function toAppSettings(): void {
  const go = appAppSettings();
  if (go) go().catch(() => {});
}

/** The app's last status of the login by fingerprint, null in a browser or where it could not be read. */
export const app = signal<AppLoginStatus | null>(null);

/** readApp asks the app for its status (login.status) and keeps the answer; null in a browser and where the app refused. */
export async function readApp(): Promise<void> {
  const login = appLogin();
  let st: AppLoginStatus | null = null;
  if (login) {
    try {
      st = await login.status();
    } catch (e) {
      st = null;
    }
  }
  app.value = st;
}

/** The server the app opened the page for, as the sign-in card names it (signin/Server.tsx) and the rule of a new password takes its name (shared/password.tsx serverNames): its name, "" where it is the address; the address; its instance id and pinned fingerprint, "" for none. */
export interface CardServer {
  name: string;
  address: string;
  node: string;
  fp: string;
}

/**
 * serverOf is the server the app opened the page for, from its status
 * (login.status's server, contract version 5; AP MobileApp, D62-A): the
 * address it was opened at in full, https://host:port with its path but
 * the last slash, the name left out where it is that address or its host
 * and port, as a server added by its address is named (connect.js
 * saveNew); null in a browser, from an app before version 5, and for an
 * address that is not one.
 */
export function serverOf(st: AppLoginStatus | null): CardServer | null {
  const s = st?.server;
  if (!s || typeof s.url !== "string") return null;
  let u: URL;
  try {
    u = new URL(s.url);
  } catch {
    return null;
  }
  if (u.protocol !== "https:" && u.protocol !== "http:") return null;
  const address = u.origin + u.pathname.replace(/\/$/, "");
  const text = (v: unknown) => (typeof v === "string" ? v.trim() : "");
  const name = text(s.name);
  return { name: name === address || name === u.host ? "" : name, address, node: text(s.node), fp: text(s.fp) };
}

/** appKeyId is the id of the key the phone keeps for this server (login.status's key_id, contract version 5), 0 for none, one not known, or in a browser. */
export function appKeyId(st: AppLoginStatus | null): number {
  const id = st?.key_id;
  return typeof id === "number" && Number.isInteger(id) && id > 0 ? id : 0;
}

/**
 * signInRevoked is what the app's setToken of the last sign-in answered
 * (login.js tokenAnswer): the ids of the keys and sessions left there
 * that it revoked before it answered; none in a browser, from an app
 * before contract version 5, or where the bridge failed. My profile's
 * list of keys waits for it, so that a key the sign-in revoked is not
 * listed (D67-A).
 */
export async function signInRevoked(): Promise<number[]> {
  const a = (await tokenAnswer()) as { revoked?: unknown } | null;
  const ids = a && Array.isArray(a.revoked) ? a.revoked : [];
  return ids.filter((x): x is number => typeof x === "number" && Number.isInteger(x) && x > 0);
}
