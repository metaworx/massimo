// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

// The Massimo web UI, System settings → Software: which release runs and
// whether a newer one is out on the update channel, the request to the
// server's update helper to install it (POST /api/v1/update), and the
// server's restart (POST /api/v1/restart), after which the page follows
// /healthz until the server is back. Once the server restarts with a new
// version, the version check in app.js offers the reload.

"use strict";

const updState = { info: null, timer: null, until: 0, grace: 0 };

// updateBusy reports whether the helper is working on an update.
function updateBusy(h) {
  return !!h && ["checking", "installing", "restarting"].includes(h.state);
}

// setUpdateBusy turns the update button and greys out Check while an
// update runs: a check or a second request in the middle of an install
// would only confuse, and the server goes away for the restart anyway.
function setUpdateBusy(busy) {
  setBusy($("update-btn"), busy);
  setOff($("update-check"), busy);
}

// helperText is the helper's last state; unknown states are shown as
// the helper reports them.
function helperText(h) {
  const key = "update.state." + h.state;
  let s = t(key);
  if (s === key) s = h.state;
  if (h.to && h.state !== "up-to-date") s += " " + h.to;
  if (h.message) s += ": " + h.message;
  const when = h.time ? new Date(h.time) : null;
  if (when && !isNaN(when)) s += " (" + when.toLocaleString(i18n.lang) + ")";
  return s;
}

function renderUpdate(u) {
  const version = u.current || t("update.unknownVersion");
  const parts = [u.channel ? t("update.runningChannel", { version, channel: u.channel }) : t("update.running", { version })];
  if (u.check_error) parts.push(t("update.checkFailed", { error: u.check_error }));
  else if (!u.channel) parts.push(t("update.noChannel"));
  else if (!u.latest) parts.push(t("update.noRelease"));
  else if (u.available) parts.push(t("update.available", { version: u.latest }));
  else parts.push(t("update.upToDate"));
  if (u.available && u.blocked) parts.push(t("update.blocked", { reason: u.blocked }));
  $("update-text").textContent = parts.join(" ");
  const link = $("update-link");
  link.classList.toggle("hidden", !u.release_url);
  if (u.release_url) {
    link.href = u.release_url;
    link.textContent = t("update.notes", { version: u.latest });
  }
  $("update-helper").textContent = u.helper ? t("update.last", { state: helperText(u.helper) }) : "";
  const btn = $("update-btn");
  btn.classList.toggle("hidden", !u.can_update);
  btn.textContent = t("update.to", { version: u.latest });
  // The helper may be at work already: after a reload while it restarts
  // the server, or for an update started from another browser.
  if (updateBusy(u.helper) && !updState.timer) {
    setUpdateBusy(true);
    followUpdate();
  }
}

async function loadUpdate(refresh) {
  try {
    const u = await api("GET", "api/v1/update" + (refresh ? "?refresh=1" : ""));
    updState.info = u;
    renderUpdate(u);
    return u;
  } catch (e) {
    $("update-text").textContent = t("update.cannotAsk", { error: e.message });
    return null;
  }
}

// followUpdate polls while the helper works; the server goes away for
// the restart, which the polling rides out.
function followUpdate() {
  if (updState.timer) clearInterval(updState.timer);
  updState.until = Date.now() + 5 * 60 * 1000;
  updState.timer = setInterval(async () => {
    const u = await loadUpdate(false);
    // Right after a request the helper may not have picked it up yet, so
    // its last state still says done; the grace period covers that gap.
    const busy = !u || updateBusy(u.helper) || Date.now() < updState.grace;
    if (!busy || Date.now() > updState.until) {
      clearInterval(updState.timer);
      updState.timer = null;
      setUpdateBusy(false);
    }
  }, 3000);
}

// startUpdate asks in the Software section, in place of its buttons,
// whether to install the update; Cancel puts the buttons back.
function startUpdate() {
  const u = updState.info;
  if (!u || !u.latest) return;
  const actions = $("update-btn").parentElement;
  const back = () => {
    box.remove();
    actions.classList.remove("hidden");
  };
  const box = confirmBox(t("update.confirm", { version: u.latest }), { yes: t("update.to", { version: u.latest }), no: t("action.cancel") }, () => {
    back();
    runUpdate(u);
  }, () => {
    back();
    $("update-btn").focus();
  });
  actions.classList.add("hidden");
  actions.after(box);
  box.focusFirst();
}

