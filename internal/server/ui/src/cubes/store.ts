// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/cubes/store.ts
 *
 * What the Cubes' parts are drawn from beside the page's state.cubes and
 * state.found, which stay the page's plain objects until the data become
 * signals (AP UiParts, Block 15): the load counter, where they are
 * drawn, what is kept per Cube and per fold, and
 * the devices loaded for the hide choice.
 */

import { signal } from "@preact/signals";
import type { Device } from "../api";
import { counter } from "../shared/counter";
import { live, liveMap } from "../shared/live";
import { cubePath } from "../shared/path";
import { session, stillSession } from "../shared/session";
import { loadCubeRooms, loadPairing } from "./flow";

/** The Cubes' load counter: bumped at every load of the list and every redraw asked for (renderCubes, cubesPolled); every Cubes part uses it. */
export const load = counter();

/**
 * Where the Cubes' panels are drawn: System settings → Cubes, or Cube
 * settings, the chosen Cube's alone; a panel is in one place at a time,
 * as its ids are its serial's.
 */
export const place = signal<"list" | "page">("list");

/** What a Cube's panel keeps across its being drawn in another place: open, in System settings' list. */
export const cubeUI = liveMap(() => ({ open: false }));

/** The found Cubes' fold and its lines for the settings file, open or not (showFound opens the first). */
export const folds = live({ found: false, lines: false });

/** Per serial, the Cube's visible devices, for the hide choice; loaded when its panel opens. */
export const devices = signal<Record<string, Device[]>>({});

/** deviceOf is a device of any Cube's list loaded, by its radio address, for the names in the hidden list. */
export function deviceOf(rf: string): Device | undefined {
  for (const list of Object.values(devices.value)) {
    const d = list.find((x) => x.rf_address === rf);
    if (d) return d;
  }
  return undefined;
}

/** loadDevices reads a Cube's visible devices for the hide choice; none on an error. */
export async function loadDevices(serial: string): Promise<void> {
  const n = session();
  let list: Device[] = [];
  try {
    list = (await api<Device[]>("GET", cubePath(serial) + "/devices?devices=real")) || [];
  } catch {
    list = [];
  }
  if (!stillSession(n)) return; // an answer after a sign-out: nothing of it for the next user
  devices.value = { ...devices.value, [serial]: list };
}

/**
 * panelLoads are what a panel loads when it is opened or shown: its
 * devices where not yet loaded, its pairing, and the rooms its pairing
 * offers (flow.ts).
 */
export function panelLoads(serial: string): void {
  if (!devices.value[serial]) loadDevices(serial);
  loadPairing(serial);
  loadCubeRooms(serial);
}
