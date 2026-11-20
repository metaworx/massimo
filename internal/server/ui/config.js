// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

// The Massimo web UI, configuration: what of a room's configuration the
// plain scripts and the parts call. The configuration itself, inline in
// the tile, is a part since AP UiParts' Block 8 (ui/src/rooms/Config.tsx),
// and how long a name may be and the name a device is offered are the
// parts' since Block 9 (ui/src/shared/name.ts); here stays the kit the
// week programme editor (week-program.js) builds with, of the helpers of
// app.js, components.js and rooms.js.
"use strict";

// --- week programme ---------------------------------------------------

// weekKit is what the week programme editor builds with: the page's
// translation, its elements (components.js) and temperatures (rooms.js).
function weekKit() {
  return {
    t, elem, textEl, button, switchEl, fieldRow, numberInput, fmtNum, fmtTemp,
    clamp: clampTemp,
    temps: { min: MIN_TEMP, max: MAX_TEMP, step: STEP },
  };
}
