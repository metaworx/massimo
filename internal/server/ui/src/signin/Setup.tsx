// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/signin/Setup.tsx
 *
 * The first owner's form, on an open server for a caller on the local
 * network (who is in says setup; login.js createOwner before): the name,
 * checked as Add a user checks it (shared/username.tsx), the password
 * twice (shared/password.tsx), and in the app where the phone can and the
 * address takes a key the switch that keeps the login by fingerprint;
 * Create the owner creates the owner with a password of version 2 and
 * signs in at once, busy through both, then starts the page as a sign-in
 * does. In the app
 * Cancel, left of it, goes back to the app's servers. What is typed is
 * the form's own and goes with it: the form is drawn only while it is
 * shown, and who is in is null after a sign-out. So it goes too at a read
 * of who is in that fails (null as well), where the plain form was only
 * hidden and what was typed came back; nothing reads who is in while the
 * form is shown but its own sign-in. Its fields keep the plain form's
 * attributes: the name and the passwords without a name attribute.
 */

import { useEffect, useRef, useState } from "preact/hooks";
import type { Outcome } from "../shared/action";
import { toServers } from "../shared/app";
import { ResultLine } from "../shared/badge";
import { BrandName } from "../shared/brand";
import { hid } from "../shared/cls";
import { useLang } from "../shared/lang";
import { me } from "../shared/me";
import { NewPassword, newPasswordProblem } from "../shared/password";
import { busyProps, useBusy } from "../shared/question";
import { UserNameField, useNameCheck, wordNameError } from "../shared/username";
import { app, offer, readApp } from "./app";

/** SECTION is the first owner's card's classes, a tile among the rooms' (its place is the slot index.tsx draws it into). */
const SECTION = "tile setup";

/** OwnerForm is the form itself, drawn while the first owner's card is shown. */
function OwnerForm() {
  useLang();
  const [name, setName] = useState("");
  const [password, setPassword] = useState("");
  const [again, setAgain] = useState("");
  const [keep, setKeep] = useState(false);
  const [msg, setMsg] = useState<Outcome | null>(null);
  const nameRef = useRef<HTMLInputElement>(null);
  const nc = useNameCheck(() => []);
  const create = useBusy();
  const offered = offer(app.value);
  // the app asked once each time the form is shown, as each read of who is in with setup did (renderSetupKeep)
  useEffect(() => {
    readApp();
  }, []);
  const submit = (ev: Event) => {
    ev.preventDefault();
    if (create.busy) return;
    if (!nc.check(name)) {
      nameRef.current?.focus();
      return;
    }
    const n = name.trim();
    const problem = n ? newPasswordProblem(password, again) : t("login.missing");
    if (problem) {
      setMsg({ text: problem, ok: false });
      return;
    }
    (async () => {
      try {
        const session = await create.run(async () => {
          await api("POST", "api/v1/users", { name: n, role: "owner", ...(await registerPassword(n, password)) });
          return signIn(n, password);
        });
        if (!session) return; // a second submit while it creates
        setToken(session.token);
        // the login by fingerprint kept for the new owner, where asked for; not kept, the owner is in all the
        // same and may keep it at a sign-in
        let kept = true;
        const login = appLogin();
        if (keep && offer(app.value) && login) {
          try {
            kept = await login.keep();
          } catch (e) {
            kept = false;
          }
        }
        setMsg({ text: t("setup.done", { name: n }) + (kept ? "" : " " + t("setup.notKept")), ok: true });
        afterLogin();
      } catch (e) {
        const worded = wordNameError(e as Error, n);
        if (worded) nc.say(worded);
        setMsg({ text: t("setup.failed", { error: worded || (e as Error).message }), ok: false });
      }
    })();
  };
  return (
    <form id="setup-form" class="setup-form" onSubmit={submit}>
      <p class="muted small">{t("setup.text")}</p>
      <UserNameField label={t("users.name")} id="setup-user" msgId="setup-name-msg" value={name} set={setName} nc={nc} autocomplete="username" inputRef={nameRef} />
      <NewPassword ids={["setup-password", "setup-again"]} label={t("login.password")} value={password} set={setPassword} again={again} setAgain={setAgain} required />
      <label id="setup-keep-row" class={"switch login-keep" + hid(!offered)}>
        <input type="checkbox" id="setup-keep" checked={keep && offered} onChange={(ev) => setKeep(ev.currentTarget.checked)} />
        <span class="slider" aria-hidden="true" />
        <span class="switch-label">{t("login.keepFingerprint")}</span>
      </label>
      <div class="actions">
        <span class="actions-right">
          <button type="button" id="setup-cancel" class={"form-cancel" + hid(!appConnect())} onClick={toServers}>{t("action.cancel")}</button>
          <button type="submit" {...busyProps("primary", create.busy)}>{t("setup.create")}</button>
        </span>
      </div>
      <ResultLine id="setup-msg" cls="small cubes-msg" role="status" res={msg} />
    </form>
  );
}

/** Setup is the first owner's card, hidden in its place but where who is in says that the first owner can be created from here. */
export function Setup() {
  useLang();
  const shown = !!me.value?.setup;
  return (
    <section id="setup" class={SECTION + hid(!shown)} aria-labelledby="setup-title">
      <BrandName />
      <header class="tile-head"><h2 id="setup-title">{t("setup.title")}</h2></header>
      {shown && <OwnerForm />}
    </section>
  );
}
