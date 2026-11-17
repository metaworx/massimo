// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

// The Massimo web UI, 👤 → Notifications (docs/ui.md, "Notifications"):
// which events notify the signed-in user, with their delays and the
// frost threshold, kept in the preference notify; this browser's
// subscription to the server's pushes (Web Push, docs/api-v1.md,
// "Notifications") through the service worker sw.js; the user's other
// devices; and the toast an open page shows for a notification, from
// its event stream or from the service worker. The markup is
// partials/notify.html; a notification's text is notifytext.js's.
// Loaded after login.js.
"use strict";

// NOTIFY_KINDS are the kinds in the page's order, each with its default
// as the server's (internal/notify, Defaults): on or off, and the delay
// (after_min) or the threshold (below_c) where it has one.
const NOTIFY_KINDS = [
  ["window_open", { on: true, after_min: 30 }],
  ["frost", { on: true, below_c: 8 }],
  ["battery_low", { on: true }],
  ["battery_weak", { on: true }],
  ["device_error", { on: true }],
  ["cube_offline", { on: true, after_min: 10 }],
  ["cube_online", { on: true }],
  ["device_stale", { on: false }],
  ["paired", { on: false }],
  ["cube_found", { on: false }],
  ["update", { on: false }],
];

// The bounds of a delay and of the threshold, the server's.
const NOTIFY_AFTER = [1, 1440];
const NOTIFY_BELOW = [-10, 20];

// NOTIFY_SAVE_DELAY is how long a change waits for the next before it
// goes to the server, as My settings' do.
const NOTIFY_SAVE_DELAY = 1000;

// NOTIFY_TOAST_MS is how long a toast stays; NOTIFY_SEEN_MS how long a
// notification shown is not shown again, the stream's and the push's
// copies of one coming apart by a moment.
const NOTIFY_TOAST_MS = 10000;
const NOTIFY_SEEN_MS = 60000;

const notifyState = {
  rules: null, // the user's rules by kind, once loaded
  subs: [], // the user's subscriptions, GET /api/v1/me/push
  endpoint: "", // this browser's subscription's endpoint, "" for none
  timer: null, // the save of the rules, waiting
  seen: new Map(), // the notifications shown lately: key → when
};

// notifyDefaults are the rules of a user who has set none.
function notifyDefaults() {
  const rules = {};
  for (const [kind, def] of NOTIFY_KINDS) rules[kind] = Object.assign({}, def);
  return rules;
}

// notifyRulesOf reads a notify preference over the defaults, as the
// server does: a kind or a value it leaves out keeps the default.
function notifyRulesOf(doc) {
  const rules = notifyDefaults();
  if (!doc || typeof doc !== "object") return rules;
  for (const [kind] of NOTIFY_KINDS) {
    const r = doc[kind];
    if (!r || typeof r !== "object") continue;
    if (typeof r.on === "boolean") rules[kind].on = r.on;
    if ("after_min" in rules[kind] && Number.isInteger(r.after_min)) rules[kind].after_min = r.after_min;
    if ("below_c" in rules[kind] && typeof r.below_c === "number") rules[kind].below_c = r.below_c;
  }
  return rules;
}

// notifyUnavailable says why the signed-in user has no notifications, ""
// when the user has: they are a user's, with the capability preferences.
function notifyUnavailable() {
  const me = state.me;
  if (!me || me.mode === "open" || me.mode === "none") return t("notify.noUsers");
  if (me.mode !== "users" || !me.user) return t("notify.serverKey");
  if (!hasCap("preferences")) return t("notify.noCapability");
  return "";
}

// pushUnsupported says why this browser cannot be told, "" when it can.
function pushUnsupported() {
  if (!window.isSecureContext) return t("notify.insecure");
  if (!("serviceWorker" in navigator) || !("PushManager" in window) || !("Notification" in window)) return t("notify.unsupported");
  if (Notification.permission === "denied") return t("notify.denied");
  return "";
}

async function workerRegistration() {
  if (!("serviceWorker" in navigator)) return null;
  try {
    return (await navigator.serviceWorker.getRegistration()) || null;
  } catch (e) {
    return null;
  }
}

