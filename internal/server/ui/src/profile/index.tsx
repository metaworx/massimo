// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/profile/index.tsx
 *
 * My profile as a part (AP UiParts, Block 13), drawn once into its
 * card's body, #profile-sec, as parts.js loads, and updated in place:
 * the user signed in is read each time the personal page is shown, as
 * profile.js read it, and after each change made here. Who is in comes
 * through MassimoParts.me (shared/me.ts); the plain code calls
 * MassimoParts.profile.mark alone: pages.js readProfileHash, with the
 * API key the page's address names, #profile?key=<id>, which is marked
 * (store.ts markKey), from init on, parts.js loaded by then.
 */

import { mounter, register } from "../shared/region";
import { ProfileCard } from "./Profile";
import { loadProfile, profileMsg } from "./send";
import { forgetProfile, markKey, pageShown } from "./store";

const mount = mounter("profile-sec", () => <ProfileCard />);
mount();

/** The functions the plain code calls, MassimoParts.profile. */
const parts = register("profile", {
  /** mark sets the API key My profile marks, by its id, or none with null (pages.js readProfileHash, before the page is shown). */
  mark: markKey,
});

/** MassimoParts.profile, as index.ts declares it. */
export type ProfilePart = typeof parts;

/** signedOut forgets what My profile kept of the user who was signed in (MassimoParts.signedOut, index.ts). */
export const signedOut = forgetProfile;

// the personal page shown: the message line empty, the user read again with its sessions, keys and identities
onView("me", () => {
  profileMsg("");
  if (hasProfile()) loadProfile();
});
// a page shown once the keys were drawn with the mark, My profile again among them, marks none
document.addEventListener("viewshown", pageShown);
