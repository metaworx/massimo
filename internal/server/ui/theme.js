// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

// The Massimo web UI, appearance: the mode (light, dark, or as the
// system) and the colour scheme. A scheme is the 19 colour tokens of
// style.css, once for light and once for dark; the mode picks which set
// is applied, with style.setProperty on <html>, since the page's
// Content-Security-Policy would refuse a generated <style> element. The
// built-in schemes are here; the user's own are kept in the browser
// (massimo.schemes, JSON) and travel as files of the form
//
//   {"format": "massimo-scheme/1", "name": "Mine",
//    "light": {"bg": "#f6f6f4", …}, "dark": {"bg": "#151514", …}}
//
// which a server will later store per user, so the format is fixed. The
// Settings page's Appearance card, the editor with a contrast hint
// (WCAG ratio) for the pairs that must stay readable, Export and Import,
// is a part (ui/src/prefs/), which follows the settings as signals. Loaded after app.js (readStored, store, state, the DOM helpers); the
// stored choice is applied as soon as this file runs, before the page
// is drawn, so a scheme does not flash the default first.
"use strict";

const SCHEME_FORMAT = "massimo-scheme/1";
const THEME_MODES = ["system", "light", "dark"];
const DEFAULT_SCHEME = "massimo";

// The tokens, in the order of style.css and of the editor.
const THEME_TOKENS = [
  "bg", "fg", "muted", "card", "card-head", "line", "accent", "accent-fg", "heat",
  "ok", "warn", "warn-mark", "bad", "notice-bg", "notice-fg", "fill-ok", "fill-warn", "fill-bad", "fill-info", "fill-fg",
];

// The pairs that must stay readable: a background token and the text
// token drawn on it. The editor shows their contrast next to the
// background.
const CONTRAST_PAIRS = {
  bg: "fg",
  accent: "accent-fg",
  "fill-ok": "fill-fg",
  "fill-warn": "fill-fg",
  "fill-bad": "fill-fg",
  "fill-info": "fill-fg",
  "notice-bg": "notice-fg",
};

// The colours that draw icons and marks, with the background they stand
// on, graded at 3:1 (WCAG's contrast for a graphic, not text): the
// editor shows their contrast next to the icon colour. warn-mark is the
// amber of the icons and their triangles; warn, the deeper amber of the
// text, reads at 4.5.
const MARK_PAIRS = { "warn-mark": "card" };

