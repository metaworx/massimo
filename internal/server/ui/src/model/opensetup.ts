// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/model/opensetup.ts
 *
 * What was set up before the first owner and still works (AP
 * AccessEnds, Block 3): the open_setup of the first owner's answer,
 * which the sign-in card's toast says, and of the log's open_setup line,
 * whose summary holds the same lists, worded alike in the page's
 * language: the integrations by their names, the sign-in providers, one
 * whose sign-in creates users said so, the roles added or changed by
 * their names, the key where one has none, and the capabilities turned
 * off.
 */

import type { OpenSetup } from "../api";
import { capLabel } from "./cap";

/** openSetupText is what was set up before the first owner, a part for each list it holds: "1 integration: Kitchen; roles added or changed: Viewer". */
export function openSetupText(o: OpenSetup): string {
  const parts: string[] = [];
  const integrations = o.integrations || [];
  if (integrations.length) parts.push(tn("openSetup.integrations", integrations.length, { names: integrations.map((i) => i.name).join(", ") }));
  const providers = o.providers || [];
  if (providers.length) {
    const names = providers.map((p) => (p.auto_create ? t("openSetup.createsUsers", { name: p.name || p.key }) : p.name || p.key));
    parts.push(t("openSetup.providers", { names: names.join(", ") }));
  }
  const roles = o.roles || [];
  if (roles.length) parts.push(t("openSetup.roles", { names: roles.map((r) => roleLabel(r.key, r.name)).join(", ") }));
  const off = o.capabilities_off || [];
  if (off.length) parts.push(t("openSetup.capabilities", { names: off.map(capLabel).join(", ") }));
  return parts.join("; ");
}
