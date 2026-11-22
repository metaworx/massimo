// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/shared/pages.ts
 *
 * The pages of the main area (AP UiParts, Block 14; pages.js before):
 * Rooms ("tiles"), the header's one button, and the pages of ☰: the log
 * ("log"), Cube settings ("cube"), System settings ("system"),
 * Maintenance ("maint"), System info ("info") and Personal settings
 * ("me", My profile, My settings and Notifications as its cards). Which
 * one is shown is a signal (page): a drawing that reads page.value
 * follows it; isShown asks at that moment and follows nothing, for a
 * handler, a reader or a timer, so that no request is sent again at a
 * change of page. The page shown is in the address, Rooms without one,
 * the personal page at a card by the card's own (#profile, #prefs,
 * #notify, the addresses of the pages they were), and a page with an
 * address of its own (the log's choices, ui/src/log/) through what it
 * hands over (ownAddress), a card's with what its address holds after
 * the "?" through what its region hands over (readCardAddress, My
 * profile's key to mark), so that a reload or a link opens it again as
 * chosen and the browser's back and forward move between the pages. Each
 * page comes back at the scroll it was left at. The pages are the page's
 * skeleton, shown and hidden here by the ids the page's chrome hands over
 * as it starts (definePages, chrome/pages.ts), with each page's address,
 * who may open it and the cards that have addresses of their own; this
 * file draws nothing and names no element of the page.
 */

import { signal } from "@preact/signals";
import { showCard } from "./card";
import { me } from "./me";

/** The page shown: page.value in a drawing follows it, page.peek() and isShown ask once. */
export const page = signal("tiles");

/**
 * A page: its view, its address ("" for none), the elements of the
 * skeleton it shows, hidden by the class hide ("hidden" by default) while
 * another page is shown, and who may open it (anyone where may is not
 * given).
 */
export interface PageView {
  view: string;
  hash: string;
  shows: string[];
  hide?: string;
  may?: () => boolean;
}

/** A card with an address of its own: its address, the card's id, its page's view, and whether it may be shown (anyone where may is not given). */
export interface CardAddress {
  hash: string;
  card: string;
  view: string;
  may?: () => boolean;
}

/** The pages, by their views, and the cards with addresses, as the chrome hands them over. */
const views: Record<string, PageView> = {};
const cards: CardAddress[] = [];

/** definePages takes the pages and the cards with addresses of their own (the page's chrome, as parts.js loads). */
export function definePages(pages: PageView[], addressed: CardAddress[]): void {
  for (const p of pages) views[p.view] = p;
  cards.push(...addressed);
}

/** cardOf is the card an address names, the part before its "?". */
function cardOf(hash: string): CardAddress | undefined {
  const p = hash.split("?")[0];
  return cards.find((c) => c.hash === p);
}

/** A page's address of its own: read from an address of the page, written for it, and told whether the page is shown, then that it was opened. */
export interface OwnAddress {
  read(hash: string): void;
  write(): string;
  shown(on: boolean): void;
  opened(): void;
}

/** The pages with an address of their own, by their views. */
const owned: Record<string, OwnAddress> = {};

/** ownAddress hands over a page's address of its own (the log's, ui/src/log/index.tsx). */
export function ownAddress(view: string, a: OwnAddress): void {
  owned[view] = a;
}

/** What the cards with addresses read of an address after its "?", by the card's id. */
const cardReads: Record<string, (q: URLSearchParams | null) => void> = {};

/**
 * readCardAddress hands over how a card with an address of its own reads
 * what follows the address's "?" (My profile's key to mark,
 * profile/index.tsx): at every address the store reads (showAddress),
 * before the page is shown, with what follows the "?" where the address
 * names the card, null where it names another place; the address is then
 * written again as the card's alone.
 */
export function readCardAddress(card: string, read: (q: URLSearchParams | null) => void): void {
  cardReads[card] = read;
}

/** queryOf is what an address holds after its "?", nothing where it has none. */
function queryOf(hash: string): URLSearchParams {
  const i = hash.indexOf("?");
  return new URLSearchParams(i < 0 ? "" : hash.slice(i + 1));
}

/** isShown reports whether a page is the one shown, at this moment: a drawing that follows the page reads page.value. */
export function isShown(view: string): boolean {
  return page.peek() === view;
}

/** viewOfHash is the page an address names, the part before its "?", Rooms for none or another; a card's address names its page. */
function viewOfHash(hash: string): string {
  const p = hash.split("?")[0];
  const c = cardOf(p);
  if (c) return c.view;
  return Object.values(views).find((v) => v.hash && v.hash === p)?.view || "tiles";
}

/**
 * setAddress puts a page's hash into the address: as a new entry of the
 * browser's history ("push") or in place of the current one ("replace");
 * nothing where the address already says it.
 */
export function setAddress(hash: string, how?: string): void {
  if (location.hash === hash) return;
  const url = location.pathname + location.search + hash;
  if (how === "replace") history.replaceState(null, "", url);
  else history.pushState(null, "", url);
}

/**
 * showAddress shows the page the address names, a page with an address
 * of its own with the choices it holds, a card with what its address
 * holds after the "?" (readCardAddress), each card told of every address,
 * at the start and on back and forward; the address is written again in
 * place, a value dropped, and a card's after its "?".
 */
export function showAddress(): void {
  const view = viewOfHash(location.hash);
  if (owned[view]) owned[view].read(location.hash);
  const c = cardOf(location.hash);
  for (const [card, read] of Object.entries(cardReads)) read(c && c.card === card ? queryOf(location.hash) : null);
  showView(view, "replace", c?.card);
}

/**
 * mayShow reports whether the signed-in user may open a page, as the
 * page's may says (chrome/pages.ts); a view that is no page, no one. It
 * reads who is in as a signal, so that a drawing that asks follows a
 * sign-in.
 */
export function mayShow(view: string): boolean {
  void me.value;
  const v = views[view];
  return !!v && (!v.may || v.may());
}

/** viewScroll is where each page was left, by view: the pages share the window's one scroll position, and each comes back at its own. */
const viewScroll: Record<string, number> = {};

/**
 * showView shows a page, Rooms in place of one the user may not open,
 * puts it into the address as address says (a new entry by default;
 * Rooms in place of a page refused), a page with an address of its own
 * with its choices, the personal page at a card by the card's address,
 * and tells those that load what it shows (onView); the card shown comes
 * into view where it may be shown; else the page comes back where
 * it was left.
 */
export function showView(view: string, address = "push", card?: string): void {
  if (!mayShow(view)) {
    view = "tiles";
    address = "replace";
  }
  const was = page.peek();
  const left = view !== was ? was : null;
  if (left) viewScroll[left] = window.scrollY;
  const c = cards.find((x) => x.card === card);
  if (!c || c.view !== view || (c.may && !c.may())) card = undefined;
  const own = owned[view];
  setAddress(own ? own.write() : c && card ? c.hash : views[view]?.hash || "", address);
  page.value = view;
  for (const [v, a] of Object.entries(owned)) a.shown(v === view);
  for (const v of Object.values(views)) {
    for (const id of v.shows) document.getElementById(id)?.classList.toggle(v.hide || "hidden", view !== v.view);
  }
  if (own) own.opened();
  document.dispatchEvent(new CustomEvent("viewshown", { detail: view }));
  if (card) showCard(card, true);
  else if (left) restoreScroll(viewScroll[view] || 0);
}

/**
 * restoreScroll brings a page shown back to y, and again while its
 * content grows for a second after (a page that loads as it is shown),
 * until the user scrolls by hand.
 */
function restoreScroll(y: number): void {
  const go = () => window.scrollTo({ top: y, behavior: "instant" });
  go();
  if (!y) return;
  const by = ["wheel", "touchstart", "keydown", "pointerdown"];
  const ro = new ResizeObserver(go);
  const stop = () => {
    ro.disconnect();
    for (const ev of by) removeEventListener(ev, stop);
  };
  const main = document.querySelector("main");
  if (main) ro.observe(main);
  for (const ev of by) addEventListener(ev, stop, { passive: true });
  setTimeout(stop, 1000);
}

/** onView runs fn each time a page is shown (the viewshown event showView sends), for a part that loads what the page shows. */
export function onView(view: string, fn: () => void): void {
  document.addEventListener("viewshown", (ev) => {
    if ((ev as CustomEvent<string>).detail === view) fn();
  });
}

/** The functions the plain code calls, MassimoParts.pages; shown is the page shown, for login.js and the UI tests. */
export const pagesPart = { isShown, showView, showAddress, setAddress, mayShow, onView, ownAddress, shown: () => page.peek() };
