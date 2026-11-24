// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/log/text.ts
 *
 * How the log words a line (AP UiParts, Block 11; line for line from
 * log.js): its device by its room and name from the rooms loaded, its
 * Cube under All Cubes, its kind, and its text: what a state event
 * changed, a configuration's and a list's changes, and each kind's
 * summary, in the language shown. A line is worded when it is drawn.
 */

import { computed, untracked } from "@preact/signals";
import type { Device } from "../api";
import { mismatchText, radiogramName, rfLabel, roomName } from "../model/device";
import type { Loose, LogLine } from "./lines";
import { type CubeRooms, logState } from "./state";
import { confirmFieldName } from "../model/room";
import { fmtSetpoint, TEMP_OFF } from "../shared/setpoint";

/**
 * logRoomsOf are the rooms and the unlisted list loaded of a Cube, by its
 * S/N, the header's Cube for none: the page's own where they are that
 * Cube's, else those All Cubes loaded (logLoadCubeRooms); null while
 * there are none, as while a Cube switch has not brought the new Cube's,
 * since a radio address names a device on its own Cube only.
 */
export function logRoomsOf(serial?: string | null): CubeRooms | null {
  const sn = serial || state.serial;
  if (sn && sn === state.serial && state.roomsOf === sn) {
    // the list followed, its rooms as they are now: a device's state event draws no line again
    const rooms = state.rooms;
    return { rooms: untracked(() => rooms.map((r) => ({ ...r }))), unlisted: state.unlisted };
  }
  return (sn && logState.cubeRooms.get(sn)) || null;
}

/** logDevicesOf are the devices of a Cube's rooms and unlisted list, each once, by name, else by S/N. */
export function logDevicesOf(rs: CubeRooms | null): Device[] {
  const devs: Device[] = [];
  if (!rs) return devs;
  // the rooms by the list; a device's state event draws no line or choice again
  untracked(() => {
    for (const r of rs.rooms) for (const d of r.devices) devs.push(d);
  });
  for (const d of rs.unlisted) if (!devs.some((x) => x.rf_address === d.rf_address)) devs.push(d);
  return devs.sort((a, b) => (a.name || logDeviceKey(a)).localeCompare(b.name || logDeviceKey(b)));
}

/** logDevice finds a device of a Cube (logRoomsOf) by its radio address; null when the Cube has none, or nothing of the Cube is loaded. */
export function logDevice(rf: string, serial?: string): Device | null {
  const rs = logRoomsOf(serial);
  if (!rs) return null;
  // the rooms by the list; a device's state event draws no line again
  const d = untracked(() => {
    for (const r of rs.rooms) {
      const x = r.devices.find((y) => y.rf_address === rf);
      if (x) return x;
    }
    return null;
  });
  return d || rs.unlisted.find((x) => x.rf_address === rf) || null;
}

/**
 * cubeNamesKey is every Cube's serial and name, as the choices' key has
 * them (cubesKey of choices.ts): a string, so that a load that writes the
 * list whole with the same names changes nothing that follows it.
 */
const cubeNamesKey = computed(() => state.cubes.map((c) => c.serial + "\u0000" + (c.name || "")).join("\u0001"));

/** cubeNames is the Cubes' names by serial, made again only where a name or a serial changed. */
const cubeNames = computed(() => {
  const names = new Map<string, string>();
  for (const entry of cubeNamesKey.value ? cubeNamesKey.value.split("\u0001") : []) {
    const [serial, name] = entry.split("\u0000");
    if (name) names.set(serial, name);
  }
  return names;
});

/** logCubeName names a Cube by its name, else its S/N: a line under All Cubes follows its Cube's name, not each load of the Cubes. */
export function logCubeName(serial: string): string {
  return cubeNames.value.get(serial) || serial;
}

/** logLineCube is the name of a line's Cube where the log shows every Cube's lines (D2); "" under This Cube and for the server's own lines. */
export function logLineCube(line: LogLine): string {
  return logState.cubes === "all" && line.cubeSerial ? logCubeName(line.cubeSerial) : "";
}

/** logNamePath joins a name to what holds it: "Kitchen › Window". */
export function logNamePath(outer: string, inner: string): string {
  return t("log.devicePath", { room: outer, device: inner });
}

