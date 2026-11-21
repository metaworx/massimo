// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

// The Massimo web UI, Maintenance → Software update: its plain names,
// which call its part (ui/src/software/, MassimoParts.software, AP
// UiParts Block 12): the channel, the versions, the request to the update
// helper and the last updates are drawn there. The header's check for an
// update (header.js updateDue) and the server's sign
// (ui/src/header/ServerSign.tsx) call these. The server's restart and the
// computer's reboot are in Maintenance (ui/src/maintenance/). Once the
// server restarts with a new version, the version check in header.js
// offers the reload. The stream's server events go to the parts that
// follow the server while their page is shown (serverEvent).

"use strict";

// loadUpdate reads GET /api/v1/update and draws Software update; refresh
// asks for a release now. It resolves to the answer, null where the load
// failed.
function loadUpdate(refresh) {
  return MassimoParts.software.load(refresh);
}

// updateOut tells whether an update is out, for one who may install it
// (the server's sign, ui/src/header/ServerSign.tsx).
function updateOut() {
  return MassimoParts.software.out();
}

// updateLatest is the version an update would install, "" for none.
function updateLatest() {
  return MassimoParts.software.latest();
}

// serverEvent takes a server's event of the stream (app.js
// connectEvents: an update, a reboot, the settings file read, a change,
// the stream's resync): Software update and Maintenance read
// again what it says has changed while Maintenance is shown
// (MassimoParts.serverEvent, ui/src/index.ts).
function serverEvent(ev) {
  MassimoParts.serverEvent(ev);
}
