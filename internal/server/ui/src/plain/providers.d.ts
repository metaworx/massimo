// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/plain/providers.d.ts
 *
 * The globals of the plain script ui/providers.js that the parts call:
 * the enabled sign-in providers, which the sign-in and My profile read as
 * well.
 */

import type { LoginProvider } from "../api";

declare global {
  /** Reads the enabled sign-in providers (GET /api/v1/login/providers), keeps them for the sign-in and My profile, and returns them; none where the server answers none. */
  function loadLoginProviders(): Promise<LoginProvider[]>;
}

export {};
