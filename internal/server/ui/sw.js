// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

// The Massimo web UI's service worker, served as sw.js beside the page so
// that its scope is the page's (docs/ui.md, "Notifications"). It shows a
// notification the server pushed (docs/api-v1.md, "Notifications") in the
// user's language, or hands it to the page where the page is in front,
// and opens the page when it is tapped. The texts are the page's
// (ui/i18n/<lang>.json, through notifytext.js), the language the one the
// page tells it; both are kept in the worker's cache, since a push may
// come while the server is out of reach. It caches nothing else and
// leaves every request of the page alone.
"use strict";

importScripts("ui/notifytext.js");

const CACHE = "massimo-notify";
const LANG_KEY = "massimo-lang";
const LANGS = ["en", "de", "fr", "it", "nl", "pl"];

// scoped is a path relative to the worker's scope, the page's address.
function scoped(path) {
  return new URL(String(path || "").replace(/^\/+/, ""), self.registration.scope).href;
}

async function kept(key) {
  const cache = await caches.open(CACHE);
  const resp = await cache.match(scoped(key));
  return resp ? resp.text() : null;
}

async function keep(key, text) {
  const cache = await caches.open(CACHE);
  await cache.put(scoped(key), new Response(text));
}

// table is a language's texts: from the server, kept for later, or the
// copy kept when the server is out of reach; empty without either.
async function table(lang) {
  const path = "ui/i18n/" + lang + ".json";
  try {
    const resp = await fetch(scoped(path), { cache: "no-cache" });
    if (resp.ok) {
      const text = await resp.text();
      await keep(path, text);
      return JSON.parse(text);
    }
  } catch (e) {
    // out of reach: the copy kept
  }
  try {
    const text = await kept(path);
    return text ? JSON.parse(text) : {};
  } catch (e) {
    return {};
  }
}

// texts renders a notification in the language the page told.
async function texts(n) {
  const lang = (await kept(LANG_KEY)) || "en";
  const en = await table("en");
  const own = lang === "en" ? en : await table(lang);
  const t = (key, params) => {
    let s = own[key];
    if (s === undefined) s = en[key];
    if (s === undefined) return key;
    return s.replace(/\{(\w+)\}/g, (m, k) => (params[k] === undefined ? m : String(params[k])));
  };
  const num = (v, digits) => new Intl.NumberFormat(lang, { minimumFractionDigits: digits, maximumFractionDigits: digits }).format(v);
  return Object.assign({ lang }, notifyText(n, t, num));
}

// show shows a notification: to the page in front, which shows it as a
// toast, else as the system's notification, which a push must show.
async function show(n) {
  const wins = await self.clients.matchAll({ type: "window", includeUncontrolled: true });
  const front = wins.filter((c) => c.visibilityState === "visible" && c.focused);
  if (front.length) {
    for (const c of front) c.postMessage({ type: "notification", notification: n });
    return;
  }
  const { title, body, lang } = await texts(n);
  await self.registration.showNotification(title, {
    body,
    lang,
    icon: scoped("ui/icons/icon-192.png"),
    tag: [n.kind, n.device || n.cube || ""].join("-"),
    timestamp: Date.parse(n.time) || Date.now(),
    data: { url: n.url || "" },
  });
}

self.addEventListener("install", () => self.skipWaiting());
self.addEventListener("activate", (ev) => ev.waitUntil(self.clients.claim()));

self.addEventListener("push", (ev) => {
  let n = {};
  try {
    n = ev.data ? ev.data.json() : {};
  } catch (e) {
    n = {};
  }
  ev.waitUntil(show(n));
});

// A tap opens the page at the notification's address, in a window of
// the page that is open already where there is one.
self.addEventListener("notificationclick", (ev) => {
  ev.notification.close();
  const url = scoped((ev.notification.data || {}).url);
  ev.waitUntil((async () => {
    const wins = await self.clients.matchAll({ type: "window", includeUncontrolled: true });
    const open = wins.find((c) => c.url.startsWith(self.registration.scope));
    if (open) {
      await open.focus();
      if (open.url !== url && "navigate" in open) await open.navigate(url).catch(() => {});
      return;
    }
    await self.clients.openWindow(url);
  })());
});

// The page tells the language it shows, at its start and when it
// changes; the worker keeps it and that language's texts.
self.addEventListener("message", (ev) => {
  const d = ev.data || {};
  if (d.type === "lang" && LANGS.includes(d.lang)) {
    ev.waitUntil(keep(LANG_KEY, d.lang).then(() => Promise.all([table("en"), table(d.lang)])));
  }
});
