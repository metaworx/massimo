// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/prefs/index.tsx
 *
 * My settings as parts (AP UiParts, Block 13): their rows drawn once
 * into #prefs-rows and the Appearance card into #appearance as parts.js
 * loads, before init applies the settings, and updated in place: they
 * follow state.settings, the overrides of this device and theme.js'
 * schemes as signals (shared/data.ts).
 */

import { mounter } from "../shared/region";
import { Appearance } from "./Appearance";
import { forgetEditor } from "./scheme";
import { Settings } from "./Settings";

const mountRows = mounter("prefs-rows", () => <Settings />);
mountRows();
const mountAppearance = mounter("appearance", () => <Appearance />);
mountAppearance();

/** signedOut closes the scheme editor with its draft and clears its line (MassimoParts.signedOut, index.ts): nothing edited is left for the next user. */
export const signedOut = forgetEditor;
