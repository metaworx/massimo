// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/signin/store.ts
 *
 * What the sign-in's card is drawn from (AP UiParts, Block 14): the
 * opening askLogin of login.js asks for (open: the way chosen, or the
 * first owner's form, a provider's refusal, the sign-in it answers), the
 * way chosen, the form's line, the providers' buttons and their line,
 * the self-signed certificate's line, the login by fingerprint not kept
 * with the choice it waits for, and the focus asked for; on a server
 * without users whether the card asks for the first owner (asksOwner),
 * Skip for now, which holds for the page's load (AP FirstOwner, D10),
 * and the card turned to the sign-in where the server has users after
 * all; the step of the first owner's place, the form or the confirmation
 * that keeps the server open (AP FirstOwner, Block 3), its line, and
 * whether a request of the card is under way, which keeps every control
 * that ends the card off. No name, password or key typed is kept here: the card's fields
 * are not controlled (UiParts D210), read at the submit and emptied at
 * each opening.
 */

import type { LoginProvider } from "../api";
import { live } from "../shared/live";
import type { Outcome } from "../shared/action";
import { firstOwnerOf, me } from "../shared/me";

/** A way to sign in, by the button's data-way: the name and the password, or an API key. */
export type Way = "password" | "key";

/** Where the focus is asked to go: the chosen way's first field, the password, Try again, the first owner's name, the confirmation that keeps the server open, the button that opens it. */
export type FocusOn = "way" | "password" | "retry" | "owner" | "keep" | "keepButton";

/** The first owner's place: the form, or the confirmation that keeps the server open. */
export type OwnerStep = "form" | "keep";

/** The card's state; opened counts the openings, owner says the first owner's form is shown instead of the ways, focus the focus asked for. */
export const card = live({
  opened: 0,
  owner: false,
  way: "password" as Way,
  said: null as Outcome | null,
  ownerSaid: null as Outcome | null,
  ownerStep: "form" as OwnerStep,
  keepSaid: null as Outcome | null,
  working: false,
  providers: [] as LoginProvider[],
  providersSaid: null as Outcome | null,
  selfSigned: false,
  keepFailed: null as string | null,
  waysOff: false,
  focus: { n: 0, on: "way" as FocusOn },
});

/** The sign-in the card answers, as askLogin handed it over; null while none is open. */
let asking: SignInAsk | null = null;

/** skipped is Skip for now taken in this load of the page: the card asks for the first owner no more until the next (AP FirstOwner, D10). */
let skipped = false;

/** The choice the login by fingerprint not kept waits for: true Try again, false Sign in without it, null dropped. */
let choose: ((again: boolean | null) => void) | null = null;

/** askFocus asks for the focus on one of the card's places, given in the drawing that follows. */
export function askFocus(on: FocusOn): void {
  card.focus = { n: card.focus.n + 1, on };
}

/** dropChoice drops a choice still waited for: its sign-in goes no further. */
function dropChoice(): void {
  const c = choose;
  choose = null;
  card.keepFailed = null;
  card.waysOff = false;
  if (c) c(null);
}

/**
 * open takes an opening of askLogin: the way chosen, or "owner" for the
 * first owner's form, a provider's refusal to say (said, "" for none) and
 * the sign-in it answers; the card's lines cleared, a choice still waited
 * for dropped, and the fields emptied and the focus given with the
 * drawing that follows.
 */
export function open(on: Way | "owner", said: string, ask: SignInAsk): void {
  dropChoice();
  asking = ask;
  card.owner = on === "owner";
  card.way = on === "owner" ? "password" : on;
  card.ownerStep = "form";
  card.said = null;
  card.ownerSaid = null;
  card.keepSaid = null;
  card.providersSaid = said ? { text: said, ok: false } : null;
  card.opened++;
  askFocus(on === "owner" ? "owner" : "way");
}

/**
 * asksOwner says whether the card asks for the first owner after a read
 * of who is in (loadMe of login.js): an open server, which has a store
 * (AP FirstOwner, D6), where who is in says the owner can be created from
 * here, not kept open on purpose (auth.keep_open), and not skipped in
 * this load (D10).
 */
