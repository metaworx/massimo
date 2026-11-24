// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

// The Massimo web UI, the rooms: the room's facts its actions use (the
// others are ui/src/model/room.ts'), the plain names of the
// rooms' drawing, which call their parts (ui/src/rooms/, MassimoParts.rooms),
// a device's state event, the texts of a result, the actions behind a
// tile's controls, which set the tile's live state (roomUI) that the tile
// follows, and what holds for every tile (Escape closes the open panels).
"use strict";

const STEP = 0.5;
const MIN_TEMP = 4.5;
const MAX_TEMP = 30.5;
const DEFAULT_TEMP = 20; // stepper start when the room has no setpoint yet
const SEND_DELAY_MS = 2000;
// AWAIT_MS is how long a setpoint written to a room waits for the room's
// thermostats to show it, as the server's confirmation waits (confirm.go,
// five minutes after the Cube's answer).
const AWAIT_MS = 5 * 60 * 1000;

// --- room facts -------------------------------------------------------

// roomThermostat returns the thermostat whose state stands for the room:
// the wall thermostat when there is one, else the first radiator
// thermostat; null for a room without thermostats.
function roomThermostat(room) {
  const wall = room.devices.find((d) => d.type === "wall_thermostat");
  return wall || room.devices.find(isThermostat) || null;
}

function roomState(room) {
  const th = roomThermostat(room);
  return th && th.state ? th.state : null;
}

// roomModes lists the modes of the room's thermostats, boost counted as
// manual: one entry when they agree, several when a device was set on
// its own (by another client or at the device).
function roomModes(room) {
  const modes = [];
  for (const d of room.devices) {
    if (!isThermostat(d) || !d.state || !d.state.mode) continue;
    const m = d.state.mode === "boost" ? "manual" : d.state.mode;
    if (!modes.includes(m)) modes.push(m);
  }
  return modes;
}

// received is when the server received the current value of a device's
// state field, as the API's time, or undefined.
function received(d, field) {
  return d.received ? d.received[field] : undefined;
}

// roomUI is what the room's tile keeps, made the first time it is asked
// for: a live object (ui/src/rooms/store.ts newUI), whose properties the
// plain actions set as before and the tile follows. open is its device
// list, pending and timer a stepper value waiting to be sent, vacation
// and confirm the holiday form and the deletion's question, config the
// configuration open (ui/src/rooms/Config.tsx, which keeps the rest),
// sending the write running (tileWrite), hold the Hold switch, result the
// result line (showTileResult), awaiting and awaitTimer a setpoint
// written to the room that its thermostats do not all show yet
// (awaitMembers).
function roomUI(room) {
  if (!state.ui[room.id]) state.ui[room.id] = MassimoParts.rooms.ui();
  return state.ui[room.id];
}

function clampTemp(v) {
  return Math.min(MAX_TEMP, Math.max(MIN_TEMP, Math.round(v * 2) / 2));
}

// fmtUntil is a holiday's end, the Cube's wall-clock "YYYY-MM-DD HH:MM",
// as day, month and time in the page's language ("24.12., 18:00"), with
// the year where it is not this one; another text as it is.
function fmtUntil(text) {
  const m = /^(\d{4})-(\d{2})-(\d{2}) (\d{2}):(\d{2})$/.exec(text || "");
  if (!m) return text || "?";
  const [y, mo, d, hh, mm] = m.slice(1).map(Number);
  const opts = { day: "2-digit", month: "2-digit", hour: "2-digit", minute: "2-digit", hourCycle: "h23" };
  if (y !== new Date().getFullYear()) opts.year = "numeric";
  return new Intl.DateTimeFormat(i18n.lang, opts).format(new Date(y, mo - 1, d, hh, mm));
}

// renderRooms draws the rooms as the page has them now: every tile and
// every row of the devices without a room follows in place
// (MassimoParts.rooms.render), a tile's configuration with it; the House
// tile and the devices without a room are shown where they have
// something.
function renderRooms() {
  for (const r of state.rooms) settleAwaiting(r, false);
  MassimoParts.rooms.render();
  $("house").classList.toggle("hidden", !state.rooms.some(roomThermostat));
  $("unlisted").classList.toggle("hidden", !state.unlisted.length);
}


