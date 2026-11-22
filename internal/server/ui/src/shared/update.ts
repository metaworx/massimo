// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/shared/update.ts
 *
 * Whether an update of the server is out, as a signal (UiParts D219):
 * Software update's store writes GET /api/v1/update's last answer here
 * (software/), and the server's sign in the header reads it (header/), so
 * that no region reads another's: what an update would install, for one
 * who may install it.
 */

import { signal } from "@preact/signals";
import type { UpdateInfo } from "../api";
import { can } from "./me";

/** GET /api/v1/update's last answer, null before one came; every user may read the versions. */
export const update = signal<UpdateInfo | null>(null);

/** updateOut tells whether an update is out, for one who may install it. */
export function updateOut(): boolean {
  const u = update.value;
  return !!u && !!u.available && can("settings");
}

/** updateLatest is the version an update would install, "" for none. */
export function updateLatest(): string {
  return update.value?.latest || "";
}
