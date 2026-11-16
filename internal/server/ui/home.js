// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

// The Massimo web UI, the home notice, which says on Rooms why it shows
// nothing or only what the server last had, and the search for Cubes on
// the LAN from the Cube choice, with the Cubes it finds.
"use strict";

// otherNetworkEls says that a Cube's address lies outside this machine's
// networks (other_network), as the children of a notice: that the
// server reaches it only through a router, the address a Cube without a
// DHCP lease uses, and the netconfig line that gives it a fixed address,
// which copies (components.js copyEl).
function otherNetworkEls(c, address) {
  const line = "massimo netconfig -sn " + c.serial + " -broadcast -static <ip> -gateway <gateway> -netmask <netmask>";
  return [t("cubes.otherNetwork", { address }) + " " + t("cubes.otherNetworkFix") + " ",
    copyEl(line, { label: t("cubes.netconfigLine"), icon: true, cls: "mono netconfig-line" })];
}

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
    check.dataset.key = "";
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
    // A Cube outside this machine's networks: why, and the way to an
    // address here, in place of the usual checks; drawn again only where
    // it changed, so that a poll leaves a focused copy icon where it is.
    const key = cube.other_network ? [i18n.lang, cube.serial, cube.address].join(" ") : "";
    if (!key) check.textContent = t("home.check");
    else if (check.dataset.key !== key) check.replaceChildren(...otherNetworkEls(cube, hostPort(cube.address).host)); // cubes.js
    check.dataset.key = key;
  }
  check.classList.toggle("hidden", !check.textContent);
  box.classList.remove("hidden");
}

// --- Cube discovery ---------------------------------------------------

// DISCOVER is the value of the Cube choice's entry that searches the LAN.
const DISCOVER = "__discover";
const discovery = { running: false };

// discoverCubes asks the server to search the LAN for Cubes (about two
// seconds), the Cube choice's entry saying so meanwhile, then reloads the
// list and says what it found in the page's toast (components.js
// showToast): Cubes found with Show, which opens them in System settings
// for a role with cubes, as their entries in the list do; a failure in
// red, for longer.
async function discoverCubes() {
  if (discovery.running) return;
  discovery.running = true;
  renderCubeSelect();
  try {
    const res = (await api("POST", "api/v1/cubes/discover")) || {};
    await loadCubes();
    const n = (res.found || []).filter((c) => !isConfigured(c)).length;
    const show = n && hasCap("cubes") ? { label: t("discover.show"), run: showFound } : null;
    showToast(n ? tn("discover.found", n) : t("discover.none"), { ms: 8000, action: show });
  } catch (e) {
    showToast(t("discover.failed", { error: e.message }), { ms: 15000, tone: "bad" });
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
// fold of what was found open (ui/src/cubes/Found.tsx).
function showFound() {
  showFoundFold(); // cubes.js
  showView("system");
  renderCubes(); // cubes.js
  showCard($("sys-cubes"), true); // components.js
}
