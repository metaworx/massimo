// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

// The Massimo web UI, Maintenance: the server's restart, the settings
// file's reload, the reboot of the computer, the card's badge, the
// announcement on the LAN and the Cube intervals.
"use strict";

// --- the restart ------------------------------------------------------

// RESTART_POLL_MS is how often the page asks /healthz while the server
// restarts, RESTART_WAIT_MS how long it waits for it to come back.
const RESTART_POLL_MS = 2000;
const RESTART_WAIT_MS = 2 * 60 * 1000;

// pending is the settings file's settings that wait for the next start.
const restartState = { following: false, pending: [] };

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
  restartState.pending = pending;
  const p = $("restart-pending");
  p.textContent = pending.length ? t("restart.pending", { names: pending.join(", ") }) : "";
  p.classList.toggle("hidden", !pending.length);
  renderMaintBadge();
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

// renderMaintBadge counts in the Maintenance card's head what waits for
// the next start: the settings file's settings and the store's (the
// announcement and the Cube intervals, AP CubePoll v1.0).
function renderMaintBadge() {
  const n = restartState.pending.length + (announceWaits(announceUI.v) ? 1 : 0) +
    INTERVALS.filter((iv) => intervalWaits(intervalsUI.v[iv.key])).length;
  sectionBadges($("sys-maint"), n ? [badgeEl(tn("maint.badge.pending", n), "warn")] : []);
}

// --- the announcement on the LAN (the setting announce) ----------------

// announceUI keeps the setting as GET /api/v1/settings answered it.
const announceUI = { v: null };

// renderAnnounce shows the choice and what it does: whether the server
// announces itself now, that a change waits for the next start, or, where
// -announce or MASSIMO_ANNOUNCE decides, the choice read-only with what
// sets it (AP AnnounceSetting, D15-A).
function renderAnnounce() {
  const v = announceUI.v;
  const sel = $("announce-mode");
  const text = $("announce-text");
  if (!v) return;
  sel.value = v.value || "auto";
  sel.disabled = !!v.fixed_by;
  const now = t(v.in_effect === "on" ? "maint.announceNowOn" : "maint.announceNowOff");
  renderMaintBadge();
  if (v.fixed_by) {
    text.textContent = t("maint.announceFixed", { by: v.fixed_by }) + " " + now;
    return;
  }
  text.textContent = t("maint.announceHint") + " " + (announceWaits(v) ? t("maint.announceWaits") : now);
}

// announceWaits reports whether the stored choice differs from this
// run's announcement: On or Off chosen, the other in effect.
function announceWaits(v) {
  if (!v || v.fixed_by) return false;
  const wants = v.value === "on" ? "on" : v.value === "off" ? "off" : "";
  return !!wants && wants !== v.in_effect;
}

// loadAnnounce reads the settings of the card, the announcement and
// the Cube intervals, for a holder of settings.
async function loadAnnounce() {
  if (!hasCap("settings")) return;
  try {
    const all = (await api("GET", "api/v1/settings")) || {};
    announceUI.v = all.announce || null;
    for (const iv of INTERVALS) intervalsUI.v[iv.key] = all[iv.key] || null;
    renderAnnounce();
    INTERVALS.forEach(renderInterval);
  } catch (e) {
    showResult($("announce-msg"), t("maint.cannotAsk", { error: e.message }), false);
  }
}

// setAnnounce stores the choice; auto is the default, sent as null.
async function setAnnounce() {
  const sel = $("announce-mode");
  const value = sel.value;
  sel.disabled = true;
  try {
    const all = (await api("PUT", "api/v1/settings", { announce: value === "auto" ? null : value })) || {};
    announceUI.v = all.announce || announceUI.v;
    showResult($("announce-msg"), t("maint.announceSaved"), true);
  } catch (e) {
    showResult($("announce-msg"), t("maint.announceFailed", { error: e.message }), false);
  }
  sel.disabled = false;
  renderAnnounce();
}

// --- the Cube intervals (poll_interval, refresh_interval, discover_interval, duty_wait)

// INTERVALS are the Cube intervals of the Maintenance card (AP CubePoll
// v1.0, D119-B): the setting, its select and its explanation, and the
// steps offered in seconds, 0 for off, within the API's range.
const INTERVALS = [
  { key: "poll_interval", id: "interval-poll", hint: "maint.pollHint", steps: [10, 15, 20, 30, 60, 120, 300, 600, 1800, 3600] },
  { key: "refresh_interval", id: "interval-refresh", hint: "maint.refreshHint", steps: [0, 1800, 3600, 7200, 10800, 21600, 43200, 86400, 172800, 604800] },
  { key: "discover_interval", id: "interval-discover", hint: "maint.discoverHint", steps: [0, 60, 300, 600, 900, 1800, 3600, 7200, 21600, 43200, 86400] },
  // How long radio commands wait at a full duty cycle before the Cube's
  // budget is read again (AP LegacyLogs v1.0, D143-A).
  { key: "duty_wait", id: "interval-duty", hint: "maint.dutyWaitHint", steps: [60, 300, 600, 900, 1800, 3600] },
];