/** upperFirst is a text with its first letter in upper case, for a word that stands in for a name ("Radiator thermostat"). */
export function upperFirst(s: string): string {
  return s ? s.charAt(0).toUpperCase() + s.slice(1) : s;
}

/** logDeviceKey is how the log names a device in its choice and its address: by its S/N, by its radio address only when it has none. */
export function logDeviceKey(d: Device): string {
  return d.serial || d.rf_address;
}

/**
 * logDeviceName is how a line names its device (AP LogCubes v1.0, D1-A,
 * D3-A, D4-A): by its room and name from the rooms and the unlisted list
 * already loaded, "Kitchen › Window"; by the name the line kept (a state
 * line's) where those give none, until the rooms come and for a device
 * the Cube has no longer; by its type where no name is known, after its
 * room; by its S/N where not even the type is, the radio address only for
 * a device without one. The S/N is then in the tooltip and, beside the
 * radio address, in expert mode (Line.tsx).
 */
export function logDeviceName(line: LogLine): string {
  if (!line.device) return "";
  const d = logDevice(line.device, line.cubeSerial);
  const kept = line.state || {};
  const type = (d && d.type) || kept.type;
  const device = (d && d.name) || kept.name || (type ? upperFirst(typeName(type)) : "");
  if (!device) return logDeviceSerial(line) || line.device;
  const room = d ? logDeviceRoom(d, line.cubeSerial) : "";
  return room ? logNamePath(room, device) : device;
}

/** logDeviceSerial is the S/N of a line's device: the line's, else the one the rooms loaded give its radio address; "" for neither. */
export function logDeviceSerial(line: LogLine): string {
  if (line.serial) return line.serial;
  const d = line.device ? logDevice(line.device, line.cubeSerial) : null;
  return (d && d.serial) || "";
}

/** logDeviceRoom is the name of the room a device of a Cube's rooms loaded is in; "" for a device of the unlisted list. */
function logDeviceRoom(d: Device, serial: string): string {
  const rs = logRoomsOf(serial);
  const r = rs && rs.rooms.find((x) => x.devices.includes(d));
  if (!r) return "";
  return roomName(r.id, r.name);
}

/**
 * logSetpoint is a setpoint as the log writes it: the number with one
 * digit, and 4.5 °C as the devices' displays show it with its
 * temperature, "OFF (4.5 °C)" (AP RoomMajority, D6-A); "–" for none.
 */
function logSetpoint(v: unknown): string {
  if (v === null || v === undefined) return "–";
  if (v === TEMP_OFF) return t("log.setpointOff", { off: fmtSetpoint(v), temp: fmtTemp(v) });
  return fmtNum(v as number, 1);
}

/** logStateText is the short state text of a state event. */
function logStateText(s: Loose | null): string {
  if (!s) return "";
  const parts: string[] = [];
  if (s.reported === false) parts.push(t("log.notReported"));
  const st = s.state || {};
  if (st.mode) parts.push(modeName(st.mode));
  if (st.setpoint !== undefined) parts.push(t("log.set", { v: logSetpoint(st.setpoint) }));
  if (st.actual_temp !== undefined) parts.push(t("log.actual", { v: fmtNum(st.actual_temp, 1) }));
  if (st.valve_pct !== undefined) parts.push(t("log.valve", { v: st.valve_pct }));
  if (st.window_open !== undefined) parts.push(t(st.window_open ? "log.windowOpen" : "log.windowClosed"));
  if (st.until) parts.push(t("log.until", { until: st.until }));
  for (const [k, key] of [["battery_low", "log.batteryLow"], ["error", "log.error"], ["link_error", "log.linkError"], ["panel_locked", "log.locked"]]) {
    if (!st[k]) continue;
    if (k === "error" && st.failed_radiogram) parts.push(t("log.errorRadiogram", { radiogram: radiogramName(st.failed_radiogram) }));
    else parts.push(t(key));
  }
  return parts.join(", ");
}

/**
 * logChangesText says what a state event changed, one phrase per value,
 * from the API's changes ({field: {from, to}}, null for a value the
 * device did not have): "window opened", "set 18.0 → 21.0", "reports
 * again"; a communication error with the radiogram that failed,
 * "communication error: TimeInformation (0x03)", and that radiogram
 * alone where another failed while the error stayed. A field this page
 * does not know shows as it came.
 */
