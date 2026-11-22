// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

// The Massimo web UI, My settings: their rows and the Appearance card are
// a part (ui/src/prefs/), which follows the settings as signals
// (ui/src/shared/data.ts); here what init calls.
"use strict";

// --- settings panel ---------------------------------------------------

function initSettings() {
  // My settings opens from the 👤 menu (showView); its rows and the
  // Appearance card are a part (ui/src/prefs/), which saves each as it
  // is changed.
  initAppearance();
}