// The built-in schemes, by id; their names are appearance.scheme.<id>
// in the translation table. "massimo" is the default and repeats the
// token blocks of style.css, which stay the source for a page without
// a stored scheme (TestUIDefaultSchemeMatchesCSS checks that they
// agree). "contrast" is black on white with stronger lines and darker
// colours, "graphite" keeps the default's tones with a grey accent in
// place of the blue. Every pair of CONTRAST_PAIRS, the muted text on
// the page and the tile, and the ok, warn and bad texts on a tile are
// at 4.5 or better in both sets of all three, and the icons' colours of
// MARK_PAIRS, ok and bad at 3 on a tile.
const BUILTIN_SCHEMES = {
  massimo: {
    light: {
      "bg": "#f6f6f4", "fg": "#1c1c1a", "muted": "#6b6b66", "card": "#ffffff", "card-head": "#eeeeea", "line": "#dcdcd6",
      "accent": "#1f63c6", "accent-fg": "#ffffff", "heat": "#b5452c", "ok": "#2d7a3a", "warn": "#b34d0f", "warn-mark": "#d05911", "bad": "#991b1b",
      "notice-bg": "#fff4d6", "notice-fg": "#5a3f00",
      "fill-ok": "#2d7a3a", "fill-warn": "#8a5f00", "fill-bad": "#991b1b", "fill-info": "#6b3fb3", "fill-fg": "#ffffff",
    },
    dark: {
      "bg": "#151514", "fg": "#ececea", "muted": "#9b9b95", "card": "#202020", "card-head": "#2a2a29", "line": "#353533",
      "accent": "#6ea8fe", "accent-fg": "#151514", "heat": "#e0674a", "ok": "#6fc37c", "warn": "#ffa94d", "warn-mark": "#ffa94d", "bad": "#ff453a",
      "notice-bg": "#3a3012", "notice-fg": "#f1dfae",
      "fill-ok": "#6fc37c", "fill-warn": "#e3b341", "fill-bad": "#ff453a", "fill-info": "#c4a7f5", "fill-fg": "#151514",
    },
  },
  contrast: {
    light: {
      "bg": "#ffffff", "fg": "#000000", "muted": "#3d3d3d", "card": "#ffffff", "card-head": "#efefef", "line": "#6b6b6b",
      "accent": "#0b3d91", "accent-fg": "#ffffff", "heat": "#9b2f12", "ok": "#0b5e1e", "warn": "#8f3d0b", "warn-mark": "#b34d0f", "bad": "#9c1010",
      "notice-bg": "#fff1b8", "notice-fg": "#3a2a00",
      "fill-ok": "#0b5e1e", "fill-warn": "#6b4a00", "fill-bad": "#9c1010", "fill-info": "#4b1f9a", "fill-fg": "#ffffff",
    },
    dark: {
      "bg": "#000000", "fg": "#ffffff", "muted": "#c8c8c8", "card": "#0d0d0d", "card-head": "#1c1c1c", "line": "#8a8a8a",
      "accent": "#8ec5ff", "accent-fg": "#000000", "heat": "#ff8a6a", "ok": "#7ee08a", "warn": "#ffb870", "warn-mark": "#ffb870", "bad": "#ff5a50",
      "notice-bg": "#4a3c10", "notice-fg": "#fff0c2",
      "fill-ok": "#7ee08a", "fill-warn": "#ffd25e", "fill-bad": "#ff5a50", "fill-info": "#d3bcff", "fill-fg": "#000000",
    },
  },
  graphite: {
    light: {
      "bg": "#f5f5f3", "fg": "#1e1e1c", "muted": "#6a6a66", "card": "#ffffff", "card-head": "#ececea", "line": "#d9d9d4",
      "accent": "#4a4a46", "accent-fg": "#ffffff", "heat": "#b5452c", "ok": "#2d7a3a", "warn": "#b34d0f", "warn-mark": "#d05911", "bad": "#991b1b",
      "notice-bg": "#efeadb", "notice-fg": "#4a3f1e",
      "fill-ok": "#2d7a3a", "fill-warn": "#8a5f00", "fill-bad": "#991b1b", "fill-info": "#5a5f8f", "fill-fg": "#ffffff",
    },
    dark: {
      "bg": "#161616", "fg": "#eaeae8", "muted": "#9a9a96", "card": "#212121", "card-head": "#2b2b2a", "line": "#373735",
      "accent": "#c9c9c4", "accent-fg": "#161616", "heat": "#e0674a", "ok": "#6fc37c", "warn": "#ffa94d", "warn-mark": "#ffa94d", "bad": "#ff453a",
      "notice-bg": "#333028", "notice-fg": "#e8e0c8",
      "fill-ok": "#6fc37c", "fill-warn": "#e3b341", "fill-bad": "#ff453a", "fill-info": "#b8b4e0", "fill-fg": "#161616",
    },
  },
};

const CUSTOM_PREFIX = "custom:"; // a custom scheme's id is its name after this
const SCHEME_NAME_MAX = 60;

const themeState = {
  custom: [], // the user's schemes, {name, light, dark}, from massimo.schemes
  draft: null, // the scheme being edited while the editor is open: {name, light, dark}
  base: null, // the id of the scheme the editor started from
};

const darkQuery = window.matchMedia ? window.matchMedia("(prefers-color-scheme: dark)") : null;

// --- colours ----------------------------------------------------------

