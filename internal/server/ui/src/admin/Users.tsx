// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/admin/Users.tsx
 *
 * System settings → Users as a part: the password sign-in's switch with
 * the users it would lock out, a panel per user, built once per user and
 * updated in place, and the form that adds one. A user's panel holds its
 * settings form, kept by its login name (shared/forms.ts) so that what is
 * typed outlives an action in another panel, a reload of the list and
 * the panel's closing, with Reset, Save by its state and the disk in the
 * section's pill where it is unsaved; its sessions and keys; its
 * identities at the providers. A new password, one's current one and a
 * key just made are a part's own state alone, never in the forms' store.
 */

import { useLayoutEffect, useMemo, useRef, useState } from "preact/hooks";
import type { User } from "../api";
import { Adder } from "../shared/adder";
import { Badge, ResultLine } from "../shared/badge";
import { hid } from "../shared/cls";
import { DateTime, Field, FormFooter, Select, Switch, TextField, useDraft } from "../shared/field";
import { Fold, FoldHead } from "../shared/fold";
import { focusForm, useForm } from "../shared/forms";
import { BusyButton, Question, busyProps, useAsk, useBusy, useSetAtOnce } from "../shared/question";
import { isoOf, localOf } from "../shared/time";
import { type Changes, Identities } from "../shared/identities";
import { Tokens } from "../shared/tokens";
import { UserNameField, useNameCheck } from "../shared/username";
import { PasswordField, passwordRefusal } from "../shared/password";
import { useLang } from "../shared/lang";
import { firstOwnerOf, isMe, me } from "../shared/me";
import { askOwner, keptOpen } from "../shared/owner";
import { adding, enabled, identityFormKey, identityUI, keptOff, passwordLogin, passwordReveal, reads, said, userFormKey, userUI, users } from "./store";
import { createUser, keepOpenOff, mayManage, mayPassword, removeUser, roleOptions, run, saveUser, setPasswordLogin, setPasswordReveal, usersAdmin, usersMsg } from "./sendUsers";
import { cardBadges, showCard } from "../shared/card";
import { isShown, showView } from "../shared/pages";
import { useLook } from "../shared/look";

/** userName is how a user is shown: its chain (AP StoreRaise v1.2), its display name, else its login name. */
function userName(u: User): string {
  return u.display || u.display_name || u.name;
}

/**
 * openUser shows a user's settings and puts the focus into them, from any
 * page: System settings shown, the user's panel open.
 */
function openUser(name: string): void {
  if (!isShown("system")) showView("system");
  showCard("sys-users"); // a folded card unfolded, its form in sight
  userUI.of(name).open = true;
  focusForm(userFormKey(name));
}

/**
 * changesOf are the change forms of a user's identities as this list
 * keeps them (store.ts identityUI, by identityFormKey), each brought into
 * sight by its user's panel opened (openUser).
 */
function changesOf(name: string): Changes {
  return {
    key: (id) => identityFormKey(name, id),
    under: userFormKey(name) + ":identity",
    ui: (key) => identityUI.of(key),
    forget: (key) => identityUI.forget(key),
    show: () => {
      if (!isShown("system")) showView("system");
      showCard("sys-users"); // a folded card unfolded, its form in sight
      userUI.of(name).open = true;
    },
  };
}

/** Badges are a user's badges in its panel's head: you first, the role, the state, keys only, and an integration's. */
function badges(u: User) {
  const out = [];
  if (isMe(u.name)) out.push(<Badge key="you" kind="ok" text={t("users.you")} />);
  out.push(<Badge key="role" kind="mode" text={roleLabel(u.role)} />);
  if (u.disabled) out.push(<Badge key="state" kind="bad" text={t("users.state.disabled")} />);
  else if (u.expires_at && Date.parse(u.expires_at) <= Date.now()) out.push(<Badge key="state" kind="bad" text={t("users.state.expired")} />);
  else if (u.expires_at) out.push(<Badge key="state" kind="warn" text={t("users.state.until", { date: fmtShortTime(Date.parse(u.expires_at)) })} />);
  if (!u.has_password) out.push(<Badge key="keys" kind="mode" text={t("users.keysOnly")} />);
  // an integration's user, and the integrations' users that act as this one
  const ig = u.integration;
  if (ig) out.push(<Badge key="ig" kind="mode" text={t(ig.id ? "users.integrationUser" : "users.integrationOwn")} />);
  if (ig?.linked_to) out.push(<Badge key="linked" kind="ok" text={t("users.linkedTo", { name: ig.linked_to })} />);
  if (u.links?.length) out.push(<Badge key="links" kind="mode" text={tn("users.links", u.links.length)} />);
  return out;
}

