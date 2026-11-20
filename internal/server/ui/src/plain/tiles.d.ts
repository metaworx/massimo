// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/plain/tiles.d.ts
 *
 * The globals of the plain script ui/tiles.js that the parts call: the
 * tile's actions that stay plain (the More menu's presets, the holiday
 * form, the device list, the menus closed), a finding of
 * the Cube's lines in words, and the name of a radiogram that failed,
 * which the log names it by too.
 */

import type { Device, FailedRadiogram, Mismatch } from "../api";

declare global {
  /** A failed radiogram's type by the original software's name and its code, "TimeInformation (0x03)". */
  function radiogramName(r: FailedRadiogram): string;
  /** A finding of the Cube's lines against its room list in a sentence that names the device and both rooms. */
  function mismatchText(m: Mismatch): string;
  /** A finding's words: its device, its rooms and its types, by the names the page knows. */
  function mismatchArgs(m: Mismatch): Record<string, string>;
  /** A thermostat's configured eco and comfort temperatures. */
  function presetTemps(th: Device): Promise<{ eco?: number; comfort?: number }>;
  /** Sets the room to its thermostat's eco or comfort temperature in the room's mode, from the More menu. */
  function sendPreset(room: PageRoom, which: "eco" | "comfort"): Promise<unknown>;
  /** Closes the holiday form and drops a stepper value waiting to be sent, for a mode that takes over. */
  function leaveEditing(room: PageRoom): void;
  /** Opens the tile's holiday form; el is the tile, whose menus it closes. */
  function openVacation(room: PageRoom, el: HTMLElement | null): void;
  /** Closes the tile's holiday form. */
  function closeVacation(room: PageRoom): void;
  /** Opens or closes the tile's device list, the configuration closed; el is the tile, whose menus it closes. */
  function toggleDevices(room: PageRoom, el: HTMLElement | null): void;
  /** Closes the open menus of a tile. */
  function closeMenu(el: HTMLElement | null): void;
}

export {};
