// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/header/index.tsx
 *
 * The header as parts (AP UiParts, Block 6): the Cube choice and every
 * Cube's problems, the chosen Cube's age, the server's sign, the line
 * about the certificate, and the home notice on Rooms, each drawn once
 * into its place in the page and updated in place. The plain code calls
 * MassimoParts.header (header.js renderCubeSelect, renderUpdated,
 * renderServerSign and renderTLS, home.js renderHomeNotice). The facts
 * row with its fitting, the version and the refresh button around the
 * age stay plain, in elements the parts leave alone.
 */

import type { CertStatus } from "../api";
import { mounter, register } from "../shared/region";
import { Choice } from "./Choice";
import { CubeProblems } from "./CubeProblems";
import { HomeNotice } from "./HomeNotice";
import { ServerSign } from "./ServerSign";
import { TlsNotice } from "./Tls";
import { Updated } from "./Updated";
import { cert, load } from "./store";

/** The header's places, each the element of the page with the id, and what is drawn there. */
const mounts = [
  mounter("header-cube", () => (
    <>
      <Choice />
      <span id="cube-problems" class="cube-problems"><CubeProblems /></span>
    </>
  )),
  mounter("updated", () => <Updated />),
  mounter("server-sign-slot", () => <ServerSign />),
  mounter("tls-slot", () => <TlsNotice />),
  mounter("home-notice", () => <HomeNotice />),
];

/** The functions the plain code calls, MassimoParts.header. */
const header = register("header", {
  /** render draws the header again from the page's state: the Cubes, the one chosen, the server's (every plain render… of the header). */
  render(): void {
    for (const m of mounts) m();
    load.bump();
  },
  /** tls takes GET /healthz's tls, null without TLS (renderTLS). */
  tls(tls: CertStatus | null): void {
    for (const m of mounts) m();
    cert.value = tls;
  },
});

/** MassimoParts.header, as index.ts declares it. */
export type HeaderPart = typeof header;
