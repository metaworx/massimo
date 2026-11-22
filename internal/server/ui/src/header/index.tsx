// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/header/index.tsx
 *
 * The header as parts (AP UiParts, Block 6): the pile, every Cube's
 * problems led by the server's sign (AP GlobalMenu, Blocks 12 and 14); the
 * Cube choice; the reads' times in expert mode's radio budget line; the
 * line about the certificate; and the home notice on Rooms, each drawn
 * once into its place in the page and updated in place; and the row's
 * fold where it is short (fold.ts). The plain code
 * calls MassimoParts.header where a plain field the header shows changed
 * (app.js' loads, header.js renderHeader, renderServerSign and renderTLS,
 * home.js renderHomeNotice and addFound, a change of language or of the
 * icons, which folds the row anew). The logo,
 * the version, the radio budget and the pages stay plain, in elements the
 * parts leave alone.
 */

import type { CertStatus } from "../api";
import { mounter, register } from "../shared/region";
import { batch } from "@preact/signals";
import { Choice } from "./Choice";
import { nowAgain } from "./clock";
import { watchFold } from "./fold";
import { HomeNotice } from "./HomeNotice";
import { Pile } from "./Pile";
import { ExpertTimes } from "./Times";
import { TlsNotice } from "./Tls";
import { cert, pinned } from "./store";

/** The header's places, each the element of the page with the id, and what is drawn there. */
const mounts = [
  mounter("header-pile", () => <Pile />),
  mounter("header-cube", () => <Choice />),
  mounter("header-times", () => <ExpertTimes />),
  mounter("tls-slot", () => <TlsNotice />),
  mounter("home-notice", () => <HomeNotice />),
];

/** The functions the plain code calls, MassimoParts.header. */
const header = register("header", {
  /** render takes a load of the page's state (every plain render… of the header): the parts follow the data themselves, the reads' times count from now. */
  render(): void {
    for (const m of mounts) m();
    // the reads' times from now, not from the clock's last second
    nowAgain();
    // the row folded anew where the pile's icons or the Cube's name change its widths
    watchFold();
  },
  /** tls takes GET /healthz's tls, null without TLS, and whether the app pinned its fingerprint (renderTLS). */
  tls(tls: CertStatus | null, inApp = false): void {
    for (const m of mounts) m();
    batch(() => {
      cert.value = tls;
      pinned.value = inApp;
    });
  },
});

/** MassimoParts.header, as index.ts declares it. */
export type HeaderPart = typeof header;