const trim = (v: string) => v.trim();

/**
 * UserForm is a user's settings: its display name, and for its manager its
 * role, of those the server says it may give (an integration's user those
 * within its integration's maximum, AP AccessEnds, D3; a stored role or
 * one chosen beyond it kept, with a line under it, D4), whether it is
 * disabled and a guest's end; a new password and, for one's own without
 * managing it, the current one; Remove, which asks in place of the
 * actions, Reset and Save. The fields that show what is stored keep their
 * drafts by the user's name; Save sends those that differ from the user as
 * the list last gave it. open is the panel's: a password shown is hidden
 * again as the panel closes or opens. An integration's user gets no
 * password, a line in its place: it acts through its integration alone
 * (AP LooseEnds, D1); one it holds from before, its manager removes with
 * Remove the password. A new password is typed once (AP PasswordRules,
 * D51-A) and held to the rule, its line under it; a Save the rule refuses
 * sends nothing and puts the focus in it; left empty, the password stays
 * as it is and is not checked. The dice beside it suggests one (Block 4).
 */
function UserForm({ u, off, open }: { u: User; off: boolean; open: boolean }) {
  useLook();
  const manage = mayManage(u);
  const self = isMe(u.name);
  const form = useForm({ key: userFormKey(u.name), name: { key: "users.formName", args: { name: userName(u) } }, open: () => openUser(u.name) });
  const display = useDraft(u.display_name || "", trim, form.place("display_name"));
  const role = useDraft(u.role, undefined, form.place("role"));
  const disabled = useDraft(!!u.disabled, undefined, form.place("disabled"));
  const expires = useDraft(localOf(u.expires_at), undefined, form.place("expires"));
  // shown once and never kept: the part's own state, not the forms' store
  const [password, setPassword] = useState("");
  const [current, setCurrent] = useState("");
  const passRef = useRef<HTMLInputElement>(null);
  const [refused, setRefused] = useState(0);
  const ask = useAsk<HTMLButtonElement>();
  const body: Record<string, unknown> = {};
  if ((manage || self) && display.changed) body.display_name = display.value.trim();
  if (manage) {
    if (role.changed) body.role = role.value;
    if (disabled.changed) body.disabled = disabled.value;
    if (expires.changed) body.expires_at = isoOf(expires.value);
  }
  const changed = Object.keys(body).length > 0 || password !== "";
  const forget = { take: () => { setPassword(""); setCurrent(""); } };
  form.track(changed, [display, role, disabled, expires, forget]);
  const submit = (ev: Event) => {
    ev.preventDefault();
    if (form.look === "saving") return; // a second submit while the save runs, before the rule is asked
    if (!changed) {
      usersMsg(t("users.unchanged"), true);
      return;
    }
    // a new password the rule refuses: nothing sent, its line saying so, the focus in it, in the submit's handler; empty is
    // unchanged and not checked
    if (password && passwordRefusal(password, words())) {
      setRefused((r) => r + 1);
      passRef.current?.focus();
      return;
    }
    form.save(() => saveUser(u, { ...body }, password, self && !manage ? current : null));
  };
  const id = (field: string) => "user-" + u.name + "-" + field;
  const words = () => [u.name, u.display_name || "", display.value.trim()];
  const admin = "admin-only" + hid(!manage);
  const ofIntegration = !!u.integration;
  // a role chosen beyond the maximum of the user's integration, the stored one or one chosen before the list was read again: offered all the same (AP AccessEnds, D4)
  const beyond = ofIntegration && !!u.may && !(u.may.roles || []).includes(role.value);
  return (
    <form {...form.props} class={"user-form" + hid(off)} onSubmit={submit}>
      <TextField label={t("users.displayName")} id={id("display_name")} name="display_name" maxLength={64} draft={display} cls={hid(!manage && !self).trim() || undefined} />
      <Select label={t("users.role")} id={id("role")} name="role" wide cls={admin} options={roleOptions(u.role, u, role.value)} draft={role} show={(k) => roleLabel(k)}
        hint={beyond ? t("users.roleAboveMax") : undefined} hintId={id("role-hint")} />
      <Switch name="disabled" label={t("users.disabled")} rowCls={admin} draft={disabled} />
      <Field label={t("users.expires")} id={id("expires")} wide cls={admin} hint={t("users.expiresHint")} draft={expires}
        show={(v) => (v ? fmtShortTime(Date.parse(v)) : t("users.noEnd"))}>
        <DateTime id={id("expires")} cls="field-control" step={60} value={expires.value} set={expires.set} />
      </Field>
      {ofIntegration ? (
        <>
          <p class="muted small user-no-sign-in">{t("users.integrationNoSignIn")}</p>
          {manage && u.has_password && (
            <div class="actions">
              <BusyButton cls="remove-password" label={t("users.removePassword")} run={() => saveUser(u, { password: "" }, "", null)} />
            </div>
          )}
        </>
      ) : (
        <>
          <PasswordField label={t("users.password")} id={id("password")} name="password" autocomplete="new-password" placeholder={t("users.passwordHint")}
            reset={open} value={password} inputRef={passRef} rule={words} refused={refused} suggest set={(v) => {
              setPassword(v);
              form.place("password").changed();
            }} />
          <PasswordField label={t("users.currentPassword")} id={id("current_password")} name="current_password" autocomplete="current-password"
            cls={"field-current" + hid(manage || !self)} reset={open} value={current} set={setCurrent} />
        </>
      )}
      {ask.asking ? (
        <Question text={t("users.removeConfirm", { name: u.name })} yes={t("users.remove")} no={t("action.keep")} onNo={() => ask.done()}
          onYes={async () => {
            if (!(await removeUser(u))) ask.done();
          }} />
      ) : (
        <FormFooter submit={t("action.save")} look={form.look} reset={form.reset} off={!changed}
          extra={<button ref={ask.ref} type="button" class={"remove-user" + hid(!manage)} onClick={ask.ask}>{t("users.remove")}</button>} />
      )}
    </form>
  );
}

