// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/signin/app.ts
 *
 * The sign-in's part of the app (docs/app.md, "The login by
 * fingerprint"): the app's status of the login by fingerprint as a
 * signal, read again each time it is asked (readApp: the first owner's
 * form shown), and what it offers, line for line as login.js
 * renderSetupKeep read it. Nothing of it in a browser: the status is
 * null there.
 */

import { signal } from "@preact/signals";

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
