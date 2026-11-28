// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

// The Massimo app's bridge, window.massimo (docs/app.md, contract
// version 6). The Android shell injects this script at the document's
// start into the connect page and into the page of the server it opened,
// and calls it with the page's kind, "connect" or "server". The calls go as messages over the
// channel the shell adds, window.massimoChannel, which answers each one;
// the shell checks every message's origin, so this script only gives the
// calls their shape: each returns a Promise, an error rejects with an
// Error whose message the page shows, and an event the shell sends is a
// "massimo" CustomEvent on window.
(function (kind) {
  "use strict";
  if (window.massimo) return;
  let channel = null;
  let seq = 0;
  const pending = new Map();

  function onMessage(ev) {
    let m;
    try {
      m = JSON.parse(ev.data);
    } catch (e) {
      return;
    }
    if (m.event) {
      window.dispatchEvent(new CustomEvent("massimo", { detail: m.event }));
      return;
    }
    const p = pending.get(m.id);
    if (!p) return;
    pending.delete(m.id);
    if (typeof m.error === "string") p.reject(new Error(m.error));
    else p.resolve(m.result === undefined ? null : m.result);
  }

  // open finds the channel, which the shell adds beside this script, and
  // says hello, so that the shell can send events before the first call.
  function open() {
    if (channel) return channel;
    const c = window.massimoChannel;
    if (!c) throw new Error("The app's channel is missing.");
    channel = c;
    channel.addEventListener("message", onMessage);
    channel.postMessage(JSON.stringify({ id: 0, call: "hello", args: [] }));
    return channel;
  }

  function call(name, ...args) {
    return new Promise((resolve, reject) => {
      seq += 1;
      pending.set(seq, { resolve, reject });
      try {
        open().postMessage(JSON.stringify({ id: seq, call: name, args }));
      } catch (e) {
        pending.delete(seq);
        reject(e);
      }
    });
  }

  const api = { version: 6 };
  if (kind === "connect") {
    api.servers = () => call("servers");
    api.save = (server) => call("save", server);
    api.remove = (id, url) => call("remove", id, url || "");
    api.discover = (ms) => call("discover", ms);
    api.probe = (urls, fp) => call("probe", urls, fp || "");
    api.trust = (id, fp) => call("trust", id, fp);
    api.open = (id, url, opts) => call("open", id, url, opts || null);
    api.forget = (id, url) => call("forget", id, url || "");
    api.forgetAll = () => call("forgetAll");
    api.session = (id, url) => call("session", id, url || "");
    api.signOut = (id, url) => call("signOut", id, url || "");
    api.signOutAll = () => call("signOutAll");
    api.settings = () => call("settings");
    api.saveSettings = (s) => call("saveSettings", s || {});
  } else {
    api.token = () => call("token");
    api.setToken = (t) => call("setToken", t === undefined ? null : t);
    api.login = Object.freeze({
      status: () => call("login.status"),
      keep: () => call("login.keep"),
      fingerprint: () => call("login.fingerprint"),
    });
    api.connect = () => call("connect");
    api.openServerSettings = () => call("openServerSettings");
    api.openAppSettings = () => call("openAppSettings");
  }
  Object.defineProperty(window, "massimo", { value: Object.freeze(api), enumerable: true });

  try {
    open();
  } catch (e) {
    document.addEventListener("DOMContentLoaded", () => {
      try {
        open();
      } catch (e2) {
        // the first call reports it
      }
    });
  }
})
