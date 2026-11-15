// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

// The Massimo web UI, Devices without a room: the devices of the Cube
// that are in no room, each with Put into a room and Remove from the
// Cube.
"use strict";

// unlistedUI is per device without a room what its row has open: form
// (Put into a room…, with edit, what the form holds) or confirm (Remove
// from the Cube asked).
const unlistedUI = {};

// renderUnlisted is a device of Devices without a room: its row
// (renderDevice) with Put into a room… and Remove from the Cube
// (capability structure), and under it the form that puts it into a
// room (pairDeviceForm) or the question whether to remove it.
function renderUnlisted(d) {
  const el = renderDevice(d);
  const ui = unlistedUI[d.rf_address] || (unlistedUI[d.rf_address] = { edit: {} });
  const redraw = () => {
    const old = document.querySelector('#unlisted-devices .device[data-rf="' + d.rf_address + '"]');
    if (old) old.replaceWith(renderUnlisted(d));
  };
  if (!ui.form && !ui.confirm) {
    const label = deviceName(d);
    el.querySelector(".device-actions").append(
      button(t("pairing.putOpen"), { cls: "needs-structure put-open", aria: t("pairing.putOpenLabel", { device: label }) }, () => {
        ui.form = true;
        redraw();
        const row = document.querySelector('#unlisted-devices .device[data-rf="' + d.rf_address + '"]');
        if (row) row.querySelector("form select, form input").focus();
      }),
      button(t("device.remove"), { cls: "needs-structure remove-device", aria: t("device.removeLabel", { device: label }) }, () => {
        ui.confirm = true;
        redraw();
        const row = document.querySelector('#unlisted-devices .device[data-rf="' + d.rf_address + '"]');
        if (row) row.querySelector(".confirm-box").focusFirst();
      }));
  }
  if (ui.form) {
    el.appendChild(pairDeviceForm(state.serial, d, ui.edit, {
      onDone: () => {
        delete unlistedUI[d.rf_address];
        showUnlistedResult(ui.edit.result.text, true);
      },
      onFail: () => {
        // A copy that stopped put the device into its room all the same.
        if (!state.unlisted.some((x) => x.rf_address === d.rf_address)) {
          delete unlistedUI[d.rf_address];
          showUnlistedResult(ui.edit.result.text, false);
        }
      },
      onCancel: () => {
        ui.form = false;
        redraw();
      },
    }));
  }
  if (ui.confirm) {
    const box = confirmBox(t("device.removeConfirm", { device: deviceName(d) }), { yes: t("device.remove"), no: t("action.keep") }, async () => {
      if (ui.removing) return;
      ui.removing = true; // one DELETE: the question stays until it is answered
      for (const b of box.querySelectorAll("button")) b.disabled = true;
      const res = await removeFromCube(d);
      ui.confirm = false;
      ui.removing = false;
      showUnlistedResult(res.text, res.ok);
      redraw();
    }, () => {
      ui.confirm = false;
      redraw();
    });
    el.appendChild(box);
  }
  return el;
}

// showUnlistedResult says under Devices without a room how the last
// action went, the device's row being gone after it, or on the page's
// message line when the section went with its last device; a success
// fades.
function showUnlistedResult(text, ok) {
  const p = $("unlisted").classList.contains("hidden") ? $("message") : $("unlisted-msg");
  showResult(p, text, ok);
  if (ok) {
    setTimeout(() => {
      if (p.textContent === text) p.classList.add("hidden");
    }, RESULT_FADE_MS);
  }
}

// removeFromCube deletes a device from the Cube (DELETE …/devices/{sn})
// and reloads the rooms, which the metadata and device_forgotten events
// do as well; it returns the outcome as a text.
async function removeFromCube(d) {
  const label = deviceName(d);
  try {
    await api("DELETE", devicePath(d));
    delete state.details[d.rf_address];
    await loadRooms();
    return { text: t("device.removed", { device: label }), ok: true };
  } catch (e) {
    return { text: errorText(label, e), ok: false };
  }
}
