// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/signin/Owner.tsx
 *
 * The first owner's form in the sign-in card (AP FirstOwner, Block 2;
 * Setup.tsx, a tile among the rooms, before): on a server without users
 * the card opens with it instead of the ways to sign in (askLogin
 * "owner" of login.js). The name, checked as Add a user checks it
 * (shared/username.tsx), the password twice (shared/password.tsx), and in
 * the app where the phone can and the address takes a key the switch that
 * keeps the login by fingerprint, and Cancel, back to the app's servers.
 * Create the owner creates the owner with a password of version 2 and
 * signs in at once, and turns through that, the app's keeping (UiParts
 * D213) and the read of who is in until the card has closed, so that no
 * second tap meets the form, and Skip for now and the app's Cancel are
 * off meanwhile; a toast then says it (FirstOwner D22). A
 * refusal for lack of a session, or the server's word that the first
 * owner exists already, turns the card to the sign-in (askLogin): the
 * server has users now. Under the form, Skip for now closes the card for
 * this load (D10, D21), and Keep it open… shows the confirmation that
 * keeps the server open (KeepOpen.tsx, AP FirstOwner, Block 3) in the
 * form's place, the two of one height, where the server is not kept open
 * already. Its fields are not controlled, as the card's
 * (UiParts D210): no drawing writes what they hold, they are read at the
 * submit and emptied at each opening alone, never at the closing nor at
 * the turn to the sign-in, so that a password manager finds what was
 * typed when the form goes (FirstOwner D43); the form is drawn once with
 * the card and only hidden.
 */

import { useLayoutEffect, useRef, useState } from "preact/hooks";
import { toServers } from "../shared/app";
import { ResultLine } from "../shared/badge";
import { hid } from "../shared/cls";
import { useLang } from "../shared/lang";
import { NewPassword, newPasswordProblem } from "../shared/password";
import { busyProps, useBusy } from "../shared/question";
import { UserNameField, useNameCheck, wordNameError } from "../shared/username";
import { me } from "../shared/me";
import { app, offer } from "./app";
import { KeepOpen } from "./KeepOpen";
import { ask, card, keepAsk, skip } from "./store";

