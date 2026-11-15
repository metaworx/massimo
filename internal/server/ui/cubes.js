// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

// The Massimo web UI, System settings → Cubes: the Cubes the server serves,
// from its store, each in a panel with its name, address, proxy and
// hidden devices (PUT /api/v1/cubes/{sn}), a notice with "Take the
// file's value" for a line of the settings file that differs, and
// Remove (DELETE); then, in one fold, the Cubes and the Massimo proxies
// found on the network, a Cube with Add (POST /api/v1/cubes). The list is drawn when the settings open, after every
// change, and on a poll while no panel is open, so that a poll never
// throws away what is being typed. Each panel starts with Pair a device
// (capability pair), the pairing flow (pairing.js). A role
// with pair but without cubes sees the panels with the pairing alone.
// A Cube's panel itself, its head, facts and actions, is cube-panel.js.
// The page Cube settings (☰) shows the chosen Cube's panel alone, open
// (renderCubePage); a panel is drawn in one place at a time, as its ids
// are its serial's.
"use strict";

const cubesUI = {
  open: new Set(), // serials of the open panels
  edits: {}, // per serial: the hidden list being edited
  names: {}, // radio address → device name, from the device lists seen
  serials: {}, // radio address → device serial, from the device lists seen
  devices: {}, // per serial: the Cube's visible devices, for the hide choice
  rooms: {}, // per serial: the rooms of a Cube that is not the one shown
  pairing: {}, // per serial: the pairing flow, see pairingOf
  page: "", // the serial whose panel Cube settings has loaded for (renderCubePage)
  foundOpen: false, // the found Cubes' fold (foundFold)
  linesOpen: false, // its lines for the settings file (foundHints)
  busy: false,
};

// cubesMsg shows the outcome of the last action under the list, or
// under the panel of Cube settings.
function cubesMsg(text, ok) {
  showResult($(isShown("cube") ? "cube-page-msg" : "cubes-msg"), text, ok);
}

// cubesVisible reports whether a page with the Cubes' panels is shown:
// System settings, or Cube settings.
function cubesVisible() {
  return isShown("system") || isShown("cube");
}

// cubesPolled is called by loadCubes after every load of the list: the
// card's badges drawn again always; the list drawn again while no panel
// is open, otherwise each panel's head and facts refreshed in place
// (fillCubeFacts), so that what is typed stays; Cube settings' panel,
// always open, refreshed in place while it is the chosen Cube's.
function cubesPolled() {
  if (!cubesVisible()) return;
  renderCubesBadges();
  if (isShown("cube")) {
    const d = $("cube-page-body").querySelector(".cube-panel");
    const c = state.cubes.find((x) => x.serial === state.serial);
    if (d && c && d.dataset.serial === c.serial) fillCubeFacts(d, c);
    else if (!cubesUI.busy) renderCubes();
    return;
  }
  if (cubesUI.open.size === 0 && !cubesUI.busy) {
    renderCubes();
    return;
  }
  for (const d of document.querySelectorAll("#cubes-list .cube-panel")) {
    const c = state.cubes.find((x) => x.serial === d.dataset.serial);
    if (c) fillCubeFacts(d, c);
  }
}

// foundRow is a Cube or a Massimo proxy found on the network, from its
// template (tpl-found-row): its sign, its serial, for a Cube its address,
// DHCP flag and firmware, the massimo proxies that answer for it, for a
// proxy what it serves, for both the Massimo server serving it
// (foundWhat), in expert mode the radio and MAC address and the gateway
// it uses, how long ago it answered, and for a Cube Add (addCube), which
// serves it at once: a proxy is another server's, listed so that its
// Cube is not added twice. System settings → Cubes lists it (foundFold).
function foundRow(c) {
  const li = fromTemplate("tpl-found-row");
  li.dataset.serial = c.serial;
  li.dataset.kind = c.kind || "unknown";
  li.prepend(iconEl(c.kind === "proxy" ? "icon-proxy" : "icon-cube3d", "ni found-icon"));
  li.querySelector(".found-serial").textContent = c.serial;
  const part = (cls, values) => {
    const el = li.querySelector(cls);
    const v = joinParts(values);
    if (v.length) el.append(" · ", ...v);
    else el.remove();
  };
  // dhcp is absent until the Cube has answered the c request.
  const dhcp = c.dhcp === true ? t("cubes.dhcp") : c.dhcp === false ? t("cubes.static") : "";
  // proxies are the other addresses that answered for its serial.
  const proxies = (c.proxies || []).join(", ");
  const own = c.kind === "proxy" ? [] : [dhcp, c.firmware ? t("budget.firmware") + " " + c.firmware : "",
    proxies ? t("cubes.viaProxy", { address: proxies }) : ""];
  part(".found-facts", [c.address, ...own, ...foundWhat(c)]);
  part(".found-extra", [c.rf_address, c.mac, c.gateway ? t("cubes.gateway") + " " + c.gateway : ""]);
  const add = li.querySelector(".add-cube");
  const age = ageEl(c.last_seen, false, true);
  if (age) add.before(age);
  if (c.kind === "proxy") {
    add.remove();
    return li;
  }
  add.setAttribute("aria-label", t("cubes.addLabel", { serial: c.serial }));
  add.addEventListener("click", () => addCube(c.serial, add));
  return li;
}

