// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/plain/config.d.ts
 *
 * The globals of the plain script ui/config.js that the parts call: how
 * long a name may be, which pairing and the repairs check too, the name a
 * device is offered, and the kit the week programme editor builds with. A room's configuration
 * itself is a part since AP UiParts' Block 8 (rooms/Config.tsx).
 */

declare global {
  /** The longest name of a room or a device the Cube takes, in bytes. */
  const NAME_BYTES: number;
  /** A text's length in UTF-8 bytes. */
  function byteLength(s: string): number;
  /** The name a device is offered in a room: the room's name, the type's word and the lowest number no name taken has in that form. */
  function defaultDeviceName(room: string, word: string, taken: string[]): string;
  /** What the week programme editor builds with: the page's translation, its builders and its temperatures. */
  function weekKit(): WeekKit;
}

export {};
