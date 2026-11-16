// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/plain/tiles.d.ts
 *
 * The globals of the plain script ui/tiles.js that the parts call: the
 * name of a radiogram that failed, which the log names it by too.
 */

import type { FailedRadiogram } from "../api";

declare global {
  /** A failed radiogram's type by the original software's name and its code, "TimeInformation (0x03)". */
  function radiogramName(r: FailedRadiogram): string;
}

export {};
