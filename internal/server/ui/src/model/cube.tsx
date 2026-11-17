// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/model/cube.tsx
 *
 * A Cube as the parts read it beside its fields, for every region that
 * shows Cubes (the Cubes and the header): the Cube chosen in the header,
 * its address split into host and port, its network sign, its problems
 * as an entry of the shared problem list, with the label that names their
 * kinds, and the key its settings form is kept by (shared/forms.ts),
 * whose unsaved changes the entry lists. Like all of ui/src/model/
 * (docs/ui.md, "How the page is built"), it knows the API's data and no
 * region, holds no state and no part with
 * hooks of its own, and may put shared parts together into a small piece
 * of that data, as the sign is.
 */

import type { Cube } from "../api";
import { unsavedItems } from "../shared/forms";
import { Sign } from "../shared/icon";
import { entryItems, kindsLabel, type Entry, type Item } from "../shared/problems";
import { deviceEntry, deviceProblems } from "./device";

/** chosenCube is the Cube chosen in the header, undefined before there is one. */
export function chosenCube(): Cube | undefined {
  return state.cubes.find((c) => c.serial === state.serial);
}

/**
 * CubeSign is a Cube's network sign in its link's state, named for a
 * screen reader by its link, its mark blinking as the Cube's other signs
 * do (cubeSignEl of header.js, for the plain regions).
 */
export function CubeSign({ c }: { c: Cube }) {
  return <Sign kind="net" state={linkSign(c)} cls="conn-sign" blink={"cube:" + c.serial} label={linkWord(c)} />;
}

/**
 * hostPort splits a Cube's address, host:port or [v6]:port, into the host
 * and the port; an address without a port is its host.
 */
export function hostPort(addr?: string): { host: string; port: string } {
  const a = addr || "";
  const m = /^\[([^\]]+)\]:(\d+)$/.exec(a) || /^([^:]+):(\d+)$/.exec(a);
  return m ? { host: m[1], port: m[2] } : { host: a, port: "" };
}

/** cubeFormKey is the key a Cube's settings form is kept by (useForm), by its serial. */
export function cubeFormKey(serial: string): string {
  return "cube:" + serial;
}

/**
 * cubeSettingItems are a Cube's settings that need a look, as items: a
 * proxy listener that does not listen, a settings file that differs, a
 * fixed address the Cube is found elsewhere than (found_elsewhere); who
 * names the Cube in the text, "" for its own panel.
 */
export function cubeSettingItems(c: Cube, who: string): Item[] {
  const items: Item[] = [];
  const text = (key: string, args: Record<string, string>) => (who ? t("problem." + key, { cube: who, ...args }) : t("problem." + key + "Own", args));
  for (const f of c.proxy_failing || []) items.push({ kind: "cog", icon: "icon-cog", sign: "cog", tone: "warn", text: text("proxy", { addr: f.addr }) });
  if ((c.file_differs || []).length) items.push({ kind: "cog", icon: "icon-cog", sign: "cog", tone: "warn", text: text("file", {}) });
  if (c.found_elsewhere) items.push({ kind: "cog", icon: "icon-cog", sign: "cog", tone: "warn", text: text("address", { addr: c.found_elsewhere.address }) });
  return items;
}

/**
 * cubeEntry is a Cube's problems as an entry of the problem list: the
 * Cube as the heading with its network sign (green where only its devices
 * fail), its link's badge where it is not connected, with own its
 * settings' warnings too (its panel), its settings form's unsaved
 * changes, the disk that opens the form, and its devices with a problem
 * under it, "Room › Device", with their badges; a window open briefly is
 * a state, not a problem.
 */
export function cubeEntry(c: Cube, own?: boolean): Entry {
  const st = linkSign(c);
  const items: Item[] = st === "ok" ? [] : [{ kind: "net", icon: "icon-net", sign: "net", tone: st === "bad" ? "bad" : "warn", text: linkWord(c) }];
  if (own) items.push(...cubeSettingItems(c, ""));
  items.push(...unsavedItems(cubeFormKey(c.serial)));
  const subs = (c.problems || []).map((d) =>
    deviceEntry(d, deviceProblems(d, true).filter((it) => it.tone !== "fill-open"), [d.room, deviceName(d)].filter(Boolean).join(" › ")));
  return { sign: { kind: "net", state: st }, title: c.name || c.serial, sub: c.name ? c.serial : "", items, subs };
}

/** The kinds of problems a Cube's label names, in the order it names them. */
const KINDS = ["net", "radio", "battery", "window", "cog", "unsaved"];

/**
 * problemsLabelOf names the kinds of problems among entries, for a screen
 * reader and the tooltip (kindsLabel), "nothing wrong" where there is
 * none, and the Cubes found where an entry is the found ones' (info).
 */
export function problemsLabelOf(entries: Entry[]): string {
  const head = kindsLabel(entries.flatMap(entryItems), KINDS) || t("problem.none");
  const found = entries.some((e) => e.info) && state.found.length ? tn("found.pill", state.found.length) : "";
  return [head, found].filter(Boolean).join(" · ");
}
