// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

// The Massimo web UI, core: API access with the token, settings, the
// header with the Cube clock, loading of cubes and rooms, polling and
// the event stream. The tiles and their actions are in tiles.js.
"use strict";

const state = {
  cubes: [],
  serial: null,
  rooms: [],
  unlisted: [],
  token: null,
  events: null,
  eventsOpen: false,
  settings: { poll: 10, expert: false, clock: "browser" },
  tz: {}, // per serial: {winter, summer} time zone rules from the cube detail
  details: {}, // device detail by rf address, fetched on demand
  ui: {}, // per room id: {open, pending, timer, vacation, confirm}
  pollTimer: null,
  version: null, // the server version the page was loaded with
  newVersion: false,
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
    if (value === null) localStorage.removeItem(key);
    else localStorage.setItem(key, value);
  } catch (e) {
    // private window or storage blocked: the value lives for this page only
  }
}

function loadSettings() {
  const s = state.settings;
  const poll = parseInt(readStored("massimo.poll"), 10);
  if (poll >= 2) s.poll = poll;
  s.expert = readStored("massimo.expert") === "1";
  const clock = readStored("massimo.clock");
  if (clock === "cube" || clock === "browser") s.clock = clock;
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
      connectEvents();
    };
    dlg.showModal();
    input.focus();
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

// localDateTimeValue formats a Date for a datetime-local input.
function localDateTimeValue(d) {
  const pad = (n) => String(n).padStart(2, "0");
  return d.getFullYear() + "-" + pad(d.getMonth() + 1) + "-" + pad(d.getDate()) + "T" + pad(d.getHours()) + ":" + pad(d.getMinutes());
}

// tomorrowMorning is the default "until" of a temporary setting.
function tomorrowMorning() {
  const d = new Date(Date.now() + 24 * 3600 * 1000);
  d.setHours(6, 0, 0, 0);
  return localDateTimeValue(d);
}

// --- cube clock -------------------------------------------------------

// lastWeekday returns the day of month of the last given weekday
// (0 = Sunday) in month (1..12) of year.
function lastWeekday(year, month, weekday) {
  const last = new Date(Date.UTC(year, month, 0)); // last day of month
  const diff = (last.getUTCDay() - weekday + 7) % 7;
  return last.getUTCDate() - diff;
}

// cubeOffset returns the UTC offset in seconds the Cube's zone has at
// the given local wall-clock time, from its two daylight-saving rules:
// summer from the last <weekday> of tz_summer.month at its hour to the
// last <weekday> of tz_winter.month at its hour.
function cubeOffset(tz, y, m, d, hh, mm) {
  const w = tz.winter, s = tz.summer;
  if (!w || !s || !s.month || !w.month || s.offset_seconds === w.offset_seconds) {
    return w ? w.offset_seconds : null;
  }
  const minute = (month, day, hour) => ((month * 31 + day) * 24 + hour) * 60;
  const now = minute(m, d, hh) + mm;
  const start = minute(s.month, lastWeekday(y, s.month, s.weekday), s.hour);
  const end = minute(w.month, lastWeekday(y, w.month, w.weekday), w.hour);
  const summer = start < end ? now >= start && now < end : now >= start || now < end;
  return summer ? s.offset_seconds : w.offset_seconds;
}

// cubeInstant converts the Cube's "YYYY-MM-DD HH:MM" wall-clock time
// into a Date, or null when the rules are unknown.
function cubeInstant(text, tz) {
  const m = /^(\d{4})-(\d{2})-(\d{2}) (\d{2}):(\d{2})$/.exec(text || "");
  if (!m || !tz) return null;
  const [y, mo, d, hh, mm] = m.slice(1).map(Number);
  const off = cubeOffset(tz, y, mo, d, hh, mm);
  if (off === null || off === undefined) return null;
  return new Date(Date.UTC(y, mo - 1, d, hh, mm) - off * 1000);
}

function fmtInstant(date, timeZone) {
  const opts = { year: "numeric", month: "2-digit", day: "2-digit", hour: "2-digit", minute: "2-digit", hour12: false };
  if (timeZone) opts.timeZone = timeZone;
  return new Intl.DateTimeFormat(undefined, opts).format(date);
}

