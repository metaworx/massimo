// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/rooms/store.ts
 *
 * What the rooms' parts are drawn from beside the page's state.rooms,
 * which stay the page's plain objects until the data become signals (AP
 * UiParts, Block 15): the load counter of every tile, a counter per room,
 * which a state event of one of its devices raises; what
 * a tile keeps (roomUI) is live state, made here for rooms.js, and so is
 * what the House tile's panel keeps; the keys a room's forms are kept by
 * (shared/forms.ts), and the opening of a room's form asked for from
 * anywhere: a virtual thermostat's settings, a tab of its configuration.
 */

import { roomFormKey } from "../model/cube";
import { counter } from "../shared/counter";
import { focusForm } from "../shared/forms";
import { live, liveMap } from "../shared/live";
import { windowState } from "../model/device";
import { isShown, showView } from "../shared/pages";

/** The rooms' load counter: bumped at every drawing of the rooms (renderRooms); every rooms part uses it. */
export const load = counter();

/**
 * The Cubes' load counter, bumped at every load of the Cubes' list
 * (renderCubeSelect of header.js): the House tile's name, the Cube's,
 * follows a rename that the poll brings without a load of the rooms.
 */
export const cubesLoad = counter();

/** Per room, by its id: n, raised when a device of the room changed (a state event), which its tile and its configuration read. */
export const ticks = liveMap(() => ({ n: 0 }));

/** tickOf is a room's counter. */
export function tickOf(id: number): { n: number } {
  return ticks.of(String(id));
}

/** newUI is what a room's tile keeps, live (rooms.js roomUI). */
export function newUI(): RoomUI {
  return live<RoomUI>({ open: false, pending: null, timer: null, vacation: false, confirm: false, config: false, sending: null, hold: false, result: null });
}

/**
 * What the House tile's panel keeps, across its close too: open, the mode
 * chosen, permanently or until, the temperature and the end typed, the
 * rooms unticked (a room is ticked until it is unticked), and the result
 * of the last Send.
 */
export const house = live({
  open: false,
  mode: "auto",
  duration: "permanent" as "permanent" | "until",
  temp: "20",
  until: "",
  ticks: {} as Record<number, boolean>,
  result: null as { text: string; ok: boolean } | null,
});

/**
 * vwtFormKey is the key the settings of a room's virtual wall thermostat
 * are kept by (useForm), under the room's of its Cube (roomFormKey): one
 * virtual thermostat per room, whatever its radio address.
 */
export function vwtFormKey(serial: string, id: number): string {
  return roomFormKey(serial, id) + ":vwt";
}

/**
 * cfgFormKey is the key a form of a room's configuration is kept by
 * (useForm), under the room's (roomFormKey) beside the virtual
 * thermostat's: "<room>:cfg:<form>", form a tab's (room, temps, program,
 * copy), the radiator settings' (valves) or a device's ("device:<radio
 * address>"); without a form, the beginning of them all.
 */
export function cfgFormKey(serial: string, id: number, form?: string): string {
  return roomFormKey(serial, id) + ":cfg" + (form === undefined ? "" : ":" + form);
}

/**
 * A room's form asked to be opened (openRoom), by its Cube and its room,
 * with what opens it, until that Cube's rooms have loaded: choosing a
 * Cube empties the tiles' state (selectCube), so the request waits for
 * the load (takeOpening).
 */
let opening: { serial: string; id: number; then: (room: PageRoom) => void } | null = null;

/**
 * openRoom shows a form of a room from any page: its Cube chosen where
 * another is, the rooms shown, then, once that Cube's rooms are there,
 * then(room) opens it.
 */
export function openRoom(serial: string, id: number, then: (room: PageRoom) => void): void {
  opening = { serial, id, then };
  if (state.serial !== serial) selectCube(serial);
  if (!isShown("tiles")) showView("tiles");
  takeOpening();
}

/**
 * openSettings shows the settings of a room's virtual wall thermostat and
 * puts the focus into them, from any page (openRoom): the room's device
 * list opened as its info button opens it (toggleDevices) and the focus
 * put into the form (focusForm), which brings the row into view.
 */
export function openSettings(serial: string, id: number): void {
  openRoom(serial, id, (room) => {
    if (!roomUI(room).open) toggleDevices(room, null);
    focusForm(vwtFormKey(serial, id));
  });
}

/**
 * takeOpening carries out the opening asked for where the rooms loaded are
 * its Cube's (openRoom), at once or after the load (loaded); a request
 * for a Cube no longer chosen is dropped, and so is one for a room gone.
 */
export function takeOpening(): void {
  const o = opening;
  if (!o) return;
  if (state.serial !== o.serial) {
    opening = null;
    return;
  }
  if (state.roomsOf !== o.serial) return; // its rooms not loaded yet
  opening = null;
  const room = roomOf(o.id);
  if (room) o.then(room);
}

/** roomOf is the room of the id as the page has it now, undefined once it is gone. */
export function roomOf(id: number): PageRoom | undefined {
  return state.rooms.find((r) => r.id === id);
}

/** changed draws a room's tile again, its configuration with it, in place: with the focus in it or not. */
export function changed(id: number): void {
  tickOf(id).n++;
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
