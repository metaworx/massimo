// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/sysinfo/index.tsx
 *
 * System info as a part (AP UiParts, Block 12), drawn once into its card
 * the first time the page is shown and updated in place; read each time
 * it is shown, again while a proxy's certificate is being read, and for
 * the stream's events while it is shown (serverEvent,
 * MassimoParts.serverEvent).
 */

import { mounter } from "../shared/region";
import { SysInfo } from "./SysInfo";
import { forgetSysinfo, shown } from "./store";
import { onView } from "../shared/pages";

export { serverEvent } from "./store";

const mount = mounter("sys-info-body", () => <SysInfo />);

/** signedOut forgets what the page kept of the user who was signed in (MassimoParts.signedOut, index.ts). */
export const signedOut = forgetSysinfo;

// System info shown: the store and the settings in effect, then the facts, read
onView("info", () => {
  mount();
  shown();
});
