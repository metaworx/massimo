// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

// The Massimo apps' connect page (connect.html, docs/app.md): the
// servers the app keeps, each probed at its LAN and its remote address,
// marked where the app holds a session, and opened in the app by a tap,
// the app asking for the fingerprint first where a login is kept;
// the list's ☰ (Add server…, Sign out of all, Forget all stored
// credentials, App settings…); the servers discovery finds and an address
// typed, probed before they are added, in a card shown while the list is
// empty or from the ☰; the fingerprint sheet of a self-signed
// certificate; a server's settings with Open at start, its session, its
// certificate, its login by fingerprint and the keys and sessions of the
// phone's left valid there, and the row's sign-in icon that opens its
// sign-in card; the app's
// own settings, its appearance; a server's settings or App settings open at
// the start where a server's ☰ asked for them; the server on the phone where the app has
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
  const name = s.name || hostOf(s.lan || s.remote);
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
// is kept, App settings where the app has them.
function renderMenu() {
  const any = conn.servers.length > 0;
  $("menu-sign-out-all").classList.toggle("hidden", !(any && can("signOutAll")));
  $("menu-forget-all").classList.toggle("hidden", !conn.servers.some((s) => s.login));
  $("menu-app").classList.toggle("hidden", !can("settings"));
  $("menu-app-sep").classList.toggle("hidden", !can("settings"));
}

// renderServers brings the list up to date row by row, so that a probe
// that answers keeps the focus and what is typed in the settings; then
// the ☰ and the Add card.
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
  renderMenu();
  renderAdd();
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
      say(t("connect.openFailed", { name: s.name, error: e.message }), { bad: true });
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
  say(t("connect.unreachable", { name: s.name }) + (q.error ? " " + q.error : ""), { bad: true, server: id });
}

// --- a server's settings ----------------------------------------------

// toggleSettings opens a server's settings under its row, closing any
// other, or closes them. The open settings take the focus themselves,
// not their name's field, so that a phone's keyboard stays closed until
// a field is touched.
function toggleSettings(id) {
  conn.open = conn.open === id ? null : id;
  conn.panel = conn.open ? settingsPanel(serverById(conn.open)) : null;
  renderServers();
  if (conn.panel) conn.panel.focus();
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
// Open at start, when the fingerprint is asked, its session with Sign
// out, its certificate and the keys and sessions left valid there (fillFacts), the
// login by fingerprint kept with its user, its key's id, and Turn off, notifications
// where the app has push, Cancel and Save, and under them, after a rule,
// Remove server with a line that says what it does, whose question warns
// of a key or a session left.
// The panel itself takes the focus (toggleSettings).
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
  const msg = textEl("p", "", "small cubes-msg server-msg hidden");
  msg.setAttribute("role", "status");
  const form = formEl(() => saveSettings(s.id, form, { name, lan, remote, start, ask, push, msg }), "panel server-panel");
  form.tabIndex = -1;
  form.setAttribute("aria-label", t("connect.settingsOf", { name: s.name }));
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
  const facts = elem("div", "server-facts");
  form.append(
    fieldRow(t("connect.name"), name, { wide: true }),
    fieldRow(t("connect.lan"), lan, { wide: true, hint: t("connect.lanHint") }),
    fieldRow(t("connect.remote"), remote, { wide: true, hint: t("connect.remoteHint") }),
    fieldRow("", start, { hint: t("connect.startHint") }),
    fieldRow(t("connect.ask"), ask, { wide: true, hint: t("connect.askHint") }),
    facts,
  );
  fillFacts(form, s);
  if (s.login) {
    const off = button(t("connect.loginOff"), "server-login-off", () => turnOff(s.id, off, msg));
    // the kept key's id whenever the app knows it (login_key, contract version 5; AP MobileApp, D66-A)
    const key = Number.isInteger(s.login_key) && s.login_key > 0 && s.login_key <= MAX_KEY_ID ? s.login_key : 0;
    const kept = textEl("p", key ? t("connect.loginKeptKey", { name: s.login, id: key }) : t("connect.loginKept", { name: s.login }), "server-login-kept");
    kept.append(" ", off);
    form.append(kept);
  }
  if (push) form.append(fieldRow("", push));
  form.append(msg, formFooter({ submit: t("action.save"), cancel: () => toggleSettings(null) }), removeBox);
  fillRemove(form, s);
  return form;
}

