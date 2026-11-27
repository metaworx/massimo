// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

// The Massimo web UI, core: API access with the token (the sign-in is
// login.js), the helpers the
// other files share that read the page's state (fmtTemp, fmtSetpoint,
// fmtAge, helpParts; fmtNum is in i18n.js, the components in
// components.js),
// settings, the Cube clock, loading of cubes and rooms,
// polling and the event stream with its tickets. The header is in
// header.js, the home notice and the search for Cubes in home.js; the
// rooms' actions are in rooms.js and tiles.js; the regions are parts
// (ui/src/), which this file calls through window.MassimoParts.
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
  settings: { poll: 10, expert: false, clock: "browser", lang: "auto", theme: "system", scheme: "massimo", blink: "system", iconWindow: "sash", iconRadio: "wifi" },
  device: {}, // the settings kept for this device alone (deviceOverrides), as last stored
  tz: {}, // per serial: {winter, summer} time zone rules from the cube detail
  details: {}, // device detail by rf address, fetched on demand
  ui: {}, // per room id: {open, pending, timer, vacation, confirm}
  pollTimer: null,
  version: null, // the server version the page was loaded with
  newVersion: false,
  serverDown: false, // the last poll got no answer (renderServerSign)
  repliedAt: 0, // when the server last answered a request (Date.now()), for the pile's Refresh now
  updateAt: 0, // when the page last asked whether an update is out (updateDue)
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
// page's token in place of the browser's storage, and with version 2
// offers the login by fingerprint (login.js appLogin) and the way back
// to its servers; null in a browser.
function appBridge() {
  const b = window.massimo;
  return b && typeof b.token === "function" && typeof b.setToken === "function" ? b : null;
}

// readToken is the token the page starts with: the app's, else the one
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
const SETTING_KEYS = ["poll", "expert", "clock", "lang", "theme", "scheme", "blink", "iconWindow", "iconRadio"];

// SETTING_LABELS name each setting's row, for its "only on this device"
// switch.
const SETTING_LABELS = {
  poll: "settings.poll",
  expert: "settings.expertShort",
  clock: "settings.clock",
  lang: "settings.language",
  theme: "appearance.mode",
  scheme: "appearance.scheme",
  blink: "appearance.blink",
  iconWindow: "appearance.iconWindow",
  iconRadio: "appearance.iconRadio",
};

// SETTING_STORE writes a value as massimo.<key> has always held it; the
// default is kept as no entry, but for the poll and the expert switch.
const SETTING_STORE = {
  poll: (v) => String(v),
  expert: (v) => (v ? "1" : "0"),
  clock: (v) => v,
  lang: (v) => (v === "auto" ? null : v),
  theme: (v) => (v === "system" ? null : v),
  scheme: (v) => (v === "massimo" ? null : v),
  blink: (v) => (v === "system" ? null : v),
  iconWindow: (v) => (v === "sash" ? null : v),
  iconRadio: (v) => (v === "wifi" ? null : v),
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
    case "blink":
      return BLINK_MODES.includes(v) ? v : undefined; // theme.js
    case "iconWindow":
      return Object.keys(ICON_CHOICES.window).includes(v) ? v : undefined; // components.js
    case "iconRadio":
      return Object.keys(ICON_CHOICES.radio).includes(v) ? v : undefined;
  }
  return undefined;
}

