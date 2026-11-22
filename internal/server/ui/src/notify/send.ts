// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/notify/send.ts
 *
 * What the notifications' card reads and sends, line for line as
 * notify.js did: the user's rules (every preference, notify's among
 * them, which is a 404 until set), its subscriptions and this browser's,
 * read as the personal page is shown; a rule changed sent a second
 * after the last change (PUT …/me/preferences/notify), held by the
 * sign-in as login.js holdPushes holds it, and sent at once by Sign out
 * under the user's token (UiParts D205); this browser turned on (the
 * service worker, the permission, the subscription with the server's
 * key, POST …/me/push), off, and a device removed. An answer after a
 * sign-out is dropped (shared/session.ts).
 */

import type { PushSubscription } from "../api";
import { session, stillSession } from "../shared/session";
import { defaults, notify, rulesOf, type Rules, say } from "./store";
import { can, me } from "../shared/me";

/** How long a change waits for the next before it goes to the server, as My settings' do. */
const SAVE_DELAY = 1000;

/** The save of the rules, waiting, and the session it was asked for in. */
let timer: ReturnType<typeof setTimeout> | null = null;

/** unavailable says why the signed-in user has no notifications, "" where it has: they are a user's, with the capability preferences. */
export function unavailable(): string {
  const m = me.value;
  if (!m || m.mode === "open" || m.mode === "none") return t("notify.noUsers");
  if (m.mode !== "users" || !m.user) return t("notify.serverKey");
  if (!can("preferences")) return t("notify.noCapability");
  return "";
}

/** unsupported says why this browser cannot be told, "" where it can. */
export function unsupported(): string {
  if (!window.isSecureContext) return t("notify.insecure");
  if (!("serviceWorker" in navigator) || !("PushManager" in window) || !("Notification" in window)) return t("notify.unsupported");
  if (Notification.permission === "denied") return t("notify.denied");
  return "";
}

/** browserSubscription is this browser's push subscription, null for none. */
async function browserSubscription(): Promise<globalThis.PushSubscription | null> {
  const reg = await workerRegistration();
  if (!reg || !reg.pushManager) return null;
  try {
    return await reg.pushManager.getSubscription();
  } catch {
    return null;
  }
}

/** keyBytes reads a base64url key, as pushManager.subscribe takes it. */
function keyBytes(b64: string): Uint8Array {
  const s = atob(b64.replace(/-/g, "+").replace(/_/g, "/") + "===".slice((b64.length + 3) % 4));
  return Uint8Array.from(s, (c) => c.charCodeAt(0));
}

/** sameKey says whether a subscription's key is the server's. */
function sameKey(buf: ArrayBuffer | null | undefined, bytes: Uint8Array): boolean {
  if (!buf) return false;
  const a = new Uint8Array(buf);
  return a.length === bytes.length && a.every((v, i) => v === bytes[i]);
}

/** thisBrowser is the user's subscription of this browser, null for none. */
export function thisBrowser(): PushSubscription | null {
  return notify.endpoint ? notify.subs.find((s) => s.endpoint === notify.endpoint) || null : null;
}

/**
 * loadNotify reads the user's rules and subscriptions and this browser's
 * subscription, as the page is shown. Where a change of the rules waits
 * to be sent, the rules shown are kept: they are what it sends.
 */
export async function loadNotify(): Promise<void> {
  if (unavailable()) return;
  const n = session();
  try {
    const [prefs, subs, sub] = await Promise.all([
      api<Record<string, unknown>>("GET", "api/v1/me/preferences"),
      api<PushSubscription[]>("GET", "api/v1/me/push"),
      browserSubscription(),
    ]);
    if (!stillSession(n)) return;
    if (!timer) notify.rules = rulesOf((prefs || {}).notify);
    notify.loaded = true;
    notify.subs = subs || [];
    notify.endpoint = sub ? sub.endpoint : "";
    notify.ver++;
    say("load", "", true);
  } catch (e) {
    if (stillSession(n)) say("load", t("notify.loadFailed", { error: (e as Error).message }), false);
  }
}

/** The defaults as the card shows them before the rules are read. */
const DEFAULTS = defaults();

/** shown are the rules the card shows: the user's once read, else the defaults. */
export function shown(): Rules {
  return notify.rules || DEFAULTS;
}

/** rules are the rules to change, made from the defaults at the first change before any were read. */
export function rules(): Rules {
  if (!notify.rules) notify.rules = defaults();
  return notify.rules;
}

