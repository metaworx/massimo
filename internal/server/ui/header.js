// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

// The Massimo web UI, the header: the chosen Cube's radio budget, the
// time of the last update with the server's clock, the Cube choice with
// each Cube's network sign and the problems list, the version with the
// TLS notice, the facts that fit the header's width, and the server's
// sign, which follows the server's version and its updates.
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

// fmtSince is a time span (ms), briefly: seconds under a minute, then
// minutes, hours, from two days on days ("8 sec", "12 min", "5 hr").
function fmtSince(ms) {
  const sec = Math.max(0, Math.floor(ms / 1000));
  if (sec < 60) return fmtUnit(sec, "second");
  if (sec < 3600) return fmtUnit(Math.floor(sec / 60), "minute");
  if (sec < 48 * 3600) return fmtUnit(Math.floor(sec / 3600), "hour");
  return fmtUnit(Math.floor(sec / 86400), "day");
}

let updatedTimer = null;

// renderUpdated shows in the refresh button how long ago the chosen Cube
// was last heard from: its updated, the last message the server took
// from it, on the server's clock (serverSkew), the age counting on by
// itself (each second under a minute, then each minute, each hour); its
// tooltip and its name for a screen reader say the time itself, "updated
// 20:51:00, click to reload". Nothing where the Cube has no such time.
function renderUpdated() {
  const el = $("updated");
  const btn = $("refresh-btn");
  clearTimeout(updatedTimer);
  const cube = state.cubes.find((c) => c.serial === state.serial);
  const at = cube ? Date.parse(cube.updated || "") : NaN;
  if (isNaN(at)) {
    el.textContent = "";
    btn.title = t("header.refreshTitle");
    btn.setAttribute("aria-label", t("header.refreshTitle"));
    return;
  }
  const age = Math.max(0, Date.now() + serverSkew - at);
  el.textContent = fmtSince(age);
  const pad = (n) => String(n).padStart(2, "0");
  const d = new Date(at);
  const time = pad(d.getHours()) + ":" + pad(d.getMinutes()) + ":" + pad(d.getSeconds());
  btn.title = t("header.refreshUpdated", { time });
  btn.setAttribute("aria-label", t("header.refreshUpdated", { time }));
  const step = age < 60000 ? 1000 : age < 3600000 ? 60000 : 3600000;
  updatedTimer = setTimeout(renderUpdated, step - (age % step) + 20);
}

// LINK_SIGN is the state of a Cube's network sign by its link (linkState):
// connected fine, a connection held or a Cube rebooting a warning, an old
// connection, another client or none a failure.
const LINK_SIGN = { connected: "ok", held: "warn", rebooting: "warn", stale: "bad", busy: "bad", offline: "bad" };

// linkSign is the state of a Cube's network sign.
function linkSign(c) {
  return LINK_SIGN[linkState(c)] || "bad";
}

// linkWord names a Cube's link in a few words, as its badge does.
function linkWord(c) {
  const link = linkState(c);
  return link === "connected" || link === "offline" ? t(c.connected ? "cubes.connected" : "cubes.disconnected") : t("link.badge." + link);
}

// cubeSignEl is a Cube's network sign, named for a screen reader by its
// link (components.js signEl).
function cubeSignEl(c) {
  const svg = signEl("net", linkSign(c), "conn-sign", "cube:" + c.serial);
  svg.removeAttribute("aria-hidden");
  svg.setAttribute("role", "img");
  svg.setAttribute("aria-label", linkWord(c));
  return svg;
}

// renderCubeSelect fills the Cube choice (components.js pickerMenu): the
// configured Cubes, each with its network sign (cubeSignEl), its name
// (its serial without one) with the icons of its problems after it, the
// settings' cog for a role with settings, and under it its serial and
// address; then
// those found on the LAN but not configured, shown, not selectable; then
// the entry that searches the LAN for new ones. Closed it shows the
// chosen Cube's sign and name, its tooltip the serial, the address and
// the link. An open choice is left as it is, so that a poll does not
// take the focus from it; a pick closes it first. Beside it, every
// Cube's problems (renderCubeProblems).
function renderCubeSelect() {
  renderCubeProblems();
  const menu = $("cube-menu");
  if (menu.open) return;
  const items = state.cubes.map((c) => ({
    value: c.serial,
    label: c.name || c.serial,
    sub: [c.name ? c.serial : "", c.address || ""].filter(Boolean).join(" · "),
    icon: cubeSignEl(c),
    // its problems after its name, its link apart: its sign is that
    icons: kindIcons([cubeEntry(c, hasCap("settings"))], ["net"]), // components.js
  }));
  // a found Cube opens System settings → Cubes, where it can be added,
  // for a role with cubes; for others it is information
  if (state.found.length) {
    items.push({ group: t("discover.group") });
    for (const c of state.found) items.push({ value: "found:" + c.serial, label: c.serial, sub: c.address || "", disabled: !hasCap("cubes") });
  }
  if (hasCap("discover")) {
    items.push({ separator: true });
    items.push({ value: DISCOVER, label: t(discovery.running ? "discover.searching" : "discover.search"), disabled: discovery.running });
  }
  const cube = state.cubes.find((c) => c.serial === state.serial);
  const name = cube ? cube.name || cube.serial : t("header.cube");
  const conn = cube ? linkWord(cube) : "";
  pickerMenu(menu, {
    items,
    value: state.serial || "",
    onPick: (v) => {
      menu.open = false;
      if (v === DISCOVER) discoverCubes();
      else if (v.startsWith("found:")) showFound();
      else selectCube(v);
    },
    shown: {
      icon: cube ? cubeSignEl(cube) : null,
      label: name,
      title: cube ? [name, cube.name ? cube.serial : "", cube.address || "", conn].filter(Boolean).join(" · ") : name,
      aria: cube ? t("header.cubeChoice", { name, state: conn }) : name,
    },
  });
}