// updateDevice applies a state event to the room or the unlisted list:
// a room's fields are signals (ui/src/shared/data.ts), so that its tile
// follows in place, its configuration with it, with the focus in it or
// not; the list is written whole, and its row follows, the form under it
// as it is.
function updateDevice(d) {
  if (d.virtual) {
    const r = state.rooms.find((x) => x.id === d.room_id);
    if (r) r.vwt = d;
    return;
  }
  for (const r of state.rooms) {
    const i = r.devices.findIndex((x) => x.rf_address === d.rf_address);
    if (i >= 0) {
      r.devices = r.devices.map((x, k) => (k === i ? d : x));
      r.ready = r.devices.every((x) => x.reported);
      r.stale = r.devices.some((x) => x.stale);
      settleAwaiting(r, true);
      return;
    }
  }
  const j = state.unlisted.findIndex((x) => x.rf_address === d.rf_address);
  if (j >= 0) state.unlisted = state.unlisted.map((x, k) => (k === j ? d : x));
}

// updateBattery applies a battery event, a device's level that changed
// (docs/api-v1.md, "battery"), as when new batteries were marked: the
// device takes the event's level, its other values as the page has them,
// so that its row's badge follows before the next device list. A device
// without a state has no level.
function updateBattery(ev) {
  const level = ev.summary && ev.summary.level;
  if (!ev.device || !level) return;
  const all = state.rooms.flatMap((r) => r.devices).concat(state.unlisted);
  const d = all.find((x) => x.rf_address === ev.device);
  if (d && d.state && d.battery !== level) updateDevice({ ...d, battery: level });
}

// --- results ----------------------------------------------------------

function resultText(label, res) {
  if (res && res.discarded) return t("result.discarded", { label });
  let text = t("result.ok", { label });
  if (res && res.budget) text = t("result.budget", { text, duty: res.budget.duty_cycle_pct, slots: res.budget.free_memory_slots });
  return text;
}

function errorText(label, e) {
  let text = t("result.error", { label, error: e.message });
  if (e.data && e.data.no_state) {
    text += " " + t("result.noState", { devices: e.data.no_state.join(", ") });
  }
  return text;
}

// modeName is a device mode as a word, typeName a device type.
function modeName(mode) {
  const key = "mode.name." + mode;
  const s = t(key);
  return s === key ? mode : s;
}

// --- writes not confirmed ---------------------------------------------

function typeName(type) {
  const key = "type." + type;
  const s = t(key);
  return s === key ? type.replace(/_/g, " ") : s;
}

// showTileResult says how an action went on the room's tile (roomUI's
// result, MassimoParts.rooms.result), or on the page's message line when
// the tile is gone; a success fades.
function showTileResult(roomId, text, ok) {
  if (!MassimoParts.rooms.result(roomId, text, ok)) showMessage(text, ok);
}

// --- actions ----------------------------------------------------------

function stepSetpoint(room, delta) {
  const ui = roomUI(room);
  const s = roomState(room);
  const from = ui.pending !== null ? ui.pending : (s && s.setpoint !== undefined ? s.setpoint : DEFAULT_TEMP);
  ui.pending = clampTemp(from + delta);
  if (ui.timer) clearTimeout(ui.timer);
  ui.timer = setTimeout(() => flushSetpoint(room), SEND_DELAY_MS);
}

function cancelSetpoint(room) {
  const ui = roomUI(room);
  if (ui.timer) clearTimeout(ui.timer);
  ui.timer = null;
  ui.pending = null;
}

