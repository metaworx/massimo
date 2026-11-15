// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

// The Massimo web UI, System settings → Software: the update channel a
// holder of settings chooses (the setting update.channel), which release
// runs and whether a newer one is out on that channel, the request to the
// server's update helper to install it (POST /api/v1/update), the last
// three updates (GET /api/v1/update's history), and the server's
// restart (POST /api/v1/restart), after which the page follows /healthz
// until the server is back, and the computer's reboot (POST
// /api/v1/reboot), which waits its delay, can be taken back meanwhile,
// and is followed the same way. Once the server restarts with a new
// version, the version check in app.js offers the reload.

"use strict";

const updState = { info: null, setting: null, timer: null, until: 0, grace: 0 };

// UPDATE_FLAVOURS are the channels a box may follow, the most rolling
// first, as update.Flavours has them: each takes its own releases and
// those of every more stable one.
const UPDATE_FLAVOURS = ["devel", "nightly", "alpha", "beta", "rc", "stable"];

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
  let s = stateText(h.state);
  if (h.to && h.state !== "up-to-date") s += " " + h.to;
  if (h.message) s += ": " + h.message;
  const when = h.time ? new Date(h.time) : null;
  if (when && !isNaN(when)) s += " (" + when.toLocaleString(i18n.lang) + ")";
  return s;
}

// stateText is a helper state's text, as the helper reports it where
// the page has none.
function stateText(state) {
  const key = "update.state." + state;
  const s = t(key);
  return s === key ? state : s;
}

// historyLine is one of the last updates: the version reached, or the one
// asked for with how far it got (under way for the newest, else not
// reached, or the helper's outcome), when, and by whom or from outside
// Massimo with the package manager.
function historyLine(e, newest) {
  const asked = e.to || "";
  let what = e.reached;
  if (!what && e.result) what = t("update.history.ended", { version: asked, state: stateText(e.result) });
  else if (!what) what = t(newest ? "update.history.running" : "update.history.notReached", { version: asked });
  const parts = [what];
  const ms = Date.parse(e.reached_at || e.at || "");
  if (!isNaN(ms)) parts.push(fmtInstant(new Date(ms))); // app.js
  if (e.outside) parts.push(e.manager ? t("update.history.outsideWith", { manager: e.manager }) : t("update.history.outside"));
  else if (e.by) parts.push(t("update.history.by", { name: e.by.display_name || e.by.name }));
  return textEl("li", parts.join(" · "));
}

// renderHistory shows the last three updates under the version lines,
// nothing before the first.
function renderHistory(list) {
  const ul = $("update-history-list");
  clear(ul);
  list.slice(0, 3).forEach((e, i) => ul.appendChild(historyLine(e, i === 0)));
  $("update-history").classList.toggle("hidden", !list.length);
}

