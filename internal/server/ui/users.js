// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

// The Massimo web UI, System settings → Users: its plain names, which
// call its part (ui/src/admin/, MassimoParts.users). The check of a new
// user's name is the parts' (ui/src/shared/username.tsx).
"use strict";

// loadUsers reads System settings → Users again, for a change made in My
// profile (ui/src/profile/send.ts): the part reads the users and draws them.
function loadUsers() {
  return MassimoParts.users.load();
}

// renderUsers draws System settings → Users again (app.js applyLanguage).
function renderUsers() {
  MassimoParts.users.render();
}
