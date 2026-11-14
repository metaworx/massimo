// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

// The Massimo web UI, log view: the selected Cube's events from
// GET …/log, or every Cube's from GET /api/v1/log under All Cubes,
// newest first, a page at a time with "Older" at the bottom back through
// the server's log, the live events of the stream coming in at the top
// (under All Cubes those a poll brings), one line each, with a text
// filter, a choice of Cubes, kind, device and how far back (the range),
// and Clear, which sets the choices back but the range.
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
  "update", "update_result", "backup", "backup_deleted", "backups_pruned", "log_pruned", "log_rotated", "restore_started",
  "change", "command", "hold", "login", "login_failed", "locked_out", "audit_pruned",
  "proxy_connected", "proxy_disconnected", "proxy_refused", "proxy_discarded", "proxy_deleted", "proxy_delete_refused", "proxy_pairing", "proxy_warning"];

// LOG_RANGES are the choices of how far back the log goes: the newest
// LOG_COUNT events, the last 10 minutes, the last hour, today, this week
// (from Monday), this month, or all; LOG_COUNT_RANGE by default.
const LOG_RANGES = ["20", "10m", "1h", "today", "week", "month", "all"];
const LOG_COUNT_RANGE = "20";
const LOG_COUNT = 20;

// LOG_POLL_MS is how often the log asks for new lines under All Cubes,
// where the stream, which is the header's Cube's, brings none (D9-A).
const LOG_POLL_MS = 10000;

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
  openCommands: new Set(), // the commands whose detail is open, by number
  commands: new Map(), // a command's detail as GET /api/v1/commands/{id} answered it, or {error}, by number
  cubes: "", // "" for the header's Cube, "all" for every Cube (GET /api/v1/log)
  cubeRooms: new Map(), // under All Cubes, each Cube's {rooms, unlisted} by S/N, null while asked or not answered
  cubesSeen: "", // the Cubes the choices were last filled for (logCubesPolled)
  pollTimer: null, // the poll of new lines under All Cubes (logPollSet)
  polling: false, // a poll is on its way
};

// LOG_SOURCES are the sources of an event, as the server names them, in
// the order of the source choice.
const LOG_SOURCES = ["system", "config", "cube", "device", "queue", "server", "proxy"];

