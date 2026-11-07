// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

// The Massimo web UI, software updates: the Settings page says which
// release runs and whether a newer one is out on the update channel, and
// asks the server's update helper to install it (POST /api/v1/update).
// Once the server restarts with the new version, the version check in
// app.js offers the reload.

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

function initUpdate() {
  $("update-btn").addEventListener("click", startUpdate);
  $("update-check").addEventListener("click", (ev) => whileBusy(ev.currentTarget, () => loadUpdate(true)));
  $("view-settings").addEventListener("click", () => loadUpdate(false));
}

document.addEventListener("DOMContentLoaded", initUpdate);
