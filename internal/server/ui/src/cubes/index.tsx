// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/cubes/index.tsx
 *
 * The Cubes as a part (AP UiParts, Block 5): System settings → Cubes, the
 * served Cubes each in its panel and the fold of those found, and Cube
 * settings, the chosen Cube's panel alone, open. A panel is drawn in one
 * place at a time, as its ids are its serial's. The plain code calls
 * MassimoParts.cubes (cubes.js renderCubes and cubesPolled, home.js
 * showFound); the panels follow every load of the Cubes' list in place.
 */

import { useEffect } from "preact/hooks";
import type { Cube } from "../api";
import { chosenCube } from "../model/cube";
import { mounter, register } from "../shared/region";
import { msg } from "./send";
import { Found } from "./Found";
import { Panel } from "./Panel";
import { cubeUI, folds, load, loadDevices, panelLoads, place } from "./store";

/** List is System settings → Cubes: a panel per served Cube, then the fold of those found; empty while Cube settings holds the panel. */
function List() {
  load.use();
  if (place.value !== "list") return null;
  return (
    <>
      {!state.cubes.length && <p class="muted small">{t("cubes.none")}</p>}
      {state.cubes.map((c) => <Panel key={c.serial} c={c} page={false} />)}
      {state.found.length > 0 && <Found found={state.found} />}
    </>
  );
}

/** pageLoaded is the serial Cube settings has loaded the pairing and the devices for since it was last shown. */
let pageLoaded = "";

/** PagePanel is Cube settings' panel of a Cube, which loads what it shows once per Cube and showing of the page. */
function PagePanel({ c }: { c: Cube }) {
  useEffect(() => {
    if (pageLoaded === c.serial) return;
    pageLoaded = c.serial;
    panelLoads(c.serial);
  });
  return <Panel c={c} page />;
}

/** Page is Cube settings: the panel of the Cube chosen in the header, open, or that no Cube is configured. */
function Page() {
  load.use();
  if (place.value !== "page") return null;
  const c = chosenCube();
  if (!c) return <p class="muted small">{t("cubes.none")}</p>;
  return <PagePanel key={c.serial} c={c} />;
}

const mountList = mounter("cubes-list", () => <List />);
const mountPage = mounter("cube-page-body", () => <Page />);

/** The functions the plain code calls, MassimoParts.cubes. */
const cubes = register("cubes", {
  /** render draws the Cubes where they are shown now: on Cube settings while it is shown, else in System settings (renderCubes). */
  render(): void {
    mountList();
    mountPage();
    place.value = isShown("cube") ? "page" : "list";
    load.bump();
  },
  /** polled takes a load of the Cubes' list (cubesPolled): every panel follows it in place. */
  polled(): void {
    mountList();
    mountPage();
    load.bump();
  },
  /** openFound opens the fold of the Cubes found (showFound). */
  openFound(): void {
    folds.found = true;
  },
});

/** MassimoParts.cubes, as index.ts declares it. */
export type CubesPart = typeof cubes;

// Cube settings draws its panel when it is shown, which loads what it
// shows; System settings draws its list, and the panels still open from
// before load what they would load when opened.
onView("cube", () => {
  msg("");
  pageLoaded = "";
  renderCubes();
});
onView("system", () => {
  msg("");
  renderCubes();
  for (const c of state.cubes) {
    if (!cubeUI.of(c.serial).open) continue;
    loadDevices(c.serial);
    loadPairing(c.serial);
    loadCubeRooms(c.serial);
  }
});
