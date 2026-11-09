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
  "device_forgotten", "cube_found", "restore_step", "restore_done",
  "cube_gone", "address_changed", "server_started", "server_stopping", "settings_read", "restart", "reboot", "reboot_cancelled",
  "update", "update_result", "backup", "backup_deleted", "backups_pruned", "log_pruned", "restore_started",
  "change"];

// LOG_RANGES are the choices of how far back the log goes: the newest
// LOG_COUNT events, the last 10 minutes, the last hour, today, this week
// (from Monday), this month, or all; LOG_COUNT_RANGE by default.
const LOG_RANGES = ["20", "10m", "1h", "today", "week", "month", "all"];
const LOG_COUNT_RANGE = "20";
const LOG_COUNT = 20;

const logState = {
  open: false,
  range: LOG_COUNT_RANGE, // one of LOG_RANGES
  lines: [], // {id, time, at, kind, device, serial, source, text}, oldest first; drawn newest first
  filter: "",
  kind: "", // the kind chosen, "" for all
  device: "", // the device chosen by its S/N (its radio address for one without), "" for all
  cursor: null, // the oldest line's id, else its time as received: where Older pages from
  more: false, // the server may have older events
  loading: false, // a page is on its way
  gen: 0, // raised by a new log, Clear and a Cube switch: an answer of an older one is dropped
  room: 0, // the lines live events may grow the log to before the oldest go (LOG_MAX_LINES at least)
  stick: true, // keep the newest line in view at the top unless the user scrolled down
  liveSeq: 0, // counts the live lines, so that loadLog finds the ones that came while it waited
  level: "info", // the lowest level shown, one of LOG_LEVELS
  source: "", // the source chosen, one of LOG_SOURCES, "" for all
  user: "", // the user chosen by login name, "" for everyone
  users: null, // the users of the who choice, loaded once the log opens (logLoadUsers)
};

// LOG_SOURCES are the sources of an event, as the server names them, in
// the order of the source choice.
const LOG_SOURCES = ["system", "config", "cube", "device"];

// LOG_KIND_SOURCE groups the kinds by the source they come from, for the
// kind choice; a kind of more than one (config) goes with the devices.
const LOG_KIND_SOURCE = {
  state: "device", config: "device", paired: "device", device_forgotten: "device",
  connected: "cube", disconnected: "cube", hello: "cube", metadata: "cube", pairing_started: "cube", pairing_ended: "cube",
  cube_found: "cube", cube_gone: "cube", address_changed: "cube",
  change: "config",
};

// LOG_LEVELS are the levels of an event, lowest first, as the server
// names them.
const LOG_LEVELS = ["debug", "info", "warn", "error"];

// logAtLevel reports whether a line is at the level chosen or above; a
// line of before the levels counts as info.
function logAtLevel(line) {
  return LOG_LEVELS.indexOf(line.level || "info") >= LOG_LEVELS.indexOf(logState.level);
}

// logOfChoices reports whether a line is of the source and the user
// chosen.
function logOfChoices(line) {
  return (!logState.source || line.source === logState.source) && (!logState.user || (line.by && line.by.name === logState.user));
}

// logLoadUsers fills the who choice once, for a caller who may list the
// users; for others, and on a server without users, it stays hidden.
async function logLoadUsers() {
  if (logState.users !== null) return;
  logState.users = [];
  if (!state.me || state.me.mode !== "users" || !hasCap("users")) return;
  try {
    logState.users = (await api("GET", "api/v1/users")) || [];
  } catch (e) {
    logState.users = [];
  }
  fillLogSelects();
}

// logDevice finds a device of the rooms or of the unlisted list already
// loaded by its radio address; null when the Cube has none.
function logDevice(rf) {
  for (const r of state.rooms) {
    const d = r.devices.find((x) => x.rf_address === rf);
    if (d) return d;
  }
  return state.unlisted.find((x) => x.rf_address === rf) || null;
}

// logDeviceKey is how the log names a device in its choice and its
// address: by its S/N, by its radio address only when it has none.
function logDeviceKey(d) {
  return d.serial || d.rf_address;
}

