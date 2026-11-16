// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/rooms/store.ts
 *
 * What the rooms' parts are drawn from beside the page's state.rooms,
 * which stay the page's plain objects until the data become signals (AP
 * UiParts, Block 15): the load counter of every tile, a counter per room,
 * which a state event of one of its devices raises, and the counter of
 * its configuration, which stays plain inside the tile (config.js); what
 * a tile keeps (roomUI) is live state, made here for rooms.js.
 */

import { counter } from "../shared/counter";
import { live, liveMap } from "../shared/live";
import { windowState } from "../model/device";

/** The rooms' load counter: bumped at every drawing of the rooms (renderRooms); every rooms part uses it. */
export const load = counter();

/**
 * Per room, by its id: n, raised when a device of the room changed (a
 * state event, rerenderRoom), which its tile reads; and cfg, raised when
 * its configuration is to be filled again from the room as it is now.
 */
export const ticks = liveMap(() => ({ n: 0, cfg: 0 }));

/** tickOf is a room's counters. */
export function tickOf(id: number): { n: number; cfg: number } {
  return ticks.of(String(id));
}

/** newUI is what a room's tile keeps, live (rooms.js roomUI). */
export function newUI(): RoomUI {
  return live<RoomUI>({ open: false, pending: null, timer: null, vacation: false, confirm: false, config: null, deferred: false, sending: null,
    hold: false, result: null });
}

/** roomOf is the room of the id as the page has it now, undefined once it is gone. */
export function roomOf(id: number): PageRoom | undefined {
  return state.rooms.find((r) => r.id === id);
}

/**
 * changed draws a room's tile again; its configuration (config.js) is
 * filled again too, but while the focus is in it, where the fill waits
 * until the focus has left (Tile's focusout), so that it is not built
 * again under the user's typing. With force, as the configuration asks
 * for itself (rerenderRoom), it is filled at once.
 */
export function changed(id: number, force?: boolean): void {
  const room = roomOf(id);
  const tick = tickOf(id);
  tick.n++;
  if (!room) return;
  const ui = roomUI(room);
  if (!force && configFocused(room)) {
    ui.deferred = true;
    return;
  }
  ui.deferred = false;
  tick.cfg++;
}

/** The timer of the next change of an open window's state by the clock alone. */
let windowTimer: ReturnType<typeof setTimeout> | null = null;

/**
 * planWindows draws again, when an open window's state changes by the
 * clock alone (windowState), the tiles of the rooms whose window changes
 * then.
 */
export function planWindows(): void {
  if (windowTimer) clearTimeout(windowTimer);
  windowTimer = null;
  const now = Date.now();
  let next = Infinity;
  for (const r of state.rooms) {
    for (const d of r.devices) {
      if (!d.window || windowState(d) === "") continue;
      for (const at of [Date.parse(d.window.warn_at), Date.parse(d.window.alert_at)]) {
        if (at > now && at < next) next = at;
      }
    }
  }
  if (next === Infinity) return;
  windowTimer = setTimeout(() => {
    windowTimer = null;
    for (const r of state.rooms) if (r.devices.some((d) => d.window)) changed(r.id);
    planWindows();
  }, Math.min(next - now + 250, 2 ** 31 - 1));
}