// dropPendingSetpoints drops every tile's stepped setpoint that still
// waits for its time to be sent, as the sign-in opens (askLogin,
// login.js): sent after it, it would go out under whoever signs in. Its
// tile says it was not sent, as a write that met the 401; one already on
// its way gets its own answer.
function dropPendingSetpoints() {
  for (const room of state.rooms) {
    const ui = state.ui[room.id];
    if (!ui || !ui.timer || ui.pending === null) continue;
    const label = t("label.manual", { temp: fmtSetpoint(ui.pending) });
    cancelSetpoint(room);
    showTileResult(room.id, t("result.error", { label, error: notSentText() }), false); // app.js
  }
}

// flushSetpoint sends the stepper's value, which stays shown, with Save
// turning, until the answer; a value whose time comes while another
// write of the tile runs waits for that one's answer.
async function flushSetpoint(room) {
  const ui = roomUI(room);
  if (ui.timer) clearTimeout(ui.timer);
  ui.timer = null;
  if (ui.pending === null) return;
  if (ui.sending) {
    ui.timer = setTimeout(() => flushSetpoint(room), SEND_DELAY_MS);
    return;
  }
  const temp = ui.pending;
  await tileWrite(room, "setpoint", async () => {
    try {
      return await setpointRequest(room, { mode: "manual", temp }, t("label.manual", { temp: fmtSetpoint(temp) }));
    } finally {
      ui.pending = null;
    }
  });
}

// awaitMembers marks a setpoint the Cube took for the room as waiting
// for the room's thermostats to show it: res is the Cube's answer, body
// what was sent, its mode and the temperature where one was given. The
// tile does not show its members' setpoints meanwhile (Members of
// ui/src/rooms/Tile.tsx), as a room command reaches the devices one by
// one. Only a write the Cube took replaces a wait: one that failed, the
// Cube discarded or that was never sent leaves an earlier wait as it
// is, and a boost, which changes no setpoint the line shows, starts
// none. The wait ends once every thermostat of the room shows it
// (settleAwaiting), after AWAIT_MS, or with the room's next write. A
// write with a temperature that every thermostat shows already ends at
// once. One without, such as AUTO, records each thermostat's setpoint
// and when the server received it (before): the thermostats show the
// mode before they take its setpoints, so it ends at a later state event
// alone, and only once they show setpoints taken since.
function awaitMembers(room, body, res) {
  if (!res || res.discarded || !body || !body.mode || body.mode === "boost") return;
  const ui = roomUI(room);
  if (ui.awaitTimer) clearTimeout(ui.awaitTimer);
  const want = typeof body.temp === "number" ? { mode: body.mode, setpoint: body.temp } : { mode: body.mode, before: {} };
  if (want.before) {
    for (const d of room.devices) {
      if (isThermostat(d) && d.state) want.before[d.rf_address] = { setpoint: d.state.setpoint, at: received(d, "setpoint") };
    }
  }
  ui.awaiting = want;
  ui.awaitTimer = setTimeout(() => {
    if (ui.awaiting !== want) return;
    ui.awaiting = null;
    ui.awaitTimer = null;
  }, AWAIT_MS);
  if (want.setpoint !== undefined) settleAwaiting(room, false);
}

// settleAwaiting ends the wait of a setpoint written to the room
// (awaitMembers) once every thermostat of the room with a state shows
// it: its mode and the temperature sent. A write without a temperature
// ends at a state event alone (event), once the setpoints agree and they
// are new since the write: at least one differs from its thermostat's
// before it, or every one came later than its before (received); a state
// event of another kind (a valve, a window contact, a configuration)
// while they still show the setpoints from before ends nothing.
function settleAwaiting(room, event) {
  const ui = state.ui[room.id];
  const want = ui && ui.awaiting;
  if (!want) return;
  const ths = room.devices.filter((d) => isThermostat(d) && d.state);
  if (!ths.every((d) => d.state.mode === want.mode)) return;
  if (want.setpoint !== undefined && !ths.every((d) => d.state.setpoint === want.setpoint)) return;
  if (want.setpoint === undefined) {
    if (!event || new Set(ths.map((d) => d.state.setpoint)).size > 1) return;
    const was = (d) => want.before[d.rf_address];
    const changed = ths.some((d) => was(d) && d.state.setpoint !== was(d).setpoint);
    const later = (d) => {
      const at = Date.parse(received(d, "setpoint") || ""), then = Date.parse((was(d) && was(d).at) || "");
      return !isNaN(at) && (isNaN(then) || at > then);
    };
    if (!changed && !ths.every(later)) return;
  }
  if (ui.awaitTimer) clearTimeout(ui.awaitTimer);
  ui.awaitTimer = null;
  ui.awaiting = null;
}