// logDeviceName is how a line names its device: by its name from the
// rooms and the unlisted list already loaded, else by its S/N, the radio
// address only for a device without either.
function logDeviceName(line) {
  if (!line.device) return "";
  const d = logDevice(line.device);
  if (d && d.name) return d.name;
  return line.serial || (d && d.serial) || line.device;
}

// logOfDevice reports whether a line is of the device chosen, named by
// its S/N or its radio address; a line stored before events carried the
// S/N is the chosen device's by its radio address.
function logOfDevice(line, key) {
  if (!key || line.serial === key || line.device === key) return true;
  for (const r of state.rooms) if (r.devices.some((d) => d.serial === key && d.rf_address === line.device)) return true;
  return state.unlisted.some((d) => d.serial === key && d.rf_address === line.device);
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

// logListText says what changed in the room and device list, one phrase
// per change (AP Logging v1.1, Block 4): "Room “Bath” renamed
// “Bathroom”", "MEQ0000101 “Window” moved from Bath to Kitchen".
function logListText(changes) {
  return changes
    .map((c) => {
      const key = "log.list." + c.what;
      const s = t(key, { name: c.name || "", from: c.from || "–", to: c.to || "–", device: c.serial || c.device || "" });
      return s === key ? c.what : s;
    })
    .join("; ");
}

// logValue shows a value of a configuration: a temperature with one
// digit, a switch as on or off, a time zone by its name, a day by its
// name.
function logValue(key, v) {
  if (v === null || v === undefined || v === "") return "–";
  if (typeof v === "boolean") return t(v ? "log.on" : "log.off");
  if (typeof v === "number") return key.endsWith("_temp") || key.endsWith("_setpoint") || key === "temp_offset" ? fmtNum(v, 1) : String(v);
  if (typeof v === "object" && v.name !== undefined) return v.name || "–";
  if (typeof v === "string" && t("day." + v) !== "day." + v) return t("day." + v);
  return typeof v === "string" ? v : JSON.stringify(v);
}

// logConfigText says what changed in a device's configuration, one
// phrase per value: "comfort 21.0 → 21.5", "Monday's program changed".
function logConfigText(changes) {
  return Object.keys(changes)
    .sort()
    .map((k) => {
      if (k.startsWith("program_")) return t("log.cfg.program", { day: t("day." + k.slice(8)) });
      const key = "log.cfg." + k;
      const label = t(key) === key ? k : t(key);
      const c = changes[k] || {};
      return t("log.cfg.change", { label, from: logValue(k, c.from), to: logValue(k, c.to) });
    })
    .join(", ");
}

// logSummaryText is what a line says of itself where it names no
// differences: a connection's devices, rooms, firmware, budget and how
// long the Cube was lost; a hello's budget; a list's rooms and devices; a
// configuration's main values.
function logSummaryText(kind, s) {
  const num = (v) => (v === undefined || v === null ? "–" : String(v));
  switch (kind) {
    case "connected": {
      let text = t("log.sum.connected", { devices: num(s.devices), rooms: num(s.rooms), firmware: s.firmware || "–", duty: num(s.duty_cycle_pct), slots: num(s.free_memory_slots) });
      const down = s.down_for_s;
      if (down) text += "; " + (down >= 60 ? t("log.sum.downForMin", { n: Math.round(down / 60) }) : t("log.sum.downForSec", { n: down }));
      return text;
    }
    case "hello":
      return t("log.sum.hello", { firmware: s.firmware || "–", duty: num(s.duty_cycle_pct), slots: num(s.free_memory_slots) });
    case "metadata":
      return t("log.sum.metadata", { rooms: num(s.rooms), devices: num(s.devices) });
    case "config":
      if (s.portal_enabled !== undefined) return t("log.sum.cubeConfig", { portal: t(s.portal_enabled ? "log.on" : "log.off"), winter: s.tz_winter || "–", summer: s.tz_summer || "–" });
      if (s.comfort_temp !== undefined) return t("log.sum.config", { comfort: fmtNum(s.comfort_temp, 1), eco: fmtNum(s.eco_temp, 1), offset: fmtNum(s.temp_offset, 1) });
      return "";
    case "server_started":
      return s.before ? t("log.sum.startedFrom", { version: s.version || "–", before: s.before }) : t("log.sum.started", { version: s.version || "–" });
    case "server_stopping":
      return t("log.sum.stopping", { why: logWord("log.why.", s.why) });
    case "settings_read": {
      const why = logWord("log.readWhy.", { "the settings file changed": "file", SIGHUP: "sighup", "the API": "api" }[s.why] || s.why);
      const changed = (s.changed || []).join(", ");
      const pending = (s.at_next_start || []).join(", ");
      if (!changed && !pending) return t("log.sum.readNothing", { why });
      return t("log.sum.read", { why, changed: changed || "–", pending: pending || "–" });
    }
    case "restart":
      return t("log.sum.restart");
    case "reboot":
      return t("log.sum.reboot");
    case "reboot_cancelled":
      return t(s.why ? "log.sum.rebootOff" : "log.sum.rebootCancelled");
    case "update":
      return t("log.sum.update", { from: s.from || "–", to: s.to || "–" });
    case "update_result":
      return t("log.sum.updateResult", { from: s.from || "–", to: s.to || "–", state: logWord("log.updateState.", s.state) });
    case "backup":
      return s.id ? t("log.sum.backup", { id: s.id, reason: logWord("log.reason.", s.reason) }) : t("log.sum.backupFailed", { reason: logWord("log.reason.", s.reason) });
    case "backup_deleted":
      return t("log.sum.backupDeleted", { id: s.id });
    case "backups_pruned":
      return t("log.sum.backupsPruned", { n: s.removed });
    case "log_pruned":
      return t("log.sum.logPruned", { n: s.removed, days: s.keep_days });
    case "restore_started":
      return t("log.sum.restoreStarted", { steps: num(s.steps) });
    case "address_changed":
      return t("log.sum.addressChanged", { from: s.from || "–", to: s.to || "–" });
    case "change":
      return logChangeText(s);
  }
  return "";
}

// logTargetText names what a change concerns: a device by its name, else
// its S/N; a room by its name; a user, role or key as the line names it.
function logTargetText(s) {
  const target = s.target || "";
  const i = target.indexOf(":");
  if (i < 0) return target;
  const kind = target.slice(0, i);
  const id = target.slice(i + 1);
  switch (kind) {
    case "device": {
      for (const r of state.rooms) {
        const d = r.devices.find((x) => x.serial === id || x.rf_address === id);
        if (d && d.name) return d.name;
      }
      return id;
    }
    case "room": {
      const r = state.rooms.find((x) => String(x.id) === id);
      return r && r.name ? r.name : t("log.room", { id });
    }
    case "token":
      return s.subject && s.subject.name ? s.subject.name : id;
    case "user":
      return s.subject && s.subject.display_name ? s.subject.display_name : id;
  }
  return id;
}

// logChangeText is a change line (AP Logging v1.1, Block 6): what was
// done, to what, and what changed from and to, else what it set.
function logChangeText(s) {
  const key = "log.action." + s.action;
  const action = t(key) === key ? s.action : t(key);
  const target = logTargetText(s);
  let text = target ? t("log.sum.change", { action, target }) : action;
  const changes = s.changes && Object.keys(s.changes).length ? s.changes : null;
  if (changes) {
    text += ": " + logConfigText(changes);
  } else if (s.set && typeof s.set === "object") {
    const parts = Object.keys(s.set)
      .sort()
      .filter((k) => s.set[k] !== "…" && typeof s.set[k] !== "object")
      .map((k) => {
        const label = t("log.cfg." + k) === "log.cfg." + k ? k : t("log.cfg." + k);
        return label + " " + logValue(k, s.set[k]);
      });
    if (parts.length) text += ": " + parts.join(", ");
  }
  return s.via === "cli" ? t("log.viaCli", { text }) : text;
}

// logWord is a value's word in the language shown, the value itself where
// the page has none.
function logWord(prefix, v) {
  const key = prefix + v;
  const s = t(key);
  return s === key ? String(v) : s;
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
    serial: ev.serial || (ev.state && ev.state.serial) || "",
    source: ev.source || "",
    state: ev.kind === "state" ? ev.state : null,
    gone: ev.kind === "device_forgotten" && ev.state ? ev.state : null,
    level: ev.level || "info",
    listChanges: ev.kind === "metadata" && ev.list_changes && ev.list_changes.length ? ev.list_changes : null,
    configChanges: ev.kind === "config" && ev.changes && Object.keys(ev.changes).length ? ev.changes : null,
    summary: ev.summary || null,
    missed: !!ev.while_disconnected,
    changes: ev.kind === "state" && ev.changes && Object.keys(ev.changes).length ? ev.changes : null,
    cube: ev.kind === "cube_found" || ev.kind === "cube_gone" ? ev.cube : null,
    by: ev.by || null,
    error: ev.error || "",
    text: ev.error || "",
  };
}

