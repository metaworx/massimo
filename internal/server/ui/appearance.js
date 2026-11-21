// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

// The Massimo web UI, the Appearance card of My settings: its plain name,
// which applies the appearance and calls its part (ui/src/prefs/,
// MassimoParts.prefs), where the choices of mode and scheme, the scheme
// editor with its contrast hints, and a scheme's export and import as a
// file are drawn; and the system's changes it follows.
"use strict";

// applyAppearance draws the appearance settings from state.settings and
// applies them; applySettings and a change of language call it, and
// theme.js' choices of the mode and the scheme. The sign-in's choice of
// the mode is plain with the sign-in.
function applyAppearance() {
  $("login-mode").value = state.settings.theme; // the sign-in's menu
  MassimoParts.prefs.render();
  reapplyTheme();
  applyBlink();
  applyIcons();
}

// initAppearance follows the system's changes: its wish for less motion,
// which blinking "as the system" follows, and its light or dark, which
// the mode "as the system" follows.
function initAppearance() {
  REDUCED_MOTION.addEventListener("change", applyBlink);
  if (darkQuery) darkQuery.addEventListener("change", reapplyTheme);
}
