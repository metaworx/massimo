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
//            "server" for a server's page's, token, setToken, login.*
//            and connect
//   servers  the servers the app keeps, as {id, name, node, lan, …,
//            login, ask, start, session, trusted_at}
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
//   login    for kind "server", the login by fingerprint (contract
//            version 2): {status: {can, user, ask, plain}, keep: [answers
//            of login.keep in turn, true, false or "error:<message>"],
//            fingerprint: the token login.fingerprint resolves to, null,
//            or "error:<message>"}; a keep that resolves true keeps the
//            user given as keepAs; without it the page has no login
//   openLogin for kind "connect", what open(id, url, {login: true})
//            does: true (the default), false, or "error:<message>"
//
// The connect page's bridge is version 3: save keeps start at one server
// at most and turns it on for the first one added, as the adapters do;
// trust notes trusted_at; forget (Turn off) and signOut end the server's
// session; signOut answers {ended} true where it was given an address.
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
      setToken(t) {
        call("setToken", t);
        rec.token = t;
        return later(null);
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
  let seq = 0;
  window.massimo = {
    version: 3,
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
        s.session = old.session || null;
        s.trusted_at = old.trusted_at || "";
        Object.assign(old, s);
        return later(old);
      }
      s.id = "new" + ++seq;
      s.fp = "";
      s.login = "";
      s.session = null;
      s.trusted_at = "";
      rec.servers.push(s);
      return later(s);
    },
    remove(id) {
      call("remove", id);
      rec.servers = rec.servers.filter((x) => x.id !== id);
      return later(null);
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
      if (!opts || !opts.login) return later(true);
      const a = cfg.openLogin === undefined ? true : cfg.openLogin;
      if (typeof a === "string") {
        const s = rec.servers.find((x) => x.id === id);
        if (s) s.login = ""; // a key the server refused is forgotten
      }
      return answerOf(a, 30);
    },
    forget(id, url) {
      call("forget", id, url);
      const s = server(id);
      if (s) {
        s.login = "";
        s.session = null; // Turn off signs out too
      }
      return later({ revoked: !!url }, 30);
    },
    forgetAll() {
      call("forgetAll");
      for (const s of rec.servers) s.login = "";
      return later(null);
    },
    session(id, url) {
      call("session", id, url);
      const s = server(id);
      const found = cfg.sessions && id in cfg.sessions ? cfg.sessions[id] : s ? s.session || null : null;
      if (s && typeof found !== "string") s.session = null; // refused: forgotten
      if (s && typeof found === "string" && !found.startsWith("error:")) s.session = found;
      return answerOf(found, 20);
    },
    signOut(id, url) {
      call("signOut", id, url);
      const s = server(id);
      if (s) s.session = null;
      return later({ ended: !!url }, 30);
    },
    signOutAll() {
      call("signOutAll");
      for (const s of rec.servers) s.session = null;
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