/**
 * UserPanel is a user's panel: its head with the name it is shown by, the
 * login name apart (none for an integration's user, which never signs
 * in) and the badges; the integrations' users that act as it; its form,
 * or for a user that outranks the signed-in one why it cannot be changed;
 * its sessions and keys, the manager's and the user's own; its identities,
 * the manager's, where a provider is enabled. An integration's user is
 * offered no new key and no identity to assign or change: what it holds
 * is listed, to be revoked or removed.
 */
function UserPanel({ u }: { u: User }) {
  useLook();
  const ui = userUI.of(u.name);
  const manage = mayManage(u);
  const self = isMe(u.name);
  const locked = !manage && !self && !mayPassword(u);
  // the same functions for the panel's life, so that the identities' box reads by its own dependencies alone
  const changes = useMemo(() => changesOf(u.name), [u.name]);
  const links = (u.links || []).map((l) => l.display || l.name || (l.id || "").slice(0, 8)).join(", ");
  return (
    <Fold cls="panel user-panel" open={ui.open} onToggle={(o) => { ui.open = o; }} data={{ name: u.name }}
      summary={<FoldHead name={userName(u)} mono={u.integration ? "" : u.name} badges={badges(u)} />}>
      {links && <p class="muted small user-integration-links">{t("users.linksList", { users: links })}</p>}
      <UserForm u={u} off={locked} open={ui.open} />
      <Tokens u={u} off={!manage && !self} open={ui.open} reads={reads.value} run={run} say={usersMsg} labelId={"user-key-label-" + u.name}
        listOnly={!!u.integration} />
      <Identities u={u} off={!manage || !enabled.value.length} open={ui.open} reads={reads.value} providers={enabled.value} run={run} say={usersMsg}
        title={t("users.identities")} none={t("users.identitiesNone")} changes={u.integration ? undefined : changes} assign={!u.integration} />
      {locked && <p class="muted small">{t(u.role === "owner" ? "users.ownerOnly" : "users.outranked")}</p>}
    </Fold>
  );
}

