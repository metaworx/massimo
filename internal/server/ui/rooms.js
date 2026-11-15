// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

// The Massimo web UI, the rooms: a room's facts, the drawing of every
// room's tile into Rooms and again in place, a device's state event,
// the texts of a result, the actions behind a tile's controls, and what
// holds for every tile (Escape closes the open panels).
"use strict";

const STEP = 0.5;
const MIN_TEMP = 4.5;
const MAX_TEMP = 30.5;
const DEFAULT_TEMP = 20; // stepper start when the room has no setpoint yet
const SEND_DELAY_MS = 2000;
const RESULT_FADE_MS = 5000;

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

function roomUI(room) {
  if (!state.ui[room.id]) state.ui[room.id] = { open: false, pending: null, timer: null, vacation: false, confirm: false, config: null, deferred: false, sending: null, hold: false };
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

function renderRooms() {
  const section = $("rooms");
  // A tile whose configuration has the focus stays as it is, as for a
  // state event (updateDevice), and the others are drawn around it: a
  // redraw between the press and the release of a click (a reload after
  // a metadata event) swallowed the click, and one under the user's
  // typing took the focus away. It is drawn once the focus leaves the
  // configuration (renderRoom).
  const kept = state.rooms.find(configFocused);
  const keep = kept && section.querySelector('.room[data-id="' + kept.id + '"]');
  if (keep) {
    roomUI(kept).deferred = true;
    for (const el of [...section.children]) if (el !== keep) el.remove();
    let before = true;
    for (const r of state.rooms) {
      if (r === kept) before = false;
      else if (before) section.insertBefore(renderRoom(r), keep);
      else section.appendChild(renderRoom(r));
    }
  } else {
    clear(section);
    for (const r of state.rooms) section.appendChild(renderRoom(r));
  }
  $("house").classList.toggle("hidden", !state.rooms.some(roomThermostat));
  renderHouseRooms();
  const unl = $("unlisted");
  const list = $("unlisted-devices");
  clear(list);
  for (const rf of Object.keys(unlistedUI)) {
    if (!state.unlisted.some((d) => d.rf_address === rf)) delete unlistedUI[rf];
  }
  if (state.unlisted.length) {
    for (const d of state.unlisted) list.appendChild(renderUnlisted(d));
    unl.classList.remove("hidden");
  } else {
    unl.classList.add("hidden");
  }
  planWindowRedraw();
}

// rerenderRoom replaces a room's tile in place, keeping its UI state.
// Removing a tile blurs a focused input, whose change handler may ask
// for another render; that request is deferred until this one is done.
let rendering = false;
function rerenderRoom(room) {
  if (rendering) {
    queueMicrotask(() => rerenderRoom(room));
    return;
  }
  const old = document.querySelector('.room[data-id="' + room.id + '"]');
  if (!old) return;
  rendering = true;
  try {
    old.replaceWith(renderRoom(room));
  } finally {
    rendering = false;
  }
  planWindowRedraw();
}

// configFocused reports whether the focus is in a room tile's
// configuration.
function configFocused(room) {
  const area = document.querySelector('.room[data-id="' + room.id + '"] .config-area');
  return !!(area && area.contains(document.activeElement));
}

// updateDevice applies a state event to the room or the unlisted list.
// A tile whose configuration has the focus is not drawn again under the
// user's typing: the state is taken, and drawn once the focus leaves the
// configuration (renderRoom).
function updateDevice(d) {
  if (d.virtual) {
    const r = state.rooms.find((x) => x.id === d.room_id);
    if (!r) return;
    r.vwt = d;
    if (configFocused(r)) roomUI(r).deferred = true;
    else rerenderRoom(r);
    return;
  }
  for (const r of state.rooms) {
    const i = r.devices.findIndex((x) => x.rf_address === d.rf_address);
    if (i >= 0) {
      r.devices[i] = d;
      r.ready = r.devices.every((x) => x.reported);
      r.stale = r.devices.some((x) => x.stale);
      if (configFocused(r)) roomUI(r).deferred = true;
      else rerenderRoom(r);
      return;
    }
  }
  const j = state.unlisted.findIndex((x) => x.rf_address === d.rf_address);
  if (j >= 0) {
    state.unlisted[j] = d;
    const old = document.querySelector('#unlisted-devices .device[data-rf="' + d.rf_address + '"]');
    // A form being typed in keeps its row; the next render brings the state.
    if (old && !(unlistedUI[d.rf_address] || {}).form) old.replaceWith(renderUnlisted(d));
  }
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

function typeName(type) {
  const key = "type." + type;
  const s = t(key);
  return s === key ? type.replace(/_/g, " ") : s;
}

// showTileResult says how an action went on the room's tile (or the
// page's message line when the tile is gone); a success fades.
function showTileResult(roomId, text, ok) {
  const tile = document.querySelector('.room[data-id="' + roomId + '"]');
  if (!tile) {
    showMessage(text, ok);
    return;
  }
  const p = tile.querySelector(".tile-result");
  showResult(p, text, ok);
  if (ok) {
    setTimeout(() => {
      if (p.textContent === text) p.classList.add("hidden");
    }, RESULT_FADE_MS);
  }
}

// --- actions ----------------------------------------------------------

function stepSetpoint(room, delta) {
  const ui = roomUI(room);
  const s = roomState(room);
  const from = ui.pending !== null ? ui.pending : (s && s.setpoint !== undefined ? s.setpoint : DEFAULT_TEMP);
  ui.pending = clampTemp(from + delta);
  if (ui.timer) clearTimeout(ui.timer);
  ui.timer = setTimeout(() => flushSetpoint(room), SEND_DELAY_MS);
  rerenderRoom(room);
}

function cancelSetpoint(room) {
  const ui = roomUI(room);
  if (ui.timer) clearTimeout(ui.timer);
  ui.timer = null;
  ui.pending = null;
  rerenderRoom(room);
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
  try {
    const res = await api("PUT", base() + "/rooms/" + room.id + "/setpoint", body);
    showTileResult(room.id, resultText(label, res), !(res && res.discarded));
    return res;
  } catch (e) {
    showTileResult(room.id, errorText(label, e), false);
    return null;
  } finally {
    loadCubes().catch(() => {});
  }
}

async function setLock(d, locked) {
  const label = t(locked ? "label.lock" : "label.unlock", { name: deviceName(d) });
  const roomId = d.room_id;
  showTileResult(roomId, t("result.waiting", { label }), true);
  try {
    const res = await api("PUT", devicePath(d) + "/lock", { locked });
    showTileResult(roomId, resultText(label, res), !(res && res.discarded));
  } catch (e) {
    showTileResult(roomId, errorText(label, e), false);
  }
  loadCubes().catch(() => {});
}

async function lockRoom(room, locked, el) {
  closeMenu(el);
  await tileWrite(room, locked ? "lock" : "unlock", async () => {
    for (const d of room.devices.filter(isThermostat)) {
      await setLock(d, locked);
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
  try {
    await api("DELETE", base() + "/rooms/" + room.id);
    showMessage(t("room.deleted", { name: room.name }), true);
  } catch (e) {
    showMessage(t("room.deleteFailed", { name: room.name, error: e.message }), false);
  }
  await loadRooms();
}

// closePanels closes every inline panel (vacation, delete confirm,
// House), for the Escape key; dropdown (components.js) closes the menus.
function closePanels() {
  if (!housePanel().classList.contains("hidden")) closeHouse();
  for (const room of state.rooms) {
    const ui = state.ui[room.id];
    if (!ui) continue;
    if (ui.vacation || ui.confirm) {
      ui.vacation = false;
      ui.vacationTemp = undefined;
      ui.vacationUntil = undefined;
      ui.confirm = false;
      rerenderRoom(room);
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
