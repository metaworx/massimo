// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/sysinfo/SysInfo.tsx
 *
 * System info as a part, built once and updated in place: the table of
 * the server's facts (its version, copyable, what it was built with, its
 * start and uptime, its clock, ticking, the certificate the browser sees,
 * how it was installed, the computer, its full name, the interfaces it
 * listens on, the Cube proxies' listeners, copyable, the address it sees
 * this browser at), then the store and the settings in effect in folds of
 * their own, for a holder of settings. A new answer, a certificate's
 * reading again and the clock's tick leave the folds open, a copy's
 * "copied" until it fades and a text selected where they are; the page's
 * row counts the proxies that do not listen.
 */

import type { ComponentChildren } from "preact";
import { useLayoutEffect } from "preact/hooks";
import type { ConfigView, StoreInfo, SystemInfo } from "../api";
import { Age, Notice } from "../shared/badge";
import { Copy, CopyPair } from "../shared/copy";
import { Fold } from "../shared/fold";
import { CountList, KvList } from "../shared/kv";
import { tParts } from "../shared/lang";
import { Link } from "../shared/link";
import { data, folds, tick } from "./store";
import { cardBadges } from "../shared/card";
import { useLook } from "../shared/look";

/** OS_NAMES are the operating systems as Go names them, where the server has no distribution's name. */
const OS_NAMES: Record<string, string> = { linux: "Linux", darwin: "macOS", windows: "Windows", freebsd: "FreeBSD" };

/** SECONDS is a date and a time to the second, as System info shows its times. */
const SECONDS: Intl.DateTimeFormatOptions = { year: "numeric", month: "2-digit", day: "2-digit", hour: "2-digit", minute: "2-digit", second: "2-digit", hour12: false };

/** knownZone reports whether this browser can show a time in zone. */
function knownZone(zone: string): boolean {
  try {
    new Intl.DateTimeFormat(undefined, { timeZone: zone });
    return !!zone;
  } catch {
    return false;
  }
}

/** clockSpan says a clock difference in seconds or minutes. */
function clockSpan(ms: number): string {
  const s = Math.round(Math.abs(ms) / 1000);
  return s < 120 ? s + " s" : Math.round(s / 60) + " min";
}

/** offsetOf is the offset in minutes an ISO time carries, 0 for Z. */
function offsetOf(iso: string): number {
  const m = /([+-])(\d\d):(\d\d)$/.exec(iso || "");
  return m ? (m[1] === "-" ? -1 : 1) * (Number(m[2]) * 60 + Number(m[3])) : 0;
}

/**
 * serverTime is an instant as the server's clock reads it, to the second,
 * and the server's zone: in the zone where this browser knows it, else at
 * the offset iso carries.
 */
function serverTime(ms: number, iso: string, zone: string): string {
  const text = knownZone(zone)
    ? new Intl.DateTimeFormat(undefined, { ...SECONDS, timeZone: zone }).format(new Date(ms))
    : new Intl.DateTimeFormat(undefined, { ...SECONDS, timeZone: "UTC" }).format(new Date(ms + offsetOf(iso) * 60000));
  return [text, zone].filter(Boolean).join(" ");
}

/** browserTime is the same instant in this browser's own zone, named, for a tooltip. */
function browserTime(ms: number): string {
  const zone = Intl.DateTimeFormat().resolvedOptions().timeZone || "";
  return t("sysinfo.browserTime", { time: new Intl.DateTimeFormat(undefined, SECONDS).format(new Date(ms)), zone });
}

/** Started is when the server started as its clock read it, then its uptime; in this browser's zone as its tooltip. */
function Started({ v }: { v: SystemInfo }) {
  const ms = Date.parse(v.started_at || "");
  if (isNaN(ms)) return <td id="info-started">–</td>;
  return (
    <td id="info-started" title={browserTime(ms)}>
      {serverTime(ms, v.started_at, v.time_zone || "")}
      <Age at={v.started_at} plain />
    </td>
  );
}

