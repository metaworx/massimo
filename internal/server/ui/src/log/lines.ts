// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/log/lines.ts
 *
 * A line of the log (AP UiParts, Block 11, from log.js logLine): the
 * API's event as the log reads it, and the line the view keeps of it.
 * The API file has no schema of a log line (docs/api-v1.md, "GET
 * /api/v1/cubes/{sn}/log"; its components hold LogCursorBeyond alone),
 * so the event's type is this file's, LogEvent, not api.d.ts'. Its
 * summary, state and changes differ by kind: they are read as the texts
 * name their fields (Loose).
 */

/** Loose is a part of an event whose fields depend on its kind: read by the texts by name, as the plain code read them. */
export type Loose = Record<string, any>;

/** LogEvent is an event as GET …/log, GET /api/v1/log and the stream send it, as far as the log reads it. */
export interface LogEvent {
  id?: number;
  time?: string;
  kind?: string;
  device?: string;
  serial?: string;
  source?: string;
  level?: string;
  state?: Loose;
  summary?: Loose;
  changes?: Loose;
  list_changes?: Loose[];
  while_disconnected?: boolean;
  cube?: { serial: string; address?: string };
  by?: { name: string; display_name?: string };
  real_user?: { display?: string; name?: string };
  cube_serial?: string;
  command?: number;
  error?: string;
}

/** LogLine is a line of the view; the text of a state event is made when the line is drawn, in the language shown. */
export interface LogLine {
  /** The event's id in the server's log, 0 for a live line of the stream without one and the page's own error line. */
  id: number;
  /** The time as the server sent it, "" for none. */
  at: string;
  time: Date;
  kind: string;
  device: string;
  serial: string;
  source: string;
  state: Loose | null;
  gone: Loose | null;
  level: string;
  listChanges: Loose[] | null;
  configChanges: Loose | null;
  summary: Loose | null;
  missed: boolean;
  changes: Loose | null;
  cube: { serial: string; address?: string } | null;
  by: { name: string; display_name?: string } | null;
  /** Who started it as the page shows a user: an integration's user by its chain. */
  who: string;
  realName: string;
  /** The line's Cube under All Cubes (GET /api/v1/log). */
  cubeSerial: string;
  command: number;
  error: string;
  text: string;
  /** The live lines' count, for a live line of the stream (loadLog finds those that came while it waited). */
  seq?: number;
  /** The line's key in the view (store.ts keyLines): the same while it is the same event. */
  key?: string;
}

/**
 * logLine turns an event into a line of the view: what changed when the
 * event says so, else the whole state (a device's first event since the
 * server started), worded when the line is drawn.
 */
export function logLine(ev: LogEvent): LogLine {
  const time = ev.time ? new Date(ev.time) : new Date();
  return {
    id: ev.id || 0,
    at: ev.time || "",
    time,
    kind: ev.kind || "",
    device: ev.device || "",
    serial: ev.serial || (ev.state && ev.state.serial) || "",
    source: ev.source || "",
    state: ev.kind === "state" ? ev.state || null : null,
    gone: ev.kind === "device_forgotten" && ev.state ? ev.state : null,
    level: ev.level || "info",
    listChanges: ev.kind === "metadata" && ev.list_changes && ev.list_changes.length ? ev.list_changes : null,
    configChanges: ev.kind === "config" && ev.changes && Object.keys(ev.changes).length ? ev.changes : null,
    summary: ev.summary || null,
    missed: !!ev.while_disconnected,
    changes: ev.kind === "state" && ev.changes && Object.keys(ev.changes).length ? ev.changes : null,
    cube: ev.kind === "cube_found" || ev.kind === "cube_gone" ? ev.cube || null : null,
    by: ev.by || null,
    // who started it as the page shows a user: an integration's user by
    // its chain, ", as <login name>" where it acted as a Massimo user
    who: (ev.real_user && ev.real_user.display) || "",
    realName: (ev.real_user && ev.real_user.name) || "",
    cubeSerial: ev.cube_serial || "", // the line's Cube under All Cubes (GET /api/v1/log)
    command: ev.command || 0,
    error: ev.error || "",
    text: ev.error || "",
  };
}

/** errorLine is the page's own line where the log cannot load. */
export function errorLine(text: string): LogLine {
  return { ...logLine({ kind: "error" }), time: new Date(), text };
}
