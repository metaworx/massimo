// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/plain/theme.d.ts
 *
 * The globals of the plain script ui/theme.js that the parts call: the
 * built-in schemes and the user's own, the scheme being edited, the
 * colours' checks and contrast, a scheme stored, chosen, exported and
 * imported, and the choices of the mode, the blinking and the icons,
 * each kept as the other settings are.
 */

declare global {
  /** A scheme's colours, by token. */
  type SchemeSet = Record<string, string>;
  /** A scheme resolved by its id: built-in or the user's own. */
  interface Scheme {
    id: string;
    name: string;
    builtin: boolean;
    light: SchemeSet;
    dark: SchemeSet;
  }
  /** The user's schemes, the scheme being edited while the editor is open, and the id of the one it started from. */
  const themeState: {
    custom: { name: string; light: SchemeSet; dark: SchemeSet }[];
    draft: { name: string; light: SchemeSet; dark: SchemeSet } | null;
    base: string | null;
  };
  /** The built-in schemes, by id; their names are appearance.scheme.<id>. */
  const BUILTIN_SCHEMES: Record<string, { light: SchemeSet; dark: SchemeSet }>;
  /** The default scheme's id. */
  const DEFAULT_SCHEME: string;
  /** What a custom scheme's id starts with, its name after it. */
  const CUSTOM_PREFIX: string;
  /** How long a scheme's name may be. */
  const SCHEME_NAME_MAX: number;
  /** The colour tokens, in the order of style.css and of the editor. */
  const THEME_TOKENS: string[];
  /** A background token and the text token drawn on it, graded for text. */
  const CONTRAST_PAIRS: Record<string, string>;
  /** An icon colour and the background it stands on, graded at 3:1. */
  const MARK_PAIRS: Record<string, string>;
  /** A scheme by its id, null for none. */
  function schemeById(id: string | null): Scheme | null;
  /** A name trimmed to what the select and a file name take, else fallback. */
  function schemeName(v: string, fallback: string): string;
  /** A colour as #rrggbb in lower case, null where the value is none. */
  function hexColour(v: string): string | null;
  /** A colour typed or pasted, with or without its #, null where it is none. */
  function typedColour(v: string): string | null;
  /** The WCAG contrast ratio of two colours. */
  function contrastRatio(a: string, b: string): number;
  /** A ratio's grade as a class: ok, warn or bad. */
  function contrastClass(ratio: number): string;
  /** Stores a scheme under its name, replacing one of that name, and returns its id. */
  function putCustomScheme(name: string, light: SchemeSet, dark: SchemeSet): string;
  /** Keeps the user's schemes in the browser and sends them to the server. */
  function storeCustomSchemes(): void;
  /** A scheme's document, as its file carries it. */
  function schemeDoc(s: { name: string; light: SchemeSet; dark: SchemeSet }): { name: string };
  /** An imported document checked: the scheme, or an error with the key of the reason. */
  function parseSchemeDoc(doc: unknown): { name: string; light: SchemeSet; dark: SchemeSet };
  /** Applies the mode and the scheme chosen again, or the draft's colours while it is edited. */
  function reapplyTheme(): void;
  /** The choices of the mode, the scheme, the blinking and a drawing, each kept as the other settings are, and applied. */
  function chooseMode(mode: string): void;
  function chooseScheme(id: string): void;
  function chooseBlink(mode: string): void;
  function chooseIcon(key: string, value: string): void;
}

export {};