// browserSubscription is this browser's push subscription, null for none.
async function browserSubscription() {
  const reg = await workerRegistration();
  if (!reg || !reg.pushManager) return null;
  try {
    return await reg.pushManager.getSubscription();
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

// keyBytes reads a base64url key, as pushManager.subscribe takes it.
function keyBytes(b64) {
  const s = atob(b64.replace(/-/g, "+").replace(/_/g, "/") + "===".slice((b64.length + 3) % 4));
  return Uint8Array.from(s, (c) => c.charCodeAt(0));
}

function sameKey(buf, bytes) {
  if (!buf) return false;
  const a = new Uint8Array(buf);
  return a.length === bytes.length && a.every((v, i) => v === bytes[i]);
}

// loadNotify reads the user's rules and subscriptions and this browser's
// subscription, as the page is shown.
async function loadNotify() {
  renderNotify();
  if (notifyUnavailable()) return;
  try {
    // every preference, not notify's alone, which is a 404 until set
    const [prefs, subs, sub] = await Promise.all([api("GET", "api/v1/me/preferences"), api("GET", "api/v1/me/push"), browserSubscription()]);
    notifyState.rules = notifyRulesOf((prefs || {}).notify);
    notifyState.subs = subs || [];
    notifyState.endpoint = sub ? sub.endpoint : "";
    notifySay("load", "", true);
  } catch (e) {
    notifySay("load", t("notify.loadFailed", { error: e.message }), false);
  }
  renderNotify();
}

// renderNotify draws the page from notifyState.
function renderNotify() {
  const none = notifyUnavailable();
  $("notify-none").textContent = none;
  $("notify-none").classList.toggle("hidden", !none);
  $("notify-body").classList.toggle("hidden", !!none);
  if (none) return;
  renderNotifyDevice();
  renderNotifyRules();
  renderNotifyDevices();
}

// thisBrowser is the user's subscription of this browser, null for none.
function thisBrowser() {
  return notifyState.endpoint ? notifyState.subs.find((s) => s.endpoint === notifyState.endpoint) || null : null;
}

// renderNotifyDevice draws "On this device": whether this browser is
// told, why it cannot be, and the button that turns it on or off.
function renderNotifyDevice() {
  const why = pushUnsupported();
  const on = !!thisBrowser();
  $("notify-device-state").textContent = why || t(on ? "notify.isOn" : "notify.isOff");
  $("notify-on").classList.toggle("hidden", !!why || on);
  $("notify-off").classList.toggle("hidden", !on);
  $("notify-label-row").classList.toggle("hidden", !!why || on);
}

// renderNotifyRules draws a row per kind: its switch, and its delay or
// threshold where it has one, off while the kind is.
function renderNotifyRules() {
  const box = $("notify-rules");
  clear(box);
  const rules = notifyState.rules || notifyDefaults();
  for (const [kind] of NOTIFY_KINDS) {
    const r = rules[kind];
    const row = elem("div", "notify-row");
    row.dataset.kind = kind;
    let value = null;
    const sw = switchEl(t("notify." + kind + ".label"), r.on, (on) => {
      r.on = on;
      if (value) value.disabled = !on;
      saveNotifyRules();
    });
    row.appendChild(fieldRow(null, sw, { help: "notify." + kind }));
    let field = null;
    if ("after_min" in r) field = notifyValue(r, "after_min", NOTIFY_AFTER, 1, t("notify.after"), t("notify.minutes"));
    else if ("below_c" in r) field = notifyValue(r, "below_c", NOTIFY_BELOW, 0.5, t("notify.below"), "°C");
    if (field) {
      value = field.input;
      value.disabled = !r.on;
      row.appendChild(field.row);
    }
    box.appendChild(row);
  }
}

// notifyValue is the field of a rule's delay (after_min, in whole
// minutes) or threshold (below_c), r[key] within bounds: a value out of
// them is marked on the field, the range in amber before the box
// (markRange), and not saved until the field holds one inside; a save
// meanwhile leaves the mark alone. It gives the field's row and input.
function notifyValue(r, key, bounds, step, label, unit) {
  const range = textEl("span", t("notify.range", { min: bounds[0], max: bounds[1] }), "small warn-text hidden");
  range.setAttribute("role", "status");
  const input = numberInput(r[key], bounds[0], bounds[1], step, (v) => {
    const ok = typeof v === "number" && v >= bounds[0] && v <= bounds[1] && (key !== "after_min" || Number.isInteger(v));
    if (!markRange(input, range, ok)) return;
    r[key] = v;
    saveNotifyRules();
  });
  const row = fieldRow(label, input, { unit });
  range.id = input.id + "-range";
  input.before(range);
  return { row, input };
}

// renderNotifyDevices draws the user's devices: each with its label,
// whether it is this browser, when it was added and last told, and
// Remove.
function renderNotifyDevices() {
  const box = $("notify-devices");
  clear(box);
  if (!notifyState.subs.length) {
    box.appendChild(textEl("p", t("notify.noDevices"), "muted small"));
    return;
  }
  for (const s of notifyState.subs) {
    const row = elem("div", "notify-device");
    const name = elem("span", "notify-device-name", textEl("span", s.label || t("notify.noLabel")));
    if (s.endpoint === notifyState.endpoint) name.append(" ", badgeEl(t("notify.thisBrowser"), "ok"));
    row.appendChild(name);
    row.appendChild(button(t("notify.remove"), "", (ev) => removeNotifyDevice(s, ev.currentTarget)));
    const facts = [t("notify.added", { when: fmtWhen(s.created_at) })];
    if (s.last_ok_at) facts.push(t("notify.lastOk", { when: fmtWhen(s.last_ok_at) }));
    if (s.failures) facts.push(t("notify.failures", { n: s.failures }));
    row.appendChild(textEl("p", facts.join(" · "), "muted small"));
    box.appendChild(row);
  }
}

// notifySay says on the line under the lists how something went, from
// naming whose outcome it is: "load" the rules and devices read, "save"
// the rules sent, "device" this browser turned on or off or a device
// removed. No text clears the line only where it shows from's own
// outcome, so that the reload after turning this browser on keeps what
// that said; from "" clears whatever it shows.
function notifySay(from, text, ok) {
  const msg = $("notify-msg");
  if (!text && from && msg.dataset.from !== from) return;
  msg.dataset.from = text ? from : "";
  showResult(msg, text, ok);
}

// saveNotifyRules sends the rules shortly, the next change waiting a
// second, as My settings do.
function saveNotifyRules() {
  if (notifyState.timer) clearTimeout(notifyState.timer);
  notifyState.timer = setTimeout(async () => {
    notifyState.timer = null;
    try {
      await api("PUT", "api/v1/me/preferences/notify", notifyState.rules);
      notifySay("save", "", true);
    } catch (e) {
      notifySay("save", t("notify.saveFailed", { error: e.message }), false);
    }
  }, NOTIFY_SAVE_DELAY);
}

// takeNotifySave ends the wait of saveNotifyRules and reports whether a
// save waited, for the sign-in to hold it (holdPushes, login.js).
function takeNotifySave() {
  if (!notifyState.timer) return false;
  clearTimeout(notifyState.timer);
  notifyState.timer = null;
  return true;
}

// turnOnNotify subscribes this browser: the service worker, the
// browser's permission, the subscription with the server's VAPID key,
// and the server told of it with the label given.
async function turnOnNotify(ev) {
  const btn = ev.currentTarget;
  try {
    await whileBusy(btn, async () => {
      await navigator.serviceWorker.register("sw.js");
      const reg = await navigator.serviceWorker.ready;
      if ((await Notification.requestPermission()) !== "granted") throw new Error(t("notify.notGranted"));
      const { key } = await api("GET", "api/v1/push/vapid");
      const bytes = keyBytes(key);
      let sub = await reg.pushManager.getSubscription();
      if (sub && !sameKey(sub.options && sub.options.applicationServerKey, bytes)) {
        await sub.unsubscribe(); // made for another key: the server's changed
        sub = null;
      }
      if (!sub) sub = await reg.pushManager.subscribe({ userVisibleOnly: true, applicationServerKey: bytes });
      const body = Object.assign(sub.toJSON(), { label: $("notify-label").value.trim() });
      await api("POST", "api/v1/me/push", body);
      notifyState.endpoint = sub.endpoint;
      await tellWorkerLanguage();
    });
    notifySay("device", t("notify.turnedOn"), true);
  } catch (e) {
    notifySay("device", t("notify.turnOnFailed", { error: e.message }), false);
  }
  await loadNotify();
}

// turnOffNotify ends this browser's subscription, at the server and in
// the browser.
async function turnOffNotify(ev) {
  const btn = ev.currentTarget;
  try {
    await whileBusy(btn, async () => {
      const row = thisBrowser();
      if (row) await api("DELETE", "api/v1/me/push/" + row.id);
      const sub = await browserSubscription();
      if (sub) await sub.unsubscribe();
      notifyState.endpoint = "";
    });
    notifySay("device", t("notify.turnedOff"), true);
  } catch (e) {
    notifySay("device", t("notify.turnOffFailed", { error: e.message }), false);
  }
  await loadNotify();
}

// removeNotifyDevice removes one of the user's devices; this browser's
// own subscription ends with it.
async function removeNotifyDevice(s, btn) {
  try {
    await whileBusy(btn, async () => {
      await api("DELETE", "api/v1/me/push/" + s.id);
      if (s.endpoint === notifyState.endpoint) {
        const sub = await browserSubscription();
        if (sub) await sub.unsubscribe();
        notifyState.endpoint = "";
      }
    });
    notifySay("device", "", true);
  } catch (e) {
    notifySay("device", t("notify.removeFailed", { error: e.message }), false);
  }
  await loadNotify();
}

// notifyToast shows a notification at the foot of the page for a while,
// once: the stream's copy and the push's of one show one toast.
function notifyToast(n) {
  if (!n || !n.kind) return;
  const now = Date.now();
  for (const [k, at] of notifyState.seen) if (now - at > NOTIFY_SEEN_MS) notifyState.seen.delete(k);
  const key = [n.kind, n.cube, n.device, n.time].join("|");
  if (notifyState.seen.has(key)) return;
  notifyState.seen.set(key, now);
  const { title, body } = notifyText(n, t, fmtNum);
  showToast(body, { title, ms: NOTIFY_TOAST_MS }); // components.js
}

function initNotify() {
  $("notify-on").addEventListener("click", turnOnNotify);
  $("notify-off").addEventListener("click", turnOffNotify);
  // the page shown again starts with an empty line, then reads the rules
  onView("me", () => {
    notifySay("", "", true);
    loadNotify();
  });
  // a push while this page is in front: the service worker hands it over
  if ("serviceWorker" in navigator) {
    navigator.serviceWorker.addEventListener("message", (ev) => {
      const d = ev.data || {};
      if (d.type === "notification") notifyToast(d.notification);
    });
  }
  tellWorkerLanguage();
}
