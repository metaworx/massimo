// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/prefs/store.ts
 *
 * What My settings are drawn from: the page's settings stay app.js'
 * plain state.settings, saved by setSetting, and the counter says "draw
 * them again", raised where the plain code applied them (applySettings,
 * a change of language) and after each change made here.
 */

import { counter } from "../shared/counter";

/** My settings' load counter: bumped where the settings were applied or changed; every part of them uses it. */
export const load = counter();
