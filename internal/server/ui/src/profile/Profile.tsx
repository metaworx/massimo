// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/profile/Profile.tsx
 *
 * My profile as a part, drawn once into its card and updated in place,
 * for a user signed in: the display name, a settings form kept by its
 * key (shared/forms.ts) with Reset, Save by its state and the disk in the
 * card's pill where it is unsaved; Change password, or Set a password
 * for a user with keys only, its fields the form's own state alone, never
 * a draft; the own sessions and keys (shared/tokens.tsx), the API key the
 * page's address names marked and brought into view; and the own linked
 * accounts (shared/identities.tsx) with Link per enabled provider
 * and a link come back shown before it is stored. Behind the last button
 * of each group an ⓘ explains its buttons. Another user signed in draws
 * all of it from nothing.
 */

import type { ComponentChildren } from "preact";
import { useLayoutEffect, useRef, useState } from "preact/hooks";
import type { User } from "../api";
import { ResultLine } from "../shared/badge";
import { hid } from "../shared/cls";
import { FormFooter, TextField, useDraft } from "../shared/field";
import { Fold } from "../shared/fold";
import { focusForm, useForm } from "../shared/forms";
import { useHelp } from "../shared/help";
import { Identities } from "../shared/identities";
import { useLang } from "../shared/lang";
import { NewPassword, newPasswordProblem, PasswordField } from "../shared/password";
import { me, profileOf } from "../shared/me";
import { BusyButton } from "../shared/question";
import { Tokens } from "../shared/tokens";
import { cancelLink, changePassword, confirmLink, passwordMsg, profileMsg, run, saveName, startLink } from "./send";
import { type Confirm, firstDrawn, NAME_FORM, PASSWORD_FORM, profile, said } from "./store";
import { keepInView, showCard } from "../shared/card";
import { isShown, showView } from "../shared/pages";

/** openForm shows a form of My profile and puts the focus into it, from any page: the personal page shown, its card unfolded. */
function openForm(key: string): void {
  if (!isShown("me")) showView("me");
  showCard("profile"); // a folded card unfolded, its form in sight
  if (key === PASSWORD_FORM) profile.passwordOpen = true;
  focusForm(key);
}

/**
 * keysDrawn is told of every read of the own keys drawn, with the row
 * marked (store.ts markKey): the first since the mark was set brings it
 * into the middle of the window, and keeps it there while the page
 * settles, in place of its card (shared/card.ts keepInView).
 */
function keysDrawn(row: HTMLElement | null): void {
  if (!firstDrawn() || !row) return;
  row.scrollIntoView({ block: "center" });
  keepInView(row, 2000, "center");
}

/** NameForm is the display name, the name the personal page and the users list show, with Reset and Save, off while it is the one stored. */
function NameForm({ u }: { u: User }) {
  useLang();
  const form = useForm({ key: NAME_FORM, name: { key: "users.displayName" }, open: () => openForm(NAME_FORM) });
  const display = useDraft(u.display_name || "", (v) => v.trim(), form.place("display_name"));
  form.track(display.changed, [display]);
  const help = useHelp("profileName", t("users.displayName"));
  const submit = (ev: Event) => {
    ev.preventDefault();
    if (!display.changed) return;
    form.save(() => saveName(u.name, display.value.trim()));
  };
  return (
    <form {...form.props} class="profile-form" onSubmit={submit}>
      <TextField label={t("users.displayName")} id="profile-display-name" name="display_name" maxLength={64} draft={display} />
      <FormFooter submit={t("action.save")} look={form.look} reset={form.reset} off={!display.changed} after={help.button} />
      {help.text}
    </form>
  );
}

/**
 * PasswordForm is Change password, for a user with a password, with the
 * current one, or Set a password for a user with keys only, without it
 * and with a line saying what a password adds; the new one twice. What is
 * typed is this part's own and goes with it, kept by no key; the form's
 * marks know only that a new password is typed (UnsavedMarks D28), the
 * current one alone, as a password manager fills it in, being none. The
 * new one is held to the rule, which its line says while it is typed; a
 * Save it refuses sends nothing, says so in that line alone and puts the
 * focus in it (AP PasswordRules, Block 2).
 */