/** Now is the server's time, ticking every second while the page is shown, with a note where it differs from this browser's by more than 5 s. */
function Now({ v }: { v: SystemInfo }) {
  void tick.value;
  const ms = Date.parse(v.now || "");
  if (isNaN(ms)) return <td id="info-now">–</td>;
  const skew = data.skew;
  const note = Math.abs(skew) > 5000 ? t(skew > 0 ? "sysinfo.ahead" : "sysinfo.behind", { d: clockSpan(skew) }) : "";
  const now = Date.now() + skew;
  return (
    <td id="info-now" title={browserTime(now)}>
      {serverTime(now, v.now, v.time_zone || "")}
      {note && <span class="muted small sys-skew">{note}</span>}
    </td>
  );
}

/** Host is the host's full name, the host bold and its domain as it is. */
function Host({ v }: { v: SystemInfo }) {
  const name = v.fqdn || v.hostname || "";
  const dot = name.indexOf(".");
  return (
    <td id="info-host" class="mono">
      {!name ? "–" : <><strong>{dot < 0 ? name : name.slice(0, dot)}</strong>{dot >= 0 ? name.slice(dot) : ""}</>}
    </td>
  );
}

/**
 * Addresses lists the interfaces the server listens on, a line each: the
 * name, the MAC, then its addresses, those of the web interface marked
 * with an asterisk, which a note under the lines explains; where the web
 * interface takes every address, the note says so instead.
 */
function Addresses({ v }: { v: SystemInfo }) {
  const ifs = v.interfaces || [];
  if (!ifs.length) return <td id="info-addrs">–</td>;
  const all = ifs.every((i) => i.addresses.every((a) => a.api));
  const port = v.api_port || "";
  const some = ifs.some((i) => i.addresses.some((a) => a.api));
  return (
    <td id="info-addrs">
      {ifs.map((i) => (
        // text that wraps as words do: the name, the MAC, the addresses
        <div key={i.name} class="sys-iface">
          <span class="mono sys-name">{i.name}</span>
          {i.mac && <>{" "}<span class="mono sys-mac">{i.mac}</span></>}
          {" "}<span class="mono">{i.addresses.map((a) => a.ip + (a.api && !all ? "*" : "")).join(", ")}</span>
        </div>
      ))}
      {all ? <p class="muted small sys-note">{t("sysinfo.apiAll", { port })}</p>
        : some && <p class="muted small sys-note">{t("sysinfo.apiMark", { port })}</p>}
    </td>
  );
}

/**
 * Proxies lists the Cube proxies' listeners, a line each: the Cube's
 * serial, the serial it answers discovery with and the address the
 * original software connects to, each copyable; then "listening", or why
 * not and since when, the server trying it again.
 */
function Proxies({ v }: { v: SystemInfo }) {
  const list = v.cube_proxies || [];
  return (
    <td id="info-proxies">
      {!list.length && "–"}
      {list.map((p) => (
        <div key={(p.serial || "") + " " + p.addr} class="sys-proxy">
          <Copy text={p.serial || ""} label={p.serial} icon />
          {p.discovery_serial && <>{" "}{tParts("proxy.appearsAs", {}, { serial: <Copy text={p.discovery_serial} label={p.discovery_serial} icon /> })}</>}
          {" "}<Copy text={p.addr} label={p.addr} icon />
          {p.listening ? <>{" "}<span class="muted">{t("proxy.listening")}</span></>
            : <Notice text={t("proxy.notListening", { addr: p.addr, error: p.error || "" })} since={p.since} next={p.next_try} />}
        </div>
      ))}
    </td>
  );
}

/** Address is an address served: a link to it in a new window, and its copy icon. */
function Address({ url }: { url: string }) {
  return (
    <div class="mono muted small sys-url">
      <CopyPair get={() => url} label={t("copy.what", { what: url })}><Link url={url} text={url} /></CopyPair>
    </div>
  );
}

/**
 * Certificate is the certificate the browser sees: the proxy's,
 * "(proxied)", where a trusted proxy serves the page (its names, expiry
 * and issuer as the server last read them, or why it could not, "being
 * read…" while it reads it, "could not be read" once the readings gave
 * up), with the address the proxy serves, the page's own where the proxy
 * names no scheme, and the header it does not send; the page's own HTTPS
 * while the server serves none, which only a proxy does, with the setting
 * that would tell more; else the server's own.
 */
