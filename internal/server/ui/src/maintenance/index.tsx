// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/maintenance/index.tsx
 *
 * Maintenance → Maintenance as a part (AP UiParts, Block 12), drawn once
 * into its card's body the first time Maintenance is shown and updated in
 * place; read each time Maintenance is shown, after each of its changes
 * and for the stream's events while it is shown (serverEvent,
 * MassimoParts.serverEvent), and followed through a restart and a
 * reboot.
 */

import { mounter } from "../shared/region";
import { Maintenance } from "./Maintenance";
import { forgetMaintenance, loadReboot, loadRestart, loadSettings, quiet } from "./store";
import { onView } from "../shared/pages";

export { serverEvent } from "./store";

const mount = mounter("sys-maint-body", () => <Maintenance />);

/** signedOut forgets what the card kept of the user who was signed in (MassimoParts.signedOut, index.ts). */
export const signedOut = forgetMaintenance;

// Maintenance shown: the settings' messages taken back, then the announcement and the intervals, the restart's row and the reboot's read
onView("maint", () => {
  mount();
  quiet();
  loadSettings();
  loadRestart();
  loadReboot();
});
