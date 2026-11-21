// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/profile/index.tsx
 *
 * My profile as a part (AP UiParts, Block 13), drawn once into its
 * card's body, #profile-sec, as parts.js loads, and updated in place:
 * the user signed in is read each time the personal page is shown, as
 * profile.js read it, and after each change made here. No plain script
 * calls it: who is in comes through MassimoParts.me (shared/me.ts).
 */

import { me, profileOf } from "../shared/me";
import { mounter } from "../shared/region";
import { ProfileCard } from "./Profile";
import { loadProfile, profileMsg } from "./send";
import { forgetProfile } from "./store";
import { onView } from "../shared/pages";

const mount = mounter("profile-sec", () => <ProfileCard />);
mount();

/** signedOut forgets what My profile kept of the user who was signed in (MassimoParts.signedOut, index.ts). */
export const signedOut = forgetProfile;

// the personal page shown: the message line empty, the user read again with its sessions, keys and identities
onView("me", () => {
  profileMsg("");
  if (profileOf(me.peek())) loadProfile();
});