function renderUpdate(u) {
  const version = u.current || t("update.unknownVersion");
  const parts = [u.channel ? t("update.runningChannel", { version, channel: u.channel }) : t("update.running", { version })];
  // a package manager owns the binary: the Debian package's helper
  // installs through apt
  if (u.manager) parts.push(t("update.manager", { manager: u.manager }));
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
  renderHistory(u.history || []);
  // the card's badge, also while it is folded: up to date, or what is out
  const known = !u.check_error && u.channel && u.latest;
  sectionBadges($("sys-software"), known ? [u.available ? badgeEl(t("update.badge.available", { version: u.latest }), "warn") : badgeEl(t("update.badge.upToDate"), "ok")] : []);
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

// updateOut tells whether an update is out, for one who may install it
// (the server's sign, app.js renderServerSign).
function updateOut() {
  const u = updState.info;
  return !!u && !!u.available && hasCap("settings"); // login.js
}

// updateLatest is the version an update would install, "" for none.
function updateLatest() {
  return (updState.info && updState.info.latest) || "";
}

async function loadUpdate(refresh) {
  try {
    const u = await api("GET", "api/v1/update" + (refresh ? "?refresh=1" : ""));
    updState.info = u;
    renderUpdate(u);
    renderServerSign(); // app.js
    return u;
  } catch (e) {
    $("update-text").textContent = t("update.cannotAsk", { error: e.message });
    return null;
  }
}

// channelFixed says why the channel cannot be chosen here, "" where it
// can: brew's formula decides it, or MASSIMO_UPDATE_CHANNEL overrides the
// setting, so that the channel in effect is neither the setting's nor,
// without one, apt's suite or the build's flavour.
function channelFixed(u, setting) {
  if (u.manager === "brew") {
    return t("update.channelBrew", { channel: u.channel, formula: u.channel === "stable" ? "massimo" : "massimo-" + u.channel });
  }
  const chosen = setting.set ? setting.value : u.suite || u.flavour;
  if (setting.in_effect !== chosen) return t("update.channelVariable", { channel: setting.in_effect });
  return "";
}

// channelNote is what the choice does where it can be made: on a box apt
// updates, the update helper switches apt's suite at the next update;
// elsewhere the hint on the channels' order.
function channelNote(u) {
  if (u.manager !== "apt" || !u.suite) return t("update.channelHint");
  if (u.channel !== u.suite) return t("update.channelAptSwitch", { suite: u.suite, channel: u.channel });
  return t("update.channelApt", { channel: u.suite });
}

// renderChannel shows the channel's choice to a holder of settings, who
// alone may change it: the build's flavour or a channel of its own, or
// the channel fixed, with the reason and the way to change it.
function renderChannel() {
  const u = updState.info;
  const setting = updState.setting;
  const show = !!u && !!setting && hasCap("settings"); // login.js
  $("update-channel-field").classList.toggle("hidden", !show);
  if (!show) return;
  const sel = $("update-channel");
  // Without a choice of its own the box follows apt's suite, or else its
  // build's flavour.
  let none = u.flavour ? t("update.channelFlavour", { flavour: u.flavour }) : t("update.channelNone");
  if (u.manager === "apt" && u.suite) none = t("update.channelSuite", { suite: u.suite });
  const own = [["", none]];
  sel.replaceChildren(...own.concat(UPDATE_FLAVOURS.map((f) => [f, t("update.flavour." + f)])).map(([value, label]) => {
    const o = document.createElement("option");
    o.value = value;
    o.textContent = label;
    return o;
  }));
  const fixed = channelFixed(u, setting);
  sel.disabled = !!fixed;
  sel.value = fixed ? setting.in_effect : setting.set ? setting.value : "";
  $("update-channel-note").textContent = fixed || channelNote(u);
}

// loadChannel reads the setting update.channel, for a holder of settings.
async function loadChannel() {
  if (!hasCap("settings")) return; // login.js
  try {
    const all = (await api("GET", "api/v1/settings")) || {};
    updState.setting = all["update.channel"] || null;
  } catch (e) {
    updState.setting = null;
  }
  renderChannel();
}

// setChannel stores the chosen channel, none for the build's flavour, and
// checks again for a release on it.
async function setChannel() {
  const sel = $("update-channel");
  let failed = null;
  sel.disabled = true; // renderChannel decides again
  try {
    await api("PUT", "api/v1/settings", { "update.channel": sel.value || null });
  } catch (e) {
    failed = e;
  }
  await loadUpdate(true);
  await loadChannel();
  if (failed) $("update-channel-note").textContent = t("update.channelFailed", { error: failed.message });
  sel.focus();
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
  // apt puts nothing back: the helper says how to go back instead
  const question = u.manager ? t("update.confirmManaged", { version: u.latest, manager: u.manager }) : t("update.confirm", { version: u.latest });
  const box = confirmBox(question, { yes: t("update.to", { version: u.latest }), no: t("action.cancel") }, () => {
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

// initUpdate wires the Software update card: Install, Check now and the
// channel, and its load each time Maintenance is shown.
function initUpdate() {
  $("update-btn").addEventListener("click", startUpdate);
  $("update-check").addEventListener("click", (ev) => whileBusy(ev.currentTarget, () => loadUpdate(true)));
  $("update-channel").addEventListener("change", setChannel);
  onView("maint", () => {
    loadUpdate(false).then(loadChannel);
  });
}

document.addEventListener("DOMContentLoaded", initUpdate);
