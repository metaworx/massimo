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
import { focusForm } from "../shared/forms";
import { effect } from "@preact/signals";
import { windowNow } from "../shared/data";
import { live } from "../shared/live";
import { windowState } from "../model/device";
import { isShown, showView } from "../shared/pages";


/** newUI is what a room's tile keeps, live (rooms.js roomUI). */
export function newUI(): RoomUI {
  return live<RoomUI>({ open: false, pending: null, timer: null, vacation: false, confirm: false, config: false, sending: null, hold: false, result: null,
    awaiting: null, awaitTimer: null });
}

/**
 * What the House tile's panel keeps, across its close too: open, the mode
 * chosen, permanently or until, the temperature and the end, whether the
 * user changed either (tempTyped since the panel opened, untilTyped while
 * the end typed is still to come at an open), so that a choice gives its
 * defaults to the others alone, whether Send refused the end as passed
 * (untilRefused, until it is changed), the rooms unticked (a room is
 * ticked until it is unticked), and the result of the last Send.
 */
export const house = live({
  open: false,
  mode: "auto",
  duration: "permanent" as "permanent" | "until",
  temp: "20",
  until: "",
  tempTyped: false,
  untilTyped: false,
  untilRefused: false,
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

/** The timer of the next change of an open window's state by the clock alone. */
let windowTimer: ReturnType<typeof setTimeout> | null = null;

/**
 * planWindows raises windowNow when an open window's state changes by the
 * clock alone (windowState), so that what shows it is drawn again then;
 * an effect, run again at every write of the rooms or of a room's
 * devices.
 */
function planWindows(): void {
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
    windowNow.value++;
  }, Math.min(next - now + 250, 2 ** 31 - 1));
}
effect(() => {
  void windowNow.value; // the next time planned anew once one has come
  planWindows();
});
