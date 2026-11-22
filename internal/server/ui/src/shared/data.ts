// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/shared/data.ts
 *
 * The page's data as signals (AP UiParts, Block 15, D123-A): app.js'
 * state and theme.js' themeState stay the plain objects the plain
 * scripts read and write, and their fields become signals underneath as
 * parts.js loads, before init runs. A plain write is an assignment, as
 * before, and a part that reads a field in its drawing is drawn again
 * when it is written: one place writes each collection, and nothing tells
 * a part besides. A collection is written whole, never in place
 * (TestStateWrittenWhole); the settings are a live object, a signal per
 * setting, written by key. This file is imported first by index.ts.
 */

import { signal } from "@preact/signals";
import { live } from "./live";

/** adopt makes the fields of obj named signals underneath, each starting from its plain value. */
function adopt<T extends object>(obj: T, keys: (keyof T)[]): void {
  for (const k of keys) {
    const s = signal(obj[k]);
    Object.defineProperty(obj, k, { enumerable: true, configurable: true, get: () => s.value, set: (v) => { s.value = v; } });
  }
}

// The settings in effect, a signal per setting: app.js writes them by key
// (loadSettings, setSetting); the field holds that one live object, and
// the overrides kept for this device alone beside it (storeDeviceOverrides).
state.settings = live({ ...state.settings });
adopt(state, ["device"]);
// The user's colour schemes, the editor's draft and the scheme it started
// from (theme.js themeState), each written whole.
adopt(themeState, ["custom", "draft", "base"]);