// foundWhat is what a found entry is and who serves it, as parts for
// joinParts: a Massimo proxy and the Cube it serves, and the Massimo
// server serving it, the proxy's own or the one a Cube's proxy runs on
// (host), its name a link to the address its announcement on the
// network gives.
function foundWhat(c) {
  const parts = [];
  if (c.kind === "proxy") parts.push(c.serves ? t("found.proxyServes", { serial: c.serves }) : t("found.proxy"));
  if (c.host) parts.push(tNodes("found.servedBy", { host: linkEl(c.host.url, c.host.name || c.host.url, "found-host") })); // i18n.js, components.js
  return parts;
}

// joinParts joins parts, each a text or an array of texts and elements,
// with " · ", leaving out the empty ones.
function joinParts(parts) {
  const out = [];
  for (const p of parts) {
    if (!p || (Array.isArray(p) && !p.length)) continue;
    if (out.length) out.push(" · ");
    out.push(...[].concat(p));
  }
  return out;
}

// foundFold is what was found on the network and is not served, one fold
// under the served Cubes, closed at first and kept as it was across
// redraws (cubesUI.foundOpen): the found Cubes' sign (foundCountEl), its
// name and how many Cubes and Massimo proxies, then a row each (foundRow)
// and how to serve them (foundHints). For a role with cubes.
function foundFold(found) {
  const proxies = found.filter((c) => c.kind === "proxy").length;
  const cubes = found.length - proxies;
  const badges = [];
  if (cubes) badges.push(badgeEl(tn("found.cubes", cubes), "mode"));
  if (proxies) badges.push(badgeEl(tn("found.proxies", proxies), "mode"));
  const ul = elem("ul", "cube-found-list");
  for (const c of found) {
    const li = foundRow(c);
    li.classList.add("cube-found-row");
    ul.appendChild(li);
  }
  const d = detailsPanel({ name: t("found.title"), badges, open: cubesUI.foundOpen, onToggle: (o) => { cubesUI.foundOpen = o; }, cls: "found-panel needs-cubes" },
    [ul, foundHints(found)]);
  d.querySelector(":scope > summary").prepend(foundCountEl(found.length));
  return d;
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

// renderCubes draws the Cubes' panels where they are shown: Cube
// settings' while that page is shown (renderCubePage), else System
// settings → Cubes from state.cubes and state.found; the other place
// empty, as a panel's ids are its serial's.
function renderCubes() {
  const box = $("cubes-list");
  const page = $("cube-page-body");
  if (!box || !page) return;
  clear(box);
  clear(page);
  if (isShown("cube")) {
    renderCubePage(page);
    return;
  }
  if (!state.cubes.length) box.appendChild(textEl("p", t("cubes.none"), "muted small"));
  for (const c of state.cubes) box.appendChild(cubePanel(c));
  renderCubesBadges();
  if (state.found.length) box.appendChild(foundFold(state.found));
}

// renderCubePage draws Cube settings into box: the chosen Cube's panel,
// open, or that no Cube is configured.
function renderCubePage(box) {
  const c = state.cubes.find((x) => x.serial === state.serial);
  if (!c) {
    box.appendChild(textEl("p", t("cubes.none"), "muted small"));
    return;
  }
  box.appendChild(cubePanel(c, { page: true }));
}

// foundHints is how the found Cubes are served: Add at once, or the lines
// for the server's settings file and a restart, in a fold of their own
// with one copy icon for them all (cubesUI.linesOpen), and where that
// file is. The address makes serve take the Cube; a name alone would as
// well, serve then follows the address the Cube answers discovery from.
// A Massimo proxy has no lines: it is another server's.
function foundHints(found) {
  const box = elem("div", "cubes-found-hints");
  box.appendChild(textEl("p", t("discover.hint"), "muted small"));
  const lines = [];
  for (const c of found) {
    if (c.kind === "proxy") continue;
    const p = "MASSIMO_CUBE_" + c.serial + "_";
    lines.push(p + "ADDRESS=" + (c.address || ""), p + "NAME=");
  }
  if (!lines.length) return box;
  const text = lines.join("\n");
  const sum = elem("summary", "", textEl("span", t("found.lines"), "panel-name"), copyIcon(() => text, t("found.copyLines"))); // components.js
  const d = elem("details", "sys-sub found-lines", sum, textEl("pre", text, "env-lines mono"), textEl("p", t("discover.hintFile"), "muted small"));
  d.open = cubesUI.linesOpen;
  d.addEventListener("toggle", () => { cubesUI.linesOpen = d.open; });
  box.appendChild(d);
  return box;
}

// initCubes draws System settings → Cubes when the page opens; the panels
// still open from before load what they would load when opened. Cube
// settings draws its panel when it opens, which loads what it shows.
function initCubes() {
  onView("cube", () => {
    cubesMsg("");
    cubesUI.page = "";
    renderCubes();
  });
  onView("system", () => {
    cubesMsg("");
    renderCubes();
    for (const serial of cubesUI.open) {
      loadCubeDevices(serial).then(() => devicesLoaded(serial));
      loadPairing(serial);
      loadCubeRooms(serial);
    }
  });
}

document.addEventListener("DOMContentLoaded", initCubes);
