// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/software/index.tsx
 *
 * Maintenance → Software update as a part (AP UiParts, Block 12), drawn
 * once into its card's body as parts.js loads, since the header's check
 * for an update draws the card's badge before Maintenance is shown, and
 * updated in place. It is read each time Maintenance is shown, by Check
 * for updates, by the header's check every half hour, every 3 s while
 * the helper works, and for the stream's events while Maintenance is
 * shown (serverEvent, MassimoParts.serverEvent). The plain code calls MassimoParts.software
 * (update.js loadUpdate, updateOut and updateLatest, for the header).
 */

import type { UpdateInfo } from "../api";
import { mounter, register } from "../shared/region";
import { Software } from "./Software";
import { forgetSoftware, loadChannel, loadUpdate, updateLatest, updateOut } from "./store";
import { onView } from "../shared/pages";

export { serverEvent } from "./store";

const mount = mounter("sys-software-body", () => <Software />);
mount();

/** The functions the plain code calls, MassimoParts.software. */
const software = register("software", {
  /** load reads GET /api/v1/update and draws it; refresh asks for a release now (update.js loadUpdate, the header's check). */
  load(refresh: boolean): Promise<UpdateInfo | null> {
    return loadUpdate(refresh);
  },
  /** out tells whether an update is out, for one who may install it (update.js updateOut). */
  out: updateOut,
  /** latest is the version an update would install, "" for none (update.js updateLatest). */
  latest: updateLatest,
});

/** MassimoParts.software, as index.ts declares it. */
export type SoftwarePart = typeof software;

/** signedOut forgets what the section kept of the user who was signed in (MassimoParts.signedOut, index.ts). */
export const signedOut = forgetSoftware;

// Maintenance shown: the versions read, then the channel's setting
onView("maint", () => {
  loadUpdate(false).then(() => loadChannel());
});
