// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

// The Massimo web UI, login: who the page is signed in as (GET
// /api/v1/me), the sign-in dialog in place of the token prompt (POST
// /api/v1/login with a name and a password, or an API key pasted), Log
// out, hiding what the role may not do, the own password, and the form
// that creates the first owner on a server without users. Loaded after
// app.js; api() calls askLogin on a 401.
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

// hasSystem reports whether the signed-in user sees System settings: one
// of the capabilities of its panels, Cubes, backups, users, roles and the
// software update.
function hasSystem() {
  return ["cubes", "pair", "backup", "roles", "capabilities", "update"].some(hasCap) || canManageUsers();
}

// hasProfile reports whether the page is signed in as a user, who has My
// profile; an open server and the server's token have none.
function hasProfile() {
  const me = state.me;
  return !!(me && me.mode === "users" && me.user);
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
  body.classList.toggle("no-system", !hasSystem());
  $("setup").classList.toggle("hidden", !(me && me.setup));
  renderMe();
  if (!mayShow(currentView)) showView("tiles"); // log.js: a page the user signed in now may not open
}

// renderMe says in the 👤 menu who is signed in, and what Sign out
// does: a session ends, an API key or the token is only forgotten (Forget
// key), and an open server or one without authentication has nothing to
// end and no profile, so the menu offers My settings alone.
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
      label = t("me.forgetKey");
    } else {
      label = t("me.signOut");
    }
  } else if (me && me.mode === "open") {
    text = t("me.open");
  } else if (me && me.mode === "token") {
    text = t("me.token");
    label = t("me.forgetKey");
  } else if (me) {
    text = t("me.none");
  }
  line.textContent = text;
  line.classList.toggle("hidden", !text);
  btn.textContent = label || t("me.signOut");
  btn.classList.toggle("hidden", !label);
  $("menu-profile").classList.toggle("hidden", !hasProfile());
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
    const submit = form.querySelector("button[type=submit]");
    const done = (ok) => {
      form.onsubmit = null;
      dlg.onclose = null;
      if (dlg.open) dlg.close();
      loginPromise = null;
      resolve(ok);
    };
    form.onsubmit = async (ev) => {
      ev.preventDefault();
      const key = $("login-token").value.trim();
      if (key) {
        setToken(key);
        done(true);
        if (!startWaits) afterLogin();
        return;
      }
      if (isBusy(submit)) return; // a second Enter while it signs in
      const user = $("login-user").value.trim();
      const password = $("login-password").value;
      if (!user || !password) {
        showResult(error, t("login.missing"), false);
        return;
      }
      showResult(error, "", false);
      let session;
      try {
        session = await whileBusy(submit, () => signIn(user, password));
      } catch (e) {
        showResult(error, t("login.failed", { error: e.message }), false);
        $("login-password").focus();
        return;
      }
      setToken(session.token);
      done(true);
      if (!startWaits) afterLogin();
    };
    dlg.onclose = () => done(false);
    dlg.showModal();
    $("login-user").focus();
  });
  return loginPromise;
}

// signIn posts the login, without a token and without the dialog's
// retry, and returns the session; the answer's error is thrown.
function signIn(user, password) {
  return apiRequest("POST", "api/v1/login", { user, password }, { auth: false });
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
  const submit = $("me-password-form").querySelector("button[type=submit]");
  if (isBusy(submit)) return;
  const current = $("me-current").value;
  const password = $("me-new").value;
  if (password !== $("me-again").value) return showResult(msg, t("setup.mismatch"), false);
  if (!password) return showResult(msg, t("login.missing"), false);
  try {
    await whileBusy(submit, () => api("PUT", "api/v1/users/" + encodeURIComponent(state.me.user.name), { password, current_password: current }));
    showResult(msg, t("me.passwordChanged"), true);
    $("me-current").value = "";
    $("me-new").value = "";
    $("me-again").value = "";
  } catch (e) {
    showResult(msg, t("users.failed", { name: state.me.user.name, error: e.message }), false);
  }
}

// --- the first owner --------------------------------------------------

// createOwner handles the form of a server without users: the owner is
// created and signed in at once, the button busy through both.
async function createOwner(ev) {
  ev.preventDefault();
  const msg = $("setup-msg");
  const submit = $("setup-form").querySelector("button[type=submit]");
  if (isBusy(submit)) return;
  if (!checkOwnerName()) {
    $("setup-user").focus();
    return;
  }
  const name = $("setup-user").value.trim();
  const password = $("setup-password").value;
  if (!name || !password) return showResult(msg, t("login.missing"), false);
  if (password !== $("setup-again").value) return showResult(msg, t("setup.mismatch"), false);
  try {
    const session = await whileBusy(submit, async () => {
      await api("POST", "api/v1/users", { name, password, role: "owner" });
      return signIn(name, password);
    });
    setToken(session.token);
    showResult(msg, t("setup.done", { name }), true);
    afterLogin();
  } catch (e) {
    const worded = wordNameError(e, name); // users.js
    if (worded) checkOwnerName.say(worded);
    showResult(msg, t("setup.failed", { error: worded || e.message }), false);
  }
}

// checkOwnerName checks the first owner's name as it is typed
// (watchName, users.js); set by initLogin.
let checkOwnerName = () => true;

function initLogin() {
  window.addEventListener("pagehide", flushPreferences);
  document.addEventListener("visibilitychange", () => {
    if (document.visibilityState === "hidden") flushPreferences();
  });
  $("logout-btn").addEventListener("click", logout);
  $("setup-form").addEventListener("submit", createOwner);
  checkOwnerName = watchName($("setup-user"), $("setup-name-msg"), () => []);
  $("me-password-form").addEventListener("submit", changeOwnPassword);
}

document.addEventListener("DOMContentLoaded", initLogin);
