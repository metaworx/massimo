// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/log/store.ts
 *
 * What the log is drawn from, read and kept (AP UiParts, Block 11; line
 * for line from log.js): the selected Cube's events from GET …/log, or
 * every Cube's from GET /api/v1/log under All Cubes, newest first, a page
 * at a time with Older at the bottom back through the server's log, the
 * live events of the stream coming in at the top (under All Cubes those a
 * poll brings), with a text filter and the choices of Cubes, kind,
 * device, level, who and how far back (the range).
 *
 * The lines are logState.lines; view is the lines drawn, newest first,
 * followed in place: a new line is put on top and the oldest taken out,
 * the whole list worked out again only where log.js drew it again
 * (renderLog). Each line keeps a key while it is the same event
 * (keyLines), so that its element is kept through a load. The scroll of
 * the lines' box is moved at the moments log.js moved it, once the part
 * has drawn (afterDraw). An answer of an older load, choice or Cube is
 * dropped by logState.gen, and one of the session before by its number
 * (shared/session.ts).
 */

import { signal } from "@preact/signals";
import { can, me } from "../shared/me";
import { setAddress } from "../shared/pages";
import { session, stillSession } from "../shared/session";
import { LOG_COUNT, LOG_COUNT_RANGE, LOG_LEVELS, LOG_MAX_LINES, LOG_PAGE, LOG_POLL_MS, LOG_POLL_PAGES } from "./kinds";
import { errorLine, type LogEvent, type LogLine, logLine } from "./lines";
import { type CubeRooms, type LogUser, logState } from "./state";
import { logDeviceName, logDeviceSerial, logKind, logLineCube, logRangeStart, logRoomsOf, logText } from "./text";
import { logHash } from "./address";
import { keepLevelsOf, logAlerts, markLogSeen } from "./alerts";
import { cubesKey, settleChoices } from "./choices";

/** view is the lines drawn, newest first: those of logState.lines that match the filter and the choices. */
export const view = signal<LogLine[]>([]);

/** The lines' box, which the part hands over as it draws it; null before. */
let box: HTMLElement | null = null;

/** What is done to the box's scroll once the part has drawn the change that queued it. */
const steps: (() => void)[] = [];

/** setBox takes the lines' box from the part (Log.tsx). */
export function setBox(el: HTMLElement | null): void {
  box = el;
}

/** A place in the box: a line's element and how far below the box's top it stands. */
interface Place {
  el: Element | null;
  top: number;
}

/** anchor is where the line the user reads stood before the first new line on top since the last drawing, null for none: lines that come together move the view once. */
let anchor: Place | null = null;

/** drawn runs the scroll steps queued for the drawing just done (Log.tsx, in its layout effect). */
export function drawn(): void {
  anchor = null;
  for (const step of steps.splice(0)) step();
}

/**
 * placeOf is the first line in the box's view and how far below the box's
 * top it stands, among all but the last cut lines (those that are about
 * to go, at the bottom): where the view shows only those, the last line
 * that stays; no element for an empty box.
 */
function placeOf(b: HTMLElement, cut: number): Place {
  const top = b.getBoundingClientRect().top;
  const els = b.children;
  const stays = Math.max(0, els.length - cut);
  for (let i = 0; i < stays; i++) {
    const r = els[i].getBoundingClientRect();
    if (r.bottom > top) return { el: els[i], top: r.top - top };
  }
  const last = stays ? els[stays - 1] : null;
  return last ? { el: last, top: last.getBoundingClientRect().top - top } : { el: null, top: 0 };
}

/** afterDraw queues a step for the box once view's change is drawn; nothing without a box. */
function afterDraw(step: (b: HTMLElement) => void): void {
  const b = box;
  if (b) steps.push(() => step(b));
}

/** scrolled follows the user's scroll of the lines: the view sticks to the newest while at the top. */
export function scrolled(top: number): void {
  logState.stick = top <= 8;
}

/** logAtLevel reports whether a line is at the level chosen or above; a line of before the levels counts as info. */
function logAtLevel(line: LogLine): boolean {
  return LOG_LEVELS.indexOf(line.level || "info") >= LOG_LEVELS.indexOf(logState.level);
}

