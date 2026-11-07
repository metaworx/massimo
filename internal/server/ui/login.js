// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

// The Massimo web UI, login: who the page is signed in as (GET
// /api/v1/me), the sign-in dialog in place of the token prompt (a name
// and a password, signed in by OPAQUE so that the password stays in the
// browser, or an API key pasted), Log out, hiding what the role may not
// do, the own password, and the form that creates the first owner on a
// server without users. Loaded after app.js; api() calls askLogin on a
// 401.
"use strict";

// CAPABILITIES are the server's, in its order (docs/api-v1.md,
// "Authentication"); each hides, as a class needs-<capability> in the
// markup, what it alone allows.
const CAPABILITIES = ["read", "preferences", "setpoint", "lock", "configure", "rename", "structure", "pair", "discover", "backup", "restore",
  "guests", "users", "roles", "roles.disable", "capabilities", "cubes", "settings", "update", "audit"];

// DEFAULT_ROLES are the keys of the six default roles, for a role choice
// that cannot read the roles.
const DEFAULT_ROLES = ["owner", "manager", "operator", "guest", "viewer", "integration"];

// capClass is the class a capability is hidden by: the dot of
// roles.disable becomes a dash.
function capClass(c) {
  return c.replace(".", "-");
}

// hasCap reports whether the signed-in user may do something; outside
// users mode everyone may do everything.
function hasCap(c) {
  const me = state.me;
  if (!me || me.mode !== "users") return true;
  return (me.capabilities || []).includes(c);
}

// canManageUsers: the users routes, with users or with guests.
function canManageUsers() {
  return hasCap("users") || hasCap("guests");
}

// isSelf reports whether a user is the signed-in one.
function isSelf(name) {
  return !!(state.me && state.me.user && state.me.user.name === name);
}

// roleLabel is a role's display name, the key's translated name, or
// the key.
function roleLabel(key, name) {
  if (name) return name;
  const k = "role." + key;
  const s = t(k);
  return s === k ? key : s;
}

function setToken(token) {
  state.token = token;
  store("massimo.token", token);
}

// loadMe reads who the page is signed in as and applies it; a server
// that refuses opens the sign-in dialog through api().
async function loadMe() {
  try {
    state.me = await api("GET", "api/v1/me");
  } catch (e) {
    state.me = null;
  }
  applyCaps();
  return state.me;
}

// applyCaps hides what the signed-in user may not do: the body gets a
// class no-<capability> per capability the user lacks, and the markup
// marks what each one hides with needs-<capability>; the first-owner
// form shows on an open server for a caller on the local network.
function applyCaps() {
  const me = state.me;
  const body = document.body;
  for (const c of CAPABILITIES) body.classList.toggle("no-" + capClass(c), !hasCap(c));
  body.classList.toggle("no-users-admin", !canManageUsers());
  body.classList.toggle("no-sync", !canSync());
  $("setup").classList.toggle("hidden", !(me && me.setup));
  renderMe();
}

// renderMe says in the settings who is signed in, and what Log out does.
function renderMe() {
  const me = state.me;
  const line = $("me-line");
  const btn = $("logout-btn");
  let text = "";
  let label = null;
  if (me && me.mode === "users" && me.user) {
    text = t("me.signedIn", { name: me.user.display_name || me.user.name, role: roleLabel(me.role.key, me.role.name) });
    if (me.token && me.token.kind === "api") {
      text = t("me.withKey", { name: me.user.name, label: me.token.label || "" });
      label = t("settings.forgetKey");
    } else {
      label = t("settings.logout");
    }
  } else if (me && me.mode === "open") {
    text = t("me.open");
  } else if (me && me.mode === "token") {
    text = t("me.token");
    label = t("settings.forgetKey");
  } else if (me) {
    text = t("me.none");
  }
  line.textContent = text;
  line.classList.toggle("hidden", !text);
  btn.textContent = label || t("settings.logout");
  btn.classList.toggle("hidden", !label);
  $("me-password").classList.toggle("hidden", !(me && me.mode === "users" && me.user && me.user.has_password));
}

// --- the sign-in dialog -----------------------------------------------

let loginPromise = null;

// startWaits counts the starts that wait for who is in, the page's own
// and Log out's (loadMeAndPreferences): each goes on by itself after a
// sign-in, with the settings, the data and the stream, so the dialog
// starts nothing beside it (afterLogin), which would start the page twice.
let startWaits = 0;

