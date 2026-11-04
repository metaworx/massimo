// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

// The Massimo web UI, tiles: one tile per room with the setpoint
// stepper and the mode buttons, the House tile, the device list, and
// the actions behind them. Uses the state, api and helpers of app.js.
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
  const t = roomThermostat(room);
  return t && t.state ? t.state : null;
}

function roomActual(room) {
  const wall = room.devices.find((d) => d.type === "wall_thermostat" && d.state && d.state.actual_temp !== undefined);
  if (wall) return wall.state.actual_temp;
  const rad = room.devices.find((d) => isThermostat(d) && d.state && d.state.actual_temp !== undefined);
  return rad ? rad.state.actual_temp : undefined;
}

function roomUI(room) {
  if (!state.ui[room.id]) state.ui[room.id] = { open: false, pending: null, timer: null };
  return state.ui[room.id];
}

function clampTemp(v) {
  return Math.min(MAX_TEMP, Math.max(MIN_TEMP, Math.round(v * 2) / 2));
}

// --- rendering --------------------------------------------------------

function renderRooms() {
  const section = $("rooms");
  clear(section);
  for (const r of state.rooms) section.appendChild(renderRoom(r));
  $("house").classList.toggle("hidden", !state.rooms.some(roomThermostat));
  const unl = $("unlisted");
  const list = $("unlisted-devices");
  clear(list);
  if (state.unlisted.length) {
    for (const d of state.unlisted) list.appendChild(renderDevice(d));
    unl.classList.remove("hidden");
  } else {
    unl.classList.add("hidden");
  }
}

function renderRoom(room) {
  const el = $("room-template").content.firstElementChild.cloneNode(true);
  const ui = roomUI(room);
  el.dataset.id = room.id;
  el.querySelector(".room-name").textContent = room.name;
  el.querySelector(".room-name-btn").addEventListener("click", () => toggleDevices(room));
  const actual = roomActual(room);
  el.querySelector(".room-actual").textContent = actual === undefined ? "" : actual.toFixed(1) + "°";

  renderNotReady(el, room);
  renderSetpoint(el, room);
  renderMenu(el, room);

  const devs = el.querySelector(".devices");
  if (ui.open) {
    devs.classList.remove("hidden");
    fillDevices(devs, room);
  }
  return el;
}

function renderNotReady(el, room) {
  const notReady = el.querySelector(".not-ready");
  if (room.ready) return;
  const silent = room.devices.filter((d) => !d.reported);
  notReady.textContent = "Room commands are discarded until every device has reported";
  if (silent.length) {
    notReady.appendChild(document.createTextNode(": "));
    silent.forEach((d, i) => {
      if (i > 0) notReady.appendChild(document.createTextNode(", "));
      notReady.appendChild(textEl("span", d.name || d.rf_address));
      if (d.name) notReady.appendChild(textEl("span", " " + d.rf_address, "mono expert-only"));
    });
  }
  notReady.appendChild(document.createTextNode("."));
  notReady.classList.remove("hidden");
}

