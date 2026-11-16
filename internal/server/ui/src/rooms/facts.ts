// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/rooms/facts.ts
 *
 * What the rooms' parts show of a room and its devices beside their
 * fields, which no other region reads: a mode's badge, the problems of a
 * tile's head and of a device's row with the label that names them, a
 * device's badges in words, and the tile's note of the devices that have
 * not been seen for long. A room's facts (roomThermostat, roomModes,
 * staleText, …) and the texts of a write not confirmed (confirmItem and
 * its helpers) stay in rooms.js, the parts calling them there: other
 * sessions change them on main, and a move would set their changes
 * against this one.
 */

import type { Device } from "../api";
import { deviceEntry, deviceProblems, WINDOW_TONE, windowState } from "../model/device";
import { kindsLabel, type Entry, type Item } from "../shared/problems";

/** The kinds of problems a tile's head and a device's row name, in their order there; a holiday is listed with its end. */
const KINDS = ["radio", "battery", "window", "unconfirmed", "mixed"];

/** MODE_TONE colours a mode's badge: auto green, manual (and boost) amber, holiday violet. */
const MODE_TONE: Record<string, string> = { auto: "fill-ok", manual: "fill-warn", holiday: "fill-info" };

/** modeTone is the badge kind of a thermostat's mode. */
export function modeTone(mode: string): string {
  const m = mode === "vacation" ? "holiday" : mode === "boost" ? "manual" : mode;
  return MODE_TONE[m] || "mode";
}

/**
 * problemsLabel names the kinds of problems among items, and a holiday
 * with its end, for a screen reader and the tooltip.
 */
export function problemsLabel(items: Item[]): string {
  const parts = [kindsLabel(items, KINDS)].filter(Boolean);
  const holiday = items.find((it) => it.kind === "holiday");
  if (holiday) parts.push(holiday.text);
  return parts.join("; ");
}

/**
 * deviceItems are what is wrong with a device: its problems
 * (deviceProblems; with withWindow an open window too, which a device's
 * row shows as its badge) and a write it has not shown by its deadline
 * (confirmItem).
 */
export function deviceItems(d: Device, withWindow?: boolean): Item[] {
  const items = deviceProblems(d, withWindow);
  const unconfirmed = confirmItem(d);
  if (unconfirmed) items.push(unconfirmed);
  return items;
}

/**
 * roomEntries are what a tile's head lists of the room: per device its
 * problems with an open window, a write it has not shown by its deadline,
 * and the thermostats' modes where they differ; a thermostat on holiday
 * with the holiday's end, in its mode's colour, in place of its mode. The
 * room's virtual wall thermostat is not listed: its flags are the wall
 * thermostat's.
 */
export function roomEntries(room: PageRoom): Entry[] {
  const mixed = roomModes(room).length > 1;
  return room.devices.map((d) => {
    const items = deviceItems(d, true);
    const s = d.state || {};
    if (isThermostat(d) && s.mode === "vacation") {
      items.push({ kind: "holiday", icon: "icon-suitcase", tone: modeTone(s.mode), text: t("room.holidayUntil", { until: fmtUntil(s.until) }) });
    } else if (mixed && isThermostat(d) && s.mode) {
      items.push({ kind: "mixed", icon: "icon-shuffle", tone: modeTone(s.mode), mark: "warn", text: modeName(s.mode) });
    }
    return deviceEntry(d, items);
  });
}

/** A badge of a device's row in words: its text, its kind, and the time of the value for its age. */
export interface Words {
  text: string;
  kind: string;
  at?: string;
}

/**
 * deviceBadges are the badges in words of a device: not reported, stale
 * and locked, outlined, then its mode or window state as a filled badge,
 * always the last one; its problems are icons (deviceItems).
 */
export function deviceBadges(d: Device): Words[] {
  const s = d.state || {};
  if (!d.reported) {
    // A device the Cube has not heard for some minutes keeps its last
    // values, out of date; one it never heard has none.
    return [{ text: t(d.out_of_date ? "badge.outOfDate" : "badge.notReported"), kind: "warn", at: d.out_of_date ? d.last_seen : undefined }];
  }
  const out: Words[] = [];
  if (d.stale) out.push({ text: staleText(d), kind: "warn stale-badge" });
  if (s.panel_locked) out.push({ text: t("badge.locked"), kind: "warn" });
  if (s.window_open !== undefined) {
    const at = received(d, "window_open");
    out.push(s.window_open ? { text: t("badge.open"), kind: WINDOW_TONE[windowState(d)] || "fill-bad", at } : { text: t("badge.closed"), kind: "fill-ok", at });
  } else if (s.mode && isThermostat(d)) {
    out.push({ text: modeName(s.mode), kind: modeTone(s.mode) }); // a boost says so, coloured as manual
  }
  return out;
}

/**
 * staleNote is the tile's note that its devices have not been seen for
 * long, by how long each, "Not seen for 30 hours: Radiator."; "" where
 * none is stale.
 */
export function staleNote(room: PageRoom): string {
  const stale = room.devices.filter((d) => d.stale);
  if (!room.stale && !stale.length) return "";
  const groups = new Map<string, string[]>();
  for (const d of stale) {
    const k = staleText(d);
    groups.set(k, [...(groups.get(k) || []), deviceName(d)]);
  }
  return [t("room.stale"), ...[...groups].map(([text, names]) => " " + text + ": " + names.join(", ") + ".")].join("");
}