export function asksOwner(): boolean {
  const m = me.peek();
  return !skipped && firstOwnerOf(m) && !m?.kept_open;
}

/**
 * toSignIn turns the first owner's form into the sign-in, in the same
 * opening, where a request was refused for lack of a session or the
 * owner exists already: the server has users now. The password's way
 * chosen and its first field given the focus, the reason said; the
 * fields stay as the opening left them; the card's request that met the
 * refusal is over as far as the card goes, so its ways out are on again.
 * It reports whether it turned.
 */
export function toSignIn(): boolean {
  if (!card.owner) return false;
  card.owner = false;
  card.working = false;
  card.way = "password";
  card.ownerStep = "form";
  card.ownerSaid = null;
  card.keepSaid = null;
  card.said = { text: t("setup.exists"), ok: false };
  askFocus("way");
  return true;
}

/** keepAsk shows the confirmation that keeps the server open in the form's place, its line cleared (AP FirstOwner, D4). */
export function keepAsk(): void {
  card.ownerStep = "keep";
  card.keepSaid = null;
  askFocus("keep");
}

/** keepBack shows the first owner's form again, what was typed in both kept, the focus on the button that asked. */
export function keepBack(): void {
  card.ownerStep = "form";
  card.keepSaid = null;
  askFocus("keepButton");
}

/**
 * sameAddress says whether an address entered is this page's own (AP
 * FirstOwner, D13, D28): parsed as a URL, trimmed, its origin the page's
 * (scheme, host and port as the browser has them, so letters' case, a
 * default port typed and another spelling of the same host do not
 * matter), its path "/" or the page's own base with or without the slash
 * at its end, and no user, password, query or fragment. An address
 * without a scheme is none.
 */
export function sameAddress(input: string): boolean {
  const v = input.trim();
  if (!v) return false;
  let u: URL;
  try {
    u = new URL(v);
  } catch {
    return false;
  }
  if (u.origin !== location.origin || u.username || u.password || u.search || u.hash || v.includes("?") || v.includes("#")) return false;
  const base = new URL(pageBase()).pathname;
  return u.pathname === "/" || u.pathname === base || u.pathname + "/" === base;
}

/** skip takes Skip for now: the card closes, as a dismissal, and asks no more in this load (AP FirstOwner, D10). */
export function skip(): void {
  skipped = true;
  if (asking) asking.dismiss();
}

/** closed takes the card's closing: a choice still waited for dropped. */
export function closed(): void {
  dropChoice();
  asking = null;
}

/** ask is the sign-in the card answers now, taken at a submit's start; null while none is open. */
export function ask(): SignInAsk | null {
  return asking;
}

/** chooseWay shows a way's form and asks for the focus in its first field. */
export function chooseWay(way: Way): void {
  card.way = way;
  askFocus("way");
}

/** askKeep says that the login by fingerprint was not kept (text), hides the form and the ways, and resolves with the choice: true Try again, false Sign in without it, null dropped. */
export function askKeep(text: string): Promise<boolean | null> {
  card.keepFailed = text;
  card.waysOff = true;
  askFocus("retry");
  return new Promise((resolve) => {
    choose = resolve;
  });
}

/** keepChosen takes the choice of the box that says the login was not kept. */
export function keepChosen(again: boolean): void {
  const c = choose;
  choose = null;
  card.keepFailed = null;
  if (c) c(again);
}

/** keepEnded shows the form and the ways again once the login by fingerprint is kept or left. */
export function keepEnded(): void {
  card.keepFailed = null;
  card.waysOff = false;
}

/** forgetCard forgets at a sign-out what the card holds of the last opening: a choice waited for, the lines (MassimoParts.signedOut). */
export function forgetCard(): void {
  dropChoice();
  card.said = null;
  card.ownerSaid = null;
  card.keepSaid = null;
  card.providersSaid = null;
}
