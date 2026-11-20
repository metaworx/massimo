// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

// The Massimo web UI, System settings → Roles and Capabilities for every
// role: their plain name, which calls their parts (ui/src/admin/,
// MassimoParts.roles).
"use strict";

// renderRoles draws Roles and Capabilities for every role again (app.js
// applyLanguage).
function renderRoles() {
  MassimoParts.roles.render();
}
