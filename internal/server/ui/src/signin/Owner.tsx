// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/signin/Owner.tsx
 *
 * The first owner's form in the sign-in card (AP FirstOwner, Block 2;
 * Setup.tsx, a tile among the rooms, before): on a server without users
 * the card opens with it instead of the ways to sign in (askLogin
 * "owner" of login.js). Its text, then the command that creates the
 * owner on the box, `massimo user add`, on a line of its own, copied by
 * a tap or by its icon (AP MobileApp, Block 20). The name, checked as Add a user checks it
 * (shared/username.tsx), the password twice (shared/password.tsx), each
 * with its eye, hidden again at each opening and after a success, and in
 * the app where the phone can and the address takes a key the switch that
 * keeps the login by fingerprint, and Cancel, back to the app's servers.
 * Create the owner creates the owner with a password of version 2 and
 * signs in at once, and turns through that, the app's keeping (UiParts
 * D213) and the read of who is in until the card has closed, so that no
 * second tap meets the form, and Skip for now and the app's Cancel are
 * off meanwhile; a toast then says it (FirstOwner D22), and what the
 * open server left that still works, where the creation's answer lists
 * anything (AP AccessEnds, Block 3). What a submit
 * finds wrong is said alone, an earlier attempt's line cleared, and the
 * focus goes to the field to change: the name, Password where the rule
 * refuses it, before the two are compared, which the line under
 * Password says alone, as it does while it is typed (AP PasswordRules,
 * Block 2), with the name as it is typed, so that the line follows a
 * name changed after the password (Block 3), or Again where the two
 * passwords differ; a dice beside Password suggests one and fills both
 * (Block 4); a
 * missing password the browser refuses itself, the fields being
 * required, its focus on Password (AP LooseEnds, Block 2). A
 * refusal for lack of a session, or the server's word that the first
 * owner exists already, turns the card to the sign-in (askLogin): the
 * server has users now. Under the form, Skip for now closes the card for
 * this load (D10, D21), or "Back to the page" where the user opened the
 * card on a running page, which marks nothing (D36), and Escape there
 * too, at either step and not while a request of the card runs, the
 * card's ☰ closed first where it is open (AP LooseEnds, D3); and Keep it
 * open… shows the confirmation that keeps the server open (KeepOpen.tsx,
 * AP FirstOwner, Block 3) in the form's place, the two of one height,
 * where the server is not kept open already. Its fields are not controlled, as the card's
 * (UiParts D210): no drawing writes what they hold, they are read at the
 * submit and emptied at each opening, never at a dismissal nor at the
 * turn to the sign-in, and the two passwords once the card has closed on
 * the new owner and the page's first reads after it have settled
 * (store.ts settled; AP LooseEnds, D13, D41-A; it changes FirstOwner D43,
 * which kept them for a password manager); the form is drawn once with
 * the card and only hidden.
 */

import { useEffect, useLayoutEffect, useRef, useState } from "preact/hooks";
import type { OpenSetup, UserCreated } from "../api";
import { openSetupText } from "../model/opensetup";
import { app, toServers } from "../shared/app";
import { showCard } from "../shared/card";
import { ResultLine } from "../shared/badge";
import { Copy } from "../shared/copy";
import { hid } from "../shared/cls";
import { useLang } from "../shared/lang";
import { NewPassword, newPasswordProblem } from "../shared/password";
import { busyProps, useBusy } from "../shared/question";
import { UserNameField, useNameCheck, wordNameError } from "../shared/username";
import { me } from "../shared/me";
import { keptOpen } from "../shared/owner";
import { showView } from "../shared/pages";
import { session, stillSession } from "../shared/session";
import { offer } from "./app";
import { KeepOpen } from "./KeepOpen";
import { ask, card, creationEnded, keepAsk, leaves, skip } from "./store";

/**
 * doneToast says that the owner name was created and signed in, and in the app where the login by fingerprint was not
 * kept; where anything set up before the owner still works (open, the answer's open_setup), it says what, for the
 * owner to check, in the warn tone for 30 s, with Show, which opens System settings → Integrations where an
 * integration is listed, else the log, whose open_setup line says it too, the focus on the card's heading or the
 * log's title (AP AccessEnds, D11, D13). A sign-out takes the toast away (login.js forgetSession), and its Show
 * does nothing once the owner's session has ended.
 */