// sendRoomSetpoint sends a mode or a setpoint of the tile as the write
// key (tileWrite).
function sendRoomSetpoint(room, body, label, key) {
  return tileWrite(room, key, () => setpointRequest(room, body, label));
}

async function setpointRequest(room, body, label) {
  showTileResult(room.id, t("result.waiting", { label }), true);
  const n = MassimoParts.session();
  try {
    const res = await api("PUT", base() + "/rooms/" + room.id + "/setpoint", body);
    if (!MassimoParts.stillSession(n)) return null; // answered after the session ended: no line on the next user's tile
    awaitMembers(room, body, res);
    showTileResult(room.id, resultText(label, res), !(res && res.discarded));
    return res;
  } catch (e) {
    if (!MassimoParts.stillSession(n)) return null;
    showTileResult(room.id, errorText(label, e), false);
    return null;
  } finally {
    if (MassimoParts.stillSession(n)) loadCubes().catch(() => {});
  }
}

async function setLock(d, locked) {
  const label = t(locked ? "label.lock" : "label.unlock", { name: deviceName(d) });
  const roomId = d.room_id;
  showTileResult(roomId, t("result.waiting", { label }), true);
  const n = MassimoParts.session();
  try {
    const res = await api("PUT", devicePath(d) + "/lock", { locked });
    if (!MassimoParts.stillSession(n)) return false; // answered after the session ended
    showTileResult(roomId, resultText(label, res), !(res && res.discarded));
  } catch (e) {
    if (!MassimoParts.stillSession(n)) return false;
    showTileResult(roomId, errorText(label, e), false);
  }
  loadCubes().catch(() => {});
  return true;
}

async function lockRoom(room, locked, el) {
  closeMenu(el);
  await tileWrite(room, locked ? "lock" : "unlock", async () => {
    for (const d of room.devices.filter(isThermostat)) {
      if (!(await setLock(d, locked))) return; // the session ended: the next device is not written
    }
  });
}

// lockDevice locks or unlocks one device from its row, as a write of
// its room's tile, or with its button busy for a device in no room.
function lockDevice(d, locked, btn) {
  const room = state.rooms.find((r) => r.id === d.room_id);
  if (room) return tileWrite(room, "device:" + d.rf_address, () => setLock(d, locked));
  return whileBusy(btn, () => setLock(d, locked));
}

async function deleteRoom(room) {
  const n = MassimoParts.session();
  try {
    await api("DELETE", base() + "/rooms/" + room.id);
    if (!MassimoParts.stillSession(n)) return;
    showMessage(t("room.deleted", { name: room.name }), true);
  } catch (e) {
    if (!MassimoParts.stillSession(n)) return;
    showMessage(t("room.deleteFailed", { name: room.name, error: e.message }), false);
  }
  await loadRooms();
}

// closePanels closes every inline panel (vacation, delete confirm,
// House), for the Escape key; dropdown (components.js) closes the menus.
function closePanels() {
  if (MassimoParts.rooms.houseOpen()) MassimoParts.rooms.closeHouse();
  for (const room of state.rooms) {
    const ui = state.ui[room.id];
    if (!ui) continue;
    if (ui.vacation || ui.confirm) {
      ui.vacation = false;
      ui.confirm = false;
    }
  }
}

// --- wiring -----------------------------------------------------------

// initRooms wires what holds for every room tile: Escape closes the
// open panels.
function initRooms() {
  document.addEventListener("keydown", (ev) => {
    if (ev.key !== "Escape") return;
    closePanels();
  });
}
