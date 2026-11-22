// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/rooms/send.ts
 *
 * What the rooms' parts send that two of them share: a device removed
 * from the Cube, from the devices without a room and from a room's
 * configuration (UiParts D233, from unlisted.js). It takes the session's
 * number before its request, so that an answer after a sign-out reads
 * nothing and says nothing for the next user.
 */

import type { Device } from "../api";
import { session, stillSession } from "../shared/session";

/**
 * removeFromCube deletes a device from the Cube (DELETE …/devices/{sn})
 * and reloads the rooms, which the metadata and device_forgotten events
 * do as well; it returns the outcome as a text, or null where a sign-out
 * came meanwhile.
 */
export async function removeFromCube(d: Device): Promise<{ text: string; ok: boolean } | null> {
  const label = deviceName(d);
  const n = session();
  try {
    const res = await api<{ confirmation?: string } | null>("DELETE", devicePath(d));
    if (!stillSession(n)) return null; // an answer after a sign-out: no read under the next user's key
    delete state.details[d.rf_address];
    await loadRooms();
    let text = t("device.removed", { device: label });
    if (res && res.confirmation === "pending") text += " " + t("result.structurePending");
    return { text, ok: true };
  } catch (e) {
    return { text: errorText(label, e as Error), ok: false };
  }
}