/**
 * NewUser is the form that adds a user: its name, checked NAME_CHECK_MS
 * after the last keystroke and when the field is left, and Create sends
 * nothing while it is wrong; its display name, role, password and end.
 * What is typed is the form's own, until it is cancelled or the user is
 * added; the password never leaves it but registered by OPAQUE. The
 * password is typed once (AP PasswordRules, D51-A) and held to the rule,
 * its line under it; Create sends nothing while the rule refuses it and
 * puts the focus in it; left empty, the user has keys only, not checked.
 * The dice beside it suggests one (Block 4).
 */
function NewUser() {
  useLook();
  const [name, setName] = useState("");
  const [display, setDisplay] = useState("");
  const [chosen, setRole] = useState("");
  // the role chosen while it is offered, else the first default offered: who is in may change the roles offered (roleOptions)
  const offered = roleOptions().map(([k]) => k);
  const fallback = usersAdmin() === "guests" ? "guest" : "viewer";
  const role = offered.includes(chosen) ? chosen : offered.includes(fallback) ? fallback : offered[0] || fallback;
  const [password, setPassword] = useState("");
  const [expires, setExpires] = useState("");
  const nameRef = useRef<HTMLInputElement>(null);
  const passRef = useRef<HTMLInputElement>(null);
  const [refused, setRefused] = useState(0);
  const create = useBusy();
  const nc = useNameCheck(() => users.list.map((u) => u.name));
  const words = () => [name.trim(), display.trim()];
  const submit = (ev: Event) => {
    ev.preventDefault();
    if (create.busy) return; // a second submit while the creation runs, before the rule is asked
    if (!nc.check(name)) {
      nameRef.current?.focus();
      return;
    }
    // a password the rule refuses: nothing sent, its line saying so, the focus in it; empty is a user with keys only
    if (password && passwordRefusal(password, words())) {
      setRefused((r) => r + 1);
      passRef.current?.focus();
      return;
    }
    const body: Record<string, unknown> & { name: string } = { name: name.trim(), role };
    if (display.trim()) body.display_name = display.trim();
    const end = isoOf(expires);
    if (end) body.expires_at = end;
    create.run(() => createUser(body, password, nc.say)).then((ok) => {
      if (ok) adding.user = false;
    });
  };
  return (
    <form class="panel user-new" onSubmit={submit}>
      <UserNameField label={t("users.name")} id="user-new-name" msgId="user-new-name-msg" value={name} set={setName} nc={nc} name="name" autocomplete="off"
        placeholder={t("users.nameHint")} inputRef={nameRef} />
      <TextField label={t("users.displayName")} id="user-new-display_name" name="display_name" maxLength={64} value={display} set={setDisplay} />
      <Select label={t("users.role")} id="user-new-role" name="role" wide options={roleOptions()} value={role} onChange={setRole} />
      <PasswordField label={t("login.password")} id="user-new-password" name="password" autocomplete="new-password" placeholder={t("users.noPasswordHint")}
        value={password} set={setPassword} inputRef={passRef} rule={words} refused={refused} suggest />
      <Field label={t("users.expires")} id="user-new-expires" wide>
        <DateTime id="user-new-expires" cls="field-control" step={60} value={expires} set={setExpires} />
      </Field>
      <div class="actions">
        <span class="actions-right">
          <button type="button" class="form-cancel" onClick={() => { adding.user = false; }}>{t("action.cancel")}</button>
          <button type="submit" {...busyProps("primary", create.busy)}>{t("users.create")}</button>
        </span>
      </div>
    </form>
  );
}

/**
 * PasswordLogin is the password sign-in's switch, for a holder of
 * settings, saved at once and off while its request runs, and the line
 * that counts the users whose password the server still checks: with the
 * switch off they cannot sign in until a password is set for them here;
 * then the switch of the password fields' eye (AP MobileApp, D84-A), on by
 * default, saved so, with the line that says what it does.
 */