function Certificate({ v }: { v: SystemInfo }) {
  const p = v.proxy;
  const own = v.tls && <div class="muted small">{t("sysinfo.ownCert", { cert: certText(v.tls) })}</div>;
  if (p) {
    if (!p.proto) {
      // no scheme from the proxy: the page's own, which the browser knows
      const https = location.protocol === "https:";
      return (
        <td id="info-tls">
          <div>{t(https ? "sysinfo.proxyHttps" : "sysinfo.proxyPlain") + " " + t("sysinfo.proxied")}</div>
          <Address url={location.protocol + "//" + location.host} />
          <div class="muted small">{t("sysinfo.proxyNoScheme")}</div>
          {own}
        </td>
      );
    }
    const c = p.certificate;
    let text: string;
    if (p.proto !== "https") text = t("sysinfo.proxyPlain");
    else if (!c) text = t(data.unread ? "sysinfo.proxyUnread" : "sysinfo.proxyPending");
    else if (c.error) text = t("sysinfo.proxyError", { error: c.error });
    else {
      const until = Date.parse(c.not_after || "");
      text = [(c.names || []).join(", "), isNaN(until) ? "" : t("header.certUntil", { date: fmtInstant(new Date(until)) }),
        c.issuer ? t("sysinfo.issuer", { issuer: c.issuer }) : "", c.trusted ? "" : t("sysinfo.untrusted")].filter(Boolean).join(" · ");
    }
    // the scheme's own port left out, as the announcement's url has it
    const ownPort = (p.proto === "https" && p.port === 443) || (p.proto === "http" && p.port === 80) || !p.port;
    return (
      <td id="info-tls">
        <div>{text + " " + t("sysinfo.proxied")}</div>
        <Address url={p.proto + "://" + (p.host.includes(":") ? "[" + p.host + "]" : p.host) + (ownPort ? "" : ":" + p.port)} />
        {own}
      </td>
    );
  }
  if (location.protocol === "https:" && !v.tls) {
    return (
      <td id="info-tls">
        <div>{t("sysinfo.proxyHttps") + " " + t("sysinfo.proxied")}</div>
        <Address url={"https://" + location.host} />
        <div class="muted small">{t("sysinfo.proxyHint")}</div>
      </td>
    );
  }
  return <td id="info-tls">{v.tls ? certText(v.tls) : t("sysinfo.noTLS")}</td>;
}

/** Facts is the table of the server's facts, dashes before the first answer. */
function Facts() {
  useLook();
  const v = data.sys;
  const dash = (id: string, cls?: string) => <td id={id} class={cls}>–</td>;
  const parts = splitVersion((v && v.version) || "");
  let install = "–";
  if (v) {
    // a kind this page has no text for, a package manager's, as it is
    const kind = v.install || "other";
    const label = t("sysinfo.install." + kind);
    install = label === "sysinfo.install." + kind ? kind : label;
  }
  const os = v ? v.os_name || OS_NAMES[v.os] || v.os || "" : "";
  const row = (head: string, cell: ComponentChildren) => <tr><th scope="row">{t(head)}</th>{cell}</tr>;
  return (
    <table class="info-table">
      <tbody>
        {/* the short version shown, the whole one copied, by a click or the icon */}
        {row("budget.server", <td id="info-version">{v ? <Copy text={parts.short || "–"} value={v.version || ""} name={t("copy.version")} icon={!!v.version} /> : "–"}</td>)}
        {row("budget.built", <td id="info-built">{(v && parts.rest) || "–"}</td>)}
        {row("budget.started", v ? <Started v={v} /> : dash("info-started"))}
        {row("sysinfo.now", v ? <Now v={v} /> : dash("info-now"))}
        {row("budget.certificate", v ? <Certificate v={v} /> : dash("info-tls"))}
        {row("sysinfo.install", <td id="info-install">{install}</td>)}
        {row("sysinfo.system", <td id="info-system">{(v && [os, v.arch].filter(Boolean).join(" · ")) || "–"}</td>)}
        {row("sysinfo.host", v ? <Host v={v} /> : dash("info-host", "mono"))}
        {row("sysinfo.addresses", v ? <Addresses v={v} /> : dash("info-addrs"))}
        {row("sysinfo.proxies", v ? <Proxies v={v} /> : dash("info-proxies"))}
        {row("sysinfo.client", <td id="info-client" class="mono">{(v && v.client_ip) || "–"}</td>)}
      </tbody>
    </table>
  );
}

