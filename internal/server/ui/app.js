// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

// The Massimo web UI: reads the JSON API, shows rooms and devices,
// sends setpoints and locks, and follows the server-sent events.
"use strict";

const state = {
  cubes: [],
  serial: null,
  rooms: [],
  unlisted: [],
  token: null,
  events: null,
  pendingAction: null,
};

const $ = (id) => document.getElementById(id);

// --- API -------------------------------------------------------------

function readToken() {
  try {
    return localStorage.getItem("massimo.token") || null;
  } catch (e) {
    return null;
  }
}

function storeToken(t) {
  try {
    localStorage.setItem("massimo.token", t);
  } catch (e) {
    // private window or storage blocked: the token lives for this page only
  }
}

async function api(method, path, body) {
  const headers = {};
  if (state.token) headers.Authorization = "Bearer " + state.token;
  if (body !== undefined) headers["Content-Type"] = "application/json";
  const resp = await fetch(path, { method, headers, body: body === undefined ? undefined : JSON.stringify(body) });
  if (resp.status === 401) {
    const t = await askToken();
    if (t) return api(method, path, body);
    throw new Error("missing or wrong token");
  }
  let data = null;
  const text = await resp.text();
  if (text) {
    try {
      data = JSON.parse(text);
    } catch (e) {
      data = { error: text };
    }
  }
  if (!resp.ok) {
    const err = new Error((data && data.error) || resp.status + " " + resp.statusText);
    err.data = data;
    throw err;
  }
  return data;
}

let tokenPromise = null;
function askToken() {
  if (tokenPromise) return tokenPromise;
  tokenPromise = new Promise((resolve) => {
    const dlg = $("token-dialog");
    const input = $("token-input");
    input.value = "";
    $("token-form").onsubmit = () => {
      state.token = input.value.trim();
      storeToken(state.token);
      tokenPromise = null;
      resolve(state.token);
    };
    dlg.showModal();
  });
  return tokenPromise;
}

// --- rendering helpers ----------------------------------------------

function clear(el) {
  while (el.firstChild) el.removeChild(el.firstChild);
}

function textEl(tag, text, cls) {
  const el = document.createElement(tag);
  el.textContent = text;
  if (cls) el.className = cls;
  return el;
}

function button(label, cls, onClick) {
  const b = document.createElement("button");
  b.type = "button";
  b.textContent = label;
  if (cls) b.className = cls;
  b.addEventListener("click", onClick);
  return b;
}

function showMessage(text, ok) {
  const m = $("message");
  m.textContent = text;
  m.className = "message " + (ok ? "ok" : "bad");
}

function fmtTemp(v) {
  return v === undefined || v === null ? "" : v.toFixed(1) + " °C";
}

function isThermostat(d) {
  return d.type === "radiator_thermostat" || d.type === "radiator_thermostat_plus" || d.type === "wall_thermostat";
}

// --- header -----------------------------------------------------------

function renderHeader(cube) {
  const conn = $("connection");
  conn.textContent = cube.connected ? "connected" : "disconnected";
  conn.className = "pill " + (cube.connected ? "ok" : "bad");
  const b = cube.budget || {};
  $("duty").textContent = b.duty_cycle_pct === undefined ? "–" : b.duty_cycle_pct + " %";
  $("slots").textContent = b.free_memory_slots === undefined ? "–" : String(b.free_memory_slots);
  $("queued").textContent = cube.queued === undefined ? "–" : String(cube.queued);
  $("clock").textContent = cube.cube_time || "–";
  const p = $("pending");
  const n = cube.pending_radiograms || 0;
  if (n > 0) {
    p.textContent = n + " radiogram" + (n === 1 ? "" : "s") + " not yet acknowledged, states may still change";
    p.classList.remove("hidden");
  } else {
    p.classList.add("hidden");
  }
}

function renderCubeSelect() {
  const sel = $("cube-select");
  clear(sel);
  for (const c of state.cubes) {
    const o = document.createElement("option");
    o.value = c.serial;
    o.textContent = c.serial + (c.connected ? "" : " (disconnected)");
    sel.appendChild(o);
  }
  sel.value = state.serial;
  sel.disabled = state.cubes.length < 2;
}