/** logOfChoices reports whether a line is of the source and the user chosen. */
export function logOfChoices(line: LogLine): boolean {
  return (!logState.source || line.source === logState.source) && (!logState.user || (!!line.by && line.by.name === logState.user) || line.realName === logState.user);
}

/**
 * logOfDevice reports whether a line is of the device chosen, named by
 * its S/N or its radio address; a line stored before events carried the
 * S/N is the chosen device's by its radio address on the line's Cube.
 */
function logOfDevice(line: LogLine, key: string): boolean {
  if (!key || line.serial === key || line.device === key) return true;
  const rs = logRoomsOf(line.cubeSerial);
  if (!rs) return false;
  for (const r of rs.rooms) if (r.devices.some((d) => d.serial === key && d.rf_address === line.device)) return true;
  return rs.unlisted.some((d) => d.serial === key && d.rf_address === line.device);
}

/** logInQuery reports whether a line is of the kind and the device chosen, as the server answers them. */
function logInQuery(line: LogLine): boolean {
  return (!logState.kind || line.kind === logState.kind) && logOfDevice(line, logState.device) && logAtLevel(line) && logOfChoices(line);
}

/** logLineMatches reports whether a line is drawn: of the choices, and with the text filter in its words. */
export function logLineMatches(line: LogLine): boolean {
  if (line.kind === "error") return true; // the page's own line: the log could not load
  if (logState.kind && line.kind !== logState.kind) return false;
  if (!logOfDevice(line, logState.device) || !logAtLevel(line) || !logOfChoices(line)) return false;
  const f = logState.filter.trim().toLowerCase();
  if (!f) return true;
  const hay = [fmtTimeOfDay(line.time), line.kind, logKind(line.kind), logLineCube(line), line.device, logDeviceSerial(line), logDeviceName(line), logText(line)]
    .join(" ")
    .toLowerCase();
  return hay.includes(f);
}

/** sameEvent reports whether a line is the same event as a live one, which has no id (loadLog, a read on): by sameKey. */
function sameEvent(a: LogLine, b: LogLine): boolean {
  return sameKey(a) === sameKey(b);
}

/** idKey is a line's key by its event's id, with its Cube under All Cubes: ids of two Cubes' memories can be equal. */
function idKey(line: LogLine): string {
  return line.cubeSerial + ":" + line.id;
}

/**
 * sameKey names a line by what makes a live line, which has no id, the
 * same event as a line of a page loaded after it, as loadLog decides it
 * (logSameLine cannot: one has an id, the other none): its time as the
 * server sent it, its kind and its device.
 */
function sameKey(line: LogLine): string {
  return "same:" + line.at + "\u0000" + line.kind + "\u0000" + line.device;
}

/**
 * keyLines gives each line that comes in its key: the one a line it
 * replaces (before) had for the same event, by its id, or for one that
 * came live, without an id, by sameEvent, so that the element of an
 * event once drawn is kept; else its id's. A key the lines kept beside
 * them hold (kept) is not given again, nor one twice: a second line of
 * the same key gets a count after it.
 */
function keyLines(lines: LogLine[], before: LogLine[], kept: LogLine[]): void {
  const was = new Map<string, string>();
  for (const l of before) {
    if (!l.key) continue;
    was.set(l.id ? idKey(l) : sameKey(l), l.key);
  }
  const used = new Set<string>();
  for (const l of kept) if (l.key) used.add(l.key);
  for (const l of lines) {
    if (l.key && !used.has(l.key)) {
      used.add(l.key);
      continue;
    }
    const own = l.id ? idKey(l) : sameKey(l);
    let k = (l.id ? was.get(own) : undefined) || was.get(sameKey(l)) || own;
    for (let n = 2; used.has(k); n++) k = own + "#" + n;
    used.add(k);
    l.key = k;
  }
}

/** draw works view out again from the lines, newest first, and keeps the newest in view where the view sticks to it (renderLog). */
function draw(): void {
  const out: LogLine[] = [];
  for (let i = logState.lines.length - 1; i >= 0; i--) {
    const line = logState.lines[i];
    if (logLineMatches(line)) out.push(line);
  }
  view.value = out;
  keepLevelsOf(logState.lines);
  const stick = logState.stick;
  afterDraw((b) => {
    if (stick) b.scrollTop = 0;
  });
}

/** renderLog works the lines shown out again, as the filter matches them now: the rooms came, the language changed; each line follows its data and the language itself. */
export function renderLog(): void {
  draw();
}

