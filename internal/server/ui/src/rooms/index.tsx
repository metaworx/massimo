// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/rooms/index.tsx
 *
 * The rooms as parts (AP UiParts, Block 7): the House tile's modes and its
 * panel in #house-slot, a tile per room of the chosen Cube in #rooms, with
 * its device list, and the devices without a room in #unlisted-devices,
 * each built once per room or device and updated in place. The plain code
 * calls MassimoParts.rooms (rooms.js renderRooms,
 * updateDevice, showTileResult, roomUI and roomsLoaded, house.js houseOpen and
 * closeHouse);
 * the tile's actions stay plain and set the tile's live state, which the
 * tile follows. A room's forms are forgotten once a load no longer gives
 * the room, or the virtual thermostat whose settings they hold, and the
 * settings asked to be opened from elsewhere are opened once their
 * Cube's rooms have loaded. A tile's configuration is a part of the tile
 * (Config.tsx); the repairs (repair.js) and
 * the form that puts a device into a room (pairing.js) stay plain inside.
 */

import { roomFormKey } from "../model/cube";
import { fader, type Outcome } from "../shared/action";
import { forgetForms } from "../shared/forms";
import { mounter, register } from "../shared/region";
import { closeHouse, House } from "./House";
import { Tile } from "./Tile";
import { UNLISTED, Unlisted } from "./Unlisted";
import { cfgFormKey, changed, cubesLoad, house, load, newUI, planWindows, roomOf, takeOpening, tickOf, vwtFormKey } from "./store";

/** Rooms is a tile per room, kept by its Cube and its id for as long as the room is there. */
function Rooms() {
  load.useLoad();
  return <>{state.rooms.map((r) => <Tile key={(state.roomsOf || "") + ":" + r.id} room={r} />)}</>;
}

const mounts = [mounter("house-slot", () => <House />), mounter("rooms", () => <Rooms />), mounter("unlisted-devices", () => <Unlisted />)];

/** Per room, how its tile's result line is set, a success fading as an action runner's does (fader). */
const results = new Map<number, (res: Outcome | null) => void>();

/** resultOf sets the result line of the room's tile, as the page has the room when it is set. */
function resultOf(id: number): (res: Outcome | null) => void {
  let set = results.get(id);
  if (!set) {
    set = fader((res) => {
      const room = roomOf(id);
      if (room) roomUI(room).result = res;
    }, RESULT_FADE_MS);
    results.set(id, set);
  }
  return set;
}

/** What a load of a Cube's rooms gave of a room: whether it had its virtual wall thermostat, and its devices by their radio addresses. */
interface Loaded {
  vwt: boolean;
  devices: Set<string>;
}

/** Per Cube, the rooms its last load gave, by id. */
const loaded = new Map<string, Map<number, Loaded>>();

/**
 * forgetGone forgets the forms of the rooms the last load of the Cube's
 * rooms no longer gives, the settings of a virtual thermostat it no
 * longer gives, and the form of a device in a room's configuration that
 * the room no longer has: deleted, hidden, moved or removed, by this page
 * or another client. It
 * runs at loaded alone, which loadRooms calls after a load that went
 * through, with the rooms of the Cube state.roomsOf names; a load that
 * fails throws before it. It forgets only where that Cube is connected
 * with its greeting (connected of the Cubes' list): a Cube the server has
 * not greeted since it started answers no rooms at all, and a draft is not
 * forgotten at a disconnect. A Cube no longer served takes its rooms'
 * forms with it (the Cubes' forgetGone); a sign-out takes every form.
 */
function forgetGone(): void {
  const serial = state.roomsOf;
  if (!serial || !state.cubes.find((c) => c.serial === serial)?.connected) return;
  const now = new Map(state.rooms.map((r): [number, Loaded] => [r.id, { vwt: !!r.vwt, devices: new Set(r.devices.map((d) => d.rf_address)) }]));
  for (const [id, was] of loaded.get(serial) || []) {
    const is = now.get(id);
    if (!is) {
      forgetForms(roomFormKey(serial, id));
      continue;
    }
    if (was.vwt && !is.vwt) forgetForms(vwtFormKey(serial, id));
    for (const rf of was.devices) if (!is.devices.has(rf)) forgetForms(cfgFormKey(serial, id, "device:" + rf));
  }
  loaded.set(serial, now);
}

/** The functions the plain code calls, MassimoParts.rooms. */
const rooms = register("rooms", {
  /** render draws the rooms as the page has them now (renderRooms): every tile follows in place, its configuration with it. */
  render(): void {
    for (const m of mounts) m();
    load.bump();
    for (const r of state.rooms) changed(r.id);
    planWindows();
  },
  /** data takes a state event of a device of the room (updateDevice): its tile follows, its configuration with it. */
  data(id: number): void {
    changed(id);
    planWindows();
  },
  /** cubes takes a load of the Cubes' list (renderCubeSelect): the House tile's name follows the Cube's. */
  cubes(): void {
    cubesLoad.bump();
  },
  /** unlisted takes a state event of a device without a room (updateDevice): its row follows. */
  unlisted(): void {
    tickOf(UNLISTED).n++;
  },
  /**
   * loaded takes a load of the rooms that went through, before they are
   * drawn (roomsLoaded): the forms of what it no longer gives are
   * forgotten, and a virtual thermostat's settings asked to be opened
   * before its Cube's rooms were there are opened.
   */
  loaded(): void {
    forgetGone();
    takeOpening();
  },
  /**
   * result shows how an action went on the room's tile (showTileResult), a
   * success fading after a while; false where the room has no tile, which
   * the page's message line then says.
   */
  result(id: number, text: string, ok?: boolean): boolean {
    if (!roomOf(id) || !document.querySelector('.room[data-id="' + id + '"]')) return false;
    resultOf(id)(text ? { text, ok: !!ok } : null);
    return true;
  },
  /** ui makes what a room's tile keeps, live (roomUI). */
  ui: newUI,
  /** houseOpen says whether the House tile's panel is open (house.js houseOpen). */
  houseOpen(): boolean {
    return house.open;
  },
  /** closeHouse closes the House tile's panel (house.js closeHouse, Escape). */
  closeHouse,
});

/** MassimoParts.rooms, as index.ts declares it. */
export type RoomsPart = typeof rooms;
