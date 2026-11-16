// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/header/Choice.tsx
 *
 * The Cube choice in the header, a menu of the page (the shared Picker):
 * closed, the chosen Cube's network sign and its name; open, each Cube
 * with its sign, its name with the icons of its problems after it, its
 * serial and address, then those found on the LAN and not served, then
 * the entry that searches the LAN for new ones. Built once and updated in
 * place, so that an open choice stays open, with the focus where it was,
 * and shows what a poll brings.
 */

import type { Cube } from "../api";
import { CubeSign, chosenCube, cubeEntry } from "../model/cube";
import { Picker, type PickEntry } from "../shared/menu";
import { KindIcons, problemKinds } from "../shared/problems";
import { discoverCubes } from "./send";
import { discovery, load } from "./store";

/** DISCOVER is the value of the entry that searches the LAN. */
const DISCOVER = "__discover";

/**
 * entryOf is a served Cube's entry: its sign, its name (its serial without
 * one), the icons of its problems after it, its link left out, which its
 * sign shows, and the settings' cog for a role with settings; under it its
 * serial and address.
 */
function entryOf(c: Cube): PickEntry {
  const entries = [cubeEntry(c, hasCap("settings"))];
  const skip = ["net"];
  return {
    value: c.serial,
    label: c.name || c.serial,
    sub: [c.name ? c.serial : "", c.address || ""].filter(Boolean).join(" · "),
    icon: <CubeSign c={c} />,
    icons: problemKinds(entries, skip).length ? <KindIcons entries={entries} skip={skip} /> : undefined,
  };
}

/** pick acts on a choice: the search, a found Cube's way to System settings, or the Cube chosen. */
function pick(v: string): void {
  if (v === DISCOVER) discoverCubes();
  else if (v.startsWith("found:")) showFound();
  else selectCube(v);
}

/**
 * Choice is the Cube choice: the served Cubes; those found on the LAN and
 * not served, which open System settings → Cubes for a role with cubes
 * and are information for others; for a role with discover, the search.
 * Its tooltip names the chosen Cube's serial, address and link.
 */
export function Choice() {
  load.useLoad();
  const entries: PickEntry[] = state.cubes.map(entryOf);
  if (state.found.length) {
    entries.push({ group: t("discover.group") });
    for (const c of state.found) entries.push({ value: "found:" + c.serial, label: c.serial, sub: c.address || "", disabled: !hasCap("cubes") });
  }
  if (hasCap("discover")) {
    entries.push({ separator: true });
    entries.push({ value: DISCOVER, label: t(discovery.running ? "discover.searching" : "discover.search"), disabled: discovery.running });
  }
  const cube = chosenCube();
  const name = cube ? cube.name || cube.serial : t("header.cube");
  const conn = cube ? linkWord(cube) : "";
  return (
    <Picker id="cube-menu" cls="cube-menu" summaryId="cube-choice" summaryCls="cube-choice" entries={entries} value={state.serial || ""} onPick={pick}
      shown={{
        icon: cube && <CubeSign c={cube} />,
        label: name,
        title: cube ? [name, cube.name ? cube.serial : "", cube.address || "", conn].filter(Boolean).join(" · ") : name,
        aria: cube ? t("header.cubeChoice", { name, state: conn }) : name,
      }} />
  );
}
