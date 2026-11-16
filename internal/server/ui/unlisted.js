// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

// The Massimo web UI, Devices without a room: the devices of the Cube
// that are in no room, each with Put into a room and Remove from the
// Cube, are a part (ui/src/rooms/Unlisted.tsx), with the message line
// under them; here what stays plain, the removal from the Cube, which a
// room's configuration removes a device by too.
"use strict";

// removeFromCube deletes a device from the Cube (DELETE …/devices/{sn})
// and reloads the rooms, which the metadata and device_forgotten events
// do as well; it returns the outcome as a text.
async function removeFromCube(d) {
  const label = deviceName(d);
  try {
    const res = await api("DELETE", devicePath(d));
    delete state.details[d.rf_address];
    await loadRooms();
    let text = t("device.removed", { device: label });
    if (res && res.confirmation === "pending") text += " " + t("result.structurePending");
    return { text, ok: true };
  } catch (e) {
    return { text: errorText(label, e), ok: false };
  }
}
