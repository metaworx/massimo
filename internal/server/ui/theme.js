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
// Settings page has the two choices, the editor with a contrast hint
// (WCAG ratio) for the pairs that must stay readable, Export and Import.
// Loaded after app.js (readStored, store, state, the DOM helpers); the
// stored choice is applied as soon as this file runs, before the page
// is drawn, so a scheme does not flash the default first.
"use strict";

const SCHEME_FORMAT = "massimo-scheme/1";
const THEME_MODES = ["system", "light", "dark"];
const DEFAULT_SCHEME = "massimo";

// The tokens, in the order of style.css and of the editor.
const THEME_TOKENS = [
  "bg", "fg", "muted", "card", "card-head", "line", "accent", "accent-fg", "heat",
  "ok", "warn", "bad", "notice-bg", "notice-fg", "fill-ok", "fill-warn", "fill-bad", "fill-info", "fill-fg",
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

// The built-in schemes, by id; their names are appearance.scheme.<id>
// in the translation table. "massimo" is the default and repeats the
// token blocks of style.css, which stay the source for a page without
// a stored scheme (TestUIDefaultSchemeMatchesCSS checks that they
// agree). "contrast" is black on white with stronger lines and darker
// colours, "graphite" keeps the default's tones with a grey accent in
// place of the blue. Every pair of CONTRAST_PAIRS, the muted text on
// the page and the tile, and the ok, warn and bad texts on a tile are
// at 4.5 or better in both sets of all three.
const BUILTIN_SCHEMES = {
  massimo: {
    light: {
      "bg": "#f6f6f4", "fg": "#1c1c1a", "muted": "#6b6b66", "card": "#ffffff", "card-head": "#eeeeea", "line": "#dcdcd6",
      "accent": "#1f63c6", "accent-fg": "#ffffff", "heat": "#b5452c", "ok": "#2d7a3a", "warn": "#9a6b00", "bad": "#b3261e",
      "notice-bg": "#fff4d6", "notice-fg": "#5a3f00",
      "fill-ok": "#2d7a3a", "fill-warn": "#8a5f00", "fill-bad": "#b3261e", "fill-info": "#6b3fb3", "fill-fg": "#ffffff",
    },
    dark: {
      "bg": "#151514", "fg": "#ececea", "muted": "#9b9b95", "card": "#202020", "card-head": "#2a2a29", "line": "#353533",
      "accent": "#6ea8fe", "accent-fg": "#151514", "heat": "#e0674a", "ok": "#6fc37c", "warn": "#e3b341", "bad": "#f28b82",
      "notice-bg": "#3a3012", "notice-fg": "#f1dfae",
      "fill-ok": "#6fc37c", "fill-warn": "#e3b341", "fill-bad": "#f28b82", "fill-info": "#c4a7f5", "fill-fg": "#151514",
    },
  },
  contrast: {
    light: {
      "bg": "#ffffff", "fg": "#000000", "muted": "#3d3d3d", "card": "#ffffff", "card-head": "#efefef", "line": "#6b6b6b",
      "accent": "#0b3d91", "accent-fg": "#ffffff", "heat": "#9b2f12", "ok": "#0b5e1e", "warn": "#6b4a00", "bad": "#9c1010",
      "notice-bg": "#fff1b8", "notice-fg": "#3a2a00",
      "fill-ok": "#0b5e1e", "fill-warn": "#6b4a00", "fill-bad": "#9c1010", "fill-info": "#4b1f9a", "fill-fg": "#ffffff",
    },
    dark: {
      "bg": "#000000", "fg": "#ffffff", "muted": "#c8c8c8", "card": "#0d0d0d", "card-head": "#1c1c1c", "line": "#8a8a8a",
      "accent": "#8ec5ff", "accent-fg": "#000000", "heat": "#ff8a6a", "ok": "#7ee08a", "warn": "#ffd25e", "bad": "#ff9b94",
      "notice-bg": "#4a3c10", "notice-fg": "#fff0c2",
      "fill-ok": "#7ee08a", "fill-warn": "#ffd25e", "fill-bad": "#ff9b94", "fill-info": "#d3bcff", "fill-fg": "#000000",
    },
  },
  graphite: {
    light: {
      "bg": "#f5f5f3", "fg": "#1e1e1c", "muted": "#6a6a66", "card": "#ffffff", "card-head": "#ececea", "line": "#d9d9d4",
      "accent": "#4a4a46", "accent-fg": "#ffffff", "heat": "#b5452c", "ok": "#2d7a3a", "warn": "#9a6b00", "bad": "#b3261e",
      "notice-bg": "#efeadb", "notice-fg": "#4a3f1e",
      "fill-ok": "#2d7a3a", "fill-warn": "#8a5f00", "fill-bad": "#b3261e", "fill-info": "#5a5f8f", "fill-fg": "#ffffff",
    },
    dark: {
      "bg": "#161616", "fg": "#eaeae8", "muted": "#9a9a96", "card": "#212121", "card-head": "#2b2b2a", "line": "#373735",
      "accent": "#c9c9c4", "accent-fg": "#161616", "heat": "#e0674a", "ok": "#6fc37c", "warn": "#e3b341", "bad": "#f28b82",
      "notice-bg": "#333028", "notice-fg": "#e8e0c8",
      "fill-ok": "#6fc37c", "fill-warn": "#e3b341", "fill-bad": "#f28b82", "fill-info": "#b8b4e0", "fill-fg": "#161616",
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

// loadCustomSchemes reads the user's schemes; a broken entry is dropped.
function loadCustomSchemes() {
  themeState.custom = [];
  let list;
  try {
    list = JSON.parse(readStored("massimo.schemes") || "[]");
  } catch (e) {
    return;
  }
  if (!Array.isArray(list)) return;
  for (const s of list) {
    const name = s && schemeName(s.name, "");
    if (!name || themeState.custom.some((c) => c.name === name)) continue;
    themeState.custom.push({ name, light: tokenSet(s.light, "light"), dark: tokenSet(s.dark, "dark") });
  }
}

function storeCustomSchemes() {
  store("massimo.schemes", themeState.custom.length ? JSON.stringify(themeState.custom) : null);
}

// putCustomScheme stores a scheme under its name, replacing one with
// that name, and returns its id.
function putCustomScheme(name, light, dark) {
  const entry = { name, light: Object.assign({}, light), dark: Object.assign({}, dark) };
  const i = themeState.custom.findIndex((c) => c.name === name);
  if (i >= 0) themeState.custom[i] = entry;
  else themeState.custom.push(entry);
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

// chooseMode and chooseScheme are the settings' choices, kept in the
// browser; the default is kept as no entry.
function chooseMode(mode) {
  state.settings.theme = validMode(mode);
  store("massimo.theme", state.settings.theme === "system" ? null : state.settings.theme);
  applyAppearance();
}

function chooseScheme(id) {
  state.settings.scheme = validScheme(id);
  store("massimo.scheme", state.settings.scheme === DEFAULT_SCHEME ? null : state.settings.scheme);
  applyAppearance();
}

// --- settings page ----------------------------------------------------

// renderSchemeSelect fills the scheme choice: the built-in schemes by
// their translated names, then the user's own under a group.
function renderSchemeSelect() {
  const sel = $("scheme-select");
  clear(sel);
  for (const id of Object.keys(BUILTIN_SCHEMES)) {
    const o = document.createElement("option");
    o.value = id;
    o.textContent = t("appearance.scheme." + id);
    sel.appendChild(o);
  }
  if (themeState.custom.length) {
    const group = document.createElement("optgroup");
    group.label = t("appearance.customGroup");
    for (const c of themeState.custom) {
      const o = document.createElement("option");
      o.value = CUSTOM_PREFIX + c.name;
      o.textContent = c.name;
      group.appendChild(o);
    }
    sel.appendChild(group);
  }
  sel.value = state.settings.scheme;
}

// applyAppearance draws the appearance settings from state.settings and
// applies them; applySettings and a change of language call it.
function applyAppearance() {
  $("theme-select").value = state.settings.theme;
  renderSchemeSelect();
  if (themeState.draft) renderSchemeEditor();
  reapplyTheme();
}

// schemeMessage shows the result of a save, delete or import under the
// buttons; empty clears it.
function schemeMessage(text, ok) {
  const p = $("scheme-msg");
  p.textContent = text;
  p.className = "small scheme-msg" + (text ? (ok ? " ok" : " bad") : " hidden");
}

function initAppearance() {
  $("theme-select").addEventListener("change", (ev) => chooseMode(ev.target.value));
  $("scheme-select").addEventListener("change", (ev) => {
    closeSchemeEditor();
    schemeMessage("");
    chooseScheme(ev.target.value);
  });
  $("scheme-customise").addEventListener("click", () => {
    if (themeState.draft) closeSchemeEditor();
    else openSchemeEditor();
  });
  $("scheme-export").addEventListener("click", exportScheme);
  $("scheme-import").addEventListener("click", () => $("scheme-file").click());
  $("scheme-file").addEventListener("change", (ev) => {
    const file = ev.target.files && ev.target.files[0];
    ev.target.value = "";
    if (file) importSchemeFile(file);
  });
  if (darkQuery) darkQuery.addEventListener("change", reapplyTheme);
}

// --- the editor -------------------------------------------------------

// openSchemeEditor starts a draft from the chosen scheme and shows it;
// every change applies to the page at once.
function openSchemeEditor() {
  const s = schemeById(state.settings.scheme) || schemeById(DEFAULT_SCHEME);
  themeState.base = s.id;
  themeState.draft = { name: s.name, light: Object.assign({}, s.light), dark: Object.assign({}, s.dark) };
  schemeMessage("");
  renderSchemeEditor();
  $("scheme-editor").classList.remove("hidden");
  $("scheme-customise").setAttribute("aria-expanded", "true");
  reapplyTheme();
  const first = $("scheme-editor").querySelector("input");
  if (first) first.focus();
}

// closeSchemeEditor drops the draft; the chosen scheme is applied again.
function closeSchemeEditor() {
  if (!themeState.draft) return;
  themeState.draft = null;
  themeState.base = null;
  const ed = $("scheme-editor");
  clear(ed);
  ed.classList.add("hidden");
  $("scheme-customise").setAttribute("aria-expanded", "false");
  reapplyTheme();
}

// resetSchemeDraft puts the draft back to the scheme the editor started
// from, as it is stored.
function resetSchemeDraft() {
  const s = schemeById(themeState.base) || schemeById(DEFAULT_SCHEME);
  themeState.draft.light = Object.assign({}, s.light);
  themeState.draft.dark = Object.assign({}, s.dark);
  renderSchemeEditor();
  reapplyTheme();
}

// saveSchemeDraft keeps the draft in the browser under the name given,
// replacing a scheme of that name, and chooses it.
function saveSchemeDraft() {
  const name = schemeName($("scheme-name").value, t("appearance.unnamed"));
  const d = themeState.draft;
  d.name = name;
  themeState.base = putCustomScheme(name, d.light, d.dark);
  chooseScheme(themeState.base);
  schemeMessage(t("appearance.saved", { name }), true);
}

// deleteSchemeDraft removes the custom scheme the editor started from
// and goes back to the default.
function deleteSchemeDraft() {
  const s = schemeById(themeState.base);
  if (!s || s.builtin) return;
  themeState.custom = themeState.custom.filter((c) => c.name !== s.name);
  storeCustomSchemes();
  closeSchemeEditor();
  chooseScheme(DEFAULT_SCHEME);
  schemeMessage(t("appearance.deleted", { name: s.name }), true);
}

// tokenLabel is a token's name for people.
function tokenLabel(k) {
  return t("appearance.token." + k);
}

// fillContrast writes the ratio of the pair into a badge: the number,
// its grade as class, the pair as title and label.
function fillContrast(badge, set, bgToken) {
  const textToken = CONTRAST_PAIRS[bgToken];
  const ratio = contrastRatio(set[textToken], set[bgToken]);
  badge.textContent = fmtNum(ratio, 1);
  badge.className = "contrast " + contrastClass(ratio);
  const label = t("appearance.contrast", { text: tokenLabel(textToken), bg: tokenLabel(bgToken), ratio: fmtNum(ratio, 1) });
  badge.title = label;
  badge.setAttribute("aria-label", label);
}

// renderSchemeSet builds the inputs of one set (light or dark) of the
// draft: a row per token with its colour, the value as text and, for
// a background of CONTRAST_PAIRS, the contrast badge.
function renderSchemeSet(mode) {
  const d = themeState.draft;
  const fs = document.createElement("fieldset");
  fs.className = "scheme-set";
  fs.appendChild(textEl("legend", t("appearance." + mode)));
  const badges = {};
  const refresh = () => {
    for (const [k, b] of Object.entries(badges)) fillContrast(b, d[mode], k);
  };
  for (const k of THEME_TOKENS) {
    const row = document.createElement("div");
    row.className = "scheme-row";
    const input = document.createElement("input");
    input.type = "color";
    input.id = "scheme-" + mode + "-" + k;
    input.value = d[mode][k];
    input.title = "--" + k;
    const label = textEl("label", tokenLabel(k));
    label.htmlFor = input.id;
    const hex = textEl("span", d[mode][k], "scheme-hex mono");
    input.addEventListener("input", () => {
      d[mode][k] = hexColour(input.value) || d[mode][k];
      hex.textContent = d[mode][k];
      refresh();
      reapplyTheme();
    });
    row.appendChild(label);
    row.appendChild(input);
    row.appendChild(hex);
    if (CONTRAST_PAIRS[k]) {
      const b = document.createElement("span");
      b.setAttribute("role", "img"); // so the label is read out
      badges[k] = b;
      row.appendChild(b);
    }
    fs.appendChild(row);
  }
  refresh();
  return fs;
}

// renderSchemeEditor draws the editor for the draft: the two sets, the
// contrast note, the name with Save as, then Reset, Delete and Close.
function renderSchemeEditor() {
  const d = themeState.draft;
  const base = schemeById(themeState.base);
  const ed = $("scheme-editor");
  clear(ed);
  const title = textEl("h3", t("appearance.editorTitle"));
  title.id = "scheme-editor-title";
  ed.appendChild(title);
  const sets = document.createElement("div");
  sets.className = "scheme-sets";
  sets.appendChild(renderSchemeSet("light"));
  sets.appendChild(renderSchemeSet("dark"));
  ed.appendChild(sets);
  ed.appendChild(textEl("p", t("appearance.contrastNote"), "muted small"));
  const nameRow = document.createElement("div");
  nameRow.className = "scheme-name";
  const nameLabel = textEl("label", t("appearance.name"), "muted");
  nameLabel.htmlFor = "scheme-name";
  const name = document.createElement("input");
  name.type = "text";
  name.id = "scheme-name";
  name.maxLength = SCHEME_NAME_MAX;
  name.value = d.name;
  name.addEventListener("keydown", (ev) => {
    if (ev.key === "Enter") {
      ev.preventDefault();
      saveSchemeDraft();
    }
  });
  nameRow.appendChild(nameLabel);
  nameRow.appendChild(name);
  nameRow.appendChild(button(t("appearance.saveAs"), "primary", saveSchemeDraft));
  ed.appendChild(nameRow);
  const actions = document.createElement("div");
  actions.className = "actions";
  const left = document.createElement("span");
  left.className = "actions-left";
  left.appendChild(button(t("appearance.reset"), "", resetSchemeDraft));
  const del = button(t("action.delete"), "", deleteSchemeDraft);
  del.id = "scheme-delete";
  del.disabled = !base || base.builtin;
  left.appendChild(del);
  const right = document.createElement("span");
  right.className = "actions-right";
  right.appendChild(button(t("action.close"), "", closeSchemeEditor));
  actions.appendChild(left);
  actions.appendChild(right);
  ed.appendChild(actions);
}

// --- export and import ------------------------------------------------

// fileSlug makes a scheme's name safe for a file name.
function fileSlug(name) {
  return name.toLowerCase().replace(/[^a-z0-9]+/g, "-").replace(/^-+|-+$/g, "") || "scheme";
}

// exportScheme downloads the draft, or else the chosen scheme, as a
// file: a blob: link with the download attribute, which the page's
// Content-Security-Policy allows.
function exportScheme() {
  const s = themeState.draft || schemeById(state.settings.scheme) || schemeById(DEFAULT_SCHEME);
  const doc = schemeDoc(s);
  const blob = new Blob([JSON.stringify(doc, null, 1) + "\n"], { type: "application/json" });
  const url = URL.createObjectURL(blob);
  const a = document.createElement("a");
  a.href = url;
  a.download = "massimo-scheme-" + fileSlug(doc.name) + ".json";
  document.body.appendChild(a);
  a.click();
  a.remove();
  setTimeout(() => URL.revokeObjectURL(url), 1000);
}

// importSchemeFile reads a scheme file, keeps it under its name and
// chooses it; a file that is not a scheme shows why.
async function importSchemeFile(file) {
  let s;
  try {
    let doc;
    try {
      doc = JSON.parse(await file.text());
    } catch (e) {
      throw new Error("appearance.errNotJson");
    }
    s = parseSchemeDoc(doc);
  } catch (e) {
    schemeMessage(t("appearance.importBad", { error: t(e.message) }), false);
    return;
  }
  closeSchemeEditor();
  chooseScheme(putCustomScheme(s.name, s.light, s.dark));
  schemeMessage(t("appearance.importDone", { name: s.name }), true);
}

// The stored choice, applied before the page is drawn; init() in app.js
// applies it again from the settings.
loadCustomSchemes();
applyTheme(readStored("massimo.theme"), readStored("massimo.scheme"));
