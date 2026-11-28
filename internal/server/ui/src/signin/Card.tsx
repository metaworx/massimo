// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/signin/Card.tsx
 *
 * The sign-in's card, drawn once into the skeleton's #login-dialog as
 * parts.js loads (AP UiParts, Block 14; partials/login.html and askLogin
 * of login.js before), the dialog itself the skeleton's: askLogin shows
 * it, not modal, and closes it. The logo and the title, in the app the
 * server it signs in to between them (Server.tsx); a button per enabled
 * provider, with its refusal under them; the row of the ways to
 * sign in, the fingerprint first in the app where a login is kept, then
 * the password, chosen at first, and an API key, and the card's ☰ at its
 * end with the version, the language, the mode and in the app Select
 * server; the notice where the address takes no kept key; a form for
 * each way, in the app with Cancel, the password's field with its eye
 * (shared/password.tsx), hidden again at each opening and after a
 * success; the box that says the login by
 * fingerprint was not kept, the two forms and the box in one place of one
 * height (style.css .login-forms), so that a switch of the way moves
 * nothing of the card (UiParts D224); on a server without users, in
 * their place and the ways', the first owner's form (Owner.tsx), the card
 * at its own height, its title saying that there are no users yet and no
 * provider read (AP FirstOwner, D23); the version at the foot. The forms
 * are real forms with named fields, for a password manager, and their fields are
 * not controlled (UiParts D210): a draw never sets what they hold, they
 * are read at the submit and emptied at each opening, as askLogin did,
 * and the password and the key once the card has closed on a sign-in and
 * the page's first reads after it have settled (store.ts settled; AP
 * LooseEnds, D13, D41-A), so that a password manager has taken the
 * sign-in for done first; a dismissal empties nothing, nor does a
 * refused password, kept for the retry, where a refused API key goes
 * from its field (below). Each way signs in as askLogin's handlers did, then
 * finishSignIn of login.js goes on as they went on. The card is never
 * drawn anew: its buttons, their busy states and its fields stay through
 * every draw, the app's status, the providers, the language, a 401's
 * askLogin, which hands out the same opening. The server's 401 is said
 * as a wrong name or password in the page's language, a sign-in that
 * took too long included, which a new try passes; any other refusal (too
 * many attempts, a server not ready) with the server's words; Sign in
 * turns while a key signs in as while a password does, so that a second
 * Enter meanwhile signs in no second time (AP LooseEnds, Block 2). A key
 * is read with first and becomes the page's token only once the server
 * has taken it: a refused one is said on the key's way, under its field,
 * the field emptied and in focus, and the page's token and the app's
 * bridge stay as they were (AP LooseEnds, D43). An answer of any way, the
 * password's, the key's read, the fingerprint's and a provider's start,
 * that comes once its opening has ended (another way signed in meanwhile,
 * the card closed or opened anew, a sign-out) writes no line, takes no
 * focus and sets no token (store.ts opening; AP LooseEnds, D44), and
 * neither does one that comes once another way of the same opening has
 * set its token: a session that came late is ended at the server, and a
 * provider's page is not opened (AP AccessEnds, D2). The password's and
 * the key's Sign in and a provider's button turn for a request of their
 * opening alone, ready again in a card opened anew. The login by
 * fingerprint, whose session the app keeps as setToken would, ending the
 * one it held, keeps the card working from its press until the card has
 * closed, as the first owner's creation does: the other ways' Sign in,
 * the providers' buttons, Cancel and Select server are off, and it starts
 * only while no other way of the opening runs; one that answers once the
 * session was forgotten meanwhile has its session ended, and kept by
 * nobody where the page holds no token, as after a sign-out (on a page
 * that still holds a session, which only a script's close of the card
 * leaves, the app has ended that session as it kept the fingerprint's,
 * and keeps the fingerprint's, ended). The first owner's form needs no
 * check of its opening (Owner.tsx): every way out of the card is off
 * while it creates (AP AccessEnds, D1).
 */

import { useLayoutEffect, useRef, useState } from "preact/hooks";
import { app, readApp, toServers } from "../shared/app";
import { ResultLine } from "../shared/badge";
import { BrandName } from "../shared/brand";
import { hid } from "../shared/cls";
import { Select } from "../shared/field";
import { Icon } from "../shared/icon";
import { languageOptions, useLang } from "../shared/lang";
import { Dropdown, MenuItem } from "../shared/menu";
import { ModeSelect } from "../shared/mode";
import { PasswordInput } from "../shared/password";
import { BusyButton, busyProps, useBusy } from "../shared/question";
import { VersionRow, version } from "../shared/server";
import { keepLogin, kept, offer, plainHere } from "./app";
import { Owner } from "./Owner";
import { ServerLine } from "./Server";
import { askFocus, card, chooseWay, keepChosen, opening, type Way } from "./store";

/** The ways to sign in by a form, their data-way and their name's key, in the row's order after the fingerprint. */
const WAYS: [Way, string][] = [["password", "login.password"], ["key", "login.token"]];

/** Menu is the card's ☰: the version, the language and the mode before anyone signs in, as My settings sets them, and in the app Select server last, off while a request of the card is under way. */
function Menu() {
  useLang(); // the language and the mode chosen are state.settings', signals (shared/data.ts)
  const servers = !!appConnect();
  return (
    <Dropdown id="login-menu" cls="nav-menu login-menu" summaryCls="nav-btn" label={t("login.menu")} summary={<Icon symbol="icon-menu" cls="ni" />}>
      <VersionRow id="login-menu-version" />
      <hr class="menu-sep" />
      <Select label={t("settings.language")} id="login-lang" wide options={languageOptions()} value={state.settings.lang} onChange={(v) => {
        setSetting("lang", validSetting("lang", v) || "auto");
        changeLanguage(state.settings.lang).then(applyLanguage);
      }} />
      <ModeSelect id="login-mode" label={t("appearance.title")} wide />
      <hr id="login-menu-servers-sep" class={"menu-sep" + hid(!servers)} />
      <MenuItem id="login-menu-servers" cls={servers ? undefined : "hidden"} icon={<Icon symbol="icon-servers" cls="ni" />} label={t("app.selectServer")}
        disabled={card.working} onClick={toServers} />
    </Dropdown>
  );
}

/** Card is the sign-in's card, the dialog's content. */
export function Card() {
  useLang();
  const opened = card.opened;
  const succeeded = card.succeeded;
  const focus = card.focus;
  const way = card.way;
  const owner = card.owner;
  const st = app.value;
  const fpShown = kept(st);
  const offered = offer(st);
  const servers = !!appConnect();
  const [keep, setKeep] = useState(false);
  // the password's and the key's Sign in turn for a request of this opening alone: in a card opened anew they are
  // ready again, an earlier request's answer doing nothing there; the fingerprint's, one system prompt at a time, for
  // the card
  const pw = useBusy(opened);
  const kb = useBusy(opened);
  const fp = useBusy();
  const userRef = useRef<HTMLInputElement>(null);
  const passRef = useRef<HTMLInputElement>(null);
  const keyRef = useRef<HTMLInputElement>(null);
  const retryRef = useRef<HTMLButtonElement>(null);
  // each opening empties the fields and turns the switch off, as askLogin did; a closing empties nothing but after a
  // success, below (AP LooseEnds, D13, which changes UiParts D210's closing that emptied nothing)
  // biome-ignore lint/correctness/useExhaustiveDependencies: opened is read from the card's store, which Biome takes for a stable import; the effect is for each opening, so it is listed.
  useLayoutEffect(() => {
    if (!opened) return;
    for (const r of [userRef, passRef, keyRef]) if (r.current) r.current.value = "";
    setKeep(false);
  }, [opened]);
  // a sign-in done, the card closed and the page's first reads after it settled (store.ts settled): the password and
  // the key emptied (AP LooseEnds, D13, D41-A), the password's eye hidden again by its reset; the name stays until the
  // next opening. A refusal, a dismissal and a card opened again meanwhile are left as they are: no timer empties a
  // field the user may be typing in
  // biome-ignore lint/correctness/useExhaustiveDependencies: succeeded is read from the card's store, which Biome takes for a stable import; the effect is for each success, so it is listed.
  useLayoutEffect(() => {
    if (!succeeded) return;
    for (const r of [passRef, keyRef]) if (r.current) r.current.value = "";
  }, [succeeded]);
  // the focus asked for, given in the drawing that shows its place: the chosen way's first field at an opening and at a
  // way's click, the password after a refusal, Try again where the login by fingerprint was not kept; the first owner's
  // name is Owner's
  // biome-ignore lint/correctness/useExhaustiveDependencies: focus is read from the card's store, which Biome takes for a stable import; the effect is for each focus asked, so it is listed.
  useLayoutEffect(() => {
    if (!focus.n) return;
    if (focus.on === "way") wayFocus();
    else if (focus.on === "password") passRef.current?.focus();
    else if (focus.on === "key") keyRef.current?.focus();
    else if (focus.on === "retry") retryRef.current?.focus();
  }, [focus]);
  // the one place that puts the focus into the chosen way's first field, at an opening and at a way's click; none on a
  // touch screen, where it would bring the soft keyboard up by itself and move the card (UiParts D224): the keyboard
  // comes when a field is tapped
  function wayFocus(): void {
    if (matchMedia("(pointer: coarse)").matches) return;
    (card.way === "key" ? keyRef : userRef).current?.focus();
  }
  const fail = (text: string) => {
    card.said = { text, ok: false };
  };
  const submitPassword = (ev: Event) => {
    ev.preventDefault();
    if (pw.busy || card.working) return; // a second Enter while it signs in; nothing while the fingerprint's prompt is open
    // the opening the password was typed in: an answer once it has ended writes nothing and sets no token (AP
    // LooseEnds, D44)
    const { a, still, take } = opening();
    const user = (userRef.current?.value || "").trim();
    const password = passRef.current?.value || "";
    if (!user || !password) return fail(t("login.missing"));
    card.said = null;
    pw.run(async () => {
      let session: { token: string };
      try {
        session = await signIn(user, password);
      } catch (e) {
        if (!still()) return;
        const err = e as Error & { status?: number };
        // the server's 401 in the page's language; any other refusal (too many attempts, a server not ready) says more
        // in the server's own words than a key would
        fail(err.status === 401 ? t("login.wrong") : t("login.failed", { error: err.message }));
        askFocus("password");
        return;
      }
      if (!still() || !a) {
        // a session that came late is no token of the page's, which another way set meanwhile: ended at the server,
        // so that no live session is left that nobody holds (AP AccessEnds, D2)
        endSession(session.token);
        return;
      }
      take(); // the opening this sign-in's: another way's answer from now on is late
      const put = setToken(session.token);
      const login = appLogin();
      if (keep && offer(app.value) && login) {
        await put; // in the app the session kept by the bridge first, which login.keep reads on another thread (D68-A)
        if (!(await keepLogin(login))) return;
      }
      await finishSignIn(a); // an opening that ended while the login was kept: finishSignIn does nothing for it
    });
  };
  const submitKey = (ev: Event) => {
    ev.preventDefault();
    if (kb.busy || card.working) return; // a second Enter while it signs in; nothing while the fingerprint's prompt is open
    // the opening the key was typed in still the card's and open, in the same session: an answer after another way
    // signed in, after the card closed or opened anew, or after a sign-out, makes no token and writes nothing
    const { a, still, take } = opening();
    const key = (keyRef.current?.value || "").trim();
    if (!key) return;
    card.keySaid = null;
    kb.run(async () => {
      // the key read with first, as who it is, before it becomes the page's token: a key the server refuses replaces
      // no session of the page's, and no bridge keeps it (AP LooseEnds, D43)
      let me: unknown;
      try {
        me = await apiRequest("GET", "api/v1/me", undefined, { token: key });
      } catch (e) {
        if (!still()) return;
        const err = e as Error & { status?: number };
        // a key the server does not know (401) in the page's language; a key it refuses for a reason (403, such as an
        // integration disabled), a lockout, a server not reached with the server's or the browser's words, as the
        // password's way says them; a refused key, a secret still, goes from its field
        card.keySaid = { text: err.status === 401 ? t("login.keyRefused") : t("login.failed", { error: err.message }), ok: false };
        if ((err.status === 401 || err.status === 403) && keyRef.current) keyRef.current.value = "";
        askFocus("key");
        return;
      }
      if (!still() || !a) return;
      take();
      await setToken(key); // in the app, the key kept by the bridge before the sign-in's reads go out with it
      await finishSignIn(a, me); // who is in as the key read it: settleSignIn reads it no second time
    });
  };
  // the login by fingerprint: a session from the key the app keeps, which the app keeps as setToken would and so ends
  // the session it held. While the system's prompt and the session's request run the card is working, as while the
  // first owner is created (AP AccessEnds, D1): no other way signs in meanwhile and nothing leaves the card, and it
  // starts only while no other way's request of this opening runs, so that no session of another way is made beside it
  // for the app to end. Its answer acts only while the opening it was asked in is still the card's; one that comes
  // once the session was forgotten meanwhile, by a sign-out, is ended at the server and the app keeps none of it (AP
  // AccessEnds, D2)
  const fingerprint = () => {
    if (pw.busy || kb.busy) return;
    fp.run(async () => {
      const { a, still, take } = opening();
      const login = appLogin();
      if (!login) return;
      card.said = null;
      card.working = true;
      try {
        let token: string | null;
        try {
          token = await login.fingerprint();
        } catch (e) {
          if (still()) fail(t("login.fingerprintFailed", { error: (e as Error).message }));
          readApp(); // a key the server refused is gone, whatever the opening
          return;
        }
        if (!token) return; // the prompt dismissed: the card stays
        if (!still() || !a) {
          endSession(token, true);
          return;
        }
        take();
        setToken(token);
        await finishSignIn(a); // working on until the card has closed, as Create the owner is
      } finally {
        card.working = false;
      }
    });
  };
  // a provider's button: its start answers the provider's address, which the page leaves for only while the opening
  // the button was pressed in is still the card's; a refusal after it has ended says nothing. It takes no opening: it
  // sets no token, a page it leaves ends its opening, and one a navigation leaves standing (stopped, answered with no
  // content) keeps every way of it
  const provider = (key: string) => {
    const { still } = opening();
    return startProvider(key).then(
      (url) => {
        if (still()) location.assign(url);
      },
      (e: Error) => {
        if (still()) card.providersSaid = { text: t("login.failed", { error: e.message }), ok: false };
      },
    );
  };
  const fpLabel = fpShown && st ? t("connect.loginAs", { name: st.user || "" }) : t("login.fingerprint");
  const v = version.value;
  const keepFailed = card.keepFailed;
  return (
    <>
      <BrandName />
      <ServerLine />
      <h3 id="login-title" class="login-title">{t(!owner ? "login.title" : card.ownerStep === "keep" ? "keepOpen.title" : card.ownerStep === "backup" ? "newBox.title" : "setup.title")}</h3>
      <div id="login-providers" class={"login-providers" + hid(owner || (!card.providers.length && !card.providersSaid))}>
        <div id="login-provider-buttons" class="login-provider-buttons">
          {card.providers.map((p) => (
            <BusyButton key={p.key} label={t("login.with", { name: p.name })} data={{ provider: p.key }} epoch={opened} off={card.working}
              run={() => provider(p.key)} />
          ))}
        </div>
        <ResultLine id="login-providers-error" cls="small" role="alert" res={card.providersSaid} />
      </div>
      <div id="login-row" class="login-row">
        <div id="login-ways" class={"login-ways" + hid(card.waysOff || owner)} role="group" aria-label={t("login.ways")}>
          <button type="button" id="login-way-fp" {...busyProps("login-way login-way-fp" + hid(!fpShown), fp.busy, pw.busy || kb.busy)} aria-label={fpLabel} title={fpLabel}
            onClick={fingerprint}><Icon symbol="icon-fingerprint" cls="ni" /></button>
          {WAYS.map(([w, key]) => (
            <button key={w} type="button" class="login-way" data-way={w} aria-pressed={way === w ? "true" : "false"} onClick={() => chooseWay(w)}>{t(key)}</button>
          ))}
        </div>
        <Menu />
      </div>
      <p id="login-fp-plain" class={"notice small" + hid(!plainHere(st) || owner)} role="note">{t("login.fingerprintPlain")}</p>
      <div class={"login-forms" + hid(owner)}>
        <form id="login-form" method="post" class={hid(way !== "password" || !!keepFailed).trim() || undefined} onSubmit={submitPassword}>
          <p class="muted">{t("login.text")}</p>
          <p id="login-tls" class={"notice small" + hid(!card.selfSigned)} role="note">{t("header.selfSigned")}</p>
          <label><span>{t("login.user")}</span> <input ref={userRef} type="text" id="login-user" name="username" autocomplete="username" autocapitalize="none" spellcheck={false} /></label>
          <div class="login-field">
            <label for="login-password">{t("login.password")}</label>
            <PasswordInput id="login-password" name="password" autocomplete="current-password" inputRef={passRef} reset={opened + succeeded} />
          </div>
          <label id="login-keep-row" class={"switch login-keep" + hid(!offered)}>
            <input type="checkbox" id="login-keep" checked={keep && offered} onChange={(ev) => setKeep(ev.currentTarget.checked)} />
            <span class="slider" aria-hidden="true" />
            <span class="switch-label">{t("login.keepFingerprint")}</span>
          </label>
          <ResultLine id="login-error" cls="small" role="alert" res={card.said} />
          <div class="actions">
            <button type="button" id="login-cancel" class={"form-cancel" + hid(!servers)} disabled={card.working} onClick={toServers}>{t("action.cancel")}</button>
            <button type="submit" {...busyProps("primary", pw.busy, card.working)}>{t("login.submit")}</button>
          </div>
        </form>
        <div id="login-keep-failed" class={"login-keep-failed" + hid(!keepFailed)}>
          <p id="login-keep-failed-text" class="small" role="alert">{keepFailed || ""}</p>
          <div class="actions">
            <button type="button" id="login-keep-skip" onClick={() => keepChosen(false)}>{t("login.keepSkip")}</button>
            <button ref={retryRef} type="button" id="login-keep-retry" class="primary" onClick={() => keepChosen(true)}>{t("login.keepRetry")}</button>
          </div>
        </div>
        <form id="login-key-form" class={hid(way !== "key").trim() || undefined} onSubmit={submitKey}>
          <label><span>{t("login.token")}</span> <input ref={keyRef} type="password" id="login-token" name="api-key" autocomplete="off" /></label>
          <ResultLine id="login-key-error" cls="small" role="alert" res={card.keySaid} />
          <div class="actions">
            <button type="button" id="login-key-cancel" class={"form-cancel" + hid(!servers)} disabled={card.working} onClick={toServers}>{t("action.cancel")}</button>
            <button type="submit" {...busyProps("primary", kb.busy, card.working)}>{t("login.submit")}</button>
          </div>
        </form>
      </div>
      <Owner />
      <p id="login-version" class="login-version mono small muted">{v ? splitVersion(v).short : ""}</p>
    </>
  );
}
