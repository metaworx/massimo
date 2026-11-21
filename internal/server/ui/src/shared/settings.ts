// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/shared/settings.ts
 *
 * The page's settings applied, as a counter: the settings stay app.js'
 * plain state.settings, saved by setSetting, and the counter says "draw
 * them again", raised where the plain code applied them (applySettings,
 * a change of language: MassimoParts.prefs.render) and after each change
 * My settings make. My settings read it, and the sign-in card's choices
 * of the language and the mode (ModeSelect), which show the same
 * settings before anyone signs in.
 */

import { counter } from "./counter";

/** The settings' counter: bumped where the settings were applied or changed. */
export const applied = counter();
