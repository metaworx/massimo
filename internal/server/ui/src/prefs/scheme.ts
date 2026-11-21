// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/prefs/scheme.ts
 *
 * The scheme editor's state and what it does, as appearance.js did it:
 * the scheme being edited stays theme.js' themeState.draft, which the
 * page applies as it changes (reapplyTheme), and the one it started
 * from themeState.base; beside them here, whether the editor is open,
 * a counter of the draft's changes, what each hex field holds typed or
 * refused, and the line under the card's buttons. Opening starts a draft
 * from the chosen scheme, closing drops it and forgets the editor's
 * form (shared/forms.ts); Save as keeps it in the browser, Delete removes
 * the user's scheme it started from, Export downloads it, Import keeps a
 * file's.
 */

import type { Outcome } from "../shared/action";
import { forgetForms } from "../shared/forms";
import { live, liveMap } from "../shared/live";
import { session, stillSession } from "../shared/session";

/** The key of the scheme editor's form (shared/forms.ts); My settings' pill shows it. */
export const SCHEME_FORM = "scheme";

/** Whether the editor is open; ver counts the draft's changes and the editor's starts, which draw it again. */
export const editor = live({ open: false, ver: 0, refusedLast: "" });

/** Per hex field (by its colour input's id): the text typed where it is not the colour, and the text refused. */
export const hexUI = liveMap(() => ({ typed: null as string | null, refused: null as string | null }));

/** The outcome of a save, a delete or an import, under the card's buttons (#scheme-msg). */
export const said = live({ scheme: null as Outcome | null });

/** schemeMessage shows text under the buttons, ok green, else red; empty clears it. */
export function schemeMessage(text: string, ok?: boolean): void {
  said.scheme = text ? { text, ok: !!ok } : null;
}

/** changed draws the editor again after a change of the draft. */
export function changed(): void {
  editor.ver++;
}

/** clearHex forgets what the hex fields hold typed or refused: the draft started again or put back. */
function clearHex(): void {
  hexUI.clear();
  editor.refusedLast = "";
}

/** openEditor starts a draft from the chosen scheme and shows it; every change applies to the page at once. */
export function openEditor(): void {
  const s = schemeById(state.settings.scheme) || schemeById(DEFAULT_SCHEME);
  if (!s) return;
  themeState.base = s.id;
  themeState.draft = { name: s.name, light: { ...s.light }, dark: { ...s.dark } };
  schemeMessage("");
  clearHex();
  editor.open = true;
  changed();
  reapplyTheme();
}

/** closeEditor drops the draft and the editor's form; the chosen scheme is applied again. */
export function closeEditor(): void {
  if (!themeState.draft && !editor.open) return;
  themeState.draft = null;
  themeState.base = null;
  editor.open = false;
  clearHex();
  forgetForms(SCHEME_FORM);
  changed();
  reapplyTheme();
}

/** baseScheme is the scheme the editor started from, as it is stored, else the default. */
export function baseScheme(): Scheme | null {
  return schemeById(themeState.base) || schemeById(DEFAULT_SCHEME);
}

/** resetColours puts the draft's colours back to the scheme it started from. */
export function resetColours(): void {
  const s = baseScheme();
  const d = themeState.draft;
  if (!s || !d) return;
  d.light = { ...s.light };
  d.dark = { ...s.dark };
  clearHex();
  changed();
  reapplyTheme();
}

/** coloursChanged says whether a colour of the draft differs from the scheme it started from. */
export function coloursChanged(): boolean {
  const s = baseScheme();
  const d = themeState.draft;
  if (!s || !d) return false;
  return THEME_TOKENS.some((k) => d.light[k] !== s.light[k] || d.dark[k] !== s.dark[k]);
}

/** takeColour sets a token's colour of a set in the draft and applies it. */
export function takeColour(mode: "light" | "dark", k: string, c: string): void {
  const d = themeState.draft;
  if (!d) return;
  d[mode][k] = c;
  changed();
  reapplyTheme();
}

/** saveDraft keeps the draft in the browser under the name given, replacing a scheme of that name, and chooses it. */
export function saveDraft(typed: string): void {
  const d = themeState.draft;
  if (!d) return;
  const name = schemeName(typed, t("appearance.unnamed"));
  d.name = name;
  themeState.base = putCustomScheme(name, d.light, d.dark);
  chooseScheme(themeState.base);
  schemeMessage(t("appearance.saved", { name }), true);
  changed();
}

/** deleteDraft removes the user's scheme the editor started from and goes back to the default. */
export function deleteDraft(): void {
  const s = schemeById(themeState.base);
  if (!s || s.builtin) return;
  themeState.custom = themeState.custom.filter((c) => c.name !== s.name);
  storeCustomSchemes();
  closeEditor();
  chooseScheme(DEFAULT_SCHEME);
  schemeMessage(t("appearance.deleted", { name: s.name }), true);
}

/** fileSlug makes a scheme's name safe for a file name. */
function fileSlug(name: string): string {
  return name.toLowerCase().replace(/[^a-z0-9]+/g, "-").replace(/^-+|-+$/g, "") || "scheme";
}

/** exportScheme downloads the draft, or else the chosen scheme, as a file. */
export function exportScheme(): void {
  const s = themeState.draft || schemeById(state.settings.scheme) || schemeById(DEFAULT_SCHEME);
  if (!s) return;
  const doc = schemeDoc(s);
  downloadBlob("massimo-scheme-" + fileSlug(doc.name) + ".json", new Blob([JSON.stringify(doc, null, 1) + "\n"], { type: "application/json" }));
}

/** importScheme reads a scheme file, keeps it under its name and chooses it; a file that is not a scheme says why. */
export async function importScheme(file: File): Promise<void> {
  const n = session();
  let s: { name: string; light: SchemeSet; dark: SchemeSet };
  try {
    let doc: unknown;
    try {
      doc = JSON.parse(await file.text());
    } catch {
      throw new Error("appearance.errNotJson");
    }
    s = parseSchemeDoc(doc);
  } catch (e) {
    if (stillSession(n)) schemeMessage(t("appearance.importBad", { error: t((e as Error).message) }), false);
    return;
  }
  if (!stillSession(n)) return; // read for the user who was: nothing of it for the next
  closeEditor();
  chooseScheme(putCustomScheme(s.name, s.light, s.dark));
  schemeMessage(t("appearance.importDone", { name: s.name }), true);
}

/** forgetEditor closes the editor and clears its line at a sign-out (MassimoParts.signedOut): nothing of the scheme being edited is left for the next user. */
export function forgetEditor(): void {
  closeEditor();
  schemeMessage("");
}
