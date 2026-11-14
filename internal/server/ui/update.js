// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

// The Massimo web UI, System settings → Software: the update channel a
// holder of settings chooses (the setting update.channel), which release
// runs and whether a newer one is out on that channel, the request to the
// server's update helper to install it (POST /api/v1/update), and the
// server's restart (POST /api/v1/restart), after which the page follows
// /healthz until the server is back, and the computer's reboot (POST
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

// renderRestart draws System settings → Maintenance from GET
// /api/v1/restart: the restart's button while this page may restart the
// server, and otherwise why not; the settings that changed in the
// settings file and wait for the next start, also as the card's badge;
// the settings file and when it was last read.
function renderRestart(v) {
  $("restart-btn").classList.toggle("hidden", !(v && v.available));
  showRestartText(v && !v.available && v.blocked ? t("restart.unavailable", { reason: v.blocked }) : v ? t("maint.restartText") : "");
  const pending = (v && v.pending) || [];
  const p = $("restart-pending");
  p.textContent = pending.length ? t("restart.pending", { names: pending.join(", ") }) : "";
  p.classList.toggle("hidden", !pending.length);
  sectionBadges($("sys-maint"), pending.length ? [badgeEl(tn("maint.badge.pending", pending.length), "warn")] : []);
  const file = [];
  if (v && v.file) file.push(t("maint.fileText", { file: v.file }));
  if (v && v.read_at) file.push(t("maint.lastRead", { time: fmtInstant(new Date(Date.parse(v.read_at))) }));
  $("settings-file-text").textContent = file.join(" ");
}

