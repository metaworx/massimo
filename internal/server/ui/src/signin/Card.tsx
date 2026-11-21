// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/signin/Card.tsx
 *
 * The sign-in's card, drawn once into the skeleton's #login-dialog as
 * parts.js loads (AP UiParts, Block 14; partials/login.html and askLogin
 * of login.js before), the dialog itself the skeleton's: askLogin shows
 * it, not modal, and closes it. The logo and the title; a button per
 * enabled provider, with its refusal under them; the row of the ways to
 * sign in, the fingerprint first in the app where a login is kept, then
 * the password, chosen at first, and an API key, and the card's ☰ at its
 * end with the version, the language, the mode and in the app Select
 * server; the notice where the address takes no kept key; a form for
 * each way, in the app with Cancel; the box that says the login by
 * fingerprint was not kept, the two forms and the box in one place of one
 * height (style.css .login-forms), so that a switch of the way moves
 * nothing of the card (UiParts D224); the version at the foot. The forms
 * are real forms with named fields, for a password manager, and their fields are
 * not controlled (UiParts D210): a draw never sets what they hold, they
 * are read at the submit and emptied at each opening alone, as
 * askLogin did. Each way signs in as askLogin's handlers did, then
 * finishSignIn of login.js goes on as they went on. The card is never
 * drawn anew: its buttons, their busy states and its fields stay through
 * every draw, the app's status, the providers, the language, a 401's
 * askLogin, which hands out the same opening.
 */

import { useLayoutEffect, useRef, useState } from "preact/hooks";
import { toServers } from "../shared/app";
import { ResultLine } from "../shared/badge";
import { BrandName } from "../shared/brand";
import { hid } from "../shared/cls";
import { Select } from "../shared/field";
import { Icon } from "../shared/icon";
import { languageOptions, useLang } from "../shared/lang";
import { Dropdown, MenuItem } from "../shared/menu";
import { ModeSelect } from "../shared/mode";
import { BusyButton, busyProps, useBusy } from "../shared/question";
import { VersionRow, version } from "../shared/server";
import { applied } from "../shared/settings";
import { app, keepLogin, kept, offer, plainHere, readApp } from "./app";
import { ask, askFocus, card, chooseWay, keepChosen, type Way } from "./store";

/** The ways to sign in by a form, their data-way and their name's key, in the row's order after the fingerprint. */
const WAYS: [Way, string][] = [["password", "login.password"], ["key", "login.token"]];

/** Menu is the card's ☰: the version, the language and the mode before anyone signs in, as My settings sets them, and in the app Select server last. */
function Menu() {
  useLang();
  applied.useLoad(); // the language and the mode chosen are the plain code's, drawn again where they were applied
  const servers = !!appConnect();
  return (
    <Dropdown id="login-menu" cls="nav-menu login-menu" summaryCls="nav-btn" label={t("login.menu")} summary={<Icon symbol="icon-menu" cls="ni" />}>
      <VersionRow id="login-menu-version" />
      <hr class="menu-sep" />
      <Select label={t("settings.language")} id="login-lang" wide options={languageOptions()} value={i18n.setting} onChange={(v) => {
        setSetting("lang", validSetting("lang", v) || "auto");
        changeLanguage(state.settings.lang).then(applyLanguage);
      }} />
      <ModeSelect id="login-mode" label={t("appearance.title")} wide />
      <hr id="login-menu-servers-sep" class={"menu-sep" + hid(!servers)} />
      <MenuItem id="login-menu-servers" cls={servers ? undefined : "hidden"} icon={<Icon symbol="icon-servers" cls="ni" />} label={t("app.selectServer")}
        onClick={toServers} />
    </Dropdown>
  );
}