// clockText returns the clock to show and its tooltip.
function clockText(cube) {
  const raw = cube.cube_time || "";
  const tz = state.tz[cube.serial];
  const inst = cubeInstant(raw, tz);
  if (!inst) return { text: raw || "–", title: "" };
  const utc = fmtInstant(inst, "UTC") + " UTC";
  if (state.settings.clock === "cube") {
    const off = cubeOffset(tz, ...raw.split(/[- :]/).map(Number));
    const name = (off === tz.summer.offset_seconds ? tz.summer.name : tz.winter.name) || "cube time";
    return { text: raw + " " + name, title: utc };
  }
  return { text: fmtInstant(inst), title: utc };
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
  const clock = clockText(cube);
  for (const id of ["clock", "clock-plain"]) {
    $(id).textContent = clock.text;
    $(id).title = clock.title;
  }
  const p = $("pending");
  const n = cube.pending_radiograms || 0;
  if (n > 0) {
    p.textContent = state.settings.expert
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

function applySettings() {
  const s = state.settings;
  document.body.classList.toggle("expert", s.expert);
  $("expert-toggle").checked = s.expert;
  $("expert-header").checked = s.expert;
  $("poll-input").value = s.poll;
  $("clock-select").value = s.clock;
  const cube = state.cubes.find((c) => c.serial === state.serial);
  if (cube) renderHeader(cube);
  startPolling();
}

// --- loading ----------------------------------------------------------

async function loadCubes() {
  state.cubes = (await api("GET", "/api/v1/cubes")) || [];
  if (!state.serial || !state.cubes.some((c) => c.serial === state.serial)) {
    state.serial = state.cubes.length ? state.cubes[0].serial : null;
  }
  renderCubeSelect();
  const cube = state.cubes.find((c) => c.serial === state.serial);
  if (cube) {
    if (!state.tz[cube.serial]) await loadTimeZone(cube.serial);
    renderHeader(cube);
  }
}

// loadTimeZone reads the Cube's daylight-saving rules once per Cube.
async function loadTimeZone(serial) {
  try {
    const detail = await api("GET", "/api/v1/cubes/" + encodeURIComponent(serial));
    const cfg = detail && detail.config;
    state.tz[serial] = cfg && cfg.tz_winter ? { winter: cfg.tz_winter, summer: cfg.tz_summer } : false;
  } catch (e) {
    state.tz[serial] = false;
  }
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
    $("message").classList.add("hidden");
  } catch (e) {
    showMessage("Cannot load: " + e.message, false);
  }
}

function selectCube(serial) {
  state.serial = serial;
  store("massimo.serial", serial);
  state.details = {};
  state.ui = {};
  refresh().then(connectEvents);
}

// --- version ----------------------------------------------------------

// somethingOpen reports whether a reload would interrupt the user: a
// panel, a dialog, a device list, or a stepper value not yet sent.
function somethingOpen() {
  if (!$("settings").classList.contains("hidden")) return true;
  if (!$("house-panel").classList.contains("hidden")) return true;
  if ($("token-dialog").open) return true;
  for (const ui of Object.values(state.ui)) {
    if (ui.pending !== null || ui.vacation || ui.confirm || ui.open) return true;
  }
  return false;
}

// checkVersion compares the server's version with the one the page was
// loaded with; a newer server reloads the page when nothing is open,
// otherwise it offers a button.
async function checkVersion() {
  let h;
  try {
    h = await api("GET", "/healthz");
  } catch (e) {
    return;
  }
  const v = h && h.version ? h.version : "";
  if (state.version === null) {
    state.version = v;
    $("version").textContent = v;
    return;
  }
  if (v === state.version) return;
  state.newVersion = true;
  $("reload-btn").classList.remove("hidden");
  if (!somethingOpen()) location.reload();
}

// --- polling ----------------------------------------------------------

// startPolling refreshes the header every poll interval and the rooms
// as well while the event stream is not connected.
function startPolling() {
  if (state.pollTimer) clearInterval(state.pollTimer);
  state.pollTimer = setInterval(() => {
    checkVersion();
    loadCubes().then(() => {
      if (!state.eventsOpen) return loadRooms();
    }).catch(() => {});
  }, state.settings.poll * 1000);
}

// --- events -----------------------------------------------------------

function connectEvents() {
  if (state.events) {
    state.events.close();
    state.events = null;
    state.eventsOpen = false;
  }
  if (!state.serial) return;
  let url = base() + "/events";
  if (state.token) url += "?token=" + encodeURIComponent(state.token);
  const es = new EventSource(url);
  state.events = es;
  es.onopen = () => {
    state.eventsOpen = true;
  };
  es.addEventListener("state", (ev) => {
    const data = JSON.parse(ev.data);
    if (data.state) updateDevice(data.state);
  });
  es.addEventListener("hello", () => loadCubes());
  es.addEventListener("metadata", () => loadRooms());
  es.addEventListener("connected", () => refresh());
  es.addEventListener("disconnected", () => loadCubes());
  es.onerror = () => {
    // The browser retries by itself; the poll covers the rooms meanwhile.
    state.eventsOpen = false;
    loadCubes().catch(() => {});
  };
}

// --- settings panel ---------------------------------------------------

function initSettings() {
  const panel = $("settings");
  $("settings-toggle").addEventListener("click", () => panel.classList.toggle("hidden"));
  $("settings-close").addEventListener("click", () => panel.classList.add("hidden"));
  $("poll-input").addEventListener("change", (ev) => {
    const v = Math.max(2, parseInt(ev.target.value, 10) || 10);
    state.settings.poll = v;
    ev.target.value = v;
    store("massimo.poll", String(v));
    startPolling();
  });
  const onExpert = (ev) => {
    state.settings.expert = ev.target.checked;
    store("massimo.expert", state.settings.expert ? "1" : "0");
    applySettings();
  };
  $("expert-toggle").addEventListener("change", onExpert);
  $("expert-header").addEventListener("change", onExpert);
  $("reload-btn").addEventListener("click", () => location.reload());
  $("clock-select").addEventListener("change", (ev) => {
    state.settings.clock = ev.target.value;
    store("massimo.clock", state.settings.clock);
    applySettings();
  });
  $("forget-token").addEventListener("click", () => {
    state.token = null;
    store("massimo.token", null);
    panel.classList.add("hidden");
    refresh().then(connectEvents);
  });
}

// --- wiring -----------------------------------------------------------

function init() {
  state.token = readStored("massimo.token");
  state.serial = readStored("massimo.serial");
  loadSettings();
  initSettings();
  applySettings();
  checkVersion();
  $("cube-select").addEventListener("change", (ev) => selectCube(ev.target.value));
  initTiles();
  refresh().then(() => {
    store("massimo.serial", state.serial);
    connectEvents();
  });
}

document.addEventListener("DOMContentLoaded", init);
