// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/signin/store.ts
 *
 * What the sign-in's card is drawn from (AP UiParts, Block 14): the
 * opening askLogin of login.js asks for (open: the way chosen, a
 * provider's refusal, the sign-in it answers), the way chosen, the
 * form's line, the providers' buttons and their line, the self-signed
 * certificate's line, the login by fingerprint not kept with the choice
 * it waits for, and the focus asked for. No name, password or key typed
 * is kept here: the card's fields are not controlled (UiParts D210),
 * read at the submit and emptied at each opening.
 */

import type { LoginProvider } from "../api";
import { live } from "../shared/live";
import type { Outcome } from "../shared/action";

/** A way to sign in, by the button's data-way: the name and the password, or an API key. */
export type Way = "password" | "key";

/** Where the focus is asked to go: the chosen way's first field, the password, Try again. */
export type FocusOn = "way" | "password" | "retry";

/** The card's state; opened counts the openings, focus the focus asked for. */
export const card = live({
  opened: 0,
  way: "password" as Way,
  said: null as Outcome | null,
  providers: [] as LoginProvider[],
  providersSaid: null as Outcome | null,
  selfSigned: false,
  keepFailed: null as string | null,
  waysOff: false,
  focus: { n: 0, on: "way" as FocusOn },
});

/** The sign-in the card answers, as askLogin handed it over; null while none is open. */
let asking: SignInAsk | null = null;

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
 * open takes an opening of askLogin: the way chosen, a provider's
 * refusal to say (said, "" for none) and the sign-in it answers; the
 * card's lines cleared, a choice still waited for dropped, and the
 * fields emptied and the focus given with the drawing that follows.
 */
export function open(way: Way, said: string, ask: SignInAsk): void {
  dropChoice();
  asking = ask;
  card.way = way;
  card.said = null;
  card.providersSaid = said ? { text: said, ok: false } : null;
  card.opened++;
  askFocus("way");
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
  card.providersSaid = null;
}
