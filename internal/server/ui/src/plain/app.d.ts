// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/plain/app.d.ts
 *
 * The globals of the plain script ui/app.js that the parts call: the
 * page's state, the API, the reload, the loads and the poll, the Cube
 * chosen, the ages of values and the times, a Cube's link and clock, a
 * device's path, detail and name, a temperature, the app's bridge, the
 * explanations shown.
 */

import type { Cube, Device, DeviceDetail, FoundCube, Me, Room } from "../api";

declare global {
  /** A room as the page keeps it: its virtual wall thermostat taken out of its devices (app.js splitVirtual). */
  type PageRoom = Room & { vwt: Device | null };
  /** What the page loaded, as far as a part reads it (app.js state); the fields shared/data.ts adopts are signals underneath, written whole. */
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
    /** The event stream is open: the server reports the chosen Cube's changes as they come. */
    eventsOpen: boolean;
    /** Who the page is signed in as (GET /api/v1/me), null before it is read and after a sign-out. */
    me: Me | null;
    /** The person's settings in effect, as far as a part reads them: this device's where it keeps one, else the user's. */
    settings: { poll: number; expert: boolean; clock: string; lang: string; theme: string; scheme: string; blink: string; iconWindow: string; iconRadio: string };
    /** Each Cube's daylight-saving rules by its serial, false where it has none, from the Cube's detail (app.js loadTimeZone). */
    tz: Record<string, unknown>;
    /** The settings kept for this device alone, by key, as last stored (app.js storeDeviceOverrides). */
    device: Record<string, unknown>;
  };
  /** The text key of each setting's name, for its "only on this device" switch. */
  const SETTING_LABELS: Record<string, string>;
  /** A setting's value where v is one, else undefined (a poll's text taken as its number). */
  function validSetting(key: string, v: unknown): any;
  /** Changes a setting: on this device alone where it is kept so, else the user's, which goes to the server too. */
  function setSetting(key: string, v: unknown): void;
  /** Whether a setting is kept for this device alone. */
  function deviceOnly(key: string): boolean;
  /** Keeps a setting for this device alone with the value in effect, or stops: the user's value applies again. */
  function setDeviceOnly(key: string, on: boolean): void;
  /** Applies the settings in effect to the page: expert mode, the appearance, the header, the poll's pace where it runs. */
  function applySettings(): void;
  /** Starts the page's poll again at the poll setting's pace. */
  function startPolling(): void;
  /** Starts the page's poll again at the poll setting's pace where it runs; a poll that does not run stays stopped. */
  function restartPolling(): void;
  /** Loads the language of a setting (auto: the browser's) and makes it the page's. */
  function changeLanguage(setting: string): Promise<void>;
  /** Draws every text of the page again after a change of language. */
  function applyLanguage(): void;
  /** An instant as the browser's time of day, "20:51:03", its date before it when it is not today's. */
  function fmtTimeOfDay(d: Date): string;
  /** An API time as a Date, null for none (a zero time). */
  function timeOf(iso: string | undefined): Date | null;
  /** The element of the page with the id. */
  function $(id: string): HTMLElement | null;
  /** A value kept in this browser (localStorage), null where there is none or the storage is blocked. */
  function readStored(key: string): string | null;
  /** Keeps a value in this browser, null to forget it; where the storage is blocked it lives for this page only. */
  function store(key: string, value: string | null): void;
  /** The Massimo app's bridge on the page it opened (docs/app.md), null in a browser; connect, from version 2, shows the app's servers. */
  function appBridge(): { connect?: () => Promise<null> } | null;
  /** A request to the API, without the page's token where auth is false (/healthz, always open); the answer's JSON, or an error with the server's message. */
  function apiRequest<T = unknown>(method: string, path: string, body?: unknown, opts?: { auth?: boolean; token?: string }): Promise<T>;
  /** Opens the chosen Cube's event stream again. */
  function connectEvents(): Promise<void>;
  /** A request to the API with the page's token, headers added to its own; the answer's JSON, or an error with the server's message (and its status). */
  function api<T = unknown>(method: string, path: string, body?: unknown, headers?: Record<string, string>): Promise<T>;
  /** Loads the Cubes, the rooms and the log's alerts again. */
  function refresh(): Promise<boolean>;
  /** Loads the Cubes' list into state.cubes and state.found and draws what shows them. */
  function loadCubes(): Promise<boolean>;
  /** Loads the chosen Cube's rooms and its devices in no room, and draws them; one load at a time, one more waiting behind it for every call meanwhile. */
  function loadRooms(): Promise<boolean>;
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
  /** How old a value received at ms is, in the page's language ("3 min"); since marks one already there when the server first heard from the Cube. */
  function fmtAge(ms: number, since: boolean): string;
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
  /** A Date as a datetime-local input's value, the browser's time. */
  function localDateTimeValue(d: Date): string;
  /** A request to the API as the answer itself, for a file or a body of another type; an error with the server's message where it fails. */
  function apiRaw(method: string, path: string, body?: unknown, contentType?: string): Promise<Response>;
  /** The default end of a temporary setting, tomorrow at six, as a datetime-local value. */
  function tomorrowMorning(): string;
  /** The page's message line: a text, ok or a failure. */
  function showMessage(text: string, ok: boolean): void;
  /** A Cube's rooms, their virtual wall thermostats apart, and its devices without a room or unlisted. */
  function fetchRooms(serial: string): Promise<{ rooms: PageRoom[]; unlisted: Device[] }>;
}

export {};
