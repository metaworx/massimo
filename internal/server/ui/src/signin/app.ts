// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/signin/app.ts
 *
 * The sign-in's part of the app (docs/app.md, "The login by
 * fingerprint"): the app's status of the login by fingerprint as a
 * signal, read again each time it is asked (readApp: each opening of the
 * sign-in, a fingerprint refused, the first owner's form shown), what it
 * offers, line for line as login.js renderAppLogin and renderSetupKeep
 * read it, and the keeping of the login after a password's sign-in
 * (keepLogin). Nothing of it in a browser: the status is
 * null there.
 */

import { signal } from "@preact/signals";
import { askKeep, keepEnded } from "./store";

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
