// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/log/choices.ts
 *
 * The log's choices (AP UiParts, Block 11; log.js fillLogSelects): the
 * Cubes, shown with more than one Cube (or All Cubes chosen); every kind
 * the server sends, grouped by source; the level; who, for those who may
 * list the users; how far back; and a source or a device, the Cube's
 * devices by name and S/N, under All Cubes every Cube's, a group each.
 * settleChoices puts a device chosen right against the devices there are
 * (an old link's radio address takes its S/N, a device gone falls back
 * to all); the options are worked out as the head is drawn.
 */

import type { Device } from "../api";
import type { SelectOption } from "../shared/field";
import { LOG_KIND_SOURCE, LOG_KINDS, LOG_LEVELS, LOG_RANGES, LOG_SOURCES } from "./kinds";
import { logState } from "./state";
import { logDeviceKey, logDevicesOf, logKind, logRoomsOf } from "./text";

/** cubesKey names the Cubes and their names as the choices were filled for them (logState.cubesSeen). */
export function cubesKey(): string {
  return state.cubes.map((c) => c.serial + "\u0000" + (c.name || "")).join("\u0001");
}

/** devGroups are the devices of the device choice: under All Cubes every Cube's, a group each, else the header's Cube's. */
function devGroups(): { group: string; devs: Device[] }[] {
  return logState.cubes === "all"
    ? state.cubes.map((c) => ({ group: c.name || c.serial, devs: logDevicesOf(logRoomsOf(c.serial)) }))
    : [{ group: t("log.devicesGroup"), devs: logDevicesOf({ rooms: state.rooms, unlisted: state.unlisted }) }];
}

/**
 * settleChoices puts the device chosen right against the devices loaded
 * (logRoomsOf): one chosen by its radio address takes its S/N (but under
 * All Cubes, where it names that address on every Cube, D11-A), one gone
 * falls back to all once the devices are loaded, until then it stays as
 * the address names it; the answer is true when the choice changed so.
 */
export function settleChoices(): boolean {
  logState.cubesSeen = cubesKey();
  const all = logState.cubes === "all";
  const devs = devGroups().flatMap((g) => g.devs);
  const byRF = !all && devs.find((d) => d.rf_address === logState.device && d.serial);
  let changed = false;
  if (byRF) {
    logState.device = byRF.serial || ""; // an old link's radio address
    changed = true;
  }
  const listed = devs.some((d) => logDeviceKey(d) === logState.device);
  const unknown = logState.device !== "" && !listed && !(all && devs.some((d) => d.rf_address === logState.device));
  const known = all ? state.cubes.length > 0 && state.cubes.every((c) => logRoomsOf(c.serial)) : !!state.serial && state.roomsOf === state.serial;
  if (unknown && known) {
    logState.device = "";
    changed = true;
  }
  return changed;
}

/** cubeOptions are the Cubes choice's: this Cube, all Cubes. */
export function cubeOptions(): SelectOption[] {
  return [
    ["", t("log.thisCube")],
    ["all", t("log.allCubes")],
  ];
}

/** cubesHidden says whether the Cubes choice is left out: one Cube and This Cube chosen. */
export function cubesHidden(): boolean {
  return state.cubes.length < 2 && logState.cubes !== "all";
}

/** kindOptions are every kind the server sends, a group per source. */
export function kindOptions(): SelectOption[] {
  const groups = LOG_SOURCES.map((src) => ({ group: t("log.src." + src), options: LOG_KINDS.filter((k) => (LOG_KIND_SOURCE[k] || "system") === src).map((k): [string, string] => [k, logKind(k)]) }));
  return ([["", t("log.allKinds")]] as SelectOption[]).concat(groups);
}

/** levelOptions are the levels and above, but error alone. */
export function levelOptions(): SelectOption[] {
  return LOG_LEVELS.filter((l) => l !== "error").map((l): SelectOption => [l, t("log.level." + l)]);
}

/** userOptions are everyone and the users listed, and the user chosen where the list has it not (an address's). */
export function userOptions(): SelectOption[] {
  const users = logState.users || [];
  const out: SelectOption[] = [["", t("log.allUsers")]];
  for (const u of users) out.push([u.name, u.display || u.display_name || u.name]);
  if (logState.user && !users.some((u) => u.name === logState.user)) out.push([logState.user, logState.user]);
  return out;
}

/** usersHidden says whether the who choice is left out: no users listed and none chosen. */
export function usersHidden(): boolean {
  return (logState.users || []).length === 0 && !logState.user;
}

/** rangeOptions are the ranges of how far back. */
export function rangeOptions(): SelectOption[] {
  return LOG_RANGES.map((r): SelectOption => [r, t("log.range." + r)]);
}

/** deviceOptions are every device, the sources, then the devices by group; a device chosen that is not listed, as its address names it. */
export function deviceOptions(): SelectOption[] {
  const groups = devGroups();
  const devs = groups.flatMap((g) => g.devs);
  const options: SelectOption[] = ([["", t("log.allDevices")]] as SelectOption[])
    .concat(LOG_SOURCES.map((src): SelectOption => ["src:" + src, t("log.src." + src)]))
    .concat(
      groups
        .filter((g) => g.devs.length)
        .map((g) => ({ group: g.group, options: g.devs.map((d): [string, string] => [logDeviceKey(d), d.name ? d.name + " · " + logDeviceKey(d) : logDeviceKey(d)]) })),
    );
  if (logState.device !== "" && !devs.some((d) => logDeviceKey(d) === logState.device)) options.push([logState.device, logState.device]);
  return options;
}

/** deviceValue is the device choice's value: the device, else the source as src:…. */
export function deviceValue(): string {
  return logState.device || (logState.source ? "src:" + logState.source : "");
}

/** clearOff says whether Clear is off: every choice at its default, no text filter, this Cube, every kind, source and device, info and everyone. */
export function clearOff(): boolean {
  return !(logState.filter || logState.cubes || logState.kind || logState.device || logState.source || logState.user || logState.level !== "info");
}
