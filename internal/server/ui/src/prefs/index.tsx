// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/prefs/index.tsx
 *
 * My settings as parts (AP UiParts, Block 13): their rows drawn once
 * into #prefs-rows as parts.js loads, before init applies the settings,
 * and updated in place. The plain code calls MassimoParts.prefs.render
 * (prefs.js renderDeviceSwitches and renderLangSelect) where it applied
 * the settings or changed the language.
 */

import { mounter, register } from "../shared/region";
import { Settings } from "./Settings";
import { load } from "./store";

const mountRows = mounter("prefs-rows", () => <Settings />);
mountRows();

/** The functions the plain code calls, MassimoParts.prefs. */
const prefs = register("prefs", {
  /** render draws My settings again from state.settings (prefs.js renderDeviceSwitches, renderLangSelect). */
  render(): void {
    load.bump();
  },
});

/** MassimoParts.prefs, as index.ts declares it. */
export type PrefsPart = typeof prefs;
