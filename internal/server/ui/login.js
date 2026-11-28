// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

// The Massimo web UI, login: who the page is signed in as (GET
// /api/v1/me), the sign-in dialog in place of the token prompt (a name
// and a password, signed in by OPAQUE so that the password stays in the
// browser, or an API key pasted, and in the app the login by
// fingerprint, and on a server without users the first owner's form,
// its card a part: ui/src/signin/), Log out, hiding what
// the role may not do, the proof of the own password and a password
// registered by OPAQUE (which My profile, ui/src/profile/, sets the own
// password with, and the first owner's form, ui/src/signin/, signs in
// with). Loaded after app.js; api() calls askLogin on a 401, and sends
// only a read again once it is in.
"use strict";

// CAPABILITIES are the server's, in its order (docs/api-v1.md,
// "Authentication"); each hides, as a class needs-<capability> in the
// markup, what it alone allows.
const CAPABILITIES = ["read", "preferences", "setpoint", "lock", "configure", "rename", "structure", "pair", "discover", "backup", "restore", "repair",
  "guests", "users", "roles", "roles.disable", "capabilities", "cubes", "cubes.reboot", "settings", "update", "restart", "reboot", "audit",
  "auth", "integrations"];

// DEFAULT_ROLES are the keys of the six default roles, for a role choice
// that cannot read the roles.
const DEFAULT_ROLES = ["owner", "manager", "operator", "guest", "viewer", "integration"];

// capClass is the class a capability is hidden by: the dot of
// roles.disable becomes a dash.
function capClass(c) {
  return c.replace(".", "-");
}

// hasCap reports whether the signed-in user may do something; outside
// users mode everyone may do everything.
function hasCap(c) {
  const me = state.me;
  if (!me || me.mode !== "users") return true;
  return (me.capabilities || []).includes(c);
}

// canManageUsers: the users routes, with users, guests, passwords, or
// integrations for an integration's users (AP AccessEnds, D46-A).
function canManageUsers() {
  return hasCap("users") || hasCap("guests") || hasCap("passwords") || hasCap("integrations");
}

// hasSystem reports whether the signed-in user sees System settings: one
// of the capabilities of its panels, Cubes, backups, users, roles, and
// the software's update, the server's restart and the computer's reboot.
function hasSystem() {
  return ["cubes", "pair", "backup", "roles", "capabilities", "auth", "integrations", "update", "restart", "reboot"].some(hasCap) || canManageUsers();
}

// hasSettingsPage reports whether the signed-in user may open System
// settings: one of its cards' capabilities, without those only
// Maintenance's cards use (update, restart, reboot).
function hasSettingsPage() {
  return ["cubes", "pair", "backup", "roles", "capabilities", "auth", "integrations"].some(hasCap) || canManageUsers();
}

// isSelf reports whether a user is the signed-in one.
function isSelf(name) {
  return !!(state.me && state.me.user && state.me.user.name === name);
}

// roleLabel is a role's display name, the key's translated name, or
// the key.
function roleLabel(key, name) {
  if (name) return name;
  const k = "role." + key;
  const s = t(k);
  return s === k ? key : s;
}

// meToken is the token state.me was read with (loadMe), "" for none: the
// kind of the token the page holds is known only for that one.
let meToken = "";

// tokenAnswered is the bridge's answer to the last token setToken handed
// it, a sign-in's: from contract version 5 {revoked}, the keys and
// sessions left there that the app revoked before it answered (AP
// MobileApp, D67-A); null in a browser.
let tokenAnswered = Promise.resolve(null);

// tokenAnswer is that answer, for My profile, whose keys are read once the
// app has revoked what the sign-in revokes (ui/src/shared/app.ts).
function tokenAnswer() {
  return tokenAnswered;
}

// setToken keeps the page's token, or forgets it with null: in the
// browser's storage, or in the app's where the page runs in the app
// (appBridge, app.js). A token the app could not keep still serves this
// page, and the app asks for a sign-in at its next start. A session that
// a new token replaces is ended at the server once (AP MobileApp, D68-A):
// in a browser here, POST /api/v1/logout with it (endSession), not awaited
// and a refusal ignored, where who is in was read with it and it is a session
// (a key answers 400 there, and one not read yet is left as it is); in
// the app by the bridge's setToken, which holds it. It returns the
// bridge's answer, null in a browser and where the bridge failed, so that
// what reads the session from the app next (login.keep, which the bridge
// answers from the session it holds) waits for it to be kept there.
function setToken(token) {
  const old = state.token;
  state.token = token;
  const b = appBridge();
  if (!b) {
    store("massimo.token", token);
    if (token && old && old !== token && old === meToken && state.me && state.me.token && state.me.token.kind === "session") {
      endSession(old);
    }
    return Promise.resolve(null);
  }
  const answer = Promise.resolve(b.setToken(token)).catch(() => null);
  if (token) tokenAnswered = answer;
  return answer;
}

