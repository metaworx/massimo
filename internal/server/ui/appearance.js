// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

// The Massimo web UI, the Appearance card of My settings: the choices
// of mode and scheme, the scheme editor with its contrast hints, and a
// scheme's export and import as a file.
"use strict";

// --- settings page ----------------------------------------------------

// renderSchemeSelect fills the scheme choice: the built-in schemes by
// their translated names, then the user's own under a group.
function renderSchemeSelect() {
  const options = Object.keys(BUILTIN_SCHEMES).map((id) => [id, t("appearance.scheme." + id)]);
  if (themeState.custom.length) {
    options.push({ group: t("appearance.customGroup"), options: themeState.custom.map((c) => [CUSTOM_PREFIX + c.name, c.name]) });
  }
  const sel = fillOptions($("scheme-select"), options);
  sel.value = state.settings.scheme;
}

// applyAppearance draws the appearance settings from state.settings and
// applies them; applySettings and a change of language call it.
function applyAppearance() {
  $("theme-select").value = state.settings.theme;
  $("login-mode").value = state.settings.theme; // the sign-in's menu
  renderSchemeSelect();
  if (themeState.draft) renderSchemeEditor();
  reapplyTheme();
  applyBlink();
  applyIcons();
}

// schemeMessage shows the result of a save, delete or import under the
// buttons; empty clears it.
function schemeMessage(text, ok) {
  showResult($("scheme-msg"), text, ok);
}

