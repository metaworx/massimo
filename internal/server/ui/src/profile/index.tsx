// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/profile/index.tsx
 *
 * My profile as a part (AP UiParts, Block 13), drawn once into its
 * card's body, #profile-sec, as parts.js loads, and updated in place:
 * the user signed in is read each time the personal page is shown, as
 * profile.js read it, and after each change made here. Who is in is
 * the signal me of shared/me.ts, state.me underneath. The card's address,
 * #profile?key=<id>, may name an API key, which is marked (store.ts
 * markKey): the pages' store hands this part what follows the "?" as it
 * reads the address, before the page is shown (shared/pages.ts
 * readCardAddress). In the app, the app's status is read again as the
 * page is shown, for its key of this device (shared/app.ts), and a key
 * marked that the last sign-in revoked is marked no more, a toast saying
 * so (AP MobileApp, D67-A). No plain script calls it.
 */

import { readApp, signInRevoked } from "../shared/app";
import { me, profileOf } from "../shared/me";
import { mounter } from "../shared/region";
import { ProfileCard } from "./Profile";
import { loadProfile, profileMsg } from "./send";
import { forgetProfile, markKey, pageShown, profile } from "./store";
import { onView, readCardAddress } from "../shared/pages";

const mount = mounter("profile-sec", () => <ProfileCard />);
mount();

// the key the card's address names, #profile?key=<id>, a whole number of at
// most 15 digits, marked; another address, or an id that is none, marks none
readCardAddress("profile", (q) => {
  const id = q?.get("key") || "";
  markKey(/^[1-9][0-9]{0,14}$/.test(id) ? Number(id) : null);
});

/** signedOut forgets what My profile kept of the user who was signed in (MassimoParts.signedOut, index.ts). */
export const signedOut = forgetProfile;

// the personal page shown: the message line empty, the user read again with its sessions, keys and identities, and in
// the app its status; a key marked that the last sign-in revoked marked no more, the toast naming it
onView("me", () => {
  profileMsg("");
  if (appLogin()) readApp();
  if (profileOf(me.peek())) loadProfile();
  const marked = profile.marked;
  if (!marked) return;
  signInRevoked().then((ids) => {
    if (profile.marked !== marked || !ids.includes(marked)) return;
    markKey(null);
    showToast(t("profile.keyRevokedAtSignIn", { id: marked }));
  });
});
// a page shown once the keys were drawn with the mark, My profile again among them, marks none
document.addEventListener("viewshown", pageShown);
