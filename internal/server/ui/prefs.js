// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

// The Massimo web UI, My settings: the switches that keep a setting on
// this device alone, the wiring of the settings' rows, and the choice
// of language.
"use strict";

// renderDeviceSwitches shows which settings are kept for this device
// only, each switch named for its row.
function renderDeviceSwitches() {
  for (const input of document.querySelectorAll("input[data-device-only]")) {
    const key = input.dataset.deviceOnly;
    input.checked = deviceOnly(key);
    const label = t("settings.thisDeviceFor", { setting: t(SETTING_LABELS[key]) });
    input.setAttribute("aria-label", label);
    input.closest("label").title = label; // the column's switch has no text of its own
  }
}

// --- settings panel ---------------------------------------------------

function initSettings() {
  // My settings opens from the 👤 menu (showView).
  $("poll-input").addEventListener("change", (ev) => {
    const v = validSetting("poll", ev.target.value) || 10;
    ev.target.value = v;
    setSetting("poll", v);
    startPolling();
  });
  const onExpert = (ev) => {
    setSetting("expert", ev.target.checked);
    applySettings();
  };
  $("expert-toggle").addEventListener("change", onExpert);
  // the version copies whole, as its tooltip names it (D94-A)
  copyable($("version-short"), () => state.version, ""); // components.js
  $("refresh-btn").addEventListener("click", (ev) => whileBusy(ev.currentTarget, () => poll(true)));
  $("clock-select").addEventListener("change", (ev) => {
    setSetting("clock", validSetting("clock", ev.target.value) || "browser");
    applySettings();
  });
  $("lang-select").addEventListener("change", (ev) => {
    setSetting("lang", validSetting("lang", ev.target.value) || "auto");
    changeLanguage(state.settings.lang).then(applyLanguage);
  });
  for (const input of document.querySelectorAll("input[data-device-only]")) {
    input.addEventListener("change", () => setDeviceOnly(input.dataset.deviceOnly, input.checked));
  }
  initAppearance();
}

// renderLangSelect fills the language choice: from the browser, then
// every language with a translation table, by its own name.
function renderLangSelect() {
  const options = [["auto", t("lang.auto") + " (" + LANGUAGE_NAMES[browserLanguage()] + ")"]];
  for (const l of languages()) options.push([l, LANGUAGE_NAMES[l] || l]);
  for (const id of ["lang-select", "login-lang"]) fillOptions($(id), options).value = i18n.setting; // My settings, the sign-in's menu
  const note = $("lang-note");
  const texts = machineNote();
  clear(note);
  note.classList.toggle("hidden", !texts);
  if (texts) for (const s of texts) note.appendChild(textEl("span", s));
}