// --- devices and rooms ------------------------------------------------

function deviceStateText(d) {
  const parts = [];
  if (!d.reported) {
    parts.push("not reported since the cube restarted");
    return parts;
  }
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
  if (d.unlisted) flags.push(["not in a room", ""]);
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
  const fl = el.querySelector(".device-flags");
  for (const [label, cls] of deviceFlags(d)) fl.appendChild(textEl("span", label, "flag " + cls));
  const actions = el.querySelector(".device-actions");
  if (isThermostat(d)) {
    actions.appendChild(button("Setpoint", "", () => openSetpoint({ kind: "device", rf: d.rf_address, name: d.name })));
    const locked = !!(d.state && d.state.panel_locked);
    actions.appendChild(button(locked ? "Unlock" : "Lock", "", () => setLock(d, !locked)));
  }
  return el;
}

function renderRoom(room) {
  const el = $("room-template").content.firstElementChild.cloneNode(true);
  el.dataset.id = room.id;
  el.querySelector(".room-name").textContent = room.id + " · " + room.name;
  const notReady = el.querySelector(".not-ready");
  if (!room.ready) {
    const silent = room.devices.filter((d) => !d.reported).map((d) => (d.name || d.rf_address));
    notReady.textContent = "Room commands are discarded until every device has reported" + (silent.length ? ": " + silent.join(", ") : "") + ".";
    notReady.classList.remove("hidden");
  }
  el.querySelector(".room-setpoint").addEventListener("click", () => openSetpoint({ kind: "room", id: room.id, name: room.name }));
  const del = el.querySelector(".room-delete");
  if (room.devices.length === 0) {
    del.classList.remove("hidden");
    del.addEventListener("click", () => deleteRoom(room));
  }
  const devs = el.querySelector(".devices");
  for (const d of room.devices) devs.appendChild(renderDevice(d));
  return el;
}

