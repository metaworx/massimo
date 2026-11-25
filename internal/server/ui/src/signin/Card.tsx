// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/signin/Card.tsx
 *
 * The sign-in's card, drawn once into the skeleton's #login-dialog as
 * parts.js loads (AP UiParts, Block 14; partials/login.html and askLogin
 * of login.js before), the dialog itself the skeleton's: askLogin shows
 * it, not modal, and closes it. The logo and the title, in the app the
 * server it signs in to under it (Server.tsx); a button per enabled
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
 * sign-in for done first; never at a refusal or a dismissal. Each way signs in as askLogin's handlers did, then
 * finishSignIn of login.js goes on as they went on. The card is never
 * drawn anew: its buttons, their busy states and its fields stay through
 * every draw, the app's status, the providers, the language, a 401's
 * askLogin, which hands out the same opening. The server's 401 is said
 * as a wrong name or password in the page's language, a sign-in that
 * took too long included, which a new try passes; any other refusal (too
 * many attempts, a server not ready) with the server's words; Sign in
 * turns while a key signs in as while a password does, so that a second
 * Enter meanwhile signs in no second time (AP LooseEnds, Block 2).
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
import { ask, askFocus, card, chooseWay, keepChosen, type Way } from "./store";

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
  const pw = useBusy();
  const kb = useBusy();
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
    if (pw.busy) return; // a second Enter while it signs in
    const a = ask();
    const user = (userRef.current?.value || "").trim();
    const password = passRef.current?.value || "";
    if (!user || !password) return fail(t("login.missing"));
    card.said = null;
    pw.run(async () => {
      let session: { token: string };
      try {
        session = await signIn(user, password);
      } catch (e) {
        const err = e as Error & { status?: number };
        // the server's 401 in the page's language; any other refusal (too many attempts, a server not ready) says more
        // in the server's own words than a key would
        fail(err.status === 401 ? t("login.wrong") : t("login.failed", { error: err.message }));
        askFocus("password");
        return;
      }
      const put = setToken(session.token);
      const login = appLogin();
      if (keep && offer(app.value) && login) {
        await put; // in the app the session kept by the bridge first, which login.keep reads on another thread (D68-A)
        if (!(await keepLogin(login))) return;
      }
      if (a) await finishSignIn(a);
    });
  };
  const submitKey = (ev: Event) => {
    ev.preventDefault();
    if (kb.busy) return; // a second Enter while it signs in
    const a = ask();
    const key = (keyRef.current?.value || "").trim();
    if (!key) return;
    kb.run(async () => {
      await setToken(key); // in the app, the key kept by the bridge before the sign-in's reads go out with it
      if (a) await finishSignIn(a);
    });
  };
  // the login by fingerprint: a session from the key the app keeps
  const fingerprint = () => fp.run(async () => {
    const a = ask();
    const login = appLogin();
    if (!login) return;
    card.said = null;
    let token: string | null;
    try {
      token = await login.fingerprint();
    } catch (e) {
      fail(t("login.fingerprintFailed", { error: (e as Error).message }));
      readApp(); // a key the server refused is gone
      return;
    }
    if (!token) return; // the prompt dismissed: the card stays
    setToken(token);
    if (a) await finishSignIn(a);
  });
  const fpLabel = fpShown && st ? t("connect.loginAs", { name: st.user || "" }) : t("login.fingerprint");
  const v = version.value;
  const keepFailed = card.keepFailed;
  return (
    <>
      <BrandName />
      <h3 id="login-title" class="login-title">{t(!owner ? "login.title" : card.ownerStep === "keep" ? "keepOpen.title" : "setup.title")}</h3>
      <ServerLine />
      <div id="login-providers" class={"login-providers" + hid(owner || (!card.providers.length && !card.providersSaid))}>
        <div id="login-provider-buttons" class="login-provider-buttons">
          {card.providers.map((p) => (
            <BusyButton key={p.key} label={t("login.with", { name: p.name })} data={{ provider: p.key }}
              run={() => startProvider(p.key).catch((e: Error) => { card.providersSaid = { text: t("login.failed", { error: e.message }), ok: false }; })} />
          ))}
        </div>
        <ResultLine id="login-providers-error" cls="small" role="alert" res={card.providersSaid} />
      </div>
      <div id="login-row" class="login-row">
        <div id="login-ways" class={"login-ways" + hid(card.waysOff || owner)} role="group" aria-label={t("login.ways")}>
          <button type="button" id="login-way-fp" {...busyProps("login-way login-way-fp" + hid(!fpShown), fp.busy)} aria-label={fpLabel} title={fpLabel}
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
            <button type="button" id="login-cancel" class={"form-cancel" + hid(!servers)} onClick={toServers}>{t("action.cancel")}</button>
            <button type="submit" {...busyProps("primary", pw.busy)}>{t("login.submit")}</button>
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
          <div class="actions">
            <button type="button" id="login-key-cancel" class={"form-cancel" + hid(!servers)} onClick={toServers}>{t("action.cancel")}</button>
            <button type="submit" {...busyProps("primary", kb.busy)}>{t("login.submit")}</button>
          </div>
        </form>
      </div>
      <Owner />
      <p id="login-version" class="login-version mono small muted">{v ? splitVersion(v).short : ""}</p>
    </>
  );
}
