// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/header/Choice.tsx
 *
 * The Cube choice in the header, a menu of the page (the shared Picker):
 * closed, the cube in the chosen Cube's link's colour and its name;
 * open, each Cube with the cube, its name, its sign and the icons of its
 * problems after it, its serial and address, then those found on the LAN and not served, then
 * the entry that searches the LAN for new ones, and in the app "Select
 * server", which shows the app's servers. Built once and updated in
 * place, so that an open choice stays open, with the focus where it was,
 * and shows what a poll brings.
 */

import type { Cube } from "../api";
import { CubeIcon, CubeSign, chosenCube, cubeEntry } from "../model/cube";
import { Picker, type PickEntry } from "../shared/menu";
import { KindIcons, problemKinds } from "../shared/problems";
import { discoverCubes } from "./send";
import { discovery } from "./store";
import { can } from "../shared/me";
import { useLook } from "../shared/look";

/** DISCOVER is the value of the entry that searches the LAN. */
const DISCOVER = "__discover";

/** SERVERS is the value of the app's entry "Select server" (docs/app.md, contract version 2). */
const SERVERS = "__servers";

/** connectOf is the app's way back to its servers, null in a browser or an app without it. */
function connectOf(): (() => Promise<null>) | null {
  const b = appBridge();
  const go = b?.connect;
  return b && typeof go === "function" ? () => go.call(b) : null;
}

/**
 * entryOf is a served Cube's entry: the cube, its name (its serial without
 * one), after it its sign, always, then the icons of its problems, its
 * link left out, which its sign shows, and the settings' cog for a role
 * with settings; under it its serial and address.
 */
function entryOf(c: Cube): PickEntry {
  const entries = [cubeEntry(c, { settings: can("settings") })];
  const skip = ["net"];
  return {
    value: c.serial,
    label: c.name || c.serial,
    sub: [c.name ? c.serial : "", c.address || ""].filter(Boolean).join(" · "),
    icon: <CubeIcon />,
    icons: (
      <>
        <CubeSign c={c} />
        {problemKinds(entries, skip).length > 0 && <KindIcons entries={entries} skip={skip} />}
      </>
    ),
  };
}

/** pick acts on a choice: the search, the app's servers, a found Cube's way to System settings, or the Cube chosen. */
function pick(v: string): void {
  if (v === DISCOVER) discoverCubes();
  else if (v === SERVERS) connectOf()?.().catch(() => {});
  else if (v.startsWith("found:")) showFound();
  else selectCube(v);
}

/**
 * Choice is the Cube choice: the served Cubes; those found on the LAN and
 * not served, which open System settings → Cubes for a role with cubes
 * and are information for others; for a role with discover, the search;
 * in the app, "Select server".
 * Closed it shows the cube in the chosen Cube's link's colour, green,
 * amber or red, and its name; folded to a button (fold.ts) the cube
 * alone. Its tooltip names the chosen Cube, its serial, address and
 * link, which its name for a screen reader says too (AP GlobalMenu,
 * D200-A, D201-A).
 */
export function Choice() {
  useLook();
  const entries: PickEntry[] = state.cubes.map(entryOf);
  if (state.found.length) {
    entries.push({ group: t("discover.group") });
    for (const c of state.found) entries.push({ value: "found:" + c.serial, label: c.serial, sub: c.address || "", disabled: !can("cubes") });
  }
  if (can("discover")) {
    entries.push({ separator: true });
    entries.push({ value: DISCOVER, label: t(discovery.running ? "discover.searching" : "discover.search"), disabled: discovery.running });
  }
  if (connectOf()) {
    entries.push({ separator: true });
    entries.push({ value: SERVERS, label: t("app.selectServer") });
  }
  const cube = chosenCube();
  const name = cube ? cube.name || cube.serial : t("header.cube");
  const conn = cube ? linkWord(cube) : "";
  return (
    <Picker id="cube-menu" cls="cube-menu" summaryId="cube-choice" summaryCls="cube-choice" entries={entries} value={state.serial || ""} onPick={pick}
      shown={{
        icon: <CubeIcon state={cube ? linkSign(cube) : undefined} />,
        label: name,
        title: cube ? [name, cube.name ? cube.serial : "", cube.address || "", conn].filter(Boolean).join(" · ") : name,
        aria: cube ? t("header.cubeChoice", { name, state: conn }) : name,
      }} />
  );
}
