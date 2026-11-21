// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/chrome/store.ts
 *
 * What the plain code hands the page's chrome (MassimoParts.chrome): the
 * log's unseen problems, which ☰ and its Log entry show (the log's part,
 * log/alerts.ts renderLogSign), and the server's
 * version, ☰'s first row (header.js checkVersion).
 */

import { signal } from "@preact/signals";

/** The log's unseen problems: their count and the tone of the worst, "" for none. */
export const logSign = signal<{ count: number; tone: "" | "warn" | "bad" }>({ count: 0, tone: "" });

/** The server's whole version, as /healthz gave it when the page loaded; "" until then. */
export const version = signal("");
