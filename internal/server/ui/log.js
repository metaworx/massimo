// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

// The Massimo web UI, log view: the selected Cube's events from
// GET …/log, newest first, a page at a time with "Older" at the bottom
// back through the server's log, the live events of the stream coming in
// at the top, one line each, with a text
// filter, a choice of kind, of device and of how far back (the range),
// and Clear, which sets the first three back.
// Uses the state, api and helpers of app.js and tiles.js.
"use strict";

const LOG_MAX_LINES = 2000;

// LOG_PAGE is how many events a page asks for; a shorter page is the
// log's beginning.
const LOG_PAGE = 200;

// LOG_KINDS are the kinds of event the server sends, in its order
// (eventKinds in internal/server/log.go, which TestLogKinds compares).
const LOG_KINDS = ["connected", "disconnected", "hello", "metadata", "config", "state", "pairing_started", "paired", "pairing_ended",
  "device_forgotten", "cube_found", "restore_step", "restore_done"];

// LOG_RANGES are the choices of how far back the log goes: the newest
// LOG_COUNT events, the last 10 minutes, the last hour, today, this week
// (from Monday), this month, or all; LOG_COUNT_RANGE by default.
const LOG_RANGES = ["20", "10m", "1h", "today", "week", "month", "all"];
const LOG_COUNT_RANGE = "20";
const LOG_COUNT = 20;

const logState = {
  open: false,
  range: LOG_COUNT_RANGE, // one of LOG_RANGES
  lines: [], // {id, time, at, kind, device, text}, oldest first; drawn newest first
  filter: "",
  kind: "", // the kind chosen, "" for all
  device: "", // the radio address chosen, "" for all
  cursor: null, // the oldest line's id, else its time as received: where Older pages from
  more: false, // the server may have older events
  loading: false, // a page is on its way
  gen: 0, // raised by a new log, Clear and a Cube switch: an answer of an older one is dropped
  room: 0, // the lines live events may grow the log to before the oldest go (LOG_MAX_LINES at least)
  stick: true, // keep the newest line in view at the top unless the user scrolled down
  liveSeq: 0, // counts the live lines, so that loadLog finds the ones that came while it waited
};

// logDeviceName resolves a device's name from the rooms and the unlisted
// list already loaded; the address when unknown.
function logDeviceName(rf) {
  if (!rf) return "";
  for (const r of state.rooms) {
    const d = r.devices.find((x) => x.rf_address === rf);
    if (d) return d.name || rf;
  }
  const u = state.unlisted.find((x) => x.rf_address === rf);
  return u && u.name ? u.name : rf;
}

// logStateText is the short state text of a state event.
function logStateText(s) {
  if (!s) return "";
  const parts = [];
  if (s.reported === false) parts.push(t("log.notReported"));
  const st = s.state || {};
  if (st.mode) parts.push(modeName(st.mode));
  if (st.setpoint !== undefined) parts.push(t("log.set", { v: fmtNum(st.setpoint, 1) }));
  if (st.actual_temp !== undefined) parts.push(t("log.actual", { v: fmtNum(st.actual_temp, 1) }));
  if (st.valve_pct !== undefined) parts.push(t("log.valve", { v: st.valve_pct }));
  if (st.window_open !== undefined) parts.push(t(st.window_open ? "log.windowOpen" : "log.windowClosed"));
  if (st.until) parts.push(t("log.until", { until: st.until }));
  for (const [k, key] of [["battery_low", "log.batteryLow"], ["error", "log.error"], ["link_error", "log.linkError"], ["panel_locked", "log.locked"]]) {
    if (st[k]) parts.push(t(key));
  }
  return parts.join(", ");
}

