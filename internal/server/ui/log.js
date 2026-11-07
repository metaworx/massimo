// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

// The Massimo web UI, log view: the selected Cube's events from
// GET …/log, a page at a time with "Older" back through the server's
// log, then the live events of the stream, one line each, with a text
// filter, a choice of kind and of device, and a clear that only empties
// the view. Uses the state, api and helpers of app.js and tiles.js.
"use strict";

const LOG_MAX_LINES = 2000;

// LOG_PAGE is how many events a page asks for; a shorter page is the
// log's beginning.
const LOG_PAGE = 200;

// LOG_KINDS are the kinds of event the server sends, in its order
// (eventKinds in internal/server/log.go, which TestLogKinds compares).
const LOG_KINDS = ["connected", "disconnected", "hello", "metadata", "config", "state", "pairing_started", "paired", "pairing_ended",
  "device_forgotten", "cube_found", "restore_step", "restore_done"];

const logState = {
  open: false,
  lines: [], // {id, time, at, kind, device, text}
  filter: "",
  kind: "", // the kind chosen, "" for all
  device: "", // the radio address chosen, "" for all
  cursor: null, // the oldest line's id, else its time as received: where Older pages from
  more: false, // the server may have older events
  loading: false, // a page is on its way
  gen: 0, // raised by a new log, Clear and a Cube switch: an answer of an older one is dropped
  room: 0, // the lines live events may grow the log to before the oldest go (LOG_MAX_LINES at least)
  stick: true, // auto-scroll to the bottom unless the user scrolled up
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

function fmtLogTime(d) {
  const pad = (n) => String(n).padStart(2, "0");
  return pad(d.getHours()) + ":" + pad(d.getMinutes()) + ":" + pad(d.getSeconds());
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

function renderLog() {
  const box = $("log-lines");
  clear(box);
  for (const line of logState.lines) {
    if (logLineMatches(line)) box.appendChild(renderLogLine(line));
  }
  if (logState.stick) box.scrollTop = box.scrollHeight;
  renderOlder();
}

// renderOlder shows whether Older can page further back.
function renderOlder() {
  const btn = $("log-older");
  const can = logState.more && logState.cursor !== null;
  btn.disabled = !can || logState.loading;
  btn.textContent = t(can ? "log.older" : "log.noOlder");
}

// logCursor is where the next older page starts: the oldest line's id,
// else its time as the server sent it (a Cube without a row in the
// store, whose log has no ids).
function logCursor() {
  const first = logState.lines[0];
  if (!first) return null;
  return first.id ? String(first.id) : first.at || null;
}

// logQuery is the query of a page: the size, the kind and the device
// chosen, and where it starts.
function logQuery(before) {
  let q = "?limit=" + LOG_PAGE;
  if (logState.kind) q += "&kind=" + encodeURIComponent(logState.kind);
  if (logState.device) q += "&device=" + encodeURIComponent(logState.device);
  if (before) q += "&before=" + encodeURIComponent(before);
  return q;
}

// appendLogEvent adds a live event; the view is updated in place when
// it is open. Past the room the oldest lines go: after Older, the room
// holds the pages fetched and LOG_MAX_LINES more, so that a live event
// does not take back a page just fetched.
function appendLogEvent(ev) {
  const line = logLine(ev);
  line.seq = ++logState.liveSeq;
  logState.lines.push(line);
  const room = Math.max(logState.room, LOG_MAX_LINES);
  let gone = [];
  if (logState.lines.length > room) {
    gone = logState.lines.splice(0, logState.lines.length - room);
    logState.cursor = logCursor(); // Older goes on from the oldest line kept
    logState.more = true;
  }
  if (!logState.open) return;
  const box = $("log-lines");
  for (const old of gone) {
    if (logLineMatches(old) && box.firstChild) box.removeChild(box.firstChild); // the view drops them too
  }
  if (gone.length) renderOlder();
  if (logLineMatches(line)) {
    box.appendChild(renderLogLine(line));
    if (logState.stick) box.scrollTop = box.scrollHeight;
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
    logState.more = evs.length >= LOG_PAGE;
  }
  const page = logState.lines.slice();
  for (const l of live) {
    if (!page.some((x) => x.at === l.at && x.kind === l.kind && x.device === l.device)) logState.lines.push(l);
  }
  logState.room = LOG_MAX_LINES;
  logState.stick = true;
  renderLog();
}

// loadOlder puts the page before the oldest line above the lines, the
// view staying where it was; an answer that comes after a Clear, another
// choice or a Cube switch is dropped.
async function loadOlder() {
  if (logState.loading || !logState.more || logState.cursor === null || !state.serial) return;
  const gen = logState.gen;
  logState.loading = true;
  renderOlder();
  const box = $("log-lines");
  const height = box.scrollHeight;
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
  box.scrollTop = top + (box.scrollHeight - height);
  logState.stick = stick && box.scrollTop + box.clientHeight >= box.scrollHeight - 8;
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
  const sel = fillOptions($("log-device"), [["", t("log.allDevices")]].concat(devs.map((d) => [d.rf_address, d.name ? d.name + " · " + d.rf_address : d.rf_address])));
  const gone = logState.device !== "" && !devs.some((d) => d.rf_address === logState.device);
  if (gone) logState.device = "";
  sel.value = logState.device;
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
  if (!logState.open) return;
  $("log-device").value = "";
  renderLog();
  loadLog();
}

// showView switches the main area between the three pages: the tiles
// ("tiles"), the log ("log") and the settings ("settings"). The tiles
// keep their state while hidden. The page is kept in memory only.
function showView(view) {
  logState.open = view === "log";
  for (const id of ["house", "rooms", "unlisted"]) {
    $(id).classList.toggle("view-hidden", view !== "tiles");
  }
  $("logview").classList.toggle("hidden", view !== "log");
  $("settings").classList.toggle("hidden", view !== "settings");
  const buttons = { tiles: "view-tiles", log: "view-log", settings: "view-settings" };
  for (const [name, id] of Object.entries(buttons)) {
    $(id).classList.toggle("active", view === name);
    if (view === name) $(id).setAttribute("aria-current", "page");
    else $(id).removeAttribute("aria-current");
  }
  if (view === "log") {
    fillLogSelects();
    loadLog();
  }
}

function initLog() {
  $("view-tiles").addEventListener("click", () => showView("tiles"));
  $("view-log").addEventListener("click", () => showView("log"));
  $("view-settings").addEventListener("click", () => showView("settings"));
  $("menu-prefs").addEventListener("click", () => showView("settings"));
  dropdown($("user-menu"));
  $("log-filter").addEventListener("input", (ev) => {
    logState.filter = ev.target.value;
    renderLog();
  });
  $("log-clear").addEventListener("click", () => {
    logState.gen++; // a page on its way is dropped
    logState.lines = [];
    logState.cursor = null;
    logState.more = false;
    logState.loading = false;
    logState.room = LOG_MAX_LINES;
    renderLog();
  });
  $("log-kind").addEventListener("change", (ev) => {
    logState.kind = ev.target.value;
    loadLog();
  });
  $("log-device").addEventListener("change", (ev) => {
    logState.device = ev.target.value;
    loadLog();
  });
  $("log-older").addEventListener("click", loadOlder);
  const box = $("log-lines");
  box.addEventListener("scroll", () => {
    logState.stick = box.scrollTop + box.clientHeight >= box.scrollHeight - 8;
  });
}
