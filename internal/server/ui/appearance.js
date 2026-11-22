// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

// The Massimo web UI, the Appearance card of My settings: its plain name,
// which applies the appearance; the card is a part (ui/src/prefs/), where
// the choices of mode and scheme, the scheme editor with its contrast
// hints, and a scheme's export and import as a file are drawn, following
// the settings as signals; and the system's changes it follows.
"use strict";

// applyAppearance applies the appearance settings of state.settings;
// applySettings and a change of language call it, and theme.js' choices
// of the mode and the scheme.
function applyAppearance() {
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
