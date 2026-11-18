// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/cubes/Panel.tsx
 *
 * A Cube's panel, in System settings → Cubes and on Cube settings: its
 * head (its network sign, name, serial, host and radio budget, its
 * problem list, where its settings come from), its own badges as its top
 * line, the notices of its link, of an address found elsewhere and of
 * its memory slots, then its facts, its settings and its actions. Built
 * once per Cube and place and updated in place, so that an open problem
 * list, a focused button and what is typed stay when data comes in.
 */

import type { TargetedMouseEvent } from "preact";
import type { Cube, FoundCube } from "../api";
import { CubeIcon, CubeLink, CubeSign, cubeEntry, hostPort, problemsLabelOf } from "../model/cube";
import { Badge, Notice } from "../shared/badge";
import { Fold } from "../shared/fold";
import { Island } from "../shared/island";
import { Problems } from "../shared/problems";
import { useBusy, busyProps } from "../shared/question";
import { Actions } from "./Actions";
import { Facts } from "./Facts";
import { Form } from "./Form";
import { saveCube } from "./send";
import { cubeUI, load, panelLoads } from "./store";

/** proxyFailing counts a Cube's proxy listeners that do not listen. */
function proxyFailing(c: Cube): number {
  return (c.proxy_failing || []).length;
}

/**
 * TopLine is the open panel's first line, the Cube's own badges: its
 * link where it is not connected, a proxy that does not listen, a
 * settings file that differs, a fixed address it is found elsewhere
 * than, its memory slots held, where its settings come from.
 */
function TopLine({ c }: { c: Cube }) {
  const st = linkSign(c);
  const failing = proxyFailing(c);
  const differs = (c.file_differs || []).length;
  const full = c.slots_low?.level === "error";
  return (
    <div class="cube-top-line">
      {st !== "ok" && <Badge key="link" kind={st === "bad" ? "bad" : "warn"} text={linkWord(c)} />}
      {failing > 0 && <Badge key="proxy" kind="warn" text={tn("proxy.badge.failing", failing)} />}
      {differs > 0 && <Badge key="file" kind="warn" text={tn("cubes.differsCount", differs)} />}
      {c.found_elsewhere && <Badge key="elsewhere" kind="warn" text={t("cubes.elsewhereBadge")} />}
      {c.slots_low && <Badge key="slots" kind={full ? "bad" : "warn"} text={t(full ? "cubes.slotsBadge" : "cubes.slotsLowBadge")} />}
      {c.source && <Badge key="source" kind="mode" text={t("cubes.source." + c.source)} />}
    </div>
  );
}

/**
 * OtherNetwork is the notice that the Cube's address lies outside this
 * machine's networks, with the netconfig line that copies: plain
 * (home.js otherNetworkEls), filled into an element the part leaves to
 * it, again only where the language, the Cube or the address changed.
 */
export function OtherNetwork({ c, address }: { c: Cube | FoundCube; address: string }) {
  return <Island tag="p" cls="notice other-network" keys={[c.serial, address]} fill={(el) => el.replaceChildren(...otherNetworkEls(c, address))} />;
}

/** LinkState says above the facts why the Cube is not connected, where the server knows, and since when. */
function LinkState({ c }: { c: Cube }) {
  const text = linkText(c);
  const away = c.other_network && !c.connected ? hostPort(c.address).host : "";
  return (
    <div class="cube-link-state">
      {text && <Notice text={text} since={c.link?.since} />}
      {away && <OtherNetwork c={c} address={away} />}
    </div>
  );
}

/**
 * AddressState says that discovery finds a Cube with a fixed address at
 * another one, since when, and where the Cube has the form (not -addr or
 * -sn) offers to take the found address or to follow discovery.
 */
function AddressState({ c }: { c: Cube }) {
  const f = c.found_elsewhere;
  const use = useBusy();
  const follow = useBusy();
  const editable = !!f && !!c.source && c.source !== "flag";
  return (
    <div class="cube-address-state">
      {f && <Notice text={t("cubes.elsewhere", { found: f.address, configured: f.configured })} since={f.since} />}
      {f && editable && (
        <div class="cube-elsewhere-fix needs-cubes">
          <button type="button" {...busyProps("use-found", use.busy)} onClick={() => use.run(() => saveCube(c.serial, { address: f.address }))}>
            {t("cubes.useFound")}
          </button>
          <button type="button" {...busyProps("follow-found", follow.busy)} onClick={() => follow.run(() => saveCube(c.serial, { follow_discovery: true }))}>
            {t("cubes.followFound")}
          </button>
        </div>
      )}
    </div>
  );
}