// cubeProblemEntries are every Cube's problems as problemsEl's entries:
// a Cube with a problem as the heading, its network sign its own state
// (green where only its devices fail), its link's badge where it is not
// connected, and its devices with a problem under it, "Room › Device",
// with their badges (tiles.js deviceProblems); a window open briefly is a
// state, not a problem. Then, for one who may change the settings, the
// settings: a Cube proxy that does not listen, a settings file that
// differs from the store, a Cube found elsewhere than its fixed address.
// Last, for everyone, what was found on the network and is not served
// (foundEntry), as information.
function cubeProblemEntries() {
  const entries = state.cubes.map((c) => cubeEntry(c));
  if (hasCap("settings")) {
    const items = [];
    for (const c of state.cubes) items.push(...cubeSettingItems(c, c.name || c.serial));
    entries.push({ sign: { kind: "cog", state: items.length ? "warn" : "" }, title: t("problem.settings"), items });
  }
  if (state.found.length) entries.push(foundEntry(state.found));
  return entries;
}

// foundEntry is what was found on the network and is not served, as the
// problem list's last group: the found Cubes' sign with their number,
// shown in the summary too, "not set up" and, for a role with cubes, a
// link to System settings → Cubes (showFound); under it each Cube and
// Massimo proxy, its address, what it is and the server serving it
// (cubes.js foundWhat).
function foundEntry(found) {
  const sub = [t("found.notSetUp")];
  if (hasCap("cubes")) {
    const go = textEl("button", t("found.toCubes"), "link-button found-to-cubes");
    go.type = "button";
    go.addEventListener("click", showFound);
    sub.push(" · ", go);
  }
  const subs = found.map((c) => ({
    icon: c.kind === "proxy" ? "icon-proxy" : "icon-cube3d",
    title: c.serial,
    sub: joinParts([c.address, ...foundWhat(c)]), // cubes.js
  }));
  return { node: foundCountEl(found.length), title: t("found.group"), sub, subs, info: true, summary: foundCountEl(found.length) };
}

// cubeEntry is a Cube's problems as an entry of problemsEl: the Cube as
// the heading with its network sign, its link's badge where it is not
// connected, own its settings' warnings too (a Cube's panel, ui/src/cubes/Panel.tsx),
// and its devices with a problem under it, "Room › Device", with their
// badges (tiles.js deviceProblems); a window open briefly is a state.
function cubeEntry(c, own) {
  const st = linkSign(c);
  const items = st === "ok" ? [] : [{ kind: "net", icon: "icon-net", sign: "net", tone: st === "bad" ? "bad" : "warn", text: linkWord(c) }];
  if (own) items.push(...cubeSettingItems(c, ""));
  const subs = (c.problems || []).map((d) => ({
    icon: deviceIcon(d), // tiles.js
    title: [d.room, deviceName(d)].filter(Boolean).join(" › "),
    sub: d.serial || d.rf_address,
    items: deviceProblems(d, true).filter((it) => it.tone !== "fill-open"), // tiles.js
  }));
  return { sign: { kind: "net", state: st }, title: c.name || c.serial, sub: c.name ? c.serial : "", items, subs };
}

// cubeSettingItems are a Cube's settings that need a look, as items: a
// proxy listener that does not listen, a settings file that differs, a
// fixed address the Cube is found elsewhere than (found_elsewhere); who
// names the Cube in the text, "" for its own panel.
function cubeSettingItems(c, who) {
  const items = [];
  const text = (key, args) => (who ? t("problem." + key, { cube: who, ...args }) : t("problem." + key + "Own", args));
  for (const f of c.proxy_failing || []) items.push({ kind: "cog", icon: "icon-cog", sign: "cog", tone: "warn", text: text("proxy", { addr: f.addr }) });
  if ((c.file_differs || []).length) items.push({ kind: "cog", icon: "icon-cog", sign: "cog", tone: "warn", text: text("file", {}) });
  if (c.found_elsewhere) items.push({ kind: "cog", icon: "icon-cog", sign: "cog", tone: "warn", text: text("address", { addr: c.found_elsewhere.address }) });
  return items;
}