// loadMeAndPreferences reads who is in, through the sign-in dialog where
// the server asks for one, then the user's settings from the server.
async function loadMeAndPreferences() {
  startWaits++;
  try {
    await loadMe();
  } finally {
    startWaits--;
  }
  await loadPreferences();
}

// askLogin opens the sign-in dialog and resolves true once a session or
// a key is in, false when the dialog was dismissed.
function askLogin() {
  if (loginPromise) return loginPromise;
  loginPromise = new Promise((resolve) => {
    const dlg = $("login-dialog");
    const form = $("login-form");
    const error = $("login-error");
    $("login-user").value = "";
    $("login-password").value = "";
    $("login-token").value = "";
    showResult(error, "", false);
    const done = (ok) => {
      form.onsubmit = null;
      dlg.onclose = null;
      if (dlg.open) dlg.close();
      loginPromise = null;
      resolve(ok);
    };
    // One sign-in at a time: the server refuses a second one from the
    // same address while the first is checked, so a submit meanwhile is
    // ignored.
    let signing = false;
    const submit = form.querySelector("button[type=submit]");
    form.onsubmit = async (ev) => {
      ev.preventDefault();
      if (signing) return;
      const key = $("login-token").value.trim();
      if (key) {
        setToken(key);
        done(true);
        if (!startWaits) afterLogin();
        return;
      }
      const user = $("login-user").value.trim();
      const password = $("login-password").value;
      if (!user || !password) {
        showResult(error, t("login.missing"), false);
        return;
      }
      // The key stretching runs in the browser and takes a moment on a
      // phone: the button says so meanwhile.
      signing = true;
      submit.disabled = true;
      submit.textContent = t("login.signingIn");
      try {
        const session = await signIn(user, password);
        setToken(session.token);
        done(true);
        if (!startWaits) afterLogin();
      } catch (e) {
        showResult(error, t("login.failed", { error: e.message }), false);
        $("login-password").focus();
      } finally {
        signing = false;
        submit.disabled = false;
        submit.textContent = t("login.submit");
      }
    };
    dlg.onclose = () => done(false);
    dlg.showModal();
    $("login-user").focus();
  });
  return loginPromise;
}

// --- the sign-in by OPAQUE (docs/api-v1.md, "The sign-in by OPAQUE") ----

// opaqueLoad is the vendored OPAQUE client (ui/vendor/opaque.js,
// @serenity-kit/opaque), imported when a sign-in or a password first
// needs it rather than with the page.
let opaqueLoad = null;

function opaqueModule() {
  if (!opaqueLoad) {
    opaqueLoad = import("./vendor/opaque.js").then(async (m) => {
      await m.ready;
      return m;
    });
    opaqueLoad.catch(() => {
      opaqueLoad = null;
    });
  }
  return opaqueLoad;
}

// keyStretching is the argon2id a record asks for, "m=19456,t=2,p=1" as
// the server sends it, in the client's terms.
function keyStretching(ksf) {
  const m = /^m=(\d+),t=(\d+),p=(\d+)$/.exec(ksf || "");
  if (!m) throw new Error("the server's key stretching " + JSON.stringify(ksf));
  return { "argon2id-custom": { memory: Number(m[1]), iterations: Number(m[2]), parallelism: Number(m[3]) } };
}

// NO_KE3 ends a sign-in whose answer the password did not open: a KE3 the
// server refuses as a wrong password, so that it lets the address go at
// once rather than after the start's minute.
const NO_KE3 = "A".repeat(86);

// signIn signs in, without a token and without the dialog's retry, and
// returns the session, or with the purpose "proof" a proof of the
// password; the answer's error is thrown. The password stays in the
// browser: the client runs OPAQUE and the server sees none of it. Where
// it does not open the server's answer and the server still takes the
// password sign-in, that follows, for a user still of version 1.
async function signIn(user, password, purpose) {
  const opaque = await opaqueModule();
  const start = opaque.client.startLogin({ password });
  const started = await apiRequest("POST", "api/v1/login/start", { user, ke1: start.startLoginRequest, purpose }, { auth: false });
  const done = opaque.client.finishLogin({
    clientLoginState: start.clientLoginState,
    loginResponse: started.ke2,
    password,
    keyStretching: keyStretching(started.ksf),
  });
  if (done) return apiRequest("POST", "api/v1/login/finish", { login: started.login, ke3: done.finishLoginRequest }, { auth: false });
  if (!started.v1 || purpose) {
    return apiRequest("POST", "api/v1/login/finish", { login: started.login, ke3: NO_KE3 }, { auth: false });
  }
  // The password sign-in takes the start over, so that the attempt counts
  // once; a user still of version 1 is upgraded at once.
  const session = await apiRequest("POST", "api/v1/login", { user, password, login: started.login }, { auth: false });
  if (session.upgrade) await upgradePassword(session, password);
  return session;
}

