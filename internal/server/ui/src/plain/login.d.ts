// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/plain/login.d.ts
 *
 * The globals of the plain script ui/login.js that the parts call (who
 * is in and what it may is shared/me.ts'): whether the settings follow
 * the one signed in, a link come back for it, the roles' names, who is read
 * again, the sign-out, a password registered by OPAQUE and the current
 * one proved, and the page's address; for the sign-in's parts the app's
 * bridge, the token kept, the sign-in by OPAQUE and the page's start
 * after it.
 */

import type { User } from "../api";

declare global {
  /** A link of the own account come back from a provider (oidcReturn), for My profile to show or say why there is none; null for none, set back to null once taken. */
  let oidcLinked: { confirm?: string; error?: string } | null;
  /** Whether the settings follow the one signed in: a user with the capability preferences. */
  function canSync(): boolean;
  /** A role's display name: the name given, else the key's translated name, else the key. */
  function roleLabel(key: string, name?: string): string;
  /** The keys of the six default roles, for a role choice that cannot read the roles. */
  const DEFAULT_ROLES: string[];
  /** The app's status of the login by fingerprint (docs/app.md): whether the phone can keep one, the user kept, whether the address is plain HTTP outside the private ranges. */
  interface AppLoginStatus {
    can?: boolean;
    user?: string;
    plain?: boolean;
    ask?: string;
  }
  /** In the app, the bridge's login by fingerprint (contract version 2 and later); null in a browser. */
  function appLogin(): {
    status(): Promise<AppLoginStatus | null>;
    keep(): Promise<boolean>;
    fingerprint(): Promise<string | null>;
  } | null;
  /** In the app, the way back to its servers (Select server); null in a browser. */
  function appConnect(): (() => Promise<unknown>) | null;
  /** A sign-in askLogin opened, which the sign-in's card hands back to finishSignIn: who was in, the pushes held, its end. */
  interface SignInAsk {
    readonly was: string;
  }
  /** Goes on after a sign-in of the card: settleSignIn, the card closed and askLogin answered, the page started where no start waits for it. */
  function finishSignIn(ask: SignInAsk): Promise<void>;
  /** Starts a sign-in through a provider and opens its page; its refusal is thrown. */
  function startProvider(key: string): Promise<void>;
  /** Keeps the page's token, or forgets it with null: in the browser's storage, or in the app's. */
  function setToken(token: string | null): void;
  /** Signs in by OPAQUE, without a token and without the dialog: the session, its token among it; the answer's error is thrown. */
  function signIn(user: string, password: string): Promise<{ token: string }>;
  /** Reads who is in again and starts the poll, the stream and the data, for a sign-in that no start waited for (the first owner). */
  function afterLogin(): void;
  /** Signs out as Sign out does: the page's dialog first where settings are unsaved, then a session ends and the page forgets who was in. */
  function logout(): Promise<void>;
  /** Reads who the page is signed in as again and applies it. */
  function loadMe(): Promise<void>;
  /** A password of version 2 made in the browser for a name: the fields that set it, for POST /api/v1/users or PUT …/users/{name}. */
  function registerPassword(name: string, password: string): Promise<{ record: string; ksf: string }>;
  /** The proof of one's own current password, or for a user still of version 1 the password itself, for PUT …/users/{name}. */
  function currentPassword(user: User, password: string): Promise<{ proof?: string; current_password?: string }>;
  /** The page's own address, with a slash at its end: the base of the redirect URI and of a link's return. */
  function pageBase(): string;
}

export {};
