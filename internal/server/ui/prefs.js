// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

// The Massimo web UI, My settings: their plain names, which call their
// part (ui/src/prefs/, MassimoParts.prefs), the sign-in's choice of
// language, and the copy of the version in ☰.
"use strict";

// renderDeviceSwitches shows which settings are kept for this device
// only, each switch named for its row: My settings' rows and the
// Appearance card are a part (ui/src/prefs/, MassimoParts.prefs).
function renderDeviceSwitches() {
  MassimoParts.prefs.render();
}

// --- settings panel ---------------------------------------------------

function initSettings() {
  // My settings opens from the 👤 menu (showView); its rows and the
  // Appearance card are a part (ui/src/prefs/), which saves each as it
  // is changed.
  initAppearance();
}

// renderLangSelect fills the sign-in's choice of language: from the
// browser, then every language with a translation table, by its own
// name; My settings' choice is a part, drawn again with it.
function renderLangSelect() {
  const options = [["auto", t("lang.auto") + " (" + LANGUAGE_NAMES[browserLanguage()] + ")"]];
  for (const l of languages()) options.push([l, LANGUAGE_NAMES[l] || l]);
  fillOptions($("login-lang"), options).value = i18n.setting; // the sign-in's menu
  MassimoParts.prefs.render();
}