/** Card is the sign-in's card, the dialog's content. */
export function Card() {
  useLang();
  const opened = card.opened;
  const focus = card.focus;
  const way = card.way;
  const st = app.value;
  const fpShown = kept(st);
  const offered = offer(st);
  const servers = !!appConnect();
  const [keep, setKeep] = useState(false);
  const pw = useBusy();
  const fp = useBusy();
  const userRef = useRef<HTMLInputElement>(null);
  const passRef = useRef<HTMLInputElement>(null);
  const keyRef = useRef<HTMLInputElement>(null);
  const retryRef = useRef<HTMLButtonElement>(null);
  // each opening empties the fields and turns the switch off, as askLogin did; never the closing, so that a password
  // manager still finds what was signed in with (the app's word on UiParts D210)
  // biome-ignore lint/correctness/useExhaustiveDependencies: opened is read from the card's store, which Biome takes for a stable import; the effect is for each opening, so it is listed.
  useLayoutEffect(() => {
    if (!opened) return;
    for (const r of [userRef, passRef, keyRef]) if (r.current) r.current.value = "";
    setKeep(false);
  }, [opened]);
  // the focus asked for, given in the drawing that shows its place: the chosen way's first field at an opening and at a
  // way's click, the password after a refusal, Try again where the login by fingerprint was not kept
  // biome-ignore lint/correctness/useExhaustiveDependencies: focus is read from the card's store, which Biome takes for a stable import; the effect is for each focus asked, so it is listed.
  useLayoutEffect(() => {
    if (!focus.n) return;
    if (focus.on === "way") wayFocus();
    else if (focus.on === "password") passRef.current?.focus();
    else retryRef.current?.focus();
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
        fail(t("login.failed", { error: (e as Error).message }));
        askFocus("password");
        return;
      }
      setToken(session.token);
      const login = appLogin();
      if (keep && offer(app.value) && login && !(await keepLogin(login))) return;
      if (a) await finishSignIn(a);
    });
  };
  const submitKey = (ev: Event) => {
    ev.preventDefault();
    const a = ask();
    const key = (keyRef.current?.value || "").trim();
    if (!key) return;
    setToken(key);
    if (a) finishSignIn(a);
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
      <h3 id="login-title" class="login-title">{t("login.title")}</h3>
      <div id="login-providers" class={"login-providers" + hid(!card.providers.length && !card.providersSaid)}>
        <div id="login-provider-buttons" class="login-provider-buttons">
          {card.providers.map((p) => (
            <BusyButton key={p.key} label={t("login.with", { name: p.name })} data={{ provider: p.key }}
              run={() => startProvider(p.key).catch((e: Error) => { card.providersSaid = { text: t("login.failed", { error: e.message }), ok: false }; })} />
          ))}
        </div>
        <ResultLine id="login-providers-error" cls="small" role="alert" res={card.providersSaid} />
      </div>
      <div id="login-row" class="login-row">
        <div id="login-ways" class={"login-ways" + hid(card.waysOff)} role="group" aria-label={t("login.ways")}>
          <button type="button" id="login-way-fp" {...busyProps("login-way login-way-fp" + hid(!fpShown), fp.busy)} aria-label={fpLabel} title={fpLabel}
            onClick={fingerprint}><Icon symbol="icon-fingerprint" cls="ni" /></button>
          {WAYS.map(([w, key]) => (
            <button key={w} type="button" class="login-way" data-way={w} aria-pressed={way === w ? "true" : "false"} onClick={() => chooseWay(w)}>{t(key)}</button>
          ))}
        </div>
        <Menu />
      </div>
      <p id="login-fp-plain" class={"notice small" + hid(!plainHere(st))} role="note">{t("login.fingerprintPlain")}</p>
      <div class="login-forms">
        <form id="login-form" method="post" class={hid(way !== "password" || !!keepFailed).trim() || undefined} onSubmit={submitPassword}>
          <p class="muted">{t("login.text")}</p>
          <p id="login-tls" class={"notice small" + hid(!card.selfSigned)} role="note">{t("header.selfSigned")}</p>
          <label><span>{t("login.user")}</span> <input ref={userRef} type="text" id="login-user" name="username" autocomplete="username" autocapitalize="none" spellcheck={false} /></label>
          <label><span>{t("login.password")}</span> <input ref={passRef} type="password" id="login-password" name="password" autocomplete="current-password" /></label>
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
            <button type="submit" class="primary">{t("login.submit")}</button>
          </div>
        </form>
      </div>
      <p id="login-version" class="login-version mono small muted">{v ? splitVersion(v).short : ""}</p>
    </>
  );
}
