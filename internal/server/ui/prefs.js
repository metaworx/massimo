// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

// The Massimo web UI, My settings: their plain names, which call their
// part (ui/src/prefs/, MassimoParts.prefs).
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

// renderLangSelect draws the choices of the language again, My
// settings' and the sign-in card's, both parts (ui/src/prefs/,
// ui/src/signin/), which read the settings applied.
function renderLangSelect() {
  MassimoParts.prefs.render();
}