/** logCursor is where the next older page starts: the oldest line's id, else its time as the server sent it (a live line of the stream). */
function logCursor(): string | null {
  const first = logState.lines[0];
  if (!first) return null;
  return first.id ? String(first.id) : first.at || null;
}

/** logURL is where a page of the log comes from: every Cube's under All Cubes, else the header's Cube's. */
function logURL(): string {
  return logState.cubes === "all" ? "api/v1/log" : base() + "/log";
}

/**
 * logQuery is the query of a page: the size (LOG_COUNT for the count,
 * else LOG_PAGE), the start of a time range, or the line's id given (a
 * poll's, after which the page reads forward: LOG_PAGE whatever the
 * range, and no time), the kind and the device chosen, and where it
 * starts.
 */
function logQuery(before?: string | null, after?: number): string {
  let q = "?limit=" + (logState.range === LOG_COUNT_RANGE && !after ? LOG_COUNT : LOG_PAGE);
  const start = logRangeStart(logState.range);
  if (after) q += "&after=" + after;
  else if (start) q += "&since=" + encodeURIComponent(start.toISOString());
  if (logState.kind) q += "&kind=" + encodeURIComponent(logState.kind);
  if (logState.source) q += "&source=" + logState.source;
  if (logState.device) q += "&device=" + encodeURIComponent(logState.device);
  if (logState.level !== "info") q += "&level=" + logState.level;
  if (logState.user) q += "&user=" + encodeURIComponent(logState.user);
  if (before) q += "&before=" + encodeURIComponent(before);
  return q;
}

/**
 * appendLogEvent adds a live event of the stream; under All Cubes the
 * polls bring every Cube's (D9-A).
 */
export function appendLogEvent(ev: LogEvent): void {
  if (logState.cubes === "all") return;
  const line = logLine(ev);
  line.seq = ++logState.liveSeq;
  line.key = "live:" + line.seq;
  addLogLines([line]);
}

/**
 * addLogLines puts new lines on top, oldest first; the view is updated
 * in place when it is open, the lines on top, once for the lines that
 * come together, and a view scrolled down keeps the line it shows first
 * where it is. The oldest lines go, at the bottom: with the count, those
 * past the newest LOG_COUNT of the kind and the device chosen; else those
 * past the room: after Older, the room holds the pages fetched and
 * LOG_MAX_LINES more, so that a live event does not take back a page
 * just fetched.
 */
function addLogLines(batch: LogLine[]): void {
  if (!batch.length) return;
  for (const l of batch) logState.lines.push(l);
  let cut = 0;
  if (logState.range === LOG_COUNT_RANGE) {
    for (let i = logState.lines.length - 1, n = 0; i >= 0; i--) {
      if (logInQuery(logState.lines[i]) && ++n > LOG_COUNT) {
        cut = i + 1;
        break;
      }
    }
  } else {
    const room = Math.max(logState.room, LOG_MAX_LINES);
    if (logState.lines.length > room) cut = logState.lines.length - room;
  }
  let gone: LogLine[] = [];
  if (cut) {
    gone = logState.lines.splice(0, cut);
    logState.cursor = logCursor(); // Older goes on from the oldest line kept
    logState.more = logState.range !== LOG_COUNT_RANGE;
    keepLevelsOf(logState.lines);
  }
  if (!logState.open) return;
  const out = new Set(gone);
  const fresh: LogLine[] = [];
  for (let i = batch.length - 1; i >= 0; i--) if (!out.has(batch[i]) && logLineMatches(batch[i])) fresh.push(batch[i]);
  const kept = gone.length ? view.value.filter((l) => !out.has(l)) : view.value; // the view drops them too
  if (!fresh.length && kept === view.value) return;
  if (fresh.length && box && !anchor) {
    // where the line read first stands now, before anything of the box changes
    const place = (anchor = logState.stick ? { el: null, top: 0 } : placeOf(box, view.value.length - kept.length));
    afterDraw((b) => {
      if (logState.stick) b.scrollTop = 0;
      else if (place.el && place.el.isConnected) b.scrollTop += place.el.getBoundingClientRect().top - b.getBoundingClientRect().top - place.top;
    });
  }
  view.value = fresh.length ? fresh.concat(kept) : kept;
}

