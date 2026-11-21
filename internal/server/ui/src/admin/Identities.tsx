// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/admin/Identities.tsx
 *
 * A user's identities at the sign-in providers in its panel of System
 * settings → Users, the manager's, as providers.js drew them: each with
 * its provider, its e-mail or user id there, whether it is bound or waits
 * for its first sign-in, what the provider said of it and when, Remove,
 * and Change of the e-mail or user id while no sign-in used it, a
 * settings form kept by the identity (shared/forms.ts); then Assign, by
 * an e-mail or by the user id at a provider. The list is read when the
 * panel opens and after every change of the users while it is open. My
 * profile's linked accounts draw the same rows plain (linked.js) until AP
 * UiParts' Block 13, which takes them to shared/.
 */

import { useEffect, useRef, useState } from "preact/hooks";
import type { Identity, User } from "../api";
import { Age } from "../shared/badge";
import { hid } from "../shared/cls";
import { FormFooter, Meanwhile, Select, TextField, useDraft } from "../shared/field";
import { focusForm, forgetForms, hasChanges, useForm } from "../shared/forms";
import { BusyButton } from "../shared/question";
import { enabled, identityFormKey, identityUI, load, reads, userFormKey, userUI } from "./store";
import { addIdentity, changeIdentity, loadIdentities, removeIdentity, usersMsg } from "./sendUsers";

/** Details is what the provider said of the account at its last sign-in or link: its name and user name, its groups, and when. */
function Details({ i }: { i: Identity }) {
  const parts: string[] = [];
  if (i.name) parts.push(i.name);
  if (i.username && i.username !== i.name) parts.push(i.username);
  if (i.email && i.subject && i.subject !== i.username) parts.push(t("users.identityUserID", { id: i.subject }));
  if (i.groups?.length) parts.push(t("users.identityGroups", { groups: i.groups.join(", ") }));
  return (
    <span class="muted small identity-details">
      {parts.length > 0 && parts.join(" · ") + " · "}
      {i.last_used_at ? <>{t("users.identityLastUsed") + " "}<Age at={i.last_used_at} plain /></> : t("users.neverUsed")}
    </span>
  );
}

/**
 * Change is the form that changes an identity's e-mail, or its user id
 * where it has no e-mail, while no sign-in used it: its draft kept by the
 * identity, Save by its state, Cancel back to the value stored and the
 * form closed, which gives the focus back to Change.
 */
function Change({ u, i, name, done }: { u: User; i: Identity; name: string; done: () => void }) {
  load.useLoad();
  const bySubject = !i.email;
  const key = identityFormKey(u.name, i.id);
  const form = useForm({
    key,
    name: { key: "users.identityFormName", args: { name, account: i.email || i.subject } },
    open: () => {
      if (!isShown("system")) showView("system");
      showCard($("sys-users")); // a folded card unfolded, its form in sight
      userUI.of(u.name).open = true;
      identityUI.of(key).changing = true;
      focusForm(key);
    },
  });
  const account = useDraft(bySubject ? i.subject : i.email, (v) => v.trim(), form.place("account"));
  form.track(account.changed, [account]);
  const submit = (ev: Event) => {
    ev.preventDefault();
    const value = account.value.trim();
    if (!value) {
      usersMsg(t(bySubject ? "users.identitySubjectNeeded" : "users.identityEmailNeeded"), false);
      return;
    }
    // taken, the form closes, as the plain one did
    form.save(() => changeIdentity(u.name, i, bySubject, value).then((ok) => {
      if (ok) done();
      return ok;
    }));
  };
  return (
    <>
      <form {...form.props} class="identity-change" onSubmit={submit}>
        <input type="text" name="account" class="field-control" autocomplete="off" spellcheck={false}
          aria-label={t(bySubject ? "users.identitySubject" : "users.identityEmail")} value={account.value} onInput={(ev) => account.set(ev.currentTarget.value)} />
        <FormFooter submit={t("action.save")} look={form.look} off={!account.changed} cancel={() => {
          form.reset();
          done();
        }} />
      </form>
      <Meanwhile draft={account} />
    </>
  );
}

