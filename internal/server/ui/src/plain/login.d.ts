// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/plain/login.d.ts
 *
 * The globals of the plain script ui/login.js that the parts call: the
 * capabilities of the one signed in, who it is, the roles' names, who is
 * read again, the sign-out, a password registered by OPAQUE and the
 * current one proved, and the page's address.
 */

import type { User } from "../api";

declare global {
  /** Whether the one signed in has a capability; true without users. */
  function hasCap(c: string): boolean;
  /** Whether the one signed in may read the users: with users, guests or passwords. */
  function canManageUsers(): boolean;
  /** Whether a user's login name is the one signed in. */
  function isSelf(name: string): boolean;
  /** A role's display name: the name given, else the key's translated name, else the key. */
  function roleLabel(key: string, name?: string): string;
  /** The keys of the six default roles, for a role choice that cannot read the roles. */
  const DEFAULT_ROLES: string[];
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