// hexColour returns a colour as #rrggbb in lower case (#rgb is widened),
// or null when the value is not one.
function hexColour(v) {
  if (typeof v !== "string") return null;
  const s = v.trim().toLowerCase();
  if (/^#[0-9a-f]{6}$/.test(s)) return s;
  if (/^#[0-9a-f]{3}$/.test(s)) return "#" + s[1] + s[1] + s[2] + s[2] + s[3] + s[3];
  return null;
}

// typedColour reads a colour typed or pasted into a hex field: #rrggbb
// or #rgb in any case, the # optional, spaces around ignored. It returns
// #rrggbb in lower case, or null when the value is not one.
function typedColour(v) {
  if (typeof v !== "string") return null;
  const s = v.trim();
  return hexColour(s.startsWith("#") ? s : "#" + s);
}

// luminance is the relative luminance of a #rrggbb colour (WCAG 2).
function luminance(hex) {
  const chan = (i) => {
    const c = parseInt(hex.slice(i, i + 2), 16) / 255;
    return c <= 0.03928 ? c / 12.92 : Math.pow((c + 0.055) / 1.055, 2.4);
  };
  return 0.2126 * chan(1) + 0.7152 * chan(3) + 0.0722 * chan(5);
}

// contrastRatio is the WCAG contrast ratio of two colours, 1 to 21.
function contrastRatio(a, b) {
  const la = luminance(a), lb = luminance(b);
  return (Math.max(la, lb) + 0.05) / (Math.min(la, lb) + 0.05);
}

// contrastClass grades a ratio: ok from 4.5 (WCAG AA for text), warn
// from 3 (large text only), bad below.
function contrastClass(ratio) {
  return ratio >= 4.5 ? "ok" : ratio >= 3 ? "warn" : "bad";
}

// --- schemes ----------------------------------------------------------

// tokenSet fills a set of tokens from an object: a token that is missing
// or not a colour takes the default's value, unknown keys are ignored.
function tokenSet(obj, mode) {
  const def = BUILTIN_SCHEMES[DEFAULT_SCHEME][mode];
  const set = {};
  for (const k of THEME_TOKENS) set[k] = hexColour(obj && obj[k]) || def[k];
  return set;
}

// schemeName trims a name to what the select and a file name take.
function schemeName(v, fallback) {
  const s = typeof v === "string" ? v.trim().slice(0, SCHEME_NAME_MAX).trim() : "";
  return s || fallback;
}

// schemeById resolves an id, "massimo" or "custom:<name>", to
// {id, name, builtin, light, dark}, or null.
function schemeById(id) {
  if (typeof id !== "string") return null;
  if (BUILTIN_SCHEMES[id]) {
    return { id, name: t("appearance.scheme." + id), builtin: true, light: BUILTIN_SCHEMES[id].light, dark: BUILTIN_SCHEMES[id].dark };
  }
  if (!id.startsWith(CUSTOM_PREFIX)) return null;
  const c = themeState.custom.find((s) => s.name === id.slice(CUSTOM_PREFIX.length));
  return c ? { id, name: c.name, builtin: false, light: c.light, dark: c.dark } : null;
}

function validMode(v) {
  return THEME_MODES.includes(v) ? v : "system";
}

function validScheme(id) {
  return schemeById(id) ? id : DEFAULT_SCHEME;
}

// storedSchemes is the user's schemes as the browser keeps them
// (massimo.schemes); a broken entry is dropped.
function storedSchemes() {
  let list;
  try {
    list = JSON.parse(readStored("massimo.schemes") || "[]");
  } catch (e) {
    return [];
  }
  if (!Array.isArray(list)) return [];
  const custom = [];
  for (const s of list) {
    const name = s && schemeName(s.name, "");
    if (!name || custom.some((c) => c.name === name)) continue;
    custom.push({ name, light: tokenSet(s.light, "light"), dark: tokenSet(s.dark, "dark") });
  }
  return custom;
}

// loadCustomSchemes reads the user's schemes (storedSchemes). The list is
// written whole, the parts' signal under it (ui/src/shared/data.ts).
function loadCustomSchemes() {
  themeState.custom = storedSchemes();
}

// storeCustomSchemes keeps the page's schemes (themeState.custom) in the
// browser and sends them to the server shortly, when they follow the
// user. The schemes the browser kept until then are the ones that hold
// where the page knows none (login.js noteHeld), and a push that meets a
// 401 goes back to them (takeBack).
function storeCustomSchemes() {
  noteHeld("schemes"); // login.js: the browser's schemes before the change
  store("massimo.schemes", themeState.custom.length ? JSON.stringify(themeState.custom) : null);
  schedulePreferences("schemes"); // login.js: the user's schemes follow the user
}

// forgetCustomSchemes drops the user's schemes from the browser, and
// sends nothing: the server keeps them for the user (Log out, app.js
// forgetUserSettings).
function forgetCustomSchemes() {
  themeState.custom = [];
  store("massimo.schemes", null);
}

// schemesDocument is the user's schemes as the server keeps them, the
// preference schemes: a list of massimo-scheme/1 documents, of the list
// given, else of the page's (themeState.custom), their colours copied.
function schemesDocument(list = themeState.custom) {
  return { format: "massimo-schemes/1", schemes: list.map((c) => ({ format: SCHEME_FORMAT, name: c.name, light: { ...c.light }, dark: { ...c.dark } })) };
}

// takeBackSchemes takes back the schemes a push sent, sent, that met a
// 401, to the schemes that hold, held, both as schemesDocument has them
// (login.js takeBack; AP SteadyTests v1.2, Block 19): each scheme the push
// sent otherwise than held, or sent where held has none, or left out
// where held has it, goes back to held's, or goes, where the page still
// has it as it was sent. A scheme changed again since keeps its change,
// which a push of its own sends. The list is written whole and kept in
// the browser, and nothing is sent; it returns whether a scheme went back.
function takeBackSchemes(sent, held) {
  const colours = (s) => (s ? THEME_TOKENS.map((k) => s.light[k] + "/" + s.dark[k]).join(",") : "");
  const of = (list, name) => list.find((s) => s.name === name);
  let list = themeState.custom;
  let back = false;
  const names = new Set([...sent.schemes, ...held.schemes].map((s) => s.name));
  for (const name of names) {
    const was = of(held.schemes, name);
    const went = colours(of(sent.schemes, name));
    if (went === colours(was) || colours(of(list, name)) !== went) continue;
    const entry = was && { name, light: { ...was.light }, dark: { ...was.dark } };
    const i = list.findIndex((c) => c.name === name);
    list = !entry ? list.filter((c) => c.name !== name) : i >= 0 ? list.map((c, j) => (j === i ? entry : c)) : [...list, entry];
    back = true;
  }
  if (!back) return false;
  themeState.custom = list;
  store("massimo.schemes", list.length ? JSON.stringify(list) : null);
  return true;
}

// takeSchemes takes the user's schemes from the server as the list: a
// scheme the browser has and the server lacks was deleted elsewhere, and
// is dropped, not sent back. The browser's own reach the server only
// while the server has no list yet (login.js, loadPreferences).
function takeSchemes(doc) {
  const list = doc && Array.isArray(doc.schemes) ? doc.schemes : [];
  store("massimo.schemes", JSON.stringify(list.map((s) => ({ name: s && s.name, light: s && s.light, dark: s && s.dark }))));
  loadCustomSchemes();
  store("massimo.schemes", themeState.custom.length ? JSON.stringify(themeState.custom) : null);
}

// putCustomScheme stores a scheme under its name, replacing one with
// that name, and returns its id.
function putCustomScheme(name, light, dark) {
  const entry = { name, light: Object.assign({}, light), dark: Object.assign({}, dark) };
  const i = themeState.custom.findIndex((c) => c.name === name);
  themeState.custom = i >= 0 ? themeState.custom.map((c, j) => (j === i ? entry : c)) : [...themeState.custom, entry];
  storeCustomSchemes();
  return CUSTOM_PREFIX + name;
}

// schemeDoc is the document of a scheme as the file and, later, the
// server carry it.
function schemeDoc(s) {
  return { format: SCHEME_FORMAT, name: s.name, light: tokenSet(s.light, "light"), dark: tokenSet(s.dark, "dark") };
}

// parseSchemeDoc checks an imported document and returns {name, light,
// dark}, or throws with the key of the reason.
function parseSchemeDoc(doc) {
  if (!doc || typeof doc !== "object" || doc.format !== SCHEME_FORMAT) throw new Error("appearance.errFormat");
  const sets = ["light", "dark"].filter((m) => doc[m] && typeof doc[m] === "object");
  if (!sets.length) throw new Error("appearance.errNoColours");
  return { name: schemeName(doc.name, t("appearance.imported")), light: tokenSet(doc.light, "light"), dark: tokenSet(doc.dark, "dark") };
}

// --- applying ---------------------------------------------------------

// effectiveMode is the set in use: the mode, or the system's choice.
function effectiveMode(mode) {
  if (mode === "light" || mode === "dark") return mode;
  return darkQuery && darkQuery.matches ? "dark" : "light";
}

// applyTheme puts the mode on <html> (data-theme, which style.css
// honours; none for the system's choice) and the scheme's colours of
// the set in use on its style, or the draft's while the editor is open.
// The default scheme without a draft removes them, so style.css and its
// media query stay the truth.
function applyTheme(mode, id) {
  mode = validMode(mode);
  const root = document.documentElement;
  if (mode === "system") root.removeAttribute("data-theme");
  else root.dataset.theme = mode;
  root.style.colorScheme = mode === "system" ? "" : mode; // the browser's own controls follow
  const s = themeState.draft || schemeById(validScheme(id));
  const set = s[effectiveMode(mode)];
  const plain = !themeState.draft && s.id === DEFAULT_SCHEME;
  for (const k of THEME_TOKENS) {
    if (plain) root.style.removeProperty("--" + k);
    else root.style.setProperty("--" + k, set[k]);
  }
}

function reapplyTheme() {
  applyTheme(state.settings.theme, state.settings.scheme);
}

// BLINK_MODES are the choices of the setting "Blinking signs": as the
// system (blinking unless it asks for reduced motion), always, for the
// first five seconds a sign is shown, never.
const BLINK_MODES = ["system", "on", "first", "off"];

// REDUCED_MOTION is the system's wish for less motion, which "system"
// follows as it changes.
const REDUCED_MOTION = matchMedia("(prefers-reduced-motion: reduce)");

// applyBlink sets the page's class for the marks' blinking (style.css):
// blink-on, blink-first or none.
function applyBlink() {
  let m = state.settings.blink;
  if (m === "system") m = REDUCED_MOTION.matches ? "off" : "on";
  const root = document.documentElement;
  root.classList.toggle("blink-on", m === "on");
  root.classList.toggle("blink-first", m === "first");
}

// chooseBlink is the setting's choice, kept as the other settings are.
function chooseBlink(mode) {
  setSetting("blink", BLINK_MODES.includes(mode) ? mode : "system"); // app.js
  applyBlink();
}

// iconsDrawn is the drawings the page was last drawn with, so that a
// change of them draws again what shows them, from the settings page or
// from the user's settings arriving or changing elsewhere.
let iconsDrawn = null;

// applyIcons draws again what shows the window and the radio where the
// choice changed since the page was drawn: the rooms, the Cubes and the
// header's problems. The Appearance card draws its choices and their
// previews itself (ui/src/prefs/).
function applyIcons() {
  const now = state.settings.iconWindow + "/" + state.settings.iconRadio;
  if (iconsDrawn !== null && iconsDrawn !== now) redrawSigns();
  iconsDrawn = now;
}

// redrawSigns draws again what shows the window and the radio: the
// header's row folded anew, the parts through the icons' signal, which their
// signs read (ui/src/shared/look.ts).
function redrawSigns() {
  MassimoParts.icons();
  MassimoParts.header.render(); // the header's row folded anew for the drawings' widths
}

// chooseIcon is a drawing's choice, kept as the other settings are.
function chooseIcon(key, value) {
  setSetting(key, validSetting(key, value) || (key === "iconWindow" ? "sash" : "wifi")); // app.js
  applyIcons();
}

// chooseMode and chooseScheme are the settings' choices, kept as the
// other settings are (app.js setSetting): for this device, or as the
// user's, in the browser and for a signed-in user on the server.
function chooseMode(mode) {
  setSetting("theme", validMode(mode)); // app.js: this device's, or the user's
  applyAppearance();
}

function chooseScheme(id) {
  setSetting("scheme", validScheme(id));
  applyAppearance();
}

// applyAppearance applies the appearance settings of state.settings;
// applySettings and a change of language call it, and the choices of the
// mode and the scheme. My settings' Appearance card is a part
// (ui/src/prefs/), which follows the settings as signals.
function applyAppearance() {
  reapplyTheme();
  applyBlink();
  applyIcons();
}

// initAppearance follows the system's changes: its wish for less motion,
// which blinking "as the system" follows, and its light or dark, which
// the mode "as the system" follows (app.js init).
function initAppearance() {
  REDUCED_MOTION.addEventListener("change", applyBlink);
  if (darkQuery) darkQuery.addEventListener("change", reapplyTheme);
}

// The stored choice, applied before the page is drawn; init() in app.js
// applies it again from the settings.
loadCustomSchemes();
applyTheme(readSetting("theme"), readSetting("scheme"));
