// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/plain/i18n.d.ts
 *
 * The globals of the plain script ui/i18n.js that the parts call, as
 * TypeScript sees them: the texts of the page's language. A file per
 * plain script, each naming only what a part calls; it goes with its
 * script's last caller among the parts.
 */

declare global {
  /** The page's language (i18n.lang); applyLanguage tells the parts of a change (shared/lang.ts). */
  const i18n: { lang: string; setting: string };
  /** The text of a key in the page's language, {name} filled from params. */
  function t(key: string, params?: Record<string, string | number>): string;
  /** The plural form of a key for n, {n} filled. */
  function tn(key: string, n: number, params?: Record<string, string | number>): string;
}

export {};
