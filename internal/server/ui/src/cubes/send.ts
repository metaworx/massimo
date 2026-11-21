// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/cubes/send.ts
 *
 * What the Cubes' parts send: a change of a Cube's settings, a Cube
 * added, rebooted or removed. Each runs through the Cubes' action runner
 * (shared/action.ts): a reload of the page's data afterwards, so that the
 * header, the tiles and the Cubes follow, and the outcome under the list
 * or under Cube settings' panel.
 */

import { actionRunner } from "../shared/action";
import { cubePath } from "../shared/path";
import { load } from "./store";
import { isShown } from "../shared/pages";

/** The Cubes' runner: its message under the list, or under Cube settings' panel while that page is shown. */
const runner = actionRunner({
  bound: true, // an answer after a sign-out reloads nothing and says nothing
  where: () => $(isShown("cube") ? "cube-page-msg" : "cubes-msg"),
  reload: async () => {
    await refresh();
    load.bump();
  },
});

/** msg shows text in the Cubes' message line; no text hides it. */
export const msg = runner.say;

/** run sends a Cube's request with its message on success; the error names the Cube. */
function run(serial: string, fn: () => Promise<unknown>, ok: string): Promise<boolean> {
  return runner.run(fn, ok, (error) => t("cubes.failed", { serial, error }));
}

/** saveCube sends a change of a Cube's settings, the fields that differ (PUT). */
export function saveCube(serial: string, body: Record<string, unknown>): Promise<boolean> {
  return run(serial, () => api("PUT", cubePath(serial), body), t("cubes.saved", { serial }));
}

/**
 * addCube serves a found Cube (POST): as new, or with restore the
 * removed row of its serial that uuid names, with its history.
 */
export function addCube(serial: string, restore?: string): Promise<boolean> {
  const body = restore ? { serial, restore } : { serial };
  return run(serial, () => api("POST", "api/v1/cubes", body), t(restore ? "cubes.restored" : "cubes.added", { serial }));
}

/** rebootCube restarts a Cube, with the radiograms pending its question named (POST …/reboot). */
export function rebootCube(serial: string, pending: number): Promise<boolean> {
  return run(serial, () => api("POST", cubePath(serial) + "/reboot", { pending }), t("cubes.rebooted", { serial }));
}

/** removeCube stops serving a Cube (DELETE). */
export function removeCube(serial: string): Promise<boolean> {
  return run(serial, () => api("DELETE", cubePath(serial)), t("cubes.removed", { serial }));
}
