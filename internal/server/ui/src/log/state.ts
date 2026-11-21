// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/log/state.ts
 *
 * What the log keeps (AP UiParts, Block 11; log.js logState), as live
 * state: the plain code and the UI tests read and write it as the page
 * global logState (log.js), the parts follow it. The lines are the API's
 * events as plain objects, oldest first; which of them are drawn, newest
 * first, is view (store.ts).
 */

import type { Device } from "../api";
import { live } from "../shared/live";
import { LOG_COUNT_RANGE } from "./kinds";
import type { Loose, LogLine } from "./lines";

/** A Cube's rooms and its devices without a room, as fetchRooms (app.js) loads them. */
export interface CubeRooms {
  rooms: PageRoom[];
  unlisted: Device[];
}

/** A user of the who choice, as GET /api/v1/users lists it. */
export interface LogUser {
  name: string;
  display?: string;
  display_name?: string;
}

/** logState is what the log keeps, each field a signal underneath. */
export const logState = live({
  /** The log's page is shown. */
  open: false,
  /** One of LOG_RANGES. */
  range: LOG_COUNT_RANGE,
  /** The lines, oldest first; drawn newest first. */
  lines: [] as LogLine[],
  /** The text filter typed. */
  filter: "",
  /** The kind chosen, "" for all. */
  kind: "",
  /** The device chosen by its S/N (its radio address for one without), "" for all. */
  device: "",
  /** The oldest line's id, else its time as received: where Older pages from. */
  cursor: null as string | null,
  /** The server may have older events. */
  more: false,
  /** A page is on its way. */
  loading: false,
  /** Raised by a new log, Clear and a Cube switch: an answer of an older one is dropped. */
  gen: 0,
  /** The lines live events may grow the log to before the oldest go (LOG_MAX_LINES at least). */
  room: 0,
  /** Keep the newest line in view at the top unless the user scrolled down. */
  stick: true,
  /** Counts the live lines, so that loadLog finds the ones that came while it waited. */
  liveSeq: 0,
  /** The lowest level shown, one of LOG_LEVELS. */
  level: "info",
  /** The source chosen, one of LOG_SOURCES, "" for all. */
  source: "",
  /** The user chosen by login name, "" for everyone. */
  user: "",
  /** The users of the who choice, loaded once the log opens (logLoadUsers). */
  users: null as LogUser[] | null,
  /** The commands whose detail is open, by number (a new set at each change). */
  openCommands: new Set<number>(),
  /** A command's detail as GET /api/v1/commands/{id} answered it, or {error}, by number (a new map at each change). */
  commands: new Map<number, Loose>(),
  /** "" for the header's Cube, "all" for every Cube (GET /api/v1/log). */
  cubes: "",
  /** Under All Cubes, each Cube's rooms by S/N, null while asked or not answered. */
  cubeRooms: new Map<string, CubeRooms | null>(),
  /** The Cubes the choices were last filled for (logCubesPolled). */
  cubesSeen: "",
  /** The poll of new lines under All Cubes (logPollSet). */
  pollTimer: null as ReturnType<typeof setInterval> | null,
  /** A poll is on its way. */
  polling: false,
});