// storedSettings are the user's values as the browser keeps them, the
// defaults where it keeps none.
function storedSettings() {
  const s = { poll: 10, expert: false, clock: "browser", lang: "auto", theme: "system", scheme: "massimo", blink: "system", iconWindow: "sash", iconRadio: "wifi" };
  const poll = validSetting("poll", readStored("massimo.poll"));
  if (poll !== undefined) s.poll = poll;
  s.expert = readStored("massimo.expert") === "1";
  for (const k of ["clock", "lang", "theme", "scheme", "blink", "iconWindow", "iconRadio"]) {
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
  state.device = { ...d };
}

// deviceOnly reports whether a setting is kept for this device alone, as
// state.device has it, which My settings' switches draw from.
function deviceOnly(key) {
  return Object.prototype.hasOwnProperty.call(state.device, key);
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
  state.device = deviceOverrides();
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

// --- API --------------------------------------------------------------

function base() {
  return "api/v1/cubes/" + encodeURIComponent(state.serial);
}

// API_WAIT_MS is how long a request waits for the server's answer (AP
// MobileApp, F32, D82-A): a link that stalls answers nothing, and what
// waits for it, a button turning, would wait for good. A route that waits
// longer by design has its own limit (waitOf).
const API_WAIT_MS = 20000;

// waitOf is how long a request of method to path waits for its answer:
// API_WAIT_MS, but for the routes that wait longer by design, each a
// little longer than the server's own limit (internal/server): a repair's
// run ten minutes (repair_api.go); every other write to a Cube, its rooms
// or its devices, and a Cube added, the Cube's answer through its queue,
// up to five minutes for a device's settings, its removal and the
// configurations and week programs (config_api.go); the search for Cubes
// waits for a run going on and its own, each up to 30 s while another
// program holds the discovery port; the look for an update, asked or
// before an update is started, 30 s under the server's lock of the look,
// which a request may first wait for while another holds it, so twice
// that (update_api.go); the look for integrations on the LAN 30 s; the end
// of a sign-in through a provider up to 10 s for each of its calls there
// (oidc.go); a job of the server's backups, which has no limit of its own
// and waits for the one that runs, a backup made, copied, checked or
// retried, a copy deleted or a target added and read, a quarter of an
// hour, and a target's test, a minute there (serverbackups.go). The
// stream's ticket asks no Cube.
function waitOf(method, path) {
  const route = path.split("?")[0];
  if (route === "api/v1/cubes/discover") return 90000;
  if (method === "POST" && /^api\/v1\/server-backup-targets\/[^/]+\/test$/.test(route)) return 75000;
  if (method !== "GET" && /^api\/v1\/server-backups(\/|$)/.test(route) && !/\/pin$/.test(route)) return 900000;
  if (method === "POST" && route === "api/v1/server-backup-targets") return 900000;
  if (method !== "GET" && /^api\/v1\/cubes\/[^/]+\/events\/ticket$/.test(route)) return API_WAIT_MS;
  if (method !== "GET" && /^api\/v1\/cubes\/[^/]+\/(.+\/)?repair\//.test(route)) return 630000;
  if (method !== "GET" && (route === "api/v1/cubes" || /^api\/v1\/cubes\/[^/]+\/./.test(route))) return 330000;
  if (route === "api/v1/update") return 75000;
  if (method === "GET" && route === "api/v1/integrations/found") return 45000;
  if (route === "api/v1/login/oidc/finish") return 45000;
  return API_WAIT_MS;
}

// noAnswer is the error of a request the server did not answer within its
// limit (waitOf): a read failed, a write's text saying that it may have
// been done all the same, since it may have reached the server; its status
// 0, as no answer has one.
function noAnswer(method) {
  const err = new Error(t(method === "GET" ? "api.noAnswer" : "api.noAnswerWrite"));
  err.status = 0;
  err.noAnswer = true;
  return err;
}

// apiRequest sends one request and returns the answer's JSON, null for
// an empty one; an answer that is not ok is thrown as an Error whose
// message is the refusal in the page's language, else the server's
// error, else the status (refusalError). The token
// goes along unless auth is false, and a token given in its place;
// keepalive lets the request outlive the page, and has no time limit;
// every other request is aborted where its answer has not come within
// its limit (waitOf), and fails as no answer (noAnswer). api() adds the
// sign-in on a 401; the sign-in itself and the last preferences sent as
// the page is left use it as it is (login.js).
async function apiRequest(method, path, body, { auth = true, keepalive = false, token = "", headers: extra = {} } = {}) {
  const headers = { ...extra };
  const bearer = token || (auth && state.token);
  if (bearer) headers.Authorization = "Bearer " + bearer;
  if (body !== undefined) headers["Content-Type"] = "application/json";
  const stop = keepalive ? null : new AbortController();
  const timer = stop ? setTimeout(() => stop.abort(), waitOf(method, path)) : null;
  let resp, text;
  try {
    resp = await fetch(path, { method, headers, body: body === undefined ? undefined : JSON.stringify(body), keepalive, signal: stop ? stop.signal : undefined });
    text = await resp.text(); // the answer's body within the same limit
  } catch (e) {
    if (stop && stop.signal.aborted) throw noAnswer(method);
    throw e;
  } finally {
    if (timer) clearTimeout(timer);
  }
  noteServerClock(resp);
  state.repliedAt = Date.now();
  let data = null;
  if (text) {
    try {
      data = JSON.parse(text);
    } catch (e) {
      data = { error: text };
    }
  }
  if (!resp.ok) {
    const err = refusalError(resp, data, path);
    err.sentWith = bearer || ""; // the token it carried, for dropToken
    throw err;
  }
  return data;
}

// refusalError is the Error of an answer that is not ok, data its JSON
// (null for none), to path: its message the page's text of the refusal
// (refusalText), else the status line; the status, the data, the code
// ("" for none) and the params (null for none) kept on it.
function refusalError(resp, data, path) {
  const err = new Error(refusalText(data, cubeOfPath(path), resp.status + " " + resp.statusText));
  err.status = resp.status;
  err.data = data;
  err.code = (data && typeof data.code === "string" && data.code) || "";
  err.params = (data && data.params) || null;
  return err;
}

// cubeOfPath is the serial of the Cube an API path is for, "" for a path
// of no Cube's.
function cubeOfPath(path) {
  const m = /^api\/v1\/cubes\/([^/?]+)/.exec(path || "");
  if (!m) return "";
  try {
    return decodeURIComponent(m[1]);
  } catch (e) {
    return m[1];
  }
}

// refusalText is the page's text of a refusal of the server (AP
// ErrorCodes): an error answer's {error, code, params}, or one kept with
// a result that was no thrown error. Where English has a text for its
// code (err.<code>), that text in the page's language, its params
// filled (refusalParams) for serial, the Cube it concerns; else the
// server's English error; else fallback. A text, never markup.
function refusalText(obj, serial, fallback) {
  const o = obj && typeof obj === "object" ? obj : {};
  if (typeof o.code === "string" && o.code) {
    const key = "err." + o.code;
    if (t(key) !== key) return t(key, refusalParams(o.params, serial));
  }
  return (typeof o.error === "string" && o.error) || fallback || "";
}

// refusalParams are a refusal's params as its text shows them: room, a
// room's id, by the room's name, device, a radio address or a serial, by
// the device's (deviceName), and each of devices so, where serial is the
// selected Cube's and its rooms are loaded; every other value, and these
// of another Cube, as given, a list joined by commas.
function refusalParams(params, serial) {
  const out = {};
  const own = !!serial && !!state.serial && state.roomsOf === state.serial && String(serial).toLowerCase() === String(state.serial).toLowerCase();
  const devices = own ? state.rooms.flatMap((r) => (r.vwt ? r.devices.concat([r.vwt]) : r.devices)).concat(state.unlisted) : [];
  const device = (v) => {
    const k = String(v).toLowerCase();
    const d = devices.find((x) => x.rf_address === k || (!!x.serial && x.serial.toLowerCase() === k));
    return d ? deviceName(d) : String(v);
  };
  for (const [k, v] of Object.entries(params && typeof params === "object" ? params : {})) {
    if (k === "room" && own) {
      const r = state.rooms.find((x) => x.id === Number(v));
      out[k] = r ? r.name : String(v);
    } else if (k === "device" && own) {
      out[k] = device(v);
    } else if (Array.isArray(v)) {
      out[k] = v.map((x) => (k === "devices" && own ? device(x) : String(x))).join(", ");
    } else {
      out[k] = v;
    }
  }
  return out;
}

// dropToken forgets the page's token at a 401 that a request carrying it
// met: the session has ended, so what the page sends until the sign-in
// carries none, which the server does not count against the address,
// where a dead token sent by each poll and stream ticket would lock the
// address out within the minute. A 401 of a token the page no longer
// holds, one a sign-in since replaced, forgets nothing. The app forgets
// it too (setToken, login.js). In a browser the stored token goes only
// where it is still the one refused: another tab's sign-in may have stored
// its own since, ending the one this page held (AP MobileApp, D68-A).
function dropToken(sentWith) {
  if (!sentWith || sentWith !== state.token) return;
  if (!appBridge() && readStored("massimo.token") !== sentWith) {
    state.token = null;
    return;
  }
  setToken(null);
}

// sentAgain reports whether a request that met a 401 is sent again once
// a sign-in is in: a read, and the page's own requests that change
// nothing. A write, every other request, is never sent again: it went out
// under a session that has ended, and sent later it would go under
// whoever signs in, however much later (AP SignInResend).
function sentAgain(method, path) {
  if (method === "GET") return true;
  // the event stream's ticket: the page asks for it by itself to open the
  // stream, which the sign-in starts again, and it changes nothing
  return method === "POST" && /\/events\/ticket$/.test(path);
}

// notSentTexts are the texts that said a write was not sent, in each
// language they were said in, so that the page knows them again on its
// lines (saysNotSent); notSentNow is set once a write was not sent since
// the sign-in last opened (askLogin, login.js).
const notSentTexts = new Set();
let notSentNow = false;

// notSentText is the text of a write not sent since the user was signed
// out, noted in notSentTexts and notSentNow.
function notSentText() {
  const text = t("api.notSent");
  notSentTexts.add(text);
  notSentNow = true;
  return text;
}

// saysNotSent reports whether a line's text says a write was not sent.
function saysNotSent(text) {
  return [...notSentTexts].some((s) => text.includes(s));
}

// forgetNotSent takes back every line of the plain regions that says a
// write was not sent (failedLines, components.js), for a user who is
// gone (forgetSession, login.js); the parts' lines go with the Cube data
// and the House's panel.
function forgetNotSent() {
  for (const el of failedLines) if (saysNotSent(el.textContent)) showResult(el, "");
  notSentNow = false;
}

// notSent opens the sign-in and returns the error a write that met a 401
// fails with at once: not sent, to be done again after the sign-in; its
// status stays 401.
function notSent() {
  askLogin(); // login.js: the write does not wait for it
  const err = new Error(notSentText());
  err.status = 401;
  return err;
}

// earlierSignIn reports whether a 401 is of a sign-in before the page's
// current one: its request carried another token than the one the page
// holds now, or none, while it holds one, as a read sent while the card
// was open whose answer comes once the card has closed (AP MobileApp,
// F36, D68-A). Such a 401 opens no card and drops nothing: a read is sent
// again with the token held, a write fails as not sent (lateNotSent). The
// session's number cannot tell it: a sign-in nobody was signed in before
// raises none (settleSignIn, login.js).
function earlierSignIn(sentWith) {
  return !!state.token && sentWith !== state.token;
}

// lateNotSent is the error a write fails with whose 401 was of an earlier
// sign-in (earlierSignIn): not sent, to be done again, as notSent says it,
// without opening the sign-in, which is in; its status stays 401.
function lateNotSent() {
  const err = new Error(notSentText());
  err.status = 401;
  return err;
}

// api is apiRequest with the token, asking for a sign-in on a 401: a read
// is sent again once one is in, a write fails at once (sentAgain); a 401
// of an earlier sign-in asks for none (earlierSignIn); headers are added
// to the request's own.
async function api(method, path, body, headers) {
  try {
    return await apiRequest(method, path, body, { headers });
  } catch (e) {
    if (e.status !== 401) throw e;
    if (earlierSignIn(e.sentWith)) {
      if (!sentAgain(method, path)) throw lateNotSent();
      return api(method, path, body, headers);
    }
    dropToken(e.sentWith);
    if (!sentAgain(method, path)) throw notSent();
    if (await askLogin()) return api(method, path, body, headers); // login.js
    throw new Error(t("api.badToken"));
  }
}

// apiRaw is api for what is not JSON, a backup file: body goes as it is,
// with contentType, and the answer is the Response; an error is thrown
// as api throws it, a write that met a 401 not sent again.
async function apiRaw(method, path, body, contentType) {
  const headers = {};
  const sentWith = state.token;
  if (sentWith) headers.Authorization = "Bearer " + sentWith;
  if (body !== undefined) headers["Content-Type"] = contentType || "application/octet-stream";
  const resp = await fetch(path, { method, headers, body });
  if (resp.status === 401) {
    if (earlierSignIn(sentWith || "")) {
      if (!sentAgain(method, path)) throw lateNotSent();
      return apiRaw(method, path, body, contentType);
    }
    dropToken(sentWith);
    if (!sentAgain(method, path)) throw notSent();
    if (await askLogin()) return apiRaw(method, path, body, contentType); // login.js
    throw new Error(t("api.badToken"));
  }
  if (!resp.ok) {
    let data = null;
    try {
      data = JSON.parse(await resp.text());
    } catch (e) {
      // not JSON: the status says it
    }
    throw refusalError(resp, data, path);
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
  const n = MassimoParts.session();
  const d = await api("GET", devicePath(dev));
  if (!MassimoParts.stillSession(n)) return d; // the user before's: kept nowhere, its awaiter asks its own session
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

// TEMP_OFF is the setpoint a device shows as OFF, its valve shut
// (cube.TempOff).
const TEMP_OFF = 4.5;

// fmtSetpoint is a setpoint as a device's display shows it, for the
// plain scripts (the tiles' result lines of rooms.js and tiles.js, the
// week programme's segments through its kit): OFF for 4.5 °C, without a
// unit, else as fmtTemp gives it; no ON for 30.5 °C. The number fields
// keep 4.5. The parts' is fmtSetpoint of ui/src/shared/setpoint.tsx.
function fmtSetpoint(v, unit) {
  return v === TEMP_OFF ? t("temp.off") : fmtTemp(v, unit);
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

// fmtTimeOfDay is an instant as the browser's time of day, "20:51:03",
// with its date before it when it is not today's (a log line's time, the
// header's Refresh now).
function fmtTimeOfDay(d) {
  const pad = (n) => String(n).padStart(2, "0");
  const time = pad(d.getHours()) + ":" + pad(d.getMinutes()) + ":" + pad(d.getSeconds());
  if (d.toDateString() === new Date().toDateString()) return time;
  return new Intl.DateTimeFormat(undefined, { day: "2-digit", month: "2-digit" }).format(d) + " " + time;
}

// fmtShortTime is an instant (ms) as day, month and time.
function fmtShortTime(ms) {
  const opts = { day: "2-digit", month: "2-digit", hour: "2-digit", minute: "2-digit", hour12: false };
  return new Intl.DateTimeFormat(i18n.lang, opts).format(new Date(ms));
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

// tomorrowMorning is the default "until" of a temporary setting: tomorrow
// at 06:00, the next day by the calendar rather than 24 hours on, since
// the day of a change of the clocks has 23 or 25 hours.
function tomorrowMorning() {
  const d = new Date();
  d.setDate(d.getDate() + 1);
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

function applySettings() {
  const s = state.settings;
  document.body.classList.toggle("expert", s.expert);
  applyAppearance();
  const cube = state.cubes.find((c) => c.serial === state.serial);
  if (cube) renderHeader(cube);
  restartPolling();
}

// --- loading ----------------------------------------------------------

// A plain load takes the session's number before its request
// (MassimoParts.session(); a sign-out, or another user's sign-in after
// a 401, raises it: UiParts D212). Where its answer comes after the
// session it was asked in had ended, it writes, draws and says nothing
// and still settles: it resolves false (a device's detail: the answer,
// kept nowhere), so that every awaiter's finally runs and nothing of
// the next session waits on it. A plain chain that would go on from it
// tests the value; a part's awaiter asks its own session.

// loadCubes reads the Cubes; false where the answer came after the
// session ended (above), else true.
async function loadCubes() {
  const n = MassimoParts.session();
  const all = (await api("GET", "api/v1/cubes")) || [];
  if (!MassimoParts.stillSession(n)) return false; // the user before's
  state.cubes = all.filter(isConfigured);
  state.found = all.filter((c) => !isConfigured(c));
  MassimoParts.cubes.polled(); // the panels follow the list, a Cube gone forgets its forms
  state.lastPoll = new Date();
  if (!state.serial || !state.cubes.some((c) => c.serial === state.serial)) {
    state.serial = state.cubes.length ? state.cubes[0].serial : null;
  }
  renderHomeNotice(); // the header with it (home.js)
  MassimoParts.log.cubesPolled(); // the Cubes choice, and under All Cubes a new Cube's rooms
  const cube = state.cubes.find((c) => c.serial === state.serial);
  if (cube) {
    if (!state.tz[cube.serial]) await loadTimeZone(cube.serial);
    if (!MassimoParts.stillSession(n)) return false;
    renderHeader(cube);
  }
  return true;
}

// timeOf is an API time as a Date, null for none (a zero time).
function timeOf(iso) {
  const d = iso ? new Date(iso) : null;
  return d && d.getFullYear() > 2000 ? d : null;
}

// linkState is how the server stands with a Cube (GET /api/v1/cubes,
// link): connected, held, stale, busy, rebooting or offline.
function linkState(c) {
  return (c && c.link && c.link.state) || (c && c.connected ? "connected" : "offline");
}

// linkText says in a sentence why a Cube is not connected where the
// server knows more than that: its connection held open, a Cube that
// refuses the server, or one that reboots; "" otherwise.
function linkText(c) {
  const s = linkState(c);
  if (s === "held") {
    const until = Date.parse((c.link && c.link.until) || "");
    return t("link.held", { time: isNaN(until) ? "–" : fmtShortTime(until) });
  }
  if (s === "stale" || s === "busy" || s === "rebooting") return t("link." + s);
  return "";
}

// loadTimeZone reads the Cube's daylight-saving rules once per Cube.
async function loadTimeZone(serial) {
  const n = MassimoParts.session();
  try {
    const detail = await api("GET", "api/v1/cubes/" + encodeURIComponent(serial));
    if (!MassimoParts.stillSession(n)) return;
    const cfg = detail && detail.config;
    state.tz = { ...state.tz, [serial]: cfg && cfg.tz_winter ? { winter: cfg.tz_winter, summer: cfg.tz_summer } : false };
  } catch (e) {
    if (!MassimoParts.stillSession(n)) return;
    state.tz = { ...state.tz, [serial]: false };
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

// loadRooms reads the chosen Cube's rooms; false where the answer came
// after the session ended or another Cube was chosen, else true. One read
// runs at a time, and one more waits behind it, which every call while it
// runs shares: a burst of calls, as a reconnect's state events and a
// write's re-reads bring them, reads the rooms at most twice, the last
// time after the last call, and an answer from before a later call cannot
// take that one's place. A call of another session or for another Cube
// than the read running starts its own at once (the earlier read's answer
// changes nothing, readRooms), and a read waiting for a session that ended
// or a Cube no longer chosen is not made.
let roomsRun = null; // the read running: { n, serial, promise }, n its session
let roomsNext = null; // the read waiting behind it, for the same session and Cube
function loadRooms() {
  const n = MassimoParts.session();
  const serial = state.serial;
  if (roomsRun && roomsRun.n === n && roomsRun.serial === serial) {
    if (!roomsNext) {
      const next = roomsRun.promise.catch(() => {}).then(() => {
        if (roomsNext === next) roomsNext = null;
        return MassimoParts.stillSession(n) && state.serial === serial ? loadRooms() : false;
      });
      roomsNext = next;
    }
    return roomsNext;
  }
  const run = { n, serial, promise: null };
  run.promise = readRooms().finally(() => {
    if (roomsRun === run) roomsRun = null;
  });
  roomsRun = run;
  roomsNext = null;
  return run.promise;
}

// readRooms is one read of the chosen Cube's rooms (loadRooms).
async function readRooms() {
  if (!state.serial) return true;
  const serial = state.serial;
  const n = MassimoParts.session();
  const { rooms, unlisted } = await fetchRooms(serial);
  if (!MassimoParts.stillSession(n)) return false; // the user before's: the next user's load reads theirs
  if (state.serial !== serial) return false; // the Cube before's: the chosen one's load reads its own
  state.rooms = rooms;
  state.unlisted = unlisted;
  state.roomsOf = serial;
  MassimoParts.rooms.loaded(); // the forms of what it no longer gives forgotten, a room asked to be opened opened
  renderRooms();
  // the log: the device chosen is gone, or else the lines drawn before the
  // rooms came (a reload on the log, a Cube switch there) get their names
  const log = MassimoParts.log;
  if (log.state.open) {
    if (log.selects()) log.choicesChanged();
    else log.render();
  }
  return true;
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

// refresh reads the Cubes and the rooms; false where an answer came after
// the session ended, else true, a failure said on the message line.
async function refresh() {
  const n = MassimoParts.session();
  try {
    if (!(await loadCubes()) || !(await loadRooms())) return false;
    MassimoParts.log.loadAlerts(); // the log button's sign
    if (langError) showLanguageError();
    else $("message").classList.add("hidden");
  } catch (e) {
    if (!MassimoParts.stillSession(n)) return false;
    showMessage(t("msg.cannotLoad", { error: e.message }), false);
  }
  return true;
}

// isConfigured tells the Cubes the server serves from those it only
// found on the LAN; a server without discovery sends no flag.
function isConfigured(c) {
  return c.configured !== false;
}

function selectCube(serial) {
  state.serial = serial;
  store("massimo.serial", serial);
  state.details = {};
  for (const ui of Object.values(state.ui)) if (ui.awaitTimer) clearTimeout(ui.awaitTimer); // rooms.js awaitMembers
  state.ui = {};
  refresh().then((fresh) => fresh && connectEvents());
  MassimoParts.log.cubeChanged();
  MassimoParts.backups.cubeChanged();
}

// --- polling ----------------------------------------------------------

// poll refreshes the header, the log button's sign, and runs the version
// check; the rooms are refreshed as well while the event stream is not
// connected, or always when the user asked (the pile's Refresh now).
function poll(rooms) {
  checkVersion();
  MassimoParts.log.loadAlerts(); // the log button's sign
  return loadCubes().then((fresh) => {
    if (fresh && (rooms || !state.eventsOpen)) return loadRooms();
  }).catch(() => {});
}

// clearCubeData forgets what the page loaded of the Cubes and draws it
// empty, for the next to sign in, who may be someone else (Sign out,
// login.js): the Cubes and the found ones, the rooms and the devices,
// the tiles' states with a setpoint not yet sent and a setpoint's wait for
// the thermostats (rooms.js awaitMembers), the last poll and the Cube's
// clock; the poll stops until the sign-in starts it again.
function clearCubeData() {
  if (state.pollTimer) clearInterval(state.pollTimer);
  state.pollTimer = null;
  for (const ui of Object.values(state.ui)) {
    if (ui.timer) clearTimeout(ui.timer);
    if (ui.awaitTimer) clearTimeout(ui.awaitTimer);
  }
  Object.assign(state, { cubes: [], found: [], rooms: [], unlisted: [], roomsOf: null, details: {}, ui: {}, lastPoll: null });
  renderRooms(); // rooms.js
  renderHomeNotice(); // the header with it (home.js)
  for (const id of ["stats-cube", "duty", "slots", "pending-n", "queued"]) $(id).textContent = "";
}

// startPolling runs poll every poll interval. The page's start starts it
// once the card it waits for has closed (init), and so do a sign-in no
// start waits for, a sign-out's start and a card left that the running
// page stood still behind (login.js).
function startPolling() {
  if (state.pollTimer) clearInterval(state.pollTimer);
  state.pollTimer = setInterval(() => poll(false), state.settings.poll * 1000);
}

// restartPolling starts the poll again at the poll setting's pace where it
// runs, for a change of the settings; a poll that does not run stays
// stopped: nothing of the page is read behind the sign-in card or the
// first owner's (AP LooseEnds, Block 3).
function restartPolling() {
  if (state.pollTimer) startPolling();
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
  // a page standing still behind the first owner's card opens no stream: the card opens it again where it is left
  // (askLogin, login.js; AP FirstOwner, D35), whatever asks meanwhile (a restart's end, serverBack)
  if (loginPromise && loginPromise.ask && loginPromise.ask.still) return;
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
    MassimoParts.backups.resynced(); // a restore's end the stream lost
    MassimoParts.serverEvent({ kind: "resync" }); // the parts that follow the server read again what they show
    Promise.resolve(refresh()).catch(() => {});
  });
  // inRoom says whether a device, by its radio address, is a thermostat of
  // one of the rooms.
  const inRoom = (rf) => state.rooms.some((r) => r.devices.some((d) => d.rf_address === rf && isThermostat(d)));
  on("state", (ev) => {
    const data = JSON.parse(ev.data);
    if (data.state) updateDevice(data.state);
    if (!initial && !(data.state && data.state.virtual)) MassimoParts.log.event(data); // the log is the real devices
    // the Cube gained or lost the state of a thermostat of a room: the one
    // that stands for the room, the first of its order with the dead passed
    // over (room.thermostat), and the room's eco and comfort may be others;
    // the connect burst's events carry no changes
    if (data.changes && data.changes.reported && inRoom(data.device)) loadRooms().catch(() => {});
  });
  // A handler's load that fails, as when the server is briefly gone, is
  // left to the next poll or event: its rejection is caught here, as the
  // other call sites catch theirs.
  const logged = (handler) => (ev) => {
    const data = JSON.parse(ev.data);
    MassimoParts.log.event(data);
    const done = handler(data);
    if (done && typeof done.catch === "function") done.catch(() => {});
  };
  on("hello", logged(() => loadCubes()));
  on("metadata", logged(() => loadRooms()));
  // Pairing (ui/src/cubes/flow.ts): a device that joined is among the devices
  // without a room once the server read its C line, which the reload
  // after its config event finds as well.
  on("pairing_started", logged(() => MassimoParts.cubes.pairingEvent(serial)));
  on("pairing_ended", logged(() => MassimoParts.cubes.pairingEvent(serial)));
  on("paired", (ev) => {
    const data = JSON.parse(ev.data);
    MassimoParts.log.event(data);
    MassimoParts.cubes.pairingJoined(serial, data.paired);
    loadRooms().catch(() => {});
  });
  on("device_forgotten", logged(() => loadRooms()));
  // A device's battery level changed: its row's badge follows (rooms.js),
  // and the parts that follow the server see it as before.
  on("battery", logged((data) => {
    updateBattery(data);
    MassimoParts.serverEvent(data);
  }));
  // A write a device has not shown by its deadline, or showed late: the
  // rooms' confirmations changed without a state event (confirm.go).
  on("unconfirmed", logged(() => loadRooms()));
  on("connected", logged(() => refresh()));
  on("disconnected", logged(() => loadCubes()));
  // the Cube's memory slots full, or free again: its panel says so
  on("slots", logged(() => loadCubes()));
  // a C line and the room list, or a room's members, that differ or
  // agree again: the tiles' marks
  on("mismatch", logged(() => loadRooms()));
  on("config", logged((data) => {
    // the device's entry with its new configuration (its eco and comfort
    // temperatures, which mark its setpoint), as a state event brings it
    if (data && data.state && !data.state.virtual) updateDevice(data.state);
    // a thermostat of a room with new presets: the room's eco and comfort,
    // decided over its thermostats (room.eco_temp, room.comfort_temp), which
    // mark the tile's setpoint and More's presets, may be others
    if (data && Array.isArray(data.changed) && data.changed.includes("temps") && inRoom(data.device)) loadRooms().catch(() => {});
    // a device that just joined pairing: its C line makes it a device without a room
    if (data && data.device && !state.unlisted.some((d) => d.rf_address === data.device) &&
        !state.rooms.some((r) => r.devices.some((d) => d.rf_address === data.device))) loadRooms().catch(() => {});
  }));
  on("cube_found", (ev) => {
    const data = JSON.parse(ev.data);
    MassimoParts.log.event(data);
    addFound(data.cube);
  });
  // A restore's steps and its end go to the log and to the backup panel
  // that sent it (ui/src/backups/).
  on("restore_step", logged((data) => MassimoParts.backups.restoreEvent(data)));
  on("restore_done", logged((data) => MassimoParts.backups.restoreEvent(data)));
  // A notification for the signed-in user, which only that user's
  // streams get: a toast, not a line of the log (notify.js).
  on("notification", (ev) => notifyToast(JSON.parse(ev.data).notification));
  // The log's other kinds (ui/src/log/kinds.ts): the server's own lines, a
  // Cube's address and its leaving the LAN, the changes, for the log, and
  // for the parts that follow the server while their page is shown
  // (MassimoParts.serverEvent).
  const handled = new Set(["state", "hello", "metadata", "pairing_started", "pairing_ended", "paired", "device_forgotten", "battery",
    "connected", "disconnected", "slots", "mismatch", "config", "cube_found", "restore_step", "restore_done", "unconfirmed"]);
  for (const kind of MassimoParts.log.kinds) if (!handled.has(kind)) on(kind, logged((data) => MassimoParts.serverEvent(data)));
  es.onerror = () => {
    if (gen !== eventsGen) return;
    es.close();
    state.events = null;
    state.eventsOpen = false;
    loadCubes().catch(() => {});
    retryEvents(gen);
  };
}

// --- wiring -----------------------------------------------------------

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
  MassimoParts.language(); // the parts draw their texts again (ui/src/shared/lang.ts)
  initHelpSlots();
  applyAppearance();
  MassimoParts.header.render(); // the header's row folded anew for the texts' widths
  renderServerSign();
  const cube = state.cubes.find((c) => c.serial === state.serial);
  if (cube) renderHeader(cube); // the radio budget's plain texts
  MassimoParts.log.selects();
  MassimoParts.log.render(); // the lines shown as the filter matches their texts in the new language
  MassimoParts.log.sign();
  if (MassimoParts.pages.isShown("me")) MassimoParts.notify.render();
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
  MassimoParts.language(); // the page's language loaded: the parts drawn as parts.js loaded, before any table had come, draw again in it (ui/src/shared/lang.ts)
  initHelpSlots();
  initAppearance(); // theme.js: the system's changes the appearance follows
  applySettings();
  checkVersion();
  await loadMeAndPreferences(); // login.js: who is signed in, or the sign-in dialog, then the user's settings
  initRooms();
  window.addEventListener("popstate", () => MassimoParts.pages.showAddress()); // back and forward, a page named in the address bar
  MassimoParts.log.mount(); // the log's page into its section
  initNotify(); // notify.js, before the page the address names is shown
  const hadSerial = !!state.serial;
  MassimoParts.pages.showAddress(); // the page the address names, the log with its choices
  startPolling(); // only now, beside the first refresh: nothing of the page is read behind the start's card
  refresh().then((fresh) => {
    if (!fresh) return; // another user signed in meanwhile: their sign-in started the data (login.js afterLogin)
    store("massimo.serial", state.serial);
    if (MassimoParts.pages.isShown("log") && !hadSerial) MassimoParts.log.load(); // it had no Cube to load from yet
    connectEvents();
  });
}

document.addEventListener("DOMContentLoaded", init);