function PasswordLogin() {
  useLook();
  const on = useSetAtOnce(!!passwordLogin.value);
  const eye = useSetAtOnce(passwordReveal.value !== false);
  const list = users.list;
  const left = list.filter((u) => u.has_password && u.auth_version === 1).length;
  return (
    <div class="password-login needs-settings">
      <Switch id="password-login" label={t("users.passwordLogin")} checked={on.value} onChange={(v) => on.set(v, setPasswordLogin)} />
      <p id="password-login-hint" class="muted small">{users.loaded ? (left ? tn("users.passwordLoginHint", left) : t("users.passwordLoginNone")) : ""}</p>
      <Switch id="password-reveal" label={t("users.passwordReveal")} checked={eye.value} onChange={(v) => eye.set(v, setPasswordReveal)} />
      <p id="password-reveal-hint" class="muted small">{t("users.passwordRevealHint")}</p>
    </div>
  );
}

/**
 * OpenServer is the section on a server without users (AP FirstOwner,
 * D14, Block 4): a line that says so; Create the owner…, which opens the
 * sign-in card with the first owner's form where the owner can be created
 * from here, the focus back on it when the card is left; and while the
 * server is kept open the switch that undoes it (D5, D32): on; turned off
 * it is saved at once, and its row stays for this showing of the section,
 * off and no longer to be turned, with the line that the page asks for
 * the owner again at its next load, the focus on that line; turning it on
 * is the sign-in card's confirmation alone. Whether it is kept open is
 * shared/owner.ts keptOpen's answer, from who is in, read again after the
 * turning off (D44).
 */
function OpenServer() {
  useLang();
  const m = me.value;
  const kept = keptOpen(m);
  const off = keptOff.showing && !kept; // kept open again from the card meanwhile: the switch is back, on
  const on = useSetAtOnce(kept);
  const lineRef = useRef<HTMLParagraphElement>(null);
  const createRef = useRef<HTMLButtonElement>(null);
  // turned off: the switch can no longer be turned, so the focus goes to the line that says what follows
  useLayoutEffect(() => {
    if (off && document.activeElement && document.activeElement.id === "keep-open-switch") lineRef.current?.focus();
  }, [off]);
  return (
    <div id="users-open" class="users-open">
      <p id="users-no-owner" class="muted small">{t("users.noOwner")}</p>
      <div class={"actions" + hid(!firstOwnerOf(m))}>
        <button ref={createRef} type="button" id="users-create-owner" class="primary" onClick={() => askOwner(createRef.current)}>{t("setup.ask")}</button>
      </div>
      {(kept || off) && (
        <div class="users-kept-open needs-settings">
          <Switch id="keep-open-switch" label={t("users.keptOpen")} checked={!off && on.value} disabled={off}
            onChange={(v) => { if (!v) on.set(false, keepOpenOff); }} />
          <p id="keep-open-switch-hint" ref={lineRef} class="muted small" tabIndex={-1}>{t(off ? "users.keptOpenOff" : "users.keptOpenHint")}</p>
        </div>
      )}
    </div>
  );
}

/** UsersCard is the section's body: the password sign-in, the users' panels, the form that adds one, and the message line; on a server without users its own section instead of the three (OpenServer), and the line. */
export function UsersCard() {
  useLook();
  const list = users.list;
  const count = users.loaded ? tn("roles.users", list.length) : "";
  // the card's head counts them, in the page's language, once they are read; nothing once forgotten
  useLayoutEffect(() => {
    cardBadges("sys-users", count ? [{ text: count, kind: "mode" }] : []);
  }, [count]);
  if (me.value?.mode === "open") {
    return (
      <>
        <OpenServer />
        <ResultLine id="users-msg" cls="small cubes-msg" role="status" res={said.users} />
      </>
    );
  }
  return (
    <>
      <PasswordLogin />
      <div id="users-list">
        {users.loaded && !list.length && <p class="muted small">{t("users.none")}</p>}
        {list.map((u) => <UserPanel key={u.name} u={u} />)}
      </div>
      <Adder id="user-add" formId="user-new" label={t("users.add")} shown={adding.user} set={(v) => { adding.user = v; }}>
        <NewUser />
      </Adder>
      <ResultLine id="users-msg" cls="small cubes-msg" role="status" res={said.users} />
    </>
  );
}