// runUpdate asks the server's update helper to install u.latest.
async function runUpdate(u) {
  setUpdateBusy(true);
  try {
    await api("POST", "api/v1/update", {});
    $("update-helper").textContent = t("update.requested", { version: u.latest });
    updState.grace = Date.now() + 30 * 1000;
    followUpdate();
  } catch (e) {
    setUpdateBusy(false);
    $("update-helper").textContent = t("update.notStarted", { error: e.message });
  }
}

// --- the restart ------------------------------------------------------

// RESTART_POLL_MS is how often the page asks /healthz while the server
// restarts, RESTART_WAIT_MS how long it waits for it to come back.
const RESTART_POLL_MS = 2000;
const RESTART_WAIT_MS = 2 * 60 * 1000;

const restartState = { following: false };

// showRestartText says how the restart stands, under its button.
function showRestartText(text) {
  const p = $("restart-text");
  p.textContent = text;
  p.classList.toggle("hidden", !text);
}

// renderRestart shows the button while GET /api/v1/restart says this
// page may restart the server, and otherwise why not; and the settings
// that changed in the settings file and wait for the next start.
function renderRestart(v) {
  $("restart-btn").classList.toggle("hidden", !(v && v.available));
  const parts = [];
  if (v && v.pending && v.pending.length) parts.push(t("restart.pending", { names: v.pending.join(", ") }));
  if (v && !v.available && v.blocked) parts.push(t("restart.unavailable", { reason: v.blocked }));
  showRestartText(parts.join(" "));
}

// loadRestart asks whether the server can be restarted from here, for a
// holder of restart; the others see nothing of it.
async function loadRestart() {
  if (!hasCap("restart") || restartState.following) return; // login.js
  try {
    renderRestart(await api("GET", "api/v1/restart"));
  } catch (e) {
    $("restart-btn").classList.add("hidden");
    showRestartText(t("restart.cannotAsk", { error: e.message }));
  }
}

// startedAt is the server's start from /healthz, "" when it does not
// answer; /healthz is always open, so it needs no sign-in.
async function startedAt() {
  try {
    const h = await apiRequest("GET", "healthz", undefined, { auth: false });
    return (h && h.started_at) || "";
  } catch (e) {
    return "";
  }
}

// startRestart asks in place of the restart's button whether to restart
// the server; Cancel puts the button back.
function startRestart() {
  const actions = $("restart-btn").parentElement;
  const back = () => {
    box.remove();
    actions.classList.remove("hidden");
  };
  const box = confirmBox(t("restart.confirm"), { yes: t("restart.button"), no: t("action.cancel") }, () => {
    back();
    return runRestart();
  }, () => {
    back();
    $("restart-btn").focus();
  });
  actions.classList.add("hidden");
  actions.after(box);
  box.focusFirst();
}

// runRestart asks the server to restart and follows it until /healthz
// reports a new start.
async function runRestart() {
  const btn = $("restart-btn");
  btn.disabled = true;
  const before = await startedAt();
  try {
    await api("POST", "api/v1/restart", {});
  } catch (e) {
    btn.disabled = false;
    showRestartText(t("restart.failed", { error: e.message }));
    return;
  }
  showRestartText(t("restart.restarting"));
  restartState.following = true;
  followRestart(before, Date.now() + RESTART_WAIT_MS);
}

// followRestart asks /healthz every RESTART_POLL_MS until the server
// answers with another start than before, then reads the data and opens
// the stream again; after until it gives up and says so.
function followRestart(before, until) {
  setTimeout(async () => {
    const now = await startedAt();
    if (now && now !== before) {
      restartState.following = false;
      $("restart-btn").disabled = false;
      showRestartText(t("restart.back", { time: fmtInstant(new Date(Date.parse(now))) }));
      checkVersion(); // app.js: the start in the settings, and a reload for a new version
      refresh().then(connectEvents);
      return;
    }
    if (Date.now() > until) {
      restartState.following = false;
      $("restart-btn").disabled = false;
      showRestartText(t("restart.notBack"));
      return;
    }
    followRestart(before, until);
  }, RESTART_POLL_MS);
}

function initUpdate() {
  $("update-btn").addEventListener("click", startUpdate);
  $("update-check").addEventListener("click", (ev) => whileBusy(ev.currentTarget, () => loadUpdate(true)));
  $("restart-btn").addEventListener("click", startRestart);
  onView("system", () => {
    loadUpdate(false);
    loadRestart();
  });
}

document.addEventListener("DOMContentLoaded", initUpdate);
