// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/cubes/util.ts
 *
 * A small helper of the Cubes' parts: a value of a settings file's
 * difference as text.
 */

/** valueText shows a value of a file difference: a list joined, an empty one as "(empty)". */
export function valueText(v: unknown): string {
  const s = Array.isArray(v) ? v.join(", ") : String(v === null || v === undefined ? "" : v);
  return s === "" ? t("cubes.empty") : s;
}