/** saveRules sends the rules shortly, the next change waiting a second, as My settings do. */
export function saveRules(): void {
  if (timer) clearTimeout(timer);
  const n = session();
  timer = setTimeout(async () => {
    timer = null;
    if (!stillSession(n)) return;
    try {
      await api("PUT", "api/v1/me/preferences/notify", notify.rules);
      if (stillSession(n)) say("save", "", true);
    } catch (e) {
      if (stillSession(n)) say("save", t("notify.saveFailed", { error: (e as Error).message }), false);
    }
  }, SAVE_DELAY);
}

/** takeSave ends the wait of saveRules and says whether a save waited, for the sign-in to hold it (holdPushes, login.js). */
export function takeSave(): boolean {
  if (!timer) return false;
  clearTimeout(timer);
  timer = null;
  return true;
}

/**
 * turnOn subscribes this browser: the service worker, the browser's
 * permission, the subscription with the server's VAPID key, and the
 * server told of it with the label given; then the card is read again.
 */
export async function turnOn(): Promise<void> {
  const n = session();
  try {
    await navigator.serviceWorker.register("sw.js");
    const reg = await navigator.serviceWorker.ready;
    if ((await Notification.requestPermission()) !== "granted") throw new Error(t("notify.notGranted"));
    const { key } = await api<{ key: string }>("GET", "api/v1/push/vapid");
    const bytes = keyBytes(key);
    let sub = await reg.pushManager.getSubscription();
    if (sub && !sameKey(sub.options?.applicationServerKey, bytes)) {
      await sub.unsubscribe(); // made for another key: the server's changed
      sub = null;
    }
    if (!sub) sub = await reg.pushManager.subscribe({ userVisibleOnly: true, applicationServerKey: bytes as BufferSource });
    const body = Object.assign(sub.toJSON(), { label: notify.label.trim() });
    await api("POST", "api/v1/me/push", body);
    if (!stillSession(n)) return;
    notify.endpoint = sub.endpoint;
    await tellWorkerLanguage();
    if (!stillSession(n)) return;
    say("device", t("notify.turnedOn"), true);
  } catch (e) {
    if (!stillSession(n)) return;
    say("device", t("notify.turnOnFailed", { error: (e as Error).message }), false);
  }
  await loadNotify();
}

/** turnOff ends this browser's subscription, at the server and in the browser; then the card is read again. */
export async function turnOff(): Promise<void> {
  const n = session();
  try {
    const row = thisBrowser();
    if (row) await api("DELETE", "api/v1/me/push/" + row.id);
    const sub = await browserSubscription();
    if (sub) await sub.unsubscribe();
    if (!stillSession(n)) return;
    notify.endpoint = "";
    say("device", t("notify.turnedOff"), true);
  } catch (e) {
    if (!stillSession(n)) return;
    say("device", t("notify.turnOffFailed", { error: (e as Error).message }), false);
  }
  await loadNotify();
}

/** removeDevice removes one of the user's devices; this browser's own subscription ends with it; then the card is read again. */
export async function removeDevice(s: PushSubscription): Promise<void> {
  const n = session();
  try {
    await api("DELETE", "api/v1/me/push/" + s.id);
    if (s.endpoint === notify.endpoint) {
      const sub = await browserSubscription();
      if (sub) await sub.unsubscribe();
      if (stillSession(n)) notify.endpoint = "";
    }
    if (!stillSession(n)) return;
    say("device", "", true);
  } catch (e) {
    if (!stillSession(n)) return;
    say("device", t("notify.removeFailed", { error: (e as Error).message }), false);
  }
  await loadNotify();
}

/**
 * sendNow sends a save that waits at once, under the user's own token
 * and without asking for a sign-in, as logout sends the waiting ui and
 * schemes (login.js logout, notify.js sendNotifySave; UiParts D205); a
 * refusal changes nothing; nothing waits, nothing is sent.
 */
export async function sendNow(): Promise<void> {
  if (!takeSave()) return;
  await apiRequest("PUT", "api/v1/me/preferences/notify", notify.rules).catch(() => {});
}

/**
 * dropSave drops a save still waiting at a sign-out, for the page's
 * safety alone: logout has sent it already (sendNow), and for another
 * user signed in after an expired session login.js holdPushes took it
 * and dropped it; sent later, it would go under whoever signs in next.
 */
export function dropSave(): void {
  takeSave();
}
