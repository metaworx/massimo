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
import { me } from "./me";

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
// Who is in: state.me is shared/me.ts' signal me underneath, so that
// applyCaps' write and a sign-out's clearing are the parts' at once.
me.value = state.me;
Object.defineProperty(state, "me", { enumerable: true, configurable: true, get: () => me.value, set: (v) => { me.value = v; } });
// The Cubes the server serves and those found on the network, the Cube
// chosen, each Cube's daylight-saving rules, and the server's state: the
// loads of app.js and the stream write them (loadCubes, selectCube,
// addFound, loadTimeZone, serverReached, checkVersion).
adopt(state, ["cubes", "found", "serial", "tz", "serverDown", "newVersion"]);
// The user's colour schemes, the editor's draft and the scheme it started
// from (theme.js themeState), each written whole.
adopt(themeState, ["custom", "draft", "base"]);