function PasswordForm({ who }: { who: User }) {
  useLang();
  const hasPw = who.has_password;
  const title = hasPw ? "me.changePassword" : "me.setPassword";
  const form = useForm({ key: PASSWORD_FORM, name: { key: title }, open: () => openForm(PASSWORD_FORM) });
  const [current, setCurrent] = useState("");
  const [password, setPassword] = useState("");
  const [again, setAgain] = useState("");
  const newRef = useRef<HTMLInputElement>(null);
  const [refused, setRefused] = useState(0);
  // the new password's two fields alone: a current one a password manager fills in is no change
  const changed = password !== "" || again !== "";
  const forget = { take: () => { setCurrent(""); setPassword(""); setAgain(""); } };
  form.track(changed, [forget]);
  const typed = (set: (v: string) => void) => (v: string) => {
    set(v);
    form.place("password").changed();
  };
  const help = useHelp("profilePassword", t("me.changePassword"));
  const submit = (ev: Event) => {
    ev.preventDefault();
    if (form.look === "saving") return;
    const problem = newPasswordProblem(password, again, [who.name, who.display_name || ""]);
    if (problem) {
      // the rule's refusal is said by the new password's line alone, the focus in it, in the submit's handler; the form's
      // line, of an earlier attempt, cleared, so that nothing contradicts the line once the password can be set
      if (problem.rule) {
        said.password = null;
        setRefused((r) => r + 1);
        newRef.current?.focus();
      } else passwordMsg(problem.text, false);
      return;
    }
    form.save(() => changePassword(who, current, password));
  };
  return (
    <Fold id="me-password" cls="" open={profile.passwordOpen} onToggle={(o) => { profile.passwordOpen = o; }} summary={t(title)} summaryProps={{ id: "me-password-title" }}>
      <p id="me-password-hint" class={"muted small" + hid(hasPw)}>{t("me.setPasswordHint")}</p>
      <form {...form.props} id="me-password-form" onSubmit={submit}>
        <PasswordField label={t("users.currentPassword")} id="me-current" name="current_password" autocomplete="current-password"
          cls={"me-current-field" + hid(!hasPw)} reset={profile.passwordOpen} value={current} set={typed(setCurrent)} />
        <NewPassword ids={["me-new", "me-again"]} names={["password", "again"]} label={t("users.password")} reset={profile.passwordOpen}
          value={password} set={typed(setPassword)} again={again} setAgain={typed(setAgain)} inputRef={newRef} context={() => [who.name, who.display_name || ""]}
          refused={refused} />
        <FormFooter submit={t("action.save")} look={form.look} reset={form.reset} after={help.button} />
        <ResultLine id="me-password-msg" cls="small cubes-msg" role="status" res={said.password} />
        {help.text}
      </form>
    </Fold>
  );
}

/**
 * LinkConfirm is a link come back, at the top of the linked accounts,
 * before it is stored: the account at the provider, the link it
 * replaces, Link, which stores it, and Cancel, which drops it; the focus
 * on its title as it comes.
 */
