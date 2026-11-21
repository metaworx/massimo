// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/maintenance/store.ts
 *
 * What Maintenance → Maintenance is drawn from, read and sent, line for
 * line as maintenance.js did: the announcement on the LAN and the Cube
 * intervals (GET and PUT /api/v1/settings, each saved at once), the
 * settings file read again (POST /api/v1/reload), the server's restart
 * (GET and POST /api/v1/restart) and the computer's reboot (GET, POST
 * and DELETE /api/v1/reboot), each followed by /healthz every two seconds
 * until the server reports a new start. While Maintenance is shown, an
 * event of the stream that says one of these changed reads it again
 * (serverEvent). The state is what the section shows, live, so that the part draws it in place; an answer after a
 * sign-out is dropped (shared/session.ts), and a sign-out ends what
 * follows the server while it is away.
 */

import type { RebootInfo, RestartInfo, Setting } from "../api";
import { type ServerEvent, settingsChanged } from "../model/event";
import type { Outcome } from "../shared/action";
import { burstReader } from "../shared/burst";
import { counter } from "../shared/counter";
import { live } from "../shared/live";
import { session, stillSession } from "../shared/session";
import { isShown } from "../shared/pages";

/** A text said in the page's language at its drawing. */
export type Say = () => string;

/** The section's load counter: every part of it uses it. */
export const load = counter();

/** RESTART_POLL_MS is how often the page asks /healthz while the server restarts, RESTART_WAIT_MS how long it waits for it to come back. */
const RESTART_POLL_MS = 2000;
const RESTART_WAIT_MS = 2 * 60 * 1000;

/** REBOOT_WAIT_MS is how long the page waits for the computer to come back, from the reboot's time; REBOOT_DELAY_MAX the longest delay. */
const REBOOT_WAIT_MS = 5 * 60 * 1000;
export const REBOOT_DELAY_MAX = 3600;

/** The restart's row and the settings file's, as the last answers left them. */
export const restart = live({
  /** Restart the server is offered. */
  shown: false,
  /** It is off while its request runs and while the page follows the server. */
  off: false,
  /** How the restart stands, under its title; null hides the line. */
  text: null as Say | null,
  /** The settings file's settings that wait for the next start. */
  pending: [] as string[],
  /** The settings file and when it was last read. */
  file: null as Say | null,
  /** The page follows the server through its restart. */
  following: false,
  /** How Reload now went. */
  reloaded: null as Outcome | null,
});

/** The restart's question in place of its button. */
export const restartAsk = live({ asking: false });

/** The reboot's row, as the last answers left it. */
export const reboot = live({
  /** The delay and Reboot are offered. */
  shown: false,
  /** Take the reboot back is offered: a reboot waits. */
  cancel: false,
  /** Reboot is off while its request runs. */
  off: false,
  /** How the reboot stands, under its title; null hides the line. */
  text: null as Say | null,
  /** How the last request went, or a delay refused. */
  msg: null as Outcome | null,
  /** The page follows the computer through its reboot. */
  following: false,
  /** The delay typed, in seconds, 60 to begin with. */
  delay: "60",
});

/** The reboot's question in place of its button, and the delay it names. */
export const rebootAsk = live({ asking: false, delay: 0 });

/** The settings of the card, for a holder of settings, as GET /api/v1/settings answered them. */
export const settings = live({
  announce: null as Setting | null,
  intervals: {} as Record<string, Setting | null>,
  /** How the announcement's last change went, or why the settings could not be read. */
  announceSaid: null as Outcome | null,
  /** How each interval's last change went, by its setting. */
  intervalSaid: {} as Record<string, Outcome | null>,
});

/** One of the Cube intervals of the card (AP CubePoll v1.0, D119-B). */
export interface Interval {
  /** The setting. */
  key: string;
  /** Its select's id. */
  id: string;
  /** The text key of what it does. */
  hint: string;
  /** Its title's text key. */
  title: string;
  /** The steps offered in seconds, 0 for off, within the API's range. */
  steps: number[];
}

