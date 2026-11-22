// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

// The Massimo web UI, the OpenID Connect providers (docs/api-v1.md,
// "Providers" and "Identities"): the enabled ones, which the sign-in
// (login.js), My profile's linked accounts (ui/src/profile/) and a user's
// identities read; System settings → Sign-in providers is a part
// (ui/src/admin/).
"use strict";

const providersUI = {
  login: [], // GET /api/v1/login/providers: the enabled ones, by key and name
};

// loadLoginProviders reads the enabled providers, for the sign-in's
// buttons, My profile's linked accounts and a user's identities (the
// part); none where the server answers none.
async function loadLoginProviders() {
  try {
    providersUI.login = (await apiRequest("GET", "api/v1/login/providers", undefined, { auth: false })) || [];
  } catch (e) {
    providersUI.login = [];
  }
  return providersUI.login;
}
