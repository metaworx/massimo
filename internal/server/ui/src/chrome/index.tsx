// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/chrome/index.tsx
 *
 * The page's chrome (AP UiParts, Block 14), a region of its own, drawn
 * once into its places as parts.js loads: the header's page buttons with
 * ☰ (Menu.tsx), into the header's nav; the section cards' badges and
 * pills, their folds wired (Cards.tsx, shared/card.ts); the header kept
 * at the top and compacted as the page scrolls, and back to top
 * (scroll.ts, ToTop.tsx). The pages it shows are the store of
 * shared/pages.ts, which it first hands the page's pages (pages.ts). The log tells its count and tone through
 * MassimoParts.chrome.log (log.js renderLogSign, plain until Block 11),
 * header.js the server's version. No region may import another: it knows
 * the forms through shared/forms.ts alone.
 */

import { render } from "preact";
import { type BadgeSpec, cardBadges, showCard, wireCards } from "../shared/card";
import { definePages } from "../shared/pages";
import { register } from "../shared/region";
import { drawCards } from "./Cards";
import { Nav } from "./Menu";
import { CARD_ADDRESSES, PAGE_VIEWS } from "./pages";
import { stickyHeader } from "./scroll";
import { logSign, version } from "./store";
import { ToTop } from "./ToTop";

// the pages handed to their store first; drawn once into the places the
// page has for them; the cards' folds wired; the header kept at the top
definePages(PAGE_VIEWS, CARD_ADDRESSES);
const nav = document.getElementById("nav");
if (nav) render(<Nav />, nav);
wireCards();
drawCards();
stickyHeader();
const toTopSlot = document.getElementById("to-top-slot");
if (toTopSlot) render(<ToTop />, toTopSlot);

/** The functions the plain code calls, MassimoParts.chrome. */
const chrome = register("chrome", {
  /** log takes the log's unseen problems, their count and the worst one's tone, for ☰'s mark and name and its Log entry (log.js renderLogSign). */
  log(count: number, tone: "" | "warn" | "bad"): void {
    const was = logSign.peek();
    if (was.count !== count || was.tone !== tone) logSign.value = { count, tone };
  },
  /** version takes the server's whole version, ☰'s first row (header.js checkVersion). */
  version(v: string): void {
    version.value = v;
  },
  /** badges shows a card's badges in its head, by the card's id (cubes.js renderCubesBadges; a part calls cardBadges of shared/card.ts). */
  badges(id: string, list: BadgeSpec[]): void {
    cardBadges(id, list);
  },
  /** showCard brings the card of the id into view, opened, the focus on its heading (components.js showCard). */
  showCard(id: string, instant?: boolean): void {
    showCard(id, instant);
  },
});

/** MassimoParts.chrome, as index.ts declares it. */
export type ChromePart = typeof chrome;
