// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

// The Massimo web UI, the rooms: a room's facts, the plain names of the
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

// roomActualDevice is the device whose measured temperature stands for
// the room: its virtual wall thermostat, which the server computes from
// the wall thermostat, else the radiator thermostats' median or mean, so
// that every client shows the same; null while it has none.
function roomActualDevice(room) {
  const v = room.vwt;
  return v && v.state && v.state.actual_temp !== undefined ? v : null;
}

function roomActual(room) {
  const d = roomActualDevice(room);
  return d ? d.state.actual_temp : undefined;
}

// staleText is the warning for a device the API calls stale ("Not seen
// for 30 hours", from two days on in days), "" for any other.
function staleText(d) {
  if (!d.stale) return "";
  const ms = Date.parse(d.last_seen || "");
  const h = isNaN(ms) ? 24 : Math.max(0, Math.floor((Date.now() - ms) / 3600000));
  return h < 48 ? tn("stale.hours", h) : tn("stale.days", Math.floor(h / 24));
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
// result line (showTileResult).
function roomUI(room) {
  if (!state.ui[room.id]) state.ui[room.id] = MassimoParts.rooms.ui();
  return state.ui[room.id];
}

function clampTemp(v) {
  return Math.min(MAX_TEMP, Math.max(MIN_TEMP, Math.round(v * 2) / 2));
}

// apiUntil turns a datetime-local value into the API's "YYYY-MM-DD HH:MM".
function apiUntil(value) {
  return (value || "").replace("T", " ").slice(0, 16);
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
      return;
    }
  }
  const j = state.unlisted.findIndex((x) => x.rf_address === d.rf_address);
  if (j >= 0) state.unlisted = state.unlisted.map((x, k) => (k === j ? d : x));
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

// confirmFieldName names a value a write's confirmation waits for (the
// server's confirm.go): the state's mode, setpoint, vacation end, key
// lock and display, a configuration's by the log's names, a day of the
// week programme by its day.
function confirmFieldName(f) {
  const day = /^program\.([a-z]+)(\.2)?$/.exec(f);
  if (day) return t("confirm.program", { day: t("day." + day[1]) });
  if (f === "locked") return t("confirm.locked");
  if (f === "display_actual") return t("confirm.display");
  if (f === "mode" || f === "setpoint" || f === "until") return t("log.disagree.what." + f);
  const key = "log.cfg." + f;
  return t(key) === key ? f : t(key);
}

// confirmShown is what a device's state shows of a value its write waits
// for; "" for a configuration's value, which the state does not hold.
function confirmShown(d, f) {
  const s = d.state || {};
  switch (f) {
    case "mode":
      return s.mode ? modeName(s.mode) : "";
    case "setpoint":
      return s.setpoint !== undefined ? fmtTemp(s.setpoint) : "";
    case "until":
      return s.until ? fmtUntil(s.until) : "";
    case "locked":
      return t(s.panel_locked ? "confirm.lockedOn" : "confirm.lockedOff");
  }
  return "";
}

// confirmFieldsText lists what a device has not shown of the writes to
// it, each value with what the device shows where its state says:
// "mode (shows auto), setpoint (shows 20.0 °C)".
function confirmFieldsText(d) {
  const out = [];
  for (const f of (d.confirmation && d.confirmation.fields) || []) {
    const shown = confirmShown(d, f);
    const text = confirmFieldName(f) + (shown ? " (" + t("confirm.shows", { value: shown }) + ")" : "");
    if (!out.includes(text)) out.push(text); // a long day's two parts are one day
  }
  return out.join(", ");
}

// confirmItem is the mark of a device that has not shown a write by its
// deadline, an amber problem of its tile and of its row whose title says
// what it does not show; none while the write is pending, nor for a
// virtual wall thermostat, whose room's devices carry it.
function confirmItem(d) {
  const c = d.confirmation;
  if (!c || c.state !== "unconfirmed" || d.virtual) return null;
  return { kind: "unconfirmed", icon: "icon-unconfirmed", tone: "warn", text: t("badge.notConfirmed"),
    title: t("badge.notConfirmedTitle", { fields: confirmFieldsText(d) }) };
}

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
    const label = t("label.manual", { temp: fmtNum(ui.pending, 1) });
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
      return await setpointRequest(room, { mode: "manual", temp }, t("label.manual", { temp: fmtNum(temp, 1) }));
    } finally {
      ui.pending = null;
    }
  });
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
