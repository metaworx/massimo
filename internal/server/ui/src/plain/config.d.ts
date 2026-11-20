// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/plain/config.d.ts
 *
 * The globals of the plain script ui/config.js that the parts call: the
 * kit the week programme editor builds with. How long a name may be and
 * the name a device is offered are the parts' own since AP UiParts'
 * Block 9 (shared/name.ts). A room's configuration
 * itself is a part since AP UiParts' Block 8 (rooms/Config.tsx).
 */

declare global {
  /** What the week programme editor builds with: the page's translation, its builders and its temperatures. */
  function weekKit(): WeekKit;
}

export {};
