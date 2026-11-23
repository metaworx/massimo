// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/signin/index.tsx
 *
 * The sign-in as parts (AP UiParts, Block 14): the card into the
 * skeleton's #login-dialog, with the first owner's form in it on a server
 * without users (AP FirstOwner), and the open server's warning into
 * #open-banner-slot, each drawn once as parts.js loads, before any
 * sign-in is asked for, and from then on by their signals: who is in
 * (shared/me.ts, state.me underneath, which login.js applyCaps writes at
 * every read of it), and what askLogin and the plain code
 * hand over through MassimoParts.signIn.
 */

import type { LoginProvider } from "../api";
import { register, mounter } from "../shared/region";
import { readApp } from "../shared/app";
import { Banner } from "./Banner";
import { Card } from "./Card";
import { asksOwner, card, closed, forgetCard, open, toSignIn, type Way } from "./store";

mounter("login-dialog", () => <Card />)();
mounter("open-banner-slot", () => <Banner />)();

/** signedOut forgets what the card holds of the last opening (MassimoParts.signedOut, index.ts): a choice it waits for, its lines. */
export const signedOut = forgetCard;

/** The functions the plain code calls, MassimoParts.signIn. */
const signIn = register("signIn", {
  /** open takes askLogin's opening: the way chosen, or "owner" for the first owner's form, a provider's refusal to say ("" for none) and the sign-in it answers. */
  open(on: Way | "owner", said: string, ask: SignInAsk): void {
    open(on, said, ask);
  },
  /** asksOwner says whether the card asks for the first owner, after a read of who is in (login.js loadMe). */
  asksOwner(): boolean {
    return asksOwner();
  },
  /** toSignIn turns the first owner's form into the sign-in where a refusal says the server has users now (login.js askLogin); it reports whether it turned. */
  toSignIn(): boolean {
    return toSignIn();
  },
  /** closed takes the card's closing (askLogin's done). */
  closed(): void {
    closed();
  },
  /** providers takes the enabled providers for their buttons (login.js renderLoginProviders); none in the app. */
  providers(list: LoginProvider[]): void {
    card.providers = list;
  },
  /** readApp asks the app for its status of the login by fingerprint (login.js askLogin, not awaited). */
  readApp(): Promise<void> {
    return readApp();
  },
  /** selfSigned takes whether the card asks to compare the certificate's fingerprint first (header.js renderTLS). */
  selfSigned(on: boolean): void {
    card.selfSigned = on;
  },
});

/** MassimoParts.signIn, as index.ts declares it. */
export type SignInPart = typeof signIn;
