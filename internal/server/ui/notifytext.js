// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

// The Massimo web UI, a notification's text (docs/api-v1.md,
// "Notifications"): the title and the body of a notification the server
// sent, in the language of the lookup given, from the keys
// notify.<kind>.title and notify.<kind>.body; a device_error's body
// names its flags instead (notifyBody). The page's toast (notify.js)
// and the service worker (sw.js), which has no i18n.js, share it.
"use strict";

// notifyText returns a notification's {title, body}: t(key, params) is the
// lookup, num(value, digits) formats a number in its language. A kind
// the tables do not know shows its name.
function notifyText(n, t, num) {
  const v = (n && n.values) || {};
  const device = n.device_name || n.serial || n.device || "";
  const params = {
    device,
    where: n.room_name ? device + " (" + n.room_name + ")" : device,
    room: n.room_name || "",
    cube: n.cube || "",
    serial: n.serial || "",
    minutes: v.minutes === undefined ? "" : String(v.minutes),
    down: v.down_minutes === undefined ? "" : String(v.down_minutes),
    actual: typeof v.actual_c === "number" ? num(v.actual_c, 1) : "",
    below: typeof v.below_c === "number" ? num(v.below_c, 1) : "",
    version: v.version || "",
    address: v.address || "",
  };
  const kind = String((n && n.kind) || "");
  const title = t("notify." + kind + ".title", params);
  return {
    title: title === "notify." + kind + ".title" ? "Massimo" : title,
    body: notifyBody(kind, v, t, params),
  };
}

// notifyBody is a notification's body. A device_error's names its flags
// (D180-B): the communication error (byte 5's 0x08), with the radiogram
// that failed where the server gives it, the link error (byte 6's 0x40),
// or both; one that names neither keeps the kind's own body.
function notifyBody(kind, v, t, params) {
  if (kind === "device_error" && (v.error || v.link_error)) {
    let body;
    if (v.error && v.link_error) body = t("notify.device_error.both", params);
    else if (v.error) body = t("notify.device_error.error", params);
    else body = t("notify.device_error.link", params);
    if (v.error && v.failed_radiogram) body += " " + t("notify.device_error.radiogram", { radiogram: notifyRadiogram(v.failed_radiogram) });
    return body;
  }
  const body = t("notify." + kind + ".body", params);
  return body === "notify." + kind + ".body" ? kind : body;
}

// notifyRadiogram is a failed radiogram's type as the API gives it,
// {code, name}, written as tiles.js's radiogramName writes it:
// "ControlMode (0x40)", or the code alone for one the list lacks.
function notifyRadiogram(r) {
  const code = "0x" + (Number(r.code) & 0xff).toString(16).padStart(2, "0");
  return r.name ? r.name + " (" + code + ")" : code;
}
