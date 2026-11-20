// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

// The Massimo web UI, configuration: what of a room's configuration the
// plain scripts and the parts call. The configuration itself, inline in
// the tile, is a part since AP UiParts' Block 8 (ui/src/rooms/Config.tsx);
// here stay how long a name may be and the name a device is offered,
// which repair.js and the form that puts a device into a room
// (ui/src/shared/put.tsx) use, and the kit the week programme editor
// (week-program.js) builds with. Uses the helpers of app.js,
// components.js and rooms.js.
"use strict";

// NAME_BYTES is the longest name of a room or a device the Cube takes,
// in bytes.
const NAME_BYTES = 32;

// byteLength is a text's length in UTF-8 bytes.
function byteLength(s) {
  return new TextEncoder().encode(s).length;
}

// --- a device's default name --------------------------------------------

// defaultDeviceName is the name a device is offered where it is put into
// a room (ui/src/shared/put.tsx, D200-A) or listed there (repair.js, D185), as the
// server's default (repair_api.go, defaultDeviceName) with the type's
// word in the page's language: "<room name> <type> <n>", n the lowest
// number from 1 that no name of the room's (taken) has in that form,
// whatever its case. One longer than NAME_BYTES is offered as it is: its
// field says so (markNameBytes), and the user shortens it before it is
// sent (D201), where the server, with nobody to ask, drops the room's
// name.
function defaultDeviceName(room, word, taken) {
  const prefix = (String(room || "").trim() + " " + word).trim();
  return prefix + " " + freeNameNumber(prefix, taken);
}

// markNameBytes marks a name's field, as markRange marks a number's
// (components.js): a name longer than NAME_BYTES outlines it in red and
// shows hint, the amber limit before the box; it returns whether the
// name fits.
function markNameBytes(input, hint) {
  return markRange(input, hint, byteLength(input.value.trim()) <= NAME_BYTES);
}

// freeNameNumber is the lowest number from 1 that no name of taken has
// in the form "<prefix> <n>", whatever its case.
function freeNameNumber(prefix, taken) {
  const p = prefix.toLowerCase() + " ";
  const used = new Set();
  for (const name of taken) {
    const s = String(name || "").trim();
    if (!s.toLowerCase().startsWith(p)) continue;
    const rest = s.slice(p.length);
    if (/^[0-9]+$/.test(rest) && Number(rest) > 0) used.add(Number(rest));
  }
  let n = 1;
  while (used.has(n)) n++;
  return n;
}

// roomDeviceNames are the names of the devices a Cube's list has in a
// room, as the page holds them: the chosen Cube's, whose devices a
// repair lists.
function roomDeviceNames(serial, room) {
  const r = serial === state.serial ? state.rooms.find((x) => x.id === room) : null;
  return r ? (r.devices || []).map((d) => d.name || "") : [];
}

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
