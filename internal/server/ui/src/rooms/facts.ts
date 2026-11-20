// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/rooms/facts.ts
 *
 * What the rooms' parts show of a room and its devices beside their
 * fields, which no other region reads: a mode's badge, the problems of a
 * tile's head and of a device's row with the label that names them, the
 * unsaved settings of a room's virtual wall thermostat and of its
 * configuration's forms among them, the
 * room's notices among them and their count for the tile's line, and a
 * device's badges in words. A room's facts (roomThermostat, roomModes,
 * staleText, …) and the texts of a write not confirmed (confirmItem and
 * its helpers) stay in rooms.js, the parts calling them there: other
 * sessions change them on main, and a move would set their changes
 * against this one.
 */

import type { Device, Mismatch } from "../api";
import { deviceEntry, deviceProblems, WINDOW_TONE, windowState } from "../model/device";
import { unsavedItems } from "../shared/forms";
import { entryItems, kindsLabel, type Entry, type Item } from "../shared/problems";
import { cfgFormKey, vwtFormKey } from "./store";

/** The kinds of problems a tile's head and a device's row name, in their order there; a holiday is listed with its end. */
const KINDS = ["radio", "battery", "window", "unconfirmed", "mixed", "unsaved", "notready", "stale", "mismatch"];

/** The kinds that were the tile's notices, which its one line counts (roomNotices). */
const NOTICE_KINDS = ["notready", "stale", "mismatch"];

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

/** mismatchWords are a finding's words on its device: those of the device row's badge, "in Office for the Cube". */
export function mismatchWords(m: Mismatch): string {
  return t("mismatchBadge." + m.finding, mismatchArgs(m));
}

/**
 * mismatchItem is a finding of the room's mismatches (docs/api-v1.md) as
 * an item, since when it holds: under its device, the words of the
 * device row's badge, the whole sentence its tooltip; under the room
 * (whole), the whole sentence, which names the device.
 */
function mismatchItem(m: Mismatch, whole?: boolean): Item {
  const text = whole ? mismatchText(m) : mismatchWords(m);
  return { kind: "mismatch", icon: "icon-sliders", tone: "warn", text, title: whole ? undefined : mismatchText(m), since: m.since };
}

/** The item of a device whose values are old: not seen for how long. */
function staleItem(text: string): Item {
  return { kind: "stale", icon: "icon-set-dial", tone: "warn", text };
}

/** The item of a room or a device that holds the room's commands back until every device has reported; at, the time of a device's last values. */
function notReadyItem(text: string, at?: string): Item {
  return { kind: "notready", icon: "icon-paused", tone: "warn", text, at };
}

/**
 * roomOwnItems are the room's own notices as items: its commands
 * discarded until every device has reported, its values old where no
 * device names which, and the findings that name no device of the room;
 * then the forms of its configuration that are unsaved or whose save
 * failed (unsavedItems), each the disk that opens its tab.
 */
function roomOwnItems(room: PageRoom): Item[] {
  const items: Item[] = [];
  if (!room.ready) items.push(notReadyItem(t("room.notReady")));
  if (room.stale && !room.devices.some((d) => d.stale)) items.push(staleItem(t("room.stale")));
  const own = new Set(room.devices.map((d) => d.rf_address));
  for (const m of room.mismatches || []) if (!m.device || !own.has(m.device)) items.push(mismatchItem(m, true));
  items.push(...unsavedItems(cfgFormKey(state.roomsOf || "", room.id)));
  return items;
}

/**
 * roomNotices counts the notices among a tile's entries (roomEntries), as
 * its line says them: its commands held back, each device not reported or
 * not seen for long, each finding of its mismatches; as its list shows them.
 */
export function roomNotices(entries: Entry[]): number {
  return entries.flatMap(entryItems).filter((it) => NOTICE_KINDS.includes(it.kind || "")).length;
}

/**
 * roomEntries are what a tile's head lists of the room: first the room
 * itself with its own notices and its configuration's unsaved forms
 * (roomOwnItems); then per device its
 * problems with an open window, a write it has not shown by its deadline,
 * its values not seen for long, not reported while the room waits for
 * it, the room's findings that name it, and the thermostats' modes where
 * they differ; a thermostat on holiday with the holiday's end, in its
 * mode's colour, in place of its mode. The room's virtual wall thermostat
 * is listed for its settings alone, while they are unsaved or their save
 * failed (vwtItems), in every mode: its flags are the wall thermostat's.
 */
export function roomEntries(room: PageRoom): Entry[] {
  const mixed = roomModes(room).length > 1;
  const vwt = room.vwt ? vwtItems(room) : [];
  const own = vwt.length && room.vwt ? [deviceEntry(room.vwt, vwt)] : [];
  const notices = roomOwnItems(room);
  const head: Entry[] = notices.length ? [{ icon: "icon-room", title: room.name, items: notices }] : [];
  return head.concat(room.devices.map((d) => {
    const items = deviceItems(d, true);
    if (d.stale) items.push(staleItem(staleText(d)));
    // As the device's row says it (deviceBadges): out of date with its age, or not reported.
    if (!room.ready && !d.reported) items.push(notReadyItem(t(d.out_of_date ? "badge.outOfDate" : "badge.notReported"), d.out_of_date ? d.last_seen : undefined));
    for (const m of room.mismatches || []) if (m.device === d.rf_address) items.push(mismatchItem(m));
    const s = d.state || {};
    if (isThermostat(d) && s.mode === "vacation") {
      items.push({ kind: "holiday", icon: "icon-suitcase", tone: modeTone(s.mode), text: t("room.holidayUntil", { until: fmtUntil(s.until) }) });
    } else if (mixed && isThermostat(d) && s.mode) {
      items.push({ kind: "mixed", icon: "icon-shuffle", tone: modeTone(s.mode), mark: "warn", text: modeName(s.mode) });
    }
    return deviceEntry(d, items);
  })).concat(own);
}

/**
 * vwtItems are the items of the settings of the room's virtual wall
 * thermostat while they are unsaved or their save failed (unsavedItems):
 * the disk, whose click opens them.
 */
export function vwtItems(room: PageRoom): Item[] {
  return unsavedItems(vwtFormKey(state.roomsOf || "", room.id));
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