function renderRooms() {
  const section = $("rooms");
  clear(section);
  for (const r of state.rooms) section.appendChild(renderRoom(r));
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

// updateDevice replaces one device's card in place after a state event.
function updateDevice(d) {
  const old = document.querySelector('.device[data-rf="' + d.rf_address + '"]');
  if (!old) return;
  for (const r of state.rooms) {
    const i = r.devices.findIndex((x) => x.rf_address === d.rf_address);
    if (i >= 0) r.devices[i] = d;
  }
  const j = state.unlisted.findIndex((x) => x.rf_address === d.rf_address);
  if (j >= 0) state.unlisted[j] = d;
  old.replaceWith(renderDevice(d));
}

// --- loading ----------------------------------------------------------

async function loadCubes() {
  state.cubes = (await api("GET", "/api/v1/cubes")) || [];
  if (!state.serial || !state.cubes.some((c) => c.serial === state.serial)) {
    state.serial = state.cubes.length ? state.cubes[0].serial : null;
  }
  renderCubeSelect();
  const cube = state.cubes.find((c) => c.serial === state.serial);
  if (cube) renderHeader(cube);
}

async function loadRooms() {
  if (!state.serial) return;
  const base = "/api/v1/cubes/" + encodeURIComponent(state.serial);
  const [rooms, devices] = await Promise.all([api("GET", base + "/rooms"), api("GET", base + "/devices")]);
  state.rooms = rooms || [];
  state.unlisted = (devices || []).filter((d) => d.unlisted || d.room_id === 0);
  renderRooms();
}

async function refresh() {
  try {
    await loadCubes();
    await loadRooms();
  } catch (e) {
    showMessage("Cannot load: " + e.message, false);
  }
}

// --- actions ----------------------------------------------------------

function openSetpoint(target) {
  state.pendingAction = target;
  $("setpoint-title").textContent = (target.kind === "room" ? "Room " : "Device ") + (target.name || target.rf || target.id);
  $("setpoint-mode").value = "manual";
  updateSetpointFields();
  $("setpoint-dialog").showModal();
}

function updateSetpointFields() {
  const mode = $("setpoint-mode").value;
  $("setpoint-temp-label").classList.toggle("hidden", false);
  $("setpoint-until-label").classList.toggle("hidden", mode !== "vacation");
}

function untilFromInput(v) {
  // datetime-local gives "YYYY-MM-DDTHH:MM"; the API wants "YYYY-MM-DD HH:MM".
  return v.replace("T", " ").slice(0, 16);
}

async function sendSetpoint() {
  const target = state.pendingAction;
  if (!target) return;
  const mode = $("setpoint-mode").value;
  const body = { mode };
  const temp = parseFloat($("setpoint-temp").value);
  if (mode !== "auto" || !isNaN(temp)) body.temp = temp;
  if (mode === "vacation") body.until = untilFromInput($("setpoint-until").value);
  const base = "/api/v1/cubes/" + encodeURIComponent(state.serial);
  const path = target.kind === "room" ? base + "/rooms/" + target.id + "/setpoint" : base + "/devices/" + target.rf + "/setpoint";
  await runAction("setpoint " + (target.name || target.rf || target.id), () => api("PUT", path, body));
}

async function setLock(d, locked) {
  const path = "/api/v1/cubes/" + encodeURIComponent(state.serial) + "/devices/" + d.rf_address + "/lock";
  await runAction((locked ? "lock " : "unlock ") + (d.name || d.rf_address), () => api("PUT", path, { locked }));
}

async function deleteRoom(room) {
  if (!window.confirm("Delete the empty room " + room.id + " " + room.name + " from the cube?")) return;
  const path = "/api/v1/cubes/" + encodeURIComponent(state.serial) + "/rooms/" + room.id;
  await runAction("delete room " + room.name, () => api("DELETE", path));
  await loadRooms();
}

async function runAction(label, fn) {
  showMessage(label + ": waiting for the cube …", true);
  try {
    const res = await fn();
    let text = label + ": ok";
    if (res && res.budget) text += " (duty cycle " + res.budget.duty_cycle_pct + " %, " + res.budget.free_memory_slots + " free slots)";
    if (res && res.discarded) text = label + ": discarded by the cube";
    showMessage(text, !(res && res.discarded));
  } catch (e) {
    let text = label + ": " + e.message;
    if (e.data && e.data.no_state) text += "\nDevices without state: " + e.data.no_state.join(", ") + ". Open and close the window, or turn the thermostat's wheel, then retry.";
    showMessage(text, false);
  }
  await loadCubes();
}

// --- events -----------------------------------------------------------

function connectEvents() {
  if (state.events) {
    state.events.close();
    state.events = null;
  }
  if (!state.serial) return;
  let url = "/api/v1/cubes/" + encodeURIComponent(state.serial) + "/events";
  if (state.token) url += "?token=" + encodeURIComponent(state.token);
  const es = new EventSource(url);
  state.events = es;
  es.addEventListener("state", (ev) => {
    const data = JSON.parse(ev.data);
    if (data.state) updateDevice(data.state);
  });
  es.addEventListener("hello", () => loadCubes());
  es.addEventListener("metadata", () => loadRooms());
  es.addEventListener("connected", () => refresh());
  es.addEventListener("disconnected", () => loadCubes());
  es.onerror = () => {
    // The browser retries by itself; make sure the header says so.
    loadCubes().catch(() => {});
  };
}

// --- wiring -----------------------------------------------------------

function init() {
  state.token = readToken();
  api("GET", "/healthz").then((h) => {
    $("version").textContent = h && h.version ? h.version : "";
  }).catch(() => {});
  $("cube-select").addEventListener("change", (ev) => {
    state.serial = ev.target.value;
    refresh().then(connectEvents);
  });
  $("setpoint-mode").addEventListener("change", updateSetpointFields);
  $("setpoint-cancel").addEventListener("click", () => $("setpoint-dialog").close());
  $("setpoint-form").addEventListener("submit", () => sendSetpoint());
  refresh().then(connectEvents);
  setInterval(() => loadCubes().catch(() => {}), 60000);
}

document.addEventListener("DOMContentLoaded", init);
