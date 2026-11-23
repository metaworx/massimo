// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/signin/app.ts
 *
 * The sign-in's part of the app (docs/app.md, "The login by
 * fingerprint"): what the app's status of the login by fingerprint
 * offers (shared/app.ts, which My profile reads too), line for line as
 * login.js renderAppLogin and renderSetupKeep read it, the server the
 * page was opened for (serverOf, contract version 5), and the keeping of
 * the login after a password's sign-in (keepLogin). Nothing of it in a
 * browser: the status is null there.
 */

import { askKeep, keepEnded } from "./store";

/** offer says whether the switch that keeps the login by fingerprint is offered: the phone can, the address takes a key and none is kept. */
export function offer(st: AppLoginStatus | null): boolean {
  return !!(st && st.can && !st.plain && !st.user);
}

/** kept says whether the app keeps a login by fingerprint the address takes: the fingerprint first among the ways, named with its user. */
export function kept(st: AppLoginStatus | null): boolean {
  return !!(st && st.user && !st.plain);
}

/** plainHere says whether the phone could keep a login but the address takes no key (plain HTTP outside the home network): the card's notice. */
export function plainHere(st: AppLoginStatus | null): boolean {
  return !!(st && st.can && st.plain);
}

/** The server the card names in the app (Server.tsx): its name, "" where it is the address; the address; its instance id and pinned fingerprint, "" for none. */
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

/**
 * keepLogin keeps the login by fingerprint after a sign-in with the
 * switch on (login.keep, which asks for the fingerprint, the sign-in's
 * button busy meanwhile); where it is not kept the card says so and
 * offers Try again and Sign in without it, and goes on once either is
 * chosen (login.js keepLogin before). It reports whether the sign-in goes
 * on: false where the choice was dropped, the card closed or opened anew.
 */
export async function keepLogin(login: { keep(): Promise<boolean> }): Promise<boolean> {
  for (;;) {
    let said: string;
    try {
      if (await login.keep()) break;
      said = t("login.keepFailed");
    } catch (e) {
      said = t("login.keepError", { error: (e as Error).message });
    }
    const again = await askKeep(said);
    if (again === null) return false;
    if (!again) break;
  }
  keepEnded();
  return true;
}
