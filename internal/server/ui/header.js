// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

// The Massimo web UI, the header: the plain names of the region, which
// call its parts (ui/src/header/, MassimoParts.header, AP UiParts Block
// 6), and what stays plain: the chosen Cube's radio budget, the refresh
// button with its tooltip and the server's clock, a Cube's link's sign
// and word for the plain regions, the version and the certificate's text,
// the facts that fit the header's width, and the server's state, which
// follows its version and its updates. The parts draw the Cube choice,
// every Cube's problems, the chosen Cube's age, the server's sign and the
// certificate's line, built once and updated in place, so that what is
// open in the header stays open and follows the data (docs/ui.md,
// "Header").
"use strict";

// --- header -----------------------------------------------------------

// budgetFacts is a Cube's radio budget as the page shows it, in the
// header and in its panel (ui/src/cubes/): the duty cycle, the free memory
// slots, the radiograms pending and the commands queued, "–" while unknown.
function budgetFacts(cube) {
  const b = cube.budget || {};
  return {
    duty: b.duty_cycle_pct === undefined ? "–" : b.duty_cycle_pct + " %",
    slots: b.free_memory_slots === undefined ? "–" : String(b.free_memory_slots),
    pending: String(cube.pending_radiograms || 0),
    queued: cube.queued === undefined ? "–" : String(cube.queued),
  };
}

// renderHeader shows the chosen Cube in the header: its connection in
// the Cube choice, and in expert mode its name with its radio budget,
// queued only while commands wait. Its clock is in its facts (ui/src/cubes/Facts.tsx).
function renderHeader(cube) {
  renderCubeSelect(); // the chosen Cube's connection, its plug
  const f = budgetFacts(cube);
  $("stats-cube").textContent = cube.name || cube.serial;
  $("duty").textContent = f.duty;
  $("slots").textContent = f.slots;
  $("queued").textContent = f.queued;
  $("stat-queued").classList.toggle("hidden", !(cube.queued > 0));
  $("pending-n").textContent = f.pending;
  const p = $("pending");
  const n = cube.pending_radiograms || 0;
  if (n > 0) {
    p.textContent = state.settings.expert ? tn("header.pendingExpert", n) : t("header.pending", { n });
    p.classList.remove("hidden");
  } else {
    p.classList.add("hidden");
  }
  renderUpdated();
}

// serverSkew is the server's clock less the browser's, in ms, from the
// Date header of the server's answers (noteServerClock), 0 before one
// came.
let serverSkew = 0;

// noteServerClock takes the server's clock from an answer's Date header:
// whole seconds, cut, so the middle of its second; kept unless it moved
// by more than a second and a half, so that an age does not jump back
// and forth with the cut.
function noteServerClock(resp) {
  const ms = Date.parse(resp.headers.get("Date") || "");
  if (isNaN(ms)) return;
  const skew = ms + 500 - Date.now();
  if (Math.abs(skew - serverSkew) > 1500) serverSkew = skew;
}

// renderUpdated draws the chosen Cube's age in the refresh button
// (ui/src/header/Updated.tsx), which counts on by itself; the button's
// tooltip and its name for a screen reader say the time itself, "updated
// 20:51:00, click to reload", the button's own words where the Cube has
// no such time.
function renderUpdated() {
  MassimoParts.header.render();
  const btn = $("refresh-btn");
  const cube = state.cubes.find((c) => c.serial === state.serial);
  const at = cube ? Date.parse(cube.updated || "") : NaN;
  if (isNaN(at)) {
    btn.title = t("header.refreshTitle");
    btn.setAttribute("aria-label", t("header.refreshTitle"));
    return;
  }
  const pad = (n) => String(n).padStart(2, "0");
  const d = new Date(at);
  const time = pad(d.getHours()) + ":" + pad(d.getMinutes()) + ":" + pad(d.getSeconds());
  btn.title = t("header.refreshUpdated", { time });
  btn.setAttribute("aria-label", t("header.refreshUpdated", { time }));
}

