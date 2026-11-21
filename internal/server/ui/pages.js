// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

// The Massimo web UI, the pages: the plain names of the pages' store
// (ui/src/shared/pages.ts, MassimoParts.pages), which shows a page, keeps
// it in the address and each page's scroll, and says who may open which;
// and the log's choices in the address, the log's page plain until AP
// UiParts' Block 11.
"use strict";

// logHash is the log's address: its page, then All Cubes, the range, the
// kind and the device chosen (by its S/N), in that order, each left out
// at its default, so that a plain #log is the header's Cube's last 20
// events of every kind and device.
function logHash() {
  const q = new URLSearchParams();
  if (logState.cubes) q.set("cubes", logState.cubes);
  if (logState.range !== LOG_COUNT_RANGE) q.set("range", logState.range);
  if (logState.kind) q.set("kind", logState.kind);
  if (logState.source) q.set("source", logState.source);
  if (logState.device) q.set("device", logState.device);
  if (logState.level !== "info") q.set("level", logState.level);
  if (logState.user) q.set("user", logState.user);
  const s = q.toString();
  return "#log" + (s ? "?" + s : "");
}

// readLogHash takes the log's Cubes, range, kind and device from an
// address of the log, each at its default where it is missing or not one of its
// values; the device by its S/N, or an old link's radio address, which
// takes the S/N, and a device the Cube does not have goes, once its
// devices are loaded (fillLogSelects). The text filter is not in the
// address.
function readLogHash(hash) {
  const i = hash.indexOf("?");
  const q = new URLSearchParams(i < 0 ? "" : hash.slice(i + 1));
  const range = q.get("range");
  const kind = q.get("kind");
  const device = (q.get("device") || "").trim();
  logState.cubes = q.get("cubes") === "all" ? "all" : "";
  logState.range = LOG_RANGES.includes(range) ? range : LOG_COUNT_RANGE;
  logState.kind = LOG_KINDS.includes(kind) ? kind : "";
  if (/^[a-z]{3}[0-9]{7}$/i.test(device)) logState.device = device.toUpperCase();
  else logState.device = /^[0-9a-f]{6}$/i.test(device) ? device.toLowerCase() : "";
  const source = q.get("source");
  logState.source = LOG_SOURCES.includes(source) && !logState.device ? source : "";
  const level = q.get("level");
  logState.level = ["debug", "info", "warn"].includes(level) ? level : "info";
  const user = (q.get("user") || "").trim().toLowerCase();
  logState.user = /^[a-z0-9._-]{1,64}$/.test(user) ? user : "";
}

// setAddress puts a page's hash into the address, a new entry of the
// history ("push") or in place of the current one ("replace"), as the
// store does (log.js, for the log's choices).
function setAddress(hash, how) {
  MassimoParts.pages.setAddress(hash, how);
}

// showAddress shows the page the address names, at the start (app.js
// init) and on back and forward.
function showAddress() {
  MassimoParts.pages.showAddress();
}

// mayShow reports whether the signed-in user may open a page (login.js
// applyCaps).
function mayShow(view) {
  return MassimoParts.pages.mayShow(view);
}

// isShown reports whether a page is the one shown, at this moment.
function isShown(view) {
  return MassimoParts.pages.isShown(view);
}

// showView shows a page, Rooms in place of one the user may not open,
// and puts it into the address: a new entry unless address is "replace";
// card is a card of the personal page to bring into view.
function showView(view, address, card) {
  MassimoParts.pages.showView(view, address, card);
}

// currentView is the page shown, which login.js and the UI tests read.
Object.defineProperty(window, "currentView", { get: () => MassimoParts.pages.shown() });

// initPages wires back and forward, and hands the store the log's address of its own, which the store reads and
// writes and tells when the log is shown (ui/src/shared/pages.ts).
function initPages() {
  MassimoParts.pages.ownAddress("log", {
    read: readLogHash,
    // a device the Cube does not have goes before the address takes it
    write: () => {
      fillLogSelects();
      return logHash();
    },
    shown: (on) => {
      logState.open = on;
      logPollSet();
    },
    opened: () => {
      loadLog();
      logLoadUsers();
    },
  });
  // back and forward, and a page named in the address bar
  window.addEventListener("popstate", () => showAddress());
}
