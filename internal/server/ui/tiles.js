// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

// The Massimo web UI, tiles: the actions of a room's tile that stay
// plain (the More menu's presets, a write of the tile, the holiday form,
// the device list, its menus closed), a finding of the Cube's lines in
// words, and what a device row of the devices without a room still draws
// with (devices.js). The tile itself is a part (ui/src/rooms/Tile.tsx),
// drawn from the tile's live state (rooms.js roomUI) that these set. The
// rooms' facts and the other actions behind a tile are in rooms.js, the
// devices without a room in unlisted.js, the House tile in house.js. Uses
// the state, api and helpers of app.js.
"use strict";

// --- a device's problems, for devices.js ------------------------------
//
// What a row of the devices without a room still draws with, until they
// are a part: their twins are in ui/src/model/device.ts and
// ui/src/rooms/facts.ts.

// PROBLEM_KINDS are the problems a tile's head shows as icons, in their
// order there, each with its icon (index.html's sprite): a sign where
// its state has a mark (components.js signOf), the radio and the window,
// drawn as the person chooses (problemIcon); and a holiday, which is no
// problem but is listed with its end.
const PROBLEM_KINDS = {
  radio: null,
  battery: "icon-battery",
  window: null,
  unconfirmed: "icon-unconfirmed",
  mixed: "icon-shuffle",
  holiday: "icon-suitcase",
};

// PROBLEM_SIGNS are the kinds of PROBLEM_KINDS drawn as signs.
const PROBLEM_SIGNS = { radio: "radio", window: "window" };

// problemIcon is a kind's icon: its sign's, as the person chooses it,
// for the radio and the window.
function problemIcon(kind) {
  return PROBLEM_SIGNS[kind] ? signOf(PROBLEM_SIGNS[kind]).icon : PROBLEM_KINDS[kind];
}

// windowState is an open window's state by the page's clock and the
// window's times (the server's limits): "open" until its warning time,
// "warn" until its alert time, "bad" after; "" for a window not open.
function windowState(d) {
  const s = d.state || {};
  if (s.window_open !== true) return "";
  const w = d.window;
  if (!w) return "open";
  const now = Date.now();
  if (now >= Date.parse(w.alert_at)) return "bad";
  if (now >= Date.parse(w.warn_at)) return "warn";
  return "open";
}

// WINDOW_TONE is the badge of an open window's state: filled blue while
// it is open briefly, an amber outline (a warning) once it is open long,
// filled red (a failure) once it is open too long; a closed window's is
// filled green.
const WINDOW_TONE = { open: "fill-open", warn: "warn", bad: "fill-bad" };

// MODE_TONE colours a mode's badge: auto green, manual (and boost) amber,
// holiday violet.
const MODE_TONE = { auto: "fill-ok", manual: "fill-warn", holiday: "fill-info" };

// modeTone is the badge kind of a thermostat's mode.
function modeTone(mode) {
  const m = mode === "vacation" ? "holiday" : mode === "boost" ? "manual" : mode;
  return MODE_TONE[m] || "mode";
}

// deviceIcon is the icon of a device's type: a radiator, a wall
// thermostat, a window as the person chooses it, closed, or a leaf for
// the eco button. The parts take its TypeScript twin of
// ui/src/model/device.ts; this one goes once the devices without a room
// (devices.js) are a part.
function deviceIcon(d) {
  if (d.type === "wall_thermostat") return "icon-wall";
  if (d.type === "shutter_contact") return iconChoice("window").type; // components.js
  if (d.type === "eco_switch") return "icon-leaf";
  return "icon-radiator";
}

// radiogramName is a failed radiogram's type as the API gives it,
// {code, name}: the original software's name with its code,
// "TimeInformation (0x03)", or the code alone for one its list lacks.
// The names are technical and stay untranslated.
function radiogramName(r) {
  const code = "0x" + (Number(r.code) & 0xff).toString(16).padStart(2, "0");
  return r.name ? r.name + " (" + code + ")" : code;
}

// communicationErrorTitle explains a device's communication error, and
// names the radiogram that failed where the state gives it.
function communicationErrorTitle(s) {
  const lines = [t("badge.communicationErrorTitle")];
  if (s.failed_radiogram) lines.push(t("badge.failedRadiogram", { radiogram: radiogramName(s.failed_radiogram) }));
  return lines.join("\n");
}

