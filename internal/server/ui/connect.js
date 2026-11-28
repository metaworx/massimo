// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

// The Massimo apps' connect page (connect.html, docs/app.md): the
// servers the app keeps, sorted by the name shown, each probed at its LAN
// and its remote address, marked where the app holds a session, and
// opened in the app by a tap, the app asking for the fingerprint first
// where a login is kept; the list's ☰ in the header (the app's version,
// Add server…, Sign out of all, Forget all stored credentials, App
// settings, Exit); the servers discovery finds and an address
// typed, probed before they are added, in a card shown while the list is
// empty or from the ☰; the fingerprint sheet of a self-signed
// certificate; a server's settings with Open at start, its session, its
// certificate, its login by fingerprint with Store login, and the keys and
// sessions of the phone's left valid there, and the row's sign-in icon
// that opens its sign-in card; the app's
// own settings, its appearance; a server's settings or App settings open at
// the start where a server's ☰ asked for them; a server whose page failed
// to load said not to answer; Back, which closes the open
// menu, then the open panel, through a history entry of the page's own;
// a pull down in the app, which checks every server again as ↻ does; the
// server on the phone where the app has
// it built in; what an action came to, in the page's toast. Everything
// goes through the app's bridge, window.massimo, whose calls answer with
// promises and whose events arrive as "massimo" events on window; a
// browser has no bridge and sees a line saying that the page is the
// app's. Loaded after i18n.js and components.js, without app.js.
"use strict";

const $ = (id) => document.getElementById(id);

// SEARCH_MS is how long a search for servers on the LAN runs.
const SEARCH_MS = 5000;

// DEFAULT_PORT is serve's port, tried for an address that names none
// (README.md, "Server").
const DEFAULT_PORT = 8099;

// APPEARANCES are the app's appearances (docs/app.md, `settings`), as
// the system's the default.
const APPEARANCES = ["system", "light", "dark"];

// APP_HASH is the page's address's fragment while App settings are open,
// so that the page the app loads again once it took a new appearance
// opens them again; the app loads the page with it from a server's ☰,
// App settings… (contract version 5).
const APP_HASH = "#app-settings";

// SETTINGS_HASH is the fragment the app loads the page with from a
// server's ☰, This server's settings… (contract version 5): that server's
// settings open at the page's start (openFromAddress).
const SETTINGS_HASH = /^#settings=([A-Za-z0-9_.~-]{1,64})$/;

// UNREACHABLE_HASH is the fragment the app loads the page with where a
// server's page failed to load (contract version 7; AP MobileApp, D108-A):
// that server is probed at the page's start and said not to answer
// (unreachableFromAddress).
const UNREACHABLE_HASH = /^#unreachable=([A-Za-z0-9_.~-]{1,64})$/;

const conn = {
  bridge: null, // window.massimo
  servers: [], // as the app keeps them: {id, name, node, lan, remote, fp, login, login_key, ask, push, start, session, trusted_at, unrevoked, unended}
  probes: {}, // by server id: {state, url, fingerprint, cert, error}, the state probing, lan, remote, cert or offline
  probeSeq: {}, // by server id: the latest probe, whose answer alone counts
  seen: {}, // by node: the URL discovery found it at
  heard: {}, // by node: the name discovery heard for it, "" for none
  found: [], // the servers discovery found that are not in the list: {name, url, node}
  searching: false,
  searchError: "",
  adding: false, // the Add card chosen from the ☰, shown with a list that is not empty
  app: { appearance: "system" }, // the app's settings, as settings() said
  open: null, // the id of the server whose settings are open
  opening: false, // a server being opened, its prompt perhaps shown (openServer)
  panel: null, // their form
  fields: null, // its fields, {name, lan, remote, start, ask, push} (connectionDiffers)
  opened: null, // what they held as the settings opened, or as what is kept last changed (takeKept)
  discard: null, // the question of settings with changes, while it is asked (closeSettingsAsked)
  panels: [], // the panels open, "settings", "add" or "app", the one opened last at the end (closeTop)
  can: true, // the phone has a strong biometric enrolled, as about() said (Store login)
  toastServer: null, // the id of the server the toast says does not answer (say)
};

// BACK_READY_MS is how long after the page's load the page waits before
// its first history entry, so that the app's forgetting of the pages
// before the list (MainActivity onPageFinished, clearHistory) is done.
const BACK_READY_MS = 500;

// BACK_MS is how long a take-back of the page's history entry waits for
// its popstate before it ends all the same: there may be no entry left.
const BACK_MS = 1000;

// back is the page's own history entry (AP MobileApp, Block 19): one
// while a menu or a panel is open, so that the app's Back reaches the
// page as a popstate, which closes it (onPopState); none while nothing is
// open, so that Back leaves the app. entry: it is there; leaving: a
// take-back of it runs; ask: the question that Back asked, during which
// there is none; ready: the page may push one; touch: it waits for the
// user's first touch (awaitTouch).
const back = { entry: false, leaving: false, ask: null, ready: false, timer: null, touch: false };

// theBridge is the app's bridge on this page, null in a browser.
function theBridge() {
  const b = window.massimo;
  return b && typeof b.servers === "function" && typeof b.probe === "function" ? b : null;
}

function serverById(id) {
  return conn.servers.find((s) => s.id === id) || null;
}