// logChangesText says what a state event changed, one phrase per value,
// from the API's changes ({field: {from, to}}, null for a value the
// device did not have): "window opened", "set 18.0 → 21.0", "reports
// again". A field this page does not know shows as it came.
function logChangesText(changes) {
  const num = (v, digits) => (v === null || v === undefined ? "–" : fmtNum(v, digits));
  const flag = (to, on, off) => t(to ? on : off);
  const parts = [];
  for (const [k, c] of Object.entries(changes)) {
    const to = c ? c.to : null;
    const from = c ? c.from : null;
    switch (k) {
      case "reported":
        parts.push(flag(to, "log.ch.reportsAgain", "log.ch.lostState"));
        break;
      case "window_open":
        parts.push(flag(to, "log.ch.windowOpened", "log.ch.windowClosed"));
        break;
      case "mode":
        parts.push(t("log.ch.mode", { mode: to ? modeName(to) : "–" }));
        break;
      case "setpoint":
        parts.push(t("log.ch.set", { from: num(from, 1), to: num(to, 1) }));
        break;
      case "actual_temp":
        parts.push(t("log.ch.actual", { from: num(from, 1), to: num(to, 1) }));
        break;
      case "valve_pct":
        parts.push(t("log.ch.valve", { from: num(from, 0), to: num(to, 0) }));
        break;
      case "until":
        parts.push(to ? t("log.until", { until: to }) : t("log.ch.untilCleared"));
        break;
      case "battery_low":
        parts.push(flag(to, "log.batteryLow", "log.ch.batteryOk"));
        break;
      case "error":
        parts.push(flag(to, "log.error", "log.ch.errorCleared"));
        break;
      case "link_error":
        parts.push(flag(to, "log.linkError", "log.ch.linkErrorCleared"));
        break;
      case "panel_locked":
        parts.push(flag(to, "log.locked", "log.ch.unlocked"));
        break;
      default:
        parts.push(k + ": " + JSON.stringify(from) + " → " + JSON.stringify(to));
    }
  }
  return parts.join(", ");
}

// logLine turns an event object into a line of the view; the text of a
// state event is made when the line is drawn, in the current language:
// what changed when the event says so, else the whole state (a device's
// first event since the server started).
function logLine(ev) {
  const time = ev.time ? new Date(ev.time) : new Date();
  return {
    id: ev.id || 0,
    at: ev.time || "",
    time,
    kind: ev.kind || "",
    device: ev.device || "",
    state: ev.kind === "state" ? ev.state : null,
    changes: ev.kind === "state" && ev.changes && Object.keys(ev.changes).length ? ev.changes : null,
    cube: ev.kind === "cube_found" ? ev.cube : null,
    text: ev.error || "",
  };
}

function logText(line) {
  if (line.changes) return logChangesText(line.changes);
  if (line.state) return logStateText(line.state);
  if (line.cube) return t("discover.log", { serial: line.cube.serial, address: line.cube.address || "–" });
  return line.text;
}

// logKind is the event kind as shown; the class keeps the API's name.
function logKind(kind) {
  const key = "log.kind." + kind;
  const s = t(key);
  return s === key ? kind : s;
}

// fmtLogTime is a line's time, with its date when it is not today's.
function fmtLogTime(d) {
  const pad = (n) => String(n).padStart(2, "0");
  const time = pad(d.getHours()) + ":" + pad(d.getMinutes()) + ":" + pad(d.getSeconds());
  if (d.toDateString() === new Date().toDateString()) return time;
  return new Intl.DateTimeFormat(undefined, { day: "2-digit", month: "2-digit" }).format(d) + " " + time;
}

// logRangeStart is where a time range starts, seen from now; null for the
// count and for all.
function logRangeStart(range, now = new Date()) {
  switch (range) {
    case "10m":
      return new Date(now.getTime() - 10 * 60000);
    case "1h":
      return new Date(now.getTime() - 3600000);
    case "today":
      return new Date(now.getFullYear(), now.getMonth(), now.getDate());
    case "week": {
      const d = new Date(now.getFullYear(), now.getMonth(), now.getDate());
      d.setDate(d.getDate() - ((d.getDay() + 6) % 7));
      return d;
    }
    case "month":
      return new Date(now.getFullYear(), now.getMonth(), 1);
  }
  return null;
}

// logInQuery reports whether a line is of the kind and the device chosen,
// as the server answers them.
function logInQuery(line) {
  return (!logState.kind || line.kind === logState.kind) && (!logState.device || line.device === logState.device);
}

function logLineMatches(line) {
  if (line.kind === "error") return true; // the page's own line: the log could not load
  if (logState.kind && line.kind !== logState.kind) return false;
  if (logState.device && line.device !== logState.device) return false;
  const f = logState.filter.trim().toLowerCase();
  if (!f) return true;
  const hay = [fmtLogTime(line.time), line.kind, logKind(line.kind), line.device, logDeviceName(line.device), logText(line)].join(" ").toLowerCase();
  return hay.includes(f);
}

// renderLogLine is one line of the log, from its template
// (tpl-log-line): the time, the kind, the device by name and radio
// address when the line has one, and the text.
function renderLogLine(line) {
  const row = fromTemplate("tpl-log-line");
  row.className += " kind-" + line.kind;
  row.querySelector(".log-time").textContent = fmtLogTime(line.time);
  row.querySelector(".log-kind").textContent = logKind(line.kind);
  const dev = row.querySelector(".log-device");
  const rf = row.querySelector(".log-rf");
  if (line.device) {
    dev.textContent = logDeviceName(line.device);
    rf.textContent = line.device;
  } else {
    dev.remove();
    rf.remove();
  }
  row.querySelector(".log-text").textContent = logText(line);
  return row;
}

