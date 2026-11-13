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
  roomsOf: null, // the Cube whose rooms and devices rooms and unlisted hold, once loaded
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

// appBridge is the bridge of the Massimo app on the page it opened
// (docs/app.md, "The calls of a server's page"): the app keeps the
// page's token, behind the phone's fingerprint or PIN where the user
// asked for it, in place of the browser's storage; null in a browser.
function appBridge() {
  const b = window.massimo;
  return b && typeof b.token === "function" && typeof b.setToken === "function" ? b : null;
}

// readToken is the token the page starts with: the app's, which may ask
// for the fingerprint first (none when that is dismissed), else the one
// the browser kept. In the app the browser keeps none.
async function readToken() {
  const b = appBridge();
  if (!b) return readStored("massimo.token");
  store("massimo.token", null);
  try {
    return (await b.token()) || null;
  } catch (e) {
    return null;
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

// forgetUserSettings takes the user's values out of the browser, the
// settings and the schemes (theme.js), as Log out does for a user whose
// settings followed it; what is kept for this device only stays, and the
// defaults apply where it keeps nothing.
function forgetUserSettings() {
  for (const k of SETTING_KEYS) store("massimo." + k, null);
  forgetCustomSchemes();
  return applyStoredSettings();
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
    const label = t("settings.thisDeviceFor", { setting: t(SETTING_LABELS[key]) });
    input.setAttribute("aria-label", label);
    input.closest("label").title = label; // the column's switch has no text of its own
  }
}

// --- API --------------------------------------------------------------

function base() {
  return "api/v1/cubes/" + encodeURIComponent(state.serial);
}

// apiRequest sends one request and returns the answer's JSON, null for
// an empty one; an answer that is not ok is thrown as an Error with the
// server's error (or the status), its status and its data. The token
// goes along unless auth is false, and a token given in its place;
// keepalive lets the request outlive the page. api() adds the sign-in on
// a 401; the sign-in itself and the last preferences sent as the page is
// left use it as it is (login.js).
async function apiRequest(method, path, body, { auth = true, keepalive = false, token = "", headers: extra = {} } = {}) {
  const headers = { ...extra };
  const bearer = token || (auth && state.token);
  if (bearer) headers.Authorization = "Bearer " + bearer;
  if (body !== undefined) headers["Content-Type"] = "application/json";
  const resp = await fetch(path, { method, headers, body: body === undefined ? undefined : JSON.stringify(body), keepalive });
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
    err.status = resp.status;
    err.data = data;
    throw err;
  }
  return data;
}

// api is apiRequest with the token, asking for a sign-in on a 401 and
// sending the request again once one is in; headers are added to the
// request's own.
async function api(method, path, body, headers) {
  try {
    return await apiRequest(method, path, body, { headers });
  } catch (e) {
    if (e.status !== 401) throw e;
    if (await askLogin()) return api(method, path, body, headers); // login.js
    throw new Error(t("api.badToken"));
  }
}

// apiRaw is api for what is not JSON, a backup file: body goes as it is,
// with contentType, and the answer is the Response; an error is thrown
// as api throws it.
async function apiRaw(method, path, body, contentType) {
  const headers = {};
  if (state.token) headers.Authorization = "Bearer " + state.token;
  if (body !== undefined) headers["Content-Type"] = contentType || "application/octet-stream";
  const resp = await fetch(path, { method, headers, body });
  if (resp.status === 401) {
    if (await askLogin()) return apiRaw(method, path, body, contentType); // login.js
    throw new Error(t("api.badToken"));
  }
  if (!resp.ok) {
    let msg = resp.status + " " + resp.statusText;
    try {
      const data = JSON.parse(await resp.text());
      if (data && data.error) msg = data.error;
    } catch (e) {
      // not JSON: the status says it
    }
    const err = new Error(msg);
    err.status = resp.status;
    throw err;
  }
  return resp;
}

// devicePath is the API path of a device of the selected Cube, by its
// S/N, by its radio address only for a device without one (the API
// takes either).
function devicePath(d) {
  return base() + "/devices/" + encodeURIComponent(d.serial || d.rf_address);
}

// deviceDetail fetches and caches a device's configuration.
async function deviceDetail(dev) {
  const rf = dev.rf_address;
  if (state.details[rf]) return state.details[rf];
  const d = await api("GET", devicePath(dev));
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
  pruneLogRange(); // log.js: the lines a time range no longer holds
}

