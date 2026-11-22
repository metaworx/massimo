// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/model/device.ts
 *
 * A device as the parts read it beside its fields, for every region that
 * shows devices (the rooms with their tiles and device rows, and through a
 * Cube's problems the Cubes and the header, and the log): its name by its
 * radio address, its type's icon, an open
 * window's state by the page's clock, what is wrong with it as items of
 * the shared problem list, and the device as an entry of that list. The
 * age of a value is the plain received of rooms.js. Like all of ui/src/model/ (docs/ui.md, "How the
 * page is built"), it knows the API's data and no region, and holds no
 * state.
 */

import type { Device, State } from "../api";
import { untracked } from "@preact/signals";
import { windowNow } from "../shared/data";
import type { Entry, Item } from "../shared/problems";

/**
 * deviceIcon is the symbol of a device's type: a radiator, a wall
 * thermostat, a window as the person chooses it, closed, or a leaf for the
 * eco button.
 */
export function deviceIcon(d: Device): string {
  if (d.type === "wall_thermostat") return "icon-wall";
  if (d.type === "shutter_contact") return iconChoice("window").type || "icon-window-sash";
  if (d.type === "eco_switch") return "icon-leaf";
  return "icon-radiator";
}

/** An open window's state: open briefly, open long (a warning), open too long (a failure); "" for one not open. */
export type WindowState = "" | "open" | "warn" | "bad";

/**
 * windowState is an open window's state by the page's clock and the
 * window's times (the server's limits): "open" until its warning time,
 * "warn" until its alert time, "bad" after; "" for a window not open.
 */
export function windowState(d: Device): WindowState {
  if (d.state?.window_open !== true) return "";
  const w = d.window;
  if (!w) return "open";
  void windowNow.value; // an open window's times: drawn again when the clock alone changes its state
  const now = Date.now();
  if (now >= Date.parse(w.alert_at)) return "bad";
  if (now >= Date.parse(w.warn_at)) return "warn";
  return "open";
}

/**
 * WINDOW_TONE is the badge of an open window's state: filled blue while it
 * is open briefly, an amber outline (a warning) once it is open long,
 * filled red (a failure) once it is open too long.
 */
export const WINDOW_TONE: Record<string, string> = { open: "fill-open", warn: "warn", bad: "fill-bad" };

/**
 * communicationErrorTitle explains a device's communication error, and
 * names the radiogram that failed where the state gives it (radiogramName
 * of tiles.js, which the log names it by too).
 */
function communicationErrorTitle(s: State): string {
  const lines = [t("badge.communicationErrorTitle")];
  if (s.failed_radiogram) lines.push(t("badge.failedRadiogram", { radiogram: radiogramName(s.failed_radiogram) }));
  return lines.join("\n");
}

/**
 * deviceProblems are what is wrong with a device, as items of the problem
 * list: a radio fault (the Cube could not reach it, its communication
 * error, or its link to another device of the room is disturbed, its link
 * error), each explained in its title, and a weak or low battery; with
 * withWindow, also an open window, with its age. A device that has not
 * reported has none.
 */
export function deviceProblems(d: Device, withWindow?: boolean): Item[] {
  const s = d.state || {};
  const out: Item[] = [];
  if (!d.reported) return out;
  const radio = signOf("radio").icon;
  if (s.error) out.push({ kind: "radio", icon: radio, sign: "radio", tone: "bad", text: t("badge.communicationError"), title: communicationErrorTitle(s) });
  if (s.link_error) out.push({ kind: "radio", icon: radio, sign: "radio", tone: "warn", text: t("badge.linkError"), title: t("badge.linkErrorTitle") });
  // The battery's level, the server's (internal/battery): weak amber, low
  // red; a device of a server without levels by its flag, amber.
  const battery = (tone: string, key: string): Item => ({ kind: "battery", icon: "icon-battery", tone, text: t(key), at: received(d, "battery_low"), stale: d.stale });
  if (d.battery === "low") out.push(battery("bad", "badge.batteryLow"));
  else if (d.battery === "weak") out.push(battery("warn", "badge.batteryWeak"));
  else if (d.battery === undefined && s.battery_low) out.push(battery("warn", "badge.batteryLow"));
  if (withWindow && s.window_open === true) {
    out.push({ kind: "window", icon: signOf("window").icon, sign: "window", tone: WINDOW_TONE[windowState(d)], text: t("badge.open"),
      at: received(d, "window_open"), stale: d.stale });
  }
  return out;
}

/**
 * rfLabel names a device by its radio address as the page names it
 * (deviceName of app.js), among the chosen Cube's rooms, the address
 * where the page does not know it: a structure event's devices in the
 * log, a finding's device (tiles.js mismatchArgs, through the plain name
 * of log.js).
 */
export function rfLabel(rf: string): string {
  // the rooms by the list; a device's state event draws no name again
  const rooms = state.rooms || [];
  return untracked(() => {
    for (const r of rooms) {
      for (const d of r.devices || []) {
        if (d.rf_address === rf) return deviceName(d);
      }
    }
    return rf;
  });
}

/**
 * deviceEntry is a device as an entry of the problem list: its type's
 * icon, its name (title, where the place names it otherwise, "Room ›
 * Device"), its serial or else its radio address in grey, and items.
 */
export function deviceEntry(d: Device, items: Item[], title?: string): Entry {
  return { icon: deviceIcon(d), title: title || deviceName(d), sub: d.serial || d.rf_address, items };
}