// endSession ends a session at the server that the page does not keep:
// POST /api/v1/logout with it, not awaited and a refusal ignored; nothing
// for no token, which would send the page's own. For a session setToken
// replaces, and for a sign-in's that answered once its opening of the
// sign-in's card had ended (ui/src/signin/Card.tsx; AP AccessEnds, D2),
// so that no live session is left that nobody holds. kept says that the
// app's bridge keeps it, as login.fingerprint keeps its session: where
// the page holds no token, the session having been forgotten meanwhile
// by a sign-out, the bridge is left with none too.
function endSession(token, kept) {
  if (!token) return;
  apiRequest("POST", "api/v1/logout", undefined, { token }).catch(() => {});
  if (kept && !state.token) setToken(null);
}

// sawOpen is set once the page has seen a server without a store ask for
// no key: the sign-in dialog then opens its API-key part, since the only
// way in there is MASSIMO_API_TOKEN (or -token). A server seen open with
// a store gets its owner from the first owner's form, with a password,
// so its later sign-in opens on the password (AP FirstOwner, D42).
let sawOpen = false;

// A sign-in waits for none of its reads to come back in this session:
// askLogin settles before forgetSession raises the session's number, and
// the start that waited for it (loadMeAndPreferences) and afterLogin each
// read who is in again after it, in the new session, so that a late
// answer here, which writes nothing, leaves who is in to that read.
// loadMe reads who the page is signed in as and applies it; a server
// that refuses opens the sign-in dialog through api(). The page starts
// signed out (index.html), so that no button of the header shows before
// the sign-in; once the server has answered, signed in, open, or not at
// all (a server out of reach shows the page and its message), the page
// shows, unless the sign-in dialog is still open (askLogin). A server
// without users that asks for its owner gets the sign-in card with the
// first owner's form (askLogin "owner"), opened here, after every read
// of who is in, and not waited for: the starts wait for it
// (loadMeAndPreferences), so that nothing of the page is read or shown
// behind it (AP FirstOwner, D27); with asks false the read asks for
// nobody, for the read that follows keeping the server open turned off,
// after which the next load asks (D31, D44).
async function loadMe(asks = true) {
  const n = MassimoParts.session();
  let me = null;
  try {
    me = await api("GET", "api/v1/me");
  } catch (e) {
    me = null;
  }
  if (!MassimoParts.stillSession(n)) return state.me; // who was in before a sign-out: the next sign-in reads who is in, and asks for the owner there
  meToken = state.token || ""; // a read sent again after a sign-in carried the token held now (api, app.js)
  state.me = me;
  if (state.me && state.me.mode === "none") sawOpen = true;
  applyCaps();
  if (asks && MassimoParts.signIn.asksOwner()) askLogin("owner"); // ui/src/signin/store.ts: open, with a store, kept open by nobody, not skipped in this load
  if (!$("login-dialog").open) document.body.classList.remove("signed-out");
  return state.me;
}

// applyCaps hides what the signed-in user may not do: the body gets a
// class no-<capability> per capability the user lacks, and the markup
// marks what each one hides with needs-<capability>. Who is in is the
// parts' signal underneath state.me (ui/src/shared/data.ts): the open
// server's warning is drawn from it, and the sign-in card says by it
// whether to ask for the first owner (ui/src/signin/).
function applyCaps() {
  const body = document.body;
  for (const c of CAPABILITIES) body.classList.toggle("no-" + capClass(c), !hasCap(c));
  body.classList.toggle("no-users-admin", !canManageUsers());
  body.classList.toggle("no-sync", !canSync());
  body.classList.toggle("no-system", !hasSettingsPage());
  body.classList.toggle("no-maint", !hasSystem());
  const pages = MassimoParts.pages;
  if (!pages.mayShow(pages.shown())) pages.showView("tiles", "replace"); // a page the user signed in now may not open
}

// --- the sign-in through a provider (docs/api-v1.md, "Sign-in through OpenID Connect")

// pageBase is the page's own address, without its file, query or
// fragment: the provider sends the browser back to it, and the redirect
// URI is under it.
function pageBase() {
  return new URL(".", location.origin + location.pathname).href;
}