// deviceProblems are what is wrong with a device, as problemsEl's items:
// a radio fault (the Cube could not reach it, its communication error,
// or its link to another device of the room is disturbed, its link
// error), each explained in its title, and a weak or low battery;
// withWindow, also an open window, with its age. The parts take its
// TypeScript twin of ui/src/model/device.ts; this one goes once the
// devices without a room (devices.js) are a part.
function deviceProblems(d, withWindow) {
  const s = d.state || {};
  const out = [];
  if (!d.reported) return out;
  if (s.error) {
    out.push({ kind: "radio", icon: problemIcon("radio"), sign: PROBLEM_SIGNS.radio, tone: "bad", text: t("badge.communicationError"),
      title: communicationErrorTitle(s) });
  }
  if (s.link_error) {
    out.push({ kind: "radio", icon: problemIcon("radio"), sign: PROBLEM_SIGNS.radio, tone: "warn", text: t("badge.linkError"),
      title: t("badge.linkErrorTitle") });
  }
  // The battery's level, the server's (internal/battery): weak amber, low
  // red; a device of a server without levels by its flag, amber.
  if (d.battery === "low") out.push({ kind: "battery", icon: PROBLEM_KINDS.battery, tone: "bad", text: t("badge.batteryLow"), at: received(d, "battery_low"), stale: d.stale });
  else if (d.battery === "weak") out.push({ kind: "battery", icon: PROBLEM_KINDS.battery, tone: "warn", text: t("badge.batteryWeak"), at: received(d, "battery_low"), stale: d.stale });
  else if (d.battery === undefined && s.battery_low) out.push({ kind: "battery", icon: PROBLEM_KINDS.battery, tone: "warn", text: t("badge.batteryLow"), at: received(d, "battery_low"), stale: d.stale });
  if (withWindow && s.window_open === true) {
    out.push({ kind: "window", icon: problemIcon("window"), sign: PROBLEM_SIGNS.window, tone: WINDOW_TONE[windowState(d)], text: t("badge.open"), at: received(d, "window_open"), stale: d.stale });
  }
  return out;
}

// problemsLabel names the kinds of problems among items, and a holiday
// with its end, for a screen reader and the tooltip.
function problemsLabel(items) {
  const kinds = Object.keys(PROBLEM_KINDS).filter((k) => k !== "holiday" && items.some((it) => it.kind === k));
  const parts = kinds.length ? [t("problem.label", { list: kinds.map((k) => t("problem." + k)).join(", ") })] : [];
  const holiday = items.find((it) => it.kind === "holiday");
  if (holiday) parts.push(holiday.text);
  return parts.join("; ");
}

// mismatchArgs are a finding's words: its device by the name the page
// knows, else the room list's, else its radio address; its rooms by
// their names, else their numbers; its types.
function mismatchArgs(m) {
  const room = (id, name) => name || (id === 0 ? t("mismatch.noRoom") : t("log.room", { id }));
  const known = m.device ? rfLabel(m.device) : ""; // log.js
  return {
    device: known && known !== m.device ? known : m.name || m.device || "–",
    room: room(m.room, m.room_name),
    cube: m.cube_room === undefined ? "–" : room(m.cube_room, m.cube_room_name),
    list: m.list_room === undefined ? "–" : room(m.list_room, m.list_room_name),
    cubeType: m.cube_type ? typeName(m.cube_type) : "–", // rooms.js
    listType: m.list_type ? typeName(m.list_type) : "–",
  };
}

// mismatchText is a finding in a sentence that names the device and both
// rooms; a room master's by why.
function mismatchText(m) {
  const key = m.finding === "master" ? "mismatch.master." + m.why : "mismatch." + m.finding;
  const s = t(key, mismatchArgs(m));
  return s === key ? m.finding : s;
}

// presetTemps are a thermostat's configured eco and comfort
// temperatures, { eco, comfort }, undefined where it has none.
async function presetTemps(th) {
  const detail = await deviceDetail(th);
  const cfg = detail.thermostat || detail.wall_thermostat || {};
  return { eco: cfg.eco_temp, comfort: cfg.comfort_temp };
}

