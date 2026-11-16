// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/rooms/index.tsx
 *
 * The rooms as parts (AP UiParts, Block 7): a tile per room of the chosen
 * Cube in #rooms, with its device list, each built once per room and
 * updated in place. The plain code calls MassimoParts.rooms (rooms.js
 * renderRooms, rerenderRoom, updateDevice, showTileResult and roomUI);
 * the tile's actions stay plain and set the tile's live state, which the
 * tile follows. The configuration (config.js) and the repairs (repair.js)
 * stay plain inside the tile.
 */

import { fader, type Outcome } from "../shared/action";
import { mounter, register } from "../shared/region";
import { Tile } from "./Tile";
import { changed, load, newUI, planWindows, roomOf } from "./store";

/** Rooms is a tile per room, kept by its Cube and its id for as long as the room is there. */
function Rooms() {
  load.use();
  return <>{state.rooms.map((r) => <Tile key={(state.roomsOf || "") + ":" + r.id} room={r} />)}</>;
}

const mountRooms = mounter("rooms", () => <Rooms />);

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

/** The functions the plain code calls, MassimoParts.rooms. */
const rooms = register("rooms", {
  /** render draws the rooms as the page has them now (renderRooms): every tile follows in place, its configuration filled again unless the focus is in it. */
  render(): void {
    mountRooms();
    load.bump();
    for (const r of state.rooms) changed(r.id);
    planWindows();
  },
  /** data takes a state event of a device of the room (updateDevice): its tile follows, its configuration is filled again once the focus is not in it. */
  data(id: number): void {
    changed(id);
    planWindows();
  },
  /** room draws the room's tile again and fills its configuration again at once, as the configuration asks for (rerenderRoom). */
  room(id: number): void {
    changed(id, true);
    planWindows();
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
});

/** MassimoParts.rooms, as index.ts declares it. */
export type RoomsPart = typeof rooms;
