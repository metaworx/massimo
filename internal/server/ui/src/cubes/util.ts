// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/cubes/util.ts
 *
 * Small helpers of the Cubes' parts: an address split into host and
 * port, and a value of a settings file's difference as text.
 */

/**
 * hostPort splits a Cube's address, host:port or [v6]:port, into the host
 * and the port. Its plain twin is cubes.js hostPort, which home.js calls:
 * the header's move to a part (AP UiParts, Block 6) takes this one and
 * deletes the plain one.
 */
export function hostPort(addr?: string): { host: string; port: string } {
  const a = addr || "";
  const m = /^\[([^\]]+)\]:(\d+)$/.exec(a) || /^([^:]+):(\d+)$/.exec(a);
  return m ? { host: m[1], port: m[2] } : { host: a, port: "" };
}

/** valueText shows a value of a file difference: a list joined, an empty one as "(empty)". */
export function valueText(v: unknown): string {
  const s = Array.isArray(v) ? v.join(", ") : String(v === null || v === undefined ? "" : v);
  return s === "" ? t("cubes.empty") : s;
}
