// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/shared/card.ts
 *
 * A page's section cards (AP UiParts, Block 14): what a card's head shows
 * at a glance, and its fold. A card is the page's skeleton, a
 * section.section-card with an id: its head an h3 holding a
 * button.section-fold (aria-controls its body) with the title, then a
 * span.section-badges, its pill and its help; its body the element the
 * button controls. Its badges are a signal per card's id, which a region
 * sets from its drawing's effect (cardBadges) and the page's chrome draws
 * into the head (chrome/Cards.tsx), also while it is folded. Its fold is
 * kept in this browser (massimo.fold.<card id>) and set on the skeleton
 * at once, so that a card shown is open before it is scrolled to. The
 * caller names the card by its id; this file knows no region's.
 */

import { type Signal, signal } from "@preact/signals";

/** A badge of a card's head: its text and kind (ok, warn, bad, mode), as the shared Badge draws it. */
export interface BadgeSpec {
  text: string;
  kind: string;
}

/** The badges of each card, by its id. */
const badges = new Map<string, Signal<BadgeSpec[]>>();

/** badgesOf is the signal of a card's badges, made on first use: the chrome reads it, cardBadges writes it. */
export function badgesOf(id: string): Signal<BadgeSpec[]> {
  let s = badges.get(id);
  if (!s) {
    s = signal<BadgeSpec[]>([]);
    badges.set(id, s);
  }
  return s;
}

/** cardBadges shows what a card holds in its head, also while it is folded: the badges given in place of the ones it had, [] for none. */
export function cardBadges(id: string, list: BadgeSpec[]): void {
  const s = badgesOf(id);
  const was = s.peek();
  if (was.length === list.length && was.every((b, i) => b.text === list[i].text && b.kind === list[i].kind)) return;
  s.value = list;
}

/** foldKey is where a card's fold is kept in this browser. */
function foldKey(card: HTMLElement): string {
  return "massimo.fold." + card.id;
}

/** foldCard folds a card or opens it: the body hidden, the head with its badges still shown. */
function foldCard(card: HTMLElement, folded: boolean): void {
  const btn = card.querySelector<HTMLElement>(".section-fold");
  if (!btn) return;
  btn.setAttribute("aria-expanded", folded ? "false" : "true");
  document.getElementById(btn.getAttribute("aria-controls") || "")?.classList.toggle("hidden", folded);
  card.classList.toggle("folded", folded);
}

/**
 * wireCards makes the cards of the page fold from the button in their
 * heading, each card's fold kept in this browser and taken up again as
 * the page loads. Cards wired once are left alone.
 */
export function wireCards(): void {
  for (const card of document.querySelectorAll<HTMLElement>(".section-card")) {
    const btn = card.querySelector<HTMLElement>(".section-fold");
    if (!btn || btn.dataset.wired) continue;
    btn.dataset.wired = "1";
    foldCard(card, readStored(foldKey(card)) === "1");
    btn.addEventListener("click", () => {
      const folded = btn.getAttribute("aria-expanded") === "true";
      foldCard(card, folded);
      store(foldKey(card), folded ? "1" : null);
    });
  }
}

/**
 * showCard brings the card of the id into view, opened, with the focus on
 * its heading's button (a page-section's on its summary, where it has
 * one). On a page just opened (instant) the cards above it still grow as
 * their data comes, and a smooth scroll keeps the target it started with
 * and stops short: the card is brought there at once, and again while the
 * cards around it change size for two seconds, until the user scrolls.
 */
export function showCard(id: string, instant?: boolean): void {
  const card = document.getElementById(id);
  if (!card) return;
  if (card.classList.contains("folded")) {
    foldCard(card, false);
    store(foldKey(card), null);
  }
  const smooth = !instant && !matchMedia("(prefers-reduced-motion: reduce)").matches;
  card.scrollIntoView({ behavior: smooth ? "smooth" : "auto", block: "start" });
  const head = card.querySelector<HTMLElement>(".section-fold") || card.querySelector<HTMLElement>("summary");
  if (head) head.focus({ preventScroll: true });
  if (instant) keepCardInView(card, 2000);
}

/** keepCardInView brings a card to the top again whenever its page's cards change size, for ms or until the user scrolls by hand. */
function keepCardInView(card: HTMLElement, ms: number): void {
  const by = ["wheel", "touchstart", "keydown", "pointerdown"];
  const ro = new ResizeObserver(() => card.scrollIntoView({ block: "start" }));
  const stop = () => {
    ro.disconnect();
    for (const ev of by) removeEventListener(ev, stop);
  };
  if (card.parentElement) ro.observe(card.parentElement);
  for (const ev of by) addEventListener(ev, stop, { passive: true });
  setTimeout(stop, ms);
}