// fillRemove says under the rule what Remove does: the server leaves the
// list, and where it answers the app is signed out there, the kept
// login's key revoked first where one is kept.
function fillRemove(form, s) {
  const hint = form.querySelector(".server-remove-hint");
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
  conn.open = null;
  conn.panel = null;
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

// fillFacts writes what a server's open settings say of it as it stands,
// under its choices: the session the app holds there, with Sign out,
// which says while the server does not answer that Sign out ends it there
// the next time the app reaches the server (F42, D105-A), the
// certificate the probe found at the address in use, with Certificate…,
// each key of the phone's left valid there, with Show in My profile,
// which opens the server's page at My profile with that key marked, off
// while the server does not answer, its title saying why (AP MobileApp,
// F33), and each session of the phone's left valid there, a key or a
// session whose revoke code waits saying that it ends the next time the app
// reaches the server (`pending`, contract version 6); drawn again only
// where it changed, whether the server is shown down among it (shownDown,
// which a probe still running leaves as it was), which over plain HTTP
// nothing else in it tells, so that a probe that answers keeps the focus,
// the buttons Show in My profile turned on and off in place; and what
// Remove does (fillRemove).
function fillFacts(form, s) {
  fillRemove(form, s);
  const box = form.querySelector(".server-facts");
  if (!box) return;
  const p = conn.probes[s.id] || {};
  const answers = p.state === "lan" || p.state === "remote";
  const cert = answers && p.cert && p.cert.not_after ? p.cert : null;
  const left = keysLeft(s);
  const unended = sessionsLeft(s);
  const down = shownDown(p);
  const key = JSON.stringify([s.session || "", down, cert && cert.fingerprint, cert && cert.not_after, cert && p.url, left, unended]);
  if (box.dataset.key === key) {
    showsKeys(box, answers);
    return;
  }
  box.dataset.key = key;
  clear(box);
  if (s.session) {
    const out = button(t("connect.signOut"), "server-sign-out", () => signOut(s.id, out, form.querySelector(".server-msg")));
    const line = down ? t("connect.sessionDown", { name: s.session }) : t("connect.sessionLine", { name: s.session });
    box.appendChild(elem("p", "server-fact server-session", line, " ", out));
  }
  if (cert) {
    const line = t("connect.certLine", { name: commonName(cert.subject) || hostOf(p.url), until: fmtDay(cert.not_after) });
    const show = button(t("connect.certButton"), "server-cert-open", () => showCert(s.id));
    box.appendChild(elem("p", "server-fact server-cert", line, " ", show));
  }
  for (const k of left) {
    const show = button(t("connect.keyLeftShow"), { cls: "server-key-show", aria: t("connect.keyLeftShowLabel", { id: k.id }) },
      () => openServer(s.id, null, "#profile?key=" + k.id));
    const text = k.pending === true ? t("connect.keyPending", { id: k.id, name: k.user }) : t("connect.keyLeft", { id: k.id, name: k.user });
    box.appendChild(elem("p", "server-fact server-key-left", text, " ", show));
  }
  for (const x of unended) {
    const text = x.pending === true ? t("connect.sessionPending", { id: x.id, name: x.user }) : t("connect.sessionLeft", { id: x.id, name: x.user });
    box.appendChild(elem("p", "server-fact server-session-left", text));
  }
  box.classList.toggle("hidden", !box.firstChild);
  showsKeys(box, answers);
}

// showsKeys turns each Show in My profile of the open settings on where
// the server answers, else off with its title saying why: the page it
// opens is the server's (AP MobileApp, F33).
function showsKeys(box, answers) {
  const why = answers ? "" : t("connect.keyLeftShowOff");
  for (const b of box.querySelectorAll(".server-key-show")) {
    setOff(b, !answers);
    if (b.title !== why) b.title = why;
  }
}

// signOut signs the app out of a server, at the address the probe chose
// (signOut, docs/app.md, the session ended there and forgotten in the
// app); the open settings lose the session's line, show it left where the
// server did not end it (the list read again: its unended, contract
// version 5) and keep what is typed in them, and the toast says whether
// the server ended it, and where it did not whether its revoke code ends
// it the next time the app reaches the server (`pending`, version 6).
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

// commonName is the common name (CN) of a distinguished name as the app
// writes it (RFC 4514, "CN=massimo,O=Massimo"), "" for none.
function commonName(dn) {
  const m = /(?:^|,)\s*CN=((?:\\.|[^,\\])*)/i.exec(dn || "");
  return m ? m[1].replace(/\\(.)/g, "$1").trim() : "";
}

// fmtDay is a time the app gives (RFC 3339) as a day in the page's
// language; fmtWhen the same with the time of day. Anything else stays
// as it is.
function fmtDay(iso) {
  const d = new Date(iso);
  return isNaN(d) ? String(iso || "") : new Intl.DateTimeFormat(i18n.lang, { dateStyle: "medium" }).format(d);
}

function fmtWhen(iso) {
  const d = new Date(iso);
  return isNaN(d) ? String(iso || "") : new Intl.DateTimeFormat(i18n.lang, { dateStyle: "medium", timeStyle: "short" }).format(d);
}

// showCert shows a server's certificate as its probe found it at the
// address in use: its subject, issuer, names, validity and fingerprint,
// and since when it is trusted: since the user compared its fingerprint
// (trust), or by the phone, a certificate authority having signed it.
function showCert(id) {
  const s = serverById(id);
  const p = conn.probes[id] || {};
  const c = p.cert;
  if (!s || !c) return;
  const none = "—";
  $("cert-address").textContent = hostOf(p.url);
  $("cert-subject").textContent = c.subject || none;
  $("cert-issuer").textContent = c.self_signed ? t("connect.cert.selfSigned") : c.issuer || none;
  $("cert-names").textContent = c.names.length ? c.names.join(", ") : none;
  $("cert-from").textContent = c.not_before ? fmtWhen(c.not_before) : none;
  $("cert-until").textContent = c.not_after ? fmtWhen(c.not_after) : none;
  showFingerprint($("cert-fp"), c.fingerprint);
  let trusted = t("connect.cert.trustedSystem");
  if (s.fp && s.fp.toUpperCase() === c.fingerprint.toUpperCase()) {
    trusted = s.trusted_at ? t("connect.cert.trustedSince", { when: fmtWhen(s.trusted_at) }) : t("connect.cert.trustedPinned");
  }
  $("cert-trusted").textContent = trusted;
  const dlg = $("cert-dialog");
  dlg.showModal();
  $("cert-close").focus();
}

// turnOff turns a server's login by fingerprint off (forget, which
// revokes the key at the address the probe chose, with the session the
// app holds there or with the key itself after the fingerprint, and
// signs the app out there); the open settings lose its line and the
// session's and keep what is typed in them, and the toast says whether
// the server revoked the key, or whether this Turn off left it, to revoke
// at the next sign-in there (`left`, contract version 4), or by its revoke
// code the next time the app reaches the server (`pending`, version 6).
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
  if (res && res.revoked) say(t("connect.loginOffDone", { name }));
  else if (res && res.pending === true) say(t("connect.loginOffPending", { name }));
  else if (res && res.left === true) say(t("connect.loginOffLeft", { name }));
  else say(t("connect.loginOffLocal", { name }));
}

