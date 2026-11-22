// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/header/CubeProblems.tsx
 *
 * Every Cube's problems as the entries of the header's pile (Pile.tsx):
 * a Cube with a problem as the heading, its devices with a problem under
 * it; for one who may change the settings, the settings; last, what was
 * found on the network and is not served.
 */

import type { FoundCube } from "../api";
import { cubeEntry, cubeSettingItems } from "../model/cube";
import { foundWhat } from "../model/found";
import { CountIcon } from "../shared/icon";
import { joinParts } from "../shared/lang";
import type { Entry, Item } from "../shared/problems";
import { can } from "../shared/me";

/**
 * foundEntry is what was found on the network and is not served, as the
 * list's last group: the found Cubes' sign with their number, shown in
 * the summary too, "not set up" and, for a role with cubes, a link to
 * System settings → Cubes (showFound); under it each Cube and Massimo
 * proxy, its address, what it is and the server serving it.
 */
function foundEntry(found: FoundCube[]): Entry {
  const sub = [t("found.notSetUp")];
  const go = can("cubes") && (
    <button type="button" class="link-button found-to-cubes" onClick={showFound}>{t("found.toCubes")}</button>
  );
  return {
    node: <CountIcon symbol="icon-cube3d" n={found.length} cls="found-count problem-icon" />,
    title: t("found.group"),
    sub: go ? [...sub, " · ", go] : sub,
    subs: found.map((c) => ({
      icon: c.kind === "proxy" ? "icon-proxy" : "icon-cube3d",
      title: c.serial,
      sub: joinParts([c.address, ...foundWhat(c)]),
    })),
    info: true,
    summary: <CountIcon symbol="icon-cube3d" n={found.length} cls="found-count" />,
  };
}

/**
 * cubeProblems are every Cube's problems as the list's entries (cubeEntry,
 * its link's badge where it is not connected, its devices under it);
 * then, for one who may change the settings, the settings of every Cube
 * that need a look; last what was found and is not served.
 */
export function cubeProblems(): Entry[] {
  const out = state.cubes.map((c) => cubeEntry(c));
  if (can("settings")) {
    const items: Item[] = [];
    for (const c of state.cubes) items.push(...cubeSettingItems(c, c.name || c.serial));
    out.push({ sign: { kind: "cog", state: items.length ? "warn" : "" }, title: t("problem.settings"), items });
  }
  if (state.found.length) out.push(foundEntry(state.found));
  return out;
}
