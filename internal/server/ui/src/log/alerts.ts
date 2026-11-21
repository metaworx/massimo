// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/log/alerts.ts
 *
 * The log's unseen problems (AP LogAlerts; AP UiParts, Block 11, line for
 * line from log.js): the worst of the log's warnings and errors that
 * nothing has settled (GET /api/v1/log/alerts) and the person has not
 * marked seen, which ☰ and its Log entry show (MassimoParts.chrome.log)
 * and the log highlights. A line is marked with its Mark seen, every one
 * with Mark all as seen; opening the log marks nothing. The marks follow
 * a signed-in user with the capability preferences (the preference
 * log.seen); else this browser keeps them. The page global logAlerts
 * (log.js) is this file's state.
 */

import { computed, type ReadonlySignal } from "@preact/signals";
import { live } from "../shared/live";
import { session, stillSession } from "../shared/session";
import type { LogLine } from "./lines";

/** LOG_SEEN_KEY is where a browser keeps its marks for nobody signed in. */
const LOG_SEEN_KEY = "massimo.log.seen";

/** LOG_SEEN_LINES is how many lines are kept marked one by one, the server's limit. */
const LOG_SEEN_LINES = 500;

/** An unsettled problem of the log, as GET /api/v1/log/alerts lists it. */
export interface LogAlert {
  id: number;
  level: string;
  kind?: string;
  time?: string;
  cube_serial?: string;
}

/** The marks: every problem up to all, and lines one by one after it. */
interface Seen {
  all: number;
  lines: number[];
}

/** logAlerts is the log's unseen problems, each field a signal underneath. */
export const logAlerts = live({
  /** The alerts not marked seen, newest first. */
  list: [] as LogAlert[],
  /** Their ids to their levels. */
  levels: new Map<number, string>(),
  /** The worst: error, warn or "". */
  level: "",
  /** The marks. */
  seen: { all: 0, lines: [] } as Seen,
  /** Whose marks seen holds: the user's name, "" for this browser's, null before they are read. */
  seenOf: null as string | null,
});

/** The level of each line's id as a signal of its own, made the first time a line asks (levelOf). */
let levelSignals = new Map<number, ReadonlySignal<string>>();

/**
 * levelOf is a line's level among the problems not yet seen, by its id,
 * "" for none: a signal of its own, so that a line drawn reads its own
 * and is drawn again when that changes alone, also after its id changed
 * (a live line a load brought).
 */
export function levelOf(id: number): ReadonlySignal<string> {
  let s = levelSignals.get(id);
  if (!s) {
    s = computed(() => (id ? logAlerts.levels.get(id) || "" : ""));
    levelSignals.set(id, s);
  }
  return s;
}

/** keepLevelsOf forgets the level signals of the ids no longer among lines, at a load and as lines are cut: they go with their lines. */
export function keepLevelsOf(lines: LogLine[]): void {
  if (!levelSignals.size) return;
  const ids = new Set<number>();
  for (const l of lines) if (l.id) ids.add(l.id);
  for (const id of levelSignals.keys()) if (!ids.has(id)) levelSignals.delete(id);
}

/** logSeenOwner is whose marks the page keeps: the signed-in user's where they follow the user (canSync, login.js), else this browser's. */
function logSeenOwner(): string {
  return canSync() && state.me?.user ? state.me.user.name : "";
}

/** validSeen is a marks document as the page keeps it, or none. */
function validSeen(v: unknown): Seen {
  if (!v || typeof v !== "object") return { all: 0, lines: [] };
  const doc = v as { all?: unknown; lines?: unknown };
  const all = Number.isSafeInteger(doc.all) && (doc.all as number) > 0 ? (doc.all as number) : 0;
  const lines = Array.isArray(doc.lines) ? doc.lines.filter((id) => Number.isSafeInteger(id) && id > all) : [];
  return { all, lines: lines.slice(-LOG_SEEN_LINES) };
}