// reloadSettings reads the settings file again at once (POST
// /api/v1/reload), says what changed, and draws the card again.
async function reloadSettings(ev) {
  const msg = $("settings-reload-msg");
  await whileBusy(ev.currentTarget, async () => {
    try {
      const r = await api("POST", "api/v1/reload", {});
      showResult(msg, r.changed && r.changed.length ? t("maint.reloaded", { names: r.changed.join(", ") }) : t("maint.reloadedNone"), true);
    } catch (e) {
      showResult(msg, t("maint.reloadFailed", { error: e.message }), false);
    }
    await loadRestart();
  });
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

// --- the reboot of the computer (with reboot) -------------------------

// REBOOT_WAIT_MS is how long the page waits for the computer to come
// back, from the reboot's time; REBOOT_DELAY_MAX the longest delay.
const REBOOT_WAIT_MS = 5 * 60 * 1000;
const REBOOT_DELAY_MAX = 3600;

const rebootState = { following: false, timer: null, before: "" };

// showRebootText says how the reboot stands, under its title.
function showRebootText(text) {
  const p = $("reboot-text");
  p.textContent = text;
  p.classList.toggle("hidden", !text);
}

// renderReboot draws the reboot's row from GET /api/v1/reboot: the delay
// and the button while this page may reboot the computer, and otherwise
// why not; while a reboot waits, when, and the button that takes it
// back; once it has begun, the page follows the computer back.
function renderReboot(v) {
  clearTimeout(rebootState.timer);
  const waits = !!(v && v.at && !v.begun);
  $("reboot-btn").classList.toggle("hidden", !(v && v.available));
  $("reboot-delay-row").classList.toggle("hidden", !(v && v.available));
  $("reboot-cancel").classList.toggle("hidden", !waits);
  if (!v) return;
  if (v.begun) {
    followReboot(Date.parse(v.at));
    return;
  }
  if (waits) {
    const at = Date.parse(v.at);
    showRebootText(t("reboot.waiting", { time: fmtInstant(new Date(at)) }));
    // The start the page waits to see change; a page opened while the
    // reboot waits takes the server's start now.
    if (!rebootState.before) startedAt().then((s) => { rebootState.before = rebootState.before || s; });
    // setTimeout fires at once beyond 2^31 ms; the delay is an hour at most.
    rebootState.timer = setTimeout(() => followReboot(at), Math.min(Math.max(0, at - Date.now()), 2147483647));
    return;
  }
  const text = [v.available ? t("maint.rebootText") : t("reboot.unavailable", { reason: v.blocked || "" })];
  if (v.error) text.push(t("reboot.lastFailed", { error: v.error }));
  showRebootText(text.join(" "));
}

// loadReboot asks whether the computer can be rebooted from here, for a
// holder of reboot; the others see nothing of it.
async function loadReboot() {
  if (!hasCap("reboot") || rebootState.following) return; // login.js
  try {
    renderReboot(await api("GET", "api/v1/reboot"));
  } catch (e) {
    renderReboot(null);
    showRebootText(t("reboot.cannotAsk", { error: e.message }));
  }
}

// rebootDelay is the delay the field holds, in whole seconds, or null
// when it is not one the server takes.
function rebootDelay() {
  const raw = $("reboot-delay").value.trim();
  const n = Number(raw);
  return raw !== "" && Number.isInteger(n) && n >= 0 && n <= REBOOT_DELAY_MAX ? n : null;
}

// startReboot asks in place of the reboot's button whether to reboot the
// computer after the delay; Cancel puts the button back.
function startReboot() {
  const msg = $("reboot-msg");
  const delay = rebootDelay();
  if (delay === null) {
    showResult(msg, t("reboot.badDelay", { max: REBOOT_DELAY_MAX }), false);
    $("reboot-delay").focus();
    return;
  }
  showResult(msg, "", true);
  const actions = $("reboot-btn").parentElement;
  const back = () => {
    box.remove();
    actions.classList.remove("hidden");
  };
  const question = delay === 0 ? t("reboot.confirmNow") : tn("reboot.confirm", delay);
  const box = confirmBox(question, { yes: t("reboot.button"), no: t("action.cancel") }, () => {
    back();
    return runReboot(delay);
  }, () => {
    back();
    $("reboot-btn").focus();
  });
  actions.classList.add("hidden");
  actions.after(box);
  box.focusFirst();
}

// runReboot asks the server for the reboot after delay seconds, then
// draws the wait, or follows the computer at once.
async function runReboot(delay) {
  const btn = $("reboot-btn");
  btn.disabled = true;
  rebootState.before = await startedAt();
  try {
    await api("POST", "api/v1/reboot", { delay });
  } catch (e) {
    btn.disabled = false;
    showResult($("reboot-msg"), t("reboot.failed", { error: e.message }), false);
    return;
  }
  btn.disabled = false;
  await loadReboot();
}

// cancelReboot takes a reboot that waits back (DELETE /api/v1/reboot).
async function cancelReboot(ev) {
  const msg = $("reboot-msg");
  await whileBusy(ev.currentTarget, async () => {
    try {
      await api("DELETE", "api/v1/reboot");
      showResult(msg, t("reboot.cancelled"), true);
    } catch (e) {
      showResult(msg, t("reboot.cancelFailed", { error: e.message }), false);
    }
    await loadReboot();
  });
}

// followReboot follows the computer from the reboot's time: it asks
// /healthz every RESTART_POLL_MS until the server answers with another
// start than before, then reads the data and opens the stream again;
// while the old server still answers it asks whether the reboot failed,
// and after REBOOT_WAIT_MS it gives up and says so.
function followReboot(at) {
  clearTimeout(rebootState.timer);
  if (rebootState.following) return;
  rebootState.following = true;
  $("reboot-btn").classList.add("hidden");
  $("reboot-delay-row").classList.add("hidden");
  $("reboot-cancel").classList.add("hidden");
  showRebootText(t("reboot.rebooting"));
  const until = Math.max(at || 0, Date.now()) + REBOOT_WAIT_MS;
  const step = async () => {
    const now = await startedAt();
    if (!rebootState.before && now) rebootState.before = now; // a page that saw no request of its own
    if (now && now !== rebootState.before) {
      rebootState.following = false;
      showRebootText(t("reboot.back", { time: fmtInstant(new Date(Date.parse(now))) }));
      checkVersion(); // app.js
      refresh().then(connectEvents);
      return;
    }
    if (now) {
      // The old server still answers: the reboot may have failed.
      try {
        const v = await api("GET", "api/v1/reboot");
        if (!v.at) {
          rebootState.following = false;
          renderReboot(v);
          return;
        }
      } catch (e) {
        // going down
      }
    }
    if (Date.now() > until) {
      rebootState.following = false;
      showRebootText(t("reboot.notBack"));
      return;
    }
    rebootState.timer = setTimeout(step, RESTART_POLL_MS);
  };
  rebootState.timer = setTimeout(step, RESTART_POLL_MS);
}

// --- the store and the settings in effect (with settings) -------------

const infoState = { store: null, config: null, error: "" };

// renderServerInfo draws Maintenance's Store and Settings in effect, from
// GET /api/v1/store and GET /api/v1/config; a secret is "set, not shown".
function renderServerInfo() {
  const s = infoState.store;
  const c = infoState.config;
  const sbody = $("store-info-body");
  const cbody = $("config-info-body");
  clear(sbody);
  clear(cbody);
  if (infoState.error) sbody.appendChild(textEl("p", t("maint.cannotAsk", { error: infoState.error }), "small cubes-msg bad"));
  if (s) {
    const rows = Object.values(s.rows || {}).reduce((a, b) => a + b, 0);
    $("store-info-sum").textContent = t("maint.storeSum", { file: s.file.split(/[\\/]/).pop(), schema: s.schema, rows: fmtNum(rows, 0) });
    sbody.append(kvList([
      [t("maint.storeFile"), s.file, true],
      [t("maint.storeSchema"), t("maint.storeSchemaValue", { schema: s.schema, min: s.min_reader, binary: s.binary })],
      [t("maint.node"), s.node, true],
      [t("maint.tenant"), s.tenant, true],
    ]), countList(s.rows || {}));
  }
  if (c) {
    $("config-info-sum").textContent = c.file;
    cbody.appendChild(kvList([
      [t("maint.cfgVersion"), c.version || "–"],
      [t("maint.file"), c.file + " (" + t(c.file_found ? "maint.fileFound" : "maint.fileMissing") + ")", true],
      [t("maint.cfgData"), c.data_dir, true],
      [t("maint.cfgListen"), c.listen, true],
      [t("maint.cfgTLS"), c.tls],
      [t("maint.cfgLog"), c.log_level],
    ]));
    if (c.variables && c.variables.length) {
      cbody.appendChild(textEl("p", t("maint.cfgVars"), "muted small"));
      cbody.appendChild(kvList(c.variables.map((v) => [v.name,
        (v.secret ? t("maint.secretSet") : v.value) + " (" + v.source + ")" + (v.unknown ? " · " + t("maint.varUnknown") : ""), !v.secret])));
    }
  }
}

// loadServerInfo asks for the store and the settings in effect, for a
// holder of settings; the others see neither.
async function loadServerInfo() {
  if (!hasCap("settings")) return; // login.js
  infoState.error = "";
  try {
    [infoState.store, infoState.config] = await Promise.all([api("GET", "api/v1/store"), api("GET", "api/v1/config")]);
  } catch (e) {
    infoState.error = e.message;
  }
  renderServerInfo();
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

// OS_NAMES are the operating systems as Go names them, for System Info
// where the server has no distribution's name.
const OS_NAMES = { linux: "Linux", darwin: "macOS", windows: "Windows", freebsd: "FreeBSD" };

// sysClock ticks System Info's server time: the server's clock as GET
// /api/v1/system gave it, kept by its difference to this browser's.
const sysClock = { timer: null, skew: 0, zone: "" };

// knownZone reports whether this browser can show a time in zone.
function knownZone(zone) {
  try {
    new Intl.DateTimeFormat(undefined, { timeZone: zone });
    return !!zone;
  } catch (e) {
    return false;
  }
}

// clockSpan says a clock difference in seconds or minutes.
function clockSpan(ms) {
  const s = Math.round(Math.abs(ms) / 1000);
  return s < 120 ? s + " s" : Math.round(s / 60) + " min";
}

// SECONDS is a date and a time to the second, as System Info shows its
// times.
const SECONDS = { year: "numeric", month: "2-digit", day: "2-digit", hour: "2-digit", minute: "2-digit", second: "2-digit", hour12: false };

// offsetOf is the offset in minutes an ISO time carries, 0 for Z.
function offsetOf(iso) {
  const m = /([+-])(\d\d):(\d\d)$/.exec(iso || "");
  return m ? (m[1] === "-" ? -1 : 1) * (Number(m[2]) * 60 + Number(m[3])) : 0;
}

// serverTime is an instant as the server's clock reads it, to the
// second, and the server's zone: in the zone where this browser knows
// it, else at the offset iso carries.
function serverTime(ms, iso) {
  const text = knownZone(sysClock.zone)
    ? new Intl.DateTimeFormat(undefined, { ...SECONDS, timeZone: sysClock.zone }).format(new Date(ms))
    : new Intl.DateTimeFormat(undefined, { ...SECONDS, timeZone: "UTC" }).format(new Date(ms + offsetOf(iso) * 60000));
  return [text, sysClock.zone].filter(Boolean).join(" ");
}

// browserTime is the same instant in this browser's own zone, named, for
// a tooltip.
function browserTime(ms) {
  const zone = Intl.DateTimeFormat().resolvedOptions().timeZone || "";
  return t("sysinfo.browserTime", { time: new Intl.DateTimeFormat(undefined, SECONDS).format(new Date(ms)), zone });
}

// startSystemClock shows the server's time, ticking every second while
// System settings is shown (serverTime), the same in this browser's zone
// as its tooltip; with a note where the server's clock and this
// browser's differ by more than 5 s.
function startSystemClock(v) {
  const cell = $("info-now");
  if (sysClock.timer) clearInterval(sysClock.timer);
  sysClock.timer = null;
  const ms = Date.parse(v.now || "");
  if (isNaN(ms)) {
    cell.textContent = "–";
    return;
  }
  sysClock.skew = ms - Date.now();
  const note = Math.abs(sysClock.skew) > 5000 ? t(sysClock.skew > 0 ? "sysinfo.ahead" : "sysinfo.behind", { d: clockSpan(sysClock.skew) }) : "";
  const tick = () => {
    if (!isShown("system")) return;
    const now = Date.now() + sysClock.skew;
    clear(cell);
    cell.appendChild(document.createTextNode(serverTime(now, v.now)));
    if (note) cell.appendChild(textEl("span", note, "muted small sys-skew"));
    cell.title = browserTime(now);
  };
  tick();
  sysClock.timer = setInterval(tick, 1000);
}

// fillStarted shows when the server started as its clock read it
// (serverTime), then how long ago, its uptime; in this browser's zone as
// its tooltip.
function fillStarted(v) {
  const cell = $("info-started");
  clear(cell);
  cell.removeAttribute("title");
  const ms = Date.parse(v.started_at || "");
  if (isNaN(ms)) {
    cell.textContent = "–";
    return;
  }
  cell.appendChild(document.createTextNode(serverTime(ms, v.started_at)));
  const age = ageEl(v.started_at, false, true);
  if (age) cell.appendChild(age);
  cell.title = browserTime(ms);
}

// fillHostName shows the host's full name, the host bold and its domain
// as it is.
function fillHostName(v) {
  const cell = $("info-host");
  clear(cell);
  const name = v.fqdn || v.hostname || "";
  if (!name) {
    cell.textContent = "–";
    return;
  }
  const dot = name.indexOf(".");
  cell.appendChild(textEl("strong", dot < 0 ? name : name.slice(0, dot)));
  if (dot >= 0) cell.appendChild(document.createTextNode(name.slice(dot)));
}

// fillAddresses lists the interfaces the server listens on, a line each:
// the MAC (the interface's name where it has none, and as the line's
// tooltip), then its addresses, those of the web interface marked with an
// asterisk, which a note under the lines explains; where the web
// interface takes every address, the note says so instead.
function fillAddresses(v) {
  const cell = $("info-addrs");
  clear(cell);
  const ifs = v.interfaces || [];
  if (!ifs.length) {
    cell.textContent = "–";
    return;
  }
  const all = ifs.every((i) => i.addresses.every((a) => a.api));
  for (const i of ifs) {
    // text that wraps as words do: the name, the MAC, the addresses
    const line = elem("div", "sys-iface");
    const parts = [textEl("span", i.name, "mono sys-name")];
    if (i.mac) parts.push(textEl("span", i.mac, "mono sys-mac"));
    parts.push(textEl("span", i.addresses.map((a) => a.ip + (a.api && !all ? "*" : "")).join(", "), "mono"));
    parts.forEach((part, n) => {
      if (n) line.appendChild(document.createTextNode(" "));
      line.appendChild(part);
    });
    cell.appendChild(line);
  }
  const port = v.api_port || "";
  if (all) cell.appendChild(textEl("p", t("sysinfo.apiAll", { port }), "muted small sys-note"));
  else if (ifs.some((i) => i.addresses.some((a) => a.api))) cell.appendChild(textEl("p", t("sysinfo.apiMark", { port }), "muted small sys-note"));
}

// fillProxies lists the Cube proxies' listeners, a line each: the Cube's
// serial and the address, then "listening", or why not and since when
// (noticeEl), the server trying it again; the card's badge counts those
// that do not listen.
function fillProxies(v) {
  const cell = $("info-proxies");
  clear(cell);
  const list = v.cube_proxies || [];
  if (!list.length) cell.textContent = "–";
  for (const p of list) {
    const line = elem("div", "sys-proxy", textEl("span", p.serial || "", "mono"), " ", textEl("span", p.addr, "mono"));
    if (p.listening) line.append(" ", textEl("span", t("proxy.listening"), "muted"));
    else line.appendChild(noticeEl(t("proxy.notListening", { addr: p.addr, error: p.error || "" }), { since: p.since, next: p.next_try }));
    cell.appendChild(line);
  }
  const failing = list.filter((p) => !p.listening).length;
  sectionBadges($("sys-info"), failing ? [badgeEl(tn("proxy.badge.failing", failing), "warn")] : []);
}

// fillCertificate shows the certificate the browser sees: the proxy's,
// "(proxied)", where a trusted proxy serves the page (its names, expiry
// and issuer as the server last read them, or why it could not), with the
// address the proxy serves, the page's own where the proxy names no
// scheme, and the header it does not send; the page's own HTTPS while the
// server serves none, which only a proxy does, with the setting that would
// tell more; else the server's own (certText, app.js).
function fillCertificate(v) {
  const cell = $("info-tls");
  clear(cell);
  const line = (text, cls) => cell.appendChild(textEl("div", text, cls));
  const p = v.proxy;
  if (p) {
    const c = p.certificate;
    if (!p.proto) {
      // no scheme from the proxy: the page's own, which the browser knows
      const https = location.protocol === "https:";
      line(t(https ? "sysinfo.proxyHttps" : "sysinfo.proxyPlain") + " " + t("sysinfo.proxied"));
      line(location.protocol + "//" + location.host, "mono muted small");
      line(t("sysinfo.proxyNoScheme"), "muted small");
      if (v.tls) line(t("sysinfo.ownCert", { cert: certText(v.tls) }), "muted small");
      return;
    }
    let text;
    if (p.proto !== "https") text = t("sysinfo.proxyPlain");
    else if (!c) text = t("sysinfo.proxyPending");
    else if (c.error) text = t("sysinfo.proxyError", { error: c.error });
    else {
      const until = Date.parse(c.not_after || "");
      text = [(c.names || []).join(", "), isNaN(until) ? "" : t("header.certUntil", { date: fmtInstant(new Date(until)) }),
        c.issuer ? t("sysinfo.issuer", { issuer: c.issuer }) : "", c.trusted ? "" : t("sysinfo.untrusted")].filter(Boolean).join(" · ");
    }
    line(text + " " + t("sysinfo.proxied"));
    // the scheme's own port left out, as the announcement's url has it
    const own = (p.proto === "https" && p.port === 443) || (p.proto === "http" && p.port === 80) || !p.port;
    line(p.proto + "://" + (p.host.includes(":") ? "[" + p.host + "]" : p.host) + (own ? "" : ":" + p.port), "mono muted small");
    if (v.tls) line(t("sysinfo.ownCert", { cert: certText(v.tls) }), "muted small");
    return;
  }
  if (location.protocol === "https:" && !v.tls) {
    line(t("sysinfo.proxyHttps") + " " + t("sysinfo.proxied"));
    line("https://" + location.host, "mono muted small");
    line(t("sysinfo.proxyHint"), "muted small");
    return;
  }
  cell.textContent = v.tls ? certText(v.tls) : t("sysinfo.noTLS");
}

// loadSystemInfo fills System settings → System Info from GET
// /api/v1/system: the version and what it was built with, when the
// server started and how long ago, the server's clock (startSystemClock),
// the certificate the browser sees (fillCertificate), how it was
// installed, the operating system and processor, the host's full name
// (fillHostName), the interfaces it listens on (fillAddresses), the Cube
// proxies' listeners (fillProxies), and the address the server sees this
// browser at.
async function loadSystemInfo() {
  let v;
  try {
    v = await api("GET", "api/v1/system");
  } catch (e) {
    return; // the rows keep their dashes
  }
  if (!v) return;
  const parts = splitVersion(v.version || "");
  $("info-version").textContent = parts.short || "–";
  $("info-built").textContent = parts.rest || "–";
  sysClock.zone = v.time_zone || "";
  fillStarted(v);
  startSystemClock(v);
  fillCertificate(v);
  // a kind this page has no text for, a package manager's, as it is
  const kind = v.install || "other";
  const label = t("sysinfo.install." + kind);
  $("info-install").textContent = label === "sysinfo.install." + kind ? kind : label;
  const os = v.os_name || OS_NAMES[v.os] || v.os || "";
  $("info-system").textContent = [os, v.arch].filter(Boolean).join(" · ") || "–";
  fillHostName(v);
  fillAddresses(v);
  fillProxies(v);
  $("info-client").textContent = v.client_ip || "–";
}

function initUpdate() {
  $("update-btn").addEventListener("click", startUpdate);
  $("update-check").addEventListener("click", (ev) => whileBusy(ev.currentTarget, () => loadUpdate(true)));
  $("update-channel").addEventListener("change", setChannel);
  $("restart-btn").addEventListener("click", startRestart);
  $("settings-reload").addEventListener("click", reloadSettings);
  $("reboot-btn").addEventListener("click", startReboot);
  $("reboot-cancel").addEventListener("click", cancelReboot);
  onView("system", () => {
    loadUpdate(false).then(loadChannel);
    loadRestart();
    loadReboot();
    loadServerInfo();
    loadSystemInfo();
  });
}

document.addEventListener("DOMContentLoaded", initUpdate);
