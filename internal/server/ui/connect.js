// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

// The Massimo apps' connect page (connect.html, docs/app.md): the
// servers the app keeps, each probed at its LAN and its remote address
// and opened in the app by a tap; the servers discovery finds and an
// address typed, probed before they are added; the fingerprint sheet of
// a self-signed certificate; a server's settings with its login by
// fingerprint, and the row's button that opens it signed in; the server
// on the phone where the app has it built in; what an action came to, in
// the page's toast. Everything goes through the app's bridge,
// window.massimo, whose calls answer with promises and whose events
// arrive as "massimo" events on window; a browser has no bridge and sees
// a line saying that the page is the app's. Loaded after i18n.js and
// components.js, without app.js.
"use strict";

const $ = (id) => document.getElementById(id);

// SEARCH_MS is how long a search for servers on the LAN runs.
const SEARCH_MS = 5000;

// DEFAULT_PORT is serve's port, tried for an address that names none
// (README.md, "Server").
const DEFAULT_PORT = 8099;

const conn = {
  bridge: null, // window.massimo
  servers: [], // as the app keeps them: {id, name, node, lan, remote, fp, login, ask, push}
  probes: {}, // by server id: {state, url, fingerprint, error}, the state probing, lan, remote, cert or offline
  probeSeq: {}, // by server id: the latest probe, whose answer alone counts
  seen: {}, // by node: the URL discovery found it at
  heard: {}, // by node: the name discovery heard for it, "" for none
  found: [], // the servers discovery found that are not in the list: {name, url, node}
  searching: false,
  searchError: "",
  open: null, // the id of the server whose settings are open
  panel: null, // their form
  forgetRow: null, // the row of Forget all stored credentials, held while its question takes its place
  toastServer: null, // the id of the server the toast says does not answer (say)
};

// theBridge is the app's bridge on this page, null in a browser.
function theBridge() {
  const b = window.massimo;
  return b && typeof b.servers === "function" && typeof b.probe === "function" ? b : null;
}

function serverById(id) {
  return conn.servers.find((s) => s.id === id) || null;
}

// say shows what an action on the list or the add form came to in the
// page's toast (showToast of components.js), a newer outcome in the
// place of the one shown, a failure with the toast's red edge. server is
// the id of the server the toast says does not answer, whose probe takes
// it back once the server answers (probeServer).
function say(text, { bad = false, server = null } = {}) {
  conn.toastServer = server;
  showToast(text, { tone: bad ? "bad" : "" });
}

// --- addresses ----------------------------------------------------------

// parseURL makes a server's address of a URL: http or https, a host, no
// user, no query, ending in /; null for anything else.
function parseURL(s) {
  let u;
  try {
    u = new URL(s);
  } catch (e) {
    return null;
  }
  if ((u.protocol !== "https:" && u.protocol !== "http:") || !u.hostname || u.username || u.password) return null;
  u.search = "";
  u.hash = "";
  if (!u.pathname.endsWith("/")) u.pathname += "/";
  return u.href;
}

