// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

// The fake bridge of the web UI suite (test_connect.py, test_tls.py):
// window.massimo as docs/app.md describes it, put into the page before
// the page's own scripts by Playwright's add_init_script, as an app's
// adapter puts the real one there. It answers from window.__fakeBridgeConfig, an init
// script of the test's before this one, which the test may change while
// the page runs:
//
//   kind     "connect" (the default) for the connect page's calls,
//            "server" for a server's page's, token, setToken, login.*,
//            connect, openServerSettings and openAppSettings
//   servers  the servers the app keeps, as {id, name, node, lan, …,
//            login, login_key, ask, start, session, trusted_at, unrevoked,
//            unended}
//   answers  by URL, how /healthz answers there: {node, version,
//            fingerprint, self_signed, subject, issuer, names,
//            not_before, not_after}; a self-signed certificate is
//            trusted only with its fingerprint given; no answer is a
//            refused connection; an HTTPS answer without the
//            certificate's details names the host in them
//   sessions by server id, what session(id, url) finds of the session
//            the app holds (contract version 3): a user's name, null for
//            one the server refused (forgotten), or "error:<message>";
//            without an entry the server's session stands
//   settings the app's settings settings() answers, {appearance}
//            ("system" by default); saveSettings keeps what it is given,
//            or rejects with saveSettingsError where that is set
//   found    the services a search finds, sent as found events
//   core     the server on the phone's status, for a bridge with core
//   push     true for a bridge with push
//   token    the token the app keeps, for kind "server"
//   appEntries for kind "server", false for an app before contract
//            version 5: no openServerSettings and openAppSettings, which
//            are there otherwise, login or none
//   revokeAtSignIn for kind "server", {user, ids, ms}: the keys and
//            sessions left there that the app revokes with a new token
//            before setToken answers, as {revoked: ids} (contract version
//            5): each revoked at the page's server with that token, ms
//            after the call, as the app's check takes its time
//   login    for kind "server", the login by fingerprint (contract
//            version 2): {status: {can, user, ask, plain, key_id, server}, keep: [answers
//            of login.keep in turn, true, false or "error:<message>"],
//            fingerprint: the token login.fingerprint resolves to, null,
//            or "error:<message>"}; a keep that resolves true keeps the
//            user given as keepAs; without it the page has no login
//   openLogin for kind "connect", what the prompt of open does at a
//            server with a kept login, a tap's (open(id, url), the rule
//            of contract version 5, its prompt simplified to every tap
//            there) and {login: true}'s: true (the default) or false, a
//            tap opening the page, at its card where false, and
//            {login: true} opening nothing for false; or
//            "error:<message>", the kept key refused and forgotten
//   keys     by server id, the key {user, id} its kept login holds, the
//            id 0 for a login kept before the id was (contract version 4)
//   unrevokable the ids of the servers whose key the server does not
//            revoke (it fails there)
//   sessionIds by server id, the session the app holds there as its last
//            check found it, {user, id} (contract version 5): where the app
//            forgets it without reaching the server, it is left in the
//            server's unended
//
// The connect page's bridge is version 5: save keeps start at one server
// at most and turns it on for the first one added, as the adapters do,
// and a server's unrevoked as it was; trust notes trusted_at; open takes
// a server's lan or remote, alone or with #profile?key=<id>, and refuses
// any other URL; forget (Turn off) and signOut end the server's session;
// signOut answers {ended} true where it was given an address; open
// with {signIn: true} opens at once, without a prompt. Turning a
// login off, as Turn off, Forget all and Remove do, as the app's Revoker
// does: the key revoked where the server answers (an address given, or
// one of its addresses in answers, as the app looks for one) and is not
// unrevokable; else, with a key id from keys, left once in the server's
// unrevoked; else forgotten on the phone alone. forget answers
// {revoked, left}, forgetAll {revoked, left, local} by server id, remove
// {revoked, keys, sessions, local} with keys and sessions the ids of the
// keys and the sessions still valid there. A session the app forgets
// without reaching the server (Sign out without an address, Turn off,
// Forget all and Remove where the server does not answer) is left in the
// server's unended by its id in sessionIds; Remove signs out first. A
// session found (session) revokes the keys and the sessions left there for
// its user.
// Every call is recorded in window.__fakeBridge.calls as [name, args…],
// and the servers, the settings and the token as they stand in
// window.__fakeBridge.
(() => {
  const cfg = window.__fakeBridgeConfig || {};
  const copy = (v) => (v === undefined ? null : JSON.parse(JSON.stringify(v)));
  const rec = {
    calls: [],
    servers: copy(cfg.servers || []),
    settings: copy(cfg.settings || { appearance: "system" }),
    token: cfg.token === undefined ? null : cfg.token,
  };
  window.__fakeBridge = rec;
  const call = (name, ...args) => rec.calls.push([name, ...copy(args)]);
  const later = (v, ms) => new Promise((resolve) => setTimeout(() => resolve(copy(v)), ms || 0));
  // answerOf resolves an answer, or rejects one of "error:<message>".
  const answerOf = (v, ms) => (typeof v === "string" && v.startsWith("error:") ? new Promise((_, reject) => setTimeout(() => reject(new Error(v.slice(6))), ms || 0)) : later(v, ms));

  if (cfg.kind === "server") {
    const api = {
      version: cfg.login ? 2 : 1,
      token() {
        call("token");
        return later(rec.token, 20);
      },
      async setToken(t) {
        call("setToken", t);
        rec.token = t;
        if (!t) return later(null);
        // as the app's check before its answer: what is left there revoked with the new token (D67-A)
        const left = cfg.revokeAtSignIn || { user: "", ids: [] };
        if (left.ms) await new Promise((resolve) => setTimeout(resolve, left.ms));
        for (const id of left.ids || []) {
          await fetch("api/v1/users/" + encodeURIComponent(left.user) + "/tokens/" + id, { method: "DELETE", headers: { Authorization: "Bearer " + t } });
        }
        return later({ revoked: left.ids || [] });
      },
    };
    if (cfg.login) {
      rec.status = copy(cfg.login.status || { can: true, user: "", ask: "start", plain: false });
      const keeps = copy(cfg.login.keep || [true]);
      api.login = {
        status() {
          call("login.status");
          return later(rec.status);
        },
        keep() {
          call("login.keep");
          const a = keeps.length > 1 ? keeps.shift() : keeps[0];
          if (a === true) rec.status.user = cfg.login.keepAs || "operator";
          return answerOf(a, 30);
        },
        fingerprint() {
          call("login.fingerprint");
          return answerOf(cfg.login.fingerprint === undefined ? null : cfg.login.fingerprint, 30);
        },
      };
      api.connect = () => {
        call("connect");
        return later(null);
      };
    }
    if (cfg.appEntries !== false) {
      // ☰'s two entries (contract version 5): the app's own page with this server's settings, or App settings
      api.openServerSettings = () => {
        call("openServerSettings");
        return later(null);
      };
      api.openAppSettings = () => {
        call("openAppSettings");
        return later(null);
      };
    }
    window.massimo = api;
    return;
  }

  const answer = (url, fp) => {
    const a = (cfg.answers || {})[url];
    if (!a) return { url, ok: false, error: "connection refused" };
    if (!url.startsWith("https:")) return { url, ok: true, node: a.node || "", version: a.version || "" };
    const host = new URL(url).hostname;
    const r = {
      url,
      fingerprint: a.fingerprint || "",
      self_signed: !!a.self_signed,
      subject: a.subject || "CN=" + host,
      issuer: a.issuer || (a.self_signed ? "CN=" + host : "CN=Test CA,O=Massimo Test"),
      names: copy(a.names || [host]),
      not_before: a.not_before || "2026-11-01T00:00:00Z",
      not_after: a.not_after || "2027-11-01T00:00:00Z",
    };
    if (a.self_signed && a.fingerprint !== fp) return Object.assign(r, { ok: false, trusted: false });
    return Object.assign(r, { ok: true, node: a.node || "", version: a.version || "" });
  };
  const server = (id) => rec.servers.find((x) => x.id === id);
  // signedOut forgets the session the app holds for s, left by its id in
  // the server's unended, once, where the server was not reached.
  const signedOut = (s, reached) => {
    const held = (cfg.sessionIds || {})[s.id];
    if (s.session && !reached && held && held.id > 0) {
      s.unended = s.unended || [];
      if (!s.unended.some((x) => x.user === held.user && x.id === held.id)) s.unended.push(copy(held));
    }
    s.session = null;
  };
  // turnOff turns a server's kept login off, as the app's Revoker does,
  // and says how its key went: "revoked", "left" (by its id, once, in the
  // server's unrevoked) or "local"; the session signed out.
  const turnOff = (s, reached) => {
    const key = (cfg.keys || {})[s.id];
    s.login = "";
    s.login_key = 0;
    signedOut(s, reached);
    if (reached && !(cfg.unrevokable || []).includes(s.id)) return "revoked";
    if (!key || !(key.id > 0)) return "local";
    s.unrevoked = s.unrevoked || [];
    if (!s.unrevoked.some((k) => k.user === key.user && k.id === key.id)) s.unrevoked.push(copy(key));
    return "left";
  };
  // reached reports whether the app reaches s: at url where the page gave
  // one, else at an address that answers, as the app looks for one.
  const reached = (s, url) => !!url || !!((cfg.answers || {})[s.lan] || (cfg.answers || {})[s.remote]);
  // opens reports whether open may load url for s: its lan or remote,
  // alone or with My profile's fragment that marks a key.
  const opens = (s, url) => {
    const at = url.indexOf("#");
    const base = at < 0 ? url : url.slice(0, at);
    if (!base || (base !== s.lan && base !== s.remote)) return false;
    return at < 0 || /^profile\?key=[1-9][0-9]{0,14}$/.test(url.slice(at + 1));
  };
  let seq = 0;
  window.massimo = {
    version: 5,
    servers() {
      call("servers");
      return later(rec.servers);
    },
    save(input) {
      call("save", input);
      const s = copy(input);
      const old = server(s.id);
      s.start = s.start === true || (!old && rec.servers.length === 0);
      if (s.start) for (const x of rec.servers) x.start = false;
      if (old) {
        s.fp = old.fp;
        s.login = old.login || "";
        s.login_key = old.login_key || 0;
        s.session = old.session || null;
        s.trusted_at = old.trusted_at || "";
        s.unrevoked = old.unrevoked || [];
        s.unended = old.unended || [];
        Object.assign(old, s);
        return later(old);
      }
      s.id = "new" + ++seq;
      s.fp = "";
      s.login = "";
      s.login_key = 0;
      s.session = null;
      s.trusted_at = "";
      s.unrevoked = [];
      s.unended = [];
      rec.servers.push(s);
      return later(s);
    },
    remove(id, url) {
      call("remove", id, url);
      const s = server(id);
      let went = "";
      if (s && s.login) went = turnOff(s, reached(s, url));
      else if (s) signedOut(s, reached(s, url)); // signed out first (D69-A)
      const keys = s ? (s.unrevoked || []).map((k) => k.id) : [];
      const sessions = s ? (s.unended || []).map((x) => x.id) : [];
      rec.servers = rec.servers.filter((x) => x.id !== id);
      return later({ revoked: went === "revoked", keys, sessions, local: went === "local" }, 30);
    },
    discover(ms) {
      call("discover", ms);
      (cfg.found || []).forEach((service, i) => {
        setTimeout(() => window.dispatchEvent(new CustomEvent("massimo", { detail: { type: "found", service: copy(service) } })), 50 * (i + 1));
      });
      return later(null, 50 * ((cfg.found || []).length + 2));
    },
    probe(urls, fp) {
      call("probe", urls, fp);
      return later(urls.map((u) => answer(u, fp)), 30);
    },
    trust(id, fp) {
      call("trust", id, fp);
      const s = rec.servers.find((x) => x.id === id);
      if (!s) return Promise.reject(new Error("no such server"));
      s.fp = fp;
      s.trusted_at = cfg.trustedAt || "2026-11-20T12:00:00Z";
      return later(null);
    },
    open(id, url, opts) {
      if (opts) call("open", id, url, opts);
      else call("open", id, url);
      const target = server(id);
      if (!target || !opens(target, url)) return answerOf("error:That address is not one of the server's.", 20);
      if (opts && opts.signIn) return later(true); // the sign-in card, no prompt
      if (!target.login || (opts && !opts.login)) return later(true);
      const a = cfg.openLogin === undefined ? true : cfg.openLogin;
      if (typeof a === "string") target.login = ""; // a key the server refused is forgotten
      if (typeof a === "string" || (opts && opts.login)) return answerOf(a, 30);
      return later(true, 30); // a tap: the page, signed in or at its card
    },
    forget(id, url) {
      call("forget", id, url);
      const s = server(id);
      const went = s && s.login ? turnOff(s, reached(s, url)) : "local"; // Turn off signs out too
      return later({ revoked: went === "revoked", left: went === "left" }, 30);
    },
    forgetAll() {
      call("forgetAll");
      const res = { revoked: [], left: [], local: [] };
      for (const s of rec.servers) {
        if (s.login) res[turnOff(s, reached(s, ""))].push(s.id); // at the address that answers
      }
      return later(res, 30);
    },
    session(id, url) {
      call("session", id, url);
      const s = server(id);
      const found = cfg.sessions && id in cfg.sessions ? cfg.sessions[id] : s ? s.session || null : null;
      if (s && typeof found !== "string") s.session = null; // refused: forgotten
      if (s && typeof found === "string" && !found.startsWith("error:")) {
        s.session = found;
        s.unrevoked = (s.unrevoked || []).filter((k) => k.user !== found); // revoked with that session
        s.unended = (s.unended || []).filter((x) => x.user !== found);
      }
      return answerOf(found, 20);
    },
    signOut(id, url) {
      call("signOut", id, url);
      const s = server(id);
      if (s) signedOut(s, !!url);
      return later({ ended: !!url }, 30);
    },
    signOutAll() {
      call("signOutAll");
      for (const s of rec.servers) signedOut(s, reached(s, ""));
      return later(null, 30);
    },
    settings() {
      call("settings");
      return later(rec.settings);
    },
    saveSettings(input) {
      call("saveSettings", input);
      if (cfg.saveSettingsError) return answerOf("error:" + cfg.saveSettingsError, 20);
      rec.settings = { appearance: input.appearance };
      return later(rec.settings, 20);
    },
  };
  if (cfg.push) window.massimo.push = {};
  if (cfg.core) {
    const st = copy(cfg.core);
    window.massimo.core = {
      status() {
        call("core.status");
        return later(st);
      },
      start() {
        call("core.start");
        st.running = true;
        return later(st, 50);
      },
      stop() {
        call("core.stop");
        st.running = false;
        return later(st, 50);
      },
    };
  }
})();
