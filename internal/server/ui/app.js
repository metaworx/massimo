// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

// The Massimo web UI, core: API access with the token, the header,
// loading of cubes and rooms, the event stream, expert mode. The
// tiles and their actions are in tiles.js.
"use strict";

const state = {
  cubes: [],
  serial: null,
  rooms: [],
  unlisted: [],
  token: null,
  events: null,
  expert: false,
  details: {}, // device detail by rf address, fetched on demand
  ui: {}, // per room id: {open, pending, timer}
};

const $ = (id) => document.getElementById(id);

// --- storage ----------------------------------------------------------

function readStored(key) {
  try {
    return localStorage.getItem(key);
  } catch (e) {
    return null;
  }
}

function store(key, value) {
  try {
    localStorage.setItem(key, value);
  } catch (e) {
    // private window or storage blocked: the value lives for this page only
  }
}

// --- API --------------------------------------------------------------

function base() {
  return "/api/v1/cubes/" + encodeURIComponent(state.serial);
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
      store("massimo.token", state.token);
      tokenPromise = null;
      resolve(state.token);
    };
    dlg.showModal();
  });
  return tokenPromise;
}

// deviceDetail fetches and caches a device's configuration.
async function deviceDetail(rf) {
  if (state.details[rf]) return state.details[rf];
  const d = await api("GET", base() + "/devices/" + rf);
  state.details[rf] = d;
  return d;
}

// --- helpers ----------------------------------------------------------

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

function deviceName(d) {
  return d.name || d.rf_address;
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
    p.textContent = state.expert
      ? n + " radiogram" + (n === 1 ? "" : "s") + " not yet acknowledged, states may still change"
      : n + " pending, states may still change";
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

function applyExpert() {
  document.body.classList.toggle("expert", state.expert);
  $("expert-toggle").checked = state.expert;
  const cube = state.cubes.find((c) => c.serial === state.serial);
  if (cube) renderHeader(cube);
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
  const [rooms, devices] = await Promise.all([api("GET", base() + "/rooms"), api("GET", base() + "/devices")]);
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

// --- events -----------------------------------------------------------

function connectEvents() {
  if (state.events) {
    state.events.close();
    state.events = null;
  }
  if (!state.serial) return;
  let url = base() + "/events";
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
  state.token = readStored("massimo.token");
  state.expert = readStored("massimo.expert") === "1";
  applyExpert();
  api("GET", "/healthz").then((h) => {
    $("version").textContent = h && h.version ? h.version : "";
  }).catch(() => {});
  $("cube-select").addEventListener("change", (ev) => {
    state.serial = ev.target.value;
    state.details = {};
    state.ui = {};
    refresh().then(connectEvents);
  });
  $("expert-toggle").addEventListener("change", (ev) => {
    state.expert = ev.target.checked;
    store("massimo.expert", state.expert ? "1" : "0");
    applyExpert();
  });
  initTiles();
  refresh().then(connectEvents);
  setInterval(() => loadCubes().catch(() => {}), 60000);
}

document.addEventListener("DOMContentLoaded", init);
