// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

// The Massimo web UI, tiles: one tile per room with the setpoint
// stepper, the mode buttons and the inline panels (vacation, delete),
// the House tile with its panel, the device list, and the actions
// behind them. Uses the state, api and helpers of app.js.
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
  if (!state.ui[room.id]) state.ui[room.id] = { open: false, pending: null, timer: null, vacation: false, confirm: false };
  return state.ui[room.id];
}

function clampTemp(v) {
  return Math.min(MAX_TEMP, Math.max(MIN_TEMP, Math.round(v * 2) / 2));
}

// apiUntil turns a datetime-local value into the API's "YYYY-MM-DD HH:MM".
function apiUntil(value) {
  return (value || "").replace("T", " ").slice(0, 16);
}

// --- rendering --------------------------------------------------------

function renderRooms() {
  const section = $("rooms");
  clear(section);
  for (const r of state.rooms) section.appendChild(renderRoom(r));
  $("house").classList.toggle("hidden", !state.rooms.some(roomThermostat));
  renderHouseRooms();
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
  el.querySelector(".temp-is").textContent = actual === undefined ? "–" : actual.toFixed(1) + "°";
  el.querySelector(".temp-is").title = actual === undefined ? "no device of this room reports a measured temperature" : "measured temperature";

  renderNotReady(el, room);
  renderSetpoint(el, room);
  renderMenu(el, room);
  renderVacation(el, room);
  renderDeleteConfirm(el, room);

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
  const holidayBtn = el.querySelector(".mode-holiday");
  autoBtn.classList.toggle("active", mode === "auto");
  manuBtn.classList.toggle("active", mode === "manual" || mode === "boost");
  holidayBtn.classList.toggle("active", mode === "vacation");
  holidayBtn.addEventListener("click", () => openVacation(room, el));
  autoBtn.addEventListener("click", () => sendRoomSetpoint(room, { mode: "auto" }, "auto"));
  manuBtn.addEventListener("click", () => {
    // The displayed setpoint, or the stepper's starting value when the
    // room has not reported one yet.
    const temp = shown !== null ? shown : DEFAULT_TEMP;
    sendRoomSetpoint(room, { mode: "manual", temp }, "manual " + temp.toFixed(1) + " °C");
  });
  const vac = el.querySelector(".mode-vacation");
  if (mode === "vacation") {
    const until = s.until || "?";
    holidayBtn.title = "holiday until " + until;
    vac.appendChild(textEl("span", "\u{1F9F3}", "suitcase"));
    vac.appendChild(document.createTextNode(" until " + until));
    vac.title = "holiday until " + until;
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
    del.addEventListener("click", () => {
      closeMenu(el);
      roomUI(room).confirm = true;
      rerenderRoom(room);
    });
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

// renderVacation wires the inline vacation panel of a tile.
function renderVacation(el, room) {
  const ui = roomUI(room);
  const panel = el.querySelector(".vacation-panel");
  panel.classList.toggle("hidden", !ui.vacation);
  const temp = panel.querySelector(".vacation-temp");
  const until = panel.querySelector(".vacation-until");
  if (ui.vacation) {
    const s = roomState(room);
    if (!temp.dataset.set) {
      temp.value = ui.vacationTemp !== undefined ? ui.vacationTemp : (s && s.setpoint !== undefined ? s.setpoint : 17);
      until.value = ui.vacationUntil || tomorrowMorning();
      temp.dataset.set = "1";
    }
  }
  const keep = () => {
    ui.vacationTemp = parseFloat(temp.value);
    ui.vacationUntil = until.value;
  };
  temp.addEventListener("change", keep);
  until.addEventListener("change", keep);
  panel.querySelector(".vacation-minus").addEventListener("click", () => {
    temp.value = clampTemp(parseFloat(temp.value) - STEP);
    keep();
  });
  panel.querySelector(".vacation-plus").addEventListener("click", () => {
    temp.value = clampTemp(parseFloat(temp.value) + STEP);
    keep();
  });
  panel.querySelector(".vacation-cancel").addEventListener("click", () => closeVacation(room));
  panel.addEventListener("submit", (ev) => {
    ev.preventDefault();
    keep();
    const t = clampTemp(parseFloat(temp.value));
    const u = apiUntil(until.value);
    if (!u) return;
    closeVacation(room);
    sendRoomSetpoint(room, { mode: "vacation", temp: t, until: u }, "vacation " + t.toFixed(1) + " °C until " + u);
  });
}

function openVacation(room, el) {
  closeMenu(el);
  const ui = roomUI(room);
  ui.vacation = true;
  ui.vacationTemp = undefined;
  ui.vacationUntil = undefined;
  rerenderRoom(room);
  const tile = document.querySelector('.room[data-id="' + room.id + '"]');
  const input = tile && tile.querySelector(".vacation-until");
  if (input) input.focus();
}

function closeVacation(room) {
  const ui = roomUI(room);
  ui.vacation = false;
  ui.vacationTemp = undefined;
  ui.vacationUntil = undefined;
  rerenderRoom(room);
}

// renderDeleteConfirm wires the one-line confirmation of a room deletion.
function renderDeleteConfirm(el, room) {
  const ui = roomUI(room);
  const box = el.querySelector(".delete-confirm");
  box.classList.toggle("hidden", !ui.confirm);
  box.querySelector(".delete-no").addEventListener("click", () => {
    ui.confirm = false;
    rerenderRoom(room);
  });
  box.querySelector(".delete-yes").addEventListener("click", () => {
    ui.confirm = false;
    deleteRoom(room);
  });
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
  try {
    await api("DELETE", base() + "/rooms/" + room.id);
    showMessage("Room " + room.name + " deleted.", true);
  } catch (e) {
    showMessage("Delete room " + room.name + ": " + e.message, false);
  }
  await loadRooms();
}

// --- house ------------------------------------------------------------

function housePanel() {
  return $("house-panel");
}

function houseMode() {
  const checked = document.querySelector('input[name="house-mode"]:checked');
  return checked ? checked.value : "auto";
}

function houseDuration() {
  const checked = document.querySelector('input[name="house-duration"]:checked');
  return checked ? checked.value : "permanent";
}

// renderHouseRooms fills the room checkboxes of the House panel.
function renderHouseRooms() {
  const box = $("house-rooms");
  const wasChecked = {};
  for (const cb of box.querySelectorAll("input[data-room]")) wasChecked[cb.dataset.room] = cb.checked;
  clear(box);
  const all = document.createElement("label");
  const allCb = document.createElement("input");
  allCb.type = "checkbox";
  allCb.id = "house-all";
  allCb.checked = true;
  all.appendChild(allCb);
  all.appendChild(document.createTextNode(" All rooms"));
  box.appendChild(all);
  for (const room of state.rooms) {
    if (!roomThermostat(room)) continue;
    const label = document.createElement("label");
    const cb = document.createElement("input");
    cb.type = "checkbox";
    cb.dataset.room = String(room.id);
    cb.checked = wasChecked[room.id] === undefined ? true : wasChecked[room.id];
    cb.addEventListener("change", () => {
      allCb.checked = [...box.querySelectorAll("input[data-room]")].every((x) => x.checked);
    });
    label.appendChild(cb);
    label.appendChild(document.createTextNode(" " + room.name));
    box.appendChild(label);
  }
  allCb.checked = [...box.querySelectorAll("input[data-room]")].every((x) => x.checked);
  allCb.addEventListener("change", () => {
    for (const cb of box.querySelectorAll("input[data-room]")) cb.checked = allCb.checked;
  });
}

// updateHousePanel shows or hides the parts that depend on the mode.
function updateHousePanel() {
  const mode = houseMode();
  $("house-temp-row").classList.toggle("hidden", mode !== "manual");
  $("house-duration-row").classList.toggle("hidden", mode === "auto");
  $("house-until").disabled = houseDuration() !== "until";
}

function openHouse(mode) {
  const panel = housePanel();
  for (const r of document.querySelectorAll('input[name="house-mode"]')) r.checked = r.value === mode;
  if (!$("house-until").value) $("house-until").value = tomorrowMorning();
  const first = state.rooms.find(roomThermostat);
  const s = first ? roomState(first) : null;
  if (s && s.setpoint !== undefined) $("house-temp").value = s.setpoint;
  $("house-result").classList.add("hidden");
  panel.classList.remove("hidden");
  updateHousePanel();
  const focus = mode === "manual" ? $("house-temp") : panel.querySelector('input[name="house-mode"]:checked');
  if (focus) focus.focus();
}

function closeHouse() {
  housePanel().classList.add("hidden");
}

// sendHouse applies the panel: one room command after another, with the
// per-room results in the panel.
async function sendHouse() {
  const mode = houseMode();
  const until = houseDuration() === "until" && mode !== "auto" ? apiUntil($("house-until").value) : "";
  const manualTemp = clampTemp(parseFloat($("house-temp").value) || DEFAULT_TEMP);
  const ids = [...document.querySelectorAll("#house-rooms input[data-room]:checked")].map((cb) => Number(cb.dataset.room));
  const out = $("house-result");
  out.classList.remove("hidden");
  out.className = "tile-result ok";
  if (ids.length === 0) {
    out.textContent = "No room selected.";
    return;
  }
  out.textContent = "Sending …";
  const results = [];
  let failed = false;
  for (const room of state.rooms.filter((r) => ids.includes(r.id))) {
    const t = roomThermostat(room);
    if (!t) continue;
    try {
      let temp = manualTemp;
      if (mode === "eco" || mode === "comfort") {
        const detail = await deviceDetail(t.rf_address);
        const cfg = detail.thermostat || detail.wall_thermostat || {};
        temp = mode === "eco" ? cfg.eco_temp : cfg.comfort_temp;
        if (temp === undefined) throw new Error("no " + mode + " temperature configured");
      }
      let body = { mode: "auto" };
      if (mode !== "auto") body = until ? { mode: "vacation", temp, until } : { mode: "manual", temp };
      const res = await api("PUT", base() + "/rooms/" + room.id + "/setpoint", body);
      const what = mode === "auto" ? "auto" : temp.toFixed(1) + " °C" + (until ? " until " + until : "");
      results.push(room.name + ": " + (res && res.discarded ? "discarded" : what + " ok"));
      if (res && res.discarded) failed = true;
    } catch (e) {
      results.push(room.name + ": " + e.message);
      failed = true;
    }
    out.textContent = results.join("\n");
  }
  out.className = "tile-result " + (failed ? "bad" : "ok");
  loadCubes().catch(() => {});
}

// --- wiring -----------------------------------------------------------

function initTiles() {
  $("house-eco").addEventListener("click", () => openHouse("eco"));
  $("house-auto").addEventListener("click", () => openHouse("auto"));
  $("house-comfort").addEventListener("click", () => openHouse("comfort"));
  $("house-manu").addEventListener("click", () => openHouse("manual"));
  $("house-cancel").addEventListener("click", closeHouse);
  housePanel().addEventListener("submit", (ev) => {
    ev.preventDefault();
    sendHouse();
  });
  housePanel().addEventListener("change", updateHousePanel);
  $("house-temp-minus").addEventListener("click", () => {
    $("house-temp").value = clampTemp(parseFloat($("house-temp").value) - STEP);
  });
  $("house-temp-plus").addEventListener("click", () => {
    $("house-temp").value = clampTemp(parseFloat($("house-temp").value) + STEP);
  });
  document.addEventListener("click", (ev) => {
    // Clicking outside closes an open menu, and the House panel once
    // it has sent.
    if (!ev.target.closest("details.menu")) {
      for (const m of document.querySelectorAll("details.menu[open]")) m.open = false;
    }
    const panel = housePanel();
    if (!panel.classList.contains("hidden") && !ev.target.closest("#house") && !$("house-result").classList.contains("hidden")) {
      closeHouse();
    }
  });
}