function initAppearance() {
  $("theme-select").addEventListener("change", (ev) => chooseMode(ev.target.value));
  $("blink-select").addEventListener("change", (ev) => chooseBlink(ev.target.value));
  for (const [key, [sel]] of Object.entries(ICON_SELECTS)) $(sel).addEventListener("change", (ev) => chooseIcon(key, ev.target.value));
  REDUCED_MOTION.addEventListener("change", applyBlink);
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
  const picker = filePicker("application/json,.json", importSchemeFile);
  picker.id = "scheme-file";
  picker.setAttribute("aria-label", t("appearance.importFile"));
  picker.dataset.i18nAttr = "aria-label:appearance.importFile"; // for a change of language
  $("scheme-import").after(picker);
  $("scheme-import").addEventListener("click", () => picker.click());
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

// fillContrast writes the ratio of a token's pair into a badge: the
// number, its grade as class, the pair as title and label. The token is
// a background of CONTRAST_PAIRS, graded for text on it, or an icon
// colour of MARK_PAIRS, graded at 3:1 on its background.
function fillContrast(badge, set, token) {
  const mark = MARK_PAIRS[token];
  const [fg, bg] = mark ? [token, mark] : [CONTRAST_PAIRS[token], token];
  const ratio = contrastRatio(set[fg], set[bg]);
  badge.textContent = fmtNum(ratio, 1);
  badge.className = "contrast " + (mark ? (ratio >= 3 ? "ok" : "bad") : contrastClass(ratio));
  const label = t("appearance.contrast", { text: tokenLabel(fg), bg: tokenLabel(bg), ratio: fmtNum(ratio, 1) });
  badge.title = label;
  badge.setAttribute("aria-label", label);
}

// hexMessage shows under the sets why a hex field was refused: the
// field just refused, or else one still marked; none clears it.
function hexMessage(field) {
  const p = $("scheme-hex-msg");
  if (!p) return;
  const bad = field || $("scheme-editor").querySelector(".scheme-hex[aria-invalid=true]");
  showResult(p, bad ? t("appearance.hexBad", { token: bad.dataset.token, value: bad.dataset.refused }) : "", false);
}

// markHex marks a hex field as refused (the text it holds) or as good.
function markHex(field, refused) {
  if (refused === null) {
    field.removeAttribute("aria-invalid");
    field.removeAttribute("aria-describedby");
    delete field.dataset.refused;
  } else {
    field.setAttribute("aria-invalid", "true");
    field.setAttribute("aria-describedby", "scheme-hex-msg");
    field.dataset.refused = refused;
  }
  hexMessage(refused === null ? null : field);
}

// renderSchemeSet fills the fieldset of one set (light or dark) of the
// draft: a row per token (tpl-scheme-row) with its name, the contrast badge for a
// background of CONTRAST_PAIRS or an icon colour of MARK_PAIRS (an empty placeholder of the same width
// on the other rows, so the colours and the hex fields line up), the
// colour input and the hex field. The hex field takes a typed or pasted
// value: a colour applies as it is typed and is written as #rrggbb once
// the field is left; anything else is refused, the field marked
// invalid, and the colour stays.
function renderSchemeSet(fs, mode) {
  const d = themeState.draft;
  const badges = {};
  const refresh = () => {
    for (const [k, b] of Object.entries(badges)) fillContrast(b, d[mode], k);
  };
  for (const k of THEME_TOKENS) {
    const row = fromTemplate("tpl-scheme-row");
    const input = row.querySelector("input[type=color]");
    input.id = "scheme-" + mode + "-" + k;
    input.value = d[mode][k];
    input.title = "--" + k;
    const label = row.querySelector("label");
    label.textContent = tokenLabel(k);
    label.htmlFor = input.id;
    const hex = row.querySelector(".scheme-hex");
    hex.id = "scheme-" + mode + "-" + k + "-hex";
    hex.value = d[mode][k];
    hex.setAttribute("aria-label", t("appearance.hex", { token: tokenLabel(k) }));
    hex.dataset.token = tokenLabel(k);
    const take = (c) => {
      d[mode][k] = c;
      refresh();
      reapplyTheme();
    };
    input.addEventListener("input", () => {
      const c = hexColour(input.value);
      if (!c) return;
      hex.value = c;
      markHex(hex, null);
      take(c);
    });
    hex.addEventListener("input", () => {
      const c = typedColour(hex.value);
      if (!c) return; // perhaps not typed to the end yet; judged on change
      input.value = c;
      if (hex.hasAttribute("aria-invalid")) markHex(hex, null);
      take(c);
    });
    hex.addEventListener("change", () => {
      const c = typedColour(hex.value);
      if (c) {
        hex.value = c;
        markHex(hex, null);
      } else {
        markHex(hex, hex.value.trim());
      }
    });
    hex.addEventListener("keydown", (ev) => {
      if (ev.key === "Escape" && hex.hasAttribute("aria-invalid")) {
        ev.preventDefault();
        hex.value = d[mode][k];
        markHex(hex, null);
      }
    });
    const badge = label.nextElementSibling;
    if (CONTRAST_PAIRS[k] || MARK_PAIRS[k]) {
      badge.setAttribute("role", "img"); // so the label is read out
      badges[k] = badge;
    } else {
      badge.className = "contrast none";
      badge.setAttribute("aria-hidden", "true");
    }
    fs.appendChild(row);
  }
  refresh();
}

// renderSchemeEditor draws the editor for the draft from its template
// (tpl-scheme-editor): the two sets, the contrast note, the name with
// Save as, then Reset, Delete and Close.
function renderSchemeEditor() {
  const d = themeState.draft;
  const base = schemeById(themeState.base);
  const ed = $("scheme-editor");
  clear(ed);
  const frame = fragmentFromTemplate("tpl-scheme-editor");
  for (const fs of frame.querySelectorAll(".scheme-set")) renderSchemeSet(fs, fs.dataset.mode);
  const name = frame.querySelector("#scheme-name");
  name.maxLength = SCHEME_NAME_MAX;
  name.value = d.name;
  name.addEventListener("keydown", (ev) => {
    if (ev.key === "Enter") {
      ev.preventDefault();
      saveSchemeDraft();
    }
  });
  name.nextElementSibling.addEventListener("click", saveSchemeDraft);
  frame.querySelector(".scheme-reset").addEventListener("click", resetSchemeDraft);
  const del = frame.querySelector("#scheme-delete");
  del.disabled = !base || base.builtin;
  del.addEventListener("click", deleteSchemeDraft);
  frame.querySelector(".scheme-close").addEventListener("click", closeSchemeEditor);
  ed.appendChild(frame);
}

// --- export and import ------------------------------------------------

// fileSlug makes a scheme's name safe for a file name.
function fileSlug(name) {
  return name.toLowerCase().replace(/[^a-z0-9]+/g, "-").replace(/^-+|-+$/g, "") || "scheme";
}

// exportScheme downloads the draft, or else the chosen scheme, as a
// file (downloadBlob).
function exportScheme() {
  const s = themeState.draft || schemeById(state.settings.scheme) || schemeById(DEFAULT_SCHEME);
  const doc = schemeDoc(s);
  downloadBlob("massimo-scheme-" + fileSlug(doc.name) + ".json", new Blob([JSON.stringify(doc, null, 1) + "\n"], { type: "application/json" }));
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