// upgradePassword moves a user of version 1 to version 2 right after its
// password sign-in: the password registered by OPAQUE in the browser and
// set with the one-time proof the sign-in answered, so that it crosses
// no more. A failure leaves the user of version 1, and the next sign-in
// tries again.
async function upgradePassword(session, password) {
  const name = session.user.name;
  try {
    const body = await registerPassword(name, password, session.token);
    body.proof = session.upgrade;
    await apiRequest("PUT", "api/v1/users/" + encodeURIComponent(name), body, { token: session.token });
    session.user.auth_version = 2;
  } catch (e) {
    // still of version 1
  }
}

// registerPassword makes a password of version 2 for a name, the password
// kept in the browser: the server answers the client's registration
// request, and the client's record comes back as the fields that set it,
// {record, ksf}, for POST /api/v1/users or PUT /api/v1/users/{name}. A
// token given is the one asked with, else the page's.
async function registerPassword(name, password, token) {
  const opaque = await opaqueModule();
  const reg = opaque.client.startRegistration({ password });
  const path = "api/v1/users/" + encodeURIComponent(name) + "/register";
  const request = { request: reg.registrationRequest };
  const answer = token ? await apiRequest("POST", path, request, { token }) : await api("POST", path, request);
  const done = opaque.client.finishRegistration({
    clientRegistrationState: reg.clientRegistrationState,
    registrationResponse: answer.response,
    password,
    keyStretching: keyStretching(answer.ksf),
  });
  return { record: done.registrationRecord, ksf: answer.ksf };
}

// currentPassword proves the current password for one's own change: for a
// user of version 2 a proof from a sign-in by OPAQUE, which keeps it in
// the browser, for one still of version 1 the password itself.
async function currentPassword(user, password) {
  if (user.auth_version === 2) return { proof: (await signIn(user.name, password, "proof")).proof };
  return { current_password: password };
}

// afterLogin reads who is in, and starts the stream and the data again,
// for a sign-in that no start waited for: a session that ended while the
// page ran, a key pasted, the first owner.
function afterLogin() {
  loadMe()
    .then(loadPreferences)
    .then(() => {
      refresh().then(connectEvents);
    });
}

// logout ends the session (an API key is only forgotten) and asks who
// is there again. The settings that followed the user go with it: a
// change still waiting is sent under the user's token first, then the
// browser forgets the user's values and schemes and keeps what is kept
// for this device only, so that the next to sign in neither sees them
// nor hands them to its own account, as loadPreferences would hand over
// what a server lacks. Neither request asks for a sign-in.
async function logout() {
  const me = state.me;
  const synced = canSync();
  for (const key of takeScheduled()) {
    await apiRequest("PUT", "api/v1/me/preferences/" + key, preferenceBody(key)).catch(() => {});
  }
  if (me && me.mode === "users" && me.token && me.token.kind === "session") {
    try {
      await apiRequest("POST", "api/v1/logout");
    } catch (e) {
      // the session may be gone already
    }
  }
  setToken(null);
  state.me = null;
  if (synced) await forgetUserSettings(); // app.js
  closeEvents();
  showView("tiles");
  await loadMeAndPreferences();
  refresh().then(connectEvents);
}

// --- the user's settings on the server ---------------------------------

// canSync reports whether the settings follow the signed-in user: a user
// with the capability preferences. A guest, an open server and the API
// token keep them in the browser only.
function canSync() {
  const me = state.me;
  return !!(me && me.mode === "users" && me.user && hasCap("preferences"));
}

const prefsSync = { timer: null, keys: new Set() };

// PREFS_DELAY is how long a change waits for the next before it goes to
// the server, so that a poll interval typed digit by digit is one write.
const PREFS_DELAY = 1000;

// schedulePreferences sends a preference (ui or schemes) to the server
// shortly, when the settings follow the user.
function schedulePreferences(key) {
  if (!canSync()) return;
  prefsSync.keys.add(key);
  if (prefsSync.timer) clearTimeout(prefsSync.timer);
  prefsSync.timer = setTimeout(pushPreferences, PREFS_DELAY);
}

