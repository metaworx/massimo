// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/plain/rooms.d.ts
 *
 * The globals of the plain script ui/rooms.js that the parts call: what a
 * room's tile keeps, a room's facts, the texts of a device's values, and
 * the actions behind a tile, which stay plain and change the tile's live
 * state (roomUI) that the rooms' parts are drawn from.
 */

import type { Device, State } from "../api";
import type { Item } from "../shared/problems";

declare global {
  /**
   * What a room's tile keeps (rooms.js roomUI), a live object whose
   * properties the plain actions set and the tile follows: its device list
   * open, a stepper value waiting to be sent and its timer, the holiday
   * form and the deletion's question shown, the configuration open
   * (rooms/Config.tsx, which keeps the rest itself), the write running,
   * the Hold switch, and the result line.
   */
  interface RoomUI {
    open: boolean;
    pending: number | null;
    timer: number | null;
    vacation: boolean;
    confirm: boolean;
    config: boolean;
    sending: string | null;
    hold: boolean;
    result: { text: string; ok: boolean } | null;
  }
  /** A temperature step of the tile's chevrons and of a temperature's stepper, 0.5 °C. */
  const STEP: number;
  /** The lowest temperature a thermostat takes, 4.5 °C. */
  const MIN_TEMP: number;
  /** The highest temperature a thermostat takes, 30.5 °C. */
  const MAX_TEMP: number;
  /** The setpoint the House panel sends for MANU where no temperature is given. */
  const DEFAULT_TEMP: number;
  /** What the room's tile keeps, made the first time it is asked for. */
  function roomUI(room: PageRoom): RoomUI;
  /** The thermostat whose state stands for the room: its wall thermostat, else its first radiator thermostat. */
  function roomThermostat(room: PageRoom): Device | null;
  /** The state of the room's thermostat, null without one. */
  function roomState(room: PageRoom): State | null;
  /** The modes of the room's thermostats, boost counted as manual, each once. */
  function roomModes(room: PageRoom): string[];
  /** The device whose measured temperature stands for the room, its virtual wall thermostat; null while it has none. */
  function roomActualDevice(room: PageRoom): Device | null;
  /** The room's measured temperature. */
  function roomActual(room: PageRoom): number | undefined;
  /** How long a stale device has not been seen, "Not seen for 30 hours"; "" for one that is not stale. */
  function staleText(d: Device): string;
  /** When the server received the current value of a field of the device's state. */
  function received(d: Device, field: string): string | undefined;
  /** A temperature within the thermostats' range, on a half degree. */
  function clampTemp(v: number): number;
  /** A datetime-local value as the API's "YYYY-MM-DD HH:MM". */
  function apiUntil(value: string): string;
  /** A holiday's end, "YYYY-MM-DD HH:MM", as day, month and time in the page's language. */
  function fmtUntil(text?: string): string;
  /** A mode in a word. */
  function modeName(mode: string): string;
  /** A device's type in words. */
  function typeName(type: string): string;
  /** The mark of a write a device has not shown by its deadline, as an item; null for none. */
  function confirmItem(d: Device): Item | null;
  /** The values of a write a device has not shown by its deadline, in words, "eco, Monday's program". */
  function confirmFieldsText(d: Device): string;
  /** The result line of a write that went through: ok, discarded, the radio's budget. */
  function resultText(label: string, res: unknown): string;
  /** The result line of a write that failed, with the devices without a state where the server names them. */
  function errorText(label: string, e: Error): string;
  /** Says how an action went on the room's tile, or on the page's message line where the tile is gone. */
  function showTileResult(roomId: number, text: string, ok?: boolean): void;
  /** Steps the stepper's value waiting to be sent by delta, and sends it two seconds after the last step. */
  function stepSetpoint(room: PageRoom, delta: number): void;
  /** Drops the stepper's value waiting to be sent. */
  function cancelSetpoint(room: PageRoom): void;
  /** Sends the stepper's value waiting to be sent, now. */
  function flushSetpoint(room: PageRoom): Promise<void>;
  /** Sends a mode or a setpoint of the tile as the write key. */
  function sendRoomSetpoint(room: PageRoom, body: Record<string, unknown>, label: string, key: string): Promise<unknown>;
  /** Locks or unlocks the room's thermostats; el is the tile, whose menus it closes. */
  function lockRoom(room: PageRoom, locked: boolean, el: HTMLElement | null): Promise<void>;
  /** Locks or unlocks one device from its row, as a write of its room's tile, else with btn turning. */
  function lockDevice(d: Device, locked: boolean, btn: HTMLElement | null): Promise<unknown>;
  /** Deletes a room that has no devices, and loads the rooms again. */
  function deleteRoom(room: PageRoom): Promise<void>;
}

export {};
