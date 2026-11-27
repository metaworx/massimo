// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/serverbackups/index.tsx
 *
 * Maintenance → Server backups as a part (AP ServerBackup, Block 4),
 * for a holder of backup.retry, which backup.server includes: drawn once
 * into its card's body the first time Maintenance is shown to one, and
 * updated in place; read each time Maintenance is shown (the list; the
 * targets, the keys and, once, the kinds for a holder of backup.server),
 * after each of its actions and for the stream's events while it is
 * shown (serverEvent, MassimoParts.serverEvent).
 */

import { mounter } from "../shared/region";
import { can } from "../shared/me";
import { onView } from "../shared/pages";
import { Card } from "./Card";
import { loadShown } from "./send";
import { forgetServerBackups, quiet } from "./store";

export { serverEvent } from "./send";

const mount = mounter("sys-server-backups-body", () => <Card />);

/** signedOut forgets what the card kept of the user who was signed in, a key made's secret part first (MassimoParts.signedOut, index.ts). */
export const signedOut = forgetServerBackups;

// Maintenance shown: the card's messages taken back, then what it shows read, for whoever may see it
onView("maint", () => {
  quiet();
  if (!can("backup.retry")) return;
  mount();
  void loadShown();
});