/**
 * loadLog replaces the lines with the Cube's newest events of the kind
 * and the device chosen; Older waits for it. An answer that comes after
 * a Clear, another choice, a Cube switch or a sign-out is dropped. The
 * live events that came while it waited follow the page, but for one the
 * page holds already: the server may have stored it before it answered.
 */
export async function loadLog(): Promise<void> {
  if (!state.serial) return;
  logLoadCubeRooms();
  const gen = ++logState.gen;
  const n = session();
  const since = logState.liveSeq;
  logState.loading = true;
  let evs: LogEvent[] | null = null;
  let failed: Error | null = null;
  try {
    evs = (await api<LogEvent[]>("GET", logURL() + logQuery())) || [];
  } catch (e) {
    failed = e as Error;
  }
  if (gen !== logState.gen) return;
  logState.loading = false;
  if (!stillSession(n)) return;
  const before = logState.lines;
  const live = before.filter((l) => (l.seq || 0) > since);
  let lines: LogLine[];
  if (failed || !evs) {
    lines = [errorLine(t("log.cannotLoad", { error: failed ? failed.message : "" }))];
    lines[0].key = "error:" + gen;
    logState.cursor = null;
    logState.more = false;
  } else {
    lines = evs.map(logLine);
  }
  // the live lines that came while it waited, but those the page holds already
  const page = lines;
  const kept = live.filter((l) => !page.some((x) => sameEvent(x, l)));
  if (!failed && evs) {
    keyLines(lines, before, kept);
    logState.lines = lines;
    logState.cursor = logCursor();
    logState.more = logState.range !== LOG_COUNT_RANGE && evs.length >= LOG_PAGE;
  }
  lines = lines.concat(kept);
  logState.lines = lines;
  logState.room = LOG_MAX_LINES;
  logState.stick = true;
  draw();
  flowEnded();
}

/**
 * loadOlder puts the page before the oldest line under the lines, the
 * view staying where it was; an answer that comes after a Clear, another
 * choice, a Cube switch or a sign-out is dropped. Where the log the lines
 * came from is gone (logGone), it loads the log again instead.
 */
export async function loadOlder(): Promise<void> {
  if (logState.loading || !logState.more || logState.cursor === null || !state.serial) return;
  const gen = logState.gen;
  const n = session();
  logState.loading = true;
  const gone = await logGone();
  if (gen !== logState.gen) return;
  if (!stillSession(n)) {
    logState.loading = false;
    return;
  }
  if (gone) return loadLog();
  const top = box ? box.scrollTop : 0;
  let evs: LogEvent[] | null = null;
  let failed: Error | null = null;
  try {
    evs = (await api<LogEvent[]>("GET", logURL() + logQuery(logState.cursor))) || [];
  } catch (e) {
    failed = e as Error;
  }
  if (gen !== logState.gen) return;
  logState.loading = false;
  if (!stillSession(n)) return;
  if (failed || !evs) {
    showMessage(t("log.cannotLoad", { error: failed ? failed.message : "" }), false);
  } else {
    const older = evs.map(logLine);
    keyLines(older, [], logState.lines);
    logState.lines = older.concat(logState.lines);
    logState.more = evs.length >= LOG_PAGE;
    if (evs.length) logState.cursor = logCursor();
    logState.room = logState.lines.length + LOG_MAX_LINES;
  }
  const stick = logState.stick;
  logState.stick = false;
  draw();
  logState.stick = stick;
  afterDraw((b) => {
    b.scrollTop = top; // the older lines came in below
  });
  flowEnded();
}

/** logSameLine reports whether two lines are one event: by its id, else (a line without one, as the error line) by its time, kind, device and Cube. */
function logSameLine(a: LogLine, b: LogLine): boolean {
  if (a.id || b.id) return a.id === b.id;
  return a.at === b.at && a.kind === b.kind && a.device === b.device && a.cubeSerial === b.cubeSerial;
}

/** logNewestID is the id of the newest line the view holds of the server's log, 0 for none: a live line of the stream has no id. */
function logNewestID(): number {
  let newest = 0;
  for (const l of logState.lines) if (l.id > newest) newest = l.id;
  return newest;
}