function doneToast(name: string, kept: boolean, open?: OpenSetup): void {
  const text = t("setup.done", { name }) + (kept ? "" : " " + t("setup.notKept"));
  const what = open ? openSetupText(open) : "";
  if (!open || !what) {
    showToast(text);
    return;
  }
  const n = session();
  const integrations = !!open.integrations?.length;
  const show = () => {
    if (!stillSession(n)) return;
    if (!integrations) {
      showView("log");
      window.MassimoParts.log.focusHead(); // another region's title, through its part
      return;
    }
    showView("system");
    showCard("sys-integrations", true); // on a page just shown, as home.js showFound: a smooth scroll stops short
  };
  showToast(text + " " + t("setup.openSetup", { what }), { ms: 30000, tone: "warn", action: { label: t("setup.openSetupShow"), run: show } });
}

/** OWNER_COMMAND is the command that creates the owner on the server's box, as setup.text names it. */
const OWNER_COMMAND = "massimo user add";

/** Owner is the first owner's place in the card: the form, and under it the row with Skip for now. */
export function Owner() {
  useLang();
  const shown = card.owner;
  const step = card.ownerStep;
  const working = card.working;
  const kept = keptOpen(me.value);
  const opened = card.opened;
  const succeeded = card.succeeded;
  const focus = card.focus;
  const offered = offer(app.value);
  const [keep, setKeep] = useState(false);
  const [refused, setRefused] = useState(0);
  // the name as typed, for the password's line, whose rule takes it (AP PasswordRules, D50-A): the field is not
  // controlled, so its keystrokes and change events tell it, and each submit and opening read it as it is
  const [typedName, setTypedName] = useState("");
  const nameRef = useRef<HTMLInputElement>(null);
  const passRef = useRef<HTMLInputElement>(null);
  const againRef = useRef<HTMLInputElement>(null);
  const keepRef = useRef<HTMLButtonElement>(null);
  const nc = useNameCheck(() => []);
  const create = useBusy();
  const byUser = shown && card.byUser && card.shown;
  // Escape leaves a card the user opened, as Back to the page does, at either step (AP LooseEnds, D3): a listener on
  // the document while it is open, after the menus' own (components.js), which mark the Escape that closed a menu,
  // so that with the card's ☰ open the first Escape closes the menu alone; nothing while a request of the card runs
  useEffect(() => {
    if (!byUser) return;
    const onKey = (ev: KeyboardEvent) => {
      if (ev.key === "Escape" && !ev.defaultPrevented && leaves()) skip();
    };
    document.addEventListener("keydown", onKey);
    return () => document.removeEventListener("keydown", onKey);
  }, [byUser]);
  // each opening empties the fields, the name's line and the switch, as the card's own (UiParts D210)
  // biome-ignore lint/correctness/useExhaustiveDependencies: opened is read from the card's store, which Biome takes for a stable import; the effect is for each opening, so it is listed, and nc's line is cleared with it.
  useLayoutEffect(() => {
    if (!opened) return;
    for (const r of [nameRef, passRef, againRef]) if (r.current) r.current.value = "";
    setTypedName("");
    setKeep(false);
    nc.say("");
  }, [opened]);
  // the owner made, signed in, the card closed and the page's first reads after it settled (store.ts settled): the two
  // passwords emptied (AP LooseEnds, D13, D41-A), their eyes hidden again by their reset; the name stays until the
  // next opening
  // biome-ignore lint/correctness/useExhaustiveDependencies: succeeded is read from the card's store, which Biome takes for a stable import; the effect is for each success, so it is listed.
  useLayoutEffect(() => {
    if (!succeeded) return;
    for (const r of [passRef, againRef]) if (r.current) r.current.value = "";
  }, [succeeded]);
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
    card.ownerSaid = null; // the line says what is wrong now: none of an earlier attempt beside the name's rule
    const name = nameRef.current?.value || "";
    setTypedName(name);
    if (!nc.check(name)) {
      nameRef.current?.focus();
      return;
    }
    const n = name.trim();
    const password = passRef.current?.value || "";
    const again = againRef.current?.value || "";
    const problem = n ? newPasswordProblem(password, again, [name]) : { text: t("login.missing"), rule: false };
    if (problem) {
      // the focus on the field to change, in the submit's handler: the name missing, the password the rule refuses, which
      // its line says (it alone, so that nothing contradicts it once the password can be set), or the two differing; no
      // password at all never comes here, the browser refusing the required fields first
      if (problem.rule) setRefused((r) => r + 1);
      else say(problem.text);
      (!n ? nameRef : problem.rule ? passRef : againRef).current?.focus();
      return;
    }
    card.working = true;
    card.creating = true;
    // busy through the creation, the sign-in, the app's keeping of the login (UiParts D213) and the read of who is in
    // until the card has closed: a second tap meanwhile creates nothing
    create.run(async () => {
      let made = false;
      try {
        const created = await api<UserCreated | null>("POST", "api/v1/users", { name: n, role: "owner", ...(await registerPassword(n, password)) });
        made = true;
        const session = await signIn(n, password);
        const put = setToken(session.token); // in the app, the session kept by the bridge before login.keep reads it (D68-A)
        creationEnded(true); // the owner signed in: a refusal now is of this session, and turns the card
        // the login by fingerprint kept for the new owner, where asked for; not kept, the owner is in all the same and
        // may keep it at a sign-in (FirstOwner D26)
        let kept = true;
        const login = appLogin();
        if (keep && offer(app.value) && login) {
          await put;
          try {
            kept = await login.keep();
          } catch (e) {
            kept = false;
          }
        }
        if (a && (await finishSignIn(a))) doneToast(n, kept, created?.open_setup);
      } catch (e) {
        const err = e as Error & { status?: number };
        // the server has users now: refused for lack of a session (api() has turned the card already), the first owner
        // made by another request (409), or this one made and its sign-in failed: the card becomes the sign-in
        if (made || err.status === 401 || err.status === 409) {
          creationEnded(false); // the creation has ended: the card turns, once
          askLogin();
          return;
        }
        const worded = wordNameError(err, n);
        if (worded) nc.say(worded);
        say(t("setup.failed", { error: worded || err.message }));
      } finally {
        card.working = false;
        if (card.creating) creationEnded(false); // ended without a session: a refusal meanwhile turns the card now
      }
    });
  };
  return (
    <div id="setup" class={"login-owner" + hid(!shown)}>
      <div class="owner-steps">
        <div id="setup-step" class={"owner-step" + hid(step !== "form")}>
          <form id="setup-form" class="setup-form" onSubmit={submit}>
            <p class="muted">{t("setup.text")}</p>
            {/* the command on a line of its own, never wrapped, copied by a tap or by its icon (AP MobileApp, Block 20) */}
            <p id="setup-command" class="setup-command" translate={false}><Copy text={OWNER_COMMAND} icon cls="mono setup-command-text" /></p>
            <p id="setup-tls" class={"notice small" + hid(!card.selfSigned)} role="note">{t("header.selfSigned")}</p>
            <UserNameField label={t("users.name")} id="setup-user" msgId="setup-name-msg" nc={nc} autocomplete="username" inputRef={nameRef} typed={setTypedName} />
            <NewPassword ids={["setup-password", "setup-again"]} label={t("login.password")} inputRefs={[passRef, againRef]} required reset={opened + succeeded}
              context={() => [typedName]} refused={refused} suggest />
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
            <button type="button" id="setup-skip" disabled={working} onClick={skip}>{t(card.byUser ? "setup.back" : "setup.skip")}</button>
            <button ref={keepRef} type="button" id="setup-keep-open" class={hid(kept).trim() || undefined} disabled={working} onClick={keepAsk}>{t("setup.keepOpen")}</button>
          </div>
        </div>
        <KeepOpen />
      </div>
    </div>
  );
}
