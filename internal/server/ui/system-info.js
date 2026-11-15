// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

// The Massimo web UI, System info: the store and the settings in
// effect, the server's facts, its addresses, its proxies and their
// certificates.
"use strict";

// --- the store and the settings in effect (with settings) -------------

const infoState = { store: null, config: null, error: "" };

// renderServerInfo draws System Info's Store and Settings in effect, from
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
    if (!isShown("info")) return;
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
    // the Cube's serial, the serial the proxy answers discovery with, and
    // the address the original software connects to, each copyable
    // (components.js)
    const line = elem("div", "sys-proxy", copyEl(p.serial || "", { label: p.serial, icon: true }));
    if (p.discovery_serial) line.append(" ", ...tNodes("proxy.appearsAs", { serial: copyEl(p.discovery_serial, { label: p.discovery_serial, icon: true }) })); // i18n.js
    line.append(" ", copyEl(p.addr, { label: p.addr, icon: true }));
    if (p.listening) line.append(" ", textEl("span", t("proxy.listening"), "muted"));
    else line.appendChild(noticeEl(t("proxy.notListening", { addr: p.addr, error: p.error || "" }), { since: p.since, next: p.next_try }));
    cell.appendChild(line);
  }
  const failing = list.filter((p) => !p.listening).length;
  sectionBadges($("info-page"), failing ? [badgeEl(tn("proxy.badge.failing", failing), "warn")] : []); // in its row
}

// certRetry follows a proxy's certificate the server has not read yet
// (it reads it within seconds of the first request): System info asks
// again every `every` ms while it is shown, `max` times; after the last,
// the certificate could not be read. Opening System info starts it again.
const certRetry = { tries: 0, timer: null, every: 2000, max: 15, last: null };

// certPending reports whether the proxy's certificate is still to be read.
function certPending(v) {
  return !!(v && v.proxy && v.proxy.proto === "https" && !v.proxy.certificate);
}

// certFollow asks again while the certificate is being read and System
// info is shown (certRetry), a request that failed counting as a try;
// after the last, it says the certificate could not be read.
function certFollow(v) {
  clearTimeout(certRetry.timer);
  if (!certPending(v) || !isShown("info")) return;
  if (certRetry.tries < certRetry.max) {
    certRetry.tries++;
    certRetry.timer = setTimeout(loadSystemInfo, certRetry.every);
  } else fillCertificate(v);
}

// fillCertificate shows the certificate the browser sees: the proxy's,
// "(proxied)", where a trusted proxy serves the page (its names, expiry
// and issuer as the server last read them, or why it could not, "being
// read…" while it reads it, "could not be read" once certRetry gave up),
// with the address the proxy serves, a link and copyable, the page's own
// where the proxy names no scheme, and the header it does not send; the
// page's own HTTPS while the server serves none, which only a proxy does,
// with the setting that would tell more; else the server's own (certText,
// header.js).
function fillCertificate(v) {
  const cell = $("info-tls");
  clear(cell);
  const line = (text, cls) => cell.appendChild(textEl("div", text, cls));
  // an address: a link to it in a new window, and its copy icon
  const address = (url) =>
    cell.appendChild(elem("div", "mono muted small sys-url", elem("span", "copy-pair", linkEl(url, url), copyIcon(() => url, t("copy.what", { what: url })))));
  const p = v.proxy;
  if (p) {
    const c = p.certificate;
    if (!p.proto) {
      // no scheme from the proxy: the page's own, which the browser knows
      const https = location.protocol === "https:";
      line(t(https ? "sysinfo.proxyHttps" : "sysinfo.proxyPlain") + " " + t("sysinfo.proxied"));
      address(location.protocol + "//" + location.host);
      line(t("sysinfo.proxyNoScheme"), "muted small");
      if (v.tls) line(t("sysinfo.ownCert", { cert: certText(v.tls) }), "muted small");
      return;
    }
    let text;
    if (p.proto !== "https") text = t("sysinfo.proxyPlain");
    else if (!c) text = t(certRetry.tries >= certRetry.max ? "sysinfo.proxyUnread" : "sysinfo.proxyPending");
    else if (c.error) text = t("sysinfo.proxyError", { error: c.error });
    else {
      const until = Date.parse(c.not_after || "");
      text = [(c.names || []).join(", "), isNaN(until) ? "" : t("header.certUntil", { date: fmtInstant(new Date(until)) }),
        c.issuer ? t("sysinfo.issuer", { issuer: c.issuer }) : "", c.trusted ? "" : t("sysinfo.untrusted")].filter(Boolean).join(" · ");
    }
    line(text + " " + t("sysinfo.proxied"));
    // the scheme's own port left out, as the announcement's url has it
    const own = (p.proto === "https" && p.port === 443) || (p.proto === "http" && p.port === 80) || !p.port;
    address(p.proto + "://" + (p.host.includes(":") ? "[" + p.host + "]" : p.host) + (own ? "" : ":" + p.port));
    if (v.tls) line(t("sysinfo.ownCert", { cert: certText(v.tls) }), "muted small");
    return;
  }
  if (location.protocol === "https:" && !v.tls) {
    line(t("sysinfo.proxyHttps") + " " + t("sysinfo.proxied"));
    address("https://" + location.host);
    line(t("sysinfo.proxyHint"), "muted small");
    return;
  }
  cell.textContent = v.tls ? certText(v.tls) : t("sysinfo.noTLS");
}

// loadSystemInfo fills System info, the page, from GET
// /api/v1/system: the version and what it was built with, when the
// server started and how long ago, the server's clock (startSystemClock),
// the certificate the browser sees (fillCertificate, asked for again
// while the server reads it, certFollow), how it was installed, the
// operating system and processor, the host's full name (fillHostName),
// the interfaces it listens on (fillAddresses), the Cube proxies'
// listeners (fillProxies), and the address the server sees this browser
// at.
async function loadSystemInfo() {
  let v;
  try {
    v = await api("GET", "api/v1/system");
  } catch (e) {
    certFollow(certRetry.last);
    return; // the rows keep their dashes, or what they showed
  }
  if (!v) return;
  certRetry.last = v;
  const parts = splitVersion(v.version || "");
  // the short version shown, the whole one copied, by a click or the icon
  const name = t("copy.version");
  const version = copyable(textEl("span", parts.short || "–", "mono"), () => v.version || "", name); // components.js
  $("info-version").replaceChildren(v.version ? elem("span", "copy-pair", version, copyIcon(() => v.version, name)) : version);
  $("info-built").textContent = parts.rest || "–";
  sysClock.zone = v.time_zone || "";
  fillStarted(v);
  startSystemClock(v);
  fillCertificate(v);
  certFollow(v);
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

// initSystemInfo loads System info each time it is shown.
function initSystemInfo() {
  onView("info", () => {
    loadServerInfo();
    Object.assign(certRetry, { tries: 0, last: null });
    loadSystemInfo();
  });
}

document.addEventListener("DOMContentLoaded", initSystemInfo);