function logChangesText(changes: Loose): string {
  const failed = changes.failed_radiogram ? changes.failed_radiogram.to : null;
  const num = (v: unknown, digits: number) => (v === null || v === undefined ? "–" : fmtNum(v as number, digits));
  const flag = (to: unknown, on: string, off: string) => t(to ? on : off);
  const parts: string[] = [];
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
        parts.push(t("log.ch.set", { from: logSetpoint(from), to: logSetpoint(to) }));
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
        if (to && failed) parts.push(t("log.errorRadiogram", { radiogram: radiogramName(failed) }));
        else parts.push(flag(to, "log.error", "log.ch.errorCleared"));
        break;
      case "failed_radiogram":
        // said with the error where it came or went with it
        if (to && !changes.error) parts.push(t("log.ch.failedRadiogram", { radiogram: radiogramName(to) }));
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

/**
 * logListText says what changed in the room and device list, one phrase
 * per change (AP Logging v1.1, Block 4): "Room “Bath” renamed
 * “Bathroom”", "MEQ0000101 “Window” moved from Bath to Kitchen".
 */
function logListText(changes: Loose[]): string {
  return changes
    .map((c) => {
      const key = "log.list." + c.what;
      const s = t(key, { name: c.name || "", from: c.from || "–", to: c.to || "–", device: c.serial || c.device || "" });
      return s === key ? c.what : s;
    })
    .join("; ");
}

/**
 * logValue shows a value of a configuration: a temperature with one
 * digit, a switch as on or off, a time zone by its name, a day by its
 * name.
 */
function logValue(key: string, v: unknown): string {
  if (v === null || v === undefined || v === "") return "–";
  if (typeof v === "boolean") return t(v ? "log.on" : "log.off");
  if (typeof v === "number") return key.endsWith("_temp") || key.endsWith("_setpoint") || key === "temp_offset" ? fmtNum(v, 1) : String(v);
  if (typeof v === "object" && (v as Loose).name !== undefined) return (v as Loose).name || "–";
  if (typeof v === "string" && t("day." + v) !== "day." + v) return t("day." + v);
  return typeof v === "string" ? v : JSON.stringify(v);
}

/** logConfigText says what changed in a device's configuration, one phrase per value: "comfort 21.0 → 21.5", "Monday's program changed". */
function logConfigText(changes: Loose): string {
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

/**
 * logSummaryText is what a line says of itself where it names no
 * differences: a connection's devices, rooms, firmware, budget and how
 * long the Cube was lost; a hello's budget; a list's rooms and devices; a
 * configuration's main values.
 */
function logSummaryText(kind: string, s: Loose, serial: string): string {
  const num = (v: unknown) => (v === undefined || v === null ? "–" : String(v));
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
      const why = logWord("log.readWhy.", ({ "the settings file changed": "file", SIGHUP: "sighup", "the API": "api" } as Record<string, string>)[s.why] || s.why);
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
    case "update_postponed":
      return t("log.sum.updatePostponed", { version: s.version || "–", why: s.why || "–" });
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
    case "wall_reset":
      return t("log.sum.wallReset", { mode: logWord("log.mode.", s.mode), was: logWord("log.mode.", s.was_mode), silent: num(s.silent_min) });
    case "battery":
      // the level's words, "Batteries low, replace now", the level before
      // in a short word, "(before: weak)"; new batteries marked say so,
      // and who marked them is the line's by
      return t(s.cause === "replaced" ? "log.sum.batteryReplaced" : "log.sum.battery", { level: logWord("log.battery.", s.level), from: logWord("log.batteryBefore.", s.from) });
    case "error_reset":
      return t("log.sum.errorReset");
    case "error_watch":
      return logErrorWatchText(s);
    case "structure": {
      const action = logWord("log.structure.", s.action);
      let text: string;
      if (s.outcome === "done") text = t("log.sum.structureDone", { action, slots: num(s.free_slots), minutes: num(s.minutes) });
      else if (s.no_reading) text = t("log.sum.structureNoReading", { action });
      else text = t("log.sum.structureHeld", { action, held: num(s.held), slots: num(s.free_slots), minutes: num(s.minutes) });
      const parts = [text];
      if (s.flagged) parts.push(t("log.sum.structureFlagged", { devices: s.flagged.map(rfLabel).join(", ") }));
      if (s.link_error) parts.push(t("log.sum.structureLink", { devices: s.link_error.map(rfLabel).join(", ") }));
      return parts.join("; ");
    }
    case "unconfirmed":
      return logUnconfirmedText(s, serial);
    case "slots": {
      const at = (iso: string) => (iso ? fmtTimeOfDay(new Date(iso)) : "–");
      if (s.state === "full") return t("log.sum.slotsFull", { slots: num(s.free_slots), since: at(s.full_since || s.since) });
      if (s.state === "low") return t("log.sum.slotsLow", { slots: num(s.free_slots), since: at(s.since) });
      return t("log.sum.slotsFree", { slots: num(s.free_slots), since: at(s.since) });
    }
    case "mismatch": {
      const text = mismatchText(s as Parameters<typeof mismatchText>[0]);
      return s.state === "cleared" ? t("log.sum.mismatchCleared", { text }) : text;
    }
    case "disagree":
      return logDisagreeText(s, serial);
    case "drift":
      return logDriftText(s);
    case "hold":
      if (s.reason === "duty_cycle") return t("log.sum.holdDuty", { duty: num(s.duty_pct), until: s.until ? fmtTimeOfDay(new Date(s.until)) : "–" });
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

/**
 * logErrorWatchText is an error_watch line's text (AP RadioHealth v1.2,
 * Block 6): a communication error back after the automatic reset, with
 * the radiogram that failed; a communication error or a link error set
 * for 24 hours; one clear again.
 */
function logErrorWatchText(s: Loose): string {
  const num = (v: unknown) => (v === undefined || v === null ? "–" : String(v));
  const link = s.flag === "link_error";
  const parts: string[] = [];
  switch (s.what) {
    case "back":
      parts.push(t(s.next_report ? "log.sum.watchBackNext" : "log.sum.watchBack", { minutes: num(s.minutes) }));
      break;
    case "lasting":
      parts.push(t(link ? "log.sum.watchLastingLink" : "log.sum.watchLasting", {
        hours: num(s.hours), since: s.since ? fmtTimeOfDay(new Date(s.since)) : "–" }));
      if (s.silent) parts.push(t("log.sum.watchSilent"));
      break;
    case "clear":
      parts.push(t(link ? "log.sum.watchClearLink" : "log.sum.watchClear"));
      if (s.hours !== undefined && s.hours !== null) parts.push(t("log.sum.watchFor", { hours: num(s.hours) }));
      break;
    default:
      return "";
  }
  if (!link && s.failed_radiogram) parts.push(t("log.ch.failedRadiogram", { radiogram: radiogramName(s.failed_radiogram) }));
  if (s.what === "back") parts.push(t("log.sum.watchHeld"));
  else if (s.what === "clear" && s.held) parts.push(t("log.sum.watchResume"));
  return parts.join("; ");
}

/** logRoomName names a room of a line's Cube by its name, else its number. */
function logRoomName(id: number, serial: string): string {
  const r = (logRoomsOf(serial) || { rooms: [] as PageRoom[] }).rooms.find((x) => x.id === id);
  return roomName(id, r?.name);
}

/**
 * logSettingText is a thermostat's setting as a room check names it: its
 * mode, its setpoint (or the week programme, for auto without one) and
 * its vacation's end.
 */
function logSettingText(m: Loose): string {
  const parts = [logWord("log.mode.", m.mode)];
  if (m.setpoint !== undefined) parts.push(t("log.set", { v: logSetpoint(m.setpoint) }));
  else if (m.mode === "auto") parts.push(t("log.disagree.program"));
  if (m.until) parts.push(t("log.until", { until: m.until }));
  return parts.join(" ");
}

/**
 * logDisagreeText is a room check's line of a room whose thermostats
 * disagree (drift.go): the report with each member's setting, the
 * room's state sent again and from where (AP RoomMajority, D9: Massimo,
 * the majority, the newest change, the master, the wall thermostat, a
 * radiator thermostat), the room's state from before a device was put
 * into it named by the rule that took it then (before_change), or why
 * not, and the members agreeing again.
 */
function logDisagreeText(s: Loose, serial: string): string {
  const room = logRoomName(s.room, serial);
  const num = (v: unknown) => (v === undefined || v === null ? "–" : String(v));
  const member = (rf: string) => {
    const d = logDevice(rf, serial);
    return d ? deviceName(d) : rf;
  };
  switch (s.outcome) {
    case "differ": {
      const members = ((s.members || []) as Loose[]).map((m) => member(m.rf) + ": " + logSettingText(m)).join(", ");
      const what = ((s.differs || []) as string[]).map((w) => logWord("log.disagree.what.", w)).join(", ");
      return t("log.sum.disagreeDiffer", { room, what: what || "–", minutes: num(s.minutes), members: members || "–" });
    }
    case "resent": {
      const state = logSettingText(s);
      if (s.before_change) return t("log.sum.disagreeResentBefore", { room, state, from: logWord("log.disagree.before.", s.from) });
      return t("log.sum.disagreeResent", { room, state, from: logWord("log.disagree.from.", s.from) });
    }
    case "resend_failed":
      return t("log.sum.disagreeFailed", { room, state: logSettingText(s) });
    case "not_resent":
      return t("log.sum.disagreeNotResent", { room, why: logWord("log.disagree.why.", s.why) });
    case "agree":
      return t("log.sum.disagreeAgree", { room, minutes: num(s.minutes) });
  }
  return "";
}

/**
 * logUnconfirmedText is the line of a write its devices did not show by
 * its deadline (confirm.go): the write, its room, each device by name with
 * what it shows; or a device that showed it late; or, for a room's
 * setpoint, its re-send (resend.go): sent once more or why not, then
 * shown by every thermostat or still not, again with each device.
 */
function logUnconfirmedText(s: Loose, serial: string): string {
  const num = (v: unknown) => (v === undefined || v === null ? "–" : String(v));
  const write = logWord("log.write.", s.write);
  const room = s.room ? logRoomName(s.room, serial) : "–";
  switch (s.outcome) {
    case "confirmed_late":
      return t("log.sum.confirmedLate", { write, seconds: num(s.seconds) });
    case "resent": {
      const text = t("log.sum.unconfirmedResent", { write, room });
      return s.program ? text + ", " + t("log.disagree.program") : text;
    }
    case "not_resent":
      return t("log.sum.unconfirmedNotResent", { write, room, why: logWord("log.unconfirmed.why.", s.why) });
    case "resend_failed":
      return t("log.sum.unconfirmedResendFailed", { write, room });
    case "resend_confirmed":
      return t("log.sum.unconfirmedResendConfirmed", { write, room, seconds: num(s.seconds) });
  }
  const value = (k: string, v: unknown): string => {
    if (Array.isArray(v)) return ""; // a day's switch points: its name says it
    if (k === "mode") return logWord("log.mode.", v);
    if (k === "setpoint") return logSetpoint(v);
    if (k === "locked") return t(v ? "confirm.lockedOn" : "confirm.lockedOff");
    return logValue(k, v);
  };
  const devices = ((s.devices || []) as Loose[]).map((d) => {
    const dev = logDevice(d.rf_address, serial);
    const name = dev ? deviceName(dev) : d.rf_address;
    const shows = Object.keys(d.shows || {}).sort().map((k) => (confirmFieldName(k) + " " + value(k, d.shows[k])).trim());
    return shows.length ? name + " (" + shows.join(", ") + ")" : name;
  }).join(", ");
  const args: Record<string, string> = { write, minutes: num(s.minutes), devices: devices || "–" };
  if (!s.room) return t("log.sum.unconfirmed", args);
  if (s.outcome === "resend_not_confirmed") return t("log.sum.unconfirmedResendNot", Object.assign(args, { room }));
  return t("log.sum.unconfirmedRoom", Object.assign(args, { room }));
}

/**
 * logDriftText is a room check's line of a thermostat that does not
 * follow its week programme (drift.go): its setpoint and the
 * programme's, its clock an hour off, the radiogram that failed; and
 * its following again.
 */
function logDriftText(s: Loose): string {
  const num = (v: unknown) => (v === undefined || v === null ? "–" : String(v));
  switch (s.outcome) {
    case "not_following": {
      const parts = [t("log.sum.driftNot", { setpoint: logSetpoint(s.setpoint), expected: logSetpoint(s.expected), at: s.switch_at || "–" })];
      if (s.hour_off) parts.push(t(s.auto_dst === false ? "log.sum.driftHourNoDst" : "log.sum.driftHour"));
      if (s.failed_radiogram) parts.push(t("log.errorRadiogram", { radiogram: radiogramName(s.failed_radiogram) }));
      return parts.join("; ");
    }
    case "follows":
      return t("log.sum.driftFollows", { minutes: num(s.minutes) });
    case "left_auto":
      return t("log.sum.driftLeftAuto", { mode: logWord("log.mode.", s.mode) });
  }
  return "";
}

/**
 * logTargetText names what a change concerns: a device by its name, else
 * its S/N; a room by its name, of the Cube of the line (its S/N, the
 * header's for none); a user, role or key as the line names it.
 */
function logTargetText(s: Loose, serial: string): string {
  const target: string = s.target || "";
  const i = target.indexOf(":");
  if (i < 0) return target;
  const kind = target.slice(0, i);
  const id = target.slice(i + 1);
  switch (kind) {
    case "device": {
      for (const r of (logRoomsOf(serial) || { rooms: [] as PageRoom[] }).rooms) {
        const d = r.devices.find((x) => x.serial === id || x.rf_address === id);
        if (d && d.name) return d.name;
      }
      return id;
    }
    case "room": {
      const r = (logRoomsOf(serial) || { rooms: [] as PageRoom[] }).rooms.find((x) => String(x.id) === id);
      return roomName(Number(id), r?.name);
    }
    case "token":
      return s.subject && s.subject.name ? s.subject.name : id;
    case "user":
      return s.subject && s.subject.display_name ? s.subject.display_name : id;
  }
  return id;
}

/** SET_SAID are the actions whose words say what they set, the body of new batteries marked ({"replaced": true}), which a change line leaves out. */
const SET_SAID: Record<string, boolean> = { "device.battery": true };

/** logChangeText is a change line (AP Logging v1.1, Block 6): what was done, to what, and what changed from and to, else what it set. */
function logChangeText(s: Loose, serial: string): string {
  const key = "log.action." + s.action;
  const action = t(key) === key ? s.action : t(key);
  const target = logTargetText(s, serial);
  let text = target ? t("log.sum.change", { action, target }) : action;
  const changes = s.changes && Object.keys(s.changes).length ? s.changes : null;
  if (changes) {
    text += ": " + logConfigText(changes);
  } else if (s.set && typeof s.set === "object" && !SET_SAID[s.action]) {
    const parts = Object.keys(s.set)
      .sort()
      .filter((k) => s.set[k] !== "…" && typeof s.set[k] !== "object")
      .map((k) => {
        const label = t("log.cfg." + k) === "log.cfg." + k ? k : t("log.cfg." + k);
        return label + " " + (k === "temp" ? logSetpoint(s.set[k]) : logValue(k, s.set[k])); // a setpoint's temperature, OFF at 4.5 °C
      });
    if (parts.length) text += ": " + parts.join(", ");
  }
  return s.via === "cli" ? t("log.viaCli", { text }) : text;
}

/** logWord is a value's word in the language shown, the value itself where the page has none. */
export function logWord(prefix: string, v: unknown): string {
  const key = prefix + v;
  const s = t(key);
  return s === key ? String(v) : s;
}

/** logText is a line's whole text: its own, its error, while the Cube was not connected, and who started it. */
export function logText(line: LogLine): string {
  let text = logOwnText(line);
  if (line.error && text !== line.error) text = text ? text + ": " + line.error : line.error;
  if (line.missed && text) text = t("log.whileDisconnected", { text });
  if (line.by || line.who) text = t("log.by", { text, name: (line.who || line.by?.display_name || line.by?.name) as string });
  return text;
}

/** logOwnText is a line's text without the note of when it changed. */
function logOwnText(line: LogLine): string {
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

/** logKind is the event kind as shown; the class keeps the API's name. */
export function logKind(kind: string): string {
  const key = "log.kind." + kind;
  const s = t(key);
  return s === key ? kind : s;
}

/** logRangeStart is where a time range starts, seen from now; null for the count and for all. */
export function logRangeStart(range: string, now = new Date()): Date | null {
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
