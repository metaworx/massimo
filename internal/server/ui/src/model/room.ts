// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/model/room.ts
 *
 * A room's and a device's facts as the rooms' parts and the log read them
 * (UiParts D218, from rooms.js, which keeps the facts its own actions
 * use): the device whose measured temperature stands for the room, how
 * long a stale device has not been seen, a holiday's end as the API takes
 * it, and what a device has not shown of the writes to it, as an amber
 * problem of its tile and its row.
 */

import type { Device } from "../api";
import type { Item } from "../shared/problems";
import { fmtSetpoint } from "../shared/setpoint";

/**
 * roomActualDevice is the device whose measured temperature stands for
 * the room: its virtual wall thermostat, which the server computes from
 * the wall thermostat, else the radiator thermostats' median or mean, so
 * that every client shows the same; null while it has none.
 */
export function roomActualDevice(room: PageRoom): Device | null {
  const v = room.vwt;
  return v && v.state && v.state.actual_temp !== undefined ? v : null;
}

/** roomActual is the room's measured temperature, undefined while it has none. */
export function roomActual(room: PageRoom): number | undefined {
  const d = roomActualDevice(room);
  return d ? d.state?.actual_temp : undefined;
}

/** staleText is the warning for a device the API calls stale ("Not seen for 30 hours", from two days on in days), "" for any other. */
export function staleText(d: Device): string {
  if (!d.stale) return "";
  const ms = Date.parse(d.last_seen || "");
  const h = isNaN(ms) ? 24 : Math.max(0, Math.floor((Date.now() - ms) / 3600000));
  return h < 48 ? tn("stale.hours", h) : tn("stale.days", Math.floor(h / 24));
}

/** apiUntil turns a datetime-local value into the API's "YYYY-MM-DD HH:MM". */
export function apiUntil(value: string): string {
  return (value || "").replace("T", " ").slice(0, 16);
}

/**
 * confirmFieldName names a value a write's confirmation waits for (the
 * server's confirm.go): the state's mode, setpoint, vacation end, key lock
 * and display, a configuration's by the log's names, a day of the week
 * programme by its day.
 */
export function confirmFieldName(f: string): string {
  const day = /^program\.([a-z]+)(\.2)?$/.exec(f);
  if (day) return t("confirm.program", { day: t("day." + day[1]) });
  if (f === "locked") return t("confirm.locked");
  if (f === "display_actual") return t("confirm.display");
  if (f === "mode" || f === "setpoint" || f === "until") return t("log.disagree.what." + f);
  const key = "log.cfg." + f;
  return t(key) === key ? f : t(key);
}

/** confirmShown is what a device's state shows of a value its write waits for, a setpoint as its display shows it (OFF for 4.5 °C); "" for a configuration's value, which the state does not hold. */
function confirmShown(d: Device, f: string): string {
  const s = d.state || {};
  switch (f) {
    case "mode":
      return s.mode ? modeName(s.mode) : "";
    case "setpoint":
      return s.setpoint !== undefined ? fmtSetpoint(s.setpoint) : "";
    case "until":
      return s.until ? fmtUntil(s.until) : "";
    case "locked":
      return t(s.panel_locked ? "confirm.lockedOn" : "confirm.lockedOff");
  }
  return "";
}

/**
 * confirmFieldsText lists what a device has not shown of the writes to
 * it, each value with what the device shows where its state says: "mode
 * (shows auto), setpoint (shows 20.0 °C)".
 */
export function confirmFieldsText(d: Device): string {
  const out: string[] = [];
  for (const f of (d.confirmation && d.confirmation.fields) || []) {
    const shown = confirmShown(d, f);
    const text = confirmFieldName(f) + (shown ? " (" + t("confirm.shows", { value: shown }) + ")" : "");
    if (!out.includes(text)) out.push(text); // a long day's two parts are one day
  }
  return out.join(", ");
}

/**
 * confirmItem is the mark of a device that has not shown a write by its
 * deadline, an amber problem of its tile and of its row whose title says
 * what it does not show; none while the write is pending, nor for a
 * virtual wall thermostat, whose room's devices carry it.
 */
export function confirmItem(d: Device): Item | null {
  const c = d.confirmation;
  if (!c || c.state !== "unconfirmed" || d.virtual) return null;
  return { kind: "unconfirmed", icon: "icon-unconfirmed", tone: "warn", text: t("badge.notConfirmed"),
    title: t("badge.notConfirmedTitle", { fields: confirmFieldsText(d) }) };
}