function logText(line) {
  let text = logOwnText(line);
  if (line.error && text !== line.error) text = text ? text + ": " + line.error : line.error;
  if (line.missed && text) text = t("log.whileDisconnected", { text });
  if (line.by) text = t("log.by", { text, name: line.by.display_name || line.by.name });
  return text;
}

// logOwnText is a line's text without the note of when it changed.
function logOwnText(line) {
  if (line.gone) {
    // The device as it was: the Cube has it no longer.
    const vars = { type: typeName(line.gone.type), serial: line.serial || line.device, name: line.gone.name };
    return t(line.gone.name ? "log.removedNamed" : "log.removed", vars);
  }
  if (line.changes) return logChangesText(line.changes);
  if (line.configChanges) return logConfigText(line.configChanges);
  if (line.listChanges) return logListText(line.listChanges);
  if (line.summary) {
    const s = logSummaryText(line.kind, line.summary);
    if (s) return s;
  }
  if (line.state) return logStateText(line.state);
  if (line.cube) return t(line.kind === "cube_gone" ? "log.sum.cubeGone" : "discover.log", { serial: line.cube.serial, address: line.cube.address || "–" });
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
  return (!logState.kind || line.kind === logState.kind) && logOfDevice(line, logState.device) && logAtLevel(line) && logOfChoices(line);
}