/** loadLogSeen reads the marks of whoever the page is for now: the user's preference, else this browser's. */
async function loadLogSeen(): Promise<boolean> {
  const owner = logSeenOwner();
  if (logAlerts.seenOf === owner) return true;
  let v: unknown = null;
  if (owner) {
    const n = session();
    try {
      // every key, {} for none: the key alone would answer 404 before the first mark
      v = ((await api<Record<string, unknown>>("GET", "api/v1/me/preferences")) || {})["log.seen"];
    } catch {
      return true; // asked again at the next poll
    }
    if (!stillSession(n)) return false;
  } else {
    try {
      v = JSON.parse(readStored(LOG_SEEN_KEY) || "null");
    } catch {
      v = null;
    }
  }
  logAlerts.seen = validSeen(v);
  logAlerts.seenOf = owner;
  return true;
}

/** saveLogSeen keeps the marks: as the user's preference, else in this browser. */
async function saveLogSeen(): Promise<void> {
  const doc = { all: logAlerts.seen.all, lines: logAlerts.seen.lines };
  if (!logAlerts.seenOf) {
    store(LOG_SEEN_KEY, JSON.stringify(doc));
    return;
  }
  const n = session();
  try {
    await api("PUT", "api/v1/me/preferences/log.seen", doc);
  } catch (e) {
    if (stillSession(n)) showMessage(t("log.markFailed", { error: (e as Error).message }), false);
  }
}

/**
 * loadLogAlerts asks for the log's unsettled problems after the last
 * Mark all as seen, leaves out the lines marked, and hands ☰ its sign;
 * the lines' highlights follow.
 */
export async function loadLogAlerts(): Promise<void> {
  const n = session();
  if (!(await loadLogSeen()) || !stillSession(n)) return;
  let a: { alerts?: LogAlert[] } | null;
  try {
    a = await api<{ alerts?: LogAlert[] }>("GET", "api/v1/log/alerts" + (logAlerts.seen.all ? "?after=" + logAlerts.seen.all : ""));
  } catch {
    return; // the sign stays as it was
  }
  if (!stillSession(n)) return;
  setLogAlerts(((a && a.alerts) || []).filter((x) => x.id > logAlerts.seen.all && !logAlerts.seen.lines.includes(x.id)));
}

/** sameLevels says whether two maps of ids to levels are the same, in order. */
function sameLevels(a: Map<number, string>, b: Map<number, string>): boolean {
  return [...a].join() === [...b].join();
}

/** setLogAlerts takes the alerts not marked seen; the lines whose level changed are drawn again, alone. */
function setLogAlerts(list: LogAlert[]): void {
  const levels = new Map(list.map((x) => [x.id, x.level]));
  logAlerts.list = list;
  if (!sameLevels(levels, logAlerts.levels)) logAlerts.levels = levels;
  logAlerts.level = list.some((x) => x.level === "error") ? "error" : list.length ? "warn" : "";
  renderLogSign();
}

/** renderLogSign hands ☰ the log's unseen problems: its Log entry's sign and count, and ☰'s mark (ui/src/chrome/, MassimoParts.chrome.log). */
export function renderLogSign(): void {
  const tone = logAlerts.level === "error" ? "bad" : logAlerts.level ? "warn" : "";
  window.MassimoParts.chrome.log(logAlerts.list.length, tone);
}

/** markLogSeen marks one line seen. */
export function markLogSeen(id: number): void {
  const seen = logAlerts.seen;
  if (!seen.lines.includes(id)) logAlerts.seen = { all: seen.all, lines: seen.lines.concat(id).slice(-LOG_SEEN_LINES) };
  setLogAlerts(logAlerts.list.filter((x) => x.id !== id));
  saveLogSeen();
}

/** markAllLogSeen marks every problem there is now seen: all moves up to the newest, and the lines it covers go. */
export function markAllLogSeen(): void {
  const all = Math.max(logAlerts.seen.all, ...logAlerts.list.map((x) => x.id));
  logAlerts.seen = { all, lines: logAlerts.seen.lines.filter((id) => id > all) };
  setLogAlerts([]);
  saveLogSeen();
}

/** forgetLogAlerts forgets the problems and whose marks they were, at a sign-out: ☰ shows none until the next user's are read. */
export function forgetLogAlerts(): void {
  logAlerts.list = [];
  logAlerts.levels = new Map();
  logAlerts.level = "";
  logAlerts.seen = { all: 0, lines: [] };
  logAlerts.seenOf = null;
  levelSignals = new Map();
  renderLogSign();
}