// renderLog draws the lines that match, newest first.
function renderLog() {
  const box = $("log-lines");
  clear(box);
  for (let i = logState.lines.length - 1; i >= 0; i--) {
    const line = logState.lines[i];
    if (logLineMatches(line)) box.appendChild(renderLogLine(line));
  }
  if (logState.stick) box.scrollTop = 0;
  renderOlder();
}

// renderOlder shows whether Older can page further back; the newest
// LOG_COUNT have no Older, a time range pages back within it.
function renderOlder() {
  const btn = $("log-older");
  const can = logState.more && logState.cursor !== null;
  btn.disabled = !can || logState.loading;
  btn.textContent = t(can ? "log.older" : "log.noOlder");
  btn.parentElement.classList.toggle("hidden", logState.range === LOG_COUNT_RANGE);
}

// logCursor is where the next older page starts: the oldest line's id,
// else its time as the server sent it (a Cube without a row in the
// store, whose log has no ids).
function logCursor() {
  const first = logState.lines[0];
  if (!first) return null;
  return first.id ? String(first.id) : first.at || null;
}

// logQuery is the query of a page: the size (LOG_COUNT for the count,
// else LOG_PAGE), the start of a time range, the kind and the device
// chosen, and where it starts.
function logQuery(before) {
  let q = "?limit=" + (logState.range === LOG_COUNT_RANGE ? LOG_COUNT : LOG_PAGE);
  const start = logRangeStart(logState.range);
  if (start) q += "&since=" + encodeURIComponent(start.toISOString());
  if (logState.kind) q += "&kind=" + encodeURIComponent(logState.kind);
  if (logState.device) q += "&device=" + encodeURIComponent(logState.device);
  if (before) q += "&before=" + encodeURIComponent(before);
  return q;
}

// appendLogEvent adds a live event; the view is updated in place when
// it is open, the line on top, and a view scrolled down keeps the lines
// it shows where they are. The oldest lines go, at the bottom: with the
// count, those past the newest LOG_COUNT of the kind and the device
// chosen; else those past the room: after Older, the room holds the pages
// fetched and LOG_MAX_LINES more, so that a live event does not take back
// a page just fetched.
function appendLogEvent(ev) {
  const line = logLine(ev);
  line.seq = ++logState.liveSeq;
  logState.lines.push(line);
  let cut = 0;
  if (logState.range === LOG_COUNT_RANGE) {
    for (let i = logState.lines.length - 1, n = 0; i >= 0; i--) {
      if (logInQuery(logState.lines[i]) && ++n > LOG_COUNT) {
        cut = i + 1;
        break;
      }
    }
  } else {
    const room = Math.max(logState.room, LOG_MAX_LINES);
    if (logState.lines.length > room) cut = logState.lines.length - room;
  }
  let gone = [];
  if (cut) {
    gone = logState.lines.splice(0, cut);
    logState.cursor = logCursor(); // Older goes on from the oldest line kept
    logState.more = logState.range !== LOG_COUNT_RANGE;
  }
  if (!logState.open) return;
  const box = $("log-lines");
  for (const old of gone) {
    if (logLineMatches(old) && box.lastChild) box.removeChild(box.lastChild); // the view drops them too
  }
  if (gone.length) renderOlder();
  if (logLineMatches(line)) {
    const height = box.scrollHeight;
    box.insertBefore(renderLogLine(line), box.firstChild);
    if (logState.stick) box.scrollTop = 0;
    else box.scrollTop += box.scrollHeight - height;
  }
}

// loadLog replaces the lines with the Cube's newest events of the kind
// and the device chosen; Older waits for it. An answer that comes after
// a Clear, another choice or a Cube switch is dropped. The live events
// that came while it waited follow the page, but for one the page holds
// already: the server may have stored it before it answered.
async function loadLog() {
  if (!state.serial) return;
  const gen = ++logState.gen;
  const since = logState.liveSeq;
  logState.loading = true;
  renderOlder();
  let evs = null;
  let failed = null;
  try {
    evs = (await api("GET", base() + "/log" + logQuery())) || [];
  } catch (e) {
    failed = e;
  }
  if (gen !== logState.gen) return;
  logState.loading = false;
  const live = logState.lines.filter((l) => l.seq > since);
  if (failed) {
    logState.lines = [{ id: 0, at: "", time: new Date(), kind: "error", device: "", state: null, text: t("log.cannotLoad", { error: failed.message }) }];
    logState.cursor = null;
    logState.more = false;
  } else {
    logState.lines = evs.map(logLine);
    logState.cursor = logCursor();
    logState.more = logState.range !== LOG_COUNT_RANGE && evs.length >= LOG_PAGE;
  }
  const page = logState.lines.slice();
  for (const l of live) {
    if (!page.some((x) => x.at === l.at && x.kind === l.kind && x.device === l.device)) logState.lines.push(l);
  }
  logState.room = LOG_MAX_LINES;
  logState.stick = true;
  renderLog();
}