/**
 * logGone reports whether the log the lines came from is no longer the
 * server's: the server answers 409 to the newest line's id as after=,
 * a line its log never gave out (AP LogCursor v1.0, D3-A). That is a
 * Cube served for this run that has gained its row in the store, whose
 * log is then the store's, its ids far below those of the lines it kept
 * in memory, so that a page before the oldest line would be the store's
 * newest, shown under the lines as if older; a store restored from an
 * older copy; a server without a store restarted with its clock set
 * back. The prune never makes one, since it keeps the log's newest row
 * (AP LogEdges, D5-A); a line it removed gives no sign (LogCursor's
 * D4-A). A question that fails says nothing is gone.
 */
async function logGone(): Promise<boolean> {
  const newest = logNewestID();
  if (!newest) return false;
  try {
    await api("GET", logURL() + "?limit=1&after=" + newest);
    return false;
  } catch (e) {
    return (e as { status?: number }).status === 409;
  }
}

/**
 * readOn asks for the lines after the newest one by its id (AP LogCubes
 * v1.0, D9-A; AP LogCursor v1.0, D6-A): under All Cubes the poll's,
 * every Cube's; page after page while a page is full, so that each comes
 * once, a burst longer than a page too; a log without a line of the
 * server's asks for its newest page, as loadLog does. A line at or below
 * the cursor is dropped: a server before after= answers its newest page,
 * which may reach back before a time range; so is one a live line of the
 * stream holds already (sameEvent). The lines the log does not hold yet
 * come in on top as live lines do, together, once the pages are in. Past
 * LOG_POLL_PAGES full pages, and at a 409, a line the server's log never
 * gave out (logGone), it loads the log again instead. An answer that
 * comes after a new page, another choice, Clear or a sign-out is
 * dropped; one that fails otherwise leaves the rest to the next read.
 */
async function readOn(all: boolean): Promise<void> {
  const gen = logState.gen;
  const n = session();
  const current = () => gen === logState.gen && logState.open && (logState.cubes === "all") === all && stillSession(n);
  const url = logURL();
  let after = logNewestID();
  let reload = false;
  const got: LogLine[] = [];
  logState.polling = true;
  try {
    for (let pages = 1; ; pages++) {
      const evs = (await api<LogEvent[]>("GET", url + logQuery(null, after))) || [];
      if (!current()) break;
      for (const ev of evs) {
        const line = logLine(ev);
        if (after ? line.id <= after : logState.lines.some((l) => logSameLine(l, line))) continue;
        if (logState.lines.some((l) => !l.id && sameEvent(l, line))) continue;
        got.push(line);
      }
      const last = evs.length ? evs[evs.length - 1].id || 0 : 0;
      if (!after || evs.length < LOG_PAGE || last <= after) break;
      if (pages === LOG_POLL_PAGES) {
        reload = true;
        break;
      }
      after = last;
    }
  } catch (e) {
    reload = (e as { status?: number }).status === 409;
  }
  logState.polling = false;
  if (!current()) return;
  if (reload) {
    loadLog();
    return;
  }
  keyLines(got, [], logState.lines);
  addLogLines(got);
  flowEnded();
}

/**
 * pollLog asks, under All Cubes while the log is shown, for the lines
 * after the newest one (readOn); none while a page or a poll is on its
 * way.
 */
export async function pollLog(): Promise<void> {
  if (!logState.open || logState.cubes !== "all" || logState.loading || logState.polling) return;
  return readOn(true);
}

/** logPollSet runs pollLog every LOG_POLL_MS while the log is shown under All Cubes, and stops it otherwise. */
export function logPollSet(): void {
  const want = logState.open && logState.cubes === "all";
  if (want && !logState.pollTimer) logState.pollTimer = setInterval(pollLog, LOG_POLL_MS);
  if (!want && logState.pollTimer) {
    clearInterval(logState.pollTimer);
    logState.pollTimer = null;
  }
}

/**
 * logLoadCubeRooms loads under All Cubes the rooms and the unlisted list
 * of each Cube the page has none of yet, once (D8-A), and names the lines
 * again once they are there; a Cube that does not answer stays without,
 * its lines named by what they kept.
 */
