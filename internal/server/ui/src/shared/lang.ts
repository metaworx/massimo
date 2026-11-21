// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/shared/lang.ts
 *
 * The page's language as a signal, which every part that shows a text
 * reads, so that a change of language draws it again; applyLanguage
 * (app.js) raises it through MassimoParts.language. A part calls t() for
 * its texts and carries no data-i18n, which applyI18n would write over.
 * Beside it, the choices of the language, a text with parts in it, and
 * parts joined into a line.
 */

import { signal } from "@preact/signals";
import type { ComponentChildren } from "preact";

/**
 * The page's language, i18n.lang, as the parts last drew it: "" until
 * the page's language has loaded and init of app.js raises it, so that
 * that first raise draws again every part drawn as parts.js loaded,
 * before any table had come, English too.
 */
export const lang = signal("");

/** languageChanged takes the page's language after applyLanguage (app.js). */
export function languageChanged(): void {
  lang.value = i18n.lang;
}

/**
 * useLang reads the language in a part's drawing, so that the part is
 * drawn again when it changes (rule 2 of AP UiParts: a part reads all it
 * depends on as signals); it returns it.
 */
export function useLang(): string {
  return lang.value;
}

/**
 * languageOptions are the choices of the language, as My settings and the
 * sign-in card's ☰ offer them: from the browser, then every language with
 * a translation table, by its own name.
 */
export function languageOptions(): [string, string][] {
  const options: [string, string][] = [["auto", t("lang.auto") + " (" + LANGUAGE_NAMES[browserLanguage()] + ")"]];
  for (const l of languages()) options.push([l, LANGUAGE_NAMES[l] || l]);
  return options;
}

/**
 * tParts is t() with parts among params, for a link inside a sentence:
 * the text in pieces, each part in its placeholder's place (tNodes of
 * i18n.js, for a part's children).
 */
export function tParts(key: string, texts: Record<string, string | number>, parts: Record<string, ComponentChildren>): ComponentChildren[] {
  const s = t(key, texts);
  const out: ComponentChildren[] = [];
  let last = 0;
  for (const m of s.matchAll(/\{(\w+)\}/g)) {
    if (!(m[1] in parts)) continue;
    out.push(s.slice(last, m.index), parts[m[1]]);
    last = (m.index ?? 0) + m[0].length;
  }
  out.push(s.slice(last));
  return out.filter((x) => x !== "");
}

/**
 * joinParts are parts with " · " between them, the empty ones left out;
 * with lead, also before the first, for a line that goes on after
 * something else.
 */
export function joinParts(parts: ComponentChildren[], lead?: boolean): ComponentChildren[] {
  const out = parts.filter((p) => p !== "" && p !== undefined && p !== null && !(Array.isArray(p) && !p.length)).flatMap((p) => [" · ", p]);
  return lead ? out : out.slice(1);
}
