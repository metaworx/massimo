// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

// The Massimo web UI, a notification's text (docs/api-v1.md,
// "Notifications"): the title and the body of a notification the server
// sent, in the language of the lookup given, from the keys
// notify.<kind>.title and notify.<kind>.body. The page's toast (notify.js)
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
  const body = t("notify." + kind + ".body", params);
  return {
    title: title === "notify." + kind + ".title" ? "Massimo" : title,
    body: body === "notify." + kind + ".body" ? kind : body,
  };
}