// oidcError is why a sign-in through a provider came back refused, for
// the sign-in card to say; null for none.
let oidcError = null;

// oidcLinked is a link of the user's own account come back, for My
// profile (loadProfile, ui/src/profile/send.ts), which sets it back to
// null once taken: {confirm} its code, to show it before it is stored,
// or {error}; null for none.
let oidcLinked = null;

// oidcReturn reads the fragment a provider's sign-in comes back with,
// before the page reads its own (MassimoParts.pages.showAddress): #oidc=<code>,
// traded for the session, or #oidc-error=<reason>; a link's,
// #oidc-link-confirm=<code> or #oidc-link-error=<reason>, opens My
// profile, which shows the link to confirm or says why there is none.
// The fragment goes from the address either way.
async function oidcReturn() {
  const m = /^#oidc(-error|-link-confirm|-link-error)?=([A-Za-z0-9_-]*)$/.exec(location.hash);
  if (!m) return;
  if (m[1] === "-link-confirm" || m[1] === "-link-error") {
    oidcLinked = m[1] === "-link-confirm" ? { confirm: m[2] } : { error: ["taken", "group"].includes(m[2]) ? m[2] : "failed" };
    history.replaceState(null, "", location.pathname + location.search + "#profile");
    return;
  }
  history.replaceState(null, "", location.pathname + location.search);
  if (m[1]) {
    oidcError = m[2];
    return;
  }
  try {
    const session = await apiRequest("POST", "api/v1/login/oidc/finish", { code: m[2] }, { auth: false });
    setToken(session.token);
  } catch (e) {
    oidcError = "failed";
  }
}

// renderLoginProviders hands the sign-in's card the enabled providers,
// a button each (ui/src/signin/Card.tsx); none in the app, whose WebView
// the providers refuse.
async function renderLoginProviders() {
  const list = appBridge() ? [] : await loadLoginProviders(); // providers.js
  MassimoParts.signIn.providers(list);
}

// startProvider starts a sign-in through a provider and answers the
// provider's page to go to, which the sign-in's card opens only while the
// opening its button was pressed in is still the card's (AP AccessEnds,
// Block 1); its refusal is thrown, for the card to say under the
// providers' buttons.
async function startProvider(key) {
  const started = await apiRequest("POST", "api/v1/login/oidc/" + encodeURIComponent(key), { redirect: pageBase() }, { auth: false });
  return started.url;
}

// --- the sign-in dialog -----------------------------------------------

let loginPromise = null;

// startWaits counts the starts that wait for who is in, the page's own
// and Log out's (loadMeAndPreferences): each goes on by itself after a
// sign-in, with the settings, the data and the stream, so the dialog
// starts nothing beside it (afterLogin), which would start the page twice.
let startWaits = 0;

// loadMeAndPreferences reads who is in, through the sign-in dialog where
// the server asks for one, and through the first owner's card where the
// server asks for its owner (loadMe), then the user's settings from the
// server; once both have settled, a card closed on a sign-in or a new
// owner meanwhile empties its secret fields (MassimoParts.signIn.settled,
// AP LooseEnds, D13, D41-A).
async function loadMeAndPreferences() {
  startWaits++;
  try {
    await loadMe();
    if (loginPromise) await loginPromise; // the first owner's card: created, skipped or turned to a sign-in
  } finally {
    startWaits--;
  }
  try {
    await loadPreferences();
  } finally {
    MassimoParts.signIn.settled(); // the first reads after a sign-in of the card settled: its secrets go (ui/src/signin/)
  }
}

// whoIs names who the page is signed in as, from /me: a user by name,
// else the mode (the server's token, an open server); "" before the
// server has said.
function whoIs(me) {
  if (!me) return "";
  return me.user ? "user:" + me.user.name : "mode:" + me.mode;
}

// holdPushes takes the settings pushes that wait for their second
// (schedulePreferences, MassimoParts.notify's save) as the sign-in of
// askLogin opens, so that none goes out under whoever signs in;
// resumePushes schedules them again, for the same user.
function holdPushes() {
  return { prefs: takeScheduled(), notify: MassimoParts.notify.takeSave() };
}

function resumePushes(held) {
  for (const key of held.prefs) schedulePreferences(key);
  if (held.notify) MassimoParts.notify.save(); // ui/src/notify/
}