// loadOlder puts the page before the oldest line under the lines, the
// view staying where it was; an answer that comes after a Clear, another
// choice or a Cube switch is dropped.
async function loadOlder() {
  if (logState.loading || !logState.more || logState.cursor === null || !state.serial) return;
  const gen = logState.gen;
  logState.loading = true;
  renderOlder();
  const box = $("log-lines");
  const top = box.scrollTop;
  let evs = null;
  let failed = null;
  try {
    evs = (await api("GET", base() + "/log" + logQuery(logState.cursor))) || [];
  } catch (e) {
    failed = e;
  }
  if (gen !== logState.gen) return;
  logState.loading = false;
  if (failed) {
    showMessage(t("log.cannotLoad", { error: failed.message }), false);
  } else {
    logState.lines = evs.map(logLine).concat(logState.lines);
    logState.more = evs.length >= LOG_PAGE;
    if (evs.length) logState.cursor = logCursor();
    logState.room = logState.lines.length + LOG_MAX_LINES;
  }
  const stick = logState.stick;
  logState.stick = false;
  renderLog();
  box.scrollTop = top; // the older lines came in below
  logState.stick = stick;
}

// pruneLogRange drops the lines a time range no longer holds, now or at
// the time given; the age timer of app.js calls it every minute. Every
// event of the range is then in the view, so Older has nothing more.
function pruneLogRange(now = new Date()) {
  const start = logRangeStart(logState.range, now);
  if (!start) return;
  const keep = logState.lines.findIndex((l) => l.kind === "error" || l.time >= start);
  const cut = keep === -1 ? logState.lines.length : keep;
  if (!cut) return;
  logState.lines.splice(0, cut);
  logState.cursor = logCursor();
  logState.more = false;
  if (logState.open) renderLog();
}

// renderLogClear turns Clear off while nothing narrows the log: no text
// filter, every kind and every device.
function renderLogClear() {
  $("log-clear").disabled = !(logState.filter || logState.kind || logState.device);
}

// clearLogFilter sets the text filter, the kind and the device back to
// all and loads the log again; a page on its way is dropped (loadLog).
function clearLogFilter() {
  logState.filter = "";
  logState.kind = "";
  logState.device = "";
  $("log-filter").value = "";
  $("log-kind").value = "";
  $("log-device").value = "";
  renderLogClear();
  loadLog();
}

// fillLogSelects fills the choice of kind, every kind the server sends,
// and of device, the Cube's devices by name; a device chosen that is gone
// falls back to all, and the answer is true then.
function fillLogSelects() {
  const kinds = fillOptions($("log-kind"), [["", t("log.allKinds")]].concat(LOG_KINDS.map((k) => [k, logKind(k)])));
  kinds.value = logState.kind;
  const devs = [];
  for (const r of state.rooms) for (const d of r.devices) devs.push(d);
  for (const d of state.unlisted) if (!devs.some((x) => x.rf_address === d.rf_address)) devs.push(d);
  devs.sort((a, b) => (a.name || a.rf_address).localeCompare(b.name || b.rf_address));
  fillOptions($("log-range"), LOG_RANGES.map((r) => [r, t("log.range." + r)])).value = logState.range;
  const sel = fillOptions($("log-device"), [["", t("log.allDevices")]].concat(devs.map((d) => [d.rf_address, d.name ? d.name + " · " + d.rf_address : d.rf_address])));
  const gone = logState.device !== "" && !devs.some((d) => d.rf_address === logState.device);
  if (gone) logState.device = "";
  sel.value = logState.device;
  renderLogClear();
  return gone;
}

// logCubeChanged starts the log over for another Cube: the device chosen
// back to all, the lines and the paging reset, an answer still on its way
// dropped, and the Cube's log loaded when the view is open.
function logCubeChanged() {
  logState.gen++;
  logState.device = "";
  logState.lines = [];
  logState.cursor = null;
  logState.more = false;
  logState.loading = false;
  logState.room = LOG_MAX_LINES;
  renderLogClear();
  if (!logState.open) return;
  $("log-device").value = "";
  renderLog();
  loadLog();
}