// addressURLs are the URLs an address typed is probed at, in the order
// in which the first answer wins (docs/app.md, "How the page uses it"):
// a URL as it is, host:port by HTTPS and then HTTP, a host alone at the
// usual ports and then at serve's.
function addressURLs(text) {
  const s = text.trim();
  if (/^[a-z][a-z0-9+.-]*:\/\//i.test(s)) {
    const u = parseURL(s);
    return u ? [u] : [];
  }
  const m = /^(\[[0-9a-f:.]+\]|[^\s/:?#[\]@]+)(?::(\d{1,5}))?(\/[^\s?#]*)?$/i.exec(s);
  if (!m) return [];
  const at = (scheme, port) => parseURL(scheme + "://" + m[1] + (port ? ":" + port : "") + (m[3] || "/"));
  const urls = m[2] ? [at("https", m[2]), at("http", m[2])] : [at("https"), at("http"), at("https", DEFAULT_PORT), at("http", DEFAULT_PORT)];
  return [...new Set(urls.filter(Boolean))];
}

// hostOf is the host and port of an address, for the page to show.
function hostOf(url) {
  try {
    return new URL(url).host;
  } catch (e) {
    return url || "";
  }
}

// isPrivateHost reports whether a host is on the home network: the
// private IPv4 ranges (RFC 1918), loopback, link-local, IPv6 unique
// local and link-local addresses, a .local name or a name without a dot.
function isPrivateHost(host) {
  const h = String(host).toLowerCase().replace(/^\[|\]$/g, "");
  if (h === "localhost" || h.endsWith(".local") || (!h.includes(".") && !h.includes(":"))) return true;
  const v4 = /^(\d+)\.(\d+)\.\d+\.\d+$/.exec(h);
  if (v4) {
    const a = Number(v4[1]);
    const b = Number(v4[2]);
    return a === 10 || a === 127 || (a === 172 && b >= 16 && b <= 31) || (a === 192 && b === 168) || (a === 169 && b === 254);
  }
  if (h.includes(":")) return h === "::1" || /^f[cd]/.test(h) || /^fe[89ab]/.test(h);
  return false;
}

// plainOutside reports a plain-HTTP address outside the home network,
// where anyone on the way reads the sign-in and can change the page.
function plainOutside(url) {
  try {
    const u = new URL(url);
    return u.protocol === "http:" && !isPrivateHost(u.hostname);
  } catch (e) {
    return false;
  }
}

// serviceURL is the address of a server discovery found.
function serviceURL(svc) {
  const host = String(svc.host || "");
  return parseURL((svc.tls ? "https" : "http") + "://" + (host.includes(":") ? "[" + host + "]" : host) + ":" + Number(svc.port) + "/");
}

// --- the servers ----------------------------------------------------------

async function loadServers() {
  try {
    conn.servers = (await conn.bridge.servers()) || [];
  } catch (e) {
    conn.servers = [];
    say(t("connect.loadFailed", { error: e.message }), { bad: true });
  }
}

// keepServer puts a server as the app saved it in the list.
function keepServer(saved) {
  const i = conn.servers.findIndex((s) => s.id === saved.id);
  if (i >= 0) conn.servers[i] = saved;
  else conn.servers.push(saved);
  return saved;
}

// probeState reads a server's probe: the first address that answered
// with the server's node, the LAN one first; else a certificate that is
// not the one trusted, with its fingerprint; else nothing.
function probeState(s, results) {
  for (const r of results) {
    if (r && r.ok && (!s.node || r.node === s.node)) return { state: r.url === s.lan ? "lan" : "remote", url: r.url };
  }
  const cert = results.find((r) => r && r.trusted === false && r.fingerprint);
  if (cert) return { state: "cert", url: cert.url, fingerprint: cert.fingerprint };
  const failed = results.find((r) => r && r.error);
  return { state: "offline", error: failed ? failed.error : "" };
}

// probeServer probes a server at its two addresses with its pinned
// fingerprint, and keeps what it found; a later probe of the same
// server wins over one still running. A server that answers takes back
// the toast that said it does not.
async function probeServer(s) {
  if (!s) return;
  const seq = (conn.probeSeq[s.id] || 0) + 1;
  conn.probeSeq[s.id] = seq;
  conn.probes[s.id] = { state: "probing" };
  renderServers();
  const urls = [s.lan, s.remote].filter(Boolean);
  let results = [];
  try {
    results = urls.length ? (await conn.bridge.probe(urls, s.fp || "")) || [] : [];
  } catch (e) {
    results = [{ ok: false, error: e.message }];
  }
  if (conn.probeSeq[s.id] !== seq) return;
  const p = probeState(s, results);
  conn.probes[s.id] = p;
  if ((p.state === "lan" || p.state === "remote") && conn.toastServer === s.id) {
    conn.toastServer = null;
    hideToast();
  }
  renderServers();
  maybeMoveLan(serverById(s.id));
}

function probeAll() {
  return Promise.all(conn.servers.map(probeServer));
}

// maybeMoveLan takes the address discovery found a saved server at as
// its LAN address, where the one it had did not answer: a LAN address
// that changed is found again.
function maybeMoveLan(s) {
  const url = s && s.node && conn.seen[s.node];
  const p = (s && conn.probes[s.id]) || {};
  if (!url || url === s.lan || (p.state !== "offline" && p.state !== "remote")) return;
  conn.bridge
    .save(Object.assign({}, s, { lan: url }))
    .then((saved) => probeServer(keepServer(saved)))
    .catch(() => {});
}

// stateBadge says what a server's probe found.
function stateBadge(state) {
  switch (state) {
    case "lan":
      return badgeEl(t("connect.state.lan"), "ok");
    case "remote":
      return badgeEl(t("connect.state.remote"), "ok");
    case "cert":
      return badgeEl(t("connect.state.cert"), "warn");
    case "offline":
      return badgeEl(t("connect.state.offline"), "bad");
    default:
      return badgeEl(t("connect.state.probing"), "mode");
  }
}

// ASKS are the choices of when the fingerprint is asked (docs/app.md,
// "The login by fingerprint"), "start" the app's default.
const ASKS = ["return", "start", "logout"];

// serverRow is a server's row, wired by its id: a tap opens the server,
// the fingerprint, where a login is kept, opens it signed in, a long
// press (the context menu) and the cog its settings.
function serverRow(id) {
  const li = fromTemplate("tpl-server");
  li.dataset.id = id;
  const open = li.querySelector(".server-open");
  open.addEventListener("click", () => openServer(id));
  const login = li.querySelector(".server-login");
  login.addEventListener("click", () => whileBusy(login, () => openServer(id, { login: true })));
  open.addEventListener("contextmenu", (ev) => {
    ev.preventDefault();
    toggleSettings(id);
  });
  li.querySelector(".server-cog").addEventListener("click", () => toggleSettings(id));
  return li;
}

// showWhere writes a row's address, or leaves it out where it is the
// row's name already, as for a server named by its address.
function showWhere(el, where, name) {
  el.textContent = where === name ? "" : where;
  el.classList.toggle("hidden", where === name);
}

// fillRow writes a server's name, address and state into its row, and
// its settings under it while they are open.
function fillRow(li, s) {
  const p = conn.probes[s.id] || { state: "probing" };
  const name = s.name || hostOf(s.lan || s.remote);
  li.querySelector(".server-name").textContent = name;
  showWhere(li.querySelector(".server-where"), hostOf(p.url || s.lan || s.remote), name);
  const badges = li.querySelector(".server-badges");
  clear(badges);
  badges.appendChild(stateBadge(p.state));
  if (p.url && plainOutside(p.url)) badges.appendChild(badgeEl(t("connect.state.plain"), "warn"));
  const login = li.querySelector(".server-login");
  login.classList.toggle("hidden", !s.login);
  if (s.login) {
    login.setAttribute("aria-label", t("connect.loginAs", { name: s.login }));
    login.title = t("connect.loginAs", { name: s.login });
  }
  const cog = li.querySelector(".server-cog");
  cog.setAttribute("aria-expanded", String(conn.open === s.id));
  if (conn.open === s.id && conn.panel && conn.panel.parentNode !== li) li.appendChild(conn.panel);
  if (conn.open !== s.id) {
    const panel = li.querySelector(".server-panel");
    if (panel) panel.remove();
  }
}

// renderServers brings the list up to date row by row, so that a probe
// that answers keeps the focus and what is typed in the settings.
function renderServers() {
  const list = $("server-list");
  const rows = new Map([...list.children].map((li) => [li.dataset.id, li]));
  let prev = null;
  for (const s of conn.servers) {
    let li = rows.get(s.id);
    rows.delete(s.id);
    if (!li) li = serverRow(s.id);
    fillRow(li, s);
    const at = prev ? prev.nextElementSibling : list.firstElementChild;
    if (at !== li) list.insertBefore(li, at);
    prev = li;
  }
  for (const li of rows.values()) li.remove();
  $("servers-empty").classList.toggle("hidden", conn.servers.length > 0);
  if (conn.forgetRow) conn.forgetRow.classList.toggle("hidden", !conn.servers.some((s) => s.login));
}

// openServer opens a server in the app at the address its probe found,
// with opts {login: true} signed in by fingerprint first (docs/app.md);
// a certificate that is not the one trusted asks first, and a server
// that did not answer is probed again, and said not to answer until a
// probe finds it. A kept login the server refused is gone from the list
// afterwards.
async function openServer(id, opts) {
  const s = serverById(id);
  const p = conn.probes[id] || {};
  if (!s || p.state === "probing") return;
  if (p.state === "lan" || p.state === "remote") {
    try {
      if (opts) await conn.bridge.open(s.id, p.url, opts);
      else await conn.bridge.open(s.id, p.url);
    } catch (e) {
      say(t("connect.openFailed", { name: s.name, error: e.message }), { bad: true });
      if (opts && opts.login) {
        await loadServers();
        renderServers();
      }
    }
    return;
  }
  if (p.state === "cert") {
    if (!(await askTrust({ url: p.url, old: s.fp, fresh: p.fingerprint }))) return;
    try {
      await conn.bridge.trust(s.id, p.fingerprint);
    } catch (e) {
      say(t("connect.saveFailed", { error: e.message }), { bad: true });
      return;
    }
    await loadServers();
    renderServers();
  }
  await probeServer(serverById(id));
  const q = conn.probes[id] || {};
  if (q.state === "lan" || q.state === "remote" || (q.state === "cert" && p.state !== "cert")) return openServer(id, opts);
  say(t("connect.unreachable", { name: s.name }) + (q.error ? " " + q.error : ""), { bad: true, server: id });
}

// --- a server's settings ----------------------------------------------

// toggleSettings opens a server's settings under its row, closing any
// other, or closes them.
function toggleSettings(id) {
  conn.open = conn.open === id ? null : id;
  conn.panel = conn.open ? settingsPanel(serverById(conn.open)) : null;
  renderServers();
  if (conn.panel) conn.panel.querySelector("input").focus();
}

function addressInput(value) {
  const i = document.createElement("input");
  i.type = "text";
  i.inputMode = "url";
  i.autocomplete = "off";
  i.setAttribute("autocapitalize", "none");
  i.spellcheck = false;
  i.value = value || "";
  return i;
}

// settingsPanel is a server's settings: its name, its two addresses,
// when the fingerprint is asked, the login by fingerprint kept with its
// user and Turn off, notifications where the app has push, and Remove.
function settingsPanel(s) {
  const name = document.createElement("input");
  name.type = "text";
  name.maxLength = 64;
  name.value = s.name || "";
  const lan = addressInput(s.lan);
  const remote = addressInput(s.remote);
  const ask = document.createElement("select");
  ask.name = "ask";
  for (const a of ASKS) {
    const o = textEl("option", t("connect.ask." + a));
    o.value = a;
    ask.appendChild(o);
  }
  ask.value = ASKS.includes(s.ask) ? s.ask : "start";
  const push = conn.bridge.push ? switchEl(t("connect.push"), s.push, null, "push") : null;
  const msg = textEl("p", "", "small cubes-msg server-msg hidden");
  msg.setAttribute("role", "status");
  const form = formEl(() => saveSettings(s.id, form, { name, lan, remote, ask, push, msg }), "panel server-panel");
  const remove = button(t("connect.remove"), "server-remove", () => {
    confirmInPlace(form.querySelector(".actions"), t("connect.removeQuestion", { name: s.name }), { yes: t("connect.remove") },
      () => removeServer(s.id, msg), ".server-remove");
  });
  form.append(
    fieldRow(t("connect.name"), name, { wide: true }),
    fieldRow(t("connect.lan"), lan, { wide: true, hint: t("connect.lanHint") }),
    fieldRow(t("connect.remote"), remote, { wide: true, hint: t("connect.remoteHint") }),
    fieldRow(t("connect.ask"), ask, { wide: true, hint: t("connect.askHint") }),
  );
  if (s.login) {
    const off = button(t("connect.loginOff"), "server-login-off", () => turnOff(s.id, off, msg));
    const kept = textEl("p", t("connect.loginKept", { name: s.login }), "server-login-kept");
    kept.append(" ", off);
    form.append(kept);
  }
  if (push) form.append(fieldRow("", push));
  form.append(msg, formFooter({ submit: t("action.save"), cancel: () => toggleSettings(null), extra: [remove] }));
  return form;
}

async function saveSettings(id, form, f) {
  const s = serverById(id);
  const lanText = f.lan.value.trim();
  const remoteText = f.remote.value.trim();
  const lan = lanText ? parseURL(lanText) : "";
  const remote = remoteText ? parseURL(remoteText) : "";
  if (lan === null || remote === null) return showResult(f.msg, t("connect.badURL"), false);
  if (!lan && !remote) return showResult(f.msg, t("connect.noAddress"), false);
  const name = f.name.value.trim() || hostOf(lan || remote);
  const want = Object.assign({}, s, {
    name,
    lan,
    remote,
    ask: ASKS.includes(f.ask.value) ? f.ask.value : "start",
    push: f.push ? f.push.querySelector("input").checked : !!s.push,
  });
  try {
    await whileBusy(form.querySelector("button[type=submit]"), async () => keepServer(await conn.bridge.save(want)));
  } catch (e) {
    return showResult(f.msg, t("connect.saveFailed", { error: e.message }), false);
  }
  conn.open = null;
  conn.panel = null;
  say(t("connect.saved", { name }));
  probeServer(serverById(id));
}

// turnOff turns a server's login by fingerprint off (forget, which asks
// for the fingerprint once and revokes the key at the address the probe
// chose); the open settings lose its line and keep what is typed in
// them, and the toast says whether the server revoked the key.
async function turnOff(id, btn, msg) {
  const s = serverById(id);
  const p = conn.probes[id] || {};
  const url = p.state === "lan" || p.state === "remote" ? p.url : "";
  let res;
  try {
    res = await whileBusy(btn, () => conn.bridge.forget(id, url));
  } catch (e) {
    showResult(msg, t("connect.loginOffFailed", { error: e.message }), false);
    return;
  }
  await loadServers();
  dropLoginLine();
  renderServers();
  const name = s ? s.name : "";
  say(res && res.revoked ? t("connect.loginOffDone", { name }) : t("connect.loginOffLocal", { name }));
}

// dropLoginLine takes the line of a kept login out of the open settings
// once the app forgot it, so that what is typed in them stays.
function dropLoginLine() {
  const s = conn.open && serverById(conn.open);
  const line = conn.panel && conn.panel.querySelector(".server-login-kept");
  if (line && !(s && s.login)) line.remove();
}

// forgetAll forgets every kept login, once the user confirmed it in
// place of the button; the keys stay at their servers.
function forgetAll() {
  const row = conn.forgetRow;
  const box = confirmInPlace(row, t("connect.forgetAllQuestion"), { yes: t("connect.forgetAll") }, async () => {
    try {
      await conn.bridge.forgetAll();
    } catch (e) {
      box.replaceWith(row);
      say(t("connect.loginOffFailed", { error: e.message }), { bad: true });
      return;
    }
    box.replaceWith(row);
    await loadServers();
    dropLoginLine();
    renderServers();
    say(t("connect.forgotAll"));
  }, "#forget-all");
}

// removeServer forgets a server, its certificate, its session and its
// kept login, and says so in the toast; an error is said in its
// settings, which stay open.
async function removeServer(id, msg) {
  const s = serverById(id);
  try {
    await conn.bridge.remove(id);
  } catch (e) {
    showResult(msg, t("connect.removeFailed", { error: e.message }), false);
    return;
  }
  conn.servers = conn.servers.filter((x) => x.id !== id);
  delete conn.probes[id];
  conn.open = null;
  conn.panel = null;
  renderServers();
  say(t("connect.removed", { name: s ? s.name : "" }));
  $("add-address").focus();
}

// --- adding a server --------------------------------------------------

// showFingerprint writes a fingerprint into an element so that a line
// breaks only after one of its colons, never inside a pair of digits,
// and a copy of it is the fingerprint as it is.
function showFingerprint(el, fp) {
  clear(el);
  const parts = (fp || "").split(":");
  parts.forEach((part, i) => {
    el.append(i < parts.length - 1 ? part + ":" : part);
    if (i < parts.length - 1) el.appendChild(document.createElement("wbr"));
  });
}

// askTrust shows the fingerprint sheet for the server at url and
// resolves true when the user trusts the certificate: fresh, the
// server's, for the first time, or in place of old, the one trusted
// until now, whose change it warns of.
function askTrust({ url, old, fresh }) {
  const dlg = $("fp-dialog");
  const params = { address: hostOf(url) };
  $("fp-title").textContent = old ? t("connect.fpChangedTitle") : t("connect.fpTitle");
  $("fp-text").textContent = old ? t("connect.fpChangedText", params) : t("connect.fpText", params);
  $("fp-old-row").classList.toggle("hidden", !old);
  showFingerprint($("fp-old"), old);
  showFingerprint($("fp-new"), fresh);
  dlg.classList.toggle("fp-changed", !!old);
  return new Promise((resolve) => {
    let trusted = false;
    $("fp-trust").onclick = () => {
      trusted = true;
      dlg.close();
    };
    $("fp-cancel").onclick = () => dlg.close();
    dlg.onclose = () => {
      $("fp-trust").onclick = null;
      $("fp-cancel").onclick = null;
      dlg.onclose = null;
      resolve(trusted);
    };
    dlg.showModal();
    $("fp-cancel").focus();
  });
}

// addServer probes the addresses of a server to add and adds it at the
// first that answers, after the fingerprint sheet where its certificate
// is not one the phone trusts; what happened goes to the toast. A server
// already in the list by its node gets the address instead. It resolves
// whether the server is in the list now.
async function addServer(urls, { name, node }) {
  let results;
  try {
    results = (await conn.bridge.probe(urls, "")) || [];
  } catch (e) {
    say(t("connect.noAnswer", { address: hostOf(urls[0]) }) + " " + e.message, { bad: true });
    return false;
  }
  const r = results.find((x) => x && (x.ok || (x.trusted === false && x.fingerprint)));
  if (!r) {
    const failed = results.find((x) => x && x.error);
    say(t("connect.noAnswer", { address: hostOf(urls[0]) }) + (failed ? " " + failed.error : ""), { bad: true });
    return false;
  }
  let answer = r;
  let fp = "";
  if (!r.ok) {
    if (!(await askTrust({ url: r.url, old: "", fresh: r.fingerprint }))) {
      say(t("connect.notTrusted"), { bad: true });
      return false;
    }
    fp = r.fingerprint;
    try {
      [answer] = (await conn.bridge.probe([r.url], fp)) || [];
    } catch (e) {
      answer = { ok: false, error: e.message };
    }
    if (!answer || !answer.ok) {
      say(t("connect.noAnswer", { address: hostOf(r.url) }) + (answer && answer.error ? " " + answer.error : ""), { bad: true });
      return false;
    }
  }
  return saveNew({ name, url: answer.url || r.url, node: answer.node || node || "", fp });
}

// saveNew keeps a server that answered: a new one named as discovery
// heard its node where it did, else by its address, until discovery
// hears it (takeHeardName); one in the list by its node takes the
// address.
async function saveNew({ name, url, node, fp }) {
  const where = isPrivateHost(new URL(url).hostname) ? "lan" : "remote";
  const known = node ? conn.servers.find((s) => s.node === node) : null;
  const heard = node ? conn.heard[node] : "";
  let saved;
  try {
    if (known) {
      saved = await conn.bridge.save(Object.assign({}, known, { [where]: url }));
    } else {
      saved = await conn.bridge.save({ name: name || heard || hostOf(url), node, lan: where === "lan" ? url : "", remote: where === "remote" ? url : "", fp: "", ask: "start", push: false });
    }
    if (fp && fp !== saved.fp) await conn.bridge.trust(saved.id, fp);
  } catch (e) {
    say(t("connect.saveFailed", { error: e.message }), { bad: true });
    return false;
  }
  await loadServers();
  conn.found = conn.found.filter((f) => !(node && f.node === node) && f.url !== url);
  renderFound();
  renderServers();
  say(known ? t("connect.known", { name: saved.name }) : t("connect.added", { name: saved.name }));
  probeServer(serverById(saved.id));
  return true;
}

// addAddress adds the server at the address typed; one that is not an
// address is said under the field, where it is typed.
async function addAddress() {
  const input = $("add-address");
  const msg = $("add-msg");
  const urls = addressURLs(input.value);
  if (!urls.length) {
    showResult(msg, t("connect.badAddress"), false);
    input.focus();
    return;
  }
  showResult(msg, "", false);
  if (await whileBusy($("add-form").querySelector("button[type=submit]"), () => addServer(urls, {}))) {
    input.value = "";
    warnPlain();
  }
}

// warnPlain warns under the address of a plain-HTTP URL typed outside
// the home network.
function warnPlain() {
  const urls = addressURLs($("add-address").value);
  $("add-warn").classList.toggle("hidden", !(urls.length === 1 && plainOutside(urls[0])));
}

// --- discovery ----------------------------------------------------------

// search asks the app to look for servers on the LAN; they arrive as
// found events while it runs.
async function search() {
  if (conn.searching) return;
  conn.searching = true;
  conn.searchError = "";
  renderFound();
  try {
    await conn.bridge.discover(SEARCH_MS);
  } catch (e) {
    conn.searchError = e.message;
  }
  conn.searching = false;
  renderFound();
}

// namedByAddress reports whether a server is still named by one of its
// addresses, as the page names a server added by its address (saveNew)
// or saved without a name (saveSettings): not by a name the user gave.
function namedByAddress(s) {
  return [s.lan, s.remote].some((u) => u && s.name === hostOf(u));
}

// takeHeardName names a saved server by the name discovery heard for its
// node while it is still named by its address; a name the user gave it
// stays. It resolves the server as it stands.
async function takeHeardName(s, name) {
  if (!name || name === s.name || !namedByAddress(s)) return s;
  try {
    keepServer(await conn.bridge.save(Object.assign({}, s, { name })));
  } catch (e) {
    return s; // named again at the next search
  }
  renderServers();
  return serverById(s.id) || s;
}

// onFound takes a server discovery found: a saved one by its node takes
// the name heard where it is named by its address (takeHeardName), then
// is noted for its LAN address (maybeMoveLan); any other is offered.
function onFound(svc) {
  const url = svc && serviceURL(svc);
  if (!url) return;
  const node = String(svc.node || "");
  const name = String(svc.name || "");
  const saved = node ? conn.servers.find((s) => s.node === node) : null;
  if (node) {
    conn.seen[node] = url;
    conn.heard[node] = name;
  }
  if (saved) {
    takeHeardName(saved, name).then(maybeMoveLan);
    return;
  }
  const f = { name: name || hostOf(url), url, node };
  const i = conn.found.findIndex((x) => (node && x.node === node) || x.url === url);
  if (i >= 0) conn.found[i] = f;
  else conn.found.push(f);
  renderFound();
}

function foundRow(f) {
  const li = fromTemplate("tpl-found");
  li.dataset.url = f.url;
  const add = li.querySelector(".found-add");
  add.addEventListener("click", () => whileBusy(add, () => addServer([f.url], { name: f.name, node: f.node })));
  return li;
}

// renderFound brings the found servers up to date row by row, and says
// how the search goes.
function renderFound() {
  const list = $("found-list");
  const rows = new Map([...list.children].map((li) => [li.dataset.url, li]));
  for (const f of conn.found) {
    let li = rows.get(f.url);
    rows.delete(f.url);
    if (!li) {
      li = foundRow(f);
      list.appendChild(li);
    }
    li.querySelector(".found-name").textContent = f.name;
    showWhere(li.querySelector(".found-where"), hostOf(f.url), f.name);
  }
  for (const li of rows.values()) li.remove();
  let state = "";
  if (conn.searching) state = t("connect.searching");
  else if (conn.searchError) state = t("connect.searchFailed", { error: conn.searchError });
  else if (!conn.found.length) state = t("connect.foundNone");
  $("found-state").textContent = state;
  setBusy($("search-btn"), conn.searching);
}

// --- the server on the phone (Block 7 of AP MobileApp) -----------------

function renderCore(st) {
  const s = st || {};
  let text = t("connect.coreStopped");
  if (s.running) text = t("connect.coreRunning", { address: hostOf(s.address || "") });
  else if (s.error) text = t("connect.coreError", { error: s.error });
  $("core-status").textContent = text;
  setOff($("core-start"), !!s.running);
  setOff($("core-stop"), !s.running);
}

async function coreDo(btn, fn) {
  try {
    const st = await whileBusy(btn, fn);
    if (st) renderCore(st);
  } catch (e) {
    renderCore({ running: false, error: e.message });
  }
}

// initCore shows the card of the server on the phone where the app has
// it built in.
async function initCore() {
  const core = conn.bridge.core;
  if (!core || typeof core.status !== "function") return;
  $("core-card").classList.remove("hidden");
  $("core-start").addEventListener("click", (ev) => coreDo(ev.currentTarget, () => core.start()));
  $("core-stop").addEventListener("click", (ev) => coreDo(ev.currentTarget, () => core.stop()));
  try {
    renderCore(await core.status());
  } catch (e) {
    renderCore({ running: false, error: e.message });
  }
}

// --- start ----------------------------------------------------------------

// onBridgeEvent takes the app's events; an unknown type is ignored.
function onBridgeEvent(ev) {
  const d = (ev && ev.detail) || {};
  if (d.type === "found") onFound(d.service);
  else if (d.type === "network") probeAll();
  else if (d.type === "core") renderCore(d.status);
}

async function init() {
  try {
    await setLanguage("auto");
  } catch (e) {
    // the texts stay those of the markup, in English
  }
  applyI18n(document);
  conn.bridge = theBridge();
  if (!conn.bridge) {
    $("no-bridge").classList.remove("hidden");
    return;
  }
  $("servers").classList.remove("hidden");
  $("add").classList.remove("hidden");
  window.addEventListener("massimo", onBridgeEvent);
  $("add-form").addEventListener("submit", (ev) => {
    ev.preventDefault();
    addAddress();
  });
  $("add-address").addEventListener("input", warnPlain);
  $("probe-btn").addEventListener("click", (ev) => whileBusy(ev.currentTarget, probeAll));
  $("search-btn").addEventListener("click", search);
  conn.forgetRow = $("forget-all-row");
  $("forget-all").addEventListener("click", forgetAll);
  await loadServers();
  renderServers();
  probeAll();
  search();
  initCore();
}

document.addEventListener("DOMContentLoaded", init);
