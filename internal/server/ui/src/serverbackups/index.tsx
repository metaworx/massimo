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
 * shown (serverEvent, MassimoParts.serverEvent). A key made's secret
 * part goes as another page is shown or the card is folded; the restore's
 * dialog, with the key typed into it, as another page is shown and at a
 * sign-out.
 */

import { onFold } from "../shared/card";
import { mounter } from "../shared/region";
import { can } from "../shared/me";
import { onView } from "../shared/pages";
import { Card } from "./Card";
import { forgetRestore, pageShown, quietRestore } from "./restoring";
import { loadShown } from "./send";
import { dropMade, forgetServerBackups, quiet } from "./store";

export { serverEvent } from "./send";

const mount = mounter("sys-server-backups-body", () => <Card />);

/** signedOut forgets what the card kept of the user who was signed in, the key typed into the restore's dialog and a key made's secret part first (MassimoParts.signedOut, index.ts). */
export function signedOut(): void {
  forgetRestore();
  forgetServerBackups();
}

// Maintenance shown: the card's messages taken back, then what it shows read, for whoever may see it
onView("maint", () => {
  quiet();
  quietRestore();
  if (!can("backup.retry")) return;
  mount();
  void loadShown();
});

// a key made's secret part is shown that once (D4-A): another page shown, or the card folded, takes it away; another page closes the restore's dialog, its key with it, or takes it away while its restore runs
document.addEventListener("viewshown", (ev) => {
  const maint = (ev as CustomEvent<string>).detail === "maint";
  pageShown(maint);
  if (!maint) dropMade();
});
onFold("sys-server-backups", dropMade);