/** INTERVALS are the Cube intervals of the card, in its order. */
export const INTERVALS: Interval[] = [
  { key: "poll_interval", id: "interval-poll", title: "maint.poll", hint: "maint.pollHint", steps: [10, 15, 20, 30, 60, 120, 300, 600, 1800, 3600] },
  { key: "refresh_interval", id: "interval-refresh", title: "maint.refresh", hint: "maint.refreshHint", steps: [0, 1800, 3600, 7200, 10800, 21600, 43200, 86400, 172800, 604800] },
  { key: "discover_interval", id: "interval-discover", title: "maint.discover", hint: "maint.discoverHint", steps: [0, 60, 300, 600, 900, 1800, 3600, 7200, 21600, 43200, 86400] },
  // How long radio commands wait at a full duty cycle before the Cube's
  // budget is read again (AP LegacyLogs v1.0, D143-A).
  { key: "duty_wait", id: "interval-duty", title: "maint.dutyWait", hint: "maint.dutyWaitHint", steps: [60, 300, 600, 900, 1800, 3600] },
];

/** The timers following the server: the restart's step, the reboot's wait or step. */
const timers = { restart: 0 as ReturnType<typeof setTimeout> | 0, reboot: 0 as ReturnType<typeof setTimeout> | 0 };

/** rebootBefore is the start the page waits to see change through a reboot. */
let rebootBefore = "";

/** waitAt is the time of the reboot whose wait the reboot's timer holds, NaN for none. */
let waitAt = NaN;

/**
 * This page's own requests of the settings (a PUT of the announcement or
 * an interval) and of the reboot (asked for, taken back), each with the
 * reads that follow it: how many are under way, and a count of them, so
 * that an event's read of the same thing is dropped while one is under
 * way, and its answer shows nothing where one was made meanwhile.
 */
const owns = { settings: { n: 0, seq: 0, missed: false, what: "settings" }, reboot: { n: 0, seq: 0, missed: false, what: "reboot" } };

/** One of this page's own kinds of request: how many run, a count of them, an event's read skipped meanwhile, and the read it asks for. */
type Own = (typeof owns)["settings"];

/** own runs fn as one of this page's own requests of o. */
async function own<T>(o: Own, fn: () => Promise<T>): Promise<T> {
  o.n++;
  o.seq++;
  try {
    return await fn();
  } finally {
    o.n--;
    // an event's read skipped meanwhile is made once, now
    if (!o.n && o.missed) {
      o.missed = false;
      reader(o.what);
    }
  }
}

/** eventMine says, for an event's read of o begun now, whether its answer may still be shown: no own request of o made or under way since. */
function eventMine(o: Own): () => boolean {
  const seq = o.seq;
  return () => o.seq === seq && !o.n;
}

/** lastRestart is the restart's row as GET /api/v1/restart last gave it, which a settings_read event updates. */
let lastRestart: RestartInfo | null = null;

