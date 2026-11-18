// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/header/send.ts
 *
 * The header's request: the search for Cubes on the LAN from the Cube
 * choice. A poll now is the pile's Refresh now (Pile.tsx).
 */

import type { Discovery } from "../api";
import { discovery } from "./store";

/**
 * discoverCubes asks the server to search the LAN for Cubes (about two
 * seconds), the Cube choice's entry saying so meanwhile, then reloads the
 * list and says what it found in the page's toast (showToast of
 * components.js): Cubes found with Show, which opens them in System
 * settings for a role with cubes, as their entries in the list do; a
 * failure in red, for longer.
 */
export async function discoverCubes(): Promise<void> {
  if (discovery.running) return;
  discovery.running = true;
  try {
    const res = await api<Discovery | null>("POST", "api/v1/cubes/discover");
    await loadCubes();
    const n = ((res && res.found) || []).filter((c) => !isConfigured(c)).length;
    const show = n && hasCap("cubes") ? { label: t("discover.show"), run: showFound } : null;
    showToast(n ? tn("discover.found", n) : t("discover.none"), { ms: 8000, action: show });
  } catch (e) {
    showToast(t("discover.failed", { error: (e as Error).message }), { ms: 15000, tone: "bad" });
  } finally {
    discovery.running = false;
  }
}
