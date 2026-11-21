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

import { useEffect, useMemo, useRef, useState } from "preact/hooks";
import type { User } from "../api";
import { Adder } from "../shared/adder";
import { Badge, ResultLine } from "../shared/badge";
import { hid } from "../shared/cls";
import { DateTime, Field, FormFooter, Select, Switch, TextField, useDraft } from "../shared/field";
import { Fold, FoldHead } from "../shared/fold";
import { focusForm, useForm } from "../shared/forms";
import { Question, busyProps, useAsk, useBusy, useSetAtOnce } from "../shared/question";
import { isoOf, localOf } from "../shared/time";
import { type Changes, Identities } from "../shared/identities";
import { Tokens } from "../shared/tokens";
import { adding, enabled, identityFormKey, identityUI, load, passwordLogin, reads, said, userFormKey, userUI, users } from "./store";
import { createUser, mayManage, mayPassword, removeUser, roleOptions, run, saveUser, setPasswordLogin, usersAdmin, usersMsg } from "./sendUsers";
import { cardBadges, showCard } from "../shared/card";
import { isShown, showView } from "../shared/pages";

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
  if (isSelf(u.name)) out.push(<Badge key="you" kind="ok" text={t("users.you")} />);
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
 * UserForm is a user's settings: its display name, and for its manager
 * its role, whether it is disabled and a guest's end; a new password and,
 * for one's own without managing it, the current one; Remove, which asks
 * in place of the actions, Reset and Save. The fields that show what is
 * stored keep their drafts by the user's name; Save sends those that
 * differ from the user as the list last gave it.
 */