// settleSignIn follows a sign-in of askLogin after a 401, before the
// reads that waited for it are sent again. Where it is of was, who was
// signed in, the session merely expired: the drafts stay and the pushes
// held go on. Where it is another's, or /me cannot be read, the page
// forgets the one who was, as a sign-out does (forgetSession), and drops
// the pushes held: the next user neither sees nor saves nor receives
// anything of the last. A sign-in that nobody was signed in before has
// nothing to forget. known is who is in as the sign-in read it already
// (the API key's way reads it with the key first, AP LooseEnds, D43), so
// that it is not read twice; undefined reads it.
async function settleSignIn(was, held, known) {
  if (!was) return resumePushes(held);
  let me = known === undefined ? null : known;
  if (known === undefined) {
    try {
      me = await apiRequest("GET", "api/v1/me");
    } catch (e) {
      // not known: forgotten, to be safe
    }
  }
  if (whoIs(me) === was) return resumePushes(held);
  await forgetSession(canSync());
}

// forgetSession forgets on the page all that was of whoever was signed
// in, for a sign-out (logout) and for another user's sign-in after a 401
// (settleSignIn) alike: who is in, for the plain code and the parts in
// one turn, with the parts' forms, drafts and answers still on their way
// (MassimoParts.signedOut), the settings that followed the user, where
// they did (forgetUserSettings, app.js), the lines saying a write was not
// sent (forgetNotSent, app.js), the House's panel, the stream, the
// Cubes' data and the toast; the page shows the tiles. All of it before
// the signed-out language's table is awaited, which comes last (UiParts
// D211), so that nothing of the user who left is drawn, sent or read
// meanwhile, nor shown (applyLanguage, after it, needs no page of the
// user's). Who is in is read again after it.
async function forgetSession(synced) {
  state.me = null;
  MassimoParts.signedOut(); // the session's number first, then every region's forgetting; who is in is null already (state.me is the parts' signal)
  const language = synced ? forgetUserSettings() : null; // applied at once, its language's table awaited below
  prefsSync.held = null; // the settings that held were the user's who left
  forgetNotSent();
  MassimoParts.rooms.closeHouse();
  hideToast(); // components.js: nothing a toast says of the user who left stays
  closeEvents();
  clearCubeData(); // app.js: nothing of this session behind the sign-in; the poll stops
  MassimoParts.pages.showView("tiles", "replace");
  await language;
}

// --- the login by fingerprint in the app (docs/app.md, "The login by fingerprint")

// appLogin is the bridge's login by fingerprint where the page runs in
// an app that has it (contract version 2), else null.
function appLogin() {
  const b = appBridge();
  return b && b.login && typeof b.login.status === "function" ? b.login : null;
}

// appConnect is the bridge's way back to the app's servers, else null.
function appConnect() {
  const b = appBridge();
  return b && typeof b.connect === "function" ? () => b.connect() : null;
}

// appServerSettings is the bridge's way to this server's settings on the
// app's own page, and appAppSettings to the app's settings there (☰'s two
// entries, contract version 5; AP MobileApp, D73-A), else null: the page
// passes no id and reads nothing of them.
function appServerSettings() {
  const b = appBridge();
  return b && typeof b.openServerSettings === "function" ? () => b.openServerSettings() : null;
}

function appAppSettings() {
  const b = appBridge();
  return b && typeof b.openAppSettings === "function" ? () => b.openAppSettings() : null;
}

