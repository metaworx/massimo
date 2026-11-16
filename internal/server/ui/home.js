// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

// The Massimo web UI, the home notice, which says on Rooms why it shows
// nothing or only what the server last had, drawn by its part
// (ui/src/header/HomeNotice.tsx); the Cubes found on the LAN, which a
// cube_found event adds and the way to them in System settings.
"use strict";

// otherNetworkEls says that a Cube's address lies outside this machine's
// networks (other_network), as the children of a notice: that the
// server reaches it only through a router, the address a Cube without a
// DHCP lease uses, and the netconfig line that gives it a fixed address,
// which copies (components.js copyEl). The parts put it in an element they
// leave to it: a Cube's panel's and the home notice's.
function otherNetworkEls(c, address) {
  const line = "massimo netconfig -sn " + c.serial + " -broadcast -static <ip> -gateway <gateway> -netmask <netmask>";
  return [t("cubes.otherNetwork", { address }) + " " + t("cubes.otherNetworkFix") + " ",
    copyEl(line, { label: t("cubes.netconfigLine"), icon: true, cls: "mono netconfig-line" })];
}

// renderHomeNotice shows the home notice on Rooms where the Cube chosen
// is not connected, or no Cube is configured, and draws it again
// (ui/src/header/HomeNotice.tsx): why Rooms shows nothing, or only what
// the server last had. The rooms the server still has stay under it.
function renderHomeNotice() {
  const cube = state.cubes.find((c) => c.serial === state.serial);
  $("home-notice").classList.toggle("hidden", !!(cube && cube.connected));
  MassimoParts.header.render();
}

// --- Cube discovery ---------------------------------------------------

// addFound takes a Cube from a cube_found event into the list.
function addFound(c) {
  if (!c || !c.serial || state.cubes.some((x) => x.serial === c.serial)) return;
  const i = state.found.findIndex((x) => x.serial === c.serial);
  if (i >= 0) state.found[i] = c;
  else state.found.push(c);
  renderCubeSelect();
}

// showFound opens System settings with the Cubes card in view and the
// fold of what was found open (ui/src/cubes/Found.tsx).
function showFound() {
  showFoundFold(); // cubes.js
  showView("system");
  renderCubes(); // cubes.js
  showCard($("sys-cubes"), true); // components.js
}
