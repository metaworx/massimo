// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/chrome/pages.ts
 *
 * The pages of the main area as the page's skeleton has them (AP UiParts,
 * Block 14; VIEW_SECTION, VIEW_HASH, CARD_HASH and MENU_VIEWS of pages.js
 * before), handed to the pages' store (shared/pages.ts definePages) as the
 * chrome starts: each page's view, its address, the elements it shows, and
 * who may open it (System settings with one of its capabilities,
 * Maintenance with one of those or update, restart or reboot, Software
 * update being for all of them, the Cube's settings with cubes or with
 * pair, its pairing alone, System info with read); the personal page's
 * cards with addresses of their own (those of the pages they were), My
 * profile for a user alone (profileOf of shared/me.ts); and the pages of
 * ☰ in its order, each with its entry's id, its icon, the key of its
 * name, the class that hides it from a role that may not open it
 * (login.js applyCaps, style.css), and the beginnings of the keys of the
 * settings forms its page holds, whose unsaved state its entry shows
 * (shared/forms.ts).
 */

import { can, me, profileOf, settingsPageOf, systemOf } from "../shared/me";
import type { CardAddress, PageView } from "../shared/pages";

/** The pages, Rooms first: what each shows, hidden while another is. */
export const PAGE_VIEWS: PageView[] = [
  { view: "tiles", hash: "", shows: ["home-notice", "house", "rooms", "unlisted"], hide: "view-hidden" },
  { view: "log", hash: "#log", shows: ["logview"] },
  { view: "me", hash: "#me", shows: ["me"] },
  { view: "system", hash: "#system", shows: ["system"], may: () => settingsPageOf() },
  { view: "maint", hash: "#maintenance", shows: ["maint-page"], may: () => systemOf() },
  { view: "info", hash: "#info", shows: ["info-page"], may: () => can("read") },
  { view: "cube", hash: "#cube", shows: ["cube-page"], may: () => can("cubes") || can("pair") },
];

/** The personal page's cards with addresses of their own. */
export const CARD_ADDRESSES: CardAddress[] = [
  { hash: "#profile", card: "profile", view: "me", may: () => profileOf(me.peek()) },
  { hash: "#prefs", card: "prefs", view: "me" },
  { hash: "#notify", card: "notify", view: "me" },
];

/** A page of ☰. */
interface MenuPage {
  view: string;
  id: string;
  icon: string;
  key: string;
  needs: string;
  forms: string[];
}

/** The pages of ☰, in its order: the log, the settings pages, Personal settings. */
export const MENU_PAGES: MenuPage[] = [
  { view: "log", id: "menu-log", icon: "icon-log", key: "nav.logTitle", needs: "", forms: [] },
  { view: "cube", id: "menu-cube", icon: "icon-cube3d", key: "nav.cubeTitle", needs: "needs-cubes-or-pair", forms: ["cube"] },
  { view: "system", id: "menu-system", icon: "icon-cog", key: "nav.settingsTitle", needs: "needs-system", forms: ["users", "roles", "providers", "integrations"] },
  { view: "maint", id: "menu-maint", icon: "icon-tool", key: "nav.maintTitle", needs: "needs-maint", forms: [] },
  { view: "info", id: "menu-info", icon: "icon-info", key: "nav.infoTitle", needs: "needs-read", forms: [] },
  { view: "me", id: "menu-me", icon: "icon-user", key: "nav.userTitle", needs: "", forms: ["profile", "scheme"] },
];

/** The settings pages, in ☰'s order: the four in the middle, which each settings page's row lists. */
export const SETTINGS_PAGES: MenuPage[] = MENU_PAGES.slice(1, 5);
