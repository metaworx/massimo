// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

// The Massimo web UI, core: API access with the token (the sign-in is
// login.js), the helpers the
// other files share that read the page's state (fmtTemp, ageEl,
// helpParts; fmtNum is in i18n.js, the components in components.js),
// settings, the header with the Cube clock, loading of cubes and rooms,
// polling and the event stream with its tickets. The tiles and their
// actions are in tiles.js.
"use strict";

const state = {
  cubes: [], // the configured Cubes, which the server serves
  found: [], // Cubes that answered discovery but are not configured
  serial: null,
  rooms: [],
  unlisted: [],
  token: null,
  me: null, // GET /api/v1/me: the mode, the user, the role, the capabilities, setup (login.js)
  events: null,
  eventsOpen: false,
  settings: { poll: 10, expert: false, clock: "browser", lang: "auto", theme: "system", scheme: "massimo" },
  tz: {}, // per serial: {winter, summer} time zone rules from the cube detail
  details: {}, // device detail by rf address, fetched on demand
  ui: {}, // per room id: {open, pending, timer, vacation, confirm}
  pollTimer: null,
  version: null, // the server version the page was loaded with
  newVersion: false,
  lastPoll: null, // time of the last successful poll (browser time)
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

// --- the scopes of the settings ----------------------------------------

// SETTING_KEYS are the page's settings. Each is the user's value, kept in
// the browser under massimo.<key> as before, which a signed-in user with
// the capability preferences also keeps on the server, so that it follows
// the user to every browser (login.js, loadPreferences); a setting can be
// kept for this device only instead, in massimo.device, which then wins.
const SETTING_KEYS = ["poll", "expert", "clock", "lang", "theme", "scheme"];

// SETTING_LABELS name each setting's row, for its "only on this device"
// switch.
const SETTING_LABELS = { poll: "settings.poll", expert: "settings.expertShort", clock: "settings.clock", lang: "settings.language", theme: "appearance.mode", scheme: "appearance.scheme" };

// SETTING_STORE writes a value as massimo.<key> has always held it; the
// default is kept as no entry, but for the poll and the expert switch.
const SETTING_STORE = {
  poll: (v) => String(v),
  expert: (v) => (v ? "1" : "0"),
  clock: (v) => v,
  lang: (v) => (v === "auto" ? null : v),
  theme: (v) => (v === "system" ? null : v),
  scheme: (v) => (v === "massimo" ? null : v),
};

// validSetting is a value of a setting when it is one, else undefined.
function validSetting(key, v) {
  switch (key) {
    case "poll": {
      const n = parseInt(v, 10);
      return n >= 2 && n <= 3600 ? n : undefined;
    }
    case "expert":
      return typeof v === "boolean" ? v : undefined;
    case "clock":
      return v === "cube" || v === "browser" ? v : undefined;
    case "lang":
      return v === "auto" || languages().includes(v) ? v : undefined;
    case "theme":
      return v === "system" || v === "light" || v === "dark" ? v : undefined;
    case "scheme":
      return typeof v === "string" && v !== "" ? v : undefined;
  }
  return undefined;
}

// storedSettings are the user's values as the browser keeps them, the
// defaults where it keeps none.
function storedSettings() {
  const s = { poll: 10, expert: false, clock: "browser", lang: "auto", theme: "system", scheme: "massimo" };
  const poll = validSetting("poll", readStored("massimo.poll"));
  if (poll !== undefined) s.poll = poll;
  s.expert = readStored("massimo.expert") === "1";
  for (const k of ["clock", "lang", "theme", "scheme"]) {
    const v = validSetting(k, readStored("massimo." + k));
    if (v !== undefined) s[k] = v;
  }
  return s;
}

// deviceOverrides are the settings kept for this device only.
function deviceOverrides() {
  try {
    const d = JSON.parse(readStored("massimo.device") || "{}");
    return d && typeof d === "object" && !Array.isArray(d) ? d : {};
  } catch (e) {
    return {};
  }
}

function storeDeviceOverrides(d) {
  store("massimo.device", Object.keys(d).length ? JSON.stringify(d) : null);
}

function deviceOnly(key) {
  return Object.prototype.hasOwnProperty.call(deviceOverrides(), key);
}

// readSetting is the value in effect of a setting: this device's, else
// the user's.
function readSetting(key) {
  const d = deviceOverrides();
  if (Object.prototype.hasOwnProperty.call(d, key)) {
    const v = validSetting(key, d[key]);
    if (v !== undefined) return v;
  }
  return storedSettings()[key];
}

function loadSettings() {
  const s = state.settings;
  for (const k of SETTING_KEYS) s[k] = readSetting(k);
  s.theme = validMode(s.theme); // theme.js
  s.scheme = validScheme(s.scheme);
}

// storeUserSetting keeps the user's value of a setting in the browser.
function storeUserSetting(key, v) {
  store("massimo." + key, SETTING_STORE[key](v));
}

// setSetting changes a setting: on this device only where it is kept for
// this device, else the user's value, which goes to the server as well
// (login.js schedulePreferences).
function setSetting(key, v) {
  state.settings[key] = v;
  const d = deviceOverrides();
  if (Object.prototype.hasOwnProperty.call(d, key)) {
    d[key] = v;
    storeDeviceOverrides(d);
    return;
  }
  storeUserSetting(key, v);
  schedulePreferences("ui");
}

// setDeviceOnly keeps a setting for this device only, with the value in
// effect, or stops: the user's value applies again.
function setDeviceOnly(key, on) {
  const d = deviceOverrides();
  if (on) d[key] = state.settings[key];
  else delete d[key];
  storeDeviceOverrides(d);
  if (on) return;
  applyStoredSettings();
}

// applyStoredSettings applies the settings again after the user's values
// or the overrides changed; a change of language redraws the page.
function applyStoredSettings() {
  const lang = state.settings.lang;
  loadSettings();
  applySettings();
  if (state.settings.lang !== lang) return changeLanguage(state.settings.lang).then(applyLanguage);
  return Promise.resolve();
}

// renderDeviceSwitches shows which settings are kept for this device
// only, each switch named for its row.
function renderDeviceSwitches() {
  for (const input of document.querySelectorAll("input[data-device-only]")) {
    const key = input.dataset.deviceOnly;
    input.checked = deviceOnly(key);
    input.setAttribute("aria-label", t("settings.thisDeviceFor", { setting: t(SETTING_LABELS[key]) }));
  }
}

// --- API --------------------------------------------------------------

function base() {
  return "api/v1/cubes/" + encodeURIComponent(state.serial);
}

async function api(method, path, body) {
  const headers = {};
  if (state.token) headers.Authorization = "Bearer " + state.token;
  if (body !== undefined) headers["Content-Type"] = "application/json";
  const resp = await fetch(path, { method, headers, body: body === undefined ? undefined : JSON.stringify(body) });
  if (resp.status === 401) {
    if (await askLogin()) return api(method, path, body); // login.js
    throw new Error(t("api.badToken"));
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

// deviceDetail fetches and caches a device's configuration.
async function deviceDetail(rf) {
  if (state.details[rf]) return state.details[rf];
  const d = await api("GET", base() + "/devices/" + rf);
  state.details[rf] = d;
  return d;
}

// --- helpers ----------------------------------------------------------

// showMessage says how something went on the page's message line.
function showMessage(text, ok) {
  showResult($("message"), text, ok);
}

// fmtTemp is a temperature with one decimal and a unit ("°C" unless
// given), "–" when unknown.
function fmtTemp(v, unit) {
  if (v === undefined || v === null) return "–";
  return fmtNum(v, 1) + (unit === undefined ? " °C" : unit);
}

// --- ages of values ---------------------------------------------------

// observedSince is when the server first heard from the selected Cube,
// in ms (NaN when unknown).
function observedSince() {
  const c = state.cubes.find((x) => x.serial === state.serial);
  return c && c.observed_since ? Date.parse(c.observed_since) : NaN;
}

// SINCE_MS: a value received this soon after observedSince came with the
// Cube's first device list, so it was already there and may be older.
const SINCE_MS = 5000;

// fmtUnit is n minutes, hours or days in the language's short form
// ("5 min", "3 Std.").
function fmtUnit(n, unit) {
  try {
    return new Intl.NumberFormat(i18n.lang, { style: "unit", unit, unitDisplay: "short" }).format(n);
  } catch (e) {
    return n + " " + unit.charAt(0);
  }
}

// fmtAge is how long ago an instant (ms) was, briefly: "now", "12 min",
// "5 h", from two days on in days. A value that was already there when
// the server connected (since) gets "≥": it may be older.
function fmtAge(ms, since) {
  const sec = Math.max(0, (Date.now() - ms) / 1000);
  if (sec < 60 && !since) return t("age.now");
  let text;
  if (sec < 3600) text = fmtUnit(Math.floor(sec / 60), "minute");
  else if (sec < 48 * 3600) text = fmtUnit(Math.floor(sec / 3600), "hour");
  else text = fmtUnit(Math.floor(sec / 86400), "day");
  return since ? "≥ " + text : text;
}

// fmtShortTime is an instant (ms) as day, month and time.
function fmtShortTime(ms) {
  const opts = { day: "2-digit", month: "2-digit", hour: "2-digit", minute: "2-digit", hour12: false };
  return new Intl.DateTimeFormat(i18n.lang, opts).format(new Date(ms));
}

// ageEl shows how old a value is that the server received at iso (an API
// time): the age, the exact time as its tooltip and, in expert mode,
// after it; greyed when stale. refreshAges keeps it current. null when
// there is no time. plain leaves out the "already there" mark, for
// times that are not device values.
function ageEl(iso, stale, plain) {
  const ms = Date.parse(iso || "");
  if (isNaN(ms)) return null;
  const el = textEl("span", "", "age" + (stale ? " stale" : ""));
  el.dataset.at = String(ms);
  const since = ms - observedSince();
  if (!plain && since >= 0 && since < SINCE_MS) el.dataset.since = "1";
  el.appendChild(textEl("span", "", "age-text"));
  el.appendChild(textEl("span", "", "age-exact expert-only"));
  fillAge(el);
  return el;
}

function fillAge(el) {
  const ms = Number(el.dataset.at);
  const since = el.dataset.since === "1";
  el.firstChild.textContent = fmtAge(ms, since);
  el.lastChild.textContent = fmtShortTime(ms);
  el.title = t(since ? "age.titleSince" : "age.title", { time: fmtInstant(new Date(ms)) });
}

// refreshAges updates every age on the page; it runs once a minute.
function refreshAges() {
  for (const el of document.querySelectorAll(".age[data-at]")) fillAge(el);
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

// --- help -------------------------------------------------------------

const helpOpen = new Set(); // keys of the explanations shown
let helpSeq = 0;

// helpParts returns a labelled button that toggles the explanation of a
// setting (help.<key> in the translation table), and the explanation,
// hidden until asked for. An open explanation stays open when its tile
// is drawn again.
function helpParts(key, label) {
  const text = textEl("p", t("help." + key), "help-text");
  text.id = "help-" + key + "-" + ++helpSeq;
  const btn = iconButton("icon-help", t("help.button", { label }), null, { cls: "help-btn", title: t("help.title"), iconCls: "hi" });
  btn.setAttribute("aria-controls", text.id);
  const show = (on) => {
    text.classList.toggle("hidden", !on);
    btn.classList.toggle("active", on);
    btn.setAttribute("aria-expanded", on ? "true" : "false");
  };
  show(helpOpen.has(key));
  btn.addEventListener("click", () => {
    const on = !helpOpen.has(key);
    if (on) helpOpen.add(key);
    else helpOpen.delete(key);
    show(on);
  });
  return { btn, text };
}

// initHelpSlots puts a help button into every element of the page with
// a data-help key (data-help-label: the key of the setting's label), and
// its explanation at the end of the slot's parent; again after a change
// of language.
function initHelpSlots() {
  for (const slot of document.querySelectorAll("[data-help]")) {
    const h = helpParts(slot.dataset.help, t(slot.dataset.helpLabel || ""));
    if (slot.helpText) slot.helpText.remove();
    slot.helpText = h.text;
    clear(slot);
    slot.appendChild(h.btn);
    slot.parentElement.appendChild(h.text);
  }
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
    const name = (off === tz.summer.offset_seconds ? tz.summer.name : tz.winter.name) || t("header.cubeTime");
    return { text: raw + " " + name, title: utc };
  }
  return { text: fmtInstant(inst), title: utc };
}

// --- header -----------------------------------------------------------

function renderHeader(cube) {
  const conn = $("connection");
  conn.textContent = t(cube.connected ? "header.connected" : "header.disconnected");
  conn.className = "pill " + (cube.connected ? "ok" : "bad");
  const b = cube.budget || {};
  $("duty").textContent = b.duty_cycle_pct === undefined ? "–" : b.duty_cycle_pct + " %";
  $("slots").textContent = b.free_memory_slots === undefined ? "–" : String(b.free_memory_slots);
  $("queued").textContent = cube.queued === undefined ? "–" : String(cube.queued);
  $("pending-n").textContent = String(cube.pending_radiograms || 0);
  $("cube-fw").textContent = cube.firmware || "–";
  $("cube-addr").textContent = cube.address || "–";
  $("cube-rf").textContent = cube.rf_address || "–";
  const clock = clockText(cube);
  for (const id of ["clock", "clock-plain"]) {
    $(id).textContent = clock.text;
    $(id).title = clock.title;
  }
  const p = $("pending");
  const n = cube.pending_radiograms || 0;
  if (n > 0) {
    p.textContent = state.settings.expert ? tn("header.pendingExpert", n) : t("header.pending", { n });
    p.classList.remove("hidden");
  } else {
    p.classList.add("hidden");
  }
  renderUpdated();
}

// renderUpdated shows the time of the last successful poll with
// seconds, in the browser's time; the Cube's clock has no seconds.
function renderUpdated() {
  const el = $("updated");
  if (!state.lastPoll) {
    el.textContent = "";
    return;
  }
  const pad = (n) => String(n).padStart(2, "0");
  const last = state.lastPoll;
  el.textContent = t("header.updated", { time: pad(last.getHours()) + ":" + pad(last.getMinutes()) + ":" + pad(last.getSeconds()) });
}

// renderCubeSelect fills the Cube choice: the configured Cubes, then
// those found on the LAN but not configured (shown, not selectable),
// then the entry that searches the LAN for new ones.
function renderCubeSelect() {
  const options = state.cubes.map((c) => {
    const name = c.name ? c.name + " (" + c.serial + ")" : c.serial;
    return [c.serial, c.connected ? name : t("header.cubeDisconnected", { name })];
  });
  if (state.found.length) {
    options.push({ group: t("discover.group"), options: state.found.map((c) => ["found:" + c.serial, c.serial + (c.address ? " · " + c.address : ""), true]) });
  }
  if (hasCap("discover")) options.push([DISCOVER, t(discovery.running ? "discover.searching" : "discover.search"), discovery.running]);
  const sel = fillOptions($("cube-select"), options);
  sel.value = state.serial || "";
}

function applySettings() {
  const s = state.settings;
  document.body.classList.toggle("expert", s.expert);
  $("expert-toggle").checked = s.expert;
  $("poll-input").value = s.poll;
  $("clock-select").value = s.clock;
  applyAppearance();
  renderDeviceSwitches();
  const cube = state.cubes.find((c) => c.serial === state.serial);
  if (cube) renderHeader(cube);
  startPolling();
}

// --- loading ----------------------------------------------------------

async function loadCubes() {
  const all = (await api("GET", "api/v1/cubes")) || [];
  state.cubes = all.filter(isConfigured);
  state.found = all.filter((c) => !isConfigured(c));
  renderFound();
  cubesPolled(); // cubes.js
  state.lastPoll = new Date();
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
    const detail = await api("GET", "api/v1/cubes/" + encodeURIComponent(serial));
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
    if (langError) showLanguageError();
    else $("message").classList.add("hidden");
  } catch (e) {
    showMessage(t("msg.cannotLoad", { error: e.message }), false);
  }
}

// --- Cube discovery ---------------------------------------------------

// DISCOVER is the value of the Cube choice's entry that searches the LAN.
const DISCOVER = "__discover";
const discovery = { running: false, timer: null };

// isConfigured tells the Cubes the server serves from those it only
// found on the LAN; a server without discovery sends no flag.
function isConfigured(c) {
  return c.configured !== false;
}

// showSearch shows the state of a search next to the Cube choice, for
// ms milliseconds when given.
function showSearch(text, ms, bad) {
  const el = $("cube-search");
  if (discovery.timer) clearTimeout(discovery.timer);
  discovery.timer = null;
  el.textContent = text;
  el.classList.toggle("bad", !!bad);
  el.classList.toggle("hidden", !text);
  if (ms) discovery.timer = setTimeout(() => showSearch(""), ms);
}

// discoverCubes asks the server to search the LAN for Cubes (about two
// seconds), then reloads the list and says what it found.
async function discoverCubes() {
  if (discovery.running) return;
  discovery.running = true;
  showSearch(t("discover.searching"));
  renderCubeSelect();
  try {
    const res = (await api("POST", "api/v1/cubes/discover")) || {};
    await loadCubes();
    const n = (res.found || []).filter((c) => !isConfigured(c)).length;
    showSearch(n ? tn("discover.found", n) : t("discover.none"), 8000);
    if (n) $("cube-found").open = true;
  } catch (e) {
    showSearch(t("discover.failed", { error: e.message }), 15000, true);
  } finally {
    discovery.running = false;
    renderCubeSelect();
  }
}

// addFound takes a Cube from a cube_found event into the list.
function addFound(c) {
  if (!c || !c.serial || state.cubes.some((x) => x.serial === c.serial)) return;
  const i = state.found.findIndex((x) => x.serial === c.serial);
  if (i >= 0) state.found[i] = c;
  else state.found.push(c);
  renderCubeSelect();
  renderFound();
}

// renderFound lists, under the header, the Cubes found on the LAN that
// the server does not serve, each with Add (foundRow, cubes.js), which
// serves it at once, and the other way to add them: the lines for the server's
// env file, ready to copy. The address makes serve take the Cube; a
// name alone would as well, serve then follows the address the Cube
// answers discovery from.
function renderFound() {
  const box = $("cube-found");
  const n = state.found.length;
  box.classList.toggle("hidden", !n);
  if (!n) return;
  box.querySelector("summary").textContent = tn("discover.found", n);
  const body = box.querySelector(".cube-found-body");
  clear(body);
  const list = elem("ul");
  for (const c of state.found) list.appendChild(foundRow(c)); // cubes.js
  body.appendChild(list);
  body.appendChild(textEl("p", t("discover.hintAdd")));
  body.appendChild(textEl("p", t("discover.hint")));
  const lines = [];
  for (const c of state.found) {
    const p = "MASSIMO_CUBE_" + c.serial + "_";
    lines.push(p + "ADDRESS=" + (c.address || ""), p + "NAME=");
  }
  body.appendChild(textEl("pre", lines.join("\n"), "env-lines mono"));
  body.appendChild(textEl("p", t("discover.hintFile"), "muted small"));
}

function selectCube(serial) {
  state.serial = serial;
  store("massimo.serial", serial);
  state.details = {};
  state.ui = {};
  refresh().then(connectEvents);
  if (logState.open) loadLog();
}

// --- version ----------------------------------------------------------

// somethingOpen reports whether a reload would interrupt the user: a
// panel, a dialog, a device list, or a stepper value not yet sent.
function somethingOpen() {
  if (!$("settings").classList.contains("hidden")) return true;
  if (!$("house-panel").classList.contains("hidden")) return true;
  if ($("login-dialog").open) return true;
  for (const ui of Object.values(state.ui)) {
    if (ui.pending !== null || ui.vacation || ui.confirm || ui.open || ui.config) return true;
  }
  return false;
}

// splitVersion separates "massimo v0.1.0-alpha.4-3-gfb15396 (go1.26.8,
// linux/amd64)" into the short version and the rest.
function splitVersion(v) {
  const m = /^(?:massimo\s+)?(\S+)\s*(.*)$/.exec(v || "");
  if (!m) return { short: v || "", rest: "" };
  return { short: m[1], rest: m[2].replace(/^\((.*)\)$/, "$1") };
}

// checkVersion compares the server's version with the one the page was
// loaded with; a newer server reloads the page when nothing is open,
// otherwise it offers a button.
// renderStarted shows in expert mode when the server process started,
// and how long ago (an older server sends no time).
function renderStarted(iso) {
  const dd = $("server-started");
  if (dd.dataset.iso === (iso || "")) return;
  dd.dataset.iso = iso || "";
  clear(dd);
  const ms = Date.parse(iso || "");
  if (isNaN(ms)) {
    dd.textContent = "–";
    return;
  }
  dd.appendChild(document.createTextNode(fmtInstant(new Date(ms))));
  const age = ageEl(iso, false, true);
  if (age) dd.appendChild(age);
}

// renderTLS shows the certificate the server serves, from /healthz's
// tls: in expert mode its source, names and expiry, and under the header
// a notice with the fingerprint while it is self-signed, so the owner
// can compare it with the server's log after the browser's warning, and
// a warning while the certificate ends within two weeks (or has none
// yet), computed here from not_after so that it speaks the page's
// language.
function renderTLS(tls) {
  const dd = $("tls-cert");
  const notice = $("tls-notice");
  if (!tls) {
    dd.textContent = "–";
    notice.classList.add("hidden");
    return;
  }
  const until = Date.parse(tls.not_after || "");
  const parts = [tls.source, (tls.names || []).join(", ")];
  if (!isNaN(until)) parts.push(t("header.certUntil", { date: fmtInstant(new Date(until)) }));
  dd.textContent = parts.filter(Boolean).join(" · ");
  const texts = [];
  if (tls.self_signed) texts.push(t("header.selfSigned"));
  if (isNaN(until)) {
    if (tls.warning) texts.push(t("header.certMissing"));
  } else {
    const days = Math.floor((until - Date.now()) / 86400000);
    if (days < 0) texts.push(t("header.certExpired"));
    else if (days < 14) texts.push(tn("header.certExpires", days));
  }
  $("tls-notice-text").textContent = texts.join(" ");
  $("tls-fingerprint").textContent = tls.self_signed ? tls.fingerprint || "" : "";
  notice.classList.toggle("hidden", texts.length === 0);
}

async function checkVersion() {
  let h;
  try {
    h = await api("GET", "healthz");
  } catch (e) {
    return;
  }
  const v = h && h.version ? h.version : "";
  renderStarted(h && h.started_at);
  renderTLS(h && h.tls);
  if (state.version === null) {
    state.version = v;
    const parts = splitVersion(v);
    $("version-short").textContent = parts.short;
    $("version").textContent = parts.short;
    $("version-rest").textContent = parts.rest || "–";
    return;
  }
  if (v === state.version) return;
  state.newVersion = true;
  $("reload-btn").classList.remove("hidden");
  if (!somethingOpen()) location.reload();
}

// --- polling ----------------------------------------------------------

// poll refreshes the header and runs the version check; the rooms are
// refreshed as well while the event stream is not connected, or always
// when the user asked (the refresh button).
function poll(rooms) {
  checkVersion();
  return loadCubes().then(() => {
    if (rooms || !state.eventsOpen) return loadRooms();
  }).catch(() => {});
}

// startPolling runs poll every poll interval.
function startPolling() {
  if (state.pollTimer) clearInterval(state.pollTimer);
  state.pollTimer = setInterval(() => poll(false), state.settings.poll * 1000);
}

// --- events -----------------------------------------------------------

// The stream opens with a single-use ticket from POST …/events/ticket,
// since a browser cannot send the token header on it; without a token
// on the server the ticket is not needed but answered all the same, and
// a missing or wrong token shows up there, on an authenticated call. A
// connection uses its ticket up, so after an error the page closes the
// stream itself and opens a new one with a fresh ticket, waiting longer
// after each failure: the browser's own retry would reuse the ticket,
// and a used ticket counts as a failed token check. The poll covers the
// rooms meanwhile.
const EVENTS_RETRY_MS = 2000;
const EVENTS_RETRY_MAX_MS = 60000;
let eventsGen = 0; // counts the connections; an outdated one does nothing
let eventsRetry = EVENTS_RETRY_MS;
let eventsTimer = null;

function closeEvents() {
  eventsGen++;
  if (eventsTimer) clearTimeout(eventsTimer);
  eventsTimer = null;
  if (state.events) state.events.close();
  state.events = null;
  state.eventsOpen = false;
}

function retryEvents(gen) {
  eventsTimer = setTimeout(() => {
    eventsTimer = null;
    if (gen === eventsGen) connectEvents();
  }, eventsRetry);
  eventsRetry = Math.min(eventsRetry * 2, EVENTS_RETRY_MAX_MS);
}

async function connectEvents() {
  closeEvents();
  const gen = eventsGen;
  if (!state.serial) return;
  let ticket;
  try {
    ticket = (await api("POST", base() + "/events/ticket")).ticket;
  } catch (e) {
    if (gen === eventsGen) retryEvents(gen);
    return;
  }
  if (gen !== eventsGen) return; // another connect or close came first
  const es = new EventSource(base() + "/events?ticket=" + encodeURIComponent(ticket));
  state.events = es;
  es.onopen = () => {
    state.eventsOpen = true;
    eventsRetry = EVENTS_RETRY_MS;
  };
  let initial = true; // the connect burst of state events is not log traffic
  setTimeout(() => {
    initial = false;
  }, 1500);
  es.addEventListener("state", (ev) => {
    const data = JSON.parse(ev.data);
    if (data.state) updateDevice(data.state);
    if (!initial) appendLogEvent(data);
  });
  const logged = (handler) => (ev) => {
    const data = JSON.parse(ev.data);
    appendLogEvent(data);
    handler(data);
  };
  es.addEventListener("hello", logged(() => loadCubes()));
  es.addEventListener("metadata", logged(() => loadRooms()));
  // Pairing (cubes.js): a device that joined is among the devices
  // without a room once the server read its C line, which the reload
  // after its config event finds as well.
  const serial = state.serial;
  es.addEventListener("pairing_started", logged(() => pairingEvent(serial)));
  es.addEventListener("pairing_ended", logged(() => pairingEvent(serial)));
  es.addEventListener("paired", (ev) => {
    const data = JSON.parse(ev.data);
    appendLogEvent(data);
    pairingJoined(serial, data.paired);
    loadRooms().catch(() => {});
  });
  es.addEventListener("device_forgotten", logged(() => loadRooms()));
  es.addEventListener("connected", logged(() => refresh()));
  es.addEventListener("disconnected", logged(() => loadCubes()));
  es.addEventListener("config", logged((data) => {
    // a device that just joined pairing: its C line makes it a device without a room
    if (data && data.device && !state.unlisted.some((d) => d.rf_address === data.device) &&
        !state.rooms.some((r) => r.devices.some((d) => d.rf_address === data.device))) loadRooms().catch(() => {});
  }));
  es.addEventListener("cube_found", (ev) => {
    const data = JSON.parse(ev.data);
    appendLogEvent(data);
    addFound(data.cube);
  });
  es.onerror = () => {
    if (gen !== eventsGen) return;
    es.close();
    state.events = null;
    state.eventsOpen = false;
    loadCubes().catch(() => {});
    retryEvents(gen);
  };
}

// --- settings panel ---------------------------------------------------

function initSettings() {
  // The settings page is opened with the cog in the header (showView).
  $("poll-input").addEventListener("change", (ev) => {
    const v = validSetting("poll", ev.target.value) || 10;
    ev.target.value = v;
    setSetting("poll", v);
    startPolling();
  });
  const onExpert = (ev) => {
    setSetting("expert", ev.target.checked);
    applySettings();
  };
  $("expert-toggle").addEventListener("change", onExpert);
  $("reload-btn").addEventListener("click", () => location.reload());
  $("refresh-btn").addEventListener("click", (ev) => {
    const btn = ev.currentTarget;
    btn.disabled = true;
    poll(true).finally(() => {
      btn.disabled = false;
    });
  });
  $("clock-select").addEventListener("change", (ev) => {
    setSetting("clock", validSetting("clock", ev.target.value) || "browser");
    applySettings();
  });
  $("lang-select").addEventListener("change", (ev) => {
    setSetting("lang", validSetting("lang", ev.target.value) || "auto");
    changeLanguage(state.settings.lang).then(applyLanguage);
  });
  for (const input of document.querySelectorAll("input[data-device-only]")) {
    input.addEventListener("change", () => setDeviceOnly(input.dataset.deviceOnly, input.checked));
  }
  initAppearance();
}

// --- wiring -----------------------------------------------------------

// renderLangSelect fills the language choice: from the browser, then
// every language with a translation table, by its own name.
function renderLangSelect() {
  const options = [["auto", t("lang.auto") + " (" + LANGUAGE_NAMES[browserLanguage()] + ")"]];
  for (const l of languages()) options.push([l, LANGUAGE_NAMES[l] || l]);
  const sel = fillOptions($("lang-select"), options);
  sel.value = i18n.setting;
  const note = $("lang-note");
  const texts = machineNote();
  clear(note);
  note.classList.toggle("hidden", !texts);
  if (texts) for (const s of texts) note.appendChild(textEl("span", s));
}

// langError is the error of the last language load that failed, null
// once one succeeds.
let langError = null;

// changeLanguage loads and applies a language setting; when the language
// cannot be loaded, the page stays in English (in the static English of
// the markup when even English is missing) and the message line says why.
async function changeLanguage(setting) {
  try {
    await setLanguage(setting);
    if (langError) $("message").classList.add("hidden");
    langError = null;
  } catch (e) {
    langError = e;
    showLanguageError();
  }
}

function showLanguageError() {
  showMessage(I18N.en ? t("msg.cannotLoad", { error: langError.message }) : langError.message, false);
}

// applyLanguage redraws every text after a change of language.
function applyLanguage() {
  if (I18N.en) applyI18n(document);
  initHelpSlots();
  renderLangSelect();
  applyAppearance();
  renderCubeSelect();
  renderFound();
  renderMe();
  renderDeviceSwitches();
  renderUsers();
  renderRoles();
  renderCubes();
  const cube = state.cubes.find((c) => c.serial === state.serial);
  if (cube) renderHeader(cube);
  renderRooms();
  renderLog();
  if (updState.info) renderUpdate(updState.info);
}

// init starts the page once English and the chosen language are loaded.
async function init() {
  state.token = readStored("massimo.token");
  state.serial = readStored("massimo.serial");
  loadSettings();
  await changeLanguage(state.settings.lang);
  if (I18N.en) applyI18n(document);
  renderLangSelect();
  initHelpSlots();
  initSettings();
  applySettings();
  checkVersion();
  await loadMe(); // login.js: who is signed in, or the sign-in dialog
  await loadPreferences(); // login.js: the user's settings from the server
  $("cube-select").addEventListener("change", (ev) => {
    if (ev.target.value !== DISCOVER) return selectCube(ev.target.value);
    ev.target.value = state.serial || "";
    discoverCubes();
  });
  initTiles();
  initLog();
  refresh().then(() => {
    store("massimo.serial", state.serial);
    connectEvents();
  });
  setInterval(refreshAges, 60000);
}

document.addEventListener("DOMContentLoaded", init);
