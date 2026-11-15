// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

// The Massimo web UI, the pages and the main menu: which page is shown,
// who may open it, the page and the log's choices in the address, and
// the scroll each page keeps.
"use strict";

// The pages of the main area: Rooms ("tiles"), the header's one button,
// and the pages of ☰ (MENU_VIEWS): the log ("log"), the settings pages,
// the chosen Cube's ("cube"), System settings ("system"), Maintenance
// ("maint") and System info ("info"), and the personal settings ("me": My profile, My settings and
// Notifications as its cards), each with its section; ☰ is highlighted
// while one of its pages is shown. The tiles keep their state while
// hidden. The page shown is in the address
// (VIEW_HASH), Rooms without one, the personal page at a card by the
// card's own (CARD_HASH, the addresses of the pages they were), and the
// log's range, kind and device after its page (logHash), so that a
// reload or a link opens it again as chosen and the browser's back and
// forward move between the pages.

const VIEW_SECTION = { log: "logview", me: "me", system: "system", maint: "maint-page", info: "info-page", cube: "cube-page" };
const VIEW_BUTTON = { tiles: "view-tiles" };
const VIEW_HASH = { tiles: "", log: "#log", me: "#me", system: "#system", maint: "#maintenance", info: "#info", cube: "#cube" };
// MENU_VIEWS are the pages of ☰, every page but Rooms, with the ids of
// their entries.
const MENU_VIEWS = { log: "menu-log", cube: "menu-cube", system: "menu-system", maint: "menu-maint", info: "menu-info", me: "menu-me" };
// SETTINGS_PAGES are the settings pages, in ☰'s order, each with its
// icon, the key of its name and the class that hides it from a role that
// may not open it (login.js, style.css); each page's row holds them all
// in this order (components.js pageSwitch).
const SETTINGS_PAGES = [
  { view: "cube", icon: "icon-cube3d", key: "nav.cubeTitle", needs: "needs-cubes-or-pair" },
  { view: "system", icon: "icon-cog", key: "nav.settingsTitle", needs: "needs-system" },
  { view: "maint", icon: "icon-tool", key: "nav.maintTitle", needs: "needs-maint" },
  { view: "info", icon: "icon-info", key: "nav.infoTitle", needs: "needs-read" },
];
const CARD_HASH = { "#profile": "profile", "#prefs": "prefs", "#notify": "notify" };
let currentView = "tiles";

// viewOfHash is the page an address names, the part before its "?",
// Rooms for none or another; a card's address names its page.
function viewOfHash(hash) {
  const page = hash.split("?")[0];
  if (CARD_HASH[page]) return "me";
  return Object.keys(VIEW_HASH).find((v) => VIEW_HASH[v] && VIEW_HASH[v] === page) || "tiles";
}

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
  return VIEW_HASH.log + (s ? "?" + s : "");
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

// setAddress puts a page's hash into the address: as a new entry of the
// browser's history ("push") or in place of the current one ("replace");
// nothing where the address already says it.
function setAddress(hash, how) {
  if (location.hash === hash) return;
  const url = location.pathname + location.search + hash;
  if (how === "replace") history.replaceState(null, "", url);
  else history.pushState(null, "", url);
}

// showAddress shows the page the address names, the log with the choices
// it holds, at the start and on back and forward; the address is written
// again in place, a value dropped and the log's choices in their order.
function showAddress() {
  const view = viewOfHash(location.hash);
  if (view === "log") readLogHash(location.hash);
  showView(view, "replace", CARD_HASH[location.hash.split("?")[0]]);
}

// mayShow reports whether the signed-in user may open a page: System
// settings with one of its capabilities, Maintenance with one of those
// or update, restart or reboot (Software update is for all of them), the
// Cube's settings with cubes or with pair (its pairing alone), System
// info with read.
function mayShow(view) {
  if (view === "system") return hasSettingsPage(); // login.js
  if (view === "maint") return hasSystem(); // login.js
  if (view === "cube") return hasCap("cubes") || hasCap("pair"); // login.js
  if (view === "info") return hasCap("read");
  return view in VIEW_BUTTON || view in VIEW_SECTION;
}

// markSettingsPages shows each settings page's title in place of its
// buttons (components.js pageSwitch) where the signed-in user may open
// no other settings page (mayShow); login.js applyCaps calls it.
function markSettingsPages() {
  const single = SETTINGS_PAGES.filter((p) => mayShow(p.view)).length < 2;
  for (const p of SETTINGS_PAGES) $(VIEW_SECTION[p.view]).querySelector(".page-bar").classList.toggle("single-page", single);
  fitSettingsPages();
}

