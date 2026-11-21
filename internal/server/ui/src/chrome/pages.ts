// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/chrome/pages.ts
 *
 * The pages of the main area as the page's skeleton has them (AP UiParts,
 * Block 14; VIEW_SECTION, VIEW_HASH and CARD_HASH of pages.js before),
 * handed to the pages' store (shared/pages.ts definePages) as the chrome
 * starts: each page's view, its address, the elements it shows, and who
 * may open it (System settings with one of its capabilities, Maintenance
 * with one of those or update, restart or reboot, Software update being
 * for all of them, the Cube's settings with cubes or with pair, its
 * pairing alone, System info with read); and the personal page's cards
 * with addresses of their own (those of the pages they were), My profile
 * for a user alone (profileOf of shared/me.ts).
 */

import { me, profileOf } from "../shared/me";
import type { CardAddress, PageView } from "../shared/pages";

/** The pages, Rooms first: what each shows, hidden while another is. */
export const PAGE_VIEWS: PageView[] = [
  { view: "tiles", hash: "", shows: ["home-notice", "house", "rooms", "unlisted"], hide: "view-hidden" },
  { view: "log", hash: "#log", shows: ["logview"] },
  { view: "me", hash: "#me", shows: ["me"] },
  { view: "system", hash: "#system", shows: ["system"], may: () => hasSettingsPage() },
  { view: "maint", hash: "#maintenance", shows: ["maint-page"], may: () => hasSystem() },
  { view: "info", hash: "#info", shows: ["info-page"], may: () => hasCap("read") },
  { view: "cube", hash: "#cube", shows: ["cube-page"], may: () => hasCap("cubes") || hasCap("pair") },
];

/** The personal page's cards with addresses of their own. */
export const CARD_ADDRESSES: CardAddress[] = [
  { hash: "#profile", card: "profile", view: "me", may: () => profileOf(me.peek()) },
  { hash: "#prefs", card: "prefs", view: "me" },
  { hash: "#notify", card: "notify", view: "me" },
];
