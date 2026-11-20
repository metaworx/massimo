// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/shared/cls.ts
 *
 * A class added to an element by its state, for the parts that keep an
 * element in its place and hide it rather than leave it out.
 */

/** hid is the class hidden, with its space, where off; "" otherwise. */
export function hid(off: boolean): string {
  return off ? " hidden" : "";
}
