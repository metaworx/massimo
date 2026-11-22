// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/header/HomeNotice.tsx
 *
 * The home notice on Rooms, in #home-notice: why Rooms shows nothing, or
 * only what the server last had: no Cube configured, or the Cube chosen
 * not connected, with the address the server tries, when the Cube last
 * sent data or else last answered discovery, what to check, and for a
 * role with cubes the way to System settings → Cubes. The rooms the
 * server still has stay under it. Its element is the page's, shown and
 * hidden by home.js renderHomeNotice and pages.js; the part draws inside.
 */

import type { Cube } from "../api";
import { chosenCube, hostPort } from "../model/cube";
import { Island } from "../shared/island";
import { load } from "./store";
import { showView } from "../shared/pages";
import { can } from "../shared/me";

/** detailOf is what the notice says of a Cube that is not connected, in one paragraph. */
function detailOf(cube: Cube): string {
  const parts = [cube.address ? t("home.address", { address: cube.address }) : t("home.noAddress")];
  const why = linkText(cube); // a held connection, a refusal: first
  if (why) parts.unshift(why);
  if (cube.name) parts.push(t("home.serial", { serial: cube.serial }));
  const data = timeOf(cube.updated);
  const seen = timeOf(cube.last_seen);
  if (data) parts.push(t("home.lastData", { time: fmtInstant(data) }));
  else if (seen) parts.push(t("home.lastSeen", { time: fmtInstant(seen) }));
  else parts.push(t("home.noData"));
  return parts.join(" ");
}

/**
 * Check is what to check: the usual checks; for a Cube outside this
 * machine's networks why and the way to an address here, with the
 * netconfig line that copies, plain (home.js otherNetworkEls), filled
 * again only where the Cube or its address changed, so that a poll leaves
 * a focused copy icon where it is; nothing without a Cube.
 */
function Check({ cube }: { cube?: Cube }) {
  if (!cube) return <p class="home-notice-check small hidden" />;
  if (!cube.other_network) return <p class="home-notice-check small">{t("home.check")}</p>;
  const host = hostPort(cube.address).host;
  return <Island tag="p" cls="home-notice-check small" keys={[cube.serial, host]} fill={(el) => el.replaceChildren(...otherNetworkEls(cube, host))} />;
}

/** HomeNotice is the notice's content, nothing while the Cube chosen is connected. */
export function HomeNotice() {
  load.useLoad();
  const cube = chosenCube();
  if (cube && cube.connected) return null;
  return (
    <>
      <p class="home-notice-title">{cube ? t("home.notConnected", { name: cube.name || cube.serial }) : t("home.noCube")}</p>
      <p class="home-notice-detail small">{cube ? detailOf(cube) : t(can("cubes") ? "home.noCubeAdd" : "home.noCubeAsk")}</p>
      <Check cube={cube} />
      <button type="button" id="home-notice-cubes" class="needs-cubes" onClick={() => showView("system")}>{t("home.toCubes")}</button>
    </>
  );
}
