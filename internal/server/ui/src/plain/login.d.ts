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
 * bridge, the token kept, a late sign-in's session ended, the sign-in by
 * OPAQUE and the page's start after it; for ☰ the app's ways to its
 * servers and to its settings.
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
  /** The server a page in the app was opened for (login.status's server, contract version 5): its name in the app's list, the address it was opened at, its node (the instance id of /healthz) and its pinned certificate fingerprint, "" for none. */
  interface AppServer {
    name?: string;
    url?: string;
    node?: string;
    fp?: string;
  }
  /** The app's status of the login by fingerprint (docs/app.md): whether the phone can keep one, the user kept, whether the address is plain HTTP outside the private ranges; from contract version 5 the id of the key kept, 0 for none or not known, and the server the page was opened for. */
  interface AppLoginStatus {
    can?: boolean;
    user?: string;
    plain?: boolean;
    ask?: string;
    key_id?: number;
    server?: AppServer;
  }
  /** In the app, the bridge's login by fingerprint (contract version 2 and later); null in a browser. */
  function appLogin(): {
    status(): Promise<AppLoginStatus | null>;
    keep(): Promise<boolean>;
    fingerprint(): Promise<string | null>;
  } | null;
  /** In the app, the way back to its servers (Select server); null in a browser. */
  function appConnect(): (() => Promise<unknown>) | null;
  /** In the app, the way to this server's settings on the app's own page (☰'s This server's settings, contract version 5); null in a browser and in an app before it. */
  function appServerSettings(): (() => Promise<unknown>) | null;
  /** In the app, the way to the app's settings on its own page (☰'s App settings, contract version 5); null in a browser and in an app before it. */
  function appAppSettings(): (() => Promise<unknown>) | null;
  /** A sign-in askLogin opened, which the sign-in's card hands back to finishSignIn: who was in, the pushes held, its end; whether it opened on the first owner's form, and its dismissal (Skip for now). */
  interface SignInAsk {
    readonly was: string;
    readonly owner: boolean;
    /** Whether it has ended already: its card closed or opened anew. */
    readonly closed: boolean;
    dismiss(): void;
  }
  /** Opens the sign-in's card, or with "owner" the first owner's form in it, and answers whether one is in; while it is open, a refusal turns the first owner's form to the sign-in and the same promise is handed out. */
  function askLogin(on?: "owner"): Promise<boolean>;
  /** Goes on after a sign-in of the card: settleSignIn, the card closed and askLogin answered, the page started where no start waits for it; nothing for an ask closed already, and it reports whether it ended the ask. */
  function finishSignIn(ask: SignInAsk, me?: unknown): Promise<boolean>;
  /** Starts a sign-in through a provider and answers the provider's page to go to, which the card opens; its refusal is thrown. */
  function startProvider(key: string): Promise<string>;
  /** Keeps the page's token, or forgets it with null: in the browser's storage, or in the app's; a session it replaces is ended at the server once. Its promise is the bridge's answer, null in a browser and where the bridge failed. */
  function setToken(token: string | null): Promise<unknown>;
  /** Ends a session at the server that the page does not keep, a sign-in's that answered once its opening had ended: POST /api/v1/logout with it, not awaited, a refusal ignored, nothing for no token; kept, the app's bridge keeps it (the login by fingerprint's), which is left with none where the page holds no token. */
  function endSession(token: string, kept?: boolean): void;
  /** The bridge's answer to the last token setToken handed it, a sign-in's ({revoked}, contract version 5); null in a browser, before any, and where the bridge failed. */
  function tokenAnswer(): Promise<unknown>;
  /** Signs in by OPAQUE, without a token and without the dialog: the session, its token among it; the answer's error is thrown. */
  function signIn(user: string, password: string): Promise<{ token: string }>;
  /** Signs out as Sign out does: the page's dialog first where settings are unsaved, then a session ends and the page forgets who was in. */
  function logout(): Promise<void>;
  /** Reads who the page is signed in as again and applies it. */
  function loadMe(asks?: boolean): Promise<void>;
  /** A password of version 2 made in the browser for a name: the fields that set it, for POST /api/v1/users or PUT …/users/{name}. */
  function registerPassword(name: string, password: string): Promise<{ record: string; ksf: string }>;
  /** The proof of one's own current password, or for a user still of version 1 the password itself, for PUT …/users/{name}. */
  function currentPassword(user: User, password: string): Promise<{ proof?: string; current_password?: string }>;
  /** The page's own address, with a slash at its end: the base of the redirect URI and of a link's return. */
  function pageBase(): string;
}

export {};