/** Owner is the first owner's place in the card: the form, and under it the row with Skip for now. */
export function Owner() {
  useLang();
  const shown = card.owner;
  const step = card.ownerStep;
  const working = card.working;
  const kept = !!me.value?.kept_open;
  const opened = card.opened;
  const focus = card.focus;
  const offered = offer(app.value);
  const [keep, setKeep] = useState(false);
  const nameRef = useRef<HTMLInputElement>(null);
  const passRef = useRef<HTMLInputElement>(null);
  const againRef = useRef<HTMLInputElement>(null);
  const keepRef = useRef<HTMLButtonElement>(null);
  const nc = useNameCheck(() => []);
  const create = useBusy();
  // each opening empties the fields, the name's line and the switch, as the card's own (UiParts D210)
  // biome-ignore lint/correctness/useExhaustiveDependencies: opened is read from the card's store, which Biome takes for a stable import; the effect is for each opening, so it is listed, and nc's line is cleared with it.
  useLayoutEffect(() => {
    if (!opened) return;
    for (const r of [nameRef, passRef, againRef]) if (r.current) r.current.value = "";
    setKeep(false);
    nc.say("");
  }, [opened]);
  // the name in focus at the first owner's opening, in the drawing that shows the form; none on a touch screen, where it
  // would bring the soft keyboard up by itself (UiParts D224)
  // and Keep it open… in focus again after Back
  // biome-ignore lint/correctness/useExhaustiveDependencies: focus is read from the card's store, which Biome takes for a stable import; the effect is for each focus asked, so it is listed.
  useLayoutEffect(() => {
    if (!focus.n) return;
    if (focus.on === "keepButton") keepRef.current?.focus();
    if (focus.on !== "owner") return;
    if (matchMedia("(pointer: coarse)").matches) return;
    nameRef.current?.focus();
  }, [focus]);
  const say = (text: string) => {
    card.ownerSaid = { text, ok: false };
  };
  const submit = (ev: Event) => {
    ev.preventDefault();
    if (create.busy) return;
    const a = ask();
    const name = nameRef.current?.value || "";
    if (!nc.check(name)) {
      nameRef.current?.focus();
      return;
    }
    const n = name.trim();
    const password = passRef.current?.value || "";
    const problem = n ? newPasswordProblem(password, againRef.current?.value || "") : t("login.missing");
    if (problem) return say(problem);
    card.ownerSaid = null;
    card.working = true;
    // busy through the creation, the sign-in, the app's keeping of the login (UiParts D213) and the read of who is in
    // until the card has closed: a second tap meanwhile creates nothing
    create.run(async () => {
      let made = false;
      try {
        await api("POST", "api/v1/users", { name: n, role: "owner", ...(await registerPassword(n, password)) });
        made = true;
        const session = await signIn(n, password);
        setToken(session.token);
        // the login by fingerprint kept for the new owner, where asked for; not kept, the owner is in all the same and
        // may keep it at a sign-in (FirstOwner D26)
        let kept = true;
        const login = appLogin();
        if (keep && offer(app.value) && login) {
          try {
            kept = await login.keep();
          } catch (e) {
            kept = false;
          }
        }
        if (a && (await finishSignIn(a))) showToast(t("setup.done", { name: n }) + (kept ? "" : " " + t("setup.notKept")));
      } catch (e) {
        const err = e as Error & { status?: number };
        // the server has users now: refused for lack of a session (api() has turned the card already), the first owner
        // made by another request (409), or this one made and its sign-in failed: the card becomes the sign-in
        if (made || err.status === 401 || err.status === 409) {
          askLogin();
          return;
        }
        const worded = wordNameError(err, n);
        if (worded) nc.say(worded);
        say(t("setup.failed", { error: worded || err.message }));
      } finally {
        card.working = false;
      }
    });
  };
  return (
    <div id="setup" class={"login-owner" + hid(!shown)}>
      <div class="owner-steps">
        <div id="setup-step" class={"owner-step" + hid(step !== "form")}>
          <form id="setup-form" class="setup-form" onSubmit={submit}>
            <p class="muted">{t("setup.text")}</p>
            <p id="setup-tls" class={"notice small" + hid(!card.selfSigned)} role="note">{t("header.selfSigned")}</p>
            <UserNameField label={t("users.name")} id="setup-user" msgId="setup-name-msg" nc={nc} autocomplete="username" inputRef={nameRef} />
            <NewPassword ids={["setup-password", "setup-again"]} label={t("login.password")} inputRefs={[passRef, againRef]} required />
            <label id="setup-keep-row" class={"switch login-keep" + hid(!offered)}>
              <input type="checkbox" id="setup-keep" checked={keep && offered} onChange={(ev) => setKeep(ev.currentTarget.checked)} />
              <span class="slider" aria-hidden="true" />
              <span class="switch-label">{t("login.keepFingerprint")}</span>
            </label>
            <div class="actions">
              <span class="actions-right">
                <button type="button" id="setup-cancel" class={"form-cancel" + hid(!appConnect())} disabled={working} onClick={toServers}>{t("action.cancel")}</button>
                <button type="submit" {...busyProps("primary", create.busy)}>{t("setup.create")}</button>
              </span>
            </div>
            <ResultLine id="setup-msg" cls="small cubes-msg" role="status" res={card.ownerSaid} />
          </form>
          <div id="setup-more" class="login-owner-more">
            <button type="button" id="setup-skip" disabled={working} onClick={skip}>{t("setup.skip")}</button>
            <button ref={keepRef} type="button" id="setup-keep-open" class={hid(kept).trim() || undefined} disabled={working} onClick={keepAsk}>{t("setup.keepOpen")}</button>
          </div>
        </div>
        <KeepOpen />
      </div>
    </div>
  );
}
