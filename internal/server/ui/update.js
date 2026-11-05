// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

// The Massimo web UI, software updates: the Settings page says which
// release runs and whether a newer one is out on the update channel, and
// asks the server's update helper to install it (POST /api/v1/update).
// Once the server restarts with the new version, the version check in
// app.js offers the reload.

"use strict";

const updState = { info: null, timer: null, until: 0 };

const helperStates = {
  checking: "checking for a release",
  installing: "installing",
  restarting: "restarting the server",
  done: "installed",
  "up-to-date": "nothing to install",
  failed: "failed",
  "rolled-back": "failed and rolled back",
};

function helperText(h) {
  let t = helperStates[h.state] || h.state;
  if (h.to && h.state !== "up-to-date") t += " " + h.to;
  if (h.message) t += ": " + h.message;
  const when = h.time ? new Date(h.time) : null;
  if (when && !isNaN(when)) t += " (" + when.toLocaleString() + ")";
  return t;
}

function renderUpdate(u) {
  let t = "Running " + (u.current || "an unknown version");
  if (u.channel) t += " from channel " + u.channel;
  t += ". ";
  if (u.check_error) t += "The release check failed: " + u.check_error + ". ";
  else if (!u.channel) t += "No update channel set. ";
  else if (!u.latest) t += "No release on this channel yet. ";
  else if (u.available) t += u.latest + " is out. ";
  else t += "Up to date. ";
  if (u.available && u.blocked) t += "Updating from here is not possible: " + u.blocked + ".";
  $("update-text").textContent = t;
  const link = $("update-link");
  link.classList.toggle("hidden", !u.release_url);
  if (u.release_url) {
    link.href = u.release_url;
    link.textContent = "Release notes for " + u.latest;
  }
  $("update-helper").textContent = u.helper ? "Last update: " + helperText(u.helper) : "";
  const btn = $("update-btn");
  btn.classList.toggle("hidden", !u.can_update);
  btn.textContent = "Update to " + u.latest;
}

async function loadUpdate(refresh) {
  try {
    const u = await api("GET", "/api/v1/update" + (refresh ? "?refresh=1" : ""));
    updState.info = u;
    renderUpdate(u);
    return u;
  } catch (e) {
    $("update-text").textContent = "Cannot ask the server about updates: " + e.message;
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
    const h = u && u.helper;
    const busy = !u || (h && ["checking", "installing", "restarting"].includes(h.state));
    if (!busy || Date.now() > updState.until) {
      clearInterval(updState.timer);
      updState.timer = null;
      $("update-btn").disabled = false;
    }
  }, 3000);
}

async function startUpdate() {
  const u = updState.info;
  if (!u || !u.latest) return;
  if (!confirm("Install " + u.latest + " and restart the server? It is offline for a short while; if the new version does not come up, the old one is put back.")) return;
  const btn = $("update-btn");
  btn.disabled = true;
  try {
    await api("POST", "/api/v1/update", {});
    $("update-helper").textContent = "Update to " + u.latest + " requested …";
    followUpdate();
  } catch (e) {
    btn.disabled = false;
    $("update-helper").textContent = "Update not started: " + e.message;
  }
}

function initUpdate() {
  $("update-btn").addEventListener("click", startUpdate);
  $("update-check").addEventListener("click", () => loadUpdate(true));
  $("view-settings").addEventListener("click", () => loadUpdate(false));
}

document.addEventListener("DOMContentLoaded", initUpdate);