// askLogin opens the sign-in dialog and resolves true once a session or
// a key is in, false when the dialog was dismissed. While it is open the
// page is signed out: the body's class signed-out hides all of it but the
// dialog, which holds the logo and the version, centred in the window
// (style.css). The dialog is the skeleton's; its card is a part
// (ui/src/signin/), handed the way to open on and the sign-in it
// answers: it empties its fields, chooses the way and gives the focus as
// it draws the opening, and each of its ways goes on to finishSignIn.
// With on "owner" it opens the card with the first owner's form, on a
// server without users (loadMe), where nothing was refused: nobody was
// signed in, no write was refused, so nothing is held, dropped or said
// not sent, and no provider is read (AP FirstOwner). A refusal while the
// first owner's form is shown means the server has users now: the card
// turns to the sign-in, the same opening (D15): it keeps the owner's ask,
// nobody before it and nothing held, and notSentNow is not reset at the
// owner's opening nor at the turn. That is right at a start and after a
// sign-out, where nothing of the page ran behind the card. An opening on
// a running page (the banner, ☰ and Users, by the user's word; a read of
// who is in) leaves the page standing still behind the card: the page's
// poll stopped and its stream closed (connectEvents opens none
// meanwhile, a restart's end included), both started again where the
// card is left with nobody signed in (AP FirstOwner, D35). What a region
// follows by itself goes on (the log's own poll of All Cubes, an update
// or a restart being followed): a read of theirs refused while the owner
// is being made waits for the sign-in the creation ends in and is sent
// again, so it does no harm. Its turn to the sign-in
// answers what the open page had under way as a refused request's sign-in
// does: who was in (the open server), the pushes held, the setpoints
// still waiting dropped, only the writes it answers said not sent. Each
// opening's ask is closed once: its end (done, finishSignIn) does nothing
// for an ask that was closed already, a late way's answer after the card
// was dismissed or opened anew.
function askLogin(on) {
  if (loginPromise) {
    if (on !== "owner" && MassimoParts.signIn.toSignIn()) {
      const ask = loginPromise.ask;
      if (ask.still) {
        notSentNow = false; // app.js: only the writes this sign-in answers
        ask.was = whoIs(state.me);
        ask.held = holdPushes();
        dropPendingSetpoints(); // rooms.js
      }
      renderLoginProviders();
    }
    return loginPromise;
  }
  const owner = on === "owner";
  const still = owner && !startWaits; // on a running page: the page stands still behind the card
  if (still) {
    if (state.pollTimer) clearInterval(state.pollTimer);
    state.pollTimer = null;
    closeEvents(); // app.js
  }
  if (!owner) notSentNow = false; // app.js: only the writes this sign-in answers
  const was = owner ? "" : whoIs(state.me); // a 401 while signed in: the drafts stay for this user alone (settleSignIn)
  const held = owner ? { prefs: [], notify: false } : holdPushes();
  if (!owner) dropPendingSetpoints(); // rooms.js
  let made = null;
  loginPromise = new Promise((resolve) => {
    const dlg = $("login-dialog");
    // the API key's form where a server without a store was seen, whose
    // only way in is MASSIMO_API_TOKEN (AP FirstOwner, D42)
    const way = owner ? "owner" : sawOpen ? "key" : "password";
    // a provider's refusal under the providers' buttons
    let said = "";
    if (!owner && oidcError) {
      const key = "login.oidcError." + oidcError;
      said = t(key) === key ? t("login.oidcError.failed") : t(key);
      oidcError = null;
    }
    const done = (ok) => {
      if (ask.closed) return; // closed already: a late answer closes nothing of a later opening
      ask.closed = true;
      dlg.onclose = null;
      if (dlg.open) dlg.close();
      MassimoParts.signIn.closed(ok); // ui/src/signin/: on a success the card's secrets go once the reads after it settle
      document.body.classList.remove("signed-out");
      loginPromise = null;
      resolve(ok);
    };
    // dismissed: the pushes held go as they would have (the dialog's close, and Skip for now); a running page stood
    // still behind the card goes on
    const dismiss = () => {
      if (ask.closed) return;
      resumePushes(ask.held);
      done(false);
      if (still) {
        startPolling(); // app.js
        connectEvents();
      }
    };
    const ask = { was, held, done, owner, still, dismiss, closed: false };
    made = ask;
    MassimoParts.signIn.open(way, said, ask); // the fields emptied, the way chosen, the lines cleared, the switch off
    if (!owner) renderLoginProviders();
    MassimoParts.signIn.readApp(); // the app's login by fingerprint, as login.status() answers, not awaited (ui/src/shared/app.ts)
    dlg.onclose = dismiss;
    document.body.classList.add("signed-out");
    // Not modal: the page behind is empty, and a modal dialog makes the
    // rest of the document inert, where a password manager draws its
    // choices, which then take no click. The card gives the focus to the
    // first field of the form shown in the drawing that follows.
    dlg.show();
  });
  loginPromise.ask = made; // the opening a later refusal turns
  return loginPromise;
}

// finishSignIn follows a sign-in of the card, by the password, a key or
// the fingerprint, once its token is set (ask, the sign-in askLogin
// opened): the sign-in settled (the same user goes on, another starts
// anew), the dialog closed, askLogin answered, and the page started where
// no start waits for who is in. The first owner's opening reads who is in
// before the card closes, so that the page shows the owner at once and
// not the open server for a moment, and Create the owner turns until
// then (AP FirstOwner); on a running page the page it was opened on is
// shown again, read anew under the owner (D37). An ask closed meanwhile (dismissed, or ended by
// another way) is left as it is: nothing is closed or started twice. me,
// where given, is who is in as the sign-in read it (settleSignIn). It
// reports whether it ended the ask.
async function finishSignIn(ask, me) {
  if (ask.closed) return false;
  await settleSignIn(ask.was, ask.held, me);
  if (ask.closed) return false;
  if (ask.owner) await loadMe();
  if (ask.closed) return false;
  ask.done(true);
  if (!startWaits) {
    afterLogin(!ask.owner);
    if (ask.still) MassimoParts.pages.showView(MassimoParts.pages.shown(), "replace"); // the page it was opened on, read anew
  }
  return true;
}

