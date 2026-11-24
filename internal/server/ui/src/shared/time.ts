// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/shared/time.ts
 *
 * A DateTime's value ("YYYY-MM-DDTHH:MM", the browser's time) and the
 * API's time as each other: for a field that shows a stored end and sends
 * it (a user's end), whose draft compares the minute the field holds.
 * Moved from users.js with System settings → Users (AP UiParts, Block
 * 10); apiUntil, the API's form of a holiday's end, is model/room.ts'. An API time as date and time, fmtWhen, moved from backup.js
 * with Backup and restore; the notifications' card reads it too.
 */

/** localOf is an API time as a DateTime's value, "" for none (localDateTimeValue of app.js). */
export function localOf(iso: string | null | undefined): string {
  const ms = Date.parse(iso || "");
  return isNaN(ms) ? "" : localDateTimeValue(new Date(ms));
}

/** isoOf is a DateTime's value as an API time, null where it is empty or no time. */
export function isoOf(local: string): string | null {
  if (!local) return null;
  const d = new Date(local);
  return isNaN(d.getTime()) ? null : d.toISOString();
}

/** fmtWhen is an API time as date and time, in the page's language, the hour of 24. */
export function fmtWhen(iso: string): string {
  return new Intl.DateTimeFormat(i18n.lang, { year: "numeric", month: "2-digit", day: "2-digit", hour: "2-digit", minute: "2-digit", hour12: false })
    .format(new Date(iso));
}

/**
 * fmtServerClock is an API time as the server's clock reads it: at the
 * offset the time carries (Z for none), which the server writes in its
 * own zone, in the page's language, the hour of 24; "" for no time.
 */
export function fmtServerClock(iso: string): string {
  const ms = Date.parse(iso || "");
  if (isNaN(ms)) return "";
  const m = /([+-])(\d\d):(\d\d)$/.exec(iso);
  const offset = m ? (m[1] === "-" ? -1 : 1) * (Number(m[2]) * 60 + Number(m[3])) : 0;
  return new Intl.DateTimeFormat(i18n.lang, { year: "numeric", month: "2-digit", day: "2-digit", hour: "2-digit", minute: "2-digit", hour12: false, timeZone: "UTC" })
    .format(new Date(ms + offset * 60000));
}
