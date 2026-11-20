// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/plain/app.d.ts
 *
 * The globals of the plain script ui/app.js that the parts call: the
 * page's state, the API, the reload, the loads and the poll, the Cube
 * chosen, the ages of values and the times, a Cube's link and clock, a
 * device's path, detail and name, a temperature, the explanations shown.
 */

import type { Cube, Device, DeviceDetail, FoundCube, Room } from "../api";

declare global {
  /** A room as the page keeps it: its virtual wall thermostat taken out of its devices (app.js splitVirtual). */
  type PageRoom = Room & { vwt: Device | null };
  /** What the page loaded, as far as a part reads it (app.js state). */
  const state: {
    /** The Cubes the server serves, as the last load of GET /api/v1/cubes gave them. */
    cubes: Cube[];
    /** The Cubes and proxies found on the network and not served. */
    found: FoundCube[];
    /** The Cube chosen in the header. */
    serial: string | null;
    /** The chosen Cube's rooms, as the last load gave them and the state events since changed them. */
    rooms: PageRoom[];
    /** The chosen Cube's devices in no room. */
    unlisted: Device[];
    /** The Cube whose rooms and devices rooms and unlisted hold, once loaded. */
    roomsOf: string | null;
    /** A device's detail by its radio address, fetched on demand (deviceDetail); emptied after a write that changes one. */
    details: Record<string, DeviceDetail | undefined>;
    /** What each room's tile keeps, by the room's id (rooms.js roomUI). */
    ui: Record<number, RoomUI>;
    /** The last poll got no answer from the server. */
    serverDown: boolean;
    /** The server runs another version than the page was loaded with. */
    newVersion: boolean;
    /** When the server last answered a request of the page (Date.now()), 0 before. */
    repliedAt: number;
    /** The person's settings, as far as a part reads them. */
    settings: { poll: number; expert: boolean };
  };
  /** An instant as the browser's time of day, "20:51:03", its date before it when it is not today's. */
  function fmtTimeOfDay(d: Date): string;
  /** An API time as a Date, null for none (a zero time). */
  function timeOf(iso: string | undefined): Date | null;
  /** The element of the page with the id. */
  function $(id: string): HTMLElement | null;
  /** A request to the API with the page's token, headers added to its own; the answer's JSON, or an error with the server's message (and its status). */
  function api<T = unknown>(method: string, path: string, body?: unknown, headers?: Record<string, string>): Promise<T>;
  /** Loads the Cubes, the rooms and the log's alerts again. */
  function refresh(): Promise<void>;
  /** Loads the Cubes' list into state.cubes and state.found and draws what shows them. */
  function loadCubes(): Promise<void>;
  /** Loads the chosen Cube's rooms and its devices in no room, and draws them. */
  function loadRooms(): Promise<void>;
  /** Asks the server for the current state now, the rooms too with rooms. */
  function poll(rooms?: boolean): Promise<void>;
  /** Chooses the Cube the page shows, by its serial, and loads it. */
  function selectCube(serial: string): void;
  /** Whether an entry of GET /api/v1/cubes is a Cube the server serves. */
  function isConfigured(c: Cube | FoundCube): boolean;
  /** The server's clock less the browser's, in ms (header.js noteServerClock). */
  let serverSkew: number;
  /** A number with its unit, short, in the page's language ("8 sec"). */
  function fmtUnit(n: number, unit: string): string;
  /** An instant as date and time. */
  function fmtInstant(date: Date, timeZone?: string): string;
  /** An API time as a Date, null for none (a zero time). */
  function timeOf(iso?: string): Date | null;
  /** A time of day, short, from milliseconds. */
  function fmtShortTime(ms: number): string;
  /** Fills an age (.age[data-at]) with how old it is; refreshAges does so once a minute. */
  function fillAge(el: HTMLElement): void;
  /** When the server first heard from the chosen Cube, in milliseconds; NaN before. */
  function observedSince(): number;
  /** How long after observedSince a value counts as already there. */
  const SINCE_MS: number;
  /** How the server stands with a Cube: connected, held, stale, busy, rebooting or offline. */
  function linkState(c: Cube): string;
  /** Why a Cube is not connected, in a sentence, where the server knows more; "" otherwise. */
  function linkText(c: Cube): string;
  /** A Cube's clock as the page shows it, and its tooltip. */
  function clockText(c: Cube): { text: string; title: string };
  /** A device's name, else its serial, else its radio address. */
  function deviceName(d: Device): string;
  /** Whether a device is a radiator or a wall thermostat. */
  function isThermostat(d: Device): boolean;
  /** The API path of the chosen Cube. */
  function base(): string;
  /** The API path of a device of the chosen Cube. */
  function devicePath(d: Device): string;
  /** A device's detail (GET …/devices/{rf}), from state.details where it is there. */
  function deviceDetail(d: Device): Promise<DeviceDetail>;
  /** A temperature with one decimal and its unit, " °C" unless given; "–" when unknown. */
  function fmtTemp(v: number | undefined | null, unit?: string): string;
  /** The keys of the settings' explanations shown (helpParts), the plain help buttons' and the parts'. */
  const helpOpen: Set<string>;
  /** The default end of a temporary setting, tomorrow at six, as a datetime-local value. */
  function tomorrowMorning(): string;
}

export {};
