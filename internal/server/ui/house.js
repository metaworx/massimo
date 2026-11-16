// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

// The Massimo web UI, the House tile: the mode for the whole house or
// the rooms ticked, with its panel, is a part (ui/src/rooms/House.tsx);
// here the plain names other scripts call, which call it.
"use strict";

// houseOpen reports whether the House tile's panel is open (header.js
// somethingOpen).
function houseOpen() {
  return MassimoParts.rooms.houseOpen();
}

// closeHouse closes the House tile's panel (rooms.js closePanels, for
// Escape); what it holds stays for the next time it opens.
function closeHouse() {
  MassimoParts.rooms.closeHouse();
}