function renderSetpoint(el, room) {
  const ui = roomUI(room);
  const t = roomThermostat(room);
  const s = roomState(room);
  const row = el.querySelector(".setpoint-row");
  const modeRow = el.querySelector(".mode-row");
  if (!t) {
    row.classList.add("hidden");
    modeRow.classList.add("hidden");
    return;
  }
  const big = el.querySelector(".setpoint-big");
  const current = s && s.setpoint !== undefined ? s.setpoint : null;
  const shown = ui.pending !== null ? ui.pending : current;
  big.textContent = shown === null ? "–" : shown.toFixed(1) + "°";
  big.classList.toggle("pending", ui.pending !== null);
  el.querySelector(".step-minus").addEventListener("click", () => stepSetpoint(room, -STEP));
  el.querySelector(".step-plus").addEventListener("click", () => stepSetpoint(room, STEP));
  const pendingRow = el.querySelector(".pending-row");
  pendingRow.classList.toggle("hidden", ui.pending === null);
  el.querySelector(".pending-save").addEventListener("click", () => flushSetpoint(room));
  el.querySelector(".pending-cancel").addEventListener("click", () => cancelSetpoint(room));

  const mode = s ? s.mode : null;
  const autoBtn = el.querySelector(".mode-auto");
  const manuBtn = el.querySelector(".mode-manual");
  autoBtn.classList.toggle("active", mode === "auto");
  manuBtn.classList.toggle("active", mode === "manual" || mode === "boost");
  autoBtn.addEventListener("click", () => sendRoomSetpoint(room, { mode: "auto" }, "auto"));
  manuBtn.addEventListener("click", () => {
    // The displayed setpoint, or the stepper's starting value when the
    // room has not reported one yet.
    const temp = shown !== null ? shown : DEFAULT_TEMP;
    sendRoomSetpoint(room, { mode: "manual", temp }, "manual " + temp.toFixed(1) + " °C");
  });
  const vac = el.querySelector(".mode-vacation");
  if (mode === "vacation") {
    vac.textContent = "vacation until " + (s.until || "?");
    vac.classList.remove("hidden");
  }
}

function renderMenu(el, room) {
  const thermostats = room.devices.filter(isThermostat);
  el.querySelector(".menu-devices").addEventListener("click", () => toggleDevices(room, el));
  const vacation = el.querySelector(".menu-vacation");
  const lock = el.querySelector(".menu-lock");
  const unlock = el.querySelector(".menu-unlock");
  const del = el.querySelector(".menu-delete");
  if (thermostats.length) {
    vacation.addEventListener("click", () => openVacation(room, el));
    lock.addEventListener("click", () => lockRoom(room, true, el));
    unlock.addEventListener("click", () => lockRoom(room, false, el));
  } else {
    vacation.classList.add("hidden");
    lock.classList.add("hidden");
    unlock.classList.add("hidden");
  }
  if (room.devices.length === 0) {
    del.addEventListener("click", () => deleteRoom(room));
  } else {
    del.classList.add("hidden");
  }
  el.querySelector(".menu").addEventListener("toggle", (ev) => {
    // One open menu at a time.
    if (ev.target.open) {
      for (const other of document.querySelectorAll("details.menu[open]")) {
        if (other !== ev.target) other.open = false;
      }
    }
  });
}

function closeMenu(el) {
  const m = el && el.querySelector("details.menu");
  if (m) m.open = false;
}

// --- devices ----------------------------------------------------------

function deviceStateText(d) {
  if (!d.reported) return ["not reported since the cube restarted"];
  const parts = [];
  const s = d.state || {};
  if (s.window_open !== undefined) parts.push(s.window_open ? "window open" : "window closed");
  if (s.mode && isThermostat(d)) parts.push(s.mode);
  if (s.setpoint !== undefined) parts.push("set " + fmtTemp(s.setpoint));
  if (s.actual_temp !== undefined) parts.push("actual " + fmtTemp(s.actual_temp));
  if (s.valve_pct !== undefined) parts.push("valve " + s.valve_pct + " %");
  if (s.until) parts.push("until " + s.until);
  if (parts.length === 0) parts.push("reported");
  return parts;
}

function deviceFlags(d) {
  const s = d.state || {};
  const flags = [];
  if (s.battery_low) flags.push(["battery low", "bad"]);
  if (s.error) flags.push(["transmit error", "bad"]);
  if (s.link_error) flags.push(["link error", "warn"]);
  if (s.panel_locked) flags.push(["locked", "warn"]);
  return flags;
}

