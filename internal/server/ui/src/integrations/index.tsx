// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/integrations/index.tsx
 *
 * System settings → Integrations as a part (AP UiParts, Block 10), drawn
 * once into its card's body and updated in place, read each time System
 * settings is shown and after each of its changes; no plain script calls
 * it. The readings of an approved integration again stop at a sign-out
 * with the rest it kept.
 */

import { mounter } from "../shared/region";
import { IntegrationsCard } from "./Integrations";
import { session } from "../shared/session";
import { forgetIntegrations, loadRequestsSwitch, msgIn, refresh, said } from "./store";
import { onView } from "../shared/pages";

const mount = mounter("sys-integrations-body", () => <IntegrationsCard />);

/** signedOut forgets what the section kept of the user who was signed in (MassimoParts.signedOut, index.ts). */
export const signedOut = forgetIntegrations;

// System settings shown: everything the section shows read and drawn, and whether integrations may ask
onView("system", () => {
  said.value = null;
  if (!hasCap("integrations")) return;
  mount();
  const n = session();
  refresh()
    .then(loadRequestsSwitch)
    .catch((e: Error) => msgIn(n, t("integrations.failed", { name: "", error: e.message }), false));
});