// problemsLabelOf names the kinds of problems among entries, for a
// screen reader and the tooltip.
function problemsLabelOf(entries) {
  const kinds = ["net", "radio", "battery", "window", "cog"].filter((k) => entries.flatMap(entryItems).some((it) => it.kind === k)); // components.js
  const found = entries.some((e) => e.info) && state.found.length ? tn("found.pill", state.found.length) : "";
  if (!kinds.length) return [t("problem.none"), found].filter(Boolean).join(" · ");
  return [t("problem.label", { list: kinds.map((k) => t("problem." + k)).join(", ") }), found].filter(Boolean).join(" · ");
}

// renderCubeProblems draws every Cube's problems beside the Cube choice
// (problemsEl), a green check where none has one (okPill), nothing before
// there is a Cube; its list kept open across a poll where it was open.
function renderCubeProblems() {
  const box = $("cube-problems");
  if (!box) return;
  const was = box.querySelector("details.problems");
  if (was && was.open) return;
  const entries = cubeProblemEntries();
  const p = problemsEl(entries, problemsLabelOf(entries));
  if (p) box.replaceChildren(p);
  else box.replaceChildren(...(state.cubes.length ? [okPill(t("problem.none"))] : [])); // components.js
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

// checkVersion compares the server's version with the one the page was
// loaded with; a newer server reloads the page when nothing is open,
// otherwise it offers a button.
// renderTLS shows, from /healthz's tls, while the certificate the server
// serves is self-signed, a notice under the header and in the sign-in dialog
// asking to compare the fingerprint the browser shows for it with
// massimo tls show on the box before signing in; and a warning while
// the certificate ends within two weeks (or has none yet), computed
// here from not_after so that it speaks the page's language. The page
// shows no fingerprint: it comes over the very connection in question,
// and a man in the middle would pass the server's along, or rewrite it.
function renderTLS(tls) {
  const notice = $("tls-notice");
  $("login-tls").classList.toggle("hidden", !(tls && tls.self_signed));
  if (!tls) {
    notice.classList.add("hidden");
    return;
  }
  const until = Date.parse(tls.not_after || "");
  const texts = [];
  if (tls.self_signed) texts.push(t("header.selfSigned"));
  if (isNaN(until)) {
    if (tls.warning) texts.push(t("header.certMissing"));
  } else {
    const days = Math.floor((until - Date.now()) / 86400000);
    if (days < 0) texts.push(t("header.certExpired"));
    else if (days < 14) texts.push(tn("header.certExpires", days));
  }
  $("tls-notice-text").textContent = texts.join(" ");
  notice.classList.toggle("hidden", texts.length === 0);
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

// SERVER_SIGN is the icon of each state of the server's sign.
const SERVER_SIGN = { up: "icon-plug-on", info: "icon-info", new: "mark-warn", down: "icon-plug-off" };

// serverState is the state the server's sign shows: "down" while the
// last poll failed, "new" once the server runs another version than the
// page, "info" while an update is out for one who may install it
// (update.js updateOut), "up" otherwise.
function serverState() {
  if (state.serverDown) return "down";
  if (state.newVersion) return "new";
  return updateOut() ? "info" : "up"; // update.js
}

// renderServerSign draws the server's sign before the time of the last
// update: its icon in a frame, green, blue, amber (its triangle blinks)
// or filled red (it blinks), its label saying what a click does; while
// the server is gone, the page's other signs are greyed, their state
// unknown, and the time stays that of the last update.
function renderServerSign() {
  const st = serverState();
  const b = $("server-sign");
  if (!b) return;
  b.dataset.state = st;
  const icon = iconEl(SERVER_SIGN[st], "ni"); // components.js
  if (st === "new") icon.firstChild.setAttribute("class", "mark");
  b.replaceChildren(icon);
  const label = t("server." + st, { version: updateLatest(), seconds: state.settings.poll }); // update.js
  b.title = label;
  b.setAttribute("aria-label", label);
  document.body.classList.toggle("server-down", st === "down");
}

// serverSignClick acts by the sign's state: a new version reloads the
// page, an update out opens Software update, otherwise it polls now.
function serverSignClick(ev) {
  const st = serverState();
  if (st === "new") location.reload();
  else if (st === "info") {
    showView("maint"); // pages.js
    showCard($("sys-software")); // components.js
  } else whileBusy($("refresh-btn"), () => poll(true));
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