function renderDevice(d) {
  const el = $("device-template").content.firstElementChild.cloneNode(true);
  el.dataset.rf = d.rf_address;
  el.querySelector(".device-name").textContent = d.name || "(unnamed)";
  el.querySelector(".device-type").textContent = d.type.replace(/_/g, " ");
  el.querySelector(".device-rf").textContent = d.rf_address;
  const st = el.querySelector(".device-state");
  for (const p of deviceStateText(d)) st.appendChild(textEl("span", p));
  if (isThermostat(d)) {
    const offset = textEl("span", "", "device-offset muted");
    st.appendChild(offset);
    deviceDetail(d.rf_address).then((detail) => {
      const cfg = detail.thermostat || detail.wall_thermostat;
      if (cfg && cfg.temp_offset !== undefined) {
        offset.textContent = "offset " + (cfg.temp_offset > 0 ? "+" : "") + cfg.temp_offset.toFixed(1) + " °C";
      }
    }).catch(() => {});
  }
  const fl = el.querySelector(".device-flags");
  for (const [label, cls] of deviceFlags(d)) fl.appendChild(textEl("span", label, "flag " + cls));
  const actions = el.querySelector(".device-actions");
  if (isThermostat(d)) {
    const locked = !!(d.state && d.state.panel_locked);
    actions.appendChild(button(locked ? "Unlock" : "Lock", "", () => setLock(d, !locked)));
  }
  return el;
}

function fillDevices(container, room) {
  clear(container);
  if (room.devices.length === 0) {
    container.appendChild(textEl("p", "No devices in this room.", "muted"));
    return;
  }
  for (const d of room.devices) container.appendChild(renderDevice(d));
}

function toggleDevices(room, el) {
  const ui = roomUI(room);
  ui.open = !ui.open;
  closeMenu(el);
  rerenderRoom(room);
}

// rerenderRoom replaces a room's tile in place, keeping its UI state.
function rerenderRoom(room) {
  const old = document.querySelector('.room[data-id="' + room.id + '"]');
  if (old) old.replaceWith(renderRoom(room));
}

// updateDevice applies a state event to the room or the unlisted list.
function updateDevice(d) {
  for (const r of state.rooms) {
    const i = r.devices.findIndex((x) => x.rf_address === d.rf_address);
    if (i >= 0) {
      r.devices[i] = d;
      r.ready = r.devices.every((x) => x.reported);
      rerenderRoom(r);
      return;
    }
  }
  const j = state.unlisted.findIndex((x) => x.rf_address === d.rf_address);
  if (j >= 0) {
    state.unlisted[j] = d;
    const old = document.querySelector('#unlisted-devices .device[data-rf="' + d.rf_address + '"]');
    if (old) old.replaceWith(renderDevice(d));
  }
}

// --- results ----------------------------------------------------------

function resultText(label, res) {
  let text = label + ": ok";
  if (res && res.budget) text += " (duty cycle " + res.budget.duty_cycle_pct + " %, " + res.budget.free_memory_slots + " free slots)";
  if (res && res.discarded) text = label + ": discarded by the cube";
  return text;
}

function errorText(label, e) {
  let text = label + ": " + e.message;
  if (e.data && e.data.no_state) {
    text += " Devices without state: " + e.data.no_state.join(", ") + ". Open and close the window, or turn the thermostat's wheel, then retry.";
  }
  return text;
}

