// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/header/index.tsx
 *
 * The header as parts (AP UiParts, Block 6): the pile, the chosen Cube's
 * age above every Cube's problems led by the server's sign (AP GlobalMenu,
 * Block 12); the Cube choice; the line about the certificate; and the
 * home notice on Rooms, each drawn once into its place in the page and
 * updated in place. The plain code calls MassimoParts.header (header.js
 * renderCubeSelect, renderUpdated, renderServerSign and renderTLS,
 * home.js renderHomeNotice). The logo, the version and the pages stay
 * plain, in elements the parts leave alone.
 */

import type { CertStatus } from "../api";
import { mounter, register } from "../shared/region";
import { Choice } from "./Choice";
import { HomeNotice } from "./HomeNotice";
import { Pile } from "./Pile";
import { TlsNotice } from "./Tls";
import { cert, load } from "./store";

/** The header's places, each the element of the page with the id, and what is drawn there. */
const mounts = [
  mounter("header-pile", () => <Pile />),
  mounter("header-cube", () => <Choice />),
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