// LOG_KIND_SOURCE groups the kinds by the source they come from, for the
// kind choice; a kind of more than one (config) goes with the devices.
const LOG_KIND_SOURCE = {
  state: "device", config: "device", paired: "device", device_forgotten: "device",
  connected: "cube", disconnected: "cube", hello: "cube", metadata: "cube", pairing_started: "cube", pairing_ended: "cube",
  cube_found: "cube", cube_gone: "cube", address_changed: "cube",
  change: "config",
  command: "queue", hold: "queue",
  login: "server", login_failed: "server", locked_out: "server",
  proxy_connected: "proxy", proxy_disconnected: "proxy", proxy_refused: "proxy", proxy_discarded: "proxy",
  proxy_deleted: "proxy", proxy_delete_refused: "proxy", proxy_pairing: "proxy", proxy_warning: "proxy",
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

// logRoomsOf are the rooms and the unlisted list loaded of a Cube, by its
// S/N, the header's Cube for none: the page's own where they are that
// Cube's, else those All Cubes loaded (logLoadCubeRooms); null while
// there are none, as while a Cube switch has not brought the new Cube's,
// since a radio address names a device on its own Cube only.
function logRoomsOf(serial) {
  const sn = serial || state.serial;
  if (sn && sn === state.serial && state.roomsOf === sn) return { rooms: state.rooms, unlisted: state.unlisted };
  return logState.cubeRooms.get(sn) || null;
}

// logDevicesOf are the devices of a Cube's rooms and unlisted list, each
// once, by name, else by S/N.
function logDevicesOf(rs) {
  const devs = [];
  if (!rs) return devs;
  for (const r of rs.rooms) for (const d of r.devices) devs.push(d);
  for (const d of rs.unlisted) if (!devs.some((x) => x.rf_address === d.rf_address)) devs.push(d);
  return devs.sort((a, b) => (a.name || logDeviceKey(a)).localeCompare(b.name || logDeviceKey(b)));
}

// logDevice finds a device of a Cube (logRoomsOf) by its radio address;
// null when the Cube has none, or nothing of the Cube is loaded.
function logDevice(rf, serial) {
  const rs = logRoomsOf(serial);
  if (!rs) return null;
  for (const r of rs.rooms) {
    const d = r.devices.find((x) => x.rf_address === rf);
    if (d) return d;
  }
  return rs.unlisted.find((x) => x.rf_address === rf) || null;
}

// logCubeName names a Cube by its name, else its S/N.
function logCubeName(serial) {
  const c = state.cubes.find((x) => x.serial === serial);
  return c && c.name ? c.name : serial;
}

// logLineCube is the name of a line's Cube where the log shows every
// Cube's lines (D2); "" under This Cube and for the server's own lines.
function logLineCube(line) {
  return logState.cubes === "all" && line.cubeSerial ? logCubeName(line.cubeSerial) : "";
}

// logNamePath joins a name to what holds it: "Kitchen › Window".
function logNamePath(outer, inner) {
  return t("log.devicePath", { room: outer, device: inner });
}

// logDeviceKey is how the log names a device in its choice and its
// address: by its S/N, by its radio address only when it has none.
function logDeviceKey(d) {
  return d.serial || d.rf_address;
}

// logDeviceName is how a line names its device (AP LogCubes v1.0, D1-A,
// D3-A, D4-A): by its room and name from the rooms and the unlisted list
// already loaded, "Kitchen › Window"; by the name the line kept (a state
// line's) where those give none, until the rooms come and for a device
// the Cube has no longer; by its type where no name is known, after its
// room; by its S/N where not even the type is, the radio address only for
// a device without one. The S/N is then in the tooltip and, beside the
// radio address, in expert mode (renderLogLine).
function logDeviceName(line) {
  if (!line.device) return "";
  const d = logDevice(line.device, line.cubeSerial);
  const kept = line.state || {};
  const type = (d && d.type) || kept.type;
  const device = (d && d.name) || kept.name || (type ? upperFirst(typeName(type)) : "");
  if (!device) return logDeviceSerial(line) || line.device;
  const room = d ? logDeviceRoom(d, line.cubeSerial) : "";
  return room ? logNamePath(room, device) : device;
}

// logDeviceSerial is the S/N of a line's device: the line's, else the one
// the rooms loaded give its radio address; "" for neither.
function logDeviceSerial(line) {
  if (line.serial) return line.serial;
  const d = line.device ? logDevice(line.device, line.cubeSerial) : null;
  return (d && d.serial) || "";
}

// logDeviceRoom is the name of the room a device of a Cube's rooms loaded
// is in; "" for a device of the unlisted list.
function logDeviceRoom(d, serial) {
  const rs = logRoomsOf(serial);
  const r = rs && rs.rooms.find((x) => x.devices.includes(d));
  if (!r) return "";
  return r.name || t("log.room", { id: r.id });
}

// upperFirst is a text with its first letter in upper case, for a word
// that stands in for a name ("radiator thermostat").
function upperFirst(s) {
  return s ? s.charAt(0).toUpperCase() + s.slice(1) : s;
}

// logOfDevice reports whether a line is of the device chosen, named by
// its S/N or its radio address; a line stored before events carried the
// S/N is the chosen device's by its radio address on the line's Cube.
function logOfDevice(line, key) {
  if (!key || line.serial === key || line.device === key) return true;
  const rs = logRoomsOf(line.cubeSerial);
  if (!rs) return false;
  for (const r of rs.rooms) if (r.devices.some((d) => d.serial === key && d.rf_address === line.device)) return true;
  return rs.unlisted.some((d) => d.serial === key && d.rf_address === line.device);
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
function logSummaryText(kind, s, serial) {
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
    case "audit_pruned":
      return t("log.sum.auditPruned", { n: s.removed, days: s.keep_days });
    case "log_rotated":
      return t("log.sum.logRotated", { why: logWord("log.rotateWhy.", s.why), kept: num(s.kept) });
    case "restore_started":
      return t("log.sum.restoreStarted", { steps: num(s.steps) });
    case "address_changed":
      return t("log.sum.addressChanged", { from: s.from || "–", to: s.to || "–" });
    case "change":
      return logChangeText(s, serial);
    case "command": {
      const state = logWord("log.cmdState.", s.state);
      if (s.duty_pct === undefined) return t("log.sum.command", { state });
      return t("log.sum.commandBudget", { state, duty: num(s.duty_pct), slots: num(s.free_slots) });
    }
    case "hold":
      if (s.reason === "duty_cycle") return t("log.sum.holdDuty", { duty: num(s.duty_pct), until: s.until ? fmtLogTime(new Date(s.until)) : "–" });
      return t("log.sum.holdSlots", { slots: num(s.free_slots), wait: num(s.wait_s) });
    case "login":
      return t("log.sum.login", { user: s.user || "–", remote: s.remote || "–" });
    case "login_failed":
      return t("log.sum.loginFailed", { user: s.user || "–", remote: s.remote || "–" });
    case "proxy_connected":
      return t("log.sum.proxyConnected", { remote: s.remote || "–" });
    case "proxy_disconnected":
      return t("log.sum.proxyDisconnected", { remote: s.remote || "–" });
    case "proxy_refused":
      return t("log.sum.proxyRefused", { remote: s.remote || "–" });
    case "proxy_discarded":
      return t("log.sum.proxyDiscarded", { remote: s.remote || "–", duty: num(s.duty_pct) });
    case "proxy_deleted":
      return t("log.sum.proxyDeleted", { remote: s.remote || "–", devices: (s.devices || []).join(", ") || "–" });
    case "proxy_delete_refused":
      if (s.why === "hidden") return t("log.sum.proxyDeleteHidden", { remote: s.remote || "–", devices: (s.devices || []).join(", ") || "–" });
      return t("log.sum.proxyDeleteRefused", { remote: s.remote || "–" });
    case "proxy_pairing":
      if (s.joined) return t("log.sum.proxyJoined", { remote: s.remote || "–" });
      return t("log.sum.proxyPairing", { remote: s.remote || "–", seconds: num(s.seconds) });
    case "proxy_warning":
      return logWord("log.proxyWarn.", s.what);
    case "locked_out":
      return t("log.sum.lockedOut", { remote: s.remote || "–", for: s.for || "–", after: logWord("log.lockedAfter.", s.after) });
  }
  return "";
}

// logTargetText names what a change concerns: a device by its name, else
// its S/N; a room by its name, of the Cube of the line (its S/N, the
// header's for none); a user, role or key as the line names it.
function logTargetText(s, serial) {
  const target = s.target || "";
  const i = target.indexOf(":");
  if (i < 0) return target;
  const kind = target.slice(0, i);
  const id = target.slice(i + 1);
  switch (kind) {
    case "device": {
      for (const r of (logRoomsOf(serial) || { rooms: [] }).rooms) {
        const d = r.devices.find((x) => x.serial === id || x.rf_address === id);
        if (d && d.name) return d.name;
      }
      return id;
    }
    case "room": {
      const r = (logRoomsOf(serial) || { rooms: [] }).rooms.find((x) => String(x.id) === id);
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
function logChangeText(s, serial) {
  const key = "log.action." + s.action;
  const action = t(key) === key ? s.action : t(key);
  const target = logTargetText(s, serial);
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
    cubeSerial: ev.cube_serial || "", // the line's Cube under All Cubes (GET /api/v1/log)
    command: ev.command || 0,
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
    const s = logSummaryText(line.kind, line.summary, line.cubeSerial);
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
  const hay = [fmtLogTime(line.time), line.kind, logKind(line.kind), logLineCube(line), line.device, logDeviceSerial(line), logDeviceName(line), logText(line)]
    .join(" ")
    .toLowerCase();
  return hay.includes(f);
}

// renderLogLine is one line of the log, from its template
// (tpl-log-line): the time, the kind, the device when the line has one,
// by its room and name (logDeviceName), with its S/N as the tooltip and,
// with its radio address, in expert mode, and the text. The Cube's own
// configuration is the Cube's, by its S/N; a removed device is named by
// the text alone. Under All Cubes a Cube's line names its Cube first,
// "Upstairs › Kitchen › Window", and the Cube's own lines by it alone.
function renderLogLine(line) {
  const row = fromTemplate("tpl-log-line");
  row.className += " kind-" + line.kind;
  row.querySelector(".log-time").textContent = fmtLogTime(line.time);
  row.querySelector(".log-kind").textContent = logKind(line.kind);
  const dev = row.querySelector(".log-device");
  const sn = row.querySelector(".log-sn");
  const rf = row.querySelector(".log-rf");
  const cube = logLineCube(line);
  let name = "";
  let serial = "";
  if (line.device && !line.gone && line.source !== "cube") {
    name = logDeviceName(line);
    serial = logDeviceSerial(line);
  } else if (cube) {
    name = cube;
  } else if (line.device && !line.gone) {
    name = t("log.cubeDevice", { serial: line.serial || line.device });
  }
  if (name) dev.textContent = cube && name !== cube ? logNamePath(cube, name) : name;
  else dev.remove();
  if (serial && name !== serial) {
    sn.textContent = serial;
    dev.title = serial;
  } else {
    sn.remove();
  }
  if (line.device) rf.textContent = line.device;
  else rf.remove();
  row.querySelector(".log-text").textContent = logText(line);
  if (line.command) {
    const open = logState.openCommands.has(line.command);
    const b = button(t("log.cmd.open", { id: line.command }), { cls: "subtle log-cmd", title: t("log.cmd.title") }, () => toggleLogCommand(line.command));
    b.setAttribute("aria-expanded", open ? "true" : "false");
    row.appendChild(b);
    if (open) row.appendChild(renderLogCommand(line.command));
  }
  return row;
}

// toggleLogCommand opens or closes the detail of a command under each
// of its lines, loading it the first time.
async function toggleLogCommand(id) {
  if (logState.openCommands.has(id)) {
    logState.openCommands.delete(id);
    renderLog();
    return;
  }
  logState.openCommands.add(id);
  renderLog();
  if (logState.commands.has(id)) return;
  let view;
  try {
    view = await api("GET", "api/v1/commands/" + id);
  } catch (e) {
    view = { error: e.status === 404 ? t("log.cmd.missing") : e.message };
  }
  logState.commands.set(id, view);
  if (logState.openCommands.has(id)) renderLog();
}

// renderLogCommand is a command's detail: its action, target, state and
// who asked, its steps, and its audit row for a user with audit.
function renderLogCommand(id) {
  const box = elem("div", "log-cmd-detail");
  const c = logState.commands.get(id);
  if (!c) {
    box.appendChild(textEl("p", t("log.cmd.loading")));
    return box;
  }
  if (c.error) {
    box.appendChild(textEl("p", c.error));
    return box;
  }
  const state = logWord("log.cmdState.", c.state);
  let head = t("log.cmd.head", { action: c.action, target: c.target || "–", state });
  if (c.user) head = t("log.by", { text: head, name: c.user });
  box.appendChild(textEl("p", head, "log-cmd-head"));
  const steps = elem("ol", "log-cmd-steps");
  for (const st of c.steps || []) {
    const r = st.result || {};
    let text = t("log.cmd.step", { state: logWord("log.cmdState.", st.state), target: st.target || "–" });
    if (r.discarded) text += "; " + t("log.cmd.discarded");
    if (r.error) text += "; " + r.error;
    steps.appendChild(textEl("li", text));
  }
  if (steps.childElementCount) box.appendChild(steps);
  if (c.audit) {
    const a = c.audit;
    box.appendChild(textEl("p", t("log.cmd.audit", { action: a.action, user: a.user || "–", remote: a.remote || "–", result: logWord("log.auditResult.", a.result) }), "log-cmd-audit"));
  }
  return box;
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

// logURL is where a page of the log comes from: every Cube's under All
// Cubes, else the header's Cube's.
function logURL() {
  return logState.cubes === "all" ? "api/v1/log" : base() + "/log";
}

// logQuery is the query of a page: the size (LOG_COUNT for the count,
// else LOG_PAGE), the start of a time range, or the time given (a poll's,
// as the server sent it), the kind and the device chosen, and where it
// starts.
function logQuery(before, since) {
  let q = "?limit=" + (logState.range === LOG_COUNT_RANGE ? LOG_COUNT : LOG_PAGE);
  const start = logRangeStart(logState.range);
  const from = since || (start ? start.toISOString() : "");
  if (from) q += "&since=" + encodeURIComponent(from);
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
  if (logState.cubes === "all") return; // the polls bring every Cube's (D9-A)
  const line = logLine(ev);
  line.seq = ++logState.liveSeq;
  addLogLine(line);
}

// addLogLine puts a new line on top, as appendLogEvent says.
function addLogLine(line) {
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
  logLoadCubeRooms();
  const gen = ++logState.gen;
  const since = logState.liveSeq;
  logState.loading = true;
  renderOlder();
  let evs = null;
  let failed = null;
  try {
    evs = (await api("GET", logURL() + logQuery())) || [];
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
    evs = (await api("GET", logURL() + logQuery(logState.cursor))) || [];
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

// logSameLine reports whether two lines are one event: by its id, else
// (a log without a store) by its time, kind, device and Cube.
function logSameLine(a, b) {
  if (a.id || b.id) return a.id === b.id;
  return a.at === b.at && a.kind === b.kind && a.device === b.device && a.cubeSerial === b.cubeSerial;
}

// pollLog asks, under All Cubes while the log is shown, for the lines
// since the newest one (D9-A); those the log does not hold yet come in on
// top as a live line does. None is asked while a page or a poll is on
// its way, and an answer that comes after a new page, another choice or
// Clear is dropped; one that fails leaves it to the next poll.
async function pollLog() {
  if (!logState.open || logState.cubes !== "all" || logState.loading || logState.polling) return;
  const gen = logState.gen;
  let newest = null;
  for (let i = logState.lines.length - 1; i >= 0 && !newest; i--) if (logState.lines[i].at) newest = logState.lines[i];
  logState.polling = true;
  let evs = null;
  try {
    evs = (await api("GET", "api/v1/log" + logQuery(null, newest ? newest.at : ""))) || [];
  } catch (e) {
    evs = null;
  }
  logState.polling = false;
  if (!evs || gen !== logState.gen || logState.cubes !== "all") return;
  for (const ev of evs) {
    const line = logLine(ev);
    if (!logState.lines.some((l) => logSameLine(l, line))) addLogLine(line);
  }
}

// logPollSet runs pollLog every LOG_POLL_MS while the log is shown under
// All Cubes, and stops it otherwise.
function logPollSet() {
  const want = logState.open && logState.cubes === "all";
  if (want && !logState.pollTimer) logState.pollTimer = setInterval(pollLog, LOG_POLL_MS);
  if (!want && logState.pollTimer) {
    clearInterval(logState.pollTimer);
    logState.pollTimer = null;
  }
}

// logLoadCubeRooms loads under All Cubes the rooms and the unlisted list
// of each Cube the page has none of yet, once (D8-A), and names the lines
// again once they are there; a Cube that does not answer stays without,
// its lines named by what they kept.
async function logLoadCubeRooms() {
  if (logState.cubes !== "all") return;
  const todo = state.cubes.filter((c) => !logState.cubeRooms.has(c.serial));
  if (!todo.length) return;
  await Promise.all(
    todo.map(async (c) => {
      logState.cubeRooms.set(c.serial, null);
      try {
        logState.cubeRooms.set(c.serial, await fetchRooms(c.serial));
      } catch (e) {
        // its lines keep the names they came with
      }
    })
  );
  if (!logState.open) return;
  if (fillLogSelects()) logChoicesChanged();
  else renderLog();
}

// logCubesKey names the Cubes and their names as the choices were filled
// for them (logState.cubesSeen).
function logCubesKey() {
  return state.cubes.map((c) => c.serial + "\u0000" + (c.name || "")).join("\u0001");
}

// logCubesPolled follows the Cubes a poll found while the log is shown,
// when they changed: the Cubes choice, shown with more than one, and
// under All Cubes the rooms of a Cube new to the page.
function logCubesPolled() {
  if (!logState.open || logCubesKey() === logState.cubesSeen) return;
  if (fillLogSelects()) logChoicesChanged();
  else renderLog();
  logLoadCubeRooms();
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

// renderLogClear turns Clear off while every choice is at its default:
// no text filter, this Cube, every kind, source and device, info and
// everyone.
function renderLogClear() {
  $("log-clear").disabled = !(logState.filter || logState.cubes || logState.kind || logState.device || logState.source || logState.user || logState.level !== "info");
}

// logChoicesChanged follows a new range, kind or device: the address in
// place of the current entry while the log is shown (logHash), Clear,
// and the log loaded again; a page on its way is dropped (loadLog).
function logChoicesChanged() {
  if (logState.open) setAddress(logHash(), "replace");
  renderLogClear();
  logPollSet();
  loadLog();
}

// clearLogFilter sets the text filter, the Cubes, the kind, the source or
// device, the level and who back to their defaults and loads the log
// again.
function clearLogFilter() {
  logState.filter = "";
  logState.cubes = "";
  logState.kind = "";
  logState.device = "";
  logState.source = "";
  logState.user = "";
  logState.level = "info";
  fillLogSelects(); // the devices of the header's Cube again
  $("log-filter").value = "";
  $("log-cubes").value = "";
  $("log-kind").value = "";
  $("log-device").value = "";
  $("log-level").value = "info";
  $("log-user").value = "";
  logChoicesChanged();
}

// fillLogSelects fills the choice of Cubes, shown with more than one
// Cube (or All Cubes chosen), of kind, every kind the server sends, and
// of device, the Cube's devices by name and S/N, under All Cubes every
// Cube's, a group each; a device chosen that is gone falls back to all,
// one chosen by its radio address takes its S/N (but under All Cubes,
// where it names that address on every Cube, D11-A), and the answer is
// true when the choice changed so. Until the devices are loaded
// (logRoomsOf), a device chosen from the address stays, shown as the
// address names it.
function fillLogSelects() {
  logState.cubesSeen = logCubesKey();
  const all = logState.cubes === "all";
  const cubes = fillOptions($("log-cubes"), [
    ["", t("log.thisCube")],
    ["all", t("log.allCubes")],
  ]);
  cubes.value = logState.cubes;
  cubes.classList.toggle("hidden", state.cubes.length < 2 && !all);
  const groups = LOG_SOURCES.map((src) => ({ group: t("log.src." + src), options: LOG_KINDS.filter((k) => (LOG_KIND_SOURCE[k] || "system") === src).map((k) => [k, logKind(k)]) }));
  const kinds = fillOptions($("log-kind"), [["", t("log.allKinds")]].concat(groups));
  kinds.value = logState.kind;
  fillOptions($("log-level"), LOG_LEVELS.filter((l) => l !== "error").map((l) => [l, t("log.level." + l)])).value = logState.level;
  const users = logState.users || [];
  const who = fillOptions($("log-user"), [["", t("log.allUsers")]].concat(users.map((u) => [u.name, u.display_name || u.name])));
  if (logState.user && !users.some((u) => u.name === logState.user)) who.appendChild(new Option(logState.user, logState.user));
  who.value = logState.user;
  who.classList.toggle("hidden", users.length === 0 && !logState.user);
  const devGroups = all
    ? state.cubes.map((c) => ({ group: c.name || c.serial, devs: logDevicesOf(logRoomsOf(c.serial)) }))
    : [{ group: t("log.devicesGroup"), devs: logDevicesOf({ rooms: state.rooms, unlisted: state.unlisted }) }];
  const devs = devGroups.flatMap((g) => g.devs);
  fillOptions($("log-range"), LOG_RANGES.map((r) => [r, t("log.range." + r)])).value = logState.range;
  const options = [["", t("log.allDevices")]]
    .concat(LOG_SOURCES.map((src) => ["src:" + src, t("log.src." + src)]))
    .concat(
      devGroups
        .filter((g) => g.devs.length)
        .map((g) => ({ group: g.group, options: g.devs.map((d) => [logDeviceKey(d), d.name ? d.name + " · " + logDeviceKey(d) : logDeviceKey(d)]) }))
    );
  const byRF = !all && devs.find((d) => d.rf_address === logState.device && d.serial);
  let changed = false;
  if (byRF) {
    logState.device = byRF.serial; // an old link's radio address
    changed = true;
  }
  const listed = devs.some((d) => logDeviceKey(d) === logState.device);
  const unknown = logState.device !== "" && !listed && !(all && devs.some((d) => d.rf_address === logState.device));
  const known = all ? state.cubes.length > 0 && state.cubes.every((c) => logRoomsOf(c.serial)) : !!state.serial && state.roomsOf === state.serial;
  if (logState.device !== "" && !listed && !(unknown && known)) options.push([logState.device, logState.device]);
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
// the Cube's log loaded. Under All Cubes the log stays as it is, every
// Cube's, and only names its lines again.
function logCubeChanged() {
  if (logState.cubes === "all") {
    if (logState.open) renderLog();
    return;
  }
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

// The pages of the main area: Rooms ("tiles"), the log ("log"), the
// personal settings ("me": My profile, My settings and Notifications as
// its cards) and System settings ("system"), each with its section and
// its button in the header; ☰ is highlighted while Log or System
// settings is shown, which it holds on a narrow screen. The tiles keep
// their state while hidden. The page shown is in the address
// (VIEW_HASH), Rooms without one, the personal page at a card by the
// card's own (CARD_HASH, the addresses of the pages they were), and the
// log's range, kind and device after its page (logHash), so that a
// reload or a link opens it again as chosen and the browser's back and
// forward move between the pages.
const VIEW_SECTION = { log: "logview", me: "me", system: "system" };
const VIEW_BUTTON = { tiles: "view-tiles", log: "view-log", me: "view-me", system: "view-settings" };
const VIEW_HASH = { tiles: "", log: "#log", me: "#me", system: "#system" };
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
// settings with one of its capabilities.
function mayShow(view) {
  if (view === "system") return hasSystem(); // login.js
  return view in VIEW_BUTTON || view in VIEW_SECTION;
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
  $("nav-menu").querySelector("summary").classList.toggle("active", view === "log" || view === "system");
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

function initLog() {
  $("view-tiles").addEventListener("click", () => showView("tiles"));
  $("view-log").addEventListener("click", () => showView("log"));
  $("view-settings").addEventListener("click", () => showView("system"));
  $("view-me").addEventListener("click", () => showView("me"));
  $("menu-log").addEventListener("click", () => showView("log"));
  $("menu-system").addEventListener("click", () => showView("system"));
  dropdown($("nav-menu"));
  // back and forward, and a page named in the address bar
  window.addEventListener("popstate", () => showAddress());
  $("log-filter").addEventListener("input", (ev) => {
    logState.filter = ev.target.value;
    renderLogClear();
    renderLog();
  });
  $("log-clear").addEventListener("click", clearLogFilter);
  $("log-cubes").addEventListener("change", (ev) => {
    logState.cubes = ev.target.value === "all" ? "all" : "";
    fillLogSelects(); // the devices of every Cube, or the header's alone
    logChoicesChanged();
  });
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