// --- the sign-in by OPAQUE (docs/api-v1.md, "The sign-in by OPAQUE") ----

// opaqueLoad is the vendored OPAQUE client (ui/vendor/opaque.js,
// @serenity-kit/opaque), imported when a sign-in or a password first
// needs it rather than with the page.
let opaqueLoad = null;

function opaqueModule() {
  if (!opaqueLoad) {
    opaqueLoad = import("./vendor/opaque.js").then(async (m) => {
      await m.ready;
      return m;
    });
    opaqueLoad.catch(() => {
      opaqueLoad = null;
    });
  }
  return opaqueLoad;
}

// keyStretching is the argon2id a record asks for, "m=19456,t=2,p=1" as
// the server sends it, in the client's terms.
function keyStretching(ksf) {
  const m = /^m=(\d+),t=(\d+),p=(\d+)$/.exec(ksf || "");
  if (!m) throw new Error("the server's key stretching " + JSON.stringify(ksf));
  return { "argon2id-custom": { memory: Number(m[1]), iterations: Number(m[2]), parallelism: Number(m[3]) } };
}

// NO_KE3 ends a sign-in whose answer the password did not open: a KE3 the
// server refuses as a wrong password, so that it lets the address go at
// once rather than after the start's minute.
const NO_KE3 = "A".repeat(86);

// signIn signs in, without a token and without the dialog's retry, and
// returns the session, or with the purpose "proof" a proof of the
// password; the answer's error is thrown. The password stays in the
// browser: the client runs OPAQUE and the server sees none of it. Where
// it does not open the server's answer and the server still takes the
// password sign-in, that follows, for a user still of version 1.
async function signIn(user, password, purpose) {
  const opaque = await opaqueModule();
  const start = opaque.client.startLogin({ password });
  const started = await apiRequest("POST", "api/v1/login/start", { user, ke1: start.startLoginRequest, purpose }, { auth: false });
  const done = opaque.client.finishLogin({
    clientLoginState: start.clientLoginState,
    loginResponse: started.ke2,
    password,
    keyStretching: keyStretching(started.ksf),
  });
  if (done) return apiRequest("POST", "api/v1/login/finish", { login: started.login, ke3: done.finishLoginRequest }, { auth: false });
  if (!started.v1 || purpose) {
    return apiRequest("POST", "api/v1/login/finish", { login: started.login, ke3: NO_KE3 }, { auth: false });
  }
  // The password sign-in takes the start over, so that the attempt counts
  // once; a user still of version 1 is upgraded at once.
  const session = await apiRequest("POST", "api/v1/login", { user, password, login: started.login }, { auth: false });
  if (session.upgrade) await upgradePassword(session, password);
  return session;
}

// upgradePassword moves a user of version 1 to version 2 right after its
// password sign-in: the password registered by OPAQUE in the browser and
// set with the one-time proof the sign-in answered, so that it crosses
// no more. A failure leaves the user of version 1, and the next sign-in
// tries again.
async function upgradePassword(session, password) {
  const name = session.user.name;
  try {
    const body = await registerPassword(name, password, session.token);
    body.proof = session.upgrade;
    await apiRequest("PUT", "api/v1/users/" + encodeURIComponent(name), body, { token: session.token });
    session.user.auth_version = 2;
  } catch (e) {
    // still of version 1
  }
}

// registerPassword makes a password of version 2 for a name, the password
// kept in the browser: the server answers the client's registration
// request, and the client's record comes back as the fields that set it,
// {record, ksf}, for POST /api/v1/users or PUT /api/v1/users/{name}. A
// token given is the one asked with, else the page's.
async function registerPassword(name, password, token) {
  const opaque = await opaqueModule();
  const reg = opaque.client.startRegistration({ password });
  const path = "api/v1/users/" + encodeURIComponent(name) + "/register";
  const request = { request: reg.registrationRequest };
  const answer = token ? await apiRequest("POST", path, request, { token }) : await api("POST", path, request);
  const done = opaque.client.finishRegistration({
    clientRegistrationState: reg.clientRegistrationState,
    registrationResponse: answer.response,
    password,
    keyStretching: keyStretching(answer.ksf),
  });
  return { record: done.registrationRecord, ksf: answer.ksf };
}