// LINK_SIGN is the state of a Cube's network sign by its link (linkState):
// connected fine, a connection held or a Cube rebooting a warning, an old
// connection, another client or none a failure.
const LINK_SIGN = { connected: "ok", held: "warn", rebooting: "warn", stale: "bad", busy: "bad", offline: "bad" };

// linkSign is the state of a Cube's network sign, for the parts and the
// plain regions alike.
function linkSign(c) {
  return LINK_SIGN[linkState(c)] || "bad";
}

// linkWord names a Cube's link in a few words, as its badge does.
function linkWord(c) {
  const link = linkState(c);
  return link === "connected" || link === "offline" ? t(c.connected ? "cubes.connected" : "cubes.disconnected") : t("link.badge." + link);
}

// cubeSignEl is a Cube's network sign, named for a screen reader by its
// link (components.js signEl), for the plain regions (backup.js); the
// parts draw it with the shared Sign.
function cubeSignEl(c) {
  const svg = signEl("net", linkSign(c), "conn-sign", "cube:" + c.serial);
  svg.removeAttribute("aria-hidden");
  svg.setAttribute("role", "img");
  svg.setAttribute("aria-label", linkWord(c));
  return svg;
}

// renderCubeSelect draws the Cube choice and every Cube's problems
// beside it (ui/src/header/Choice.tsx, CubeProblems.tsx) from the Cubes
// as the page has them: in place, an open choice or list staying open
// and following the data.
function renderCubeSelect() {
  MassimoParts.header.render();
}

// --- version ----------------------------------------------------------

// somethingOpen reports whether a reload would interrupt the user: a
// panel, a dialog, a device list, or a stepper value not yet sent.
function somethingOpen() {
  if (isShown("me") || isShown("system") || isShown("maint") || isShown("cube")) return true;
  if (!$("house-panel").classList.contains("hidden")) return true;
  if ($("login-dialog").open) return true;
  for (const ui of Object.values(state.ui)) {
    if (ui.pending !== null || ui.vacation || ui.confirm || ui.open || ui.config) return true;
  }
  return false;
}

// splitVersion separates "massimo v0.1.0-alpha.4-3-gfb15396 (go1.26.8,
// linux/amd64)" into the short version and the rest.
function splitVersion(v) {
  const m = /^(?:massimo\s+)?(\S+)\s*(.*)$/.exec(v || "");
  if (!m) return { short: v || "", rest: "" };
  return { short: m[1], rest: m[2].replace(/^\((.*)\)$/, "$1") };
}

// renderTLS takes /healthz's tls: the line under the header
// (ui/src/header/Tls.tsx), and while the certificate is self-signed the
// sign-in dialog's line asking to compare its fingerprint before signing
// in.
function renderTLS(tls) {
  $("login-tls").classList.toggle("hidden", !(tls && tls.self_signed));
  MassimoParts.header.tls(tls || null);
}

// certText is the certificate the server serves, as GET /api/v1/system
// has it, for System Info (system-info.js): its source, names and expiry.
function certText(tls) {
  if (!tls) return "–";
  const until = Date.parse(tls.not_after || "");
  const parts = [tls.source, (tls.names || []).join(", ")];
  if (!isNaN(until)) parts.push(t("header.certUntil", { date: fmtInstant(new Date(until)) }));
  return parts.filter(Boolean).join(" · ") || "–";
}

// renderVersionTitle names the server's whole version in the tooltip of
// the short one in the header.
function renderVersionTitle() {
  copyLabel($("version-short"), state.version ? t("header.versionTitle", { version: state.version }) + " · " + t("copy.hint") : ""); // components.js
  fitFacts();
}