function UserForm({ u, off }: { u: User; off: boolean }) {
  load.useLoad();
  const manage = mayManage(u);
  const self = isSelf(u.name);
  const form = useForm({ key: userFormKey(u.name), name: { key: "users.formName", args: { name: userName(u) } }, open: () => openUser(u.name) });
  const display = useDraft(u.display_name || "", trim, form.place("display_name"));
  const role = useDraft(u.role, undefined, form.place("role"));
  const disabled = useDraft(!!u.disabled, undefined, form.place("disabled"));
  const expires = useDraft(localOf(u.expires_at), undefined, form.place("expires"));
  // shown once and never kept: the part's own state, not the forms' store
  const [password, setPassword] = useState("");
  const [current, setCurrent] = useState("");
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
    if (!changed) {
      usersMsg(t("users.unchanged"), true);
      return;
    }
    form.save(() => saveUser(u, { ...body }, password, self && !manage ? current : null));
  };
  const id = (field: string) => "user-" + u.name + "-" + field;
  const admin = "admin-only" + hid(!manage);
  return (
    <form {...form.props} class={"user-form" + hid(off)} onSubmit={submit}>
      <TextField label={t("users.displayName")} id={id("display_name")} name="display_name" maxLength={64} draft={display} cls={hid(!manage && !self).trim() || undefined} />
      <Select label={t("users.role")} id={id("role")} name="role" wide cls={admin} options={roleOptions(u.role)} draft={role} show={(k) => roleLabel(k)} />
      <Switch name="disabled" label={t("users.disabled")} rowCls={admin} draft={disabled} />
      <Field label={t("users.expires")} id={id("expires")} wide cls={admin} hint={t("users.expiresHint")} draft={expires}
        show={(v) => (v ? fmtShortTime(Date.parse(v)) : t("users.noEnd"))}>
        <DateTime id={id("expires")} cls="field-control" step={60} value={expires.value} set={expires.set} />
      </Field>
      <TextField label={t("users.password")} id={id("password")} name="password" type="password" autocomplete="new-password" placeholder={t("users.passwordHint")}
        value={password} set={(v) => {
          setPassword(v);
          form.place("password").changed();
        }} />
      <TextField label={t("users.currentPassword")} id={id("current_password")} name="current_password" type="password" autocomplete="current-password"
        cls={"field-current" + hid(manage || !self)} value={current} set={setCurrent} />
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
 * the manager's, where a provider is enabled.
 */
function UserPanel({ u }: { u: User }) {
  load.useLoad();
  const ui = userUI.of(u.name);
  const manage = mayManage(u);
  const self = isSelf(u.name);
  const locked = !manage && !self && !mayPassword(u);
  // the same functions for the panel's life, so that the identities' box reads by its own dependencies alone
  const changes = useMemo(() => changesOf(u.name), [u.name]);
  const links = (u.links || []).map((l) => l.display || l.name || (l.id || "").slice(0, 8)).join(", ");
  return (
    <Fold cls="panel user-panel" open={ui.open} onToggle={(o) => { ui.open = o; }} data={{ name: u.name }}
      summary={<FoldHead name={userName(u)} mono={u.integration ? "" : u.name} badges={badges(u)} />}>
      {links && <p class="muted small user-integration-links">{t("users.linksList", { users: links })}</p>}
      <UserForm u={u} off={locked} />
      <Tokens u={u} off={!manage && !self} open={ui.open} reads={reads.value} run={run} say={usersMsg} labelId={"user-key-label-" + u.name} />
      <Identities u={u} off={!manage || !enabled.value.length} open={ui.open} reads={reads.value} providers={enabled.value} run={run} say={usersMsg}
        title={t("users.identities")} none={t("users.identitiesNone")} changes={changes} assign />
      {locked && <p class="muted small">{t(u.role === "owner" ? "users.ownerOnly" : "users.outranked")}</p>}
    </Fold>
  );
}

/**
 * NewUser is the form that adds a user: its name, checked NAME_CHECK_MS
 * after the last keystroke and when the field is left, and Create sends
 * nothing while it is wrong; its display name, role, password and end.
 * What is typed is the form's own, until it is cancelled or the user is
 * added; the password never leaves it but registered by OPAQUE.
 */
function NewUser() {
  load.useLoad();
  const [name, setName] = useState("");
  const [display, setDisplay] = useState("");
  const [role, setRole] = useState(usersAdmin() === "guests" ? "guest" : "viewer");
  const [password, setPassword] = useState("");
  const [expires, setExpires] = useState("");
  const [problem, setProblem] = useState("");
  const timer = useRef<ReturnType<typeof setTimeout> | undefined>(undefined);
  const nameRef = useRef<HTMLInputElement>(null);
  const create = useBusy();
  useEffect(() => () => clearTimeout(timer.current), []);
  const taken = () => users.list.map((u) => u.name);
  const check = (v: string = name) => {
    clearTimeout(timer.current);
    const p = nameProblem(v, taken());
    setProblem(p);
    return !p;
  };
  const submit = (ev: Event) => {
    ev.preventDefault();
    if (!check()) {
      nameRef.current?.focus();
      return;
    }
    const body: Record<string, unknown> & { name: string } = { name: name.trim(), role };
    if (display.trim()) body.display_name = display.trim();
    const end = isoOf(expires);
    if (end) body.expires_at = end;
    create.run(() => createUser(body, password, setProblem)).then((ok) => {
      if (ok) adding.user = false;
    });
  };
  const msgId = "user-new-name-msg";
  return (
    <form class="panel user-new" onSubmit={submit}>
      <Field label={t("users.name")} id="user-new-name" wide>
        <input ref={nameRef} id="user-new-name" type="text" name="name" class="field-control" autocomplete="off" autocapitalize="none" spellcheck={false}
          maxLength={64} required placeholder={t("users.nameHint")} value={name} aria-invalid={problem ? "true" : undefined} aria-describedby={msgId}
          onInput={(ev) => {
            const v = ev.currentTarget.value;
            setName(v);
            clearTimeout(timer.current);
            if (!nameProblem(v, taken())) setProblem("");
            else timer.current = setTimeout(() => check(v), NAME_CHECK_MS);
          }}
          onBlur={() => check()} />
        <ResultLine id={msgId} cls="small cubes-msg field-hint name-msg" role="status" res={problem ? { text: problem, ok: false } : null} />
      </Field>
      <TextField label={t("users.displayName")} id="user-new-display_name" name="display_name" maxLength={64} value={display} set={setDisplay} />
      <Select label={t("users.role")} id="user-new-role" name="role" wide options={roleOptions()} value={role} onChange={setRole} />
      <TextField label={t("login.password")} id="user-new-password" name="password" type="password" autocomplete="new-password" placeholder={t("users.noPasswordHint")}
        value={password} set={setPassword} />
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
 * switch off they cannot sign in until a password is set for them here.
 */
function PasswordLogin() {
  load.useLoad();
  const on = useSetAtOnce(!!passwordLogin.value);
  const list = users.list;
  const left = list.filter((u) => u.has_password && u.auth_version === 1).length;
  return (
    <div class="password-login needs-settings">
      <Switch id="password-login" label={t("users.passwordLogin")} checked={on.value} onChange={(v) => on.set(v, setPasswordLogin)} />
      <p id="password-login-hint" class="muted small">{users.loaded ? (left ? tn("users.passwordLoginHint", left) : t("users.passwordLoginNone")) : ""}</p>
    </div>
  );
}

/** UsersCard is the section's body: the password sign-in, the users' panels, the form that adds one, and the message line. */
export function UsersCard() {
  load.useLoad();
  const list = users.list;
  const count = users.loaded ? tn("roles.users", list.length) : "";
  // the card's head counts them, in the page's language, once they are read; nothing once forgotten
  useEffect(() => {
    cardBadges("sys-users", count ? [{ text: count, kind: "mode" }] : []);
  }, [count]);
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