async function logLoadCubeRooms(): Promise<void> {
  if (logState.cubes !== "all") return;
  const todo = state.cubes.filter((c) => !logState.cubeRooms.has(c.serial));
  if (!todo.length) return;
  const n = session();
  await Promise.all(
    todo.map(async (c) => {
      logState.cubeRooms = new Map(logState.cubeRooms).set(c.serial, null); // written whole, a signal (log/state.ts)
      try {
        const rs: CubeRooms = await fetchRooms(c.serial);
        if (stillSession(n)) logState.cubeRooms = new Map(logState.cubeRooms).set(c.serial, rs);
      } catch {
        // its lines keep the names they came with
      }
    }),
  );
  if (!logState.open || !stillSession(n)) return;
  if (fillLogSelects()) logChoicesChanged();
  else renderLog();
}

/** Whom the who choice's users were read for, by login name, "" for nobody. */
let usersFor = "";

/**
 * logLoadUsers fills the who choice once per user signed in,
 * for a caller who may list the users; for others, and on a server
 * without users, it stays hidden.
 */
export async function logLoadUsers(): Promise<void> {
  const who = me.peek()?.user?.name || "";
  if (logState.users !== null && usersFor === who) return;
  usersFor = who; // another user signed in without a sign-out before (a key pasted, the first owner made): asked again
  logState.users = [];
  if (!me.value || me.value.mode !== "users" || !can("users")) return;
  const n = session();
  let users: LogUser[];
  try {
    users = (await api<LogUser[]>("GET", "api/v1/users")) || [];
  } catch {
    users = [];
  }
  if (!stillSession(n)) return;
  logState.users = users;
  fillLogSelects();
}

/**
 * logCubesPolled follows the Cubes a poll found while the log is shown,
 * when they changed: the Cubes choice, shown with more than one, and
 * under All Cubes the rooms of a Cube new to the page.
 */
export function logCubesPolled(): void {
  if (!logState.open || cubesKey() === logState.cubesSeen) return;
  if (fillLogSelects()) logChoicesChanged();
  else renderLog();
  logLoadCubeRooms();
}

/**
 * pruneLogRange drops the lines a time range no longer holds, now or at
 * the time given; the age timer of app.js calls it every minute. Every
 * event of the range is then in the view, so Older has nothing more.
 */
export function pruneLogRange(now = new Date()): void {
  const start = logRangeStart(logState.range, now);
  if (!start) return;
  const keep = logState.lines.findIndex((l) => l.kind === "error" || l.time >= start);
  const cut = keep === -1 ? logState.lines.length : keep;
  if (!cut) return;
  logState.lines = logState.lines.slice(cut);
  logState.cursor = logCursor();
  logState.more = false;
  if (logState.open) draw();
}

/** fillLogSelects settles the choices against the Cubes and devices there are and draws them again; true where a choice changed so. */
export function fillLogSelects(): boolean {
  const changed = settleChoices();
  return changed;
}

/**
 * logChoicesChanged follows a new range, kind or device: the address in
 * place of the current entry while the log is shown (logHash), and the
 * log loaded again; a page on its way is dropped (loadLog).
 */
export function logChoicesChanged(): void {
  if (logState.open) setAddress(logHash(), "replace");
  logPollSet();
  loadLog();
}

/** filterTyped takes the text filter as typed and draws the lines that match. */
export function filterTyped(text: string): void {
  logState.filter = text;
  draw();
}

/**
 * clearLogFilter sets the text filter, the Cubes, the kind, the source or
 * device, the level and who back to their defaults and loads the log
 * again.
 */
export function clearLogFilter(): void {
  logState.filter = "";
  logState.cubes = "";
  logState.kind = "";
  logState.device = "";
  logState.source = "";
  logState.user = "";
  logState.level = "info";
  fillLogSelects(); // the devices of the header's Cube again
  logChoicesChanged();
}

/**
 * logCubeChanged starts the log over for another Cube: the device chosen
 * back to all, the lines and the paging reset, an answer still on its way
 * dropped, and, when the view is open, the address without the device and
 * the Cube's log loaded. Under All Cubes the log stays as it is, every
 * Cube's, and only names its lines again.
 */
export function logCubeChanged(): void {
  if (logState.cubes === "all") {
    if (logState.open) renderLog();
    return;
  }
  logState.gen++;
  logState.device = "";
  logState.lines = [];
  logState.cursor = null;
  logState.more = false;
  logState.loading = false;
  logState.room = LOG_MAX_LINES;
  if (!logState.open) return;
  setAddress(logHash(), "replace");
  draw();
  loadLog();
}

