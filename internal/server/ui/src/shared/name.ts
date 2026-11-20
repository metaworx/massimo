// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/shared/name.ts
 *
 * A name as the Cube keeps it: at most NAME_BYTES bytes of UTF-8, and the
 * name a device is offered where it is put into a room (PutForm, D200-A)
 * or listed in one (a repair's listing, D185), as the server's default
 * (repair_api.go, defaultDeviceName) with the type's word in the page's
 * language. Moved from config.js with the pairing and the repairs (AP
 * UiParts, Block 9).
 */

/** NAME_BYTES is the longest name of a room or a device the Cube takes, in bytes. */
export const NAME_BYTES = 32;

/** byteLength is a text's length in UTF-8 bytes. */
export function byteLength(s: string): number {
  return new TextEncoder().encode(s).length;
}

/** nameFits says whether a name, trimmed, is one the Cube takes. */
export function nameFits(s: string): boolean {
  return byteLength(s.trim()) <= NAME_BYTES;
}

/**
 * freeNameNumber is the lowest number from 1 that no name of taken has
 * in the form "<prefix> <n>", whatever its case.
 */
function freeNameNumber(prefix: string, taken: string[]): number {
  const p = prefix.toLowerCase() + " ";
  const used = new Set<number>();
  for (const name of taken) {
    const s = String(name || "").trim();
    if (!s.toLowerCase().startsWith(p)) continue;
    const rest = s.slice(p.length);
    if (/^[0-9]+$/.test(rest) && Number(rest) > 0) used.add(Number(rest));
  }
  let n = 1;
  while (used.has(n)) n++;
  return n;
}

/**
 * defaultDeviceName is the name a device is offered in a room: "<room
 * name> <type> <n>", word the type in the page's language, n the lowest
 * number from 1 that no name of the room's (taken) has in that form,
 * whatever its case. One longer than NAME_BYTES is offered as it is: its
 * field says so, and the user shortens it before it is sent (D201),
 * where the server, with nobody to ask, drops the room's name.
 */
export function defaultDeviceName(room: string | undefined, word: string, taken: string[]): string {
  const prefix = (String(room || "").trim() + " " + word).trim();
  return prefix + " " + freeNameNumber(prefix, taken);
}