function logLineMatches(line) {
  if (line.kind === "error") return true; // the page's own line: the log could not load
  if (logState.kind && line.kind !== logState.kind) return false;
  if (!logOfDevice(line, logState.device) || !logAtLevel(line) || !logOfChoices(line)) return false;
  const f = logState.filter.trim().toLowerCase();
  if (!f) return true;
  const hay = [fmtLogTime(line.time), line.kind, logKind(line.kind), line.device, line.serial, logDeviceName(line), logText(line)].join(" ").toLowerCase();
  return hay.includes(f);
}

// renderLogLine is one line of the log, from its template
// (tpl-log-line): the time, the kind, the device when the line has one,
// by its name, else its S/N, with its radio address in expert mode, and
// the text. The Cube's own configuration is the Cube's, by its S/N; a
// removed device is named by the text alone.
function renderLogLine(line) {
  const row = fromTemplate("tpl-log-line");
  row.className += " kind-" + line.kind;
  row.querySelector(".log-time").textContent = fmtLogTime(line.time);
  row.querySelector(".log-kind").textContent = logKind(line.kind);
  const dev = row.querySelector(".log-device");
  const rf = row.querySelector(".log-rf");
  if (line.device) {
    rf.textContent = line.device;
    if (line.gone) dev.remove();
    else dev.textContent = line.source === "cube" ? t("log.cubeDevice", { serial: line.serial || line.device }) : logDeviceName(line);
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
  if (logState.source) q += "&source=" + logState.source;
  if (logState.device) q += "&device=" + encodeURIComponent(logState.device);
  if (logState.level !== "info") q += "&level=" + logState.level;
  if (logState.user) q += "&user=" + encodeURIComponent(logState.user);
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
  $("log-clear").disabled = !(logState.filter || logState.kind || logState.device || logState.source || logState.user || logState.level !== "info");
}

// logChoicesChanged follows a new range, kind or device: the address in
// place of the current entry while the log is shown (logHash), Clear,
// and the log loaded again; a page on its way is dropped (loadLog).
function logChoicesChanged() {
  if (logState.open) setAddress(logHash(), "replace");
  renderLogClear();
  loadLog();
}

// clearLogFilter sets the text filter, the kind, the source or device,
// the level and who back to all and loads the log again.
function clearLogFilter() {
  logState.filter = "";
  logState.kind = "";
  logState.device = "";
  logState.source = "";
  logState.user = "";
  logState.level = "info";
  $("log-filter").value = "";
  $("log-kind").value = "";
  $("log-device").value = "";
  $("log-level").value = "info";
  $("log-user").value = "";
  logChoicesChanged();
}

// fillLogSelects fills the choice of kind, every kind the server sends,
// and of device, the Cube's devices by name and S/N; a device chosen that
// is gone falls back to all, one chosen by its radio address takes its
// S/N, and the answer is true when the choice changed so. Until the
// Cube's devices are loaded (state.roomsOf), a device chosen from the
// address stays, shown as the address names it.
function fillLogSelects() {
  const groups = LOG_SOURCES.map((src) => ({ group: t("log.src." + src), options: LOG_KINDS.filter((k) => (LOG_KIND_SOURCE[k] || "system") === src).map((k) => [k, logKind(k)]) }));
  const kinds = fillOptions($("log-kind"), [["", t("log.allKinds")]].concat(groups));
  kinds.value = logState.kind;
  fillOptions($("log-level"), LOG_LEVELS.filter((l) => l !== "error").map((l) => [l, t("log.level." + l)])).value = logState.level;
  const users = logState.users || [];
  const who = fillOptions($("log-user"), [["", t("log.allUsers")]].concat(users.map((u) => [u.name, u.display_name || u.name])));
  if (logState.user && !users.some((u) => u.name === logState.user)) who.appendChild(new Option(logState.user, logState.user));
  who.value = logState.user;
  who.classList.toggle("hidden", users.length === 0 && !logState.user);
  const devs = [];
  for (const r of state.rooms) for (const d of r.devices) devs.push(d);
  for (const d of state.unlisted) if (!devs.some((x) => x.rf_address === d.rf_address)) devs.push(d);
  devs.sort((a, b) => (a.name || logDeviceKey(a)).localeCompare(b.name || logDeviceKey(b)));
  fillOptions($("log-range"), LOG_RANGES.map((r) => [r, t("log.range." + r)])).value = logState.range;
  const options = [["", t("log.allDevices")]]
    .concat(LOG_SOURCES.map((src) => ["src:" + src, t("log.src." + src)]))
    .concat(devs.length ? [{ group: t("log.devicesGroup"), options: devs.map((d) => [logDeviceKey(d), d.name ? d.name + " · " + logDeviceKey(d) : logDeviceKey(d)]) }] : []);
  const byRF = devs.find((d) => d.rf_address === logState.device && d.serial);
  let changed = false;
  if (byRF) {
    logState.device = byRF.serial; // an old link's radio address
    changed = true;
  }
  const unknown = logState.device !== "" && !devs.some((d) => logDeviceKey(d) === logState.device);
  const known = !!state.serial && state.roomsOf === state.serial;
  if (unknown && !known) options.push([logState.device, logState.device]);
  const sel = fillOptions($("log-device"), options);
  if (unknown && known) {
    logState.device = "";
    changed = true;
  }
  sel.value = logState.device || (logState.source ? "src:" + logState.source : "");
  renderLogClear();
  return changed;
}

// logCubeChanged starts the log over for another Cube: the device chosen
// back to all, the lines and the paging reset, an answer still on its way
// dropped, and, when the view is open, the address without the device and
// the Cube's log loaded.
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
  setAddress(logHash(), "replace");
  renderLog();
  loadLog();
}