function isThermostat(d) {
  return d.type === "radiator_thermostat" || d.type === "radiator_thermostat_plus" || d.type === "wall_thermostat";
}

// deviceName is how the page names a device: by its name, else its S/N,
// the radio address only for a device without either.
function deviceName(d) {
  return d.name || d.serial || d.rf_address;
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

// clockText returns the Cube's clock to show and its tooltip. The Cube
// tells its clock in its greeting, at a connect; the server says when it
// read it (cube_time_at), and the clock is moved on by the time since. An
// offset from that moment of 90 s or more is said after it: the Cube's
// clock has minutes only.
function clockText(cube) {
  const raw = cube.cube_time || "";
  const tz = state.tz[cube.serial];
  const read = cubeInstant(raw, tz);
  if (!read) return { text: raw || "–", title: "" };
  const at = Date.parse(cube.cube_time_at || "");
  const since = isNaN(at) ? 0 : Math.max(0, Date.now() - at);
  const inst = new Date(read.getTime() + since);
  const utc = fmtInstant(inst, "UTC") + " UTC";
  let text;
  if (state.settings.clock === "cube") {
    // the Cube's own time, moved on as its zone's offset at the reading
    const [y, mo, d, hh, mm] = raw.split(/[- :]/).map(Number);
    const off = cubeOffset(tz, y, mo, d, hh, mm);
    const local = new Date(Date.UTC(y, mo - 1, d, hh, mm) + since);
    const pad = (n) => String(n).padStart(2, "0");
    const shown = local.getUTCFullYear() + "-" + pad(local.getUTCMonth() + 1) + "-" + pad(local.getUTCDate()) + " " + pad(local.getUTCHours()) + ":" + pad(local.getUTCMinutes());
    const name = (off === tz.summer.offset_seconds ? tz.summer.name : tz.winter.name) || t("header.cubeTime");
    text = shown + " " + name;
  } else {
    text = fmtInstant(inst);
  }
  if (!isNaN(at)) {
    const skew = Math.round((read.getTime() - at) / 60000);
    if (Math.abs(read.getTime() - at) >= 90000) text += " (" + tn(skew > 0 ? "budget.clockAhead" : "budget.clockBehind", Math.abs(skew)) + ")";
  }
  return { text, title: utc };
}

// --- header -----------------------------------------------------------

// budgetFacts is a Cube's radio budget as the page shows it, in the
// header and in its panel (cubes.js): the duty cycle, the free memory
// slots, the radiograms pending and the commands queued, "–" while unknown.
function budgetFacts(cube) {
  const b = cube.budget || {};
  return {
    duty: b.duty_cycle_pct === undefined ? "–" : b.duty_cycle_pct + " %",
    slots: b.free_memory_slots === undefined ? "–" : String(b.free_memory_slots),
    pending: String(cube.pending_radiograms || 0),
    queued: cube.queued === undefined ? "–" : String(cube.queued),
  };
}

// renderHeader shows the chosen Cube in the header: its connection in
// the Cube choice, and in expert mode its name with its radio budget,
// queued only while commands wait. Its clock is in its facts (cubes.js).
function renderHeader(cube) {
  renderCubeSelect(); // the chosen Cube's connection, its plug
  const f = budgetFacts(cube);
  $("stats-cube").textContent = cube.name || cube.serial;
  $("duty").textContent = f.duty;
  $("slots").textContent = f.slots;
  $("queued").textContent = f.queued;
  $("stat-queued").classList.toggle("hidden", !(cube.queued > 0));
  $("pending-n").textContent = f.pending;
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
// seconds, in the browser's time, in the refresh button, which says both
// to a screen reader.
function renderUpdated() {
  const el = $("updated");
  const btn = $("refresh-btn");
  if (!state.lastPoll) {
    el.textContent = "";
    btn.setAttribute("aria-label", t("header.refreshTitle"));
    return;
  }
  const pad = (n) => String(n).padStart(2, "0");
  const last = state.lastPoll;
  const time = pad(last.getHours()) + ":" + pad(last.getMinutes()) + ":" + pad(last.getSeconds());
  el.textContent = t("header.updated", { time });
  btn.setAttribute("aria-label", t("header.refreshUpdated", { time }));
}

// renderCubeSelect fills the Cube choice (components.js pickerMenu): the
// configured Cubes, each with its connection as a plug, its name (its
// serial without one) and under it its serial and address; then those
// found on the LAN but not configured, shown, not selectable; then the
// entry that searches the LAN for new ones. Closed it shows the chosen
// Cube's plug and name, its tooltip the serial, the address and the
// connection. An open choice is left as it is, so that a poll does not
// take the focus from it; a pick closes it first.
function renderCubeSelect() {
  const menu = $("cube-menu");
  if (menu.open) return;
  const items = state.cubes.map((c) => ({
    value: c.serial,
    label: c.name || c.serial,
    sub: [c.name ? c.serial : "", c.address || ""].filter(Boolean).join(" · "),
    icon: connIcon(c.connected),
  }));
  // a found Cube opens System settings → Cubes, where it can be added,
  // for a role with cubes; for others it is information
  if (state.found.length) {
    items.push({ group: t("discover.group") });
    for (const c of state.found) items.push({ value: "found:" + c.serial, label: c.serial, sub: c.address || "", disabled: !hasCap("cubes") });
  }
  if (hasCap("discover")) {
    items.push({ separator: true });
    items.push({ value: DISCOVER, label: t(discovery.running ? "discover.searching" : "discover.search"), disabled: discovery.running });
  }
  const cube = state.cubes.find((c) => c.serial === state.serial);
  const name = cube ? cube.name || cube.serial : t("header.cube");
  const conn = cube ? t(cube.connected ? "header.connected" : "header.disconnected") : "";
  pickerMenu(menu, {
    items,
    value: state.serial || "",
    onPick: (v) => {
      menu.open = false;
      if (v === DISCOVER) discoverCubes();
      else if (v.startsWith("found:")) showFound();
      else selectCube(v);
    },
    shown: {
      icon: cube ? connIcon(cube.connected) : null,
      label: name,
      title: cube ? [name, cube.name ? cube.serial : "", cube.address || "", conn].filter(Boolean).join(" · ") : name,
      aria: cube ? t("header.cubeChoice", { name, state: conn }) : name,
    },
  });
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
  renderHomeNotice();
  logCubesPolled(); // log.js: the Cubes choice, and under All Cubes a new Cube's rooms
  const cube = state.cubes.find((c) => c.serial === state.serial);
  if (cube) {
    if (!state.tz[cube.serial]) await loadTimeZone(cube.serial);
    renderHeader(cube);
  }
}

// timeOf is an API time as a Date, null for none (a zero time).
function timeOf(iso) {
  const d = iso ? new Date(iso) : null;
  return d && d.getFullYear() > 2000 ? d : null;
}

// linkState is how the server stands with a Cube (GET /api/v1/cubes,
// link): connected, held, stale, busy or offline.
function linkState(c) {
  return (c && c.link && c.link.state) || (c && c.connected ? "connected" : "offline");
}

// linkText says in a sentence why a Cube is not connected where the
// server knows more than that: its connection held open, or a Cube that
// refuses the server; "" otherwise.
function linkText(c) {
  const s = linkState(c);
  if (s === "held") {
    const until = Date.parse((c.link && c.link.until) || "");
    return t("link.held", { time: isNaN(until) ? "–" : fmtShortTime(until) });
  }
  if (s === "stale" || s === "busy") return t("link." + s);
  return "";
}

// renderHomeNotice says on Rooms why it shows nothing, or only what the
// server last had: no Cube configured, or the Cube chosen not connected,
// with the address the server tries, when the Cube last sent data or
// else last answered discovery, what to check, and for a role with cubes
// the way to System settings → Cubes. The rooms the server still has
// stay under it.
function renderHomeNotice() {
  const box = $("home-notice");
  const cube = state.cubes.find((c) => c.serial === state.serial);
  if (cube && cube.connected) {
    box.classList.add("hidden");
    return;
  }
  const title = box.querySelector(".home-notice-title");
  const detail = box.querySelector(".home-notice-detail");
  const check = box.querySelector(".home-notice-check");
  if (!cube) {
    title.textContent = t("home.noCube");
    detail.textContent = t(hasCap("cubes") ? "home.noCubeAdd" : "home.noCubeAsk");
    check.textContent = "";
  } else {
    // its name where it has one, its serial in the detail (as on its label)
    title.textContent = t("home.notConnected", { name: cube.name || cube.serial });
    const parts = [cube.address ? t("home.address", { address: cube.address }) : t("home.noAddress")];
    const why = linkText(cube); // a held connection, a refusal: first
    if (why) parts.unshift(why);
    if (cube.name) parts.push(t("home.serial", { serial: cube.serial }));
    const data = timeOf(cube.updated);
    const seen = timeOf(cube.last_seen);
    if (data) parts.push(t("home.lastData", { time: fmtInstant(data) }));
    else if (seen) parts.push(t("home.lastSeen", { time: fmtInstant(seen) }));
    else parts.push(t("home.noData"));
    detail.textContent = parts.join(" ");
    check.textContent = t("home.check");
  }
  check.classList.toggle("hidden", !check.textContent);
  box.classList.remove("hidden");
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

// fetchRooms loads a Cube's rooms, their virtual wall thermostats apart
// (splitVirtual), and its devices without a room or unlisted, as
// {rooms, unlisted}.
async function fetchRooms(serial) {
  const cube = "api/v1/cubes/" + encodeURIComponent(serial);
  const [rooms, devices] = await Promise.all([api("GET", cube + "/rooms?devices=all"), api("GET", cube + "/devices?devices=real")]);
  return { rooms: (rooms || []).map(splitVirtual), unlisted: (devices || []).filter((d) => d.unlisted || d.room_id === 0) };
}

async function loadRooms() {
  if (!state.serial) return;
  const serial = state.serial;
  const { rooms, unlisted } = await fetchRooms(serial);
  state.rooms = rooms;
  state.unlisted = unlisted;
  state.roomsOf = serial;
  renderRooms();
  // log.js: the device chosen is gone, or else the lines drawn before the
  // rooms came (a reload on the log, a Cube switch there) get their names
  if (logState.open) {
    if (fillLogSelects()) logChoicesChanged();
    else renderLog();
  }
}

// splitVirtual takes a room's virtual wall thermostat out of its devices
// into room.vwt (docs/api-v1.md, "The virtual wall thermostat"): the
// tile shows the room's temperature by it, expert mode lists it, and
// everything else goes by the real devices.
function splitVirtual(room) {
  room.vwt = room.devices.find((d) => d.virtual) || null;
  room.devices = room.devices.filter((d) => !d.virtual);
  return room;
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

// renderFound counts, under the header, the Cubes found on the LAN that
// the server does not serve: for a role with cubes a link to System
// settings → Cubes (showFound), which lists them with Add and the lines
// for the server's settings file (cubes.js); the count alone otherwise.
function renderFound() {
  const n = state.found.length;
  $("cube-found").classList.toggle("hidden", !n);
  if (!n) return;
  $("cube-found-link").textContent = tn("discover.found", n);
  $("cube-found-text").textContent = tn("discover.found", n);
}

// showFound opens System settings with the Cubes card in view, which
// lists the found Cubes.
function showFound() {
  showView("system");
  showCard($("sys-cubes"), true); // components.js
}

function selectCube(serial) {
  state.serial = serial;
  store("massimo.serial", serial);
  state.details = {};
  state.ui = {};
  refresh().then(connectEvents);
  logCubeChanged();
  backupCubeChanged();
}

// --- version ----------------------------------------------------------

// somethingOpen reports whether a reload would interrupt the user: a
// panel, a dialog, a device list, or a stepper value not yet sent.
function somethingOpen() {
  if (isShown("me") || isShown("system")) return true;
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
// renderTLS shows, from /healthz's tls, while the certificate the server
// serves is self-signed, a notice under the header and in the sign-in dialog
// asking to compare the fingerprint the browser shows for it with
// massimo tls show on the box before signing in; and a warning while
// the certificate ends within two weeks (or has none yet), computed
// here from not_after so that it speaks the page's language. The page
// shows no fingerprint: it comes over the very connection in question,
// and a man in the middle would pass the server's along, or rewrite it.
function renderTLS(tls) {
  const notice = $("tls-notice");
  $("login-tls").classList.toggle("hidden", !(tls && tls.self_signed));
  if (!tls) {
    notice.classList.add("hidden");
    return;
  }
  const until = Date.parse(tls.not_after || "");
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
  notice.classList.toggle("hidden", texts.length === 0);
}

// certText is the certificate the server serves, as GET /api/v1/system
// has it, for System Info (update.js): its source, names and expiry.
function certText(tls) {
  if (!tls) return "–";
  const until = Date.parse(tls.not_after || "");
  const parts = [tls.source, (tls.names || []).join(", ")];
  if (!isNaN(until)) parts.push(t("header.certUntil", { date: fmtInstant(new Date(until)) }));
  return parts.filter(Boolean).join(" · ") || "–";
}

// renderVersionTitle names the server's whole version in the tooltip of
// the short one in the header.
function renderVersionTitle() {
  $("version-short").title = state.version ? t("header.versionTitle", { version: state.version }) : "";
}

async function checkVersion() {
  let h;
  try {
    h = await api("GET", "healthz");
  } catch (e) {
    return;
  }
  const v = h && h.version ? h.version : "";
  renderTLS(h && h.tls);
  if (state.version === null) {
    state.version = v;
    const parts = splitVersion(v);
    $("version-short").textContent = parts.short;
    renderVersionTitle();
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

// clearCubeData forgets what the page loaded of the Cubes and draws it
// empty, for the next to sign in, who may be someone else (Sign out,
// login.js): the Cubes and the found ones, the rooms and the devices,
// the tiles' states with a setpoint not yet sent, the last poll and the
// Cube's clock; the poll stops until the sign-in starts it again.
function clearCubeData() {
  if (state.pollTimer) clearInterval(state.pollTimer);
  state.pollTimer = null;
  for (const ui of Object.values(state.ui)) if (ui.timer) clearTimeout(ui.timer);
  Object.assign(state, { cubes: [], found: [], rooms: [], unlisted: [], roomsOf: null, details: {}, ui: {}, lastPoll: null });
  logState.cubeRooms.clear(); // log.js: every Cube's rooms, as the last user saw them
  renderRooms(); // tiles.js
  renderFound();
  renderCubeSelect();
  renderHomeNotice();
  renderUpdated();
  for (const id of ["stats-cube", "duty", "slots", "pending-n", "queued"]) $(id).textContent = "";
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
// eventsLast is the last event id read and the Cube it is of: the next
// stream sends it as last_event_id and resumes after it, without the
// state burst, or answers resync when it cannot.
let eventsLast = null;

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
  const serial = state.serial;
  const resume = eventsLast && eventsLast.serial === serial ? eventsLast.id : "";
  const es = new EventSource(base() + "/events?devices=all&ticket=" + encodeURIComponent(ticket) +
    (resume ? "&last_event_id=" + encodeURIComponent(resume) : ""));
  state.events = es;
  // on adds a listener that first notes the event's id for the next stream.
  const on = (kind, fn) => es.addEventListener(kind, (ev) => {
    if (ev.lastEventId) eventsLast = { serial, id: ev.lastEventId };
    fn(ev);
  });
  es.onopen = () => {
    state.eventsOpen = true;
    eventsRetry = EVENTS_RETRY_MS;
  };
  // The connect burst of state events is not log traffic; a resumed
  // stream has none, unless the server answers resync and sends it.
  let initial = false;
  const burst = () => {
    initial = true;
    setTimeout(() => {
      initial = false;
    }, 1500);
  };
  if (!resume) burst();
  on("resync", () => {
    burst();
    Promise.resolve(refresh()).catch(() => {});
  });
  on("state", (ev) => {
    const data = JSON.parse(ev.data);
    if (data.state) updateDevice(data.state);
    if (!initial && !(data.state && data.state.virtual)) appendLogEvent(data); // the log is the real devices
  });
  // A handler's load that fails, as when the server is briefly gone, is
  // left to the next poll or event: its rejection is caught here, as the
  // other call sites catch theirs.
  const logged = (handler) => (ev) => {
    const data = JSON.parse(ev.data);
    appendLogEvent(data);
    const done = handler(data);
    if (done && typeof done.catch === "function") done.catch(() => {});
  };
  on("hello", logged(() => loadCubes()));
  on("metadata", logged(() => loadRooms()));
  // Pairing (cubes.js): a device that joined is among the devices
  // without a room once the server read its C line, which the reload
  // after its config event finds as well.
  on("pairing_started", logged(() => pairingEvent(serial)));
  on("pairing_ended", logged(() => pairingEvent(serial)));
  on("paired", (ev) => {
    const data = JSON.parse(ev.data);
    appendLogEvent(data);
    pairingJoined(serial, data.paired);
    loadRooms().catch(() => {});
  });
  on("device_forgotten", logged(() => loadRooms()));
  on("connected", logged(() => refresh()));
  on("disconnected", logged(() => loadCubes()));
  on("config", logged((data) => {
    // a device that just joined pairing: its C line makes it a device without a room
    if (data && data.device && !state.unlisted.some((d) => d.rf_address === data.device) &&
        !state.rooms.some((r) => r.devices.some((d) => d.rf_address === data.device))) loadRooms().catch(() => {});
  }));
  on("cube_found", (ev) => {
    const data = JSON.parse(ev.data);
    appendLogEvent(data);
    addFound(data.cube);
  });
  // A restore's steps and its end go to the log and to the backup panel
  // that sent it (backup.js).
  on("restore_step", logged(restoreEvent));
  on("restore_done", logged(restoreEvent));
  // A notification for the signed-in user, which only that user's
  // streams get: a toast, not a line of the log (notify.js).
  on("notification", (ev) => notifyToast(JSON.parse(ev.data).notification));
  // The log's other kinds (log.js LOG_KINDS): the server's own lines, a
  // Cube's address and its leaving the LAN, the changes, for the log
  // alone.
  const handled = new Set(["state", "hello", "metadata", "pairing_started", "pairing_ended", "paired", "device_forgotten",
    "connected", "disconnected", "config", "cube_found", "restore_step", "restore_done"]);
  for (const kind of LOG_KINDS) if (!handled.has(kind)) on(kind, logged(() => {}));
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
  // My settings opens from the 👤 menu (showView).
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
  $("refresh-btn").addEventListener("click", (ev) => whileBusy(ev.currentTarget, () => poll(true)));
  $("home-notice-cubes").addEventListener("click", () => showView("system"));
  $("cube-found-link").addEventListener("click", showFound);
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
  for (const id of ["lang-select", "login-lang"]) fillOptions($(id), options).value = i18n.setting; // My settings, the sign-in's menu
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
  pageBar($("system"));
  pageBar($("me"));
  renderLangSelect();
  applyAppearance();
  renderCubeSelect();
  renderFound();
  renderMe();
  renderVersionTitle();
  renderDeviceSwitches();
  renderUsers();
  renderProviders(); // providers.js
  if ($("login-dialog").open) renderLoginProviders(); // login.js
  renderBackups();
  renderPlan();
  renderRoles();
  renderCubes();
  const cube = state.cubes.find((c) => c.serial === state.serial);
  if (cube) renderHeader(cube);
  renderRooms();
  fillLogSelects();
  renderLog();
  if (updState.info) renderUpdate(updState.info);
  if (infoState.store || infoState.config) renderServerInfo();
  if (isShown("me")) renderNotify(); // notify.js
  tellWorkerLanguage();
}

// init starts the page once English and the chosen language are loaded.
async function init() {
  state.token = await readToken();
  await oidcReturn(); // login.js: back from a provider's sign-in, before the page reads its address
  state.serial = readStored("massimo.serial");
  loadSettings();
  await changeLanguage(state.settings.lang);
  if (I18N.en) applyI18n(document);
  renderLangSelect();
  initHelpSlots();
  // the header kept at the top; System settings' row there with its
  // sections menu, its cards that fold, back to top
  stickyHeader();
  for (const page of [$("system"), $("me")]) {
    sectionCards(page);
    pageBar(page);
    backToTop(page);
  }
  initSettings();
  applySettings();
  checkVersion();
  await loadMeAndPreferences(); // login.js: who is signed in, or the sign-in dialog, then the user's settings
  initTiles();
  initLog();
  initNotify(); // notify.js, before the page the address names is shown
  const hadSerial = !!state.serial;
  showAddress(); // log.js: the page the address names, the log with its choices
  refresh().then(() => {
    store("massimo.serial", state.serial);
    if (isShown("log") && !hadSerial) loadLog(); // it had no Cube to load from yet
    connectEvents();
  });
  setInterval(refreshAges, 60000);
}

document.addEventListener("DOMContentLoaded", init);
