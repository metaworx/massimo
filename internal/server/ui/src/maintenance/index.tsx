// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/maintenance/index.tsx
 *
 * Maintenance → Maintenance as a part (AP UiParts, Block 12), and
 * Maintenance → Limits beside it (AP ServerLimits, Block 1), each drawn
 * once into its card's body the first time Maintenance is shown and
 * updated in place; read each time Maintenance is shown, in one read of
 * the settings, after each of their changes and for the stream's events
 * while it is shown (serverEvent, MassimoParts.serverEvent), and followed
 * through a restart and a reboot. A window limit's change reads the rooms
 * again on every page (serverEvent).
 */

import { mounter } from "../shared/region";
import { Limits } from "./Limits";
import { Maintenance } from "./Maintenance";
import { forgetMaintenance, loadReboot, loadRestart, loadSettings, quiet } from "./store";
import { onView } from "../shared/pages";

export { serverEvent } from "./store";

const mount = mounter("sys-maint-body", () => <Maintenance />);
const mountLimits = mounter("sys-limits-body", () => <Limits />);

/** signedOut forgets what the card kept of the user who was signed in (MassimoParts.signedOut, index.ts). */
export const signedOut = forgetMaintenance;

// Maintenance shown: the settings' messages taken back, then the announcement, the intervals and the limits, the restart's row and the reboot's read
onView("maint", () => {
  mount();
  mountLimits();
  quiet();
  loadSettings();
  loadRestart();
  loadReboot();
});