// can reports whether the app's bridge has a call of contract version 3
// or later (docs/app.md): the page tests for what it uses.
function can(name) {
  return !!conn.bridge && typeof conn.bridge[name] === "function";
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

// certOf is what a probe result tells of the certificate at its address
// (docs/app.md, "A probe result"), null for plain HTTP.
function certOf(r) {
  if (!r || !r.fingerprint) return null;
  return {
    fingerprint: r.fingerprint,
    self_signed: !!r.self_signed,
    subject: String(r.subject || ""),
    issuer: String(r.issuer || ""),
    names: Array.isArray(r.names) ? r.names.map(String) : [],
    not_before: String(r.not_before || ""),
    not_after: String(r.not_after || ""),
  };
}

// probeState reads a server's probe: the first address that answered
// with the server's node, the LAN one first, with its certificate; else
// a certificate that is not the one trusted, with its fingerprint; else
// nothing.
function probeState(s, results) {
  for (const r of results) {
    if (r && r.ok && (!s.node || r.node === s.node)) return { state: r.url === s.lan ? "lan" : "remote", url: r.url, cert: certOf(r) };
  }
  const cert = results.find((r) => r && r.trusted === false && r.fingerprint);
  if (cert) return { state: "cert", url: cert.url, fingerprint: cert.fingerprint };
  const failed = results.find((r) => r && r.error);
  return { state: "offline", error: failed ? failed.error : "" };
}

// probeServer probes a server at its two addresses with its pinned
// fingerprint, and keeps what it found; a later probe of the same
// server wins over one still running. A server that answers takes back
// the toast that said it does not, and has the session the app holds
// for it checked there (checkSession).
async function probeServer(s) {
  if (!s) return;
  const seq = (conn.probeSeq[s.id] || 0) + 1;
  conn.probeSeq[s.id] = seq;
  // what the last probe found, kept while this one runs (shownDown)
  const prev = conn.probes[s.id];
  conn.probes[s.id] = { state: "probing", was: prev ? (prev.state === "probing" ? prev.was : prev.state) : undefined };
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
  const answers = p.state === "lan" || p.state === "remote";
  if (answers && conn.toastServer === s.id) {
    conn.toastServer = null;
    hideToast();
  }
  renderServers();
  if (answers) checkSession(s.id, p.url, seq);
  maybeMoveLan(serverById(s.id));
}

// checkSession asks the app who the session it holds for a server is,
// checked at the address its probe found (session, docs/app.md), the
// app posting the server's revoke codes there first (contract version 6):
// the row's badge and the settings' line follow; a session the server
// refused the app has forgotten. A call that fails leaves what the app
// said before; a newer probe's check alone counts.
async function checkSession(id, url, seq) {
  if (!can("session")) return;
  let user;
  try {
    user = (await conn.bridge.session(id, url)) || null;
  } catch (e) {
    return;
  }
  const s = serverById(id);
  if (!s || conn.probeSeq[id] !== seq) return;
  // a session the app holds revokes the keys left there for its user, and
  // a code the server took ends its entry: the list read again, so that
  // their lines go
  if ((user && keysLeft(s).some((k) => k.user === user)) || hasPending(s)) {
    await loadServers();
    renderServers();
    return;
  }
  if ((s.session || null) === user) return;
  s.session = user;
  renderServers();
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

// shownName is the name a server is shown by: its name, else the host of
// its address.
function shownName(s) {
  return s.name || hostOf(s.lan || s.remote);
}

// byName are the servers in the order the list shows them (AP MobileApp,
// D95-A): by the name shown, in the collation of the page's language, case
// and accents ignored and numbers by their value, "server 9" before
// "server 10"; the order the app keeps them in stays as it is.
function byName(servers) {
  const c = new Intl.Collator(i18n.lang, { sensitivity: "base", numeric: true });
  return [...servers].sort((a, b) => c.compare(shownName(a), shownName(b)));
}

// serverRow is a server's row, wired by its id: a tap opens the server,
// the app asking for the fingerprint first where a login is kept, as its
// rule says (open, contract version 5); the sign-in icon, where a login is
// kept, opens it at its sign-in card without the prompt ({signIn: true});
// a long press (the context menu) and the cog open its settings.
function serverRow(id) {
  const li = fromTemplate("tpl-server");
  li.dataset.id = id;
  const open = li.querySelector(".server-open");
  open.addEventListener("click", () => openServer(id));
  const signIn = li.querySelector(".server-signin");
  signIn.addEventListener("click", () => whileBusy(signIn, () => openServer(id, { signIn: true })));
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

// shownDown reports whether a server is shown as one that does not answer
// (AP MobileApp, F42, D105-A): its probe found it at neither address; a
// probe still running keeps what the last one found, and the first, which
// has found nothing yet, shows none down.
function shownDown(p) {
  const st = p.state === "probing" ? p.was : p.state;
  return st !== undefined && st !== "lan" && st !== "remote";
}

// fillRow writes a server's name, address, state and session into its
// row, and its settings under it while they are open; the session's badge
// is grey while the server is shown down (shownDown).
function fillRow(li, s) {
  const p = conn.probes[s.id] || { state: "probing" };
  const name = shownName(s);
  li.querySelector(".server-name").textContent = name;
  showWhere(li.querySelector(".server-where"), hostOf(p.url || s.lan || s.remote), name);
  const badges = li.querySelector(".server-badges");
  clear(badges);
  badges.appendChild(stateBadge(p.state));
  if (p.url && plainOutside(p.url)) badges.appendChild(badgeEl(t("connect.state.plain"), "warn"));
  if (s.session) badges.appendChild(badgeEl(t("connect.signedIn", { name: s.session }), (shownDown(p) ? "mode" : "fill-ok") + " session-badge"));
  li.querySelector(".server-signin").classList.toggle("hidden", !s.login);
  const cog = li.querySelector(".server-cog");
  cog.setAttribute("aria-expanded", String(conn.open === s.id));
  if (conn.open === s.id && conn.panel && conn.panel.parentNode !== li) li.appendChild(conn.panel);
  if (conn.open === s.id && conn.panel) fillFacts(conn.panel, s);
  if (conn.open !== s.id) {
    const panel = li.querySelector(".server-panel");
    if (panel) panel.remove();
  }
}

// renderMenu shows the ☰'s entries that do something: Sign out of all
// where there is a server, Forget all stored credentials where a login
// is kept, App settings and Exit where the app has them, the rule before
// them with them.
function renderMenu() {
  const any = conn.servers.length > 0;
  $("menu-sign-out-all").classList.toggle("hidden", !(any && can("signOutAll")));
  $("menu-forget-all").classList.toggle("hidden", !conn.servers.some((s) => s.login));
  $("menu-app").classList.toggle("hidden", !can("settings"));
  $("menu-exit").classList.toggle("hidden", !can("exit"));
  $("menu-app-sep").classList.toggle("hidden", !can("settings") && !can("exit"));
}

// renderServers brings the list up to date row by row, in the order of
// the names shown (byName), so that a probe that answers keeps the focus
// and what is typed in the settings; then the ☰ and the Add card, and the
// page's history entry (syncBack).
function renderServers() {
  const list = $("server-list");
  const rows = new Map([...list.children].map((li) => [li.dataset.id, li]));
  let prev = null;
  for (const s of byName(conn.servers)) {
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
  renderMenu();
  renderAdd();
  syncBack();
}

// openServer opens a server in the app at the address its probe found,
// the app asking for the fingerprint first where its rule says so, and
// with opts {signIn: true} at its sign-in card without a prompt
// (docs/app.md, contract version 5), and at hash, My profile's
// `#profile?key=<id>`, with a key marked (contract version 4); one opening
// at a time, so that a second tap while the prompt shows asks nothing; a
// certificate that is not the one trusted asks first, and a server that
// did not answer is probed again, and said not to answer until a probe
// finds it. A kept login the server refused, or that the phone no longer
// opens, is gone from the list afterwards.
async function openServer(id, opts, hash) {
  const s = serverById(id);
  const p = conn.probes[id] || {};
  if (!s || p.state === "probing") return;
  if (p.state === "lan" || p.state === "remote") {
    if (conn.opening) return;
    const url = p.url + (hash || "");
    conn.opening = true;
    try {
      if (opts) await conn.bridge.open(s.id, url, opts);
      else await conn.bridge.open(s.id, url);
    } catch (e) {
      say(t("connect.openFailed", { name: shownName(s), error: e.message }), { bad: true });
      if (s.login) {
        await loadServers();
        renderServers();
      }
    } finally {
      conn.opening = false;
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
  if (q.state === "lan" || q.state === "remote" || (q.state === "cert" && p.state !== "cert")) return openServer(id, opts, hash);
  say(t("connect.unreachable", { name: shownName(s) }) + (q.error ? " " + q.error : ""), { bad: true, server: id });
}

// --- a server's settings ----------------------------------------------

// toggleSettings opens a server's settings under its row, closing any
// other, or closes them. The open settings take the focus themselves,
// not their name's field, so that a phone's keyboard stays closed until
// a field is touched.
function toggleSettings(id) {
  if (!id || conn.open === id) settingsClosed();
  else {
    conn.open = id;
    conn.panel = settingsPanel(serverById(id));
    panelShown("settings", true);
    askCan();
  }
  renderServers();
  if (conn.panel) conn.panel.focus();
}

// settingsClosed notes that the open settings are gone, closed, or their
// server saved or removed: none open, and the page's history entry
// brought up to date (panelShown), so that Back does not reach them.
function settingsClosed() {
  conn.open = null;
  conn.panel = null;
  conn.fields = null;
  panelShown("settings", false);
}

// closeSettings closes the open settings without asking, the focus on
// their server's cog.
function closeSettings() {
  const id = conn.open;
  if (!id) return;
  toggleSettings(id);
  const cog = document.querySelector(`.server-row[data-id="${CSS.escape(id)}"] .server-cog`);
  if (cog) cog.focus();
}

// connectionDiffers reports whether the open settings' fields differ from
// what the app keeps of the server s, field by field (AP MobileApp, D93-A).
function connectionDiffers(s) {
  const f = conn.fields;
  if (!f || !s) return false;
  const on = (sw) => sw.querySelector("input").checked;
  return f.name.value.trim() !== (s.name || "") || f.lan.value.trim() !== (s.lan || "") || f.remote.value.trim() !== (s.remote || "") ||
    on(f.start) !== !!s.start || f.ask.value !== (ASKS.includes(s.ask) ? s.ask : "start") || (!!f.push && on(f.push) !== !!s.push);
}

// markConnection turns Save and Cancel of the open settings off while
// Connection holds what is kept (AP MobileApp, D94-A), compared field by
// field at each input, as the server's forms are, and marks Connection's
// tab while it differs, AP UnsavedMarks' disk in amber (signEl), named
// for a screen reader.
function markConnection() {
  const panel = conn.panel;
  if (!panel) return;
  const differs = connectionDiffers(serverById(conn.open));
  const form = panel.querySelector(".server-connection");
  setOff(form.querySelector("button[type=submit]"), !differs);
  setOff(form.querySelector(".form-cancel"), !differs);
  const mark = panel.querySelector('.server-tab[data-tab="connection"] .server-tab-mark');
  if (differs === !!mark.firstChild) return;
  clear(mark);
  if (differs) mark.appendChild(signEl("disk", "warn", { cls: "server-tab-unsaved", label: t("problem.unsaved") }));
}

// keptValues are what the app keeps of a server s as Connection's fields
// show it: the name, the two addresses, Open at start, when the
// fingerprint is asked, notifications.
function keptValues(s) {
  return { name: s.name || "", lan: s.lan || "", remote: s.remote || "", start: !!s.start, ask: ASKS.includes(s.ask) ? s.ask : "start", push: !!s.push };
}

// fieldValues are what Connection's fields f hold now, as keptValues has
// them; push is the kept one where there is no switch for it.
function fieldValues(f, s) {
  const on = (sw) => sw.querySelector("input").checked;
  return { name: f.name.value.trim(), lan: f.lan.value.trim(), remote: f.remote.value.trim(), start: on(f.start), ask: f.ask.value, push: f.push ? on(f.push) : !!s.push };
}

// setFields gives Connection's fields f the values v, each where it
// differs; only (a set of keys), where given, the ones to set.
function setFields(f, v, only) {
  for (const k of ["name", "lan", "remote"]) if ((!only || only.has(k)) && f[k].value.trim() !== v[k]) f[k].value = v[k];
  for (const k of ["start", "push"]) if (f[k] && (!only || only.has(k))) f[k].querySelector("input").checked = v[k];
  if ((!only || only.has("ask")) && f.ask.value !== v.ask) f.ask.value = v.ask;
}

// takeKept gives each of Connection's fields what the app keeps of the
// server s now, where the field still holds what the settings opened
// with (conn.opened): a name discovery heard or a LAN address found
// again, saved while the settings are open (takeHeardName, maybeMoveLan),
// is no change of the user's, so that it neither marks Connection nor
// is written back by Save; a field the user changed keeps the change.
function takeKept(s) {
  const f = conn.fields;
  const was = conn.opened;
  if (!f || !was || !s) return;
  const now = keptValues(s);
  const has = fieldValues(f, s);
  const take = new Set(Object.keys(now).filter((k) => now[k] !== was[k] && has[k] === was[k]));
  if (take.size) setFields(f, now, take);
  conn.opened = now;
}

// resetConnection gives Connection's fields what the app keeps of its
// server again: Cancel.
function resetConnection() {
  const s = serverById(conn.open);
  const f = conn.fields;
  if (!s || !f) return;
  conn.opened = keptValues(s);
  setFields(f, conn.opened);
  showResult(conn.panel.querySelector(".server-connection .server-msg"), "", false);
  markConnection();
}

// showSection shows one section of a server's settings, key connection,
// login or certificate (AP MobileApp, D111-A), and hides the others,
// its tab filled and selected; the draft, the focus and the lines drawn in
// place outlive a switch, every section staying built.
function showSection(panel, key) {
  for (const tab of panel.querySelectorAll(".server-tab")) {
    const on = tab.dataset.tab === key;
    tab.classList.toggle("current", on);
    tab.setAttribute("aria-selected", String(on));
  }
  for (const sec of panel.querySelectorAll(".server-section")) sec.classList.toggle("hidden", sec.dataset.section !== key);
}

// closeSettingsAsked closes the open settings as Back does (viaBack) and
// as their ✕ does, asking once where Connection's fields differ from what
// is kept (D93-A, D94-A): Connection shown first where another section is,
// the question in place of its Cancel and Save, the focus on it; Discard
// closes them, Stay keeps them, and the page's history entry, which Back
// took, is there again. A question asked already takes the focus again.
function closeSettingsAsked(viaBack) {
  const s = serverById(conn.open);
  if (!s || !conn.panel || !connectionDiffers(s)) {
    closeSettings();
    return;
  }
  showSection(conn.panel, "connection");
  if (conn.discard && conn.discard.isConnected) {
    if (viaBack) back.ask = conn.discard;
    conn.discard.focusFirst();
    return;
  }
  const actions = conn.panel.querySelector(".server-connection > .actions");
  conn.discard = confirmInPlace(actions, t("connect.discardQuestion", { name: shownName(s) }), { yes: t("connect.discard"), no: t("leave.stay") },
    () => closeSettings(), "button[type=submit]", () => {
      back.ask = null;
      markConnection(); // the fields as they stand now, put back meanwhile perhaps
      syncBack();
    });
  if (viaBack) back.ask = conn.discard;
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

// sectionSeq numbers the settings' sections, for their tabs' ids.
let sectionSeq = 0;

// settingsPanel is a server's settings (AP MobileApp, D111-A), a panel
// with a head, the server's name and a labelled ✕, which closes it as
// Back does; a row of tabs, built with the roles of a tab list; and three
// sections, all built as it opens and hidden but the one shown,
// Connection first: Connection, the form, with the name, the two
// addresses, Open at start, notifications where the app has push, when
// the fingerprint is asked, the message line, Cancel and Save, off while
// it holds what is kept, and under them, after a rule, Remove server
// with a line that says what it does, whose question warns of a key or a
// session left; Login, its session with Sign out, the login by
// fingerprint kept with its user, its key's id and Turn off, or Store
// login, and the keys and sessions left valid there, each acting at once
// (fillLogin); Certificate, the certificate in place (fillCert). The
// panel itself takes the focus (toggleSettings).
function settingsPanel(s) {
  const name = document.createElement("input");
  name.type = "text";
  name.maxLength = 64;
  name.value = s.name || "";
  const lan = addressInput(s.lan);
  const remote = addressInput(s.remote);
  const start = switchEl(t("connect.start"), !!s.start, null, "start");
  const ask = document.createElement("select");
  ask.name = "ask";
  for (const a of ASKS) {
    const o = textEl("option", t("connect.ask." + a));
    o.value = a;
    ask.appendChild(o);
  }
  ask.value = ASKS.includes(s.ask) ? s.ask : "start";
  const push = conn.bridge.push ? switchEl(t("connect.push"), s.push, null, "push") : null;
  const lineOf = () => {
    const m = textEl("p", "", "small cubes-msg server-msg hidden");
    m.setAttribute("role", "status");
    return m;
  };
  const msg = lineOf();
  const form = formEl(() => saveSettings(s.id, form, { name, lan, remote, start, ask, push, msg }), "server-connection");
  const remove = button(t("connect.removeServer"), "server-remove", () => {
    const now = serverById(s.id) || s;
    const left = keysLeft(now);
    const unended = sessionsLeft(now);
    let question = now.login ? t("connect.removeQuestionLogin", { name: s.name }) : t("connect.removeQuestion", { name: s.name });
    if (left.length) question += " " + tn("connect.removeKeysLeft", left.length, { ids: left.map((k) => k.id).join(", ") });
    if (unended.length) question += " " + tn("connect.removeSessionsLeft", unended.length, { ids: unended.map((x) => x.id).join(", ") });
    confirmInPlace(removeBox.querySelector(".actions"), question, { yes: t("connect.remove") }, () => removeServer(s.id, msg), ".server-remove");
  });
  // Remove on a line of its own under Cancel and Save, after a rule, with
  // what it does (AP MobileApp, F34); its line follows the login (fillFacts)
  const removeBox = elem("div", "server-remove-box", textEl("p", "", "small muted server-remove-hint"), elem("div", "actions", remove));
  form.append(
    fieldRow(t("connect.name"), name, { wide: true }),
    fieldRow(t("connect.lan"), lan, { wide: true, hint: t("connect.lanHint") }),
    fieldRow(t("connect.remote"), remote, { wide: true, hint: t("connect.remoteHint") }),
    fieldRow("", start, { hint: t("connect.startHint") }),
  );
  if (push) form.append(fieldRow("", push));
  form.append(fieldRow(t("connect.ask"), ask, { wide: true, hint: t("connect.askHint") }),
    msg, formFooter({ submit: t("action.save"), cancel: resetConnection }), removeBox);
  form.addEventListener("input", markConnection);
  form.addEventListener("change", markConnection);
  // Login and Certificate are no forms: Enter in a field saves Connection alone
  const login = elem("div", "server-section server-login", elem("div", "server-facts"), lineOf());
  const cert = elem("div", "server-section server-certificate");
  const panel = elem("div", "panel server-panel");
  panel.tabIndex = -1;
  panel.setAttribute("aria-label", t("connect.settingsOf", { name: s.name }));
  const close = button("×", { cls: "icon-btn card-close server-close", aria: t("action.close"), title: t("action.close") }, () => closeSettingsAsked(false));
  const head = elem("div", "server-panel-head", textEl("h3", shownName(s), "server-panel-title"), close);
  const tabs = elem("div", "server-tabs");
  tabs.setAttribute("role", "tablist");
  tabs.setAttribute("aria-label", t("connect.sections"));
  const n = ++sectionSeq;
  // a form takes no role of its own: Connection's tab panel holds it
  const connection = elem("div", "server-section", form);
  const sections = [["connection", t("connect.tab.connection"), connection], ["login", t("connect.tab.login"), login], ["certificate", t("connect.tab.certificate"), cert]];
  for (const [key, label, sec] of sections) {
    const tab = button("", { cls: "page-link server-tab" }, () => showSection(panel, key));
    tab.dataset.tab = key;
    tab.id = `server-tab-${n}-${key}`;
    tab.setAttribute("role", "tab");
    tab.setAttribute("aria-controls", `server-section-${n}-${key}`);
    tab.append(textEl("span", label, "page-link-label"));
    // Connection's mark while it differs (markConnection), Login's dot while a key or a session is left (fillLogin)
    if (key === "connection") tab.append(elem("span", "server-tab-mark"));
    if (key === "login") tab.append(elem("span", "server-tab-left hidden", elem("span", "server-tab-dot"), textEl("span", t("connect.tab.leftMark"), "visually-hidden")));
    tabs.append(tab);
    sec.id = `server-section-${n}-${key}`;
    sec.dataset.section = key;
    sec.setAttribute("role", "tabpanel");
    sec.setAttribute("aria-labelledby", tab.id);
  }
  panel.append(head, tabs, connection, login, cert);
  conn.fields = { name, lan, remote, start, ask, push };
  conn.opened = keptValues(s);
  conn.discard = null;
  showSection(panel, "connection");
  fillFacts(panel, s);
  return panel;
}

// fillRemove says under the rule what Remove does: the server leaves the
// list, and where it answers the app is signed out there, the kept
// login's key revoked first where one is kept.
function fillRemove(panel, s) {
  const hint = panel.querySelector(".server-remove-hint");
  const text = s.login ? t("connect.removeHintLogin") : t("connect.removeHint");
  if (hint && hint.textContent !== text) hint.textContent = text;
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
    start: f.start.querySelector("input").checked,
    ask: ASKS.includes(f.ask.value) ? f.ask.value : "start",
    push: f.push ? f.push.querySelector("input").checked : !!s.push,
  });
  try {
    await whileBusy(form.querySelector("button[type=submit]"), async () => {
      keepServer(await conn.bridge.save(want));
      // Open at start on here is off at every other server
      if (want.start && !s.start) await loadServers();
    });
  } catch (e) {
    return showResult(f.msg, t("connect.saveFailed", { error: e.message }), false);
  }
  settingsClosed();
  say(t("connect.saved", { name }));
  probeServer(serverById(id));
}

// MAX_KEY_ID is the largest id a key left may have: 15 digits, as My
// profile's address takes it (ui/src/profile/index.tsx).
const MAX_KEY_ID = 999999999999999;

// keysLeft are the keys of the phone's that Turn off or Forget all could
// not revoke at a server, each {user, id} (`unrevoked`, contract version
// 4); none from an app before it.
function keysLeft(s) {
  const good = (k) => k && Number.isInteger(k.id) && k.id > 0 && k.id <= MAX_KEY_ID && typeof k.user === "string";
  return s && Array.isArray(s.unrevoked) ? s.unrevoked.filter(good) : [];
}

// sessionsLeft are the sessions the app forgot at a server without the
// server ending them, each {user, id} (`unended`, contract version 5),
// which the next session there for that user revokes; none from an app
// before it.
function sessionsLeft(s) {
  const good = (x) => x && Number.isInteger(x.id) && x.id > 0 && x.id <= MAX_KEY_ID && typeof x.user === "string";
  return s && Array.isArray(s.unended) ? s.unended.filter(good) : [];
}

// hasPending reports whether a key or a session left at a server waits for
// its revoke code (`pending`, contract version 6; AP MobileApp, D112-A),
// which the app posts at the next contact.
function hasPending(s) {
  return keysLeft(s).some((k) => k.pending === true) || sessionsLeft(s).some((x) => x.pending === true);
}

// storesLogin reports whether a server's settings offer Store login (AP
// MobileApp, Block 19): the app holds a session there and keeps no login,
// and its bridge has keep (contract version 7); where the phone has no
// strong biometric enrolled (about's can) it is shown, off, its title
// saying so (showsKeys).
function storesLogin(s) {
  return can("keep") && !!s.session && !s.login;
}

// fillFacts writes what a server's open settings say of it as it stands:
// what Remove does (fillRemove), the Login section (fillLogin), the
// Certificate section (fillCert), the name in the head, what Connection's
// untouched fields take of what is kept now (takeKept), and whether
// Connection differs from it (markConnection).
function fillFacts(panel, s) {
  fillRemove(panel, s);
  fillLogin(panel, s);
  fillCert(panel, s);
  const title = panel.querySelector(".server-panel-title");
  if (title && title.textContent !== shownName(s)) title.textContent = shownName(s);
  if (panel !== conn.panel) return;
  takeKept(s);
  markConnection();
}

// fillLogin writes the Login section of a server's open settings (AP
// MobileApp, D111-A): the session the app holds there, with Sign out,
// which says while the server does not answer that Sign out ends it there
// the next time the app reaches the server (F42, D105-A), the login by
// fingerprint kept there, with its user, its key's id and Turn off, or,
// where the app holds a session and keeps no login, Store login,
// each key of the phone's left valid there, with Show in My profile,
// which opens the server's page at My profile with that key marked, off
// while the server does not answer, its title saying why (AP MobileApp,
// F33), and each session of the phone's left valid there, a key or a
// session whose revoke code waits saying that it ends the next time the app
// reaches the server (`pending`, contract version 6), and a line where
// there is none of these; each acts at once, and none touches
// Connection's draft. While a key or a session is left there, Login's tab
// carries a dot, named for a screen reader. Drawn again only
// where it changed, whether the server is shown down among it (shownDown,
// which a probe still running leaves as it was), which over plain HTTP
// nothing else in it tells, so that a probe that answers keeps the focus,
// the buttons Show in My profile and Store login turned on and off in
// place.
function fillLogin(panel, s) {
  const box = panel.querySelector(".server-login .server-facts");
  if (!box) return;
  const p = conn.probes[s.id] || {};
  const left = keysLeft(s);
  const unended = sessionsLeft(s);
  const down = shownDown(p);
  const msg = () => panel.querySelector(".server-login .server-msg");
  panel.querySelector(".server-tab-left").classList.toggle("hidden", !left.length && !unended.length);
  const key = JSON.stringify([s.session || "", s.login || "", s.login_key || 0, storesLogin(s), down, left, unended]);
  if (box.dataset.key === key) {
    showsKeys(box, p);
    return;
  }
  box.dataset.key = key;
  clear(box);
  if (s.session) {
    const out = button(t("connect.signOut"), "server-sign-out", () => signOut(s.id, out, msg()));
    const line = down ? t("connect.sessionDown", { name: s.session }) : t("connect.sessionLine", { name: s.session });
    box.appendChild(elem("p", "server-fact server-session", line, " ", out));
  }
  if (s.login) {
    const off = button(t("connect.loginOff"), "server-login-off", () => turnOff(s.id, off, msg()));
    // the kept key's id whenever the app knows it (login_key, contract version 5; AP MobileApp, D66-A)
    const id = Number.isInteger(s.login_key) && s.login_key > 0 && s.login_key <= MAX_KEY_ID ? s.login_key : 0;
    const text = id ? t("connect.loginKeptKey", { name: s.login, id }) : t("connect.loginKept", { name: s.login });
    box.appendChild(elem("p", "server-login-kept", text, " ", off));
  }
  if (storesLogin(s)) {
    const store = button(t("connect.storeLogin"), "server-store-login", () => storeLogin(s.id, store, msg()));
    box.appendChild(elem("p", "server-fact server-store", store));
  }
  for (const k of left) {
    const show = button(t("connect.keyLeftShow"), { cls: "server-key-show", aria: t("connect.keyLeftShowLabel", { id: k.id }) },
      () => showInProfile(s.id, k.id, show));
    const text = k.pending === true ? t("connect.keyPending", { id: k.id, name: k.user }) : t("connect.keyLeft", { id: k.id, name: k.user });
    box.appendChild(elem("p", "server-fact server-key-left", text, " ", show));
  }
  for (const x of unended) {
    const text = x.pending === true ? t("connect.sessionPending", { id: x.id, name: x.user }) : t("connect.sessionLeft", { id: x.id, name: x.user });
    box.appendChild(elem("p", "server-fact server-session-left", text));
  }
  if (!box.firstChild) box.appendChild(textEl("p", t("connect.loginNone"), "muted server-fact server-login-none"));
  showsKeys(box, p);
}

// showsKeys turns each Show in My profile of the open settings on where
// the server answers, else off with its title saying why: the page it
// opens is the server's (AP MobileApp, F33); and Store login so, which is
// also off where the phone has no strong biometric enrolled (conn.can)
// and at an address of plain HTTP outside the home network, where the
// app sends no kept key (D36-A), its title saying what it does where it
// is on.
function showsKeys(box, p) {
  const answers = p.state === "lan" || p.state === "remote";
  const why = answers ? "" : t("connect.keyLeftShowOff");
  for (const b of box.querySelectorAll(".server-key-show")) {
    setOff(b, !answers);
    if (b.title !== why) b.title = why;
  }
  const store = box.querySelector(".server-store-login");
  if (store) {
    const plain = answers && plainOutside(p.url);
    let title = t("connect.storeLoginTitle");
    if (!conn.can) title = t("connect.storeLoginNoBiometric");
    else if (!answers) title = why;
    else if (plain) title = t("connect.storeLoginPlain");
    setOff(store, !conn.can || !answers || plain);
    if (store.title !== title) store.title = title;
  }
}

// probed waits until the latest probe of the server id has answered: a
// newer probe replaces one still running (probeSeq), whose answer then
// counts for nothing; 10 s at most, a probe taking 4 s at an address.
async function probed(id) {
  for (let i = 0; i < 200 && (conn.probes[id] || {}).state === "probing"; i++) await new Promise((resolve) => setTimeout(resolve, 50));
}

// showInProfile opens a server's page at My profile with the key keyId
// marked (`#profile?key=<id>`, contract version 4), once a probe has found
// the server now, the latest where a newer one replaced its own (probed)
// (AP MobileApp, D108-A, F48): the list's last probe may be older than
// the server's fall. Where it does not answer, the toast says
// so with the probe's reason, the button off with its title (showsKeys);
// a certificate that changed asks first, as a tap does (openServer).
async function showInProfile(id, keyId, btn) {
  await whileBusy(btn, async () => {
    await probeServer(serverById(id));
    await probed(id);
  });
  const s = serverById(id);
  const q = conn.probes[id] || {};
  if (!s) return;
  if (q.state === "lan" || q.state === "remote" || q.state === "cert") return openServer(id, null, "#profile?key=" + keyId);
  say(t("connect.unreachable", { name: shownName(s) }) + (q.error ? " " + q.error : ""), { bad: true, server: id });
}

// storeLogin keeps the login by fingerprint for the session the app holds
// at a server (keep, contract version 7; AP MobileApp, Block 19), at the
// address the probe found: the app makes a key of the session's user with
// it and asks for the fingerprint once, as a server's page's sign-in that
// keeps the login does; a prompt not passed revokes the key again. The
// list read again shows the login kept; the toast says how it went, and a
// failure is said in the settings.
async function storeLogin(id, btn, msg) {
  const s = serverById(id);
  const p = conn.probes[id] || {};
  if (!s || !(p.state === "lan" || p.state === "remote")) return;
  let kept;
  try {
    kept = await whileBusy(btn, () => conn.bridge.keep(id, p.url));
  } catch (e) {
    showResult(msg, t("login.keepError", { error: e.message }), false);
    return;
  }
  if (kept === undefined) return; // a second tap while the first runs
  await loadServers();
  renderServers();
  if (kept === true) say(t("connect.storeLoginDone", { name: shownName(s) }));
  else say(t("login.keepFailed"), { bad: true });
}

// signOut signs the app out of a server, at the address the probe chose
// (signOut, docs/app.md, the session ended there and forgotten in the
// app); the open settings lose the session's line, show it left where the
// server did not end it (the list read again: its unended, contract
// version 5) and keep what is typed in them, and the toast says whether
// the server ended it, and where it did not whether its revoke code ends
// it the next time the app reaches the server (`pending`, version 6); a
// server that did not end it is probed again (AP MobileApp, D108-A).
async function signOut(id, btn, msg) {
  const s = serverById(id);
  const p = conn.probes[id] || {};
  const url = p.state === "lan" || p.state === "remote" ? p.url : "";
  let res;
  try {
    res = await whileBusy(btn, () => conn.bridge.signOut(id, url));
  } catch (e) {
    showResult(msg, t("connect.signOutFailed", { error: e.message }), false);
    return;
  }
  const name = s ? s.name : "";
  if (s) s.session = null;
  await loadServers();
  renderServers();
  if (res && res.ended) say(t("connect.signedOut", { name }));
  else if (res && res.pending === true) say(t("connect.signedOutPending", { name }));
  else say(t("connect.signedOutLocal", { name }));
  // the server did not answer: probed again, so that what needs it turns off at once (D108-A)
  if (!(res && res.ended)) probeServer(serverById(id));
}

// signOutAll signs the app out of every server (signOutAll), from the ☰.
async function signOutAll() {
  try {
    await conn.bridge.signOutAll();
  } catch (e) {
    say(t("connect.signOutFailed", { error: e.message }), { bad: true });
    return;
  }
  for (const s of conn.servers) s.session = null;
  await loadServers();
  renderServers();
  say(t("connect.signedOutAll"));
}

// fmtWhen is a time the app gives (RFC 3339) as a day and the time of
// day in the page's language. Anything else stays as it is.
function fmtWhen(iso) {
  const d = new Date(iso);
  return isNaN(d) ? String(iso || "") : new Intl.DateTimeFormat(i18n.lang, { dateStyle: "medium", timeStyle: "short" }).format(d);
}

// fillCert writes the Certificate section of a server's open settings
// (AP MobileApp, D111-A), what the certificate's dialog showed, in place
// (tpl-cert): until a probe has answered, that the certificate shows once
// the server answers; where the certificate changed, the one trusted and
// the new one; where the server answers over HTTPS, the certificate the
// probe found at the address in use, its subject, issuer, names, validity
// and fingerprint, and since when it is trusted: since the user compared
// its fingerprint (trust), or by the phone, a certificate authority
// having signed it; where it does not answer, the fingerprint pinned and
// since when (connect.certPinned), or where none is pinned that the
// certificate shows once it answers (connect.certWaiting); over plain
// HTTP, that there is none (connect.certPlain). Drawn again only where it
// changed; a probe still running keeps what the last one found.
function fillCert(panel, s) {
  const box = panel.querySelector(".server-certificate");
  if (!box) return;
  const p = conn.probes[s.id] || {};
  let st = p.state;
  if (st === undefined || st === "probing") {
    if (box.dataset.key) return;
    // the settings open while a probe runs: what the last one found, where it is not lost with it, else waiting
    st = p.was === "offline" ? "offline" : "probing";
  }
  const answers = st === "lan" || st === "remote";
  const c = answers ? p.cert : null;
  const plain = (answers && !c) || (!answers && !s.fp && [s.lan, s.remote].filter(Boolean).every((u) => u.startsWith("http:")));
  let kind = c ? "cert" : plain ? "plain" : s.fp ? "pinned" : "waiting";
  if (st === "probing" && kind !== "plain") kind = "waiting"; // plain HTTP alone, nothing pinned: none, whatever the probe finds
  if (st === "cert") kind = "changed";
  const key = JSON.stringify([kind, c, answers ? p.url : "", s.fp || "", s.trusted_at || "", kind === "changed" ? p.fingerprint : ""]);
  if (box.dataset.key === key) return;
  box.dataset.key = key;
  clear(box);
  if (kind === "changed") {
    // the server shows another certificate than the one trusted: both, the trusted one first
    const row = (label, fp, cls) => {
      const v = textEl("span", "", "mono tls-fp " + cls);
      showFingerprint(v, fp);
      return elem("div", "fp-row", textEl("span", label, "muted small"), v);
    };
    box.appendChild(textEl("p", t("connect.fpChangedTitle"), "server-fact warn-text server-cert-changed"));
    if (s.fp) box.appendChild(row(t("connect.fpOld"), s.fp, "cert-fp-old"));
    box.appendChild(row(t("connect.fpNew"), p.fingerprint || "", "cert-fp-new"));
    return;
  }
  if (kind === "plain") {
    box.appendChild(textEl("p", t("connect.certPlain"), "muted server-fact server-cert-none"));
    return;
  }
  if (kind === "waiting") {
    box.appendChild(textEl("p", t("connect.certWaiting"), "muted server-fact server-cert-none"));
    return;
  }
  const facts = fromTemplate("tpl-cert");
  const set = (cls, text) => {
    facts.querySelector("." + cls).textContent = text;
  };
  const sinceTrusted = () => (s.trusted_at ? t("connect.cert.trustedSince", { when: fmtWhen(s.trusted_at) }) : t("connect.cert.trustedPinned"));
  if (kind === "pinned") {
    // the server does not answer: the fingerprint pinned, and since when
    for (const row of facts.querySelectorAll(".cert-of-probe")) row.remove();
    facts.prepend(textEl("p", t("connect.certPinned"), "muted small server-cert-pinned"));
    showFingerprint(facts.querySelector(".cert-fp"), s.fp);
    set("cert-trusted", sinceTrusted());
  } else {
    const none = "—";
    set("cert-address", hostOf(p.url));
    set("cert-subject", c.subject || none);
    set("cert-issuer", c.self_signed ? t("connect.cert.selfSigned") : c.issuer || none);
    set("cert-names", c.names.length ? c.names.join(", ") : none);
    set("cert-from", c.not_before ? fmtWhen(c.not_before) : none);
    set("cert-until", c.not_after ? fmtWhen(c.not_after) : none);
    showFingerprint(facts.querySelector(".cert-fp"), c.fingerprint);
    const pinned = s.fp && s.fp.toUpperCase() === c.fingerprint.toUpperCase();
    set("cert-trusted", pinned ? sinceTrusted() : t("connect.cert.trustedSystem"));
  }
  box.appendChild(facts);
}

// turnOff turns a server's login by fingerprint off (forget, which
// revokes the key at the address the probe chose, with the session the
// app holds there or with the key itself after the fingerprint, and
// signs the app out there); the open settings lose its line and the
// session's, drawn again from the list read again (fillFacts), and keep
// what is typed in them, and the toast says whether
// the server revoked the key, or whether this Turn off left it, to revoke
// at the next sign-in there (`left`, contract version 4), or by its revoke
// code the next time the app reaches the server (`pending`, version 6); a
// server that did not revoke it is probed again (AP MobileApp, D108-A).
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
  renderServers();
  const name = s ? s.name : "";
  if (res && res.revoked) say(t("connect.loginOffDone", { name }));
  else if (res && res.pending === true) say(t("connect.loginOffPending", { name }));
  else if (res && res.left === true) say(t("connect.loginOffLeft", { name }));
  else say(t("connect.loginOffLocal", { name }));
  // the server did not revoke the key: probed again (D108-A)
  if (!(res && res.revoked)) probeServer(serverById(id));
}

// menuButton is the summary of the list's ☰, in the header, where the
// focus goes back to once what it opened is done.
function menuButton() {
  return $("servers-menu").querySelector("summary");
}

// askInCard asks in the Servers card, under the list, what an entry of
// the ☰ asks before it acts: No and Escape take the question back, Yes
// runs onYes (which returns its promise, see confirmBox) and then takes
// it back; the focus goes back to the ☰ either way. The toast of an
// earlier action goes, so that it covers no part of the question on a
// phone.
function askInCard(text, yes, onYes) {
  conn.toastServer = null;
  hideToast();
  const spot = $("servers-ask");
  const done = () => {
    clear(spot);
    spot.classList.add("hidden");
    menuButton().focus();
  };
  const box = confirmBox(text, { yes }, async () => {
    try {
      await onYes();
    } finally {
      done();
    }
  }, done);
  clear(spot);
  spot.appendChild(box);
  spot.classList.remove("hidden");
  box.focusFirst();
  box.scrollIntoView({ block: "nearest" });
}

// forgetAll turns every kept login off, from the ☰, once the user
// confirmed it in the card: each key revoked at its server as Turn off
// revokes it and the app signed out there, the fingerprint asked once per
// server where the app holds no session there, then forgotten on the phone
// (forgetAll, contract version 4); the toast names the servers that keep a
// key: left, which the app revokes at its next sign-in there as the same
// user, or forgotten on the phone alone. An app before version 4 answers
// without them, and forgot on the phone alone.
function forgetAll() {
  askInCard(t("connect.forgetAllQuestion"), t("connect.forgetAll"), async () => {
    const names = new Map(conn.servers.map((s) => [s.id, s.name || hostOf(s.lan || s.remote)]));
    const named = (ids) => ids.map((id) => names.get(id) || id).join(", ");
    let res;
    try {
      res = await conn.bridge.forgetAll();
    } catch (e) {
      // some may be off already: the list as it stands
      await loadServers();
      renderServers();
      say(t("connect.loginOffFailed", { error: e.message }), { bad: true });
      return;
    }
    await loadServers();
    renderServers();
    if (!res || !Array.isArray(res.revoked) || !Array.isArray(res.left) || !Array.isArray(res.local)) {
      say(t("connect.forgotAllPhone"));
      return;
    }
    if (!res.left.length && !res.local.length) {
      say(t("connect.forgotAll"));
      return;
    }
    let text = t("connect.forgotAllPhone");
    if (res.left.length) text += " " + tn("connect.forgotKeysLeft", res.left.length, { names: named(res.left) });
    if (res.local.length) text += " " + tn("connect.forgotKeysLocal", res.local.length, { names: named(res.local) });
    say(text);
  });
}

// removeServer forgets a server, its certificate, its session and its
// kept login, the login turned off first as Turn off does, else the session
// signed out, at the address the probe chose (remove, contract version 5),
// and says so in the toast: the login's key revoked, the ids of the phone's
// keys and sessions still valid there, whether their revoke codes end them
// the next time the app reaches the server (`pending`, contract version 6),
// and a key listed at the server alone, to revoke under My profile. An
// error is said in its settings, which stay open, the list read again for
// what the app did before it. The focus goes to the Add card where the
// list is empty now, else to the ☰.
async function removeServer(id, msg) {
  const s = serverById(id);
  const p = conn.probes[id] || {};
  const url = p.state === "lan" || p.state === "remote" ? p.url : "";
  let res;
  try {
    res = await conn.bridge.remove(id, url);
  } catch (e) {
    showResult(msg, t("connect.removeFailed", { error: e.message }), false);
    await loadServers();
    renderServers();
    probeServer(serverById(id)); // the server stays, perhaps gone since the last probe (D108-A)
    return;
  }
  conn.servers = conn.servers.filter((x) => x.id !== id);
  delete conn.probes[id];
  settingsClosed();
  renderServers();
  const name = s ? s.name || hostOf(s.lan || s.remote) : "";
  const ids = (list) => (Array.isArray(list) ? list.filter((k) => Number.isInteger(k) && k > 0 && k <= MAX_KEY_ID) : []);
  const keys = ids(res && res.keys);
  const unended = ids(res && res.sessions);
  let text = res && res.revoked === true ? t("connect.removedRevoked", { name }) : t("connect.removed", { name });
  if (keys.length) text += " " + tn("connect.removedKeys", keys.length, { ids: keys.join(", ") });
  if (unended.length) text += " " + tn("connect.removedSessions", unended.length, { ids: unended.join(", ") });
  if (res && res.pending === true) text += " " + t("connect.removedPending");
  if (res && res.local === true) text += " " + t("connect.removedKeyListed");
  say(text);
  if (conn.servers.length) menuButton().focus();
  else $("add").focus();
}

// --- the Add card and App settings --------------------------------------

// renderAdd shows the Add card while the list is empty, or once the ☰
// chose it, with × where the list is not empty.
function renderAdd() {
  const empty = conn.servers.length === 0;
  $("add").classList.toggle("hidden", !(empty || conn.adding));
  $("add-close").classList.toggle("hidden", empty);
}

// openAdd shows the Add card from the ☰ and starts its search; the card
// takes the focus, not its address field, so that a phone's keyboard
// stays closed until the field is touched.
function openAdd() {
  conn.adding = true;
  panelShown("add", true);
  renderAdd();
  const card = $("add");
  card.focus({ preventScroll: true });
  card.scrollIntoView({ block: "nearest" });
  search();
}

// closeAdd hides the Add card where the list is not empty: its ×, and a
// server added. With refocus the focus goes back to the ☰.
function closeAdd(refocus) {
  conn.adding = false;
  showResult($("add-msg"), "", false);
  panelShown("add", false);
  renderAdd();
  if (refocus && conn.servers.length) menuButton().focus();
}

// applyAppearance gives the page the app's appearance, as theme.js gives
// the web UI its mode: data-theme on <html>, and the browser's own
// controls with it; the app gives the same to every server's page set to
// "as the system", through the phone's mode it sets for itself.
function applyAppearance(a) {
  const root = document.documentElement;
  if (a === "light" || a === "dark") {
    root.dataset.theme = a;
    root.style.colorScheme = a;
  } else {
    root.removeAttribute("data-theme");
    root.style.colorScheme = "";
  }
}

// openApp shows App settings, from the ☰, and notes them in the page's
// address: the app loads the page again when it takes another
// appearance, and the page opens them again (initApp).
function openApp() {
  const card = $("app-card");
  card.classList.remove("hidden");
  history.replaceState(history.state, "", location.pathname + location.search + APP_HASH);
  panelShown("app", true);
  card.focus({ preventScroll: true });
  card.scrollIntoView({ block: "nearest" });
}

function closeApp() {
  $("app-card").classList.add("hidden");
  showResult($("app-msg"), "", false);
  history.replaceState(history.state, "", location.pathname + location.search);
  panelShown("app", false);
  menuButton().focus();
}

// saveAppearance keeps the appearance chosen (saveSettings) and gives it
// to the page; one the app refused is said in the card, the choice as
// it was.
async function saveAppearance() {
  const sel = $("app-appearance");
  const want = Object.assign({}, conn.app, { appearance: sel.value });
  let saved;
  try {
    saved = await whileBusy(sel, () => conn.bridge.saveSettings(want));
  } catch (e) {
    sel.value = conn.app.appearance;
    showResult($("app-msg"), t("connect.saveFailed", { error: e.message }), false);
    return;
  }
  conn.app = Object.assign({ appearance: "system" }, saved || want);
  applyAppearance(conn.app.appearance);
  showResult($("app-msg"), "", true);
}

// initApp reads the app's settings where it has them and gives the page
// its appearance; App settings open again where the address says they
// were open when the app loaded the page anew.
async function initApp() {
  if (!can("settings")) return;
  try {
    conn.app = Object.assign({ appearance: "system" }, (await conn.bridge.settings()) || {});
  } catch (e) {
    conn.app = { appearance: "system" };
  }
  if (!APPEARANCES.includes(conn.app.appearance)) conn.app.appearance = "system";
  $("app-appearance").value = conn.app.appearance;
  applyAppearance(conn.app.appearance);
  if (location.hash === APP_HASH) openApp();
}

// initMenu shows the list's ☰ in the header, makes it a dropdown of the
// page (dropdown, components.js) and wires its entries.
function initMenu() {
  const menu = $("servers-menu");
  menu.classList.remove("hidden");
  dropdown(menu);
  menuArrows(menu, ".menu-items button:not(.hidden)");
  $("menu-add").addEventListener("click", openAdd);
  $("menu-sign-out-all").addEventListener("click", signOutAll);
  $("menu-forget-all").addEventListener("click", forgetAll);
  $("menu-app").addEventListener("click", openApp);
  $("menu-exit").addEventListener("click", exitApp);
  $("add-close").addEventListener("click", () => closeAdd(true));
  $("app-close").addEventListener("click", closeApp);
  $("app-appearance").addEventListener("change", saveAppearance);
}

// initVersion fills the ☰'s first row where the bridge gives the app's
// version (about, contract version 7), and notes whether the phone has a
// strong biometric enrolled (its can, conn.can): Massimo's mark, the version and
// its copy icon; a tap on either copies the whole version (copied, as
// every copy goes) and closes the ☰, the focus back on its summary. No
// row where the bridge has no about, or it gives no version.
async function initVersion() {
  if (!can("about")) return;
  let v = "";
  try {
    const a = (await conn.bridge.about()) || {};
    v = typeof a.version === "string" ? a.version.trim() : "";
    conn.can = a.can !== false;
  } catch (e) {
    return;
  }
  if (!v) return;
  const text = $("menu-version");
  const name = t("copy.what", { what: t("connect.version", { version: v }) });
  text.textContent = v;
  copyable(text, () => v, name);
  const icon = copyIcon(() => v, name);
  $("menu-version-row").appendChild(icon);
  // after the copy's own listener, which keeps the click from the menu's
  const close = () =>
    setTimeout(() => {
      $("servers-menu").open = false;
      menuButton().focus();
    }, 0);
  for (const el of [text, icon]) {
    el.addEventListener("click", close);
    el.addEventListener("keydown", (ev) => {
      if (ev.key === "Enter" || ev.key === " ") close();
    });
  }
  $("menu-version-row").classList.remove("hidden");
  $("menu-version-sep").classList.remove("hidden");
}

// askCan reads again whether the phone has a strong biometric enrolled
// (about's can, contract version 7), as a server's settings open, since
// the user may have enrolled one meanwhile: Store login follows.
async function askCan() {
  if (!can("about")) return;
  let a;
  try {
    a = (await conn.bridge.about()) || {};
  } catch (e) {
    return;
  }
  const c = a.can !== false;
  if (c === conn.can) return;
  conn.can = c;
  renderServers();
}

// exitApp ends the app as a swipe from the recent apps does (exit,
// contract version 7; AP MobileApp, D91-A): the sessions held in memory
// dropped, each one recorded as held left to its revoke code at the next
// start's contact, nothing sent to a server, the app out of the recent
// apps.
async function exitApp() {
  try {
    await conn.bridge.exit();
  } catch (e) {
    say(e.message, { bad: true });
  }
}

// --- Back -------------------------------------------------------------

// panelShown notes that a panel ("settings", "add" or "app") was opened,
// the last opened at the end, or closed, and brings the page's history
// entry up to date (syncBack).
function panelShown(name, on) {
  conn.panels = conn.panels.filter((x) => x !== name);
  if (on) conn.panels.push(name);
  syncBack();
}

// counts reports whether a panel open counts for Back: the Add card only
// where the list is not empty, without which it is always shown.
function counts(name) {
  return name !== "add" || conn.servers.length > 0;
}

// anyOpen reports whether a menu or a panel is open, which Back closes.
function anyOpen() {
  return $("servers-menu").open || $("fp-dialog").open || conn.panels.some(counts);
}

// syncBack brings the page's own history entry up to date (back): one
// pushed, the address unchanged, as a menu or a panel opens, none while
// Back's question is asked, and taken back as the last closes by other
// means than Back, its popstate then the page's own (onPopState); from
// BACK_READY_MS after the page's load on, while no take-back runs, and
// once the user has touched the page: an entry a page adds before that is
// one the WebView's Back passes over, as Chromium's history intervention
// marks it (a panel opened at the start by the app's handover), so it
// waits for the first touch (awaitTouch).
function syncBack() {
  if (!back.ready || back.leaving) return;
  const want = anyOpen() && !(back.ask && back.ask.isConnected);
  if (want && !back.entry) {
    if (navigator.userActivation && !navigator.userActivation.hasBeenActive) {
      awaitTouch();
      return;
    }
    history.pushState({ massimo: "open" }, "");
    back.entry = true;
  } else if (!want && back.entry) {
    back.entry = false;
    back.leaving = true;
    back.timer = setTimeout(endLeaving, BACK_MS);
    history.back();
  }
}

// awaitTouch has syncBack run again after the user's first touch or key,
// once (back.touch): the page has been active then.
function awaitTouch() {
  if (back.touch) return;
  back.touch = true;
  const again = () => {
    if (!navigator.userActivation.hasBeenActive) return;
    for (const type of ["pointerup", "click", "keydown"]) document.removeEventListener(type, later, true);
    syncBack();
  };
  const later = () => setTimeout(again, 0);
  for (const type of ["pointerup", "click", "keydown"]) document.addEventListener(type, later, true);
}

// endLeaving ends a take-back of the page's history entry: at its
// popstate, or after BACK_MS where none came.
function endLeaving() {
  clearTimeout(back.timer);
  back.timer = null;
  back.leaving = false;
  syncBack();
}

// keepHash gives the entry Back reached the page's fragment as it
// stands: App settings' where they are open (openApp), else none.
function keepHash() {
  const want = location.pathname + location.search + ($("app-card").classList.contains("hidden") ? "" : APP_HASH);
  if (location.pathname + location.search + location.hash !== want) history.replaceState(history.state, "", want);
}

// onPopState takes Back (the app's Back, history.back() in a browser):
// the end of a take-back of the page's own, or Back, which closes the
// fingerprint sheet first, as its Cancel does (askTrust resolves false),
// then the open menu, then the panel opened last (closeTop), a server's
// settings with changes asking first; the entry is there again where
// something is still open.
function onPopState() {
  if (back.leaving) {
    keepHash();
    endLeaving();
    return;
  }
  if (!back.entry) return;
  back.entry = false;
  keepHash();
  if ($("fp-dialog").open) $("fp-dialog").close();
  else closeTop();
  syncBack();
}

// closeTop closes what Back closes: the open menu, the focus back on its
// summary; else the panel opened last, the focus where its own close puts
// it.
function closeTop() {
  const menu = $("servers-menu");
  if (menu.open) {
    menu.open = false;
    menuButton().focus();
    return;
  }
  const top = conn.panels.filter(counts).pop();
  if (top === "settings" && !conn.open) panelShown("settings", false); // gone by other means: nothing to close
  else if (top === "settings") closeSettingsAsked(true);
  else if (top === "add") closeAdd(true);
  else if (top === "app") closeApp();
}

// initBack wires Back: the popstate of the page's entries, and the menu's
// opening and closing, which push and take back its entry.
function initBack() {
  window.addEventListener("popstate", onPopState);
  $("servers-menu").addEventListener("toggle", syncBack);
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
// until now, whose change it warns of. While it shows, the page has its
// history entry, and Back closes it as Cancel does.
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
      syncBack();
    };
    dlg.showModal();
    $("fp-cancel").focus();
    syncBack(); // Back closes the sheet as its Cancel does (onPopState)
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
// hears it (takeHeardName), opened at start where it is the first; one
// in the list by its node takes the address. The Add card closes, and
// the server's row takes the focus.
async function saveNew({ name, url, node, fp }) {
  const where = isPrivateHost(new URL(url).hostname) ? "lan" : "remote";
  const known = node ? conn.servers.find((s) => s.node === node) : null;
  const heard = node ? conn.heard[node] : "";
  let saved;
  try {
    if (known) {
      saved = await conn.bridge.save(Object.assign({}, known, { [where]: url }));
    } else {
      saved = await conn.bridge.save({ name: name || heard || hostOf(url), node, lan: where === "lan" ? url : "", remote: where === "remote" ? url : "", fp: "", ask: "start", push: false, start: conn.servers.length === 0 });
    }
    if (fp && fp !== saved.fp) await conn.bridge.trust(saved.id, fp);
  } catch (e) {
    say(t("connect.saveFailed", { error: e.message }), { bad: true });
    return false;
  }
  await loadServers();
  conn.found = conn.found.filter((f) => !(node && f.node === node) && f.url !== url);
  renderFound();
  closeAdd(false);
  renderServers();
  say(known ? t("connect.known", { name: saved.name }) : t("connect.added", { name: saved.name }));
  const open = document.querySelector(`.server-row[data-id="${CSS.escape(saved.id)}"] .server-open`);
  if (open) open.focus();
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

// unreachableFromAddress takes the fragment #unreachable=<id> from the
// page's address, with which the app loads the page where that server's
// page failed to load (contract version 7; AP MobileApp, D108-A), and gives
// the id where it is the list's, else null.
function unreachableFromAddress() {
  const m = UNREACHABLE_HASH.exec(location.hash);
  if (!m) return null;
  history.replaceState(history.state, "", location.pathname + location.search);
  return serverById(m[1]) ? m[1] : null;
}

// sayUnreachable says in the toast, once the probe at the page's start has
// run, and a newer one that replaced it (probed), that the server id whose
// page failed to load does not answer, with
// the probe's reason, as a tap's failed probe does (openServer), the toast
// tied to the server, so that a probe that finds it takes the toast back;
// nothing where it answers now.
async function sayUnreachable(id) {
  await probed(id);
  const s = serverById(id);
  const q = conn.probes[id] || {};
  if (!s || q.state === "lan" || q.state === "remote") return;
  say(t("connect.unreachable", { name: shownName(s) }) + (q.error ? " " + q.error : ""), { bad: true, server: id });
}

// openFromAddress opens the settings of the server the page's address
// names, #settings=<id>, as the app loads it from that server's ☰ (AP
// MobileApp, D73-A), and takes the fragment from the address; an id that
// is none of the list's opens nothing.
function openFromAddress() {
  const m = SETTINGS_HASH.exec(location.hash);
  if (!m) return;
  history.replaceState(null, "", location.pathname + location.search);
  if (!serverById(m[1])) return;
  toggleSettings(m[1]);
  if (conn.panel) conn.panel.scrollIntoView({ block: "nearest" });
}

// --- start ----------------------------------------------------------------

// onBridgeEvent takes the app's events; an unknown type is ignored. The
// list pulled down (refresh, contract version 7) checks every server again
// as ↻ does, ↻ turning meanwhile.
function onBridgeEvent(ev) {
  const d = (ev && ev.detail) || {};
  if (d.type === "found") onFound(d.service);
  else if (d.type === "network") probeAll();
  else if (d.type === "refresh") whileBusy($("probe-btn"), probeAll);
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
  window.addEventListener("massimo", onBridgeEvent);
  $("add-form").addEventListener("submit", (ev) => {
    ev.preventDefault();
    addAddress();
  });
  $("add-address").addEventListener("input", warnPlain);
  $("probe-btn").addEventListener("click", (ev) => whileBusy(ev.currentTarget, probeAll));
  $("search-btn").addEventListener("click", search);
  initMenu();
  initBack();
  initVersion();
  await loadServers();
  renderServers();
  openFromAddress();
  const down = unreachableFromAddress();
  probeAll().then(() => down && sayUnreachable(down));
  // at the start for the saved servers too: a LAN address that changed,
  // and the names discovery hears (onFound)
  search();
  initApp();
  initCore();
}

document.addEventListener("DOMContentLoaded", init);
// the page's history entries from BACK_READY_MS after its load on (syncBack)
window.addEventListener("load", () =>
  setTimeout(() => {
    back.ready = true;
    if (conn.bridge) syncBack();
  }, BACK_READY_MS),
);
