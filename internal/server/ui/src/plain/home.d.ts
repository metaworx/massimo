// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/plain/home.d.ts
 *
 * The globals of the plain script ui/home.js that the parts call: the
 * notice of a Cube outside this machine's networks, which a part puts in
 * an element it leaves to it.
 */

import type { Cube, FoundCube } from "../api";

declare global {
  /** The children of the notice that a Cube's address lies outside this machine's networks, with the netconfig line. */
  function otherNetworkEls(c: Cube | FoundCube, address: string): (string | Node)[];
}

export {};
