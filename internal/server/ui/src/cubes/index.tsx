// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/cubes/index.tsx
 *
 * The Cubes as a part (AP UiParts, Block 5): System settings → Cubes, the
 * served Cubes each in its panel and the fold of those found, and Cube
 * settings, the chosen Cube's panel alone, open. A panel is drawn in one
 * place at a time, as its ids are its serial's. The plain code calls
 * MassimoParts.cubes (app.js' loads and stream, home.js showFound); the
 * panels follow the Cubes' list in place, the settings typed for a Cube
 * that is no longer served, and for its rooms, are forgotten, and the
 * Cubes card's head says at a glance how they stand (an effect, below).
 */

import { useEffect } from "preact/hooks";
import type { Cube } from "../api";
import { chosenCube, cubeFormKey, roomFormKey } from "../model/cube";
import { forgetForms } from "../shared/forms";
import { mounter, register } from "../shared/region";
import { forgetPairing, loadCubeRooms, loadPairing, pairingEvent, pairingJoined } from "./flow";
import { msg } from "./send";
import { Found } from "./Found";
import { Panel } from "./Panel";
import { cubeUI, devices, folds, loadDevices, panelLoads, place } from "./store";
import { isShown, onView } from "../shared/pages";
import { useLook } from "../shared/look";
import { effect } from "@preact/signals";
import { type BadgeSpec, cardBadges } from "../shared/card";
import { lang } from "../shared/lang";

/** List is System settings → Cubes: a panel per served Cube, then the fold of those found; empty while Cube settings holds the panel. */
function List() {
  useLook();
  if (place.value !== "list") return null;
  return (
    <>
      {!state.cubes.length && <p class="muted small">{t("cubes.none")}</p>}
      {state.cubes.map((c) => <Panel key={c.serial} c={c} page={false} />)}
      {state.found.length > 0 && <Found found={state.found} />}
    </>
  );
}

/** pageLoaded is the serial Cube settings has loaded the pairing and the devices for since it was last shown: its showing loads them, and its panel where another Cube is chosen while it is shown. */
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
  useLook();
  if (place.value !== "page") return null;
  const c = chosenCube();
  if (!c) return <p class="muted small">{t("cubes.none")}</p>;
  return <PagePanel key={c.serial} c={c} />;
}

/** served are the serials of the Cubes the list gave last. */
let served = new Set<string>();

/**
 * forgetGone forgets the settings typed for the Cubes the list no longer
 * gives, the Cube's own and its rooms': removed, by this page or another
 * client. It runs at polled
 * alone, which loadCubes calls after a load that went through; a load
 * that fails throws before it, and leaves state.cubes as it was.
 */
function forgetGone(): void {
  const now = new Set(state.cubes.map((c) => c.serial));
  for (const serial of served) {
    if (now.has(serial)) continue;
    forgetForms(cubeFormKey(serial));
    forgetForms(roomFormKey(serial));
  }
  served = now;
}

const mountList = mounter("cubes-list", () => <List />);
const mountPage = mounter("cube-page-body", () => <Page />);

/** The functions the plain code calls, MassimoParts.cubes. */
const cubes = register("cubes", {
  /** render draws the Cubes where they are shown now: on Cube settings while it is shown, else in System settings (home.js, a Cube found shown). */
  render(): void {
    mountList();
    mountPage();
    place.value = isShown("cube") ? "page" : "list";
  },
  /** polled takes a load of the Cubes' list (cubesPolled): every panel follows it in place, and a Cube no longer given forgets its form. */
  polled(): void {
    mountList();
    mountPage();
    forgetGone();
  },
  /** openFound opens the fold of the Cubes found (showFound). */
  openFound(): void {
    folds.found = true;
  },
  /** pairingEvent takes the shown Cube's pairing_started and pairing_ended (app.js connectEvents): its pairing state is asked for. */
  pairingEvent,
  /** pairingJoined takes a paired event of the shown Cube (app.js connectEvents): the device is listed at once. */
  pairingJoined,
});

/**
 * signedOut forgets what the Cubes' parts kept of the user who was signed
 * in (MassimoParts.signedOut, index.ts): every Cube's pairing and rooms,
 * and the devices its panel read; the panels follow, drawn from nothing.
 */
export function signedOut(): void {
  forgetPairing();
  devices.value = {}; // the devices a panel read, the user's who was
}

/**
 * The Cubes card's head, which cubes.js drew at every poll before: how many are served; those not connected by their link's
 * state, as their signs show it, a cross red ("not connected"), a held
 * connection or a reboot amber; the proxies that do not listen, the Cubes
 * found at another address, those found and not served. An effect of the
 * Cubes' list and the language, which writes the card's badges alone
 * (shared/card.ts), its panels open or not.
 */
effect(() => {
  void lang.value; // the badges' words in the page's language
  const cs = state.cubes;
  const count = (link: string) => cs.filter((c) => linkState(c) === link).length;
  const lost = cs.filter((c) => linkSign(c) === "bad").length;
  const badges: BadgeSpec[] = [{ text: tn("cubes.badge.served", cs.length), kind: "mode" }];
  if (lost) badges.push({ text: tn("cubes.badge.disconnected", lost), kind: "bad" });
  if (count("held")) badges.push({ text: tn("cubes.badge.held", count("held")), kind: "warn" });
  if (count("rebooting")) badges.push({ text: tn("cubes.badge.rebooting", count("rebooting")), kind: "warn" });
  const failing = cs.reduce((n, c) => n + (c.proxy_failing || []).length, 0);
  if (failing) badges.push({ text: tn("proxy.badge.failing", failing), kind: "warn" });
  const elsewhere = cs.filter((c) => c.found_elsewhere).length;
  if (elsewhere) badges.push({ text: tn("cubes.badge.elsewhere", elsewhere), kind: "warn" });
  if (state.found.length) badges.push({ text: tn("cubes.badge.found", state.found.length), kind: "warn" });
  cardBadges("sys-cubes", badges);
});

/** MassimoParts.cubes, as index.ts declares it. */
export type CubesPart = typeof cubes;

// Cube settings draws its panel when it is shown, which loads what it
// shows; System settings draws its list, and the panels still open from
// before load what they would load when opened.
onView("cube", () => {
  msg("");
  cubes.render();
  // each showing loads what the panel shows, the page drawn again or not:
  // the place and the Cube may be what they were at the last showing
  const c = chosenCube();
  pageLoaded = c ? c.serial : "";
  if (c) panelLoads(c.serial);
});
onView("system", () => {
  msg("");
  cubes.render();
  for (const c of state.cubes) {
    if (!cubeUI.of(c.serial).open) continue;
    loadDevices(c.serial);
    loadPairing(c.serial);
    loadCubeRooms(c.serial);
  }
});
