// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/shared/app.ts
 *
 * The app's bridge as the parts reach it (appConnect and appLogin of
 * login.js): back to the app's servers, from ☰'s Select server, the
 * sign-in card's ☰ and the Cancels of its forms and of the first owner's
 * form; nothing in a browser.
 */

/** toServers goes back to the app's servers, where the page is in the app. */
export function toServers(): void {
  const go = appConnect();
  if (go) go().catch(() => {});
}
