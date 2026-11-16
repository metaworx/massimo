// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/shared/region.ts
 *
 * How a region becomes a part of the page: its functions registered on
 * window.MassimoParts under the region's name, which the plain code calls
 * (the plain render… names stay, as calls of these), and its root drawn
 * once into the container the page has for it, then updated in place.
 */

import { render, type VNode } from "preact";

/** parts are the functions the plain code calls, by region; index.ts makes them window.MassimoParts. */
export const parts: Record<string, unknown> = {};

/**
 * register gives the plain code a region's functions, as
 * MassimoParts.<name>; a name registered twice is an error, so that two
 * regions cannot take each other's.
 */
export function register<T>(name: string, fns: T): T {
  if (name in parts) throw new Error("MassimoParts." + name + " is registered twice");
  parts[name] = fns;
  return fns;
}

/**
 * mounter draws a region's root into the element of the page with the
 * id the first time it is called, and does nothing after: from then on
 * the root follows its signals. It reports whether the root is there;
 * a page without the element (connect.html) draws nothing.
 */
export function mounter(id: string, root: () => VNode): () => boolean {
  let done = false;
  return () => {
    if (done) return true;
    const box = document.getElementById(id);
    if (!box) return false;
    render(root(), box);
    done = true;
    return true;
  };
}