// currentPassword proves the current password for one's own change: for a
// user of version 2 a proof from a sign-in by OPAQUE, which keeps it in
// the browser, for one still of version 1 the password itself.
async function currentPassword(user, password) {
  if (user.auth_version === 2) return { proof: (await signIn(user.name, password, "proof")).proof };
  return { current_password: password };
}

// afterLogin reads who is in, unless read is false (the first owner's
// card read it before it closed), and starts the poll, the stream and the
// data again, for a sign-in that no start waited for: a session that
// ended while the page ran, a key pasted, the first owner. A write that
// was not sent while the sign-in was open (notSent, app.js) and said so
// on the page's message line, which the reload of the data clears, is
// said there again, where the reload left the line empty. Once who is in
// and the settings are read, answered or failed, the card empties its
// secret fields (MassimoParts.signIn.settled, AP LooseEnds, D13, D41-A).
function afterLogin(read = true) {
  const said = notSentSaid();
  (read ? loadMe() : Promise.resolve())
    .then(loadPreferences)
    .finally(() => MassimoParts.signIn.settled()) // the first reads after the sign-in settled: the card's secrets go (ui/src/signin/)
    .then(() => {
      startPolling(); // app.js: stopped where another user's sign-in forgot the session
      refresh().then((fresh) => {
        if (!fresh) return; // answered after the session ended: the next sign-in starts the data again
        store("massimo.serial", state.serial); // as the page's start does, which another user's sign-in may have cut short
        if (said && $("message").classList.contains("hidden")) showMessage(said, false);
        return connectEvents();
      });
    });
}

// notSentSaid is the page's message line's text where it says that a
// write was not sent during this sign-in (notSentNow, app.js), else "".
function notSentSaid() {
  const msg = $("message");
  const text = msg.classList.contains("hidden") ? "" : msg.textContent;
  const said = notSentNow && saysNotSent(text) ? text : "";
  notSentNow = false;
  return said;
}

// logout ends the session (an API key is only forgotten), clears what
// the page loaded of the Cubes and asks who is there again; where a
// settings form has changes not saved, which it drops, it asks first.
// The settings that followed the user go with it: a
// change still waiting is sent under the user's token first, then the
// browser forgets the user's values and schemes and keeps what is kept
// for this device only, so that the next to sign in neither sees them
// nor hands them to its own account, as loadPreferences would hand over
// what a server lacks (forgetSession). Neither request asks for a
// sign-in.
async function logout() {
  // a settings form with changes not saved: the page's dialog lists them and asks first (ui/src/leaving/)
  if (!(await MassimoParts.leaving.signOut())) return;
  const me = state.me;
  const synced = canSync();
  for (const key of takeScheduled()) {
    await apiRequest("PUT", "api/v1/me/preferences/" + key, preferenceBody(key)).catch(() => {});
  }
  await MassimoParts.notify.sendNow(); // ui/src/notify/: a notification rule changed in the last second, the same way (D205)
  if (me && me.mode === "users" && me.token && me.token.kind === "session") {
    try {
      await apiRequest("POST", "api/v1/logout");
    } catch (e) {
      // the session may be gone already
    }
  }
  setToken(null);
  await forgetSession(synced);
  await loadMeAndPreferences();
  startPolling();
  refresh().then((fresh) => fresh && connectEvents());
}

// --- the user's settings on the server ---------------------------------

// canSync reports whether the settings follow the signed-in user: a user
// with the capability preferences. A guest, an open server and the API
// token keep them in the browser only.
function canSync() {
  const me = state.me;
  return !!(me && me.mode === "users" && me.user && hasCap("preferences"));
}

// prefsSync holds the preferences waiting for their second to be sent
// (timer, keys) and held, the user's settings that hold: as the server
// took them last or gave them (pushPreferences, loadPreferences), else as
// the browser kept them before the first change since (noteHeld); null
// where the page knows none. A change whose push met a 401 goes back to
// them (takeBack).
const prefsSync = { timer: null, keys: new Set(), held: null };

// PREFS_DELAY is how long a change waits for the next before it goes to
// the server, so that a poll interval typed digit by digit is one write.
const PREFS_DELAY = 1000;

// noteHeld keeps the user's settings as the browser has them before a
// change as the ones that hold, where the page knows none (prefsSync):
// those of a server that has not taken any yet, which a sign-in would
// hand the browser's (loadPreferences). setSetting of app.js calls it.
function noteHeld() {
  if (!prefsSync.held && canSync()) prefsSync.held = storedSettings(); // app.js
}

