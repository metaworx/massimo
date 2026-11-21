// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/log/address.ts
 *
 * The log's address of its own (AP UiParts, Block 11; line for line from
 * pages.js): its page, then All Cubes, the range, the kind, the source,
 * the device, the level and who, which the pages' store reads at a
 * reload, a link, back and forward, and writes as the log is shown
 * (index.tsx hands it over, ownAddress).
 */

import { LOG_COUNT_RANGE, LOG_KINDS, LOG_RANGES, LOG_SOURCES } from "./kinds";
import { logState } from "./state";

/**
 * logHash is the log's address: its page, then All Cubes, the range, the
 * kind and the device chosen (by its S/N), in that order, each left out
 * at its default, so that a plain #log is the header's Cube's last 20
 * events of every kind and device.
 */
export function logHash(): string {
  const q = new URLSearchParams();
  if (logState.cubes) q.set("cubes", logState.cubes);
  if (logState.range !== LOG_COUNT_RANGE) q.set("range", logState.range);
  if (logState.kind) q.set("kind", logState.kind);
  if (logState.source) q.set("source", logState.source);
  if (logState.device) q.set("device", logState.device);
  if (logState.level !== "info") q.set("level", logState.level);
  if (logState.user) q.set("user", logState.user);
  const s = q.toString();
  return "#log" + (s ? "?" + s : "");
}

/**
 * readLogHash takes the log's Cubes, range, kind and device from an
 * address of the log, each at its default where it is missing or not one
 * of its values; the device by its S/N, or an old link's radio address,
 * which takes the S/N, and a device the Cube does not have goes, once its
 * devices are loaded (settleChoices). The text filter is not in the
 * address.
 */
export function readLogHash(hash: string): void {
  const i = hash.indexOf("?");
  const q = new URLSearchParams(i < 0 ? "" : hash.slice(i + 1));
  const range = q.get("range") || "";
  const kind = q.get("kind") || "";
  const device = (q.get("device") || "").trim();
  logState.cubes = q.get("cubes") === "all" ? "all" : "";
  logState.range = LOG_RANGES.includes(range) ? range : LOG_COUNT_RANGE;
  logState.kind = LOG_KINDS.includes(kind) ? kind : "";
  if (/^[a-z]{3}[0-9]{7}$/i.test(device)) logState.device = device.toUpperCase();
  else logState.device = /^[0-9a-f]{6}$/i.test(device) ? device.toLowerCase() : "";
  const source = q.get("source") || "";
  logState.source = LOG_SOURCES.includes(source) && !logState.device ? source : "";
  const level = q.get("level") || "";
  logState.level = ["debug", "info", "warn"].includes(level) ? level : "info";
  const user = (q.get("user") || "").trim().toLowerCase();
  logState.user = /^[a-z0-9._-]{1,64}$/.test(user) ? user : "";
}
