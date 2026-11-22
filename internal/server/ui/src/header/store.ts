// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/header/store.ts
 *
 * What the header's parts are drawn from beside the page's state.cubes,
 * state.found and state.serial, signals underneath (shared/data.ts): the
 * search for Cubes under way, and the certificate the server serves, with
 * whether the app pinned it. The Cube chosen is the model's (chosenCube
 * of model/cube.tsx).
 */

import { signal } from "@preact/signals";
import type { CertStatus } from "../api";
import { live } from "../shared/live";

/** The search for Cubes on the LAN, while it runs (discoverCubes). */
export const discovery = live({ running: false });

/** The certificate the server serves, from GET /healthz's tls; null without TLS (renderTLS). */
export const cert = signal<CertStatus | null>(null);

/** Whether the page runs in the app, which pinned the certificate's fingerprint when the server was added (renderTLS). */
export const pinned = signal(false);