// The pages of the main area: Rooms ("tiles"), the log ("log"), My
// profile ("profile"), My settings ("prefs") and System settings
// ("system"), each with its section and, where it has one, its button in
// the header; My profile and My settings open from the 👤 menu, which is
// highlighted while one of them is shown, as ☰ is for Log and System
// settings, which it holds on a narrow screen. The tiles keep their state
// while hidden. The page shown is in the address (VIEW_HASH), Rooms
// without one, so that a reload opens it again and the browser's back
// and forward move between the pages.
const VIEW_SECTION = { log: "logview", profile: "profile", prefs: "prefs", system: "system" };
const VIEW_BUTTON = { tiles: "view-tiles", log: "view-log", system: "view-settings" };
const VIEW_HASH = { tiles: "", log: "#log", profile: "#profile", prefs: "#prefs", system: "#system" };
let currentView = "tiles";

// viewOfHash is the page an address names, Rooms for none or another.
function viewOfHash(hash) {
  return Object.keys(VIEW_HASH).find((v) => VIEW_HASH[v] && VIEW_HASH[v] === hash) || "tiles";
}

// setAddress puts a page's hash into the address: as a new entry of the
// browser's history ("push"), in place of the current one ("replace"),
// or not at all ("none", for a page shown from the address).
function setAddress(hash, how) {
  if (how === "none" || location.hash === hash) return;
  const url = location.pathname + location.search + hash;
  if (how === "replace") history.replaceState(null, "", url);
  else history.pushState(null, "", url);
}

// mayShow reports whether the signed-in user may open a page: System
// settings with one of its capabilities, My profile as a user.
function mayShow(view) {
  if (view === "system") return hasSystem(); // login.js
  if (view === "profile") return hasProfile();
  return view in VIEW_BUTTON || view in VIEW_SECTION;
}

// isShown reports whether a page is the one shown.
function isShown(view) {
  return currentView === view;
}

// showView shows a page, Rooms in place of one the user may not open,
// puts it into the address as address says (setAddress, a new entry by
// default; Rooms in place of a page refused), and tells the scripts that
// load what it shows (onView).
function showView(view, address = "push") {
  if (!mayShow(view)) {
    view = "tiles";
    address = "replace";
  }
  setAddress(VIEW_HASH[view], address);
  currentView = view;
  logState.open = view === "log";
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
  $("user-menu").querySelector("summary").classList.toggle("active", view === "profile" || view === "prefs");
  $("nav-menu").querySelector("summary").classList.toggle("active", view === "log" || view === "system");
  if (view === "log") {
    fillLogSelects();
    loadLog();
  }
  document.dispatchEvent(new CustomEvent("viewshown", { detail: view }));
}

// onView runs fn each time a page is shown, for a script that loads
// what the page shows.
function onView(view, fn) {
  document.addEventListener("viewshown", (ev) => {
    if (ev.detail === view) fn();
  });
}

function initLog() {
  $("view-tiles").addEventListener("click", () => showView("tiles"));
  $("view-log").addEventListener("click", () => showView("log"));
  $("view-settings").addEventListener("click", () => showView("system"));
  $("menu-profile").addEventListener("click", () => showView("profile"));
  $("menu-prefs").addEventListener("click", () => showView("prefs"));
  $("menu-log").addEventListener("click", () => showView("log"));
  $("menu-system").addEventListener("click", () => showView("system"));
  dropdown($("nav-menu"));
  // back and forward, and a page named in the address bar
  window.addEventListener("popstate", () => showView(viewOfHash(location.hash), "none"));
  dropdown($("user-menu"));
  $("log-filter").addEventListener("input", (ev) => {
    logState.filter = ev.target.value;
    renderLogClear();
    renderLog();
  });
  $("log-clear").addEventListener("click", clearLogFilter);
  $("log-kind").addEventListener("change", (ev) => {
    logState.kind = ev.target.value;
    renderLogClear();
    loadLog();
  });
  $("log-device").addEventListener("change", (ev) => {
    logState.device = ev.target.value;
    renderLogClear();
    loadLog();
  });
  $("log-range").addEventListener("change", (ev) => {
    logState.range = LOG_RANGES.includes(ev.target.value) ? ev.target.value : LOG_COUNT_RANGE;
    loadLog();
  });
  renderLogClear();
  $("log-older").addEventListener("click", loadOlder);
  const box = $("log-lines");
  box.addEventListener("scroll", () => {
    logState.stick = box.scrollTop <= 8;
  });
}
