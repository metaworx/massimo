// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/model/found.tsx
 *
 * What the parts say of a Cube or a Massimo proxy found on the network and
 * not served, for every region that lists them (the Cubes' found fold, the
 * header's problem list): what it is and who serves it, as parts of a line
 * with the server's name a shared Link. Like all of ui/src/model/, it knows
 * the API's data and no region, and holds no state.
 */

import type { ComponentChildren } from "preact";
import type { FoundCube } from "../api";
import { tParts } from "../shared/lang";
import { Link } from "../shared/link";

/**
 * foundWhat is what a found entry is and who serves it: a Massimo proxy
 * and the Cube it serves, and the Massimo server serving it, the proxy's
 * own or the one a Cube's proxy runs on, its name a link to the address
 * its announcement on the network gives.
 */
export function foundWhat(c: FoundCube): ComponentChildren[] {
  const out: ComponentChildren[] = [];
  if (c.kind === "proxy") out.push(c.serves ? t("found.proxyServes", { serial: c.serves }) : t("found.proxy"));
  if (c.host) out.push(tParts("found.servedBy", {}, { host: <Link url={c.host.url} text={c.host.name || c.host.url} cls="found-host" /> }));
  return out;
}
