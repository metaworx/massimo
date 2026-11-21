// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

// The Massimo web UI, My settings: their plain names, which call their
// part (ui/src/prefs/, MassimoParts.prefs), the Appearance card's
// switches that keep a setting on this device alone, the sign-in's
// choice of language, and the copy of the version in ☰.
"use strict";

// renderDeviceSwitches shows which settings are kept for this device
// only, each switch named for its row: My settings' rows are a part
// (ui/src/prefs/, MassimoParts.prefs), the Appearance card's are plain.
function renderDeviceSwitches() {
  MassimoParts.prefs.render();
  for (const input of document.querySelectorAll("#prefs-sec .appearance input[data-device-only]")) {
    const key = input.dataset.deviceOnly;
    input.checked = deviceOnly(key);
    const label = t("settings.thisDeviceFor", { setting: t(SETTING_LABELS[key]) });
    input.setAttribute("aria-label", label);
    input.closest("label").title = label; // the column's switch has no text of its own
  }
}

// --- settings panel ---------------------------------------------------

function initSettings() {
  // My settings opens from the 👤 menu (showView); its rows are a part
  // (ui/src/prefs/), which saves each as it is changed.
  // the version copies whole, as its tooltip names it (D94-A)
  copyable($("version-short"), () => state.version, ""); // components.js
  for (const input of document.querySelectorAll("#prefs-sec .appearance input[data-device-only]")) {
    input.addEventListener("change", () => setDeviceOnly(input.dataset.deviceOnly, input.checked));
  }
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