/** Row is one of a user's identities, with Remove and, for one no sign-in used yet, Change. */
function Row({ u, i, name }: { u: User; i: Identity; name: string }) {
  const key = identityFormKey(u.name, i.id);
  const ui = identityUI.of(key);
  const changeRef = useRef<HTMLButtonElement>(null);
  const back = useRef(false);
  const known = enabled.value.find((p) => p.key === i.provider);
  const provider = known ? known.name : i.provider;
  const account = i.email || t("users.identityUserID", { id: i.subject });
  const changeable = !i.last_used_at;
  const changing = changeable && ui.changing;
  // Cancel gives the focus back to Change, once it is shown again
  useEffect(() => {
    if (!changing && back.current) {
      back.current = false;
      changeRef.current?.focus();
    }
  }, [changing]);
  return (
    <li data-id={String(i.id)}>
      <span class="flag mode identity-provider">{provider}</span>
      <span class="identity-email">{account}</span>
      <span class="muted small identity-state">{t(i.bound ? "users.identityBound" : "users.identityWaiting")}</span>
      <span class="identity-buttons">
        <button ref={changeRef} type="button" class={"change-identity" + hid(!changeable || changing)}
          aria-label={t("users.identityChangeLabel", { email: account, provider })} onClick={() => { ui.changing = true; }}>
          {t("users.identityChange")}
        </button>
        <BusyButton cls="remove-identity" label={t("users.identityRemove")} aria={t("users.identityRemoveLabel", { email: account, provider })}
          run={() => removeIdentity(u.name, i)} />
      </span>
      <Details i={i} />
      {changing && <Change u={u} i={i} name={name} done={() => {
        back.current = true;
        ui.changing = false;
      }} />}
    </li>
  );
}

/**
 * Assign is the choice of a provider, by e-mail or by the user id there,
 * and the account, with Assign; what is typed is the part's own.
 */
function Assign({ u }: { u: User }) {
  const list = enabled.value;
  const [chosen, setChosen] = useState("");
  const [kind, setKind] = useState("email");
  const [account, setAccount] = useState("");
  const provider = list.some((p) => p.key === chosen) ? chosen : list[0]?.key || "";
  const bySubject = kind === "subject";
  const id = (what: string) => "user-identity-" + what + "-" + u.name;
  return (
    <>
      <Select label={t("users.identityProvider")} id={id("provider")} name="provider" wide options={list.map((p) => [p.key, p.name])} value={provider} onChange={setChosen} />
      <Select label={t("users.identityKind")} id={id("kind")} name="kind" wide options={[["email", t("users.identityByEmail")], ["subject", t("users.identityBySubject")]]}
        value={kind} onChange={setKind} />
      <TextField label={t(bySubject ? "users.identitySubject" : "users.identityEmail")} id={id("account")} name="account" type={bySubject ? "text" : "email"}
        value={account} set={setAccount} />
      <div class="actions">
        <span class="actions-right">
          <BusyButton cls="add-identity" label={t("users.identityAdd")} run={async () => {
            const value = account.trim();
            if (!value) {
              usersMsg(t(bySubject ? "users.identitySubjectNeeded" : "users.identityEmailNeeded"), false);
              return;
            }
            if (await addIdentity(u.name, provider, bySubject, value)) setAccount("");
          }} />
        </span>
      </div>
    </>
  );
}

/**
 * Identities is the box, hidden where off (the manager's, where a
 * provider is enabled); read while open says the panel is.
 */
export function Identities({ u, off, open }: { u: User; off: boolean; open: boolean }) {
  load.useLoad();
  const n = reads.value;
  const [list, setList] = useState<Identity[] | null>(null);
  const [error, setError] = useState("");
  const shown = useRef<Identity[]>([]);
  // read when the panel opens and at every change of the users while it is open; the change forms of
  // those it no longer gives, or that a sign-in made unchangeable, are forgotten with what they hold
  // biome-ignore lint/correctness/useExhaustiveDependencies: n is the users' change counter, which asks for the read again by its change alone.
  useEffect(() => {
    if (off) {
      // the box off (no provider enabled any more): what its change forms held goes, as their identities may be gone
      forgetForms(userFormKey(u.name) + ":identity");
      for (const was of shown.current) identityUI.forget(identityFormKey(u.name, was.id));
      shown.current = [];
      return;
    }
    // closed, it reads again only where one of its change forms holds a draft, so that a gone identity's form is forgotten
    if (!open && !hasChanges(userFormKey(u.name) + ":identity")) return;
    let stale = false;
    loadIdentities(u.name).then((l) => {
      if (stale) return;
      for (const was of shown.current) {
        if (l.some((x) => x.id === was.id && !x.last_used_at)) continue;
        const key = identityFormKey(u.name, was.id);
        forgetForms(key);
        identityUI.forget(key);
      }
      shown.current = l;
      setList(l);
      setError("");
    }, (e: Error) => {
      if (stale) return;
      setList(null);
      setError(e.message);
    });
    return () => {
      stale = true;
    };
  }, [open, off, n, u.name]);
  const name = u.display || u.display_name || u.name;
  return (
    <div class={"user-identities-box" + hid(off)}>
      <span class="muted small">{t("users.identities")}</span>
      <ul class="user-identities">
        {error && <li class="muted small bad">{error}</li>}
        {list && !list.length && <li class="muted small">{t("users.identitiesNone")}</li>}
        {(list || []).map((i) => <Row key={i.id} u={u} i={i} name={name} />)}
      </ul>
      {!off && <Assign u={u} />}
    </div>
  );
}