// takeBack takes back a change whose push met a 401, which the page said
// was not sent (AP SteadyTests v1.1, D14-A): each setting the push sent
// otherwise than the settings that hold (prefsSync.held) goes back to
// theirs, where the browser still keeps what was sent, and the page shows
// it. So that "not sent" stays true, the page does not send the change
// later by itself: not with a later push, nor with the browser's settings
// that a sign-in hands a server without the user's; the user makes it
// again. A setting changed again since keeps its new value, which a push
// of its own sends.
function takeBack(sent) {
  const held = prefsSync.held;
  if (!held) return Promise.resolve();
  const now = storedSettings(); // app.js
  let back = false;
  for (const k of SETTING_KEYS) {
    if (sent[k] !== held[k] && now[k] === sent[k]) {
      storeUserSetting(k, held[k]);
      back = true;
    }
  }
  return back ? applyStoredSettings() : Promise.resolve();
}

// schedulePreferences sends a preference (ui or schemes) to the server
// shortly, when the settings follow the user.
function schedulePreferences(key) {
  if (!canSync()) return;
  prefsSync.keys.add(key);
  if (prefsSync.timer) clearTimeout(prefsSync.timer);
  prefsSync.timer = setTimeout(pushPreferences, PREFS_DELAY);
}

// takeScheduled ends the wait of schedulePreferences and returns the
// preferences it held.
function takeScheduled() {
  if (prefsSync.timer) clearTimeout(prefsSync.timer);
  prefsSync.timer = null;
  const keys = [...prefsSync.keys];
  prefsSync.keys.clear();
  return keys;
}

// preferenceBody is what a preference sends: ui is the user's values as
// the browser keeps them, without this device's, schemes the user's
// schemes.
function preferenceBody(key) {
  return key === "ui" ? storedSettings() : schemesDocument(); // app.js, theme.js
}

// pushPreferences sends the preferences scheduled. The settings sent are
// then the ones that hold; a 401 takes their change back (takeBack).
async function pushPreferences() {
  for (const key of takeScheduled()) {
    const n = MassimoParts.session();
    const body = preferenceBody(key);
    try {
      await api("PUT", "api/v1/me/preferences/" + key, body);
      if (key === "ui" && MassimoParts.stillSession(n)) prefsSync.held = body;
    } catch (e) {
      if (!MassimoParts.stillSession(n)) return;
      if (key === "ui" && e.status === 401) await takeBack(body); // said not sent (app.js notSent), and not sent later
      showMessage(t("prefs.saveFailed", { error: e.message }), false);
    }
  }
}

// flushPreferences sends what is scheduled at once, as the page is left
// or hidden: a change made in the last second before a tab is closed
// would otherwise stay in this browser, and the next sign-in would take
// the server's older value over it. keepalive lets the request outlive
// the page.
function flushPreferences() {
  if (!prefsSync.timer) return;
  for (const key of takeScheduled()) {
    apiRequest("PUT", "api/v1/me/preferences/" + key, preferenceBody(key), { keepalive: true }).catch(() => {});
  }
}

// loadPreferences takes the signed-in user's settings from the server:
// the schemes first, the server's list as it is, so that a chosen scheme
// of the user's resolves and a deleted one stays deleted, then ui,
// applied where this device does not keep its own. What the server lacks
// it learns from the browser, so that the first sign-in keeps the
// settings and the schemes the browser had; a change said not sent is no
// longer among them (takeBack). The server's ui is then the settings
// that hold (prefsSync.held).
async function loadPreferences() {
  if (!canSync()) return;
  const n = MassimoParts.session();
  let prefs;
  try {
    prefs = (await api("GET", "api/v1/me/preferences")) || {};
  } catch (e) {
    return;
  }
  if (!MassimoParts.stillSession(n)) return; // the settings of the user before
  if (prefs.schemes) {
    takeSchemes(prefs.schemes); // theme.js
  } else if (themeState.custom.length) {
    schedulePreferences("schemes");
  }
  const ui = prefs.ui;
  if (ui && typeof ui === "object") {
    for (const k of SETTING_KEYS) {
      const v = validSetting(k, ui[k]);
      if (v !== undefined) storeUserSetting(k, v);
    }
    prefsSync.held = storedSettings(); // app.js
  } else {
    schedulePreferences("ui");
  }
  await applyStoredSettings();
}

function initLogin() {
  window.addEventListener("pagehide", flushPreferences);
  document.addEventListener("visibilitychange", () => {
    if (document.visibilityState === "hidden") flushPreferences();
  });
}

document.addEventListener("DOMContentLoaded", initLogin);