// The pages of the main area: Rooms ("tiles"), the log ("log"), My
// profile ("profile"), My settings ("prefs"), Notifications ("notify")
// and System settings ("system"), each with its section and, where it
// has one, its button in the header; My profile, My settings and
// Notifications open from the 👤 menu, which is highlighted while one of
// them is shown, as ☰ is for Log and System
// settings, which it holds on a narrow screen. The tiles keep their state
// while hidden. The page shown is in the address (VIEW_HASH), Rooms
// without one, and the log's range, kind and device after its page
// (logHash), so that a reload or a link opens it again as chosen and the
// browser's back and forward move between the pages.
const VIEW_SECTION = { log: "logview", profile: "profile", prefs: "prefs", notify: "notify", system: "system" };
const VIEW_BUTTON = { tiles: "view-tiles", log: "view-log", system: "view-settings" };
const VIEW_HASH = { tiles: "", log: "#log", profile: "#profile", prefs: "#prefs", notify: "#notify", system: "#system" };
let currentView = "tiles";

// viewOfHash is the page an address names, the part before its "?",
// Rooms for none or another.
function viewOfHash(hash) {
  const page = hash.split("?")[0];
  return Object.keys(VIEW_HASH).find((v) => VIEW_HASH[v] && VIEW_HASH[v] === page) || "tiles";
}