// presetBody is the setpoint body that sets temp and keeps the room's
// mode (D99): in Auto until the next switch point, or in Manual where
// the tile's Hold switch is on; Manual and a boost hold it, a holiday
// keeps its end. A room whose thermostats differ takes its room
// thermostat's mode, and no Hold, which it does not show.
function presetBody(room, temp) {
  const s = roomState(room) || {};
  if (s.mode === "auto") return roomModes(room).length <= 1 && roomUI(room).hold ? { mode: "manual", temp } : { mode: "auto", temp };
  if (s.mode === "vacation" && /^\d{4}-\d{2}-\d{2} \d{2}:\d{2}$/.test(s.until || "")) return { mode: "vacation", temp, until: s.until };
  return { mode: "manual", temp };
}

// presetLabel names a preset's write by the body it sends.
function presetLabel(which, body) {
  const temp = fmtNum(body.temp, 1);
  const label = which === "eco" ? t("label.eco", { temp }) : t("label.comfort", { temp });
  if (body.mode === "auto") return t("label.presetAuto", { label });
  if (body.mode === "vacation") return t("label.presetUntil", { label, until: fmtUntil(body.until) });
  return t("label.presetManual", { label });
}

// sendPreset sets the room to its thermostat's eco or comfort
// temperature (which) in the room's mode, from the More menu, which
// keeps the focus.
function sendPreset(room, which) {
  leaveEditing(room);
  const sum = document.querySelector('.room[data-id="' + room.id + '"] .more-sum');
  if (sum) sum.focus();
  return tileWrite(room, which, async () => {
    const what = t(which === "eco" ? "room.eco" : "room.comfort");
    let temp;
    try {
      temp = (await presetTemps(roomThermostat(room)))[which];
    } catch (e) {
      showTileResult(room.id, errorText(what, e), false);
      return null;
    }
    if (temp === undefined) {
      showTileResult(room.id, t("result.error", { label: what, error: t(which === "eco" ? "house.noEco" : "house.noComfort") }), false);
      return null;
    }
    const body = presetBody(room, temp);
    return setpointRequest(room, body, presetLabel(which, body));
  });
}

// tileWrite runs fn, a write of the room's tile, as key: the tile's
// write buttons are off meanwhile, the one of key turning (the tile
// follows ui.sending), so that a double click sends one radio command;
// a write asked for while one runs sends nothing and returns null. A
// key is a mode (auto, manual, vacation, boost), a preset (eco,
// comfort), the stepper's setpoint, lock, unlock, or device:<rf> for a
// device's Lock.
async function tileWrite(room, key, fn) {
  const ui = roomUI(room);
  if (ui.sending) return null;
  ui.sending = key;
  try {
    return await fn();
  } finally {
    ui.sending = null;
  }
}

// roomRepairScope is the scope of a room's repairs (repair.js).
function roomRepairScope(room) {
  return repairScope(state.serial, "room", room.id);
}

// closeMenu closes the open menus of a tile: its ⋯, More and its
// problems.
function closeMenu(el) {
  if (!el) return;
  for (const m of el.querySelectorAll("details.menu[open]")) m.open = false;
}

// leaveEditing closes the tile's vacation panel and drops a pending
// stepper value, for a mode button that takes over.
function leaveEditing(room) {
  const ui = roomUI(room);
  if (ui.timer) clearTimeout(ui.timer);
  ui.timer = null;
  ui.pending = null;
  ui.vacation = false;
}

// openVacation opens the tile's holiday form, which puts the focus on
// its end (Tile.tsx Vacation); el is the tile, whose menus it closes.
function openVacation(room, el) {
  closeMenu(el);
  roomUI(room).vacation = true;
}

function closeVacation(room) {
  roomUI(room).vacation = false;
}

// toggleDevices opens or closes the tile's device list, one of it and
// the configuration open at a time; el is the tile, whose menus it
// closes.
function toggleDevices(room, el) {
  const ui = roomUI(room);
  ui.open = !ui.open;
  if (ui.open) ui.config = null; // one of device list and configuration at a time
  closeMenu(el);
}
