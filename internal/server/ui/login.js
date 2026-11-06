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
const CAPABILITIES = ["read", "setpoint", "lock", "configure", "rename", "structure", "pair", "discover", "backup", "restore",
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
    form.onsubmit = async (ev) => {
      ev.preventDefault();
      const key = $("login-token").value.trim();
      if (key) {
        setToken(key);
        done(true);
        afterLogin();
        return;
      }
      const user = $("login-user").value.trim();
      const password = $("login-password").value;
      if (!user || !password) {
        showResult(error, t("login.missing"), false);
        return;
      }
      try {
        const session = await signIn(user, password);
        setToken(session.token);
        done(true);
        afterLogin();
      } catch (e) {
        showResult(error, t("login.failed", { error: e.message }), false);
        $("login-password").focus();
      }
    };
    dlg.onclose = () => done(false);
    dlg.showModal();
    $("login-user").focus();
  });
  return loginPromise;
}

// signIn posts the login and returns the session; the answer's error is
// thrown.
async function signIn(user, password) {
  const resp = await fetch("api/v1/login", { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ user, password }) });
  let data = null;
  try {
    data = await resp.json();
  } catch (e) {
    // no JSON: the status says enough
  }
  if (!resp.ok) throw new Error((data && data.error) || resp.status + " " + resp.statusText);
  return data;
}

// afterLogin reads who is in, and starts the stream and the data again.
function afterLogin() {
  loadMe().then(() => {
    refresh().then(connectEvents);
  });
}

// logout ends the session (an API key is only forgotten) and asks who
// is there again.
async function logout() {
  const me = state.me;
  if (me && me.mode === "users" && me.token && me.token.kind === "session") {
    try {
      await api("POST", "api/v1/logout");
    } catch (e) {
      // the session may be gone already
    }
  }
  setToken(null);
  state.me = null;
  closeEvents();
  showView("tiles");
  await loadMe();
  refresh().then(connectEvents);
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
    await api("PUT", "api/v1/users/" + encodeURIComponent(state.me.user.name), { password, current_password: current });
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
// created and signed in at once.
async function createOwner(ev) {
  ev.preventDefault();
  const msg = $("setup-msg");
  const name = $("setup-user").value.trim();
  const password = $("setup-password").value;
  if (!name || !password) return showResult(msg, t("login.missing"), false);
  if (password !== $("setup-again").value) return showResult(msg, t("setup.mismatch"), false);
  try {
    await api("POST", "api/v1/users", { name, password, role: "owner" });
    const session = await signIn(name, password);
    setToken(session.token);
    showResult(msg, t("setup.done", { name }), true);
    afterLogin();
  } catch (e) {
    showResult(msg, t("setup.failed", { error: e.message }), false);
  }
}

function initLogin() {
  $("logout-btn").addEventListener("click", logout);
  $("setup-form").addEventListener("submit", createOwner);
  $("me-password-form").addEventListener("submit", changeOwnPassword);
}

document.addEventListener("DOMContentLoaded", initLogin);