// fitFacts puts the version, the server's sign and the time beside the
// logo where all three fit there on one line, on the logo's baseline, and
// on a row of their own across the header otherwise (.top-line.facts-row):
// under the logo and the Cube choice, on a phone under the logo and the
// pages, the version on a line of its own where that row is narrow too.
// It measures them beside the logo each time, so that it never swings;
// the compacted header is left as it is.
function fitFacts() {
  const line = document.querySelector(".top-line");
  if (!line || document.body.classList.contains("scrolled")) return;
  const brand = line.querySelector(".brand");
  const facts = line.querySelector(".top-facts");
  const version = $("version-short");
  line.classList.remove("facts-row");
  const b = brand.getBoundingClientRect();
  const f = facts.getBoundingClientRect();
  const beside = f.top < b.bottom - 1 && version.scrollWidth <= version.clientWidth + 1;
  line.classList.toggle("facts-row", !beside);
}

// watchFacts fits the facts again when the header's width or the logo's
// size changes, when what the facts show changes (the time of the last
// update, which comes after the version, and the server's sign), when the
// page's fonts have come, or when the header is whole again after
// compacting (fitFacts), after the frame, so that the layout it changes
// does not call it back within the same one.
function watchFacts() {
  const line = document.querySelector(".top-line");
  let width = -1;
  let brand = "";
  const ro = new ResizeObserver(() => {
    const w = line.getBoundingClientRect().width;
    // the logo's height too: on a phone the drawing that replaces the
    // word keeps its width and changes its height
    const r = line.querySelector(".brand").getBoundingClientRect();
    const b = r.width + "x" + r.height;
    if (w === width && b === brand) return;
    width = w;
    brand = b;
    requestAnimationFrame(fitFacts);
  });
  ro.observe(line);
  ro.observe(line.querySelector(".brand"));
  // the facts beside the logo wrapped under it once the time came, the
  // version cut; fitFacts changes no text, so it does not call itself
  let queued = false;
  new MutationObserver(() => {
    if (queued) return;
    queued = true;
    requestAnimationFrame(() => {
      queued = false;
      fitFacts();
    });
  }).observe(line.querySelector(".top-facts"), { childList: true, characterData: true, subtree: true });
  // the page's fonts change the version's width when they come
  if (document.fonts) document.fonts.ready.then(() => requestAnimationFrame(fitFacts));
  // a resize while compacted is fitted once the header is whole again
  let was = document.body.classList.contains("scrolled");
  new MutationObserver(() => {
    const now = document.body.classList.contains("scrolled");
    if (was && !now) requestAnimationFrame(fitFacts);
    was = now;
  }).observe(document.body, { attributes: true, attributeFilter: ["class"] });
}

// checkVersion compares the server's version with the one the page was
// loaded with; a newer server reloads the page when nothing is open,
// otherwise its sign offers the reload.
async function checkVersion() {
  let h;
  try {
    h = await api("GET", "healthz");
  } catch (e) {
    serverReached(false);
    return;
  }
  serverReached(true);
  updateDue();
  const v = h && h.version ? h.version : "";
  renderTLS(h && h.tls);
  if (state.version === null) {
    state.version = v;
    const parts = splitVersion(v);
    $("version-short").textContent = parts.short;
    renderVersionTitle();
    renderServerSign();
    return;
  }
  if (v === state.version) return;
  state.newVersion = true;
  renderServerSign();
  if (!somethingOpen()) location.reload();
}

// renderServerSign draws the server's sign (ui/src/header/ServerSign.tsx)
// from the server's state; while the server is gone, the page's other
// signs are greyed, their state unknown, and the time stays that of the
// last update.
function renderServerSign() {
  MassimoParts.header.render();
  document.body.classList.toggle("server-down", !!state.serverDown);
}

// serverReached notes whether the last request to the server got an
// answer, and redraws the sign where that changed.
function serverReached(ok) {
  if (state.serverDown === !ok) return;
  state.serverDown = !ok;
  renderServerSign();
}

// updateDue loads, for one who may install updates, whether one is out:
// at the first poll and every half hour after (update.js loadUpdate).
function updateDue() {
  if (!hasCap("settings")) return; // login.js
  if (state.updateAt && Date.now() - state.updateAt < 30 * 60000) return;
  state.updateAt = Date.now();
  loadUpdate(false).then(renderServerSign);
}
