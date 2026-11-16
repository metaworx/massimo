// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/cubes/Found.tsx
 *
 * What was found on the network and is not served, one fold under the
 * served Cubes, for a role with cubes: its sign with how many, a row per
 * Cube and Massimo proxy, a Cube with Add, and how to serve them, Add or
 * the lines for the settings file in a fold of their own.
 */

import type { FoundCube } from "../api";
import { foundWhat } from "../model/found";
import { Age, Badge } from "../shared/badge";
import { CopyIcon } from "../shared/copy";
import { Fold, FoldHead } from "../shared/fold";
import { CountIcon, Icon } from "../shared/icon";
import { joinParts } from "../shared/lang";
import { busyProps, useBusy } from "../shared/question";
import { addCube } from "./send";
import { OtherNetwork } from "./Panel";
import { folds, load } from "./store";

/**
 * Row is a Cube or a Massimo proxy found: its sign, its serial, for a Cube
 * its address, DHCP flag, firmware and the proxies that answer for it, for
 * a proxy what it serves, for both the server serving it, in expert mode
 * the radio and MAC address and the gateway, how long ago it answered,
 * and for a Cube Add, which serves it at once; under a Cube outside this
 * machine's networks, why and the way to an address here.
 */
function Row({ c }: { c: FoundCube }) {
  const add = useBusy();
  const proxy = c.kind === "proxy";
  const dhcp = c.dhcp === true ? t("cubes.dhcp") : c.dhcp === false ? t("cubes.static") : "";
  const proxies = (c.proxies || []).join(", ");
  const own = proxy ? [] : [dhcp, c.firmware ? t("budget.firmware") + " " + c.firmware : "", proxies ? t("cubes.viaProxy", { address: proxies }) : ""];
  const facts = joinParts([c.address, ...own, ...foundWhat(c)], true);
  const extra = joinParts([c.rf_address, c.mac || "", c.gateway ? t("cubes.gateway") + " " + c.gateway : ""], true);
  return (
    <li class="cube-found-row" data-serial={c.serial} data-kind={c.kind || "unknown"}>
      <Icon symbol={proxy ? "icon-proxy" : "icon-cube3d"} cls="ni found-icon" />
      <span class="mono found-serial">{c.serial}</span>
      {facts.length > 0 && <span class="found-facts">{facts}</span>}
      {extra.length > 0 && <span class="mono expert-only found-extra">{extra}</span>}
      <Age at={c.last_seen} plain />
      {!proxy && (
        <button type="button" {...busyProps("add-cube needs-cubes", add.busy)} aria-label={t("cubes.addLabel", { serial: c.serial })}
          onClick={() => add.run(() => addCube(c.serial))}>
          {t("cubes.add")}
        </button>
      )}
      {!proxy && c.other_network && <OtherNetwork c={c} address={c.address} />}
    </li>
  );
}

/**
 * Hints say how the found Cubes are served: Add at once, or the lines for
 * the server's settings file and a restart, in a fold of their own with
 * one copy icon for them all, and where that file is. A Massimo proxy has
 * no lines: it is another server's.
 */
function Hints({ found }: { found: FoundCube[] }) {
  const lines: string[] = [];
  for (const c of found) {
    if (c.kind === "proxy") continue;
    const p = "MASSIMO_CUBE_" + c.serial + "_";
    lines.push(p + "ADDRESS=" + (c.address || ""), p + "NAME=");
  }
  const text = lines.join("\n");
  return (
    <div class="cubes-found-hints">
      <p class="muted small">{t("discover.hint")}</p>
      {lines.length > 0 && (
        <Fold cls="sys-sub found-lines" open={folds.lines} onToggle={(open) => { folds.lines = open; }} summary={
          <>
            <FoldHead name={t("found.lines")} />
            <CopyIcon get={() => text} label={t("found.copyLines")} />
          </>
        }>
          <pre class="env-lines mono">{text}</pre>
          <p class="muted small">{t("discover.hintFile")}</p>
        </Fold>
      )}
    </div>
  );
}

/**
 * Found is the fold of what was found and is not served, closed at first
 * and kept as it was (folds.found): the found Cubes' sign, its name and how
 * many Cubes and Massimo proxies, then a row each and how to serve them.
 */
export function Found({ found }: { found: FoundCube[] }) {
  load.useLoad();
  const proxies = found.filter((c) => c.kind === "proxy").length;
  const cubes = found.length - proxies;
  return (
    <Fold cls="panel found-panel needs-cubes" open={folds.found} onToggle={(open) => { folds.found = open; }} summary={
      <FoldHead before={<CountIcon symbol="icon-cube3d" n={found.length} cls="found-count" />} name={t("found.title")} badges={[
        cubes > 0 && <Badge key="cubes" kind="mode" text={tn("found.cubes", cubes)} />,
        proxies > 0 && <Badge key="proxies" kind="mode" text={tn("found.proxies", proxies)} />,
      ]} />
    }>
      <ul class="cube-found-list">
        {found.map((c) => <Row key={c.serial} c={c} />)}
      </ul>
      <Hints found={found} />
    </Fold>
  );
}