// takeScheduled ends the wait of schedulePreferences and returns the
// preferences it held.
function takeScheduled() {
  if (prefsSync.timer) clearTimeout(prefsSync.timer);
  prefsSync.timer = null;
  const keys = [...prefsSync.keys];
  prefsSync.keys.clear();
  return keys;
}

// preferenceBody is what a preference sends: ui is the user's values as
// the browser keeps them, without this device's, schemes the user's
// schemes.
function preferenceBody(key) {
  return key === "ui" ? storedSettings() : schemesDocument(); // app.js, theme.js
}

// pushPreferences sends the preferences scheduled.
async function pushPreferences() {
  for (const key of takeScheduled()) {
    try {
      await api("PUT", "api/v1/me/preferences/" + key, preferenceBody(key));
    } catch (e) {
      showMessage(t("prefs.saveFailed", { error: e.message }), false);
    }
  }
}

// flushPreferences sends what is scheduled at once, as the page is left
// or hidden: a change made in the last second before a tab is closed
// would otherwise stay in this browser, and the next sign-in would take
// the server's older value over it. keepalive lets the request outlive
// the page.
function flushPreferences() {
  if (!prefsSync.timer) return;
  for (const key of takeScheduled()) {
    apiRequest("PUT", "api/v1/me/preferences/" + key, preferenceBody(key), { keepalive: true }).catch(() => {});
  }
}

// loadPreferences takes the signed-in user's settings from the server:
// the schemes first, the server's list as it is, so that a chosen scheme
// of the user's resolves and a deleted one stays deleted, then ui,
// applied where this device does not keep its own. What the server lacks
// it learns from the browser, so that the first sign-in keeps the
// settings and the schemes the browser had.
async function loadPreferences() {
  if (!canSync()) return;
  let prefs;
  try {
    prefs = (await api("GET", "api/v1/me/preferences")) || {};
  } catch (e) {
    return;
  }
  if (prefs.schemes) {
    takeSchemes(prefs.schemes); // theme.js
  } else if (themeState.custom.length) {
    schedulePreferences("schemes");
  }
  const ui = prefs.ui;
  if (ui && typeof ui === "object") {
    for (const k of SETTING_KEYS) {
      const v = validSetting(k, ui[k]);
      if (v !== undefined) storeUserSetting(k, v);
    }
  } else {
    schedulePreferences("ui");
  }
  await applyStoredSettings();
}

// --- the own password -------------------------------------------------

async function changeOwnPassword(ev) {
  ev.preventDefault();
  const msg = $("me-password-msg");
  const current = $("me-current").value;
  const password = $("me-new").value;
  if (password !== $("me-again").value) return showResult(msg, t("setup.mismatch"), false);
  if (!password) return showResult(msg, t("login.missing"), false);
  try {
    const me = state.me.user;
    const body = await currentPassword(me, current);
    Object.assign(body, await registerPassword(me.name, password));
    await api("PUT", "api/v1/users/" + encodeURIComponent(me.name), body);
    showResult(msg, t("me.passwordChanged"), true);
    $("me-current").value = "";
    $("me-new").value = "";
    $("me-again").value = "";
    loadMe();
  } catch (e) {
    showResult(msg, t("users.failed", { name: state.me.user.name, error: e.message }), false);
  }
}

// --- the first owner --------------------------------------------------

// createOwner handles the form of a server without users: the owner is
// created with a password of version 2 and signed in at once.
async function createOwner(ev) {
  ev.preventDefault();
  const msg = $("setup-msg");
  const name = $("setup-user").value.trim();
  const password = $("setup-password").value;
  if (!name || !password) return showResult(msg, t("login.missing"), false);
  if (password !== $("setup-again").value) return showResult(msg, t("setup.mismatch"), false);
  try {
    await api("POST", "api/v1/users", { name, role: "owner", ...(await registerPassword(name, password)) });
    const session = await signIn(name, password);
    setToken(session.token);
    showResult(msg, t("setup.done", { name }), true);
    afterLogin();
  } catch (e) {
    showResult(msg, t("setup.failed", { error: e.message }), false);
  }
}

function initLogin() {
  window.addEventListener("pagehide", flushPreferences);
  document.addEventListener("visibilitychange", () => {
    if (document.visibilityState === "hidden") flushPreferences();
  });
  $("logout-btn").addEventListener("click", logout);
  $("setup-form").addEventListener("submit", createOwner);
  $("me-password-form").addEventListener("submit", changeOwnPassword);
}

document.addEventListener("DOMContentLoaded", initLogin);
