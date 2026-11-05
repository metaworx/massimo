// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

// The Massimo web UI, log view: the selected Cube's recent events from
// GET …/log, then the live events of the stream, one line each, with a
// filter and a clear that only empties the view. Uses the state, api
// and helpers of app.js and tiles.js.
"use strict";

const LOG_MAX_LINES = 2000;

const logState = {
  open: false,
  lines: [], // {time, kind, device, text}
  filter: "",
  stick: true, // auto-scroll to the bottom unless the user scrolled up
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

// logLine turns an event object into a line of the view; the text of a
// state event is made when the line is drawn, in the current language.
function logLine(ev) {
  const time = ev.time ? new Date(ev.time) : new Date();
  return { time, kind: ev.kind || "", device: ev.device || "", state: ev.kind === "state" ? ev.state : null, text: ev.error || "" };
}

function logText(line) {
  return line.state ? logStateText(line.state) : line.text;
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
  const f = logState.filter.trim().toLowerCase();
  if (!f) return true;
  const hay = [fmtLogTime(line.time), line.kind, logKind(line.kind), line.device, logDeviceName(line.device), logText(line)].join(" ").toLowerCase();
  return hay.includes(f);
}

function renderLogLine(line) {
  const row = document.createElement("div");
  row.className = "log-line kind-" + line.kind;
  row.appendChild(textEl("span", fmtLogTime(line.time), "log-time mono"));
  row.appendChild(textEl("span", logKind(line.kind), "log-kind"));
  if (line.device) {
    row.appendChild(textEl("span", logDeviceName(line.device), "log-device"));
    row.appendChild(textEl("span", line.device, "mono expert-only"));
  }
  row.appendChild(textEl("span", logText(line), "log-text"));
  return row;
}

function renderLog() {
  const box = $("log-lines");
  clear(box);
  for (const line of logState.lines) {
    if (logLineMatches(line)) box.appendChild(renderLogLine(line));
  }
  if (logState.stick) box.scrollTop = box.scrollHeight;
}

// appendLogEvent adds a live event; the view is updated in place when
// it is open.
function appendLogEvent(ev) {
  const line = logLine(ev);
  logState.lines.push(line);
  if (logState.lines.length > LOG_MAX_LINES) logState.lines.splice(0, logState.lines.length - LOG_MAX_LINES);
  if (!logState.open) return;
  if (logLineMatches(line)) {
    const box = $("log-lines");
    box.appendChild(renderLogLine(line));
    if (logState.stick) box.scrollTop = box.scrollHeight;
  }
}

// loadLog replaces the lines with the Cube's recent events.
async function loadLog() {
  if (!state.serial) return;
  try {
    const evs = (await api("GET", base() + "/log?limit=200")) || [];
    logState.lines = evs.map(logLine);
  } catch (e) {
    logState.lines = [{ time: new Date(), kind: "error", device: "", state: null, text: t("log.cannotLoad", { error: e.message }) }];
  }
  logState.stick = true;
  renderLog();
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
  if (view === "log") loadLog();
}

function initLog() {
  $("view-tiles").addEventListener("click", () => showView("tiles"));
  $("view-log").addEventListener("click", () => showView("log"));
  $("view-settings").addEventListener("click", () => showView("settings"));
  $("log-filter").addEventListener("input", (ev) => {
    logState.filter = ev.target.value;
    renderLog();
  });
  $("log-clear").addEventListener("click", () => {
    logState.lines = [];
    renderLog();
  });
  const box = $("log-lines");
  box.addEventListener("scroll", () => {
    logState.stick = box.scrollTop + box.clientHeight >= box.scrollHeight - 8;
  });
}
