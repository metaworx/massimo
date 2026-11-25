// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

// The Massimo web UI, the header: the plain names of the region, which
// call its parts (ui/src/header/, MassimoParts.header, AP UiParts Block
// 6), and what stays plain: the chosen Cube's radio budget, the server's
// clock and the time of its last answer, a Cube's link's sign and word for
// the plain regions, the version in ☰ (and in the sign-in card) and the
// certificate's text, and the server's state, which follows
// its version and its updates. The parts draw the pile (the chosen Cube's
// age, the server's sign, every Cube's problems, Refresh now), the Cube
// choice and the certificate's line, built once and updated in place,
// so that what is open in the header stays open and follows the data
// (docs/ui.md, "Header").
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
  MassimoParts.header.render(); // the chosen Cube's connection, its plug
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
  MassimoParts.header.render(); // how long ago the Cube was read, from now (ui/src/header/Times.tsx)
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


// --- version ----------------------------------------------------------

// somethingOpen reports whether a reload would interrupt the user: a
// panel, a dialog, a device list, a stepper value not yet sent, or a
// settings form with changes not saved, for which the browser would ask.
function somethingOpen() {
  if (MassimoParts.leaving.changes()) return true; // ui/src/leaving/
  if (["me", "system", "maint", "cube"].some((v) => MassimoParts.pages.isShown(v))) return true;
  if (MassimoParts.rooms.houseOpen()) return true;
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
// in. In the app (appBridge, app.js) neither asks for the comparison:
// the app pinned the fingerprint the user compared when the server was
// added, and refuses a certificate with another; the warnings of a
// certificate near its end stay (docs/app.md).
function renderTLS(tls) {
  const pinned = !!appBridge();
  MassimoParts.signIn.selfSigned(!pinned && !!(tls && tls.self_signed)); // the card's line (ui/src/signin/)
  MassimoParts.header.tls(tls || null, pinned);
}

// certText is the certificate the server serves, as GET /api/v1/system
// has it, for System Info (ui/src/sysinfo/): its source, names and expiry.
function certText(tls) {
  if (!tls) return "–";
  const until = Date.parse(tls.not_after || "");
  const parts = [tls.source, (tls.names || []).join(", ")];
  if (!isNaN(until)) parts.push(t("header.certUntil", { date: fmtInstant(new Date(until)) }));
  return parts.filter(Boolean).join(" · ") || "–";
}

// checkVersion compares the server's version with the one the page was
// loaded with; a newer server reloads the page when nothing is open,
// otherwise the pile's list offers the reload. Its answer also says,
// before anyone signs in, whether the password fields offer their eye.
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
  // the password fields' eye, as the server's setting auth.password_reveal says, read at each look (ui/src/shared/password.tsx)
  MassimoParts.chrome.reveal(!(h && h.password_reveal === false));
  if (state.version === null) {
    state.version = v;
    MassimoParts.chrome.version(v); // ☰'s first row, and signed out the sign-in card's foot and the first row of its ☰ (ui/src/shared/server.ts)
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

// updateDue loads, for one who may install updates or choose their
// channel, whether one is out, and the automatic update's question: at the
// first poll and every half hour after (ui/src/software/).
function updateDue() {
  if (!hasCap("settings") && !hasCap("update")) return; // login.js
  if (state.updateAt && Date.now() - state.updateAt < 30 * 60000) return;
  state.updateAt = Date.now();
  MassimoParts.software.load(false).then(renderServerSign);
}