/** parseGoDuration is a duration as the API writes it (10s, 1m30s, 6h, 0) in seconds; NaN for anything else. */
export function parseGoDuration(text: string | undefined): number {
  const s = String(text || "").trim();
  if (s === "0") return 0;
  if (!s) return NaN;
  const units: Record<string, number> = { h: 3600, m: 60, s: 1 };
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

/** apiDuration writes seconds as the API reads a duration: 10s, 30m, 6h, 0. */
function apiDuration(sec: number): string {
  if (sec === 0) return "0";
  if (sec % 3600 === 0) return sec / 3600 + "h";
  if (sec % 60 === 0) return sec / 60 + "m";
  return sec + "s";
}

/** announceWaits reports whether the stored choice differs from this run's announcement: On or Off chosen, the other in effect. */
export function announceWaits(v: Setting | null): boolean {
  if (!v || v.fixed_by) return false;
  const wants = v.value === "on" ? "on" : v.value === "off" ? "off" : "";
  return !!wants && wants !== v.in_effect;
}

/** intervalWaits reports whether a stored interval differs from this run's, so that it waits for the next start. */
export function intervalWaits(v: Setting | null | undefined): boolean {
  return !!v && !v.fixed_by && parseGoDuration(v.value) !== parseGoDuration(v.in_effect);
}

/** waiting counts what waits for the next start: the settings file's settings and the card's own (the card's badge). */
export function waiting(): number {
  return restart.pending.length + (announceWaits(settings.announce) ? 1 : 0) + INTERVALS.filter((iv) => intervalWaits(settings.intervals[iv.key])).length;
}

/** startedAt is the server's start from /healthz, "" when it does not answer; /healthz is always open, so it needs no sign-in. */
async function startedAt(): Promise<string> {
  try {
    const h = await apiRequest<{ started_at?: string }>("GET", "healthz", undefined, { auth: false });
    return (h && h.started_at) || "";
  } catch {
    return "";
  }
}

/** backAt is the start the server reports, as the page says it. */
function backAt(key: string, now: string): Say {
  return () => t(key, { time: fmtInstant(new Date(Date.parse(now))) });
}

// --- the settings of the card (with settings) ----------------------------

/** loadSettings reads the announcement and the Cube intervals, for a holder of settings. */
export async function loadSettings(mine: () => boolean = () => true): Promise<void> {
  if (!hasCap("settings")) return; // login.js
  const n = session();
  try {
    const all = (await api<Record<string, Setting>>("GET", "api/v1/settings")) || {};
    if (!stillSession(n) || !mine()) return;
    settings.announce = all.announce || null;
    const iv: Record<string, Setting | null> = {};
    for (const x of INTERVALS) iv[x.key] = all[x.key] || null;
    settings.intervals = iv;
  } catch (e) {
    if (stillSession(n) && mine()) settings.announceSaid = { text: t("maint.cannotAsk", { error: (e as Error).message }), ok: false };
  }
  load.bump();
}

/** setAnnounce stores the choice, auto, the default, sent as null; it says how it went. */
export function setAnnounce(value: string): Promise<void> {
  return own(owns.settings, () => putAnnounce(value));
}

/** putAnnounce is setAnnounce's request. */
async function putAnnounce(value: string): Promise<void> {
  const n = session();
  try {
    const all = (await api<Record<string, Setting>>("PUT", "api/v1/settings", { announce: value === "auto" ? null : value })) || {};
    if (!stillSession(n)) return;
    settings.announce = all.announce || settings.announce;
    settings.announceSaid = { text: t("maint.announceSaved"), ok: true };
  } catch (e) {
    if (stillSession(n)) settings.announceSaid = { text: t("maint.announceFailed", { error: (e as Error).message }), ok: false };
  }
  load.bump();
}

/** saveInterval stores the interval chosen, in seconds; the default is sent as null, so that the store keeps no copy of it. */
export function saveInterval(iv: Interval, sec: number): Promise<void> {
  return own(owns.settings, () => putInterval(iv, sec));
}

/** putInterval is saveInterval's request. */
async function putInterval(iv: Interval, sec: number): Promise<void> {
  const v = settings.intervals[iv.key];
  const def = v ? parseGoDuration(v.default) : NaN;
  const n = session();
  let said: Outcome;
  try {
    const all = (await api<Record<string, Setting>>("PUT", "api/v1/settings", { [iv.key]: sec === def ? null : apiDuration(sec) })) || {};
    if (!stillSession(n)) return;
    settings.intervals = { ...settings.intervals, [iv.key]: all[iv.key] || v };
    said = { text: t("maint.intervalSaved"), ok: true };
  } catch (e) {
    if (!stillSession(n)) return;
    said = { text: t("maint.intervalFailed", { error: (e as Error).message }), ok: false };
  }
  settings.intervalSaid = { ...settings.intervalSaid, [iv.key]: said };
  load.bump();
}

// --- the settings file and the restart (with restart) --------------------

/**
 * renderRestart takes GET /api/v1/restart: the restart's button while
 * this page may restart the server, and otherwise why not; the settings
 * that wait for the next start; the settings file and when it was last
 * read. The question goes where the restart is no longer offered.
 */
function renderRestart(v: RestartInfo): void {
  lastRestart = v;
  restart.shown = v.available;
  const reason = v.blocked || "";
  restart.text = !v.available && reason ? () => t("restart.unavailable", { reason }) : () => t("maint.restartText");
  restart.pending = v.pending || [];
  restart.file = () => {
    const file: string[] = [];
    if (v.file) file.push(t("maint.fileText", { file: v.file }));
    if (v.read_at) file.push(t("maint.lastRead", { time: fmtInstant(new Date(Date.parse(v.read_at))) }));
    return file.join(" ");
  };
  if (!v.available) restartAsk.asking = false;
  load.bump();
}

/** loadRestart asks whether the server can be restarted from here, for a holder of restart, not while the page follows a restart. */
export async function loadRestart(): Promise<void> {
  if (!hasCap("restart") || restart.following) return; // login.js
  const n = session();
  try {
    const v = await api<RestartInfo>("GET", "api/v1/restart");
    if (stillSession(n)) renderRestart(v);
  } catch (e) {
    if (!stillSession(n)) return;
    const error = (e as Error).message;
    restart.shown = false;
    restartAsk.asking = false;
    restart.text = () => t("restart.cannotAsk", { error });
  }
}

/** reloadSettings reads the settings file again at once (POST /api/v1/reload), says what changed, and reads the restart's row again. */
export async function reloadSettings(): Promise<void> {
  const n = session();
  try {
    const r = await api<{ changed?: string[] }>("POST", "api/v1/reload", {});
    if (!stillSession(n)) return;
    restart.reloaded = { text: r.changed && r.changed.length ? t("maint.reloaded", { names: r.changed.join(", ") }) : t("maint.reloadedNone"), ok: true };
  } catch (e) {
    if (!stillSession(n)) return;
    restart.reloaded = { text: t("maint.reloadFailed", { error: (e as Error).message }), ok: false };
  }
  await loadRestart();
}

/** serverBack reads the data again and opens the stream again, the server back with a new start. */
function serverBack(): void {
  checkVersion(); // header.js: the start in the settings, and a reload for a new version
  refresh().then(connectEvents);
}

/**
 * followRestart asks /healthz every RESTART_POLL_MS until the server
 * answers with another start than before, then reads the data and opens
 * the stream again; after until it gives up and says so. A sign-out ends
 * it.
 */
function followRestart(n: number, before: string, until: number): void {
  timers.restart = setTimeout(async () => {
    const now = await startedAt();
    if (!stillSession(n)) return;
    if (now && now !== before) {
      restart.following = false;
      restart.off = false;
      restart.text = backAt("restart.back", now);
      serverBack();
      return;
    }
    if (Date.now() > until) {
      restart.following = false;
      restart.off = false;
      restart.text = () => t("restart.notBack");
      return;
    }
    followRestart(n, before, until);
  }, RESTART_POLL_MS);
}

/** runRestart asks the server to restart and follows it until /healthz reports a new start. */
export async function runRestart(): Promise<void> {
  const n = session();
  restart.off = true;
  const before = await startedAt();
  if (!stillSession(n)) return;
  try {
    await api("POST", "api/v1/restart", {});
  } catch (e) {
    if (!stillSession(n)) return;
    const error = (e as Error).message;
    restart.off = false;
    restart.text = () => t("restart.failed", { error });
    return;
  }
  if (!stillSession(n)) return;
  restart.text = () => t("restart.restarting");
  restart.following = true;
  followRestart(n, before, Date.now() + RESTART_WAIT_MS);
}

// --- the reboot of the computer (with reboot) ----------------------------

/**
 * renderReboot takes GET /api/v1/reboot: the delay and the button while
 * this page may reboot the computer, and otherwise why not; while a
 * reboot waits, when, and the button that takes it back; once it has
 * begun, the page follows the computer back. The question goes where
 * the reboot is no longer offered, a reboot waiting among it.
 */
function renderReboot(v: RebootInfo | null): void {
  const waits = !!(v && v.at && !v.begun);
  // the wait this page holds for the same reboot keeps its timer: a read brings data, and moves nothing of it
  const same = waits && !!timers.reboot && Date.parse((v && v.at) || "") === waitAt;
  if (!same) {
    if (timers.reboot) clearTimeout(timers.reboot);
    timers.reboot = 0;
    waitAt = NaN;
  }
  reboot.shown = !!(v && v.available);
  reboot.cancel = waits;
  if (!reboot.shown || waits) rebootAsk.asking = false;
  if (!v) return;
  if (v.begun) {
    followReboot(Date.parse(v.at || ""));
    return;
  }
  if (waits) {
    const at = Date.parse(v.at || "");
    reboot.text = () => t("reboot.waiting", { time: fmtInstant(new Date(at)) });
    // The start the page waits to see change; a page opened while the
    // reboot waits takes the server's start now.
    if (!rebootBefore) {
      const n = session();
      startedAt().then((s) => {
        if (stillSession(n)) rebootBefore = rebootBefore || s;
      });
    }
    if (same) return;
    // setTimeout fires at once beyond 2^31 ms; the delay is an hour at most.
    waitAt = at;
    timers.reboot = setTimeout(() => followReboot(at), Math.min(Math.max(0, at - Date.now()), 2147483647));
    return;
  }
  const text = [v.available ? t("maint.rebootText") : t("reboot.unavailable", { reason: v.blocked || "" })];
  if (v.error) text.push(t("reboot.lastFailed", { error: v.error }));
  const said = text.join(" ");
  reboot.text = () => said;
}

/** loadReboot asks whether the computer can be rebooted from here, for a holder of reboot, not while the page follows a reboot. */
export async function loadReboot(mine: () => boolean = () => true): Promise<void> {
  if (!hasCap("reboot") || reboot.following) return; // login.js
  const n = session();
  try {
    const v = await api<RebootInfo>("GET", "api/v1/reboot");
    if (stillSession(n) && mine()) renderReboot(v);
  } catch (e) {
    if (!stillSession(n) || !mine()) return;
    renderReboot(null);
    const error = (e as Error).message;
    reboot.text = () => t("reboot.cannotAsk", { error });
  }
}

/** rebootDelay is the delay typed, in whole seconds, or null when it is not one the server takes. */
export function rebootDelay(): number | null {
  const raw = reboot.delay.trim();
  const n = Number(raw);
  return raw !== "" && Number.isInteger(n) && n >= 0 && n <= REBOOT_DELAY_MAX ? n : null;
}

/** runReboot asks the server for the reboot after delay seconds, then draws the wait, or follows the computer at once. */
export function runReboot(delay: number): Promise<void> {
  return own(owns.reboot, () => postReboot(delay));
}

/** postReboot is runReboot's request and the read that follows it. */
async function postReboot(delay: number): Promise<void> {
  const n = session();
  reboot.off = true;
  rebootBefore = await startedAt();
  if (!stillSession(n)) return;
  try {
    await api("POST", "api/v1/reboot", { delay });
  } catch (e) {
    if (!stillSession(n)) return;
    reboot.off = false;
    reboot.msg = { text: t("reboot.failed", { error: (e as Error).message }), ok: false };
    return;
  }
  if (!stillSession(n)) return;
  reboot.off = false;
  await loadReboot();
}

/** cancelReboot takes a reboot that waits back (DELETE /api/v1/reboot). */
export function cancelReboot(): Promise<void> {
  return own(owns.reboot, deleteReboot);
}

/** deleteReboot is cancelReboot's request and the read that follows it. */
async function deleteReboot(): Promise<void> {
  const n = session();
  try {
    await api("DELETE", "api/v1/reboot");
    if (!stillSession(n)) return;
    reboot.msg = { text: t("reboot.cancelled"), ok: true };
  } catch (e) {
    if (!stillSession(n)) return;
    reboot.msg = { text: t("reboot.cancelFailed", { error: (e as Error).message }), ok: false };
  }
  await loadReboot();
}

/**
 * followReboot follows the computer from the reboot's time: it asks
 * /healthz every RESTART_POLL_MS until the server answers with another
 * start than before, then reads the data and opens the stream again;
 * while the old server still answers it asks whether the reboot failed,
 * and after REBOOT_WAIT_MS it gives up and says so. A sign-out ends it.
 */
function followReboot(at: number): void {
  if (timers.reboot) clearTimeout(timers.reboot);
  timers.reboot = 0;
  waitAt = NaN;
  if (reboot.following) return;
  const n = session();
  reboot.following = true;
  reboot.shown = false;
  reboot.cancel = false;
  rebootAsk.asking = false;
  reboot.text = () => t("reboot.rebooting");
  const until = Math.max(at || 0, Date.now()) + REBOOT_WAIT_MS;
  const step = async () => {
    timers.reboot = 0;
    const now = await startedAt();
    if (!stillSession(n)) return;
    if (!rebootBefore && now) rebootBefore = now; // a page that saw no request of its own
    if (now && now !== rebootBefore) {
      reboot.following = false;
      reboot.text = backAt("reboot.back", now);
      serverBack();
      return;
    }
    if (now) {
      // The old server still answers: the reboot may have failed.
      try {
        const v = await api<RebootInfo>("GET", "api/v1/reboot");
        if (!stillSession(n)) return;
        if (!v.at) {
          reboot.following = false;
          renderReboot(v);
          return;
        }
      } catch {
        // going down
      }
      if (!stillSession(n)) return;
    }
    if (Date.now() > until) {
      reboot.following = false;
      reboot.text = () => t("reboot.notBack");
      return;
    }
    timers.reboot = setTimeout(step, RESTART_POLL_MS);
  };
  timers.reboot = setTimeout(step, RESTART_POLL_MS);
}

/** SHOWN_SETTINGS are the settings the card shows: a change event that sets one reads them again. */
const SHOWN_SETTINGS = ["announce", ...INTERVALS.map((iv) => iv.key)];

/**
 * The card's reads for the stream's events, one at a time while
 * Maintenance is shown: the settings, the restart's row, the reboot's;
 * the settings' and the reboot's dropped while this page's own request of
 * them is under way, their answers showing nothing where one was made
 * meanwhile.
 */
const reader = burstReader(async (what) => {
  for (const o of [owns.settings, owns.reboot]) if (what.has(o.what) && o.n) o.missed = true; // read once the own request has ended
  if (what.has("settings") && !owns.settings.n) await loadSettings(eventMine(owns.settings));
  if (what.has("restart")) await loadRestart();
  if (what.has("reboot") && !owns.reboot.n) await loadReboot(eventMine(owns.reboot));
}, () => isShown("maint"));

/**
 * pendingOf takes a settings_read event, which carries what waits for the
 * next start (at_next_start) and when the file was read: the restart's
 * row shows them with nothing read. Not while the page follows a restart.
 */
function pendingOf(ev: ServerEvent): void {
  const p = (ev.summary || {}).at_next_start;
  if (!hasCap("restart") || restart.following || !lastRestart || !Array.isArray(p)) return;
  renderRestart({ ...lastRestart, pending: p.map(String), read_at: ev.time || lastRestart.read_at });
}

/**
 * serverEvent reads again, while Maintenance is shown, what an event of
 * the stream says has changed: everything the card shows at a resync of
 * the stream, which is how the page learns of a restart (the server's
 * start is written before any stream opens); the announcement or an
 * interval set (change); a reboot asked for, begun, failed or taken back
 * (reboot, reboot_cancelled); a settings_read event says itself what
 * waits for the next start. A restart or a reboot
 * this page follows reads by its own pace (loadRestart and loadReboot
 * read nothing meanwhile), and a reboot's wait keeps its timer.
 */
export function serverEvent(ev: ServerEvent): void {
  if (!isShown("maint")) return;
  if (settingsChanged(ev).some((k) => SHOWN_SETTINGS.includes(k))) reader("settings");
  if (ev.kind === "resync") reader("settings", "restart", "reboot");
  else if (ev.kind === "settings_read") pendingOf(ev);
  else if (ev.kind === "reboot" || ev.kind === "reboot_cancelled") reader("reboot");
}

/** quiet takes the settings' messages back, as Maintenance is shown. */
export function quiet(): void {
  settings.announceSaid = null;
  settings.intervalSaid = {};
}

/**
 * forgetMaintenance forgets what the card kept of the user who was
 * signed in (MassimoParts.signedOut): the following of a restart or a
 * reboot ends with its timers, the questions go, the delay typed is 60
 * again, and the restart's, the reboot's and the settings' rows are read
 * again for the next user.
 */
export function forgetMaintenance(): void {
  if (timers.restart) clearTimeout(timers.restart);
  if (timers.reboot) clearTimeout(timers.reboot);
  timers.restart = 0;
  timers.reboot = 0;
  rebootBefore = "";
  waitAt = NaN;
  lastRestart = null;
  Object.assign(restart, { shown: false, off: false, text: null, pending: [], file: null, following: false, reloaded: null });
  Object.assign(reboot, { shown: false, cancel: false, off: false, text: null, msg: null, following: false, delay: "60" });
  restartAsk.asking = false;
  rebootAsk.asking = false;
  Object.assign(settings, { announce: null, intervals: {}, announceSaid: null, intervalSaid: {} });
  load.bump();
}