/** StoreBody is the store: its file, schema, node and tenant, and the rows per table; why it could not be read first. */
function StoreBody({ s, error }: { s: StoreInfo | null; error: string }) {
  return (
    <div id="store-info-body">
      {error && <p class="small cubes-msg bad">{t("maint.cannotAsk", { error })}</p>}
      {s && (
        <>
          <KvList facts={[
            [t("maint.storeFile"), s.file, true],
            [t("maint.storeSchema"), t("maint.storeSchemaValue", { schema: s.schema, min: s.min_reader, binary: s.binary })],
            [t("maint.node"), s.node, true],
            [t("maint.tenant"), s.tenant, true],
          ]} />
          <CountList counts={s.rows || {}} />
        </>
      )}
    </div>
  );
}

/** ConfigBody is the settings in effect, and every MASSIMO_* variable with where it came from, a secret "set, not shown". */
function ConfigBody({ c }: { c: ConfigView | null }) {
  return (
    <div id="config-info-body">
      {c && (
        <>
          <KvList facts={[
            [t("maint.cfgVersion"), c.version || "–"],
            [t("maint.file"), c.file + " (" + t(c.file_found ? "maint.fileFound" : "maint.fileMissing") + ")", true],
            [t("maint.cfgData"), c.data_dir, true],
            [t("maint.cfgListen"), c.listen, true],
            [t("maint.cfgTLS"), c.tls],
            [t("maint.cfgLog"), c.log_level],
          ]} />
          {c.variables && c.variables.length > 0 && (
            <>
              <p class="muted small">{t("maint.cfgVars")}</p>
              <KvList facts={c.variables.map((v): [string, string, boolean] => [v.name,
                (v.secret ? t("maint.secretSet") : v.value || "") + " (" + v.source + ")" + (v.unknown ? " · " + t("maint.varUnknown") : ""), !v.secret])} />
            </>
          )}
        </>
      )}
    </div>
  );
}

/** SysInfo is the page's card: the facts, then the store and the settings in effect (with settings), and the page row's badge. */
export function SysInfo() {
  useLook();
  const v = data.sys;
  const s = data.store;
  const c = data.config;
  const failing = v ? (v.cube_proxies || []).filter((p) => !p.listening).length : 0;
  const badge = failing ? tn("proxy.badge.failing", failing) : "";
  // the page's row counts the proxies that do not listen
  useLayoutEffect(() => {
    cardBadges("info-page", badge ? [{ text: badge, kind: "warn" }] : []);
  }, [badge]);
  const rows = s ? Object.values(s.rows || {}).reduce((a, b) => a + b, 0) : 0;
  return (
    <>
      <Facts />
      <Fold id="store-info" cls="sys-sub needs-settings" open={folds.store} onToggle={(open) => {
        folds.store = open;
      }} summary={<><span>{t("maint.store")}</span><span id="store-info-sum" class="muted">
        {s ? t("maint.storeSum", { file: s.file.split(/[\\/]/).pop() || "", schema: s.schema, rows: fmtNum(rows, 0) }) : ""}</span></>}>
        <StoreBody s={s} error={data.error} />
      </Fold>
      <Fold id="config-info" cls="sys-sub needs-settings" open={folds.config} onToggle={(open) => {
        folds.config = open;
      }} summary={<><span>{t("maint.config")}</span><span id="config-info-sum" class="muted">{c ? c.file : ""}</span></>}>
        <ConfigBody c={c} />
      </Fold>
    </>
  );
}
