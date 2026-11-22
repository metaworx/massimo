// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/sysinfo/store.ts
 *
 * What System info is drawn from, read line for line as system-info.js
 * did: the server's facts (GET /api/v1/system), read again every 2 s
 * while a proxy's certificate is being read and the page is shown, at
 * most 15 times; the store and the settings in effect, for a holder of
 * settings (GET /api/v1/store and GET /api/v1/config); and the server's
 * clock, which ticks every second while the page is shown; while the
 * page is shown, a resync of the stream, the settings file read again
 * and a Cube changed read them again (serverEvent). The state is live, so that the part draws a new answer in place; an answer after a
 * sign-out is dropped (shared/session.ts), and a sign-out ends the
 * readings and the clock.
 */

import { signal } from "@preact/signals";
import type { ConfigView, StoreInfo, SystemInfo } from "../api";
import type { ServerEvent } from "../model/event";
import { burstReader } from "../shared/burst";
import { live } from "../shared/live";
import { session, stillSession } from "../shared/session";
import { isShown } from "../shared/pages";
import { can } from "../shared/me";

/** What the page shows, as the last loads that went through gave it. */
export const data = live({
  /** The server's facts. */
  sys: null as SystemInfo | null,
  /** The store, for a holder of settings. */
  store: null as StoreInfo | null,
  /** The settings in effect, for a holder of settings. */
  config: null as ConfigView | null,
  /** Why the last read of the store and the settings failed, "" where it went through. */
  error: "",
  /** The last reading of a proxy's certificate is over and it was not read. */
  unread: false,
  /** The server's clock less this browser's, in ms, from the last answer. */
  skew: 0,
});

/** Whether the store's and the settings' folds are open, as the user left them. */
export const folds = live({ store: false, config: false });

/** The clock's tick, raised every second while System info is shown. */
export const tick = signal(0);

/** CERT_EVERY is how often a proxy's certificate is read again, CERT_MAX how many times. */
const CERT_EVERY = 2000;
const CERT_MAX = 15;

/** The readings of a proxy's certificate: how many, their timer, the last answer; and the clock's timer. */
const cert = { tries: 0, timer: 0 as ReturnType<typeof setTimeout> | 0, reading: false, last: null as SystemInfo | null };

/** certRead is a reading of the certificate's, its timer's: under way until it has answered. */
function certRead(): void {
  cert.timer = 0;
  cert.reading = true;
  loadSystemInfo().finally(() => {
    cert.reading = false;
  });
}
let clockTimer: ReturnType<typeof setInterval> | 0 = 0;

/** loadServerInfo reads the store and the settings in effect, for a holder of settings; the others see neither. */
export async function loadServerInfo(): Promise<void> {
  if (!can("settings")) return;
  const n = session();
  data.error = "";
  try {
    const [store, config] = await Promise.all([api<StoreInfo>("GET", "api/v1/store"), api<ConfigView>("GET", "api/v1/config")]);
    if (!stillSession(n)) return;
    data.store = store;
    data.config = config;
  } catch (e) {
    if (stillSession(n)) data.error = (e as Error).message;
  }
}

/** certPending reports whether the proxy's certificate is still to be read. */
function certPending(v: SystemInfo | null): boolean {
  return !!(v && v.proxy && v.proxy.proto === "https" && !v.proxy.certificate);
}

/**
 * certFollow reads again while the certificate is being read and System
 * info is shown, a request that failed counting as a try; after the
 * last, the certificate could not be read.
 */
function certFollow(v: SystemInfo | null): void {
  if (cert.timer) clearTimeout(cert.timer);
  cert.timer = 0;
  if (!certPending(v) || !isShown("info")) return;
  if (cert.tries < CERT_MAX) {
    cert.tries++;
    cert.timer = setTimeout(certRead, CERT_EVERY);
  } else data.unread = true;
}

/** startClock keeps the server's clock by its difference to this browser's, ticking every second while System info is shown. */
function startClock(v: SystemInfo): void {
  if (clockTimer) clearInterval(clockTimer);
  clockTimer = 0;
  const ms = Date.parse(v.now || "");
  if (isNaN(ms)) return;
  data.skew = ms - Date.now();
  clockTimer = setInterval(() => {
    if (isShown("info")) tick.value++;
  }, 1000);
}

/**
 * loadSystemInfo reads the server's facts and draws them; a read that
 * fails leaves them as they were, and counts as a try of the
 * certificate's. follow false is an event's read: it brings the facts and
 * leaves the certificate's readings, their count and their timer, as they
 * are.
 */
export async function loadSystemInfo(follow = true): Promise<void> {
  const n = session();
  let v: SystemInfo;
  try {
    v = await api<SystemInfo>("GET", "api/v1/system");
  } catch {
    if (stillSession(n) && follow) certFollow(cert.last);
    return; // the rows keep their dashes, or what they showed
  }
  if (!v || !stillSession(n)) return;
  if (follow) cert.last = v;
  data.sys = v;
  startClock(v);
  tick.value++;
  if (follow) certFollow(v);
  else if (certPending(v) && !cert.timer && !cert.reading) {
    // an event's read finds the certificate being read and nothing reading it (a new server after a restart): the readings start anew
    cert.tries = 0;
    data.unread = false;
    cert.last = v;
    certFollow(v);
  }
}

/** The page's reads for the stream's events, one at a time: "config" the store and the settings in effect, "facts" the server's facts. */
const reader = burstReader(async (what) => {
  if (what.has("config")) await loadServerInfo();
  if (what.has("facts")) await loadSystemInfo(false);
}, () => isShown("info"));

/** CUBE_CHANGES are the change events' actions that add, change or take away a Cube, and with it a proxy's listener and the interfaces listened on. */
const CUBE_CHANGES = ["cube.add", "cube.update", "cube.remove"];

/**
 * serverEvent reads again, while System info is shown, what an event of
 * the stream says has changed: everything at a resync of the stream,
 * which is how the page learns of a restart (the server's start is
 * written before any stream opens); the settings in effect when the
 * settings file was read again (settings_read); the facts when a Cube was
 * added, changed or taken away (change), which changes the proxies'
 * listeners and the interfaces listened on. No other event changes a fact
 * the page shows.
 */
export function serverEvent(ev: ServerEvent): void {
  if (!isShown("info")) return;
  if (ev.kind === "resync") reader("config", "facts");
  else if (ev.kind === "settings_read") reader("config");
  else if (ev.kind === "change" && CUBE_CHANGES.includes(String((ev.summary || {}).action))) reader("facts");
}

/** shown reads everything System info shows, as the page is shown; the certificate's readings start again. */
export function shown(): void {
  loadServerInfo();
  cert.tries = 0;
  cert.last = null;
  data.unread = false;
  loadSystemInfo();
}

/**
 * forgetSysinfo forgets what the page kept of the user who was signed
 * in (MassimoParts.signedOut): the facts, the store and the settings in
 * effect, the readings of a certificate and the clock with their timers.
 */
export function forgetSysinfo(): void {
  if (cert.timer) clearTimeout(cert.timer);
  if (clockTimer) clearInterval(clockTimer);
  cert.timer = 0;
  cert.reading = false;
  clockTimer = 0;
  cert.tries = 0;
  cert.last = null;
  Object.assign(data, { sys: null, store: null, config: null, error: "", unread: false, skew: 0 });
}