// logHash is the log's address: its page, then the range, the kind and
// the device chosen (by its S/N), in that order, each left out at its
// default, so that a plain #log is the last 20 events of every kind and
// device.
function logHash() {
  const q = new URLSearchParams();
  if (logState.range !== LOG_COUNT_RANGE) q.set("range", logState.range);
  if (logState.kind) q.set("kind", logState.kind);
  if (logState.source) q.set("source", logState.source);
  if (logState.device) q.set("device", logState.device);
  if (logState.level !== "info") q.set("level", logState.level);
  if (logState.user) q.set("user", logState.user);
  const s = q.toString();
  return VIEW_HASH.log + (s ? "?" + s : "");
}

// readLogHash takes the log's range, kind and device from an address of
// the log, each at its default where it is missing or not one of its
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
  showView(view, "replace");
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
// default; Rooms in place of a page refused), the log with its choices
// (logHash), and tells the scripts that load what it shows (onView).
function showView(view, address = "push") {
  if (!mayShow(view)) {
    view = "tiles";
    address = "replace";
  }
  if (view === "log") fillLogSelects(); // a device the Cube does not have goes before the address takes it
  setAddress(view === "log" ? logHash() : VIEW_HASH[view], address);
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
  $("user-menu").querySelector("summary").classList.toggle("active", view === "profile" || view === "prefs" || view === "notify");
  $("nav-menu").querySelector("summary").classList.toggle("active", view === "log" || view === "system");
  if (view === "log") {
    loadLog();
    logLoadUsers();
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
  window.addEventListener("popstate", () => showAddress());
  dropdown($("user-menu"));
  $("log-filter").addEventListener("input", (ev) => {
    logState.filter = ev.target.value;
    renderLogClear();
    renderLog();
  });
  $("log-clear").addEventListener("click", clearLogFilter);
  $("log-kind").addEventListener("change", (ev) => {
    logState.kind = ev.target.value;
    logChoicesChanged();
  });
  $("log-device").addEventListener("change", (ev) => {
    const v = ev.target.value;
    logState.source = v.startsWith("src:") ? v.slice(4) : "";
    logState.device = v.startsWith("src:") ? "" : v;
    logChoicesChanged();
  });
  $("log-level").addEventListener("change", (ev) => {
    logState.level = ["debug", "info", "warn"].includes(ev.target.value) ? ev.target.value : "info";
    logChoicesChanged();
  });
  $("log-user").addEventListener("change", (ev) => {
    logState.user = ev.target.value;
    logChoicesChanged();
  });
  $("log-range").addEventListener("change", (ev) => {
    logState.range = LOG_RANGES.includes(ev.target.value) ? ev.target.value : LOG_COUNT_RANGE;
    logChoicesChanged();
  });
  renderLogClear();
  $("log-older").addEventListener("click", loadOlder);
  const box = $("log-lines");
  box.addEventListener("scroll", () => {
    logState.stick = box.scrollTop <= 8;
  });
}