/**
 * SlotsState says that few of the Cube's memory slots stayed free, or
 * none, since when, and that its Reboot below drops what they hold.
 */
function SlotsState({ c }: { c: Cube }) {
  const f = c.slots_low;
  const full = f?.level === "error";
  return (
    <div class="cube-slots-state">
      {f && <Notice text={t(full ? "cubes.slotsFull" : "cubes.slotsLow", { slots: f.free_slots })} since={full ? f.full_since : f.since} />}
    </div>
  );
}

/** keepPanel keeps a click in the head's problem list from folding the panel; one on the list's summary opens the list. */
function keepPanel(ev: TargetedMouseEvent<HTMLDetailsElement>) {
  if (!(ev.target as Element).closest("summary.problems-sum")) ev.preventDefault();
}

/**
 * Head is what the panel's summary shows: the cube, the Cube's name,
 * serial with its host and radio budget at the left, wrapping; its
 * problem list top right whatever wraps, led by the Cube's sign, which
 * stands for the green check where nothing else is wrong, its list the
 * link in words first (CubeLink); where its settings come from on a line
 * of its own.
 */
function Head({ c }: { c: Cube }) {
  load.useLoad(); // it reads the unsaved forms' signal too, and a part that reads signals is drawn again only for them or new properties
  const { host } = hostPort(c.address);
  const f = budgetFacts(c);
  const entries = [cubeEntry(c, true)];
  return (
    <>
      <span class="cube-head-who">
        <CubeIcon />
        {c.name && <span class="panel-name">{c.name}</span>}
        <span class="mono">{host ? c.serial + " · " + host : c.serial}</span>
        <span class="cube-head-stats muted small">
          {[t("budget.duty") + " " + f.duty, t("budget.slots") + " " + f.slots, t("budget.pending") + " " + f.pending].join(" · ")}
        </span>
      </span>
      <span class="cube-pill">
        <Problems entries={entries} label={[linkWord(c), problemsLabelOf(entries)].join(" · ")} always lead={<CubeSign c={c} />}
          head={<CubeLink c={c} />} onClick={keepPanel} />
      </span>
      {c.source && (
        <span class="panel-badges">
          <Badge kind="mode" text={t("cubes.source." + c.source)} />
        </span>
      )}
    </>
  );
}

/** keepOpen keeps a click on Cube settings' head from folding the panel; one on its problem list opens the list. */
function keepOpen(ev: TargetedMouseEvent<HTMLElement>) {
  if (!(ev.target as Element).closest(".cube-pill")) ev.preventDefault();
}

/**
 * Panel is one configured Cube, a Fold. In System settings' list it opens
 * and closes with a click on its head, which loads what it shows; on Cube
 * settings (page) it is open and stays open, its head out of the tab
 * order, and a click on its head folds nothing.
 */
export function Panel({ c, page }: { c: Cube; page: boolean }) {
  load.useLoad();
  const ui = cubeUI.of(c.serial);
  const toggle = (open: boolean) => {
    if (page) return;
    ui.open = open;
    if (open) panelLoads(c.serial);
  };
  const editable = !!c.source && c.source !== "flag";
  return (
    <Fold cls="panel cube-panel" data={{ serial: c.serial }} open={page || ui.open} onToggle={toggle} summary={<Head c={c} />}
      summaryProps={page ? { tabIndex: -1, onClick: keepOpen } : undefined}>
      <TopLine c={c} />
      <LinkState c={c} />
      <AddressState c={c} />
      <SlotsState c={c} />
      <div class="cube-cols">
        <Facts c={c} />
        {editable ? <Form c={c} /> : <p class="muted small needs-cubes">{t("cubes.flagNote")}</p>}
        <Actions c={c} editable={editable} />
      </div>
    </Fold>
  );
}
