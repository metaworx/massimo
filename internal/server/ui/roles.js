// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

// The Massimo web UI, System settings → Roles and Capabilities for every
// role: their plain name, which calls their parts (ui/src/admin/,
// MassimoParts.roles), and what a capability allows in words, which
// Integrations reads.
"use strict";

// capLabel is what a capability allows, translated.
function capLabel(c) {
  return t("cap." + c);
}

// renderRoles draws Roles and Capabilities for every role again (app.js
// applyLanguage).
function renderRoles() {
  MassimoParts.roles.render();
}