function LinkConfirm({ u, c }: { u: User; c: Confirm }) {
  useLang();
  const title = useRef<HTMLParagraphElement>(null);
  // in the drawing's own turn, as the plain box took it
  useLayoutEffect(() => {
    title.current?.focus();
  }, []);
  const p = c.p;
  const rows: [string, string | undefined][] = [
    ["profile.confirm.email", p.email], ["profile.confirm.name", p.name], ["profile.confirm.username", p.username],
    ["profile.confirm.subject", p.subject], ["profile.confirm.groups", (p.groups || []).join(", ")],
  ];
  const replaces = (p.replaces || []).map((i) => i.email || i.username || i.subject).join(", ");
  return (
    <div class="link-confirm notice" role="group">
      <p ref={title} class="link-confirm-title" tabIndex={-1}>{t("profile.confirm.title", { name: p.provider_name || "" })}</p>
      <dl class="link-confirm-details">
        {rows.filter(([, v]) => v).map(([k, v]) => [<dt key={k + ":t"}>{t(k)}</dt>, <dd key={k + ":d"}>{v}</dd>])}
      </dl>
      <p class={"link-confirm-replaces small" + hid(!replaces)}>{replaces ? t("profile.confirm.replaces", { accounts: replaces }) : ""}</p>
      <div class="actions">
        <span class="actions-right">
          <BusyButton cls="cancel-link" label={t("action.cancel")} run={() => cancelLink(u.name, c)} />
          <BusyButton cls="primary confirm-link" label={t("profile.confirm.link")} run={() => confirmLink(u.name, c)} />
        </span>
      </div>
    </div>
  );
}

/** LinkButtons are Link per enabled provider, Link again where the account there is linked, and the group's ⓘ after the last. */
function LinkButtons({ u, list, help }: { u: User; list: { provider: string; bound: boolean }[]; help: ComponentChildren }) {
  const providers = appBridge() ? [] : profile.providers; // the app's WebView the providers refuse
  return (
    <div class="profile-link-buttons">
      {providers.map((p) => {
        const linked = list.some((i) => i.provider === p.key && i.bound);
        return <BusyButton key={p.key} label={t(linked ? "profile.linkAgain" : "profile.link", { name: p.name })} data={{ provider: p.key }} run={() => startLink(u.name, p.key)} />;
      })}
      {help}
    </div>
  );
}

/**
 * Mine is My profile of the user signed in, who: the display name once
 * read, the password, the sessions and keys with the one marked, the
 * linked accounts. An integration's user, in with a key it holds from
 * before it could have none, gets a line in place of the password, its
 * keys listed to be revoked, no new one and no Link (AP LooseEnds, D1).
 */
function Mine({ who }: { who: User }) {
  useLang();
  const u = profile.user && profile.user.name === who.name ? profile.user : null;
  const c = profile.confirm;
  const linked = useHelp("profileLinked", t("profile.linked"));
  const ofIntegration = !!who.integration;
  return (
    <>
      <div id="profile-name">{u && <NameForm u={u} />}</div>
      {ofIntegration ? <p id="me-no-sign-in" class="muted small user-no-sign-in">{t("users.integrationNoSignIn")}</p> : <PasswordForm who={who} />}
      <div id="profile-tokens">
        {u && <Tokens u={u} off={false} open reads={profile.reads} run={run} say={profileMsg} labelId={"profile-key-label-" + u.name} help="profileTokens"
          marked={profile.marked} drawn={keysDrawn} listOnly={ofIntegration} />}
      </div>
      <div id="profile-identities">
        {u && c && !ofIntegration && <LinkConfirm key={c.code} u={u} c={c} />}
        {u && (
          <Identities u={u} off={false} open reads={profile.reads} providers={profile.providers} run={run} say={profileMsg} title={t("profile.linked")}
            none={t("profile.linkedNone")} cls="profile-identities" quiet offer={!ofIntegration && !appBridge() && profile.providers.length > 0} after={(list) => (
              ofIntegration ? null : (
                <>
                  <LinkButtons u={u} list={list} help={linked.button} />
                  {linked.text}
                </>
              )
            )} />
        )}
      </div>
    </>
  );
}

/** ProfileCard is the card's body: My profile of the user signed in, nothing on an open server or with the server's token, and the message line. */
export function ProfileCard() {
  useLang();
  const m = me.value;
  const who = profileOf(m) ? m?.user || null : null;
  return (
    <div class="profile" aria-live="polite">
      {who && <Mine key={who.name} who={who} />}
      <ResultLine id="profile-msg" cls="small cubes-msg" role="status" res={said.profile} />
    </div>
  );
}