/** toggleLogCommand opens or closes the detail of a command under each of its lines, loading it the first time. */
export async function toggleLogCommand(id: number): Promise<void> {
  const open = new Set(logState.openCommands);
  if (open.has(id)) {
    open.delete(id);
    logState.openCommands = open;
    return;
  }
  open.add(id);
  logState.openCommands = open;
  if (logState.commands.has(id)) return;
  const n = session();
  let detail: Record<string, unknown>;
  try {
    detail = (await api<Record<string, unknown>>("GET", "api/v1/commands/" + id)) || {};
  } catch (e) {
    detail = { error: (e as { status?: number }).status === 404 ? t("log.cmd.missing") : (e as Error).message };
  }
  if (!stillSession(n)) return;
  const commands = new Map(logState.commands);
  commands.set(id, detail);
  logState.commands = commands;
}

/** Where the focus goes once the drawing is done: a line's Mark seen by its id, "search" for the search; null for nowhere. */
export const markFocus = signal<number | "search" | null>(null);

/**
 * seen marks a line seen from its own Mark seen, which then goes, and
 * hands the focus on, in the drawing's own turn (Line.tsx, Log.tsx): to
 * the Mark seen of the next problem below it in the view, else of the one
 * above, else, with none left, to the search.
 */
export function seen(id: number): void {
  const lines = view.value;
  const i = lines.findIndex((l) => l.id === id);
  const problem = (l: LogLine) => !!l.id && l.id !== id && !!logAlerts.levels.get(l.id);
  let to: number | "search" = "search";
  for (let j = i + 1; j < lines.length; j++) if (problem(lines[j])) {
    to = lines[j].id;
    break;
  }
  if (to === "search") for (let j = i - 1; j >= 0; j--) if (problem(lines[j])) {
    to = lines[j].id;
    break;
  }
  markFocus.value = to;
  markLogSeen(id);
  // taken in the drawing's own turn where its button is still there; else it goes, so that no later line takes it
  setTimeout(() => {
    if (markFocus.peek() === to) markFocus.value = null;
  }, 0);
}

/**
 * forgetLog forgets the log of the user who was signed in, at a sign-out
 * and for another user's sign-in after a 401 (MassimoParts.signedOut):
 * the lines and the paging, the text typed in the filter, the users of
 * the who choice, the commands' details and which are open, every Cube's
 * rooms; an answer still on its way is dropped (logState.gen, the
 * session's number). The choices stay: they are the log's address.
 */
export function forgetLog(): void {
  logState.gen++;
  logState.lines = [];
  logState.cursor = null;
  logState.more = false;
  logState.loading = false;
  logState.polling = false;
  logState.room = LOG_MAX_LINES;
  logState.stick = true;
  logState.filter = "";
  logState.users = null;
  logState.openCommands = new Set();
  logState.commands = new Map();
  logState.cubeRooms = new Map();
  markFocus.value = null;
  view.value = [];
}

/** A resync noted while the page's own load, Older or read on was under way: its generation and session. */
let resyncFor: { gen: number; n: number } | null = null;

/**
 * serverEvent follows the stream's events beside its lines: at a resync,
 * a Cube's own log shown reads on from its newest line (readOn), or loads
 * where it holds no stored line (resynced), for the
 * lines the stream missed (UiParts D208), the place and what is open kept;
 * not while the page's own load, Older or read on is under way, but once
 * after it (flowEnded).
 */
export function serverEvent(ev: { kind: string }): void {
  if (ev.kind !== "resync" || !logState.open || logState.cubes === "all") return;
  if (logState.loading || logState.polling) {
    resyncFor = { gen: logState.gen, n: session() };
    return;
  }
  resynced();
}

/** resynced reads on from the newest stored line; where the log holds none (a load that failed, a log that loaded empty), there is no place to read on from: it loads the log. */
function resynced(): void {
  if (logNewestID()) readOn(false);
  else loadLog();
}

/** flowEnded reads on once for a resync that came while the page's own flow ran, in the same generation and session, the Cube's log still shown. */
function flowEnded(): void {
  const r = resyncFor;
  if (!r) return;
  resyncFor = null;
  if (r.gen !== logState.gen || !stillSession(r.n) || !logState.open || logState.cubes === "all" || logState.loading || logState.polling) return;
  resynced();
}
