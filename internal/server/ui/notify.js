// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

// The Massimo web UI, 👤 → Notifications (docs/ui.md, "Notifications"):
// its plain names, which call its part (ui/src/notify/,
// MassimoParts.notify), where the rules, this browser's subscription to
// the server's pushes and the user's devices are drawn; the language the
// service worker sw.js shows a notification in; and the toast an open
// page shows for a notification, from its event stream or from the
// service worker. A notification's text is notifytext.js's. Loaded after
// login.js.
"use strict";

// NOTIFY_TOAST_MS is how long a toast stays; NOTIFY_SEEN_MS how long a
// notification shown is not shown again, the stream's and the push's
// copies of one coming apart by a moment.
const NOTIFY_TOAST_MS = 10000;
const NOTIFY_SEEN_MS = 60000;

// notifySeen are the notifications shown lately: key → when.
const notifySeen = new Map();

async function workerRegistration() {
  if (!("serviceWorker" in navigator)) return null;
  try {
    return (await navigator.serviceWorker.getRegistration()) || null;
  } catch (e) {
    return null;
  }
}

// tellWorkerLanguage tells the service worker the page's language, in
// which it shows a notification; nothing without one.
async function tellWorkerLanguage() {
  const reg = await workerRegistration();
  const w = reg && (reg.active || reg.waiting || reg.installing);
  if (w) w.postMessage({ type: "lang", lang: i18n.lang });
}

// renderNotify draws the card again (app.js applyLanguage).
function renderNotify() {
  MassimoParts.notify.render();
}

// saveNotifyRules sends the rules shortly, the next change waiting a
// second (login.js resumePushes).
function saveNotifyRules() {
  MassimoParts.notify.save();
}

// takeNotifySave ends the wait of saveNotifyRules and reports whether a
// save waited, for the sign-in to hold it (holdPushes, login.js).
function takeNotifySave() {
  return MassimoParts.notify.takeSave();
}

// sendNotifySave sends a rule changed within the last second at once,
// under the user's own token, for Sign out before the session ends
// (logout, login.js; UiParts D205).
function sendNotifySave() {
  return MassimoParts.notify.sendNow();
}

// notifyToast shows a notification at the foot of the page for a while,
// once: the stream's copy and the push's of one show one toast.
function notifyToast(n) {
  if (!n || !n.kind) return;
  const now = Date.now();
  for (const [k, at] of notifySeen) if (now - at > NOTIFY_SEEN_MS) notifySeen.delete(k);
  const key = [n.kind, n.cube, n.device, n.time].join("|");
  if (notifySeen.has(key)) return;
  notifySeen.set(key, now);
  const { title, body } = notifyText(n, t, fmtNum);
  showToast(body, { title, ms: NOTIFY_TOAST_MS }); // components.js
}

function initNotify() {
  // a push while this page is in front: the service worker hands it over
  if ("serviceWorker" in navigator) {
    navigator.serviceWorker.addEventListener("message", (ev) => {
      const d = ev.data || {};
      if (d.type === "notification") notifyToast(d.notification);
    });
  }
  tellWorkerLanguage();
}