// dropLoginLine takes the line of a kept login out of the open settings
// once the app forgot it, so that what is typed in them stays.
function dropLoginLine() {
  const s = conn.open && serverById(conn.open);
  const line = conn.panel && conn.panel.querySelector(".server-login-kept");
  if (line && !(s && s.login)) line.remove();
}

// menuButton is the summary of the Servers card's ☰, where the focus
// goes back to once what it opened is done.
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
      dropLoginLine();
      renderServers();
      say(t("connect.loginOffFailed", { error: e.message }), { bad: true });
      return;
    }
    await loadServers();
    dropLoginLine();
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
    return;
  }
  conn.servers = conn.servers.filter((x) => x.id !== id);
  delete conn.probes[id];
  conn.open = null;
  conn.panel = null;
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
  history.replaceState(null, "", location.pathname + location.search + APP_HASH);
  card.focus({ preventScroll: true });
  card.scrollIntoView({ block: "nearest" });
}

function closeApp() {
  $("app-card").classList.add("hidden");
  showResult($("app-msg"), "", false);
  history.replaceState(null, "", location.pathname + location.search);
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

// initMenu makes the Servers card's ☰ a dropdown of the page (dropdown,
// components.js) and wires its entries.
function initMenu() {
  const menu = $("servers-menu");
  dropdown(menu);
  menuArrows(menu, ".menu-items button:not(.hidden)");
  $("menu-add").addEventListener("click", openAdd);
  $("menu-sign-out-all").addEventListener("click", signOutAll);
  $("menu-forget-all").addEventListener("click", forgetAll);
  $("menu-app").addEventListener("click", openApp);
  $("add-close").addEventListener("click", () => closeAdd(true));
  $("app-close").addEventListener("click", closeApp);
  $("app-appearance").addEventListener("change", saveAppearance);
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
  window.addEventListener("massimo", onBridgeEvent);
  $("add-form").addEventListener("submit", (ev) => {
    ev.preventDefault();
    addAddress();
  });
  $("add-address").addEventListener("input", warnPlain);
  $("probe-btn").addEventListener("click", (ev) => whileBusy(ev.currentTarget, probeAll));
  $("search-btn").addEventListener("click", search);
  $("cert-close").addEventListener("click", () => $("cert-dialog").close());
  initMenu();
  await loadServers();
  renderServers();
  openFromAddress();
  probeAll();
  // at the start for the saved servers too: a LAN address that changed,
  // and the names discovery hears (onFound)
  search();
  initApp();
  initCore();
}

document.addEventListener("DOMContentLoaded", init);
