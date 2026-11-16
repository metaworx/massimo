// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

// The Massimo web UI, System settings → Cubes and Cube settings: the
// plain names of the region, which call its part (ui/src/cubes/,
// MassimoParts.cubes, AP UiParts Block 5), and the Cubes card's badges
// in its head. The part draws every served Cube in its panel, the found
// Cubes in one fold, and Cube settings the chosen Cube's panel alone; it
// follows every load of the list in place, so that what is open, focused
// or typed stays (docs/ui.md, "Cube settings").
"use strict";

// cubesVisible reports whether a page with the Cubes' panels is shown:
// System settings, or Cube settings.
function cubesVisible() {
  return isShown("system") || isShown("cube");
}

// cubesPolled is called by loadCubes after every load of the list: the
// panels follow it in place, and the card's badges are drawn again
// while the Cubes are shown.
function cubesPolled() {
  MassimoParts.cubes.polled();
  if (cubesVisible()) renderCubesBadges();
}

// renderCubes draws the Cubes where they are shown: on Cube settings
// while that page is shown, else in System settings → Cubes, the other
// place empty, as a panel's ids are its serial's; and the card's badges.
function renderCubes() {
  MassimoParts.cubes.render();
  if (!isShown("cube")) renderCubesBadges();
}

// showFoundFold opens the fold of the Cubes found on the network (home.js
// showFound).
function showFoundFold() {
  MassimoParts.cubes.openFound();
}

// proxyFailing counts the Cube proxy listeners of cubes that do not listen.
function proxyFailing(cubes) {
  return cubes.reduce((n, c) => n + (c.proxy_failing || []).length, 0);
}

// renderCubesBadges draws the Cubes card's badges from the list as the
// last poll gave it, at every poll, a panel open or not: how many are
// served; those not connected by their link's state, as their signs
// show it, a cross red ("not connected"), a held connection or a reboot
// amber; the proxies that do not listen, the Cubes found at another
// address, those found and not served.
function renderCubesBadges() {
  const count = (link) => state.cubes.filter((c) => linkState(c) === link).length; // app.js
  const lost = state.cubes.filter((c) => linkSign(c) === "bad").length; // header.js
  const badges = [badgeEl(tn("cubes.badge.served", state.cubes.length), "mode")];
  if (lost) badges.push(badgeEl(tn("cubes.badge.disconnected", lost), "bad"));
  if (count("held")) badges.push(badgeEl(tn("cubes.badge.held", count("held")), "warn"));
  if (count("rebooting")) badges.push(badgeEl(tn("cubes.badge.rebooting", count("rebooting")), "warn"));
  const failing = proxyFailing(state.cubes);
  if (failing) badges.push(badgeEl(tn("proxy.badge.failing", failing), "warn"));
  const elsewhere = state.cubes.filter((c) => c.found_elsewhere).length;
  if (elsewhere) badges.push(badgeEl(tn("cubes.badge.elsewhere", elsewhere), "warn"));
  if (state.found.length) badges.push(badgeEl(tn("cubes.badge.found", state.found.length), "warn"));
  sectionBadges($("sys-cubes"), badges);
}