// fitSettingsPages measures the names in the row of the settings page
// shown, for the width of its page's button (components.js fitPageTabs);
// a page shown, a change of language (app.js applyLanguage) and of the
// signed-in user (markSettingsPages) call it.
function fitSettingsPages() {
  for (const p of SETTINGS_PAGES) fitPageTabs($(VIEW_SECTION[p.view]));
}

// isShown reports whether a page is the one shown.
function isShown(view) {
  return currentView === view;
}

// showView shows a page, Rooms in place of one the user may not open,
// puts it into the address as address says (setAddress, a new entry by
// default; Rooms in place of a page refused), the log with its choices
// (logHash), the personal page at a card by the card's address, and
// tells the scripts that load what it shows (onView); the card shown
// comes into view (showCard), My profile only for a user.
function showView(view, address = "push", card) {
  if (!mayShow(view)) {
    view = "tiles";
    address = "replace";
  }
  const left = view !== currentView ? currentView : null;
  if (left) viewScroll[left] = window.scrollY;
  if (view !== "me" || (card === "profile" && !hasProfile())) card = undefined;
  if (view === "log") fillLogSelects(); // a device the Cube does not have goes before the address takes it
  setAddress(view === "log" ? logHash() : card ? "#" + card : VIEW_HASH[view], address);
  currentView = view;
  logState.open = view === "log";
  logPollSet();
  for (const id of ["home-notice", "house", "rooms", "unlisted"]) {
    $(id).classList.toggle("view-hidden", view !== "tiles");
  }
  for (const [name, id] of Object.entries(VIEW_SECTION)) {
    $(id).classList.toggle("hidden", view !== name);
  }
  for (const [name, id] of Object.entries(VIEW_BUTTON)) {
    $(id).classList.toggle("active", view === name);
    if (view === name) $(id).setAttribute("aria-current", "page");
    else $(id).removeAttribute("aria-current");
  }
  for (const [name, id] of Object.entries(MENU_VIEWS)) {
    if (view === name) $(id).setAttribute("aria-current", "page");
    else $(id).removeAttribute("aria-current");
  }
  $("nav-menu").querySelector("summary").classList.toggle("active", view in MENU_VIEWS);
  if (view === "log") {
    loadLog();
    logLoadUsers();
  }
  document.dispatchEvent(new CustomEvent("viewshown", { detail: view }));
  if (card) showCard($(card), true); // components.js
  else if (left) restoreScroll(viewScroll[view] || 0);
}

// viewScroll is where each page was left, by view: the pages share the
// window's one scroll position, and each comes back at its own.
const viewScroll = {};

// restoreScroll brings a page shown back to y, and again while its
// content grows for a second after (a page that loads as it is shown),
// until the user scrolls by hand.
function restoreScroll(y) {
  const go = () => window.scrollTo({ top: y, behavior: "instant" });
  go();
  if (!y) return;
  const by = ["wheel", "touchstart", "keydown", "pointerdown"];
  const ro = new ResizeObserver(go);
  const stop = () => {
    ro.disconnect();
    for (const ev of by) removeEventListener(ev, stop);
  };
  ro.observe(document.querySelector("main"));
  for (const ev of by) addEventListener(ev, stop, { passive: true });
  setTimeout(stop, 1000);
}

// onView runs fn each time a page is shown, for a script that loads
// what the page shows.
function onView(view, fn) {
  document.addEventListener("viewshown", (ev) => {
    if (ev.detail === view) fn();
  });
}

// initPages wires the pages and the main menu: the header's Rooms
// button, the menu's items, each settings page's switch, and back and
// forward.
function initPages() {
  $("view-tiles").addEventListener("click", () => showView("tiles"));
  for (const [view, id] of Object.entries(MENU_VIEWS)) $(id).addEventListener("click", () => showView(view));
  dropdown($("nav-menu"));
  for (const p of SETTINGS_PAGES) pageSwitch($(VIEW_SECTION[p.view]), p.view, SETTINGS_PAGES, (v) => showView(v)); // components.js
  for (const p of SETTINGS_PAGES) onView(p.view, fitSettingsPages);
  // back and forward, and a page named in the address bar
  window.addEventListener("popstate", () => showAddress());
}