function showTileResult(roomId, text, ok) {
  const tile = document.querySelector('.room[data-id="' + roomId + '"]');
  if (!tile) {
    showMessage(text, ok);
    return;
  }
  const p = tile.querySelector(".tile-result");
  p.textContent = text;
  p.className = "tile-result " + (ok ? "ok" : "bad");
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

async function flushSetpoint(room) {
  const ui = roomUI(room);
  if (ui.timer) clearTimeout(ui.timer);
  ui.timer = null;
  if (ui.pending === null) return;
  const temp = ui.pending;
  ui.pending = null;
  rerenderRoom(room);
  await sendRoomSetpoint(room, { mode: "manual", temp }, "manual " + temp.toFixed(1) + " °C");
}

async function sendRoomSetpoint(room, body, label) {
  showTileResult(room.id, label + ": waiting for the cube …", true);
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
  const label = (locked ? "lock " : "unlock ") + deviceName(d);
  const roomId = d.room_id;
  showTileResult(roomId, label + ": waiting for the cube …", true);
  try {
    const res = await api("PUT", base() + "/devices/" + d.rf_address + "/lock", { locked });
    showTileResult(roomId, resultText(label, res), !(res && res.discarded));
  } catch (e) {
    showTileResult(roomId, errorText(label, e), false);
  }
  loadCubes().catch(() => {});
}

async function lockRoom(room, locked, el) {
  closeMenu(el);
  for (const d of room.devices.filter(isThermostat)) {
    await setLock(d, locked);
  }
}

async function deleteRoom(room) {
  if (!window.confirm("Delete the empty room " + room.name + " from the cube?")) return;
  try {
    await api("DELETE", base() + "/rooms/" + room.id);
    showMessage("Room " + room.name + " deleted.", true);
  } catch (e) {
    showMessage("Delete room " + room.name + ": " + e.message, false);
  }
  await loadRooms();
}

// --- vacation ---------------------------------------------------------

let vacationRoom = null;

function openVacation(room, el) {
  closeMenu(el);
  vacationRoom = room;
  const s = roomState(room);
  $("vacation-title").textContent = "Vacation for " + room.name;
  $("vacation-temp").value = s && s.setpoint !== undefined ? s.setpoint : 17;
  const d = new Date(Date.now() + 24 * 3600 * 1000);
  d.setMinutes(0, 0, 0);
  $("vacation-until").value = localDateTimeValue(d);
  $("vacation-dialog").showModal();
}

function localDateTimeValue(d) {
  const pad = (n) => String(n).padStart(2, "0");
  return d.getFullYear() + "-" + pad(d.getMonth() + 1) + "-" + pad(d.getDate()) + "T" + pad(d.getHours()) + ":" + pad(d.getMinutes());
}

function sendVacation() {
  const room = vacationRoom;
  if (!room) return;
  const temp = parseFloat($("vacation-temp").value);
  // datetime-local gives "YYYY-MM-DDTHH:MM"; the API wants "YYYY-MM-DD HH:MM".
  const until = $("vacation-until").value.replace("T", " ").slice(0, 16);
  sendRoomSetpoint(room, { mode: "vacation", temp, until }, "vacation " + temp.toFixed(1) + " °C until " + until);
}

// --- house ------------------------------------------------------------

// houseMode sets every room with a thermostat: auto, or manual at the
// room's eco or comfort temperature from its thermostat's configuration.
async function houseMode(mode) {
  const out = $("house-result");
  out.className = "tile-result ok";
  out.textContent = "Sending …";
  const results = [];
  for (const room of state.rooms) {
    const t = roomThermostat(room);
    if (!t) continue;
    let body = { mode: "auto" };
    try {
      if (mode !== "auto") {
        const detail = await deviceDetail(t.rf_address);
        const cfg = detail.thermostat || detail.wall_thermostat || {};
        const temp = mode === "eco" ? cfg.eco_temp : cfg.comfort_temp;
        if (temp === undefined) throw new Error("no " + mode + " temperature configured");
        body = { mode: "manual", temp };
      }
      const res = await api("PUT", base() + "/rooms/" + room.id + "/setpoint", body);
      results.push(room.name + (res && res.discarded ? " discarded" : " ok"));
    } catch (e) {
      results.push(room.name + " failed (" + e.message + ")");
    }
    out.textContent = results.join(", ");
  }
  if (results.length === 0) out.textContent = "No room with a thermostat.";
  out.className = "tile-result " + (results.some((r) => !r.endsWith(" ok")) ? "bad" : "ok");
  loadCubes().catch(() => {});
}

// --- wiring -----------------------------------------------------------

function initTiles() {
  $("house-eco").addEventListener("click", () => houseMode("eco"));
  $("house-auto").addEventListener("click", () => houseMode("auto"));
  $("house-comfort").addEventListener("click", () => houseMode("comfort"));
  $("vacation-cancel").addEventListener("click", () => $("vacation-dialog").close());
  $("vacation-form").addEventListener("submit", () => sendVacation());
  document.addEventListener("click", (ev) => {
    // Clicking outside closes an open menu.
    if (!ev.target.closest("details.menu")) {
      for (const m of document.querySelectorAll("details.menu[open]")) m.open = false;
    }
  });
}
