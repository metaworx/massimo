// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/shared/time.ts
 *
 * A DateTime's value ("YYYY-MM-DDTHH:MM", the browser's time) and the
 * API's time as each other: for a field that shows a stored end and sends
 * it (a user's end), whose draft compares the minute the field holds.
 * Moved from users.js with System settings → Users (AP UiParts, Block
 * 10); rooms.js apiUntil, the stepped setpoint's end, stays with its
 * users. An API time as date and time, fmtWhen, moved from backup.js
 * with Backup and restore; the plain page calls it as fmtWhen of
 * backup.js (MassimoParts.fmtWhen) until Block 13.
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