// intervalsUI keeps the settings as GET /api/v1/settings answered them.
const intervalsUI = { v: {} };

// parseGoDuration is a duration as the API writes it (10s, 1m30s, 6h,
// 0) in seconds; NaN for anything else.
function parseGoDuration(text) {
  const s = String(text || "").trim();
  if (s === "0") return 0;
  if (!s) return NaN;
  const units = { h: 3600, m: 60, s: 1 };
  const part = /^(\d+(?:\.\d+)?)(h|m|s)/;
  let sec = 0;
  let rest = s;
  while (rest) {
    const m = part.exec(rest);
    if (!m) return NaN;
    sec += Number(m[1]) * units[m[2]];
    rest = rest.slice(m[0].length);
  }
  return Math.round(sec);
}

// apiDuration writes seconds as the API reads a duration: 10s, 30m, 6h, 0.
function apiDuration(sec) {
  if (sec === 0) return "0";
  if (sec % 3600 === 0) return sec / 3600 + "h";
  if (sec % 60 === 0) return sec / 60 + "m";
  return sec + "s";
}

// intervalLabel is seconds in the language's short units ("10 s",
// "6 h", "1 min, 30 s"), Off for 0.
function intervalLabel(sec) {
  if (isNaN(sec)) return "–";
  if (sec === 0) return t("maint.off");
  const parts = [];
  for (const [unit, n] of [["day", 86400], ["hour", 3600], ["minute", 60], ["second", 1]]) {
    if (sec >= n) {
      parts.push(fmtUnit(Math.floor(sec / n), unit)); // app.js
      sec %= n;
    }
  }
  return parts.join(", ");
}

// intervalWaits reports whether a stored interval differs from this
// run's, so that it waits for the next start.
function intervalWaits(v) {
  return !!v && !v.fixed_by && parseGoDuration(v.value) !== parseGoDuration(v.in_effect);
}

// renderInterval shows one interval: its steps, a stored value that is
// none of them added; the line saying what it does and this run's
// value, or that a change waits for the restart; read-only with what
// sets it where a flag decides (as the announcement, D15-A).
function renderInterval(iv) {
  const v = intervalsUI.v[iv.key];
  const sel = $(iv.id);
  const text = $(iv.id + "-text");
  if (!v) return;
  const value = parseGoDuration(v.value);
  const steps = isNaN(value) || iv.steps.includes(value) ? iv.steps : [...iv.steps, value].sort((a, b) => a - b);
  sel.replaceChildren(...steps.map((sec) => {
    const o = document.createElement("option");
    o.value = String(sec);
    o.textContent = intervalLabel(sec);
    return o;
  }));
  sel.value = String(value);
  sel.disabled = !!v.fixed_by;
  const now = intervalLabel(parseGoDuration(v.in_effect));
  const state = v.fixed_by ? t("maint.intervalFixed", { by: v.fixed_by, value: now })
    : intervalWaits(v) ? t("maint.intervalWaits", { value: now })
      : t("maint.intervalNow", { value: now });
  // a unit that ends in a full stop ("10 Sek.") ends the sentence too
  text.textContent = t(iv.hint) + " " + state.replace(/\.\.$/, ".");
  renderMaintBadge();
}

// saveInterval stores the chosen interval; the default is sent as null,
// so that the store keeps no copy of it.
async function saveInterval(iv) {
  const sel = $(iv.id);
  const sec = Number(sel.value);
  const v = intervalsUI.v[iv.key];
  const def = v ? parseGoDuration(v.default) : NaN;
  sel.disabled = true;
  try {
    const all = (await api("PUT", "api/v1/settings", { [iv.key]: sec === def ? null : apiDuration(sec) })) || {};
    intervalsUI.v[iv.key] = all[iv.key] || v;
    showResult($(iv.id + "-msg"), t("maint.intervalSaved"), true);
  } catch (e) {
    showResult($(iv.id + "-msg"), t("maint.intervalFailed", { error: e.message }), false);
  }
  sel.disabled = false;
  renderInterval(iv);
}

// initMaintenance wires the Maintenance card: the restart, the settings
// file's reload, the reboot, the announcement on the LAN and the Cube
// intervals, and their loads each time Maintenance is shown.
function initMaintenance() {
  $("restart-btn").addEventListener("click", startRestart);
  $("settings-reload").addEventListener("click", reloadSettings);
  $("reboot-btn").addEventListener("click", startReboot);
  $("reboot-cancel").addEventListener("click", cancelReboot);
  $("announce-mode").addEventListener("change", setAnnounce);
  for (const iv of INTERVALS) $(iv.id).addEventListener("change", () => saveInterval(iv));
  onView("maint", () => {
    showResult($("announce-msg"), "");
    for (const iv of INTERVALS) showResult($(iv.id + "-msg"), "");
    loadAnnounce();
    loadRestart();
    loadReboot();
  });
}

document.addEventListener("DOMContentLoaded", initMaintenance);
