// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

// The Massimo web UI, the home notice, which says on Rooms why it shows
// nothing or only what the server last had, and the search for Cubes on
// the LAN from the Cube choice, with the Cubes it finds.
"use strict";

// renderHomeNotice says on Rooms why it shows nothing, or only what the
// server last had: no Cube configured, or the Cube chosen not connected,
// with the address the server tries, when the Cube last sent data or
// else last answered discovery, what to check, and for a role with cubes
// the way to System settings → Cubes. The rooms the server still has
// stay under it.
function renderHomeNotice() {
  const box = $("home-notice");
  const cube = state.cubes.find((c) => c.serial === state.serial);
  if (cube && cube.connected) {
    box.classList.add("hidden");
    return;
  }
  const title = box.querySelector(".home-notice-title");
  const detail = box.querySelector(".home-notice-detail");
  const check = box.querySelector(".home-notice-check");
  if (!cube) {
    title.textContent = t("home.noCube");
    detail.textContent = t(hasCap("cubes") ? "home.noCubeAdd" : "home.noCubeAsk");
    check.textContent = "";
  } else {
    // its name where it has one, its serial in the detail (as on its label)
    title.textContent = t("home.notConnected", { name: cube.name || cube.serial });
    const parts = [cube.address ? t("home.address", { address: cube.address }) : t("home.noAddress")];
    const why = linkText(cube); // a held connection, a refusal: first
    if (why) parts.unshift(why);
    if (cube.name) parts.push(t("home.serial", { serial: cube.serial }));
    const data = timeOf(cube.updated);
    const seen = timeOf(cube.last_seen);
    if (data) parts.push(t("home.lastData", { time: fmtInstant(data) }));
    else if (seen) parts.push(t("home.lastSeen", { time: fmtInstant(seen) }));
    else parts.push(t("home.noData"));
    detail.textContent = parts.join(" ");
    check.textContent = t("home.check");
  }
  check.classList.toggle("hidden", !check.textContent);
  box.classList.remove("hidden");
}

// --- Cube discovery ---------------------------------------------------

// DISCOVER is the value of the Cube choice's entry that searches the LAN.
const DISCOVER = "__discover";
const discovery = { running: false, timer: null };

// showSearch shows the state of a search next to the Cube choice, for
// ms milliseconds when given.
function showSearch(text, ms, bad) {
  const el = $("cube-search");
  if (discovery.timer) clearTimeout(discovery.timer);
  discovery.timer = null;
  el.textContent = text;
  el.classList.toggle("bad", !!bad);
  el.classList.toggle("hidden", !text);
  if (ms) discovery.timer = setTimeout(() => showSearch(""), ms);
}

// discoverCubes asks the server to search the LAN for Cubes (about two
// seconds), then reloads the list and says what it found.
async function discoverCubes() {
  if (discovery.running) return;
  discovery.running = true;
  showSearch(t("discover.searching"));
  renderCubeSelect();
  try {
    const res = (await api("POST", "api/v1/cubes/discover")) || {};
    await loadCubes();
    const n = (res.found || []).filter((c) => !isConfigured(c)).length;
    showSearch(n ? tn("discover.found", n) : t("discover.none"), 8000);
  } catch (e) {
    showSearch(t("discover.failed", { error: e.message }), 15000, true);
  } finally {
    discovery.running = false;
    renderCubeSelect();
  }
}

// addFound takes a Cube from a cube_found event into the list.
function addFound(c) {
  if (!c || !c.serial || state.cubes.some((x) => x.serial === c.serial)) return;
  const i = state.found.findIndex((x) => x.serial === c.serial);
  if (i >= 0) state.found[i] = c;
  else state.found.push(c);
  renderCubeSelect();
}

// showFound opens System settings with the Cubes card in view and the
// fold of what was found open (cubes.js foundFold).
function showFound() {
  cubesUI.foundOpen = true; // cubes.js
  showView("system");
  renderCubes(); // cubes.js
  showCard($("sys-cubes"), true); // components.js
}
