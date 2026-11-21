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
 * all. No name, password or key typed is kept here: the card's fields
 * are not controlled (UiParts D210), read at the submit and emptied at
 * each opening.
 */

import type { LoginProvider } from "../api";
import { live } from "../shared/live";
import type { Outcome } from "../shared/action";
import { firstOwnerOf, me } from "../shared/me";

/** A way to sign in, by the button's data-way: the name and the password, or an API key. */
export type Way = "password" | "key";

/** Where the focus is asked to go: the chosen way's first field, the password, Try again, the first owner's name. */
export type FocusOn = "way" | "password" | "retry" | "owner";

/** The card's state; opened counts the openings, owner says the first owner's form is shown instead of the ways, focus the focus asked for. */
export const card = live({
  opened: 0,
  owner: false,
  way: "password" as Way,
  said: null as Outcome | null,
  ownerSaid: null as Outcome | null,
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
  card.said = null;
  card.ownerSaid = null;
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
 * fields stay as the opening left them. It reports whether it turned.
 */
export function toSignIn(): boolean {
  if (!card.owner) return false;
  card.owner = false;
  card.way = "password";
  card.ownerSaid = null;
  card.said = { text: t("setup.exists"), ok: false };
  askFocus("way");
  return true;
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
  card.providersSaid = null;
}
